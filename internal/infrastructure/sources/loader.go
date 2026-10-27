// Package sources loads and fingerprints the OpenAPI and requirements inputs
// for the v2 generate pipeline. It is the concrete domain.SourceLoader.
package sources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"sort"

	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/openapi"
)

// serverNamePattern is the accepted `--api` server identifier. It mirrors the
// CLI parser so a loader invoked with programmatic inputs rejects the same
// names the command line would.
var serverNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// Loader reads `--api` OpenAPI files and the optional requirements file,
// normalizes the operations, and fingerprints the raw source bytes.
type Loader struct{}

// NewLoader returns a ready-to-use Loader. It holds no state.
func NewLoader() *Loader { return &Loader{} }

// Load reads each API input in order, normalizes its operations, and returns
// the assembled SourceSet. Servers preserves the input order because the first
// server is the pipeline default; APIs is sorted by server, path, then method.
//
// OpenAPI entries store the file path and the raw-content hash. Requirements
// bytes are stored verbatim: the renderer reads the file as-is, so the loader
// neither extracts structure nor hashes it here.
//
// The context is checked before and after every file read so a canceled run
// stops without touching later files.
func (l *Loader) Load(ctx context.Context, inputs []domain.APIInput, requirementsPath string) (*domain.SourceSet, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("at least one --api input is required")
	}

	set := &domain.SourceSet{
		Servers:    make([]string, 0, len(inputs)),
		APIs:       make([]domain.API, 0),
		OpenAPI:    make(map[string]domain.FileState, len(inputs)),
		OpenAPIRaw: make(map[string]string, len(inputs)),
	}
	seen := make(map[string]struct{}, len(inputs))

	for _, input := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("load sources: %w", err)
		}
		if !serverNamePattern.MatchString(input.Server) {
			return nil, fmt.Errorf("invalid server %q: must match ^[a-z][a-z0-9_-]*$", input.Server)
		}
		if input.Path == "" {
			return nil, fmt.Errorf("invalid server %q: path must not be empty", input.Server)
		}
		if _, exists := seen[input.Server]; exists {
			return nil, fmt.Errorf("duplicate server %q", input.Server)
		}
		seen[input.Server] = struct{}{}

		raw, err := os.ReadFile(input.Path)
		if err != nil {
			return nil, fmt.Errorf("read openapi file %q: %w", input.Path, err)
		}
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("load sources: %w", err)
		}

		apis, err := openapi.Normalize(input.Server, raw)
		if err != nil {
			return nil, fmt.Errorf("normalize openapi for server %q (%s): %w", input.Server, input.Path, err)
		}
		if len(apis) == 0 {
			return nil, fmt.Errorf("openapi for server %q (%s) has no operations", input.Server, input.Path)
		}

		set.Servers = append(set.Servers, input.Server)
		set.OpenAPI[input.Server] = domain.FileState{File: input.Path, Hash: sourceHash(raw)}
		set.OpenAPIRaw[input.Server] = string(raw)
		set.APIs = append(set.APIs, apis...)
	}

	sort.Slice(set.APIs, func(i, j int) bool {
		if set.APIs[i].Server != set.APIs[j].Server {
			return set.APIs[i].Server < set.APIs[j].Server
		}
		if set.APIs[i].Path != set.APIs[j].Path {
			return set.APIs[i].Path < set.APIs[j].Path
		}
		return set.APIs[i].Method < set.APIs[j].Method
	})

	if requirementsPath != "" {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("load sources: %w", err)
		}
		raw, err := os.ReadFile(requirementsPath)
		if err != nil {
			return nil, fmt.Errorf("read requirements file %q: %w", requirementsPath, err)
		}
		set.Requirements = &domain.RequirementSource{File: requirementsPath, Raw: string(raw)}
		set.Input = domain.InputBoth
	} else {
		set.Input = domain.InputOpenAPI
	}

	return set, nil
}

// sourceHash returns the first 12 lowercase hex characters of the SHA-256
// digest of data. It is the raw-content fingerprint for OpenAPI inputs; the
// value is intentionally independent of the canonical API hashing in usecase.
func sourceHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:12]
}
