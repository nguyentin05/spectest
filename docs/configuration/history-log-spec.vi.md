# Đặc tả Lịch sử `spectest.log.yaml`

`spectest.log.yaml` là file nhật ký kiểm toán ghi nối tiếp (append-only) của SpecTest. File ghi lại snapshot baseline ban đầu, lưu vết từng bản cập nhật tăng dần và duy trì các mã băm mật mã để đảm bảo tính toàn vẹn của bộ kiểm thử.

---

## Ví dụ Chú giải Chi tiết

```yaml
base:
  at: "2026-10-26T00:00:00+07:00"
  by: ai
  spectest:
    version: 1
    input: openapi
    servers: [shop]
    apis: ["POST {shop}/api/login", "GET {shop}/api/products", "POST {shop}/api/orders"]
    analysis: |
      Bộ test xác thực phiên trước, sau đó duyệt sản phẩm và tạo đơn hàng.
    seed:
      credentials:
        vars:
          email: demo@example.com
          password: demo-password
    setup:
      authenticated_user:
        steps:
          - call: "POST {shop}/api/login"
            body:
              email: "{{email}}"
              password: "{{password}}"
            expect: 200
            save:
              access_token: body.access_token
    scenarios:
      - id: list-products
        name: Liệt kê sản phẩm với tư cách người dùng đã đăng nhập
        setup: [authenticated_user]
        steps:
          - call: "GET {shop}/api/products"
            headers:
              Authorization: "Bearer {{access_token}}"
            expect: 200
  after_hash: 14514a8a7144

update:
  - n: 1
    at: "2026-10-26T00:05:00+07:00"
    by: ai
    note: "Thêm kịch bản tạo đơn hàng sau khi cập nhật OpenAPI spec"
    trigger:
      openapi:
        added:
          - "POST /api/orders"
    impact:
      scenarios:
        - create-order
    change:
      - target: scenario
        id: create-order
        reason: "Phát hiện endpoint mới POST /api/orders trong spec"
        before: null
        after:
          id: create-order
          name: Tạo đơn hàng
    after_hash: 3b9e4a11f82c
```

---

## Bảng Tham chiếu Schema

### Cấu trúc Root

| Trường | Kiểu | Mô tả |
| :--- | :--- | :--- |
| `base` | BaseEntry | Snapshot baseline khởi đầu của suite khi được sinh lần đầu tiên. |
| `update` | array[UpdateEntry] | Danh sách ghi nối tiếp các bản cập nhật tiếp theo. |

---

### Khối `base`

| Trường | Kiểu | Mô tả |
| :--- | :--- | :--- |
| `at` | string | Dấu thời gian ISO 8601 khi sinh baseline ban đầu. |
| `by` | string | Tác giả sinh baseline: `"ai"` hoặc `"user"`. |
| `spectest` | Suite | Snapshot toàn bộ đối tượng `spectest.yaml` khởi điểm. |
| `after_hash` | string | Mã băm SHA-256 12 ký tự của suite baseline. |
| `api_catalog` | object | Snapshot các route, operationId và method của OpenAPI tại thời điểm tạo. |

---

### Đối tượng `update[]`

Mỗi bản cập nhật sẽ nối tiếp một phần tử mới vào mảng `update`:

| Trường | Kiểu | Mô tả |
| :--- | :--- | :--- |
| `n` | integer | Số thứ tự phiên bản cập nhật. Bắt đầu từ `1` và tăng liên tục không có khoảng trống (`1, 2, 3...`). |
| `at` | string | Dấu thời gian ISO 8601 khi ghi nhận bản cập nhật. |
| `by` | string | Tác giả cập nhật: `"ai"` (AI tự vá khi spec đổi) hoặc `"user"` (người dùng sửa). |
| `note` | string | Tóm tắt lý do cập nhật bằng ngôn ngữ tự nhiên. |
| `trigger` | object | Chi tiết nguồn gốc dẫn đến việc cập nhật. |
| `impact` | object | Danh sách các scenario và setup bị ảnh hưởng trực tiếp (phục vụ cờ `--only-changed`). |
| `create` | object | Các thực thể mới được thêm vào (`seed`, `setup`, `scenarios`). |
| `change` | array[Change]| Chi tiết thay đổi ở cấp độ trường (`target`, `id`, `reason`, `before`, `after`). |
| `after_hash` | string | Mã băm SHA-256 12 ký tự của `spectest.yaml` ngay sau khi áp dụng bản update này. |

---

### Khối `trigger`

| Trường | Kiểu | Mô tả |
| :--- | :--- | :--- |
| `openapi.added` | array[string] | Các endpoint mới được bổ sung vào hợp đồng OpenAPI. |
| `openapi.changed` | array[string] | Các endpoint có schema, tham số hoặc phản hồi bị thay đổi. |
| `openapi.removed` | array[string] | Các endpoint bị xóa khỏi đặc tả. |
| `openapi.restored`| array[string] | Các endpoint từng bị xóa nay được khôi phục lại. |
| `requirements.before_hash` | string | Mã băm của tài liệu yêu cầu trước khi sửa. |
| `requirements.after_hash` | string | Mã băm của tài liệu yêu cầu sau khi sửa. |

---

### Đối tượng `change[]`

Ghi nhận các biến đổi chi tiết của từng phần tử:

```yaml
change:
  - target: scenario        # Đối tượng tác động: analysis, seed, setup, hoặc scenario
    id: create-order        # Định danh của phần tử
    reason: "Thêm API mới"  # Giải trình kỹ thuật
    before: null            # Giá trị trước thay đổi (null nếu là tạo mới)
    after:                  # Giá trị sau thay đổi (null nếu là xóa bỏ)
      id: create-order
```

---

## Thuật toán Băm Chuẩn tắc của Suite

SpecTest tính toán `after_hash` theo nguyên tắc tất định:
1. Trích xuất 4 khối mutable từ `spectest.yaml`:
   - `analysis`
   - `seed`
   - `setup`
   - `scenarios`
2. Marshal các khối thành chuỗi YAML chuẩn tắc (bỏ qua khối rỗng, sắp xếp key).
3. Tính mã băm SHA-256.
4. Cắt lấy **12 ký tự thập lục phân đầu tiên**.

### Ý nghĩa của việc Kiểm chứng Hash:
Khi chạy lệnh `spectest run --only-changed`:
- Runner tính `currentHash := domain.SuiteHash(suite)`.
- Nếu `currentHash != latest.AfterHash`, runner sẽ lập tức từ chối chạy.
- Cơ chế này ngăn chặn việc chạy test cập nhật một phần trên một file test suite đã bị người dùng sửa tay lệch nhịp.
