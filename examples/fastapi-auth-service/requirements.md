# Product Requirements Document (PRD): Auth Service

## 1. Overview & Objectives

The **Auth Service** is a lightweight, stateless microservice providing authentication and identity capabilities for web and mobile clients. It handles user account registration, credential-based authentication via JSON Web Tokens (JWT), and authenticated profile lookups.

### Key Goals
- Secure account registration with input sanitization and duplicate prevention.
- Stateless, cryptographically signed token-based session management (JWT HS256).
- Standardized RESTful JSON endpoints following OpenAPI 3.1 specifications.
- Deterministic error responses for API contract testing and client-side integration.

---

## 2. System Architecture & Context

- **Service Alias**: `auth`
- **Base Path**: `/api/v1`
- **Specification Document**: `openapi.yaml`
- **Data Persistence**: Relational storage (SQLite / PostgreSQL) with `users` table.
- **Security Standard**: Passwords hashed with bcrypt; authorization header via `Bearer <JWT>`.

---

## 3. Functional Requirements & User Journeys

### Journey 1: New User Registration (`POST /api/v1/register`)
As a new client, I want to create an account by providing my email, full name, and a secure password.

- **Request Body**:
  - `email` (string, required): Valid email address (5 to 255 chars).
  - `full_name` (string, required): User display name (1 to 100 chars).
  - `password` (string, required): Raw password (6 to 64 chars).
- **Business Rules**:
  - Emails must be normalized to lowercase before database query and insertion.
  - If the email already exists in the system, reject immediately with **HTTP 409 Conflict** (`"Email đã được đăng ký"`).
  - Passwords must be hashed using bcrypt before persistence. Never store or log raw passwords.
- **Success Response**:
  - Status: **HTTP 201 Created**
  - Payload: User object containing `id`, `email`, `full_name`, and ISO-8601 `created_at`.
  - The response must NEVER include `hashed_password` or the raw password.
- **Validation Failure**:
  - Malformed email, missing fields, or password length < 6 chars must return **HTTP 422 Unprocessable Entity**.

### Journey 2: User Login & Token Issuance (`POST /api/v1/login`)
As a registered user, I want to authenticate using my credentials to receive an access token.

- **Request Body**:
  - `email` (string, required): Registered email.
  - `password` (string, required): User password.
- **Business Rules**:
  - Query user by case-insensitive email.
  - Verify password hash against the stored bcrypt hash.
  - If user is not found OR password does not match, return **HTTP 401 Unauthorized** with message `"Email hoặc mật khẩu không đúng"`. Avoid leaking whether the email exists.
  - On verification success, generate a signed JWT with subject claim `sub: "<user_id>"`.
- **Success Response**:
  - Status: **HTTP 200 OK**
  - Payload:
    ```json
    {
      "access_token": "<jwt-token-string>",
      "token_type": "bearer"
    }
    ```
- **Validation Failure**:
  - Missing email or password returns **HTTP 422 Unprocessable Entity**.

### Journey 3: Profile Retrieval (`GET /api/v1/me`)
As an authenticated user, I want to fetch my current profile details using my Bearer token.

- **Request Headers**:
  - `Authorization: Bearer <access_token>` (required)
- **Business Rules**:
  - Validate JWT signature, algorithm (`HS256`), and expiration.
  - Extract `sub` (User ID) from payload.
  - Retrieve user record from database.
  - If header is missing, token is malformed, expired, or user does not exist, reject with **HTTP 401 Unauthorized**.
- **Success Response**:
  - Status: **HTTP 200 OK**
  - Payload: User object (`id`, `email`, `full_name`, `created_at`).

### Journey 4: Service Health Check (`GET /health`)
As an orchestrator (Docker Compose, Kubernetes, Load Balancer), I want to poll the service health status without authentication.

- **Request**: Unauthenticated `GET /health`
- **Success Response**:
  - Status: **HTTP 200 OK**
  - Payload: `{"status": "ok"}`

---

## 4. Input Validation & Data Constraints

| Field | Type | Min | Max | Format / Pattern | Rules |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `email` | string | 5 | 255 | `^[^@\s]+@[^@\s]+\.[^@\s]+$` | Lowercased, Unique |
| `full_name` | string | 1 | 100 | Text | Non-empty |
| `password` | string | 6 | 64 | Plaintext | Salted & hashed via bcrypt |
| `Authorization` | header | - | - | `Bearer <JWT>` | Required for `/api/v1/me` |

---

## 5. Test Scenarios & Expectations

- **User Registration**:
  - Register new account with dynamic unique email (`test_{{random}}@example.com`) -> Expect **HTTP 201 Created**.
  - Duplicate registration with the same email -> Expect **HTTP 409 Conflict**.
  - Invalid registration (short password < 6 chars, invalid email format) -> Expect **HTTP 422 Unprocessable Entity**.
- **User Authentication**:
  - Login with registered email and password -> Expect **HTTP 200 OK** and receive `access_token`.
  - Login with wrong password or non-existent email -> Expect **HTTP 401 Unauthorized**.
  - Login with missing fields -> Expect **HTTP 422 Unprocessable Entity**.
- **User Profile**:
  - Get profile `/api/v1/me` with valid `Authorization: Bearer <token>` header -> Expect **HTTP 200 OK**.
  - Get profile `/api/v1/me` without token or with invalid header -> Expect **HTTP 401 Unauthorized**.
- **System Health**:
  - Public check `/health` -> Expect **HTTP 200 OK**.
