package generate

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nguyentin05/spectest/internal/adapter/cli/cmdutil"
	"github.com/nguyentin05/spectest/internal/infrastructure/suitefile"
)

func TestGenerateCmd_HelpFlags(t *testing.T) {
	cmd := NewCommand(cmdutil.NewFactory())

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("help failed: %v", err)
	}

	got := out.String()
	for _, flag := range []string{"--api", "--req", "--output", "--dry-run", "--mode"} {
		if !strings.Contains(got, flag) {
			t.Errorf("expected help output to document %q, got:\n%s", flag, got)
		}
	}
	for _, removed := range []string{"--enrich", "--export", "--concurrency"} {
		if strings.Contains(got, removed) {
			t.Errorf("help output must not document removed flag %q, got:\n%s", removed, got)
		}
	}
}

func TestGenerateCmd_UpdateRequiresExistingFiles(t *testing.T) {
	cmd := NewCommand(cmdutil.NewFactory())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--mode", "update", "--api", "auth=whatever.yaml", "--output", t.TempDir()})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "spectest.yaml") {
		t.Fatalf("error = %v, want missing spectest.yaml error", err)
	}
}

func TestGenerateCmd_InvalidMode(t *testing.T) {
	cmd := NewCommand(cmdutil.NewFactory())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--mode", "migrate"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "must be init or update") {
		t.Fatalf("error = %v, want invalid mode", err)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Errorf("invalid mode must still print usage, got:\n%s", out.String())
	}
}

func TestGenerateCmd_UsesCommandContext(t *testing.T) {
	cmd := NewCommand(cmdutil.NewFactory())
	cmd.SetArgs([]string{"--api", "auth=whatever.yaml", "--output", t.TempDir()})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cmd.ExecuteContext(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestGenerateCmd_RejectsPositionalArgs(t *testing.T) {
	cmd := NewCommand(cmdutil.NewFactory())

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"auth=openapi.yaml"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected positional argument to be rejected")
	}
}

func TestGenerateCmd_InitDeclineLeavesFilesUntouched(t *testing.T) {
	dir := t.TempDir()
	suitePath := filepath.Join(dir, suitefile.SuiteFileName)
	if err := os.WriteFile(suitePath, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := NewCommand(cmdutil.NewFactory())
	cmd.SetIn(strings.NewReader("n\n"))
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--api", "auth=whatever.yaml", "--output", dir})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Fatalf("error = %v, want abort", err)
	}
	got, _ := os.ReadFile(suitePath)
	if string(got) != "keep" {
		t.Fatalf("suite changed to %q, want keep", got)
	}
	if !strings.Contains(out.String(), "Overwrite") {
		t.Fatal("expected confirmation prompt")
	}
	if !strings.HasSuffix(out.String(), "[y/N] \n") {
		t.Fatalf("prompt output = %q, want a newline after the prompt", out.String())
	}
	if !strings.Contains(errOut.String(), "aborted") {
		t.Fatalf("stderr = %q, want abort message", errOut.String())
	}
	combined := out.String() + errOut.String()
	if strings.Contains(combined, "Usage:") {
		t.Fatalf("decline must not print usage, got:\n%s", combined)
	}
}

func TestGenerateCmd_MissingAPI(t *testing.T) {
	cmd := NewCommand(cmdutil.NewFactory())

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--api (-a) is missing") {
		t.Fatalf("error = %v, want missing --api validation", err)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Errorf("missing --api must still print usage, got:\n%s", out.String())
	}
}
