package cmdutil

import (
	"path/filepath"
	"testing"

	"github.com/nguyentin05/spectest/internal/infrastructure/storage"
	"github.com/spf13/pflag"
)

func TestFactory_InitializationAndSingletons(t *testing.T) {
	f := NewFactory()
	if f == nil {
		t.Fatal("expected NewFactory to return non-nil instance")
	}

	// Test BindFlags
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	f.BindFlags(flags)

	flag := flags.Lookup("verbose")
	if flag == nil {
		t.Fatal("expected --verbose flag to be bound")
	}
	if flag.DefValue != "false" {
		t.Errorf("expected default verbose value 'false', got %q", flag.DefValue)
	}

	// Test CredentialStore singleton
	store1 := f.CredentialStore()
	if store1 == nil {
		t.Fatal("expected CredentialStore to return non-nil instance")
	}
	store2 := f.CredentialStore()
	if store1 != store2 {
		t.Error("expected CredentialStore to return singleton instance")
	}

	// Test SuiteRepository singleton
	repo1 := f.SuiteRepository()
	if repo1 == nil {
		t.Fatal("expected SuiteRepository to return non-nil instance")
	}
	repo2 := f.SuiteRepository()
	if repo1 != repo2 {
		t.Error("expected SuiteRepository to return singleton instance")
	}
}

func TestFactory_LLMService_Resolution(t *testing.T) {
	t.Run("fails when unconfigured", func(t *testing.T) {
		t.Setenv("SPECTEST_PROVIDER", "")
		t.Setenv("OPENAI_API_KEY", "")
		t.Setenv("ANTHROPIC_API_KEY", "")
		t.Setenv("GEMINI_API_KEY", "")

		tmpDir := t.TempDir()
		emptyStore := storage.NewFileCredentialStoreWithPath(
			filepath.Join(tmpDir, "credentials.yaml"),
		)
		f := NewFactoryWith(emptyStore, nil)
		_, err := f.LLMService()
		if err == nil {
			t.Fatal("expected LLMService to return error when provider is unconfigured")
		}
	})

	t.Run("succeeds when configured via environment", func(t *testing.T) {
		t.Setenv("SPECTEST_PROVIDER", "openai")
		t.Setenv("SPECTEST_ENDPOINT", "https://api.openai.com/v1")
		t.Setenv("OPENAI_API_KEY", "sk-test-mock-key")
		t.Setenv("SPECTEST_MODEL", "gpt-4o-mini")

		f := NewFactory()
		svc, err := f.LLMService()
		if err != nil {
			t.Fatalf("expected LLMService to succeed, got: %v", err)
		}
		if svc == nil {
			t.Fatal("expected non-nil LLMService")
		}
	})
}
