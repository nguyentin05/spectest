package llm

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/nguyentin05/spectest/internal/infrastructure/storage"
)

func newIsolatedStore(t *testing.T) *storage.FileCredentialStore {
	t.Helper()
	for _, k := range []string{
		"SPECTEST_PROVIDER", "SPECTEST_MODEL", "SPECTEST_ENDPOINT", "SPECTEST_API_KEY",
		"MYLLM_API_KEY", "MYLLM_ENDPOINT", "MYLLM_MODEL",
	} {
		t.Setenv(k, "")
	}
	dir := t.TempDir()
	return storage.NewFileCredentialStoreWithPath(
		filepath.Join(dir, "credentials.yaml"),
		filepath.Join(dir, ".spectestrc.yaml"),
	)
}

func TestNewProvider_MissingSettings(t *testing.T) {
	store := newIsolatedStore(t)

	if _, err := NewProvider(store, "", "", ""); err == nil || !strings.Contains(err.Error(), "provider is not configured") {
		t.Errorf("expected missing provider error, got %v", err)
	}
	if _, err := NewProvider(store, "myllm", "", ""); err == nil || !strings.Contains(err.Error(), "model is not configured") {
		t.Errorf("expected missing model error, got %v", err)
	}
	if _, err := NewProvider(store, "myllm", "m1", ""); err == nil || !strings.Contains(err.Error(), "endpoint is not configured") {
		t.Errorf("expected missing endpoint error, got %v", err)
	}
}

func TestNewProvider_FromStore(t *testing.T) {
	store := newIsolatedStore(t)
	if err := store.SetDefaultProvider("myllm"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetDefaultModel("myllm", "model-x"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetEndpoint("myllm", "http://localhost:9000"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAPIKey("myllm", "key-123"); err != nil {
		t.Fatal(err)
	}

	svc, err := NewProvider(store, "", "", "")
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	c, ok := svc.(*Client)
	if !ok {
		t.Fatalf("expected *Client, got %T", svc)
	}
	if c.provider != "myllm" || c.model != "model-x" || c.endpoint != "http://localhost:9000/v1" || c.apiKey != "key-123" {
		t.Errorf("unexpected client config: %+v", c)
	}
}

func TestNewProvider_ExplicitArgsOverrideStore(t *testing.T) {
	store := newIsolatedStore(t)
	if err := store.SetDefaultProvider("myllm"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetDefaultModel("myllm", "stored-model"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetEndpoint("myllm", "http://stored"); err != nil {
		t.Fatal(err)
	}

	svc, err := NewProvider(store, "MyLLM", "flag-model", "http://flag")
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	c := svc.(*Client)
	if c.provider != "myllm" || c.model != "flag-model" || c.endpoint != "http://flag/v1" {
		t.Errorf("explicit arguments should override store: %+v", c)
	}
}
