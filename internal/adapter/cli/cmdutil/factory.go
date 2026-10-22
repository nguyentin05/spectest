package cmdutil

import (
	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/llm"
	"github.com/nguyentin05/spectest/internal/infrastructure/storage"
	"github.com/spf13/pflag"
)

// Factory holds global runtime state and provides lazy initialization for Clean Architecture
// services shared across CLI subcommands.
type Factory struct {
	Verbose bool

	credStore *storage.FileCredentialStore
	suiteRepo *storage.YAMLRepository
}

// NewFactory creates a new Factory container with default settings.
func NewFactory() *Factory {
	return &Factory{}
}

// BindFlags registers global persistent flags (--verbose) on the main command.
func (f *Factory) BindFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&f.Verbose, "verbose", false, "Enable verbose debug logging")
}

// CredentialStore returns the shared credential store instance.
func (f *Factory) CredentialStore() *storage.FileCredentialStore {
	if f.credStore == nil {
		f.credStore = storage.NewFileCredentialStore()
	}
	return f.credStore
}

// SuiteRepository returns the YAML suite repository instance.
func (f *Factory) SuiteRepository() *storage.YAMLRepository {
	if f.suiteRepo == nil {
		f.suiteRepo = storage.NewYAMLRepository()
	}
	return f.suiteRepo
}

// LLMService resolves and initializes the active LLM provider configured via `spectest config`
// or environment variables.
func (f *Factory) LLMService() (domain.LLMService, error) {
	return llm.NewProvider(f.CredentialStore(), "", "", "")
}
