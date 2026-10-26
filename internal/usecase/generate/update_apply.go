package generate

import (
	"github.com/nguyentin05/spectest/internal/domain"
)

// applyPlanToSuite creates a modified clone of suite by applying planned tool skips and repairs.
// It returns the updated Suite and whether any scenario, setup, or field was mutated.
func applyPlanToSuite(suite *domain.Suite, actions []UpdateAction) (*domain.Suite, bool) {
	if suite == nil {
		return nil, false
	}

	cloned := cloneSuite(suite)
	suiteChanged := false

	for _, act := range actions {
		switch act.Kind {
		case ActionAddToolSkip:
			for i := range cloned.Scenarios {
				if cloned.Scenarios[i].ID == act.ScenarioID {
					if cloned.Scenarios[i].Skip != act.Reason {
						cloned.Scenarios[i].Skip = act.Reason
						suiteChanged = true
					}
					break
				}
			}
		case ActionRemoveToolSkip:
			for i := range cloned.Scenarios {
				if cloned.Scenarios[i].ID == act.ScenarioID {
					if cloned.Scenarios[i].Skip != "" {
						cloned.Scenarios[i].Skip = ""
						suiteChanged = true
					}
					break
				}
			}
		}
	}

	return cloned, suiteChanged
}

// buildCatalogPatch creates the delta patch of APIState entries for an update.
func buildCatalogPatch(diff APIDiff) domain.APICatalog {
	patch := make(domain.APICatalog)

	for _, a := range diff.Added {
		patch[a.Key] = domain.APIState{
			Status: domain.StatusActive,
			Hash:   a.Hash,
			Sig:    a.Sig,
		}
	}
	for _, ch := range diff.Changed {
		patch[ch.Key] = domain.APIState{
			Status: domain.StatusActive,
			Hash:   ch.After.Hash,
			Sig:    ch.After.Sig,
		}
	}
	for _, rest := range diff.Restored {
		patch[rest.Key] = domain.APIState{
			Status: domain.StatusActive,
			Hash:   rest.Hash,
			Sig:    rest.Sig,
		}
	}
	for _, rem := range diff.Removed {
		patch[rem.Key] = domain.APIState{
			Status: domain.StatusRemoved,
			Hash:   rem.State.Hash,
			Sig:    rem.State.Sig,
		}
	}

	return patch
}

// buildOpenAPITrigger compiles the list of changed API keys for the update log trigger.
func buildOpenAPITrigger(diff APIDiff) *domain.OpenAPITrigger {
	var trigger domain.OpenAPITrigger

	for _, a := range diff.Added {
		trigger.Added = append(trigger.Added, a.Key)
	}
	for _, ch := range diff.Changed {
		trigger.Changed = append(trigger.Changed, ch.Key)
	}
	for _, rem := range diff.Removed {
		trigger.Removed = append(trigger.Removed, rem.Key)
	}
	for _, rest := range diff.Restored {
		trigger.Restored = append(trigger.Restored, rest.Key)
	}

	return &trigger
}
