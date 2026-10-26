package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"reflect"
	"slices"
	"sort"
	"strings"
	"unicode"
)

// Hash12 returns the first 12 lowercase hex characters of the SHA-256 digest.
// Twelve characters is part of the format version 1 contract.
func Hash12(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:12]
}

// SuiteHash hashes only the four mutable blocks of a suite: analysis, seed,
// setup, and scenarios. Derived fields (version, servers, apis) are excluded so
// CLI/OpenAPI churn does not look like a content edit.
func SuiteHash(suite *Suite) string {
	if suite == nil {
		return MutableHash(MutableSuite{})
	}
	return MutableHash(suite.Mutable())
}

// Hash returns the 12-character canonical hash of the mutable blocks of the suite.
func (s *Suite) Hash() string {
	return SuiteHash(s)
}

// Clone returns a deep copy of the suite.
func (s *Suite) Clone() *Suite {
	return CloneSuite(s)
}

// CloneSuite returns a canonical deep copy of the suite.
func CloneSuite(suite *Suite) *Suite {
	if suite == nil {
		return nil
	}
	return &Suite{
		Version:   suite.Version,
		Input:     suite.Input,
		Servers:   slices.Clone(suite.Servers),
		APIs:      slices.Clone(suite.APIs),
		Analysis:  suite.Analysis,
		Seed:      cloneSeedMap(suite.Seed),
		Setup:     cloneSetupMap(suite.Setup),
		Scenarios: cloneScenarios(suite.Scenarios),
	}
}

// MutableHash hashes the canonical JSON form of the mutable blocks.
func MutableHash(mutable MutableSuite) string {
	data, err := json.Marshal(NormalizeMutable(mutable))
	if err != nil {
		panic("domain: marshal canonical suite: " + err.Error())
	}
	return Hash12(data)
}

// NormalizeMutable returns a deep copy of the mutable blocks with canonical
// ordering and empty optional fields removed. It never mutates its input.
//
// Maps (seed, setup, vars, headers, save) are left unsorted because
// encoding/json sorts map keys while marshaling. Step order and server order
// are preserved because they carry meaning.
func NormalizeMutable(mutable MutableSuite) MutableSuite {
	normalized := MutableSuite{
		Analysis:  NormalizeText(mutable.Analysis),
		Seed:      cloneSeedMap(mutable.Seed),
		Setup:     cloneSetupMap(mutable.Setup),
		Scenarios: cloneScenarios(mutable.Scenarios),
	}

	for name, seed := range normalized.Seed {
		seed.Vars = nonEmptyVars(seed.Vars)
		normalized.Seed[name] = seed
	}
	for name, setup := range normalized.Setup {
		setup.Use = sortedNonEmpty(setup.Use)
		setup.Vars = nonEmptyVars(setup.Vars)
		setup.Steps = normalizeSteps(setup.Steps)
		normalized.Setup[name] = setup
	}
	for i := range normalized.Scenarios {
		scenario := &normalized.Scenarios[i]
		scenario.Setup = sortedNonEmpty(scenario.Setup)
		scenario.Steps = normalizeSteps(scenario.Steps)
	}

	// Stable sort keeps duplicate IDs deterministic; duplicate IDs are
	// rejected by the suite validator, so ties should not occur.
	sort.SliceStable(normalized.Scenarios, func(i, j int) bool {
		return normalized.Scenarios[i].ID < normalized.Scenarios[j].ID
	})

	if len(normalized.Seed) == 0 {
		normalized.Seed = nil
	}
	if len(normalized.Setup) == 0 {
		normalized.Setup = nil
	}
	// scenarios is a required block: encode nil and empty alike.
	if normalized.Scenarios == nil {
		normalized.Scenarios = []Scenario{}
	}
	return normalized
}

// NormalizeText applies the shared line-oriented canonicalization used by
// analysis and requirements text.
func NormalizeText(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")

	lines := strings.Split(raw, "\n")
	for i := range lines {
		lines[i] = strings.TrimRightFunc(lines[i], unicode.IsSpace)
	}

	end := len(lines)
	for end > 0 && lines[end-1] == "" {
		end--
	}
	if end == 0 {
		return ""
	}
	return strings.Join(lines[:end], "\n") + "\n"
}

func cloneSeedMap(in map[string]Seed) map[string]Seed {
	if in == nil {
		return nil
	}
	out := make(map[string]Seed, len(in))
	for name, seed := range in {
		out[name] = Seed{Vars: cloneVars(seed.Vars)}
	}
	return out
}

func cloneSetupMap(in map[string]Setup) map[string]Setup {
	if in == nil {
		return nil
	}
	out := make(map[string]Setup, len(in))
	for name, setup := range in {
		out[name] = Setup{
			Use:   slices.Clone(setup.Use),
			Vars:  cloneVars(setup.Vars),
			Steps: cloneSteps(setup.Steps),
		}
	}
	return out
}

func cloneScenarios(in []Scenario) []Scenario {
	if in == nil {
		return nil
	}
	out := make([]Scenario, len(in))
	for i, scenario := range in {
		out[i] = Scenario{
			ID:     scenario.ID,
			Name:   scenario.Name,
			Setup:  slices.Clone(scenario.Setup),
			Steps:  cloneSteps(scenario.Steps),
			Locked: scenario.Locked,
			Skip:   scenario.Skip,
		}
	}
	return out
}

func cloneSteps(in []Step) []Step {
	if in == nil {
		return nil
	}
	out := make([]Step, len(in))
	for i, step := range in {
		out[i] = Step{
			Call:    step.Call,
			Headers: cloneStringMap(step.Headers),
			Body:    cloneDynamic(step.Body),
			Expect:  step.Expect,
			Save:    cloneStringMap(step.Save),
		}
	}
	return out
}

func cloneVars(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneDynamic(value)
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	maps.Copy(out, in)
	return out
}

func cloneDynamic(value any) any {
	if value == nil {
		return nil
	}
	return cloneReflect(reflect.ValueOf(value)).Interface()
}

func cloneReflect(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return value
		}
		out := reflect.New(value.Type()).Elem()
		out.Set(cloneReflect(value.Elem()))
		return out
	case reflect.Map:
		if value.IsNil() {
			return value
		}
		out := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			out.SetMapIndex(iter.Key(), cloneReflect(iter.Value()))
		}
		return out
	case reflect.Slice:
		if value.IsNil() {
			return value
		}
		out := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := range value.Len() {
			out.Index(i).Set(cloneReflect(value.Index(i)))
		}
		return out
	case reflect.Pointer:
		if value.IsNil() {
			return value
		}
		out := reflect.New(value.Type().Elem())
		out.Elem().Set(cloneReflect(value.Elem()))
		return out
	default:
		return value
	}
}

func normalizeSteps(in []Step) []Step {
	steps := cloneSteps(in)
	if steps == nil {
		steps = []Step{}
	}
	for i := range steps {
		steps[i].Headers = nonEmptyStringMap(steps[i].Headers)
		steps[i].Save = nonEmptyStringMap(steps[i].Save)
	}
	return steps
}

func sortedNonEmpty(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := slices.Clone(in)
	sort.Strings(out)
	return out
}

func nonEmptyVars(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	return in
}

func nonEmptyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	return in
}
