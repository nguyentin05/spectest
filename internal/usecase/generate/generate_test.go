package generate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/openapi"
	"github.com/nguyentin05/spectest/internal/infrastructure/storage"
)

// mockLLMService provides a controllable mock implementation of domain.LLMService for testing.
type mockLLMService struct {
	name             string
	generateFunc     func(ctx context.Context, sys, user string, schema interface{}) (string, error)
	generateCallHits atomic.Int64
}

func (m *mockLLMService) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock-llm"
}

func (m *mockLLMService) GenerateStructured(ctx context.Context, sys, user string, schema interface{}) (string, error) {
	m.generateCallHits.Add(1)
	if m.generateFunc != nil {
		return m.generateFunc(ctx, sys, user, schema)
	}
	return "", errors.New("unimplemented mock GenerateStructured")
}

const sampleOpenAPISpec = `openapi: 3.0.3
info:
  title: PetStore Sample API
  version: 1.0.0
servers:
  - url: https://api.petstore.test/v1
paths:
  /auth/token:
    post:
      summary: Obtain bearer token
      tags: [Auth]
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                username: { type: string }
                password: { type: string }
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  access_token: { type: string }
  /pets:
    post:
      summary: Create pet
      tags: [Pets]
      security:
        - BearerAuth: []
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name: { type: string }
                age: { type: integer }
                tags: { type: array, items: { type: string } }
      responses:
        '201':
          description: Created
    get:
      summary: List pets
      tags: [Pets]
      responses:
        '200':
          description: OK
components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
`

const samplePRD = `# Pet Management Requirements

## Section 1: Pets
Users must be authenticated before creating a pet. Age must be a positive integer.
Name must be sanitized to prevent injection attacks.
`

func TestGenerator_Execute_Success(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(sampleOpenAPISpec), 0600); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}
	prdPath := filepath.Join(tmpDir, "requirements.md")
	if err := os.WriteFile(prdPath, []byte(samplePRD), 0600); err != nil {
		t.Fatalf("failed to write prd: %v", err)
	}

	outDir := filepath.Join(tmpDir, "output")

	mockLLM := &mockLLMService{
		generateFunc: func(ctx context.Context, sys, user string, schema interface{}) (string, error) {
			return `{
				"id": "SCN_PETS",
				"name": "Pet Operations Scenario",
				"steps": [
					{
						"id": "STEP_CREATE_PET",
						"name": "Create a new pet",
						"request": {
							"method": "POST",
							"endpoint": "/pets",
							"headers": {
								"Authorization": "Bearer {{access_token}}"
							},
							"body": {
								"name": "Fido",
								"age": 3
							}
						},
						"test_cases": [
							{
								"id": "TC_HAPPY",
								"name": "Happy path create pet",
								"type": "positive",
								"payload": {
									"name": "Fido",
									"age": 3
								},
								"assert": {
									"status": 201
								}
							}
						]
					}
				]
			}`, nil
		},
	}

	gen := NewGenerator(mockLLM, storage.NewYAMLRepository())
	res, err := gen.Execute(context.Background(), Request{
		APIPath:   specPath,
		ReqPath:   prdPath,
		OutputDir: outDir,
		ExportFmt: "postman",
		Enrich:    false,
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if res == nil {
		t.Fatal("expected non-nil Result")
	}
	if res.Suite == nil {
		t.Fatal("expected non-nil Suite in Result")
	}
	if res.Suite.Metadata.Name != "PetStore Sample API" {
		t.Errorf("expected suite name 'PetStore Sample API', got %q", res.Suite.Metadata.Name)
	}
	if res.Suite.Metadata.TargetBaseURL != "https://api.petstore.test/v1" {
		t.Errorf("expected TargetBaseURL 'https://api.petstore.test/v1', got %q", res.Suite.Metadata.TargetBaseURL)
	}
	if res.AuthProducer != "POST /auth/token" {
		t.Errorf("expected AuthProducer 'POST /auth/token', got %q", res.AuthProducer)
	}
	if res.ChunksProcessed == 0 {
		t.Errorf("expected ChunksProcessed > 0, got %d", res.ChunksProcessed)
	}

	// Verify generated files on disk
	if _, err := os.Stat(res.SuitePath); err != nil {
		t.Errorf("expected SuitePath %s to exist: %v", res.SuitePath, err)
	}
	if _, err := os.Stat(res.MatrixPath); err != nil {
		t.Errorf("expected MatrixPath %s to exist: %v", res.MatrixPath, err)
	}
	if _, err := os.Stat(res.ExportPath); err != nil {
		t.Errorf("expected ExportPath %s to exist: %v", res.ExportPath, err)
	}

	// Verify hybrid data enrichment on POST /pets
	foundFuzz := false
	foundSec := false
	for _, sc := range res.Suite.Base.Scenarios {
		for _, st := range sc.Steps {
			if st.Request.Endpoint == "/pets" && st.Request.Method == "POST" {
				for _, tc := range st.TestCases {
					if tc.Type == "go_fuzzing" {
						foundFuzz = true
						if tc.ID != "TC_GO_ZERO_BOUNDARY" {
							t.Errorf("expected TC ID TC_GO_ZERO_BOUNDARY, got %s", tc.ID)
						}
					}
					if tc.Type == "go_security" {
						foundSec = true
						if tc.ID != "TC_GO_SQL_INJECTION" {
							t.Errorf("expected TC ID TC_GO_SQL_INJECTION, got %s", tc.ID)
						}
					}
				}
			}
		}
	}

	if !foundFuzz {
		t.Errorf("expected POST /pets to be enriched with go_fuzzing test case")
	}
	if !foundSec {
		t.Errorf("expected POST /pets to be enriched with go_security test case")
	}
}

func TestGenerator_Execute_FailFastOnLLMError(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(sampleOpenAPISpec), 0600); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	mockLLM := &mockLLMService{
		generateFunc: func(ctx context.Context, sys, user string, schema interface{}) (string, error) {
			return "", errors.New("upstream LLM rate limit exceeded (429)")
		},
	}

	gen := NewGenerator(mockLLM, storage.NewYAMLRepository())
	res, err := gen.Execute(context.Background(), Request{
		APIPath:   specPath,
		OutputDir: filepath.Join(tmpDir, "out"),
	})

	if err == nil {
		t.Fatal("expected Execute to fail fast on LLM error, but got nil error")
	}
	if res != nil {
		t.Fatalf("expected nil Result on error, got: %+v", res)
	}
	if !errors.Is(err, err) && err.Error() == "" {
		t.Fatal("expected descriptive error message")
	}
}

func TestGenerator_Execute_InvalidLLMResponses(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(sampleOpenAPISpec), 0600); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	tests := []struct {
		name        string
		llmOutput   string
		errContains string
	}{
		{
			name:        "empty response",
			llmOutput:   "   ",
			errContains: "llm returned empty response",
		},
		{
			name:        "invalid json",
			llmOutput:   "not valid json at all",
			errContains: "parse llm scenario json",
		},
		{
			name:        "scenario with no steps",
			llmOutput:   `{"id": "SCN_EMPTY", "name": "No steps", "steps": []}`,
			errContains: "contains no steps",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLLM := &mockLLMService{
				generateFunc: func(ctx context.Context, sys, user string, schema interface{}) (string, error) {
					return tt.llmOutput, nil
				},
			}
			gen := NewGenerator(mockLLM, nil)
			res, err := gen.Execute(context.Background(), Request{
				APIPath:   specPath,
				OutputDir: filepath.Join(tmpDir, "out"),
			})
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.errContains)
			}
			if res != nil {
				t.Fatalf("expected nil result on error, got %+v", res)
			}
		})
	}
}

func TestGenerator_Execute_ValidationErrors(t *testing.T) {
	t.Run("nil llm service", func(t *testing.T) {
		gen := NewGenerator(nil, nil)
		_, err := gen.Execute(context.Background(), Request{APIPath: "spec.yaml"})
		if err == nil {
			t.Fatal("expected error with nil LLM service")
		}
	})

	t.Run("missing openapi file", func(t *testing.T) {
		mockLLM := &mockLLMService{}
		gen := NewGenerator(mockLLM, nil)
		_, err := gen.Execute(context.Background(), Request{APIPath: "/nonexistent/openapi.yaml"})
		if err == nil {
			t.Fatal("expected error reading missing openapi file")
		}
	})

	t.Run("invalid openapi spec syntax", func(t *testing.T) {
		tmpDir := t.TempDir()
		specPath := filepath.Join(tmpDir, "invalid.yaml")
		_ = os.WriteFile(specPath, []byte("::not valid yaml::"), 0600)

		mockLLM := &mockLLMService{}
		gen := NewGenerator(mockLLM, nil)
		_, err := gen.Execute(context.Background(), Request{APIPath: specPath})
		if err == nil {
			t.Fatal("expected prune error on invalid openapi YAML")
		}
	})

	t.Run("missing prd file", func(t *testing.T) {
		tmpDir := t.TempDir()
		specPath := filepath.Join(tmpDir, "valid.yaml")
		_ = os.WriteFile(specPath, []byte(sampleOpenAPISpec), 0600)

		mockLLM := &mockLLMService{}
		gen := NewGenerator(mockLLM, nil)
		_, err := gen.Execute(context.Background(), Request{
			APIPath: specPath,
			ReqPath: "/nonexistent/path/to/prd.md",
		})
		if err == nil {
			t.Fatal("expected error reading nonexistent PRD file")
		}
	})
}

func TestEnrichStepWithHybridData(t *testing.T) {
	t.Run("skips token producer", func(t *testing.T) {
		step := &domain.Step{
			Request: domain.RequestDef{Method: "POST", Endpoint: "/login"},
		}
		op := openapi.NormalizedOperation{
			Method:          "POST",
			Endpoint:        "/login",
			IsTokenProducer: true,
			RequestSchema: &openapi.SchemaObject{
				Properties: map[string]openapi.SchemaObject{
					"user": {Type: "string"},
				},
			},
		}
		EnrichStepWithHybridData(step, op)
		if len(step.TestCases) != 0 {
			t.Errorf("expected 0 test cases for token producer, got %d", len(step.TestCases))
		}
	})

	t.Run("skips non-mutating methods", func(t *testing.T) {
		step := &domain.Step{
			Request: domain.RequestDef{Method: "GET", Endpoint: "/items"},
		}
		op := openapi.NormalizedOperation{
			Method:   "GET",
			Endpoint: "/items",
			RequestSchema: &openapi.SchemaObject{
				Properties: map[string]openapi.SchemaObject{
					"query": {Type: "string"},
				},
			},
		}
		EnrichStepWithHybridData(step, op)
		if len(step.TestCases) != 0 {
			t.Errorf("expected 0 test cases for GET method, got %d", len(step.TestCases))
		}
	})

	t.Run("skips nil or empty properties", func(t *testing.T) {
		step := &domain.Step{
			Request: domain.RequestDef{Method: "POST", Endpoint: "/empty"},
		}
		op := openapi.NormalizedOperation{
			Method:        "POST",
			Endpoint:      "/empty",
			RequestSchema: &openapi.SchemaObject{},
		}
		EnrichStepWithHybridData(step, op)
		if len(step.TestCases) != 0 {
			t.Errorf("expected 0 test cases for empty schema, got %d", len(step.TestCases))
		}
	})

	t.Run("enriches integer and array fields with zero boundary", func(t *testing.T) {
		step := &domain.Step{
			Request: domain.RequestDef{
				Method:   "POST",
				Endpoint: "/products",
				Body: map[string]interface{}{
					"count": 10,
					"items": []interface{}{"a", "b"},
				},
			},
		}
		op := openapi.NormalizedOperation{
			Method:   "POST",
			Endpoint: "/products",
			RequestSchema: &openapi.SchemaObject{
				Properties: map[string]openapi.SchemaObject{
					"count": {Type: "integer"},
					"items": {Type: "array"},
				},
			},
		}
		EnrichStepWithHybridData(step, op)
		if len(step.TestCases) != 1 {
			t.Fatalf("expected 1 test case (fuzzing), got %d", len(step.TestCases))
		}
		tc := step.TestCases[0]
		if tc.Type != "go_fuzzing" {
			t.Errorf("expected type go_fuzzing, got %s", tc.Type)
		}
		payload, ok := tc.Payload.(map[string]interface{})
		if !ok {
			t.Fatalf("expected payload to be map, got %T", tc.Payload)
		}
		if payload["count"] != 0 {
			t.Errorf("expected count to be 0, got %v", payload["count"])
		}
		arr, ok := payload["items"].([]interface{})
		if !ok || len(arr) != 0 {
			t.Errorf("expected empty array for items, got %v", payload["items"])
		}
	})

	t.Run("enriches string field with SQL injection", func(t *testing.T) {
		step := &domain.Step{
			Request: domain.RequestDef{
				Method:   "PUT",
				Endpoint: "/users/profile",
			},
			TestCases: []domain.TestCase{
				{
					ID:      "TC_EXISTING",
					Type:    "positive",
					Payload: map[string]interface{}{"bio": "Hello world"},
				},
			},
		}
		op := openapi.NormalizedOperation{
			Method:   "PUT",
			Endpoint: "/users/profile",
			RequestSchema: &openapi.SchemaObject{
				Properties: map[string]openapi.SchemaObject{
					"bio": {Type: "string"},
				},
			},
		}
		EnrichStepWithHybridData(step, op)
		if len(step.TestCases) != 2 {
			t.Fatalf("expected 2 test cases (1 existing + 1 sec), got %d", len(step.TestCases))
		}
		tc := step.TestCases[1]
		if tc.Type != "go_security" {
			t.Errorf("expected type go_security, got %s", tc.Type)
		}
		payload := tc.Payload.(map[string]interface{})
		if payload["bio"] != "' OR '1'='1" {
			t.Errorf("expected injection string, got %v", payload["bio"])
		}
	})

	t.Run("idempotent when test cases already exist", func(t *testing.T) {
		step := &domain.Step{
			Request: domain.RequestDef{Method: "POST", Endpoint: "/items"},
			TestCases: []domain.TestCase{
				{ID: "TC_GO_ZERO_BOUNDARY", Type: "go_fuzzing"},
				{ID: "TC_GO_SQL_INJECTION", Type: "go_security"},
			},
		}
		op := openapi.NormalizedOperation{
			Method:   "POST",
			Endpoint: "/items",
			RequestSchema: &openapi.SchemaObject{
				Properties: map[string]openapi.SchemaObject{
					"qty":  {Type: "integer"},
					"name": {Type: "string"},
				},
			},
		}
		EnrichStepWithHybridData(step, op)
		if len(step.TestCases) != 2 {
			t.Errorf("expected no duplicate additions, got count %d", len(step.TestCases))
		}
	})
}

func TestFindMatchingOp(t *testing.T) {
	ops := []openapi.NormalizedOperation{
		{Method: "POST", Endpoint: "/v1/orders"},
		{Method: "GET", Endpoint: "/v1/orders/{id}"},
	}

	t.Run("match by request", func(t *testing.T) {
		step := domain.Step{
			Request: domain.RequestDef{Method: "post", Endpoint: "/v1/orders"},
		}
		matched, found := findMatchingOp(step, ops)
		if !found || matched.Endpoint != "/v1/orders" {
			t.Errorf("expected match by request, got found=%v, op=%+v", found, matched)
		}
	})

	t.Run("match by target ref", func(t *testing.T) {
		step := domain.Step{
			TargetRef: &domain.TargetRef{Method: "GET", Endpoint: "/v1/orders/{id}"},
		}
		matched, found := findMatchingOp(step, ops)
		if !found || matched.Endpoint != "/v1/orders/{id}" {
			t.Errorf("expected match by target ref, got found=%v, op=%+v", found, matched)
		}
	})

	t.Run("not found", func(t *testing.T) {
		step := domain.Step{
			Request: domain.RequestDef{Method: "DELETE", Endpoint: "/v1/orders/123"},
		}
		_, found := findMatchingOp(step, ops)
		if found {
			t.Error("expected not found, but found match")
		}
	})
}
