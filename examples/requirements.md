# Business Requirements Document (PRD) — Order & Voucher Management

## 1. Authentication
* Users must provide valid email and password to receive a Bearer JWT Token.
* The response must not leak sensitive fields such as passwords or password hashes.

## 2. Order Checkout & Voucher Policy
* Voucher `SALE50` offers a 50,000 VND discount for orders with `total_amount` of at least 500,000 VND.
* If `total_amount` is strictly below 500,000 VND (e.g. 499,999 VND), the voucher is invalid and the API must return HTTP 400 with error code `MIN_ORDER_NOT_MET`.
* Non-positive orders (e.g., total_amount <= 0 or empty items) must be rejected with HTTP 400.
