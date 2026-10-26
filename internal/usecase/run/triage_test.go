package run

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type mockLLMService struct {
	nameFn     func() string
	generateFn func(ctx context.Context, sys, user string, schema interface{}) (string, error)
}

func (m *mockLLMService) Name() string {
	if m.nameFn != nil {
		return m.nameFn()
	}
	return "mock-llm"
}

func (m *mockLLMService) GenerateStructured(ctx context.Context, sys, user string, schema interface{}) (string, error) {
	if m.generateFn != nil {
		return m.generateFn(ctx, sys, user, schema)
	}
	return `{"root_cause": "Invalid token", "suggested_fix": "Login first to obtain bearer token"}`, nil
}

func TestTriage_NilLLM(t *testing.T) {
	diag, err := DiagnoseFailure(context.Background(), nil, TriageContext{
		ScenarioID:     "sc1",
		ExpectedStatus: 200,
		ActualStatus:   401,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diag != "" {
		t.Errorf("expected empty diagnosis for nil LLM, got: %s", diag)
	}
}

func TestTriage_SuccessfulDiagnosis(t *testing.T) {
	mock := &mockLLMService{
		generateFn: func(ctx context.Context, sys, user string, schema interface{}) (string, error) {
			if !strings.Contains(user, "ACTUAL STATUS: 422") {
				t.Errorf("expected user prompt to contain actual status 422")
			}
			return `{"root_cause": "Product out of stock", "suggested_fix": "Increase inventory before ordering"}`, nil
		},
	}

	diag, err := DiagnoseFailure(context.Background(), mock, TriageContext{
		ScenarioID:     "sc-order",
		ScenarioName:   "Create order",
		StepCall:       "POST {order}/api/orders",
		ExpectedStatus: 201,
		ActualStatus:   422,
		ResponseBody:   `{"error": "insufficient_stock"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(diag, "Root cause: Product out of stock") || !strings.Contains(diag, "Fix: Increase inventory") {
		t.Errorf("unexpected diagnosis output: %s", diag)
	}
}

func TestTriage_LLMError(t *testing.T) {
	mock := &mockLLMService{
		generateFn: func(ctx context.Context, sys, user string, schema interface{}) (string, error) {
			return "", errors.New("rate limit exceeded")
		},
	}

	_, err := DiagnoseFailure(context.Background(), mock, TriageContext{
		ScenarioID:     "sc1",
		ExpectedStatus: 200,
		ActualStatus:   500,
	})
	if err == nil {
		t.Fatalf("expected error from LLM failure, got nil")
	}
}
