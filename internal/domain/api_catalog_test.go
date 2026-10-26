package domain_test

import (
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
	"gopkg.in/yaml.v3"
)

func TestAPICatalog_YAMLSerialization(t *testing.T) {
	entry := domain.UpdateEntry{
		N:  1,
		At: "2026-10-26T12:00:00Z",
		By: domain.ByAI,
		Trigger: &domain.Trigger{
			OpenAPI: &domain.OpenAPITrigger{
				Added:    []string{"POST {shop}/api/orders"},
				Restored: []string{"GET {shop}/api/products"},
			},
		},
		CatalogPatch: domain.APICatalog{
			"POST {shop}/api/orders": {
				Hash:   "abc123456789",
				Status: domain.StatusActive,
				Sig: domain.APISignature{
					Responses: []int{201},
				},
			},
		},
		Impact: domain.UpdateImpact{
			Setups:    []string{"authenticated_user"},
			Scenarios: []string{"create-order"},
		},
		AfterHash: "123456789012",
	}

	data, err := yaml.Marshal(entry)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded domain.UpdateEntry
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if len(decoded.Impact.Scenarios) != 1 || decoded.Impact.Scenarios[0] != "create-order" {
		t.Errorf("expected scenario create-order, got %v", decoded.Impact.Scenarios)
	}
	if decoded.Trigger == nil || decoded.Trigger.OpenAPI == nil || len(decoded.Trigger.OpenAPI.Restored) != 1 || decoded.Trigger.OpenAPI.Restored[0] != "GET {shop}/api/products" {
		t.Errorf("expected restored API, got %v", decoded.Trigger)
	}
}
