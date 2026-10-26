package run

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nguyentin05/spectest/internal/domain"
)

// TriageContext contains all telemetry and specification context for an HTTP step failure.
type TriageContext struct {
	ScenarioID     string
	ScenarioName   string
	StepCall       string
	Method         string
	URL            string
	ExpectedStatus int
	ActualStatus   int
	RequestHeaders map[string]string
	RequestBody    string
	ResponseBody   string
	SuiteAnalysis  string
}

type triageResponse struct {
	RootCause    string `json:"root_cause"`
	SuggestedFix string `json:"suggested_fix"`
}

// DiagnoseFailure uses the configured LLM service to triage a test step defect,
// pinpointing the root cause and suggesting a remediation.
func DiagnoseFailure(ctx context.Context, llm domain.LLMService, tc TriageContext) (string, error) {
	if llm == nil {
		return "", nil
	}

	systemPrompt := `You are an autonomous AI test engineering defect triage engine.
Analyze the failed API test step provided in the user prompt.
Determine the root cause of the unexpected HTTP status code (e.g. invalid credentials, missing header, validation schema mismatch, business rule violation, database conflict).
Provide a concise root cause and actionable fix.
You MUST reply with ONLY a valid JSON object matching this schema:
{
  "root_cause": "<concise 1-2 sentence explanation of why the call failed>",
  "suggested_fix": "<concrete action to resolve or fix the test/system>"
}`

	var sb strings.Builder
	fmt.Fprintf(&sb, "SCENARIO: [%s] %s\n", tc.ScenarioID, tc.ScenarioName)
	fmt.Fprintf(&sb, "FAILED STEP: %s\n", tc.StepCall)
	fmt.Fprintf(&sb, "HTTP REQUEST: %s %s\n", tc.Method, tc.URL)
	fmt.Fprintf(&sb, "EXPECTED STATUS: %d\n", tc.ExpectedStatus)
	fmt.Fprintf(&sb, "ACTUAL STATUS: %d\n", tc.ActualStatus)

	if len(tc.RequestHeaders) > 0 {
		headersJSON, _ := json.Marshal(tc.RequestHeaders)
		fmt.Fprintf(&sb, "REQUEST HEADERS: %s\n", string(headersJSON))
	}
	if tc.RequestBody != "" {
		fmt.Fprintf(&sb, "REQUEST BODY:\n%s\n", tc.RequestBody)
	}
	if tc.ResponseBody != "" {
		fmt.Fprintf(&sb, "RESPONSE BODY:\n%s\n", tc.ResponseBody)
	}
	if tc.SuiteAnalysis != "" {
		fmt.Fprintf(&sb, "BUSINESS SPECIFICATION CONTEXT:\n%s\n", tc.SuiteAnalysis)
	}

	rawJSON, err := llm.GenerateStructured(ctx, systemPrompt, sb.String(), nil)
	if err != nil {
		return "", fmt.Errorf("ai triage query failed: %w", err)
	}

	var res triageResponse
	if err := json.Unmarshal([]byte(rawJSON), &res); err != nil {
		// If LLM returned text rather than strict JSON, return trimmed string
		return strings.TrimSpace(rawJSON), nil
	}

	parts := make([]string, 0, 2)
	if strings.TrimSpace(res.RootCause) != "" {
		parts = append(parts, "Root cause: "+strings.TrimSpace(res.RootCause))
	}
	if strings.TrimSpace(res.SuggestedFix) != "" {
		parts = append(parts, "Fix: "+strings.TrimSpace(res.SuggestedFix))
	}

	return strings.Join(parts, " | "), nil
}
