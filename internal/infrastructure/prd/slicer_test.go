package prd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const samplePRD = `# Product Requirements

Intro text before any section.

## User Authentication
Users log in with email and password.

## Order Checkout
Orders above 500,000 VND may apply voucher SALE50.

## Empty Section
`

func TestSliceMarkdown(t *testing.T) {
	sections := SliceMarkdown(samplePRD)
	if len(sections) != 3 {
		t.Fatalf("expected 3 non-empty sections, got %d: %+v", len(sections), sections)
	}

	if sections[0].Heading != "General Requirements" || !strings.Contains(sections[0].Content, "Intro text") {
		t.Errorf("preamble should be captured as General Requirements: %+v", sections[0])
	}
	if sections[1].Heading != "User Authentication" {
		t.Errorf("unexpected heading: %q", sections[1].Heading)
	}
	if !reflect.DeepEqual(sections[2].Keywords, []string{"order", "checkout"}) {
		t.Errorf("keywords = %v", sections[2].Keywords)
	}
}

func TestSliceMarkdownFile(t *testing.T) {
	sections, err := SliceMarkdownFile("")
	if err != nil || sections != nil {
		t.Errorf("empty path should return nil, nil; got %v, %v", sections, err)
	}

	if _, err := SliceMarkdownFile(filepath.Join(t.TempDir(), "missing.md")); err == nil {
		t.Error("missing file should return error")
	}

	path := filepath.Join(t.TempDir(), "prd.md")
	if err := os.WriteFile(path, []byte(samplePRD), 0o644); err != nil {
		t.Fatal(err)
	}
	sections, err = SliceMarkdownFile(path)
	if err != nil || len(sections) != 3 {
		t.Errorf("expected 3 sections from file, got %d (err=%v)", len(sections), err)
	}
}

func TestMatchSectionToTag(t *testing.T) {
	sections := SliceMarkdown(samplePRD)

	if got := MatchSectionToTag(nil, "orders"); got != "" {
		t.Errorf("no sections should return empty string, got %q", got)
	}

	got := MatchSectionToTag(sections, "checkout")
	if !strings.Contains(got, "## Order Checkout") || strings.Contains(got, "User Authentication") {
		t.Errorf("checkout tag should match only Order Checkout section, got:\n%s", got)
	}

	got = MatchSectionToTag(sections, "authentication")
	if !strings.Contains(got, "## User Authentication") {
		t.Errorf("authentication tag should match heading, got:\n%s", got)
	}

	got = MatchSectionToTag(sections, "orders")
	if !strings.Contains(got, "## Order Checkout") {
		t.Errorf("orders tag should match keyword 'order', got:\n%s", got)
	}

	got = MatchSectionToTag(sections, "inventory")
	for _, h := range []string{"General Requirements", "User Authentication", "Order Checkout"} {
		if !strings.Contains(got, h) {
			t.Errorf("unmatched tag should fall back to all sections, missing %q", h)
		}
	}
}

func TestTokenizeHeading(t *testing.T) {
	got := tokenizeHeading("API v2: Order-Checkout & Payments")
	want := []string{"api", "order", "checkout", "payments"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokenizeHeading() = %v, want %v", got, want)
	}
}
