package run

import (
	"errors"
	"fmt"

	"github.com/nguyentin05/spectest/internal/domain"
)

// Selection records the scenario selection criteria and metadata for a test run.
type Selection struct {
	Mode        string   `json:"mode"` // "all" or "latest_update"
	UpdateN     int      `json:"update_n,omitempty"`
	ScenarioIDs []string `json:"scenario_ids,omitempty"`
}

// SelectLatestUpdateScenarios selects scenarios affected by the latest history update
// in log, returning a new Suite shallow-copy with a filtered slice of Scenarios (preserving
// original suite scenario order), or an error if log or references are invalid.
func SelectLatestUpdateScenarios(
	suite *domain.Suite,
	log *domain.LogFile,
) (*domain.Suite, Selection, error) {
	if suite == nil {
		return nil, Selection{}, errors.New("suite is nil")
	}
	if log == nil {
		return nil, Selection{}, errors.New("log file is required for only-changed selection")
	}

	// If log has no update entries, report success with 0 scenarios selected
	if len(log.Update) == 0 {
		cloned := *suite
		cloned.Scenarios = []domain.Scenario{}
		return &cloned, Selection{
			Mode:        "latest_update",
			UpdateN:     0,
			ScenarioIDs: []string{},
		}, nil
	}

	// 1. Find update entry with the largest N (do not assume sorted order)
	var latest *domain.UpdateEntry
	for i := range log.Update {
		u := &log.Update[i]
		if latest == nil || u.N > latest.N {
			latest = u
		}
	}

	// Verify that latest update's AfterHash matches the current suite's canonical hash
	currentHash := domain.SuiteHash(suite)
	if latest.AfterHash == "" {
		return nil, Selection{}, fmt.Errorf("history log update #%d is missing after_hash", latest.N)
	}
	if latest.AfterHash != currentHash {
		return nil, Selection{}, fmt.Errorf("suite hash mismatch: spectest.yaml hash %q does not match latest update #%d after_hash %q (suite may have been modified manually or history log is outdated)", currentHash, latest.N, latest.AfterHash)
	}

	selectAll := false
	selectedScenarioIDs := make(map[string]bool)
	changedSetups := make(map[string]bool)

	// 2. Check create block
	if latest.Create != nil {
		if len(latest.Create.Seed) > 0 {
			selectAll = true
		}
		for _, sc := range latest.Create.Scenarios {
			if sc.ID != "" {
				selectedScenarioIDs[sc.ID] = true
			}
		}
		for setupName := range latest.Create.Setup {
			changedSetups[setupName] = true
		}
	}

	// 3. Check changes list
	for _, ch := range latest.Change {
		switch ch.Target {
		case domain.TargetSeed:
			selectAll = true
		case domain.TargetScenario:
			if ch.ID != "" && ch.After != nil {
				selectedScenarioIDs[ch.ID] = true
			}
		case domain.TargetSetup:
			if ch.ID != "" {
				changedSetups[ch.ID] = true
			}
		case domain.TargetAnalysis:
			// Analysis change does not select any specific scenario
		}
	}

	// 4. Resolve transitive setup dependencies for each scenario if setups changed
	if !selectAll && len(changedSetups) > 0 {
		for _, sc := range suite.Scenarios {
			dag, err := ResolveSetupDAG(sc.Setup, suite.Setup)
			if err != nil {
				return nil, Selection{}, fmt.Errorf("resolve setup DAG for scenario %q: %w", sc.ID, err)
			}
			for _, sName := range dag {
				if changedSetups[sName] {
					selectedScenarioIDs[sc.ID] = true
					break
				}
			}
		}
	}

	// 5. Fail closed consistency check:
	// Verify that every scenario ID selected by the log exists in the current suite
	suiteIDSet := make(map[string]bool, len(suite.Scenarios))
	for _, sc := range suite.Scenarios {
		suiteIDSet[sc.ID] = true
	}

	for id := range selectedScenarioIDs {
		if !suiteIDSet[id] {
			return nil, Selection{}, fmt.Errorf("scenario %q referenced in log update #%d not found in suite", id, latest.N)
		}
	}

	// 6. Build filtered scenarios preserving original order
	var filtered []domain.Scenario
	var resultIDs []string

	for _, sc := range suite.Scenarios {
		if selectAll || selectedScenarioIDs[sc.ID] {
			filtered = append(filtered, sc)
			resultIDs = append(resultIDs, sc.ID)
		}
	}

	if filtered == nil {
		filtered = []domain.Scenario{}
	}
	if resultIDs == nil {
		resultIDs = []string{}
	}

	cloned := *suite
	cloned.Scenarios = filtered

	return &cloned, Selection{
		Mode:        "latest_update",
		UpdateN:     latest.N,
		ScenarioIDs: resultIDs,
	}, nil
}
