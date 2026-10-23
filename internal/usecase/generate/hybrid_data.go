package generate

import (
	"fmt"
	"sort"

	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/openapi"
)

// EnrichStepWithHybridData augments an LLM-generated mutating Step (`POST`, `PUT`, `PATCH`)
// with deterministic Go boundary fuzzing (`go_fuzzing`) and security injection (`go_security`)
// test cases at zero token cost, mutating step.TestCases in place.
func EnrichStepWithHybridData(step *domain.Step, op openapi.NormalizedOperation) {
	if op.IsTokenProducer || op.RequestSchema == nil || len(op.RequestSchema.Properties) == 0 {
		return
	}
	if op.Method != "POST" && op.Method != "PUT" && op.Method != "PATCH" {
		return
	}

	basePayload := extractBasePayload(step, *op.RequestSchema)

	var propNames []string
	for k := range op.RequestSchema.Properties {
		propNames = append(propNames, k)
	}
	sort.Strings(propNames)

	var numericField string
	var stringField string
	for _, k := range propNames {
		prop := op.RequestSchema.Properties[k]
		if (prop.Type == "integer" || prop.Type == "number") && numericField == "" {
			numericField = k
		}
		if prop.Type == "string" && stringField == "" {
			stringField = k
		}
	}

	// 1. Go Fuzzing: Zero Boundary Value Test Case (0 Tokens)
	if numericField != "" && !hasCaseID(step.TestCases, "TC_GO_ZERO_BOUNDARY") {
		zeroPayload := cloneMap(basePayload)
		zeroPayload[numericField] = 0
		for k, prop := range op.RequestSchema.Properties {
			if prop.Type == "array" {
				zeroPayload[k] = []any{}
			}
		}
		step.TestCases = append(step.TestCases, domain.TestCase{
			ID:      "TC_GO_ZERO_BOUNDARY",
			Name:    fmt.Sprintf("[Go Fuzzing] Zero boundary value on field %q", numericField),
			Type:    "go_fuzzing",
			Payload: zeroPayload,
			Assert: domain.AssertionRule{
				Status: 400,
			},
		})
	}

	// 2. Go Fuzzing: Empty String Value Test Case (0 Tokens)
	if stringField != "" && !hasCaseID(step.TestCases, "TC_GO_EMPTY_STRING") {
		emptyPayload := cloneMap(basePayload)
		emptyPayload[stringField] = ""
		step.TestCases = append(step.TestCases, domain.TestCase{
			ID:      "TC_GO_EMPTY_STRING",
			Name:    fmt.Sprintf("[Go Fuzzing] Empty string value on field %q", stringField),
			Type:    "go_fuzzing",
			Payload: emptyPayload,
			Assert: domain.AssertionRule{
				Status: 400,
			},
		})
	}

	// 3. Go Security: SQL Injection Payload Test Case (0 Tokens)
	if stringField != "" && !hasCaseID(step.TestCases, "TC_GO_SQL_INJECTION") {
		sqliPayload := cloneMap(basePayload)
		sqliPayload[stringField] = "' OR '1'='1"
		step.TestCases = append(step.TestCases, domain.TestCase{
			ID:      "TC_GO_SQL_INJECTION",
			Name:    fmt.Sprintf("[Go Security] SQL injection payload on field %q", stringField),
			Type:    "go_security",
			Payload: sqliPayload,
			Assert: domain.AssertionRule{
				Status: 400,
			},
		})
	}

	// 4. Go Security: Cross-Site Scripting (XSS) Payload Test Case (0 Tokens)
	if stringField != "" && !hasCaseID(step.TestCases, "TC_GO_XSS_INJECTION") {
		xssPayload := cloneMap(basePayload)
		xssPayload[stringField] = "<script>alert(1)</script>"
		step.TestCases = append(step.TestCases, domain.TestCase{
			ID:      "TC_GO_XSS_INJECTION",
			Name:    fmt.Sprintf("[Go Security] Cross-site scripting (XSS) payload on field %q", stringField),
			Type:    "go_security",
			Payload: xssPayload,
			Assert: domain.AssertionRule{
				Status: 400,
			},
		})
	}
}

// extractBasePayload selects a template payload map from the step's request body,
// its first existing test case payload, or a schema-synthesized sample object.
func extractBasePayload(step *domain.Step, schema openapi.SchemaObject) map[string]any {
	if m, ok := step.Request.Body.(map[string]any); ok && len(m) > 0 {
		return cloneMap(m)
	}
	for _, tc := range step.TestCases {
		if m, ok := tc.Payload.(map[string]any); ok && len(m) > 0 {
			return cloneMap(m)
		}
	}
	return openapi.SynthesizeSamplePayload(schema)
}

// hasCaseType reports whether cases already contains a TestCase of the specified type.
func hasCaseType(cases []domain.TestCase, caseType string) bool {
	for _, tc := range cases {
		if tc.Type == caseType {
			return true
		}
	}
	return false
}

// hasCaseID reports whether cases already contains a TestCase with the specified ID.
func hasCaseID(cases []domain.TestCase, id string) bool {
	for _, tc := range cases {
		if tc.ID == id {
			return true
		}
	}
	return false
}

// cloneMap creates a shallow copy of a string-keyed payload map so mutations do not alias.
func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
