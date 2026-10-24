package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNormalizeTargetURL(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "empty", in: "", wantErr: true},
		{name: "whitespace", in: "   ", wantErr: true},
		{name: "with http", in: "http://localhost:8080", want: "http://localhost:8080"},
		{name: "with https", in: "https://api.example.com/v1", want: "https://api.example.com/v1"},
		{name: "auto-prefix localhost", in: "localhost:8080", want: "http://localhost:8080"},
		{name: "auto-prefix ip", in: "127.0.0.1:9000/api", want: "http://127.0.0.1:9000/api"},
		{name: "unsupported scheme", in: "ftp://files.example.com", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeTargetURL(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizeTargetURL(%q) err = %v, wantErr = %v", tt.in, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("NormalizeTargetURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestWaitForReady_Rejects502UntilReady(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 {
			w.WriteHeader(http.StatusBadGateway) // 502 Bad Gateway
			return
		}
		w.WriteHeader(http.StatusOK) // 200 OK after upstream finishes booting
	}))
	defer srv.Close()

	ctx := context.Background()
	if err := WaitForReady(ctx, srv.URL, 2*time.Second); err != nil {
		t.Fatalf("expected WaitForReady to succeed after initial 502s, got: %v", err)
	}
	if attempts <= 2 {
		t.Errorf("expected at least 3 attempts to transition past 502, got %d", attempts)
	}
}

func TestWaitForReady_FailsOnPersistent502(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway) // Reverse proxy always returns 502
	}))
	defer srv.Close()

	ctx := context.Background()
	err := WaitForReady(ctx, srv.URL, 300*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error when server continuously returns 502, got nil")
	}
	if !strings.Contains(err.Error(), "not ready within") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestWaitForReady_FastFailOnInvalidURL(t *testing.T) {
	start := time.Now()
	err := WaitForReady(context.Background(), "ftp://invalid-scheme", 30*time.Second)
	if err == nil {
		t.Fatal("expected error on unsupported scheme")
	}
	// Should fail immediately (< 50ms), not hang for 30s
	if time.Since(start) > 2*time.Second {
		t.Errorf("expected fail-fast, but took %v", time.Since(start))
	}
}

func TestWaitForReady_AutoPrefixesScheme(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Strip "http://" from srv.URL
	rawHostPort := strings.TrimPrefix(srv.URL, "http://")
	if err := WaitForReady(context.Background(), rawHostPort, 2*time.Second); err != nil {
		t.Fatalf("expected auto-prefixed url %q to be ready: %v", rawHostPort, err)
	}
}

func TestWaitForReady_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	ctx := context.Background()
	if err := WaitForReady(ctx, srv.URL, 2*time.Second); err != nil {
		t.Fatalf("WaitForReady failed for running server: %v", err)
	}
}

func TestWaitForReady_Handles404AsReady(t *testing.T) {
	// A 404 response still confirms that the HTTP server is up and listening on the port
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	ctx := context.Background()
	if err := WaitForReady(ctx, srv.URL+"/non-existent", 2*time.Second); err != nil {
		t.Fatalf("WaitForReady failed: %v", err)
	}
}

func TestWaitForReady_TimeoutWhenUnreachable(t *testing.T) {
	ctx := context.Background()
	// Target an unbound port to force connection failure
	err := WaitForReady(ctx, "http://127.0.0.1:54321", 300*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error for unreachable server, got nil")
	}
	if !strings.Contains(err.Error(), "not ready within") {
		t.Errorf("expected timeout message, got: %v", err)
	}
}

func TestWaitForReady_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := WaitForReady(ctx, "http://127.0.0.1:54321", 5*time.Second)
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
}

func TestLifecycleManager_ExistingServer_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	mgr := NewLifecycleManager("", srv.URL, 5)
	teardown, err := mgr.Up(context.Background())
	if err != nil {
		t.Fatalf("mgr.Up failed: %v", err)
	}
	if teardown == nil {
		t.Fatal("expected non-nil teardown function")
	}
	// Calling teardown should be a safe no-op
	teardown()
}

func TestLifecycleManager_ExistingServer_Unreachable(t *testing.T) {
	mgr := NewLifecycleManager("", "http://127.0.0.1:54322", 1)
	_, err := mgr.Up(context.Background())
	if err == nil {
		t.Fatal("expected error for unreachable existing server")
	}
	if !strings.Contains(err.Error(), "existing server check") {
		t.Errorf("unexpected error format: %v", err)
	}
}

func TestLifecycleManager_DockerCompose_FileNotFound(t *testing.T) {
	mgr := NewLifecycleManager("non-existent-docker-compose.yml", "http://localhost:8080", 5)
	_, err := mgr.Up(context.Background())
	if err == nil {
		t.Fatal("expected error for non-existent compose file")
	}
	if !strings.Contains(err.Error(), "docker compose file not found") {
		t.Errorf("unexpected error format: %v", err)
	}
}

func TestLifecycleManager_DockerCompose_MockExecution(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte("version: '3'\nservices: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Use standard true command as mock docker CLI
	mgr := NewLifecycleManager(composePath, srv.URL, 5)
	mgr.SetDockerCmd([]string{"true"})

	teardown, err := mgr.Up(context.Background())
	if err != nil {
		t.Fatalf("expected mock docker compose up to succeed, got: %v", err)
	}
	if teardown == nil {
		t.Fatal("expected non-nil teardown")
	}
	teardown()
}
