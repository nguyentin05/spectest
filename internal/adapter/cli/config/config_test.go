package config

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/nguyentin05/spectest/internal/adapter/cli/cmdutil"
)

func cleanEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SPECTEST_PROVIDER", "")
	t.Setenv("SPECTEST_ENDPOINT", "")
	t.Setenv("SPECTEST_MODEL", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
}

func TestConfigCmd_Help(t *testing.T) {
	f := cmdutil.NewFactory()
	cmd := NewCommand(f)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("help failed: %v", err)
	}

	got := out.String()
	for _, sub := range []string{"init", "set", "list"} {
		if !strings.Contains(got, sub) {
			t.Errorf("expected config help to contain subcommand %q, got:\n%s", sub, got)
		}
	}
}

func TestConfigInit_WithFlags(t *testing.T) {
	cleanEnv(t)
	f := cmdutil.NewFactory()
	cmd := NewCommand(f)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"init",
		"--provider", "openai",
		"--endpoint", "https://api.openai.com/v1",
		"--model", "gpt-4o-mini",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("init with flags failed: %v", err)
	}

	store := f.CredentialStore()
	if got := store.GetDefaultProvider(); got != "openai" {
		t.Errorf("expected provider 'openai', got %q", got)
	}
	ep, err := store.GetEndpoint("openai")
	if err != nil || ep != "https://api.openai.com/v1" {
		t.Errorf("expected endpoint 'https://api.openai.com/v1', got %q (err: %v)", ep, err)
	}
	if model := store.GetDefaultModel("openai"); model != "gpt-4o-mini" {
		t.Errorf("expected model 'gpt-4o-mini', got %q", model)
	}
}

func TestConfigInit_MissingProviderWithFlags(t *testing.T) {
	cleanEnv(t)
	f := cmdutil.NewFactory()
	cmd := NewCommand(f)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"init",
		"--endpoint", "https://api.openai.com/v1",
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--provider is required") {
		t.Fatalf("expected error requiring --provider, got: %v", err)
	}
}

func TestConfigInit_Interactive(t *testing.T) {
	cleanEnv(t)
	f := cmdutil.NewFactory()
	cmd := NewCommand(f)

	var in bytes.Buffer
	in.WriteString("ollama\n")
	in.WriteString("http://localhost:11434\n")
	in.WriteString("qwen2.5-coder:7b\n")
	in.WriteString("test-secret-key-12345678\n")

	var out bytes.Buffer
	cmd.SetIn(&in)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"init"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("interactive init failed: %v", err)
	}

	store := f.CredentialStore()
	if got := store.GetDefaultProvider(); got != "ollama" {
		t.Errorf("expected provider 'ollama', got %q", got)
	}
	ep, _ := store.GetEndpoint("ollama")
	if ep != "http://localhost:11434" {
		t.Errorf("expected endpoint 'http://localhost:11434', got %q", ep)
	}
	model := store.GetDefaultModel("ollama")
	if model != "qwen2.5-coder:7b" {
		t.Errorf("expected model 'qwen2.5-coder:7b', got %q", model)
	}
	key, _ := store.GetAPIKey("ollama")
	if key != "test-secret-key-12345678" {
		t.Errorf("expected secret key 'test-secret-key-12345678', got %q", key)
	}
}

func TestConfigInit_Interactive_EmptyProviderError(t *testing.T) {
	cleanEnv(t)
	f := cmdutil.NewFactory()
	cmd := NewCommand(f)

	var in bytes.Buffer
	in.WriteString("  \n") // empty provider input

	var out bytes.Buffer
	cmd.SetIn(&in)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"init"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "provider name is required") {
		t.Fatalf("expected provider name is required error, got: %v", err)
	}
}

func TestConfigSet(t *testing.T) {
	cleanEnv(t)
	f := cmdutil.NewFactory()

	// 1. Set default provider
	cmd := NewCommand(f)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"set", "provider", "anthropic"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set provider failed: %v", err)
	}

	// 2. Set endpoint
	cmd = NewCommand(f)
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"set", "endpoint", "https://api.anthropic.com/v1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set endpoint failed: %v", err)
	}

	// 3. Set model
	cmd = NewCommand(f)
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"set", "model", "claude-3-5-sonnet"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set model failed: %v", err)
	}

	// 4. Set api-key via hidden prompt (non-terminal stdin)
	cmd = NewCommand(f)
	out.Reset()
	var in bytes.Buffer
	in.WriteString("sk-ant-1234567890abcdef\n")
	cmd.SetIn(&in)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"set", "api-key"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set api-key via prompt failed: %v", err)
	}

	// Verify all settings in store
	store := f.CredentialStore()
	if store.GetDefaultProvider() != "anthropic" {
		t.Errorf("expected provider anthropic, got %s", store.GetDefaultProvider())
	}
	ep, _ := store.GetEndpoint("anthropic")
	if ep != "https://api.anthropic.com/v1" {
		t.Errorf("expected endpoint https://api.anthropic.com/v1, got %s", ep)
	}
	model := store.GetDefaultModel("anthropic")
	if model != "claude-3-5-sonnet" {
		t.Errorf("expected model claude-3-5-sonnet, got %s", model)
	}
	key, _ := store.GetAPIKey("anthropic")
	if key != "sk-ant-1234567890abcdef" {
		t.Errorf("expected key sk-ant-1234567890abcdef, got %s", key)
	}

	// 5. Reject passing api-key on command line
	cmd = NewCommand(f)
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"set", "api-key", "sk-leaked-in-bash-history"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "do not pass api-key on the command line") {
		t.Fatalf("expected error rejecting plaintext api-key flag, got: %v", err)
	}

	// 6. Reject empty api-key at prompt
	cmd = NewCommand(f)
	out.Reset()
	in.Reset()
	in.WriteString("   \n") // empty
	cmd.SetIn(&in)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"set", "api-key"})
	err = cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "api-key cannot be empty") {
		t.Fatalf("expected error rejecting empty api-key, got: %v", err)
	}

	// 7. Reject missing value for key
	cmd = NewCommand(f)
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"set", "endpoint"})
	err = cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "missing value for key") {
		t.Fatalf("expected missing value error, got: %v", err)
	}

	// 8. Reject unknown configuration key
	cmd = NewCommand(f)
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"set", "unknown_key", "value"})
	err = cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "unknown configuration key") {
		t.Fatalf("expected unknown key error, got: %v", err)
	}
}

func TestConfigList(t *testing.T) {
	cleanEnv(t)
	f := cmdutil.NewFactory()
	store := f.CredentialStore()

	_ = store.SetDefaultProvider("custom-prov")
	_ = store.SetEndpoint("custom-prov", "https://custom.endpoint.test/v1")
	_ = store.SetDefaultModel("custom-prov", "deepseek-coder")
	_ = store.SetAPIKey("custom-prov", "sk-secret-1234567890abcdef")

	cmd := NewCommand(f)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"list"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config list failed: %v", err)
	}

	got := out.String()
	expectedSubstrings := []string{
		"Active SpecTest Configuration:",
		"Default Provider : custom-prov",
		"Active Model     : deepseek-coder",
		"Endpoint URL     : https://custom.endpoint.test/v1",
		"API Key          : sk-s...cdef", // masked
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(got, sub) {
			t.Errorf("expected config list output to contain %q, got:\n%s", sub, got)
		}
	}
}

func TestDefaultLabel(t *testing.T) {
	if got := defaultLabel(""); got != "None" {
		t.Errorf("expected 'None' for empty string, got %q", got)
	}
	if got := defaultLabel("   "); got != "None" {
		t.Errorf("expected 'None' for whitespace string, got %q", got)
	}
	if got := defaultLabel("openai"); got != "openai" {
		t.Errorf("expected 'openai', got %q", got)
	}
}

func TestReadSecretLine_NonTerminal(t *testing.T) {
	input := "my-secret-key-123\n"
	in := strings.NewReader(input)
	var out bytes.Buffer
	reader := bufio.NewReader(in)

	secret, err := readSecretLine(in, &out, reader)
	if err != nil {
		t.Fatalf("readSecretLine failed: %v", err)
	}
	if secret != "my-secret-key-123" {
		t.Errorf("expected 'my-secret-key-123', got %q", secret)
	}
}

func TestIsTerminalReader(t *testing.T) {
	buf := bytes.NewBufferString("hello")
	if isTerminalReader(buf) {
		t.Error("expected bytes.Buffer to not be recognized as terminal reader")
	}
}
