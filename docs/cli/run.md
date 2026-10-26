# `spectest run`

Execute `spectest.yaml` suites against target APIs with setup DAG dependency resolution, runtime state chaining, multi-target support, and automated AI defect triage.

---

## Synopsis

```bash
spectest run [flags]
```

---

## Description

The `run` command executes scenarios sequentially:
1. **Target Validation & Readiness Probing**: Normalizes single target URLs or multiple named aliases (`-t alias=url`), booting Docker Compose if configured and pinging every target until healthy.
2. **Setup DAG Resolution**: Dynamically resolves dependency hierarchies (`use: [...]`), checks for cycles, and runs setup steps before scenario steps.
3. **Dynamic State Chaining**: Replaces `{{variable}}` and `{{random}}` placeholders in paths, headers, and request bodies with extracted state values.
4. **Step Execution**: Sends HTTP requests, bounds response payloads to 1MB, checks status codes, and extracts variables via `save`.
5. **AI Defect Triage**: If `--triage` is enabled and any step fails, captures request/response snapshots and triggers LLM Root Cause Analysis.

---

## Flags & Options

| Flag | Short | Type | Default | Required | Description |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `--file` | `-f` | string | `"./tests/spectest.yaml"` | No | Path to the test suite YAML file. |
| `--target` | `-t` | stringArray | `[]` | No | Target base URL (single URL or alias mapping e.g. `-t auth=http://localhost:8001`). Can be repeated. |
| `--compose` | `-c` | string | `""` | No | Path to Docker Compose file. Automatically handles boot, healthcheck, and teardown. |
| `--timeout` | | int | `30` | No | Server readiness healthcheck timeout in seconds. |
| `--triage` | | boolean | `false` | No | Enable AI defect triage on step failure (requires configured LLM). |
| `--only-changed` | | boolean | `false` | No | Run only the scenarios affected by the latest history update in `spectest.log.yaml`. |
| `--log` | | string | `""` | No | Path to `spectest.log.yaml`; defaults to the directory beside `--file`. |
| `--report-dir` | | string | `"./reports"` | No | Directory to output test reports (`junit.xml`, `summary.json`, `report.html`). |
| `--verbose` | | boolean | `false` | No | Enable detailed debug logs for all HTTP requests and response payloads. |

---

## Examples

### 1. Run Against Local Server
```bash
spectest run -f ./tests/spectest.yaml -t http://localhost:8080
```

### 2. Multi-Target Microservice Execution
Map multiple server aliases declared in `spectest.yaml`:
```bash
spectest run -f ./tests/spectest.yaml \
  -t shop=http://localhost:8001 \
  -t billing=http://localhost:8002
```

### 3. Run with Automated Docker Compose & AI Triage
Spin up backend containers, run tests, diagnose any failures with LLM root cause analysis, and cleanly tear down:
```bash
spectest run -f ./tests/spectest.yaml \
  --compose docker-compose.test.yml \
  -t http://localhost:8080 \
  --triage
```

### 4. Run Only Changed Scenarios
Execute only the scenarios impacted by the most recent update entry in `spectest.log.yaml`:
```bash
spectest run -f ./tests/spectest.yaml -t http://localhost:8080 --only-changed
```

---

## Output Reports

Upon completion, test reports are generated in `--report-dir`:
- **`summary.json`**: Structured JSON report with duration, checks evaluated, and per-step records.
- **`junit.xml`**: Standard JUnit XML format compatible with CI runners (GitHub Actions, GitLab CI, Jenkins).
- **`report.html`**: Standalone HTML dashboard with execution timelines, latency percentiles, and AI triage findings.
