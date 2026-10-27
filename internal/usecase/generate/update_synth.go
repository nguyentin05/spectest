package generate

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/nguyentin05/spectest/internal/domain"
)

// computeSetupScopes builds the cumulative exported variable scope for each setup in the suite.
func computeSetupScopes(seeds map[string]domain.Seed, setups map[string]domain.Setup) map[string]map[string]struct{} {
	seedNames, _ := seedScope(seeds)
	setupOrder, err := topoSetupOrder(setups)
	if err != nil {
		setupOrder = make([]string, 0, len(setups))
		for k := range setups {
			setupOrder = append(setupOrder, k)
		}
	}
	scopes := make(map[string]map[string]struct{}, len(setups))
	for _, name := range setupOrder {
		st := setups[name]
		scope := cloneScope(seedNames)
		for _, u := range st.Use {
			if depScope, ok := scopes[u]; ok {
				mergeScope(scope, depScope)
			}
		}
		for v := range st.Vars {
			scope[v] = struct{}{}
		}
		for _, step := range st.Steps {
			for saveName := range step.Save {
				if strings.TrimSpace(saveName) != "" {
					scope[saveName] = struct{}{}
				}
			}
		}
		scopes[name] = scope
	}
	return scopes
}

var nonAlphanumericRe = regexp.MustCompile(`[^a-z0-9]+`)

func sanitizeIDPart(s string) string {
	s = strings.ToLower(s)
	s = nonAlphanumericRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func deriveScenarioID(apiKey string, existing map[string]bool) string {
	pc, err := parseCall(apiKey)
	if err != nil {
		pc = parsedCall{Method: "CALL", Server: "api", Path: "/endpoint"}
	}
	method := strings.ToLower(pc.Method)
	path := sanitizeIDPart(pc.Path)
	if path == "" {
		path = "root"
	}
	baseID := fmt.Sprintf("%s-%s", method, path)
	if pc.Server != "" && pc.Server != "default" {
		baseID = fmt.Sprintf("%s-%s-%s", sanitizeIDPart(pc.Server), method, path)
	}
	baseID = strings.Trim(baseID, "-")
	if baseID == "" {
		baseID = "new-scenario"
	}

	cand := baseID
	counter := 1
	for existing[cand] {
		counter++
		cand = fmt.Sprintf("%s-%d", baseID, counter)
	}
	return cand
}

// repairSetup calls the LLM to rewrite a setup's steps/vars for incompatible API changes.
func (g *Generator) repairSetup(
	ctx context.Context,
	suite *domain.Suite,
	src *domain.SourceSet,
	act UpdateAction,
) (domain.Setup, error) {
	orig, exists := suite.Setup[act.SetupName]
	if !exists {
		return domain.Setup{}, fmt.Errorf("setup %q not found in suite", act.SetupName)
	}

	currentAPIs := make(map[string]domain.API, len(src.APIs))
	for _, a := range src.APIs {
		currentAPIs[a.Key] = a
	}
	serverSet := make(map[string]struct{}, len(src.Servers))
	for _, s := range src.Servers {
		serverSet[s] = struct{}{}
	}

	seedNames, bareIdx := seedScope(suite.Seed)
	setupScopes := computeSetupScopes(suite.Seed, suite.Setup)

	parent := cloneScope(seedNames)
	for _, u := range orig.Use {
		if s, ok := setupScopes[u]; ok {
			mergeScope(parent, s)
		}
	}

	deps := collectDependencySetups(orig.Use, suite.Setup)
	allowed := make(map[string]domain.API)
	for _, k := range act.APIKeys {
		if api, ok := currentAPIs[k]; ok {
			allowed[k] = api
		}
	}
	for _, st := range orig.Steps {
		if api, ok, _ := matchAPI(currentAPIs, st.Call); ok {
			allowed[api.Key] = api
		}
	}

	ps := planSetup{
		Name:    act.SetupName,
		Use:     orig.Use,
		Purpose: fmt.Sprintf("Repair setup %s for updated API contracts", act.SetupName),
		APIs:    act.APIKeys,
	}

	user, err := marshalPayload(struct {
		Mode        string                 `json:"mode"`
		Seed        map[string]domain.Seed `json:"seed,omitempty"`
		Setup       promptSetupOutline     `json:"setup"`
		Deps        []promptSetupRef       `json:"dependencies,omitempty"`
		AllowedAPIs []promptAPI            `json:"allowed_apis"`
	}{
		"repair_setup", suite.Seed,
		promptSetupOutline{Name: ps.Name, Use: ps.Use, Purpose: ps.Purpose, APIs: ps.APIs},
		promptRefsFrom(deps), promptAPIsFromMap(allowed),
	})
	if err != nil {
		return domain.Setup{}, fmt.Errorf("build prompt: %w", err)
	}

	out, err := callValidated(ctx, g.llm, "repair setup "+act.SetupName, updateSetupSystemPrompt, user,
		func(v *synthesizedSetup) error {
			setupNames := append([]string(nil), orig.Use...)
			for k, val := range v.Vars {
				v.Vars[k] = sanitizeSetupTemplates(val, setupNames)
			}
			for idx := range v.Steps {
				v.Steps[idx] = sanitizeStep(v.Steps[idx], "", serverSet)
				v.Steps[idx].Body = sanitizeSetupTemplates(v.Steps[idx].Body, setupNames)
				if v.Steps[idx].Headers != nil {
					for hk, hv := range v.Steps[idx].Headers {
						v.Steps[idx].Headers[hk] = sanitizeSetupTemplates(hv, setupNames).(string)
					}
				}
			}

			var errs []error
			for k, val := range v.Vars {
				errs = append(errs, checkTemplates(parent, bareIdx,
					fmt.Sprintf("setup %q var %q", ps.Name, k), val)...)
			}
			stepScope := cloneScope(parent)
			for k := range v.Vars {
				stepScope[k] = struct{}{}
			}
			stepErrs, _ := validateStepsDeep("setup "+ps.Name, v.Steps, allowed, serverSet, stepScope, bareIdx)
			errs = append(errs, stepErrs...)

			called := calledKeys(v.Steps, allowed)
			for _, k := range act.APIKeys {
				if _, ok := called[k]; !ok {
					errs = append(errs, fmt.Errorf("setup %q must call api %q", ps.Name, k))
				}
			}
			return errors.Join(errs...)
		})
	if err != nil {
		return domain.Setup{}, err
	}

	return domain.Setup{
		Use:   orig.Use,
		Vars:  out.Vars,
		Steps: out.Steps,
	}, nil
}

// repairScenario calls the LLM to rewrite a scenario's steps for incompatible API changes.
func (g *Generator) repairScenario(
	ctx context.Context,
	suite *domain.Suite,
	src *domain.SourceSet,
	act UpdateAction,
) (domain.Scenario, error) {
	var orig domain.Scenario
	found := false
	for _, sc := range suite.Scenarios {
		if sc.ID == act.ScenarioID {
			orig = sc
			found = true
			break
		}
	}
	if !found {
		return domain.Scenario{}, fmt.Errorf("scenario %q not found in suite", act.ScenarioID)
	}

	currentAPIs := make(map[string]domain.API, len(src.APIs))
	for _, a := range src.APIs {
		currentAPIs[a.Key] = a
	}
	serverSet := make(map[string]struct{}, len(src.Servers))
	for _, s := range src.Servers {
		serverSet[s] = struct{}{}
	}

	seedNames, bareIdx := seedScope(suite.Seed)
	setupScopes := computeSetupScopes(suite.Seed, suite.Setup)

	parent := cloneScope(seedNames)
	for _, u := range orig.Setup {
		if s, ok := setupScopes[u]; ok {
			mergeScope(parent, s)
		}
	}

	allowed := make(map[string]domain.API)
	for _, k := range act.APIKeys {
		if api, ok := currentAPIs[k]; ok {
			allowed[k] = api
		}
	}
	for _, st := range orig.Steps {
		if api, ok, _ := matchAPI(currentAPIs, st.Call); ok {
			allowed[api.Key] = api
		}
	}

	closure, err := setupClosure(orig.Setup, suite.Setup)
	if err != nil {
		return domain.Scenario{}, fmt.Errorf("scenario %q setup closure: %w", orig.ID, err)
	}

	pjob := promptScenarioJob{
		Scenario: promptScenarioOutline{
			ID:      orig.ID,
			Name:    orig.Name,
			Setup:   orig.Setup,
			Purpose: fmt.Sprintf("Repair scenario %s for updated API contracts. Reason: %s", orig.ID, act.Reason),
			APIs:    act.APIKeys,
		},
		SetupClosure: promptRefsFrom(closure),
		AllowedAPIs:  promptAPIsFromMap(allowed),
		MustCall:     act.APIKeys,
	}

	user, err := marshalPayload(struct {
		Mode      string              `json:"mode"`
		Scenarios []promptScenarioJob `json:"scenarios"`
	}{
		"repair_scenario",
		[]promptScenarioJob{pjob},
	})
	if err != nil {
		return domain.Scenario{}, fmt.Errorf("build prompt: %w", err)
	}

	out, err := callValidated(ctx, g.llm, "repair scenario "+orig.ID, updateScenarioSystemPrompt, user,
		func(v *synthesizedBatch) error {
			if len(v.Scenarios) != 1 {
				return fmt.Errorf("expected 1 scenario in repair response, got %d", len(v.Scenarios))
			}
			sc := &v.Scenarios[0]
			setupNames := append([]string(nil), orig.Setup...)
			for _, c := range closure {
				setupNames = append(setupNames, c.Name)
			}
			for idx := range sc.Steps {
				sc.Steps[idx] = sanitizeStep(sc.Steps[idx], "", serverSet)
				sc.Steps[idx].Body = sanitizeSetupTemplates(sc.Steps[idx].Body, setupNames)
				if sc.Steps[idx].Headers != nil {
					for hk, hv := range sc.Steps[idx].Headers {
						sc.Steps[idx].Headers[hk] = sanitizeSetupTemplates(hv, setupNames).(string)
					}
				}
			}

			stepErrs, _ := validateStepsDeep("scenario "+orig.ID, sc.Steps, allowed, serverSet, parent, bareIdx)
			if len(stepErrs) > 0 {
				return errors.Join(stepErrs...)
			}
			called := calledKeys(sc.Steps, allowed)
			for _, k := range act.APIKeys {
				if _, ok := called[k]; !ok {
					return fmt.Errorf("scenario %q must call api %q", orig.ID, k)
				}
			}
			return nil
		})
	if err != nil {
		return domain.Scenario{}, err
	}

	return domain.Scenario{
		ID:     orig.ID,
		Name:   orig.Name,
		Setup:  orig.Setup,
		Steps:  out.Scenarios[0].Steps,
		Locked: orig.Locked,
		Skip:   orig.Skip,
	}, nil
}

// createScenarios calls the LLM to synthesize new scenarios covering newly added APIs.
func (g *Generator) createScenarios(
	ctx context.Context,
	suite *domain.Suite,
	src *domain.SourceSet,
	apiKeys []string,
) ([]domain.Scenario, error) {
	currentAPIs := make(map[string]domain.API, len(src.APIs))
	for _, a := range src.APIs {
		currentAPIs[a.Key] = a
	}
	serverSet := make(map[string]struct{}, len(src.Servers))
	for _, s := range src.Servers {
		serverSet[s] = struct{}{}
	}

	existingIDs := make(map[string]bool, len(suite.Scenarios))
	for _, sc := range suite.Scenarios {
		existingIDs[sc.ID] = true
	}

	seedNames, bareIdx := seedScope(suite.Seed)

	var newScenarios []domain.Scenario
	for _, apiKey := range apiKeys {
		api, ok := currentAPIs[apiKey]
		if !ok {
			continue
		}

		scID := deriveScenarioID(apiKey, existingIDs)
		scName := fmt.Sprintf("%s %s", api.Method, api.Path)

		allowed := map[string]domain.API{api.Key: api}
		pjob := promptScenarioJob{
			Scenario: promptScenarioOutline{
				ID:      scID,
				Name:    scName,
				Setup:   nil,
				Purpose: fmt.Sprintf("Verify newly added API %s", api.Key),
				APIs:    []string{api.Key},
			},
			SetupClosure: nil,
			AllowedAPIs:  promptAPIsFromMap(allowed),
			MustCall:     []string{api.Key},
		}

		user, err := marshalPayload(struct {
			Mode      string              `json:"mode"`
			Scenarios []promptScenarioJob `json:"scenarios"`
		}{
			"create_scenario",
			[]promptScenarioJob{pjob},
		})
		if err != nil {
			return nil, fmt.Errorf("build prompt: %w", err)
		}

		out, err := callValidated(ctx, g.llm, "create scenario "+scID, updateScenarioSystemPrompt, user,
			func(v *synthesizedBatch) error {
				if len(v.Scenarios) != 1 {
					return fmt.Errorf("expected 1 scenario in create response, got %d", len(v.Scenarios))
				}
				sc := v.Scenarios[0]
				stepErrs, _ := validateStepsDeep("scenario "+scID, sc.Steps, allowed, serverSet, seedNames, bareIdx)
				if len(stepErrs) > 0 {
					return errors.Join(stepErrs...)
				}
				called := calledKeys(sc.Steps, allowed)
				if _, ok := called[api.Key]; !ok {
					return fmt.Errorf("scenario %q must call api %q", scID, api.Key)
				}
				return nil
			})
		if err != nil {
			return nil, err
		}

		actualID := scID
		if len(out.Scenarios) > 0 && out.Scenarios[0].ID != "" && validateIDRe.MatchString(out.Scenarios[0].ID) && !existingIDs[out.Scenarios[0].ID] {
			actualID = out.Scenarios[0].ID
		}
		existingIDs[actualID] = true

		newScenarios = append(newScenarios, domain.Scenario{
			ID:    actualID,
			Name:  scName,
			Steps: out.Scenarios[0].Steps,
		})
	}

	return newScenarios, nil
}

func sanitizeSetupTemplates(v any, setupNames []string) any {
	if len(setupNames) == 0 || v == nil {
		return v
	}
	switch val := v.(type) {
	case string:
		res := val
		for _, sName := range setupNames {
			prefix := "{{" + sName + "."
			if strings.Contains(res, prefix) {
				res = strings.ReplaceAll(res, prefix, "{{")
			}
		}
		return res
	case map[string]any:
		res := make(map[string]any, len(val))
		for k, inner := range val {
			res[k] = sanitizeSetupTemplates(inner, setupNames)
		}
		return res
	case []any:
		res := make([]any, len(val))
		for i, inner := range val {
			res[i] = sanitizeSetupTemplates(inner, setupNames)
		}
		return res
	default:
		return v
	}
}
