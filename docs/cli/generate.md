# `spectest generate`

Generate a new SpecTest suite or incrementally update an existing suite from OpenAPI documents and optional business requirements.

---

## Synopsis

```bash
# Initialize a new suite
spectest generate --mode init -a <server>=<path-to-openapi> [flags]

# Update an existing suite with OpenAPI contract changes
spectest generate --mode update -a <server>=<path-to-openapi> [flags]
```

---

## Modes of Operation

`spectest generate` supports two primary operational modes via the `--mode` flag:

### 1. Initial Mode (`--mode init`, Default)
Generates a new test suite from scratch:
- Validates OpenAPI documents and optional Markdown requirements.
- Uses prompt engineering to synthesize multi-step business journeys.
- Packages output into `spectest.yaml` and initializes `spectest.log.yaml` with the baseline snapshot and 12-character SHA-256 hash.
- **Overwrite Protection**: If existing suite files are detected in `--output`, prompts for confirmation before overwriting (bypassed in non-interactive terminals with error).

### 2. Update Mode (`--mode update`)
Incrementally synchronizes an existing test suite when API specifications change:
- **Two-File Architecture**: Operates directly on `spectest.yaml` and `spectest.log.yaml`. Never requires legacy tracking files.
- **Fail-Closed Hash Verification**: Computes the SHA-256 hash of the current `spectest.yaml`. If it does not match the latest `after_hash` in `spectest.log.yaml`, halts immediately to prevent corruption.
- **Catalog Reconstruction**: Reconstructs the full API catalog by applying all incremental `catalog_patch` records on top of the log's `base.api_catalog`.
- **Deterministic API Diffing**: Diffs the new OpenAPI contract against the reconstructed catalog to identify added, changed, removed, or restored operations.
- **Impact Graph Resolution**: Maps changed APIs to directly affected scenarios, as well as scenarios that depend on affected `setup` blocks via the setup DAG.
- **Automated Tool Skips on Removed APIs**: If an API is removed from OpenAPI, marks affected scenarios with `skip: "spectest: API <key> removed"` without deleting user tests. Fails closed if a `locked: true` scenario is affected.
- **Restored APIs**: Automatically removes tool-generated `spectest: API` skips when an endpoint returns, while strictly preserving user-written custom skips.
- **Append-Only History Recording**: Appends a new `UpdateEntry` (incrementing `n` without gaps) with triggers, catalog patches, impact lists, and the new suite hash.

---

## Flags & Options

| Flag | Short | Type | Default | Required | Description |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `--mode` | | string | `"init"` | No | Generation mode: `init` (create new suite) or `update` (sync existing suite). |
| `--api` | `-a` | stringArray | `[]` | **Yes** | OpenAPI document specified as `<server>=<path>` (repeatable; order defines server aliases). |
| `--req` | `-r` | string | `""` | No | Path to raw business requirements (markdown). |
| `--output` | `-o` | string | `"./tests"` | No | Directory containing `spectest.yaml` and `spectest.log.yaml`. |
| `--dry-run` | | boolean | `false` | No | Execute the complete generation/update pipeline without writing files to disk. |
| `--verbose` | | boolean | `false` | No | Enable detailed debug logging. |

---

## Examples

### 1. Initialize Single-Server Suite
```bash
spectest generate --mode init -a shop=openapi.yaml -o ./tests
```

### 2. Initialize Multi-Server Suite with Requirements
Specify multiple server aliases in order:
```bash
spectest generate --mode init \
  -a auth=openapi-auth.yaml \
  -a order=openapi-order.yaml \
  -r requirements.md \
  -o ./tests
```

### 3. Dry-Run Generation
Inspect what would be generated without writing to disk:
```bash
spectest generate --mode init -a shop=openapi.yaml --dry-run
```

### 4. Incremental Update on API Contract Change
Update an existing suite after modifying `openapi.yaml`:
```bash
spectest generate --mode update -a shop=openapi.yaml -o ./tests
```

---

## Output Artifacts

- **`spectest.yaml`**: The active test suite (version 1) containing `servers`, `apis`, `analysis`, `seed`, `setup`, and `scenarios`.
- **`spectest.log.yaml`**: Append-only history logging the initial baseline or the new incremental `UpdateEntry` with updated SHA-256 suite hash.
