# Hướng dẫn Khởi động Nhanh (Quick Start)

Tài liệu hướng dẫn cài đặt và sử dụng **SpecTest-AI** trong 3 phút.

---

## 1. Cài đặt SpecTest-AI

### Cài đặt từ mã nguồn Go
Yêu cầu Go phiên bản 1.22 trở lên:
```bash
go install github.com/nguyentin05/spectest/cmd/spectest@latest
```

### Biên dịch thủ công từ mã nguồn
```bash
git clone https://github.com/nguyentin05/spectest.git
cd spectest
make build
./bin/spectest --help
```

---

## 2. Thiết lập thông tin AI Provider (`spectest config`)

SpecTest-AI hỗ trợ Google Gemini hoặc mô hình cục bộ qua Ollama:

```bash
# Cấu hình Google Gemini
spectest config init --provider gemini --endpoint https://generativelanguage.googleapis.com/v1beta/openai/ --model gemini-3.5-flash-lite
export SPECTEST_API_KEY="YOUR_GEMINI_API_KEY"

# Hoặc cấu hình Ollama (Offline / On-Premise)
spectest config init --provider ollama --endpoint http://localhost:11434/v1 --model qwen2.5-coder:7b
```

---

## 3. Khởi tạo Kịch bản Kiểm thử (`spectest generate`)

Chuyển đổi file mô tả API `openapi.yaml` (kèm file yêu cầu nghiệp vụ `requirements.md` nếu có):

```bash
spectest generate -a openapi.yaml -r requirements.md -o ./tests
```

Lệnh này sẽ tạo ra file kịch bản duy nhất `tests/spectest.yaml` kèm tài liệu ma trận kiểm thử `tests/test_matrix.md`.
