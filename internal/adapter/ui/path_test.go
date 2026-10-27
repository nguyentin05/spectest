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

func TestSafePath_Unit(t *testing.T) {
	tmpDir := t.TempDir()
	srv := &Server{workDir: tmpDir}

	// 1. Valid relative path inside workDir
	p, err := srv.safeFilePath("foo/bar.yaml")
	if err != nil {
		t.Fatalf("expected valid relative path to succeed, got %v", err)
	}
	expected := filepath.Join(tmpDir, "foo", "bar.yaml")
	if p != expected {
		t.Errorf("got %q, want %q", p, expected)
	}

	// 2. Valid absolute path inside workDir
	absInside := filepath.Join(tmpDir, "spec.yaml")
	p, err = srv.safeFilePath(absInside)
	if err != nil {
		t.Fatalf("expected valid absolute path to succeed, got %v", err)
	}
	if p != absInside {
		t.Errorf("got %q, want %q", p, absInside)
	}

	// 3. Traversal attack via ..
	_, err = srv.safeFilePath("../../../etc/passwd")
	if err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Errorf("expected access denied for traversal path, got %v", err)
	}

	// 4. Absolute path outside workDir
	_, err = srv.safeFilePath("/etc/passwd")
	if err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Errorf("expected access denied for external absolute path, got %v", err)
	}

	// 5. Root directory check in safeFilePath vs safePath
	_, err = srv.safeFilePath(".")
	if err == nil {
		t.Error("expected safeFilePath on root '.' to fail")
	}
	pDir, err := srv.safePath(".")
	if err != nil {
		t.Errorf("expected safePath on root '.' to succeed, got %v", err)
	}
	if pDir != tmpDir {
		t.Errorf("got %q, want %q", pDir, tmpDir)
	}

	// 6. safeUploadDest rejection
	uploadDir := filepath.Join(tmpDir, "uploads")
	_ = os.MkdirAll(uploadDir, 0o755)

	_, _, err = safeUploadDest(uploadDir, "../evil.sh")
	if err == nil {
		t.Error("expected safeUploadDest with '..' to fail")
	}

	_, _, err = safeUploadDest(uploadDir, "/evil.sh")
	if err == nil {
		t.Error("expected safeUploadDest with leading slash to fail")
	}

	dest, base, err := safeUploadDest(uploadDir, "valid-file.yaml")
	if err != nil {
		t.Fatalf("expected valid upload filename to succeed, got %v", err)
	}
	if base != "valid-file.yaml" || dest != filepath.Join(uploadDir, "valid-file.yaml") {
		t.Errorf("unexpected upload dest result: %s, %s", dest, base)
	}
}

func TestServer_PathTraversalAttacksBlocked(t *testing.T) {
	srv, _, _ := newTestServer(t)
	handler := srv.Routes()

	// 1. Markdown read traversal
	body, _ := json.Marshal(map[string]string{
		"path": "../../../etc/passwd",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/markdown/read", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "access denied") {
		t.Errorf("markdown read traversal: got %d, body: %s", rec.Code, rec.Body.String())
	}

	// 2. Specs plan traversal
	req = httptest.NewRequest(http.MethodGet, "/api/specs/plan?path=../../../../etc/passwd", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "access denied") {
		t.Errorf("specs plan traversal: got %d, body: %s", rec.Code, rec.Body.String())
	}

	// 3. Specs history traversal
	req = httptest.NewRequest(http.MethodGet, "/api/specs/history?path=../../../../etc/passwd", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "access denied") {
		t.Errorf("specs history traversal: got %d, body: %s", rec.Code, rec.Body.String())
	}

	// 4. Specs upload JSON traversal
	uploadBody, _ := json.Marshal(map[string]string{
		"file_name": "../../evil.txt",
		"content":   "malicious content",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/specs/upload", bytes.NewReader(uploadBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("specs upload traversal: got %d, want 400", rec.Code)
	}

	// 5. Generate API spec path traversal
	genBody, _ := json.Marshal(map[string]any{
		"apis": []map[string]string{
			{"server": "test", "path": "../../../etc/passwd"},
		},
	})
	req = httptest.NewRequest(http.MethodPost, "/api/generate", bytes.NewReader(genBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "access denied") {
		t.Errorf("generate api traversal: got %d, body: %s", rec.Code, rec.Body.String())
	}

	// 6. Generate output_dir traversal
	genBody, _ = json.Marshal(map[string]any{
		"apis": []map[string]string{
			{"server": "test", "path": "spec.yaml"},
		},
		"output_dir": "../../../tmp/evil",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/generate", bytes.NewReader(genBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("generate output_dir traversal: got %d, want 400", rec.Code)
	}

	// 7. Server healthcheck compose file traversal
	hcBody, _ := json.Marshal(map[string]any{
		"target_url":   "http://localhost:8080",
		"mode":         "compose",
		"compose_file": "../../../docker-compose.yml",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/server/healthcheck", bytes.NewReader(hcBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "access denied") {
		t.Errorf("healthcheck compose traversal: got %d, body: %s", rec.Code, rec.Body.String())
	}

	// 8. Run start suite_path traversal
	runBody, _ := json.Marshal(map[string]any{
		"target_url": "http://localhost:8080",
		"suite_path": "../../../spectest.yaml",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(runBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "access denied") {
		t.Errorf("run start suite traversal: got %d, body: %s", rec.Code, rec.Body.String())
	}
}
