package generate

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/nguyentin05/spectest/internal/domain"
)

// Mode identifies the outcome of one Execute call.
type Mode string

const (
	ModeInitial      Mode = "initial"
	ModeUpdate       Mode = "update"
	ModeNoop         Mode = "no-op"
	ModeSync         Mode = "sync"
	ModeRecoverTrack Mode = "recover-track"
)

// buildBaseEntry constructs the single base entry for spectest.log.yaml. The
// suite snapshot is deep-cloned so later mutations cannot alter the log.
func (g *Generator) buildBaseEntry(src *domain.SourceSet, s *domain.Suite) domain.BaseEntry {
	catalog := make(domain.APICatalog, len(src.APIs))
	for _, a := range src.APIs {
		catalog[a.Key] = domain.APIState{Status: domain.StatusActive, Hash: a.Hash, Sig: a.Sig}
	}
	var reqHash string
	if src.Requirements != nil {
		reqHash = requirementsHash(src.Requirements.Raw)
	}
	return domain.BaseEntry{
		At:               g.clock.Now().Format(time.RFC3339),
		By:               "ai",
		Spectest:         cloneSuite(s),
		AfterHash:        suiteHash(s),
		APICatalog:       catalog,
		RequirementsHash: reqHash,
	}
}

// persist delegates to store.Save. On dry-run it returns immediately without
// touching the filesystem (no directory, no marker, no file).
func (g *Generator) persist(
	ctx context.Context, req Request, files *domain.FileSet, opts domain.WriteOptions,
) error {
	if req.DryRun {
		return nil
	}
	return g.store.Save(ctx, req.OutputDir, files, opts)
}

// joinDisplayPath returns the path as it should appear in the CLI summary.
// Relative paths inside the working directory get a "./" prefix.
func joinDisplayPath(dir, name string) string {
	joined := filepath.Join(dir, name)
	if filepath.IsAbs(joined) || joined == ".." || strings.HasPrefix(joined, ".."+string(filepath.Separator)) {
		return joined
	}
	return "." + string(filepath.Separator) + joined
}

// buildInitialFileSet assembles the files for a first run.
func (g *Generator) buildInitialFileSet(
	src *domain.SourceSet, suite *domain.Suite,
) *domain.FileSet {
	normalized := normalizeSuite(suite)
	return &domain.FileSet{
		Suite: normalized,
		Log: &domain.LogFile{
			Base:   g.buildBaseEntry(src, normalized),
			Update: []domain.UpdateEntry{},
		},
	}
}
