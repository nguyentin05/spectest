# Tích hợp GitHub Actions

SpecTest cung cấp sẵn một **Composite GitHub Action** chuẩn mực ([action.yml](file:///home/tin/Project/current/spectest/action.yml)), chạy trực tiếp trong GitHub Actions workflows trên các runner Linux và macOS (`amd64` và `arm64`).

---

## Mẫu Workflow Hoàn chỉnh

Tạo file `.github/workflows/api-tests.yml`:

```yaml
name: API Regression & Contract Quality Gate

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  spectest:
    name: Run SpecTest E2E Suite
    runs-on: ubuntu-latest
    steps:
      - name: Check out code
        uses: actions/checkout@v4

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

      - name: Upload HTML Test Report
        uses: actions/upload-artifact@v4
        if: always()
        with:
          name: spectest-report
          path: reports/
```

---

## Bảng Tham chiếu Inputs

| Input | Bắt buộc | Mặc định | Mô tả |
| :--- | :--- | :--- | :--- |
| `version` | Không | `"latest"` | Phiên bản binary SpecTest cần cài đặt (ví dụ `'v1.0.0'` hoặc `'latest'`). |
| `test_file` | Không | `"tests/spectest.yaml"` | Đường dẫn tới file suite YAML trong repo của bạn. |
| `target_url` | Không | `""` | URL API mục tiêu. Ghi đè target mặc định. |
| `compose_file` | Không | `""` | Đường dẫn tới `docker-compose.yml` để tự động bật/tắt dịch vụ backend. |
| `gemini_api_key`| Không | `""` | Google Gemini API key để tự động phân tích nguyên nhân lỗi (AI Defect Triage). |
| `openai_api_key`| Không | `""` | API key thay thế cho OpenAI. |
| `report_dir` | Không | `"reports"` | Thư mục lưu các báo cáo kiểm thử (`junit.xml` và `report.html`). |

---

## Bảng Tham chiếu Outputs

| Output | Mô tả |
| :--- | :--- |
| `junit_report` | Đường dẫn tới file JUnit XML được sinh ra (`reports/junit.xml`). |
| `html_report` | Đường dẫn tới file báo cáo HTML dashboard (`reports/report.html`). |
| `test_status` | Trạng thái thực thi cuối cùng: `'success'` hoặc `'failure'`. |

---

## Kiểm thử Đa Container với Docker Compose trong CI

Nếu backend của bạn cần cơ sở dữ liệu (PostgreSQL, MongoDB, Redis) và các container phụ trợ:

```yaml
      - name: Run SpecTest with Auto Docker Lifecycle
        uses: nguyentin05/spectest@v1
        with:
          test_file: "tests/spectest.yaml"
          compose_file: "docker-compose.test.yml"
          target_url: "http://localhost:8080"
          gemini_api_key: ${{ secrets.GEMINI_API_KEY }}
```

SpecTest sẽ tự động:
1. Khởi động tất cả container qua `docker compose up -d`.
2. Thăm dò cho tới khi endpoint API phản hồi khỏe mạnh.
3. Chạy toàn bộ kịch bản kiểm thử và phân tích lỗi bằng AI nếu có.
4. Đảm bảo dọn dẹp sạch sẽ (`docker compose down -v`) khi workflow kết thúc hoặc bị hủy.
