package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewClient_NormalizesEndpoint(t *testing.T) {
	tests := map[string]string{
		"http://localhost:11434":    "http://localhost:11434/v1",
		"http://localhost:11434/":   "http://localhost:11434/v1",
		"https://api.openai.com/v1": "https://api.openai.com/v1",
		"https://host/openai/v1/x":  "https://host/openai/v1/x",
		"":                          "",
	}
	for in, want := range tests {
		if got := NewClient("p", in, "", "m").endpoint; got != want {
			t.Errorf("NewClient(%q).endpoint = %q, want %q", in, got, want)
		}
	}
}

func TestClient_Name(t *testing.T) {
	c := NewClient("ollama", "http://localhost:11434", "", "qwen")
	if got := c.Name(); got != "ollama/qwen (http://localhost:11434/v1)" {
		t.Errorf("Name() = %q", got)
	}
}

func TestClient_GenerateStructured_Success(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"` + "```json\\n{\\\"id\\\":\\\"SC\\\"}\\n```" + `"}}]}`))
	}))
	defer srv.Close()

	c := NewClient("custom", srv.URL, "secret", "test-model")
	out, err := c.GenerateStructured(context.Background(), "sys", "user", nil)
	if err != nil {
		t.Fatalf("GenerateStructured: %v", err)
	}
	if out != `{"id":"SC"}` {
		t.Errorf("output = %q, code fences should be stripped", out)
	}
	if gotPath != "/v1/chat/completions" {
		t.Errorf("request path = %q", gotPath)
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("Authorization header = %q", gotAuth)
	}
	if gotBody["model"] != "test-model" {
		t.Errorf("request model = %v", gotBody["model"])
	}
	msgs, _ := gotBody["messages"].([]interface{})
	if len(msgs) != 2 {
		t.Errorf("expected system + user messages, got %d", len(msgs))
	}
}

func TestClient_GenerateStructured_NoAuthHeaderWithoutKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h := r.Header.Get("Authorization"); h != "" {
			t.Errorf("unexpected Authorization header %q", h)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
	}))
	defer srv.Close()

	if _, err := NewClient("ollama", srv.URL, "", "m").GenerateStructured(context.Background(), "s", "u", nil); err != nil {
		t.Fatalf("GenerateStructured: %v", err)
	}
}

func TestClient_GenerateStructured_Errors(t *testing.T) {
	t.Run("missing endpoint", func(t *testing.T) {
		_, err := NewClient("p", "", "", "m").GenerateStructured(context.Background(), "s", "u", nil)
		if err == nil || !strings.Contains(err.Error(), "endpoint is not configured") {
			t.Errorf("got %v", err)
		}
	})

	t.Run("missing model", func(t *testing.T) {
		_, err := NewClient("p", "http://x", "", "").GenerateStructured(context.Background(), "s", "u", nil)
		if err == nil || !strings.Contains(err.Error(), "model is not configured") {
			t.Errorf("got %v", err)
		}
	})

	t.Run("non-200 status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`invalid api key`))
		}))
		defer srv.Close()
		_, err := NewClient("p", srv.URL, "", "m").GenerateStructured(context.Background(), "s", "u", nil)
		if err == nil || !strings.Contains(err.Error(), "llm status 401") {
			t.Errorf("got %v", err)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`not-json`))
		}))
		defer srv.Close()
		_, err := NewClient("p", srv.URL, "", "m").GenerateStructured(context.Background(), "s", "u", nil)
		if err == nil || !strings.Contains(err.Error(), "decode llm response") {
			t.Errorf("got %v", err)
		}
	})

	t.Run("empty choices", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"choices":[]}`))
		}))
		defer srv.Close()
		_, err := NewClient("p", srv.URL, "", "m").GenerateStructured(context.Background(), "s", "u", nil)
		if err == nil || !strings.Contains(err.Error(), "empty response choices") {
			t.Errorf("got %v", err)
		}
	})

	t.Run("unreachable server", func(t *testing.T) {
		_, err := NewClient("p", "http://127.0.0.1:1", "", "m").GenerateStructured(context.Background(), "s", "u", nil)
		if err == nil || !strings.Contains(err.Error(), "failed") {
			t.Errorf("got %v", err)
		}
	})
}

func TestCleanJSONFence(t *testing.T) {
	tests := map[string]string{
		"{\"a\":1}":               `{"a":1}`,
		"```json\n{\"a\":1}\n```": `{"a":1}`,
		"```\n{\"a\":1}\n```":     `{"a":1}`,
		"  \n{\"a\":1}\n  ":       `{"a":1}`,
	}
	for in, want := range tests {
		if got := cleanJSONFence(in); got != want {
			t.Errorf("cleanJSONFence(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTruncateStr(t *testing.T) {
	if got := truncateStr("abc", 5); got != "abc" {
		t.Errorf("got %q", got)
	}
	if got := truncateStr("abcdef", 3); got != "abc..." {
		t.Errorf("got %q", got)
	}
}

func TestClient_GenerateStructured_RetriesOn429(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`rate limited`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"id\":\"SC\"}"}}]}`))
	}))
	defer srv.Close()

	c := NewClient("p", srv.URL, "", "m")
	c.sleep = func(context.Context, time.Duration) error { return nil }

	out, err := c.GenerateStructured(context.Background(), "s", "u", nil)
	if err != nil {
		t.Fatalf("GenerateStructured: %v", err)
	}
	if out != `{"id":"SC"}` {
		t.Errorf("output = %q", out)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2", calls.Load())
	}
}

func TestClient_GenerateStructured_RetriesOn5xxUntilExhausted(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`server error`))
	}))
	defer srv.Close()

	c := NewClient("p", srv.URL, "", "m")
	c.sleep = func(context.Context, time.Duration) error { return nil }

	_, err := c.GenerateStructured(context.Background(), "s", "u", nil)
	if err == nil || !strings.Contains(err.Error(), "llm status 500") {
		t.Fatalf("error = %v, want llm status 500", err)
	}
	if calls.Load() != 3 {
		t.Errorf("calls = %d, want 3", calls.Load())
	}
}

func TestClient_GenerateStructured_DoesNotRetry4xx(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`invalid api key`))
	}))
	defer srv.Close()

	c := NewClient("p", srv.URL, "", "m")
	c.sleep = func(context.Context, time.Duration) error { return nil }

	_, err := c.GenerateStructured(context.Background(), "s", "u", nil)
	if err == nil || !strings.Contains(err.Error(), "llm status 401") {
		t.Fatalf("error = %v, want llm status 401", err)
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d, want 1", calls.Load())
	}
}

func TestClient_GenerateStructured_RespectsRetryAfter(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
	}))
	defer srv.Close()

	c := NewClient("p", srv.URL, "", "m")
	var gotDelay time.Duration
	c.sleep = func(_ context.Context, d time.Duration) error {
		gotDelay = d
		return nil
	}

	if _, err := c.GenerateStructured(context.Background(), "s", "u", nil); err != nil {
		t.Fatalf("GenerateStructured: %v", err)
	}
	if gotDelay != 2*time.Second {
		t.Errorf("sleep delay = %v, want 2s", gotDelay)
	}
}

func TestClient_GenerateStructured_StopsOnCanceledContext(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	c := NewClient("p", srv.URL, "", "m")
	c.sleep = func(ctx context.Context, _ time.Duration) error {
		cancel()
		return ctx.Err()
	}

	_, err := c.GenerateStructured(ctx, "s", "u", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d, want 1", calls.Load())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestClient_GenerateStructured_RetriesOnAttemptTimeout(t *testing.T) {
	var calls atomic.Int64
	c := NewClient("p", "http://example.test", "", "m")
	c.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	c.timeout = 10 * time.Millisecond
	c.sleep = func(context.Context, time.Duration) error { return nil }

	_, err := c.GenerateStructured(context.Background(), "s", "u", nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if calls.Load() != 3 {
		t.Errorf("calls = %d, want 3", calls.Load())
	}
}

func TestParseRetryAfter(t *testing.T) {
	future := time.Now().Add(3 * time.Second).UTC().Format(http.TimeFormat)
	tests := []struct {
		name string
		in   string
		want time.Duration
		ok   func(time.Duration) bool
	}{
		{name: "seconds", in: "2", want: 2 * time.Second, ok: func(d time.Duration) bool { return d == 2*time.Second }},
		{name: "zero", in: "0", want: 0, ok: func(d time.Duration) bool { return d == 0 }},
		{name: "negative", in: "-1", want: 0, ok: func(d time.Duration) bool { return d == 0 }},
		{name: "empty", in: "", want: 0, ok: func(d time.Duration) bool { return d == 0 }},
		{name: "garbage", in: "soon", want: 0, ok: func(d time.Duration) bool { return d == 0 }},
		{name: "capped", in: "1000000000", want: maxRetryAfter, ok: func(d time.Duration) bool { return d == maxRetryAfter }},
		{name: "http date", in: future, want: 0, ok: func(d time.Duration) bool { return d > 0 && d <= 4*time.Second }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRetryAfter(tt.in)
			if !tt.ok(got) {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestBackoffDelay(t *testing.T) {
	if got := backoffDelay(1, 45*time.Second); got != maxRetryAfter {
		t.Errorf("cap = %v, want %v", got, maxRetryAfter)
	}
	if got := backoffDelay(1, 2*time.Second); got != 2*time.Second {
		t.Errorf("retry-after = %v, want 2s", got)
	}
	for _, attempt := range []int{1, 2, 3} {
		base := baseBackoff << (attempt - 1)
		got := backoffDelay(attempt, 0)
		if got < base*3/4 || got > base*5/4 {
			t.Errorf("backoffDelay(%d, 0) = %v, want within [%v, %v]", attempt, got, base*3/4, base*5/4)
		}
	}
}

func TestCleanJSONFence_AdvancedPreamble(t *testing.T) {
	in := "Certainly! Here is your test scenario JSON:\n```json\n{\n  \"id\": \"SCN_1\"\n}\n```\nLet me know if you need anything else!"
	got := cleanJSONFence(in)
	want := "{\n  \"id\": \"SCN_1\"\n}"
	if got != want {
		t.Errorf("cleanJSONFence = %q, want %q", got, want)
	}
}
