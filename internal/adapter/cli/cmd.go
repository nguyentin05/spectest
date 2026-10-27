// Package cli is the command-line adapter for SpecTest. It builds the Cobra command tree
// and delegates each subcommand to its own package under internal/adapter/cli/.
package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nguyentin05/spectest/internal/adapter/cli/cmdutil"
	"github.com/nguyentin05/spectest/internal/adapter/cli/config"
	"github.com/nguyentin05/spectest/internal/adapter/cli/generate"
	climcp "github.com/nguyentin05/spectest/internal/adapter/cli/mcp"
	"github.com/nguyentin05/spectest/internal/adapter/cli/run"
	"github.com/nguyentin05/spectest/internal/adapter/cli/ui"
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
generates an initial API test suite from OpenAPI documents and optional raw
requirements, and manages the resulting suite, log, and track files along with
their configuration. Incremental test updates are reserved for a future release
and are not implemented yet.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if f.Verbose {
				slog.SetDefault(logger.New("debug"))
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			if showVersion {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "spectest %s\n", buildinfo.App())
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
	spectestCmd.AddCommand(run.NewCommand(f))
	spectestCmd.AddCommand(config.NewCommand(f))
	spectestCmd.AddCommand(ui.NewCommand(f))
	spectestCmd.AddCommand(climcp.NewCommand())

	return spectestCmd
}

// Execute builds and runs the root command against os.Args with a signal-aware
// context: SIGINT (Ctrl-C) or SIGTERM cancels in-flight work and the process
// exits with status 130. Any other failure prints "Error: <message>" to stderr
// and terminates the process with exit status 1.
func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := NewSpectestCmd().ExecuteContext(ctx)
	switch exitCode(err) {
	case 0:
		return
	case 130:
		_, _ = fmt.Fprintln(os.Stderr, "Canceled.")
		os.Exit(130)
	default:
		_, _ = fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// exitCode maps a command error to a process exit code.
func exitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, context.Canceled):
		return 130
	default:
		return 1
	}
}
