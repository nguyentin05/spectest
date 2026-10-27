# GitHub Actions Integration

SpecTest provides a native **Setup Composite Action** (`action.yml`) that downloads, verifies checksums, caches, installs the `spectest` CLI binary into `PATH`, and sets up LLM credentials across Linux, macOS, and Windows runners (`amd64` and `arm64`).

---

## Example 1: Run Test Suite & Upload SARIF to GitHub Code Scanning

Create `.github/workflows/api-tests.yml`:

```yaml
name: API Regression & Quality Gate

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

permissions:
  contents: read
  security-events: write # Required for Code Scanning SARIF upload

jobs:
  spectest:
    name: Run SpecTest E2E Suite
    runs-on: ubuntu-latest
    steps:
      - name: Check out code
        uses: actions/checkout@v4

      - name: Setup SpecTest-AI & Configure LLM
        uses: nguyentin05/spectest@v1
        with:
          version: "latest"
          provider: "gemini"
          api_key: ${{ secrets.GEMINI_API_KEY }}
          model: "gemini-2.5-pro"
          endpoint: "https://generativelanguage.googleapis.com/v1beta"

      - name: Execute SpecTest Suite
        run: |
          mkdir -p reports
          spectest run \
            -f tests/spectest.yaml \
            -t http://localhost:8080 \
            --report-dir reports \
            --sarif reports/spectest.sarif

      - name: Upload Test Reports
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: spectest-reports
          path: reports/

      - name: Upload SARIF to GitHub Code Scanning
        if: always()
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: reports/spectest.sarif
```

---

## Example 2: AI Auto-Update Test Suite on OpenAPI Changes

SpecTest can automatically sync test scenarios with OpenAPI spec changes in Pull Requests:

```yaml
name: Auto-Sync Test Suite

on:
  pull_request:
    paths:
      - 'openapi/**'

jobs:
  update-suite:
    name: Update Tests from OpenAPI
    runs-on: ubuntu-latest
    steps:
      - name: Check out code
        uses: actions/checkout@v4

      - name: Setup SpecTest-AI & Configure LLM
        uses: nguyentin05/spectest@v1
        with:
          provider: "openai"
          api_key: ${{ secrets.OPENAI_API_KEY }}
          model: "gpt-4o-mini"
          endpoint: "https://api.openai.com/v1"

      - name: Generate updated scenarios
        run: |
          spectest generate -f tests/spectest.yaml -a openapi/api.yaml

      - name: Commit updated tests back to PR
        uses: stefanzweifel/git-auto-commit-action@v5
        with:
          commit_message: "chore(test): auto-update spectest.yaml from OpenAPI spec changes"
          file_pattern: "tests/spectest.yaml tests/spectest.log.yaml"
```

---

## Action Reference

### Inputs

| Input | Required | Default | Description |
| :--- | :--- | :--- | :--- |
| `version` | No | `"latest"` | SpecTest binary version to install (e.g. `'v0.2.0'` or `'latest'`). |
| `provider` | No | `""` | LLM provider for AI generation and defect triage (`gemini`, `openai`, `anthropic`, `ollama`). |
| `api_key` | No | `""` | LLM API key. Pass via `secrets.*` (automatically masked in CI logs). |
| `model` | No | `""` | Default model name (e.g. `'gpt-4o-mini'`, `'gemini-2.5-pro'`, `'qwen2.5-coder:7b'`). |
| `endpoint` | No | `""` | Custom LLM base endpoint URL (e.g. `'http://localhost:11434'` for Ollama). |

### Outputs

| Output | Description |
| :--- | :--- |
| `version` | The resolved semver version of SpecTest that was installed (e.g. `'v0.2.0'`). |
| `spectest_path` | Absolute path to the directory containing the cached executable binary. |
