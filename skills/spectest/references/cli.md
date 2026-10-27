# SpecTest CLI Command Reference

Quick reference for SpecTest CLI flags and common usage patterns.

---

## 1. `spectest generate`

Generate a new test suite (`init`) or incrementally sync an existing suite with OpenAPI changes (`update`).

```bash
spectest generate [flags]
```

### Common Flags
- `--mode <string>`: Generation mode (`init` or `update`, default `"init"`).
- `-a, --api <alias=path>`: OpenAPI document mapping (repeatable; ordering defines canonical server order in suite).
- `-o, --output <dir>`: Directory containing `spectest.yaml` and `spectest.log.yaml` (default `"./tests"`).
- `-r, --req <file>`: Path to raw business requirements Markdown file.
- `--dry-run`: Execute the generation pipeline without writing files to disk.

### Examples
```bash
# Bootstrap new suite
spectest generate --mode init -a auth=openapi-auth.yaml -o ./.spectest/

# Bootstrap new suite with multiple services and PRD requirements
spectest generate --mode init -a auth=openapi-auth.yaml -a orders=openapi-orders.yaml -r requirements.md -o ./.spectest/

# Incremental update dry-run preview
spectest generate --mode update -a auth=openapi-auth.yaml -o ./.spectest/ --dry-run

# Apply incremental update
spectest generate --mode update -a auth=openapi-auth.yaml -o ./.spectest/
```

---

## 2. `spectest run`

Execute an autonomous API test suite against target endpoints with setup DAG resolution, response extraction, and reporting.

```bash
spectest run [flags]
```

### Common Flags
- `-c, --compose <path>`: Path to Docker Compose file (automatically boots services, waits for readiness, runs tests, and tears down).
- `-f, --file <path>`: Path to the test suite YAML (default `"./tests/spectest.yaml"`).
- `-t, --target <string>`: Target base URL (single URL or alias mapping e.g. `-t auth=http://localhost:8001`, repeatable).
- `--log <path>`: Path to `spectest.log.yaml` (defaults to adjacent to `--file`).
- `--only-changed`: Run only scenarios affected by the latest history update entry.
- `--triage`: Enable AI defect triage on step failure (requires configured LLM provider).
- `--report-dir <dir>`: Directory for test output reports (default `"./reports"`).
- `--sarif <path>`: Path to output SARIF 2.1.0 report for GitHub Code Scanning / SonarQube.
- `--timeout <int>`: Server readiness timeout in seconds (default `30`).

### Examples
```bash
# Run against a live endpoint
spectest run -f ./.spectest/spectest.yaml -t http://localhost:8000

# Run with automated Compose lifecycle and multiple services
spectest run -c ./.spectest/docker-compose.test.yml -f ./.spectest/spectest.yaml -t auth=http://localhost:8001 -t orders=http://localhost:8002

# Run targeted regression for changed endpoints
spectest run -c ./.spectest/docker-compose.test.yml -f ./.spectest/spectest.yaml -t auth=http://localhost:8001 --only-changed

# Run with AI defect triage and SARIF output
spectest run -c ./.spectest/docker-compose.test.yml -f ./.spectest/spectest.yaml -t auth=http://localhost:8001 --triage --sarif reports/spectest.sarif
```

---

## 3. `spectest mcp serve`

Start the Model Context Protocol (MCP) server over standard input/output (stdio) for AI coding assistants (Claude Code, Cursor, Cline).

```bash
spectest mcp serve [flags]
```

### Flags
- `--root <dir>`: Root workspace boundary path (defaults to current working directory). Access outside `--root` is strictly prohibited.
