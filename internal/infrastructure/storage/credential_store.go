// Package storage implements filesystem persistence for `spectest.yaml` test suites,
// coverage matrices, Postman collections, and user LLM credentials.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// CredentialConfig represents the persistent YAML structure stored in
// `~/.spectest/credentials.yaml` (global).
type CredentialConfig struct {
	DefaultProvider string                       `yaml:"default_provider,omitempty"`
	DefaultModel    string                       `yaml:"default_model,omitempty"`
	Providers       map[string]ProviderCredEntry `yaml:"providers,omitempty"`
}

// ProviderCredEntry holds the API key, endpoint URL, and model name associated
// with a specific LLM provider identifier.
type ProviderCredEntry struct {
	APIKey   string `yaml:"api_key,omitempty"`
	Endpoint string `yaml:"endpoint,omitempty"`
	Model    string `yaml:"model,omitempty"`
}

// FileCredentialStore implements domain.CredentialStore using a 2-tier resolution order:
//  1. Environment variables (`SPECTEST_*` or `<PROVIDER>_*`)
//  2. User-global configuration (`~/.spectest/credentials.yaml` with `0600` permissions)
type FileCredentialStore struct {
	globalPath string
}

// NewFileCredentialStore constructs a FileCredentialStore pointing to `~/.spectest/credentials.yaml`
// for global storage.
func NewFileCredentialStore() *FileCredentialStore {
	home, err := os.UserHomeDir()
	globalPath := ".spectest/credentials.yaml"
	if err == nil && home != "" {
		globalPath = filepath.Join(home, ".spectest", "credentials.yaml")
	}
	return &FileCredentialStore{
		globalPath: globalPath,
	}
}

// NewFileCredentialStoreWithPath constructs a FileCredentialStore with an explicit global
// file path, enabling isolated unit testing without touching the user's home directory.
func NewFileCredentialStoreWithPath(globalPath string) *FileCredentialStore {
	return &FileCredentialStore{
		globalPath: globalPath,
	}
}

// GlobalPath returns the filesystem path of the user-global credentials file.
func (s *FileCredentialStore) GlobalPath() string {
	return s.globalPath
}

// loadConfig reads and unmarshals a CredentialConfig YAML file from disk.
func (s *FileCredentialStore) loadConfig(path string) (*CredentialConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg CredentialConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]ProviderCredEntry)
	}
	return &cfg, nil
}

// loadOrInitGlobal loads the global credentials file or returns an initialized empty
// CredentialConfig if the file does not exist yet.
func (s *FileCredentialStore) loadOrInitGlobal() *CredentialConfig {
	cfg, err := s.loadConfig(s.globalPath)
	if err != nil || cfg == nil {
		return &CredentialConfig{
			Providers: make(map[string]ProviderCredEntry),
		}
	}
	return cfg
}

// saveGlobal writes the given CredentialConfig to the global path and enforces
// `0700` directory permissions and `0600` file permissions so only the owner can read it.
func (s *FileCredentialStore) saveGlobal(cfg *CredentialConfig) error {
	dir := filepath.Dir(s.globalPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create credentials dir %s: %w", dir, err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	if err := os.WriteFile(s.globalPath, data, 0o600); err != nil {
		return fmt.Errorf("write credentials %s: %w", s.globalPath, err)
	}
	return os.Chmod(s.globalPath, 0o600)
}

// GetAPIKey resolves the API key for a provider by checking `SPECTEST_API_KEY`,
// `<PROVIDER>_API_KEY`, and finally the global credentials file.
func (s *FileCredentialStore) GetAPIKey(provider string) (string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = s.GetDefaultProvider()
	}

	if key := os.Getenv("SPECTEST_API_KEY"); key != "" {
		return key, nil
	}
	if provider != "" {
		if key := os.Getenv(strings.ToUpper(provider) + "_API_KEY"); key != "" {
			return key, nil
		}
	}

	if globalCfg, err := s.loadConfig(s.globalPath); err == nil && globalCfg != nil {
		if entry, ok := globalCfg.Providers[provider]; ok && entry.APIKey != "" {
			return entry.APIKey, nil
		}
	}

	return "", nil
}

// SetAPIKey saves the API key for the given (or active default) provider into
// `~/.spectest/credentials.yaml` with `0600` permissions. Returns an error if no provider is set.
func (s *FileCredentialStore) SetAPIKey(provider, key string) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = s.GetDefaultProvider()
	}
	if provider == "" {
		return fmt.Errorf("provider must be configured before setting api-key")
	}
	cfg := s.loadOrInitGlobal()
	entry := cfg.Providers[provider]
	entry.APIKey = strings.TrimSpace(key)
	cfg.Providers[provider] = entry
	return s.saveGlobal(cfg)
}

// GetEndpoint resolves the base endpoint URL for a provider by checking `SPECTEST_ENDPOINT`,
// `<PROVIDER>_ENDPOINT`, and the global credentials file.
func (s *FileCredentialStore) GetEndpoint(provider string) (string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = s.GetDefaultProvider()
	}

	if ep := os.Getenv("SPECTEST_ENDPOINT"); ep != "" {
		return ep, nil
	}
	if provider != "" {
		if ep := os.Getenv(strings.ToUpper(provider) + "_ENDPOINT"); ep != "" {
			return ep, nil
		}
	}

	if globalCfg, err := s.loadConfig(s.globalPath); err == nil && globalCfg != nil {
		if entry, ok := globalCfg.Providers[provider]; ok && entry.Endpoint != "" {
			return entry.Endpoint, nil
		}
	}

	return "", nil
}

// SetEndpoint persists the endpoint URL for the given (or active default) provider
// in `~/.spectest/credentials.yaml`. Returns an error if no provider is configured.
func (s *FileCredentialStore) SetEndpoint(provider, endpoint string) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = s.GetDefaultProvider()
	}
	if provider == "" {
		return fmt.Errorf("provider must be configured before setting endpoint")
	}
	cfg := s.loadOrInitGlobal()
	entry := cfg.Providers[provider]
	entry.Endpoint = strings.TrimSpace(endpoint)
	cfg.Providers[provider] = entry
	return s.saveGlobal(cfg)
}

// GetDefaultProvider resolves the active LLM provider identifier across `SPECTEST_PROVIDER`
// and the global credentials file. Returns empty string if unset.
func (s *FileCredentialStore) GetDefaultProvider() string {
	if p := os.Getenv("SPECTEST_PROVIDER"); p != "" {
		return strings.ToLower(strings.TrimSpace(p))
	}
	if globalCfg, err := s.loadConfig(s.globalPath); err == nil && globalCfg != nil && globalCfg.DefaultProvider != "" {
		return strings.ToLower(strings.TrimSpace(globalCfg.DefaultProvider))
	}
	return ""
}

// SetDefaultProvider updates and persists the default LLM provider identifier
// in `~/.spectest/credentials.yaml`.
func (s *FileCredentialStore) SetDefaultProvider(provider string) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return fmt.Errorf("provider cannot be empty")
	}
	cfg := s.loadOrInitGlobal()
	cfg.DefaultProvider = provider
	return s.saveGlobal(cfg)
}

// GetDefaultModel resolves the model name for the specified provider by checking
// `SPECTEST_MODEL`, `<PROVIDER>_MODEL`, provider-specific entries, and `default_model`.
func (s *FileCredentialStore) GetDefaultModel(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if m := os.Getenv("SPECTEST_MODEL"); m != "" {
		return strings.TrimSpace(m)
	}
	if provider != "" {
		if m := os.Getenv(strings.ToUpper(provider) + "_MODEL"); m != "" {
			return strings.TrimSpace(m)
		}
	}
	if globalCfg, err := s.loadConfig(s.globalPath); err == nil && globalCfg != nil {
		if entry, ok := globalCfg.Providers[provider]; ok && entry.Model != "" {
			return entry.Model
		}
		if globalCfg.DefaultModel != "" {
			return globalCfg.DefaultModel
		}
	}
	return ""
}

// SetDefaultModel updates and persists the model name for the given provider
// in `~/.spectest/credentials.yaml`.
func (s *FileCredentialStore) SetDefaultModel(provider, model string) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	model = strings.TrimSpace(model)
	if model == "" {
		return fmt.Errorf("model cannot be empty")
	}
	cfg := s.loadOrInitGlobal()
	cfg.DefaultModel = model
	if provider != "" {
		entry := cfg.Providers[provider]
		entry.Model = model
		cfg.Providers[provider] = entry
	}
	return s.saveGlobal(cfg)
}

// MaskKey redacts an API key for terminal display, preserving only the first and last
// four characters when the key is longer than 8 characters, or returning `[not set]` if empty.
func MaskKey(key string) string {
	if key == "" {
		return "[not set]"
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
