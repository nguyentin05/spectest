package generate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nguyentin05/spectest/internal/domain"
	ucrun "github.com/nguyentin05/spectest/internal/usecase/run"
)

// ActionKind represents the type of plan action to execute during an update.
type ActionKind string

const (
	ActionKeep           ActionKind = "keep"
	ActionCreateSetup    ActionKind = "create_setup"
	ActionCreateScenario ActionKind = "create_scenario"
	ActionRepairSetup    ActionKind = "repair_setup"
	ActionRepairScenario ActionKind = "repair_scenario"
	ActionAddToolSkip    ActionKind = "add_tool_skip"
	ActionRemoveToolSkip ActionKind = "remove_tool_skip"
)

// UpdateAction describes one planned modification or entity addition.
type UpdateAction struct {
	Kind        ActionKind `json:"kind"`
	SetupName   string     `json:"setup_name,omitempty"`
	ScenarioID  string     `json:"scenario_id,omitempty"`
	APIKeys     []string   `json:"api_keys,omitempty"`
	Reason      string     `json:"reason,omitempty"`
	RequiresLLM bool       `json:"requires_llm"`
}

// ToolSkipPrefix is the prefix used for skips automatically applied by spectest on removed APIs.
const ToolSkipPrefix = "spectest: API"

// BuildUpdatePlan plans deterministic and LLM-assisted mutations for an update.
func BuildUpdatePlan(
	suite *domain.Suite,
	diff APIDiff,
	impact domain.UpdateImpact,
	index ImpactIndex,
	currentAPIs map[string]domain.API,
) ([]UpdateAction, error) {
	var actions []UpdateAction

	if diff.IsEmpty() {
		return actions, nil
	}

	scenarioMap := make(map[string]domain.Scenario)
	if suite != nil {
		for _, sc := range suite.Scenarios {
			scenarioMap[sc.ID] = sc
		}
	}

	// 1. Process Removed APIs -> AddToolSkip
	if len(diff.Removed) > 0 {
		removedKeys := make(map[string]bool, len(diff.Removed))
		for _, rem := range diff.Removed {
			removedKeys[rem.Key] = true
		}

		// Find affected scenarios directly or transitively
		affectedScenarios := make(map[string][]string) // scenarioID -> removed API keys
		for _, rem := range diff.Removed {
			// Direct scenarios
			for _, scID := range index.DirectScenarios[rem.Key] {
				affectedScenarios[scID] = append(affectedScenarios[scID], rem.Key)
			}
			// Direct setups -> transitive scenarios
			for _, sName := range index.DirectSetups[rem.Key] {
				for _, sc := range suite.Scenarios {
					dag, err := ucrun.ResolveSetupDAG(sc.Setup, suite.Setup)
					if err == nil {
						for _, dep := range dag {
							if dep == sName {
								affectedScenarios[sc.ID] = append(affectedScenarios[sc.ID], rem.Key)
								break
							}
						}
					}
				}
			}
		}

		for scID, apis := range affectedScenarios {
			sc, exists := scenarioMap[scID]
			if !exists {
				continue
			}
			if sc.Locked {
				return nil, fmt.Errorf("locked scenario %q requires update because APIs were removed", scID)
			}
			// User-authored skips must not be overwritten
			if sc.Skip != "" && !strings.HasPrefix(sc.Skip, ToolSkipPrefix) {
				continue
			}

			apis = dedupeAndSort(apis)
			var reason string
			if len(apis) == 1 {
				reason = fmt.Sprintf("spectest: API %s removed", apis[0])
			} else {
				reason = fmt.Sprintf("spectest: APIs removed: %s", strings.Join(apis, ", "))
			}

			actions = append(actions, UpdateAction{
				Kind:        ActionAddToolSkip,
				ScenarioID:  scID,
				APIKeys:     apis,
				Reason:      reason,
				RequiresLLM: false,
			})
		}
	}

	// 2. Process Restored APIs -> RemoveToolSkip
	if len(diff.Restored) > 0 {
		candidateScenarios := make(map[string][]string) // scID -> restoredKeys
		for _, rest := range diff.Restored {
			// Direct scenarios
			for _, scID := range index.DirectScenarios[rest.Key] {
				candidateScenarios[scID] = append(candidateScenarios[scID], rest.Key)
			}
			// Direct setups -> transitive scenarios
			for _, sName := range index.DirectSetups[rest.Key] {
				for _, sc := range suite.Scenarios {
					dag, err := ucrun.ResolveSetupDAG(sc.Setup, suite.Setup)
					if err == nil {
						for _, dep := range dag {
							if dep == sName {
								candidateScenarios[sc.ID] = append(candidateScenarios[sc.ID], rest.Key)
								break
							}
						}
					}
				}
			}
		}

		for scID, apis := range candidateScenarios {
			sc, exists := scenarioMap[scID]
			if !exists || sc.Locked {
				continue
			}
			if !strings.HasPrefix(sc.Skip, ToolSkipPrefix) {
				continue
			}

			// Verify that ALL calls in sc and its setup DAG are now present in currentAPIs
			allPresent := true
			for _, step := range sc.Steps {
				matched, ok, _ := matchAPI(currentAPIs, step.Call)
				if !ok || matched.Key == "" {
					allPresent = false
					break
				}
			}
			if allPresent {
				dag, err := ucrun.ResolveSetupDAG(sc.Setup, suite.Setup)
				if err == nil {
					for _, dep := range dag {
						depSetup := suite.Setup[dep]
						for _, step := range depSetup.Steps {
							matched, ok, _ := matchAPI(currentAPIs, step.Call)
							if !ok || matched.Key == "" {
								allPresent = false
								break
							}
						}
						if !allPresent {
							break
						}
					}
				}
			}

			if allPresent {
				actions = append(actions, UpdateAction{
					Kind:        ActionRemoveToolSkip,
					ScenarioID:  scID,
					APIKeys:     dedupeAndSort(apis),
					Reason:      "API restored",
					RequiresLLM: false,
				})
			}
		}
	}

	// 3. Process Changed APIs -> Validate contracts, plan repair if incompatible
	if len(diff.Changed) > 0 {
		changedMap := make(map[string]domain.API, len(diff.Changed))
		for _, ch := range diff.Changed {
			changedMap[ch.Key] = ch.After
		}

		// Check setups
		repairedSetups := make(map[string]bool)
		for _, sName := range impact.Setups {
			setup, exists := suite.Setup[sName]
			if !exists {
				continue
			}

			incompatible := false
			var incompatibleAPIs []string
			var reason string

			for _, step := range setup.Steps {
				matched, ok, err := matchAPI(currentAPIs, step.Call)
				if err == nil && ok {
					if _, changed := changedMap[matched.Key]; changed {
						if valErr := validateStepContract(step, matched); valErr != nil {
							incompatible = true
							incompatibleAPIs = append(incompatibleAPIs, matched.Key)
							reason = valErr.Error()
							break
						}
					}
				}
			}

			if incompatible {
				repairedSetups[sName] = true
				actions = append(actions, UpdateAction{
					Kind:        ActionRepairSetup,
					SetupName:   sName,
					APIKeys:     dedupeAndSort(incompatibleAPIs),
					Reason:      reason,
					RequiresLLM: true,
				})
			}
		}

		// Check scenarios
		for _, scID := range impact.Scenarios {
			sc, exists := scenarioMap[scID]
			if !exists {
				continue
			}

			incompatible := false
			var incompatibleAPIs []string
			var reason string

			for _, step := range sc.Steps {
				matched, ok, err := matchAPI(currentAPIs, step.Call)
				if err == nil && ok {
					if _, changed := changedMap[matched.Key]; changed {
						if valErr := validateStepContract(step, matched); valErr != nil {
							incompatible = true
							incompatibleAPIs = append(incompatibleAPIs, matched.Key)
							reason = valErr.Error()
							break
						}
					}
				}
			}

			// If any setup this scenario depends on was repaired, this scenario also needs repair
			if !incompatible {
				dag, err := ucrun.ResolveSetupDAG(sc.Setup, suite.Setup)
				if err == nil {
					for _, dep := range dag {
						if repairedSetups[dep] {
							incompatible = true
							reason = fmt.Sprintf("depends on repaired setup %q", dep)
							break
						}
					}
				}
			}

			if incompatible {
				if sc.Locked {
					return nil, fmt.Errorf("locked scenario %q requires update due to incompatible API contract change", scID)
				}
				actions = append(actions, UpdateAction{
					Kind:        ActionRepairScenario,
					ScenarioID:  scID,
					APIKeys:     dedupeAndSort(incompatibleAPIs),
					Reason:      reason,
					RequiresLLM: true,
				})
			}
		}
	}

	// 4. Process Added APIs -> Plan CreateScenario
	if len(diff.Added) > 0 {
		var addedKeys []string
		for _, add := range diff.Added {
			addedKeys = append(addedKeys, add.Key)
		}
		sort.Strings(addedKeys)

		actions = append(actions, UpdateAction{
			Kind:        ActionCreateScenario,
			APIKeys:     addedKeys,
			Reason:      "new API operations added to specification",
			RequiresLLM: true,
		})
	}

	// Deterministic sort: Kind priority (Setups before Scenarios), SetupName, ScenarioID, APIKeys
	sort.SliceStable(actions, func(i, j int) bool {
		ri, rj := actionKindRank(actions[i].Kind), actionKindRank(actions[j].Kind)
		if ri != rj {
			return ri < rj
		}
		if actions[i].SetupName != actions[j].SetupName {
			return actions[i].SetupName < actions[j].SetupName
		}
		if actions[i].ScenarioID != actions[j].ScenarioID {
			return actions[i].ScenarioID < actions[j].ScenarioID
		}
		return strings.Join(actions[i].APIKeys, ",") < strings.Join(actions[j].APIKeys, ",")
	})

	return actions, nil
}

func actionKindRank(k ActionKind) int {
	switch k {
	case ActionRemoveToolSkip:
		return 1
	case ActionRepairSetup:
		return 2
	case ActionRepairScenario:
		return 3
	case ActionCreateScenario:
		return 4
	case ActionAddToolSkip:
		return 5
	default:
		return 99
	}
}
