package domain

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestTestSuiteFile_SerializationRoundtrip(t *testing.T) {
	orig := &TestSuiteFile{
		Version: "1.0",
		Metadata: SuiteMetadata{
			Name:          "E-Commerce Core API",
			Description:   "Generated test suite for checkout and cart workflows",
			TargetBaseURL: "https://api.store.example.com",
			Env: map[string]string{
				"STAGE": "staging",
			},
		},
		Base: BaseSuite{
			Scenarios: []Scenario{
				{
					ID:          "SCN_CHECKOUT",
					Name:        "End-to-End Checkout",
					Description: "Tests creating cart, adding items, and payment",
					Steps: []Step{
						{
							ID:   "STEP_CREATE_CART",
							Name: "Create cart",
							TargetRef: &TargetRef{
								Endpoint:    "/api/v1/cart",
								Method:      "POST",
								OperationID: "createCart",
							},
							Request: RequestDef{
								Method:   "POST",
								Endpoint: "/api/v1/cart",
								Headers: map[string]string{
									"Authorization": "Bearer {{token}}",
								},
								Body: map[string]interface{}{
									"currency": "USD",
								},
							},
							Extract: map[string]string{
								"cart_id": "$.id",
							},
							TestCases: []TestCase{
								{
									ID:      "TC_DEFAULT",
									Name:    "Standard cart creation",
									Type:    "positive",
									Payload: map[string]interface{}{"currency": "USD"},
									Assert: AssertionRule{
										Status: 201,
									},
								},
							},
							Assert: &AssertionRule{
								Status: 201,
							},
						},
					},
				},
			},
		},
	}

	// 1. YAML Roundtrip
	yamlBytes, err := yaml.Marshal(orig)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}

	var yamlLoaded TestSuiteFile
	if err := yaml.Unmarshal(yamlBytes, &yamlLoaded); err != nil {
		t.Fatalf("yaml.Unmarshal failed: %v", err)
	}

	if yamlLoaded.Version != orig.Version {
		t.Errorf("expected version %s, got %s", orig.Version, yamlLoaded.Version)
	}
	if yamlLoaded.Metadata.Name != orig.Metadata.Name {
		t.Errorf("expected metadata name %s, got %s", orig.Metadata.Name, yamlLoaded.Metadata.Name)
	}
	if len(yamlLoaded.Base.Scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(yamlLoaded.Base.Scenarios))
	}
	if yamlLoaded.Base.Scenarios[0].Steps[0].TargetRef.OperationID != "createCart" {
		t.Errorf("expected operationID createCart, got %s", yamlLoaded.Base.Scenarios[0].Steps[0].TargetRef.OperationID)
	}

	// 2. JSON Roundtrip
	jsonBytes, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var jsonLoaded TestSuiteFile
	if err := json.Unmarshal(jsonBytes, &jsonLoaded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if jsonLoaded.Metadata.TargetBaseURL != orig.Metadata.TargetBaseURL {
		t.Errorf("expected target_base_url %s, got %s", orig.Metadata.TargetBaseURL, jsonLoaded.Metadata.TargetBaseURL)
	}
	if jsonLoaded.Base.Scenarios[0].Steps[0].Extract["cart_id"] != "$.id" {
		t.Errorf("expected cart_id extraction '$.id', got %s", jsonLoaded.Base.Scenarios[0].Steps[0].Extract["cart_id"])
	}
}
