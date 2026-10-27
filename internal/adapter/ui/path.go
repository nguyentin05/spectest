package ui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// safePath validates that userPath is contained within s.workDir and resolves to a clean absolute path.
// It allows directory paths (where relative path == ".").
func (s *Server) safePath(userPath string) (string, error) {
	return safePathUnder(s.workDir, userPath, true)
}

// safeFilePath validates that userPath is contained within s.workDir, resolves to a clean absolute path,
// and does not point to the workspace root directory itself.
func (s *Server) safeFilePath(userPath string) (string, error) {
	return safePathUnder(s.workDir, userPath, false)
}

// safePathUnder validates that userPath is contained within baseDir and returns the clean absolute path.
// If allowBase is true, userPath resolving to baseDir (relative path == ".") is permitted.
func safePathUnder(baseDir, userPath string, allowBase bool) (string, error) {
	trimmed := strings.TrimSpace(userPath)
	if trimmed == "" {
		return "", errors.New("path cannot be empty")
	}

	cleanBase := filepath.Clean(baseDir)
	absBase, err := filepath.Abs(cleanBase)
	if err != nil {
		absBase = cleanBase
	}

	var target string
	if filepath.IsAbs(trimmed) {
		target = filepath.Clean(trimmed)
	} else {
		target = filepath.Join(absBase, filepath.Clean(trimmed))
	}

	rel, err := filepath.Rel(absBase, target)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." || strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, `\`) {
		return "", fmt.Errorf("access denied: path %q is outside base directory %q", userPath, baseDir)
	}

	if rel == "." && !allowBase {
		return "", fmt.Errorf("access denied: path %q cannot be the root directory", userPath)
	}

	// Guard against symlink traversal if target exists on disk
	if realTarget, err := filepath.EvalSymlinks(target); err == nil {
		realBase, errBase := filepath.EvalSymlinks(absBase)
		if errBase == nil {
			relReal, errRel := filepath.Rel(realBase, realTarget)
			if errRel != nil || strings.HasPrefix(relReal, "..") || relReal == ".." || strings.HasPrefix(relReal, "/") || strings.HasPrefix(relReal, `\`) {
				return "", fmt.Errorf("access denied: symlink path %q escapes base directory", userPath)
			}
		}
	}

	return target, nil
}

// safeUploadDest verifies that userFilename cannot escape destDir and returns the safe target path and base name.
func safeUploadDest(destDir, userFilename string) (string, string, error) {
	trimmed := strings.TrimSpace(userFilename)
	if trimmed == "" {
		return "", "", errors.New("file name cannot be empty")
	}

	if strings.ContainsAny(trimmed, `/\`) || strings.Contains(trimmed, "..") {
		return "", "", fmt.Errorf("invalid file name %q: directory separators or traversal sequences are not allowed", userFilename)
	}

	base := filepath.Base(filepath.Clean(trimmed))
	if base == "." || base == ".." || base != trimmed {
		return "", "", fmt.Errorf("invalid file name: %q", userFilename)
	}

	destPath := filepath.Join(destDir, base)
	rel, err := filepath.Rel(destDir, destPath)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." || rel == ".." || strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, `\`) {
		return "", "", fmt.Errorf("access denied: file %q escapes upload directory", userFilename)
	}

	return destPath, base, nil
}
