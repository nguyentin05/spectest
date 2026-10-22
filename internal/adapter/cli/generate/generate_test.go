package generate

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nguyentin05/spectest/internal/adapter/cli/cmdutil"
)

const testOpenAPISpec = `openapi: 3.0.3
info:
  title: Users Service API
  version: 1.0.0
servers:
  - url: https://api.users.test/v1
paths:
  /auth/login:
    post:
      summary: User login
      tags: [Auth]
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                user: { type: string }
                pass: { type: string }
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  token: { type: string }
  /users:
    get:
      summary: List users
      tags: [Users]
      security:
        - BearerAuth: []
      responses:
        '200':
          description: OK
components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
`

const testPRD = `# Requirements
## Section: Users
Users list must require valid authentication.
`

func cleanEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SPECTEST_PROVIDER", "")
	t.Setenv("SPECTEST_ENDPOINT", "")
	t.Setenv("SPECTEST_MODEL", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
}

func TestGenerateCmd_Help(t *testing.T) {
	f := cmdutil.NewFactory()
	cmd := NewCommand(f)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("help failed: %v", err)
	}

	got := out.String()
	for _, flag := range []string{"--api", "--req", "--output", "--export", "--enrich"} {
		if !strings.Contains(got, flag) {
			t.Errorf("expected help output to document flag %q, got:\n%s", flag, got)
		}
	}
}

func TestGenerateCmd_MissingAPIFlag(t *testing.T) {
	cleanEnv(t)
	f := cmdutil.NewFactory()
	cmd := NewCommand(f)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--api (-a) is missing") {
		t.Fatalf("expected missing --api flag error, got: %v", err)
	}
}

func TestGenerateCmd_UnconfiguredLLM(t *testing.T) {
	cleanEnv(t)
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "spec.yaml")
	_ = os.WriteFile(specPath, []byte(testOpenAPISpec), 0600)

	f := cmdutil.NewFactory()
	cmd := NewCommand(f)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"-a", specPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when LLM service is unconfigured, got nil")
	}
}

func TestGenerateCmd_EndToEnd_Success(t *testing.T) {
	cleanEnv(t)
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(testOpenAPISpec), 0600); err != nil {
		t.Fatalf("write spec failed: %v", err)
	}
	prdPath := filepath.Join(tmpDir, "prd.md")
	if err := os.WriteFile(prdPath, []byte(testPRD), 0600); err != nil {
		t.Fatalf("write prd failed: %v", err)
	}
	outDir := filepath.Join(tmpDir, "test_out")

	// Start a mock OpenAI-compatible HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `{
							"id": "SCN_USERS",
							"name": "Users Workflow",
							"steps": [
								{
									"id": "STEP_LIST",
									"name": "List Users",
									"request": {
										"method": "GET",
										"endpoint": "/users"
									},
									"test_cases": [
										{
											"id": "TC_OK",
											"name": "Get 200 OK",
											"type": "positive",
											"assert": {
												"status": 200
											}
										}
									]
								}
							]
						}`,
					},
				},
			},
		})
	}))
	defer mockServer.Close()

	// Configure environment to point to mock LLM server
	t.Setenv("SPECTEST_PROVIDER", "openai")
	t.Setenv("SPECTEST_ENDPOINT", mockServer.URL)
	t.Setenv("SPECTEST_MODEL", "gpt-4o")
	t.Setenv("OPENAI_API_KEY", "sk-mock-test-key")

	f := cmdutil.NewFactory()
	cmd := NewCommand(f)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"-a", specPath,
		"-r", prdPath,
		"-o", outDir,
		"-e", "postman",
		"--enrich",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("generate execution failed: %v", err)
	}

	got := out.String()
	expectedSummaries := []string{
		"LLM Provider        : openai",
		"Spec Pruning        :",
		"Auth Detection      : Detected producer [POST /auth/login] -> wired Bearer token",
		"Domain Chunks       :",
		"Test Suite File     :",
		"Coverage Matrix     :",
		"Exported Collection :",
	}

	for _, s := range expectedSummaries {
		if !strings.Contains(got, s) {
			t.Errorf("expected generate output to contain %q, got:\n%s", s, got)
		}
	}

	// Verify generated artifacts on disk
	suiteFile := filepath.Join(outDir, "spectest.yaml")
	matrixFile := filepath.Join(outDir, "test_matrix.md")
	postmanFile := filepath.Join(outDir, "postman_collection.json")

	for _, file := range []string{suiteFile, matrixFile, postmanFile} {
		if _, err := os.Stat(file); err != nil {
			t.Errorf("expected file %s to be created: %v", file, err)
		}
	}
}
