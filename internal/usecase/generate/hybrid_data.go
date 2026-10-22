package generate

import (
	"fmt"
	"sort"

	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/openapi"
)

// EnrichStepWithHybridData augments a mutating Step that has a request schema with
// deterministic Go boundary fuzzing (`go_fuzzing`) and security (`go_security`) test cases
// at zero token cost, combining them with valid and boundary business test cases.
func EnrichStepWithHybridData(step *domain.Step, op openapi.NormalizedOperation) {
	if op.IsTokenProducer || op.RequestSchema == nil || len(op.RequestSchema.Properties) == 0 {
		return
	}
	if op.Method != "POST" && op.Method != "PUT" && op.Method != "PATCH" {
		return
	}

	basePayload := openapi.SynthesizeSamplePayload(*op.RequestSchema)
	if m, ok := step.Request.Body.(map[string]interface{}); ok && len(m) > 0 {
		basePayload = cloneMap(m)
	}

	if len(step.TestCases) == 0 {
		step.TestCases = synthesizeBaselineTestCases(op, basePayload)
	}

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

func synthesizeBaselineTestCases(op openapi.NormalizedOperation, basePayload map[string]interface{}) []domain.TestCase {
	expectedValidStatus := 200
	if _, has201 := op.ResponseSchemas["201"]; has201 {
		expectedValidStatus = 201
	}

	cases := []domain.TestCase{
		{
			ID:      "TC_HAPPY_PATH",
			Name:    fmt.Sprintf("Valid request payload for %s %s", op.Method, op.Endpoint),
			Type:    "ai_business_valid",
			Payload: cloneMap(basePayload),
			Assert: domain.AssertionRule{
				Status:      expectedValidStatus,
				SchemaMatch: true,
			},
		},
	}

	// Check if any property defines a minimum constraint to generate a below-minimum boundary case
	for propName, prop := range op.RequestSchema.Properties {
		if (prop.Type == "integer" || prop.Type == "number") && prop.Minimum != nil {
			boundaryPayload := cloneMap(basePayload)
			boundaryPayload[propName] = *prop.Minimum - 1
			cases = append(cases, domain.TestCase{
				ID:      "TC_BOUNDARY_BELOW_MIN",
				Name:    fmt.Sprintf("Boundary value below minimum constraint on field %q", propName),
				Type:    "ai_business_boundary",
				Payload: boundaryPayload,
				Assert: domain.AssertionRule{
					Status: 400,
				},
			})
			break
		}
	}

	return cases
}

func hasCaseType(cases []domain.TestCase, caseType string) bool {
	for _, tc := range cases {
		if tc.Type == caseType {
			return true
		}
	}
	return false
}

func cloneMap(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
