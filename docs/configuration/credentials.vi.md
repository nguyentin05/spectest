# Quản lý Thông tin Chứng thực & Nhà cung cấp

SpecTest hỗ trợ cả các API đám mây thương mại (Google Gemini, OpenAI) và các mô hình mã nguồn mở chạy cục bộ (Ollama, vLLM, LocalAI).

---

## Lưu trữ & Phân quyền File

Các thiết lập provider và API key được lưu tại:

```text
~/.spectest/credentials.yaml
```

- **Quyền File**: Được tạo với quyền `0600` (`-rw-------`), chỉ cho phép đúng tài khoản người dùng hiện tại được đọc và ghi.
- **Quyền Thư mục**: Thư mục cha `~/.spectest/` được tạo với quyền `0700` (`drwx------`).
- **Ẩn Key trên Terminal**: Khi nhập API key tương tác, terminal sẽ tắt echo để tránh lộ màn hình hoặc lưu vào lịch sử lệnh.
- **Ẩn Key khi Hiển thị**: Lệnh `spectest config list` sẽ tự động che key: `AIzaSy************************3X81`.

---

## Các Nhà Cung Cấp Được Hỗ Trợ

### 1. Google Gemini (Cloud API)
Tối ưu cho việc sinh test với thông lượng cao và chi phí AI Defect Triage cực thấp.

```bash
spectest config init --provider gemini --model gemini-2.5-flash
spectest config set api-key
```

- **Endpoint Mặc định**: `https://generativelanguage.googleapis.com/v1beta/openai/`
- **Model Khuyến nghị**: `gemini-2.5-flash`, `gemini-1.5-pro`

### 2. OpenAI & Các Gateway Tương thích
Tương thích với OpenAI cũng như các gateway hỗ trợ chuẩn REST OpenAI (LiteLLM, Groq, OpenRouter).

```bash
spectest config init \
  --provider openai \
  --endpoint https://api.openai.com/v1 \
  --model gpt-4o-mini

spectest config set api-key
```

### 3. Ollama (Chạy Cục bộ / Offline / Air-Gapped)
Vận hành hoàn toàn offline, đảm bảo không có bất kỳ đặc tả API hay dữ liệu test nào bị gửi ra ngoài:

1. Cài đặt và bật [Ollama](https://ollama.ai):
   ```bash
   ollama pull qwen2.5-coder:7b
   ollama serve
   ```
2. Cấu hình SpecTest trỏ về endpoint cục bộ của Ollama:
   ```bash
   spectest config init \
     --provider ollama \
     --endpoint http://localhost:11434/v1 \
     --model qwen2.5-coder:7b
   ```
   *(Không cần nhập API key).*

---

## Ghi đè bằng Biến Môi trường

Trong các runner CI/CD tự động (GitHub Actions, GitLab CI, Jenkins), thông tin chứng thực có thể được truyền trực tiếp qua biến môi trường mà không cần tạo file `credentials.yaml`:

```bash
# Ghi đè API key chung
export SPECTEST_API_KEY="AIzaSyYourSecretKey"

# Key dự phòng cho từng provider
export GEMINI_API_KEY="your-gemini-key"
export OPENAI_API_KEY="your-openai-key"

# Thư mục config tùy chỉnh
export SPECTEST_CONFIG_DIR="/tmp/.spectest"
```
