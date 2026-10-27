package generate

import (
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
)

func TestGroundSetups_ValidationAndCycleDrop(t *testing.T) {
	facts := map[string]*apiFacts{
		"POST {server}/login": {
			Key:                "POST {server}/login",
			Method:             "POST",
			Server:             "server",
			Path:               "/login",
			DocumentedStatuses: []string{"200"},
			Responses:          []responseFacts{{Status: "200", BodyDocumented: true}},
		},
	}

	// 1. Setup saving body on documented response body should succeed
	rawSetups := map[string]rawSetupPlan{
		"login_user": {
			Steps: []domain.Step{
				{
					Call:   "POST {server}/login",
					Expect: 200,
					Save:   map[string]string{"token": "body.token"},
				},
			},
		},
	}
	apis := map[string]domain.API{
		"POST {server}/login": {
			Key:       "POST {server}/login",
			Method:    "POST",
			Server:    "server",
			Path:      "/login",
			Responses: map[string]domain.Schema{"200": {Type: "object"}},
		},
	}
	servers := map[string]struct{}{"server": {}}

	valid, unresolved, warns := groundSetups(rawSetups, facts, apis, servers, nil)
	if len(valid) != 1 {
		t.Fatalf("expected 1 valid setup, got %d (warns: %v, unresolved: %v)", len(valid), warns, unresolved)
	}

	// 2. Setup cycle should be dropped
	cyclicSetups := map[string]rawSetupPlan{
		"a": {Use: []string{"b"}, Steps: []domain.Step{{Call: "POST {server}/login", Expect: 200}}},
		"b": {Use: []string{"a"}, Steps: []domain.Step{{Call: "POST {server}/login", Expect: 200}}},
	}
	valid, _, _ = groundSetups(cyclicSetups, facts, apis, servers, nil)
	if len(valid) != 0 {
		t.Fatalf("expected 0 valid setups for cyclic dependency, got %d", len(valid))
	}
}

func TestGroundScenario_InferenceRequirement(t *testing.T) {
	fact := &apiFacts{
		Key:                "GET {server}/me",
		Method:             "GET",
		Server:             "server",
		Path:               "/me",
		DocumentedStatuses: []string{"200"},
	}
	apis := map[string]domain.API{
		"GET {server}/me": {
			Key:       "GET {server}/me",
			Method:    "GET",
			Server:    "server",
			Path:      "/me",
			Responses: map[string]domain.Schema{"200": {Type: "object"}},
		},
	}
	servers := map[string]struct{}{"server": {}}

	// 1. Undocumented 401 without declared inference -> dropped
	rawWithoutInference := rawScenario{
		ID:   "me-unauth",
		Name: "Me unauth",
		Steps: []domain.Step{
			{Call: "GET {server}/me", Expect: 401},
		},
	}
	_, _, err := groundScenario(rawWithoutInference, fact, apis, servers, nil, nil, nil)
	if err == nil {
		t.Error("expected error for undocumented expect without declared inference")
	}

	// 2. Undocumented 401 WITH declared inference -> accepted
	declaredInferences := []inferredExpect{
		{ScenarioID: "me-unauth", Expect: 401, Basis: "auth", Reason: "no token"},
	}
	sc, inf, err := groundScenario(rawWithoutInference, fact, apis, servers, nil, nil, declaredInferences)
	if err != nil {
		t.Fatalf("expected declared inference to pass, got: %v", err)
	}
	if sc == nil || len(inf) != 1 {
		t.Fatalf("expected grounded scenario with 1 inference, got: %v, %v", sc, inf)
	}
}

func TestMergeOutcomes_Deduplication(t *testing.T) {
	outcomes := []apiOutcome{
		{
			APIKey: "GET {server}/a",
			Scenarios: []domain.Scenario{
				{ID: "test-case", Name: "Test A"},
			},
		},
		{
			APIKey: "GET {server}/b",
			Scenarios: []domain.Scenario{
				{ID: "test-case", Name: "Test B"},
			},
		},
	}
	merged, warns := mergeOutcomes(outcomes)
	if len(merged.Scenarios) != 2 {
		t.Fatalf("expected 2 merged scenarios, got %d", len(merged.Scenarios))
	}
	if merged.Scenarios[0].ID == merged.Scenarios[1].ID {
		t.Errorf("expected scenario IDs to be deduplicated, got: %s and %s", merged.Scenarios[0].ID, merged.Scenarios[1].ID)
	}
	if len(warns) == 0 {
		t.Error("expected warning for ID collision")
	}
}

func TestFixSingleBraceTemplates(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Bearer {access_token}", "Bearer {{access_token}}"},
		{"Bearer {{access_token}}", "Bearer {{access_token}}"},
		{"{id}", "{{id}}"},
		{"{{id}}", "{{id}}"},
		{"user_{random}@example.com", "user_{{random}}@example.com"},
		{"normal text without braces", "normal text without braces"},
	}

	for _, tc := range cases {
		got := fixSingleBraceTemplates(tc.in)
		if got != tc.want {
			t.Errorf("fixSingleBraceTemplates(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

