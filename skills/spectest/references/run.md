# Run: chạy test an toàn

## Điều kiện trước khi chạy

Chạy `scripts/preflight.sh run`, rồi xác minh đủ năm điều:

1. `./.spectest/spectest.yaml` tồn tại.
2. Compose test tồn tại và đã được xác nhận an toàn.
3. Mỗi server alias có URL runtime trong `./.spectest/config.yaml` hoặc do người dùng xác nhận.
4. Compose không trỏ tới database, volume hay credentials development/production.
5. Người dùng đã yêu cầu chạy test rõ ràng.

Thiếu suite hoặc Compose thì quay lại `references/prepare.md` hoặc `references/generate.md`.

## Lệnh

Luôn truyền cả `-c` và `-f`.

```bash
spectest run \
  -c ./.spectest/docker-compose.test.yml \
  -f ./.spectest/spectest.yaml \
  -t auth=http://localhost:8000
```

- `-c`: đường dẫn Compose test đã xác minh.
- `-f`: đường dẫn suite, thường là `./.spectest/spectest.yaml`.
- `-t`: lặp lại cho từng alias, dạng `alias=URL`. Dùng URL đã lưu hoặc đã xác minh, không suy ra từ tên alias.
- `--log`: chỉ cần khi log không nằm cạnh suite; mặc định CLI tìm `spectest.log.yaml` cạnh suite.

URL `http://localhost:8000` ở trên chỉ là ví dụ.

## `--only-changed`

Chỉ thêm khi người dùng muốn chạy các scenario bị ảnh hưởng bởi lần update gần nhất, hoặc đã nói rõ chỉ chạy phần thay đổi. Đừng diễn giải thành "chạy tất cả test mới"; theo mô tả của CLI, cờ này chạy các scenario bị ảnh hưởng bởi update gần nhất.

```bash
spectest run \
  -c ./.spectest/docker-compose.test.yml \
  -f ./.spectest/spectest.yaml \
  -t auth=http://localhost:8000 \
  --only-changed
```

## `--triage`

- Mặc định không bật.
- Chỉ bật khi người dùng yêu cầu phân loại lỗi bằng LLM **và** LLM đã được cấu hình.
- Nếu người dùng yêu cầu nhưng chưa có LLM, nêu rõ điều kiện còn thiếu và hỏi họ có muốn cấu hình không. Không tự bật.
- Không tự đổi provider, model hay API key.

## An toàn khi teardown

SpecTest tự khởi động môi trường, chạy test rồi teardown, và volumes có thể bị xóa. Vì vậy:

- không dùng Compose development/production nếu có thể ảnh hưởng dữ liệu;
- không gắn volume chứa dữ liệu cần giữ;
- không trỏ test tới database thật;
- nếu không chứng minh được môi trường cô lập, hỏi người dùng và không chạy.

## Sau khi chạy

- Thu thập exit status và output thực tế.
- Tìm report theo output của CLI hoặc đường dẫn đã cấu hình. Không đoán tên report chưa xác minh.
- Test thất bại thì chuyển sang `references/analyze.md`.
- Không tuyên bố pass khi chưa xem kết quả thực tế.
