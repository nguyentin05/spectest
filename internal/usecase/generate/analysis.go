package generate

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/nguyentin05/spectest/internal/domain"
)

// ============================================================================
// Assumptions
// ============================================================================

type assumption struct {
	Topic      string   `json:"topic"`
	Assumption string   `json:"assumption"`
	Reason     string   `json:"reason"`
	Evidence   []string `json:"evidence"`
}

// validateAssumptions checks shape and, when requirements exist, that every
// assumption quotes them verbatim. Without requirements evidence is optional.
func validateAssumptions(as []assumption, reqNorm string, hasReq bool) []error {
	var errs []error
	for i, a := range as {
		if strings.TrimSpace(a.Topic) == "" {
			errs = append(errs, fmt.Errorf("assumptions[%d]: topic is empty", i))
		}
		if strings.TrimSpace(a.Assumption) == "" {
			errs = append(errs, fmt.Errorf("assumptions[%d]: assumption is empty", i))
		}
		if !hasReq {
			continue
		}
		if len(a.Evidence) == 0 {
			errs = append(errs, fmt.Errorf("assumptions[%d]: evidence is required", i))
		}
		for _, e := range a.Evidence {
			if !containsEvidence(reqNorm, e) {
				errs = append(errs, fmt.Errorf("assumptions[%d]: evidence %q is not verbatim from requirements", i, truncate(e, 60)))
			}
		}
	}
	return errs
}

// cleanAssumptions normalizes whitespace, dedupes by (topic, assumption) and
// sorts. Evidence is dropped when there are no requirements to quote.
func cleanAssumptions(as []assumption, hasReq bool) []assumption {
	seen := map[string]struct{}{}
	out := make([]assumption, 0, len(as))
	for _, a := range as {
		c := assumption{
			Topic:      normalizeEvidence(a.Topic),
			Assumption: normalizeEvidence(a.Assumption),
			Reason:     normalizeEvidence(a.Reason),
		}
		if hasReq {
			for _, e := range a.Evidence {
				if e = normalizeEvidence(e); e != "" {
					c.Evidence = append(c.Evidence, e)
				}
			}
		}
		k := c.Topic + "\x00" + c.Assumption
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Topic != out[j].Topic {
			return out[i].Topic < out[j].Topic
		}
		return out[i].Assumption < out[j].Assumption
	})
	return out
}

// ============================================================================
// Code-managed blocks inside analysis
// ============================================================================

const (
	warningsStart    = "<!-- spectest:warnings:start -->"
	warningsEnd      = "<!-- spectest:warnings:end -->"
	assumptionsStart = "<!-- spectest:assumptions:start -->"
	assumptionsEnd   = "<!-- spectest:assumptions:end -->"
)

func blockRegexp(start, end string) *regexp.Regexp {
	return regexp.MustCompile(`(?s)\n*` + regexp.QuoteMeta(start) + `.*?` + regexp.QuoteMeta(end) + `\n*`)
}

var (
	warningsBlockRe    = blockRegexp(warningsStart, warningsEnd)
	assumptionsBlockRe = blockRegexp(assumptionsStart, assumptionsEnd)
)

// stripManagedBlocks removes both code-managed blocks. Manual-edit detection
// must compare analysis only after stripping them.
func stripManagedBlocks(analysis string) string {
	analysis = warningsBlockRe.ReplaceAllString(analysis, "")
	analysis = assumptionsBlockRe.ReplaceAllString(analysis, "")
	return strings.TrimRight(analysis, " \t\r\n")
}

// applyManagedBlocks replaces the code-managed blocks at the end of analysis:
// warnings first, then assumptions. An empty list removes its block.
func applyManagedBlocks(analysis string, ws []sourceWarning, as []assumption) string {
	var b strings.Builder
	b.WriteString(stripManagedBlocks(analysis))

	if len(ws) > 0 {
		sorted := append([]sourceWarning(nil), ws...)
		sort.SliceStable(sorted, func(i, j int) bool {
			if sorted[i].Code != sorted[j].Code {
				return sorted[i].Code < sorted[j].Code
			}
			return sorted[i].Key < sorted[j].Key
		})
		b.WriteString("\n\n" + warningsStart + "\n## Source warnings\n")
		for _, w := range sorted {
			b.WriteString("- ")
			b.WriteString(w.Line)
			b.WriteString("\n")
		}
		b.WriteString(warningsEnd)
	}
	if len(as) > 0 {
		b.WriteString("\n\n" + assumptionsStart + "\n## Assumptions\n")
		for _, a := range as {
			line := fmt.Sprintf("- `[%s]`: %s", strings.ReplaceAll(a.Topic, "`", "'"), a.Assumption)
			if a.Reason != "" {
				line += " — " + a.Reason
			}
			if len(a.Evidence) > 0 {
				quoted := make([]string, len(a.Evidence))
				for i, e := range a.Evidence {
					quoted[i] = `"` + e + `"`
				}
				line += " (evidence: " + strings.Join(quoted, "; ") + ")"
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString(assumptionsEnd)
	}
	return b.String()
}

// checkManagedBlocks reports structurally broken markers (unbalanced,
// repeated or reversed). Contents are not checked.
func checkManagedBlocks(analysis string) []error {
	var errs []error
	for _, p := range [][2]string{{warningsStart, warningsEnd}, {assumptionsStart, assumptionsEnd}} {
		s, e := strings.Count(analysis, p[0]), strings.Count(analysis, p[1])
		switch {
		case s != e || s > 1:
			errs = append(errs, fmt.Errorf("analysis has unbalanced or repeated %s marker", p[0]))
		case s == 1 && strings.Index(analysis, p[0]) > strings.Index(analysis, p[1]):
			errs = append(errs, fmt.Errorf("analysis has %s after its end marker", p[0]))
		}
	}
	return errs
}

// ============================================================================
// Analysis stage
// ============================================================================

type analysisOut struct {
	Analysis string `json:"analysis"`
}

type analysisInput struct {
	src       *domain.SourceSet
	plan      *fullPlan
	scenarios []domain.Scenario
	notes     []note
	warnings  []sourceWarning
	groups    []apiGroup
}

// writeAnalysis asks the LLM for the narrative, then appends the code-managed
// warning and assumption blocks. The narrative is written last so it describes
// the suite that actually exists.
func (g *Generator) writeAnalysis(ctx context.Context, ai analysisInput) (string, error) {
	system, user, err := buildAnalysisPrompt(ai)
	if err != nil {
		return "", fmt.Errorf("build prompt: %w", err)
	}
	out, err := callValidated(ctx, g.llm, "analysis", system, user, func(v *analysisOut) error {
		if strings.TrimSpace(v.Analysis) == "" {
			return errors.New("analysis is empty")
		}
		if strings.Contains(v.Analysis, "<!-- spectest:") {
			return errors.New("analysis must not contain spectest markers; they are added by code")
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return applyManagedBlocks(out.Analysis, ai.warnings, ai.plan.Assumptions), nil
}
