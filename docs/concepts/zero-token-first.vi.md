# Triết lý Zero-Token First

Một nguyên tắc cốt lõi trong thiết kế của **SpecTest** là: **Tuyệt đối không tiêu tốn token LLM đắt đỏ cho những công việc mà thuật toán tất định của Go giải quyết được nhanh hơn, rẻ hơn và đạt độ chính xác 100%.**

---

## Vấn đề Chi phí & Sự bất ổn định khi phụ thuộc LLM

Các công cụ "sinh test bằng AI" truyền thống thường ném toàn bộ đặc tả OpenAPI (thường dài hàng chục nghìn dòng JSON/YAML) trực tiếp vào prompt của LLM. Cách làm này gây ra nhiều hệ quả nghiêm trọng:

1. **Lãng phí Token khổng lồ**: Các trường mô tả dài dòng, schema lặp lại và header mẫu chiếm hơn 80% dung lượng ngữ cảnh.
2. **Loãng ngữ cảnh (Context Dilution)**: Ngữ cảnh quá lớn làm giảm sút độ chính xác trong suy luận của LLM, dẫn đến việc ảo giác ra các endpoint không có thật, sai tham số hoặc gãy luồng xác thực.
3. **Chi phí vận hành rất cao**: Chạy sinh hoặc cập nhật test thường xuyên trong CI/CD cho các hệ thống microservices lớn sẽ nhanh chóng làm chi phí AI tăng vọt.
4. **Không ổn định (Flakiness)**: Dùng LLM để so lệch spec, kiểm tra mã trạng thái hoặc sắp xếp phụ thuộc sẽ đem lại kết quả bất định giữa các lần chạy.

---

## Phân định trách nhiệm Kiến trúc

SpecTest phân tách rạch ròi trách nhiệm giữa thuật toán Go hiệu năng cao và tư duy ngữ nghĩa của LLM:

| Tác vụ | Thành phần xử lý | Chi phí Token | Lý do triển khai |
| :--- | :--- | :--- | :--- |
| **Chuẩn hóa & Tỉa gọn OpenAPI** | `infrastructure/openapi/` | **0 tokens** | Loại bỏ model thừa, mô tả dài dòng và nhiễu định dạng trong Go. |
| **Phân mảnh Context** | `infrastructure/openapi/chunker.go` | **0 tokens** | Chia nhỏ đặc tả thành các miền nghiệp vụ theo tags và đường dẫn API. |
| **Phát hiện & Nối Auth** | `infrastructure/openapi/auth_detector.go` | **0 tokens** | Tự động nhận diện route đăng nhập (`/login`, `/token`) và gắn Bearer token. |
| **Tái tạo API Catalog** | `usecase/generate/update_catalog.go` | **0 tokens** | Tái dựng catalog trong bộ nhớ từ base và các bản vá patch trong `spectest.log.yaml`. |
| **So lệch API Tất định** | `usecase/generate/update_diff.go` | **0 tokens** | So khớp AST OpenAPI để tìm endpoint thêm, đổi, xóa, khôi phục. |
| **Giải quyết Đồ thị Tác động** | `usecase/generate/update_impact.go` | **0 tokens** | Tính toán các scenario bị ảnh hưởng trực tiếp và gián tiếp qua DAG. |
| **Phân giải Setup DAG** | `usecase/run/dag.go` | **0 tokens** | Tính bao đóng phụ thuộc, kiểm tra chu trình và sắp xếp tô-pô bằng DFS. |
| **Nội suy Scope & Bảo toàn Kiểu** | `usecase/run/scope.go` | **0 tokens** | Thay thế biến (`{{var}}`, `{{random}}`) và bảo toàn kiểu dữ liệu Go gốc. |
| **Kiểm tra Response & Trích xuất** | `usecase/run/step.go` | **0 tokens** | So khớp HTTP status, trích xuất dữ liệu (`body.<path>`, `header.<name>`) và giới hạn 1MB. |
| **Che giấu Dữ liệu Nhạy cảm** | `usecase/run/redact.go` | **0 tokens** | Làm sạch mật khẩu, token, cookie và tham số URL nhạy cảm trước khi log. |
| **Vòng đời Target & Thăm dò** | `infrastructure/server/` | **0 tokens** | Điều phối Docker Compose và thăm dò readiness probe trước khi chạy test. |
| **Sinh Luồng Nghiệp vụ** | `usecase/generate/` | **~8%–15% tokens** | Tư duy ngữ nghĩa cấp cao để thiết kế các kịch bản người dùng chân thực. |
| **AI Defect Triage** | `usecase/run/triage.go` | **Chỉ khi có lỗi** | Phân tích ngữ cảnh lỗi, payload và hợp đồng API để tìm nguyên nhân gốc rễ theo yêu cầu. |

---

## Hiệu quả Thực tế

- **Tiết kiệm 85% đến 92% chi phí LLM**: Token chỉ được tiêu thụ cho các tác vụ suy luận có giá trị thực sự.
- **Tốc độ micro giây**: Tỉa gọn spec, sắp xếp DAG, tái tạo catalog và nội suy biến diễn ra tức thì trong Go.
- **Tính lặp lại và ổn định cao**: Quá trình thực thi test và so lệch spec diễn ra tất định 100%, không bị ảnh hưởng bởi nhiệt độ (temperature) hay độ trôi của mô hình AI.
