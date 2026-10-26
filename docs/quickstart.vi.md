# Hướng dẫn Khởi động Nhanh

Bắt đầu sử dụng **SpecTest** trong chưa đầy 3 phút.

---

## Điều kiện tiên quyết

Trước khi bắt đầu, hãy đảm bảo bạn đã có:

- Môi trường **Go 1.22+** (hoặc tải file nhị phân biên dịch sẵn từ [GitHub Releases](https://github.com/nguyentin05/spectest/releases)).
- Một file đặc tả API theo định dạng OpenAPI 3.x (`openapi.yaml` hoặc `openapi.json`).
- *(Tùy chọn)* API key của nhà cung cấp LLM (Google Gemini, OpenAI) hoặc một instance [Ollama](https://ollama.ai) chạy offline trên máy để sinh test và phân tích lỗi bằng AI.

---

## Bước 1: Cài đặt SpecTest

=== "Go Toolchain"
    ```bash
    go install github.com/nguyentin05/spectest/cmd/spectest@latest
    spectest --version
    ```

=== "Biên dịch từ mã nguồn"
    ```bash
    git clone https://github.com/nguyentin05/spectest.git
    cd spectest
    make build
    ./bin/spectest --help
    ```

=== "Docker"
    ```bash
    docker run --rm -v $(pwd):/workspace ghcr.io/nguyentin05/spectest:latest --help
    ```

---

## Bước 2: Cấu hình LLM Provider

SpecTest quản lý thông tin chứng thực an toàn tại `~/.spectest/credentials.yaml` với quyền POSIX nghiêm ngặt (`0600`).

=== "Google Gemini (Khuyến nghị)"
    ```bash
    spectest config init --provider gemini --model gemini-2.5-flash
    spectest config set api-key
    # Nhập API Key tương tác (ẩn hiển thị trên terminal)
    ```

=== "OpenAI"
    ```bash
    spectest config init --provider openai --endpoint https://api.openai.com/v1 --model gpt-4o-mini
    spectest config set api-key
    ```

=== "Ollama (Chạy Offline / Cục bộ)"
    ```bash
    # Kéo mô hình và khởi chạy
    ollama pull qwen2.5-coder:7b
    
    # Cấu hình SpecTest trỏ về endpoint cục bộ
    spectest config init --provider ollama --endpoint http://localhost:11434/v1 --model qwen2.5-coder:7b
    ```

Kiểm tra cấu hình đang hoạt động:
```bash
spectest config list
```

---

## Bước 3: Sinh Bộ Kiểm thử (`spectest generate`)

Phân tích hợp đồng OpenAPI và tài liệu yêu cầu nghiệp vụ để sinh bộ kịch bản kiểm thử:

```bash
spectest generate -a ./examples/openapi.yaml -r ./examples/requirements.md -o ./tests
```

Lệnh này sẽ tạo ra:
- `tests/spectest.yaml`: Bộ kịch bản kiểm thử đầy đủ schema v1 (`servers`, `apis`, `analysis`, `seed`, `setup`, `scenarios`).
- `tests/spectest.log.yaml`: Snapshot baseline ban đầu kèm mã băm SHA-256 chuẩn tắc.
- `tests/test_matrix.md`: Báo cáo ma trận bao phủ liên kết từng endpoint với các scenario kiểm thử.

---

## Bước 4: Chạy Bộ Kiểm thử (`spectest run`)

### 1. Chạy với một Target duy nhất
Thực thi bộ kịch bản kiểm thử với API server đang chạy:

```bash
spectest run -f ./tests/spectest.yaml -t http://localhost:8080
```

### 2. Chạy với nhiều Microservices (Multi-Target)
Nếu bộ kiểm thử gọi đến nhiều alias server (ví dụ `{shop}` và `{billing}`):

```bash
spectest run -f ./tests/spectest.yaml \
  -t shop=http://localhost:8001 \
  -t billing=http://localhost:8002
```

### 3. Tự động hóa Docker Compose & AI Triage
Tự động khởi động database/dịch vụ nền qua Docker, chạy kiểm thử và kích hoạt AI phân tích nếu có lỗi:

```bash
spectest run -f ./tests/spectest.yaml \
  --compose docker-compose.test.yml \
  -t http://localhost:8080 \
  --triage
```

### 4. Chỉ chạy các kịch bản bị ảnh hưởng bởi cập nhật
Sau khi cập nhật API spec, chỉ chạy các scenario bị ảnh hưởng theo bản ghi cập nhật mới nhất trong `spectest.log.yaml`:

```bash
spectest run -f ./tests/spectest.yaml -t http://localhost:8080 --only-changed
```

---

## Bước 5: Khởi chạy Giao diện Web UI (`spectest ui`)

Nếu bạn thích thao tác trực quan trên trình duyệt:

```bash
spectest ui
```

Truy cập `http://127.0.0.1:8088` để:
- **Workspace**: Tải lên và quản lý các file OpenAPI, Markdown.
- **Suite Editor**: Xem và chỉnh sửa trực quan scenarios, setup blocks và các bước HTTP.
- **Real-Time Runner**: Bấm chạy test với luồng log thời gian thực qua Server-Sent Events (SSE), biểu đồ latency và báo cáo AI triage.
- **History**: Xem lịch sử các lần update và so sánh trực quan diff giữa các phiên bản.
