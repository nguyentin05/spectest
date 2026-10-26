package generate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nguyentin05/spectest/internal/domain"
)

// executeUpdate runs the update pipeline on an existing test suite and history log.
func (g *Generator) executeUpdate(ctx context.Context, req Request) (*Result, error) {
	// 1. Load sources
	src, err := g.resolveSources(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("load sources: %w", err)
	}

	// 2. Load existing suite files
	files, err := g.store.Load(ctx, req.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("load existing suite: %w", err)
	}
	if files == nil || files.Suite == nil || files.Log == nil {
		return nil, errors.New("cannot update: spectest.yaml and spectest.log.yaml must both exist in output directory")
	}

	// Verify requirements compatibility
	if req.RequirementsPath != "" {
		if files.Suite.Input != domain.InputBoth {
			return nil, fmt.Errorf("requirements updates are not supported: existing suite was generated with input %q", files.Suite.Input)
		}
		if src.Requirements == nil {
			return nil, fmt.Errorf("requirements file %q is empty", req.RequirementsPath)
		}
		newReqHash := requirementsHash(src.Requirements.Raw)
		var prevReqHash string
		if files.Log.Base.RequirementsHash != "" {
			prevReqHash = files.Log.Base.RequirementsHash
		}
		if prevReqHash != "" && newReqHash != prevReqHash {
			return nil, errors.New("requirements changes are not supported in update mode; requirements file has changed")
		}
	}

	// 3. Integrity verification: check suite hash against latest log AfterHash
	var expectedHash string
	latestN := 0

	if len(files.Log.Update) > 0 {
		// Verify sequence
		for i, u := range files.Log.Update {
			expectedN := i + 1
			if u.N != expectedN {
				return nil, fmt.Errorf("invalid history log: update at index %d has N=%d, want N=%d", i, u.N, expectedN)
			}
			if u.N > latestN {
				latestN = u.N
				expectedHash = u.AfterHash
			}
		}
	} else {
		expectedHash = files.Log.Base.AfterHash
	}

	if expectedHash == "" {
		return nil, errors.New("history log has empty after_hash")
	}

	currentHash := domain.SuiteHash(files.Suite)
	if currentHash != expectedHash {
		return nil, fmt.Errorf("suite hash mismatch: spectest.yaml hash %q does not match latest log after_hash %q (suite may have been modified manually)", currentHash, expectedHash)
	}

	// 4. Reconstruct API catalog from base and patches
	oldCatalog, err := ReconstructCatalog(files.Log)
	if err != nil {
		return nil, fmt.Errorf("reconstruct api catalog: %w", err)
	}

	// 5. Diff catalog against current sources
	diff := DiffCatalog(oldCatalog, src)
	if diff.IsEmpty() {
		// ModeNoop: no diff, no files written, no log entries
		return &Result{
			Mode:           ModeNoop,
			Input:          src.Input,
			Servers:        src.Servers,
			APITotal:       len(src.APIs),
			SuitePath:      joinDisplayPath(req.OutputDir, "spectest.yaml"),
			LogPath:        joinDisplayPath(req.OutputDir, "spectest.log.yaml"),
			NoFilesWritten: req.DryRun,
		}, nil
	}

	// Build map of current APIs
	currentAPIs := make(map[string]domain.API, len(src.APIs))
	for _, a := range src.APIs {
		currentAPIs[a.Key] = a
	}

	// Build map of all known APIs (including historical ones for indexing existing suite calls)
	knownAPIs := make(map[string]domain.API, len(oldCatalog)+len(src.APIs))
	for k, oldState := range oldCatalog {
		if api, ok := apiFromKey(k); ok {
			api.Sig = oldState.Sig
			knownAPIs[k] = api
		}
	}
	for _, a := range src.APIs {
		knownAPIs[a.Key] = a
	}

	// 6. Build impact index and calculate affected entities
	index, err := BuildImpactIndex(files.Suite, knownAPIs)
	if err != nil {
		return nil, fmt.Errorf("build impact index: %w", err)
	}

	impact, err := ComputeImpact(files.Suite, diff, index)
	if err != nil {
		return nil, fmt.Errorf("compute impact: %w", err)
	}

	// 7. Plan actions
	actions, err := BuildUpdatePlan(files.Suite, diff, impact, index, currentAPIs)
	if err != nil {
		return nil, fmt.Errorf("build update plan: %w", err)
	}

	// 8. Check LLM requirement
	llmUsed := false
	for _, act := range actions {
		if act.RequiresLLM {
			if g.llm == nil {
				target := act.SetupName
				if target == "" {
					target = act.ScenarioID
				}
				if target == "" && len(act.APIKeys) > 0 {
					target = act.APIKeys[0]
				}
				return nil, fmt.Errorf("update requires an LLM to %s %q; configure an LLM provider and retry", act.Kind, target)
			}
			llmUsed = true
		}
	}

	// If dry-run, return planned outcome without modifying files or calling LLM
	if req.DryRun {
		return &Result{
			Mode:           ModeUpdate,
			UpdateN:        latestN + 1,
			SuiteChanged:   len(actions) > 0,
			APIDiff:        diff,
			Impact:         impact,
			PlannedActions: actions,
			LLMUsed:        llmUsed,
			Input:          src.Input,
			Servers:        src.Servers,
			APITotal:       len(src.APIs),
			SuitePath:      joinDisplayPath(req.OutputDir, "spectest.yaml"),
			LogPath:        joinDisplayPath(req.OutputDir, "spectest.log.yaml"),
			NoFilesWritten: true,
		}, nil
	}

	// 9. Apply plan to suite
	updatedSuite := cloneSuite(files.Suite)
	suiteChanged := false
	var changes []domain.Change
	var createdScenarios []domain.Scenario

	for _, act := range actions {
		switch act.Kind {
		case ActionAddToolSkip:
			for i := range updatedSuite.Scenarios {
				if updatedSuite.Scenarios[i].ID == act.ScenarioID {
					if updatedSuite.Scenarios[i].Skip != act.Reason {
						updatedSuite.Scenarios[i].Skip = act.Reason
						suiteChanged = true
					}
					changes = append(changes, domain.Change{
						Target: domain.TargetScenario,
						ID:     act.ScenarioID,
						Reason: act.Reason,
						After:  map[string]any{"skip": act.Reason},
					})
					break
				}
			}
		case ActionRemoveToolSkip:
			for i := range updatedSuite.Scenarios {
				if updatedSuite.Scenarios[i].ID == act.ScenarioID {
					if updatedSuite.Scenarios[i].Skip != "" {
						updatedSuite.Scenarios[i].Skip = ""
						suiteChanged = true
					}
					changes = append(changes, domain.Change{
						Target: domain.TargetScenario,
						ID:     act.ScenarioID,
						Reason: act.Reason,
						After:  map[string]any{"skip": nil},
					})
					break
				}
			}
		case ActionRepairSetup:
			repaired, err := g.repairSetup(ctx, updatedSuite, src, act)
			if err != nil {
				return nil, fmt.Errorf("repair setup %q: %w", act.SetupName, err)
			}
			orig := updatedSuite.Setup[act.SetupName]
			if updatedSuite.Setup == nil {
				updatedSuite.Setup = make(map[string]domain.Setup)
			}
			updatedSuite.Setup[act.SetupName] = repaired
			suiteChanged = true
			changes = append(changes, domain.Change{
				Target: domain.TargetSetup,
				ID:     act.SetupName,
				Reason: act.Reason,
				Before: map[string]any{"steps": orig.Steps, "vars": orig.Vars},
				After:  map[string]any{"steps": repaired.Steps, "vars": repaired.Vars},
			})
		case ActionRepairScenario:
			repaired, err := g.repairScenario(ctx, updatedSuite, src, act)
			if err != nil {
				return nil, fmt.Errorf("repair scenario %q: %w", act.ScenarioID, err)
			}
			for i := range updatedSuite.Scenarios {
				if updatedSuite.Scenarios[i].ID == act.ScenarioID {
					orig := updatedSuite.Scenarios[i]
					updatedSuite.Scenarios[i] = repaired
					suiteChanged = true
					changes = append(changes, domain.Change{
						Target: domain.TargetScenario,
						ID:     act.ScenarioID,
						Reason: act.Reason,
						Before: map[string]any{"steps": orig.Steps},
						After:  map[string]any{"steps": repaired.Steps},
					})
					break
				}
			}
		case ActionCreateScenario:
			newScenarios, err := g.createScenarios(ctx, updatedSuite, src, act.APIKeys)
			if err != nil {
				return nil, fmt.Errorf("create scenarios for %v: %w", act.APIKeys, err)
			}
			for _, sc := range newScenarios {
				updatedSuite.Scenarios = append(updatedSuite.Scenarios, sc)
				createdScenarios = append(createdScenarios, sc)
				impact.Scenarios = append(impact.Scenarios, sc.ID)
				suiteChanged = true
			}
		}
	}

	impact.Scenarios = dedupeAndSort(impact.Scenarios)
	impact.Setups = dedupeAndSort(impact.Setups)

	// Update servers and APIs
	updatedSuite.Servers = src.Servers
	updatedSuite.APIs = allAPIKeys(src)

	// Validate updated suite
	validationOpt := validationOptions{
		RequireAnalysis:  false,
		RequiredCoverage: nil,
		AllowStaleAPIs:   true, // tool-skipped scenarios are allowed to hold stale APIs
		WarnUncovered:    false,
	}
	vr := validateSuite(updatedSuite, src, validationOpt)
	if err := vr.Err(); err != nil {
		return nil, fmt.Errorf("validate updated suite: %w", err)
	}

	// 10. Compute AfterHash
	afterHash := domain.SuiteHash(updatedSuite)

	// 11. Create UpdateEntry
	var createSet *domain.CreateSet
	if len(createdScenarios) > 0 {
		createSet = &domain.CreateSet{
			Scenarios: createdScenarios,
		}
	}
	updateEntry := domain.UpdateEntry{
		N:            latestN + 1,
		At:           g.clock.Now().Format(time.RFC3339),
		By:           domain.ByAI,
		Trigger:      &domain.Trigger{OpenAPI: buildOpenAPITrigger(diff)},
		CatalogPatch: buildCatalogPatch(diff),
		Impact:       impact,
		Create:       createSet,
		Change:       changes,
		AfterHash:    afterHash,
	}

	// Append to log
	updatedLog := &domain.LogFile{
		Base:   files.Log.Base,
		Update: append(append([]domain.UpdateEntry{}, files.Log.Update...), updateEntry),
	}

	// 12. Persist updated suite and log
	updatedFiles := &domain.FileSet{
		Suite: updatedSuite,
		Log:   updatedLog,
		Track: files.Track,
	}
	writeOpts := domain.WriteOptions{
		Suite: true,
		Log:   true,
		Track: false, // 2-file architecture: never write track!
		Mode:  string(ModeUpdate),
	}

	if err := g.persist(ctx, req, updatedFiles, writeOpts); err != nil {
		return nil, fmt.Errorf("persist update: %w", err)
	}

	return &Result{
		Mode:           ModeUpdate,
		UpdateN:        updateEntry.N,
		SuiteChanged:   suiteChanged,
		APIDiff:        diff,
		Impact:         impact,
		PlannedActions: actions,
		LLMUsed:        llmUsed,
		Input:          src.Input,
		Servers:        src.Servers,
		APITotal:       len(src.APIs),
		SuitePath:      joinDisplayPath(req.OutputDir, "spectest.yaml"),
		LogPath:        joinDisplayPath(req.OutputDir, "spectest.log.yaml"),
		NoFilesWritten: false,
	}, nil
}
