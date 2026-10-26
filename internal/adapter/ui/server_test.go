package ui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nguyentin05/spectest/internal/adapter/cli/cmdutil"
	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/suitefile"
)

func newTestServer(t *testing.T) (*Server, *cmdutil.Factory, string) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	f := cmdutil.NewFactory()
	srv := NewServer("127.0.0.1:0", tmpDir, f)
	return srv, f, tmpDir
}

func sampleSuite() *domain.Suite {
	return &domain.Suite{
		Version:  1,
		Input:    domain.InputOpenAPI,
		Servers:  []string{"auth"},
		APIs:     []string{"GET {auth}/api/users"},
		Analysis: "User management flow",
		Seed: map[string]domain.Seed{
			"env": {
				Vars: map[string]any{
					"role": "admin",
				},
			},
		},
		Setup: map[string]domain.Setup{
			"init": {
				Vars: map[string]any{
					"token": "test-token-123",
				},
				Steps: []domain.Step{
					{
						Call:   "GET {auth}/api/users",
						Expect: 200,
					},
				},
			},
		},
		Scenarios: []domain.Scenario{
			{
				ID:    "users-flow",
				Name:  "Users flow",
				Setup: []string{"init"},
				Steps: []domain.Step{
					{
						Call:   "GET {auth}/api/users",
						Expect: 200,
						Save: map[string]string{
							"first_id": "body.0.id",
						},
					},
				},
			},
		},
	}
}

func sampleLogFile() *domain.LogFile {
	return &domain.LogFile{
		Base: domain.BaseEntry{
			At:        "2026-10-20T09:00:00+07:00",
			By:        "ai",
			AfterHash: "5c1d02a9b7e4",
			Spectest:  sampleSuite(),
		},
		Update: []domain.UpdateEntry{
			{
				N:         1,
				At:        "2026-10-24T10:15:00+07:00",
				By:        "ai",
				Note:      "API added: POST {auth}/api/users",
				AfterHash: "8e41ab77c9d0",
			},
		},
	}
}

func TestServer_HealthAndSPA(t *testing.T) {
	srv, _, _ := newTestServer(t)
	handler := srv.Routes()

	// 1. /api/health
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", rec.Code, http.StatusOK)
	}

	var health map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &health)
	if health["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", health["status"])
	}

	// 2. SPA index.html
	spaReq := httptest.NewRequest(http.MethodGet, "/", nil)
	spaRec := httptest.NewRecorder()
	handler.ServeHTTP(spaRec, spaReq)

	if spaRec.Code != http.StatusOK {
		t.Fatalf("SPA status = %d, want %d", spaRec.Code, http.StatusOK)
	}
	body := spaRec.Body.String()
	if !bytes.Contains([]byte(body), []byte("SpecTest")) {
		t.Errorf("expected SPA body to contain 'SpecTest'")
	}
}

func TestServer_Config(t *testing.T) {
	srv, f, _ := newTestServer(t)
	handler := srv.Routes()

	// Initial Get
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("get config status = %d, want 200", rec.Code)
	}

	// Update config
	updatePayload := map[string]string{
		"provider": "openai",
		"model":    "gpt-4o",
		"endpoint": "https://custom.openai.api",
		"api_key":  "sk-test-secret-12345678",
	}
	raw, _ := json.Marshal(updatePayload)
	updateReq := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewReader(raw))
	updateRec := httptest.NewRecorder()
	handler.ServeHTTP(updateRec, updateReq)

	if updateRec.Code != http.StatusOK {
		t.Fatalf("update config status = %d, want 200: %s", updateRec.Code, updateRec.Body.String())
	}

	// Verify store was updated
	store := f.CredentialStore()
	if got := store.GetDefaultProvider(); got != "openai" {
		t.Errorf("GetDefaultProvider() = %q, want openai", got)
	}
	if got := store.GetDefaultModel("openai"); got != "gpt-4o" {
		t.Errorf("GetDefaultModel() = %q, want gpt-4o", got)
	}
	key, _ := store.GetAPIKey("openai")
	if key != "sk-test-secret-12345678" {
		t.Errorf("GetAPIKey() = %q, want sk-test-secret-12345678", key)
	}
}

func TestServer_TargetHealthCheck(t *testing.T) {
	// Start mock target server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"up"}`))
	}))
	defer ts.Close()

	srv, _, _ := newTestServer(t)
	handler := srv.Routes()

	// 1. Successful check
	payload, _ := json.Marshal(map[string]any{
		"target_url":      ts.URL,
		"timeout_seconds": 2,
		"mode":            "existing",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/server/healthcheck", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var res healthcheckResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if !res.Ready {
		t.Errorf("expected ready = true, got false (%s)", res.ErrorDetail)
	}

	// 2. Unreachable check
	badPayload, _ := json.Marshal(map[string]any{
		"target_url":      "http://127.0.0.1:54321",
		"timeout_seconds": 1,
		"mode":            "existing",
	})
	badReq := httptest.NewRequest(http.MethodPost, "/api/server/healthcheck", bytes.NewReader(badPayload))
	badRec := httptest.NewRecorder()
	handler.ServeHTTP(badRec, badReq)

	var badRes healthcheckResponse
	_ = json.Unmarshal(badRec.Body.Bytes(), &badRes)
	if badRes.Ready {
		t.Error("expected ready = false for bad port, got true")
	}
}

func TestServer_SpecsAndPlan(t *testing.T) {
	srv, _, tmpDir := newTestServer(t)
	handler := srv.Routes()

	// Save sample suite in tmpDir
	suitePath := filepath.Join(tmpDir, "spectest.yaml")
	data, err := suitefile.EncodeSuite(sampleSuite())
	if err != nil {
		t.Fatalf("failed to encode sample suite: %v", err)
	}
	if err := os.WriteFile(suitePath, data, 0o644); err != nil {
		t.Fatalf("failed to write sample suite: %v", err)
	}

	// Save sample log in tmpDir
	logPath := filepath.Join(tmpDir, "spectest.log.yaml")
	logData, err := suitefile.EncodeLog(sampleLogFile())
	if err != nil {
		t.Fatalf("failed to encode sample log: %v", err)
	}
	if err := os.WriteFile(logPath, logData, 0o644); err != nil {
		t.Fatalf("failed to write sample log: %v", err)
	}

	// 1. Get Plan
	req := httptest.NewRequest(http.MethodGet, "/api/specs/plan?path="+suitePath, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("get plan status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var planData map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &planData)
	if planData["version"] != float64(1) {
		t.Errorf("version = %v, want 1", planData["version"])
	}

	stats, ok := planData["stats"].(map[string]any)
	if !ok || stats["total_scenarios"] != float64(1) {
		t.Errorf("expected stats.total_scenarios = 1, got %v", stats)
	}

	// 2. Get History
	histReq := httptest.NewRequest(http.MethodGet, "/api/specs/history?path="+logPath, nil)
	histRec := httptest.NewRecorder()
	handler.ServeHTTP(histRec, histReq)

	if histRec.Code != http.StatusOK {
		t.Fatalf("get history status = %d, want 200: %s", histRec.Code, histRec.Body.String())
	}

	var histData map[string]any
	_ = json.Unmarshal(histRec.Body.Bytes(), &histData)
	if histData["base"] == nil {
		t.Error("expected non-nil base in history response")
	}

	// 3. Workspace list
	wsReq := httptest.NewRequest(http.MethodGet, "/api/specs/workspace", nil)
	wsRec := httptest.NewRecorder()
	handler.ServeHTTP(wsRec, wsReq)

	if wsRec.Code != http.StatusOK {
		t.Fatalf("workspace status = %d, want 200", wsRec.Code)
	}

	// 4. Upload raw spec
	uploadPayload, _ := json.Marshal(map[string]string{
		"file_name": "sample-prd.md",
		"content":   "# Requirements\nUsers must login.",
	})
	upReq := httptest.NewRequest(http.MethodPost, "/api/specs/upload", bytes.NewReader(uploadPayload))
	upRec := httptest.NewRecorder()
	handler.ServeHTTP(upRec, upReq)

	if upRec.Code != http.StatusOK {
		t.Fatalf("upload status = %d, want 200: %s", upRec.Code, upRec.Body.String())
	}
}

func TestServer_MarkdownRead(t *testing.T) {
	srv, _, _ := newTestServer(t)
	handler := srv.Routes()

	mdContent := `# Coverage Matrix
## 1. Overview
| Scenario | Steps |
| :--- | :--- |
| Login | 2 |
`
	payload, _ := json.Marshal(map[string]string{
		"content": mdContent,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/markdown/read", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp markdownReadResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Title != "Coverage Matrix" {
		t.Errorf("title = %q, want 'Coverage Matrix'", resp.Title)
	}
	if resp.TableCount != 1 {
		t.Errorf("table_count = %d, want 1", resp.TableCount)
	}
}

func TestServer_RunExecutionAndReport(t *testing.T) {
	// Start mock target server returning 200 for /api/users
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/users" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":1,"name":"Alice"}]`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	srv, _, tmpDir := newTestServer(t)
	handler := srv.Routes()

	suitePath := filepath.Join(tmpDir, "spectest.yaml")
	data, _ := suitefile.EncodeSuite(sampleSuite())
	_ = os.WriteFile(suitePath, data, 0o644)

	// 1. Start Run with multi-target
	payload, _ := json.Marshal(map[string]any{
		"targets": map[string]string{
			"auth": ts.URL,
		},
		"suite_path": suitePath,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("run status = %d, want 202: %s", rec.Code, rec.Body.String())
	}

	var startRes runStartResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &startRes)
	if startRes.RunID == "" {
		t.Fatal("expected non-empty RunID")
	}

	// Wait briefly for execution
	time.Sleep(250 * time.Millisecond)

	// 2. Fetch report
	repReq := httptest.NewRequest(http.MethodGet, "/api/run/report", nil)
	repRec := httptest.NewRecorder()
	handler.ServeHTTP(repRec, repReq)

	if repRec.Code != http.StatusOK {
		t.Fatalf("report status = %d, want 200: %s", repRec.Code, repRec.Body.String())
	}

	var report RunReport
	_ = json.Unmarshal(repRec.Body.Bytes(), &report)
	if report.TotalScenarios != 1 {
		t.Errorf("total scenarios = %d, want 1", report.TotalScenarios)
	}
	if report.PassedScenarios != 1 {
		t.Errorf("passed scenarios = %d, want 1", report.PassedScenarios)
	}
}

func TestServer_CancelActiveRun(t *testing.T) {
	srv, _, _ := newTestServer(t)
	handler := srv.Routes()

	// Cancel when nothing is running
	req := httptest.NewRequest(http.MethodPost, "/api/run/cancel", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var res map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res["success"] != false {
		t.Error("expected success=false when cancelling idle session")
	}
}

func TestServer_ConcurrentReportAccessRace(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	srv, _, tmpDir := newTestServer(t)
	handler := srv.Routes()

	suitePath := filepath.Join(tmpDir, "concurrent.yaml")
	suite := &domain.Suite{
		Version: 1,
		Input:   domain.InputOpenAPI,
		Scenarios: []domain.Scenario{
			{
				ID:   "multi-step-sc",
				Name: "Scenario with several steps",
				Steps: []domain.Step{
					{Call: "GET /step1", Expect: 200},
					{Call: "GET /step2", Expect: 200},
					{Call: "GET /step3", Expect: 200},
					{Call: "GET /step4", Expect: 200},
				},
			},
		},
	}
	data, _ := suitefile.EncodeSuite(suite)
	_ = os.WriteFile(suitePath, data, 0o644)

	// Start run
	payload, _ := json.Marshal(map[string]any{
		"target_url": ts.URL,
		"suite_path": suitePath,
	})
	startReq := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(payload))
	startRec := httptest.NewRecorder()
	handler.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusAccepted {
		t.Fatalf("run status = %d, want 202", startRec.Code)
	}

	// Concurrently query /api/run/report from multiple goroutines
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					req := httptest.NewRequest(http.MethodGet, "/api/run/report", nil)
					rec := httptest.NewRecorder()
					handler.ServeHTTP(rec, req)
					if rec.Code == http.StatusOK {
						var rep RunReport
						_ = json.Unmarshal(rec.Body.Bytes(), &rep)
					}
					time.Sleep(1 * time.Millisecond)
				}
			}
		}()
	}

	// Wait until run finishes by polling LastReport
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		rep := srv.sessionMgr.LastReport()
		if rep != nil && (rep.Status == "passed" || rep.Status == "failed") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	close(done)
}

func TestServer_InvalidTargetRejectedWith400(t *testing.T) {
	srv, _, tmpDir := newTestServer(t)
	handler := srv.Routes()

	suitePath := filepath.Join(tmpDir, "spectest.yaml")
	data, _ := suitefile.EncodeSuite(sampleSuite())
	_ = os.WriteFile(suitePath, data, 0o644)

	// Case 1: Valid default target, but one alias has an invalid protocol (ftp://bad)
	// Must return 400 and NOT silently fall back to default target
	payload1, _ := json.Marshal(map[string]any{
		"target_url": "http://localhost:8080",
		"targets": map[string]string{
			"auth": "ftp://bad-url",
		},
		"suite_path": suitePath,
	})
	req1 := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(payload1))
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid target alias URL, got: %d (%s)", rec1.Code, rec1.Body.String())
	}
	if !strings.Contains(rec1.Body.String(), "invalid target URL") {
		t.Errorf("expected error message to mention 'invalid target URL', got: %s", rec1.Body.String())
	}

	// Case 2: Empty alias name in targets map
	payload2, _ := json.Marshal(map[string]any{
		"targets": map[string]string{
			"": "http://localhost:8080",
		},
		"suite_path": suitePath,
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(payload2))
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for empty alias, got: %d", rec2.Code)
	}

	// Case 3: Invalid default target URL (non-http/https scheme)
	payload3, _ := json.Marshal(map[string]any{
		"target_url": "ftp://not-http",
		"suite_path": suitePath,
	})
	req3 := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(payload3))
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid default target URL, got: %d", rec3.Code)
	}
}
