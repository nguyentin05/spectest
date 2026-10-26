# Hướng dẫn Đóng góp

Cảm ơn bạn đã quan tâm đóng góp cho **SpecTest**! Chúng tôi hoan nghênh mọi sự đóng góp từ lập trình viên, QA engineer, chuyên gia bảo mật và người viết tài liệu kỹ thuật.

---

## Thiết lập Môi trường Phát triển

### Yêu cầu Tiên quyết
- **Go**: Phiên bản `1.22+` (khuyến nghị 1.25).
- **Git**: Phiên bản `2.30+`.
- **Docker & Docker Compose**: Dùng cho kiểm thử tích hợp và kiểm thử vòng đời container.
- **Node.js & npm** (tùy chọn): Dùng khi build giao diện Web UI (`cd web && npm install`).
- **Python 3.10+ & MkDocs** (tùy chọn): Dùng khi chạy preview tài liệu (`pip install -r requirements-docs.txt`).

### Fork và Clone Repository
```bash
git clone https://github.com/<tai-khoan-cua-ban>/spectest.git
cd spectest
git remote add upstream https://github.com/nguyentin05/spectest.git
```

---

## Các Tác vụ Phát triển Thường gặp

### 1. Biên dịch Binary
Biên dịch file thực thi `spectest` vào thư mục `./bin/`:
```bash
make build
./bin/spectest --help
```

### 2. Chạy Kiểm thử (Tests)
Chạy unit tests và use case tests:
```bash
make test
# hoặc chạy kèm race detector:
go test -race ./...
```

### 3. Kiểm tra Mã nguồn Tĩnh (Linter)
```bash
make lint
```

### 4. Build Giao diện Web UI Frontend
```bash
cd web
npm install
npm run build
cd ..
```

### 5. Xem trước Tài liệu MkDocs trên Máy
```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements-docs.txt
mkdocs serve
```
Mở địa chỉ `http://127.0.0.1:8000` trên trình duyệt.

---

## Quy tắc & Ranh giới Kiến trúc

- **Ranh giới Clean Architecture**: Mã nguồn trong `internal/domain` bắt buộc phải có **0 dependency bên ngoài** và không được import bất kỳ thư viện parse YAML hay HTTP client nào.
- **Một Nguồn Chân lý Duy nhất**: Cả Web UI và CLI đều phải gọi chung một use case engine (`internal/usecase/run`, `internal/usecase/generate`). Tuyệt đối không duplicate logic nghiệp vụ trong các transport adapter.
- **Mô hình Đúng 2 File**: Hoạt động của SpecTest xoay quanh `spectest.yaml` và `spectest.log.yaml`. Không tạo thêm các file tracking cũ.
- **Bảo mật Bí mật**: Không bao giờ ghi log, serialize hay gửi các API key, password hoặc bearer token chưa che giấu vào báo cáo hoặc prompt của LLM.
