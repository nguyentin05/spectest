package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nguyentin05/spectest/internal/domain"
)

// RawExecutionResult holds the detailed HTTP response metrics for a single HTTP call.
type RawExecutionResult struct {
	Method       string
	URL          string
	RequestBody  []byte
	StatusCode   int
	LatencyMs    int64
	ResponseBody []byte
	Extracted    map[string]string
}

// Client implements domain.HTTPRunner and executes HTTP requests with latency tracking and state extraction.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new HTTP runner client targeting baseURL.
func NewClient(baseURL string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: &http.Client{Timeout: timeout},
	}
}

// SetBaseURL updates the target base URL.
func (c *Client) SetBaseURL(baseURL string) {
	c.baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
}

// Execute satisfies domain.HTTPRunner by executing a Step and extracting state variables.
func (c *Client) Execute(ctx context.Context, step *domain.Step, state map[string]string) (*domain.StepResult, error) {
	raw, err := c.ExecuteRaw(ctx, step.Request.Method, step.Request.Endpoint, step.Request.Headers, step.Request.Body, step.Extract, state)
	if err != nil {
		return &domain.StepResult{
			StepID:     step.ID,
			Passed:     false,
			FailReason: err.Error(),
		}, err
	}

	passed := true
	failReason := ""
	if step.Assert != nil && step.Assert.Status > 0 && raw.StatusCode != step.Assert.Status {
		passed = false
		failReason = fmt.Sprintf("expected status %d, got %d", step.Assert.Status, raw.StatusCode)
	}

	return &domain.StepResult{
		StepID:     step.ID,
		Passed:     passed,
		StatusCode: raw.StatusCode,
		LatencyMs:  raw.LatencyMs,
		Extracted:  raw.Extracted,
		FailReason: failReason,
	}, nil
}

// ExecuteRaw performs a low-level HTTP request with variable interpolation and JSONPath extraction,
// returning the full RawExecutionResult for the 4-Tier Test Oracle.
func (c *Client) ExecuteRaw(
	ctx context.Context,
	method string,
	endpoint string,
	headers map[string]string,
	bodyPayload interface{},
	extractRules map[string]string,
	state map[string]string,
) (*RawExecutionResult, error) {
	interpEndpoint := InterpolateString(endpoint, state)
	fullURL := interpEndpoint
	if !strings.HasPrefix(interpEndpoint, "http://") && !strings.HasPrefix(interpEndpoint, "https://") {
		if !strings.HasPrefix(interpEndpoint, "/") {
			interpEndpoint = "/" + interpEndpoint
		}
		fullURL = c.baseURL + interpEndpoint
	}

	var reqBodyBytes []byte
	var bodyReader io.Reader
	if bodyPayload != nil {
		interpolatedBody := InterpolatePayload(bodyPayload, state)
		b, err := json.Marshal(interpolatedBody)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBodyBytes = b
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build http request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, InterpolateString(v, state))
	}
	if bodyPayload != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	latencyMs := time.Since(start).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("http request %s %s failed: %w", method, fullURL, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	extracted := make(map[string]string)
	for varName, jsonPathExpr := range extractRules {
		if val, ok := QueryJSONPath(respBytes, jsonPathExpr); ok {
			formatted := FormatExtractedValue(val)
			if formatted != "" {
				extracted[varName] = formatted
			}
		}
	}

	return &RawExecutionResult{
		Method:       strings.ToUpper(method),
		URL:          fullURL,
		RequestBody:  reqBodyBytes,
		StatusCode:   resp.StatusCode,
		LatencyMs:    latencyMs,
		ResponseBody: respBytes,
		Extracted:    extracted,
	}, nil
}
