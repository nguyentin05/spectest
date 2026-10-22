package domain

import "context"

// LLMService defines the outbound port for interacting with Large Language Model providers.
// Implementations communicate over the standard Chat Completions protocol to synthesize
// structured test scenarios from pruned specifications and business requirements.
type LLMService interface {
	// Name returns a human-readable identifier for the active provider, model, and endpoint.
	Name() string

	// GenerateStructured sends a system instruction and user prompt to the configured model
	// and returns the raw JSON payload extracted from the completion response.
	GenerateStructured(ctx context.Context, systemPrompt string, userPrompt string, responseSchema interface{}) (string, error)
}

// SuiteRepository defines the persistence port for loading and saving `spectest.yaml`
// test suite artifacts on the local filesystem.
type SuiteRepository interface {
	// Load reads and deserializes a `spectest.yaml` file from the given filesystem path.
	Load(path string) (*TestSuiteFile, error)

	// Save serializes and writes a TestSuiteFile to disk at the given path, creating parent
	// directories automatically if they do not exist.
	Save(path string, suite *TestSuiteFile) error
}

// CredentialStore defines the configuration port for resolving and persisting LLM provider
// credentials, base endpoints, and model selections across environment variables and config files.
type CredentialStore interface {
	// GetAPIKey resolves the API key for the specified provider.
	GetAPIKey(provider string) (string, error)

	// SetAPIKey persists the API key for the specified provider with restricted (0600) permissions.
	SetAPIKey(provider, key string) error

	// GetEndpoint resolves the base API endpoint URL for the specified provider.
	GetEndpoint(provider string) (string, error)

	// SetEndpoint persists the base API endpoint URL for the specified provider.
	SetEndpoint(provider, endpoint string) error
}
