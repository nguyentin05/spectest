package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestSpectestCmd_HelpAndSubcommands(t *testing.T) {
	cmd := NewSpectestCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute --help failed: %v", err)
	}

	got := out.String()
	for _, sub := range []string{"generate", "config"} {
		if !strings.Contains(got, sub) {
			t.Errorf("expected help output to list subcommand %q, got:\n%s", sub, got)
		}
	}
}

func TestSpectestCmd_Version(t *testing.T) {
	cmd := NewSpectestCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute --version failed: %v", err)
	}

	if !strings.Contains(out.String(), "spectest ") {
		t.Errorf("expected version output, got %q", out.String())
	}
}

func TestConfigSet_RejectsPlaintextAPIKeyArg(t *testing.T) {
	cmd := NewSpectestCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "set", "api-key", "sk-plaintext-leak"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "do not pass api-key on the command line") {
		t.Errorf("expected error rejecting plaintext api-key argument, got: %v", err)
	}
}

func TestGenerateCmd_MissingAPIFlag(t *testing.T) {
	cmd := NewSpectestCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"generate"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--api (-a) is missing") {
		t.Errorf("expected missing --api flag error, got: %v", err)
	}
}

func TestSpectestCmd_VerboseFlag(t *testing.T) {
	cmd := NewSpectestCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--verbose", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute with --verbose failed: %v", err)
	}
}
