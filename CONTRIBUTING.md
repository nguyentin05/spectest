# Contributing to SpecTest

Thank you for your interest in contributing to **SpecTest**! We welcome contributions from developers, QA engineers, security researchers, and documentation writers of all skill levels.

Please read this document carefully to ensure a smooth and productive collaboration.

---

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md). Please report any violations to [tin.nguyen.cs05@gmail.com](mailto:tin.nguyen.cs05@gmail.com).

---

## Getting Started

### Prerequisites

* **Go**: `1.25+` (Go 1.25 recommended)
* **Git**: `2.30+`
* **Docker & Docker Compose**: For integration tests and demo environments
* **golangci-lint**: For local static analysis (`make lint`)
* **Python & MkDocs** (optional): For building documentation locally (`pip install -r docs/requirements.txt`)

### Fork and Clone

1. Fork the repository on GitHub: `https://github.com/nguyentin05/spectest`
2. Clone your fork locally:
   ```bash
   git clone https://github.com/<your-username>/spectest.git
   cd spectest
   ```
3. Add the upstream remote:
   ```bash
   git remote add upstream https://github.com/nguyentin05/spectest.git
   ```

---

## Development Workflow

### 1. Branch Naming Convention

Create a new branch from `main` using descriptive prefixes:
* `feat/<feature-name>`: A new feature or capability (e.g., `feat/tui-live-chart`)
* `fix/<bug-description>`: A bug fix (e.g., `fix/token-refresh-race`)
* `perf/<area>`: Performance improvements (e.g., `perf/json-memory-pool`)
* `docs/<topic>`: Documentation updates (e.g., `docs/quickstart-guide`)
* `refactor/<area>`: Code refactoring without behavior change (e.g., `refactor/httpclient-retry`)
* `ci/<pipeline>`: CI/CD workflow updates (e.g., `ci/goreleaser-matrix`)

### 2. Clean Architecture Guidelines

SpecTest adheres strictly to **Clean Architecture / Hexagonal Ports & Adapters**:

* **Domain (`internal/domain/`)**: Pure Go definitions of structs and interfaces. **Zero external imports!**
* **UseCase (`internal/usecase/`)**: Orchestrates business rules (`generate`, `run`, `update`). Only depends on `internal/domain`.
* **Adapter (`internal/adapter/`)**: Interacts with the user (`cli/`, `httpui/`) and formats output (`presenter/`: JUnit XML, HTML report).
* **Infrastructure (`internal/infrastructure/`)**: Implements Domain interfaces for external systems (LLM providers, HTTP engine, Docker, OpenAPI parser/differ, file storage).

### 3. Commit Message Format

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```text
<type>(<scope>): <short description in present tense>

[optional body explaining WHY and WHAT changed]

[optional footer(s), e.g. Closes #123]
```

**Allowed types:** `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`.

**Examples:**
* `feat(run): add 4-tier oracle state consistency evaluation`
* `fix(openapi): prevent nil pointer on empty security schemes`
* `docs(readme): add architecture mermaid diagram and quickstart`

---

## Testing & Code Quality

Before opening a pull request, verify that all checks pass locally:

```bash
# 1. Run unit tests and race detector
make test

# 2. Check code formatting and run golangci-lint
make lint

# 3. Verify clean build
make build
```

We aim for test coverage > 80% on core business usecases. Mock external services (`LLMService`, `HTTPRunner`) using Go interfaces.

---

## Submitting a Pull Request (PR)

1. Push your branch to your fork:
   ```bash
   git push origin feat/my-new-feature
   ```
2. Open a Pull Request against `main` on the upstream repository.
3. Fill out the [Pull Request Template](.github/PULL_REQUEST_TEMPLATE.md) completely:
   - Provide a clear summary of changes.
   - Link related GitHub issues (e.g., `Fixes #42`).
   - Confirm that unit tests and lint checks pass.
4. Core maintainers will review your PR and provide constructive feedback. Once approved and CI passes, your PR will be squash-merged into `main`.

---

## Community & Questions

Have questions or want to discuss architectural ideas?
* Open an issue in [GitHub Issues](https://github.com/nguyentin05/spectest/issues).
* Start a topic in [GitHub Discussions](https://github.com/nguyentin05/spectest/discussions).
