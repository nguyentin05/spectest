# Analyze: Result Inspection and Failure Triage Taxonomy

**Objective:** Inspect test reports, analyze failure root causes, and systematically classify defects into a rigorous 5-tier taxonomy with clear confidence levels.

---

## 1. Data Sources

Always gather raw evidence before forming a diagnosis:
- Console output and process exit code of the `spectest run` command.
- Structured report JSON (`reports/report.json` via filesystem or `read_report` via MCP).
- AI triage diagnostics (only present if `--triage` was explicitly enabled).
- **Mandatory inspection:** You must open and parse the actual report file. Never infer results from filenames or summarized console text alone.
- **Zero token leaks:** Never expose raw API keys, Authorization headers, passwords, or cookies in triage reports.

---

## 2. Five-Tier Failure Taxonomy

Classify every failed scenario into one of the following five categories:

### Category 1: Environment & Infrastructure Failure
- **Symptoms:** Connection refused, port bind conflict, target readiness timeout, database startup failure, missing critical environment variables.
- **Root Cause:** The application under test or its dependency service failed to start or was not reachable at the specified base URL.
- **Confidence:** `[CONFIRMED]` when network or connection errors appear in step results.

### Category 2: Request, Setup & Chaining Failure
- **Symptoms:** HTTP 400/422 Bad Request on setup steps, missing prerequisite token, un-interpolated variable (`{{token}}` remaining literal), invalid setup DAG sequence.
- **Root Cause:** The scenario or its setup dependency failed to extract (`save`) or inject valid runtime values into subsequent calls.
- **Confidence:** `[CONFIRMED]` when request payload violates known schema or setup step fails before scenario steps begin.

### Category 3: Contract & Schema Assertion Failure
- **Symptoms:** HTTP status code mismatch (e.g. expected 201 Created, received 200 OK), missing required response property, unexpected data type in response body.
- **Root Cause:** The application responded successfully, but the payload format contradicts the OpenAPI 3.x contract.
- **Confidence:** `[CONFIRMED]` based on exact assertion diff against OpenAPI specification.

### Category 4: Probable Application Bug
- **Symptoms:** HTTP 500 Internal Server Error, unhandled runtime exception, database constraint violation, incorrect business logic calculation.
- **Root Cause:** The request was valid, setup succeeded, and the service was reachable, but the application crashed or computed an erroneous result.
- **Confidence:** `[PROBABLE]` or `[CONFIRMED]` if error response body contains internal stack trace or confirmed bug signature.

### Category 5: Insufficient Evidence
- **Symptoms:** Incomplete error message, truncated response body, ambiguous failure with multiple plausible root causes.
- **Root Cause:** Cannot determine root cause without additional application logs or raw HTTP trace.
- **Confidence:** `[UNKNOWN / NEEDS INFO]`. Do not speculate without evidence.

> **Caution:** Never label every test failure as an "application bug". Distinguish cleanly between environment misconfiguration, test data chaining issues, specification divergences, and actual application defects.

---

## 3. Reporting Structure

When presenting findings to the user, lead with the high-level conclusion and follow this structure:

```markdown
### SpecTest Failure Diagnosis

**Execution Summary:**
- **Status:** FAILED ({failed_count} of {total_count} scenarios failed)
- **Duration:** {duration_ms}ms
- **Target:** {target_mappings}

**Detailed Findings:**
1. **[{scenario_id}] {scenario_name}**
   - **Step:** `{step_method} {step_path}`
   - **Assertion:** Expected HTTP status `{expected}`, received `{actual}`
   - **Classification:** {Category} — `[{CONFIRMED | PROBABLE | UNKNOWN}]`
   - **Root Cause Analysis:** {Concise technical explanation based on actual response}
   - **Evidence:** `{specific_evidence_without_secrets}`
   - **Recommended Action:** {Prescriptive next step}

**Next Steps:**
- {Actionable, concrete recommendations}
```

---

## 4. Prescriptive Actions by Category

- **Environment / Infrastructure:**
  - Suggest verifying `docker-compose.test.yml`, port mappings, or database readiness probe.
  - Do not modify production configurations.
- **Request / Setup Chaining:**
  - Check the scenario's setup dependency DAG in `spectest.yaml`.
  - Verify that the setup step extracts the required variable (e.g., `save: token: body.access_token`).
- **Contract / Specification:**
  - Identify whether the OpenAPI specification or the application code is out of sync.
  - If the spec changed intentionally, suggest running an incremental update:
    `spectest generate --mode update`
- **Application Bug:**
  - Provide minimal reproduction steps and request/response payloads to the user.
  - **Do not modify application source code** unless explicitly asked by the user.
- **Targeted Re-run:**
  - Once a fix is applied, suggest re-running only the affected scenarios:
    `spectest run -c <compose> -f <suite> -t <target> --only-changed`
- **AI Triage:**
  - If deeper root-cause analysis is desired and LLM is configured, recommend re-running with `--triage`.
