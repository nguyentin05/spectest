package generate

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/nguyentin05/spectest/internal/domain"
)

var nonAlphanum = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func slugifyID(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlphanum.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// buildFacts compiles authoritative code-managed facts from the parsed SourceSet.
func buildFacts(src *domain.SourceSet) map[string]*apiFacts {
	facts := make(map[string]*apiFacts, len(src.APIs))
	for _, api := range src.APIs {
		docStatuses := make([]string, 0, len(api.Responses))
		respFacts := make([]responseFacts, 0, len(api.Responses))
		for statusStr, respSchema := range api.Responses {
			docStatuses = append(docStatuses, statusStr)
			respFacts = append(respFacts, responseFacts{
				Status:         statusStr,
				BodyDocumented: documentsBody(respSchema),
			})
		}
		sort.Strings(docStatuses)

		params := make([]paramFacts, 0, len(api.Parameters))
		for _, p := range api.Parameters {
			params = append(params, paramFacts{
				Name:     p.Name,
				In:       p.In,
				Required: p.Required,
			})
		}

		var reqFields []string
		hasReqBody := false
		if api.Request != nil {
			hasReqBody = true
			if api.Request.JSON && !api.Request.Schema.Unsupported {
				for propName := range api.Request.Schema.Properties {
					reqFields = append(reqFields, propName)
				}
				sort.Strings(reqFields)
			}
		}

		var respFields []string
		for _, rf := range respFacts {
			if rf.BodyDocumented {
				// extract top-level field hints if available
				if schema, ok := api.Responses[rf.Status]; ok {
					for propName := range schema.Properties {
						respFields = append(respFields, propName)
					}
				}
			}
		}
		sort.Strings(respFields)

		authList := append([]string(nil), api.Sig.Auth...)

		facts[api.Key] = &apiFacts{
			Key:                api.Key,
			Method:             api.Method,
			Server:             api.Server,
			Path:               api.Path,
			Description:        api.Description,
			Auth:               authList,
			Parameters:         params,
			HasRequestBody:     hasReqBody,
			RequestBodyFields:  reqFields,
			ResponseFields:     respFields,
			DocumentedStatuses: docStatuses,
			Responses:          respFacts,
			Sig:                api.Sig,
			RawSnippet:         api.RawSnippet,
		}
	}
	return facts
}

func documentsBody(schema domain.Schema) bool {
	if schema.Unsupported {
		return false
	}
	if schema.Type != "" || len(schema.Properties) > 0 || schema.Items != nil {
		return true
	}
	return false
}

func documentedStatus(statuses []string, code int) bool {
	codeStr := fmt.Sprintf("%d", code)
	prefix := fmt.Sprintf("%dXX", code/100)
	for _, s := range statuses {
		if s == codeStr || s == prefix || s == "default" {
			return true
		}
	}
	return false
}

func groundSetups(
	rawSetups map[string]rawSetupPlan,
	facts map[string]*apiFacts,
	apis map[string]domain.API,
	serverSet map[string]struct{},
	seed map[string]domain.Seed,
) (map[string]domain.Setup, []unresolvedItem, []string) {
	valid := make(map[string]domain.Setup)
	var unresolved []unresolvedItem
	var warns []string

	// 1. Filter names & dependency cycles
	type node struct {
		name string
		use  []string
		raw  rawSetupPlan
	}
	nodes := make(map[string]node)
	for name, sp := range rawSetups {
		name = strings.TrimSpace(name)
		if !validateNameRe.MatchString(name) || len(sp.Steps) == 0 {
			warns = append(warns, fmt.Sprintf("dropped setup %q: invalid name or empty steps", name))
			continue
		}
		nodes[name] = node{name: name, use: sp.Use, raw: sp}
	}

	// Topological sort for cycles
	visiting := make(map[string]bool)
	visited := make(map[string]bool)
	var topoOrder []string
	var hasCycle bool

	var visit func(n string) bool
	visit = func(n string) bool {
		if visiting[n] {
			return false // cycle
		}
		if visited[n] {
			return true
		}
		visiting[n] = true
		for _, u := range nodes[n].use {
			if _, exists := nodes[u]; !exists {
				warns = append(warns, fmt.Sprintf("setup %q uses unknown setup %q", n, u))
				return false
			}
			if !visit(u) {
				return false
			}
		}
		visiting[n] = false
		visited[n] = true
		topoOrder = append(topoOrder, n)
		return true
	}

	for n := range nodes {
		if !visited[n] {
			if !visit(n) {
				hasCycle = true
			}
		}
	}
	if hasCycle {
		warns = append(warns, "cycle detected in setup dependencies; dropped cyclic setups")
		return valid, unresolved, warns
	}

	// 2. Validate step-by-step
	seedNames, bareSeedIdx := seedScope(seed)
	setupScopes := make(map[string]map[string]struct{})

	for _, name := range topoOrder {
		nd := nodes[name]
		parentScope := cloneScope(seedNames)
		depFailed := false
		for _, u := range nd.use {
			if depScope, ok := setupScopes[u]; ok {
				mergeScope(parentScope, depScope)
			} else {
				depFailed = true
				break
			}
		}
		if depFailed {
			warns = append(warns, fmt.Sprintf("setup %q dropped because dependency failed validation", name))
			unresolved = append(unresolved, unresolvedItem{API: "*", Missing: "dependency failure", Affects: name})
			continue
		}

		stepScope := cloneScope(parentScope)
		for k := range nd.raw.Vars {
			stepScope[k] = struct{}{}
		}

		// Sanitize steps (e.g. fix single-brace {var} -> {{var}}, fix server alias)
		for idx := range nd.raw.Steps {
			nd.raw.Steps[idx] = sanitizeStep(nd.raw.Steps[idx], "", serverSet)
		}

		// Check save body documented
		saveFailed := false
		for idx, st := range nd.raw.Steps {
			matchedAPI, ok, err := matchAPI(apis, st.Call)
			if !ok || err != nil {
				warns = append(warns, fmt.Sprintf("setup %q step %d calls unknown API %q", name, idx+1, st.Call))
				saveFailed = true
				break
			}
			f, ok := facts[matchedAPI.Key]
			if !ok {
				warns = append(warns, fmt.Sprintf("setup %q step %d calls unindexed API %q", name, idx+1, st.Call))
				saveFailed = true
				break
			}
			for _, expr := range st.Save {
				if strings.HasPrefix(expr, "body.") {
					bodyDoc := false
					for _, rf := range f.Responses {
						if rf.Status == fmt.Sprintf("%d", st.Expect) && rf.BodyDocumented {
							bodyDoc = true
							break
						}
					}
					if !bodyDoc {
						warns = append(warns, fmt.Sprintf("setup %q step %d saves %q but response body is undocumented", name, idx+1, expr))
						saveFailed = true
						break
					}
				}
			}
			if saveFailed {
				break
			}
		}
		if saveFailed {
			unresolved = append(unresolved, unresolvedItem{API: "*", Missing: "save from undocumented response", Affects: name})
			continue
		}

		stepErrs, finalScope := validateStepsDeep(
			"setup "+name, nd.raw.Steps, apis, serverSet, stepScope, bareSeedIdx, false,
		)
		if len(stepErrs) > 0 {
			warns = append(warns, fmt.Sprintf("setup %q failed validation: %v", name, stepErrs[0]))
			unresolved = append(unresolved, unresolvedItem{API: "*", Missing: stepErrs[0].Error(), Affects: name})
			continue
		}

		valid[name] = domain.Setup{
			Use:   nd.raw.Use,
			Vars:  nd.raw.Vars,
			Steps: nd.raw.Steps,
		}
		setupScopes[name] = finalScope
	}

	return valid, unresolved, warns
}

func groundScenario(
	sc rawScenario,
	fact *apiFacts,
	apis map[string]domain.API,
	serverSet map[string]struct{},
	availableSetups map[string]domain.Setup,
	seed map[string]domain.Seed,
	declaredInferences []inferredExpect,
) (*domain.Scenario, []inferredExpect, error) {
	id := slugifyID(sc.ID)
	name := strings.TrimSpace(sc.Name)
	if id == "" || name == "" || len(sc.Steps) == 0 {
		return nil, nil, errors.New("empty id, name, or steps")
	}

	// Filter setup existence: drop unknown setups so valid scenarios are not discarded prematurely
	var validSetupRefs []string
	for _, u := range sc.Setup {
		if _, ok := availableSetups[u]; ok {
			validSetupRefs = append(validSetupRefs, u)
		}
	}
	sc.Setup = validSetupRefs

	// Sanitize steps (e.g. fix single-brace {var} -> {{var}}, fix server alias)
	for idx := range sc.Steps {
		sc.Steps[idx] = sanitizeStep(sc.Steps[idx], fact.Server, serverSet)
	}

	callsTarget := false
	for _, st := range sc.Steps {
		if matched, ok, err := matchAPI(apis, st.Call); err == nil && ok && matched.Key == fact.Key {
			callsTarget = true
			break
		}
		if st.Call == fact.Key {
			callsTarget = true
			break
		}
	}
	if !callsTarget {
		return nil, nil, fmt.Errorf("scenario %q does not call target API %q", id, fact.Key)
	}

	// Verify inferred status declarations
	inferredMap := make(map[int]inferredExpect)
	for _, inf := range declaredInferences {
		if inf.ScenarioID == sc.ID || inf.ScenarioID == id {
			inferredMap[inf.Expect] = inf
		}
	}

	var confirmedInferences []inferredExpect
	for idx, st := range sc.Steps {
		if !documentedStatus(fact.DocumentedStatuses, st.Expect) {
			inf, ok := inferredMap[st.Expect]
			if !ok {
				return nil, nil, fmt.Errorf("step %d expects undocumented status %d without declared inference", idx+1, st.Expect)
			}
			inf.ScenarioID = id
			confirmedInferences = append(confirmedInferences, inf)
		}
	}

	// Validate steps with deep semantics
	seedNames, bareSeedIdx := seedScope(seed)
	parentScope := cloneScope(seedNames)
	for _, u := range sc.Setup {
		if s, ok := availableSetups[u]; ok {
			for k := range s.Vars {
				parentScope[k] = struct{}{}
			}
			for _, st := range s.Steps {
				for k := range st.Save {
					parentScope[k] = struct{}{}
				}
			}
		}
	}

	stepErrs, _ := validateStepsDeep(
		"scenario "+id, sc.Steps, apis, serverSet, parentScope, bareSeedIdx, true,
	)
	if len(stepErrs) > 0 {
		return nil, nil, stepErrs[0]
	}

	return &domain.Scenario{
		ID:    id,
		Name:  name,
		Setup: sc.Setup,
		Steps: sc.Steps,
	}, confirmedInferences, nil
}

func mergeOutcomes(outcomes []apiOutcome) (*mergedOutcomes, []string) {
	var merged mergedOutcomes
	merged.NoScenarioReport = make(map[string]string)
	var warns []string

	seenIDs := make(map[string]int)
	seenNames := make(map[string]int)

	merged.TotalAPIs = len(outcomes)

	for _, o := range outcomes {
		if o.Err != nil {
			merged.NoScenarioReport[o.APIKey] = o.Err.Error()
			warns = append(warns, fmt.Sprintf("API %s generation error: %v", o.APIKey, o.Err))
			continue
		}
		if len(o.Scenarios) == 0 {
			reason := o.NoScenarioReason
			if reason == "" {
				reason = "no scenarios generated"
			}
			merged.NoScenarioReport[o.APIKey] = reason
			continue
		}

		merged.CoveredAPIs++

		for _, sc := range o.Scenarios {
			origID := sc.ID
			seenIDs[origID]++
			if count := seenIDs[origID]; count > 1 {
				sc.ID = fmt.Sprintf("%s-%d", origID, count)
				warns = append(warns, fmt.Sprintf("scenario id collision: renamed %q to %q", origID, sc.ID))
			}

			origName := sc.Name
			seenNames[origName]++
			if count := seenNames[origName]; count > 1 {
				sc.Name = fmt.Sprintf("%s (%d)", origName, count)
				warns = append(warns, fmt.Sprintf("scenario name collision: renamed %q to %q", origName, sc.Name))
			}

			merged.Scenarios = append(merged.Scenarios, sc)
		}

		merged.InferredExpectations = append(merged.InferredExpectations, o.InferredExpectations...)
		merged.SkippedCases = append(merged.SkippedCases, o.SkippedCases...)
	}

	return &merged, warns
}

func fixSingleBraceTemplates(s string) string {
	if !strings.Contains(s, "{") {
		return s
	}
	var b strings.Builder
	runes := []rune(s)
	n := len(runes)
	for i := 0; i < n; i++ {
		if runes[i] == '{' {
			if i+1 < n && runes[i+1] == '{' {
				b.WriteString("{{")
				i++
				continue
			}
			closeIdx := -1
			for j := i + 1; j < n; j++ {
				if runes[j] == '}' {
					if j+1 < n && runes[j+1] == '}' {
						break
					}
					closeIdx = j
					break
				}
				if runes[j] == '{' || runes[j] == ' ' || runes[j] == '\n' {
					break
				}
			}
			if closeIdx != -1 {
				varName := string(runes[i+1 : closeIdx])
				if initIDRe.MatchString(varName) {
					b.WriteString("{{" + varName + "}}")
					i = closeIdx
					continue
				}
			}
		}
		b.WriteRune(runes[i])
	}
	return b.String()
}

func fixServerAlias(call string, targetServer string, serverSet map[string]struct{}) string {
	call = strings.TrimSpace(call)
	if call == "" {
		return call
	}

	parts := strings.SplitN(call, " ", 2)
	if len(parts) != 2 {
		return call
	}
	method := parts[0]
	pathPart := strings.TrimSpace(parts[1])

	effectiveServer := targetServer
	if effectiveServer == "" && len(serverSet) == 1 {
		for s := range serverSet {
			effectiveServer = s
		}
	}

	// Case 1: missing server alias: "GET /path"
	if strings.HasPrefix(pathPart, "/") {
		if effectiveServer != "" {
			return method + " {" + effectiveServer + "}" + pathPart
		}
	}

	// Case 2: placeholder "{server}": "GET {server}/path"
	if strings.HasPrefix(pathPart, "{server}/") {
		if _, inSet := serverSet["server"]; !inSet && effectiveServer != "" {
			return method + " {" + effectiveServer + "}/" + strings.TrimPrefix(pathPart, "{server}/")
		}
	}

	return call
}

func sanitizeStep(st domain.Step, targetServer string, serverSet map[string]struct{}) domain.Step {
	st.Call = fixServerAlias(st.Call, targetServer, serverSet)
	if len(st.Headers) > 0 {
		newHeaders := make(map[string]string, len(st.Headers))
		for k, v := range st.Headers {
			newHeaders[k] = fixSingleBraceTemplates(v)
		}
		st.Headers = newHeaders
	}
	if st.Body != nil {
		st.Body = sanitizeBodyVal(st.Body)
	}
	return st
}

func sanitizeBodyVal(v any) any {
	switch val := v.(type) {
	case string:
		return fixSingleBraceTemplates(val)
	case map[string]any:
		res := make(map[string]any, len(val))
		for k, inner := range val {
			res[k] = sanitizeBodyVal(inner)
		}
		return res
	case []any:
		res := make([]any, len(val))
		for i, inner := range val {
			res[i] = sanitizeBodyVal(inner)
		}
		return res
	default:
		return v
	}
}

