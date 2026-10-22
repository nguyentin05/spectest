# SpecTest-AI — Backend API Specification for Web UI

Tài liệu đặc tả các REST API nội bộ do Go backend (`spectest ui`) cung cấp để phía Frontend (React + Vite, TailwindCSS) tích hợp hoàn chỉnh các tính năng:
1. **Đọc & Nạp kịch bản kiểm thử (Plan / Test Suite)**.
2. **Thực thi kiểm thử (Run Engine)** với Server-Sent Events (SSE) realtime.
3. **Đọc kết quả kiểm thử & Báo cáo chẩn đoán AI (Report & AI Triage)**.

---

## Thông tin chung

* **Base URL:** `http://127.0.0.1:8080` (hoặc proxy qua Vite `/api`)
* **Content-Type:** `application/json` (trừ endpoint stream dùng `text/event-stream` và upload dùng `multipart/form-data`)
* **Xác thực:** Không yêu cầu (chạy an toàn trên mạng localhost của người dùng).
* **CORS:** Đã kích hoạt cho `http://localhost:5173` (Vite dev server).

---

## MỤC LỤC ENDPOINTS

| Nhóm tính năng | Method | Endpoint | Mô tả |
| :--- | :--- | :--- | :--- |
| **Hạ tầng & Health** | `POST` | `/api/ping-target` | Kiểm tra kết nối đến Backend API mục tiêu |
| **Quản lý Plan** | `GET` | `/api/plan` | Lấy kịch bản kiểm thử mặc định từ thư mục hiện tại |
| | `POST` | `/api/plan/upload` | Tải lên file `.yaml` mới (kéo-thả hoặc browse file) |
| **Thực thi Run** | `POST` | `/api/run` | Kích hoạt chạy kiểm thử với Target URL |
| | `GET` | `/api/run/stream` | Nhận tiến trình chạy theo thời gian thực (SSE) |
| | `POST` | `/api/run/cancel` | Hủy đợt chạy kiểm thử đang diễn ra |
| **Báo cáo & AI Triage** | `GET` | `/api/report/latest` | Lấy dữ liệu báo cáo lần chạy gần nhất |

---

## 1. NHÓM QUẢN LÝ KỊCH BẢN (PLAN / TEST SUITE)

### 1.1. Lấy kịch bản mặc định (`GET /api/plan`)
Tự động tìm và đọc file `spectest.yaml` hoặc `tests/spectest.yaml` tại thư mục đang gõ lệnh.

* **Request:**
  ```http
  GET /api/plan HTTP/1.1
  Host: 127.0.0.1:8080
  ```

* **Response Success (200 OK):**
  ```json
  {
    "success": true,
    "file_path": "tests/spectest.yaml",
    "data": {
      "version": "1.0",
      "metadata": {
        "name": "E-Commerce Checkout & Order Test Suite",
        "target_base_url": "http://localhost:8080",
        "description": "Kịch bản kiểm thử luồng đặt hàng và áp mã giảm giá"
      },
      "scenarios": [
        {
          "id": "SCENARIO_CHECKOUT_FLOW",
          "name": "Luồng đặt hàng và áp mã giảm giá hoàn chỉnh",
          "steps": [
            {
              "id": "STEP_01_LOGIN",
              "name": "Đăng nhập lấy Auth Token",
              "request": {
                "method": "POST",
                "endpoint": "/api/v1/auth/login",
                "headers": {
                  "Content-Type": "application/json"
                },
                "body": {
                  "email": "tester@myshop.vn"
                }
              },
              "extract": {
                "user_token": "$.data.access_token"
              },
              "assert": {
                "status": 200,
                "forbid_keys": ["password", "salt"]
              }
            },
            {
              "id": "STEP_02_CHECKOUT",
              "name": "Tạo đơn hàng với bộ dữ liệu kiểm thử lai",
              "request": {
                "method": "POST",
                "endpoint": "/api/v1/orders/checkout",
                "headers": {
                  "Authorization": "Bearer {{user_token}}"
                }
              },
              "test_cases": [
                {
                  "id": "TC_HAPPY_PATH",
                  "name": "Đơn hàng 600k hợp lệ đủ điều kiện giảm giá",
                  "type": "ai_business_valid",
                  "payload": {
                    "voucher_code": "SALE50",
                    "total_amount": 600000
                  },
                  "assert": {
                    "status": 201,
                    "jsonpath": ["$.data.status == 'CREATED'"]
                  }
                },
                {
                  "id": "TC_GO_SQL_INJECTION",
                  "name": "[Go Security] Payload SQL Injection tại voucher",
                  "type": "go_security",
                  "payload": {
                    "voucher_code": "' OR '1'='1"
                  },
                  "assert": {
                    "status": 400
                  }
                }
              ]
            }
          ]
        }
      ],
      "updates_count": 0
    }
  }
  ```

* **Response Khi chưa có file (404 Not Found):**
  ```json
  {
    "success": false,
    "error": "PLAN_NOT_FOUND",
    "message": "Không tìm thấy file kịch bản spectest.yaml trong thư mục làm việc hiện tại."
  }
  ```

---

### 1.2. Nạp file kịch bản mới (`POST /api/plan/upload`)
Dành cho người dùng kéo thả hoặc bấm nút duyệt file `.yaml` từ máy tính.

* **Request:**
  ```http
  POST /api/plan/upload HTTP/1.1
  Content-Type: multipart/form-data; boundary=----WebKitFormBoundary

  ------WebKitFormBoundary
  Content-Disposition: form-data; name="file"; filename="custom_suite.yaml"
  Content-Type: application/x-yaml

  <Nội dung file YAML thô>
  ------WebKitFormBoundary--
  ```

* **Response Success (200 OK):**
  Trả về cấu trúc dữ liệu đã parse thành công (giống hệt `GET /api/plan`).

* **Response Lỗi cú pháp YAML (400 Bad Request):**
  ```json
  {
    "success": false,
    "error": "INVALID_YAML_SCHEMA",
    "message": "File YAML không đúng cấu trúc kịch bản spectest: thiếu trường 'base.scenarios'."
  }
  ```

---

## 2. NHÓM THỰC THI KIỂM THỬ (RUN ENGINE)

### 2.1. Kiểm tra Target Server Online (`POST /api/ping-target`)
Gọi trước khi bấm Run để hiển thị chấm xanh (🟢 Online) hoặc đỏ (🔴 Unreachable).

* **Request:**
  ```json
  POST /api/ping-target
  Content-Type: application/json

  {
    "target_url": "http://localhost:8080"
  }
  ```

* **Response Success (200 OK):**
  ```json
  {
    "success": true,
    "target_url": "http://localhost:8080",
    "is_alive": true,
    "latency_ms": 12,
    "status_code": 200
  }
  ```

* **Response Offline (200 OK với is_alive = false):**
  ```json
  {
    "success": true,
    "target_url": "http://localhost:8080",
    "is_alive": false,
    "error_message": "connection refused: target server is not running on port 8080"
  }
  ```

---

### 2.2. Kích hoạt chạy kiểm thử (`POST /api/run`)
Bắt đầu một phiên kiểm thử mới.

* **Request:**
  ```json
  POST /api/run
  Content-Type: application/json

  {
    "target_url": "http://localhost:8080",
    "file_path": "tests/spectest.yaml"
  }
  ```

* **Response (202 Accepted):**
  ```json
  {
    "success": true,
    "run_id": "run_20261022_213000_a1b2",
    "message": "Đã khởi tạo phiên kiểm thử thành công. Hãy kết nối SSE /api/run/stream để nhận tiến trình.",
    "stream_url": "/api/run/stream?run_id=run_20261022_213000_a1b2"
  }
  ```

---

### 2.3. Lắng nghe tiến trình Real-time qua SSE (`GET /api/run/stream`)
Frontend mở kết nối `EventSource`:
```javascript
const sse = new EventSource("/api/run/stream?run_id=run_20261022_213000_a1b2");
```

Backend sẽ bắn tuần tự các sự kiện (`event`) dưới dạng JSON:

#### Sự kiện 1: Bắt đầu chạy (`event: run_started`)
```json
data: {
  "run_id": "run_20261022_213000_a1b2",
  "total_steps": 5,
  "start_time": "2026-10-22T21:30:00Z"
}
```

#### Sự kiện 2: Tiến trình từng Test Step/Case hoàn tất (`event: step_finished`)
```json
data: {
  "scenario_id": "SCENARIO_CHECKOUT_FLOW",
  "step_id": "STEP_01_LOGIN",
  "test_case_id": "TC_DEFAULT",
  "step_name": "Đăng nhập lấy Auth Token",
  "status": "PASSED",
  "http_status": 200,
  "latency_ms": 42,
  "current_index": 1,
  "total_steps": 5,
  "progress_percentage": 20
}
```

#### Sự kiện 3: Khi có Test Step bị FAIL (`event: step_finished`)
Khi kiểm chứng thất bại, backend tự động kèm phân tích **AI Defect Triage**:
```json
data: {
  "scenario_id": "SCENARIO_CHECKOUT_FLOW",
  "step_id": "STEP_02_CHECKOUT",
  "test_case_id": "TC_PRD_BOUNDARY",
  "step_name": "Đơn hàng 499k sát biên",
  "status": "FAILED",
  "http_status": 500,
  "latency_ms": 88,
  "current_index": 3,
  "total_steps": 5,
  "progress_percentage": 60,
  "failure_detail": {
    "tier_failed": "Tier 1 (HTTP Status)",
    "expected": "400 Bad Request",
    "actual": "500 Internal Server Error",
    "request_snapshot": {
      "method": "POST",
      "url": "http://localhost:8080/api/v1/orders/checkout",
      "headers": { "Authorization": "Bearer eyJhbGciOi..." },
      "body": { "voucher_code": "SALE50", "total_amount": 499999 }
    },
    "response_snapshot": {
      "status": 500,
      "body": { "error": "panic: runtime error: integer divide by zero in discount calculator" }
    },
    "ai_triage": {
      "root_cause": "Backend bị panic chia cho 0 khi tính toán tỷ lệ giảm giá cho đơn hàng dưới biên tối thiểu 500k.",
      "file_hint": "internal/service/order.go:45",
      "suggested_fix": "Bổ sung kiểm tra: if totalAmount < 500000 { return ErrMinOrderNotMet } trước khi tính chiết khấu."
    }
  }
}
```

#### Sự kiện 4: Hoàn thành toàn bộ kịch bản (`event: run_completed`)
```json
data: {
  "run_id": "run_20261022_213000_a1b2",
  "summary": {
    "total": 5,
    "passed": 4,
    "failed": 1,
    "skipped": 0,
    "pass_rate": 80.0,
    "total_duration_ms": 350
  },
  "report_url": "/api/report/latest"
}
```

---

### 2.4. Hủy đợt chạy đang diễn ra (`POST /api/run/cancel`)
* **Request:**
  ```json
  POST /api/run/cancel
  Content-Type: application/json

  {
    "run_id": "run_20261022_213000_a1b2"
  }
  ```
* **Response (200 OK):**
  ```json
  { "success": true, "message": "Đã dừng tiến trình kiểm thử." }
  ```

---

## 3. NHÓM BÁO CÁO KẾT QUẢ & AI DEFECT HUB (REPORT)

### 3.1. Lấy kết quả lần chạy gần nhất (`GET /api/report/latest`)
Đọc từ file lưu trữ kết quả kiểm thử gần nhất (`reports/run_latest.json`).

* **Request:**
  ```http
  GET /api/report/latest HTTP/1.1
  Host: 127.0.0.1:8080
  ```

* **Response Success (200 OK):**
  ```json
  {
    "success": true,
    "run_id": "run_20261022_213000_a1b2",
    "timestamp": "2026-10-22T21:30:01Z",
    "target_url": "http://localhost:8080",
    "summary": {
      "total": 5,
      "passed": 4,
      "failed": 1,
      "pass_rate": 80.0,
      "duration_ms": 350,
      "avg_latency_ms": 52.4
    },
    "results": [
      {
        "id": "STEP_01_LOGIN",
        "name": "Đăng nhập lấy Auth Token",
        "status": "PASSED",
        "latency_ms": 42,
        "http_status": 200
      },
      {
        "id": "STEP_02_CHECKOUT__TC_PRD_BOUNDARY",
        "name": "Đơn hàng 499k sát biên",
        "status": "FAILED",
        "latency_ms": 88,
        "http_status": 500,
        "failure": {
          "expected": "Status 400",
          "actual": "Status 500",
          "ai_triage": {
            "root_cause": "Backend bị panic chia cho 0 khi tính toán tỷ lệ giảm giá.",
            "file_hint": "internal/service/order.go:45",
            "suggested_fix": "Thêm kiểm tra totalAmount < 500000."
          }
        }
      }
    ]
  }
  ```

---

## Gợi ý cho Lập trình viên Frontend (React Component Flow)

1. **Khi mở trang (Mount):**
   * Gọi song song: `GET /api/plan` và `GET /api/report/latest`.
   * Nếu có Plan $\rightarrow$ Render cây Scenario/Step. Lấy `target_base_url` điền vào ô nhập Target URL.
   * Nếu có Report cũ $\rightarrow$ Vẽ sẵn biểu đồ Pass/Fail ở góc phải.
2. **Khi bấm nút "🚀 Run Test Suite":**
   * Bước 1: Gọi `POST /api/ping-target` xem server backend có online không.
   * Bước 2: Gọi `POST /api/run` lấy `run_id`.
   * Bước 3: Mở kết nối `new EventSource("/api/run/stream?run_id=" + run_id)`.
   * Bước 4: Mỗi khi nhận event `step_finished`:
     * Đổi màu step tương ứng (Xanh lá nếu `PASSED`, Đỏ nếu `FAILED`).
     * Cập nhật thanh `progress_percentage`.
   * Bước 5: Khi nhận `run_completed`: Đóng SSE (`sse.close()`), hiển thị Modal/Card tổng kết kèm hộp thoại **AI Defect Triage** nếu có lỗi.
