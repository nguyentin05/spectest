// Package cli is the command-line adapter for SpecTest. It builds the Cobra command tree
// and delegates each subcommand to its own package under internal/adapter/cli/.
package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/nguyentin05/spectest/internal/adapter/cli/cmdutil"
	"github.com/nguyentin05/spectest/internal/adapter/cli/config"
	"github.com/nguyentin05/spectest/internal/adapter/cli/generate"
	"github.com/nguyentin05/spectest/internal/platform/buildinfo"
	"github.com/nguyentin05/spectest/internal/platform/logger"
	"github.com/spf13/cobra"
)

// NewSpectestCmd builds the root `spectest` command. It creates a shared cmdutil.Factory,
// registers the global `--verbose` and `--version` flags, defines the "core" and "settings"
// command groups shown in help output, and attaches the `generate` and `config` subcommands.
// A fresh command tree is returned on every call so tests can execute it in isolation.
func NewSpectestCmd() *cobra.Command {
	f := cmdutil.NewFactory()
	var showVersion bool

	spectestCmd := &cobra.Command{
		Use:           "spectest",
		Short:         "SpecTest: LLM-based API Testing Engine",
		SilenceErrors: true,
		Long: `SpecTest is an open-source, cost-efficient LLM test engineering engine that
converts software requirements (OpenAPI & PRD Markdown) into executable test suites,
runs multi-tier API assertions, and evolves tests when specifications change.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if f.Verbose {
				slog.SetDefault(logger.New("debug"))
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			if showVersion {
				fmt.Fprintf(cmd.OutOrStdout(), "spectest %s\n", buildinfo.App())
				return
			}
			_ = cmd.Help()
		},
	}

	spectestCmd.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "Print spectest version")
	f.BindFlags(spectestCmd.PersistentFlags())

	spectestCmd.AddGroup(&cobra.Group{
		ID:    "core",
		Title: "Core Test Engineering Commands:",
	})
	spectestCmd.AddGroup(&cobra.Group{
		ID:    "settings",
		Title: "Configuration & Workspace Commands:",
	})

	spectestCmd.AddCommand(generate.NewCommand(f))
	spectestCmd.AddCommand(config.NewCommand(f))

	return spectestCmd
}

// Execute builds and runs the root command against os.Args. On failure it prints
// "Error: <message>" to stderr and terminates the process with exit status 1.
func Execute() {
	if err := NewSpectestCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
