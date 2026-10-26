package generate

import (
	"fmt"
	"sort"

	"github.com/nguyentin05/spectest/internal/domain"
	ucrun "github.com/nguyentin05/spectest/internal/usecase/run"
)

// ImpactIndex indexes the direct relationships between API keys and suite steps.
type ImpactIndex struct {
	DirectSetups    map[string][]string // API key -> setup names
	DirectScenarios map[string][]string // API key -> scenario IDs
}

// BuildImpactIndex parses and matches all calls in suite against apis.
func BuildImpactIndex(suite *domain.Suite, apis map[string]domain.API) (ImpactIndex, error) {
	index := ImpactIndex{
		DirectSetups:    make(map[string][]string),
		DirectScenarios: make(map[string][]string),
	}

	if suite == nil {
		return index, nil
	}

	// Index setup steps
	for name, setup := range suite.Setup {
		for _, step := range setup.Steps {
			if step.Call == "" {
				continue
			}
			matched, ok, err := matchAPI(apis, step.Call)
			if err != nil {
				return index, fmt.Errorf("setup %q call %q match: %w", name, step.Call, err)
			}
			if ok {
				index.DirectSetups[matched.Key] = append(index.DirectSetups[matched.Key], name)
			}
		}
	}

	// Index scenario steps
	for _, sc := range suite.Scenarios {
		for _, step := range sc.Steps {
			if step.Call == "" {
				continue
			}
			matched, ok, err := matchAPI(apis, step.Call)
			if err != nil {
				return index, fmt.Errorf("scenario %q call %q match: %w", sc.ID, step.Call, err)
			}
			if ok {
				index.DirectScenarios[matched.Key] = append(index.DirectScenarios[matched.Key], sc.ID)
			}
		}
	}

	// Sort and deduplicate direct mappings
	for k := range index.DirectSetups {
		index.DirectSetups[k] = dedupeAndSort(index.DirectSetups[k])
	}
	for k := range index.DirectScenarios {
		index.DirectScenarios[k] = dedupeAndSort(index.DirectScenarios[k])
	}

	return index, nil
}

// ComputeImpact calculates all setups and scenarios directly or transitively affected by diff.
func ComputeImpact(suite *domain.Suite, diff APIDiff, index ImpactIndex) (domain.UpdateImpact, error) {
	var impact domain.UpdateImpact

	affectedSetups := make(map[string]bool)
	affectedScenarios := make(map[string]bool)

	// Collect affected API keys from changed and removed sets
	affectedAPIs := make(map[string]bool)
	for _, ch := range diff.Changed {
		affectedAPIs[ch.Key] = true
	}
	for _, rem := range diff.Removed {
		affectedAPIs[rem.Key] = true
	}
	for _, rest := range diff.Restored {
		affectedAPIs[rest.Key] = true
	}

	// 1. Direct setups & direct scenarios
	for apiKey := range affectedAPIs {
		for _, sName := range index.DirectSetups[apiKey] {
			affectedSetups[sName] = true
		}
		for _, scID := range index.DirectScenarios[apiKey] {
			affectedScenarios[scID] = true
		}
	}

	// 2. Expand transitive setup dependencies to scenarios
	if suite != nil && len(affectedSetups) > 0 {
		for _, sc := range suite.Scenarios {
			dag, err := ucrun.ResolveSetupDAG(sc.Setup, suite.Setup)
			if err != nil {
				return impact, fmt.Errorf("resolve DAG for scenario %q: %w", sc.ID, err)
			}
			for _, setupName := range dag {
				if affectedSetups[setupName] {
					affectedScenarios[sc.ID] = true
					break
				}
			}
		}
	}

	// Build sorted slices
	for sName := range affectedSetups {
		impact.Setups = append(impact.Setups, sName)
	}
	for scID := range affectedScenarios {
		impact.Scenarios = append(impact.Scenarios, scID)
	}

	sort.Strings(impact.Setups)
	sort.Strings(impact.Scenarios)

	return impact, nil
}

func dedupeAndSort(items []string) []string {
	if len(items) <= 1 {
		return items
	}
	seen := make(map[string]bool, len(items))
	var out []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}
