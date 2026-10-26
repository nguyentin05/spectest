# `spectest config`

Quản lý thông tin chứng thực của các nhà cung cấp LLM, endpoint API, model và các thiết lập bảo mật.

---

## Cú pháp

```bash
spectest config [command]
```

Các lệnh con khả dụng:
- `init`: Khởi tạo hoặc ghi đè toàn bộ cấu hình LLM.
- `list`: Xem cấu hình hiện tại (các khóa bí mật tự động được ẩn).
- `set`: Cập nhật một khóa cấu hình riêng lẻ.

---

## Kiến trúc Bảo mật Thông tin Chứng thực

SpecTest thực thi nghiêm ngặt các nguyên tắc bảo mật thông tin:
- **Quyền File POSIX Nghiêm ngặt**: Cấu hình được lưu tại `~/.spectest/credentials.yaml` với quyền `0600` (`-rw-------`), chỉ cho phép đúng tài khoản người dùng hiện tại được đọc và ghi.
- **Bảo vệ Thư mục Cha**: Thư mục `~/.spectest/` được tạo với quyền `0700` (`drwx------`).
- **Không Lộ Key qua Command Line**: Không cho phép truyền API key qua các cờ dòng lệnh nhằm tránh việc rò rỉ vào danh sách tiến trình (`ps aux`), lịch sử shell (`.bash_history`) hoặc telemetry hệ thống.
- **Ẩn Input trên Terminal**: Khi nhập API key tương tác, terminal sẽ tắt echo (không hiển thị ký tự gõ trên màn hình).

---

## Các Lệnh con

### `spectest config init`

Khởi tạo nhà cung cấp LLM chính và thông tin chứng thực qua tương tác hoặc bằng cờ.

```bash
spectest config init [flags]
```

#### Bảng Cờ Lệnh
| Cờ | Kiểu | Mô tả |
| :--- | :--- | :--- |
| `--provider` | string | Định danh nhà cung cấp LLM (`gemini`, `openai`, `ollama`). |
| `--endpoint` | string | URL endpoint cơ sở của API LLM. |
| `--model` | string | Tên model (ví dụ: `gemini-2.5-flash`, `gpt-4o-mini`, `qwen2.5-coder:7b`). |

#### Ví dụ
```bash
# Thiết lập Google Gemini
spectest config init --provider gemini --model gemini-2.5-flash

# Thiết lập OpenAI
spectest config init --provider openai --endpoint https://api.openai.com/v1 --model gpt-4o-mini

# Thiết lập Ollama Cục bộ (Offline / Không cần internet)
spectest config init --provider ollama --endpoint http://localhost:11434/v1 --model qwen2.5-coder:7b
```

---

### `spectest config list`

Liệt kê tất cả các thông số cấu hình đang kích hoạt. Khóa bí mật sẽ tự động được che.

```bash
spectest config list
```

#### Kết quả Mẫu:
```text
ACTIVE CONFIGURATION:
  Provider:      gemini
  Model:         gemini-2.5-flash
  Endpoint:      https://generativelanguage.googleapis.com/v1beta/openai/
  API Key:       AIzaSy************************3X81
  Storage:       ~/.spectest/credentials.yaml (0600)
```

---

### `spectest config set`

Cập nhật một trường cấu hình riêng lẻ mà không cần phải khởi tạo lại toàn bộ file.

```bash
spectest config set <khoa> [gia_tri]
```

#### Các Khóa Hỗ trợ:
- `provider`: Nhà cung cấp LLM (`gemini`, `openai`, `ollama`).
- `endpoint`: Địa chỉ URL endpoint của API.
- `model`: Định danh model.
- `api-key`: Yêu cầu nhập API key với tính năng ẩn ký tự gõ trên terminal (không truyền giá trị trên câu lệnh).

#### Ví dụ:
```bash
# Đổi model đang dùng
spectest config set model gemini-1.5-pro

# Đổi endpoint cục bộ tùy biến
spectest config set endpoint http://10.0.0.5:8000/v1

# Cập nhật API key an toàn qua terminal ẩn ký tự
spectest config set api-key
# > Enter API Key: ************************
```
