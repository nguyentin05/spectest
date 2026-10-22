package presenter

import (
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss"
	"github.com/nguyentin05/spectest/internal/usecase/generate"
)

var (
	bannerStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F8FAFC")).
		Background(lipgloss.Color("#0284C7")).
		Padding(0, 2)
)

// RenderGenerateSummary renders the Terminal TUI output for `spectest generate`.
func RenderGenerateSummary(w io.Writer, res *generate.Result, providerName string) {
	fmt.Fprintln(w, bannerStyle.Render("SPECTEST: HYBRID TEST GENERATOR"))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  LLM Provider        : %s\n", providerName)
	fmt.Fprintf(w, "  Spec Pruning        : %.1f%% reduction\n", res.CompressionRatio)
	if res.AuthProducer != "" {
		fmt.Fprintf(w, "  Auth Detection      : Detected producer [%s] -> wired Bearer token\n", res.AuthProducer)
	}
	fmt.Fprintf(w, "  Domain Chunks       : %d processed\n", res.ChunksProcessed)
	fmt.Fprintf(w, "  Test Suite File     : %s\n", res.SuitePath)
	fmt.Fprintf(w, "  Coverage Matrix     : %s\n", res.MatrixPath)
	if res.ExportPath != "" {
		fmt.Fprintf(w, "  Exported Collection : %s\n", res.ExportPath)
	}
	fmt.Fprintln(w)
}
