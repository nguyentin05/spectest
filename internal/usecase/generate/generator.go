package generate

import (
	"context"
	"fmt"
	"sort"

	"github.com/nguyentin05/spectest/internal/domain"
)

// Generator orchestrates the v3 generate pipeline.
type Generator struct {
	llm     domain.LLMService
	sources domain.SourceLoader
	store   domain.SuiteStore
	clock   domain.Clock
}

func NewGenerator(
	llm domain.LLMService,
	sources domain.SourceLoader,
	store domain.SuiteStore,
	clock domain.Clock,
) *Generator {
	return &Generator{
		llm:     llm,
		sources: sources,
		store:   store,
		clock:   clock,
	}
}

// Request is the input to Execute.
type Request struct {
	Mode             Mode
	APIs             []domain.APIInput
	RequirementsPath string
	OutputDir        string
	DryRun           bool
	Concurrency      int
}

// Result is what the CLI prints.
type Result struct {
	Mode           Mode
	UpdateN        int
	SuiteChanged   bool
	APIDiff        APIDiff
	Impact         domain.UpdateImpact
	PlannedActions []UpdateAction
	LLMUsed        bool
	Input          domain.InputMode
	Servers        []string
	APITotal       int
	Warnings       []string
	SuitePath      string
	LogPath        string
	NoFilesWritten bool
}

// Execute dispatches to the requested mode pipeline.
func (g *Generator) Execute(ctx context.Context, req Request) (*Result, error) {
	mode := req.Mode
	if mode == "" {
		mode = ModeInitial
	}

	switch mode {
	case ModeInitial:
		return g.executeInitial(ctx, req)
	case ModeUpdate:
		return g.executeUpdate(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported generate mode %q", mode)
	}
}

// executeInitial runs the full initial generation pipeline.
func (g *Generator) executeInitial(ctx context.Context, req Request) (*Result, error) {
	// ⓪ + ① CHỌN NGUỒN + PARSE OPENAPI
	src, err := g.resolveSources(ctx, req)
	if err != nil {
		return nil, err
	}

	// ② CHỮ KÝ API + NHÓM (Server + Resource)
	groups := buildGroups(src)

	// ③ NOTES + API_MENTIONS (Chỉ chạy khi có --req)
	var notesRes *notesResult
	if src.Input == domain.InputBoth {
		notesRes, err = g.extractNotes(ctx, src, groups, req.Concurrency)
		if err != nil {
			return nil, err
		}
	}

	// ④ BẢN ĐỒ API (Chuẩn bị input cho plan)
	in := newPlanInput(src, groups, notesRes)

	// ⑤ GLOBAL PLAN (Seed, Common setups, Assumptions)
	gp, globalWarns, err := g.planGlobal(ctx, in)
	if err != nil {
		return nil, err
	}

	// ⑥ OUTLINE THEO NHÓM (Chạy song song theo group)
	c := newOutlineCtx(in, gp)
	plan, err := g.planOutline(ctx, c, req.Concurrency)
	if err != nil {
		return nil, err
	}

	// ⑦ VIẾT SETUP & SCENARIO (Setup tuần tự, Scenario song song theo lô)
	sc := newSynthCtx(g, src, plan, groups, promptInfo{
		Notes:       promptNotesFrom(in.Notes),
		Assumptions: gp.Assumptions,
		Warnings:    in.warningMessages(),
	})

	// Setup chạy tuần tự (đóng băng catalog)
	setups, scopes, err := sc.synthesizeSetups(ctx)
	if err != nil {
		return nil, err
	}

	// Scenario chạy song song
	scenarios, err := sc.synthesizeScenarios(ctx, req.Concurrency, plan.Scenarios, setups, scopes, nil)
	if err != nil {
		return nil, err
	}

	// ⑧ KIỂM TRA COVERAGE (Repair entity nếu thiếu API)
	scenarios, err = sc.repairCoverage(ctx, req.Concurrency, scenarios, setups, scopes)
	if err != nil {
		return nil, err
	}

	// ⑨ ANALYSIS (Sinh narrative + Code quản lý block warnings/assumptions)
	var sw []sourceWarning
	if notesRes != nil {
		sw = notesRes.SourceWarnings
	}
	ai := analysisInput{
		src:       src,
		plan:      plan,
		scenarios: scenarios,
		notes:     in.Notes,
		warnings:  sw,
		groups:    groups,
	}
	analysisStr, err := g.writeAnalysis(ctx, ai)
	if err != nil {
		return nil, err
	}

	// ⑩ GHÉP FILE & VALIDATE
	suite := assembleSuite(src, plan, setups, scenarios, analysisStr)
	vr := validateSuite(suite, src, validationOptions{
		RequireAnalysis:  true,
		RequiredCoverage: allAPIKeys(src),
		AllowStaleAPIs:   false,
		WarnUncovered:    true,
	})
	if err := vr.Err(); err != nil {
		return nil, fmt.Errorf("validate suite: %w", err)
	}

	// Chuẩn bị file set (tạo base log + track)
	var mentions []domain.APIMention
	if notesRes != nil {
		mentions = notesRes.Mentions
	}
	files := g.buildInitialFileSet(src, suite, mentions)

	// ⑪ PERSIST (Atomic write)
	writeOpts := domain.WriteOptions{Suite: true, Log: true, Track: false, Mode: string(ModeInitial)}
	if err := g.persist(ctx, req, files, writeOpts); err != nil {
		return nil, fmt.Errorf("persist: %w", err)
	}

	// Gom tất cả warning để in ra CLI
	var allWarns []string
	if notesRes != nil {
		for _, w := range notesRes.SourceWarnings {
			allWarns = append(allWarns, w.Message)
		}
		allWarns = append(allWarns, notesRes.Warnings...)
	}
	allWarns = append(allWarns, globalWarns...)
	allWarns = append(allWarns, plan.Warnings...)
	allWarns = append(allWarns, vr.Warnings...)

	// Dedupe & Sort warnings
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
