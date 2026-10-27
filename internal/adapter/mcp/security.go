package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PathResolver provides safe path resolution contained strictly within a root directory.
type PathResolver struct {
	canonicalRoot string
}

// NewPathResolver initializes a PathResolver with a verified canonical root path.
func NewPathResolver(root string) (*PathResolver, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root absolute path: %w", err)
	}

	canonical := abs
	if eval, err := filepath.EvalSymlinks(abs); err == nil {
		canonical = eval
	}

	fi, err := os.Stat(canonical)
	if err != nil {
		return nil, fmt.Errorf("stat root path %q: %w", canonical, err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("root path %q is not a directory", canonical)
	}

	return &PathResolver{canonicalRoot: canonical}, nil
}

// Root returns the canonical root directory path.
func (r *PathResolver) Root() string {
	return r.canonicalRoot
}

// Resolve resolves a given path safely inside the root directory.
// Rejects path traversal, symlink escapes, or absolute paths outside the root boundary.
func (r *PathResolver) Resolve(p string) (string, error) {
	trimmed := strings.TrimSpace(p)
	cleanP := filepath.Clean(trimmed)
	if cleanP == "" || cleanP == "." {
		return r.canonicalRoot, nil
	}

	var candidate string
	if filepath.IsAbs(cleanP) {
		candidate = cleanP
	} else if filepath.VolumeName(cleanP) != "" {
		candidate = cleanP
	} else if strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "\\") || strings.HasPrefix(cleanP, string(filepath.Separator)) {
		// Drive-rooted or root-relative path (e.g. /etc/passwd or \etc\passwd on Windows).
		// Anchor it to the root volume so it can be evaluated against canonicalRoot.
		vol := filepath.VolumeName(r.canonicalRoot)
		candidate = vol + cleanP
	} else {
		candidate = filepath.Join(r.canonicalRoot, cleanP)
	}

	// 1. Lexical boundary containment check
	rel, err := filepath.Rel(r.canonicalRoot, candidate)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return "", fmt.Errorf("%s: path %q traverses outside root %q", ErrCodePathOutsideRoot, p, r.canonicalRoot)
	}

	// 2. Physical symlink boundary containment check (if file or parent directory exists)
	checkTarget := candidate
	for {
		eval, err := filepath.EvalSymlinks(checkTarget)
		if err == nil {
			evalRel, err := filepath.Rel(r.canonicalRoot, eval)
			if err != nil || strings.HasPrefix(evalRel, "..") || evalRel == ".." {
				return "", fmt.Errorf("%s: symlink %q resolves outside root to %q", ErrCodePathOutsideRoot, p, eval)
			}
			break
		}
		if os.IsNotExist(err) {
			parent := filepath.Dir(checkTarget)
			if parent == checkTarget {
				break
			}
			checkTarget = parent
			continue
		}
		break
	}

	return candidate, nil
}
