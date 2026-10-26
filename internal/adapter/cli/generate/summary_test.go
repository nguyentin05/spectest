package generate

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
	usecasegenerate "github.com/nguyentin05/spectest/internal/usecase/generate"
)

func TestPrintSummary(t *testing.T) {
	res := &usecasegenerate.Result{
		Mode:           usecasegenerate.ModeInitial,
		Input:          domain.InputOpenAPI,
		Servers:        []string{"auth", "order"},
		APITotal:       2,
		Warnings:       []string{"w1", "w2"},
		SuitePath:      "./tests/spectest.yaml",
		LogPath:        "./tests/spectest.log.yaml",
		NoFilesWritten: false,
	}
	var out bytes.Buffer
	printSummary(&out, res)
	for _, want := range []string{
		"Mode: initial", "Input: openapi", "Servers: auth, order", "APIs: 2",
		"w1", "w2", "./tests/spectest.yaml",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("summary missing %q:\n%s", want, out.String())
		}
	}
}

func TestPrintSummary_DryRun(t *testing.T) {
	var out bytes.Buffer
	printSummary(&out, &usecasegenerate.Result{NoFilesWritten: true})
	if !strings.Contains(out.String(), "no files written (dry-run)") {
		t.Fatalf("summary = %q, want dry-run marker", out.String())
	}
}
