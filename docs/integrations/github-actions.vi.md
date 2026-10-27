# Tích hợp GitHub Actions

SpecTest cung cấp **Setup Composite Action** (`action.yml`) chính thức giúp tự động tải, kiểm tra mã băm SHA-256, lưu bộ nhớ đệm (cache), cài đặt binary `spectest` CLI vào `$PATH`, đồng thời tự động cấu hình chứng chỉ LLM trên mọi runner Linux, macOS, và Windows (`amd64` và `arm64`).

---

## Ví dụ 1: Chạy kiểm thử & Đẩy báo cáo SARIF lên GitHub Code Scanning

Tạo file `.github/workflows/api-tests.yml`:

```yaml
name: API Regression & Quality Gate

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

permissions:
  contents: read
  security-events: write # Bắt buộc để tải SARIF lên tab Code Scanning

jobs:
  spectest:
    name: Run SpecTest E2E Suite
    runs-on: ubuntu-latest
    steps:
      - name: Check out code
        uses: actions/checkout@v4

      - name: Setup SpecTest-AI & Configure LLM
        uses: nguyentin05/spectest@v1
        with:
          version: "latest"
          provider: "gemini"
          api_key: ${{ secrets.GEMINI_API_KEY }}
          model: "gemini-2.5-pro"
          endpoint: "https://generativelanguage.googleapis.com/v1beta"

      - name: Execute SpecTest Suite
        run: |
          mkdir -p reports
          spectest run \
            -f tests/spectest.yaml \
            -t http://localhost:8080 \
            --report-dir reports \
            --sarif reports/spectest.sarif

      - name: Upload Test Reports
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: spectest-reports
          path: reports/

      - name: Upload SARIF to GitHub Code Scanning
        if: always()
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: reports/spectest.sarif
```

---

## Ví dụ 2: AI Tự động cập nhật test case khi có PR thay đổi OpenAPI

SpecTest có thể tự động đồng bộ hóa các kịch bản kiểm thử khi tài liệu OpenAPI thay đổi:

```yaml
name: Auto-Sync Test Suite

on:
  pull_request:
    paths:
      - 'openapi/**'

jobs:
  update-suite:
    name: Update Tests from OpenAPI
    runs-on: ubuntu-latest
    steps:
      - name: Check out code
        uses: actions/checkout@v4

      - name: Setup SpecTest-AI & Configure LLM
        uses: nguyentin05/spectest@v1
        with:
          provider: "openai"
          api_key: ${{ secrets.OPENAI_API_KEY }}
          model: "gpt-4o-mini"
          endpoint: "https://api.openai.com/v1"

      - name: Generate updated scenarios
        run: |
          spectest generate -f tests/spectest.yaml -a openapi/api.yaml

      - name: Commit updated tests back to PR
        uses: stefanzweifel/git-auto-commit-action@v5
        with:
          commit_message: "chore(test): auto-update spectest.yaml from OpenAPI spec changes"
          file_pattern: "tests/spectest.yaml tests/spectest.log.yaml"
```

---

## Bảng Tham chiếu Action

### Inputs

| Input | Bắt buộc | Mặc định | Mô tả |
| :--- | :--- | :--- | :--- |
| `version` | Không | `"latest"` | Phiên bản binary SpecTest cần cài đặt (ví dụ `'v0.2.0'` hoặc `'latest'`). |
| `provider` | Không | `""` | Nhà cung cấp LLM để generate và triage lỗi (`gemini`, `openai`, `anthropic`, `ollama`). |
| `api_key` | Không | `""` | API key của LLM. Truyền qua `secrets.*` (tự động mask trên log CI). |
| `model` | Không | `""` | Tên model LLM mặc định (ví dụ `'gpt-4o-mini'`, `'gemini-2.5-pro'`, `'qwen2.5-coder:7b'`). |
| `endpoint` | Không | `""` | URL endpoint tùy chỉnh (ví dụ `'http://localhost:11434'` khi dùng Ollama tự host). |

### Outputs

| Output | Mô tả |
| :--- | :--- |
| `version` | Phiên bản semver chính thức của SpecTest đã được cài đặt (ví dụ `'v0.2.0'`). |
| `spectest_path` | Đường dẫn tuyệt đối tới thư mục chứa file thực thi đã được đưa vào PATH. |
