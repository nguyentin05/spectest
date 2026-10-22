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

	// 1. Go Fuzzing: Zero/Empty Value Test Case (0 Tokens)
	if numericField != "" && !hasCaseType(step.TestCases, "go_fuzzing") {
		zeroPayload := cloneMap(basePayload)
		zeroPayload[numericField] = 0
		for k, prop := range op.RequestSchema.Properties {
			if prop.Type == "array" {
				zeroPayload[k] = []interface{}{}
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

	// 2. Go Security: SQL Injection Payload Test Case (0 Tokens)
	if stringField != "" && !hasCaseType(step.TestCases, "go_security") {
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
}

// extractBasePayload selects a template payload map from the step's request body,
// its first existing test case payload, or a schema-synthesized sample object.
func extractBasePayload(step *domain.Step, schema openapi.SchemaObject) map[string]interface{} {
	if m, ok := step.Request.Body.(map[string]interface{}); ok && len(m) > 0 {
		return cloneMap(m)
	}
	for _, tc := range step.TestCases {
		if m, ok := tc.Payload.(map[string]interface{}); ok && len(m) > 0 {
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

// cloneMap creates a shallow copy of a string-keyed payload map so mutations do not alias.
func cloneMap(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
