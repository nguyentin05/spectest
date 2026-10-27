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
)

func TestServer_Generate_Validation(t *testing.T) {
	srv, _, tmpDir := newTestServer(t)
	handler := srv.Routes()

	// 1. Invalid method
	req := httptest.NewRequest(http.MethodGet, "/api/generate", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 Method Not Allowed, got %d", rr.Code)
	}

	// 2. Empty / invalid JSON
	req = httptest.NewRequest(http.MethodPost, "/api/generate", bytes.NewBufferString("{invalid"))
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", rr.Code)
	}

	// 3. Missing APIs
	body, _ := json.Marshal(map[string]any{
		"apis": []any{},
	})
	req = httptest.NewRequest(http.MethodPost, "/api/generate", bytes.NewReader(body))
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "at least one API specification is required") {
		t.Fatalf("expected 400 with missing APIs error, got %d: %s", rr.Code, rr.Body.String())
	}

	// 4. Missing API file
	body, _ = json.Marshal(map[string]any{
		"apis": []map[string]string{
			{"server": "test", "path": "non-existent.yaml"},
		},
	})
	req = httptest.NewRequest(http.MethodPost, "/api/generate", bytes.NewReader(body))
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "API spec file not found") {
		t.Fatalf("expected 400 for non-existent API spec file, got %d: %s", rr.Code, rr.Body.String())
	}

	// 5. Existing API file but missing requirements file
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	_ = os.WriteFile(specPath, []byte("openapi: 3.0.0\ninfo:\n  title: Test\n  version: 1.0.0\npaths: {}"), 0644)
	body, _ = json.Marshal(map[string]any{
		"apis": []map[string]string{
			{"server": "test", "path": "openapi.yaml"},
		},
		"req_path": "missing_req.md",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/generate", bytes.NewReader(body))
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "requirements file not found") {
		t.Fatalf("expected 400 for non-existent req file, got %d: %s", rr.Code, rr.Body.String())
	}

	// 6. Existing files but no LLM configured in temporary HOME
	body, _ = json.Marshal(map[string]any{
		"apis": []map[string]string{
			{"server": "test", "path": "openapi.yaml"},
		},
	})
	req = httptest.NewRequest(http.MethodPost, "/api/generate", bytes.NewReader(body))
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "LLM service not configured") {
		t.Fatalf("expected 400 with LLM configuration guidance, got %d: %s", rr.Code, rr.Body.String())
	}
}
