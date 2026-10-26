package generate

import (
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
)

func TestImpact_DirectAndTransitive(t *testing.T) {
	suite := &domain.Suite{
		Setup: map[string]domain.Setup{
			"auth_setup": {
				Steps: []domain.Step{
					{Call: "POST {shop}/api/login", Expect: 200},
				},
			},
			"cart_setup": {
				Use: []string{"auth_setup"},
				Steps: []domain.Step{
					{Call: "POST {shop}/api/cart", Expect: 201},
				},
			},
		},
		Scenarios: []domain.Scenario{
			{
				ID:    "scenario-login",
				Setup: []string{"auth_setup"},
				Steps: []domain.Step{
					{Call: "GET {shop}/health", Expect: 200},
				},
			},
			{
				ID:    "scenario-order",
				Setup: []string{"cart_setup"},
				Steps: []domain.Step{
					{Call: "POST {shop}/api/orders", Expect: 201},
				},
			},
			{
				ID: "scenario-independent",
				Steps: []domain.Step{
					{Call: "GET {shop}/health", Expect: 200},
				},
			},
		},
	}

	apis := map[string]domain.API{
		"POST {shop}/api/login":  {Key: "POST {shop}/api/login", Method: "POST", Server: "shop", Path: "/api/login"},
		"POST {shop}/api/cart":   {Key: "POST {shop}/api/cart", Method: "POST", Server: "shop", Path: "/api/cart"},
		"POST {shop}/api/orders": {Key: "POST {shop}/api/orders", Method: "POST", Server: "shop", Path: "/api/orders"},
		"GET {shop}/health":      {Key: "GET {shop}/health", Method: "GET", Server: "shop", Path: "/health"},
	}

	index, err := BuildImpactIndex(suite, apis)
	if err != nil {
		t.Fatalf("BuildImpactIndex failed: %v", err)
	}

	// 1. If POST {shop}/api/login changes, auth_setup is direct setup.
	// Scenarios depending on auth_setup: scenario-login (direct dep) and scenario-order (via cart_setup).
	diff := APIDiff{
		Changed: []APIChange{
			{Key: "POST {shop}/api/login"},
		},
	}

	impact, err := ComputeImpact(suite, diff, index)
	if err != nil {
		t.Fatalf("ComputeImpact failed: %v", err)
	}

	if len(impact.Setups) != 1 || impact.Setups[0] != "auth_setup" {
		t.Errorf("expected setups [auth_setup], got %v", impact.Setups)
	}
	if len(impact.Scenarios) != 2 || impact.Scenarios[0] != "scenario-login" || impact.Scenarios[1] != "scenario-order" {
		t.Errorf("expected scenarios [scenario-login, scenario-order], got %v", impact.Scenarios)
	}
}

func TestImpact_PathParameterMatching(t *testing.T) {
	suite := &domain.Suite{
		Scenarios: []domain.Scenario{
			{
				ID: "get-user",
				Steps: []domain.Step{
					{Call: "GET {shop}/api/users/{{userId}}?include=all", Expect: 200},
				},
			},
		},
	}
	apis := map[string]domain.API{
		"GET {shop}/api/users/{id}": {
			Key:    "GET {shop}/api/users/{id}",
			Method: "GET",
			Server: "shop",
			Path:   "/api/users/{id}",
		},
	}

	index, err := BuildImpactIndex(suite, apis)
	if err != nil {
		t.Fatalf("BuildImpactIndex failed: %v", err)
	}

	diff := APIDiff{
		Changed: []APIChange{{Key: "GET {shop}/api/users/{id}"}},
	}
	impact, err := ComputeImpact(suite, diff, index)
	if err != nil {
		t.Fatalf("ComputeImpact failed: %v", err)
	}

	if len(impact.Scenarios) != 1 || impact.Scenarios[0] != "get-user" {
		t.Errorf("expected [get-user], got %v", impact.Scenarios)
	}
}
