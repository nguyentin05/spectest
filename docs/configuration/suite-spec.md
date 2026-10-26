# `spectest.yaml` Suite Specification

`spectest.yaml` is the core test contract in SpecTest (Schema Version `1`). It specifies registered server aliases, covered API operations, business analysis, global seeds, reusable setup blocks, and executable test scenarios.

---

## Annotated Example

```yaml
version: 1
input: openapi
servers: [shop]
apis: 
  - "POST {shop}/api/login"
  - "GET {shop}/api/products"
  - "POST {shop}/api/orders"

analysis: |
  The suite authenticates first, extracts the access token, lists products,
  and exercises order creation with verified database state.

# Global seed variables available to all setups and scenarios
seed:
  credentials:
    vars:
      email: demo@example.com
      password: demo-password

# Reusable setup blocks executed via DAG dependency resolution
setup:
  authenticated_user:
    steps:
      - call: "POST {shop}/api/login"
        body:
          email: "{{email}}"
          password: "{{password}}"
        expect: 200
        save:
          access_token: body.access_token

  product:
    use: [authenticated_user]
    steps:
      - call: "GET {shop}/api/products"
        headers:
          Authorization: "Bearer {{access_token}}"
        expect: 200
        save:
          product_id: body.items.0.id

# Executable business scenarios
scenarios:
  - id: list-products
    name: List products as an authenticated user
    setup: [authenticated_user]
    steps:
      - call: "GET {shop}/api/products"
        headers:
          Authorization: "Bearer {{access_token}}"
        expect: 200

  - id: create-order
    name: Create an order using an extracted product ID
    setup: [product]
    steps:
      - call: "POST {shop}/api/orders"
        headers:
          Authorization: "Bearer {{access_token}}"
        body:
          product_id: "{{product_id}}"
          quantity: 1
        expect: 201
        save:
          order_id: body.id
```

---

## Schema Reference

### Root Suite Fields

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `version` | integer | **Yes** | Schema specification version. Must be `1`. |
| `input` | string | **Yes** | Input mode that generated this suite: `openapi` or `requirements`. |
| `servers` | array[string] | **Yes** | List of server aliases referenced in `call` expressions (e.g. `[shop, billing]`). |
| `apis` | array[string] | **Yes** | Canonical list of API operations covered by the suite (e.g. `"POST {shop}/api/login"`). |
| `analysis` | string | No | Markdown documentation describing the testing strategy and rationale. (Mutable) |
| `seed` | map[string]Seed | No | Named groups of default variables shared across all scenarios. (Mutable) |
| `setup` | map[string]Setup| No | Reusable pre-execution blocks that prepare dependencies. (Mutable) |
| `scenarios`| array[Scenario]| **Yes** | Ordered list of business scenarios. (Mutable) |

---

### `seed.<group>` Object

Groups of seed variables available globally:

```yaml
seed:
  auth:
    vars:
      tenant_id: "tenant-beta"
      user_id: 1001
```

> [!IMPORTANT]
> **No Collision Rule**: Variable names across different seed groups must be unique. If `seed.group1.vars.token` and `seed.group2.vars.token` both exist, SpecTest rejects the suite with a collision error.

---

### `setup.<name>` Object

Reusable blocks of steps executed before a scenario's steps:

| Field | Type | Description |
| :--- | :--- | :--- |
| `use` | array[string] | List of other setup names that must execute before this setup. (Dependency set, not an explicit order). |
| `vars` | map[string]any | Local variable definitions for this setup block. Supports `{{var}}` interpolation. |
| `steps` | array[Step] | Sequential HTTP steps executed to prepare state. |

---

### `scenarios[]` Object

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `id` | string | **Yes** | Stable, unique identifier in kebab-case (e.g. `create-order`). |
| `name` | string | **Yes** | Human-readable title of the scenario. |
| `setup` | array[string] | No | Direct setup blocks required by this scenario. |
| `steps` | array[Step] | **Yes** | Ordered HTTP operations executed in this scenario. |
| `locked` | boolean | No | If `true`, protects the scenario from automated modification during spec-drift updates. |
| `skip` | string | No | If non-empty, skips this scenario and logs the string as the skip reason. |

---

### `Step` Object

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `call` | string | **Yes** | HTTP operation in the format `"METHOD {server}/path"`. (e.g. `"POST {shop}/api/orders"`). |
| `expect` | integer | **Yes** | Expected HTTP status code (e.g. `200`, `201`, `400`). |
| `headers` | map[string]string | No | Key-value pairs of HTTP request headers. Supports `{{var}}` interpolation. |
| `body` | any | No | JSON request payload. Supports nested objects, arrays, and `{{var}}` interpolation. |
| `save` | map[string]string | No | Variables to extract from the HTTP response into runtime scope. |

---

## Variable Interpolation & Extraction Rules

### 1. Placeholder Syntax
- `{{variable_name}}`: Replaced with the value of `variable_name` from the current scenario scope.
- `{{random}}`: Generates a fresh, unique random hexadecimal token per occurrence.

### 2. Preserved Runtime Types
When a JSON body property contains an exact variable reference without extra text, SpecTest preserves the underlying Go runtime type:

```yaml
body:
  quantity: "{{item_count}}"  # Resolved as integer if item_count is an integer
  is_active: "{{is_active}}"  # Resolved as boolean if is_active is a boolean
```

### 3. Response Extraction (`save`)
- **JSON Body Extraction (`body.<dot-path>`)**:
  - `access_token: body.token` $\rightarrow$ reads `resp.token`.
  - `product_id: body.items.0.id` $\rightarrow$ reads the `id` field from the first item of `items`.
- **HTTP Header Extraction (`header.<name>`)**:
  - `session_id: header.Set-Cookie`
  - `etag: header.ETag`
