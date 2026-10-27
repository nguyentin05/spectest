package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/llm"
	"github.com/nguyentin05/spectest/internal/infrastructure/server"
	"github.com/nguyentin05/spectest/internal/infrastructure/storage"
	"github.com/nguyentin05/spectest/internal/infrastructure/suitefile"
	"github.com/nguyentin05/spectest/internal/platform/atomicfile"
	ucrun "github.com/nguyentin05/spectest/internal/usecase/run"
)

func (s *Server) registerRunTools() {
	// 1. start_run
	s.mcpServer.AddTool(
		mcp.NewTool("start_run",
			mcp.WithDescription("Start executing a SpecTest test suite asynchronously against one or more targets."),
			mcp.WithString("suite_path",
				mcp.Description("Path to spectest.yaml file relative to server root (default: tests/spectest.yaml)"),
			),
			mcp.WithString("mode",
				mcp.Description("Execution mode: 'only_changed' to test only affected scenarios, or 'full' (default: only_changed)"),
			),
			mcp.WithObject("targets",
				mcp.Description("Map of server alias to target base URL, e.g. {\"auth\": \"http://localhost:8000\"}"),
			),
			mcp.WithString("compose_file",
				mcp.Description("Path to docker-compose.yml file relative to server root for service lifecycle management"),
			),
			mcp.WithBoolean("triage",
				mcp.Description("Enable LLM-assisted failure triage diagnosis for failed test steps (default: false)"),
			),
			mcp.WithString("report_dir",
				mcp.Description("Directory where report.json will be saved relative to server root (default: reports)"),
			),
			mcp.WithNumber("wait_timeout_seconds",
				mcp.Description("If > 0, wait synchronously up to this number of seconds before backgrounding (default: 0)"),
			),
		),
		s.handleStartRun,
	)

	// 2. get_job
	s.mcpServer.AddTool(
		mcp.NewTool("get_job",
			mcp.WithDescription("Poll status, progress events, and results of a background test run or update job."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("job_id",
				mcp.Required(),
				mcp.Description("ID of the job to inspect"),
			),
		),
		s.handleGetJob,
	)

	// 3. cancel_job
	s.mcpServer.AddTool(
		mcp.NewTool("cancel_job",
			mcp.WithDescription("Cancel an in-progress test execution or update job."),
			mcp.WithString("job_id",
				mcp.Required(),
				mcp.Description("ID of the job to cancel"),
			),
		),
		s.handleCancelJob,
	)

	// 4. read_report
	s.mcpServer.AddTool(
		mcp.NewTool("read_report",
			mcp.WithDescription("Read and summarize a test execution report with sensitive credentials redacted."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("report_path",
				mcp.Description("Path to report.json file relative to server root (default: reports/report.json)"),
			),
			mcp.WithString("job_id",
				mcp.Description("Optional job ID from which to derive the report directory"),
			),
		),
		s.handleReadReport,
	)
}

func (s *Server) handleStartRun(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	suiteArg := req.GetString("suite_path", "tests/spectest.yaml")
	if strings.TrimSpace(suiteArg) == "" {
		suiteArg = "tests/spectest.yaml"
	}

	absSuitePath, err := s.resolver.Resolve(suiteArg)
	if err != nil {
		return ToolError(ErrCodePathOutsideRoot, err.Error(), false, nil), nil
	}

	if !fileExists(absSuitePath) {
		return ToolError(ErrCodeNotFound, fmt.Sprintf("suite file %q not found", suiteArg), false, nil), nil
	}

	modeArg := req.GetString("mode", "only_changed")
	onlyChanged := modeArg != "full"

	var composePath string
	if raw := req.GetString("compose_file", ""); strings.TrimSpace(raw) != "" {
		cPath, err := s.resolver.Resolve(raw)
		if err != nil {
			return ToolError(ErrCodePathOutsideRoot, err.Error(), false, nil), nil
		}
		composePath = cPath
	}

	reportDirArg := req.GetString("report_dir", "reports")
	if strings.TrimSpace(reportDirArg) == "" {
		reportDirArg = "reports"
	}
	absReportDir, err := s.resolver.Resolve(reportDirArg)
	if err != nil {
		return ToolError(ErrCodePathOutsideRoot, err.Error(), false, nil), nil
	}

	targets := make(map[string]string)
	args := req.GetArguments()
	if rawTargets, ok := args["targets"].(map[string]any); ok {
		for k, v := range rawTargets {
			if strVal, ok := v.(string); ok {
				targets[k] = strVal
			}
		}
	}

	triage := req.GetBool("triage", false)
	waitTimeout := int(req.GetFloat("wait_timeout_seconds", 0))

	// Create asynchronous job
	jobCtx, job := s.jobs.CreateJob(context.Background(), "run")

	// Determine log path companion
	logPath := filepath.Join(filepath.Dir(absSuitePath), "spectest.log.yaml")

	// Assemble canonical runner
	var llmSvc domain.LLMService
	if triage {
		credStore := storage.NewFileCredentialStore()
		if prov, err := llm.NewProvider(credStore, "", "", ""); err == nil {
			llmSvc = prov
		}
	}

	runner := ucrun.NewRunner(ucrun.Dependencies{
		SuiteLoader: suitefile.LoadSuite,
		LogLoader:   suitefile.LoadLog,
		LifecycleFactory: func(composeFile, targetURL string, timeoutSec int) domain.ServerLifecycleManager {
			return server.NewLifecycleManager(composeFile, targetURL, timeoutSec)
		},
		Probe:      server.WaitForReady,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		LLM:        llmSvc,
	})

	doneCh := make(chan struct{})

	go func() {
		defer close(doneCh)
		job.Finish(JobRunning, nil, "")

		summary, runErr := runner.Execute(jobCtx, ucrun.Options{
			SuitePath:    absSuitePath,
			Targets:      targets,
			ComposeFile:  composePath,
			OnlyChanged:  onlyChanged,
			LogPath:      logPath,
			EnableTriage: triage,
			OnEvent: func(evt ucrun.RunEvent) {
				job.AddEvent(evt)
			},
		})

		if runErr != nil {
			job.Finish(JobFailed, summary, runErr.Error())
			return
		}

		// Write report file atomically if report dir is specified
		if absReportDir != "" && summary != nil {
			_ = os.MkdirAll(absReportDir, 0o700)
			if reportData, err := json.MarshalIndent(summary, "", "  "); err == nil {
				_ = atomicfile.Write(filepath.Join(absReportDir, "report.json"), reportData, 0o600)
			}
		}

		if summary != nil && summary.FailedScenarios > 0 {
			job.Finish(JobFailed, summary, fmt.Sprintf("%d scenario(s) failed", summary.FailedScenarios))
		} else {
			job.Finish(JobSucceeded, summary, "")
		}
	}()

	// If wait_timeout_seconds specified, wait synchronously
	if waitTimeout > 0 {
		select {
		case <-doneCh:
			snap := job.Snapshot()
			return mcp.NewToolResultJSON(snap)
		case <-time.After(time.Duration(waitTimeout) * time.Second):
			// Timeout reached, return background job reference
		}
	}

	return mcp.NewToolResultJSON(map[string]any{
		"job_id":     job.ID,
		"status":     JobRunning,
		"started_at": job.StartedAt,
		"message":    fmt.Sprintf("Run job %s dispatched in background", job.ID),
	})
}

func (s *Server) handleGetJob(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jobID := req.GetString("job_id", "")
	if strings.TrimSpace(jobID) == "" {
		return ToolError(ErrCodeInvalidInput, "missing required job_id", false, nil), nil
	}

	job, found := s.jobs.GetJob(strings.TrimSpace(jobID))
	if !found {
		return ToolError(ErrCodeJobNotFound, fmt.Sprintf("job %q not found", jobID), false, nil), nil
	}

	return mcp.NewToolResultJSON(job)
}

func (s *Server) handleCancelJob(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jobID := req.GetString("job_id", "")
	if strings.TrimSpace(jobID) == "" {
		return ToolError(ErrCodeInvalidInput, "missing required job_id", false, nil), nil
	}

	cancelled := s.jobs.CancelJob(strings.TrimSpace(jobID))
	return mcp.NewToolResultJSON(map[string]any{
		"job_id":    jobID,
		"cancelled": cancelled,
	})
}

func (s *Server) handleReadReport(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	reportPathArg := req.GetString("report_path", "reports/report.json")
	if strings.TrimSpace(reportPathArg) == "" {
		reportPathArg = "reports/report.json"
	}

	absReportPath, err := s.resolver.Resolve(reportPathArg)
	if err != nil {
		return ToolError(ErrCodePathOutsideRoot, err.Error(), false, nil), nil
	}

	data, err := os.ReadFile(absReportPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ToolError(ErrCodeNotFound, fmt.Sprintf("report file %q not found", reportPathArg), false, nil), nil
		}
		return ToolError(ErrCodeInvalidInput, fmt.Sprintf("read report file: %v", err), false, nil), nil
	}

	redactedJSON := ucrun.RedactPayload(string(data))
	var summary map[string]any
	if err := json.Unmarshal([]byte(redactedJSON), &summary); err != nil {
		return ToolError(ErrCodeInvalidInput, fmt.Sprintf("parse report json: %v", err), false, nil), nil
	}

	relPath, _ := filepath.Rel(s.resolver.Root(), absReportPath)
	return mcp.NewToolResultJSON(map[string]any{
		"report_path": relPath,
		"report":      summary,
	})
}
