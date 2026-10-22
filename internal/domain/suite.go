// Package domain defines the core entities, value objects, and port interfaces
// for the SpecTest engine. It contains pure Go types with zero external dependencies.
package domain

// TestSuiteFile represents the root schema of a `spectest.yaml` file on disk.
// It encapsulates suite-level metadata and the base set of executable test scenarios.
type TestSuiteFile struct {
	Version  string        `yaml:"version" json:"version"`
	Metadata SuiteMetadata `yaml:"metadata" json:"metadata"`
	Base     BaseSuite     `yaml:"base" json:"base"`
}

// SuiteMetadata holds descriptive information, the default target server URL,
// and global environment variables shared across all scenarios in a suite.
type SuiteMetadata struct {
	Name          string            `yaml:"name" json:"name"`
	Description   string            `yaml:"description,omitempty" json:"description,omitempty"`
	TargetBaseURL string            `yaml:"target_base_url" json:"target_base_url"`
	Env           map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

// BaseSuite contains the ordered list of scenarios generated from specifications.
type BaseSuite struct {
	Scenarios []Scenario `yaml:"scenarios" json:"scenarios"`
}

// Scenario models an end-to-end business workflow or user journey composed of
// sequential HTTP steps that can pass extracted state variables forward.
type Scenario struct {
	ID          string `yaml:"id" json:"id"`
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Steps       []Step `yaml:"steps" json:"steps"`
}

// Step represents a single HTTP operation within a scenario, including its request
// template, JSONPath variable extraction mappings, dataset variations, and assertions.
type Step struct {
	ID        string            `yaml:"id" json:"id"`
	Name      string            `yaml:"name" json:"name"`
	TargetRef *TargetRef        `yaml:"target_ref,omitempty" json:"target_ref,omitempty"`
	Request   RequestDef        `yaml:"request" json:"request"`
	Extract   map[string]string `yaml:"extract,omitempty" json:"extract,omitempty"`
	TestCases []TestCase        `yaml:"test_cases,omitempty" json:"test_cases,omitempty"`
	Assert    *AssertionRule    `yaml:"assert,omitempty" json:"assert,omitempty"`
}

// TargetRef links a test step back to its originating OpenAPI endpoint and operationId
// so downstream impact analysis can trace specification changes to affected steps.
type TargetRef struct {
	Endpoint    string `yaml:"endpoint" json:"endpoint"`
	Method      string `yaml:"method" json:"method"`
	OperationID string `yaml:"operation_id,omitempty" json:"operation_id,omitempty"`
}

// RequestDef defines the HTTP method, path, headers, and default payload for a step.
// Header and path strings may reference runtime state variables using `{{var}}` syntax.
type RequestDef struct {
	Method   string            `yaml:"method" json:"method"`
	Endpoint string            `yaml:"endpoint" json:"endpoint"`
	Headers  map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Body     interface{}       `yaml:"body,omitempty" json:"body,omitempty"`
}

// TestCase represents a specific input dataset variation executed against a Step's
// endpoint, such as an LLM-synthesized business case (`ai_business_valid`,
// `ai_business_boundary`) or a deterministic Go case (`go_fuzzing`, `go_security`).
type TestCase struct {
	ID      string        `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Type    string        `yaml:"type" json:"type"`
	Payload interface{}   `yaml:"payload" json:"payload"`
	Assert  AssertionRule `yaml:"assert" json:"assert"`
}

// AssertionRule defines the verification expectations evaluated against an HTTP response,
// spanning HTTP status codes, latency SLAs, JSON schema conformance, forbidden sensitive
// keys, and domain-specific JSONPath expressions.
type AssertionRule struct {
	Status       int      `yaml:"status" json:"status"`
	MaxLatencyMs int      `yaml:"max_latency_ms,omitempty" json:"max_latency_ms,omitempty"`
	SchemaMatch  bool     `yaml:"schema_match,omitempty" json:"schema_match,omitempty"`
	ForbidKeys   []string `yaml:"forbid_keys,omitempty" json:"forbid_keys,omitempty"`
	JSONPath     []string `yaml:"jsonpath,omitempty" json:"jsonpath,omitempty"`
}
