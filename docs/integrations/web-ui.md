# Web UI Dashboard

SpecTest features an embedded, high-performance Web UI dashboard built with React, TypeScript, and Vite. It provides full visual control over workspaces, test generation, live execution streaming, and failure diagnosis.

---

## Starting the Dashboard

Launch the UI locally:

```bash
spectest ui
```

By default, the web server starts at `http://127.0.0.1:8088` and automatically opens your default browser.

---

## Key Dashboard Modules

### 1. Workspace Manager (`/workspace`)
- Browse uploaded files in your local project workspace.
- Upload OpenAPI specifications (`.yaml`/`.json`) and requirement documents (`.md`).
- Inspect API catalog statistics (total endpoints, methods, tagged groups).

### 2. Generator Studio (`/generate`)
- Visually configure generation parameters: select input files, concurrency level, and target output directory.
- Preview synthesized business scenarios, setup chains, and steps before writing to `spectest.yaml`.

### 3. Suite Editor (`/suite`)
- Interactive visual inspector for `spectest.yaml`.
- Inspect scenario cards, setup dependencies, HTTP steps, headers, payloads, and `save` variable mappings.
- View test matrix coverage tables linking OpenAPI operations to scenarios.

### 4. Real-Time Runner (`/run`)
- Configure target URLs (single URL or `-t alias=url` mappings) and Docker Compose files.
- Launch test executions with a single click.
- **Live SSE Streaming**: Telemetry stream pushes real-time events on every step:
  - Step execution status badges (`200 OK`, `400 Bad Request`).
  - Response latency gauges and timeline visualizations.
  - Setup step vs. scenario step execution logs.

### 5. Failure Inspection & AI Triage
- When a step fails, click to view the failed assertion details.
- Inspect sanitized request payloads, response bodies, and headers.
- Trigger on-demand **AI Defect Triage** to receive root cause explanations and suggested remediations directly in the UI.

### 6. History & Diffs (`/history`)
- Browse the append-only history entries recorded in `spectest.log.yaml`.
- Inspect side-by-side YAML diffs across test revisions.
- Verify canonical SHA-256 suite hashes.

---

## Headless & Remote Access

To bind the Web UI inside a Docker container, remote VM, or devbox:

```bash
spectest ui --host 0.0.0.0 --port 8088 --no-open
```
Access via `http://<server-ip>:8088` or port-forward over SSH.
