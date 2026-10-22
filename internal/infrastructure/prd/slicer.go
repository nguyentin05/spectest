package prd

import (
	"os"
	"strings"
	"unicode"
)

// Section represents a sliced Markdown PRD section extracted by Heading level 2 (##).
type Section struct {
	Heading  string   `json:"heading"`
	Keywords []string `json:"keywords"`
	Content  string   `json:"content"`
}

// SliceMarkdownFile reads a Markdown PRD file and splits it into logical business domain sections.
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

// SliceMarkdown splits raw Markdown text by `## ` headings.
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

// MatchSectionToTag finds the best matching PRD section(s) for a given OpenAPI domain tag.
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
