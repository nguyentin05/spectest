# Quick Start Guide

Get up and running with **SpecTest** in less than 3 minutes.

---

## Prerequisites

Before starting, ensure you have:

- **Go 1.22+** installed (or download a precompiled binary from [GitHub Releases](https://github.com/nguyentin05/spectest/releases)).
- An API specification in OpenAPI 3.x format (`openapi.yaml` or `openapi.json`).
- *(Optional)* An API key for an LLM provider (Google Gemini, OpenAI) or a running local instance of [Ollama](https://ollama.ai) for scenario synthesis and AI defect triage.

---

## Step 1: Install SpecTest

=== "Go Toolchain"
    ```bash
    go install github.com/nguyentin05/spectest/cmd/spectest@latest
    spectest --version
    ```

=== "Build from Source"
    ```bash
    git clone https://github.com/nguyentin05/spectest.git
    cd spectest
    make build
    ./bin/spectest --help
    ```

=== "Docker"
    ```bash
    docker run --rm -v $(pwd):/workspace ghcr.io/nguyentin05/spectest:latest --help
    ```

---

## Step 2: Configure LLM Provider

SpecTest manages credentials securely in `~/.spectest/credentials.yaml` with strict POSIX permissions (`0600`).

=== "Google Gemini (Recommended)"
    ```bash
    spectest config init --provider gemini --model gemini-2.5-flash
    spectest config set api-key
    # Enter API Key interactively (terminal echo disabled)
    ```

=== "OpenAI"
    ```bash
    spectest config init --provider openai --endpoint https://api.openai.com/v1 --model gpt-4o-mini
    spectest config set api-key
    ```

=== "Ollama (Local / Air-Gapped)"
    ```bash
    # Pull model and serve
    ollama pull qwen2.5-coder:7b

    # Configure SpecTest to use local endpoint
    spectest config init --provider ollama --endpoint http://localhost:11434/v1 --model qwen2.5-coder:7b
    ```

Verify your active settings:
```bash
spectest config list
```

---

## Step 3: Generate Test Suite (`spectest generate`)

Analyze your OpenAPI contract and optional requirements document to synthesize an executable suite:

```bash
spectest generate -a ./examples/openapi.yaml -r ./examples/requirements.md -o ./tests
```

This creates:
- `tests/spectest.yaml`: Complete executable suite schema v1 (`servers`, `apis`, `analysis`, `seed`, `setup`, `scenarios`).
- `tests/spectest.log.yaml`: Initial base snapshot with canonical SHA-256 hash.
- `tests/test_matrix.md`: Human-readable coverage matrix mapping endpoints to scenarios.

---

## Step 4: Run the Test Suite (`spectest run`)

### 1. Single Target Execution
Execute the suite against your running backend API:

```bash
spectest run -f ./tests/spectest.yaml -t http://localhost:8080
```

### 2. Multi-Target Microservices
If your suite exercises multiple server aliases (e.g. `{shop}` and `{billing}`):

```bash
spectest run -f ./tests/spectest.yaml \
  -t shop=http://localhost:8001 \
  -t billing=http://localhost:8002
```

### 3. Automated Docker Compose Lifecycle & AI Triage
Spin up background databases or services, execute tests, and enable AI root cause analysis if any test fails:

```bash
spectest run -f ./tests/spectest.yaml \
  --compose docker-compose.test.yml \
  -t http://localhost:8080 \
  --triage
```

### 4. Run Only Changed Scenarios
After an API contract update or revision, execute only the scenarios affected by the latest update entry in `spectest.log.yaml`:

```bash
spectest run -f ./tests/spectest.yaml -t http://localhost:8080 --only-changed
```

---

## Step 5: Launch the Interactive Web UI (`spectest ui`)

Prefer an interactive browser dashboard? Launch the embedded Web UI:

```bash
spectest ui
```

Open `http://127.0.0.1:8088` to explore:
- **Workspace**: Upload OpenAPI and Markdown files.
- **Suite Editor**: Inspect scenarios, setups, and steps visually.
- **Real-Time Runner**: Trigger test runs with live Server-Sent Events (SSE) streaming logs, latency charts, and AI defect diagnoses.
- **History**: Inspect append-only audit entries and diffs.

---

## Next Steps

- Review the [spectest.yaml Suite Specification](configuration/suite-spec.md).
- Understand how [spectest.log.yaml](configuration/history-log-spec.md) audits updates and calculates SHA-256 hashes.
- Read about the [Execution Lifecycle & 9-Step HTTP Engine](concepts/execution-lifecycle.md).
- Integrate automated regression gates in [GitHub Actions](integrations/github-actions.md).
