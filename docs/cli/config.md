# `spectest config`

Manage LLM provider credentials, API endpoints, model identifiers, and security settings.

---

## Synopsis

```bash
spectest config [command]
```

Available subcommands:
- `init`: Initialize or overwrite LLM configuration settings.
- `list`: Inspect active configuration (sensitive keys are masked).
- `set`: Update an individual configuration key.

---

## Security Architecture

SpecTest enforces strict credential protection:
- **Strict POSIX Permissions**: Configurations are saved to `~/.spectest/credentials.yaml` with mode `0600` (`-rw-------`), restricting read and write access strictly to the owner.
- **Parent Directory Protection**: `~/.spectest/` is created with mode `0700` (`drwx------`).
- **No Command-Line Leakage**: API keys cannot be passed via command-line flags, preventing exposure in process listings (`ps aux`), system telemetry, or shell history.
- **Masked Terminal Input**: Setting API keys interactively prompts with terminal echo disabled.

---

## Subcommands

### `spectest config init`

Initialize the primary LLM provider and credentials interactively or via flags.

```bash
spectest config init [flags]
```

#### Flags
| Flag | Type | Description |
| :--- | :--- | :--- |
| `--provider` | string | LLM provider identifier (`gemini`, `openai`, `ollama`). |
| `--endpoint` | string | Base URL endpoint for the LLM API. |
| `--model` | string | Model name identifier (e.g. `gemini-2.5-flash`, `gpt-4o-mini`, `qwen2.5-coder:7b`). |

#### Examples
```bash
# Setup Google Gemini
spectest config init --provider gemini --model gemini-2.5-flash

# Setup OpenAI
spectest config init --provider openai --endpoint https://api.openai.com/v1 --model gpt-4o-mini

# Setup local Ollama (Offline / Air-Gapped)
spectest config init --provider ollama --endpoint http://localhost:11434/v1 --model qwen2.5-coder:7b
```

---

### `spectest config list`

List all active configuration properties. Sensitive tokens are automatically masked.

```bash
spectest config list
```

#### Sample Output:
```text
ACTIVE CONFIGURATION:
  Provider:      gemini
  Model:         gemini-2.5-flash
  Endpoint:      https://generativelanguage.googleapis.com/v1beta/openai/
  API Key:       AIzaSy************************3X81
  Storage:       ~/.spectest/credentials.yaml (0600)
```

---

### `spectest config set`

Update a specific configuration property without re-initializing the entire configuration.

```bash
spectest config set <key> [value]
```

#### Supported Keys:
- `provider`: Target LLM provider (`gemini`, `openai`, `ollama`).
- `endpoint`: API base URL for custom or local endpoints.
- `model`: Model identifier.
- `api-key`: Prompts for the API key with terminal echo disabled (omitting the value argument).

#### Examples:
```bash
# Switch active model
spectest config set model gemini-1.5-pro

# Set custom local endpoint
spectest config set endpoint http://10.0.0.5:8000/v1

# Securely set API key with masked input prompt
spectest config set api-key
# > Enter API Key: ************************
```
