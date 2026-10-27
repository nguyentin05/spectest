package generate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
)

type mockInitLLM struct {
	handler func(systemPrompt, userPrompt string) (string, error)
}

func (m *mockInitLLM) Name() string { return "mock-init-llm" }

func (m *mockInitLLM) GenerateStructured(_ context.Context, systemPrompt, userPrompt string, _ any) (string, error) {
	if m.handler != nil {
		return m.handler(systemPrompt, userPrompt)
	}
	return "{}", nil
}

func TestStage1_AnalyzeContract(t *testing.T) {
	llm := &mockInitLLM{
		handler: func(systemPrompt, userPrompt string) (string, error) {
			if strings.Contains(userPrompt, "operations") {
				return `{
					"global_notes": ["User must verify email", "User must verify email"],
					"overrides": [{"api": "POST {auth}/login", "item": "status", "openapi": "200", "spec": "201"}],
					"ambiguities": ["JWT expiration"],
					"missing_info": ["Password salt"],
					"requirements_language": ""
				}`, nil
			}
			return "{}", nil
		},
	}

	facts := map[string]*apiFacts{
		"POST {auth}/login": {
			Key:    "POST {auth}/login",
			Method: "POST",
			Path:   "/login",
		},
	}

	res, err := analyzeContract(context.Background(), llm, facts, "Simple requirements")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.GlobalNotes) != 1 || res.GlobalNotes[0] != "User must verify email" {
		t.Errorf("expected 1 deduplicated global note, got %v", res.GlobalNotes)
	}
	if res.RequirementsLanguage != "English" {
		t.Errorf("expected language to default to English, got %q", res.RequirementsLanguage)
	}
	if len(res.Overrides) != 1 {
		t.Errorf("expected 1 override, got %d", len(res.Overrides))
	}
}

func TestStage2_PlanSetup(t *testing.T) {
	llm := &mockInitLLM{
		handler: func(systemPrompt, userPrompt string) (string, error) {
			return `{
				"seed": {
					"auth": {
						"vars": {
							"email": "admin@example.com",
							"bad": "{{template_not_allowed}}"
						}
					}
				},
				"setup": {
					"valid_setup": {
						"steps": [
							{
								"call": "POST {auth}/login",
								"expect": 200,
								"save": {"token": "body.token"}
							}
						]
					}
				},
				"unresolved": [
					{"api": "*", "missing": "email server", "affects": "verification"}
				],
				"assumptions": [
					{"area": "auth", "statement": "admin exists", "reason": "seeded in db"}
				]
			}`, nil
		},
	}

	facts := map[string]*apiFacts{
		"POST {auth}/login": {
			Key:                "POST {auth}/login",
			Method:             "POST",
			Server:             "auth",
			Path:               "/login",
			DocumentedStatuses: []string{"200"},
			Responses: []responseFacts{
				{Status: "200", BodyDocumented: true},
			},
		},
	}
	apis := map[string]domain.API{
		"POST {auth}/login": {
			Key:    "POST {auth}/login",
			Method: "POST",
			Path:   "/login",
			Server: "auth",
			Responses: map[string]domain.Schema{
				"200": {Type: "object", Properties: map[string]domain.Schema{"token": {Type: "string"}}},
			},
		},
	}
	servers := map[string]struct{}{"auth": {}}

	setups, seeds, unresolved, assumptions, warns, err := planSetups(
		context.Background(),
		llm,
		facts,
		[]string{"Note"},
		"Requirements",
		apis,
		servers,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := setups["valid_setup"]; !ok {
		t.Errorf("expected valid_setup to be present")
	}
	// "bad" seed var with {{template}} should have been dropped
	if _, ok := seeds["auth"].Vars["bad"]; ok {
		t.Errorf("expected template seed variable to be dropped")
	}
	if seeds["auth"].Vars["email"] != "admin@example.com" {
		t.Errorf("expected email seed var to be preserved")
	}
	if len(unresolved) == 0 {
		t.Errorf("expected unresolved items")
	}
	if len(assumptions) != 1 {
		t.Errorf("expected 1 assumption, got %d", len(assumptions))
	}
	_ = warns
}

func TestStage3_GenerateScenarios_Resilience(t *testing.T) {
	llm := &mockInitLLM{
		handler: func(systemPrompt, userPrompt string) (string, error) {
			if strings.Contains(userPrompt, "POST {auth}/login") {
				return `{
					"api": "POST {auth}/login",
					"scenarios": [
						{
							"id": "login-success",
							"name": "Successful login",
							"steps": [
								{
									"call": "POST {auth}/login",
									"expect": 200
								}
							]
						}
					]
				}`, nil
			}
			// Simulate failure on the second API
			return "", errors.New("temporary LLM rate limit")
		},
	}

	facts := map[string]*apiFacts{
		"POST {auth}/login": {
			Key:                "POST {auth}/login",
			Method:             "POST",
			Server:             "auth",
			Path:               "/login",
			DocumentedStatuses: []string{"200"},
		},
		"GET {auth}/me": {
			Key:                "GET {auth}/me",
			Method:             "GET",
			Server:             "auth",
			Path:               "/me",
			DocumentedStatuses: []string{"200"},
		},
	}
	apis := map[string]domain.API{
		"POST {auth}/login": {
			Key:       "POST {auth}/login",
			Method:    "POST",
			Path:      "/login",
			Server:    "auth",
			Responses: map[string]domain.Schema{"200": {}},
		},
		"GET {auth}/me": {
			Key:       "GET {auth}/me",
			Method:    "GET",
			Path:      "/me",
			Server:    "auth",
			Responses: map[string]domain.Schema{"200": {}},
		},
	}
	servers := map[string]struct{}{"auth": {}}

	merged, warns, err := generateScenarios(
		context.Background(),
		llm,
		facts,
		nil,
		nil,
		nil,
		2,
		apis,
		servers,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have succeeded for login even though me failed
	if len(merged.Scenarios) != 1 {
		t.Fatalf("expected 1 scenario from the successful API, got %d", len(merged.Scenarios))
	}
	if merged.Scenarios[0].ID != "login-success" {
		t.Errorf("expected scenario id login-success, got %s", merged.Scenarios[0].ID)
	}
	if len(warns) == 0 {
		t.Errorf("expected warning for the failing API")
	}
}

func TestStage4_WriteAnalysis_AndManagedBlocks(t *testing.T) {
	llm := &mockInitLLM{
		handler: func(systemPrompt, userPrompt string) (string, error) {
			return `{
				"analysis": "This suite tests the Auth API thoroughly.",
				"assumptions": [
					{"area": "auth", "statement": "Tokens expire in 1 hour", "reason": "Standard JWT"}
				]
			}`, nil
		},
	}

	inferred := []inferredExpect{
		{
			ScenarioID: "login-unauthorized",
			Expect:     401,
			Basis:      "standard auth rejection",
			Reason:     "invalid credentials return 401",
		},
	}

	narrative, assumptions, err := writeInitAnalysis(
		context.Background(),
		llm,
		map[string]any{"total_apis": 1},
		nil,
		nil,
		nil,
		nil,
		inferred,
		nil,
		"English",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if narrative != "This suite tests the Auth API thoroughly." {
		t.Errorf("unexpected narrative: %q", narrative)
	}

	// Inferred expectations should be reflected when merged
	finalAssumptions := mergeAssumptionsWithInferences(assumptions, inferred)
	if len(finalAssumptions) < 2 {
		t.Errorf("expected inferred expectation to be added to assumptions, got %d", len(finalAssumptions))
	}

	// Apply managed blocks
	warnings := []string{"Drop unverified scenario X"}
	fullAnalysis := applyInitManagedBlocks(narrative, warnings, finalAssumptions)

	if !strings.Contains(fullAnalysis, "<!-- spectest:warnings:start -->") {
		t.Errorf("missing warnings start marker")
	}
	if !strings.Contains(fullAnalysis, "Drop unverified scenario X") {
		t.Errorf("missing warning content")
	}
	if !strings.Contains(fullAnalysis, "<!-- spectest:assumptions:start -->") {
		t.Errorf("missing assumptions start marker")
	}
	if !strings.Contains(fullAnalysis, "Tokens expire in 1 hour") {
		t.Errorf("missing assumption statement")
	}
}
