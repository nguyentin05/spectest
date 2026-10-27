package run

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nguyentin05/spectest/internal/adapter/cli/cmdutil"
	"github.com/nguyentin05/spectest/internal/adapter/ui"
	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/suitefile"
)

func TestRunCmd_Help(t *testing.T) {
	cmd := NewCommand(cmdutil.NewFactory())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run --help failed: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"spectest run",
		"-f, --file",
		"-t, --target",
		"-c, --compose",
		"--timeout",
		"--report-dir",
		"--triage",
		"--only-changed",
		"--log",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected help output to contain %q; got:\n%s", want, got)
		}
	}

	// Verify --start-cmd was successfully removed
	if strings.Contains(got, "--start-cmd") {
		t.Error("unexpected --start-cmd flag in help output")
	}
}

func TestRunCmd_Execute_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	suitePath := filepath.Join(tmpDir, "spectest.yaml")

	suite := &domain.Suite{
		Version: 1,
		Input:   domain.InputOpenAPI,
		Servers: []string{"default"},
		APIs:    []string{"GET {default}/api/items"},
		Scenarios: []domain.Scenario{
			{
				ID:   "SCN_1",
				Name: "Cart Checkout",
				Steps: []domain.Step{
					{
						Call:   "GET {default}/api/items",
						Expect: 200,
					},
				},
			},
		},
	}
	data, err := suitefile.EncodeSuite(suite)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(suitePath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	f := cmdutil.NewFactory()
	cmd := NewCommand(f)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	reportDir := filepath.Join(tmpDir, "reports")
	cmd.SetArgs([]string{"-f", suitePath, "-t", srv.URL, "--timeout", "5", "--report-dir", reportDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "SpecTest: ready and executed suite") {
		t.Errorf("expected output to mention suite execution, got: %s", got)
	}
}

func TestRunCmd_MissingSuite(t *testing.T) {
	f := cmdutil.NewFactory()
	cmd := NewCommand(f)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"-f", "non-existent-suite.yaml", "-t", "http://localhost:8080"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for non-existent suite file, got nil")
	}
}

func TestRunCmd_MultiTargetFlags(t *testing.T) {
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer authSrv.Close()

	orderSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer orderSrv.Close()

	tmpDir := t.TempDir()
	suitePath := filepath.Join(tmpDir, "spectest.yaml")

	suite := &domain.Suite{
		Version: 1,
		Input:   domain.InputOpenAPI,
		Servers: []string{"auth", "order"},
		APIs:    []string{"GET {auth}/status", "GET {order}/status"},
		Scenarios: []domain.Scenario{
			{
				ID:   "check-servers",
				Name: "Check auth and order servers",
				Steps: []domain.Step{
					{Call: "GET {auth}/status", Expect: 200},
					{Call: "GET {order}/status", Expect: 200},
				},
			},
		},
	}
	data, err := suitefile.EncodeSuite(suite)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(suitePath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	f := cmdutil.NewFactory()
	cmd := NewCommand(f)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	reportDir := filepath.Join(tmpDir, "reports")
	cmd.SetArgs([]string{
		"-f", suitePath,
		"-t", "auth=" + authSrv.URL,
		"-t", "order=" + orderSrv.URL,
		"--report-dir", reportDir,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("multi-target execution failed: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Total Scenarios: 1 | Passed: 1") {
		t.Errorf("expected passing summary in output, got:\n%s", got)
	}
}

func TestRunCmd_ScenarioFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/fail" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "crash"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	suitePath := filepath.Join(tmpDir, "spectest.yaml")

	suite := &domain.Suite{
		Version: 1,
		Input:   domain.InputOpenAPI,
		Scenarios: []domain.Scenario{
			{
				ID:   "failing-sc",
				Name: "Failing scenario",
				Steps: []domain.Step{
					{Call: "GET /api/fail", Expect: 200},
				},
			},
		},
	}
	data, err := suitefile.EncodeSuite(suite)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(suitePath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	f := cmdutil.NewFactory()
	cmd := NewCommand(f)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	reportDir := filepath.Join(tmpDir, "reports")
	sarifPath := filepath.Join(tmpDir, "reports", "spectest.sarif")
	cmd.SetArgs([]string{"-f", suitePath, "-t", srv.URL, "--report-dir", reportDir, "--sarif", sarifPath})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected failure error for failing scenario, got nil")
	}

	got := out.String()
	if !strings.Contains(got, "FAIL: expected HTTP status 200, got 500") {
		t.Errorf("expected fail reason in output, got:\n%s", got)
	}

	sarifBytes, err := os.ReadFile(sarifPath)
	if err != nil {
		t.Fatalf("failed to read generated sarif file: %v", err)
	}
	if !strings.Contains(string(sarifBytes), "SPECTEST-001") {
		t.Errorf("expected SPECTEST-001 in sarif output: %s", string(sarifBytes))
	}
}

func TestRunCmd_ReportGenerationAndPermissions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tempDir := t.TempDir()
	nestedReportDir := filepath.Join(tempDir, "deep", "nested", "reports")
	suitePath := filepath.Join(tempDir, "spectest.yaml")

	suite := &domain.Suite{
		Version: 1,
		Input:   domain.InputOpenAPI,
		Scenarios: []domain.Scenario{
			{
				ID:    "sc1",
				Name:  "Test scenario",
				Steps: []domain.Step{{Call: "GET /api/test", Expect: 200}},
			},
		},
	}
	data, _ := suitefile.EncodeSuite(suite)
	_ = os.WriteFile(suitePath, data, 0o644)

	f := cmdutil.NewFactory()
	cmd := NewCommand(f)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"-f", suitePath, "-t", srv.URL, "--report-dir", nestedReportDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}

	reportFile := filepath.Join(nestedReportDir, "report.json")
	fi, err := os.Stat(reportFile)
	if err != nil {
		t.Fatalf("stat report.json: %v", err)
	}

	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Errorf("report file mode = %04o, want 0600", fi.Mode().Perm())
	}
}

func TestRunCmd_ReportFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tempDir := t.TempDir()
	suitePath := filepath.Join(tempDir, "spectest.yaml")

	suite := &domain.Suite{
		Version: 1,
		Input:   domain.InputOpenAPI,
		Scenarios: []domain.Scenario{
			{
				ID:    "sc1",
				Name:  "Test scenario",
				Steps: []domain.Step{{Call: "GET /api/test", Expect: 200}},
			},
		},
	}
	data, _ := suitefile.EncodeSuite(suite)
	_ = os.WriteFile(suitePath, data, 0o644)

	// Create a regular file where the report directory is expected to be
	filePathAsDir := filepath.Join(tempDir, "a-file")
	_ = os.WriteFile(filePathAsDir, []byte("blocking-file"), 0o644)
	invalidReportDir := filepath.Join(filePathAsDir, "cannot-create-dir")

	f := cmdutil.NewFactory()
	cmd := NewCommand(f)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"-f", suitePath, "-t", srv.URL, "--report-dir", invalidReportDir})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "failed to write test report") {
		t.Fatalf("expected report write error, got: %v", err)
	}
}

func TestRunCmd_Contract_UIParity(t *testing.T) {
	// A mock server handling login and data endpoint
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/login":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token": "token-parity-123"}`))
		case "/api/data":
			auth := r.Header.Get("Authorization")
			if auth != "Bearer token-parity-123" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "ok"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	tmpDir := t.TempDir()
	suitePath := filepath.Join(tmpDir, "parity.yaml")
	fixtureSuite := &domain.Suite{
		Version: 1,
		Input:   domain.InputOpenAPI,
		APIs:    []string{"POST /api/login", "GET /api/data"},
		Setup: map[string]domain.Setup{
			"auth": {
				Steps: []domain.Step{
					{
						Call:   "POST /api/login",
						Expect: 200,
						Save:   map[string]string{"jwt": "body.access_token"},
					},
				},
			},
		},
		Scenarios: []domain.Scenario{
			{
				ID:    "parity-test",
				Name:  "Test parity between CLI and UI",
				Setup: []string{"auth"},
				Steps: []domain.Step{
					{
						Call:    "GET /api/data",
						Headers: map[string]string{"Authorization": "Bearer {{jwt}}"},
						Expect:  200,
					},
				},
			},
		},
	}
	suiteData, err := suitefile.EncodeSuite(fixtureSuite)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(suitePath, suiteData, 0o644); err != nil {
		t.Fatal(err)
	}

	// 1. Run through CLI
	f := cmdutil.NewFactory()
	cliCmd := NewCommand(f)
	var cliOut bytes.Buffer
	cliCmd.SetOut(&cliOut)
	cliCmd.SetErr(&cliOut)
	cliReportDir := filepath.Join(tmpDir, "cli-reports")
	cliCmd.SetArgs([]string{"-f", suitePath, "-t", mockServer.URL, "--report-dir", cliReportDir})
	cliErr := cliCmd.Execute()
	if cliErr != nil {
		t.Fatalf("cli execution failed: %v", cliErr)
	}

	cliReportFile := filepath.Join(cliReportDir, "report.json")
	cliReportBytes, err := os.ReadFile(cliReportFile)
	if err != nil {
		t.Fatalf("read cli report: %v", err)
	}
	var cliSummary map[string]any
	if err := json.Unmarshal(cliReportBytes, &cliSummary); err != nil {
		t.Fatalf("unmarshal cli report: %v", err)
	}

	// 2. Run through UI
	uiServer := ui.NewServer("127.0.0.1:0", tmpDir, f)
	uiHandler := uiServer.Routes()

	runReqBody, _ := json.Marshal(map[string]any{
		"target_url": mockServer.URL,
		"suite_path": suitePath,
	})
	startReq := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(runReqBody))
	startRec := httptest.NewRecorder()
	uiHandler.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusAccepted {
		t.Fatalf("ui start failed with code %d: %s", startRec.Code, startRec.Body.String())
	}

	// Poll UI report until finished or timeout
	var uiReport ui.RunReport
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		repReq := httptest.NewRequest(http.MethodGet, "/api/run/report", nil)
		repRec := httptest.NewRecorder()
		uiHandler.ServeHTTP(repRec, repReq)
		if repRec.Code == http.StatusOK {
			var rep ui.RunReport
			if err := json.Unmarshal(repRec.Body.Bytes(), &rep); err == nil {
				if rep.Status == "passed" || rep.Status == "failed" || rep.Status == "cancelled" {
					uiReport = rep
					break
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	if uiReport.Status == "" || uiReport.Status == "running" {
		t.Fatalf("timed out waiting for UI test run to finish; last status: %q", uiReport.Status)
	}

	// 3. Assert contract parity between CLI summary and UI report
	if int(cliSummary["total_scenarios"].(float64)) != uiReport.TotalScenarios {
		t.Errorf("TotalScenarios mismatch: CLI=%v, UI=%v", cliSummary["total_scenarios"], uiReport.TotalScenarios)
	}
	if int(cliSummary["passed_scenarios"].(float64)) != uiReport.PassedScenarios {
		t.Errorf("PassedScenarios mismatch: CLI=%v, UI=%v", cliSummary["passed_scenarios"], uiReport.PassedScenarios)
	}
	if int(cliSummary["failed_scenarios"].(float64)) != uiReport.FailedScenarios {
		t.Errorf("FailedScenarios mismatch: CLI=%v, UI=%v", cliSummary["failed_scenarios"], uiReport.FailedScenarios)
	}
	if uiReport.Status != "passed" {
		t.Errorf("UI status = %q, want passed", uiReport.Status)
	}
}

func TestRunCmd_OnlyChanged_DerivesSiblingLogPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	suitePath := filepath.Join(tmpDir, "spectest.yaml")
	logPath := filepath.Join(tmpDir, "spectest.log.yaml") // Sibling

	suite := &domain.Suite{
		Version: 1,
		Input:   domain.InputOpenAPI,
		Servers: []string{"default"},
		Scenarios: []domain.Scenario{
			{
				ID:    "sc-1",
				Name:  "Scenario 1",
				Steps: []domain.Step{{Call: "GET /api/sc1", Expect: 200}},
			},
			{
				ID:    "sc-2",
				Name:  "Scenario 2",
				Steps: []domain.Step{{Call: "GET /api/sc2", Expect: 200}},
			},
		},
	}
	suiteData, _ := suitefile.EncodeSuite(suite)
	_ = os.WriteFile(suitePath, suiteData, 0o644)

	log := &domain.LogFile{
		Update: []domain.UpdateEntry{
			{
				N:         1,
				AfterHash: domain.SuiteHash(suite),
				Create: &domain.CreateSet{
					Scenarios: []domain.Scenario{{ID: "sc-1"}},
				},
			},
		},
	}
	logData, _ := suitefile.EncodeLog(log)
	_ = os.WriteFile(logPath, logData, 0o644)

	f := cmdutil.NewFactory()
	cmd := NewCommand(f)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Note: --only-changed is supplied WITHOUT --log.
	// CLI must derive <tmpDir>/spectest.log.yaml as the sibling log file.
	reportDir := filepath.Join(tmpDir, "reports")
	cmd.SetArgs([]string{"-f", suitePath, "-t", srv.URL, "--only-changed", "--report-dir", reportDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	got := out.String()
	wantSelection := "Selection: latest update #1 (1 scenarios selected)"
	if !strings.Contains(got, wantSelection) {
		t.Errorf("expected output to contain %q; got:\n%s", wantSelection, got)
	}

	// Also test explicit --log flag with non-sibling path
	customLogDir := t.TempDir()
	customLogPath := filepath.Join(customLogDir, "custom.log.yaml")
	_ = os.WriteFile(customLogPath, logData, 0o644)

	var out2 bytes.Buffer
	cmd2 := NewCommand(f)
	cmd2.SetOut(&out2)
	cmd2.SetErr(&out2)
	customReportDir := filepath.Join(customLogDir, "reports")
	cmd2.SetArgs([]string{"-f", suitePath, "-t", srv.URL, "--only-changed", "--log", customLogPath, "--report-dir", customReportDir})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("run with explicit --log failed: %v", err)
	}
	if !strings.Contains(out2.String(), wantSelection) {
		t.Errorf("expected output with explicit --log to contain %q; got:\n%s", wantSelection, out2.String())
	}
}

func TestRunCmd_Contract_UIParity_OnlyChanged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	suitePath := filepath.Join(tmpDir, "spectest.yaml")
	logPath := filepath.Join(tmpDir, "spectest.log.yaml")

	suite := &domain.Suite{
		Version: 1,
		Input:   domain.InputOpenAPI,
		Servers: []string{"default"},
		Scenarios: []domain.Scenario{
			{
				ID:    "sc-alpha",
				Name:  "Alpha Scenario",
				Steps: []domain.Step{{Call: "GET /alpha", Expect: 200}},
			},
			{
				ID:    "sc-beta",
				Name:  "Beta Scenario",
				Steps: []domain.Step{{Call: "GET /beta", Expect: 200}},
			},
		},
	}
	suiteBytes, _ := suitefile.EncodeSuite(suite)
	_ = os.WriteFile(suitePath, suiteBytes, 0o644)

	log := &domain.LogFile{
		Update: []domain.UpdateEntry{
			{
				N:         4,
				AfterHash: domain.SuiteHash(suite),
				Change: []domain.Change{
					{
						Target: domain.TargetScenario,
						ID:     "sc-beta",
						After:  map[string]any{},
					},
				},
			},
		},
	}
	logBytes, _ := suitefile.EncodeLog(log)
	_ = os.WriteFile(logPath, logBytes, 0o644)

	f := cmdutil.NewFactory()

	// 1. Run through CLI with --only-changed
	cliCmd := NewCommand(f)
	var cliOut bytes.Buffer
	cliCmd.SetOut(&cliOut)
	cliCmd.SetErr(&cliOut)
	cliReportDir := filepath.Join(tmpDir, "cli-reports-only-changed")
	cliCmd.SetArgs([]string{"-f", suitePath, "-t", srv.URL, "--only-changed", "--report-dir", cliReportDir})
	if err := cliCmd.Execute(); err != nil {
		t.Fatalf("cli execution failed: %v", err)
	}

	cliReportFile := filepath.Join(cliReportDir, "report.json")
	cliReportBytes, err := os.ReadFile(cliReportFile)
	if err != nil {
		t.Fatalf("read cli report: %v", err)
	}
	var cliSummary map[string]any
	if err := json.Unmarshal(cliReportBytes, &cliSummary); err != nil {
		t.Fatalf("unmarshal cli report: %v", err)
	}

	// 2. Run through UI with only_changed: true
	uiServer := ui.NewServer("127.0.0.1:0", tmpDir, f)
	uiHandler := uiServer.Routes()

	runReqBody, _ := json.Marshal(map[string]any{
		"target_url":   srv.URL,
		"suite_path":   suitePath,
		"only_changed": true,
	})
	startReq := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(runReqBody))
	startRec := httptest.NewRecorder()
	uiHandler.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusAccepted {
		t.Fatalf("ui start failed with code %d: %s", startRec.Code, startRec.Body.String())
	}

	var uiReport ui.RunReport
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		repReq := httptest.NewRequest(http.MethodGet, "/api/run/report", nil)
		repRec := httptest.NewRecorder()
		uiHandler.ServeHTTP(repRec, repReq)
		if repRec.Code == http.StatusOK {
			var rep ui.RunReport
			if err := json.Unmarshal(repRec.Body.Bytes(), &rep); err == nil {
				if rep.Status == "passed" || rep.Status == "failed" || rep.Status == "cancelled" {
					uiReport = rep
					break
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	if uiReport.Status != "passed" {
		t.Fatalf("UI status = %q, want passed", uiReport.Status)
	}

	// 3. Parity checks
	if int(cliSummary["total_scenarios"].(float64)) != 1 || uiReport.TotalScenarios != 1 {
		t.Errorf("TotalScenarios mismatch: CLI=%v, UI=%d, want 1", cliSummary["total_scenarios"], uiReport.TotalScenarios)
	}
	if cliSummary["selection_mode"] != "latest_update" || uiReport.SelectionMode != "latest_update" {
		t.Errorf("SelectionMode mismatch: CLI=%v, UI=%q, want latest_update", cliSummary["selection_mode"], uiReport.SelectionMode)
	}
	if int(cliSummary["update_n"].(float64)) != 4 || uiReport.UpdateN != 4 {
		t.Errorf("UpdateN mismatch: CLI=%v, UI=%d, want 4", cliSummary["update_n"], uiReport.UpdateN)
	}
	if len(uiReport.Results) != 1 || uiReport.Results[0].ScenarioID != "sc-beta" {
		t.Errorf("UI results = %+v, want only sc-beta", uiReport.Results)
	}
}

func TestRunCmd_OnlyChanged_NoUpdates_NoTargetRequired(t *testing.T) {
	tmpDir := t.TempDir()
	suitePath := filepath.Join(tmpDir, "spectest.yaml")
	logPath := filepath.Join(tmpDir, "spectest.log.yaml")

	suite := &domain.Suite{
		Version: 1,
		Input:   domain.InputOpenAPI,
		Servers: []string{"default"},
		Scenarios: []domain.Scenario{
			{ID: "sc-1", Steps: []domain.Step{{Call: "GET /api/sc1", Expect: 200}}},
		},
	}
	suiteData, _ := suitefile.EncodeSuite(suite)
	_ = os.WriteFile(suitePath, suiteData, 0o644)

	log := &domain.LogFile{
		Update: nil, // 0 updates
	}
	logData, _ := suitefile.EncodeLog(log)
	_ = os.WriteFile(logPath, logData, 0o644)

	f := cmdutil.NewFactory()
	cmd := NewCommand(f)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Note: NO -t / --target flag is passed!
	// With 0 scenarios to run, command must succeed as a no-op without requiring target or health check.
	cmd.SetArgs([]string{"-f", suitePath, "--only-changed", "--report-dir", filepath.Join(tmpDir, "reports")})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected successful execution with no target, got: %v", err)
	}

	got := out.String()
	wantSelection := "Selection: latest update #0 (0 scenarios selected)"
	if !strings.Contains(got, wantSelection) {
		t.Errorf("expected output to contain %q; got:\n%s", wantSelection, got)
	}
}

func TestRunCmd_OnlyChanged_AfterHashMismatch_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	suitePath := filepath.Join(tmpDir, "spectest.yaml")
	logPath := filepath.Join(tmpDir, "spectest.log.yaml")

	suite := &domain.Suite{
		Version: 1,
		Input:   domain.InputOpenAPI,
		Servers: []string{"default"},
		Scenarios: []domain.Scenario{
			{ID: "sc-1", Steps: []domain.Step{{Call: "GET /api/sc1", Expect: 200}}},
		},
	}
	suiteData, _ := suitefile.EncodeSuite(suite)
	_ = os.WriteFile(suitePath, suiteData, 0o644)

	log := &domain.LogFile{
		Update: []domain.UpdateEntry{
			{
				N:         1,
				AfterHash: "mismatched99", // Invalid hash
				Create: &domain.CreateSet{
					Scenarios: []domain.Scenario{{ID: "sc-1"}},
				},
			},
		},
	}
	logData, _ := suitefile.EncodeLog(log)
	_ = os.WriteFile(logPath, logData, 0o644)

	f := cmdutil.NewFactory()
	cmd := NewCommand(f)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	cmd.SetArgs([]string{"-f", suitePath, "-t", "http://localhost:8080", "--only-changed"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error on after_hash mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "suite hash mismatch") {
		t.Errorf("error = %v, want 'suite hash mismatch'", err)
	}
}
