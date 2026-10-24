package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// NormalizeTargetURL validates targetURL, auto-prefixes "http://" if scheme is omitted,
// and ensures only http and https protocols are accepted.
func NormalizeTargetURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", errors.New("target url cannot be empty")
	}

	// Auto-prefix http:// if user passed "localhost:8080" or "127.0.0.1:8080"
	if !strings.Contains(rawURL, "://") {
		rawURL = "http://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("invalid target url %q", rawURL)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("invalid target url scheme %q: only http and https are supported", u.Scheme)
	}

	return u.String(), nil
}

// isServerReady reports whether an HTTP response status code indicates the backend server is ready.
// 2xx, 3xx, and 4xx status codes confirm the application server is up, listening, and serving HTTP responses.
// 5xx errors (especially 502 Bad Gateway, 503 Service Unavailable, 504 Gateway Timeout) indicate
// reverse proxies or upstream backends that are still booting, connecting to databases, or malfunctioning.
func isServerReady(statusCode int) bool {
	return statusCode >= 200 && statusCode < 500
}

// WaitForReady polls targetURL until the HTTP server is responsive and healthy, or timeout expires.
// It fails fast if the targetURL is malformed and rejects transient gateway errors (502, 503, 504).
func WaitForReady(ctx context.Context, targetURL string, timeout time.Duration) error {
	normalizedURL, err := NormalizeTargetURL(targetURL)
	if err != nil {
		return err
	}
	targetURL = normalizedURL

	deadline := time.Now().Add(timeout)
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	backoff := 100 * time.Millisecond
	const maxBackoff = 1 * time.Second

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("server at %s not ready within %v timeout", targetURL, timeout)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err == nil {
			resp, reqErr := client.Do(req)
			if reqErr == nil {
				_ = resp.Body.Close()
				if isServerReady(resp.StatusCode) {
					return nil
				}
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}
