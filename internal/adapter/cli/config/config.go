// Package config implements the `spectest config` CLI subcommands (`init`, `set`, `list`)
// for managing LLM provider settings. API keys are only accepted through a hidden prompt
// or environment variables and are never passed as command-line arguments.
package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/mattn/go-isatty"
	"github.com/nguyentin05/spectest/internal/adapter/cli/cmdutil"
	"github.com/nguyentin05/spectest/internal/infrastructure/storage"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

// initOptions holds the non-secret flag values accepted by `spectest config init`.
type initOptions struct {
	provider string
	model    string
	endpoint string
}

// NewCommand creates the `spectest config` command and its subcommands (`init`, `set`, `list`).
func NewCommand(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "Manage LLM credentials and global spectest configuration",
		GroupID: "settings",
		Long: `Configure LLM providers, endpoints, models, and API keys securely.
Credentials are saved to ~/.spectest/credentials.yaml with strict file permissions (0600).
API keys are read interactively with terminal echo disabled and are never passed via command-line flags.`,
		Example: `  spectest config init
  spectest config init --provider openai --endpoint https://api.openai.com/v1 --model gpt-4o-mini
  spectest config set model qwen2.5-coder:7b
  spectest config set api-key
  spectest config list`,
	}

	cmd.AddCommand(newInitCmd(f))
	cmd.AddCommand(newSetCmd(f))
	cmd.AddCommand(newListCmd(f))

	return cmd
}

// newInitCmd creates `spectest config init`. With `--provider/--endpoint/--model` flags it
// saves those values directly and only prompts for the API key when stdin is a terminal;
// without flags it runs an `aws configure`-style prompt showing current values in brackets.
func newInitCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &initOptions{}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize LLM provider, endpoint, model, and API key configuration",
		Example: `  spectest config init
  spectest config init --provider openai --endpoint https://api.openai.com/v1 --model gpt-4o-mini
  spectest config init --provider ollama --endpoint http://localhost:11434 --model qwen2.5-coder:7b`,
		RunE: func(cmd *cobra.Command, args []string) error {
			store := f.CredentialStore()
			in := cmd.InOrStdin()
			out := cmd.OutOrStdout()
			reader := bufio.NewReader(in)

			hasFlags := opts.provider != "" || opts.model != "" || opts.endpoint != ""
			if hasFlags {
				prov := strings.ToLower(strings.TrimSpace(opts.provider))
				if prov == "" {
					prov = store.GetDefaultProvider()
				}
				if prov == "" {
					return fmt.Errorf("--provider is required when initializing configuration")
				}
				if err := store.SetDefaultProvider(prov); err != nil {
					return err
				}
				if opts.endpoint != "" {
					if err := store.SetEndpoint(prov, opts.endpoint); err != nil {
						return err
					}
				}
				if opts.model != "" {
					if err := store.SetDefaultModel(prov, opts.model); err != nil {
						return err
					}
				}
				if isTerminalReader(in) {
					currentKey, _ := store.GetAPIKey(prov)
					fmt.Fprintf(out, "API Key [%s]: ", storage.MaskKey(currentKey))
					key, err := readSecretLine(in, out, reader)
					if err != nil {
						return err
					}
					if key != "" {
						if err := store.SetAPIKey(prov, key); err != nil {
							return err
						}
					}
				}
				fmt.Fprintf(out, "Saved configuration for provider %q at %s (0600)\n", prov, store.GlobalPath())
				return nil
			}

			currentProv := store.GetDefaultProvider()
			fmt.Fprintf(out, "LLM Provider [%s]: ", defaultLabel(currentProv))
			prov, _ := reader.ReadString('\n')
			prov = strings.ToLower(strings.TrimSpace(prov))
			if prov == "" {
				prov = currentProv
			}
			if prov == "" {
				return fmt.Errorf("provider name is required")
			}
			if err := store.SetDefaultProvider(prov); err != nil {
				return err
			}

			currentEP, _ := store.GetEndpoint(prov)
			fmt.Fprintf(out, "Endpoint URL [%s]: ", defaultLabel(currentEP))
			ep, _ := reader.ReadString('\n')
			ep = strings.TrimSpace(ep)
			if ep != "" {
				if err := store.SetEndpoint(prov, ep); err != nil {
					return err
				}
			}

			currentModel := store.GetDefaultModel(prov)
			fmt.Fprintf(out, "Model Name [%s]: ", defaultLabel(currentModel))
			model, _ := reader.ReadString('\n')
			model = strings.TrimSpace(model)
			if model != "" {
				if err := store.SetDefaultModel(prov, model); err != nil {
					return err
				}
			}

			currentKey, _ := store.GetAPIKey(prov)
			fmt.Fprintf(out, "API Key [%s]: ", storage.MaskKey(currentKey))
			key, err := readSecretLine(in, out, reader)
			if err != nil {
				return err
			}
			if key != "" {
				if err := store.SetAPIKey(prov, key); err != nil {
					return err
				}
			}

			fmt.Fprintf(out, "Saved configuration at %s (0600)\n", store.GlobalPath())
			return nil
		},
	}

	cmd.Flags().StringVar(&opts.provider, "provider", "", "LLM provider identifier")
	cmd.Flags().StringVar(&opts.model, "model", "", "Model name for the selected provider")
	cmd.Flags().StringVar(&opts.endpoint, "endpoint", "", "Base endpoint URL for the LLM API")

	return cmd
}

// newSetCmd creates `spectest config set <key> [value]`. `provider`, `endpoint`, and `model`
// take their value as an argument; `api-key` rejects arguments and reads from a hidden prompt.
func newSetCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "set [key] [value]",
		Short: "Set a configuration key (provider, endpoint, model, or api-key)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := f.CredentialStore()
			in := cmd.InOrStdin()
			out := cmd.OutOrStdout()
			key := strings.ToLower(strings.TrimSpace(args[0]))
			activeProv := store.GetDefaultProvider()

			if key == "api-key" || key == "apikey" || key == "api_key" {
				if len(args) > 1 {
					return fmt.Errorf("for security, do not pass api-key on the command line; run 'spectest config set api-key' and enter it at the hidden prompt")
				}
				reader := bufio.NewReader(in)
				currentKey, _ := store.GetAPIKey(activeProv)
				fmt.Fprintf(out, "API Key [%s]: ", storage.MaskKey(currentKey))
				secret, err := readSecretLine(in, out, reader)
				if err != nil {
					return err
				}
				if secret == "" {
					return fmt.Errorf("api-key cannot be empty")
				}
				if err := store.SetAPIKey(activeProv, secret); err != nil {
					return err
				}
				fmt.Fprintf(out, "Saved api-key for provider %q (%s, 0600)\n", activeProv, storage.MaskKey(secret))
				return nil
			}

			if len(args) < 2 {
				return fmt.Errorf("missing value for key %q (usage: spectest config set %s <value>)", key, key)
			}
			val := strings.TrimSpace(args[1])

			switch key {
			case "provider", "default_provider":
				if err := store.SetDefaultProvider(val); err != nil {
					return err
				}
				fmt.Fprintf(out, "Set default provider = %q\n", val)
			case "endpoint":
				if err := store.SetEndpoint(activeProv, val); err != nil {
					return err
				}
				fmt.Fprintf(out, "Set endpoint for provider %q = %q\n", activeProv, val)
			case "model":
				if err := store.SetDefaultModel(activeProv, val); err != nil {
					return err
				}
				fmt.Fprintf(out, "Set model for provider %q = %q\n", activeProv, val)
			default:
				return fmt.Errorf("unknown configuration key %q (valid keys: provider, endpoint, model, api-key)", key)
			}
			return nil
		},
	}
}

// newListCmd creates `spectest config list`, which prints the resolved provider, model,
// endpoint, and a masked API key for the active configuration.
func newListCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List active configuration settings (API keys are masked)",
		Run: func(cmd *cobra.Command, args []string) {
			store := f.CredentialStore()
			out := cmd.OutOrStdout()
			prov := store.GetDefaultProvider()
			key, _ := store.GetAPIKey(prov)
			ep, _ := store.GetEndpoint(prov)
			model := store.GetDefaultModel(prov)
			fmt.Fprintln(out, "Active SpecTest Configuration:")
			fmt.Fprintf(out, "  Config File      : %s\n", store.GlobalPath())
			fmt.Fprintf(out, "  Default Provider : %s\n", defaultLabel(prov))
			fmt.Fprintf(out, "  Active Model     : %s\n", defaultLabel(model))
			fmt.Fprintf(out, "  Endpoint URL     : %s\n", defaultLabel(ep))
			fmt.Fprintf(out, "  API Key          : %s\n", storage.MaskKey(key))
		},
	}
}

// defaultLabel returns val, or "None" when val is blank, for display in prompts and listings.
func defaultLabel(val string) string {
	if strings.TrimSpace(val) == "" {
		return "None"
	}
	return val
}

// isTerminalReader reports whether in is an *os.File attached to an interactive terminal.
func isTerminalReader(in io.Reader) bool {
	f, ok := in.(*os.File)
	return ok && isatty.IsTerminal(f.Fd())
}

// readSecretLine reads a line from in with terminal ECHO disabled when in is a TTY,
// preventing sensitive API keys from appearing on the terminal screen.
func readSecretLine(in io.Reader, out io.Writer, reader *bufio.Reader) (string, error) {
	f, ok := in.(*os.File)
	if !ok || !isatty.IsTerminal(f.Fd()) {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}
		return strings.TrimSpace(line), nil
	}

	fd := int(f.Fd())
	oldState, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return "", readErr
		}
		return strings.TrimSpace(line), nil
	}

	newState := *oldState
	newState.Lflag &^= unix.ECHO
	newState.Lflag |= unix.ICANON | unix.ISIG
	newState.Iflag |= unix.ICRNL
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &newState); err != nil {
		return "", err
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer func() {
		signal.Stop(sigCh)
		_ = unix.IoctlSetTermios(fd, unix.TCSETS, oldState)
		fmt.Fprintln(out)
	}()
	go func() {
		if _, ok := <-sigCh; ok {
			_ = unix.IoctlSetTermios(fd, unix.TCSETS, oldState)
			os.Exit(130)
		}
	}()

	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
