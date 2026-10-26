package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type apiInputItem struct {
	Server string `json:"server"`
	Path   string `json:"path"`
}

type generateRequest struct {
	APIs      []apiInputItem `json:"apis"`
	SpecPath  string         `json:"spec_path,omitempty"` // legacy fallback
	ReqPath   string         `json:"req_path"`
	PRDPath   string         `json:"prd_path,omitempty"`  // legacy fallback
	OutputDir string         `json:"output_dir"`
	DryRun    bool           `json:"dry_run"`
}

type generateResponse struct {
	Success            bool     `json:"success"`
	Status             string   `json:"status"`
	SuitePath          string   `json:"suite_path,omitempty"`
	LogPath            string   `json:"log_path,omitempty"`
	Servers            []string `json:"servers,omitempty"`
	APIsCount          int      `json:"apis_count,omitempty"`
	ScenariosGenerated int      `json:"scenarios_generated,omitempty"`
	Message            string   `json:"message"`
}

// handleGenerate handles requests to synthesize a test suite from OpenAPI specs and requirements.
func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req generateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Normalize inputs for both legacy single-file and v2 multi-server formats
	if len(req.APIs) == 0 && req.SpecPath != "" {
		req.APIs = []apiInputItem{{Server: "default", Path: strings.TrimSpace(req.SpecPath)}}
	}
	if req.ReqPath == "" {
		req.ReqPath = strings.TrimSpace(req.PRDPath)
	}
	if req.OutputDir == "" {
		req.OutputDir = "./tests"
	}

	if len(req.APIs) == 0 {
		writeError(w, http.StatusBadRequest, "at least one API specification is required in 'apis'")
		return
	}

	servers := make([]string, 0, len(req.APIs))
	for _, api := range req.APIs {
		serverName := strings.TrimSpace(api.Server)
		filePath := strings.TrimSpace(api.Path)
		if serverName == "" || filePath == "" {
			writeError(w, http.StatusBadRequest, "each API item must have non-empty 'server' and 'path'")
			return
		}
		resolvedPath := filePath
		if _, err := os.Stat(resolvedPath); err != nil && !filepath.IsAbs(resolvedPath) && s.workDir != "" {
			resolvedPath = filepath.Join(s.workDir, filePath)
		}
		if _, err := os.Stat(resolvedPath); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("API spec file not found for server %q: %s", serverName, filePath))
			return
		}
		servers = append(servers, serverName)
	}

	if req.ReqPath != "" {
		reqPath := strings.TrimSpace(req.ReqPath)
		if _, err := os.Stat(reqPath); err != nil && !filepath.IsAbs(reqPath) && s.workDir != "" {
			reqPath = filepath.Join(s.workDir, reqPath)
		}
		if _, err := os.Stat(reqPath); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("requirements file not found: %s", req.ReqPath))
			return
		}
	}

	s.sessionMgr.AppendLog(fmt.Sprintf("Generate triggered for servers [%s], output dir: %s",
		strings.Join(servers, ", "), req.OutputDir))

	writeJSON(w, http.StatusAccepted, generateResponse{
		Success:   true,
		Status:    "pending_synthesis",
		SuitePath: req.OutputDir + "/spectest.yaml",
		LogPath:   req.OutputDir + "/spectest.log.yaml",
		Servers:   servers,
		Message:   "Specification inputs validated. Synthesis pipeline v2 is being wired by core team.",
	})
}
