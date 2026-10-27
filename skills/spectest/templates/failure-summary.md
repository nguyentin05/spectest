# SpecTest Failure Summary Template

Use this format when reporting test execution failures to the user:

```markdown
### SpecTest Execution Report

**Execution Overview:**
- **Suite:** {suite_name}
- **Target:** {target_mappings}
- **Results:** Total: {total} | Passed: {passed} | Failed: {failed} | Skipped: {skipped}
- **Duration:** {duration_ms}ms

**Failed Scenarios:**
1. **[{scenario_id}] {scenario_name}**
   - **Failed Step:** `{step_call}` (HTTP status expected {expect}, got {actual})
   - **AI Diagnosis (if enabled):** {ai_diagnosis}
   - **Recommended Fix:** {actionable_suggestion}

**Artifacts:**
- Full report: `{report_path}` (sensitive credentials redacted)
```
