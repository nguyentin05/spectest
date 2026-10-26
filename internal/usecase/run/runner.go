package run

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nguyentin05/spectest/internal/domain"
)

// RunEventType represents the type of real-time runner events.
type RunEventType string

const (
	EventSuiteStart       RunEventType = "suite_start"
	EventScenarioStart    RunEventType = "scenario_start"
	EventStepResult       RunEventType = "step_result"
	EventScenarioComplete RunEventType = "scenario_complete"
	EventSuiteComplete    RunEventType = "suite_complete"
)

// RunEvent carries live telemetry during test suite execution.
type RunEvent struct {
	Type              RunEventType   `json:"type"`
	ScenarioID        string         `json:"scenario_id,omitempty"`
	ScenarioName      string         `json:"scenario_name,omitempty"`
	IsSetup           bool           `json:"is_setup,omitempty"`
	SetupName         string         `json:"setup_name,omitempty"`
	StepCall          string         `json:"step_call,omitempty"`
	Method            string         `json:"method,omitempty"`
	URL               string         `json:"url,omitempty"`
	Passed            bool           `json:"passed"`
	Skipped           bool           `json:"skipped,omitempty"`
	SkipReason        string         `json:"skip_reason,omitempty"`
	StatusCode        int            `json:"status_code,omitempty"`
	ExpectedCode      int            `json:"expected_code,omitempty"`
	LatencyMs         int64          `json:"latency_ms,omitempty"`
	FailReason        string         `json:"fail_reason,omitempty"`
	AIDiagnosis       string         `json:"ai_diagnosis,omitempty"`
	Extracted         map[string]any `json:"extracted,omitempty"`
	TotalScenarios    int            `json:"total_scenarios,omitempty"`
	PassedScenarios   int            `json:"passed_scenarios,omitempty"`
	FailedScenarios   int            `json:"failed_scenarios,omitempty"`
	SkippedScenarios  int            `json:"skipped_scenarios,omitempty"`
	Summary           *Summary       `json:"summary,omitempty"`
	SelectionMode     string         `json:"selection_mode,omitempty"`
	UpdateN           int            `json:"update_n,omitempty"`
	SelectedScenarios []string       `json:"selected_scenarios,omitempty"`
}

// HTTPExecutor executes an HTTP request and returns an HTTP response.
type HTTPExecutor interface {
	Do(req *http.Request) (*http.Response, error)
}

// ProbeFunc checks if a given target URL is ready and responding.
type ProbeFunc func(ctx context.Context, targetURL string, timeout time.Duration) error

// LifecycleFactory creates a domain.ServerLifecycleManager for compose boot/teardown.
type LifecycleFactory func(composeFile, targetURL string, timeoutSec int) domain.ServerLifecycleManager

// SuiteLoader loads a test suite from disk.
type SuiteLoader func(path string) (*domain.Suite, error)

// LogLoader loads a history log file from disk.
type LogLoader func(path string) (*domain.LogFile, error)

// Dependencies declares all external ports required by Runner.
type Dependencies struct {
	SuiteLoader      SuiteLoader
	LogLoader        LogLoader
	LifecycleFactory LifecycleFactory
	Probe            ProbeFunc
	HTTPClient       HTTPExecutor
	LLM              domain.LLMService
}

// Options configures a test suite execution run.
type Options struct {
	SuitePath    string
	Suite        *domain.Suite
	TargetURL    string
	Targets      map[string]string
	ComposeFile  string
	TimeoutSec   int
	EnableTriage bool
	OnEvent      func(event RunEvent)
	HTTPClient   HTTPExecutor

	// Selection mode
	OnlyChanged bool
	LogPath     string
}

// ExecutionRecord represents the detailed outcome of a single step execution.
type ExecutionRecord struct {
	ScenarioID     string         `json:"scenario_id"`
	ScenarioName   string         `json:"scenario_name"`
	IsSetup        bool           `json:"is_setup,omitempty"`
	SetupName      string         `json:"setup_name,omitempty"`
	Call           string         `json:"call"`
	Method         string         `json:"method"`
	Endpoint       string         `json:"endpoint"`
	Passed         bool           `json:"passed"`
	StatusCode     int            `json:"status_code"`
	ExpectedCode   int            `json:"expected_code"`
	LatencyMs      int64          `json:"latency_ms"`
	FailReason     string         `json:"fail_reason,omitempty"`
	AIDiagnosis    string         `json:"ai_diagnosis,omitempty"`
	RequestPayload string         `json:"request_payload,omitempty"`
	ResponseBody   string         `json:"response_body,omitempty"`
	Extracted      map[string]any `json:"extracted,omitempty"`
}

// Summary holds the complete test run telemetry and results for reporting.
type Summary struct {
	SuiteName        string            `json:"suite_name"`
	TargetURL        string            `json:"target_url"`
	Targets          map[string]string `json:"targets,omitempty"`
	TotalScenarios   int               `json:"total_scenarios"`
	PassedScenarios  int               `json:"passed_scenarios"`
	FailedScenarios  int               `json:"failed_scenarios"`
	SkippedScenarios int               `json:"skipped_scenarios"`
	TotalChecks      int               `json:"total_checks"`
	PassedChecks     int               `json:"passed_checks"`
	FailedChecks     int               `json:"failed_checks"`
	TotalDuration    time.Duration     `json:"total_duration"`
	Records          []ExecutionRecord `json:"records"`
	SelectionMode    string            `json:"selection_mode"`
	UpdateN          int               `json:"update_n,omitempty"`
}

// Runner orchestrates the execution lifecycle for API test suites:
// target validation → environment readiness → setup DAG → scenario steps → telemetry.
type Runner struct {
	deps Dependencies
}

// NewRunner creates a new Runner with injected dependencies.
func NewRunner(deps Dependencies) *Runner {
	if deps.HTTPClient == nil {
		deps.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Runner{
		deps: deps,
	}
}

// Execute orchestrates the complete test suite execution.
func (r *Runner) Execute(ctx context.Context, opts Options) (*Summary, error) {
	suite := opts.Suite
	suitePath := strings.TrimSpace(opts.SuitePath)
	if suitePath == "" {
		suitePath = "./tests/spectest.yaml"
	}
	// load suite from disk
	if suite == nil {
		if r.deps.SuiteLoader == nil {
			return nil, fmt.Errorf("suite loader not configured and no suite provided in options")
		}
		loaded, err := r.deps.SuiteLoader(suitePath)
		if err != nil {
			return nil, fmt.Errorf("load test suite %q: %w", suitePath, err)
		}
		suite = loaded
	}

	selection := Selection{
		Mode: "all",
	}
	for _, sc := range suite.Scenarios {
		selection.ScenarioIDs = append(selection.ScenarioIDs, sc.ID)
	}

	// only lasted update selection mode
	if opts.OnlyChanged {
		if r.deps.LogLoader == nil {
			return nil, fmt.Errorf("log loader not configured for only-changed execution")
		}
		logPath := strings.TrimSpace(opts.LogPath)
		if logPath == "" {
			logPath = filepath.Join(filepath.Dir(suitePath), "spectest.log.yaml")
		}
		logFile, err := r.deps.LogLoader(logPath)
		if err != nil {
			return nil, fmt.Errorf("load history log %q: %w", logPath, err)
		}

		// Select scenarios affected by the latest update in the log file
		filteredSuite, sel, err := SelectLatestUpdateScenarios(suite, logFile)
		if err != nil {
			return nil, fmt.Errorf("select scenarios for only-changed execution: %w", err)
		}
		suite = filteredSuite
		selection = sel

		// If 0 scenarios are selected (e.g. log has no update or only analysis changed),
		// return early as a successful no-op with 0 scenarios.
		// Do not require target flags, compose lifecycle, or readiness checks.
		if len(suite.Scenarios) == 0 {
			version := suite.Version
			if version <= 0 {
				version = 1
			}
			summary := &Summary{
				SuiteName:      fmt.Sprintf("SpecTest Suite (v%d)", version),
				TotalScenarios: 0,
				SelectionMode:  selection.Mode,
				UpdateN:        selection.UpdateN,
				Records:        []ExecutionRecord{},
			}
			if opts.OnEvent != nil {
				opts.OnEvent(RunEvent{
					Type:              EventSuiteStart,
					TotalScenarios:    0,
					SelectionMode:     selection.Mode,
					UpdateN:           selection.UpdateN,
					SelectedScenarios: []string{},
				})
				opts.OnEvent(RunEvent{
					Type:           EventSuiteComplete,
					TotalScenarios: 0,
					Summary:        summary,
				})
			}
			return summary, nil
		}
	}

	// 1. Validate and normalize targets
	targets := make(map[string]string)
	for k, v := range opts.Targets {
		norm, err := normalizeTargetURL(v)
		if err != nil {
			return nil, fmt.Errorf("target alias %q URL %q is invalid: %w", k, v, err)
		}
		targets[k] = norm
	}

	defaultTarget := strings.TrimSpace(opts.TargetURL)
	if defaultTarget != "" {
		norm, err := normalizeTargetURL(defaultTarget)
		if err != nil {
			return nil, fmt.Errorf("target URL %q: %w", defaultTarget, err)
		}
		defaultTarget = norm
	}

	if defaultTarget == "" && len(targets) == 0 {
		return nil, fmt.Errorf("target URL is required: specify via -t/--target flag")
	}

	// 2. Manage Docker Compose Lifecycle if specified
	if opts.ComposeFile != "" {
		if r.deps.LifecycleFactory == nil {
			return nil, fmt.Errorf("lifecycle factory not configured for compose file %q", opts.ComposeFile)
		}
		probeURL := defaultTarget
		if probeURL == "" {
			for _, u := range targets {
				probeURL = u
				break
			}
		}
		lifecycle := r.deps.LifecycleFactory(opts.ComposeFile, probeURL, opts.TimeoutSec)
		teardown, err := lifecycle.Up(ctx)
		if err != nil {
			return nil, fmt.Errorf("setup server environment: %w", err)
		}
		if teardown != nil {
			defer teardown()
		}
	}

	// 3. Health-check all target URLs before running scenarios (regardless of compose)
	if r.deps.Probe != nil {
		uniqueTargets := make(map[string]struct{})
		if defaultTarget != "" {
			uniqueTargets[defaultTarget] = struct{}{}
		}
		for _, u := range targets {
			if u != "" {
				uniqueTargets[u] = struct{}{}
			}
		}

		targetList := make([]string, 0, len(uniqueTargets))
		for u := range uniqueTargets {
			targetList = append(targetList, u)
		}
		sort.Strings(targetList)

		timeout := time.Duration(opts.TimeoutSec) * time.Second
		if timeout <= 0 {
			timeout = 30 * time.Second
		}

		for _, u := range targetList {
			if err := r.deps.Probe(ctx, u, timeout); err != nil {
				return nil, fmt.Errorf("readiness check failed for target %s: %w", u, err)
			}
		}
	}

	// 4. Select HTTP executor
	executor := opts.HTTPClient
	if executor == nil {
		executor = r.deps.HTTPClient
	}

	// 5. Execute test suite
	return r.executeSuite(ctx, suite, executor, targets, defaultTarget, opts, selection)
}

func (r *Runner) executeSuite(
	ctx context.Context,
	suite *domain.Suite,
	executor HTTPExecutor,
	targets map[string]string,
	defaultTarget string,
	opts Options,
	selection Selection,
) (*Summary, error) {
	startTime := time.Now()

	version := suite.Version
	if version <= 0 {
		version = 1
	}
	suiteName := fmt.Sprintf("SpecTest Suite (v%d)", version)

	summary := &Summary{
		SuiteName:      suiteName,
		TargetURL:      defaultTarget,
		Targets:        targets,
		TotalScenarios: len(suite.Scenarios),
		SelectionMode:  selection.Mode,
		UpdateN:        selection.UpdateN,
		Records:        make([]ExecutionRecord, 0),
	}

	emit := func(evt RunEvent) {
		if opts.OnEvent != nil {
			opts.OnEvent(evt)
		}
	}

	emit(RunEvent{
		Type:              EventSuiteStart,
		TotalScenarios:    len(suite.Scenarios),
		SelectionMode:     selection.Mode,
		UpdateN:           selection.UpdateN,
		SelectedScenarios: selection.ScenarioIDs,
	})

	for _, sc := range suite.Scenarios {
		if ctx.Err() != nil {
			break
		}

		if sc.Skip != "" {
			summary.SkippedScenarios++
			emit(RunEvent{
				Type:         EventScenarioStart,
				ScenarioID:   sc.ID,
				ScenarioName: sc.Name,
				Skipped:      true,
				SkipReason:   sc.Skip,
			})
			emit(RunEvent{
				Type:         EventScenarioComplete,
				ScenarioID:   sc.ID,
				ScenarioName: sc.Name,
				Skipped:      true,
				SkipReason:   sc.Skip,
				Passed:       true,
			})
			continue
		}

		emit(RunEvent{
			Type:         EventScenarioStart,
			ScenarioID:   sc.ID,
			ScenarioName: sc.Name,
		})

		// A. Initialize isolated scenario scope with Seed variables
		scScope, err := NewScopeFromSeed(suite.Seed)
		if err != nil {
			summary.FailedScenarios++
			summary.Records = append(summary.Records, ExecutionRecord{
				ScenarioID:   sc.ID,
				ScenarioName: sc.Name,
				Passed:       false,
				FailReason:   fmt.Sprintf("seed initialization error: %v", err),
			})
			emit(RunEvent{
				Type:         EventScenarioComplete,
				ScenarioID:   sc.ID,
				ScenarioName: sc.Name,
				Passed:       false,
				FailReason:   err.Error(),
			})
			continue
		}

		// B. Resolve setup DAG order
		orderedSetups, err := ResolveSetupDAG(sc.Setup, suite.Setup)
		if err != nil {
			summary.FailedScenarios++
			summary.Records = append(summary.Records, ExecutionRecord{
				ScenarioID:   sc.ID,
				ScenarioName: sc.Name,
				Passed:       false,
				FailReason:   fmt.Sprintf("setup resolution error: %v", err),
			})
			emit(RunEvent{
				Type:         EventScenarioComplete,
				ScenarioID:   sc.ID,
				ScenarioName: sc.Name,
				Passed:       false,
				FailReason:   err.Error(),
			})
			continue
		}

		scPassed := true

		// C. Execute Setup blocks sequentially
		for _, setupName := range orderedSetups {
			if !scPassed || ctx.Err() != nil {
				break
			}

			setupDef := suite.Setup[setupName]
			scScope.MergeSetupVars(setupDef.Vars)

			for _, step := range setupDef.Steps {
				if ctx.Err() != nil {
					scPassed = false
					break
				}

				rec := executeSingleStep(ctx, executor, step, targets, defaultTarget, scScope, sc.ID, sc.Name, true, setupName, suite.Analysis, opts.EnableTriage, r.deps.LLM)
				summary.TotalChecks++
				summary.Records = append(summary.Records, rec)

				if rec.Passed {
					summary.PassedChecks++
					emit(RunEvent{
						Type:         EventStepResult,
						ScenarioID:   sc.ID,
						ScenarioName: sc.Name,
						IsSetup:      true,
						SetupName:    setupName,
						StepCall:     rec.Call,
						Method:       rec.Method,
						URL:          rec.Endpoint,
						Passed:       true,
						StatusCode:   rec.StatusCode,
						ExpectedCode: rec.ExpectedCode,
						LatencyMs:    rec.LatencyMs,
						Extracted:    rec.Extracted,
					})
				} else {
					summary.FailedChecks++
					scPassed = false
					emit(RunEvent{
						Type:         EventStepResult,
						ScenarioID:   sc.ID,
						ScenarioName: sc.Name,
						IsSetup:      true,
						SetupName:    setupName,
						StepCall:     rec.Call,
						Method:       rec.Method,
						URL:          rec.Endpoint,
						Passed:       false,
						StatusCode:   rec.StatusCode,
						ExpectedCode: rec.ExpectedCode,
						LatencyMs:    rec.LatencyMs,
						FailReason:   rec.FailReason,
						AIDiagnosis:  rec.AIDiagnosis,
					})
					break
				}
			}
		}

		if !scPassed {
			summary.FailedScenarios++
			emit(RunEvent{
				Type:         EventScenarioComplete,
				ScenarioID:   sc.ID,
				ScenarioName: sc.Name,
				Passed:       false,
			})
			continue
		}

		// D. Execute Scenario Steps
		for _, step := range sc.Steps {
			if ctx.Err() != nil {
				scPassed = false
				break
			}

			rec := executeSingleStep(ctx, executor, step, targets, defaultTarget, scScope, sc.ID, sc.Name, false, "", suite.Analysis, opts.EnableTriage, r.deps.LLM)
			summary.TotalChecks++
			summary.Records = append(summary.Records, rec)

			if rec.Passed {
				summary.PassedChecks++
				emit(RunEvent{
					Type:         EventStepResult,
					ScenarioID:   sc.ID,
					ScenarioName: sc.Name,
					StepCall:     rec.Call,
					Method:       rec.Method,
					URL:          rec.Endpoint,
					Passed:       true,
					StatusCode:   rec.StatusCode,
					ExpectedCode: rec.ExpectedCode,
					LatencyMs:    rec.LatencyMs,
					Extracted:    rec.Extracted,
				})
			} else {
				summary.FailedChecks++
				scPassed = false
				emit(RunEvent{
					Type:         EventStepResult,
					ScenarioID:   sc.ID,
					ScenarioName: sc.Name,
					StepCall:     rec.Call,
					Method:       rec.Method,
					URL:          rec.Endpoint,
					Passed:       false,
					StatusCode:   rec.StatusCode,
					ExpectedCode: rec.ExpectedCode,
					LatencyMs:    rec.LatencyMs,
					FailReason:   rec.FailReason,
					AIDiagnosis:  rec.AIDiagnosis,
				})
				break // Fail-fast per scenario
			}
		}

		if scPassed {
			summary.PassedScenarios++
			emit(RunEvent{
				Type:         EventScenarioComplete,
				ScenarioID:   sc.ID,
				ScenarioName: sc.Name,
				Passed:       true,
			})
		} else {
			summary.FailedScenarios++
			emit(RunEvent{
				Type:         EventScenarioComplete,
				ScenarioID:   sc.ID,
				ScenarioName: sc.Name,
				Passed:       false,
			})
		}
	}

	summary.TotalDuration = time.Since(startTime)

	// Check if context was cancelled during execution
	if ctx.Err() != nil {
		return summary, ctx.Err()
	}

	emit(RunEvent{
		Type:    EventSuiteComplete,
		Summary: summary,
	})

	return summary, nil
}

// normalizeTargetURL validates targetURL and ensures only http and https protocols are accepted.
func normalizeTargetURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("target URL cannot be empty")
	}

	if !strings.Contains(rawURL, "://") {
		rawURL = "http://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("invalid target URL %q", rawURL)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("invalid target URL scheme %q: only http and https are supported", u.Scheme)
	}

	return strings.TrimRight(u.String(), "/"), nil
}
