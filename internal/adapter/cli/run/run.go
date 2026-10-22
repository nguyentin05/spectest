package run

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/nguyentin05/spectest/internal/adapter/cli/cmdutil"
	"github.com/nguyentin05/spectest/internal/adapter/presenter"
	ucrun "github.com/nguyentin05/spectest/internal/usecase/run"
	"github.com/spf13/cobra"
)

type options struct {
	suitePath   string
	targetURL   string
	composeFile string
	startCmd    string
	timeoutSec  int
	postmanPath string
	apiPath     string
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
  spectest run -f ./tests/spectest.yaml --compose docker-compose.test.yml
  spectest run --postman collection.json -t https://staging-api.myshop.vn`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSuite(cmd.Context(), cmd.OutOrStdout(), f, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.suitePath, "file", "f", opts.suitePath, "Path to test suite file")
	cmd.Flags().StringVarP(&opts.targetURL, "target", "t", "", "Target base URL for the tested API (overrides suite metadata)")
	cmd.Flags().StringVarP(&opts.composeFile, "compose", "c", "", "Path to Docker Compose file (auto boots DB/Redis/App, runs, tears down)")
	cmd.Flags().StringVarP(&opts.startCmd, "start-cmd", "s", "", "Command to start server locally before testing")
	cmd.Flags().IntVar(&opts.timeoutSec, "timeout", opts.timeoutSec, "Healthcheck wait timeout in seconds")
	cmd.Flags().StringVarP(&opts.postmanPath, "postman", "p", "", "Directly run a Postman Collection JSON file")
	cmd.Flags().StringVar(&opts.apiPath, "api", "", "Zero-config run: directly test an OpenAPI spec in-memory")
	cmd.Flags().StringVar(&opts.reportDir, "report-dir", opts.reportDir, "Directory to output junit.xml and report.html")

	return cmd
}

func runSuite(ctx context.Context, out io.Writer, f *cmdutil.Factory, opts *options) error {
	if opts.composeFile != "" && opts.startCmd != "" {
		return fmt.Errorf("cannot specify both --compose (-c) and --start-cmd (-s) simultaneously")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	llmSvc, err := f.LLMService()
	if err != nil {
		return err
	}

	runner := ucrun.NewRunner(f.SuiteRepository(), f.SpecParser(), llmSvc)
	summary, err := runner.Execute(ctx, ucrun.Options{
		SuitePath:   opts.suitePath,
		TargetURL:   opts.targetURL,
		ComposeFile: opts.composeFile,
		StartCmd:    opts.startCmd,
		TimeoutSec:  opts.timeoutSec,
		PostmanPath: opts.postmanPath,
		APIPath:     opts.apiPath,
		ReportDir:   opts.reportDir,
	})
	if err != nil {
		return err
	}

	presenter.RenderRunSummary(out, summary, llmSvc.Name())

	junitPath := filepath.Join(opts.reportDir, "junit.xml")
	htmlPath := filepath.Join(opts.reportDir, "report.html")
	jsonPath := filepath.Join(opts.reportDir, "summary.json")
	_ = presenter.ExportJUnitXML(junitPath, summary)
	_ = presenter.ExportHTMLDashboard(htmlPath, summary)
	_ = presenter.ExportSummaryJSON(jsonPath, summary)

	fmt.Fprintf(out, "\n📊 Reports exported: %s | %s\n", junitPath, htmlPath)

	if summary.FailedChecks > 0 {
		return fmt.Errorf("%d test case(s) failed 4-Tier Oracle verification", summary.FailedChecks)
	}
	return nil
}
