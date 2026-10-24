package run

import (
	"context"
	"fmt"
	"io"

	"github.com/nguyentin05/spectest/internal/adapter/cli/cmdutil"
	ucrun "github.com/nguyentin05/spectest/internal/usecase/run"
	"github.com/spf13/cobra"
)

type options struct {
	suitePath   string
	targetURL   string
	composeFile string
	timeoutSec  int
	reportDir   string
}

// NewCommand creates the `spectest run` subcommand.
func NewCommand(f *cmdutil.Factory) *cobra.Command {
	opts := &options{
		suitePath:  "./tests/spectest.yaml",
		timeoutSec: 30,
		reportDir:  "./reports",
	}

	cmd := &cobra.Command{
		Use:          "run",
		Short:        "Execute API test suites, verify 4-tier assertions, and diagnose defects with AI",
		GroupID:      "core",
		SilenceUsage: true,
		Long: `Execute spectest.yaml suites against target APIs with state chaining, evaluate 4-tier
assertions (HTTP status, schema integrity, business calculations, and round-trip state), and
automatically trigger AI defect triage when test cases fail.`,
		Example: `  spectest run -f ./tests/spectest.yaml -t http://localhost:8080
  spectest run -f ./tests/spectest.yaml --compose docker-compose.test.yml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSuite(cmd.Context(), cmd.OutOrStdout(), f, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.suitePath, "file", "f", opts.suitePath, "Path to test suite file")
	cmd.Flags().StringVarP(&opts.targetURL, "target", "t", "", "Target base URL for the tested API (overrides suite metadata)")
	cmd.Flags().StringVarP(&opts.composeFile, "compose", "c", "", "Path to Docker Compose file (auto boots DB/Redis/App, runs, tears down)")
	cmd.Flags().IntVar(&opts.timeoutSec, "timeout", opts.timeoutSec, "Server readiness timeout in seconds")
	cmd.Flags().StringVar(&opts.reportDir, "report-dir", opts.reportDir, "Directory to output test reports")

	return cmd
}

func runSuite(ctx context.Context, out io.Writer, f *cmdutil.Factory, opts *options) error {
	if ctx == nil {
		ctx = context.Background()
	}

	var llmSvc = func() domainLLMService {
		if f != nil {
			svc, _ := f.LLMService()
			return svc
		}
		return nil
	}()

	runner := ucrun.NewRunner(f.SuiteRepository(), llmSvc)
	summary, err := runner.Execute(ctx, ucrun.Options{
		SuitePath:   opts.suitePath,
		TargetURL:   opts.targetURL,
		ComposeFile: opts.composeFile,
		TimeoutSec:  opts.timeoutSec,
		ReportDir:   opts.reportDir,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "🚀 SpecTest: ready and executed suite %q against %s (%d checks evaluated)\n",
		summary.SuiteName, summary.TargetURL, summary.TotalChecks)

	return nil
}

type domainLLMService interface {
	Name() string
	GenerateStructured(ctx context.Context, sys, user string, schema interface{}) (string, error)
}
