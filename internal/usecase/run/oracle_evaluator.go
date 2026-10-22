package run

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/httpclient"
)

// OracleEvaluation holds the detailed result of evaluating the 4-Tier Test Oracle.
type OracleEvaluation struct {
	Passed     bool
	FailedTier string
	Reason     string
}

// EvaluateOracle executes the 4-Tier Test Oracle against an HTTP response:
// - Tier 1 (HTTP Status & Latency SLA)
// - Tier 2 (JSON Schema Integrity & Security Forbidden Keys Leak Detection)
// - Tier 3 (Business Logic & Mathematical Formula JSONPath Assertions)
// - Tier 4 (Database Round-Trip State Consistency)
func EvaluateOracle(rule *domain.AssertionRule, statusCode int, latencyMs int64, respBody []byte, state map[string]string) OracleEvaluation {
	if rule == nil {
		if statusCode >= 500 {
			return OracleEvaluation{
				Passed:     false,
				FailedTier: "Tier 1: HTTP Status",
				Reason:     fmt.Sprintf("unexpected server error HTTP %d", statusCode),
			}
		}
		return OracleEvaluation{Passed: true}
	}

	// TIER 1: HTTP Status & Max Latency SLA
	if rule.Status > 0 && statusCode != rule.Status {
		return OracleEvaluation{
			Passed:     false,
			FailedTier: "Tier 1: HTTP Status",
			Reason:     fmt.Sprintf("expected HTTP status %d, got %d", rule.Status, statusCode),
		}
	}
	if rule.MaxLatencyMs > 0 && latencyMs > int64(rule.MaxLatencyMs) {
		return OracleEvaluation{
			Passed:     false,
			FailedTier: "Tier 1: Latency SLA",
			Reason:     fmt.Sprintf("response latency %dms exceeded max_latency_ms %dms", latencyMs, rule.MaxLatencyMs),
		}
	}

	// Parse JSON if needed for Tiers 2, 3, 4
	var parsedJSON interface{}
	hasJSON := false
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &parsedJSON); err == nil {
			hasJSON = true
		}
	}

	// TIER 2: Schema Integrity & Security Forbidden Keys
	if rule.SchemaMatch && !hasJSON {
		return OracleEvaluation{
			Passed:     false,
			FailedTier: "Tier 2: Schema Integrity",
			Reason:     "schema_match requires valid JSON response body, but response was not valid JSON",
		}
	}

	if len(rule.ForbidKeys) > 0 && hasJSON {
		if leakedKey := findForbiddenKey(parsedJSON, rule.ForbidKeys); leakedKey != "" {
			return OracleEvaluation{
				Passed:     false,
				FailedTier: "Tier 2: Security (ForbidKeys)",
				Reason:     fmt.Sprintf("security violation: sensitive key '%s' leaked in response payload (forbid_keys)", leakedKey),
			}
		}
	}

	// TIER 3 & TIER 4: Business Logic JSONPath & Round-Trip State Consistency
	for _, rawExpr := range rule.JSONPath {
		isRoundTrip := strings.Contains(rawExpr, "{{")
		interpExpr := httpclient.InterpolateString(rawExpr, state)

		ok, detail := evaluateJSONPathExpression(parsedJSON, interpExpr)
		if !ok {
			tier := "Tier 3: Business Logic (JSONPath)"
			if isRoundTrip {
				tier = "Tier 4: Round-Trip State Consistency"
			}
			return OracleEvaluation{
				Passed:     false,
				FailedTier: tier,
				Reason:     detail,
			}
		}
	}

	return OracleEvaluation{Passed: true}
}

func findForbiddenKey(node interface{}, forbidKeys []string) string {
	switch v := node.(type) {
	case map[string]interface{}:
		for k, child := range v {
			for _, fk := range forbidKeys {
				if strings.EqualFold(k, fk) {
					return k
				}
			}
			if found := findForbiddenKey(child, forbidKeys); found != "" {
				return found
			}
		}
	case []interface{}:
		for _, item := range v {
			if found := findForbiddenKey(item, forbidKeys); found != "" {
				return found
			}
		}
	}
	return ""
}

func evaluateJSONPathExpression(root interface{}, expr string) (bool, string) {
	operators := []string{"==", "!=", ">=", "<=", ">", "<"}
	var op string
	var parts []string

	for _, candidate := range operators {
		if idx := strings.Index(expr, candidate); idx != -1 {
			op = candidate
			parts = []string{
				strings.TrimSpace(expr[:idx]),
				strings.TrimSpace(expr[idx+len(candidate):]),
			}
			break
		}
	}

	// Existence check if no comparison operator is present
	if op == "" {
		val, exists := httpclient.QueryJSONObject(root, expr)
		if !exists || val == nil {
			return false, fmt.Sprintf("JSONPath '%s' not found in response", expr)
		}
		return true, ""
	}

	pathStr := parts[0]
	expectedRaw := trimQuotes(parts[1])

	actualVal, exists := httpclient.QueryJSONObject(root, pathStr)
	if !exists {
		return false, fmt.Sprintf("assertion failed [%s]: path '%s' missing from response", expr, pathStr)
	}

	actualStr := httpclient.FormatExtractedValue(actualVal)

	// Numeric comparison if both sides parse as float64
	actualNum, errA := strconv.ParseFloat(actualStr, 64)
	expectedNum, errE := strconv.ParseFloat(expectedRaw, 64)
	if errA == nil && errE == nil {
		var pass bool
		switch op {
		case "==":
			pass = actualNum == expectedNum
		case "!=":
			pass = actualNum != expectedNum
		case ">=":
			pass = actualNum >= expectedNum
		case "<=":
			pass = actualNum <= expectedNum
		case ">":
			pass = actualNum > expectedNum
		case "<":
			pass = actualNum < expectedNum
		}
		if !pass {
			return false, fmt.Sprintf("assertion failed [%s]: actual value at '%s' is %v, expected %s %v", expr, pathStr, actualStr, op, expectedRaw)
		}
		return true, ""
	}

	// String comparison
	switch op {
	case "==":
		if actualStr != expectedRaw {
			return false, fmt.Sprintf("assertion failed [%s]: actual '%s' != expected '%s'", expr, actualStr, expectedRaw)
		}
	case "!=":
		if actualStr == expectedRaw {
			return false, fmt.Sprintf("assertion failed [%s]: actual '%s' == '%s'", expr, actualStr, expectedRaw)
		}
	default:
		return false, fmt.Sprintf("unsupported string operator '%s' in expression '%s'", op, expr)
	}

	return true, ""
}

func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
