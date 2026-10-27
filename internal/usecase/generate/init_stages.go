package generate

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"github.com/nguyentin05/spectest/internal/domain"
)

var initIDRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ============================================================================
// Stage 1: Contract Reconciliation
// ============================================================================

func analyzeContract(
	ctx context.Context,
	llm domain.LLMService,
	facts map[string]*apiFacts,
	reqText string,
) (*contractResult, error) {
	compactReq := reqText
	if utf8.RuneCountInString(reqText) > chunkMaxRunes {
		chunks := chunkRequirements(reqText)
		parts, err := runBounded(ctx, chunks, 4, func(c context.Context, _ int, ch reqChunk) ([]string, error) {
			sys := "You are a technical analyst extracting API rules and constraints from requirements."
			usr := "Extract concise business rules, constraints, authentication rules, and seed data hints from this chunk of requirements. Return JSON: {\"notes\": [\"...\"]}\n\nChunk:\n" + ch.Text
			type chunkNotesOut struct {
				Notes []string `json:"notes"`
			}
			out, callErr := callValidated[chunkNotesOut](c, llm, "requirements chunk", sys, usr, nil)
			if callErr != nil {
				return nil, callErr
			}
			return out.Notes, nil
		})
		if err == nil {
			var allNotes []string
			for _, p := range parts {
				allNotes = append(allNotes, p...)
			}
			compactReq = strings.Join(dedupeStrings(allNotes), "\n")
		}
	}

	system, user, err := buildContractPrompt(facts, compactReq)
	if err != nil {
		return nil, fmt.Errorf("build contract prompt: %w", err)
	}

	res, err := callValidated[contractResult](ctx, llm, "contract reconciliation", system, user, nil)
	if err != nil {
		return nil, fmt.Errorf("contract reconciliation: %w", err)
	}

	res.GlobalNotes = dedupeStrings(res.GlobalNotes)
	res.Ambiguities = dedupeStrings(res.Ambiguities)
	res.MissingInfo = dedupeStrings(res.MissingInfo)

	lang := strings.TrimSpace(res.RequirementsLanguage)
	if lang == "" || strings.EqualFold(lang, "unknown") {
		res.RequirementsLanguage = "English"
	} else {
		res.RequirementsLanguage = lang
	}

	return &res, nil
}

// ============================================================================
// Stage 2: Seed & Setup Planning
// ============================================================================

func planSetups(
	ctx context.Context,
	llm domain.LLMService,
	facts map[string]*apiFacts,
	globalNotes []string,
	reqText string,
	apis map[string]domain.API,
	serverSet map[string]struct{},
) (map[string]domain.Setup, map[string]domain.Seed, []unresolvedItem, []assumptionItem, []string, error) {
	system, user, err := buildInitSetupPrompt(facts, globalNotes, reqText)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("build setup prompt: %w", err)
	}

	res, err := callValidated[setupPlanResult](ctx, llm, "seed and setup", system, user, nil)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("seed and setup: %w", err)
	}

	// 1. Normalize seed groups and vars
	cleanSeeds, seedWarns := normalizeSeeds(res.Seed)

	// 2. Ground setups
	validSetups, groundUnresolved, groundWarns := groundSetups(res.Setup, facts, apis, serverSet, cleanSeeds)

	// 3. Merge unresolved
	var allUnresolved []unresolvedItem
	allUnresolved = append(allUnresolved, res.Unresolved...)
	allUnresolved = append(allUnresolved, groundUnresolved...)

	// 4. Clean assumptions
	cleanAssumptions := cleanAssumptionItems(res.Assumptions)

	var allWarns []string
	allWarns = append(allWarns, seedWarns...)
	allWarns = append(allWarns, groundWarns...)

	return validSetups, cleanSeeds, allUnresolved, cleanAssumptions, allWarns, nil
}

func normalizeSeeds(raw map[string]domain.Seed) (map[string]domain.Seed, []string) {
	out := make(map[string]domain.Seed)
	var warns []string

	if len(raw) == 0 {
		return out, warns
	}

	groups := make([]string, 0, len(raw))
	for g := range raw {
		groups = append(groups, g)
	}
	sort.Strings(groups)

	seenVars := make(map[string]string) // varName -> groupName

	for _, g := range groups {
		if !initIDRe.MatchString(g) {
			warns = append(warns, fmt.Sprintf("seed group %q dropped: name must match ^[a-z][a-z0-9_]*$", g))
			continue
		}
		seedGroup := raw[g]
		validVars := make(map[string]any)

		varKeys := make([]string, 0, len(seedGroup.Vars))
		for k := range seedGroup.Vars {
			varKeys = append(varKeys, k)
		}
		sort.Strings(varKeys)

		for _, k := range varKeys {
			v := seedGroup.Vars[k]
			strVal := fmt.Sprintf("%v", v)
			if strings.Contains(strVal, "{{") && strings.Contains(strVal, "}}") {
				warns = append(warns, fmt.Sprintf("seed var %q in group %q dropped: cannot contain template expressions", k, g))
				continue
			}
			if prevGroup, exists := seenVars[k]; exists {
				warns = append(warns, fmt.Sprintf("seed var %q in group %q dropped: collided with group %q", k, g, prevGroup))
				continue
			}
			seenVars[k] = g
			validVars[k] = v
		}

		if len(validVars) > 0 {
			out[g] = domain.Seed{Vars: validVars}
		}
	}

	return out, warns
}

// ============================================================================
// Stage 3: Scenario Generation per API
// ============================================================================

func generateScenarios(
	ctx context.Context,
	llm domain.LLMService,
	facts map[string]*apiFacts,
	setups map[string]domain.Setup,
	seed map[string]domain.Seed,
	globalNotes []string,
	concurrency int,
	apis map[string]domain.API,
	serverSet map[string]struct{},
	onProgress func(current, total int, message string),
) (*mergedOutcomes, []string, error) {
	keys := make([]string, 0, len(facts))
	for k := range facts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var completed int32
	total := len(keys)

	outcomes, errs := runAllBounded(ctx, keys, concurrency, func(c context.Context, _ int, apiKey string) (apiOutcome, error) {
		outcome, err := generateForAPI(c, llm, facts[apiKey], globalNotes, setups, seed, apis, serverSet)
		if onProgress != nil {
			n := atomic.AddInt32(&completed, 1)
			detail := fmt.Sprintf("%s (%d scenarios)", apiKey, len(outcome.Scenarios))
			if err != nil {
				detail = fmt.Sprintf("%s (error: %v)", apiKey, err)
			}
			onProgress(int(n), total, detail)
		}
		return outcome, err
	})

	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	var warns []string
	allFailed := len(keys) > 0
	for i, err := range errs {
		if err != nil {
			warns = append(warns, fmt.Sprintf("API %s: generation failed: %v", keys[i], err))
		} else {
			allFailed = false
		}
	}

	if allFailed {
		var errList []error
		for _, e := range errs {
			if e != nil {
				errList = append(errList, e)
			}
		}
		return nil, nil, fmt.Errorf("all %d scenario generation calls failed: %w", len(keys), errors.Join(errList...))
	}

	merged, mergeWarns := mergeOutcomes(outcomes)
	warns = append(warns, mergeWarns...)

	return merged, warns, nil
}

func generateForAPI(
	ctx context.Context,
	llm domain.LLMService,
	fact *apiFacts,
	globalNotes []string,
	setups map[string]domain.Setup,
	seed map[string]domain.Seed,
	apis map[string]domain.API,
	serverSet map[string]struct{},
) (apiOutcome, error) {
	system, user, err := buildInitScenarioPrompt(fact, globalNotes, setups, seed, nil)
	if err != nil {
		return apiOutcome{APIKey: fact.Key, Err: err}, err
	}

	res, err := callValidated[scenarioResult](ctx, llm, "scenario: "+fact.Key, system, user, nil)
	if err != nil {
		return apiOutcome{APIKey: fact.Key, Err: err}, err
	}

	outcome := apiOutcome{
		APIKey:       fact.Key,
		SkippedCases: res.SkippedCases,
	}
	if res.NoScenarioReason != nil && *res.NoScenarioReason != "" {
		outcome.NoScenarioReason = *res.NoScenarioReason
	}

	for _, sc := range res.Scenarios {
		gSc, inferred, gErr := groundScenario(sc, fact, apis, serverSet, setups, seed, res.InferredExpectations)
		if gErr != nil {
			continue
		}
		outcome.Scenarios = append(outcome.Scenarios, *gSc)
		outcome.InferredExpectations = append(outcome.InferredExpectations, inferred...)
	}

	if len(outcome.Scenarios) == 0 && outcome.NoScenarioReason == "" {
		outcome.NoScenarioReason = "No scenarios passed validation"
	}

	return outcome, nil
}

// ============================================================================
// Stage 4: Narrative Analysis & Managed Blocks
// ============================================================================

func writeInitAnalysis(
	ctx context.Context,
	llm domain.LLMService,
	stats map[string]any,
	setups []string,
	unresolved []unresolvedItem,
	skipped []skippedCase,
	noScenario map[string]string,
	inferred []inferredExpect,
	reviewNotes map[string]any,
	lang string,
) (string, []assumptionItem, error) {
	system, user, err := buildInitAnalysisPrompt(
		lang,
		stats,
		nil,
		setups,
		unresolved,
		skipped,
		noScenario,
		inferred,
		reviewNotes,
	)
	if err != nil {
		return "", nil, fmt.Errorf("build analysis prompt: %w", err)
	}

	res, err := callValidated[analysisResult](ctx, llm, "analysis", system, user, func(r analysisResult) error {
		if strings.TrimSpace(r.Analysis) == "" {
			return errors.New("empty analysis narrative")
		}
		if strings.Contains(r.Analysis, "<!-- spectest:") {
			return errors.New("analysis narrative must not contain spectest markers")
		}
		return nil
	})
	if err != nil {
		return "", nil, fmt.Errorf("analysis: %w", err)
	}

	narrative := stripManagedBlocks(res.Analysis)
	return narrative, res.Assumptions, nil
}

func mergeAssumptionsWithInferences(
	assumptions []assumptionItem,
	inferred []inferredExpect,
) []assumptionItem {
	clean := cleanAssumptionItems(assumptions)
	seen := make(map[string]struct{})
	for _, a := range clean {
		seen[strings.ToLower(a.Area+":"+a.Statement)] = struct{}{}
	}

	for _, inf := range inferred {
		stmt := fmt.Sprintf("Scenario %s expects %d (%s)", inf.ScenarioID, inf.Expect, inf.Basis)
		key := strings.ToLower("inferred expectation:" + stmt)
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			clean = append(clean, assumptionItem{
				Area:      "inferred expectation",
				Statement: stmt,
				Reason:    inf.Reason,
			})
		}
	}
	return clean
}

func applyInitManagedBlocks(
	analysis string,
	warnings []string,
	assumptions []assumptionItem,
) string {
	var b strings.Builder
	b.WriteString(stripManagedBlocks(analysis))

	if len(warnings) > 0 {
		sorted := append([]string(nil), warnings...)
		sort.Strings(sorted)
		b.WriteString("\n\n" + warningsStart + "\n## Generation warnings\n")
		for _, w := range sorted {
			b.WriteString("- ")
			b.WriteString(w)
			b.WriteString("\n")
		}
		b.WriteString(warningsEnd)
	}

	if len(assumptions) > 0 {
		b.WriteString("\n\n" + assumptionsStart + "\n## Assumptions\n")
		for _, a := range assumptions {
			line := fmt.Sprintf("- `[%s]`: %s", strings.ReplaceAll(a.Area, "`", "'"), a.Statement)
			if a.Reason != "" {
				line += " — " + a.Reason
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString(assumptionsEnd)
	}

	return b.String()
}

func cleanAssumptionItems(items []assumptionItem) []assumptionItem {
	seen := make(map[string]struct{})
	out := make([]assumptionItem, 0, len(items))
	for _, it := range items {
		area := strings.TrimSpace(it.Area)
		stmt := strings.TrimSpace(it.Statement)
		reason := strings.TrimSpace(it.Reason)
		if area == "" || stmt == "" {
			continue
		}
		key := area + "\x00" + stmt
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, assumptionItem{
			Area:      area,
			Statement: stmt,
			Reason:    reason,
		})
	}
	return out
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, s := range in {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
