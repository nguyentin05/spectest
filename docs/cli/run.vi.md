# `spectest run`

Thực thi bộ kịch bản kiểm thử `spectest.yaml` trên các dịch vụ API mục tiêu, tự động giải quyết phụ thuộc setup qua DAG, quản lý nối chuỗi biến động, hỗ trợ đa server và chẩn đoán lỗi bằng AI.

---

## Cú pháp

```bash
spectest run [flags]
```

---

## Mô tả Hoạt động

Lệnh `run` thực thi các kịch bản kiểm thử theo thứ tự:
1. **Kiểm tra Target & Thăm dò Sẵn sàng**: Chuẩn hóa target đơn lẻ hoặc nhiều alias (`-t alias=url`), tự khởi động Docker Compose nếu có cấu hình và ping kiểm tra tất cả các server cho tới khi sẵn sàng.
2. **Giải quyết Setup DAG**: Tự động tính toán cây phụ thuộc (`use: [...]`), kiểm tra chu trình và thực thi các bước setup trước khi chạy bước chính của scenario.
3. **Nối Chuỗi Biến Động**: Thay thế các placeholder `{{variable}}` và `{{random}}` trong URL, header và body bằng giá trị thu thập được từ các bước trước.
4. **Thực thi Bước**: Gửi request HTTP, giới hạn response tối đa 1MB, so khớp mã trạng thái và trích xuất dữ liệu qua `save`.
5. **AI Defect Triage**: Nếu bật `--triage` và có bước bị fail, tự động chụp snapshot và gọi LLM phân tích nguyên nhân gốc rễ (RCA).

---

## Bảng Cờ Lệnh & Tùy chọn

| Cờ | Rút gọn | Kiểu | Mặc định | Bắt buộc | Mô tả |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `--file` | `-f` | string | `"./tests/spectest.yaml"` | Không | Đường dẫn tới file suite kiểm thử YAML. |
| `--target` | `-t` | stringArray | `[]` | Không | URL máy chủ mục tiêu (URL đơn hoặc ánh xạ alias e.g. `-t auth=http://localhost:8001`). Có thể lặp lại nhiều lần. |
| `--compose` | `-c` | string | `""` | Không | Đường dẫn tới file Docker Compose. Tự động bật container, đợi healthcheck và dọn dẹp khi xong. |
| `--timeout` | | int | `30` | Không | Thời gian tối đa (giây) chờ server khởi động sẵn sàng. |
| `--triage` | | boolean | `false` | Không | Kích hoạt AI Defect Triage chẩn đoán nguyên nhân khi có bước test bị fail (yêu cầu cấu hình LLM). |
| `--only-changed` | | boolean | `false` | Không | Chỉ chạy các kịch bản bị ảnh hưởng bởi bản cập nhật mới nhất trong `spectest.log.yaml`. |
| `--log` | | string | `""` | Không | Đường dẫn tới `spectest.log.yaml`; mặc định nằm cạnh file `--file`. |
| `--report-dir` | | string | `"./reports"` | Không | Thư mục xuất báo cáo kiểm thử (`junit.xml`, `summary.json`, `report.html`). |
| `--verbose` | | boolean | `false` | Không | Bật log debug chi tiết toàn bộ HTTP request và response payload. |

---

## Ví dụ Thực tế

### 1. Chạy trên Local Server
```bash
spectest run -f ./tests/spectest.yaml -t http://localhost:8080
```

### 2. Chạy với Nhiều Microservices
Ánh xạ các alias máy chủ khai báo trong `spectest.yaml`:
```bash
spectest run -f ./tests/spectest.yaml \
  -t shop=http://localhost:8001 \
  -t billing=http://localhost:8002
```

### 3. Tự động Quản lý Docker Compose & Phân tích Lỗi bằng AI
Khởi động container backend, chạy kiểm thử, phân tích lỗi tự động nếu fail và tự dọn dẹp an toàn:
```bash
spectest run -f ./tests/spectest.yaml \
  --compose docker-compose.test.yml \
  -t http://localhost:8080 \
  --triage
```

### 4. Chỉ chạy các Kịch bản bị Ảnh hưởng (Only Changed)
Chỉ thực thi các kịch bản bị tác động bởi lần cập nhật gần nhất trong file lịch sử `spectest.log.yaml`:
```bash
spectest run -f ./tests/spectest.yaml -t http://localhost:8080 --only-changed
```

---

## Báo cáo Kết quả Đầu ra

Sau khi chạy xong, các báo cáo được lưu trong `--report-dir`:
- **`summary.json`**: Báo cáo JSON cấu trúc chứa thời lượng, tổng số kiểm tra đã đánh giá và bản ghi từng bước.
- **`junit.xml`**: Định dạng JUnit XML tiêu chuẩn tương thích với các hệ thống CI (GitHub Actions, GitLab CI, Jenkins).
- **`report.html`**: Báo cáo HTML dashboard độc lập với dòng thời gian trực quan, phần trăm phân vị latency và kết quả AI triage.
