package run

import (
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
)

func TestReplayEngineAnd4TierOracle(t *testing.T) {
	suite := &domain.TestSuiteFile{
		Version: "1.0",
		Base: domain.BaseSuite{
			Scenarios: []domain.Scenario{
				{
					ID:   "SC_01",
					Name: "Checkout Flow",
					Steps: []domain.Step{
						{
							ID:   "STEP_02_CHECKOUT",
							Name: "Checkout",
							TestCases: []domain.TestCase{
								{
									ID:   "TC_HAPPY",
									Name: "Valid",
									Payload: map[string]interface{}{
										"voucher_code": "SALE50",
										"total_amount": 600000,
									},
								},
							},
						},
					},
				},
			},
		},
		Updates: []domain.UpdateBlock{
			{
				UpdateID: "UPD_001",
				Actions: []domain.UpdateAction{
					{
						Op:             "modify_step",
						TargetScenario: "SC_01",
						TargetStep:     "STEP_02_CHECKOUT",
						OverridePayload: map[string]interface{}{
							"promo_code":    "SALE50",
							"shipping_type": "EXPRESS",
						},
						RemoveFields: []string{"voucher_code"},
					},
				},
			},
		},
	}

	effective := BuildEffectiveSuite(suite)
	payload, ok := effective.Scenarios[0].Steps[0].TestCases[0].Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map payload")
	}
	if _, hasOld := payload["voucher_code"]; hasOld {
		t.Errorf("expected voucher_code to be removed by replay engine")
	}
	if payload["promo_code"] != "SALE50" || payload["shipping_type"] != "EXPRESS" {
		t.Errorf("unexpected replayed payload: %+v", payload)
	}

	// Test 4-Tier Oracle evaluation
	rule := &domain.AssertionRule{
		Status:       201,
		MaxLatencyMs: 500,
		SchemaMatch:  true,
		ForbidKeys:   []string{"password_hash", "salt"},
		JSONPath: []string{
			"$.data.status == 'CREATED'",
			"$.data.final_amount == 550000",
			"$.data.order_id == '{{created_order_id}}'",
		},
	}

	goodBody := []byte(`{"data":{"order_id":"ORD_1001","status":"CREATED","final_amount":550000}}`)
	state := map[string]string{"created_order_id": "ORD_1001"}
	eval := EvaluateOracle(rule, 201, 45, goodBody, state)
	if !eval.Passed {
		t.Fatalf("expected oracle to pass, failed at %s: %s", eval.FailedTier, eval.Reason)
	}

	// Leak forbidden key
	leakBody := []byte(`{"data":{"order_id":"ORD_1001","password_hash":"secret"}}`)
	evalLeak := EvaluateOracle(rule, 201, 45, leakBody, state)
	if evalLeak.Passed {
		t.Fatalf("expected oracle to fail when password_hash is leaked")
	}
}
