package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/suitefile"
)

type workspaceItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"` // "openapi", "prd", "suite", "matrix", "other"
	Size int64  `json:"size"`
}

type workspaceResponse struct {
	CurrentDir string          `json:"current_dir"`
	Items      []workspaceItem `json:"items"`
}

type uploadResponse struct {
	Success  bool   `json:"success"`
	FileName string `json:"file_name"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
}

// handleListWorkspace returns specs, PRD, and test suites in workspace.
func (s *Server) handleListWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workDir := s.workDir
	if workDir == "" {
		workDir = "."
	}
	absDir, _ := filepath.Abs(workDir)

	items := make([]workspaceItem, 0)
	_ = filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := info.Name()
			if strings.HasPrefix(base, ".") || base == "node_modules" || base == "vendor" || base == "dist" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		name := strings.ToLower(info.Name())

		itemType := detectFileType(name)
		if itemType != "other" || ext == ".yaml" || ext == ".yml" || ext == ".json" || ext == ".md" {
			rel, _ := filepath.Rel(absDir, path)
			items = append(items, workspaceItem{
				Name: rel,
				Path: path,
				Type: itemType,
				Size: info.Size(),
			})
		}
		return nil
	})

	writeJSON(w, http.StatusOK, workspaceResponse{
		CurrentDir: absDir,
		Items:      items,
	})
}

// handleUploadSpec handles uploading OpenAPI, PRD, or test suite files.
func (s *Server) handleUploadSpec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Try Multipart Form
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max
			writeError(w, http.StatusBadRequest, "failed to parse multipart form: "+err.Error())
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "missing 'file' field in multipart form")
			return
		}
		defer func() {
			_ = file.Close()
		}()

		baseName := filepath.Base(header.Filename)
		destDir := filepath.Join(s.workDir, ".spectest_ui_uploads")
		_ = os.MkdirAll(destDir, 0o755)
		destPath := filepath.Join(destDir, baseName)

		out, err := os.Create(destPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save uploaded file: "+err.Error())
			return
		}
		defer func() {
			_ = out.Close()
		}()

		size, err := io.Copy(out, file)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to write file: "+err.Error())
			return
		}

		s.sessionMgr.AppendLog("Uploaded file: " + baseName)
		writeJSON(w, http.StatusOK, uploadResponse{
			Success:  true,
			FileName: baseName,
			Path:     destPath,
			Type:     detectFileType(baseName),
			Size:     size,
		})
		return
	}

	// 2. Try JSON raw content
	type rawUploadRequest struct {
		FileName string `json:"file_name"`
		Content  string `json:"content"`
	}

	var req rawUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	name := strings.TrimSpace(req.FileName)
	if name == "" {
		writeError(w, http.StatusBadRequest, "file_name is required")
		return
	}

	destDir := filepath.Join(s.workDir, ".spectest_ui_uploads")
	_ = os.MkdirAll(destDir, 0o755)
	destPath := filepath.Join(destDir, filepath.Base(name))

	if err := os.WriteFile(destPath, []byte(req.Content), 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write file: "+err.Error())
		return
	}

	s.sessionMgr.AppendLog("Saved spec file: " + name)
	writeJSON(w, http.StatusOK, uploadResponse{
		Success:  true,
		FileName: name,
		Path:     destPath,
		Type:     detectFileType(name),
		Size:     int64(len(req.Content)),
	})
}

// handleGetPlan loads and returns the parsed v1 structure of spectest.yaml.
func (s *Server) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	suitePath := r.URL.Query().Get("path")
	if suitePath == "" {
		candidates := []string{
			"spectest.yaml",
			"spectest.yml",
			filepath.Join(s.workDir, "spectest.yaml"),
			filepath.Join(s.workDir, "spectest.yml"),
			filepath.Join(s.workDir, "tests", "spectest.yaml"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				suitePath = c
				break
			}
		}
	}

	if suitePath == "" {
		writeError(w, http.StatusNotFound, "no spectest.yaml file found in workspace; please specify ?path= or upload one")
		return
	}

	suite, err := suitefile.LoadSuite(suitePath)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to parse test suite %q: %v", suitePath, err))
		return
	}

	totalSteps := 0
	for _, sc := range suite.Scenarios {
		totalSteps += len(sc.Steps)
	}
	for _, st := range suite.Setup {
		totalSteps += len(st.Steps)
	}

	lockedCount := 0
	skippedCount := 0
	for _, sc := range suite.Scenarios {
		if sc.Locked {
			lockedCount++
		}
		if sc.Skip != "" {
			skippedCount++
		}
	}

	servers := suite.Servers
	if servers == nil {
		servers = []string{}
	}
	apis := suite.APIs
	if apis == nil {
		apis = []string{}
	}
	seed := suite.Seed
	if seed == nil {
		seed = map[string]domain.Seed{}
	}
	setup := suite.Setup
	if setup == nil {
		setup = map[string]domain.Setup{}
	}
	scenarios := suite.Scenarios
	if scenarios == nil {
		scenarios = []domain.Scenario{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"path":      suitePath,
		"version":   suite.Version,
		"servers":   servers,
		"apis":      apis,
		"analysis":  suite.Analysis,
		"seed":      seed,
		"setup":     setup,
		"scenarios": scenarios,
		"stats": map[string]any{
			"total_scenarios":   len(scenarios),
			"total_setups":      len(setup),
			"total_steps":       totalSteps,
			"locked_scenarios":  lockedCount,
			"skipped_scenarios": skippedCount,
		},
	})
}

// handleGetHistory loads and returns the evolution history from spectest.log.yaml.
func (s *Server) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logPath := r.URL.Query().Get("path")
	if logPath == "" {
		candidates := []string{
			"spectest.log.yaml",
			"spectest.log.yml",
			filepath.Join(s.workDir, "spectest.log.yaml"),
			filepath.Join(s.workDir, "spectest.log.yml"),
			filepath.Join(s.workDir, "tests", "spectest.log.yaml"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				logPath = c
				break
			}
		}
	}

	if logPath == "" {
		writeError(w, http.StatusNotFound, "no spectest.log.yaml file found in workspace")
		return
	}

	logFile, err := suitefile.LoadLog(logPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to parse log file %q: %v", logPath, err))
		return
	}

	servers := []string{}
	apisCount := 0
	scenariosCount := 0
	if logFile.Base.Spectest != nil {
		if logFile.Base.Spectest.Servers != nil {
			servers = logFile.Base.Spectest.Servers
		}
		apisCount = len(logFile.Base.Spectest.APIs)
		scenariosCount = len(logFile.Base.Spectest.Scenarios)
	}

	baseMap := map[string]any{
		"at":         logFile.Base.At,
		"by":         logFile.Base.By,
		"after_hash": logFile.Base.AfterHash,
		"spectest":   logFile.Base.Spectest,
		"spectest_summary": map[string]any{
			"servers":         servers,
			"apis_count":      apisCount,
			"scenarios_count": scenariosCount,
		},
	}

	updatesList := make([]map[string]any, 0, len(logFile.Update))
	for _, upd := range logFile.Update {
		setupAdded := []string{}
		scenariosAdded := []string{}
		if upd.Create != nil {
			for k := range upd.Create.Setup {
				setupAdded = append(setupAdded, k)
			}
			sort.Strings(setupAdded)
			for _, sc := range upd.Create.Scenarios {
				scenariosAdded = append(scenariosAdded, sc.ID)
			}
		}

		var triggerData any = upd.Trigger
		if triggerData == nil {
			triggerData = map[string]any{}
		}

		changes := upd.Change
		if changes == nil {
			changes = []domain.Change{}
		}

		updatesList = append(updatesList, map[string]any{
			"n":       upd.N,
			"at":      upd.At,
			"by":      upd.By,
			"note":    upd.Note,
			"trigger": triggerData,
			"create": map[string]any{
				"setup_added":     setupAdded,
				"scenarios_added": scenariosAdded,
			},
			"change":     changes,
			"after_hash": upd.AfterHash,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"path":    logPath,
		"base":    baseMap,
		"updates": updatesList,
	})
}

func detectFileType(name string) string {
	lower := strings.ToLower(name)
	ext := filepath.Ext(lower)
	if strings.Contains(lower, "openapi") || strings.Contains(lower, "swagger") {
		return "openapi"
	}
	if strings.Contains(lower, "prd") || (ext == ".md" && !strings.Contains(lower, "matrix")) {
		return "prd"
	}
	if strings.Contains(lower, "spectest.log") {
		return "history"
	}
	if strings.Contains(lower, "spectest") || ext == ".yaml" || ext == ".yml" {
		return "suite"
	}
	if strings.Contains(lower, "matrix") {
		return "matrix"
	}
	return "other"
}
