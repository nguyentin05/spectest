package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/llm"
	"github.com/nguyentin05/spectest/internal/infrastructure/sources"
	"github.com/nguyentin05/spectest/internal/infrastructure/storage"
	"github.com/nguyentin05/spectest/internal/infrastructure/suitefile"
	usecasegenerate "github.com/nguyentin05/spectest/internal/usecase/generate"
)

func (s *Server) registerUpdateTools() {
	// 1. plan_update
	s.mcpServer.AddTool(
		mcp.NewTool("plan_update",
			mcp.WithDescription("Preview changes between OpenAPI specs and the current SpecTest suite without writing files or calling LLMs."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("output_dir",
				mcp.Description("Directory containing spectest.yaml and spectest.log.yaml relative to server root (default: tests)"),
			),
			mcp.WithArray("apis",
				mcp.Required(),
				mcp.Description("List of API server and spec path mappings, e.g. [{\"server\": \"auth\", \"path\": \"openapi.yaml\"}]"),
			),
			mcp.WithString("requirements_path",
				mcp.Description("Optional path to markdown requirements file relative to server root"),
			),
		),
		s.handlePlanUpdate,
	)

	// 2. start_update
	s.mcpServer.AddTool(
		mcp.NewTool("start_update",
			mcp.WithDescription("Apply incremental updates to spectest.yaml and spectest.log.yaml, synthesizing repairs/scenarios via LLM if needed."),
			mcp.WithString("output_dir",
				mcp.Description("Directory containing spectest.yaml and spectest.log.yaml relative to server root (default: tests)"),
			),
			mcp.WithArray("apis",
				mcp.Required(),
				mcp.Description("List of API server and spec path mappings, e.g. [{\"server\": \"auth\", \"path\": \"openapi.yaml\"}]"),
			),
			mcp.WithString("requirements_path",
				mcp.Description("Optional path to markdown requirements file relative to server root"),
			),
		),
		s.handleStartUpdate,
	)
}

func (s *Server) parseUpdateInputs(req mcp.CallToolRequest) (string, []domain.APIInput, string, error) {
	outDirArg := req.GetString("output_dir", "tests")
	if strings.TrimSpace(outDirArg) == "" {
		outDirArg = "tests"
	}

	absOutDir, err := s.resolver.Resolve(outDirArg)
	if err != nil {
		return "", nil, "", fmt.Errorf("%s: %w", ErrCodePathOutsideRoot, err)
	}

	args := req.GetArguments()
	rawAPIs, ok := args["apis"].([]any)
	if !ok || len(rawAPIs) == 0 {
		return "", nil, "", fmt.Errorf("%s: 'apis' list must not be empty", ErrCodeInvalidInput)
	}

	apiInputs := make([]domain.APIInput, 0, len(rawAPIs))
	for i, rawItem := range rawAPIs {
		itemMap, ok := rawItem.(map[string]any)
		if !ok {
			return "", nil, "", fmt.Errorf("%s: apis[%d] must be an object with server and path", ErrCodeInvalidInput, i)
		}
		srv, _ := itemMap["server"].(string)
		p, _ := itemMap["path"].(string)
		if strings.TrimSpace(srv) == "" || strings.TrimSpace(p) == "" {
			return "", nil, "", fmt.Errorf("%s: apis[%d] server and path are required", ErrCodeInvalidInput, i)
		}

		absSpecPath, err := s.resolver.Resolve(strings.TrimSpace(p))
		if err != nil {
			return "", nil, "", fmt.Errorf("%s: apis[%d] %w", ErrCodePathOutsideRoot, i, err)
		}
		apiInputs = append(apiInputs, domain.APIInput{
			Server: strings.TrimSpace(srv),
			Path:   absSpecPath,
		})
	}

	var absReqPath string
	if raw := req.GetString("requirements_path", ""); strings.TrimSpace(raw) != "" {
		p, err := s.resolver.Resolve(raw)
		if err != nil {
			return "", nil, "", fmt.Errorf("%s: requirements_path %w", ErrCodePathOutsideRoot, err)
		}
		absReqPath = p
	}

	return absOutDir, apiInputs, absReqPath, nil
}

func (s *Server) handlePlanUpdate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	absOutDir, apiInputs, absReqPath, err := s.parseUpdateInputs(req)
	if err != nil {
		return formatParseError(err), nil
	}

	loader := sources.NewLoader()
	gen := usecasegenerate.NewGenerator(nil, loader, suitefile.NewStore(), realTimeClock{})

	res, err := gen.Execute(ctx, usecasegenerate.Request{
		Mode:             usecasegenerate.ModeUpdate,
		APIs:             apiInputs,
		RequirementsPath: absReqPath,
		OutputDir:        absOutDir,
		DryRun:           true,
		Concurrency:      1,
	})
	if err != nil {
		return mapUpdateError(err), nil
	}

	return mcp.NewToolResultJSON(map[string]any{
		"output_dir":      absOutDir,
		"diff":            res.APIDiff,
		"impact":          res.Impact,
		"planned_actions": res.PlannedActions,
		"dry_run":         true,
	})
}

func (s *Server) handleStartUpdate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	absOutDir, apiInputs, absReqPath, err := s.parseUpdateInputs(req)
	if err != nil {
		return formatParseError(err), nil
	}

	// Resolve LLM
	credStore := storage.NewFileCredentialStore()
	llmProv, _ := llm.NewProvider(credStore, "", "", "")

	// Acquire per-directory mutex
	unlock := s.jobs.LockKey(absOutDir)

	jobCtx, job := s.jobs.CreateJob(context.Background(), "update")

	go func() {
		defer unlock()
		job.Finish(JobRunning, nil, "")

		loader := sources.NewLoader()
		gen := usecasegenerate.NewGenerator(llmProv, loader, suitefile.NewStore(), realTimeClock{})

		res, err := gen.Execute(jobCtx, usecasegenerate.Request{
			Mode:             usecasegenerate.ModeUpdate,
			APIs:             apiInputs,
			RequirementsPath: absReqPath,
			OutputDir:        absOutDir,
			DryRun:           false,
			Concurrency:      1,
		})

		if err != nil {
			job.Finish(JobFailed, nil, err.Error())
			return
		}

		job.Finish(JobSucceeded, map[string]any{
			"output_dir":    absOutDir,
			"suite_changed": res.SuiteChanged,
			"update_n":      res.UpdateN,
			"llm_used":      res.LLMUsed,
			"diff":          res.APIDiff,
			"impact":        res.Impact,
		}, "")
	}()

	return mcp.NewToolResultJSON(map[string]any{
		"job_id":     job.ID,
		"status":     JobRunning,
		"started_at": job.StartedAt,
		"message":    fmt.Sprintf("Update job %s started in background", job.ID),
	})
}

type realTimeClock struct{}

func (realTimeClock) Now() time.Time { return time.Now() }

func formatParseError(err error) *mcp.CallToolResult {
	msg := err.Error()
	if strings.Contains(msg, ErrCodePathOutsideRoot) {
		return ToolError(ErrCodePathOutsideRoot, msg, false, nil)
	}
	return ToolError(ErrCodeInvalidInput, msg, false, nil)
}

func mapUpdateError(err error) *mcp.CallToolResult {
	msg := err.Error()
	if strings.Contains(msg, "history hash") || strings.Contains(msg, "hash mismatch") {
		return ToolError(ErrCodeIntegrityConflict, msg, false, nil)
	}
	if strings.Contains(msg, "locked") {
		return ToolError(ErrCodeLockedEntity, msg, false, nil)
	}
	if strings.Contains(msg, "LLM required") || strings.Contains(msg, "llm") {
		return ToolError(ErrCodeLLMRequired, msg, false, nil)
	}
	return ToolError(ErrCodeJobFailed, msg, false, nil)
}
