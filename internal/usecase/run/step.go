package run

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/nguyentin05/spectest/internal/domain"
)

// maxResponseBodyBytes limits the response body size captured for telemetry and reports (1MB).
const maxResponseBodyBytes = 1 << 20

var serverPlaceholderPattern = regexp.MustCompile(`^\{([a-zA-Z0-9_-]+)\}`)

// executeSingleStep executes an individual HTTP step, records metrics, extracts saved vars,
// and evaluates response status codes against test oracle expectations.
func executeSingleStep(
	ctx context.Context,
	executor HTTPExecutor,
	step domain.Step,
	targets map[string]string,
	defaultTarget string,
	scope *Scope,
	scenarioID string,
	scenarioName string,
	isSetup bool,
	setupName string,
	suiteAnalysis string,
	enableTriage bool,
	llm domain.LLMService,
) ExecutionRecord {
	rec := ExecutionRecord{
		ScenarioID:   scenarioID,
		ScenarioName: scenarioName,
		IsSetup:      isSetup,
		SetupName:    setupName,
		Call:         step.Call,
		ExpectedCode: step.Expect,
	}

	// 1. Parse Call into Method and Path
	callParts := strings.SplitN(strings.TrimSpace(step.Call), " ", 2)
	method := "GET"
	rawPath := ""
	if len(callParts) == 1 {
		rawPath = callParts[0]
	} else {
		method = strings.ToUpper(callParts[0])
		rawPath = callParts[1]
	}
	rec.Method = method

	// 2. Resolve {server} placeholder or prepend default target
	resolvedURL := rawPath
	if match := serverPlaceholderPattern.FindStringSubmatch(rawPath); len(match) > 1 {
		placeholder := match[0]
		serverName := match[1]
		rest := rawPath[len(placeholder):]

		var base string
		if b, ok := targets[serverName]; ok && b != "" {
			base = strings.TrimRight(b, "/")
		} else if defaultTarget != "" {
			base = strings.TrimRight(defaultTarget, "/")
		} else {
			rec.Passed = false
			rec.FailReason = fmt.Sprintf("target alias {%s} is not configured and no default target URL provided", serverName)
			return rec
		}

		if !strings.HasPrefix(rest, "/") && rest != "" {
			resolvedURL = base + "/" + rest
		} else {
			resolvedURL = base + rest
		}
	} else {
		if !strings.HasPrefix(resolvedURL, "http://") && !strings.HasPrefix(resolvedURL, "https://") {
			if defaultTarget == "" {
				rec.Passed = false
				rec.FailReason = "default target URL is required for relative call path without {server} alias"
				return rec
			}
			if !strings.HasPrefix(resolvedURL, "/") {
				resolvedURL = defaultTarget + "/" + resolvedURL
			} else {
				resolvedURL = defaultTarget + resolvedURL
			}
		}
	}

	if !strings.HasPrefix(resolvedURL, "http://") && !strings.HasPrefix(resolvedURL, "https://") {
		rec.Passed = false
		rec.FailReason = fmt.Sprintf("resolved URL %q must be an absolute HTTP/HTTPS URL", resolvedURL)
		return rec
	}

	// 3. Interpolate URL and query parameters
	finalURL := scope.InterpolateString(resolvedURL)
	rec.Endpoint = redactURL(finalURL)

	// 4. Interpolate Headers and auto-add Content-Type
	headers := scope.InterpolateHeaders(step.Headers)
	var bodyReader io.Reader
	var requestPayloadStr string

	if step.Body != nil {
		interpolatedBody := scope.InterpolateBody(step.Body)
		data, err := json.Marshal(interpolatedBody)
		if err != nil {
			rec.Passed = false
			rec.FailReason = fmt.Sprintf("encode request body JSON: %v", err)
			return rec
		}
		bodyReader = bytes.NewReader(data)
		requestPayloadStr = string(data)
		rec.RequestPayload = redactPayload(requestPayloadStr)
		if headers["Content-Type"] == "" && headers["content-type"] == "" {
			headers["Content-Type"] = "application/json"
		}
	}

	// 5. Construct HTTP request
	req, err := http.NewRequestWithContext(ctx, method, finalURL, bodyReader)
	if err != nil {
		rec.Passed = false
		rec.FailReason = fmt.Sprintf("construct request: %v", err)
		return rec
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// 6. Execute HTTP request and measure latency
	stepStart := time.Now()
	resp, err := executor.Do(req)
	rec.LatencyMs = time.Since(stepStart).Milliseconds()
	if err != nil {
		rec.Passed = false
		rec.FailReason = fmt.Sprintf("http dispatch error: %v", err)
		return rec
	}
	defer func() { _ = resp.Body.Close() }()

	rec.StatusCode = resp.StatusCode

	// 7. Read response body with limit and error handling
	limitedReader := io.LimitReader(resp.Body, maxResponseBodyBytes+1)
	respBodyBytes, readErr := io.ReadAll(limitedReader)
	if readErr != nil {
		rec.Passed = false
		rec.FailReason = fmt.Sprintf("read response body: %v", readErr)
		return rec
	}

	if len(respBodyBytes) > maxResponseBodyBytes {
		respBodyBytes = respBodyBytes[:maxResponseBodyBytes]
		rec.ResponseBody = redactPayload(string(respBodyBytes)) + "\n[TRUNCATED: response body exceeded 1MB limit]"
	} else {
		rec.ResponseBody = redactPayload(string(respBodyBytes))
	}

	// 8. Oracle evaluation
	if resp.StatusCode == step.Expect {
		rec.Passed = true
		if len(step.Save) > 0 {
			extracted, err := scope.ExtractSave(step.Save, respBodyBytes, resp.Header)
			if err != nil {
				rec.Passed = false
				rec.FailReason = fmt.Sprintf("save extraction error: %v", err)
				return rec
			}
			rec.Extracted = redactExtracted(extracted)
		}
	} else {
		rec.Passed = false
		rec.FailReason = fmt.Sprintf("expected HTTP status %d, got %d", step.Expect, resp.StatusCode)

		// 9. Trigger AI Defect Triage if enabled and LLM is available
		if enableTriage && llm != nil {
			diag, _ := DiagnoseFailure(ctx, llm, TriageContext{
				ScenarioID:     scenarioID,
				ScenarioName:   scenarioName,
				StepCall:       step.Call,
				Method:         method,
				URL:            rec.Endpoint,
				ExpectedStatus: step.Expect,
				ActualStatus:   resp.StatusCode,
				RequestHeaders: redactHeaders(headers),
				RequestBody:    rec.RequestPayload,
				ResponseBody:   rec.ResponseBody,
				SuiteAnalysis:  suiteAnalysis,
			})
			rec.AIDiagnosis = diag
		}
	}

	return rec
}
