package run

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/docker"
	"github.com/nguyentin05/spectest/internal/infrastructure/httpclient"
	"github.com/nguyentin05/spectest/internal/infrastructure/openapi"
	"github.com/nguyentin05/spectest/internal/infrastructure/storage"
)

// Options configures a `spectest run` execution.
type Options struct {
	SuitePath   string
	TargetURL   string
	ComposeFile string
	StartCmd    string
	TimeoutSec  int
	PostmanPath string
	APIPath     string
	ReportDir   string
}

// ExecutionRecord represents the detailed outcome of a single step or hybrid test case execution.
type ExecutionRecord struct {
	ScenarioID   string            `json:"scenario_id"`
	ScenarioName string            `json:"scenario_name"`
	StepID       string            `json:"step_id"`
	StepName     string            `json:"step_name"`
	CaseID       string            `json:"case_id"`
	CaseName     string            `json:"case_name"`
	CaseType     string            `json:"case_type"`
	Method       string            `json:"method"`
	Endpoint     string            `json:"endpoint"`
	Passed       bool              `json:"passed"`
	StatusCode   int               `json:"status_code"`
	ExpectedCode int               `json:"expected_code"`
	LatencyMs    int64             `json:"latency_ms"`
	FailedTier     string            `json:"failed_tier,omitempty"`
	FailReason     string            `json:"fail_reason,omitempty"`
	AIDiagnosis    string            `json:"ai_diagnosis,omitempty"`
	RequestPayload string            `json:"request_payload,omitempty"`
	ResponseBody   string            `json:"response_body,omitempty"`
	Extracted      map[string]string `json:"extracted,omitempty"`
}

// Summary holds the complete test run telemetry and results for TUI, JUnit XML, and HTML reporting.
type Summary struct {
	SuiteName      string            `json:"suite_name"`
	TargetURL      string            `json:"target_url"`
	UpdatesApplied int               `json:"updates_applied"`
	TotalChecks    int               `json:"total_checks"`
	PassedChecks   int               `json:"passed_checks"`
	FailedChecks   int               `json:"failed_checks"`
	TotalDuration  time.Duration     `json:"total_duration"`
	AICallsMade    int               `json:"ai_calls_made"`
	Records        []ExecutionRecord `json:"records"`
}

// Runner orchestrates infrastructure lifecycle, in-memory event sourcing replay,
// HTTP execution with state chaining, 4-Tier Oracle evaluation, and AI defect triage.
type Runner struct {
	repo   *storage.YAMLRepository
	parser domain.SpecParser
	llm    domain.LLMService
}

// NewRunner creates a new Runner usecase instance.
func NewRunner(repo *storage.YAMLRepository, parser domain.SpecParser, llmSvc domain.LLMService) *Runner {
	if repo == nil {
		repo = storage.NewYAMLRepository()
	}
	if parser == nil {
		parser = openapi.NewParserDiffer()
	}
	return &Runner{
		repo:   repo,
		parser: parser,
		llm:    llmSvc,
	}
}

// Execute runs the full test suite lifecycle and returns the execution Summary.
func (r *Runner) Execute(ctx context.Context, opts Options) (*Summary, error) {
	suiteFile, err := r.resolveSuite(opts)
	if err != nil {
		return nil, err
	}

	targetURL := strings.TrimSpace(opts.TargetURL)
	if targetURL == "" {
		targetURL = suiteFile.Metadata.TargetBaseURL
	}
	if targetURL == "" {
		targetURL = "http://localhost:8080"
	}

	// 1. All-in-One Infrastructure Lifecycle (Docker Compose / --start-cmd + defer teardown)
	lifecycle := docker.NewLifecycleManager(opts.ComposeFile, opts.StartCmd, targetURL, opts.TimeoutSec)
	teardown, err := lifecycle.Up(ctx)
	if err != nil {
		return nil, err
	}
	defer teardown()

	// 2. Replay Engine in RAM (Base + Append-Only Updates -> EffectiveSuite)
	effectiveSuite := BuildEffectiveSuite(suiteFile)

	httpRunner := httpclient.NewClient(targetURL, time.Duration(opts.TimeoutSec)*time.Second)
	stateStore := httpclient.NewStateStore(suiteFile.Metadata.Env)

	startRun := time.Now()
	summary := &Summary{
		SuiteName:      suiteFile.Metadata.Name,
		TargetURL:      targetURL,
		UpdatesApplied: len(suiteFile.Updates),
	}

	// 3. Execute Scenarios & Steps sequentially with State Chaining
	for _, sc := range effectiveSuite.Scenarios {
		for _, st := range sc.Steps {
			if len(st.TestCases) == 0 {
				rec := r.executeSingleCheck(ctx, httpRunner, stateStore, sc, st, nil, summary)
				summary.Records = append(summary.Records, rec)
			} else {
				for i := range st.TestCases {
					tc := st.TestCases[i]
					rec := r.executeSingleCheck(ctx, httpRunner, stateStore, sc, st, &tc, summary)
					summary.Records = append(summary.Records, rec)
				}
			}
		}
	}

	summary.TotalDuration = time.Since(startRun)
	return summary, nil
}

func (r *Runner) executeSingleCheck(
	ctx context.Context,
	httpRunner *httpclient.Client,
	stateStore *httpclient.StateStore,
	sc domain.Scenario,
	st domain.Step,
	tc *domain.TestCase,
	summary *Summary,
) ExecutionRecord {
	summary.TotalChecks++

	payload := st.Request.Body
	rule := st.Assert
	caseID := "STEP_DEFAULT"
	caseName := st.Name
	caseType := "step_flow"

	if tc != nil {
		payload = tc.Payload
		rule = &tc.Assert
		caseID = tc.ID
		caseName = tc.Name
		caseType = tc.Type
	}

	expectedCode := 200
	if rule != nil && rule.Status > 0 {
		expectedCode = rule.Status
	}

	stateSnap := stateStore.Snapshot()
	rawRes, err := httpRunner.ExecuteRaw(
		ctx,
		st.Request.Method,
		st.Request.Endpoint,
		st.Request.Headers,
		payload,
		st.Extract,
		stateSnap,
	)

	rec := ExecutionRecord{
		ScenarioID:   sc.ID,
		ScenarioName: sc.Name,
		StepID:       st.ID,
		StepName:     st.Name,
		CaseID:       caseID,
		CaseName:     caseName,
		CaseType:     caseType,
		Method:       st.Request.Method,
		Endpoint:     httpclient.InterpolateString(st.Request.Endpoint, stateSnap),
		ExpectedCode: expectedCode,
	}

	if err != nil {
		rec.Passed = false
		rec.FailedTier = "Tier 1: Network / Transport"
		rec.FailReason = err.Error()
		rec.AIDiagnosis = r.triageDefect(ctx, rec, nil, nil, summary)
		summary.FailedChecks++
		return rec
	}

	rec.StatusCode = rawRes.StatusCode
	rec.LatencyMs = rawRes.LatencyMs
	rec.RequestPayload = string(rawRes.RequestBody)
	rec.ResponseBody = TruncateJSONForTriage(rawRes.ResponseBody, 10, 2000)
	rec.Extracted = rawRes.Extracted

	// Save extracted variables into StateStore if extracted (for happy-path state chaining)
	for k, v := range rawRes.Extracted {
		stateStore.Set(k, v)
	}

	// Evaluate 4-Tier Oracle
	eval := EvaluateOracle(rule, rawRes.StatusCode, rawRes.LatencyMs, rawRes.ResponseBody, stateStore.Snapshot())
	rec.Passed = eval.Passed
	rec.FailedTier = eval.FailedTier
	rec.FailReason = eval.Reason

	if rec.Passed {
		// Zero-Token First: 0 AI tokens consumed when test passes!
		summary.PassedChecks++
		return rec
	}

	// Test failed -> Trigger Response Truncation & AI Root-Cause Defect Triage
	summary.FailedChecks++
	rec.AIDiagnosis = r.triageDefect(ctx, rec, rawRes.RequestBody, rawRes.ResponseBody, summary)
	return rec
}

func (r *Runner) triageDefect(ctx context.Context, rec ExecutionRecord, reqBody, respBody []byte, summary *Summary) string {
	if r.llm == nil {
		return ""
	}
	summary.AICallsMade++

	truncatedResp := TruncateJSONForTriage(respBody, 3, 600)
	failureSnapshot := fmt.Sprintf(
		"Step: %s (%s)\nCase: %s [%s]\nEndpoint: %s %s\nExpected HTTP: %d | Actual HTTP: %d\nFailed Oracle Tier: %s\nFailure Reason: %s\nRequest Payload: %s\nTruncated Response Body: %s",
		rec.StepID, rec.StepName,
		rec.CaseID, rec.CaseType,
		rec.Method, rec.Endpoint,
		rec.ExpectedCode, rec.StatusCode,
		rec.FailedTier,
		rec.FailReason,
		string(reqBody),
		truncatedResp,
	)

	diag, err := r.llm.AnalyzeFailure(ctx, failureSnapshot)
	if err != nil {
		return err.Error()
	}
	return diag
}

// TruncateJSONForTriage implements Tier 5 Token Optimization (Response Truncation):
// It truncates long JSON arrays to at most maxArrayItems elements and caps total length,
// preventing noisy logs or large lists from exhausting the LLM context window.
func TruncateJSONForTriage(raw []byte, maxArrayItems int, maxChars int) string {
	if len(raw) == 0 {
		return "<empty>"
	}
	var parsed interface{}
	if err := json.Unmarshal(raw, &parsed); err == nil {
		compacted := truncateJSONNode(parsed, maxArrayItems)
		if b, err := json.Marshal(compacted); err == nil {
			s := string(b)
			if len(s) > maxChars {
				return s[:maxChars] + "...[truncated]"
			}
			return s
		}
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > maxChars {
		return s[:maxChars] + "...[truncated]"
	}
	return s
}

func truncateJSONNode(node interface{}, maxItems int) interface{} {
	switch v := node.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(v))
		for k, val := range v {
			// Strip noisy stacktrace fields
			if strings.EqualFold(k, "stack") || strings.EqualFold(k, "stacktrace") || strings.EqualFold(k, "trace") {
				out[k] = "[stacktrace filtered]"
				continue
			}
			out[k] = truncateJSONNode(val, maxItems)
		}
		return out
	case []interface{}:
		limit := len(v)
		if limit > maxItems {
			limit = maxItems
		}
		out := make([]interface{}, limit)
		for i := 0; i < limit; i++ {
			out[i] = truncateJSONNode(v[i], maxItems)
		}
		return out
	default:
		return node
	}
}

func (r *Runner) resolveSuite(opts Options) (*domain.TestSuiteFile, error) {
	if opts.PostmanPath != "" {
		return r.repo.ImportPostmanCollection(opts.PostmanPath)
	}
	if opts.APIPath != "" {
		raw, err := os.ReadFile(opts.APIPath)
		if err != nil {
			return nil, fmt.Errorf("read --api spec %s: %w", opts.APIPath, err)
		}
		base, err := r.parser.Parse(raw)
		if err != nil {
			return nil, err
		}
		return &domain.TestSuiteFile{
			Version: "1.0",
			Metadata: domain.SuiteMetadata{
				Name:          "Zero-Config OpenAPI Live Run",
				TargetBaseURL: opts.TargetURL,
			},
			Base: *base,
		}, nil
	}
	return r.repo.Load(opts.SuitePath)
}
