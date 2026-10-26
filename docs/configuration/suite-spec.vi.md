# Đặc tả Suite `spectest.yaml`

`spectest.yaml` là hợp đồng kịch bản kiểm thử trọng tâm của SpecTest (Schema Version `1`). File định nghĩa các alias máy chủ đã đăng ký, danh sách các API được bao phủ, tài liệu phân tích nghiệp vụ, biến seed toàn cục, các khối setup tái sử dụng và danh sách các scenario kiểm thử.

---

## Ví dụ Chú giải Toàn diện

```yaml
version: 1
input: openapi
servers: [shop]
apis: 
  - "POST {shop}/api/login"
  - "GET {shop}/api/products"
  - "POST {shop}/api/orders"

analysis: |
  Bộ kiểm thử xác thực phiên đăng nhập trước, trích xuất access token,
  liệt kê danh sách sản phẩm và thực thi tạo đơn hàng với trạng thái DB nhất quán.

# Biến seed toàn cục dùng chung cho tất cả setup và scenario
seed:
  credentials:
    vars:
      email: demo@example.com
      password: demo-password

# Các khối setup tái sử dụng, thực thi theo thuật toán DAG
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

  product:
    use: [authenticated_user]
    steps:
      - call: "GET {shop}/api/products"
        headers:
          Authorization: "Bearer {{access_token}}"
        expect: 200
        save:
          product_id: body.items.0.id

# Danh sách các kịch bản kiểm thử nghiệp vụ
scenarios:
  - id: list-products
    name: Liệt kê sản phẩm với tư cách người dùng đã đăng nhập
    setup: [authenticated_user]
    steps:
      - call: "GET {shop}/api/products"
        headers:
          Authorization: "Bearer {{access_token}}"
        expect: 200

  - id: create-order
    name: Tạo đơn hàng sử dụng ID sản phẩm đã trích xuất
    setup: [product]
    steps:
      - call: "POST {shop}/api/orders"
        headers:
          Authorization: "Bearer {{access_token}}"
        body:
          product_id: "{{product_id}}"
          quantity: 1
        expect: 201
        save:
          order_id: body.id
```

---

## Bảng Tham chiếu Schema

### Các Trường ở Cấp Root

| Trường | Kiểu | Bắt buộc | Mô tả |
| :--- | :--- | :--- | :--- |
| `version` | integer | **Có** | Phiên bản schema. Bắt buộc phải là `1`. |
| `input` | string | **Có** | Chế độ đầu vào đã tạo nên suite: `openapi` hoặc `requirements`. |
| `servers` | array[string] | **Có** | Danh sách các alias máy chủ được dùng trong các biểu thức `call` (ví dụ `[shop, billing]`). |
| `apis` | array[string] | **Có** | Danh sách các operation API được bao phủ (ví dụ `"POST {shop}/api/login"`). |
| `analysis` | string | Không | Tài liệu Markdown mô tả cơ sở và chiến lược kiểm thử. (Thuộc phần Mutable) |
| `seed` | map[string]Seed | Không | Các nhóm biến mặc định toàn cục dùng chung cho mọi scenario. (Thuộc phần Mutable) |
| `setup` | map[string]Setup| Không | Các khối chuẩn bị dữ liệu trước khi chạy scenario. (Thuộc phần Mutable) |
| `scenarios`| array[Scenario]| **Có** | Danh sách các kịch bản kiểm thử nghiệp vụ. (Thuộc phần Mutable) |

---

### Khối `seed.<nhom>`

Khai báo các nhóm biến môi trường toàn cục:

```yaml
seed:
  auth:
    vars:
      tenant_id: "tenant-beta"
      user_id: 1001
```

> [!IMPORTANT]
> **Quy tắc Chống Trùng Tên**: Tên biến giữa các nhóm seed khác nhau bắt buộc phải là duy nhất. Nếu `seed.group1.vars.token` và `seed.group2.vars.token` cùng tồn tại, SpecTest sẽ từ chối suite và báo lỗi va chạm tên biến.

---

### Khối `setup.<ten>`

Các khối tái sử dụng được thực thi trước các bước chính của scenario:

| Trường | Kiểu | Mô tả |
| :--- | :--- | :--- |
| `use` | array[string] | Danh sách tên các setup khác cần phải chạy trước setup này (tập hợp phụ thuộc, không phải thứ tự cố định). |
| `vars` | map[string]any | Các biến cục bộ của riêng khối setup này. Hỗ trợ nội suy `{{var}}`. |
| `steps` | array[Step] | Các bước HTTP tuần tự để chuẩn bị trạng thái. |

---

### Đối tượng `scenarios[]`

| Trường | Kiểu | Bắt buộc | Mô tả |
| :--- | :--- | :--- | :--- |
| `id` | string | **Có** | Mã định danh duy nhất dạng kebab-case ổn định (ví dụ `create-order`). |
| `name` | string | **Có** | Tên mô tả kịch bản kiểm thử dễ đọc cho con người. |
| `setup` | array[string] | Không | Các khối setup trực tiếp mà kịch bản này yêu cầu. |
| `steps` | array[Step] | **Có** | Các bước HTTP tuần tự được thực thi trong kịch bản này. |
| `locked` | boolean | Không | Nếu `true`, bảo vệ kịch bản không bị AI tự động sửa khi cập nhật spec. |
| `skip` | string | Không | Nếu không rỗng, bỏ qua kịch bản này và ghi chuỗi này làm lý do skip. |

---

### Đối tượng `Step`

| Trường | Kiểu | Bắt buộc | Mô tả |
| :--- | :--- | :--- | :--- |
| `call` | string | **Có** | Lệnh gọi HTTP theo cú pháp `"METHOD {server}/path"` (ví dụ `"POST {shop}/api/orders"`). |
| `expect` | integer | **Có** | Mã trạng thái HTTP kỳ vọng (ví dụ `200`, `201`, `400`). |
| `headers` | map[string]string | Không | Cặp key-value các header của request. Hỗ trợ nội suy `{{var}}`. |
| `body` | any | Không | Payload JSON của request. Hỗ trợ object lồng nhau, mảng và nội suy `{{var}}`. |
| `save` | map[string]string | Không | Quy tắc trích xuất biến từ HTTP response vào runtime scope. |

---

## Quy tắc Nội suy Biến & Trích xuất Dữ liệu

### 1. Cú pháp Placeholder
- `{{ten_bien}}`: Được thay thế bằng giá trị của `ten_bien` trong scope hiện tại của scenario.
- `{{random}}`: Tự động sinh ra một token hex ngẫu nhiên, duy nhất cho mỗi lần xuất hiện.

### 2. Giữ nguyên Kiểu Dữ liệu Nguyên thủy (Type Preservation)
Khi một trường trong JSON body chỉ chứa đúng duy nhất một tham chiếu biến, SpecTest sẽ bảo toàn kiểu dữ liệu Go gốc:

```yaml
body:
  quantity: "{{item_count}}"  # Giữ nguyên kiểu int nếu item_count là int
  is_active: "{{is_active}}"  # Giữ nguyên kiểu bool nếu is_active là bool
```

### 3. Quy tắc Trích xuất Response (`save`)
- **Trích xuất từ JSON Body (`body.<dot-path>`)**:
  - `access_token: body.token` $\rightarrow$ đọc trường `token` ở root response.
  - `product_id: body.items.0.id` $\rightarrow$ đọc trường `id` của phần tử đầu tiên trong mảng `items`.
- **Trích xuất từ HTTP Header (`header.<ten>`)**:
  - `session_id: header.Set-Cookie`
  - `etag: header.ETag`
