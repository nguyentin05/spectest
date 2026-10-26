package generate

import (
	"fmt"
	"io"
	"strings"

	usecasegenerate "github.com/nguyentin05/spectest/internal/usecase/generate"
)

// printSummary writes the outcome of a successful generate run.
func printSummary(w io.Writer, res *usecasegenerate.Result) {
	if res.Mode == usecasegenerate.ModeUpdate {
		_, _ = fmt.Fprintf(w, "Mode: update\n")
		_, _ = fmt.Fprintf(w, "API diff: added=%d changed=%d removed=%d restored=%d\n",
			len(res.APIDiff.Added), len(res.APIDiff.Changed), len(res.APIDiff.Removed), len(res.APIDiff.Restored))
		_, _ = fmt.Fprintf(w, "Impact: setups=%d scenarios=%d\n",
			len(res.Impact.Setups), len(res.Impact.Scenarios))
		suiteChangedStr := "no"
		if res.SuiteChanged {
			suiteChangedStr = "yes"
		}
		_, _ = fmt.Fprintf(w, "Suite changed: %s\n", suiteChangedStr)
		llmUsedStr := "no"
		if res.LLMUsed {
			llmUsedStr = "yes"
		}
		_, _ = fmt.Fprintf(w, "LLM used: %s\n", llmUsedStr)
		_, _ = fmt.Fprintf(w, "Update log entry: #%d\n", res.UpdateN)
		if res.NoFilesWritten {
			_, _ = fmt.Fprintln(w, "no files written (dry-run)")
		}
		return
	}

	_, _ = fmt.Fprintf(w, "Mode: %s\n", res.Mode)
	_, _ = fmt.Fprintf(w, "Input: %s\n", res.Input)
	_, _ = fmt.Fprintf(w, "Servers: %s\n", strings.Join(res.Servers, ", "))
	_, _ = fmt.Fprintf(w, "APIs: %d\n", res.APITotal)
	for _, warning := range res.Warnings {
		_, _ = fmt.Fprintf(w, "Warning: %s\n", warning)
	}
	_, _ = fmt.Fprintf(w, "Suite: %s\n", res.SuitePath)
	_, _ = fmt.Fprintf(w, "Log: %s\n", res.LogPath)
	if res.NoFilesWritten {
		_, _ = fmt.Fprintln(w, "no files written (dry-run)")
	}
}
