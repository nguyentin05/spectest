# Run: Safe Test Execution and Targeted Regression

**Objective:** Execute API test scenarios safely against isolated target environments with optional Docker Compose lifecycle management, targeted regression, and AI triage.

---

## 1. Pre-Run Checklist

Always execute `scripts/preflight.sh run [workspace]` and confirm all five prerequisites before running:
1. `spectest.yaml` exists.
2. An isolated test Docker Compose file exists and is confirmed safe.
3. Every server alias has a confirmed runtime base URL in `config.yaml` or verified by the user.
4. The Compose configuration does not bind to development/production databases or volumes.
5. **The user has explicitly requested test execution.** Do not trigger runs automatically without user intent.

If the suite or Compose file is missing, return to `references/prepare.md` or `references/generate.md`.

---

## 2. Dual-Path Execution Routing

### MCP Path (Preferred for Agents)

Start asynchronous execution via `start_run`:
```json
{
  "suite_path": ".spectest/spectest.yaml",
  "mode": "full",
  "targets": {
    "auth": "http://localhost:8000"
  },
  "compose_file": ".spectest/docker-compose.test.yml",
  "triage": false
}
```

1. Retrieve the `job_id` from the response.
2. Poll `get_job` with `{"job_id": "<job_id>"}` until `status == "succeeded"` or `"failed"`.
3. If the user requests cancellation or the run hangs, call `cancel_job` with `{"job_id": "<job_id>"}`.
4. Read results with `read_report`:
   ```json
   { "report_path": "reports/report.json" }
   ```

---

### CLI Path

Always supply both `-c` (Compose) and `-f` (Suite file) explicitly:
```bash
spectest run \
  -c ./.spectest/docker-compose.test.yml \
  -f ./.spectest/spectest.yaml \
  -t auth=http://localhost:8000
```

- **`-c, --compose`**: Path to the isolated test Compose file. SpecTest automatically starts services, healthchecks endpoints, executes scenarios, and cleanly tears down containers.
- **`-f, --file`**: Path to the test suite YAML.
- **`-t, --target`**: Repeatable flag for server base URLs (`-t alias=url`). Never infer runtime URLs from alias names.
- **`--log`**: Path to `spectest.log.yaml`. Required only if the log file is not placed adjacent to the suite file.
- **`--sarif`**: Optional path to write a SARIF 2.1.0 security report for GitHub Code Scanning.

---

## 3. Targeted Regression (`--only-changed`)

When an incremental update has been applied, do not blindly re-run the entire test suite if only specific endpoints were modified:
- **MCP:** Set `"mode": "only_changed"` in `start_run`.
- **CLI:** Add the `--only-changed` flag:
  ```bash
  spectest run \
    -c ./.spectest/docker-compose.test.yml \
    -f ./.spectest/spectest.yaml \
    -t auth=http://localhost:8000 \
    --only-changed
  ```

*Note:* `--only-changed` specifically executes scenarios that were added, repaired, or directly impacted by the most recent history log revision entry.

---

## 4. AI Defect Triage (`--triage`)

- **Default State:** Triage is **disabled** by default to save LLM tokens.
- **Enable Only When:**
  1. The user explicitly requests automated root-cause failure analysis.
  2. A valid LLM provider (Gemini, OpenAI, Anthropic, or Ollama) is configured.
- **Missing Configuration:** If the user requests triage but no LLM is configured, explain what is missing (`spectest config set-llm`) and ask if they wish to configure it first. Do not attempt to run triage with unconfigured providers.
- **Credential Safety:** Never change LLM providers, models, or API keys without explicit user approval.

---

## 5. Teardown Safety & Volume Persistence

SpecTest manages the Docker Compose lifecycle automatically:
- Boots containers before starting scenario steps.
- Waits for endpoint readiness based on `--timeout` (default 30s).
- Evaluates setup DAGs and scenario assertions.
- **Tears down the Compose environment and purges ephemeral volumes.**

Because volumes may be removed during teardown:
- Never use a development or production Compose file.
- Never mount host directories containing persistent databases.
- If true environment isolation cannot be guaranteed, halt and ask the user before running.

---

## 6. Post-Run Checklist

1. Capture process exit code and console output.
2. Read the structured test report from the report directory (e.g., `reports/report.json` or `read_report` in MCP). Do not guess report content without reading the actual file.
3. If any scenarios failed, immediately transition to **`references/analyze.md`** to classify the failure.
4. Never declare that a test run passed without reading and verifying actual results.
