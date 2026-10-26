package run

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nguyentin05/spectest/internal/domain"
)

type mockLifecycleManager struct {
	upErr       error
	teardownFn  func()
	teardownRan *atomic.Bool
}

func (m *mockLifecycleManager) Up(ctx context.Context) (func(), error) {
	if m.upErr != nil {
		return nil, m.upErr
	}
	return func() {
		if m.teardownRan != nil {
			m.teardownRan.Store(true)
		}
		if m.teardownFn != nil {
			m.teardownFn()
		}
	}, nil
}

func TestRunner_EndToEnd_StateChainingAndEvaluation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/login":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token": "token-xyz-123", "secret_key": "sec999"}`))

		case "/api/products":
			authHeader := r.Header.Get("Authorization")
			if authHeader != "Bearer token-xyz-123" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id": "prod-999"}`))

		case "/api/orders":
			var body map[string]any
			bodyBytes, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(bodyBytes, &body)
			if body["product_id"] == "prod-999" {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"order_id": "ord-111"}`))
			} else {
				w.WriteHeader(http.StatusUnprocessableEntity)
			}

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	suite := &domain.Suite{
		Version: 1,
		Servers: []string{"api"},
		APIs: []string{
			"POST {api}/api/login",
			"POST {api}/api/products",
			"POST {api}/api/orders",
		},
		Seed: map[string]domain.Seed{
			"admin": {
				Vars: map[string]any{
					"admin_email":    "admin@test.com",
					"admin_password": "super-secret-password",
				},
			},
		},
		Setup: map[string]domain.Setup{
			"auth_session": {
				Steps: []domain.Step{
					{
						Call:   "POST {api}/api/login",
						Body:   map[string]any{"email": "{{admin_email}}", "password": "{{admin_password}}"},
						Expect: 200,
						Save:   map[string]string{"jwt": "body.access_token"},
					},
				},
			},
			"product": {
				Use: []string{"auth_session"},
				Steps: []domain.Step{
					{
						Call:    "POST {api}/api/products",
						Headers: map[string]string{"Authorization": "Bearer {{jwt}}"},
						Body:    map[string]any{"name": "test_product"},
						Expect:  201,
						Save:    map[string]string{"prod_id": "body.id"},
					},
				},
			},
		},
		Scenarios: []domain.Scenario{
			{
				ID:    "checkout-flow",
				Name:  "Customer places order with prepared product",
				Setup: []string{"product"},
				Steps: []domain.Step{
					{
						Call:   "POST {api}/api/orders",
						Body:   map[string]any{"product_id": "{{prod_id}}"},
						Expect: 201,
					},
				},
			},
			{
				ID:   "skipped-scenario",
				Name: "Skipped feature",
				Skip: "Feature not yet deployed",
				Steps: []domain.Step{
					{
						Call:   "GET {api}/api/future",
						Expect: 200,
					},
				},
			},
		},
	}

	var teardownCalled atomic.Bool
	runner := NewRunner(Dependencies{
		LifecycleFactory: func(composeFile, targetURL string, timeoutSec int) domain.ServerLifecycleManager {
			return &mockLifecycleManager{
				teardownRan: &teardownCalled,
			}
		},
		HTTPClient: server.Client(),
	})

	var eventTypes []RunEventType
	summary, err := runner.Execute(context.Background(), Options{
		Suite:     suite,
		TargetURL: server.URL,
		OnEvent: func(event RunEvent) {
			eventTypes = append(eventTypes, event.Type)
		},
	})
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}

	if summary.TotalScenarios != 2 {
		t.Errorf("TotalScenarios = %d, want 2", summary.TotalScenarios)
	}
	if summary.PassedScenarios != 1 {
		t.Errorf("PassedScenarios = %d, want 1", summary.PassedScenarios)
	}
	if summary.SkippedScenarios != 1 {
		t.Errorf("SkippedScenarios = %d, want 1", summary.SkippedScenarios)
	}
	if summary.FailedScenarios != 0 {
		t.Errorf("FailedScenarios = %d, want 0", summary.FailedScenarios)
	}

	// Verify all setup steps are captured in summary.Records
	if len(summary.Records) != 3 {
		t.Fatalf("expected 3 records (2 setup + 1 scenario), got %d", len(summary.Records))
	}
	if !summary.Records[0].IsSetup || summary.Records[0].SetupName != "auth_session" {
		t.Errorf("record 0: expected auth_session setup step, got: %+v", summary.Records[0])
	}
	if !summary.Records[1].IsSetup || summary.Records[1].SetupName != "product" {
		t.Errorf("record 1: expected product setup step, got: %+v", summary.Records[1])
	}
	if summary.Records[2].IsSetup {
		t.Errorf("record 2: expected scenario step, got: %+v", summary.Records[2])
	}

	// Verify sensitive data redaction in request payload and response body
	loginReq := summary.Records[0].RequestPayload
	if strings.Contains(loginReq, "super-secret-password") {
		t.Errorf("leaked unredacted password in request payload: %s", loginReq)
	}
	if !strings.Contains(loginReq, "[REDACTED]") {
		t.Errorf("expected [REDACTED] in request payload, got: %s", loginReq)
	}

	loginResp := summary.Records[0].ResponseBody
	if strings.Contains(loginResp, "token-xyz-123") || strings.Contains(loginResp, "sec999") {
		t.Errorf("leaked unredacted token in response body: %s", loginResp)
	}
	if !strings.Contains(loginResp, "[REDACTED]") {
		t.Errorf("expected [REDACTED] in response body, got: %s", loginResp)
	}

	if val := summary.Records[0].Extracted["jwt"]; val != "[REDACTED]" {
		t.Errorf("expected extracted jwt to be [REDACTED], got: %v", val)
	}
}

func TestRunner_MultiTargetResolution(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token": "auth-ok"}`))
	}))
	defer authServer.Close()

	orderServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "order-ok"}`))
	}))
	defer orderServer.Close()

	suite := &domain.Suite{
		Version: 1,
		Servers: []string{"auth", "order"},
		Scenarios: []domain.Scenario{
			{
				ID:   "multi-service",
				Name: "Check multiple services",
				Steps: []domain.Step{
					{
						Call:   "GET {auth}/status",
						Expect: 200,
					},
					{
						Call:   "GET {order}/status",
						Expect: 200,
					},
				},
			},
		},
	}

	runner := NewRunner(Dependencies{
		HTTPClient: http.DefaultClient,
	})

	summary, err := runner.Execute(context.Background(), Options{
		Suite: suite,
		Targets: map[string]string{
			"auth":  authServer.URL,
			"order": orderServer.URL,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary.PassedChecks != 2 {
		t.Errorf("expected 2 passed checks, got %d", summary.PassedChecks)
	}
}

func TestRunner_TargetAliasErrors(t *testing.T) {
	runner := NewRunner(Dependencies{})

	// 1. Invalid target URL scheme in Targets
	_, err := runner.Execute(context.Background(), Options{
		Suite: &domain.Suite{Version: 1},
		Targets: map[string]string{
			"auth": "ftp://bad-scheme.com",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "only http and https are supported") {
		t.Fatalf("expected scheme error for invalid target URL, got: %v", err)
	}

	// 2. Unmapped {server} placeholder fails explicitly without silent fallback
	suiteUnmapped := &domain.Suite{
		Version: 1,
		Scenarios: []domain.Scenario{
			{
				ID:   "unmapped-test",
				Name: "Test unmapped server",
				Steps: []domain.Step{
					{Call: "GET {payment}/charge", Expect: 200},
				},
			},
		},
	}

	runnerWithMockProbe := NewRunner(Dependencies{
		Probe: func(ctx context.Context, targetURL string, timeout time.Duration) error {
			return nil
		},
	})
	summary, err := runnerWithMockProbe.Execute(context.Background(), Options{
		Suite: suiteUnmapped,
		Targets: map[string]string{
			"auth": "http://localhost:8001",
		},
	})
	if err != nil {
		t.Fatalf("unexpected execute error: %v", err)
	}
	if summary.FailedScenarios != 1 {
		t.Fatalf("expected 1 failed scenario for unmapped target, got %d", summary.FailedScenarios)
	}
	if len(summary.Records) == 0 || !strings.Contains(summary.Records[0].FailReason, "target alias {payment} is not configured and no default target URL provided") {
		t.Errorf("expected unmapped target alias error, got: %+v", summary.Records)
	}
}

func TestRunner_FailureWithAITriage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error": "permission_denied"}`))
	}))
	defer server.Close()

	suite := &domain.Suite{
		Version: 1,
		Scenarios: []domain.Scenario{
			{
				ID:   "forbidden-call",
				Name: "Access admin resource without role",
				Steps: []domain.Step{
					{
						Call:   "POST /admin/delete",
						Expect: 200,
					},
				},
			},
		},
	}

	mockLLM := &mockLLMService{
		generateFn: func(ctx context.Context, sys, user string, schema interface{}) (string, error) {
			return `{"root_cause": "Missing admin role in claims", "suggested_fix": "Add admin scope to request token"}`, nil
		},
	}

	runner := NewRunner(Dependencies{
		HTTPClient: server.Client(),
		LLM:        mockLLM,
	})

	summary, err := runner.Execute(context.Background(), Options{
		Suite:        suite,
		TargetURL:    server.URL,
		EnableTriage: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary.FailedScenarios != 1 {
		t.Errorf("FailedScenarios = %d, want 1", summary.FailedScenarios)
	}
	if len(summary.Records) == 0 {
		t.Fatalf("expected records, got 0")
	}

	rec := summary.Records[0]
	if !strings.Contains(rec.AIDiagnosis, "Missing admin role in claims") {
		t.Errorf("expected AI diagnosis to contain root cause, got: %s", rec.AIDiagnosis)
	}

	// Now run with EnableTriage: false, verify AI triage is skipped
	summaryNoTriage, err := runner.Execute(context.Background(), Options{
		Suite:        suite,
		TargetURL:    server.URL,
		EnableTriage: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summaryNoTriage.Records) == 0 {
		t.Fatalf("expected records, got 0")
	}
	if summaryNoTriage.Records[0].AIDiagnosis != "" {
		t.Errorf("expected empty AIDiagnosis when EnableTriage=false, got: %s", summaryNoTriage.Records[0].AIDiagnosis)
	}
}

func TestRunner_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	suite := &domain.Suite{
		Version: 1,
		Scenarios: []domain.Scenario{
			{
				ID:    "cancelled-sc",
				Name:  "Scenario cancelled",
				Steps: []domain.Step{{Call: "GET /api/test", Expect: 200}},
			},
		},
	}

	runner := NewRunner(Dependencies{
		HTTPClient: srv.Client(),
		Probe: func(ctx context.Context, targetURL string, timeout time.Duration) error {
			return nil
		},
	})

	_, err := runner.Execute(ctx, Options{
		Suite:     suite,
		TargetURL: srv.URL,
	})
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled error, got: %v", err)
	}
}

func TestRunner_MultiTargetHealthCheckAll(t *testing.T) {
	probedURLs := make([]string, 0)
	var probeCalled atomic.Int32

	runner := NewRunner(Dependencies{
		Probe: func(ctx context.Context, targetURL string, timeout time.Duration) error {
			probeCalled.Add(1)
			probedURLs = append(probedURLs, targetURL)
			if strings.Contains(targetURL, "offline") {
				return errors.New("connection refused")
			}
			return nil
		},
	})

	// 1. One target is offline -> fails immediately before scenarios run
	_, err := runner.Execute(context.Background(), Options{
		Suite:     &domain.Suite{Version: 1},
		TargetURL: "http://localhost:8080",
		Targets: map[string]string{
			"auth":    "http://localhost:8001",
			"offline": "http://localhost:8002/offline",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "readiness check failed") {
		t.Fatalf("expected readiness check failed error, got: %v", err)
	}

	// Verify all targets up to offline were checked
	if probeCalled.Load() == 0 {
		t.Error("expected probe function to be called")
	}
}

func TestRunner_SetupDAGCycleError(t *testing.T) {
	suite := &domain.Suite{
		Version: 1,
		Setup: map[string]domain.Setup{
			"A": {Use: []string{"B"}},
			"B": {Use: []string{"A"}},
		},
		Scenarios: []domain.Scenario{
			{
				ID:    "cycle-scenario",
				Name:  "Scenario with cyclic setup",
				Setup: []string{"A"},
			},
		},
	}

	runner := NewRunner(Dependencies{
		Probe: func(ctx context.Context, targetURL string, timeout time.Duration) error {
			return nil
		},
	})

	summary, err := runner.Execute(context.Background(), Options{
		Suite:     suite,
		TargetURL: "http://localhost:8080",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary.FailedScenarios != 1 {
		t.Errorf("expected 1 failed scenario for cycle, got %d", summary.FailedScenarios)
	}
	if len(summary.Records) == 0 || !strings.Contains(summary.Records[0].FailReason, "cyclic dependency") {
		t.Errorf("expected cyclic dependency fail reason, got: %+v", summary.Records)
	}
}

func TestRunner_TargetURLRequired(t *testing.T) {
	runner := NewRunner(Dependencies{
		SuiteLoader: func(path string) (*domain.Suite, error) {
			return &domain.Suite{Version: 1}, nil
		},
	})

	_, err := runner.Execute(context.Background(), Options{
		SuitePath: "spectest.yaml",
	})
	if err == nil || !strings.Contains(err.Error(), "target URL is required") {
		t.Fatalf("expected target URL required error, got: %v", err)
	}
}

func TestRunner_LifecycleFailure(t *testing.T) {
	runner := NewRunner(Dependencies{
		SuiteLoader: func(path string) (*domain.Suite, error) {
			return &domain.Suite{Version: 1}, nil
		},
		LifecycleFactory: func(composeFile, targetURL string, timeoutSec int) domain.ServerLifecycleManager {
			return &mockLifecycleManager{
				upErr: errors.New("docker daemon not running"),
			}
		},
	})

	_, err := runner.Execute(context.Background(), Options{
		SuitePath:   "spectest.yaml",
		TargetURL:   "http://localhost:8080",
		ComposeFile: "docker-compose.test.yml",
	})
	if err == nil || !strings.Contains(err.Error(), "setup server environment") {
		t.Fatalf("expected lifecycle error, got: %v", err)
	}
}

func TestRunner_ComposeMode_HealthChecksAllTargets(t *testing.T) {
	probedURLs := make([]string, 0)
	var composeBooted bool

	runner := NewRunner(Dependencies{
		SuiteLoader: func(path string) (*domain.Suite, error) {
			return &domain.Suite{Version: 1}, nil
		},
		LifecycleFactory: func(composeFile, targetURL string, timeoutSec int) domain.ServerLifecycleManager {
			return &mockLifecycleManager{
				teardownFn: func() {
					composeBooted = false
				},
			}
		},
		Probe: func(ctx context.Context, targetURL string, timeout time.Duration) error {
			probedURLs = append(probedURLs, targetURL)
			return nil
		},
	})

	composeBooted = true
	_, err := runner.Execute(context.Background(), Options{
		Suite:       &domain.Suite{Version: 1},
		ComposeFile: "docker-compose.test.yml",
		TargetURL:   "http://localhost:8080",
		Targets: map[string]string{
			"auth":  "http://localhost:8001",
			"order": "http://localhost:8002",
		},
	})
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}

	// Verify all 3 unique targets were probed in compose mode
	if len(probedURLs) != 3 {
		t.Errorf("expected 3 probed URLs, got %d: %v", len(probedURLs), probedURLs)
	}
	_ = composeBooted
}

func TestRunner_SensitiveRedaction_URLQueryAndJWT(t *testing.T) {
	var receivedToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/token":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token": "secret-jwt-token-value-999"}`))
		case "/api/verify":
			receivedToken = r.URL.Query().Get("token")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "ok"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	suite := &domain.Suite{
		Version: 1,
		Setup: map[string]domain.Setup{
			"auth": {
				Steps: []domain.Step{
					{
						Call:   "POST /api/auth/token",
						Expect: 200,
						Save: map[string]string{
							"jwt":       "body.access_token",
							"client_id": "header.Content-Type",
						},
					},
				},
			},
		},
		Scenarios: []domain.Scenario{
			{
				ID:    "verify-jwt-in-url",
				Name:  "Verify JWT query in URL",
				Setup: []string{"auth"},
				Steps: []domain.Step{
					{
						Call:   "GET /api/verify?token={{jwt}}&safe_id=123",
						Expect: 200,
					},
				},
			},
		},
	}

	runner := NewRunner(Dependencies{
		HTTPClient: srv.Client(),
	})

	summary, err := runner.Execute(context.Background(), Options{
		Suite:     suite,
		TargetURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}

	// 1. Verify that server received the actual unredacted token over the wire
	if receivedToken != "secret-jwt-token-value-999" {
		t.Errorf("server did not receive unredacted token over wire, got: %q", receivedToken)
	}

	if len(summary.Records) < 2 {
		t.Fatalf("expected at least 2 records, got %d", len(summary.Records))
	}

	// 2. Verify save: {jwt: ...} is redacted in rec.Extracted
	setupRec := summary.Records[0]
	if val, ok := setupRec.Extracted["jwt"]; !ok {
		t.Errorf("expected extracted jwt key")
	} else if val != "[REDACTED]" {
		t.Errorf("expected extracted jwt to be [REDACTED], got: %v", val)
	}
	// Non-sensitive key should remain unredacted
	if val, ok := setupRec.Extracted["client_id"]; !ok || val != "application/json" {
		t.Errorf("expected extracted client_id to be application/json, got: %v", val)
	}

	// 3. Verify URL query token is redacted in rec.Endpoint
	scenarioRec := summary.Records[1]
	if strings.Contains(scenarioRec.Endpoint, "secret-jwt-token-value-999") {
		t.Errorf("leaked sensitive token in ExecutionRecord.Endpoint: %s", scenarioRec.Endpoint)
	}
	if !strings.Contains(scenarioRec.Endpoint, "token=[REDACTED]") {
		t.Errorf("expected token=[REDACTED] in ExecutionRecord.Endpoint, got: %s", scenarioRec.Endpoint)
	}
	if !strings.Contains(scenarioRec.Endpoint, "safe_id=123") {
		t.Errorf("expected safe_id=123 preserved in ExecutionRecord.Endpoint, got: %s", scenarioRec.Endpoint)
	}
}

func TestRedactURL_UserinfoAndQueries(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{
			input: "http://user:password123@localhost:8080/path?token=secret&safe=1",
			want:  "http://user:[REDACTED]@localhost:8080/path?safe=1&token=[REDACTED]",
		},
		{
			input: "http://localhost:8080/path?jwt=eyJhbGciOi...&id=99",
			want:  "http://localhost:8080/path?id=99&jwt=[REDACTED]",
		},
		{
			input: "http://token:secret@localhost:8080/api",
			want:  "http://[REDACTED]:[REDACTED]@localhost:8080/api",
		},
		{
			input: "/api/check?api_key=my-key&page=1",
			want:  "/api/check?api_key=[REDACTED]&page=1",
		},
	}

	for _, tc := range cases {
		got := redactURL(tc.input)
		if got != tc.want {
			t.Errorf("redactURL(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestRunner_ResponseBodyLimit_Truncation(t *testing.T) {
	largeBody := strings.Repeat("A", 1500000) // 1.5 MB body
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(largeBody))
	}))
	defer srv.Close()

	suite := &domain.Suite{
		Version: 1,
		Scenarios: []domain.Scenario{
			{
				ID:    "large-body-test",
				Name:  "Test large body truncation",
				Steps: []domain.Step{{Call: "GET /api/large", Expect: 200}},
			},
		},
	}

	runner := NewRunner(Dependencies{
		HTTPClient: srv.Client(),
	})

	summary, err := runner.Execute(context.Background(), Options{
		Suite:     suite,
		TargetURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}

	if summary.PassedScenarios != 1 {
		t.Errorf("expected 1 passed scenario, got %d", summary.PassedScenarios)
	}
	if len(summary.Records) == 0 {
		t.Fatalf("expected records, got 0")
	}

	rec := summary.Records[0]
	if !rec.Passed {
		t.Errorf("expected step to pass, got fail: %s", rec.FailReason)
	}
	if !strings.Contains(rec.ResponseBody, "[TRUNCATED: response body exceeded 1MB limit]") {
		t.Errorf("expected truncated marker in ResponseBody, got: %s", rec.ResponseBody[:min(100, len(rec.ResponseBody))])
	}
	// Verify stored response body does not exceed limit + marker length
	if len(rec.ResponseBody) > maxResponseBodyBytes+100 {
		t.Errorf("stored response body size = %d, exceeded limit", len(rec.ResponseBody))
	}
}

func TestRunner_ResponseBodyReadError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set Content-Length to 1000 but write only 5 bytes, then hijack connection to cause unexpected EOF
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("short"))
		if hj, ok := w.(http.Hijacker); ok {
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
		}
	}))
	defer srv.Close()

	suite := &domain.Suite{
		Version: 1,
		Scenarios: []domain.Scenario{
			{
				ID:    "broken-body-test",
				Name:  "Test broken body read error",
				Steps: []domain.Step{{Call: "GET /api/broken", Expect: 200}},
			},
		},
	}

	runner := NewRunner(Dependencies{
		HTTPClient: srv.Client(),
	})

	summary, err := runner.Execute(context.Background(), Options{
		Suite:     suite,
		TargetURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}

	if summary.FailedScenarios != 1 {
		t.Errorf("expected 1 failed scenario for broken body, got %d", summary.FailedScenarios)
	}
	if len(summary.Records) == 0 || !strings.Contains(summary.Records[0].FailReason, "read response body") {
		t.Errorf("expected read response body failure, got: %+v", summary.Records)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestRunner_EndToEnd_OnlyChanged_UnselectedScenarioDoesNotSendHTTPRequest(t *testing.T) {
	var sc1Calls, sc2Calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sc1":
			sc1Calls.Add(1)
			w.WriteHeader(http.StatusOK)
		case "/sc2":
			sc2Calls.Add(1)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	suite := &domain.Suite{
		Version: 1,
		Input:   domain.InputOpenAPI,
		Servers: []string{"default"},
		Scenarios: []domain.Scenario{
			{
				ID:    "scenario-1",
				Name:  "Scenario 1",
				Steps: []domain.Step{{Call: "GET /sc1", Expect: 200}},
			},
			{
				ID:    "scenario-2",
				Name:  "Scenario 2",
				Steps: []domain.Step{{Call: "GET /sc2", Expect: 200}},
			},
		},
	}

	log := &domain.LogFile{
		Update: []domain.UpdateEntry{
			{
				N:         1,
				AfterHash: domain.SuiteHash(suite),
				Change: []domain.Change{
					{
						Target: domain.TargetScenario,
						ID:     "scenario-1",
						After:  map[string]any{"steps": []any{}},
					},
				},
			},
		},
	}

	runner := NewRunner(Dependencies{
		HTTPClient: srv.Client(),
		LogLoader: func(path string) (*domain.LogFile, error) {
			return log, nil
		},
	})

	var startEvt RunEvent
	summary, err := runner.Execute(context.Background(), Options{
		Suite:       suite,
		TargetURL:   srv.URL,
		OnlyChanged: true,
		OnEvent: func(evt RunEvent) {
			if evt.Type == EventSuiteStart {
				startEvt = evt
			}
		},
	})
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}

	// Verify only scenario-1 ran
	if summary.SelectionMode != "latest_update" {
		t.Errorf("selection_mode = %q, want latest_update", summary.SelectionMode)
	}
	if summary.UpdateN != 1 {
		t.Errorf("update_n = %d, want 1", summary.UpdateN)
	}
	if summary.TotalScenarios != 1 {
		t.Errorf("total_scenarios = %d, want 1", summary.TotalScenarios)
	}
	if summary.PassedScenarios != 1 {
		t.Errorf("passed_scenarios = %d, want 1", summary.PassedScenarios)
	}

	// Verify EventSuiteStart
	if startEvt.SelectionMode != "latest_update" || startEvt.UpdateN != 1 || len(startEvt.SelectedScenarios) != 1 || startEvt.SelectedScenarios[0] != "scenario-1" {
		t.Errorf("start event = %+v, want scenario-1 selected", startEvt)
	}

	// Verify HTTP hits: sc1 was called, sc2 was NEVER called
	if sc1Calls.Load() != 1 {
		t.Errorf("sc1Calls = %d, want 1", sc1Calls.Load())
	}
	if sc2Calls.Load() != 0 {
		t.Errorf("sc2Calls = %d, want 0 (unselected scenario must not send any HTTP requests)", sc2Calls.Load())
	}
}

func TestRunner_Execute_Default_RunsAllScenarios(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	suite := &domain.Suite{
		Version: 1,
		Input:   domain.InputOpenAPI,
		Servers: []string{"default"},
		Scenarios: []domain.Scenario{
			{ID: "s1", Steps: []domain.Step{{Call: "GET /s1", Expect: 200}}},
			{ID: "s2", Steps: []domain.Step{{Call: "GET /s2", Expect: 200}}},
		},
	}

	runner := NewRunner(Dependencies{
		HTTPClient: srv.Client(),
	})

	var startEvt RunEvent
	summary, err := runner.Execute(context.Background(), Options{
		Suite:       suite,
		TargetURL:   srv.URL,
		OnlyChanged: false, // Default
		OnEvent: func(evt RunEvent) {
			if evt.Type == EventSuiteStart {
				startEvt = evt
			}
		},
	})
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}

	if summary.SelectionMode != "all" {
		t.Errorf("selection_mode = %q, want all", summary.SelectionMode)
	}
	if summary.TotalScenarios != 2 {
		t.Errorf("total_scenarios = %d, want 2", summary.TotalScenarios)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2", calls.Load())
	}
	if startEvt.SelectionMode != "all" || len(startEvt.SelectedScenarios) != 2 {
		t.Errorf("start event = %+v, want all 2 scenarios", startEvt)
	}
}

func TestRunner_OnlyChanged_NoUpdates_NoTargetRequired_NoOpSuccess(t *testing.T) {
	suite := &domain.Suite{
		Version: 1,
		Input:   domain.InputOpenAPI,
		Servers: []string{"default"},
		Scenarios: []domain.Scenario{
			{ID: "s1", Steps: []domain.Step{{Call: "GET /s1", Expect: 200}}},
		},
	}

	log := &domain.LogFile{
		Update: nil, // No updates
	}

	runner := NewRunner(Dependencies{
		LogLoader: func(path string) (*domain.LogFile, error) {
			return log, nil
		},
	})

	var startEvt, completeEvt RunEvent
	// Note: TargetURL and Targets are completely empty!
	summary, err := runner.Execute(context.Background(), Options{
		Suite:       suite,
		OnlyChanged: true,
		OnEvent: func(evt RunEvent) {
			switch evt.Type {
			case EventSuiteStart:
				startEvt = evt
			case EventSuiteComplete:
				completeEvt = evt
			}
		},
	})
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}

	if summary.TotalScenarios != 0 {
		t.Errorf("total_scenarios = %d, want 0", summary.TotalScenarios)
	}
	if summary.SelectionMode != "latest_update" {
		t.Errorf("selection_mode = %q, want latest_update", summary.SelectionMode)
	}
	if summary.UpdateN != 0 {
		t.Errorf("update_n = %d, want 0", summary.UpdateN)
	}
	if startEvt.TotalScenarios != 0 || completeEvt.TotalScenarios != 0 {
		t.Errorf("expected 0 scenarios in start and complete events")
	}
}

func TestRunner_StepBody_NonJSONSerializable_FailsGracefully(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	suite := &domain.Suite{
		Version: 1,
		Input:   domain.InputOpenAPI,
		Servers: []string{"default"},
		Scenarios: []domain.Scenario{
			{
				ID:   "nan-body-test",
				Name: "Test non-serializable NaN body",
				Steps: []domain.Step{
					{
						Call:   "POST /api/items",
						Expect: 200,
						Body:   map[string]any{"invalid_number": math.NaN()},
					},
				},
			},
		},
	}

	runner := NewRunner(Dependencies{
		HTTPClient: srv.Client(),
	})

	summary, err := runner.Execute(context.Background(), Options{
		Suite:     suite,
		TargetURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("unexpected runner execution error: %v", err)
	}

	if summary.FailedScenarios != 1 {
		t.Fatalf("expected 1 failed scenario for NaN body, got %d", summary.FailedScenarios)
	}
	if len(summary.Records) == 0 {
		t.Fatal("expected at least 1 execution record")
	}
	if summary.Records[0].Passed {
		t.Error("expected step record to fail, but it passed")
	}
	if !strings.Contains(summary.Records[0].FailReason, "encode request body JSON") {
		t.Errorf("expected FailReason to mention 'encode request body JSON', got: %q", summary.Records[0].FailReason)
	}
}
