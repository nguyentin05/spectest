// Package llm implements the unified HTTP Chat Completions client and provider
// factory used to communicate with cloud and self-hosted Large Language Models.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client implements domain.LLMService using the standard HTTP `/v1/chat/completions`
// protocol supported across cloud APIs and self-hosted inference servers.
type Client struct {
	provider   string
	endpoint   string
	apiKey     string
	model      string
	httpClient *http.Client
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
		httpClient: &http.Client{Timeout: 45 * time.Second},
	}
}

// Name returns a formatted `<provider>/<model> (<endpoint>)` summary string.
func (c *Client) Name() string {
	return fmt.Sprintf("%s/%s (%s)", c.provider, c.model, c.endpoint)
}

// GenerateStructured dispatches a low-temperature (`0.1`) chat completion request
// containing the system instruction and user prompt to `<endpoint>/chat/completions`,
// strips surrounding Markdown code fences if present, and returns the raw JSON string.
func (c *Client) GenerateStructured(ctx context.Context, systemPrompt string, userPrompt string, _ interface{}) (string, error) {
	if c.endpoint == "" {
		return "", fmt.Errorf("endpoint is not configured for provider %q", c.provider)
	}
	if c.model == "" {
		return "", fmt.Errorf("model is not configured for provider %q", c.provider)
	}

	url := c.endpoint + "/chat/completions"

	reqPayload := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.1,
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm request to %s failed: %w", url, err)
	}
	defer resp.Body.Close()

	respData, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm status %d: %s", resp.StatusCode, truncateStr(string(respData), 200))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respData, &parsed); err != nil {
		return "", fmt.Errorf("decode llm response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("empty response choices from %s", url)
	}

	return cleanJSONFence(parsed.Choices[0].Message.Content), nil
}

// cleanJSONFence strips optional Markdown ```json or ``` fences wrapped around LLM output.
func cleanJSONFence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// truncateStr caps a string at max characters and appends "..." when truncated.
func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
