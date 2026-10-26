# Contributing Guide

Thank you for contributing to **SpecTest**! We welcome contributions from developers, QA engineers, security researchers, and technical writers.

---

## Development Environment Setup

### Prerequisites
- **Go**: Version `1.22+` (1.25 recommended).
- **Git**: Version `2.30+`.
- **Docker & Docker Compose**: For integration tests and container lifecycle testing.
- **Node.js & npm** (optional): For building the Web UI frontend (`cd web && npm install`).
- **Python 3.10+ & MkDocs** (optional): For previewing documentation (`pip install -r requirements-docs.txt`).

### Fork and Clone
```bash
git clone https://github.com/<your-username>/spectest.git
cd spectest
git remote add upstream https://github.com/nguyentin05/spectest.git
```

---

## Common Development Tasks

### 1. Build Binary
Compile the `spectest` CLI binary into `./bin/`:
```bash
make build
./bin/spectest --help
```

### 2. Run Tests
Execute unit and use case tests:
```bash
make test
# or run with race detector:
go test -race ./...
```

### 3. Run Static Analysis (Linter)
```bash
make lint
```

### 4. Build Web UI Frontend
```bash
cd web
npm install
npm run build
cd ..
```

### 5. Preview Documentation Locally
```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements-docs.txt
mkdocs serve
```
Open `http://127.0.0.1:8000` in your web browser.

---

## Architecture Rules & Guidelines

- **Clean Architecture Boundaries**: Code inside `internal/domain` must have **zero external dependencies** and no imports of infrastructure, YAML libraries, or HTTP clients.
- **Single Source of Truth**: The Web UI and CLI must invoke the same canonical use case engines (`internal/usecase/run`, `internal/usecase/generate`). Never duplicate business logic inside transport adapters.
- **Two-File Model**: SpecTest operations revolve around `spectest.yaml` and `spectest.log.yaml`. Do not add new use of legacy tracking files.
- **Secret Redaction**: Never log, serialize, or transmit unredacted API keys, passwords, or bearer tokens to reports or LLM triage prompts.
