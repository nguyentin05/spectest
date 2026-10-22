package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
)

func TestFileCredentialStore_SaveAndResolve(t *testing.T) {
	tmpDir := t.TempDir()
	globalPath := filepath.Join(tmpDir, ".spectest", "credentials.yaml")
	localPath := filepath.Join(tmpDir, ".spectestrc.yaml")

	store := NewFileCredentialStoreWithPath(globalPath, localPath)

	if err := store.SetDefaultProvider("openai"); err != nil {
		t.Fatalf("SetDefaultProvider failed: %v", err)
	}
	if err := store.SetEndpoint("openai", "https://api.openai.com/v1"); err != nil {
		t.Fatalf("SetEndpoint failed: %v", err)
	}
	if err := store.SetDefaultModel("openai", "gpt-4o-mini"); err != nil {
		t.Fatalf("SetDefaultModel failed: %v", err)
	}
	if err := store.SetAPIKey("openai", "sk-test-secret-123456"); err != nil {
		t.Fatalf("SetAPIKey failed: %v", err)
	}

	info, err := os.Stat(globalPath)
	if err != nil {
		t.Fatalf("stat credentials file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("expected 0600 permissions, got %04o", perm)
	}

	if got := store.GetDefaultProvider(); got != "openai" {
		t.Errorf("expected provider openai, got %q", got)
	}
	if got := store.GetDefaultModel("openai"); got != "gpt-4o-mini" {
		t.Errorf("expected model gpt-4o-mini, got %q", got)
	}
	ep, err := store.GetEndpoint("openai")
	if err != nil || ep != "https://api.openai.com/v1" {
		t.Errorf("expected endpoint https://api.openai.com/v1, got %q (err=%v)", ep, err)
	}
	key, err := store.GetAPIKey("openai")
	if err != nil || key != "sk-test-secret-123456" {
		t.Errorf("expected stored api key, got %q (err=%v)", key, err)
	}
}

func TestYAMLRepository_SaveLoadAndExport(t *testing.T) {
	tmpDir := t.TempDir()
	suitePath := filepath.Join(tmpDir, "spectest.yaml")
	matrixPath := filepath.Join(tmpDir, "test_matrix.md")
	postmanPath := filepath.Join(tmpDir, "postman.json")

	repo := NewYAMLRepository()
	suite := &domain.TestSuiteFile{
		Version: "1.0",
		Metadata: domain.SuiteMetadata{
			Name:          "Sample API Suite",
			TargetBaseURL: "http://localhost:8080",
		},
		Base: domain.BaseSuite{
			Scenarios: []domain.Scenario{
				{
					ID:   "SC_01",
					Name: "User Login Flow",
					Steps: []domain.Step{
						{
							ID:   "STEP_01",
							Name: "Login request",
							Request: domain.RequestDef{
								Method:   "POST",
								Endpoint: "/api/v1/auth/login",
							},
							Assert: &domain.AssertionRule{
								Status: 200,
							},
						},
					},
				},
			},
		},
	}

	if err := repo.Save(suitePath, suite); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := repo.Load(suitePath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Metadata.Name != "Sample API Suite" || len(loaded.Base.Scenarios) != 1 {
		t.Errorf("unexpected loaded suite: %+v", loaded)
	}

	if err := repo.ExportTestMatrix(matrixPath, loaded); err != nil {
		t.Fatalf("ExportTestMatrix failed: %v", err)
	}
	if _, err := os.Stat(matrixPath); err != nil {
		t.Errorf("expected matrix file to exist: %v", err)
	}

	if err := repo.ExportPostmanCollection(postmanPath, loaded); err != nil {
		t.Fatalf("ExportPostmanCollection failed: %v", err)
	}
	if _, err := os.Stat(postmanPath); err != nil {
		t.Errorf("expected postman file to exist: %v", err)
	}
}
