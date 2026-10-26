package generate

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/nguyentin05/spectest/internal/domain"
)

type validationOptions struct {
	RequireAnalysis  bool
	RequiredCoverage []string
	AllowStaleAPIs   bool
	WarnUncovered    bool
}

type validationResult struct {
	Errors   []error
	Warnings []string
}

func (r validationResult) Err() error {
	if len(r.Errors) == 0 {
		return nil
	}
	return errors.Join(r.Errors...)
}

var (
	validateServerRe   = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	validateIDRe       = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	validateNameRe     = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	validateAPIKeyRe   = regexp.MustCompile(`^[A-Z]+ \{[a-z][a-z0-9_-]*\}/.*$`)
	validateSaveExprRe = regexp.MustCompile(`^(body|header)\..+$`)
	templateRe         = regexp.MustCompile(`\{\{([a-zA-Z_][a-zA-Z0-9_.]*)\}\}`)
)

// validateSuite runs the full v1 rule set.
func validateSuite(s *domain.Suite, src *domain.SourceSet, opts validationOptions) validationResult {
	var res validationResult
	if s == nil {
		res.Errors = append(res.Errors, fmt.Errorf("suite is nil"))
		return res
	}

	res.Errors = append(res.Errors, validateStructure(s, opts)...)
	res.Errors = append(res.Errors, validateIDsAndNames(s)...)
	res.Errors = append(res.Errors, validateSetupGraph(s)...)

	apiLookup := buildAPILookup(s, src, opts.AllowStaleAPIs)
	res.Errors = append(res.Errors, validateSemantics(s, apiLookup, opts)...)

	covErrs, covWarns := validateCoverage(s, src, opts, apiLookup)
	res.Errors = append(res.Errors, covErrs...)
	res.Warnings = append(res.Warnings, covWarns...)

	sort.Strings(res.Warnings)
	return res
}

// ============================================================================
// Structure / IDs / Graph
// ============================================================================

func validateStructure(s *domain.Suite, opts validationOptions) []error {
	var errs []error

	if s.Version != 1 {
		errs = append(errs, fmt.Errorf("version must be 1, got %d", s.Version))
	}
	if !s.Input.Valid() {
		errs = append(errs, fmt.Errorf("input must be openapi or both, got %q", s.Input))
	}
	if opts.RequireAnalysis && strings.TrimSpace(s.Analysis) == "" {
		errs = append(errs, fmt.Errorf("analysis must not be empty"))
	}
	errs = append(errs, checkManagedBlocks(s.Analysis)...)

	if len(s.Servers) == 0 {
		errs = append(errs, fmt.Errorf("servers must not be empty"))
	}
	seenServers := map[string]struct{}{}
	for _, srv := range s.Servers {
		if !validateServerRe.MatchString(srv) {
			errs = append(errs, fmt.Errorf("server %q does not match %s", srv, validateServerRe))
		}
		if _, dup := seenServers[srv]; dup {
			errs = append(errs, fmt.Errorf("duplicate server %q", srv))
		}
		seenServers[srv] = struct{}{}
	}

	if len(s.APIs) == 0 {
		errs = append(errs, fmt.Errorf("apis must not be empty"))
	}
	seenAPIs := map[string]struct{}{}
	for _, key := range s.APIs {
		if !validateAPIKeyRe.MatchString(key) {
			errs = append(errs, fmt.Errorf("api key %q has invalid format", key))
		}
		if strings.Contains(key, "?") {
			errs = append(errs, fmt.Errorf("api key %q must not contain query", key))
		}
		if _, dup := seenAPIs[key]; dup {
			errs = append(errs, fmt.Errorf("duplicate api key %q", key))
		}
		seenAPIs[key] = struct{}{}
	}

	if !opts.AllowStaleAPIs && !hasActiveScenario(s) {
		errs = append(errs, fmt.Errorf("at least one scenario must be active (not locked or skipped)"))
	}

	return errs
}

func hasActiveScenario(s *domain.Suite) bool {
	for _, sc := range s.Scenarios {
		if !sc.Locked && sc.Skip == "" {
			return true
		}
	}
	return false
}

func validateIDsAndNames(s *domain.Suite) []error {
	var errs []error

	for name := range s.Seed {
		if !validateNameRe.MatchString(name) {
			errs = append(errs, fmt.Errorf("seed name %q does not match %s", name, validateNameRe))
		}
	}
	for name := range s.Setup {
		if !validateNameRe.MatchString(name) {
			errs = append(errs, fmt.Errorf("setup name %q does not match %s", name, validateNameRe))
		}
	}

	seenIDs := map[string]struct{}{}
	seenNames := map[string]struct{}{}
	for _, sc := range s.Scenarios {
		if !validateIDRe.MatchString(sc.ID) {
			errs = append(errs, fmt.Errorf("scenario id %q does not match %s", sc.ID, validateIDRe))
		}
		if _, dup := seenIDs[sc.ID]; dup {
			errs = append(errs, fmt.Errorf("duplicate scenario id %q", sc.ID))
		}
		seenIDs[sc.ID] = struct{}{}

		if strings.TrimSpace(sc.Name) == "" {
			errs = append(errs, fmt.Errorf("scenario %q has empty name", sc.ID))
		} else if _, dup := seenNames[sc.Name]; dup {
			errs = append(errs, fmt.Errorf("duplicate scenario name %q", sc.Name))
		}
		seenNames[sc.Name] = struct{}{}

		if sc.Skip != "" && strings.TrimSpace(sc.Skip) == "" {
			errs = append(errs, fmt.Errorf("scenario %q has whitespace-only skip reason", sc.ID))
		}
	}

	return errs
}

func validateSetupGraph(s *domain.Suite) []error {
	if _, err := topoSetupOrder(s.Setup); err != nil {
		return []error{err}
	}
	return nil
}

// ============================================================================
// API lookup
// ============================================================================

func buildAPILookup(s *domain.Suite, src *domain.SourceSet, allowStale bool) map[string]domain.API {
	out := make(map[string]domain.API, len(src.APIs))
	for _, a := range src.APIs {
		out[a.Key] = a
	}
	if allowStale {
		for _, key := range s.APIs {
			if _, ok := out[key]; ok {
				continue
			}
			if api, ok := apiFromKey(key); ok {
				out[key] = api
			}
		}
		for _, setup := range s.Setup {
			for _, step := range setup.Steps {
				if _, ok, _ := matchAPI(out, step.Call); !ok {
					if api, ok := apiFromKey(step.Call); ok {
						out[api.Key] = api
					}
				}
			}
		}
		for _, sc := range s.Scenarios {
			for _, step := range sc.Steps {
				if _, ok, _ := matchAPI(out, step.Call); !ok {
					if api, ok := apiFromKey(step.Call); ok {
						out[api.Key] = api
					}
				}
			}
		}
	}
	return out
}

func apiFromKey(key string) (domain.API, bool) {
	pc, err := parseCall(key)
	if err != nil {
		return domain.API{}, false
	}
	return domain.API{
		Key:    key,
		Method: pc.Method,
		Server: pc.Server,
		Path:   pc.Path,
	}, true
}

// ============================================================================
// Deep semantics (variables, body, status, saves)
// ============================================================================

// validateSemantics validates variables, bodies, statuses and saves. Setups are
// walked in topo order so each setup's scope contains its dependencies' vars
// and saves.
func validateSemantics(s *domain.Suite, apiLookup map[string]domain.API, opts ...validationOptions) []error {
	allowStale := false
	if len(opts) > 0 {
		allowStale = opts[0].AllowStaleAPIs
	}
	var errs []error

	serverSet := make(map[string]struct{}, len(s.Servers))
	for _, srv := range s.Servers {
		serverSet[srv] = struct{}{}
	}

	setupOrder, err := topoSetupOrder(s.Setup)
	if err != nil {
		return []error{err}
	}

	seedNames, bareSeedIndex := seedScope(s.Seed)

	setupScopes := make(map[string]map[string]struct{}, len(s.Setup))
	for _, name := range setupOrder {
		setup := s.Setup[name]

		// use refs must exist (topo already checked cycle, not existence)
		for _, u := range setup.Use {
			if _, ok := s.Setup[u]; !ok {
				errs = append(errs, fmt.Errorf("setup %q uses unknown setup %q", name, u))
			}
		}

		parentScope := cloneScope(seedNames)
		for _, u := range setup.Use {
			if depScope, ok := setupScopes[u]; ok {
				mergeScope(parentScope, depScope)
			}
		}

		// Setup vars can only reference seed + dep closure.
		for varKey, varVal := range setup.Vars {
			errs = append(errs, checkTemplates(
				parentScope, bareSeedIndex,
				fmt.Sprintf("setup %q var %q", name, varKey),
				varVal,
			)...)
		}

		// Step scope: parent + own vars (steps' saves accumulate).
		stepScope := cloneScope(parentScope)
		for varKey := range setup.Vars {
			stepScope[varKey] = struct{}{}
		}

		stepErrs, finalScope := validateStepsDeep(
			"setup "+name, setup.Steps, apiLookup, serverSet, stepScope, bareSeedIndex,
		)
		errs = append(errs, stepErrs...)
		setupScopes[name] = finalScope
	}

	for _, sc := range s.Scenarios {
		if allowStale && sc.Skip != "" {
			continue
		}
		for _, u := range sc.Setup {
			if _, ok := s.Setup[u]; !ok {
				errs = append(errs, fmt.Errorf("scenario %q uses unknown setup %q", sc.ID, u))
			}
		}

		parentScope := cloneScope(seedNames)
		for _, u := range sc.Setup {
			if depScope, ok := setupScopes[u]; ok {
				mergeScope(parentScope, depScope)
			}
		}

		stepErrs, _ := validateStepsDeep(
			"scenario "+sc.ID, sc.Steps, apiLookup, serverSet, parentScope, bareSeedIndex,
		)
		errs = append(errs, stepErrs...)
	}

	return errs
}

func validateStepsDeep(
	owner string,
	steps []domain.Step,
	apiLookup map[string]domain.API,
	serverSet map[string]struct{},
	scope map[string]struct{},
	bareIndex map[string][]string,
) ([]error, map[string]struct{}) {
	var errs []error

	if len(steps) == 0 {
		return []error{fmt.Errorf("%s: steps must not be empty", owner)}, scope
	}

	seenSaves := map[string]struct{}{}
	currentScope := cloneScope(scope)

	for i, st := range steps {
		idx := i + 1

		if strings.TrimSpace(st.Call) == "" {
			errs = append(errs, fmt.Errorf("%s: step %d: call is empty", owner, idx))
			continue
		}
		if st.Expect <= 0 {
			errs = append(errs, fmt.Errorf("%s: step %d: expect must be > 0", owner, idx))
		}

		// save maps a variable name to a body.x / header.x extraction expression.
		for saveName, expression := range st.Save {
			if !validateSaveExprRe.MatchString(expression) {
				errs = append(errs, fmt.Errorf("%s: step %d: save expression %q for %q must be body.x or header.x", owner, idx, expression, saveName))
				continue
			}
			if strings.TrimSpace(saveName) == "" {
				errs = append(errs, fmt.Errorf("%s: step %d: save name for %q is empty", owner, idx, saveName))
				continue
			}
			if _, dup := seenSaves[saveName]; dup {
				errs = append(errs, fmt.Errorf("%s: step %d: save name %q duplicated", owner, idx, saveName))
				continue
			}
			seenSaves[saveName] = struct{}{}
		}

		// Template resolution: call, headers, body
		errs = append(errs, checkTemplates(currentScope, bareIndex,
			fmt.Sprintf("%s: step %d call", owner, idx), st.Call)...)
		for k, v := range st.Headers {
			errs = append(errs, checkTemplates(currentScope, bareIndex,
				fmt.Sprintf("%s: step %d header %q", owner, idx, k), v)...)
		}
		errs = append(errs, checkTemplates(currentScope, bareIndex,
			fmt.Sprintf("%s: step %d body", owner, idx), st.Body)...)

		// API match + server + required query
		api, found, err := matchAPI(apiLookup, st.Call)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: step %d: %w", owner, idx, err))
			continue
		}
		if !found {
			errs = append(errs, fmt.Errorf("%s: step %d: call %q does not match any api", owner, idx, st.Call))
			continue
		}
		if _, ok := serverSet[api.Server]; !ok {
			errs = append(errs, fmt.Errorf("%s: step %d: server %q not in suite.servers", owner, idx, api.Server))
		}

		pc, _ := parseCall(st.Call)
		for _, p := range api.Parameters {
			if p.In != "query" || !p.Required {
				continue
			}
			if !queryHasKey(pc.RawQuery, p.Name) {
				errs = append(errs, fmt.Errorf("%s: step %d: missing required query param %q", owner, idx, p.Name))
			}
		}

		// Body schema
		if api.Request != nil {
			errs = append(errs, validateBodyDeep(owner, idx, st.Body, api.Request)...)
		}

		// Status
		errs = append(errs, validateStatus(owner, idx, st.Expect, api.Responses)...)

		// Add saves to scope for subsequent steps
		for saveName := range st.Save {
			if strings.TrimSpace(saveName) != "" {
				currentScope[saveName] = struct{}{}
			}
		}
	}

	return errs, currentScope
}

// ============================================================================
// Body / status / template helpers
// ============================================================================

func validateBodyDeep(owner string, idx int, body any, req *domain.RequestSpec) []error {
	if body == nil {
		if req.Required {
			return []error{fmt.Errorf("%s: step %d: request body is required", owner, idx)}
		}
		return nil
	}
	if !req.JSON || req.Schema.Unsupported {
		return nil
	}
	return checkBodyValue(owner, idx, body, req.Schema, true)
}

func checkBodyValue(owner string, idx int, val any, schema domain.Schema, top bool) []error {
	if schema.Unsupported {
		return nil
	}
	// A string carrying {{...}} may resolve to anything → skip type check.
	if s, ok := val.(string); ok && templateRe.MatchString(s) {
		return nil
	}

	var errs []error

	switch schema.Type {
	case "", "object":
		m, ok := val.(map[string]any)
		if !ok {
			if top && schema.Type == "object" && val != nil {
				return []error{fmt.Errorf("%s: step %d: body must be object, got %T", owner, idx, val)}
			}
			return nil
		}
		for _, req := range schema.Required {
			if _, ok := m[req]; !ok {
				errs = append(errs, fmt.Errorf("%s: step %d: body missing required field %q", owner, idx, req))
			}
		}
		for key, v := range m {
			prop, known := schema.Properties[key]
			if !known {
				if schema.AdditionalProperties != nil && !*schema.AdditionalProperties {
					errs = append(errs, fmt.Errorf("%s: step %d: body has unknown field %q", owner, idx, key))
				}
				continue
			}
			errs = append(errs, checkBodyValue(owner, idx, v, prop, false)...)
		}

	case "string":
		if _, ok := val.(string); !ok {
			errs = append(errs, fmt.Errorf("%s: step %d: body type mismatch: expected string, got %T", owner, idx, val))
		}

	case "integer":
		switch v := val.(type) {
		case int, int32, int64:
			// ok
		case float64:
			if v != float64(int64(v)) {
				errs = append(errs, fmt.Errorf("%s: step %d: body type mismatch: expected integer, got non-integer %v", owner, idx, v))
			}
		default:
			errs = append(errs, fmt.Errorf("%s: step %d: body type mismatch: expected integer, got %T", owner, idx, val))
		}

	case "number":
		switch val.(type) {
		case int, int32, int64, float32, float64:
			// ok
		default:
			errs = append(errs, fmt.Errorf("%s: step %d: body type mismatch: expected number, got %T", owner, idx, val))
		}

	case "boolean":
		if _, ok := val.(bool); !ok {
			errs = append(errs, fmt.Errorf("%s: step %d: body type mismatch: expected boolean, got %T", owner, idx, val))
		}

	case "array":
		arr, ok := val.([]any)
		if !ok {
			errs = append(errs, fmt.Errorf("%s: step %d: body type mismatch: expected array, got %T", owner, idx, val))
			break
		}
		if schema.Items != nil {
			for _, item := range arr {
				errs = append(errs, checkBodyValue(owner, idx, item, *schema.Items, false)...)
			}
		}
	}

	return errs
}

func validateStatus(owner string, idx, expect int, responses map[string]domain.Schema) []error {
	if len(responses) == 0 {
		if expect < 100 || expect > 599 {
			return []error{fmt.Errorf("%s: step %d: expect %d out of range 100-599", owner, idx, expect)}
		}
		return nil
	}
	if _, ok := responses[fmt.Sprintf("%d", expect)]; ok {
		return nil
	}
	if expect >= 100 && expect <= 599 {
		prefix := fmt.Sprintf("%dXX", expect/100)
		if _, ok := responses[prefix]; ok {
			return nil
		}
		if _, ok := responses["default"]; ok {
			return nil
		}
	}
	keys := make([]string, 0, len(responses))
	for k := range responses {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return []error{fmt.Errorf("%s: step %d: expect %d not in api responses %v", owner, idx, expect, keys)}
}

// ============================================================================
// Template traversal
// ============================================================================

func checkTemplates(scope map[string]struct{}, bareIndex map[string][]string, owner string, val any) []error {
	var errs []error
	walkStrings(val, func(s string) {
		for _, m := range templateRe.FindAllStringSubmatch(s, -1) {
			name := m[1]
			if _, ok := scope[name]; ok {
				continue
			}
			// Ambiguous bare name: exists in multiple seeds.
			if seeds, ok := bareIndex[name]; ok && len(seeds) > 1 {
				sorted := append([]string(nil), seeds...)
				sort.Strings(sorted)
				refs := make([]string, len(sorted))
				for i, sd := range sorted {
					refs[i] = "{{" + sd + "." + name + "}}"
				}
				errs = append(errs, fmt.Errorf(
					"%s: ambiguous variable {{%s}}; use one of %s",
					owner, name, strings.Join(refs, ", ")))
				continue
			}
			errs = append(errs, fmt.Errorf("%s: unresolved variable {{%s}}", owner, name))
		}
	})
	return errs
}

func walkStrings(v any, fn func(string)) {
	switch t := v.(type) {
	case string:
		fn(t)
	case map[string]any:
		for _, x := range t {
			walkStrings(x, fn)
		}
	case []any:
		for _, x := range t {
			walkStrings(x, fn)
		}
	case map[string]string:
		for _, x := range t {
			fn(x)
		}
	}
}

// ============================================================================
// Scope helpers
// ============================================================================

func seedScope(seeds map[string]domain.Seed) (scope map[string]struct{}, bareIndex map[string][]string) {
	scope = map[string]struct{}{}
	bareIndex = map[string][]string{}
	for name, seed := range seeds {
		for k := range seed.Vars {
			scope[name+"."+k] = struct{}{}
			bareIndex[k] = append(bareIndex[k], name)
		}
	}
	for k, seeds := range bareIndex {
		if len(seeds) == 1 {
			scope[k] = struct{}{}
		}
	}
	return scope, bareIndex
}

func mergeScope(dst, src map[string]struct{}) {
	for k := range src {
		dst[k] = struct{}{}
	}
}

func cloneScope(s map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(s))
	for k := range s {
		out[k] = struct{}{}
	}
	return out
}

// ============================================================================
// Setup topo order
// ============================================================================

func topoSetupOrder(setups map[string]domain.Setup) ([]string, error) {
	names := make([]string, 0, len(setups))
	for n := range setups {
		names = append(names, n)
	}
	return topoOrder(names, func(n string) ([]string, bool) {
		s, ok := setups[n]
		return s.Use, ok
	})
}

// ============================================================================
// Coverage
// ============================================================================

func validateCoverage(
	s *domain.Suite, src *domain.SourceSet, opts validationOptions, apiLookup map[string]domain.API,
) ([]error, []string) {
	var errs []error
	var warns []string

	activeSetups := map[string]struct{}{}
	var markSetup func(name string)
	markSetup = func(name string) {
		if _, done := activeSetups[name]; done {
			return
		}
		activeSetups[name] = struct{}{}
		if setup, ok := s.Setup[name]; ok {
			for _, u := range setup.Use {
				markSetup(u)
			}
		}
	}
	for _, sc := range s.Scenarios {
		if sc.Locked || sc.Skip != "" {
			continue
		}
		for _, u := range sc.Setup {
			markSetup(u)
		}
	}

	for name := range s.Setup {
		if _, ok := activeSetups[name]; !ok {
			warns = append(warns, fmt.Sprintf("setup %q is not used by any active scenario", name))
		}
	}

	covered := map[string]struct{}{}
	collect := func(steps []domain.Step) {
		for _, st := range steps {
			api, found, err := matchAPI(apiLookup, st.Call)
			if err == nil && found && api.Key != "" {
				covered[api.Key] = struct{}{}
			}
		}
	}
	for _, sc := range s.Scenarios {
		if sc.Locked || sc.Skip != "" {
			continue
		}
		collect(sc.Steps)
	}
	for name := range activeSetups {
		if setup, ok := s.Setup[name]; ok {
			collect(setup.Steps)
		}
	}

	for _, key := range opts.RequiredCoverage {
		if _, ok := covered[key]; !ok {
			errs = append(errs, fmt.Errorf("required api key %q is not covered", key))
		}
	}

	if opts.WarnUncovered {
		for _, a := range src.APIs {
			if _, ok := covered[a.Key]; !ok {
				warns = append(warns, fmt.Sprintf("api %q is not covered by any active scenario", a.Key))
			}
		}
	}

	return errs, warns
}

func queryHasKey(rawQuery, key string) bool {
	if rawQuery == "" {
		return false
	}
	for kv := range strings.SplitSeq(rawQuery, "&") {
		name, _, _ := strings.Cut(kv, "=")
		if name == key {
			return true
		}
	}
	return false
}

// allAPIKeys returns the sorted list of API keys from src.
func allAPIKeys(src *domain.SourceSet) []string {
	out := make([]string, 0, len(src.APIs))
	for _, a := range src.APIs {
		out = append(out, a.Key)
	}
	sort.Strings(out)
	return out
}
