package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathResolver_ContainmentAndEscapes(t *testing.T) {
	tmpDir := t.TempDir()
	canonicalRoot, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		canonicalRoot = tmpDir
	}

	resolver, err := NewPathResolver(canonicalRoot)
	if err != nil {
		t.Fatalf("failed to create PathResolver: %v", err)
	}

	// 1. Valid paths inside root
	validPaths := []string{
		".",
		"",
		"tests",
		"tests/spectest.yaml",
		filepath.Join(canonicalRoot, "openapi.yaml"),
	}
	for _, p := range validPaths {
		resolved, err := resolver.Resolve(p)
		if err != nil {
			t.Errorf("expected path %q to resolve, got error: %v", p, err)
		}
		if !strings.HasPrefix(resolved, canonicalRoot) {
			t.Errorf("resolved path %q not prefixed by root %q", resolved, canonicalRoot)
		}
	}

	// 2. Lexical traversal attacks
	traversalPaths := []string{
		"../outside",
		"tests/../../outside",
		"/etc/passwd",
		filepath.Dir(canonicalRoot),
	}
	for _, p := range traversalPaths {
		_, err := resolver.Resolve(p)
		if err == nil {
			t.Errorf("expected traversal path %q to fail", p)
		} else if !strings.Contains(err.Error(), ErrCodePathOutsideRoot) {
			t.Errorf("expected error %s for path %q, got: %v", ErrCodePathOutsideRoot, p, err)
		}
	}

	// 3. Symlink escape detection
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o600); err != nil {
		t.Fatalf("create outside file: %v", err)
	}

	symlinkPath := filepath.Join(canonicalRoot, "escape_link")
	if err := os.Symlink(outsideFile, symlinkPath); err != nil {
		t.Skip("skipping symlink test: symlink creation failed")
	}

	_, err = resolver.Resolve("escape_link")
	if err == nil {
		t.Fatalf("expected symlink escape to fail")
	}
	if !strings.Contains(err.Error(), ErrCodePathOutsideRoot) {
		t.Errorf("expected %s in error, got: %v", ErrCodePathOutsideRoot, err)
	}
}
