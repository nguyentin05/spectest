# `spectest ui`

Khởi chạy giao diện Web UI dashboard tương tác trực quan cho SpecTest.

---

## Cú pháp

```bash
spectest ui [flags]
```

**Các tên gọi khác (Aliases):** `dashboard`, `web`

---

## Mô tả Hoạt động

Lệnh `ui` khởi chạy một máy chủ HTTP tích hợp, cung cấp giao diện web hiện đại xây dựng trên React, TypeScript và Vite.

### Các Tính năng Chính:
- **Quản lý File Workspace**: Tải lên và duyệt nhanh các tài liệu đặc tả OpenAPI và file yêu cầu PRD Markdown.
- **Trình Soạn thảo Suite Trực quan**: Xem và chỉnh sửa các kịch bản, các khối setup và các bước HTTP có highlight cú pháp.
- **Truyền Phát Thực thi Thời gian Thực**: Bấm chạy test với luồng telemetry đẩy trực tiếp qua Server-Sent Events (SSE), biểu đồ latency từng bước và badge trạng thái pass/fail tức thì.
- **Chẩn đoán Lỗi Tương tác**: Xem chi tiết diff giữa request và response thực tế, kiểm tra assertion bị hỏng và kích hoạt AI Defect Triage ngay trên trình duyệt.
- **Xem Lịch sử & Diffs**: Theo dõi các bản ghi lịch sử ghi nối tiếp từ `spectest.log.yaml` và kiểm tra mã băm SHA-256 của suite.

---

## Bảng Cờ Lệnh & Tùy chọn

| Cờ | Rút gọn | Kiểu | Mặc định | Bắt buộc | Mô tả |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `--host` | | string | `"127.0.0.1"` | Không | Địa chỉ host gắn kết cho máy chủ web. |
| `--port` | `-p` | int | `8088` | Không | Cổng mạng (port) lắng nghe. |
| `--no-open` | | boolean | `false` | Không | Không tự động mở trình duyệt web khi khởi động xong. |
| `--verbose` | | boolean | `false` | Không | Bật log HTTP chi tiết cho server dashboard. |

---

## Ví dụ Thực tế

### 1. Khởi động Mặc định
Khởi chạy UI và tự động mở trình duyệt tại `http://127.0.0.1:8088`:
```bash
spectest ui
```

### 2. Tùy chỉnh Port & Chế độ Headless
Chạy trên port 9090 mà không tự mở trình duyệt (tiện lợi khi làm việc qua SSH port-forwarding hoặc remote dev):
```bash
spectest ui --port 9090 --no-open
```

### 3. Chia sẻ cho Mạng Nội bộ
Lắng nghe trên `0.0.0.0` để các máy khác trong mạng LAN hoặc Docker bridge có thể truy cập dashboard:
```bash
spectest ui --host 0.0.0.0 --port 8088 --no-open
```
