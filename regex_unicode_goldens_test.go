//go:build regenerate_goldens

package gin

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	if unicode.Version != regexUnicodeV15 && unicode.Version != regexUnicodeV17 {
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
		if unicode.Version == regexUnicodeV15 {
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

// TestRegenerateRegexUnicodeGarayGolden writes the frozen \p{Garay} golden
// with the current builder. Garay is new in Unicode 16, so it runs on 17.0.0
// only:
//
//	GOTOOLCHAIN=go1.27.3 go test -tags regenerate_goldens -run TestRegenerateRegexUnicodeGarayGolden .
func TestRegenerateRegexUnicodeGarayGolden(t *testing.T) {
	if unicode.Version != regexUnicodeV17 {
		t.Fatalf("unicode.Version %s: the frozen Garay golden needs 17.0.0", unicode.Version)
	}
	data := mustEncodeUncompressed(t, buildRegexUnicodeGarayIndex(t))
	if err := os.WriteFile(regexUnicodeFrozenGarayGolden, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// regexUnicodeCorpusClassMin is the length from which a char class in a
// frozen text goes into the golden's class dictionary. Expanded Unicode
// classes repeat across many rows; the dictionary keeps the golden small.
const regexUnicodeCorpusClassMin = 64

// regexUnicodeSplitClasses returns the frozen text with every char class of
// at least regexUnicodeCorpusClassMin bytes replaced by a reference
// "\x00N\x00" into dict. A NUL byte never appears in String() output (it is
// rendered as an escape), so the reference cannot clash with the text.
func regexUnicodeSplitClasses(text string, dict map[string]int, order *[]string) string {
	var b strings.Builder
	for i := 0; i < len(text); {
		switch text[i] {
		case '\\':
			end := i + 2
			if end > len(text) {
				end = len(text)
			}
			b.WriteString(text[i:end])
			i = end
		case '[':
			j := i + 1
			if j < len(text) && text[j] == '^' {
				j++
			}
			if j < len(text) && text[j] == ']' {
				j++
			}
			for j < len(text) && text[j] != ']' {
				if text[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(text) {
				b.WriteString(text[i:])
				i = len(text)
				continue
			}
			class := text[i : j+1]
			if len(class) < regexUnicodeCorpusClassMin {
				b.WriteString(class)
			} else {
				id, ok := dict[class]
				if !ok {
					id = len(*order)
					dict[class] = id
					*order = append(*order, class)
				}
				fmt.Fprintf(&b, "\x00%d\x00", id)
			}
			i = j + 1
		default:
			b.WriteByte(text[i])
			i++
		}
	}
	return b.String()
}

// regexUnicodeEncodeCorpusCell turns a full cell (Go-quoted frozen text or a
// marker) into its golden form.
func regexUnicodeEncodeCorpusCell(cell string, dict map[string]int, order *[]string) string {
	text, err := strconv.Unquote(cell)
	if err != nil {
		return cell
	}
	return strconv.QuoteToASCII(regexUnicodeSplitClasses(text, dict, order))
}

// formatRegexUnicodeCorpusGolden renders rows (full cells) as the golden file.
func formatRegexUnicodeCorpusGolden(rows []regexUnicodeCorpusRow) string {
	dict := make(map[string]int)
	var order []string
	lines := make([]string, len(rows))
	for i, row := range rows {
		lines[i] = fmt.Sprintf("%d\t%s\t%s\t%s", i,
			regexUnicodeEncodeCorpusCell(row.frozen15, dict, &order),
			regexUnicodeEncodeCorpusCell(row.frozen17, dict, &order),
			strconv.QuoteToASCII(row.pattern))
	}
	var b strings.Builder
	b.WriteString("# Detector corpus (#115 generator, seed 8). Rows: index, frozen text on Unicode 15.0.0, frozen text on Unicode 17.0.0, pattern.\n")
	b.WriteString("# A cell is a Go-quoted frozen text, \"" + regexUnicodeCorpusErrorMarker + "\" (freeze failed) or " + regexUnicodeCorpusEmptyMarker + " (not generated yet).\n")
	b.WriteString("# \\x00N\\x00 inside a cell refers to class line @N. Long expanded classes repeat, so they are stored once.\n")
	b.WriteString("# Regenerate in two steps: GOTOOLCHAIN=go1.26.9, then GOTOOLCHAIN=go1.27.3, each running\n")
	b.WriteString("#   go test -tags regenerate_goldens -run TestRegenerateRegexUnicodeGoldens .\n")
	for i, class := range order {
		fmt.Fprintf(&b, "@%d\t%s\n", i, strconv.QuoteToASCII(class))
	}
	for _, line := range lines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// regexUnicodeGarayDocs: RG0 holds two Garay letters, RG1 none, RG2 one.
var regexUnicodeGarayDocs = []string{
	`{"name":"ab` + "\U00010D4A\U00010D4B" + `cd"}`,
	`{"name":"abcd"}`,
	`{"name":"` + "\U00010D4C" + `"}`,
}

// buildRegexUnicodeGarayIndex builds the Garay docs with the current builder.
// It needs Unicode 16 or later to register the pattern.
func buildRegexUnicodeGarayIndex(t *testing.T) *GINIndex {
	t.Helper()
	cfg, err := NewConfig(WithRegexExtractTransformer("$.name", "garay", regexUnicodeGarayPattern, 1, WithTransformerFailureMode(IngestFailureSoft)))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	builder := mustNewBuilder(t, cfg, len(regexUnicodeGarayDocs))
	for rg, doc := range regexUnicodeGarayDocs {
		if err := builder.AddDocument(DocID(rg), []byte(doc)); err != nil {
			t.Fatalf("AddDocument(%d): %v", rg, err)
		}
	}
	return builder.Finalize()
}
