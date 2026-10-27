# SpecTest Update Safety & Invariants

## Hash Verification
`spectest.log.yaml` is an append-only audit trail recording the state of the suite across time:
- The suite hash is the first 12 hexadecimal characters of a SHA-256 digest over mutable suite content: `analysis`, `seed`, `setup`, and `scenarios`.
- When `spectest generate --mode update` or MCP `plan_update` runs, it recalculates the current suite hash and compares it with `after_hash` of the latest entry in `spectest.log.yaml`.
- If there is a mismatch (`integrity_conflict`), update aborts immediately to protect against corrupted history.

## Automated DAG Tool Skips
- When an API is removed from OpenAPI, scenarios calling that API are marked with `skip: "spectest: API <method> <path> removed"`.
- Setups calling removed APIs are also marked; any scenario that transitively depends on that setup is also marked skipped.
- When an API is later restored in a future update, SpecTest checks the DAG and un-skips affected scenarios automatically.

## User-Authored Skip Preservation
- If a user author manually wrote a skip reason (e.g. `skip: "pending billing service v2 deploy"`), it does NOT start with `spectest: API`.
- SpecTest strictly preserves all user-authored skips and will never un-skip or overwrite them.
