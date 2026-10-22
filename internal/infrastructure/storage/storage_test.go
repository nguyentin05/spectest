package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
)

// clearLLMEnv unsets every environment variable the credential store reads so tests
// only observe values written to their temporary config files.
func clearLLMEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"SPECTEST_PROVIDER", "SPECTEST_MODEL", "SPECTEST_ENDPOINT", "SPECTEST_API_KEY",
		"OPENAI_API_KEY", "OPENAI_ENDPOINT", "OPENAI_MODEL",
	} {
		t.Setenv(k, "")
	}
}

func newTestStore(t *testing.T) (*FileCredentialStore, string, string) {
	t.Helper()
	clearLLMEnv(t)
	dir := t.TempDir()
	globalPath := filepath.Join(dir, ".spectest", "credentials.yaml")
	localPath := filepath.Join(dir, ".spectestrc.yaml")
	return NewFileCredentialStoreWithPath(globalPath, localPath), globalPath, localPath
}

func TestFileCredentialStore_SaveAndResolve(t *testing.T) {
	store, globalPath, _ := newTestStore(t)

	if err := store.SetDefaultProvider("OpenAI"); err != nil {
		t.Fatalf("SetDefaultProvider: %v", err)
	}
	if err := store.SetEndpoint("openai", "https://api.openai.com/v1"); err != nil {
		t.Fatalf("SetEndpoint: %v", err)
	}
	if err := store.SetDefaultModel("openai", "gpt-4o-mini"); err != nil {
		t.Fatalf("SetDefaultModel: %v", err)
	}
	if err := store.SetAPIKey("openai", "sk-test-secret-123456"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}

	info, err := os.Stat(globalPath)
	if err != nil {
		t.Fatalf("stat credentials file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("credentials file permissions = %04o, want 0600", perm)
	}

	if got := store.GetDefaultProvider(); got != "openai" {
		t.Errorf("GetDefaultProvider() = %q, want openai (lowercased)", got)
	}
	if got := store.GetDefaultModel("openai"); got != "gpt-4o-mini" {
		t.Errorf("GetDefaultModel() = %q, want gpt-4o-mini", got)
	}
	if ep, _ := store.GetEndpoint("openai"); ep != "https://api.openai.com/v1" {
		t.Errorf("GetEndpoint() = %q", ep)
	}
	if key, _ := store.GetAPIKey("openai"); key != "sk-test-secret-123456" {
		t.Errorf("GetAPIKey() = %q", key)
	}
	if store.GlobalPath() != globalPath {
		t.Errorf("GlobalPath() = %q, want %q", store.GlobalPath(), globalPath)
	}
}

func TestFileCredentialStore_EmptyWhenUnconfigured(t *testing.T) {
	store, _, _ := newTestStore(t)

	if got := store.GetDefaultProvider(); got != "" {
		t.Errorf("GetDefaultProvider() = %q, want empty", got)
	}
	if got := store.GetDefaultModel("openai"); got != "" {
		t.Errorf("GetDefaultModel() = %q, want empty", got)
	}
	if ep, _ := store.GetEndpoint("openai"); ep != "" {
		t.Errorf("GetEndpoint() = %q, want empty", ep)
	}
	if key, _ := store.GetAPIKey("openai"); key != "" {
		t.Errorf("GetAPIKey() = %q, want empty", key)
	}
}

func TestFileCredentialStore_EnvironmentOverrides(t *testing.T) {
	store, _, _ := newTestStore(t)
	if err := store.SetDefaultProvider("openai"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAPIKey("openai", "file-key"); err != nil {
		t.Fatal(err)
	}

	t.Setenv("OPENAI_API_KEY", "provider-env-key")
	if key, _ := store.GetAPIKey("openai"); key != "provider-env-key" {
		t.Errorf("<PROVIDER>_API_KEY should override file, got %q", key)
	}

	t.Setenv("SPECTEST_API_KEY", "global-env-key")
	if key, _ := store.GetAPIKey("openai"); key != "global-env-key" {
		t.Errorf("SPECTEST_API_KEY should take highest priority, got %q", key)
	}

	t.Setenv("OPENAI_ENDPOINT", "http://env-endpoint")
	if ep, _ := store.GetEndpoint("openai"); ep != "http://env-endpoint" {
		t.Errorf("<PROVIDER>_ENDPOINT override failed, got %q", ep)
	}

	t.Setenv("OPENAI_MODEL", "env-model")
	if m := store.GetDefaultModel("openai"); m != "env-model" {
		t.Errorf("<PROVIDER>_MODEL override failed, got %q", m)
	}

	t.Setenv("SPECTEST_PROVIDER", "Custom")
	if p := store.GetDefaultProvider(); p != "custom" {
		t.Errorf("SPECTEST_PROVIDER override failed, got %q", p)
	}
}

func TestFileCredentialStore_LocalConfigTakesPriorityOverGlobal(t *testing.T) {
	store, _, localPath := newTestStore(t)
	if err := store.SetDefaultProvider("openai"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetEndpoint("openai", "http://global"); err != nil {
		t.Fatal(err)
	}

	local := "default_provider: local\nproviders:\n  openai:\n    endpoint: http://local\n    api_key: local-key\n    model: local-model\n"
	if err := os.WriteFile(localPath, []byte(local), 0o600); err != nil {
		t.Fatal(err)
	}

	if p := store.GetDefaultProvider(); p != "local" {
		t.Errorf("GetDefaultProvider() = %q, want local", p)
	}
	if ep, _ := store.GetEndpoint("openai"); ep != "http://local" {
		t.Errorf("GetEndpoint() = %q, want http://local", ep)
	}
	if key, _ := store.GetAPIKey("openai"); key != "local-key" {
		t.Errorf("GetAPIKey() = %q, want local-key", key)
	}
	if m := store.GetDefaultModel("openai"); m != "local-model" {
		t.Errorf("GetDefaultModel() = %q, want local-model", m)
	}
}

func TestFileCredentialStore_ValidationErrors(t *testing.T) {
	store, _, _ := newTestStore(t)

	if err := store.SetAPIKey("", "key"); err == nil {
		t.Error("SetAPIKey without provider should fail")
	}
	if err := store.SetEndpoint("", "http://x"); err == nil {
		t.Error("SetEndpoint without provider should fail")
	}
	if err := store.SetDefaultProvider("   "); err == nil {
		t.Error("SetDefaultProvider with blank value should fail")
	}
	if err := store.SetDefaultModel("openai", ""); err == nil {
		t.Error("SetDefaultModel with blank value should fail")
	}
}

func TestMaskKey(t *testing.T) {
	tests := map[string]string{
		"":                  "[not set]",
		"short":             "****",
		"12345678":          "****",
		"sk-abcdefghijklmn": "sk-a...klmn",
	}
	for in, want := range tests {
		if got := MaskKey(in); got != want {
			t.Errorf("MaskKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func sampleSuite() *domain.TestSuiteFile {
	return &domain.TestSuiteFile{
		Metadata: domain.SuiteMetadata{
			Name:          "Sample API Suite",
			TargetBaseURL: "http://localhost:8080/",
		},
		Base: domain.BaseSuite{
			Scenarios: []domain.Scenario{
				{
					ID:   "SC_01",
					Name: "Login Flow",
					Steps: []domain.Step{
						{
							ID:   "STEP_01",
							Name: "Login",
							Request: domain.RequestDef{
								Method:   "POST",
								Endpoint: "/api/v1/auth/login",
								Headers:  map[string]string{"Content-Type": "application/json"},
								Body:     map[string]interface{}{"email": "a@b.c"},
							},
							Assert: &domain.AssertionRule{
								Status:       200,
								MaxLatencyMs: 500,
								ForbidKeys:   []string{"password_hash"},
							},
						},
						{
							ID:   "STEP_02",
							Name: "Create order",
							Request: domain.RequestDef{
								Method:   "POST",
								Endpoint: "/api/v1/orders",
							},
							TestCases: []domain.TestCase{
								{
									ID:      "TC_HAPPY",
									Type:    "ai_business_valid",
									Payload: map[string]interface{}{"qty": 1},
									Assert: domain.AssertionRule{
										Status:      201,
										SchemaMatch: true,
										JSONPath:    []string{"$.data.status == 'CREATED'"},
									},
								},
								{
									ID:      "TC_SQLI",
									Type:    "go_security",
									Payload: map[string]interface{}{"code": "' OR '1'='1"},
									Assert:  domain.AssertionRule{Status: 400},
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestYAMLRepository_SaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "spectest.yaml")
	repo := NewYAMLRepository()

	if err := repo.Save(path, sampleSuite()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), "# ====") {
		t.Error("saved file should start with header banner")
	}

	loaded, err := repo.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Version != "1.0" {
		t.Errorf("Version = %q, want default 1.0", loaded.Version)
	}
	if len(loaded.Base.Scenarios) != 1 || len(loaded.Base.Scenarios[0].Steps) != 2 {
		t.Errorf("unexpected loaded structure: %+v", loaded.Base)
	}
}

func TestYAMLRepository_LoadErrors(t *testing.T) {
	repo := NewYAMLRepository()
	if _, err := repo.Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Error("Load of missing file should fail")
	}

	bad := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(bad, []byte("version: [unclosed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Load(bad); err == nil {
		t.Error("Load of malformed YAML should fail")
	}
}

func TestYAMLRepository_ExportTestMatrix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test_matrix.md")
	if err := NewYAMLRepository().ExportTestMatrix(path, sampleSuite()); err != nil {
		t.Fatalf("ExportTestMatrix: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	for _, want := range []string{
		"# SpecTest Coverage Matrix",
		"`SC_01`",
		"`1` scenarios, `2` steps, `3` test case variations",
		"latency <= 500ms",
		"forbid: password_hash",
		"`TC_SQLI`",
		"schema_match",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("matrix missing %q", want)
		}
	}
}

func TestYAMLRepository_ExportPostmanCollection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "postman.json")
	if err := NewYAMLRepository().ExportPostmanCollection(path, sampleSuite()); err != nil {
		t.Fatalf("ExportPostmanCollection: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var col postmanCollection
	if err := json.Unmarshal(raw, &col); err != nil {
		t.Fatalf("invalid postman JSON: %v", err)
	}
	if col.Info.Name != "Sample API Suite" || len(col.Item) != 1 || len(col.Item[0].Item) != 2 {
		t.Fatalf("unexpected collection structure: %+v", col)
	}
	first := col.Item[0].Item[0].Request
	if first.URL.Raw != "http://localhost:8080/api/v1/auth/login" {
		t.Errorf("URL = %q, trailing slash should be trimmed", first.URL.Raw)
	}
	second := col.Item[0].Item[1].Request
	if second.Body == nil || !strings.Contains(second.Body.Raw, "qty") {
		t.Error("step without body should fall back to first test case payload")
	}
}
