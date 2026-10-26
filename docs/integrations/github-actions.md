# GitHub Actions Integration

SpecTest provides a native **Composite GitHub Action** (`action.yml`) that runs in GitHub Actions workflows on Linux and macOS runners (`amd64` and `arm64`).

---

## Example Workflow

Create `.github/workflows/api-tests.yml`:

```yaml
name: API Regression & Contract Quality Gate

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  spectest:
    name: Run SpecTest E2E Suite
    runs-on: ubuntu-latest
    steps:
      - name: Check out code
        uses: actions/checkout@v4

      - name: Execute SpecTest API Suite
        uses: nguyentin05/spectest@v1
        with:
          test_file: "tests/spectest.yaml"
          target_url: "http://localhost:8080"
          gemini_api_key: ${{ secrets.GEMINI_API_KEY }}
          report_dir: "reports"

      - name: Publish Test Results
        uses: EnricoMi/publish-unit-test-result-action@v2
        if: always()
        with:
          files: "reports/junit.xml"

      - name: Upload HTML Test Report
        uses: actions/upload-artifact@v4
        if: always()
        with:
          name: spectest-report
          path: reports/
```

---

## Action Inputs Reference

| Input | Required | Default | Description |
| :--- | :--- | :--- | :--- |
| `version` | No | `"latest"` | SpecTest binary version to install (e.g. `'v1.0.0'` or `'latest'`). |
| `test_file` | No | `"tests/spectest.yaml"` | Path to the test suite YAML file in your repository. |
| `target_url` | No | `""` | Target API base URL. Overrides default target. |
| `compose_file` | No | `""` | Path to `docker-compose.yml` to automatically start/stop backend services. |
| `gemini_api_key`| No | `""` | Google Gemini API key for automated defect triage on failed assertions. |
| `openai_api_key`| No | `""` | OpenAI API key alternative. |
| `report_dir` | No | `"reports"` | Output directory where `junit.xml` and `report.html` are generated. |

---

## Action Outputs Reference

| Output | Description |
| :--- | :--- |
| `junit_report` | Path to the generated JUnit XML file (`reports/junit.xml`). |
| `html_report` | Path to the generated HTML report file (`reports/report.html`). |
| `test_status` | Final execution status: `'success'` or `'failure'`. |

---

## Multi-Container CI Testing with Docker Compose

If your API backend requires backing databases (Postgres, MongoDB, Redis) and application containers:

```yaml
      - name: Run SpecTest with Auto Docker Lifecycle
        uses: nguyentin05/spectest@v1
        with:
          test_file: "tests/spectest.yaml"
          compose_file: "docker-compose.test.yml"
          target_url: "http://localhost:8080"
          gemini_api_key: ${{ secrets.GEMINI_API_KEY }}
```

SpecTest automatically handles:
1. Booting all containers with `docker compose up -d`.
2. Probing until target endpoints are healthy.
3. Executing tests, evaluating assertions, and triaging failures with AI.
4. Guaranteed cleanup (`docker compose down -v`) upon workflow finish or cancellation.
