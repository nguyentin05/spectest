# SpecTest-AI

> **Autonomous AI Test Engineering Engine & GitHub Action**  
> Chuyển đổi yêu cầu phần mềm thành kịch bản kiểm thử, sinh dữ liệu kiểm thử lai, tự động thực thi và tự chữa lành khi API thay đổi.

---

## 1. Giới thiệu tổng quan

**SpecTest-AI** là công cụ mã nguồn mở (FOSS) tự động hóa kiểm thử API và dịch vụ web theo triết lý **Zero-Token First**:
1. **Khởi tạo tự động (`generate`):** Phân tích tài liệu OpenAPI (Swagger) và PRD Markdown để tạo kịch bản kiểm thử tích hợp luồng nghiệp vụ hoàn chỉnh.
2. **Sinh dữ liệu kiểm thử lai (Hybrid Test Data):** Kết hợp khả năng suy luận ngữ cảnh của AI với thuật toán tất định thuần Go (0-token fuzzing, nulls, negative numbers, SQL injection).
3. **Thực thi và chẩn đoán thông minh (`run`):** Thực thi kịch bản bằng HTTP engine, đánh giá qua **4-Tier Test Oracle**, và kích hoạt **AI Defect Triage** để phân tích nguyên nhân gốc khi có ca kiểm thử thất bại.
4. **Tiến hóa kịch bản (`update`):** Sử dụng thuật toán so khớp đặc tả `tufin/oasdiff` để bóc tách breaking changes và vá có chọn lọc (Surgical Patching).
5. **Cổng kiểm soát chất lượng CI/CD:** Sẵn sàng tích hợp vào GitHub Actions Marketplace với composite action chỉ mất vài giây cài đặt.

---

## 2. Triết lý thiết kế cốt lõi

1. **Zero-Token First:** Những gì Golang giải quyết được bằng thuật toán tất định (0 token) thì tuyệt đối không gọi AI. Tiết kiệm 85% - 92% chi phí LLM.
2. **Vendor-Agnostic & 100% FOSS:** Hỗ trợ song song cả Cloud API (Google Gemini, OpenAI) và mô hình chạy Offline/On-Premise (Ollama, vLLM với Qwen2.5-Coder hoặc DeepSeek).
3. **Single-File Portability:** Kịch bản, dữ liệu kiểm thử lai và lịch sử cập nhật gói gọn trong đúng 1 file `spectest.yaml`.
4. **All-in-One Infrastructure Lifecycle:** Tự động điều phối toàn bộ vòng đời hạ tầng (Docker Compose, cổng chờ healthcheck, teardown an toàn bằng `defer`).
5. **The Rule of 3 CLI UX:** Đóng gói thao tác hàng ngày vào đúng 3 lệnh: `generate`, `run`, `update --consolidate` (kèm lệnh phụ trợ `config`).
