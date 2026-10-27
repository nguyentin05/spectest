# Prepare: workspace, OpenAPI, server URL, Compose test

Mục tiêu: có đủ đầu vào đã xác minh để generate và chạy SpecTest, đồng thời cô lập test khỏi dữ liệu development/production.

## 1. Khảo sát dự án

Đọc README, Dockerfile, compose files, file cấu hình và scripts khởi động trước khi kết luận. Xác định framework, cách chạy app, các API service, database/cache/message broker và biến môi trường. Không sửa môi trường dev/prod và không chạy lệnh có thể xóa dữ liệu khi chưa rõ phạm vi.

## 2. Config server

Lưu thông tin đã xác nhận trong `./.spectest/config.yaml`:

```yaml
servers:
  auth:
    openapi: ./path/to/openapi.yaml
    base_url: http://localhost:8000
```

- `auth` là alias, dùng nhất quán trong `--api auth=<path>` và `-t auth=<url>`.
- `openapi` là đường dẫn tới file OpenAPI YAML thực tế.
- `base_url` là URL runtime khi test chạy.
- Không suy ra URL từ tên alias hay từ mục `servers` trong OpenAPI. Xác minh bằng cấu hình dự án hoặc hỏi người dùng.
- Không ghi secrets/token/password vào config trừ khi người dùng yêu cầu và có cách lưu an toàn.

## 3. OpenAPI

1. Tìm các OpenAPI/Swagger YAML trong repo và xác minh nội dung đúng với service cần test.
2. Nếu chưa có, xem framework và dependency để tìm cơ chế export spec chuẩn của framework, rồi hướng dẫn hoặc thực hiện theo cách đó.
3. Không viết OpenAPI giả chỉ để lệnh generate chạy được: suite sinh ra sẽ kiểm thử một hợp đồng không tồn tại.
4. Không copy/move file OpenAPI. Tham chiếu file gốc bằng đường dẫn đã xác minh; chỉ copy vào `.spectest/openapi/` khi người dùng đồng ý.
5. Với mỗi OpenAPI, thu thập: alias duy nhất, đường dẫn file, URL runtime, và nguồn của URL (cấu hình repo hay người dùng cung cấp).

## 4. Docker Compose test

`spectest run` tự khởi động Compose, chạy test rồi teardown. Hãy giả định volumes có thể bị xóa sau mỗi lần chạy.

1. Tìm Compose, Dockerfile, scripts khởi động và cấu hình môi trường hiện có.
2. Ưu tiên tạo hoặc dùng cấu hình tách biệt, ví dụ `./.spectest/docker-compose.test.yml`.
3. Chỉ tái dùng biến môi trường khi đã hiểu ý nghĩa và chắc chắn chúng trỏ tới tài nguyên test cô lập.
4. Không gắn volume chứa dữ liệu cần giữ, không trỏ tới database dev/prod, không dùng credentials production.
5. Nếu chưa chắc app build/start thế nào, database cấu hình ra sao hoặc cách cô lập volume/database, hỏi người dùng trước khi tạo hay sửa Compose.
6. Không chạy Compose hoặc thao tác destructive để "thử xem có chạy không" khi phạm vi tác động chưa rõ.

## Điều kiện sẵn sàng

Chỉ báo "chuẩn bị xong" khi đủ tất cả:

- có OpenAPI YAML hợp lệ cho từng đầu vào;
- mỗi alias có đường dẫn OpenAPI và URL runtime đã xác minh;
- config đã lưu trong `./.spectest/config.yaml`;
- có Compose test an toàn, hoặc đã hỏi người dùng cách chuẩn bị;
- không còn mơ hồ nào có thể khiến test trỏ nhầm vào môi trường thật.

Sau đó chạy `scripts/preflight.sh init` để kiểm tra lại.
