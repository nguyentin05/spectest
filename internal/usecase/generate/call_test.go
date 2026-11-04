package generate

import (
	"strconv"
	"strings"
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
	"gopkg.in/yaml.v3"
)

func TestDecodeStrictJSONPreservesIntegerScenarioPayload(t *testing.T) {
	const raw = `{"api":"orders","scenarios":[{"id":"create-order","name":"Create order","steps":[{"call":"POST /orders","body":{"id":100,"port":8080,"qty":2,"nested":{"ids":[1,2]},"ratio":2.5,"exponent":1e3},"expect":201}]}],"inferred_expectations":[],"skipped_cases":[],"no_scenario_reason":null}`

	var result scenarioResult
	if err := decodeStrictJSON([]byte(raw), &result); err != nil {
		t.Fatalf("decodeStrictJSON: %v", err)
	}
	if len(result.Scenarios) != 1 || len(result.Scenarios[0].Steps) != 1 {
		t.Fatalf("decoded scenarios = %+v, want one scenario with one step", result.Scenarios)
	}

	body, ok := result.Scenarios[0].Steps[0].Body.(map[string]any)
	if !ok {
		t.Fatalf("body type = %T, want map[string]any", result.Scenarios[0].Steps[0].Body)
	}
	for key, want := range map[string]int{"id": 100, "port": 8080, "qty": 2} {
		if got, ok := body[key].(int); !ok || got != want {
			t.Errorf("body[%q] = %T(%v), want int(%d)", key, body[key], body[key], want)
		}
	}
	nested, ok := body["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested body type = %T, want map[string]any", body["nested"])
	}
	ids, ok := nested["ids"].([]any)
	if !ok || len(ids) != 2 {
		t.Fatalf("nested ids = %T(%v), want two-element []any", nested["ids"], nested["ids"])
	}
	for i, want := range []int{1, 2} {
		if got, ok := ids[i].(int); !ok || got != want {
			t.Errorf("nested ids[%d] = %T(%v), want int(%d)", i, ids[i], ids[i], want)
		}
	}
	if got, ok := body["ratio"].(float64); !ok || got != 2.5 {
		t.Errorf("body[ratio] = %T(%v), want float64(2.5)", body["ratio"], body["ratio"])
	}
	if got, ok := body["exponent"].(float64); !ok || got != 1000 {
		t.Errorf("body[exponent] = %T(%v), want float64(1000)", body["exponent"], body["exponent"])
	}

	yamlBytes, err := yaml.Marshal(body)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	for _, want := range []string{"id: 100", "port: 8080", "qty: 2", "- 1", "- 2", "ratio: 2.5"} {
		if !strings.Contains(string(yamlBytes), want) {
			t.Errorf("YAML payload %q does not contain %q", yamlBytes, want)
		}
	}
}

func TestParseJSONNumberPreservesIntegerBounds(t *testing.T) {
	if got, err := parseJSONNumber("-1"); err != nil {
		t.Fatalf("parse negative integer: %v", err)
	} else if value, ok := got.(int); !ok || value != -1 {
		t.Errorf("negative integer = %T(%v), want int(-1)", got, got)
	}

	if got, err := parseJSONNumber("18446744073709551616"); err == nil {
		t.Errorf("out-of-range integer = %T(%v), want an error", got, got)
	}
}

func TestDecodedUnsignedIntegerPassesSchemaValidation(t *testing.T) {
	integer := uint64(^uint(0)>>1) + 1
	raw := `{"payload":{"count":` + strconv.FormatUint(integer, 10) + `}}`
	var decoded map[string]any
	if err := decodeStrictJSON([]byte(raw), &decoded); err != nil {
		t.Fatalf("decodeStrictJSON: %v", err)
	}

	schema := domain.Schema{
		Type: "object",
		Properties: map[string]domain.Schema{
			"count": {Type: "integer"},
		},
	}
	if errs := checkBodyValue("scenario", 0, decoded["payload"], schema, true); len(errs) != 0 {
		t.Fatalf("unsigned integer failed schema validation: %v", errs)
	}
}
