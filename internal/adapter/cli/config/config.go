package config

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/nguyentin05/spectest/internal/adapter/cli/cmdutil"
	"github.com/nguyentin05/spectest/internal/infrastructure/storage"
	"github.com/spf13/cobra"
)

type initOptions struct {
	provider string
	model    string
	endpoint string
	apiKey   string
}

// NewCommand creates the `spectest config` command and its subcommands (`init`, `set`, `list`).
func NewCommand(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "Manage LLM credentials and global spectest configuration",
		GroupID: "settings",
		Long: `Configure LLM providers, endpoints, models, and API keys.
Credentials are saved to ~/.spectest/credentials.yaml with strict file permissions (0600).`,
		Example: `  spectest config init
  spectest config init --provider openai --endpoint https://api.openai.com/v1 --model gpt-4o-mini --api-key sk-...
  spectest config set model qwen2.5-coder:7b
  spectest config list`,
	}

	cmd.AddCommand(newInitCmd(f))
	cmd.AddCommand(newSetCmd(f))
	cmd.AddCommand(newListCmd(f))

	return cmd
}

func newInitCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &initOptions{}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize LLM provider, endpoint, model, and API key configuration",
		Example: `  spectest config init
  spectest config init --provider openai --endpoint https://api.openai.com/v1 --model gpt-4o-mini --api-key sk-...
  spectest config init --provider ollama --endpoint http://localhost:11434 --model qwen2.5-coder:7b`,
		RunE: func(cmd *cobra.Command, args []string) error {
			store := f.CredentialStore()
			out := cmd.OutOrStdout()

			hasFlags := opts.provider != "" || opts.model != "" || opts.endpoint != "" || opts.apiKey != ""
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
				if opts.apiKey != "" {
					if err := store.SetAPIKey(prov, opts.apiKey); err != nil {
						return err
					}
				}
				fmt.Fprintf(out, "Saved configuration for provider %q at %s (0600)\n", prov, store.GlobalPath())
				return nil
			}

			// Interactive wizard when no flags are provided
			reader := bufio.NewReader(cmd.InOrStdin())
			fmt.Fprintln(out, "SpecTest Configuration Setup")

			fmt.Fprint(out, "1. Provider name: ")
			prov, _ := reader.ReadString('\n')
			prov = strings.ToLower(strings.TrimSpace(prov))
			if prov == "" {
				return fmt.Errorf("provider name is required")
			}
			if err := store.SetDefaultProvider(prov); err != nil {
				return err
			}

			fmt.Fprint(out, "2. Endpoint URL: ")
			ep, _ := reader.ReadString('\n')
			ep = strings.TrimSpace(ep)
			if ep != "" {
				if err := store.SetEndpoint(prov, ep); err != nil {
					return err
				}
			}

			fmt.Fprint(out, "3. Model name: ")
			model, _ := reader.ReadString('\n')
			model = strings.TrimSpace(model)
			if model != "" {
				if err := store.SetDefaultModel(prov, model); err != nil {
					return err
				}
			}

			fmt.Fprint(out, "4. API Key (leave blank if not required): ")
			key, _ := reader.ReadString('\n')
			key = strings.TrimSpace(key)
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
	cmd.Flags().StringVar(&opts.apiKey, "api-key", "", "API key for the selected LLM provider")

	return cmd
}

func newSetCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "set [key] [value]",
		Short: "Set a specific configuration key (provider, endpoint, model, api-key)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := f.CredentialStore()
			out := cmd.OutOrStdout()
			key := strings.ToLower(strings.TrimSpace(args[0]))
			val := strings.TrimSpace(args[1])
			activeProv := store.GetDefaultProvider()

			switch key {
			case "provider", "default_provider":
				if err := store.SetDefaultProvider(val); err != nil {
					return err
				}
				fmt.Fprintf(out, "Set default provider = %q\n", val)
			case "api-key", "apikey", "api_key":
				if err := store.SetAPIKey(activeProv, val); err != nil {
					return err
				}
				fmt.Fprintf(out, "Set api-key for provider %q = %s (0600)\n", activeProv, storage.MaskKey(val))
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
			if prov == "" {
				prov = "[not set]"
			}
			if model == "" {
				model = "[not set]"
			}
			if ep == "" {
				ep = "[not set]"
			}
			fmt.Fprintln(out, "Active SpecTest Configuration:")
			fmt.Fprintf(out, "  Config File      : %s\n", store.GlobalPath())
			fmt.Fprintf(out, "  Default Provider : %s\n", prov)
			fmt.Fprintf(out, "  Active Model     : %s\n", model)
			fmt.Fprintf(out, "  Endpoint URL     : %s\n", ep)
			fmt.Fprintf(out, "  API Key          : %s\n", storage.MaskKey(key))
		},
	}
}
