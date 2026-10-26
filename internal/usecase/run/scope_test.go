package run

import (
	"net/http"
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
)

func TestScope_SeedCollision(t *testing.T) {
	seeds := map[string]domain.Seed{
		"admin": {
			Vars: map[string]any{"shared_var": "val1", "admin_email": "admin@test.com"},
		},
		"user": {
			Vars: map[string]any{"shared_var": "val2", "user_email": "user@test.com"},
		},
	}

	_, err := NewScopeFromSeed(seeds)
	if err == nil {
		t.Fatalf("expected error on duplicate seed variable, got nil")
	}
}

func TestScope_SeedSuccessAndIsolation(t *testing.T) {
	seeds := map[string]domain.Seed{
		"admin": {
			Vars: map[string]any{"admin_email": "admin@test.com", "port": 8080},
		},
		"client": {
			Vars: map[string]any{"client_id": "c123"},
		},
	}

	parentScope, err := NewScopeFromSeed(seeds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Scenario 1 gets an isolated clone
	sc1Scope := parentScope.Clone()
	sc1Scope.Set("token", "token-sc1")

	// Scenario 2 gets an isolated clone
	sc2Scope := parentScope.Clone()
	sc2Scope.Set("token", "token-sc2")

	// Verify isolation
	tok1, _ := sc1Scope.Get("token")
	tok2, _ := sc2Scope.Get("token")
	if tok1 != "token-sc1" || tok2 != "token-sc2" {
		t.Errorf("scopes not isolated: tok1=%v, tok2=%v", tok1, tok2)
	}
	if _, ok := parentScope.Get("token"); ok {
		t.Errorf("parent scope was modified by child")
	}
}

func TestScope_InterpolateString(t *testing.T) {
	scope := NewScope()
	scope.Set("email", "john@example.com")
	scope.Set("id", 42)

	input := "POST /api/users/{{id}}?email={{email}}&client={{random}}"
	out1 := scope.InterpolateString(input)
	out2 := scope.InterpolateString(input)

	if !testing.Short() {
		if out1 == out2 {
			t.Errorf("expected different random tokens across evaluations, got %q and %q", out1, out2)
		}
	}

	if out1 == input || len(out1) < len(input) {
		t.Errorf("failed string interpolation: %s", out1)
	}
}

func TestScope_InterpolateBody_TypePreservation(t *testing.T) {
	scope := NewScope()
	scope.Set("count", 100)
	scope.Set("active", true)
	scope.Set("ratio", 3.14)
	scope.Set("tags", []any{"admin", "ops"})

	body := map[string]any{
		"name":        "user_{{count}}",
		"items_count": "{{count}}",
		"is_active":   "{{active}}",
		"rate":        "{{ratio}}",
		"user_tags":   "{{tags}}",
	}

	interpolated := scope.InterpolateBody(body).(map[string]any)

	// String interpolation
	if interpolated["name"] != "user_100" {
		t.Errorf("expected user_100, got %v", interpolated["name"])
	}
	// Raw type preservation
	if count, ok := interpolated["items_count"].(int); !ok || count != 100 {
		t.Errorf("expected int 100, got %T (%v)", interpolated["items_count"], interpolated["items_count"])
	}
	if active, ok := interpolated["is_active"].(bool); !ok || !active {
		t.Errorf("expected bool true, got %T (%v)", interpolated["is_active"], interpolated["is_active"])
	}
	if ratio, ok := interpolated["rate"].(float64); !ok || ratio != 3.14 {
		t.Errorf("expected float64 3.14, got %T (%v)", interpolated["rate"], interpolated["rate"])
	}
	if tags, ok := interpolated["user_tags"].([]any); !ok || len(tags) != 2 {
		t.Errorf("expected slice []any of len 2, got %T (%v)", interpolated["user_tags"], interpolated["user_tags"])
	}
}

func TestScope_ExtractSave(t *testing.T) {
	scope := NewScope()

	respBody := []byte(`{
		"status": "success",
		"data": {
			"token": "secret-jwt-123",
			"orders": [
				{"id": "ord-001", "total": 99.9},
				{"id": "ord-002", "total": 199.5}
			]
		}
	}`)

	headers := http.Header{}
	headers.Set("X-Custom-Header", "custom-value")
	headers.Set("Content-Type", "application/json")

	saveMapping := map[string]string{
		"jwt":          "body.data.token",
		"first_order":  "body.data.orders.0.id",
		"second_order": "body.data.orders.1.id",
		"hdr_val":      "header.X-Custom-Header",
	}

	extracted, err := scope.ExtractSave(saveMapping, respBody, headers)
	if err != nil {
		t.Fatalf("ExtractSave failed: %v", err)
	}

	if extracted["jwt"] != "secret-jwt-123" {
		t.Errorf("expected secret-jwt-123, got %v", extracted["jwt"])
	}
	if extracted["first_order"] != "ord-001" {
		t.Errorf("expected ord-001, got %v", extracted["first_order"])
	}
	if extracted["second_order"] != "ord-002" {
		t.Errorf("expected ord-002, got %v", extracted["second_order"])
	}
	if extracted["hdr_val"] != "custom-value" {
		t.Errorf("expected custom-value, got %v", extracted["hdr_val"])
	}

	// Verify values are in scope
	if val, ok := scope.Get("jwt"); !ok || val != "secret-jwt-123" {
		t.Errorf("scope missing jwt value")
	}
}

func TestScope_ExtractSave_Errors(t *testing.T) {
	scope := NewScope()
	respBody := []byte(`{"items": [1, 2]}`)
	headers := http.Header{}

	// Invalid array index
	_, err := scope.ExtractSave(map[string]string{"val": "body.items.99"}, respBody, headers)
	if err == nil {
		t.Errorf("expected out of bounds error, got nil")
	}

	// Missing map key
	_, err = scope.ExtractSave(map[string]string{"val": "body.missing_key"}, respBody, headers)
	if err == nil {
		t.Errorf("expected key not found error, got nil")
	}

	// Unsupported expression
	_, err = scope.ExtractSave(map[string]string{"val": "cookie.session"}, respBody, headers)
	if err == nil {
		t.Errorf("expected unsupported expression error, got nil")
	}
}
