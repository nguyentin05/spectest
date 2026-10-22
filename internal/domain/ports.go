package domain

import "context"

// LLMService defines the contract for communicating with LLM providers (Gemini, Ollama, OpenAI).
type LLMService interface {
	Name() string
	GenerateStructured(ctx context.Context, systemPrompt string, userPrompt string, responseSchema interface{}) (string, error)
}

// SuiteRepository defines storage operations for reading and writing spectest.yaml.
type SuiteRepository interface {
	Load(path string) (*TestSuiteFile, error)
	Save(path string, suite *TestSuiteFile) error
}

// CredentialStore defines operations for storing and resolving API keys and endpoints securely.
type CredentialStore interface {
	GetAPIKey(provider string) (string, error)
	SetAPIKey(provider, key string) error
	GetEndpoint(provider string) (string, error)
	SetEndpoint(provider, endpoint string) error
}
