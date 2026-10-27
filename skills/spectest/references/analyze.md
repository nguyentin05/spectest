# Analyze: đọc kết quả và phân loại lỗi

## Nguồn dữ liệu

- Output và exit status của lần `spectest run` vừa thực hiện.
- Các report thực tế trong report directory đã chọn, và log liên quan.
- Kết quả triage do CLI tạo ra, chỉ khi đã bật `--triage`. Không giả định triage tồn tại nếu không bật cờ.
- Phải mở file report; không suy ra nội dung từ tên file hay từ output rút gọn.

## Phân loại lỗi

Dựa trên bằng chứng, xếp mỗi lỗi vào một nhóm:

1. **Lỗi môi trường**: app/database chưa sẵn sàng, connection refused, healthcheck/readiness timeout, thiếu biến môi trường.
2. **Lỗi request hoặc dữ liệu test**: request sai định dạng, thiếu auth/setup, state chaining hoặc dữ liệu đầu vào không hợp lệ.
3. **Lỗi assertion/contract**: status code, response schema hoặc kết quả nghiệp vụ không khớp kỳ vọng đã định nghĩa.
4. **Lỗi ứng dụng có khả năng cao**: bằng chứng cho thấy API xử lý sai so với contract/requirement đã xác minh.
5. **Chưa đủ bằng chứng**: còn nhiều nguyên nhân khả dĩ; cần thêm log, response body hoặc cấu hình.

Đừng gọi mọi test failure là bug ứng dụng. Gắn nhãn rõ "đã xác nhận", "khả năng cao" hoặc "chưa rõ" cho từng kết luận.

## Cách trình bày

Mở đầu bằng kết luận, rồi tóm tắt:

- trạng thái tổng thể;
- số pass/fail/skipped nếu report có;
- scenario/step thất bại;
- request/response hoặc assertion liên quan;
- phân loại nguyên nhân và mức chắc chắn;
- bước tiếp theo để xác minh.

Trích file hoặc dòng log cụ thể khi có thể. Không lộ secrets.

## Bước tiếp theo theo loại lỗi

- **Môi trường/cấu hình**: đề xuất sửa cấu hình test hoặc Compose sau khi xác minh; không đụng tới môi trường thật.
- **Contract/requirement**: xác định spec, requirement hay implementation đang sai; hỏi người dùng trước khi đổi expected behavior còn mơ hồ.
- **Lỗi ứng dụng**: nêu bằng chứng và đề xuất cách tái hiện hoặc sửa. Không tự sửa source code trừ khi người dùng yêu cầu.
- **Chạy lại phần bị ảnh hưởng bởi update gần nhất**: đề xuất `--only-changed`, vẫn truyền `-c` và `-f` (xem `references/run.md`).
- **`--triage`**: chỉ đề xuất khi người dùng muốn dùng LLM; nếu chưa cấu hình LLM, làm theo `references/run.md`.
