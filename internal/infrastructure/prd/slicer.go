// Package prd parses and slices Markdown Product Requirements Documents (PRDs)
// into domain sections that can be paired with corresponding OpenAPI tag chunks.
package prd

import (
	"os"
	"strings"
	"unicode"
)

// Section represents a logical requirement block extracted from a Markdown PRD
// under a level-2 heading (`## `), along with tokenized heading keywords.
type Section struct {
	Heading  string   `json:"heading"`
	Keywords []string `json:"keywords"`
	Content  string   `json:"content"`
}

// SliceMarkdownFile reads a Markdown file from disk and splits it into Section blocks.
// If path is empty, it returns nil without error so PRD input remains optional.
func SliceMarkdownFile(path string) ([]Section, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return SliceMarkdown(string(data)), nil
}

// SliceMarkdown partitions raw Markdown text at every level-2 heading (`## `)
// and extracts searchable keywords from each section title.
func SliceMarkdown(markdown string) []Section {
	lines := strings.Split(markdown, "\n")
	var sections []Section

	currentHeading := "General Requirements"
	var buf []string

	flush := func() {
		body := strings.TrimSpace(strings.Join(buf, "\n"))
		if body != "" {
			sections = append(sections, Section{
				Heading:  currentHeading,
				Keywords: tokenizeHeading(currentHeading),
				Content:  body,
			})
		}
		buf = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			flush()
			currentHeading = strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			continue
		}
		buf = append(buf, line)
	}
	flush()

	return sections
}

// MatchSectionToTag selects the Markdown Section entries whose heading or keywords match
// the given OpenAPI domain tag. If no specific section matches, all sections are concatenated
// as fallback context.
func MatchSectionToTag(sections []Section, tag string) string {
	if len(sections) == 0 {
		return ""
	}
	lowerTag := strings.ToLower(strings.TrimSpace(tag))
	var matched []string

	for _, sec := range sections {
		lowerHeading := strings.ToLower(sec.Heading)
		if strings.Contains(lowerHeading, lowerTag) || strings.Contains(lowerTag, lowerHeading) {
			matched = append(matched, "## "+sec.Heading+"\n"+sec.Content)
			continue
		}
		for _, kw := range sec.Keywords {
			if kw == lowerTag || strings.Contains(lowerTag, kw) || strings.Contains(kw, lowerTag) {
				matched = append(matched, "## "+sec.Heading+"\n"+sec.Content)
				break
			}
		}
	}

	if len(matched) == 0 {
		var all []string
		for _, sec := range sections {
			all = append(all, "## "+sec.Heading+"\n"+sec.Content)
		}
		return strings.Join(all, "\n\n")
	}

	return strings.Join(matched, "\n\n")
}

// tokenizeHeading splits a heading string into lowercase alphanumeric tokens of length >= 3.
func tokenizeHeading(text string) []string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	var out []string
	for _, w := range words {
		if len(w) >= 3 {
			out = append(out, w)
		}
	}
	return out
}
