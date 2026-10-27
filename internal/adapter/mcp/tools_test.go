package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/nguyentin05/spectest/internal/domain"
)

func setupTestMCP(t *testing.T) (*Server, string) {
	t.Helper()
	tmpDir := t.TempDir()
	canonicalRoot, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		canonicalRoot = tmpDir
	}

	srv, err := NewServer(canonicalRoot)
	if err != nil {
		t.Fatalf("failed to create MCP server: %v", err)
	}
	return srv, canonicalRoot
}

func TestMCP_StatusAndInspect(t *testing.T) {
	srv, root := setupTestMCP(t)

	// 1. Initial status with no files
	res, err := srv.handleStatus(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleStatus error: %v", err)
	}
	content := res.Content[0].(mcp.TextContent).Text
	var status StatusResult
	if err := json.Unmarshal([]byte(content), &status); err != nil {
		t.Fatalf("unmarshal status result: %v", err)
	}
	if status.SuiteExists {
		t.Errorf("expected SuiteExists = false, got true")
	}

	// 2. Create sample suite and inspect
	testsDir := filepath.Join(root, "tests")
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		t.Fatalf("mkdir tests: %v", err)
	}
	suiteContent := `version: 1
input: openapi
servers:
  - default
apis:
  - GET {default}/health
scenarios:
  - id: health-check
    name: Check health
    steps:
      - call: GET {default}/health
        expect: 200
`
	suiteFile := filepath.Join(testsDir, "spectest.yaml")
	if err := os.WriteFile(suiteFile, []byte(suiteContent), 0o600); err != nil {
		t.Fatalf("write suite: %v", err)
	}

	// Re-check status
	res, err = srv.handleStatus(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleStatus error: %v", err)
	}
	content = res.Content[0].(mcp.TextContent).Text
	if err := json.Unmarshal([]byte(content), &status); err != nil {
		t.Fatalf("unmarshal status result: %v", err)
	}
	if !status.SuiteExists {
		t.Errorf("expected SuiteExists = true")
	}

	// 3. Inspect suite
	res, err = srv.handleInspectSuite(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleInspectSuite error: %v", err)
	}
	content = res.Content[0].(mcp.TextContent).Text
	var inspect InspectResult
	if err := json.Unmarshal([]byte(content), &inspect); err != nil {
		t.Fatalf("unmarshal inspect result: %v", err)
	}
	if inspect.ScenarioCount != 1 || inspect.Scenarios[0].ID != "health-check" {
		t.Errorf("unexpected inspect result: %+v", inspect)
	}
	if inspect.SuiteHash == "" {
		t.Errorf("expected non-empty SuiteHash")
	}

	// 4. Validate suite
	res, err = srv.handleValidateSuite(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleValidateSuite error: %v", err)
	}
	content = res.Content[0].(mcp.TextContent).Text
	var valRes ValidateResult
	if err := json.Unmarshal([]byte(content), &valRes); err != nil {
		t.Fatalf("unmarshal validate result: %v", err)
	}
	if !valRes.Valid {
		t.Errorf("expected valid suite, got errors: %v", valRes.Errors)
	}
}

func TestMCP_RunAndReport_Redacted(t *testing.T) {
	srv, root := setupTestMCP(t)

	// Mock target HTTP server returning a token that MUST be redacted
	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok", "token": "secret_jwt_token_12345", "password": "supersecret"}`))
	}))
	defer mockSrv.Close()

	testsDir := filepath.Join(root, "tests")
	_ = os.MkdirAll(testsDir, 0o755)
	suiteContent := `version: 1
input: openapi
servers:
  - default
apis:
  - GET {default}/login
scenarios:
  - id: login-check
    name: Check login
    steps:
      - call: GET {default}/login
        expect: 200
        save:
          auth_token: body.token
`
	suiteFile := filepath.Join(testsDir, "spectest.yaml")
	_ = os.WriteFile(suiteFile, []byte(suiteContent), 0o600)

	// Call start_run with wait_timeout_seconds=5 for synchronous completion in test
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"suite_path":           "tests/spectest.yaml",
		"mode":                 "full",
		"targets":              map[string]any{"default": mockSrv.URL},
		"report_dir":           "reports",
		"wait_timeout_seconds": float64(5),
	}

	res, err := srv.handleStartRun(context.Background(), req)
	if err != nil {
		t.Fatalf("handleStartRun error: %v", err)
	}
	content := res.Content[0].(mcp.TextContent).Text
	var jobSnapshot JobSnapshot
	if err := json.Unmarshal([]byte(content), &jobSnapshot); err != nil {
		t.Fatalf("unmarshal job result: %v, raw: %s", err, content)
	}

	if jobSnapshot.Status != JobSucceeded {
		t.Fatalf("job did not succeed: status=%s, err=%s", jobSnapshot.Status, jobSnapshot.Err)
	}

	// Test read_report tool
	reportReq := mcp.CallToolRequest{}
	reportReq.Params.Arguments = map[string]any{
		"report_path": "reports/report.json",
	}
	repRes, err := srv.handleReadReport(context.Background(), reportReq)
	if err != nil {
		t.Fatalf("handleReadReport error: %v", err)
	}
	repContent := repRes.Content[0].(mcp.TextContent).Text

	// Verify token is REDACTED and not leaked in report
	if strings.Contains(repContent, "secret_jwt_token_12345") {
		t.Fatalf("CRITICAL SECURITY LEAK: unredacted token found in read_report result: %s", repContent)
	}
	if strings.Contains(repContent, "supersecret") {
		t.Fatalf("CRITICAL SECURITY LEAK: unredacted password found in read_report result: %s", repContent)
	}
}

func TestMCP_CancelJob(t *testing.T) {
	srv, _ := setupTestMCP(t)

	// Create long running job
	_, job := srv.jobs.CreateJob(context.Background(), "run")
	job.Finish(JobRunning, nil, "")

	cancelReq := mcp.CallToolRequest{}
	cancelReq.Params.Arguments = map[string]any{
		"job_id": job.ID,
	}

	res, err := srv.handleCancelJob(context.Background(), cancelReq)
	if err != nil {
		t.Fatalf("handleCancelJob error: %v", err)
	}
	content := res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(content, `"cancelled":true`) {
		t.Errorf("expected cancelled true, got: %s", content)
	}

	snap, _ := srv.jobs.GetJob(job.ID)
	if snap.Status != JobCancelled {
		t.Errorf("job status = %s, want cancelled", snap.Status)
	}
}

func TestMCP_ConcurrentDirectoryLock(t *testing.T) {
	srv, root := setupTestMCP(t)

	targetDir := filepath.Join(root, "tests")
	unlock1 := srv.jobs.LockKey(targetDir)

	locked := make(chan bool, 1)
	go func() {
		unlock2 := srv.jobs.LockKey(targetDir)
		locked <- true
		unlock2()
	}()

	select {
	case <-locked:
		t.Fatalf("second lock acquired while first lock was still held!")
	case <-time.After(100 * time.Millisecond):
		// Expected: second goroutine is blocked
	}

	unlock1()

	select {
	case <-locked:
		// Successfully acquired after unlock
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for second lock to acquire after unlock")
	}
}

func TestMCP_PlanUpdate_And_Resources(t *testing.T) {
	srv, root := setupTestMCP(t)

	testsDir := filepath.Join(root, "tests")
	_ = os.MkdirAll(testsDir, 0o755)

	suiteContent := `version: 1
input: openapi
servers:
  - default
apis:
  - GET {default}/health
scenarios:
  - id: health-check
    name: Check health
    steps:
      - call: GET {default}/health
        expect: 200
`
	suiteFile := filepath.Join(testsDir, "spectest.yaml")
	_ = os.WriteFile(suiteFile, []byte(suiteContent), 0o600)

	// Base log entry matching suite
	logContent := `base:
  at: "2026-10-26T00:00:00Z"
  by: test
  after_hash: "` + domain.SuiteHash(&domain.Suite{
		Version: 1,
		Input:   "openapi",
		Servers: []string{"default"},
		APIs:    []string{"GET {default}/health"},
		Scenarios: []domain.Scenario{
			{
				ID:   "health-check",
				Name: "Check health",
				Steps: []domain.Step{
					{Call: "GET {default}/health", Expect: 200},
				},
			},
		},
	}) + `"
  api_catalog:
    "GET {default}/health":
      hash: "somehash"
      status: "present"
`
	logFile := filepath.Join(testsDir, "spectest.log.yaml")
	_ = os.WriteFile(logFile, []byte(logContent), 0o600)

	// New OpenAPI with added endpoint
	openAPIContent := `{
  "openapi": "3.0.0",
  "info": {"title": "Test API", "version": "1.0"},
  "paths": {
    "/health": {
      "get": {
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/logout": {
      "post": {
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`
	openAPIFile := filepath.Join(root, "openapi.json")
	_ = os.WriteFile(openAPIFile, []byte(openAPIContent), 0o600)

	// 1. Call plan_update tool
	planReq := mcp.CallToolRequest{}
	planReq.Params.Arguments = map[string]any{
		"output_dir": "tests",
		"apis": []any{
			map[string]any{"server": "default", "path": "openapi.json"},
		},
	}

	planRes, err := srv.handlePlanUpdate(context.Background(), planReq)
	if err != nil {
		t.Fatalf("handlePlanUpdate error: %v", err)
	}
	content := planRes.Content[0].(mcp.TextContent).Text
	if !strings.Contains(content, "POST {default}/logout") {
		t.Errorf("expected plan result to contain POST {default}/logout, got: %s", content)
	}

	// Verify file was NOT modified by plan
	curSuiteData, _ := os.ReadFile(suiteFile)
	if string(curSuiteData) != suiteContent {
		t.Errorf("plan_update modified suite on disk!")
	}

	// 2. Test resources: spectest://suite and spectest://history/latest
	resSuite, err := srv.handleResourceSuite(context.Background(), mcp.ReadResourceRequest{})
	if err != nil {
		t.Fatalf("read spectest://suite error: %v", err)
	}
	if len(resSuite) == 0 || !strings.Contains(resSuite[0].(mcp.TextResourceContents).Text, "health-check") {
		t.Errorf("invalid suite resource content: %+v", resSuite)
	}

	resLog, err := srv.handleResourceLatestHistory(context.Background(), mcp.ReadResourceRequest{})
	if err != nil {
		t.Fatalf("read spectest://history/latest error: %v", err)
	}
	if len(resLog) == 0 || !strings.Contains(resLog[0].(mcp.TextResourceContents).Text, "after_hash") {
		t.Errorf("invalid log resource content: %+v", resLog)
	}
}
