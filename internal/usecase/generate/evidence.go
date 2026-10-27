package generate

import (
	"strings"
)

// normalizeEvidence is the single canonical form used for every evidence
// check: line endings, NBSP and other whitespace collapse to one ASCII space,
// and both ends are trimmed.
func normalizeEvidence(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
