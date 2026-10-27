package generate

import (
	"strings"
	"testing"
)

func TestInitPrompts_Rendering(t *testing.T) {
	rendered, err := renderPrompt("contract.txt", map[string]string{
		"PAYLOAD": `{"operations": []}`,
	})
	if err != nil {
		t.Fatalf("render contract prompt: %v", err)
	}
	if !strings.Contains(rendered, "OpenAPI") || !strings.Contains(rendered, "operations") {
		t.Errorf("rendered prompt missing expected keywords: %s", rendered)
	}
}
