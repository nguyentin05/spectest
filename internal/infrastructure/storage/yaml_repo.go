package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nguyentin05/spectest/internal/domain"
	"gopkg.in/yaml.v3"
)

// YAMLRepository implements domain.SuiteRepository for reading and writing spectest.yaml files.
type YAMLRepository struct{}

// NewYAMLRepository creates a new YAMLRepository instance.
func NewYAMLRepository() *YAMLRepository {
	return &YAMLRepository{}
}

// Load reads and unmarshals a spectest.yaml file from disk.
func (r *YAMLRepository) Load(path string) (*domain.TestSuiteFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read suite file %s: %w", path, err)
	}
	var suite domain.TestSuiteFile
	if err := yaml.Unmarshal(data, &suite); err != nil {
		return nil, fmt.Errorf("unmarshal suite yaml %s: %w", path, err)
	}
	if suite.Version == "" {
		suite.Version = "1.0"
	}
	return &suite, nil
}

// Save marshals and writes a TestSuiteFile to disk, creating parent directories if needed.
func (r *YAMLRepository) Save(path string, suite *domain.TestSuiteFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent dir for %s: %w", path, err)
	}
	if suite.Version == "" {
		suite.Version = "1.0"
	}

	var buf strings.Builder
	buf.WriteString("# =============================================================================\n")
	buf.WriteString("# SpecTest Single-File Test Suite\n")
	buf.WriteString("# Generated & Maintained by SpecTest\n")
	buf.WriteString("# =============================================================================\n\n")

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(suite); err != nil {
		return fmt.Errorf("marshal suite yaml: %w", err)
	}
	_ = enc.Close()

	if err := os.WriteFile(path, []byte(buf.String()), 0o644); err != nil {
		return fmt.Errorf("write suite file %s: %w", path, err)
	}
	return nil
}
