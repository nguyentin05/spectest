# CLI Reference: Overview

The **SpecTest CLI** (`spectest`) is the command-line entrypoint for generating test suites, executing scenarios, configuring credentials, and launching the Web UI dashboard.

---

## Command Hierarchy

```text
spectest [flags] [command]

Core Test Engineering:
  generate    Analyze OpenAPI specs & PRDs to synthesize executable test suites
  run         Execute API suites against target services with DAG setup & state-chaining

Configuration & Workspace:
  config      Manage LLM provider credentials, endpoints, and models
    ├── init  Initialize provider, endpoint, and model
    ├── list  List active configuration (secrets masked)
    └── set   Set an individual configuration key

Interactive Dashboard & Utilities:
  ui          Launch the interactive Web UI dashboard (aliases: dashboard, web)
  completion  Generate shell autocompletion script (bash, zsh, fish, powershell)
  help        Help about any command
```

---

## Global Flags

Global flags apply to all commands across the CLI:

| Flag | Short | Type | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `--verbose` | | boolean | `false` | Enable structured debug logging (`slog`) with timestamps and source code locations. |
| `--version` | `-v` | boolean | `false` | Print the current binary version, Git commit SHA, and build timestamp. |
| `--help` | `-h` | boolean | `false` | Display help and usage instructions for the command. |

---

## Multi-Target Mapping Syntax

When executing test suites with `spectest run`, target endpoints can be specified in two ways:

1. **Single Target URL**:
   Applies as the default host for all steps with relative paths:
   ```bash
   spectest run -f ./tests/spectest.yaml -t http://localhost:8080
   ```

2. **Named Server Aliases (`-t alias=url`)**:
   Maps server placeholders declared in `spectest.yaml` (e.g. `{shop}`, `{billing}`) to specific endpoints:
   ```bash
   spectest run -f ./tests/spectest.yaml \
     -t shop=http://localhost:8001 \
     -t billing=http://localhost:8002
   ```

> [!NOTE]
> The `-t / --target` flag can be passed multiple times to map multiple distinct microservice endpoints in a single test run.

---

## Environment Variables

SpecTest reads the following environment variables:

| Variable | Description |
| :--- | :--- |
| `SPECTEST_API_KEY` | Overrides the LLM API key stored in `~/.spectest/credentials.yaml`. Ideal for CI/CD environments. |
| `GEMINI_API_KEY` | Fallback key for Google Gemini provider. |
| `OPENAI_API_KEY` | Fallback key for OpenAI or OpenAI-compatible providers. |
| `SPECTEST_CONFIG_DIR` | Custom directory path for credentials storage (defaults to `$HOME/.spectest`). |

---

## Exit Codes

| Exit Code | Status | Description |
| :--- | :--- | :--- |
| `0` | **Success** | All scenarios and steps passed successfully. |
| `1` | **Failure** | One or more test steps failed, target readiness probe failed, or invalid inputs. |
| `130` | **Canceled** | Process terminated by `SIGINT` (Ctrl-C) or `SIGTERM`. Docker containers are cleanly torn down before exit. |
