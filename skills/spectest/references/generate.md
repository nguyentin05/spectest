# Generate: tạo hoặc cập nhật test suite

## Trước khi chạy

- Đọc `./.spectest/config.yaml` nếu có; xác minh đường dẫn OpenAPI và alias đã lưu.
- Kiểm tra file đầu vào thực sự tồn tại và là OpenAPI YAML phù hợp.
- Xác định người dùng muốn `init` hay `update`. Không tự chọn update chỉ vì suite đã tồn tại.
- Luôn dùng `--output ./.spectest/`.
- Chỉ truyền `--req` khi có file yêu cầu nghiệp vụ Markdown thực tế. Không tạo yêu cầu giả.
- Chạy `scripts/preflight.sh init` hoặc `scripts/preflight.sh update` tương ứng.

## Init

```bash
spectest generate \
  --mode init \
  --output ./.spectest/ \
  --api auth=./path/to/auth-openapi.yaml
```

Nhiều API/server: thêm một `--api <alias>=<path>` cho từng file, và `--req` nếu có yêu cầu nghiệp vụ.

```bash
spectest generate \
  --mode init \
  --output ./.spectest/ \
  --api auth=./path/to/auth-openapi.yaml \
  --api orders=./path/to/orders-openapi.yaml \
  --req ./path/to/requirements.md
```

Ví dụ chỉ để minh họa; thay alias và đường dẫn bằng giá trị đã xác minh.

- Thứ tự `--api` có ý nghĩa với thứ tự server theo CLI.
- Nếu file đã tồn tại, CLI tự hỏi xác nhận overwrite. Để CLI hỏi trực tiếp; không tự xóa file và không dùng thủ thuật tự trả lời "yes". Nếu người dùng không xác nhận thì dừng và giữ nguyên file cũ.
- `--dry-run` chạy pipeline nhưng không ghi file. Chỉ dùng khi người dùng muốn xem trước hoặc kiểm tra pipeline.

## Update

Chỉ update khi cả hai file tồn tại:

- `./.spectest/spectest.yaml`
- `./.spectest/spectest.log.yaml`

Thiếu một trong hai thì không update; báo người dùng cần chạy lại `init`.

```bash
spectest generate \
  --mode update \
  --output ./.spectest/ \
  --api auth=./path/to/auth-openapi.yaml
```

Thêm đủ các `--api` liên quan đã xác minh; thêm `--req` chỉ khi có yêu cầu nghiệp vụ hiện hành cần dùng.

## Sau khi chạy

- Đọc thông báo của CLI và kiểm tra file đầu ra.
- Xác minh `spectest.yaml` và `spectest.log.yaml` vẫn tồn tại.
- Không khẳng định update đã áp dụng đúng nếu chỉ dựa vào exit text hoặc giả định; mở file hoặc log để xác nhận.
- Nếu thất bại, giữ lại thông tin lỗi và giải thích bước khắc phục.

## Nội dung suite

- Không bịa endpoint, HTTP method, request/response schema hay business rule.
- Khi requirement rõ ràng và không mâu thuẫn, dùng nó để diễn giải hành vi mong đợi. Khi mơ hồ, không biến suy đoán thành assertion chắc chắn.
- Không tự sửa hay ghi đè suite ngoài luồng generate mà CLI hỗ trợ, trừ khi người dùng yêu cầu.
