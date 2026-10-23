package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
