---
name: spectest
description: Hỗ trợ developer dùng SpecTest CLI để kiểm thử API từ OpenAPI - chuẩn bị workspace .spectest, generate test suite (init/update), chạy `spectest run` với Docker Compose, và phân tích kết quả/lỗi test. Hãy dùng skill này bất cứ khi nào người dùng nhắc đến spectest, test API từ OpenAPI/Swagger, tạo hoặc cập nhật test suite, spectest.yaml, spectest.log.yaml, docker-compose.test.yml, --only-changed, --triage, chạy lại test, hoặc hỏi vì sao test fail - kể cả khi họ chỉ nói "chạy test cho API này", "sinh test từ openapi", "update suite", "xem kết quả test" mà không nêu tên SpecTest.
---

# SpecTest

Skill này điều phối toàn bộ vòng đời SpecTest: **prepare → generate → run → analyze**. File này chứa quy trình và các quy tắc áp dụng cho mọi bước. Chi tiết từng bước nằm trong `references/`, chỉ đọc file của bước đang làm để tiết kiệm context.

| Bước | Khi nào | Đọc |
| --- | --- | --- |
| Prepare | Chưa có OpenAPI, alias/URL, hoặc Compose test | `references/prepare.md` |
| Generate | Tạo (`init`) hoặc cập nhật (`update`) suite | `references/generate.md` |
| Run | Chạy `spectest run` | `references/run.md` |
| Analyze | Có kết quả chạy cần giải thích | `references/analyze.md` |

Người dùng có thể chỉ cần một bước (ví dụ "xem vì sao test fail"). Xác định họ đang ở đâu rồi nhảy thẳng tới bước đó, không bắt đi lại từ đầu.

## Kiểm tra trước khi thao tác

Chạy `scripts/preflight.sh <init|update|run>` trước mỗi bước generate hoặc run. Script kiểm tra file bắt buộc, đọc `config.yaml`, đối chiếu cờ CLI thực tế với `--help`, và cảnh báo các dấu hiệu Compose không an toàn. Script chỉ phát hiện được dấu hiệu bề mặt; không thay thế việc bạn đọc Compose và xác nhận với người dùng.

Nếu script báo thiếu cờ so với skill, tin vào `spectest <lệnh> --help` của CLI đang cài, vì skill có thể lỗi thời so với phiên bản CLI.

## Quy tắc xuyên suốt

Các quy tắc dưới đây tồn tại vì `spectest run` có thể xóa volumes và vì suite sinh ra từ dữ liệu bịa sẽ cho kết quả test vô nghĩa.

- **Không bịa dữ liệu đầu vào.** Không tự viết OpenAPI giả, không đoán endpoint, URL, biến môi trường hay cách khởi động dịch vụ. Chưa có thì khảo sát repo, rồi hỏi người dùng.
- **Cô lập dữ liệu test.** Không chạy Compose trỏ tới database, volume hoặc credentials development/production. Nếu không chứng minh được môi trường cô lập thì dừng và hỏi, không chạy "thử xem sao".
- **Chỉ chạy test khi người dùng yêu cầu rõ.** Generate và prepare không đồng nghĩa với được phép run.
- **Cờ bắt buộc:** `spectest generate` luôn có `--output ./.spectest/`; `spectest run` luôn có cả `-c` và `-f`, kể cả khi `-f` có default.
- **Update cần đủ hai file** `./.spectest/spectest.yaml` và `./.spectest/spectest.log.yaml`. Thiếu một trong hai thì báo người dùng chạy lại `init`, không tự chuyển sang update.
- **`--triage` mặc định tắt.** Chỉ bật khi người dùng yêu cầu và LLM đã được cấu hình. Không tự đổi provider, model hay API key.
- **Không tuyên bố kết quả khi chưa đọc.** Không nói update thành công, test pass hay triage xong nếu chưa mở output/report thực tế.
- **Không tự sửa** source code ứng dụng, requirement hay suite ngoài luồng CLI hỗ trợ, trừ khi người dùng yêu cầu.
- **Không ghi secrets** vào config hay trích chúng ra trong báo cáo.

## Workspace

Các file do agent quản lý hoặc SpecTest tạo nằm trong `./.spectest/`:

```text
.spectest/
  config.yaml              # quy ước của skill này, CLI không tự tạo/đọc
  docker-compose.test.yml
  spectest.yaml
  spectest.log.yaml
  reports/
```

`config.yaml` là quy ước của skill để nhớ alias, đường dẫn OpenAPI và URL runtime đã xác nhận. Không nói rằng CLI quản lý file này. Không tự copy hay di chuyển OpenAPI vào `.spectest/openapi/`; chỉ làm khi người dùng đồng ý.

## Cách giao tiếp

- Khi thiếu thông tin thiết yếu, hỏi từng câu ngắn. Khảo sát repo trước để giảm số câu phải hỏi.
- Trước thao tác có thể ghi đè file, xóa dữ liệu hoặc đổi cấu hình môi trường, nói rõ phạm vi và xin xác nhận.
- Khi một bước chưa làm được, nói rõ đang thiếu gì.
- Khi báo cáo, mở đầu bằng kết luận (trạng thái, số liệu), sau đó mới tới chi tiết. Phân biệt "đã xác nhận", "khả năng cao" và "chưa rõ".
