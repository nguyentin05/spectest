package llm

import (
	"fmt"
	"strings"

	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/storage"
)

// Config holds explicit options for initializing an LLM provider.
type Config struct {
	Provider string
	Model    string
	Endpoint string
	APIKey   string
}

// NewProvider resolves the configured provider, model, endpoint, and API key from flags
// or the credential store and returns a unified LLMService client.
func NewProvider(store *storage.FileCredentialStore, flagProvider, flagModel, flagEndpoint string) (domain.LLMService, error) {
	if store == nil {
		store = storage.NewFileCredentialStore()
	}

	provider := strings.ToLower(strings.TrimSpace(flagProvider))
	if provider == "" {
		provider = store.GetDefaultProvider()
	}
	if provider == "" {
		return nil, fmt.Errorf("llm provider is not configured (run 'spectest config init' or set SPECTEST_PROVIDER)")
	}

	model := strings.TrimSpace(flagModel)
	if model == "" {
		model = store.GetDefaultModel(provider)
	}
	if model == "" {
		return nil, fmt.Errorf("llm model is not configured for provider %q (run 'spectest config set model <name>' or set SPECTEST_MODEL)", provider)
	}

	endpoint := strings.TrimSpace(flagEndpoint)
	if endpoint == "" {
		endpoint, _ = store.GetEndpoint(provider)
	}
	if endpoint == "" {
		return nil, fmt.Errorf("llm endpoint is not configured for provider %q (run 'spectest config set endpoint <url>' or set SPECTEST_ENDPOINT)", provider)
	}

	apiKey, _ := store.GetAPIKey(provider)

	return NewClient(provider, endpoint, apiKey, model), nil
}
