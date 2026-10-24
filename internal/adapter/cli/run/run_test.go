package run

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nguyentin05/spectest/internal/adapter/cli/cmdutil"
	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/storage"
)

func TestRunCmd_Help(t *testing.T) {
	cmd := NewCommand(cmdutil.NewFactory())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run --help failed: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"spectest run",
		"-f, --file",
		"-t, --target",
		"-c, --compose",
		"--timeout",
		"--report-dir",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected help output to contain %q; got:\n%s", want, got)
		}
	}

	// Verify --start-cmd was successfully removed
	if strings.Contains(got, "--start-cmd") {
		t.Error("unexpected --start-cmd flag in help output")
	}
}

func TestRunCmd_Execute_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	suitePath := filepath.Join(tmpDir, "spectest.yaml")
	repo := storage.NewYAMLRepository()

	suite := &domain.TestSuiteFile{
		Version: "1.0",
		Metadata: domain.SuiteMetadata{
			Name:          "E-commerce API",
			TargetBaseURL: srv.URL,
		},
		Base: domain.BaseSuite{
			Scenarios: []domain.Scenario{
				{
					ID:   "SCN_1",
					Name: "Cart Checkout",
					Steps: []domain.Step{
						{
							ID:   "STEP_1",
							Name: "Add item",
							TestCases: []domain.TestCase{
								{ID: "TC_1", Name: "Valid"},
							},
						},
					},
				},
			},
		},
	}
	if err := repo.Save(suitePath, suite); err != nil {
		t.Fatal(err)
	}

	f := cmdutil.NewFactoryWith(nil, repo)
	cmd := NewCommand(f)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"-f", suitePath, "-t", srv.URL, "--timeout", "5"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "E-commerce API") {
		t.Errorf("expected output to mention suite name, got: %s", got)
	}
}

func TestRunCmd_MissingSuite(t *testing.T) {
	f := cmdutil.NewFactory()
	cmd := NewCommand(f)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"-f", "non-existent-suite.yaml", "-t", "http://localhost:8080"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for non-existent suite file, got nil")
	}
}
