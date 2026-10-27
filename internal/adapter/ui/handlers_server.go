package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nguyentin05/spectest/internal/infrastructure/server"
)

type healthcheckRequest struct {
	TargetURL      string            `json:"target_url"`
	Targets        map[string]string `json:"targets,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	Mode           string            `json:"mode"`         // "existing" or "compose"
	ComposeFile    string            `json:"compose_file"` // path to docker-compose file if mode == compose
	ServiceName    string            `json:"service_name"` // compose service name
}

type healthcheckResponse struct {
	Ready       bool              `json:"ready"`
	TargetURL   string            `json:"target_url"`
	Targets     map[string]string `json:"targets,omitempty"`
	LatencyMs   int64             `json:"latency_ms"`
	StatusCode  int               `json:"status_code,omitempty"`
	Message     string            `json:"message"`
	ErrorDetail string            `json:"error,omitempty"`
}

// handleHealthCheck probes the target server or manages lifecycle.
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req healthcheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	rawURL := strings.TrimSpace(req.TargetURL)
	if rawURL == "" && len(req.Targets) > 0 {
		for _, u := range req.Targets {
			if strings.TrimSpace(u) != "" {
				rawURL = strings.TrimSpace(u)
				break
			}
		}
	}
	if rawURL == "" {
		writeError(w, http.StatusBadRequest, "target_url or targets map is required")
		return
	}

	timeoutSec := req.TimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = 5
	}
	if timeoutSec > 60 {
		timeoutSec = 60
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	s.sessionMgr.AppendLog(fmt.Sprintf("Probing target server: %s (timeout: %ds)", rawURL, timeoutSec))

	// Check server readiness
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	composeFile := ""
	if mode == "compose" {
		rawCompose := strings.TrimSpace(req.ComposeFile)
		if rawCompose == "" {
			rawCompose = "docker-compose.yml"
		}
		safeCompose, err := s.safeFilePath(rawCompose)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid compose file: %v", err))
			return
		}
		composeFile = safeCompose
	}

	lifecycle := server.NewLifecycleManager(composeFile, rawURL, timeoutSec)
	teardown, err := lifecycle.Up(ctx)
	dur := time.Since(start).Milliseconds()
	if teardown != nil {
		defer teardown()
	}

	if err != nil {
		s.sessionMgr.AppendLog(fmt.Sprintf("Target server at %s is NOT ready: %v", rawURL, err))
		writeJSON(w, http.StatusOK, healthcheckResponse{
			Ready:       false,
			TargetURL:   rawURL,
			LatencyMs:   dur,
			Message:     "Target server is unreachable or returning gateway errors",
			ErrorDetail: err.Error(),
		})
		return
	}

	s.sessionMgr.AppendLog(fmt.Sprintf("Target server at %s is ready (%dms)", rawURL, dur))
	writeJSON(w, http.StatusOK, healthcheckResponse{
		Ready:      true,
		TargetURL:  rawURL,
		LatencyMs:  dur,
		StatusCode: 200,
		Message:    "Target server is healthy and ready to accept requests",
	})
}
