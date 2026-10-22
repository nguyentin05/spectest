package run

import (
	"gopkg.in/yaml.v3"

	"github.com/nguyentin05/spectest/internal/domain"
)

// BuildEffectiveSuite implements the Single-File Event Sourcing Replay Engine in RAM.
// It clones the immutable BaseSuite and sequentially applies all append-only UpdateBlocks
// (modify_step, add_scenario, disable_step) without mutating the file on disk.
func BuildEffectiveSuite(suiteFile *domain.TestSuiteFile) *domain.BaseSuite {
	if suiteFile == nil {
		return &domain.BaseSuite{}
	}

	effective := deepCopyBase(suiteFile.Base)

	for _, upd := range suiteFile.Updates {
		for _, act := range upd.Actions {
			switch act.Op {
			case "modify_step":
				applyModifyStep(&effective, act)
			case "add_scenario":
				if act.Scenario != nil {
					applyAddScenario(&effective, *act.Scenario)
				}
			case "disable_step":
				applyDisableStep(&effective, act.TargetScenario, act.TargetStep)
			}
		}
	}

	return &effective
}

func applyModifyStep(base *domain.BaseSuite, act domain.UpdateAction) {
	for sIdx := range base.Scenarios {
		sc := &base.Scenarios[sIdx]
		if act.TargetScenario != "" && sc.ID != act.TargetScenario {
			continue
		}
		for stIdx := range sc.Steps {
			st := &sc.Steps[stIdx]
			if st.ID != act.TargetStep {
				continue
			}

			// Patch step.Request.Body if map
			if bodyMap, ok := toStringMap(st.Request.Body); ok {
				patchPayloadMap(bodyMap, act.OverridePayload, act.RemoveFields)
				st.Request.Body = bodyMap
			}

			// Patch all TestCases payloads
			for tcIdx := range st.TestCases {
				tc := &st.TestCases[tcIdx]
				if tcMap, ok := toStringMap(tc.Payload); ok {
					patchPayloadMap(tcMap, act.OverridePayload, act.RemoveFields, tc.Type)
					tc.Payload = tcMap
				}
			}
		}
	}
}

func patchPayloadMap(target map[string]interface{}, overrides map[string]interface{}, removeFields []string, caseType ...string) {
	removedValues := make(map[string]interface{})
	for _, rf := range removeFields {
		if val, exists := target[rf]; exists {
			removedValues[rf] = val
			delete(target, rf)
		}
	}

	isSecurityCase := len(caseType) > 0 && caseType[0] == "go_security"

	for k, v := range overrides {
		if isSecurityCase && (k == "promo_code" || k == "voucher_code") {
			target[k] = "' OR '1'='1"
			continue
		}
		target[k] = v
	}
}

func applyAddScenario(base *domain.BaseSuite, newSc domain.Scenario) {
	for i := range base.Scenarios {
		if base.Scenarios[i].ID == newSc.ID {
			base.Scenarios[i] = newSc
			return
		}
	}
	base.Scenarios = append(base.Scenarios, newSc)
}

func applyDisableStep(base *domain.BaseSuite, targetScenario, targetStep string) {
	for sIdx := range base.Scenarios {
		sc := &base.Scenarios[sIdx]
		if targetScenario != "" && sc.ID != targetScenario {
			continue
		}
		var filtered []domain.Step
		for _, st := range sc.Steps {
			if st.ID != targetStep {
				filtered = append(filtered, st)
			}
		}
		sc.Steps = filtered
	}
}

func toStringMap(v interface{}) (map[string]interface{}, bool) {
	if v == nil {
		return nil, false
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m, true
	}
	// Handle yaml.v3 unmarshaled map
	raw, err := yaml.Marshal(v)
	if err != nil {
		return nil, false
	}
	var out map[string]interface{}
	if err := yaml.Unmarshal(raw, &out); err != nil {
		return nil, false
	}
	return out, true
}

func deepCopyBase(in domain.BaseSuite) domain.BaseSuite {
	raw, err := yaml.Marshal(in)
	if err != nil {
		return in
	}
	var out domain.BaseSuite
	if err := yaml.Unmarshal(raw, &out); err != nil {
		return in
	}
	return out
}
