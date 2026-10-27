# Generate: Creating and Updating Test Suites

**Objective:** Generate initial test suites (`init`) or incrementally sync existing suites with OpenAPI spec changes (`update`), preserving user customizations and cryptographic integrity.

---

## 1. Dual-Path Execution Routing

Check if SpecTest MCP tools are connected in your environment:
- **MCP Connected:** Use `inspect_suite`, `plan_update`, and `start_update`.
- **MCP Unavailable:** Use `spectest generate` CLI directly.

---

## 2. Pre-Generation Verification

1. Read `./.spectest/config.yaml` (or `./tests/config.yaml`) to verify registered server aliases and OpenAPI paths.
2. Confirm user intent: Do they want to **initialize a new suite (`init`)** or **incrementally update an existing suite (`update`)**? Never assume `update` just because suite files already exist.
3. Always supply `--output` (or `output_dir` in MCP).
4. Only supply `--req` (or `requirements_path`) if a verified Markdown business requirements document actually exists. **Never create fake requirements.**
5. Run the preflight check before proceeding:
   ```bash
   scripts/preflight.sh <init|update> [workspace_dir]
   ```

---

## 3. Init Mode (New Suite Generation)

Use `init` to bootstrap a test suite from scratch.

### MCP Path
1. Call `inspect_suite` to check if a suite already exists in the target directory:
   ```json
   { "suite_path": ".spectest/spectest.yaml" }
   ```
2. If no suite exists, execute generation via CLI or MCP workflow.

### CLI Path
```bash
spectest generate \
  --mode init \
  --output ./.spectest/ \
  --api auth=./path/to/auth-openapi.yaml
```

For multi-service architectures or when business requirements are available:
```bash
spectest generate \
  --mode init \
  --output ./.spectest/ \
  --api auth=./path/to/auth-openapi.yaml \
  --api orders=./path/to/orders-openapi.yaml \
  --req ./path/to/requirements.md
```

- **Server Ordering:** The order of `--api` flags dictates the canonical ordering of servers in `spectest.yaml`.
- **Overwrite Safety:** If `spectest.yaml` or `spectest.log.yaml` already exists, the CLI prompts interactively for confirmation. Allow the CLI to prompt directly. Never manually delete existing files or pipe automated "yes" answers without explicit user consent.
- **Dry Run:** Use `--dry-run` to execute the full generation pipeline and preview output without writing any files to disk.

---

## 4. Update Mode (Incremental Spec-Drift Sync)

Use `update` when the API contracts have evolved and the existing test suite must be synchronized without losing existing scenario states.

### Mandatory Pre-Conditions
Both files must exist:
- `spectest.yaml` (Active test suite)
- `spectest.log.yaml` (Audit log catalog)

**If either file is missing, stop immediately.** Inform the user that an incremental update cannot be performed without audit history, and ask if they wish to run `init`.

### MCP Path (Recommended for Agents)
1. **Inspect Integrity:** Call `inspect_suite`:
   ```json
   { "suite_path": ".spectest/spectest.yaml" }
   ```
   Verify that `integrity_match == true`. If there is a hash mismatch (`integrity_conflict`), **STOP**.
2. **Preview Diff:** Call `plan_update` to deterministically calculate impact:
   ```json
   {
     "output_dir": ".spectest",
     "apis": [
       {"server": "auth", "path": "./path/to/auth-openapi.yaml"}
     ]
   }
   ```
   Inspect the diff returned:
   - `added`: Newly added API endpoints to generate tests for.
   - `removed`: Deprecated API endpoints to safely mark as skipped.
   - `changed`: Endpoints with modified schemas/parameters to repair.
   - `restored`: Previously removed endpoints to restore.
3. **Execute Update:** Call `start_update`:
   ```json
   {
     "output_dir": ".spectest",
     "apis": [
       {"server": "auth", "path": "./path/to/auth-openapi.yaml"}
     ]
   }
   ```
   Poll `get_job` with the returned `job_id` until `status == "succeeded"`.

### CLI Path
```bash
spectest generate \
  --mode update \
  --output ./.spectest/ \
  --api auth=./path/to/auth-openapi.yaml
```

Include all relevant `--api` mappings and `--req` if updated business requirements apply.

---

## 5. Strict Update Invariants

1. **Cryptographic Log Immutability:** Never manually modify `spectest.log.yaml`. Hand-editing breaks SHA-256 hash validation and invalidates history.
2. **Preserve User Skips:** Custom scenario skip reasons (e.g. `skip: "Waiting for feature flag"`) must **never** be removed or altered during update. Only system skips (`skip: "spectest: API ... removed"`) may be updated.
3. **Locked Scenarios:** Any scenario marked `locked: true` is strictly immutable. SpecTest will never alter locked scenarios.
4. **No Hallucinated Assertions:** Never invent response fields, error codes, or business constraints not defined in the OpenAPI or requirements document.

---

## 6. Post-Generation Verification

- Verify that both `spectest.yaml` and `spectest.log.yaml` exist in the target directory.
- Open and read the updated suite content before declaring completion. Do not assume success based solely on process exit code.
- Report the outcome to the user: total scenarios generated, newly added, updated, or skipped.
