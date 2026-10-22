// Package generate implements the `spectest generate` CLI subcommand, which turns an
// OpenAPI specification and optional Markdown PRD into a `spectest.yaml` test suite.
package generate

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/nguyentin05/spectest/internal/adapter/cli/cmdutil"
	ucgenerate "github.com/nguyentin05/spectest/internal/usecase/generate"
	"github.com/spf13/cobra"
)

// options holds the parsed flag values for a single `spectest generate` invocation.
type options struct {
	apiPath   string
	reqPath   string
	outputDir string
	exportFmt string
	enrich    bool
}

// NewCommand creates the `spectest generate` subcommand and binds its flags:
// `--api/-a` (required OpenAPI path), `--req/-r` (optional PRD path), `--output/-o`
// (output directory, default ./tests), `--export/-e` (extra format such as postman),
// and `--enrich`.
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

// runGenerate validates flags, resolves the configured LLM service, executes the
// generation usecase, and prints a plain-text summary of the produced artifacts to out.
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

	fmt.Fprintf(out, "LLM Provider        : %s\n", llmSvc.Name())
	fmt.Fprintf(out, "Spec Pruning        : %.1f%% reduction\n", res.CompressionRatio)
	if res.AuthProducer != "" {
		fmt.Fprintf(out, "Auth Detection      : Detected producer [%s] -> wired Bearer token\n", res.AuthProducer)
	}
	fmt.Fprintf(out, "Domain Chunks       : %d processed\n", res.ChunksProcessed)
	fmt.Fprintf(out, "Test Suite File     : %s\n", res.SuitePath)
	fmt.Fprintf(out, "Coverage Matrix     : %s\n", res.MatrixPath)
	if res.ExportPath != "" {
		fmt.Fprintf(out, "Exported Collection : %s\n", res.ExportPath)
	}

	return nil
}
