package generate

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"github.com/nguyentin05/spectest/internal/domain"
)

// executeInitial runs the 6-stage initial generation pipeline.
func (g *Generator) executeInitial(ctx context.Context, req Request) (*Result, error) {
	// 1. Load and normalize sources
	src, err := g.resolveSources(ctx, req)
	if err != nil {
		return nil, err
	}

	facts := buildFacts(src)

	// 2. Stage 1: Contract Reconciliation
	if req.Progress != nil {
		req.Progress(0, 0, "Stage 1/4: Analyzing contract reconciliation...")
	}
	reqText := ""
	if src.Requirements != nil {
		reqText = src.Requirements.Raw
	}
	contractRes, err := analyzeContract(ctx, g.llm, facts, reqText)
	if err != nil {
		return nil, err
	}

	// 3. Stage 2: Seed & Setup Planning
	if req.Progress != nil {
		req.Progress(0, 0, "Stage 2/4: Planning seed data and setup procedures...")
	}
	apis := make(map[string]domain.API, len(src.APIs))
	for _, a := range src.APIs {
		apis[a.Key] = a
	}
	serverSet := make(map[string]struct{}, len(src.Servers))
	for _, s := range src.Servers {
		serverSet[s] = struct{}{}
	}

	setups, seeds, unresolved, stage2Assumptions, setupWarns, err := planSetups(
		ctx, g.llm, facts, contractRes.GlobalNotes, reqText, apis, serverSet,
	)
	if err != nil {
		return nil, err
	}

	// 4. Stage 3: Scenario Generation per API (Bounded Parallelism)
	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = 2
	}
	if req.Progress != nil {
		req.Progress(0, 0, fmt.Sprintf("Stage 3/4: Generating test scenarios across %d APIs (concurrency: %d)...", len(src.APIs), concurrency))
	}
	merged, scenarioWarns, err := generateScenarios(
		ctx, g.llm, facts, setups, seeds, contractRes.GlobalNotes, concurrency, apis, serverSet, req.Progress,
	)
	if err != nil {
		return nil, err
	}

	// 5. Stage 4: Narrative Analysis & Managed Blocks
	if req.Progress != nil {
		req.Progress(0, 0, "Stage 4/4: Writing narrative analysis and audit logs...")
	}
	setupNames := make([]string, 0, len(setups))
	for k := range setups {
		setupNames = append(setupNames, k)
	}
	sort.Strings(setupNames)

	stats := map[string]any{
		"total_apis":      len(src.APIs),
		"covered_apis":    merged.CoveredAPIs,
		"total_scenarios": len(merged.Scenarios),
	}

	reviewNotes := map[string]any{
		"overrides":    contractRes.Overrides,
		"ambiguities":  contractRes.Ambiguities,
		"global_notes": contractRes.GlobalNotes,
	}

	narrative, stage4Assumptions, err := writeInitAnalysis(
		ctx,
		g.llm,
		stats,
		setupNames,
		unresolved,
		merged.SkippedCases,
		merged.NoScenarioReport,
		merged.InferredExpectations,
		reviewNotes,
		contractRes.RequirementsLanguage,
	)
	if err != nil {
		return nil, err
	}

	allAssumptions := append([]assumptionItem(nil), stage2Assumptions...)
	allAssumptions = append(allAssumptions, stage4Assumptions...)
	finalAssumptions := mergeAssumptionsWithInferences(allAssumptions, merged.InferredExpectations)

	var allWarns []string
	for _, o := range contractRes.Overrides {
		allWarns = append(allWarns, fmt.Sprintf("contract override: API %s: %s (openapi=%s, spec=%s)", o.API, o.Item, o.OpenAPI, o.Spec))
	}
	allWarns = append(allWarns, contractRes.Ambiguities...)
	allWarns = append(allWarns, setupWarns...)
	allWarns = append(allWarns, scenarioWarns...)
	for _, sc := range merged.SkippedCases {
		allWarns = append(allWarns, fmt.Sprintf("skipped case: %s: %s", sc.Case, sc.Reason))
	}
	for apiKey, reason := range merged.NoScenarioReport {
		allWarns = append(allWarns, fmt.Sprintf("API %s: no scenarios generated: %s", apiKey, reason))
	}

	fullAnalysis := applyInitManagedBlocks(narrative, allWarns, finalAssumptions)

	// 6. Assemble, Validate, and Persist
	suite := assembleSuite(src, seeds, setups, merged.Scenarios, fullAnalysis)

	vr := validateSuite(suite, src, validationOptions{
		RequireAnalysis:       true,
		RequiredCoverage:      nil,
		AllowStaleAPIs:        false,
		WarnUncovered:         true,
		AllowInferredStatuses: true,
	})
	if err := vr.Err(); err != nil {
		return nil, fmt.Errorf("validate suite: %w", err)
	}

	allWarns = append(allWarns, vr.Warnings...)

	files := g.buildInitialFileSet(src, suite)

	writeOpts := domain.WriteOptions{
		Suite: true,
		Log:   true,
		Track: false,
		Mode:  string(ModeInitial),
	}
	if err := g.persist(ctx, req, files, writeOpts); err != nil {
		return nil, fmt.Errorf("persist: %w", err)
	}

	seenWarn := make(map[string]struct{})
	var uniqueWarns []string
	for _, w := range allWarns {
		if _, ok := seenWarn[w]; !ok {
			seenWarn[w] = struct{}{}
			uniqueWarns = append(uniqueWarns, w)
		}
	}
	sort.Strings(uniqueWarns)

	return &Result{
		Mode:           ModeInitial,
		Input:          src.Input,
		Servers:        src.Servers,
		APITotal:       len(src.APIs),
		Warnings:       uniqueWarns,
		SuitePath:      joinDisplayPath(req.OutputDir, "spectest.yaml"),
		LogPath:        joinDisplayPath(req.OutputDir, "spectest.log.yaml"),
		NoFilesWritten: req.DryRun,
	}, nil
}

func assembleSuite(
	src *domain.SourceSet,
	seed map[string]domain.Seed,
	setups map[string]domain.Setup,
	scenarios []domain.Scenario,
	analysis string,
) *domain.Suite {
	apiKeys := make([]string, 0, len(src.APIs))
	for _, a := range src.APIs {
		apiKeys = append(apiKeys, a.Key)
	}
	sort.Strings(apiKeys)

	return &domain.Suite{
		Version:   1,
		Input:     src.Input,
		Servers:   slices.Clone(src.Servers),
		APIs:      apiKeys,
		Analysis:  analysis,
		Seed:      seed,
		Setup:     setups,
		Scenarios: scenarios,
	}
}
