package gin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

var (
	trigramUnicodeGramsGolden = filepath.Join("testdata", "trigram-unicode", "grams.golden")
	trigramUnicodeDriftGolden = filepath.Join("testdata", "trigram-unicode", "drift-fixture.bin")
	trigramUnicodeStdlibDiff  = filepath.Join("testdata", "trigram-unicode", "stdlib-17.0.0-diff.txt")
)

// trigramUnicodeDriftInputs holds runes whose lowercase or category differs
// between Unicode 15.0.0 (Go 1.21-1.26) and 17.0.0 (Go 1.27), plus context
// mappings and invalid UTF-8, so the goldens catch any toolchain drift (#99).
var trigramUnicodeDriftInputs = []string{
	"\uA7DCab",                         // U+A7DC lowers to U+019B only in 17.0.0
	"\u019Bab",                         // U+019B, the 17.0.0 lowercase of U+A7DC
	"\u0130stanbul",                    // dotted capital I lowers to plain i
	"\u212Aelvin",                      // Kelvin sign lowers to k
	"\u03A3\u0391\u03A3",               // no final-sigma context mapping
	"x\U0001FAE9\U0001FAE9\U0001FAE9y", // U+1FAE9 is a symbol only in 17.0.0
	"\U0001FAE9\U0001FAE9\U0001FAE9",
	"\U00016EA0\U00016EA1\U00016EA2x", // new 17.0.0 case pairs
	"\U00010D50ab",
	"\u1B4E\u1B4E\u1B4E", // punctuation only in 17.0.0
	"\u1C89ab",
	"a\xffb\xc3cd",    // invalid UTF-8
	"\xed\xa0\x80xyz", // surrogate bytes
	"Hello, World!",
	"   ",
	"!!!---",
	"ab",
}

var trigramUnicodeGramPaddings = []string{"", "$", "\U0001FAE9"}

func formatTrigramUnicodeGramList(grams []string) string {
	quoted := make([]string, len(grams))
	for i, g := range grams {
		quoted[i] = strconv.QuoteToASCII(g)
	}
	return "[" + strings.Join(quoted, " ") + "]"
}

// formatTrigramUnicodeGrams renders the GenerateNGrams output and the sorted
// TrigramIndex.Add keys for every drift input, N and padding.
func formatTrigramUnicodeGrams(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	for _, input := range trigramUnicodeDriftInputs {
		for _, n := range []int{2, 3, 4} {
			for _, pad := range trigramUnicodeGramPaddings {
				gen, err := GenerateNGrams(input, n, WithPadding(pad))
				if err != nil {
					t.Fatalf("GenerateNGrams(%q, %d, %q): %v", input, n, pad, err)
				}
				ti, err := NewTrigramIndex(1, WithN(n), WithPadding(pad))
				if err != nil {
					t.Fatalf("NewTrigramIndex(%d, %q): %v", n, pad, err)
				}
				ti.Add(input, 0)
				added := make([]string, 0, len(ti.Trigrams))
				for g := range ti.Trigrams {
					added = append(added, g)
				}
				sort.Strings(added)

				prefix := "in=" + strconv.QuoteToASCII(input) + " n=" + strconv.Itoa(n) + " pad=" + strconv.QuoteToASCII(pad)
				buf.WriteString("gen " + prefix + " " + formatTrigramUnicodeGramList(gen) + "\n")
				buf.WriteString("add " + prefix + " " + formatTrigramUnicodeGramList(added) + "\n")
			}
		}
	}
	return buf.Bytes()
}

func TestTrigramUnicodeGramGolden(t *testing.T) {
	want, err := os.ReadFile(trigramUnicodeGramsGolden)
	if err != nil {
		t.Fatalf("read %s: %v", trigramUnicodeGramsGolden, err)
	}
	got := formatTrigramUnicodeGrams(t)
	if bytes.Equal(got, want) {
		return
	}
	gotLines := strings.Split(string(got), "\n")
	wantLines := strings.Split(string(want), "\n")
	for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			t.Fatalf("grams differ from %s at line %d:\n got: %s\nwant: %s", trigramUnicodeGramsGolden, i+1, g, w)
		}
	}
	t.Fatalf("grams differ from %s", trigramUnicodeGramsGolden)
}

// trigramUnicodeDriftDocs is one document per row group. RG3 holds plain
// ASCII so that every drift query must prune it.
var trigramUnicodeDriftDocs = []string{
	"\uA7DCab xyz",
	"x\U0001FAE9\U0001FAE9\U0001FAE9y",
	"\U00016EA0\U00016EA1\U00016EA2 tail",
	"plain ascii",
}

func buildTrigramUnicodeDriftIndex(t *testing.T) *GINIndex {
	t.Helper()
	builder, err := NewBuilder(DefaultConfig(), len(trigramUnicodeDriftDocs))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	for i, v := range trigramUnicodeDriftDocs {
		doc, err := json.Marshal(map[string]string{"v": v})
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if err := builder.AddDocument(DocID(i), doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	return builder.Finalize()
}

// encodeTrigramUnicodeDriftIndex encodes without compression, so the golden
// depends only on the gram path and the wire format, not on zstd output.
func encodeTrigramUnicodeDriftIndex(t *testing.T) []byte {
	t.Helper()
	encoded, err := EncodeWithLevel(buildTrigramUnicodeDriftIndex(t), CompressionNone)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return encoded
}

func TestTrigramUnicodeDriftIndexGolden(t *testing.T) {
	want, err := os.ReadFile(trigramUnicodeDriftGolden)
	if err != nil {
		t.Fatalf("read %s: %v", trigramUnicodeDriftGolden, err)
	}
	if got := encodeTrigramUnicodeDriftIndex(t); !bytes.Equal(got, want) {
		t.Errorf("encoded drift index (%d bytes) differs from %s (%d bytes)", len(got), trigramUnicodeDriftGolden, len(want))
	}

	idx, err := Decode(want)
	if err != nil {
		t.Fatalf("Decode(%s): %v", trigramUnicodeDriftGolden, err)
	}
	exact := []struct {
		name string
		pred Predicate
		want []int
	}{
		{"contains U+A7DC", Contains("$.v", "\uA7DCab"), []int{0}},
		{"contains U+1FAE9", Contains("$.v", "\U0001FAE9\U0001FAE9\U0001FAE9"), []int{1}},
		{"contains U+16EA0", Contains("$.v", "\U00016EA0\U00016EA1\U00016EA2"), []int{2}},
		{"regex alternation", Regex("$.v", "\uA7DCab|zzzz"), []int{0}},
	}
	for _, c := range exact {
		if got := idx.Evaluate([]Predicate{c.pred}).ToSlice(); !slices.Equal(got, c.want) {
			t.Errorf("%s: got RGs %v, want %v", c.name, got, c.want)
		}
	}

	if !idx.Evaluate([]Predicate{Regex("$.v", "(?i)\uA7DCab")}).IsSet(0) {
		t.Errorf("Regex((?i)\\uA7DCab) pruned RG0")
	}
	folded := "(?i)\u019Bab"
	if regexp.MustCompile(folded).MatchString(trigramUnicodeDriftDocs[0]) {
		if !idx.Evaluate([]Predicate{Regex("$.v", folded)}).IsSet(0) {
			t.Errorf("Regex(%q) pruned RG0, which regexp matches on this toolchain", folded)
		}
	} else {
		t.Logf("regexp %q does not match RG0 on this toolchain; pruning RG0 is sound", folded)
	}
}

// trigramUnicodeStdlibDiffVersion is the newer toolchain Unicode version whose
// exact difference from the frozen tables is committed as a golden.
const trigramUnicodeStdlibDiffVersion = "17.0.0"

// formatTrigramUnicodeStdlibDiff lists every rune where a frozen lookup
// differs from the toolchain's unicode package, grouped by kind (lower,
// punctsym, space) and sorted by rune.
func formatTrigramUnicodeStdlibDiff() []byte {
	var lower, punctSym, space bytes.Buffer
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if frozen, std := trigramLowerRune(r), unicode.ToLower(r); frozen != std {
			fmt.Fprintf(&lower, "lower U+%04X frozen=U+%04X stdlib=U+%04X\n", r, frozen, std)
		}
		if frozen, std := trigramIsPunctOrSymbol(r), unicode.IsPunct(r) || unicode.IsSymbol(r); frozen != std {
			fmt.Fprintf(&punctSym, "punctsym U+%04X frozen=%t stdlib=%t\n", r, frozen, std)
		}
		if frozen, std := trigramIsSpace(r), unicode.IsSpace(r); frozen != std {
			fmt.Fprintf(&space, "space U+%04X frozen=%t stdlib=%t\n", r, frozen, std)
		}
	}
	out := append(lower.Bytes(), punctSym.Bytes()...)
	return append(out, space.Bytes()...)
}

func TestTrigramUnicodeTablesMatchStdlib(t *testing.T) {
	switch unicode.Version {
	case trigramUnicodeVersion:
		for r := rune(0); r <= unicode.MaxRune; r++ {
			if got, want := trigramLowerRune(r), unicode.ToLower(r); got != want {
				t.Fatalf("trigramLowerRune(%U) = %U, unicode.ToLower = %U", r, got, want)
			}
			if got, want := trigramIsSpace(r), unicode.IsSpace(r); got != want {
				t.Fatalf("trigramIsSpace(%U) = %t, unicode.IsSpace = %t", r, got, want)
			}
			if got, want := trigramIsPunctOrSymbol(r), unicode.IsPunct(r) || unicode.IsSymbol(r); got != want {
				t.Fatalf("trigramIsPunctOrSymbol(%U) = %t, unicode.IsPunct||IsSymbol = %t", r, got, want)
			}
		}
		samples := []string{
			"Hello, World!",
			"a\xffb\xc3",
			"\xed\xa0\x80x",
			"\u00c0\u00c9\u00ce \u03a3\u0391\u03a3 \u0130 \u212a",
			"Gr\u00f6\u00dfe \u041f\u0440\u0438\u0432\u0435\u0442 \U0001F389",
		}
		for _, s := range samples {
			if got, want := trigramLowerRunes(s), []rune(strings.ToLower(s)); !slices.Equal(got, want) {
				t.Errorf("trigramLowerRunes(%q) = %q, []rune(strings.ToLower) = %q", s, string(got), string(want))
			}
		}
	case trigramUnicodeStdlibDiffVersion:
		want, err := os.ReadFile(trigramUnicodeStdlibDiff)
		if err != nil {
			t.Fatalf("read %s: %v", trigramUnicodeStdlibDiff, err)
		}
		if got := formatTrigramUnicodeStdlibDiff(); !bytes.Equal(got, want) {
			t.Fatalf("frozen-vs-stdlib diff on Unicode %s (%d lines) differs from %s (%d lines)",
				unicode.Version, bytes.Count(got, []byte("\n")), trigramUnicodeStdlibDiff, bytes.Count(want, []byte("\n")))
		}
	default:
		t.Skipf("unicode.Version is %s: the frozen tables are checked exhaustively against Unicode %s and against the committed diff for Unicode %s only",
			unicode.Version, trigramUnicodeVersion, trigramUnicodeStdlibDiffVersion)
	}
}

func TestTrigramUnicodeSpotGoldens(t *testing.T) {
	lower := []struct{ in, want rune }{
		{0xA7DC, 0xA7DC},
		{0x019B, 0x019B},
		{0x0130, 'i'},
		{0x212A, 'k'},
		{0x03A3, 0x03C3},
		{'A', 'a'},
	}
	for _, c := range lower {
		if got := trigramLowerRune(c.in); got != c.want {
			t.Errorf("trigramLowerRune(%U) = %U, want %U", c.in, got, c.want)
		}
	}
	if trigramIsPunctOrSymbol(0x1FAE9) {
		t.Errorf("trigramIsPunctOrSymbol(U+1FAE9) = true, want false (unassigned in Unicode 15.0.0)")
	}
	if !trigramIsSpace(0x3000) {
		t.Errorf("trigramIsSpace(U+3000) = false, want true")
	}
	if got := GenerateTrigrams("\uA7DCab"); !slices.Equal(got, []string{"\uA7DCab"}) {
		t.Errorf("GenerateTrigrams(U+A7DC ab) = %q, want [\"\uA7DCab\"]", got)
	}
	if got := GenerateTrigrams(strings.Repeat("\U0001FAE9", 3)); len(got) != 1 {
		t.Errorf("GenerateTrigrams(3x U+1FAE9) = %q, want exactly 1 gram", got)
	}
}
