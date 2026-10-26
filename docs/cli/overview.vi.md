# Tra cứu Lệnh CLI: Tổng quan

**SpecTest CLI** (`spectest`) là công cụ dòng lệnh duy nhất để sinh test suite, thực thi kịch bản, quản lý thông tin chứng thực và khởi chạy giao diện Web UI dashboard.

---

## Cây Cấu trúc Lệnh

```text
spectest [flags] [command]

Lệnh Kỹ nghệ Kiểm thử Cốt lõi:
  generate    Phân tích OpenAPI & PRD để sinh kịch bản kiểm thử thực thi được
  run         Thực thi bộ test API với DAG setup và nối chuỗi trạng thái

Cấu hình & Workspace:
  config      Quản lý thông tin chứng thực provider, endpoint và model LLM
    ├── init  Khởi tạo provider, endpoint và model
    ├── list  Xem cấu hình hiện tại (khóa bí mật được ẩn)
    └── set   Cập nhật một khóa cấu hình riêng lẻ

Giao diện Trực quan & Tiện ích:
  ui          Khởi chạy Web UI dashboard tương tác (alias: dashboard, web)
  completion  Tạo script tự động gợi ý lệnh (bash, zsh, fish, powershell)
  help        Xem trợ giúp cho từng lệnh
```

---

## Cờ Toàn cục (Global Flags)

Áp dụng cho lệnh root và mọi lệnh con:

| Cờ | Rút gọn | Kiểu | Mặc định | Mô tả |
| :--- | :--- | :--- | :--- | :--- |
| `--verbose` | | boolean | `false` | Bật chế độ ghi log debug chi tiết (`slog`) kèm timestamp và vị trí file code. |
| `--version` | `-v` | boolean | `false` | Hiển thị phiên bản hiện tại, mã Git commit SHA và thời gian build. |
| `--help` | `-h` | boolean | `false` | Hiển thị hướng dẫn cú pháp và cách dùng lệnh. |

---

## Cú pháp Ánh xạ Nhiều Target (Multi-Target)

Khi chạy kiểm thử với `spectest run`, bạn có thể chỉ định target theo 2 cách:

1. **Một URL Target Duy nhất**:
   Áp dụng làm máy chủ mặc định cho mọi bước dùng đường dẫn tương đối:
   ```bash
   spectest run -f ./tests/spectest.yaml -t http://localhost:8080
   ```

2. **Ánh xạ Alias Máy chủ (`-t alias=url`)**:
   Ánh xạ các alias máy chủ khai báo trong `spectest.yaml` (ví dụ `{shop}`, `{billing}`) sang các endpoint cụ thể:
   ```bash
   spectest run -f ./tests/spectest.yaml \
     -t shop=http://localhost:8001 \
     -t billing=http://localhost:8002
   ```

> [!NOTE]
> Cờ `-t / --target` có thể được truyền nhiều lần để ánh xạ nhiều microservice khác nhau trong cùng một lượt chạy.

---

## Biến Môi trường

SpecTest nhận các biến môi trường sau:

| Biến | Mô tả |
| :--- | :--- |
| `SPECTEST_API_KEY` | Ghi đè API key lưu trong `~/.spectest/credentials.yaml`. Rất thích hợp cho môi trường CI/CD. |
| `GEMINI_API_KEY` | Key dự phòng cho Google Gemini provider. |
| `OPENAI_API_KEY` | Key dự phòng cho OpenAI hoặc các gateway tương thích. |
| `SPECTEST_CONFIG_DIR` | Đường dẫn thư mục chứa cấu hình tùy chỉnh (mặc định là `$HOME/.spectest`). |

---

## Mã Thoát (Exit Codes)

| Mã thoát | Trạng thái | Ý nghĩa |
| :--- | :--- | :--- |
| `0` | **Thành công** | Tất cả các kịch bản và bước kiểm thử đều vượt qua. |
| `1` | **Thất bại** | Có ít nhất một bước kiểm thử bị fail, lỗi thăm dò target, hoặc đầu vào không hợp lệ. |
| `130` | **Đã hủy** | Tiến trình bị ngắt bởi tín hiệu `SIGINT` (Ctrl-C) hoặc `SIGTERM`. Docker containers được dọn dẹp an toàn trước khi thoát. |
