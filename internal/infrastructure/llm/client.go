// Package llm implements the unified HTTP Chat Completions client and provider
// factory used to communicate with cloud and self-hosted Large Language Models.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTimeout = 120 * time.Second
	maxAttempts    = 3
	max429Attempts = 6
	baseBackoff    = 500 * time.Millisecond
	maxBackoff     = 8 * time.Second
	maxRetryAfter  = 30 * time.Second
)

// Client implements domain.LLMService using the standard HTTP `/v1/chat/completions`
// protocol supported across cloud APIs and self-hosted inference servers.
type Client struct {
	provider   string
	endpoint   string
	apiKey     string
	model      string
	httpClient *http.Client
	timeout    time.Duration
	sleep      func(context.Context, time.Duration) error
}

// NewClient constructs a unified LLM Client for the given provider identifier,
// base endpoint URL, optional API key, and model name. If the endpoint does not
// already end with `/v1`, it is normalized automatically.
func NewClient(provider, endpoint, apiKey, model string) *Client {
	ep := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if ep != "" && !strings.HasSuffix(ep, "/v1") && !strings.Contains(ep, "/v1/") {
		ep += "/v1"
	}
	return &Client{
		provider:   strings.TrimSpace(provider),
		endpoint:   ep,
		apiKey:     strings.TrimSpace(apiKey),
		model:      strings.TrimSpace(model),
		httpClient: &http.Client{},
		timeout:    defaultTimeout,
		sleep:      sleepContext,
	}
}

// Name returns a formatted `<provider>/<model> (<endpoint>)` summary string.
func (c *Client) Name() string {
	return fmt.Sprintf("%s/%s (%s)", c.provider, c.model, c.endpoint)
}

// GenerateStructured dispatches a low-temperature (`0.1`) chat completion request
// containing the system instruction and user prompt to `<endpoint>/chat/completions`,
// strips surrounding Markdown code fences if present, and returns the raw JSON string.
// Transient failures (408/429/5xx, network errors, per-attempt timeouts) are retried
// with exponential backoff up to maxAttempts (or max429Attempts for rate limits).
func (c *Client) GenerateStructured(ctx context.Context, systemPrompt string, userPrompt string, _ any) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= max429Attempts; attempt++ {
		raw, retryable, retryAfter, err := c.doRequest(ctx, systemPrompt, userPrompt)
		if err == nil {
			return raw, nil
		}
		lastErr = err
		limit := maxAttempts
		if isRateLimit(err) {
			limit = max429Attempts
		}
		if !retryable || attempt >= limit {
			break
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		delay := backoffDelay(attempt, retryAfter)
		if isRateLimit(err) && retryAfter == 0 {
			// Backoff for 429 rate limit to span across 60s quota windows
			delay = time.Duration(1<<attempt) * 2 * time.Second
			if delay > maxRetryAfter {
				delay = maxRetryAfter
			}
		}
		slog.Debug("retrying llm request", "attempt", attempt, "delay", delay.String(), "error", err)
		if err := c.sleep(ctx, delay); err != nil {
			return "", err
		}
	}
	return "", lastErr
}

func isRateLimit(err error) bool {
	var se *statusError
	if errors.As(err, &se) {
		return se.Code == http.StatusTooManyRequests
	}
	return false
}

// doRequest performs a single chat completion attempt bounded by c.timeout.
// It reports whether the failure is worth retrying and, for HTTP status errors,
// the server-requested Retry-After delay.
func (c *Client) doRequest(ctx context.Context, systemPrompt, userPrompt string) (raw string, retryable bool, retryAfter time.Duration, err error) {
	if c.endpoint == "" {
		return "", false, 0, fmt.Errorf("endpoint is not configured for provider %q", c.provider)
	}
	if c.model == "" {
		return "", false, 0, fmt.Errorf("model is not configured for provider %q", c.provider)
	}

	url := c.endpoint + "/chat/completions"

	reqPayload := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.1,
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", false, 0, err
	}

	attemptCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", false, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", true, 0, fmt.Errorf("llm request to %s failed: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", true, 0, fmt.Errorf("read response from %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return "", isRetryableStatus(resp.StatusCode), retryAfter, &statusError{
			Code:       resp.StatusCode,
			Body:       string(respData),
			RetryAfter: retryAfter,
		}
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respData, &parsed); err != nil {
		return "", false, 0, fmt.Errorf("decode llm response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", false, 0, fmt.Errorf("empty response choices from %s", url)
	}

	return cleanJSONFence(parsed.Choices[0].Message.Content), false, 0, nil
}

// statusError describes a non-200 chat completion response.
type statusError struct {
	Code       int
	Body       string
	RetryAfter time.Duration
}

func (e *statusError) Error() string {
	return fmt.Sprintf("llm status %d: %s", e.Code, truncateStr(e.Body, 200))
}

// isRetryableStatus reports whether an HTTP status code represents a transient
// failure worth retrying (408, 429, or 5xx gateway/server errors).
func isRetryableStatus(code int) bool {
	switch code {
	case http.StatusRequestTimeout, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// parseRetryAfter converts a Retry-After header value (delta-seconds or HTTP date)
// into a duration, capped at maxRetryAfter. Invalid or past values yield 0.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs <= 0 {
			return 0
		}
		if secs > int(maxRetryAfter/time.Second) {
			return maxRetryAfter
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// backoffDelay computes the delay before the next attempt: the server-provided
// retryAfter (capped) when present, otherwise exponential backoff from baseBackoff
// with +/-25% jitter, capped at maxBackoff.
func backoffDelay(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		if retryAfter > maxRetryAfter {
			return maxRetryAfter
		}
		return retryAfter
	}
	d := min(baseBackoff << (attempt - 1), maxBackoff)
	jitterRange := d / 4
	if jitterRange <= 0 {
		return d
	}
	return d + time.Duration(rand.Int64N(int64(2*jitterRange))) - jitterRange
}

// sleepContext waits for d or until ctx is done, whichever comes first.
func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// cleanJSONFence extracts the JSON object or array content from an LLM response string,
// stripping surrounding Markdown code fences (```json ... ```) or conversational preamble/postamble.
func cleanJSONFence(s string) string {
	s = strings.TrimSpace(s)

	// If markdown code block exists, extract within the fence
	if startIdx := strings.Index(s, "```json"); startIdx >= 0 {
		inner := s[startIdx+len("```json"):]
		if endIdx := strings.LastIndex(inner, "```"); endIdx >= 0 {
			s = strings.TrimSpace(inner[:endIdx])
		}
	} else if startIdx := strings.Index(s, "```"); startIdx >= 0 {
		inner := s[startIdx+len("```"):]
		if endIdx := strings.LastIndex(inner, "```"); endIdx >= 0 {
			s = strings.TrimSpace(inner[:endIdx])
		}
	}

	// Find the outermost balanced JSON object `{ ... }` or array `[ ... ]`
	firstObj := strings.Index(s, "{")
	lastObj := strings.LastIndex(s, "}")
	if firstObj >= 0 && lastObj > firstObj {
		return s[firstObj : lastObj+1]
	}

	firstArr := strings.Index(s, "[")
	lastArr := strings.LastIndex(s, "]")
	if firstArr >= 0 && lastArr > firstArr {
		return s[firstArr : lastArr+1]
	}

	return s
}

// truncateStr caps a string at max characters and appends "..." when truncated.
func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
