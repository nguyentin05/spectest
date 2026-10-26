# Zero-Token First Philosophy

A central design pillar of **SpecTest** is the **Zero-Token First** rule: **Never spend costly LLM tokens on tasks that deterministic Go algorithms can solve faster, cheaper, and with 100% mathematical certainty.**

---

## The LLM Cost & Flakiness Problem

Traditional "AI test generators" naively pass entire monolithic OpenAPI specifications (often 10,000+ lines of JSON or YAML) directly into LLM prompts. This approach introduces severe flaws:

1. **Massive Token Waste**: Redundant descriptions, repetitive models, and boilerplate headers consume 80%+ of the prompt context window.
2. **Context Dilution**: Large context windows degrade LLM reasoning precision, causing hallucinated endpoints, broken parameters, and invalid authentication flows.
3. **High Operational Costs**: Running full test generations or updates across multiple microservices quickly becomes cost-prohibitive in automated CI/CD pipelines.
4. **Flakiness & Non-Determinism**: Relying on LLMs for spec diffing, status evaluation, or dependency sorting yields unpredictable test outcomes between runs.

---

## Architectural Separation of Responsibilities

SpecTest strictly divides responsibilities between high-performance native Go routines and semantic LLM reasoning:

| Task Category | Component | Token Cost | Implementation Rationale |
| :--- | :--- | :--- | :--- |
| **OpenAPI Normalization & Pruning** | `infrastructure/openapi/` | **0 tokens** | Strips redundant documentation, unused models, and formatting bloat in Go. |
| **Context Chunking** | `infrastructure/openapi/chunker.go` | **0 tokens** | Partitions specs into logical sub-domains based on OpenAPI tags and operation paths. |
| **Auth-Token Wiring** | `infrastructure/openapi/auth_detector.go` | **0 tokens** | Deterministically matches login/token endpoints and configures Bearer token extraction. |
| **API Catalog Reconstruction** | `usecase/generate/update_catalog.go` | **0 tokens** | Reconstructs API catalog state in-memory from `spectest.log.yaml` base and patches. |
| **Deterministic API Diffing** | `usecase/generate/update_diff.go` | **0 tokens** | Compares OpenAPI ASTs to detect added, changed, removed, or restored operations. |
| **Impact Graph Resolution** | `usecase/generate/update_impact.go` | **0 tokens** | Computes directly affected scenarios and transitive setup DAG dependees. |
| **Setup DAG Resolution** | `usecase/run/dag.go` | **0 tokens** | Calculates dependency closures, validates nodes, and detects cycles via DFS topological sort. |
| **Scope Interpolation & Type Preservation** | `usecase/run/scope.go` | **0 tokens** | Substitutes variables (`{{var}}`, `{{random}}`) and preserves underlying Go runtime types. |
| **Response Extraction & Status Checks** | `usecase/run/step.go` | **0 tokens** | Evaluates status codes, extracts values (`body.<path>`, `header.<name>`), and bounds responses to 1MB. |
| **Redaction & Sanitization** | `usecase/run/redact.go` | **0 tokens** | Strips passwords, authorization tokens, cookies, and sensitive keys from logs and reports. |
| **Target Lifecycle & Probing** | `infrastructure/server/` | **0 tokens** | Orchestrates Docker Compose boot/teardown and executes pre-flight healthcheck probes. |
| **Business Flow Synthesis** | `usecase/generate/` | **~8%–15% tokens** | High-level semantic reasoning for end-to-end user journeys. |
| **AI Defect Triage** | `usecase/run/triage.go` | **On Failure Only** | Analyzes error logs, diffs, and responses to diagnose root causes on demand. |

---

## Real-World Impact

By offloading all deterministic tasks to Go:
- **Cost Reduction**: Token usage is reduced by **85% to 92%** compared to full-spec prompt engines.
- **Speed**: Spec pruning, DAG sorting, catalog diffing, and variable interpolation complete in sub-millisecond native Go runtime.
- **Repeatability**: Test executions, setup resolution, and assertions run 100% deterministically without LLM temperature drift.
