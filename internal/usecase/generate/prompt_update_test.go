package generate

import "testing"

func TestUpdatePrompts_Isolated(t *testing.T) {
	if updateSetupSystemPrompt == "" || updateScenarioSystemPrompt == "" {
		t.Fatal("expected update system prompts to be non-empty")
	}
}
