# `spectest.log.yaml` History Specification

`spectest.log.yaml` is the append-only audit trail and revision log for SpecTest. It captures the initial generation baseline, records all subsequent incremental updates, and maintains cryptographic suite hashes to guarantee audit integrity.

---

## Annotated Example

```yaml
base:
  at: "2026-10-26T00:00:00+07:00"
  by: ai
  spectest:
    version: 1
    input: openapi
    servers: [shop]
    apis: ["POST {shop}/api/login", "GET {shop}/api/products", "POST {shop}/api/orders"]
    analysis: |
      The suite authenticates first, then exercises product listing and order creation.
    seed:
      credentials:
        vars:
          email: demo@example.com
          password: demo-password
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
    scenarios:
      - id: list-products
        name: List products as an authenticated user
        setup: [authenticated_user]
        steps:
          - call: "GET {shop}/api/products"
            headers:
              Authorization: "Bearer {{access_token}}"
            expect: 200
  after_hash: 14514a8a7144

update:
  - n: 1
    at: "2026-10-26T00:05:00+07:00"
    by: ai
    note: "Added order creation scenario following OpenAPI spec update"
    trigger:
      openapi:
        added:
          - "POST /api/orders"
    impact:
      scenarios:
        - create-order
    change:
      - target: scenario
        id: create-order
        reason: "New POST /api/orders endpoint detected in spec"
        before: null
        after:
          id: create-order
          name: Create an order
    after_hash: 3b9e4a11f82c
```

---

## Schema Reference

### Root Structure

| Field | Type | Description |
| :--- | :--- | :--- |
| `base` | BaseEntry | The initial baseline snapshot of the test suite upon first generation. |
| `update` | array[UpdateEntry] | Sequential, append-only log of subsequent changes. |

---

### `base` Object

| Field | Type | Description |
| :--- | :--- | :--- |
| `at` | string | ISO 8601 timestamp of initial generation. |
| `by` | string | Author of the baseline: `"ai"` or `"user"`. |
| `spectest` | Suite | Snapshot of the initial `spectest.yaml` suite object. |
| `after_hash` | string | The 12-character SHA-256 canonical suite hash of the baseline suite. |
| `api_catalog` | object | Snapshot of OpenAPI routes, operation IDs, and methods at baseline. |

---

### `update[]` Entry Object

Every incremental revision adds a new entry to the `update` array:

| Field | Type | Description |
| :--- | :--- | :--- |
| `n` | integer | Revision sequence number. Starts at `1` and increments strictly without gaps (`1, 2, 3...`). |
| `at` | string | ISO 8601 timestamp when the update was recorded. |
| `by` | string | Update author: `"ai"` (automated drift patch) or `"user"` (manual edit). |
| `note` | string | Human-readable explanation of the change. |
| `trigger` | object | The source changes that provoked this update. |
| `impact` | object | Lists scenarios and setups impacted by this change (used by `--only-changed`). |
| `create` | object | New entities added by this entry (`seed`, `setup`, `scenarios`). |
| `change` | array[Change]| Field-level mutations (`target`, `id`, `reason`, `before`, `after`). |
| `after_hash` | string | 12-character SHA-256 canonical hash of `spectest.yaml` after applying this update. |

---

### `trigger` Object

| Field | Type | Description |
| :--- | :--- | :--- |
| `openapi.added` | array[string] | Endpoints added to the OpenAPI contract since the last revision. |
| `openapi.changed` | array[string] | Endpoints whose schemas, parameters, or responses changed. |
| `openapi.removed` | array[string] | Endpoints removed from the specification. |
| `openapi.restored`| array[string] | Endpoints previously removed that were restored. |
| `requirements.before_hash` | string | Hash of requirement document before this change. |
| `requirements.after_hash` | string | Hash of requirement document after this change. |

---

### `change[]` Object

Tracks granular mutations to suite elements:

```yaml
change:
  - target: scenario        # Target type: analysis, seed, setup, or scenario
    id: create-order        # Identifier of the affected entity
    reason: "API added"     # Technical justification
    before: null            # Value prior to change (null if newly created)
    after:                  # Value after change (null if deleted)
      id: create-order
```

---

## Canonical Suite Hashing Algorithm

SpecTest computes the `after_hash` deterministically:
1. Extracts the four mutable blocks from `spectest.yaml`:
   - `analysis`
   - `seed`
   - `setup`
   - `scenarios`
2. Marshals the blocks using canonical YAML ordering (empty blocks omitted, sorted keys).
3. Computes the SHA-256 digest.
4. Truncates to the **first 12 hexadecimal characters**.

### Why Hash Verification Matters:
When `spectest run --only-changed` is executed:
- The runner computes `currentHash := domain.SuiteHash(suite)`.
- If `currentHash != latest.AfterHash`, the runner halts immediately.
- This prevents running partial test updates against an edited or out-of-sync test suite.
