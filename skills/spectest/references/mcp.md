# SpecTest MCP Tools & Resources Reference

## Read-Only Tools

### `spectest_status`
Checks presence of suite and audit log files, and whether LLM capabilities are configured.
- `output_dir` (string, optional, default: `"tests"`): Directory containing test files relative to server root.

### `inspect_suite`
Reads canonical suite details, scenario list, setup list, current hash, and audit history.
- `suite_path` (string, optional, default: `"tests/spectest.yaml"`): Path to suite file.

### `validate_suite`
Validates structure, syntax, setup DAG, and semantics.
- `suite_path` (string, optional, default: `"tests/spectest.yaml"`): Path to suite file.

### `plan_update`
Calculates deterministic API diff, update impact, and planned actions without mutating disk or calling LLM.
- `output_dir` (string, optional, default: `"tests"`)
- `apis` (array of `{"server": "...", "path": "..."}`, required)
- `requirements_path` (string, optional)

## Side-Effect Tools

### `start_update`
Executes incremental update, repairs setups/scenarios, synthesizes new scenarios, and appends to audit log.
- `output_dir` (string, optional, default: `"tests"`)
- `apis` (array of `{"server": "...", "path": "..."}`, required)
- `requirements_path` (string, optional)

### `start_run`
Executes test suite against targets with isolated scopes and DAG resolution.
- `suite_path` (string, optional, default: `"tests/spectest.yaml"`)
- `mode` (string, `"only_changed"` | `"full"`, default: `"only_changed"`)
- `targets` (map of server alias to base URL)
- `compose_file` (string, optional)
- `triage` (boolean, default: false)
- `report_dir` (string, default: `"reports"`)
- `wait_timeout_seconds` (number, default: 0)

### `get_job`
Polls status and progress of an asynchronous job.
- `job_id` (string, required)

### `cancel_job`
Cancels an active background job.
- `job_id` (string, required)

### `read_report`
Reads test report with credentials, headers, and tokens redacted.
- `report_path` (string, default: `"reports/report.json"`)

## Standard Error Codes

- `invalid_input`: Missing or malformed parameters.
- `path_outside_root`: Path attempts to traverse or escape server `--root`.
- `not_found`: Requested file or job not found.
- `integrity_conflict`: `spectest.yaml` does not match latest hash in `spectest.log.yaml`.
- `llm_required`: Update requires LLM synthesis but LLM is not configured.
- `locked_entity`: Action attempted to mutate a locked scenario or setup.
- `job_not_found`: Job ID does not exist in memory.
- `job_failed`: Execution failed.
- `cancelled`: Job was cancelled by user or context.
