# SpecTest CLI Command Reference

## `spectest generate`
Generate or update a test suite.

```bash
# Initial generation
spectest generate --api <alias>=<path-to-openapi> [--req <requirements.md>] [--out <output-dir>]

# Incremental update preview (dry run)
spectest generate --mode update --file <spectest.yaml> --api <alias>=<new-openapi> --dry-run

# Execute incremental update
spectest generate --mode update --file <spectest.yaml> --api <alias>=<new-openapi>
```

## `spectest run`
Execute an API test suite.

```bash
# Run full suite against target
spectest run --file tests/spectest.yaml --target <alias>=<url> [--compose <docker-compose.yml>]

# Run ONLY scenarios affected by the latest update
spectest run --file tests/spectest.yaml --target <alias>=<url> --only-changed

# Enable LLM triage on step failures
spectest run --file tests/spectest.yaml --target <alias>=<url> --triage
```

## `spectest mcp serve`
Start the local stdio MCP server.

```bash
spectest mcp serve [--root <workspace-dir>]
```
