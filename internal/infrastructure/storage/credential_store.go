package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// CredentialConfig represents the persistent configuration stored in ~/.spectest/credentials.yaml
// or ./.spectestrc.yaml.
type CredentialConfig struct {
	DefaultProvider string                       `yaml:"default_provider,omitempty"`
	DefaultModel    string                       `yaml:"default_model,omitempty"`
	Providers       map[string]ProviderCredEntry `yaml:"providers,omitempty"`
}

// ProviderCredEntry holds credentials and endpoint settings for a specific LLM provider.
type ProviderCredEntry struct {
	APIKey   string `yaml:"api_key,omitempty"`
	Endpoint string `yaml:"endpoint,omitempty"`
	Model    string `yaml:"model,omitempty"`
}

// FileCredentialStore implements domain.CredentialStore using a 3-tier resolution hierarchy:
// 1. Environment variables (SPECTEST_*, <PROVIDER>_*)
// 2. Repository local config (./.spectestrc.yaml)
// 3. Global user config (~/.spectest/credentials.yaml with 0600 permissions)
type FileCredentialStore struct {
	globalPath string
	localPath  string
}

// NewFileCredentialStore creates a FileCredentialStore with standard config paths.
func NewFileCredentialStore() *FileCredentialStore {
	home, err := os.UserHomeDir()
	globalPath := ".spectest/credentials.yaml"
	if err == nil && home != "" {
		globalPath = filepath.Join(home, ".spectest", "credentials.yaml")
	}
	return &FileCredentialStore{
		globalPath: globalPath,
		localPath:  ".spectestrc.yaml",
	}
}

// NewFileCredentialStoreWithPath creates a store with custom paths (used in tests).
func NewFileCredentialStoreWithPath(globalPath, localPath string) *FileCredentialStore {
	return &FileCredentialStore{
		globalPath: globalPath,
		localPath:  localPath,
	}
}

// GlobalPath returns the path to the global credentials file.
func (s *FileCredentialStore) GlobalPath() string {
	return s.globalPath
}

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

func (s *FileCredentialStore) loadOrInitGlobal() *CredentialConfig {
	cfg, err := s.loadConfig(s.globalPath)
	if err != nil || cfg == nil {
		return &CredentialConfig{
			Providers: make(map[string]ProviderCredEntry),
		}
	}
	return cfg
}

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

// GetAPIKey resolves the API key for a provider.
func (s *FileCredentialStore) GetAPIKey(provider string) (string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = s.GetDefaultProvider()
	}

	// Tier 1: Environment variables
	if key := os.Getenv("SPECTEST_API_KEY"); key != "" {
		return key, nil
	}
	if provider != "" {
		if key := os.Getenv(strings.ToUpper(provider) + "_API_KEY"); key != "" {
			return key, nil
		}
	}

	// Tier 2: Repo-level ./.spectestrc.yaml
	if localCfg, err := s.loadConfig(s.localPath); err == nil && localCfg != nil {
		if entry, ok := localCfg.Providers[provider]; ok && entry.APIKey != "" {
			return entry.APIKey, nil
		}
	}

	// Tier 3: Global ~/.spectest/credentials.yaml
	if globalCfg, err := s.loadConfig(s.globalPath); err == nil && globalCfg != nil {
		if entry, ok := globalCfg.Providers[provider]; ok && entry.APIKey != "" {
			return entry.APIKey, nil
		}
	}

	return "", nil
}

// SetAPIKey saves the API key for a provider in ~/.spectest/credentials.yaml with 0600 permissions.
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

// GetEndpoint resolves the endpoint URL for a provider.
func (s *FileCredentialStore) GetEndpoint(provider string) (string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = s.GetDefaultProvider()
	}

	// Tier 1: Environment variables
	if ep := os.Getenv("SPECTEST_ENDPOINT"); ep != "" {
		return ep, nil
	}
	if provider != "" {
		if ep := os.Getenv(strings.ToUpper(provider) + "_ENDPOINT"); ep != "" {
			return ep, nil
		}
	}

	// Tier 2: Repo-level ./.spectestrc.yaml
	if localCfg, err := s.loadConfig(s.localPath); err == nil && localCfg != nil {
		if entry, ok := localCfg.Providers[provider]; ok && entry.Endpoint != "" {
			return entry.Endpoint, nil
		}
	}

	// Tier 3: Global ~/.spectest/credentials.yaml
	if globalCfg, err := s.loadConfig(s.globalPath); err == nil && globalCfg != nil {
		if entry, ok := globalCfg.Providers[provider]; ok && entry.Endpoint != "" {
			return entry.Endpoint, nil
		}
	}

	return "", nil
}

// SetEndpoint saves the endpoint URL for a provider in ~/.spectest/credentials.yaml.
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

// GetDefaultProvider returns the configured default provider, or empty string if unset.
func (s *FileCredentialStore) GetDefaultProvider() string {
	if p := os.Getenv("SPECTEST_PROVIDER"); p != "" {
		return strings.ToLower(strings.TrimSpace(p))
	}
	if localCfg, err := s.loadConfig(s.localPath); err == nil && localCfg != nil && localCfg.DefaultProvider != "" {
		return strings.ToLower(strings.TrimSpace(localCfg.DefaultProvider))
	}
	if globalCfg, err := s.loadConfig(s.globalPath); err == nil && globalCfg != nil && globalCfg.DefaultProvider != "" {
		return strings.ToLower(strings.TrimSpace(globalCfg.DefaultProvider))
	}
	return ""
}

// SetDefaultProvider sets the default LLM provider in ~/.spectest/credentials.yaml.
func (s *FileCredentialStore) SetDefaultProvider(provider string) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return fmt.Errorf("provider cannot be empty")
	}
	cfg := s.loadOrInitGlobal()
	cfg.DefaultProvider = provider
	return s.saveGlobal(cfg)
}

// GetDefaultModel returns the configured model for a provider, or empty string if unset.
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
	if localCfg, err := s.loadConfig(s.localPath); err == nil && localCfg != nil {
		if entry, ok := localCfg.Providers[provider]; ok && entry.Model != "" {
			return entry.Model
		}
		if localCfg.DefaultModel != "" {
			return localCfg.DefaultModel
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

// SetDefaultModel sets the model for a provider in ~/.spectest/credentials.yaml.
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

// MaskKey masks an API key for safe terminal display.
func MaskKey(key string) string {
	if key == "" {
		return "[not set]"
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
