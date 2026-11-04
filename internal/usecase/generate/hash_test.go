package generate

import (
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
)

func TestCloneSuiteDoesNotAliasNestedScenarioPayloads(t *testing.T) {
	sharedBody := map[string]any{
		"user":  map[string]any{"id": 1},
		"items": []any{map[string]any{"qty": 2}},
	}
	source := &domain.Suite{
		Scenarios: []domain.Scenario{
			{ID: "first", Steps: []domain.Step{{Body: sharedBody}}},
			{ID: "sibling", Steps: []domain.Step{{Body: sharedBody}}},
		},
	}

	cloned := cloneSuite(source)
	firstBody := cloned.Scenarios[0].Steps[0].Body.(map[string]any)
	firstBody["user"].(map[string]any)["id"] = 9
	firstBody["items"].([]any)[0].(map[string]any)["qty"] = 99

	assertPayload := func(name string, body any) {
		t.Helper()
		payload := body.(map[string]any)
		if got := payload["user"].(map[string]any)["id"]; got != 1 {
			t.Errorf("%s user id = %v, want original value 1", name, got)
		}
		if got := payload["items"].([]any)[0].(map[string]any)["qty"]; got != 2 {
			t.Errorf("%s item qty = %v, want original value 2", name, got)
		}
	}
	assertPayload("source", source.Scenarios[0].Steps[0].Body)
	assertPayload("sibling", cloned.Scenarios[1].Steps[0].Body)
}
