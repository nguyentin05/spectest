# Prepare: Workspace, OpenAPI, Server URLs, and Isolated Compose

**Objective:** Gather and verify all prerequisites required to generate and run SpecTest test suites, while strictly isolating test executions from development and production data.

---

## 1. Project Discovery & Repository Survey

Before making conclusions or proposing commands, thoroughly inspect the repository:
- Read `README.md`, `Dockerfile`, `docker-compose*.yml`, environment templates (`.env.example`), and startup scripts.
- Identify the backend framework (e.g., FastAPI, Go Gin/Fiber, Spring Boot, Express/NestJS, ASP.NET).
- Identify dependencies: databases (Postgres, MySQL, Mongo), caches (Redis), message queues (RabbitMQ, Kafka).
- Identify how the application boots, how it reads configuration, and what ports it exposes.
- **Do not modify development/production configurations** and do not run commands that might purge existing data without confirmed scope.

---

## 2. Server Configuration (`config.yaml`)

Persist confirmed server parameters in `./.spectest/config.yaml` (or `./tests/config.yaml`):

```yaml
servers:
  auth:
    openapi: ./path/to/openapi.yaml
    base_url: http://localhost:8000
  orders:
    openapi: ./path/to/orders.yaml
    base_url: http://localhost:8001
```

- **`alias`** (e.g. `auth`, `orders`): A stable kebab-case or short identifier used consistently in CLI flags (`--api auth=...`, `-t auth=...`) and MCP arguments.
- **`openapi`**: Relative or absolute path to the verified OpenAPI specification YAML.
- **`base_url`**: Runtime endpoint URL when test requests are executed.
- **Never infer base URLs** from alias names or arbitrary `servers` entries in the OpenAPI document. Confirm them via repository configurations, docker-compose ports, or ask the user.
- **Never store secrets, passwords, or bearer tokens** inside `config.yaml`.

---

## 3. OpenAPI Specification Verification

1. Locate existing OpenAPI/Swagger 3.x files in the repository. Verify that paths match active endpoints.
2. If the spec does not exist, check the framework's native documentation export capabilities:
   - **FastAPI**: Extract from `app.openapi()` or dump running `/openapi.json`.
   - **Go (swag/ogen/echo-swagger)**: Locate doc declarations or generation scripts.
   - **Spring Boot / Micronaut**: Check Springdoc OpenAPI endpoints.
   - **NestJS / Express**: Inspect SwaggerModule setup.
3. **Never fabricate fake OpenAPI documents** just to satisfy generation commands. Generating from fabricated contracts produces meaningless test suites that test imaginary APIs.
4. **Do not move or copy source OpenAPI files** without explicit user consent. Always reference original file paths.
5. For each API service, document:
   - A unique server alias.
   - Verified path to the OpenAPI specification.
   - Confirmed runtime base URL.

---

## 4. Test Environment Isolation (Docker Compose)

`spectest run` (and MCP `start_run`) automatically spins up the target Compose environment, waits for health checks, executes scenarios, and purges the environment upon teardown. Volumes may be removed!

1. Prioritize creating a dedicated, self-contained test compose file: `./.spectest/docker-compose.test.yml` (or `./docker-compose.test.yml`).
2. Only reuse environment variables that are confirmed safe and point exclusively to test resources.
3. **Strict Isolation Rules:**
   - **No persistent data mounts:** Never mount host directories containing real or development data (`/var/lib/postgresql/data`, `~/.pgdata`, etc.).
   - **No external resources:** Do not use `external: true` networks or volumes tied to running local development services.
   - **No production credentials:** Never inject production API keys, connection strings, or cloud tokens.
4. If service startup, database seeding, or volume isolation is unclear, stop and ask the user before creating or running Compose.
5. **Never run destructive commands** or boot untested Compose files "just to see what happens".

---

## Readiness Checklist

Transition to the **Generate** phase only when all conditions are satisfied:
- [ ] Valid OpenAPI 3.x YAML confirmed for each target service.
- [ ] Unique alias, specification path, and runtime URL documented for each service.
- [ ] Configuration persisted in `./.spectest/config.yaml` (or `./tests/config.yaml`).
- [ ] Safe, strictly isolated test Compose file prepared.
- [ ] No ambiguity remaining that could route test requests to a live or development database.

Validate preflight conditions:
```bash
scripts/preflight.sh init ./.spectest
```
