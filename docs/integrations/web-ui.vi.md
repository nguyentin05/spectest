# Giao diện Web UI Dashboard

SpecTest tích hợp sẵn giao diện Web UI dashboard hiệu năng cao, xây dựng bằng React, TypeScript và Vite. Dashboard mang lại khả năng quản lý trực quan toàn bộ workspace, sinh test, theo dõi thực thi thời gian thực và chẩn đoán lỗi.

---

## Khởi chạy Dashboard

Chạy lệnh UI trên máy cục bộ:

```bash
spectest ui
```

Mặc định, server sẽ lắng nghe tại `http://127.0.0.1:8088` và tự động mở trình duyệt web của bạn.

---

## Các Module Chính trên Dashboard

### 1. Quản lý Workspace (`/workspace`)
- Duyệt các file trong workspace của dự án.
- Tải lên các file đặc tả OpenAPI (`.yaml`/`.json`) và tài liệu PRD Markdown (`.md`).
- Xem thống kê API catalog (tổng số endpoints, methods, các nhóm tags).

### 2. Generator Studio (`/generate`)
- Cấu hình trực quan các thông số sinh test: chọn file đầu vào, số luồng concurrency và thư mục lưu trữ.
- Xem trước các scenario nghiệp vụ, chuỗi setup và các bước HTTP trước khi ghi vào `spectest.yaml`.

### 3. Trình Soạn thảo Suite (`/suite`)
- Trình xem và chỉnh sửa trực quan file `spectest.yaml`.
- Xem các thẻ scenario, phụ thuộc setup, các bước HTTP, header, body và ánh xạ trích xuất `save`.
- Xem bảng ma trận bao phủ liên kết các operation OpenAPI với scenario kiểm thử.

### 4. Runner Thực thi Thời gian Thực (`/run`)
- Cấu hình URL mục tiêu (URL đơn hoặc ánh xạ `-t alias=url`) và file Docker Compose.
- Khởi động kiểm thử chỉ với một cú click chuột.
- **Truyền phát SSE Trực tiếp**: Đẩy sự kiện telemetry theo thời gian thực tại mỗi bước:
  - Badge trạng thái HTTP (`200 OK`, `400 Bad Request`).
  - Biểu đồ thời gian phản hồi (latency) và thanh tiến trình.
  - Phân định rõ bước setup và bước kiểm thử chính của kịch bản.

### 5. Kiểm tra Thất bại & AI Triage
- Khi một bước kiểm thử bị fail, bấm vào để xem chi tiết assertion bị hỏng.
- Xem snapshot request payload, response body và headers đã được che giấu dữ liệu nhạy cảm.
- Kích hoạt **AI Defect Triage** theo yêu cầu để nhận giải thích nguyên nhân và hướng xử lý trực tiếp trên giao diện.

### 6. Lịch sử & Diffs (`/history`)
- Duyệt các bản ghi lịch sử ghi nối tiếp từ `spectest.log.yaml`.
- So sánh diff YAML trực quan song song giữa các phiên bản.
- Kiểm tra mã băm mật mã SHA-256 của suite.

---

## Chạy Headless & Truy cập Từ xa

Để chạy Web UI trong Docker container hoặc máy chủ phát triển từ xa:

```bash
spectest ui --host 0.0.0.0 --port 8088 --no-open
```
Truy cập qua `http://<dia-chi-ip>:8088` hoặc thiết lập port-forwarding qua SSH.
