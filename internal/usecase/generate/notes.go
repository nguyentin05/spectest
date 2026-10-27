package generate

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ============================================================================
// Chunking
// ============================================================================

const chunkMaxRunes = 12_000

type reqChunk struct {
	Index int
	Text  string
	norm  string // normalizeEvidence(Text), used for evidence checks
}

var blankLineRe = regexp.MustCompile(`\n[ \t]*\n`)

// chunkRequirements splits raw requirements into chunks of at most
// chunkMaxRunes runes. Paragraphs are packed greedily, a Markdown heading
// prefers to start a new chunk once the current one is half full, and adjacent
// chunks overlap by one paragraph so no sentence is lost at a boundary. It is
// deterministic and never calls the LLM.
func chunkRequirements(raw string) []reqChunk {
	text := strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")

	var paras []string
	for _, p := range blankLineRe.Split(text, -1) {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if utf8.RuneCountInString(p) > chunkMaxRunes {
			paras = append(paras, splitLong(p, chunkMaxRunes)...)
		} else {
			paras = append(paras, p)
		}
	}

	var chunks []reqChunk
	emit := func(ps []string) {
		t := strings.Join(ps, "\n\n")
		chunks = append(chunks, reqChunk{Index: len(chunks), Text: t, norm: normalizeEvidence(t)})
	}

	var cur []string
	curLen := 0
	for _, p := range paras {
		pl := utf8.RuneCountInString(p)
		overflow := curLen+2+pl > chunkMaxRunes
		headingBreak := strings.HasPrefix(p, "#") && curLen > chunkMaxRunes/2
		if len(cur) > 0 && (overflow || headingBreak) {
			emit(cur)
			last := cur[len(cur)-1]
			cur, curLen = []string{last}, utf8.RuneCountInString(last)
			if curLen+2+pl > chunkMaxRunes {
				cur, curLen = nil, 0
			}
		}
		if len(cur) > 0 {
			curLen += 2
		}
		cur = append(cur, p)
		curLen += pl
	}
	if len(cur) > 0 {
		emit(cur)
	}
	return chunks
}

func isSentenceEnd(r []rune, i int) bool {
	switch r[i] {
	case '\n', '。':
		return true
	case '.', '!', '?':
		return i+1 < len(r) && unicode.IsSpace(r[i+1])
	}
	return false
}

// splitLong cuts an oversized paragraph at the last sentence boundary in the
// second half of the window, or hard-cuts when there is none.
func splitLong(p string, limit int) []string {
	r := []rune(p)
	var out []string
	for len(r) > limit {
		cut := limit
		for i := limit - 1; i > limit/2; i-- {
			if isSentenceEnd(r, i) {
				cut = i + 1
				break
			}
		}
		if s := strings.TrimSpace(string(r[:cut])); s != "" {
			out = append(out, s)
		}
		r = r[cut:]
	}
	if s := strings.TrimSpace(string(r)); s != "" {
		out = append(out, s)
	}
	return out
}
