package ui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type markdownReadRequest struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
}

type markdownReadResponse struct {
	Path       string   `json:"path,omitempty"`
	Title      string   `json:"title"`
	Sections   []string `json:"sections"`
	TableCount int      `json:"table_count"`
	LineCount  int      `json:"line_count"`
	RawContent string   `json:"raw_content"`
}

// handleReadMarkdown parses and prepares a markdown document (matrix or GitHub Actions summary).
func (s *Server) handleReadMarkdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req markdownReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	var content string
	var path string

	if strings.TrimSpace(req.Content) != "" {
		content = req.Content
		path = req.Path
	} else if strings.TrimSpace(req.Path) != "" {
		safePath, err := s.safeFilePath(req.Path)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		data, err := os.ReadFile(safePath)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("cannot read markdown file %q: %v", safePath, err))
			return
		}
		path = safePath
		content = string(data)
	} else {
		// Look for default test_matrix.md
		candidates := []string{
			"test_matrix.md",
			filepath.Join(s.workDir, "test_matrix.md"),
		}
		for _, c := range candidates {
			if data, err := os.ReadFile(c); err == nil {
				path = c
				content = string(data)
				break
			}
		}
		if content == "" {
			writeError(w, http.StatusBadRequest, "no markdown path or content provided, and test_matrix.md was not found")
			return
		}
	}

	title := "SpecTest Markdown Document"
	sections := make([]string, 0)
	tableCount := 0
	lineCount := 0

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "# ") && title == "SpecTest Markdown Document" {
			title = strings.TrimPrefix(trimmed, "# ")
		} else if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			sections = append(sections, strings.TrimLeft(trimmed, "# "))
		} else if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
			if strings.Contains(trimmed, "---") {
				tableCount++
			}
		}
	}

	writeJSON(w, http.StatusOK, markdownReadResponse{
		Path:       path,
		Title:      title,
		Sections:   sections,
		TableCount: tableCount,
		LineCount:  lineCount,
		RawContent: content,
	})
}
