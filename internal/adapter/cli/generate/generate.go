// Package generate implements the `spectest generate` CLI subcommand, which
// creates or updates the spectest suite and history log from one or more
// OpenAPI documents and optional raw requirements.
package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nguyentin05/spectest/internal/adapter/cli/cmdutil"
	"github.com/nguyentin05/spectest/internal/infrastructure/sources"
	"github.com/nguyentin05/spectest/internal/infrastructure/suitefile"
	usecasegenerate "github.com/nguyentin05/spectest/internal/usecase/generate"
	"github.com/spf13/cobra"
)

// options holds the parsed flag values for a single `spectest generate` invocation.
type options struct {
	apiValues   []string
	reqPath     string
	outputDir   string
	mode        string
	dryRun      bool
	concurrency int
}

// NewCommand creates the `spectest generate` subcommand. It supports `--mode init`
// to generate a new suite and `--mode update` to incrementally synchronize an existing
// suite with OpenAPI changes.
func NewCommand(f *cmdutil.Factory) *cobra.Command {
	opts := &options{outputDir: "./tests", mode: "init", concurrency: 2}

	cmd := &cobra.Command{
		Use:     "generate",
		Short:   "Generate a new SpecTest suite from OpenAPI and requirements",
		GroupID: "core",
		Args:    cobra.NoArgs,
		Long: `Generate or update a spectest.yaml suite from one or more OpenAPI documents and optional
raw requirements. The command manages spectest.yaml and spectest.log.yaml together.

In init mode, if existing suite files are found, confirmation is required before overwriting.
In update mode, OpenAPI changes are diffed against the history log catalog and applied incrementally.`,
		Example: `  spectest generate --mode init --api auth=openapi-auth.yaml --api order=openapi-order.yaml
  spectest generate --mode init -a auth=openapi.yaml -r requirements.md -o ./tests --dry-run
  spectest generate --mode update -a auth=openapi.yaml -o ./tests`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerate(cmd, opts, f)
		},
	}

	cmd.Flags().StringArrayVarP(&opts.apiValues, "api", "a", nil,
		"OpenAPI document as <server>=<path> (repeatable; order defines servers)")
	cmd.Flags().StringVarP(&opts.reqPath, "req", "r", "",
		"Path to raw business requirements (markdown)")
	cmd.Flags().StringVarP(&opts.outputDir, "output", "o", opts.outputDir,
		"Directory containing spectest.yaml and spectest.log.yaml")
	cmd.Flags().StringVar(&opts.mode, "mode", opts.mode,
		"Generation mode: init or update")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false,
		"Run the full pipeline but write no files")

	return cmd
}

func validateMode(mode string) error {
	switch mode {
	case "init", "update":
		return nil
	default:
		return fmt.Errorf("invalid --mode %q: must be init or update", mode)
	}
}

func runGenerate(cmd *cobra.Command, opts *options, f *cmdutil.Factory) error {
	if err := cmd.Context().Err(); err != nil {
		return err
	}
	if err := validateMode(opts.mode); err != nil {
		return err
	}
	if len(opts.apiValues) == 0 {
		return fmt.Errorf("required flag --api (-a) is missing")
	}
	inputs, err := parseAPIFlags(opts.apiValues)
	if err != nil {
		return err
	}
	if strings.TrimSpace(opts.outputDir) == "" {
		return fmt.Errorf("--output must not be empty")
	}

	cmd.SilenceUsage = true

	reqMode := usecasegenerate.ModeInitial
	if opts.mode == "update" {
		reqMode = usecasegenerate.ModeUpdate
	}

	if reqMode == usecasegenerate.ModeInitial {
		if err := guardOverwrite(
			cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), opts.outputDir, opts.dryRun,
		); err != nil {
			return err
		}
	} else {
		// Update mode requires spectest.yaml and spectest.log.yaml to already exist
		suitePath := filepath.Join(opts.outputDir, suitefile.SuiteFileName)
		logPath := filepath.Join(opts.outputDir, suitefile.LogFileName)
		if _, err := os.Stat(suitePath); err != nil {
			return fmt.Errorf("update requires %s in %s: %w", suitefile.SuiteFileName, opts.outputDir, err)
		}
		if _, err := os.Stat(logPath); err != nil {
			return fmt.Errorf("update requires %s in %s: %w", suitefile.LogFileName, opts.outputDir, err)
		}
	}

	llmSvc, err := f.LLMService()
	if err != nil {
		if reqMode == usecasegenerate.ModeUpdate {
			// Update mode resolves LLM optionally (errors only if plan requires LLM)
			llmSvc = nil
		} else {
			return err
		}
	}

	loader := sources.NewLoader()
	gen := usecasegenerate.NewGenerator(llmSvc, loader, f.SuiteStore(), realClock{})

	res, err := gen.Execute(cmd.Context(), usecasegenerate.Request{
		Mode:             reqMode,
		APIs:             inputs,
		RequirementsPath: opts.reqPath,
		OutputDir:        opts.outputDir,
		DryRun:           opts.dryRun,
		Concurrency:      opts.concurrency,
		Progress: func(current, total int, message string) {
			if total > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  [%d/%d] %s\n", current, total, message)
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "▶ %s\n", message)
			}
		},
	})
	if err != nil {
		return err
	}
	printSummary(cmd.OutOrStdout(), res)
	return nil
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }
