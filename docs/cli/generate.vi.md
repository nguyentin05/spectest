# `spectest generate`

Sinh bộ kịch bản kiểm thử SpecTest mới hoặc cập nhật tăng dần bộ kịch bản hiện có từ tài liệu OpenAPI và yêu cầu nghiệp vụ.

---

## Cú pháp

```bash
# Khởi tạo suite mới
spectest generate --mode init -a <server>=<duong-dan-openapi> [flags]

# Cập nhật suite hiện có khi hợp đồng API thay đổi
spectest generate --mode update -a <server>=<duong-dan-openapi> [flags]
```

---

## Các Chế độ Hoạt động

`spectest generate` hỗ trợ hai chế độ vận hành chính thông qua cờ `--mode`:

### 1. Chế độ Khởi tạo (`--mode init`, Mặc định)
Sinh toàn bộ test suite mới từ đầu:
- Kiểm tra tính hợp lệ của tài liệu OpenAPI và tài liệu Markdown yêu cầu nghiệp vụ.
- Thiết kế các kịch bản kiểm thử luồng nghiệp vụ nhiều bước chân thực.
- Đóng gói dữ liệu ra file `spectest.yaml` và khởi tạo `spectest.log.yaml` với snapshot baseline ban đầu kèm mã băm SHA-256 12 ký tự.
- **Bảo vệ Tránh Ghi đè**: Nếu phát hiện file suite đã tồn tại trong thư mục `--output`, CLI sẽ yêu cầu người dùng xác nhận trước khi ghi đè (tự động báo lỗi nếu chạy trong terminal non-interactive).

### 2. Chế độ Cập nhật (`--mode update`)
Đồng bộ hóa tăng dần bộ kịch bản hiện có khi đặc tả API thay đổi:
- **Kiến trúc Đúng 2 File**: Hoạt động trực tiếp trên `spectest.yaml` và `spectest.log.yaml`, tuyệt đối không cần các file tracking trung gian.
- **Kiểm chứng Hash Ngắt Lỗi (Fail-Closed)**: Tính mã băm SHA-256 của `spectest.yaml` hiện tại. Nếu lệch so với `after_hash` của bản ghi mới nhất trong `spectest.log.yaml`, tiến trình dừng ngay để tránh hỏng dữ liệu.
- **Tái tạo API Catalog**: Tái dựng toàn bộ catalog API bằng cách áp dụng tất cả các bản vá `catalog_patch` lên `base.api_catalog` của file log.
- **So lệch Đặc tả Tất định**: So khớp OpenAPI mới với catalog đã tái tạo để tìm các endpoint được thêm mới, thay đổi, bị xóa hoặc được khôi phục.
- **Giải quyết Đồ thị Tác động (Impact Graph)**: Xác định các scenario bị ảnh hưởng trực tiếp, cũng như các scenario bị ảnh hưởng gián tiếp do phụ thuộc vào các khối `setup` bị thay đổi (thông qua DAG).
- **Tự động Gắn Tool Skip khi API bị Xóa**: Nếu một API bị xóa khỏi đặc tả, tự động đánh dấu các scenario liên quan bằng `skip: "spectest: API <key> removed"` mà không xóa kịch bản của người dùng. Báo lỗi dừng lại nếu kịch bản đó đang bị khóa (`locked: true`).
- **Khôi phục API (Restored APIs)**: Tự động gỡ bỏ các skip do công cụ sinh ra (`spectest: API`) khi endpoint quay trở lại, đồng thời bảo toàn nguyên vẹn các skip do người dùng tự viết.
- **Ghi Lịch sử Nối tiếp**: Nối thêm một bản ghi `UpdateEntry` mới (tăng số thứ tự `n` liên tục) kèm trigger, catalog patch, danh sách impact và mã băm suite mới.

---

## Bảng Cờ Lệnh & Tùy chọn

| Cờ | Rút gọn | Kiểu | Mặc định | Bắt buộc | Mô tả |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `--mode` | | string | `"init"` | Không | Chế độ sinh: `init` (tạo suite mới) hoặc `update` (cập nhật suite hiện có). |
| `--api` | `-a` | stringArray | `[]` | **Có** | Tài liệu OpenAPI theo định dạng `<server>=<path>` (có thể lặp lại; thứ tự xác định alias máy chủ). |
| `--req` | `-r` | string | `""` | Không | Đường dẫn tới tài liệu yêu cầu nghiệp vụ gốc (markdown). |
| `--output` | `-o` | string | `"./tests"` | Không | Thư mục chứa `spectest.yaml` và `spectest.log.yaml`. |
| `--dry-run` | | boolean | `false` | Không | Chạy toàn bộ pipeline kiểm tra và tính toán nhưng không ghi file ra đĩa. |
| `--verbose` | | boolean | `false` | Không | Bật ghi log debug chi tiết. |

---

## Ví dụ Thực tế

### 1. Khởi tạo Suite Đơn Server
```bash
spectest generate --mode init -a shop=openapi.yaml -o ./tests
```

### 2. Khởi tạo Suite Đa Server Kèm Yêu cầu Nghiệp vụ
Chỉ định nhiều alias server theo thứ tự mong muốn:
```bash
spectest generate --mode init \
  -a auth=openapi-auth.yaml \
  -a order=openapi-order.yaml \
  -r requirements.md \
  -o ./tests
```

### 3. Chạy Thử Không Ghi Đĩa (Dry-Run)
Xem trước kết quả xử lý mà không làm thay đổi file trên máy:
```bash
spectest generate --mode init -a shop=openapi.yaml --dry-run
```

### 4. Cập nhật Tăng dần khi API Thay đổi
Cập nhật suite hiện tại sau khi sửa file `openapi.yaml`:
```bash
spectest generate --mode update -a shop=openapi.yaml -o ./tests
```

---

## Các File Artifact Đầu ra

- **`spectest.yaml`**: Suite kiểm thử hoạt động (schema v1) gồm `servers`, `apis`, `analysis`, `seed`, `setup`, và `scenarios`.
- **`spectest.log.yaml`**: Lịch sử ghi nối tiếp ghi nhận snapshot baseline hoặc bản ghi `UpdateEntry` mới kèm mã băm SHA-256 mới.
