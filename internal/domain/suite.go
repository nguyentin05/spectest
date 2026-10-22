package domain

// TestSuiteFile represents the spectest.yaml file schema.
type TestSuiteFile struct {
	Version  string        `yaml:"version" json:"version"`
	Metadata SuiteMetadata `yaml:"metadata" json:"metadata"`
	Base     BaseSuite     `yaml:"base" json:"base"`
}

// SuiteMetadata contains metadata for the test suite.
type SuiteMetadata struct {
	Name          string            `yaml:"name" json:"name"`
	Description   string            `yaml:"description,omitempty" json:"description,omitempty"`
	TargetBaseURL string            `yaml:"target_base_url" json:"target_base_url"`
	Env           map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

// BaseSuite represents the base test suite.
type BaseSuite struct {
	Scenarios []Scenario `yaml:"scenarios" json:"scenarios"`
}

// Scenario represents an end-to-end user journey or domain flow.
type Scenario struct {
	ID          string `yaml:"id" json:"id"`
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Steps       []Step `yaml:"steps" json:"steps"`
}

// Step represents a single HTTP request step in a scenario.
type Step struct {
	ID        string            `yaml:"id" json:"id"`
	Name      string            `yaml:"name" json:"name"`
	TargetRef *TargetRef        `yaml:"target_ref,omitempty" json:"target_ref,omitempty"`
	Request   RequestDef        `yaml:"request" json:"request"`
	Extract   map[string]string `yaml:"extract,omitempty" json:"extract,omitempty"`
	TestCases []TestCase        `yaml:"test_cases,omitempty" json:"test_cases,omitempty"`
	Assert    *AssertionRule    `yaml:"assert,omitempty" json:"assert,omitempty"`
}

// TargetRef links a step to an operation in the OpenAPI specification.
type TargetRef struct {
	Endpoint    string `yaml:"endpoint" json:"endpoint"`
	Method      string `yaml:"method" json:"method"`
	OperationID string `yaml:"operation_id,omitempty" json:"operation_id,omitempty"`
}

// RequestDef defines the HTTP request template.
type RequestDef struct {
	Method   string            `yaml:"method" json:"method"`
	Endpoint string            `yaml:"endpoint" json:"endpoint"`
	Headers  map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Body     interface{}       `yaml:"body,omitempty" json:"body,omitempty"`
}

// TestCase represents an individual dataset variation (valid, boundary, fuzzing, security).
type TestCase struct {
	ID      string        `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Type    string        `yaml:"type" json:"type"`
	Payload interface{}   `yaml:"payload" json:"payload"`
	Assert  AssertionRule `yaml:"assert" json:"assert"`
}

// AssertionRule represents the multi-tier verification rules for a step or test case.
type AssertionRule struct {
	Status       int      `yaml:"status" json:"status"`
	MaxLatencyMs int      `yaml:"max_latency_ms,omitempty" json:"max_latency_ms,omitempty"`
	SchemaMatch  bool     `yaml:"schema_match,omitempty" json:"schema_match,omitempty"`
	ForbidKeys   []string `yaml:"forbid_keys,omitempty" json:"forbid_keys,omitempty"`
	JSONPath     []string `yaml:"jsonpath,omitempty" json:"jsonpath,omitempty"`
}
