package run

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/server"
	"github.com/nguyentin05/spectest/internal/infrastructure/storage"
)

// Options configures a `spectest run` execution.
type Options struct {
	SuitePath   string
	TargetURL   string
	ComposeFile string
	TimeoutSec  int
	ReportDir   string
}

// ExecutionRecord represents the detailed outcome of a single step or hybrid test case execution.
type ExecutionRecord struct {
	ScenarioID     string            `json:"scenario_id"`
	ScenarioName   string            `json:"scenario_name"`
	StepID         string            `json:"step_id"`
	StepName       string            `json:"step_name"`
	CaseID         string            `json:"case_id"`
	CaseName       string            `json:"case_name"`
	CaseType       string            `json:"case_type"`
	Method         string            `json:"method"`
	Endpoint       string            `json:"endpoint"`
	Passed         bool              `json:"passed"`
	StatusCode     int               `json:"status_code"`
	ExpectedCode   int               `json:"expected_code"`
	LatencyMs      int64             `json:"latency_ms"`
	FailedTier     string            `json:"failed_tier,omitempty"`
	FailReason     string            `json:"fail_reason,omitempty"`
	AIDiagnosis    string            `json:"ai_diagnosis,omitempty"`
	RequestPayload string            `json:"request_payload,omitempty"`
	ResponseBody   string            `json:"response_body,omitempty"`
	Extracted      map[string]string `json:"extracted,omitempty"`
}

// Summary holds the complete test run telemetry and results for reporting.
type Summary struct {
	SuiteName     string            `json:"suite_name"`
	TargetURL     string            `json:"target_url"`
	TotalChecks   int               `json:"total_checks"`
	PassedChecks  int               `json:"passed_checks"`
	FailedChecks  int               `json:"failed_checks"`
	TotalDuration time.Duration     `json:"total_duration"`
	Records       []ExecutionRecord `json:"records"`
}

// LifecycleFactory creates a domain.ServerLifecycleManager for a given compose file, target URL, and timeout.
type LifecycleFactory func(composeFile, targetURL string, timeoutSec int) domain.ServerLifecycleManager

// Runner orchestrates the execution lifecycle for API test suites:
// 1. Server lifecycle setup (existing server readiness or Docker Compose startup)
// 2. Test suite loading and in-memory replay
// 3. HTTP execution and 4-tier oracle evaluation (plugged in by teammate)
type Runner struct {
	repo        domain.SuiteRepository
	lifecycleFn LifecycleFactory
	llm         domain.LLMService
}

// NewRunner creates a new Runner with dependencies.
func NewRunner(repo domain.SuiteRepository, llmSvc domain.LLMService) *Runner {
	if repo == nil {
		repo = storage.NewYAMLRepository()
	}
	return &Runner{
		repo: repo,
		lifecycleFn: func(composeFile, targetURL string, timeoutSec int) domain.ServerLifecycleManager {
			return server.NewLifecycleManager(composeFile, targetURL, timeoutSec)
		},
		llm: llmSvc,
	}
}

// SetLifecycleFactory allows overriding the lifecycle manager constructor for testing.
func (r *Runner) SetLifecycleFactory(fn LifecycleFactory) {
	r.lifecycleFn = fn
}

// Execute orchestrates the full test suite run lifecycle.
func (r *Runner) Execute(ctx context.Context, opts Options) (*Summary, error) {
	suitePath := strings.TrimSpace(opts.SuitePath)
	if suitePath == "" {
		suitePath = "./tests/spectest.yaml"
	}

	suiteFile, err := r.repo.Load(suitePath)
	if err != nil {
		return nil, fmt.Errorf("load test suite %q: %w", suitePath, err)
	}

	targetURL := strings.TrimSpace(opts.TargetURL)
	if targetURL == "" {
		targetURL = strings.TrimSpace(suiteFile.Metadata.TargetBaseURL)
	}
	if targetURL == "" {
		return nil, fmt.Errorf("target URL is required: specify via -t/--target flag or set metadata.target_base_url in the test suite")
	}

	normalizedURL, err := server.NormalizeTargetURL(targetURL)
	if err != nil {
		return nil, fmt.Errorf("target URL %q: %w", targetURL, err)
	}
	targetURL = normalizedURL

	timeoutSec := opts.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 30
	}

	// 1. Setup Server Lifecycle (Existing server healthcheck or Docker Compose up)
	lifecycle := r.lifecycleFn(opts.ComposeFile, targetURL, timeoutSec)
	teardown, err := lifecycle.Up(ctx)
	if err != nil {
		return nil, fmt.Errorf("setup server environment: %w", err)
	}
	defer teardown()

	// 2. Execute Suite & Evaluate
	summary, err := r.executeSuite(ctx, suiteFile, targetURL)
	if err != nil {
		return nil, fmt.Errorf("execute suite: %w", err)
	}

	return summary, nil
}

// executeSuite is the execution hook that the teammate will populate with
// the RAM Replay Engine, HTTP client with state chaining, 4-tier oracle, and AI triage.
func (r *Runner) executeSuite(_ context.Context, suite *domain.TestSuiteFile, targetURL string) (*Summary, error) {
	startTime := time.Now()
	summary := &Summary{
		SuiteName:     suite.Metadata.Name,
		TargetURL:     targetURL,
		TotalDuration: time.Since(startTime),
		Records:       make([]ExecutionRecord, 0),
	}

	for _, sc := range suite.Base.Scenarios {
		for _, step := range sc.Steps {
			for range step.TestCases {
				summary.TotalChecks++
			}
			if len(step.TestCases) == 0 && step.Assert != nil {
				summary.TotalChecks++
			}
		}
	}

	return summary, nil
}
