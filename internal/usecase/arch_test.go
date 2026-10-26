package usecase_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestArchitecture_UsecaseHasNoInfrastructureImports acts as a permanent CI guardrail
// enforcing the strict Clean Architecture dependency rule:
// use cases must depend on domain entities and ports, NEVER on concrete infrastructure or adapters.
func TestArchitecture_UsecaseHasNoInfrastructureImports(t *testing.T) {
	// Root of internal/usecase relative to this test file
	root := filepath.Join(".", ".")
	fset := token.NewFileSet()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		node, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}

		for _, imp := range node.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(importPath, "internal/infrastructure") {
				t.Errorf("Architecture violation in %s: use case layer must not import infrastructure (%s)", path, importPath)
			}
			if strings.Contains(importPath, "internal/adapter") {
				t.Errorf("Architecture violation in %s: use case layer must not import adapter (%s)", path, importPath)
			}
			if strings.Contains(importPath, "internal/platform/atomicfile") {
				t.Errorf("Architecture violation in %s: use case layer must not import atomicfile (%s)", path, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk use case directory: %v", err)
	}
}
