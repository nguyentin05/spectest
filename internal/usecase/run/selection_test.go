package run

import (
	"reflect"
	"strings"
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
)

func sampleSuiteForSelection() *domain.Suite {
	return &domain.Suite{
		Version: 1,
		Input:   domain.InputOpenAPI,
		Servers: []string{"auth", "order"},
		Setup: map[string]domain.Setup{
			"setupA": {
				Steps: []domain.Step{{Call: "POST {auth}/init", Expect: 200}},
			},
			"setupB": {
				Use:   []string{"setupA"},
				Steps: []domain.Step{{Call: "POST {order}/init", Expect: 200}},
			},
			"setupC": {
				Steps: []domain.Step{{Call: "GET {auth}/status", Expect: 200}},
			},
		},
		Scenarios: []domain.Scenario{
			{
				ID:    "sc-1",
				Name:  "Scenario 1",
				Setup: []string{"setupB"}, // Depends on setupB -> transitively depends on setupA
			},
			{
				ID:    "sc-2",
				Name:  "Scenario 2",
				Setup: []string{"setupC"}, // Depends on setupC only
			},
			{
				ID:   "sc-3",
				Name: "Scenario 3",
				// No setups
			},
		},
	}
}

// Test 2: Update tạo scenario → chỉ scenario đó chạy.
func TestSelectLatestUpdateScenarios_CreateScenario(t *testing.T) {
	suite := sampleSuiteForSelection()
	log := &domain.LogFile{
		Update: []domain.UpdateEntry{
			{
				N:         1,
				AfterHash: domain.SuiteHash(suite),
				Create: &domain.CreateSet{
					Scenarios: []domain.Scenario{{ID: "sc-2"}},
				},
			},
		},
	}

	filtered, sel, err := SelectLatestUpdateScenarios(suite, log)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sel.Mode != "latest_update" || sel.UpdateN != 1 {
		t.Errorf("selection = %#v, want mode: latest_update, update: 1", sel)
	}
	wantIDs := []string{"sc-2"}
	if !reflect.DeepEqual(sel.ScenarioIDs, wantIDs) {
		t.Errorf("ScenarioIDs = %v, want %v", sel.ScenarioIDs, wantIDs)
	}
	if len(filtered.Scenarios) != 1 || filtered.Scenarios[0].ID != "sc-2" {
		t.Errorf("filtered scenarios = %v, want only sc-2", filtered.Scenarios)
	}
	// Verify original suite was not mutated
	if len(suite.Scenarios) != 3 {
		t.Errorf("original suite scenarios mutated: len = %d, want 3", len(suite.Scenarios))
	}
}

// Test 3: Update đổi scenario → chỉ scenario đó chạy.
func TestSelectLatestUpdateScenarios_ChangeScenario(t *testing.T) {
	suite := sampleSuiteForSelection()
	log := &domain.LogFile{
		Update: []domain.UpdateEntry{
			{
				N:         1,
				AfterHash: domain.SuiteHash(suite),
				Change: []domain.Change{
					{
						Target: domain.TargetScenario,
						ID:     "sc-3",
						Reason: "updated assertion",
						After:  map[string]any{"expect": 201},
					},
				},
			},
		},
	}

	filtered, sel, err := SelectLatestUpdateScenarios(suite, log)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantIDs := []string{"sc-3"}
	if !reflect.DeepEqual(sel.ScenarioIDs, wantIDs) {
		t.Errorf("ScenarioIDs = %v, want %v", sel.ScenarioIDs, wantIDs)
	}
	if len(filtered.Scenarios) != 1 || filtered.Scenarios[0].ID != "sc-3" {
		t.Errorf("filtered scenarios = %v, want only sc-3", filtered.Scenarios)
	}
}

// Test 4: Update đổi setup A → chọn scenario phụ thuộc A qua dependency transitive.
func TestSelectLatestUpdateScenarios_ChangeSetup_TransitiveDependency(t *testing.T) {
	suite := sampleSuiteForSelection()
	// setupA changed. sc-1 depends on setupB which depends on setupA.
	// sc-2 depends on setupC, sc-3 has no setups.
	log := &domain.LogFile{
		Update: []domain.UpdateEntry{
			{
				N:         2,
				AfterHash: domain.SuiteHash(suite),
				Change: []domain.Change{
					{
						Target: domain.TargetSetup,
						ID:     "setupA",
						Reason: "new auth token parameter",
						After:  map[string]any{"steps": []any{}},
					},
				},
			},
		},
	}

	filtered, sel, err := SelectLatestUpdateScenarios(suite, log)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantIDs := []string{"sc-1"}
	if !reflect.DeepEqual(sel.ScenarioIDs, wantIDs) {
		t.Errorf("ScenarioIDs = %v, want %v", sel.ScenarioIDs, wantIDs)
	}
	if len(filtered.Scenarios) != 1 || filtered.Scenarios[0].ID != "sc-1" {
		t.Errorf("filtered scenarios = %v, want only sc-1", filtered.Scenarios)
	}
}

// Test 5: Update đổi seed → chạy toàn bộ.
func TestSelectLatestUpdateScenarios_ChangeSeed_SelectsAll(t *testing.T) {
	suite := sampleSuiteForSelection()

	// Case 1: change.target: seed
	log1 := &domain.LogFile{
		Update: []domain.UpdateEntry{
			{
				N:         1,
				AfterHash: domain.SuiteHash(suite),
				Change: []domain.Change{
					{
						Target: domain.TargetSeed,
						ID:     "global",
						Reason: "updated base URL",
						After:  map[string]any{"vars": map[string]any{"base_url": "http://new.test"}},
					},
				},
			},
		},
	}

	filtered, sel, err := SelectLatestUpdateScenarios(suite, log1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantIDs := []string{"sc-1", "sc-2", "sc-3"}
	if !reflect.DeepEqual(sel.ScenarioIDs, wantIDs) {
		t.Errorf("ScenarioIDs = %v, want %v", sel.ScenarioIDs, wantIDs)
	}
	if len(filtered.Scenarios) != 3 {
		t.Errorf("filtered scenarios len = %d, want 3", len(filtered.Scenarios))
	}

	// Case 2: create.seed
	log2 := &domain.LogFile{
		Update: []domain.UpdateEntry{
			{
				N:         2,
				AfterHash: domain.SuiteHash(suite),
				Create: &domain.CreateSet{
					Seed: map[string]domain.Seed{"staging": {}},
				},
			},
		},
	}

	filtered2, sel2, err := SelectLatestUpdateScenarios(suite, log2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(sel2.ScenarioIDs, wantIDs) {
		t.Errorf("ScenarioIDs = %v, want %v", sel2.ScenarioIDs, wantIDs)
	}
	if len(filtered2.Scenarios) != 3 {
		t.Errorf("filtered scenarios len = %d, want 3", len(filtered2.Scenarios))
	}
}

// Test 6: Update chỉ đổi analysis → 0 scenarios selected.
func TestSelectLatestUpdateScenarios_ChangeAnalysis_ZeroScenarios(t *testing.T) {
	suite := sampleSuiteForSelection()
	log := &domain.LogFile{
		Update: []domain.UpdateEntry{
			{
				N:         1,
				AfterHash: domain.SuiteHash(suite),
				Change: []domain.Change{
					{
						Target: domain.TargetAnalysis,
						Reason: "updated documentation description",
						After:  "New documentation description",
					},
				},
			},
		},
	}

	filtered, sel, err := SelectLatestUpdateScenarios(suite, log)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sel.ScenarioIDs) != 0 {
		t.Errorf("ScenarioIDs = %v, want empty", sel.ScenarioIDs)
	}
	if len(filtered.Scenarios) != 0 {
		t.Errorf("filtered scenarios = %v, want empty", filtered.Scenarios)
	}
}

// Test 7: Log chọn ID không tồn tại trong current suite → error consistency.
func TestSelectLatestUpdateScenarios_NonExistentScenarioID_ConsistencyError(t *testing.T) {
	suite := sampleSuiteForSelection()
	log := &domain.LogFile{
		Update: []domain.UpdateEntry{
			{
				N:         3,
				AfterHash: domain.SuiteHash(suite),
				Create: &domain.CreateSet{
					Scenarios: []domain.Scenario{{ID: "ghost-scenario"}},
				},
			},
		},
	}

	_, _, err := SelectLatestUpdateScenarios(suite, log)
	if err == nil {
		t.Fatal("expected consistency error, got nil")
	}
	if !strings.Contains(err.Error(), "ghost-scenario") || !strings.Contains(err.Error(), "not found in suite") {
		t.Errorf("error = %v, want mentioning ghost-scenario not found in suite", err)
	}
}

func TestSelectLatestUpdateScenarios_UnsortedUpdatesPicksHighestN(t *testing.T) {
	suite := sampleSuiteForSelection()
	// Update list is out of order: N=3 comes before N=5, then N=1
	log := &domain.LogFile{
		Update: []domain.UpdateEntry{
			{
				N:         3,
				AfterHash: "stalehash3333",
				Create: &domain.CreateSet{
					Scenarios: []domain.Scenario{{ID: "sc-1"}},
				},
			},
			{
				N:         5,
				AfterHash: domain.SuiteHash(suite),
				Create: &domain.CreateSet{
					Scenarios: []domain.Scenario{{ID: "sc-3"}},
				},
			},
			{
				N:         1,
				AfterHash: "stalehash1111",
				Create: &domain.CreateSet{
					Scenarios: []domain.Scenario{{ID: "sc-2"}},
				},
			},
		},
	}

	filtered, sel, err := SelectLatestUpdateScenarios(suite, log)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sel.UpdateN != 5 {
		t.Errorf("UpdateN = %d, want 5 (highest N)", sel.UpdateN)
	}
	wantIDs := []string{"sc-3"}
	if !reflect.DeepEqual(sel.ScenarioIDs, wantIDs) {
		t.Errorf("ScenarioIDs = %v, want %v", sel.ScenarioIDs, wantIDs)
	}
	if len(filtered.Scenarios) != 1 || filtered.Scenarios[0].ID != "sc-3" {
		t.Errorf("filtered scenarios = %v, want only sc-3", filtered.Scenarios)
	}
}

func TestSelectLatestUpdateScenarios_NoUpdates_SuccessWithZeroScenarios(t *testing.T) {
	suite := sampleSuiteForSelection()
	log := &domain.LogFile{
		Update: nil,
	}

	filtered, sel, err := SelectLatestUpdateScenarios(suite, log)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sel.UpdateN != 0 || len(sel.ScenarioIDs) != 0 {
		t.Errorf("selection = %#v, want 0 scenarios", sel)
	}
	if len(filtered.Scenarios) != 0 {
		t.Errorf("filtered scenarios len = %d, want 0", len(filtered.Scenarios))
	}
}

func TestSelectLatestUpdateScenarios_InvalidSetupDAG_ReturnsError(t *testing.T) {
	suite := sampleSuiteForSelection()
	suiteHashBefore := domain.SuiteHash(suite)

	// Introduce a broken setup dependency on sc-1
	suite.Setup["setupB"] = domain.Setup{
		Use: []string{"nonExistentSetup"},
	}
	// Recompute hash for broken suite so hash check passes and we test DAG resolution
	suiteHashBroken := domain.SuiteHash(suite)
	_ = suiteHashBefore

	log := &domain.LogFile{
		Update: []domain.UpdateEntry{
			{
				N:         1,
				AfterHash: suiteHashBroken,
				Change: []domain.Change{
					{
						Target: domain.TargetSetup,
						ID:     "setupA",
						After:  map[string]any{},
					},
				},
			},
		},
	}

	_, _, err := SelectLatestUpdateScenarios(suite, log)
	if err == nil {
		t.Fatal("expected DAG error for broken setup dependency, got nil")
	}
	if !strings.Contains(err.Error(), "nonExistentSetup") {
		t.Errorf("error = %v, want DAG error mentioning nonExistentSetup", err)
	}
}

func TestSelectLatestUpdateScenarios_AfterHashMismatch_ReturnsError(t *testing.T) {
	suite := sampleSuiteForSelection()
	log := &domain.LogFile{
		Update: []domain.UpdateEntry{
			{
				N:         4,
				AfterHash: "badhash12345", // Intentionally wrong hash
				Change: []domain.Change{
					{
						Target: domain.TargetScenario,
						ID:     "sc-1",
						After:  map[string]any{},
					},
				},
			},
		},
	}

	_, _, err := SelectLatestUpdateScenarios(suite, log)
	if err == nil {
		t.Fatal("expected error on after_hash mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "suite hash mismatch") {
		t.Errorf("error = %v, want 'suite hash mismatch'", err)
	}
}

func TestSelectLatestUpdateScenarios_MissingAfterHash_ReturnsError(t *testing.T) {
	suite := sampleSuiteForSelection()
	log := &domain.LogFile{
		Update: []domain.UpdateEntry{
			{
				N:         1,
				AfterHash: "", // Missing AfterHash
				Change: []domain.Change{
					{
						Target: domain.TargetScenario,
						ID:     "sc-1",
						After:  map[string]any{},
					},
				},
			},
		},
	}

	_, _, err := SelectLatestUpdateScenarios(suite, log)
	if err == nil {
		t.Fatal("expected error on missing after_hash, got nil")
	}
	if !strings.Contains(err.Error(), "missing after_hash") {
		t.Errorf("error = %v, want 'missing after_hash'", err)
	}
}
