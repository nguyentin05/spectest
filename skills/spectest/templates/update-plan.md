# SpecTest Update Plan Summary Template

Use this format when reporting planned API suite changes to the user:

```markdown
### 📋 SpecTest Update Plan

**API Contract Changes:**
- **Added:** {N} endpoint(s)
  - `METHOD {server}/path`
- **Removed:** {N} endpoint(s)
  - `METHOD {server}/path`
- **Changed:** {N} endpoint(s)
  - `METHOD {server}/path` (details: status / request body schema)
- **Restored:** {N} endpoint(s)

**Suite Impact:**
- **Affected Setups:** {count} ({names})
- **Affected Scenarios:** {count} ({ids})

**Planned Actions:**
1. [Add Tool Skip] Mark `{scenario_id}` skipped ({reason})
2. [Repair Setup] Update steps in `{setup_name}`
3. [Synthesize Scenario] Generate test coverage for `{api_key}`

*Proceed with update?*
```
