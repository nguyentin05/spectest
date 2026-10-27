package generate

import (
	"github.com/nguyentin05/spectest/internal/domain"
)

// apiFacts contains facts extracted directly by code from OpenAPI.
type apiFacts struct {
	Key                string              `json:"key"`
	Method             string              `json:"method"`
	Server             string              `json:"server"`
	Path               string              `json:"path"`
	Description        string              `json:"description,omitempty"`
	Auth               []string            `json:"auth"`
	Parameters         []paramFacts        `json:"parameters,omitempty"`
	HasRequestBody     bool                `json:"has_request_body"`
	RequestBodyFields  []string            `json:"request_body_fields,omitempty"`
	ResponseFields     []string            `json:"response_fields,omitempty"`
	DocumentedStatuses []string            `json:"documented_statuses"`
	Responses          []responseFacts     `json:"responses,omitempty"`
	Sig                domain.APISignature `json:"sig"`
	RawSnippet         string              `json:"raw_snippet,omitempty"`
}

type paramFacts struct {
	Name     string `json:"name"`
	In       string `json:"in"`
	Required bool   `json:"required"`
}

type responseFacts struct {
	Status         string `json:"status"`
	BodyDocumented bool   `json:"body_documented"`
}

// Stage 1 DTOs
type contractResult struct {
	GlobalNotes          []string       `json:"global_notes"`
	Overrides            []overrideItem `json:"overrides"`
	Ambiguities          []string       `json:"ambiguities"`
	MissingInfo          []string       `json:"missing_info"`
	RequirementsLanguage string         `json:"requirements_language"`
}

type overrideItem struct {
	API     string `json:"api"`
	Item    string `json:"item"`
	OpenAPI string `json:"openapi"`
	Spec    string `json:"spec"`
}

// Stage 2 DTOs
type setupPlanResult struct {
	Seed        map[string]domain.Seed  `json:"seed"`
	Setup       map[string]rawSetupPlan `json:"setup"`
	Unresolved  []unresolvedItem        `json:"unresolved"`
	Assumptions []assumptionItem        `json:"assumptions"`
}

type rawSetupPlan struct {
	Use   []string          `json:"use,omitempty"`
	Vars  map[string]any    `json:"vars,omitempty"`
	Steps []domain.Step     `json:"steps"`
}

type unresolvedItem struct {
	API     string `json:"api"`
	Missing string `json:"missing"`
	Affects string `json:"affects"`
}

type assumptionItem struct {
	Area      string `json:"area"`
	Statement string `json:"statement"`
	Reason    string `json:"reason"`
}

// Stage 3 DTOs
type scenarioResult struct {
	API                  string           `json:"api"`
	Scenarios            []rawScenario    `json:"scenarios"`
	InferredExpectations []inferredExpect `json:"inferred_expectations"`
	SkippedCases         []skippedCase    `json:"skipped_cases"`
	NoScenarioReason     *string          `json:"no_scenario_reason"`
}

type rawScenario struct {
	ID    string        `json:"id"`
	Name  string        `json:"name"`
	Setup []string      `json:"setup,omitempty"`
	Steps []domain.Step `json:"steps"`
}

type inferredExpect struct {
	ScenarioID string `json:"scenario_id"`
	Expect     int    `json:"expect"`
	Basis      string `json:"basis"`
	Reason     string `json:"reason"`
}

type skippedCase struct {
	Case   string `json:"case"`
	Reason string `json:"reason"`
}

// Stage 4 DTOs
type analysisResult struct {
	Analysis    string           `json:"analysis"`
	Assumptions []assumptionItem `json:"assumptions"`
}

// Pipeline outcomes
type apiOutcome struct {
	APIKey               string
	Scenarios            []domain.Scenario
	InferredExpectations []inferredExpect
	SkippedCases         []skippedCase
	NoScenarioReason     string
	Err                  error
}

type mergedOutcomes struct {
	Scenarios            []domain.Scenario
	InferredExpectations []inferredExpect
	SkippedCases         []skippedCase
	NoScenarioReport     map[string]string
	TotalAPIs            int
	CoveredAPIs          int
}
