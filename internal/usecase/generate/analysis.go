package generate

import (
	"fmt"
	"regexp"
	"strings"
)

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
