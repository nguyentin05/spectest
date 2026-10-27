package run

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nguyentin05/spectest/internal/adapter/cli/cmdutil"
	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/server"
	"github.com/nguyentin05/spectest/internal/infrastructure/suitefile"
	"github.com/nguyentin05/spectest/internal/platform/atomicfile"
	ucrun "github.com/nguyentin05/spectest/internal/usecase/run"
	"github.com/spf13/cobra"
)

type options struct {
	suitePath   string
	targetFlags []string
	composeFile string
	timeoutSec  int
	reportDir   string
	sarifFile   string
	triage      bool
	onlyChanged bool
	logPath     string
}

// NewCommand creates the `spectest run` subcommand.
func NewCommand(f *cmdutil.Factory) *cobra.Command {
	opts := &options{
		suitePath:  "./tests/spectest.yaml",
		timeoutSec: 30,
		reportDir:  "./reports",
		triage:     false,
	}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute an autonomous API test suite against target services",
		Long: `Execute an autonomous API test suite (spectest.yaml) against one or more target endpoints.
It orchestrates setups with topological sorting, manages state-chaining, evaluates test
assertions (HTTP status, schema integrity, business calculations, and round-trip state), and
automatically trigger AI defect triage when test cases fail.`,
		Example: `  spectest run -f ./tests/spectest.yaml -t http://localhost:8080
  spectest run -f ./tests/spectest.yaml -t auth=http://localhost:8001 -t order=http://localhost:8002
  spectest run -f ./tests/spectest.yaml -t http://localhost:8080 --triage
  spectest run -f ./tests/spectest.yaml --compose docker-compose.test.yml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return runSuite(cmd.Context(), cmd.OutOrStdout(), f, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.suitePath, "file", "f", opts.suitePath, "Path to test suite file")
	cmd.Flags().StringArrayVarP(&opts.targetFlags, "target", "t", nil, "Target base URL (single URL or alias mapping e.g. -t auth=http://localhost:8001)")
	cmd.Flags().StringVarP(&opts.composeFile, "compose", "c", "", "Path to Docker Compose file (auto boots DB/Redis/App, runs, tears down)")
	cmd.Flags().IntVar(&opts.timeoutSec, "timeout", opts.timeoutSec, "Server readiness timeout in seconds")
	cmd.Flags().StringVar(&opts.reportDir, "report-dir", opts.reportDir, "Directory to output test reports")
	cmd.Flags().StringVar(&opts.sarifFile, "sarif", "", "Path to output SARIF 2.1.0 report for GitHub Code Scanning")
	cmd.Flags().BoolVar(&opts.triage, "triage", false, "Enable AI defect triage on step failure (requires configured LLM)")
	cmd.Flags().BoolVar(&opts.onlyChanged, "only-changed", false, "Run scenarios affected by the latest history update")
	cmd.Flags().StringVar(&opts.logPath, "log", "", "Path to spectest.log.yaml; defaults beside --file")

	return cmd
}

func runSuite(ctx context.Context, out io.Writer, f *cmdutil.Factory, opts *options) error {
	if ctx == nil {
		ctx = context.Background()
	}

	var llmSvc domainLLMService
	if opts.triage && f != nil {
		llmSvc, _ = f.LLMService()
	}

	defaultTarget := ""
	targets := make(map[string]string)

	for _, flagVal := range opts.targetFlags {
		flagVal = strings.TrimSpace(flagVal)
		if strings.Contains(flagVal, "=") {
			parts := strings.SplitN(flagVal, "=", 2)
			alias := strings.TrimSpace(parts[0])
			targetURL := strings.TrimSpace(parts[1])
			if alias == "" {
				return fmt.Errorf("target alias cannot be empty in flag %q", flagVal)
			}
			if targetURL == "" {
				return fmt.Errorf("target URL cannot be empty in flag %q", flagVal)
			}
			if _, exists := targets[alias]; exists {
				return fmt.Errorf("duplicate target alias %q specified in flags", alias)
			}
			norm, err := server.NormalizeTargetURL(targetURL)
			if err != nil {
				return fmt.Errorf("invalid target URL %q: %w", targetURL, err)
			}
			targets[alias] = norm
		} else if flagVal != "" {
			if defaultTarget != "" {
				return fmt.Errorf("multiple default target URLs specified: %q and %q", defaultTarget, flagVal)
			}
			norm, err := server.NormalizeTargetURL(flagVal)
			if err != nil {
				return fmt.Errorf("invalid default target URL %q: %w", flagVal, err)
			}
			defaultTarget = norm
		}
	}

	logPath := strings.TrimSpace(opts.logPath)
	if opts.onlyChanged && logPath == "" {
		dir := filepath.Dir(opts.suitePath)
		logPath = filepath.Join(dir, "spectest.log.yaml")
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

	summary, err := runner.Execute(ctx, ucrun.Options{
		SuitePath:    opts.suitePath,
		OnlyChanged:  opts.onlyChanged,
		LogPath:      logPath,
		TargetURL:    defaultTarget,
		Targets:      targets,
		ComposeFile:  opts.composeFile,
		TimeoutSec:   opts.timeoutSec,
		EnableTriage: opts.triage,
		OnEvent: func(evt ucrun.RunEvent) {
			switch evt.Type {
			case ucrun.EventScenarioStart:
				_, _ = fmt.Fprintf(out, "▶ [%s] %s\n", evt.ScenarioID, evt.ScenarioName)
			case ucrun.EventStepResult:
				if evt.Passed {
					_, _ = fmt.Fprintf(out, "  ✓ %s (%dms)\n", evt.StepCall, evt.LatencyMs)
				} else {
					_, _ = fmt.Fprintf(out, "  ✗ %s (%dms) -> FAIL: %s\n", evt.StepCall, evt.LatencyMs, evt.FailReason)
					if evt.AIDiagnosis != "" {
						_, _ = fmt.Fprintf(out, "    AI Triage: %s\n", evt.AIDiagnosis)
					}
				}
			case ucrun.EventScenarioComplete:
				if evt.Skipped {
					_, _ = fmt.Fprintf(out, "  [SKIPPED] (Reason: %s)\n", evt.SkipReason)
				}
				_, _ = fmt.Fprintln(out)
			}
		},
	})
	if err != nil {
		return err
	}

	// Write report file if report directory specified
	if opts.reportDir != "" && summary != nil {
		if writeErr := writeReport(opts.reportDir, summary); writeErr != nil {
			return fmt.Errorf("failed to write test report: %w", writeErr)
		}
	}

	// Write SARIF report if specified
	if opts.sarifFile != "" && summary != nil {
		sarifBytes, sarifErr := buildSARIF(summary, opts.suitePath)
		if sarifErr != nil {
			return fmt.Errorf("failed to build sarif report: %w", sarifErr)
		}
		dir := filepath.Dir(opts.sarifFile)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return fmt.Errorf("create sarif directory %q: %w", dir, err)
			}
		}
		if writeErr := atomicfile.Write(opts.sarifFile, sarifBytes, 0o600); writeErr != nil {
			return fmt.Errorf("failed to write sarif report: %w", writeErr)
		}
	}

	targetDisplay := summary.TargetURL
	if targetDisplay == "" && len(summary.Targets) > 0 {
		keys := make([]string, 0, len(summary.Targets))
		for k := range summary.Targets {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%s", k, summary.Targets[k]))
		}
		targetDisplay = strings.Join(parts, ", ")
	}
	if targetDisplay != "" {
		targetDisplay = " against " + targetDisplay
	}

	_, _ = fmt.Fprintf(out, "SpecTest: ready and executed suite %q%s (%d checks evaluated)\n",
		summary.SuiteName, targetDisplay, summary.TotalChecks)

	if summary.SelectionMode == "latest_update" {
		_, _ = fmt.Fprintf(out, "Selection: latest update #%d (%d scenarios selected)\n",
			summary.UpdateN, summary.TotalScenarios)
	}

	_, _ = fmt.Fprintf(out, "Results: Total Scenarios: %d | Passed: %d | Failed: %d | Skipped: %d | Duration: %v\n",
		summary.TotalScenarios, summary.PassedScenarios, summary.FailedScenarios, summary.SkippedScenarios, summary.TotalDuration.Round(100*time.Microsecond))

	if summary.FailedScenarios > 0 {
		return fmt.Errorf("test suite failed with %d scenario failure(s)", summary.FailedScenarios)
	}

	return nil
}

func writeReport(reportDir string, summary *ucrun.Summary) error {
	if err := os.MkdirAll(reportDir, 0o700); err != nil {
		return fmt.Errorf("create report directory %q: %w", reportDir, err)
	}
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("encode report json: %w", err)
	}
	reportFile := filepath.Join(reportDir, "report.json")
	if err := atomicfile.Write(reportFile, data, 0o600); err != nil {
		return fmt.Errorf("write report atomic: %w", err)
	}
	return nil
}

type domainLLMService interface {
	Name() string
	GenerateStructured(ctx context.Context, sys, user string, schema interface{}) (string, error)
}
