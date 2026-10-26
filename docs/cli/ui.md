# `spectest ui`

Launch the interactive, local Web UI dashboard for SpecTest.

---

## Synopsis

```bash
spectest ui [flags]
```

**Aliases:** `dashboard`, `web`

---

## Description

The `ui` command launches an embedded web server delivering a modern, responsive web dashboard built with React and TypeScript.

### Key Capabilities:
- **Workspace File Management**: Upload and browse OpenAPI specifications and PRD Markdown documents.
- **Visual Suite Inspector**: View and edit scenarios, setup blocks, and HTTP steps with syntax highlighting.
- **Real-Time Execution Streaming**: Trigger test runs with live Server-Sent Events (SSE) telemetry, live step latency charts, and instant pass/fail badges.
- **Interactive Defect Diagnosis**: Inspect full request and response diffs, review failed assertions, and trigger AI defect triage from the browser.
- **History & Diff Viewer**: Track append-only history entries from `spectest.log.yaml` and inspect cryptographic suite hashes.

---

## Flags & Options

| Flag | Short | Type | Default | Required | Description |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `--host` | | string | `"127.0.0.1"` | No | Host address to bind the web server. |
| `--port` | `-p` | int | `8088` | No | Port number to listen on. |
| `--no-open` | | boolean | `false` | No | Prevent the CLI from automatically launching your default web browser. |
| `--verbose` | | boolean | `false` | No | Enable verbose HTTP logging for the dashboard server. |

---

## Examples

### 1. Default Startup
Launch the UI and open the default browser at `http://127.0.0.1:8088`:
```bash
spectest ui
```

### 2. Custom Port & Headless Access
Run on port 9090 without launching a local browser (useful for remote development or SSH port forwarding):
```bash
spectest ui --port 9090 --no-open
```

### 3. Expose to Private Network
Bind to `0.0.0.0` to access the dashboard across a local development network:
```bash
spectest ui --host 0.0.0.0 --port 8088 --no-open
```
