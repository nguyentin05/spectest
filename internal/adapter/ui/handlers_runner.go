package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/server"
	"github.com/nguyentin05/spectest/internal/infrastructure/suitefile"
	ucrun "github.com/nguyentin05/spectest/internal/usecase/run"
)

type runStartRequest struct {
	TargetURL   string            `json:"target_url"`
	Targets     map[string]string `json:"targets"`
	SuitePath   string            `json:"suite_path"`
	Timeout     int               `json:"timeout_seconds"`
	Triage      bool              `json:"triage"`
	OnlyChanged bool              `json:"only_changed"`
	LogPath     string            `json:"log_path"`
}

type runStartResponse struct {
	RunID     string `json:"run_id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	TargetURL string `json:"target_url"`
	SuitePath string `json:"suite_path"`
}

// handleRunStart triggers a test execution session in background.
func (s *Server) handleRunStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req runStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	suitePath := strings.TrimSpace(req.SuitePath)
	if suitePath == "" {
		candidates := []string{
			"spectest.yaml",
			"spectest.yml",
			"tests/spectest.yaml",
		}
		for _, c := range candidates {
			if _, err := suitefile.LoadSuite(c); err == nil {
				suitePath = c
				break
			}
		}
		if suitePath == "" {
			suitePath = "spectest.yaml"
		}
	}

	suite, err := suitefile.LoadSuite(suitePath)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("cannot load suite at %q: %v", suitePath, err))
		return
	}

	targets := make(map[string]string)
	for k, v := range req.Targets {
		alias := strings.TrimSpace(k)
		if alias == "" {
			writeError(w, http.StatusBadRequest, "target alias cannot be empty")
			return
		}
		rawURL := strings.TrimSpace(v)
		if rawURL == "" {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("target URL for alias %q cannot be empty", alias))
			return
		}
		normURL, err := server.NormalizeTargetURL(rawURL)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid target URL %q for alias %q: %v", v, alias, err))
			return
		}
		targets[alias] = normURL
	}

	defaultTarget := ""
	if req.TargetURL != "" {
		normURL, err := server.NormalizeTargetURL(req.TargetURL)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid default target URL %q: %v", req.TargetURL, err))
			return
		}
		defaultTarget = normURL
	}

	if !req.OnlyChanged && defaultTarget == "" && len(targets) == 0 {
		writeError(w, http.StatusBadRequest, "target URL is required: specify 'target_url' or 'targets' map")
		return
	}

	runID := fmt.Sprintf("run-%x", time.Now().UnixNano()%0x100000000)
	ctx, cancel := context.WithCancel(context.Background())

	sess, err := s.sessionMgr.StartSession(runID, defaultTarget, suitePath, cancel)
	if err != nil {
		cancel()
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	sess.InitReport(targets, 0, 0)
	s.sessionMgr.AppendLog(fmt.Sprintf("[%s] Test run scheduled against %s", runID, defaultTarget))

	// Launch async execution using unified usecase runner
	go s.executeTestSuite(ctx, sess, suite, suitePath, targets, defaultTarget, req.Timeout, req.Triage, req.OnlyChanged, req.LogPath)

	writeJSON(w, http.StatusAccepted, runStartResponse{
		RunID:     runID,
		Status:    "running",
		Message:   "Test execution started",
		TargetURL: defaultTarget,
		SuitePath: suitePath,
	})
}

// handleRunStream provides Server-Sent Events (SSE) for real-time progress.
func (s *Server) handleRunStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sess := s.sessionMgr.ActiveSession()
	if sess == nil {
		raw, _ := json.Marshal(RunEvent{
			Type:      "idle",
			Timestamp: time.Now(),
			Message:   "No active test run is in progress",
		})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", raw)
		flusher.Flush()
		return
	}

	events := sess.Subscribe()
	defer sess.Unsubscribe(events)

	// Send initial status event
	initEvt, _ := json.Marshal(RunEvent{
		Type:      "connected",
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("Connected to test run %s (status: %s)", sess.ID, sess.Status),
		Data: map[string]any{
			"run_id":     sess.ID,
			"target_url": sess.TargetURL,
			"suite_path": sess.SuitePath,
		},
	})
	_, _ = fmt.Fprintf(w, "data: %s\n\n", initEvt)
	flusher.Flush()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case evt, ok := <-events:
			if !ok {
				return
			}
			raw, err := json.Marshal(evt)
			if err == nil {
				_, _ = fmt.Fprintf(w, "data: %s\n\n", raw)
				flusher.Flush()
			}
			if evt.Type == "complete" || evt.Type == "cancelled" || evt.Type == "error" {
				return
			}
		}
	}
}

// handleRunCancel aborts the active run session.
func (s *Server) handleRunCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cancelled := s.sessionMgr.CancelActive()
	if !cancelled {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"message": "no active test run is in progress",
		})
		return
	}

	s.sessionMgr.AppendLog("Active test run was cancelled by user")
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "test run cancelled successfully",
	})
}

// handleRunReport returns the report of the latest test run.
func (s *Server) handleRunReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rep := s.sessionMgr.LastReport()
	if rep == nil {
		writeError(w, http.StatusNotFound, "no test runs recorded yet")
		return
	}

	writeJSON(w, http.StatusOK, rep)
}

// executeTestSuite executes the v1 Suite by delegating to the single usecase Runner and mapping events to SSE.
func (s *Server) executeTestSuite(
	ctx context.Context,
	sess *RunSession,
	suite *domain.Suite,
	suitePath string,
	targets map[string]string,
	defaultBaseURL string,
	timeoutSec int,
	enableTriage bool,
	onlyChanged bool,
	logPath string,
) {
	var llmSvc domain.LLMService
	if s.factory != nil {
		llmSvc, _ = s.factory.LLMService()
	}

	clientTimeout := 30 * time.Second
	if timeoutSec > 0 {
		clientTimeout = time.Duration(timeoutSec) * time.Second
	}

	runner := ucrun.NewRunner(ucrun.Dependencies{
		SuiteLoader: suitefile.LoadSuite,
		LogLoader:   suitefile.LoadLog,
		LifecycleFactory: func(composeFile, targetURL string, timeoutSec int) domain.ServerLifecycleManager {
			return server.NewLifecycleManager(composeFile, targetURL, timeoutSec)
		},
		Probe:      server.WaitForReady,
		HTTPClient: &http.Client{Timeout: clientTimeout},
		LLM:        llmSvc,
	})

	summary, err := runner.Execute(ctx, ucrun.Options{
		Suite:        suite,
		SuitePath:    suitePath,
		OnlyChanged:  onlyChanged,
		LogPath:      logPath,
		TargetURL:    defaultBaseURL,
		Targets:      targets,
		TimeoutSec:   timeoutSec,
		EnableTriage: enableTriage && llmSvc != nil,
		OnEvent: func(evt ucrun.RunEvent) {
			switch evt.Type {
			case ucrun.EventSuiteStart:
				sess.InitReport(targets, 0, evt.TotalScenarios)
				sess.SetSelection(evt.SelectionMode, evt.UpdateN)
				sess.Broadcast(RunEvent{
					Type:      "start",
					Timestamp: time.Now(),
					Message:   fmt.Sprintf("Beginning execution of suite (version %d) with %d scenarios", suite.Version, evt.TotalScenarios),
					Data: map[string]any{
						"total_scenarios":    evt.TotalScenarios,
						"servers":            suite.Servers,
						"targets":            targets,
						"selection_mode":     evt.SelectionMode,
						"update_n":           evt.UpdateN,
						"selected_scenarios": evt.SelectedScenarios,
					},
				})
			case ucrun.EventScenarioStart:
				sess.Broadcast(RunEvent{
					Type:      "scenario_start",
					Timestamp: time.Now(),
					Message:   fmt.Sprintf("Executing scenario: %s (%s)", evt.ScenarioName, evt.ScenarioID),
					Data: map[string]any{
						"scenario_id":   evt.ScenarioID,
						"scenario_name": evt.ScenarioName,
					},
				})
			case ucrun.EventStepResult:
				status := "PASS"
				if !evt.Passed {
					status = "FAIL"
				}
				caseRes := CaseResult{
					ScenarioID: evt.ScenarioID,
					Call:       evt.StepCall,
					Method:     evt.Method,
					URL:        evt.URL,
					Status:     status,
					HTTPStatus: evt.StatusCode,
					Expected:   evt.ExpectedCode,
					LatencyMs:  evt.LatencyMs,
					Error:      evt.FailReason,
					SavedVars:  evt.Extracted,
				}
				sess.RecordResult(caseRes)
				sess.Broadcast(RunEvent{
					Type:      "step_result",
					Timestamp: time.Now(),
					Data: map[string]any{
						"scenario_id":  evt.ScenarioID,
						"setup":        evt.SetupName,
						"call":         evt.StepCall,
						"method":       evt.Method,
						"url":          evt.URL,
						"status_code":  evt.StatusCode,
						"expect":       evt.ExpectedCode,
						"latency_ms":   evt.LatencyMs,
						"passed":       evt.Passed,
						"error":        evt.FailReason,
						"saved_vars":   evt.Extracted,
						"ai_diagnosis": evt.AIDiagnosis,
					},
				})
			case ucrun.EventScenarioComplete:
				sess.Broadcast(RunEvent{
					Type:      "scenario_complete",
					Timestamp: time.Now(),
					Data: map[string]any{
						"scenario_id": evt.ScenarioID,
						"passed":      evt.Passed,
						"skipped":     evt.Skipped,
						"reason":      evt.SkipReason,
					},
				})
			}
		},
	})

	if summary != nil {
		sess.SetSelection(summary.SelectionMode, summary.UpdateN)
		sess.UpdateMetrics(
			summary.TotalScenarios,
			summary.PassedScenarios,
			summary.FailedScenarios,
			summary.SkippedScenarios,
			summary.TotalChecks,
			summary.PassedChecks,
			summary.FailedChecks,
			summary.TotalDuration.Milliseconds(),
		)
	}

	finalStatus := "passed"
	currentReport := sess.SnapshotReport()
	if ctx.Err() != nil {
		finalStatus = "cancelled"
		sess.Broadcast(RunEvent{
			Type:      "cancelled",
			Timestamp: time.Now(),
			Message:   "Test run was cancelled by user",
		})
	} else if err != nil {
		finalStatus = "failed"
		sess.Broadcast(RunEvent{
			Type:      "error",
			Timestamp: time.Now(),
			Message:   err.Error(),
		})
	} else if currentReport.FailedScenarios > 0 || currentReport.Failed > 0 {
		finalStatus = "failed"
		sess.Broadcast(RunEvent{
			Type:      "complete",
			Timestamp: time.Now(),
			Message: fmt.Sprintf("Run finished: %d passed, %d failed",
				currentReport.PassedScenarios, currentReport.FailedScenarios),
			Data: map[string]any{"report": currentReport},
		})
	} else {
		sess.Broadcast(RunEvent{
			Type:      "complete",
			Timestamp: time.Now(),
			Message: fmt.Sprintf("Run finished: all %d scenarios passed",
				currentReport.PassedScenarios),
			Data: map[string]any{"report": currentReport},
		})
	}

	sess.Finish(finalStatus)
	finishedReport := sess.SnapshotReport()
	s.sessionMgr.SetLastReport(finishedReport)
	s.sessionMgr.AppendLog(fmt.Sprintf("[%s] Suite run finished: %s (%d/%d scenarios passed)",
		sess.ID, finalStatus, finishedReport.PassedScenarios, finishedReport.TotalScenarios))
}
