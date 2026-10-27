package generate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/sources"
	"github.com/nguyentin05/spectest/internal/infrastructure/suitefile"
)

const sampleAuthOpenAPI = `openapi: 3.0.3
info:
  title: Auth API
  version: "1"
paths:
  /login:
    post:
      summary: User login
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required:
                - email
                - password
              properties:
                email:
                  type: string
                password:
                  type: string
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  access_token:
                    type: string
  /me:
    get:
      summary: Get user profile
      responses:
        "200":
          description: profile
          content:
            application/json:
              schema:
                type: object
                properties:
                  email:
                    type: string
`

type pipelineTestLLM struct{}

func (pipelineTestLLM) Name() string { return "pipeline-test-llm" }

func (pipelineTestLLM) GenerateStructured(
	_ context.Context, systemPrompt, userPrompt string, _ any,
) (string, error) {
	if strings.Contains(systemPrompt, "reconciling OpenAPI") {
		return `{
			"global_notes": ["User must supply valid password of at least 8 chars"],
			"overrides": [{"api": "POST {auth}/login", "item": "status", "openapi": "200", "spec": "201"}],
			"ambiguities": ["Token expiration duration"],
			"missing_info": [],
			"requirements_language": "English"
		}`, nil
	}
	if strings.Contains(systemPrompt, "reusable setup") {
		return `{
			"seed": {
				"auth": {
					"vars": {
						"test_email": "admin@example.com",
						"test_pass": "Secret123!"
					}
				}
			},
			"setup": {
				"login_admin": {
					"steps": [
						{
							"call": "POST {auth}/login",
							"headers": {"Content-Type": "application/json"},
							"body": {
								"email": "{{test_email}}",
								"password": "{{test_pass}}"
							},
							"expect": 200,
							"save": {
								"token": "body.access_token"
							}
						}
					]
				}
			},
			"unresolved": [],
			"assumptions": [
				{"area": "auth", "statement": "Admin user is pre-seeded", "reason": "Database migration"}
			]
		}`, nil
	}
	if strings.Contains(systemPrompt, "designing test scenarios") {
		if strings.Contains(userPrompt, `"target_api": "POST {auth}/login"`) {
			return `{
				"api": "POST {auth}/login",
				"scenarios": [
					{
						"id": "login-success",
						"name": "Successful login with valid credentials",
						"steps": [
							{
								"call": "POST {auth}/login",
								"headers": {"Content-Type": "application/json"},
								"body": {
									"email": "{{test_email}}",
									"password": "{{test_pass}}"
								},
								"expect": 200,
								"save": {
									"token": "body.access_token"
								}
							}
						]
					},
					{
						"id": "login-unauthorized",
						"name": "Login rejection with wrong password",
						"steps": [
							{
								"call": "POST {auth}/login",
								"headers": {"Content-Type": "application/json"},
								"body": {
									"email": "{{test_email}}",
									"password": "WrongPassword!"
								},
								"expect": 401
							}
						]
					}
				],
				"inferred_expectations": [
					{
						"scenario_id": "login-unauthorized",
						"expect": 401,
						"basis": "standard auth rejection",
						"reason": "invalid credentials reject with 401"
					}
				],
				"skipped_cases": []
			}`, nil
		}
		if strings.Contains(userPrompt, `"target_api": "GET {auth}/me"`) {
			return `{
				"api": "GET {auth}/me",
				"scenarios": [
					{
						"id": "me-success",
						"name": "Get user profile with valid token",
						"setup": ["login_admin"],
						"steps": [
							{
								"call": "GET {auth}/me",
								"headers": {
									"Authorization": "Bearer {{token}}"
								},
								"expect": 200
							}
						]
					}
				],
				"inferred_expectations": [],
				"skipped_cases": []
			}`, nil
		}
	}
	if strings.Contains(systemPrompt, "writing the narrative analysis") {
		return `{
			"analysis": "This suite comprehensively tests the Authentication service.",
			"assumptions": [
				{"area": "auth", "statement": "Bearer auth header scheme is expected", "reason": "Standard OpenAPI Bearer pattern"}
			]
		}`, nil
	}
	return "", fmt.Errorf("unexpected system prompt: %s", systemPrompt)
}

func TestInitPipeline_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	apiPath := filepath.Join(dir, "openapi.yaml")
	if err := os.WriteFile(apiPath, []byte(sampleAuthOpenAPI), 0o644); err != nil {
		t.Fatalf("write openapi: %v", err)
	}

	reqPath := filepath.Join(dir, "requirements.md")
	reqContent := "# Requirements\nPassword must be at least 8 characters.\n"
	if err := os.WriteFile(reqPath, []byte(reqContent), 0o644); err != nil {
		t.Fatalf("write req: %v", err)
	}

	outDir := filepath.Join(dir, "out")
	store := suitefile.NewStore()
	g := NewGenerator(pipelineTestLLM{}, sources.NewLoader(), store, smokeClock{})

	res, err := g.Execute(context.Background(), Request{
		APIs:             []domain.APIInput{{Server: "auth", Path: apiPath}},
		RequirementsPath: reqPath,
		OutputDir:        outDir,
		Concurrency:      1,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if res.Mode != ModeInitial {
		t.Errorf("Mode = %q, want %q", res.Mode, ModeInitial)
	}
	if res.Input != domain.InputBoth {
		t.Errorf("Input = %q, want %q", res.Input, domain.InputBoth)
	}
	if res.APITotal != 2 {
		t.Errorf("APITotal = %d, want 2", res.APITotal)
	}

	// Verify only 2 files are written: spectest.yaml and spectest.log.yaml
	suitePath := filepath.Join(outDir, suitefile.SuiteFileName)
	logPath := filepath.Join(outDir, suitefile.LogFileName)
	trackPath := filepath.Join(outDir, suitefile.TrackFileName)

	if _, err := os.Stat(suitePath); err != nil {
		t.Errorf("expected spectest.yaml to exist: %v", err)
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("expected spectest.log.yaml to exist: %v", err)
	}
	if _, err := os.Stat(trackPath); !os.IsNotExist(err) {
		t.Errorf("expected spectest.track.yaml to NOT exist, got err: %v", err)
	}

	// Load generated files and verify content
	fileSet, err := store.Load(context.Background(), outDir)
	if err != nil {
		t.Fatalf("failed to load generated files: %v", err)
	}

	s := fileSet.Suite
	if len(s.APIs) != 2 {
		t.Fatalf("expected 2 APIs in suite, got %d", len(s.APIs))
	}
	if len(s.Setup) != 1 {
		t.Errorf("expected 1 setup block, got %d", len(s.Setup))
	}
	if _, ok := s.Setup["login_admin"]; !ok {
		t.Errorf("expected setup login_admin to exist")
	}
	for _, sc := range s.Scenarios {
		t.Logf("Preserved scenario: id=%s", sc.ID)
	}
	if len(s.Scenarios) != 3 {
		t.Fatalf("expected 3 scenarios, got %d", len(s.Scenarios))
	}

	// Verify Analysis contains managed warnings and assumptions blocks
	if !strings.Contains(s.Analysis, "<!-- spectest:warnings:start -->") {
		t.Errorf("expected warnings start marker in Analysis")
	}
	if !strings.Contains(s.Analysis, "contract override: API POST {auth}/login") {
		t.Errorf("expected contract override warning in Analysis")
	}
	if !strings.Contains(s.Analysis, "<!-- spectest:assumptions:start -->") {
		t.Errorf("expected assumptions start marker in Analysis")
	}
	// Verify inferred expectation is captured in assumptions
	if !strings.Contains(s.Analysis, "login-unauthorized") {
		t.Errorf("expected inferred expectation for login-unauthorized to be in assumptions block")
	}
}
