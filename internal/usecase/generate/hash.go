// Package generate implements the v3 generation pipeline: canonical hashing,
// OpenAPI matching, LLM calls, suite assembly, and persistence.
package generate

import (
	"maps"
	"reflect"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/nguyentin05/spectest/internal/domain"
)

// hash12 returns the first 12 lowercase hex characters of the SHA-256 digest.
// Twelve characters is part of the format version 1 contract, so changing it
// requires a migration.
func hash12(data []byte) string {
	return domain.Hash12(data)
}

// suiteHash hashes only the four mutable blocks of a suite: analysis, seed,
// setup, and scenarios. Derived fields (version, servers, apis) are excluded so
// CLI/OpenAPI churn does not look like a content edit.
func suiteHash(suite *domain.Suite) string {
	return domain.SuiteHash(suite)
}

// mutableHash hashes the canonical JSON form of the mutable blocks. It is the
// hashing primitive used both for a whole suite and for a replayed log entry.
func mutableHash(mutable domain.MutableSuite) string {
	return domain.MutableHash(mutable)
}

// normalizeMutable returns a deep copy of the mutable blocks with canonical
// ordering and empty optional fields removed. It never mutates its input.
//
// Maps (seed, setup, vars, headers, save) are left unsorted because
// encoding/json sorts map keys while marshaling. Step order and server order
// are preserved because they carry meaning.
func normalizeMutable(mutable domain.MutableSuite) domain.MutableSuite {
	normalized := domain.MutableSuite{
		Analysis:  normalizeText(mutable.Analysis),
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
		normalized.Scenarios = []domain.Scenario{}
	}
	return normalized
}

// normalizeSuite returns a canonical deep copy suitable for hashing or writing.
// Scenarios are sorted by ID, setup/scenario dependencies are sorted, and the
// apis list is sorted. Servers keep their input order (first server is the
// default) and each step list keeps its execution order.
func normalizeSuite(suite *domain.Suite) *domain.Suite {
	if suite == nil {
		return nil
	}
	normalized := cloneSuite(suite)

	mutable := normalizeMutable(normalized.Mutable())
	normalized.Analysis = mutable.Analysis
	normalized.Seed = mutable.Seed
	normalized.Setup = mutable.Setup
	normalized.Scenarios = mutable.Scenarios
	sort.Strings(normalized.APIs)

	return normalized
}

// cloneSuite returns a deep copy of suite. Maps and slices are copied at every
// level, including values stored in any-typed vars and step bodies.
func cloneSuite(suite *domain.Suite) *domain.Suite {
	if suite == nil {
		return nil
	}
	return &domain.Suite{
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

func cloneSeedMap(in map[string]domain.Seed) map[string]domain.Seed {
	if in == nil {
		return nil
	}
	out := make(map[string]domain.Seed, len(in))
	for name, seed := range in {
		out[name] = domain.Seed{Vars: cloneVars(seed.Vars)}
	}
	return out
}

func cloneSetupMap(in map[string]domain.Setup) map[string]domain.Setup {
	if in == nil {
		return nil
	}
	out := make(map[string]domain.Setup, len(in))
	for name, setup := range in {
		out[name] = domain.Setup{
			Use:   slices.Clone(setup.Use),
			Vars:  cloneVars(setup.Vars),
			Steps: cloneSteps(setup.Steps),
		}
	}
	return out
}

func cloneScenarios(in []domain.Scenario) []domain.Scenario {
	if in == nil {
		return nil
	}
	out := make([]domain.Scenario, len(in))
	for i, scenario := range in {
		out[i] = domain.Scenario{
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

func cloneSteps(in []domain.Step) []domain.Step {
	if in == nil {
		return nil
	}
	out := make([]domain.Step, len(in))
	for i, step := range in {
		out[i] = domain.Step{
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

// cloneDynamic deep-copies the container types produced by YAML/JSON decoding.
// Scalars are immutable, so they are returned as-is.
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

func normalizeSteps(in []domain.Step) []domain.Step {
	steps := cloneSteps(in)
	if steps == nil {
		steps = []domain.Step{}
	}
	for i := range steps {
		steps[i].Headers = nonEmptyStringMap(steps[i].Headers)
		steps[i].Save = nonEmptyStringMap(steps[i].Save)
		// Body is intentionally untouched: `body: {}` is content and must
		// hash differently from a missing body.
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

// normalizeRequirements canonicalizes requirements text so that line-ending
// style, trailing whitespace, and trailing blank lines do not change its hash.
// Empty and whitespace-only input normalize to "". Non-empty output ends with
// exactly one newline.
func normalizeRequirements(raw string) string {
	return normalizeText(raw)
}

// requirementsHash hashes normalized requirements. The raw text stored in the
// track file keeps its original bytes; only the hash input is normalized.
func requirementsHash(raw string) string {
	return hash12([]byte(normalizeRequirements(raw)))
}

// normalizeText applies the shared line-oriented canonicalization used by
// analysis and requirements text.
func normalizeText(raw string) string {
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
