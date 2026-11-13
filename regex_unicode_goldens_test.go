//go:build regenerate_goldens

package gin

import (
	"os"
	"path/filepath"
	"testing"
	"unicode"
)

// TestRegenerateRegexUnicodeGoldens fills this toolchain's column of the
// detector corpus golden and keeps the other column. Run it on both
// toolchains:
//
//	GOTOOLCHAIN=go1.26.9 go test -tags regenerate_goldens -run TestRegenerateRegexUnicodeGoldens .
//	GOTOOLCHAIN=go1.27.3 go test -tags regenerate_goldens -run TestRegenerateRegexUnicodeGoldens .
func TestRegenerateRegexUnicodeGoldens(t *testing.T) {
	if unicode.Version != "15.0.0" && unicode.Version != "17.0.0" {
		t.Fatalf("unicode.Version %s has no golden column", unicode.Version)
	}
	patterns := regexUnicodeCorpusPatterns(regexUnicodeCorpusSize)
	rows := make([]regexUnicodeCorpusRow, len(patterns))
	for i, pattern := range patterns {
		rows[i] = regexUnicodeCorpusRow{pattern: pattern, frozen15: regexUnicodeCorpusEmptyMarker, frozen17: regexUnicodeCorpusEmptyMarker}
	}
	if data, err := os.ReadFile(regexUnicodeCorpusGolden); err == nil {
		existing, err := parseRegexUnicodeCorpusGolden(string(data))
		if err != nil {
			t.Fatalf("parse existing golden: %v", err)
		}
		for i, row := range existing {
			if i < len(rows) && row.pattern == rows[i].pattern {
				rows[i].frozen15, rows[i].frozen17 = row.frozen15, row.frozen17
			}
		}
	}
	for i := range rows {
		cell := regexUnicodeCorpusCell(rows[i].pattern)
		if unicode.Version == "15.0.0" {
			rows[i].frozen15 = cell
		} else {
			rows[i].frozen17 = cell
		}
	}
	if err := os.MkdirAll(filepath.Dir(regexUnicodeCorpusGolden), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regexUnicodeCorpusGolden, []byte(formatRegexUnicodeCorpusGolden(rows)), 0o644); err != nil {
		t.Fatal(err)
	}
}
