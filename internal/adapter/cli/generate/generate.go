package generate

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/nguyentin05/spectest/internal/adapter/cli/cmdutil"
	"github.com/nguyentin05/spectest/internal/adapter/presenter"
	ucgenerate "github.com/nguyentin05/spectest/internal/usecase/generate"
	"github.com/spf13/cobra"
)

type options struct {
	apiPath   string
	reqPath   string
	outputDir string
	exportFmt string
	enrich    bool
}

// NewCommand creates the `spectest generate` subcommand.
func NewCommand(f *cmdutil.Factory) *cobra.Command {
	opts := &options{
		outputDir: "./tests",
	}

	cmd := &cobra.Command{
		Use:     "generate",
		Short:   "Generate test scenarios and hybrid datasets from OpenAPI and PRD specifications",
		GroupID: "core",
		Long: `Analyze OpenAPI specs and Markdown PRD requirements to generate automated test suites,
synthesize hybrid test datasets, and produce structured spectest.yaml and test_matrix.md.`,
		Example: `  spectest generate -a openapi.yaml -r requirements.md
  spectest generate -a openapi.yaml -o ./tests --export postman`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerate(cmd.Context(), cmd.OutOrStdout(), f, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.apiPath, "api", "a", "", "Path to OpenAPI/Swagger specification (yaml/json) [required]")
	cmd.Flags().StringVarP(&opts.reqPath, "req", "r", "", "Path to business requirements / PRD (markdown)")
	cmd.Flags().StringVarP(&opts.outputDir, "output", "o", opts.outputDir, "Output directory for generated test suites")
	cmd.Flags().StringVarP(&opts.exportFmt, "export", "e", "", "Export additional formats (e.g. 'postman')")
	cmd.Flags().BoolVar(&opts.enrich, "enrich", false, "Use AI to enrich missing descriptions in OpenAPI before generating")

	return cmd
}

func runGenerate(ctx context.Context, out io.Writer, f *cmdutil.Factory, opts *options) error {
	if strings.TrimSpace(opts.apiPath) == "" {
		return fmt.Errorf("required flag --api (-a) is missing")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	llmSvc, err := f.LLMService()
	if err != nil {
		return err
	}

	gen := ucgenerate.NewGenerator(llmSvc, f.SuiteRepository())
	res, err := gen.Execute(ctx, ucgenerate.Request{
		APIPath:   opts.apiPath,
		ReqPath:   opts.reqPath,
		OutputDir: opts.outputDir,
		ExportFmt: opts.exportFmt,
		Enrich:    opts.enrich,
	})
	if err != nil {
		return err
	}

	presenter.RenderGenerateSummary(out, res, llmSvc.Name())
	return nil
}
