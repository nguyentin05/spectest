<div align="center">

<img src="assets/banner_black.png#gh-light-mode-only" alt="SpecTest" width="400">
<img src="assets/banner_white.png#gh-dark-mode-only" alt="SpecTest" width="400">

### LLM-based API Testing Engine for Multi-Platform Workflows

*Automate end-to-end API testing powered by cost-optimized LLMs deployed across multiple platforms.*

[![License](https://img.shields.io/badge/license-Apache--2.0-6d5bff?style=flat-square)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square)](go.mod)
[![Docs](https://img.shields.io/badge/docs-live-6d5bff?style=flat-square)](https://nguyentin05.github.io/spectest/)
[![CI](https://img.shields.io/badge/CI-Passing-brightgreen?style=flat-square&logo=githubactions)](https://github.com/nguyentin05/spectest/actions/workflows/ci.yml)
[![Release](https://img.shields.io/badge/Release-v1.0.0-blue?style=flat-square&logo=git)](https://github.com/nguyentin05/spectest/releases)
[![Discussions](https://img.shields.io/badge/Community-Discussions-purple?style=flat-square&logo=github)](https://github.com/nguyentin05/spectest/discussions)

</div>

> **TL;DR:** **SpecTest** is an open-source, cost-efficient AI test engineering engine that turns API specs (OpenAPI specifications + PRD) into executable test suites, catches real backend bugs, and automatically updates tests when your API changes. By offloading deterministic work to Go (**Token-Optimized First**) and supporting both Cloud LLMs and local models, it cuts AI costs by up to 90% and runs anywhere from a single binary: **Local CLI & Web UI, CI/CD Pipelines, On-Premise Servers, and AI Agents (MCP)**.

## What is SpecTest?

SpecTest is an open-source API test engineering engine that automates the E2E testing workflow. It converts OpenAPI specs and PRD documents into test scenarios, generates test data, executes deep API checks, diagnoses failures, and updates tests when APIs change. Packaged as a single Go binary around one portable spectest.yaml file, the same core engine runs across local CLI and the built-in Web UI, CI/CD pipelines, on-premise webhook servers, and Agentic AI.

To keep AI costs low and test runs predictable, SpecTest offloads all deterministic work to Go. Spec compression, auth-token wiring, boundary and security fuzzing, assertion evaluation, and API version diffing run natively with zero LLM tokens. LLMs like cloud APIs or local models are called only where semantic reasoning matters: designing business test cases, explaining failed runs, and patching steps affected by API changes.

## Core Capabilities

### Test Scenarios Generation

- **Token-optimized preprocessing** strips redundant metadata, normalizes specifications, and uses regex detection to focus the context on core logic while maintaining consistent accuracy.
- **Context window chunking** splits large specifications and requirement documents into focused domain segments to keep prompts compact and improve LLM reasoning.
- **Hybrid data** combines LLM business scenarios with deterministic boundary and security payloads generated natively with zero token cost.
- **Flexible output** exports generated test plans into executable native SpecTest YAML, human-readable coverage matrices, and standard API collections.

### Test Execution & Failure Diagnosis

- **Automated environment lifecycle** manages target services from startup and health checks to teardown across flexible modes, including live endpoints, Docker Compose, and custom start commands.
- **Multi-tier verification** validates responses with status codes, latency, schema structure, security, business logic, and database state consistency.
- **Failure diagnosis** inspects failed test cases to analyse root causes and suggest fixes.

### Spec-Drift Detection

- **Deterministic spec diffing** compares API versions natively to detect breaking changes, modified fields, and new endpoints with no cost.
- **Targeted test patching** isolates only the scenarios and steps affected by API changes, calling the LLM to update them while keeping existing test cases intact.
- **Append-only revision history** tracks test updates as incremental blocks inside the same YAML file, with an option to flatten changes into the base suite.

## Supported Platforms

| Platform | Integration | Description |
| --- | --- | --- |
| Local Development | CLI, Web UI | Native binary for Linux, macOS, and Windows with local built-in Web UI. |
| Cloud CI/CD | GitHub Actions | Automated quality gates on push and pull request events with reports. |
| On-Premise Server | Webhook Daemon, Docker | Self-hosted server for private networks triggered by internal Git webhooks. |
| AI Coding Agents | MCP Server, Agent Skills | Native Model Context Protocol integration for AI Agents. |
| LLM Providers | Cloud, Local Models | Pluggable support for cloud APIs and self-hosted local models. |

## Documentation

- **[Quick Start Guide](docs/quickstart.md)** covers installation, LLM provider setup, and running your first test suite.
- **[System Architecture](docs/architecture.md)** explains the Clean Architecture layers, data models, and core execution flows.
- **[Contributing Guide](CONTRIBUTING.md)** outlines development setup, coding standards, and pull request workflows.
- **[Security Policy](SECURITY.md)** details credential handling and vulnerability reporting procedures.

## Quickstart

### Prerequisites

- **Go 1.25+** (or download prebuilt binaries from [Releases](https://github.com/nguyentin05/spectest/releases))
- An OpenAPI 3.x document (`openapi.yaml` or `openapi.json`)
- *(Optional)* Product Requirement Document (PRD) or business requirements in Markdown (`requirements.md`)
- *(Optional)* An API key for an LLM provider (Google Gemini, OpenAI) or a running local instance of [Ollama](https://ollama.ai)

---

### 1. Local Development (CLI & Web UI)

#### Installation & Setup
```bash
# Install via Go
go install github.com/nguyentin05/spectest/cmd/spectest@latest

# Configure LLM Provider (Google Gemini, OpenAI, or local Ollama)
spectest config init --provider gemini --model gemini-2.5-flash
spectest config set api-key
```

#### Generate a Test Suite
```bash
# Initialize a new suite from OpenAPI and optional PRD requirements
spectest generate --mode init -a shop=openapi.yaml -r requirements.md -o ./tests
```

#### Run Tests
```bash
# Single target execution
spectest run -f ./tests/spectest.yaml -t http://localhost:8080

# Multi-target microservices
spectest run -f ./tests/spectest.yaml -t shop=http://localhost:8001 -t billing=http://localhost:8002

# Run with automated Docker Compose lifecycle and AI defect triage
spectest run -f ./tests/spectest.yaml --compose docker-compose.test.yml -t http://localhost:8080 --triage

# Run only scenarios impacted by the latest update
spectest run -f ./tests/spectest.yaml -t http://localhost:8080 --only-changed
```

#### Launch Web UI Dashboard
```bash
spectest ui
```
Opens the interactive React dashboard at `http://127.0.0.1:8088`.

---

### 2. Cloud CI/CD (GitHub Actions)

Add automated API contract and regression quality gates to your repository using the official composite action:

```yaml
# .github/workflows/api-tests.yml
name: API Regression Quality Gate

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  spectest:
    name: Run SpecTest API Suite
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

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
```

---

### 3. On-Premise Server (Air-Gapped & Webhook Daemon)

- *Coming soon: Self-hosted webhook daemon for private enterprise networks and Git webhooks.*

---

### 4. AI Coding Agents (MCP Server)

- *Coming soon: Native Model Context Protocol (MCP) server adapter for AI coding agents (Claude Code, Cursor, Cline).*

## Team & Contributors

SpecTest is developed and maintained by the core engineering team:

We welcome community contributions of all kinds from bug reports and documentation improvements to new LLM adapters and platform integrations. Please read our [Contributing
Guide](CONTRIBUTING.md) and [Code of Conduct](CODE_OF_CONDUCT.md) before submitting a Pull Request.

## License

Distributed under the **[Apache License, Version 2.0](LICENSE)**. See LICENSE and NOTICE for full license text and third-party attributions.
