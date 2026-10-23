# Kiến trúc Hệ thống (System Architecture)

SpecTest-AI được thiết kế tuân thủ nghiêm ngặt mô hình **Clean Architecture / Hexagonal Ports & Adapters** (học tập trực tiếp từ kiến trúc của dự án mã nguồn mở `synapse-ce`), phân chia theo **Quy tắc phụ thuộc (The Dependency Rule)**.

---

## 1. Sơ đồ phân tầng kiến trúc

```text
┌─────────────────────────────────────────────────────────────┐
│  TẦNG NGOẠI VI (Driving & Driven Adapters)                  │
│  ├── CLI & Presenter (adapter/cli, adapter/presenter)       │
│  └── External Tech (llm, httpclient, docker, storage)       │
└──────────────────────────────┬──────────────────────────────┘
                               │ (chỉ phụ thuộc vào UseCase & Ports)
                               ▼
┌─────────────────────────────────────────────────────────────┐
│  TẦNG NGHIỆP VỤ (UseCase - Interactors)                     │
│  ├── generate/ : Pruning, Chunking, Sinh dữ liệu lai        │
│  ├── run/      : Replay RAM, Chạy API, 4-Tier, AI Triage    │
│  └── update/   : Oasdiff, Impact Analysis, Surgical Patch   │
└──────────────────────────────┬──────────────────────────────┘
                               │ (chỉ phụ thuộc vào Domain)
                               ▼
┌─────────────────────────────────────────────────────────────┐
│  TẦNG LÕI BẤT BIẾN (Domain - Pure Go, 0 external imports)   │
│  ├── Entities  : TestSuiteFile, Scenario, Step, TestCase    │
│  └── Ports     : LLMService, HTTPRunner, SuiteRepo, ...     │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. 4-Tier Test Oracle

Để kiểm thử toàn diện mà không phụ thuộc tốn kém vào token AI, SpecTest-AI áp dụng kim tự tháp kiểm chứng 4 tầng:

1. **Tier 1 (Status Oracle):** Kiểm tra mã trạng thái HTTP (200, 201, 400,...) khớp kỳ vọng.
2. **Tier 2 (Schema & Security Oracle):** Kiểm tra cấu trúc phản hồi khớp JSON Schema và cấm tuyệt đối lọt các khóa nhạy cảm (`password_hash`, `salt`, `secret_key`).
3. **Tier 3 (Business Logic Oracle):** Kiểm tra các biểu thức logic và giá trị chi tiết (ví dụ: `$.data.final_amount == 550000`).
4. **Tier 4 (State Consistency Oracle):** Bắn request ghi dữ liệu (`POST /orders`) và thực hiện request đọc (`GET /orders/{id}`) để xác minh cơ sở dữ liệu backend thực sự được cập nhật nhất quán.
