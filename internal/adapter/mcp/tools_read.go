package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/llm"
	"github.com/nguyentin05/spectest/internal/infrastructure/storage"
	"github.com/nguyentin05/spectest/internal/infrastructure/suitefile"
	usecasegenerate "github.com/nguyentin05/spectest/internal/usecase/generate"
)

type StatusResult struct {
	Root         string `json:"root"`
	OutputDir    string `json:"output_dir"`
	SuiteExists  bool   `json:"suite_exists"`
	SuitePath    string `json:"suite_path,omitempty"`
	LogExists    bool   `json:"log_exists"`
	LogPath      string `json:"log_path,omitempty"`
	LLMAvailable bool   `json:"llm_available"`
	ActiveModel  string `json:"active_model,omitempty"`
}

func (s *Server) registerReadTools() {
	// 1. spectest_status
	s.mcpServer.AddTool(
		mcp.NewTool("spectest_status",
			mcp.WithDescription("Check current SpecTest suite existence, audit log existence, and LLM capability availability in the workspace."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("output_dir",
				mcp.Description("Directory where test suite files are located, relative to server root (default: tests)"),
			),
		),
		s.handleStatus,
	)

	// 2. inspect_suite
	s.mcpServer.AddTool(
		mcp.NewTool("inspect_suite",
			mcp.WithDescription("Read and inspect canonical SpecTest suite details: hash, servers, APIs, scenarios, and latest audit history."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("suite_path",
				mcp.Description("Path to spectest.yaml file relative to server root (default: tests/spectest.yaml)"),
			),
		),
		s.handleInspectSuite,
	)

	// 3. validate_suite
	s.mcpServer.AddTool(
		mcp.NewTool("validate_suite",
			mcp.WithDescription("Validate the syntax, semantic references, and DAG dependencies of a SpecTest suite."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("suite_path",
				mcp.Description("Path to spectest.yaml file relative to server root (default: tests/spectest.yaml)"),
			),
		),
		s.handleValidateSuite,
	)
}

func (s *Server) handleStatus(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	outputDirArg := req.GetString("output_dir", "tests")
	if strings.TrimSpace(outputDirArg) == "" {
		outputDirArg = "tests"
	}

	targetDir, err := s.resolver.Resolve(outputDirArg)
	if err != nil {
		return ToolError(ErrCodePathOutsideRoot, err.Error(), false, nil), nil
	}

	suiteFile := filepath.Join(targetDir, "spectest.yaml")
	logFile := filepath.Join(targetDir, "spectest.log.yaml")

	suiteExists := fileExists(suiteFile)
	logExists := fileExists(logFile)

	var relSuite, relLog string
	if suiteExists {
		relSuite, _ = filepath.Rel(s.resolver.Root(), suiteFile)
	}
	if logExists {
		relLog, _ = filepath.Rel(s.resolver.Root(), logFile)
	}

	// Check LLM
	credStore := storage.NewFileCredentialStore()
	llmProv, err := llm.NewProvider(credStore, "", "", "")
	llmAvailable := err == nil && llmProv != nil
	activeModel := ""
	if llmAvailable {
		activeModel = llmProv.Name()
	}

	res := StatusResult{
		Root:         s.resolver.Root(),
		OutputDir:    outputDirArg,
		SuiteExists:  suiteExists,
		SuitePath:    relSuite,
		LogExists:    logExists,
		LogPath:      relLog,
		LLMAvailable: llmAvailable,
		ActiveModel:  activeModel,
	}
	return mcp.NewToolResultJSON(res)
}

type InspectResult struct {
	SuitePath      string                   `json:"suite_path"`
	SuiteHash      string                   `json:"suite_hash"`
	Version        int                      `json:"version"`
	Input          string                   `json:"input"`
	Servers        []string                 `json:"servers"`
	APICount       int                      `json:"api_count"`
	APIs           []string                 `json:"apis"`
	ScenarioCount  int                      `json:"scenario_count"`
	Scenarios      []InspectScenarioSummary `json:"scenarios"`
	SetupCount     int                      `json:"setup_count"`
	LatestUpdateN  int                      `json:"latest_update_n,omitempty"`
	LatestLogHash  string                   `json:"latest_log_hash,omitempty"`
	IntegrityMatch bool                     `json:"integrity_match"`
}

type InspectScenarioSummary struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Setup []string `json:"setup,omitempty"`
	Steps int      `json:"steps"`
	Skip  string   `json:"skip,omitempty"`
}

func (s *Server) handleInspectSuite(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	suiteArg := req.GetString("suite_path", "tests/spectest.yaml")
	if strings.TrimSpace(suiteArg) == "" {
		suiteArg = "tests/spectest.yaml"
	}

	absSuitePath, err := s.resolver.Resolve(suiteArg)
	if err != nil {
		return ToolError(ErrCodePathOutsideRoot, err.Error(), false, nil), nil
	}

	suite, err := suitefile.LoadSuite(absSuitePath)
	if err != nil {
		if os.IsNotExist(err) {
			return ToolError(ErrCodeNotFound, fmt.Sprintf("suite file %q not found", suiteArg), false, nil), nil
		}
		return ToolError(ErrCodeInvalidInput, fmt.Sprintf("failed to load suite: %v", err), false, nil), nil
	}

	currentHash := domain.SuiteHash(suite)
	scSummaries := make([]InspectScenarioSummary, 0, len(suite.Scenarios))
	for _, sc := range suite.Scenarios {
		scSummaries = append(scSummaries, InspectScenarioSummary{
			ID:    sc.ID,
			Name:  sc.Name,
			Setup: sc.Setup,
			Steps: len(sc.Steps),
			Skip:  sc.Skip,
		})
	}

	// Try loading log file companion
	dir := filepath.Dir(absSuitePath)
	logPath := filepath.Join(dir, "spectest.log.yaml")
	latestN := 0
	latestLogHash := ""
	integrityMatch := true

	if fileExists(logPath) {
		if logFile, err := suitefile.LoadLog(logPath); err == nil && logFile != nil {
			latestN = len(logFile.Update)
			if len(logFile.Update) > 0 {
				latestLogHash = logFile.Update[len(logFile.Update)-1].AfterHash
			} else {
				latestLogHash = logFile.Base.AfterHash
			}
			if latestLogHash != "" && latestLogHash != currentHash {
				integrityMatch = false
			}
		}
	}

	relSuite, _ := filepath.Rel(s.resolver.Root(), absSuitePath)
	res := InspectResult{
		SuitePath:      relSuite,
		SuiteHash:      currentHash,
		Version:        suite.Version,
		Input:          string(suite.Input),
		Servers:        suite.Servers,
		APICount:       len(suite.APIs),
		APIs:           suite.APIs,
		ScenarioCount:  len(suite.Scenarios),
		Scenarios:      scSummaries,
		SetupCount:     len(suite.Setup),
		LatestUpdateN:  latestN,
		LatestLogHash:  latestLogHash,
		IntegrityMatch: integrityMatch,
	}

	return mcp.NewToolResultJSON(res)
}

type ValidateResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

func (s *Server) handleValidateSuite(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	suiteArg := req.GetString("suite_path", "tests/spectest.yaml")
	if strings.TrimSpace(suiteArg) == "" {
		suiteArg = "tests/spectest.yaml"
	}

	absSuitePath, err := s.resolver.Resolve(suiteArg)
	if err != nil {
		return ToolError(ErrCodePathOutsideRoot, err.Error(), false, nil), nil
	}

	suite, err := suitefile.LoadSuite(absSuitePath)
	if err != nil {
		if os.IsNotExist(err) {
			return ToolError(ErrCodeNotFound, fmt.Sprintf("suite file %q not found", suiteArg), false, nil), nil
		}
		return ToolError(ErrCodeInvalidInput, fmt.Sprintf("failed to parse suite: %v", err), false, nil), nil
	}

	// Validate semantics & structure
	valRes := usecasegenerate.ValidateSuite(suite, nil)
	errStrs := make([]string, 0, len(valRes.Errors))
	for _, e := range valRes.Errors {
		errStrs = append(errStrs, e.Error())
	}

	return mcp.NewToolResultJSON(ValidateResult{
		Valid:    len(errStrs) == 0,
		Errors:   errStrs,
		Warnings: valRes.Warnings,
	})
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
