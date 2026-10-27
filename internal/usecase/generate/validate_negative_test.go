package generate

import (
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
)

func TestValidateStepsDeep_NegativeTestRelaxation(t *testing.T) {
	api := domain.API{
		Key:    "POST {server}/users",
		Method: "POST",
		Server: "server",
		Path:   "/users",
		Request: &domain.RequestSpec{
			Required: true,
			JSON:     true,
			Schema: domain.Schema{
				Type:     "object",
				Required: []string{"email", "password"},
				Properties: map[string]domain.Schema{
					"email":    {Type: "string"},
					"password": {Type: "string"},
				},
			},
		},
		Responses: map[string]domain.Schema{
			"201": {Type: "object"},
			"400": {Type: "object"},
		},
	}
	apis := map[string]domain.API{api.Key: api}
	servers := map[string]struct{}{"server": {}}

	// 1. Missing required field on 201 must FAIL
	step201 := domain.Step{
		Call:   api.Key,
		Expect: 201,
		Body:   map[string]any{"email": "test@example.com"}, // missing password
	}
	errs, _ := validateStepsDeep("test", []domain.Step{step201}, apis, servers, nil, nil)
	if len(errs) == 0 {
		t.Error("expected 201 step with missing required field to fail validation")
	}

	// 2. Missing required field on 400 must PASS
	step400 := domain.Step{
		Call:   api.Key,
		Expect: 400,
		Body:   map[string]any{"email": "test@example.com"}, // missing password intentionally
	}
	errs, _ = validateStepsDeep("test", []domain.Step{step400}, apis, servers, nil, nil)
	if len(errs) > 0 {
		t.Errorf("expected 400 negative-test step with missing required field to pass, got: %v", errs)
	}
}
