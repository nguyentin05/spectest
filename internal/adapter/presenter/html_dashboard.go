package presenter

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nguyentin05/spectest/internal/usecase/run"
)

// ExportSummaryJSON writes the raw execution summary as JSON (reports/summary.json)
// so `spectest ui` can read and render the latest test execution and AI triage results.
func ExportSummaryJSON(path string, sum *run.Summary) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create summary json dir: %w", err)
	}
	data, err := json.MarshalIndent(sum, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal summary json: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// ExportHTMLDashboard generates a self-contained, modern dark-mode HTML report (reports/report.html).
func ExportHTMLDashboard(path string, sum *run.Summary) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create html report dir: %w", err)
	}

	passRate := 0.0
	if sum.TotalChecks > 0 {
		passRate = float64(sum.PassedChecks) / float64(sum.TotalChecks) * 100.0
	}

	var rows strings.Builder
	for _, rec := range sum.Records {
		statusBadge := `<span class="badge pass">PASS</span>`
		rowClass := "row-pass"
		detailHTML := `<span class="muted">4-Tier Oracle Verified (0 AI Tokens)</span>`

		if !rec.Passed {
			statusBadge = `<span class="badge fail">FAIL</span>`
			rowClass = "row-fail"
			detailHTML = fmt.Sprintf(
				`<div class="fail-reason"><strong>%s:</strong> %s</div>`,
				html.EscapeString(rec.FailedTier),
				html.EscapeString(rec.FailReason),
			)
			if rec.AIDiagnosis != "" {
				detailHTML += fmt.Sprintf(
					`<div class="ai-box"><strong>🩺 AI Root-Cause Triage:</strong><br/>%s</div>`,
					html.EscapeString(rec.AIDiagnosis),
				)
			}
		}

		rows.WriteString(fmt.Sprintf(`
        <tr class="%s">
          <td>%s</td>
          <td><code>%s</code><br/><small>%s</small></td>
          <td><code>%s</code><br/><span class="tag">%s</span></td>
          <td><span class="method">%s</span> <code>%s</code></td>
          <td>HTTP %d <small>(exp %d)</small></td>
          <td>%d ms</td>
          <td>%s</td>
        </tr>`,
			rowClass,
			statusBadge,
			html.EscapeString(rec.StepID),
			html.EscapeString(rec.StepName),
			html.EscapeString(rec.CaseID),
			html.EscapeString(rec.CaseType),
			html.EscapeString(rec.Method),
			html.EscapeString(rec.Endpoint),
			rec.StatusCode,
			rec.ExpectedCode,
			rec.LatencyMs,
			detailHTML,
		))
	}

	doc := fmt.Sprintf(`<!DOCTYPE html>
<html lang="vi">
<head>
  <meta charset="UTF-8" />
  <title>SpecTest-AI Execution Report - %s</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f172a; color: #f8fafc; margin: 0; padding: 24px; }
    .container { max-width: 1280px; margin: 0 auto; }
    header { background: linear-gradient(135deg, #0284c7, #1e3a8a); padding: 20px 28px; border-radius: 12px; margin-bottom: 24px; }
    h1 { margin: 0 0 8px 0; font-size: 24px; }
    .cards { display: grid; grid-template-columns: repeat(5, 1fr); gap: 16px; margin-bottom: 24px; }
    .card { background: #1e293b; border: 1px solid #334155; padding: 16px; border-radius: 10px; }
    .card .label { font-size: 12px; color: #94a3b8; text-transform: uppercase; }
    .card .val { font-size: 26px; font-weight: 700; margin-top: 6px; }
    table { width: 100%%; border-collapse: collapse; background: #1e293b; border-radius: 10px; overflow: hidden; }
    th, td { padding: 12px 14px; text-align: left; border-bottom: 1px solid #334155; vertical-align: top; font-size: 14px; }
    th { background: #0f172a; color: #38bdf8; text-transform: uppercase; font-size: 12px; }
    .badge { padding: 4px 10px; border-radius: 6px; font-weight: 700; font-size: 12px; }
    .badge.pass { background: #22c55e; color: #052e16; }
    .badge.fail { background: #ef4444; color: #450a0a; }
    .tag { background: #334155; color: #cbd5e1; padding: 2px 6px; border-radius: 4px; font-size: 11px; }
    .method { font-weight: 700; color: #38bdf8; }
    .fail-reason { color: #fca5a5; margin-bottom: 6px; }
    .ai-box { background: #451a03; border-left: 4px solid #f59e0b; color: #fde68a; padding: 8px 12px; border-radius: 4px; font-size: 13px; }
    .muted { color: #64748b; font-size: 13px; }
  </style>
</head>
<body>
  <div class="container">
    <header>
      <h1>⚡ SpecTest-AI: 4-Tier Oracle &amp; AI Defect Triage Dashboard</h1>
      <div>Bộ kịch bản: <strong>%s</strong> | Target: <code>%s</code> | Thời gian: <code>%s</code></div>
    </header>
    <div class="cards">
      <div class="card"><div class="label">Tổng số Ca Kiểm Thử</div><div class="val">%d</div></div>
      <div class="card"><div class="label">Passed</div><div class="val" style="color:#4ade80">%d</div></div>
      <div class="card"><div class="label">Failed</div><div class="val" style="color:#f87171">%d</div></div>
      <div class="card"><div class="label">Tỷ lệ Pass</div><div class="val">%.1f%%</div></div>
      <div class="card"><div class="label">AI Triage Calls</div><div class="val" style="color:#fbbf24">%d</div></div>
    </div>
    <table>
      <thead>
        <tr>
          <th>Trạng thái</th>
          <th>Bước (Step)</th>
          <th>Ca Kiểm Thử (Case)</th>
          <th>Endpoint</th>
          <th>HTTP Status</th>
          <th>Latency</th>
          <th>Kết quả 4-Tier Oracle &amp; Chẩn đoán AI</th>
        </tr>
      </thead>
      <tbody>%s
      </tbody>
    </table>
  </div>
</body>
</html>`,
		html.EscapeString(sum.SuiteName),
		html.EscapeString(sum.SuiteName),
		html.EscapeString(sum.TargetURL),
		time.Now().Format(time.RFC3339),
		sum.TotalChecks,
		sum.PassedChecks,
		sum.FailedChecks,
		passRate,
		sum.AICallsMade,
		rows.String(),
	)

	return os.WriteFile(path, []byte(doc), 0o644)
}
