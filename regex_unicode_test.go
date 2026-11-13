package gin

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"regexp"
	"regexp/syntax"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/pkg/errors"
)

// regexUnicodeSensitivePatterns must all be flagged by the detector on every
// toolchain (AC1, R1).
var regexUnicodeSensitivePatterns = []string{
	`(\p{Lu}+)`,
	`\PL`,
	`[^\p{L}]`,
	`(?i)ƛ`,
	`(?i)(ƛ+)`,
	`(?i:é)`,
	`a(?i)é`,
	`(?i)\x{e9}`,
	`(?i)\xE9`,
	`(?i)\351`,
	`(?-i)é`,
	`\p{Garay}`,
	`(\p{Garay}+)`,
	`[é-ü]`,
	// R1 counterexamples: syntax.Parse resolves these classes with the
	// toolchain tables, so the tree rule alone misses them.
	`([^\P{Lu}\x00-\x{A7DB}\x{A7DD}-\x{10FFFF}]+|x)`,
	`(?i)([^\x00-\x{A7DB}\x{A7DD}-\x{10FFFF}]+|x)`,
}

// regexUnicodeSafePatterns must never be flagged.
var regexUnicodeSafePatterns = []string{
	`order-(\d+)`,
	`ERROR\[(\w+)\]:`,
	`(?i)(error)`,
	`\d`, `\w`, `\s`, `\b`,
	`[[:alpha:]]`,
	`.`,
	`[^a]`,
	`\D`, `\W`, `\S`,
	`(?i)[a-z]`,
	`(?i)k`,
	`\\p`,
	`(?P<id>\d+)`,
}

func TestRegexUnicodeDetectorFlagsSensitivePatterns(t *testing.T) {
	for _, pattern := range regexUnicodeSensitivePatterns {
		if !regexTableSensitive(pattern) {
			t.Errorf("regexTableSensitive(%q) = false, want true (unicode %s)", pattern, unicode.Version)
		}
	}
}

func TestRegexUnicodeDetectorSafeList(t *testing.T) {
	for _, pattern := range regexUnicodeSafePatterns {
		if regexTableSensitive(pattern) {
			t.Errorf("regexTableSensitive(%q) = true, want false (unicode %s)", pattern, unicode.Version)
		}
	}
}

// TestRegexUnicodeDetectorSourceRuleCatchesCounterexamples proves the R1
// counterexamples need the source rule: the tree rule alone misses them.
func TestRegexUnicodeDetectorSourceRuleCatchesCounterexamples(t *testing.T) {
	for _, pattern := range []string{
		`([^\P{Lu}\x00-\x{A7DB}\x{A7DD}-\x{10FFFF}]+|x)`,
		`(?i)([^\x00-\x{A7DB}\x{A7DD}-\x{10FFFF}]+|x)`,
	} {
		if !regexSourceSensitive(pattern) {
			t.Errorf("regexSourceSensitive(%q) = false, want true", pattern)
		}
	}
}

// regexUnicodeAdversarialPatterns builds patterns that all request a Unicode
// table operation: negated classes with escaped non-ASCII ranges under an i
// flag, \P inside negated classes, scoped flags, flag groups with several
// flags and non-ASCII escapes.
func regexUnicodeAdversarialPatterns(n int) []string {
	rng := rand.New(rand.NewSource(115))
	hex := func() string {
		r := rune(0x80 + rng.Intn(0x10FFFF-0x80))
		if r >= 0xD800 && r <= 0xDFFF {
			r = 0xE000 + r%0x100
		}
		return fmt.Sprintf(`\x{%X}`, r)
	}
	nonASCII := func() string {
		switch rng.Intn(4) {
		case 0:
			return hex()
		case 1:
			return fmt.Sprintf(`\x%02X`, 0x80+rng.Intn(0x80))
		case 2:
			return fmt.Sprintf(`\%o`, 0o200+rng.Intn(0o577-0o200))
		default:
			return string([]rune{'é', 'ƛ', 'Ꟛ', 'ß', 'ΐ', '漢', 'ǳ'}[rng.Intn(7)])
		}
	}
	iFlag := func() string {
		return []string{`(?i)`, `(?-i)`, `(?si)`, `(?im)`, `(?Ui)`, `(?ms-i)`, `(?i-s)`}[rng.Intn(7)]
	}
	scripts := []string{`Lu`, `L`, `Greek`, `Han`, `Garay`, `N`, `Ll`}
	filler := []string{``, `a`, `x+`, `\d`, `[a-z]`, `|y`, `.`, `-`}
	var out []string
	for i := 0; i < n; i++ {
		pre := filler[rng.Intn(len(filler))]
		post := filler[rng.Intn(len(filler))]
		var core string
		switch rng.Intn(6) {
		case 0:
			core = iFlag() + `([^\x00-` + hex() + nonASCII() + `-\x{10FFFF}]+|x)`
		case 1:
			core = `([^\P{` + scripts[rng.Intn(len(scripts))] + `}\x00-` + hex() + `]+|x)`
		case 2:
			core = `(?i:` + pre + nonASCII() + `)`
		case 3:
			core = iFlag() + pre + nonASCII()
		case 4:
			core = `[^\p{` + scripts[rng.Intn(len(scripts))] + `}` + pre + `]`
		default:
			core = pre + iFlag() + `[` + nonASCII() + `-` + hex() + `]`
		}
		out = append(out, pre+core+post)
	}
	return out
}

func TestRegexUnicodeDetectorAdversarial(t *testing.T) {
	patterns := regexUnicodeAdversarialPatterns(2500)
	if len(patterns) < 2000 {
		t.Fatalf("generator produced %d patterns, want at least 2000", len(patterns))
	}
	for _, pattern := range patterns {
		if !regexTableSensitive(pattern) {
			t.Errorf("regexTableSensitive(%q) = false, want true", pattern)
		}
	}
}

// regexUnicodeCorpusAtoms is the spike 008 atom list.
var regexUnicodeCorpusAtoms = []string{`a`, `Z`, `k`, `s`, `\d`, `\w`, `\s`, `.`, `[a-z]`, `[^a]`, `[[:alpha:]]`, `\W`, `-`, `\b`,
	`\p{L}`, `\p{Lu}`, `\p{Ll}`, `\P{L}`, `\p{Han}`, `\p{Greek}`, `\pN`, `[^\p{L}]`, `ƛ`, `ΐ`, `ɤ`, `ß`, `é`, `[é-ü]`, `Ꟛ`, `漢`}

// regexUnicodeCorpusPatterns is the spike 008 generator (seed 8).
func regexUnicodeCorpusPatterns(n int) []string {
	rng := rand.New(rand.NewSource(8))
	quantifiers := []string{"", "", "+", "*", "?", "{1,2}"}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		var b strings.Builder
		if rng.Intn(3) == 0 {
			b.WriteString("(?i)")
		}
		parts := 1 + rng.Intn(4)
		b.WriteString("(")
		for j := 0; j < parts; j++ {
			if j > 0 && rng.Intn(5) == 0 {
				b.WriteString("|")
			}
			b.WriteString(regexUnicodeCorpusAtoms[rng.Intn(len(regexUnicodeCorpusAtoms))] + quantifiers[rng.Intn(len(quantifiers))])
		}
		b.WriteString(")")
		out = append(out, b.String())
	}
	return out
}

const (
	regexUnicodeCorpusSize        = 3000
	regexUnicodeCorpusGolden      = "testdata/regex-unicode/detector-corpus.golden"
	regexUnicodeCorpusErrorMarker = "!error"
	regexUnicodeCorpusEmptyMarker = "-"
)

// regexUnicodeCorpusRow is one golden line: per-toolchain frozen text (Go
// quoted) or a marker.
type regexUnicodeCorpusRow struct {
	pattern  string
	frozen15 string
	frozen17 string
}

func regexUnicodeCorpusCell(pattern string) string {
	frozen, err := freezeRegexPattern(pattern, maxConfigSize)
	if err != nil {
		return regexUnicodeCorpusErrorMarker
	}
	return strconv.QuoteToASCII(frozen)
}

// regexUnicodeJoinClasses reverses regexUnicodeSplitClasses.
func regexUnicodeJoinClasses(text string, classes []string) (string, error) {
	parts := strings.Split(text, "\x00")
	if len(parts)%2 == 0 {
		return "", errors.Errorf("unbalanced class reference in %q", text)
	}
	var b strings.Builder
	for i, part := range parts {
		if i%2 == 0 {
			b.WriteString(part)
			continue
		}
		id, err := strconv.Atoi(part)
		if err != nil || id < 0 || id >= len(classes) {
			return "", errors.Errorf("bad class reference %q", part)
		}
		b.WriteString(classes[id])
	}
	return b.String(), nil
}

// parseRegexUnicodeCorpusGolden parses the golden into rows with full cells.
func parseRegexUnicodeCorpusGolden(data string) ([]regexUnicodeCorpusRow, error) {
	var classes []string
	var rows []regexUnicodeCorpusRow
	decodeCell := func(cell string) (string, error) {
		if cell == regexUnicodeCorpusErrorMarker || cell == regexUnicodeCorpusEmptyMarker {
			return cell, nil
		}
		text, err := strconv.Unquote(cell)
		if err != nil {
			return "", errors.Wrapf(err, "golden cell %s", cell)
		}
		full, err := regexUnicodeJoinClasses(text, classes)
		if err != nil {
			return "", err
		}
		return strconv.QuoteToASCII(full), nil
	}
	for _, line := range strings.Split(data, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if strings.HasPrefix(line, "@") {
			if len(fields) != 2 || fields[0] != "@"+strconv.Itoa(len(classes)) {
				return nil, errors.Errorf("bad class line %q", line)
			}
			class, err := strconv.Unquote(fields[1])
			if err != nil {
				return nil, errors.Wrapf(err, "class line %q", line)
			}
			classes = append(classes, class)
			continue
		}
		if len(fields) != 4 {
			return nil, errors.Errorf("golden line %q has %d fields, want 4", line, len(fields))
		}
		if fields[0] != strconv.Itoa(len(rows)) {
			return nil, errors.Errorf("golden line index %s, want %d", fields[0], len(rows))
		}
		pattern, err := strconv.Unquote(fields[3])
		if err != nil {
			return nil, errors.Wrapf(err, "golden pattern %s", fields[3])
		}
		frozen15, err := decodeCell(fields[1])
		if err != nil {
			return nil, err
		}
		frozen17, err := decodeCell(fields[2])
		if err != nil {
			return nil, err
		}
		rows = append(rows, regexUnicodeCorpusRow{pattern: pattern, frozen15: frozen15, frozen17: frozen17})
	}
	return rows, nil
}

func readRegexUnicodeCorpusGolden(t *testing.T) []regexUnicodeCorpusRow {
	t.Helper()
	data, err := os.ReadFile(regexUnicodeCorpusGolden)
	if err != nil {
		t.Fatalf("read %s: %v", regexUnicodeCorpusGolden, err)
	}
	rows, err := parseRegexUnicodeCorpusGolden(string(data))
	if err != nil {
		t.Fatalf("parse %s: %v", regexUnicodeCorpusGolden, err)
	}
	return rows
}

// regexUnicodeBoundaryPoints returns the class and literal boundaries of a
// parsed pattern: lo and hi+1 of every range, r and r+1 of every literal rune.
func regexUnicodeBoundaryPoints(re *syntax.Regexp, points map[rune]int, sign int) {
	switch re.Op {
	case syntax.OpCharClass:
		for i := 0; i+1 < len(re.Rune); i += 2 {
			points[re.Rune[i]] += sign
			points[re.Rune[i+1]+1] += sign
		}
	case syntax.OpLiteral:
		for _, r := range re.Rune {
			orbit := []rune{r}
			if re.Flags&syntax.FoldCase != 0 {
				// The parser turns a two-rune class that is a fold orbit on
				// this toolchain into a FoldCase literal.
				orbit = regexFoldOrbit(r)
			}
			for _, o := range orbit {
				points[o] += sign
				points[o+1] += sign
			}
		}
	}
	for _, sub := range re.Sub {
		regexUnicodeBoundaryPoints(sub, points, sign)
	}
}

// regexUnicodeSpikeInputs are the fixed spike inputs.
var regexUnicodeSpikeInputs = []string{"STRASSE", "straße", "KKk", "ΐΐ", "ƛꟜ", "ɤꟋ", "é ü", "abc-12", "ſs", "漢字",
	"aKkſßΐΐƛꟜ\nb", "Straße STRASSE", "ABCꟜ", "1Ƛ2A"}

// regexUnicodeDriftInputs builds inputs that tell the two frozen texts apart:
// the spike inputs plus every boundary point that only one text has, each in
// the spike contexts (alone, "a"+r+"Z", "ZZ"+r+r).
func regexUnicodeDriftInputs(a, b *syntax.Regexp) []string {
	points := make(map[rune]int)
	regexUnicodeBoundaryPoints(a, points, 1)
	countsA := make(map[rune]int, len(points))
	for r, c := range points {
		countsA[r] = c
	}
	pointsB := make(map[rune]int)
	regexUnicodeBoundaryPoints(b, pointsB, 1)
	var drift []rune
	seen := make(map[rune]bool)
	add := func(r rune) {
		if r < 0 || r > unicode.MaxRune || !utf8.ValidRune(r) || seen[r] {
			return
		}
		seen[r] = true
		drift = append(drift, r)
	}
	for r, c := range countsA {
		if pointsB[r] != c {
			add(r)
			add(r - 1)
		}
	}
	for r, c := range pointsB {
		if countsA[r] != c {
			add(r)
			add(r - 1)
		}
	}
	sort.Slice(drift, func(i, j int) bool { return drift[i] < drift[j] })
	inputs := append([]string(nil), regexUnicodeSpikeInputs...)
	for _, r := range drift {
		s := string(r)
		inputs = append(inputs, s, "a"+s+"Z", "ZZ"+s+s)
	}
	return inputs
}

// TestRegexUnicodeDetectorCorpusGolden proves zero false negatives on the
// spike 008 corpus. The golden holds the frozen text of every pattern on
// 15.0.0 and on 17.0.0. Both frozen texts are table-free, so this toolchain
// can compile both and compare them: a semantic difference is real drift of
// the original pattern between the two toolchains, and the local detector
// must flag it. Running the test on both toolchains covers both builders.
func TestRegexUnicodeDetectorCorpusGolden(t *testing.T) {
	rows := readRegexUnicodeCorpusGolden(t)
	patterns := regexUnicodeCorpusPatterns(regexUnicodeCorpusSize)
	if len(rows) != len(patterns) {
		t.Fatalf("golden has %d rows, generator %d", len(rows), len(patterns))
	}
	var own func(row regexUnicodeCorpusRow) string
	switch unicode.Version {
	case regexUnicodeV15:
		own = func(row regexUnicodeCorpusRow) string { return row.frozen15 }
	case regexUnicodeV17:
		own = func(row regexUnicodeCorpusRow) string { return row.frozen17 }
	default:
		t.Fatalf("unicode.Version %s has no golden column", unicode.Version)
	}
	drifted, renderOnly, flagged := 0, 0, 0
	for i, row := range rows {
		if row.pattern != patterns[i] {
			t.Fatalf("row %d pattern %q, generator %q: regenerate the golden", i, row.pattern, patterns[i])
		}
		if row.frozen15 == regexUnicodeCorpusEmptyMarker || row.frozen17 == regexUnicodeCorpusEmptyMarker {
			t.Fatalf("row %d has an empty column: regenerate on both toolchains", i)
		}
		if got := regexUnicodeCorpusCell(row.pattern); got != own(row) {
			t.Fatalf("row %d %q: local frozen text %s, golden %s", i, row.pattern, got, own(row))
		}
		if row.frozen15 == row.frozen17 {
			continue
		}
		sensitive := regexTableSensitive(row.pattern)
		if sensitive {
			flagged++
		}
		drift, detail := regexUnicodeFrozenTextsDiffer(t, row)
		if !drift {
			renderOnly++
			continue
		}
		drifted++
		if !sensitive {
			t.Errorf("FALSE NEGATIVE row %d %q: %s", i, row.pattern, detail)
		}
	}
	t.Logf("unicode %s: %d rows, %d drift, %d render-only differences, %d differing rows flagged", unicode.Version, len(rows), drifted, renderOnly, flagged)
}

// regexUnicodeFrozenTextsDiffer compiles both golden columns locally and
// reports whether they match differently. A compile-error marker in exactly
// one column counts as drift.
func regexUnicodeFrozenTextsDiffer(t *testing.T, row regexUnicodeCorpusRow) (bool, string) {
	t.Helper()
	if row.frozen15 == regexUnicodeCorpusErrorMarker || row.frozen17 == regexUnicodeCorpusErrorMarker {
		return true, "compile error on one toolchain"
	}
	text15, err := strconv.Unquote(row.frozen15)
	if err != nil {
		t.Fatalf("unquote %s: %v", row.frozen15, err)
	}
	text17, err := strconv.Unquote(row.frozen17)
	if err != nil {
		t.Fatalf("unquote %s: %v", row.frozen17, err)
	}
	re15, err := regexp.Compile(text15)
	if err != nil {
		t.Fatalf("frozen 15.0.0 text %q does not compile: %v", text15, err)
	}
	re17, err := regexp.Compile(text17)
	if err != nil {
		t.Fatalf("frozen 17.0.0 text %q does not compile: %v", text17, err)
	}
	tree15, err := syntax.Parse(text15, syntax.Perl)
	if err != nil {
		t.Fatalf("parse %q: %v", text15, err)
	}
	tree17, err := syntax.Parse(text17, syntax.Perl)
	if err != nil {
		t.Fatalf("parse %q: %v", text17, err)
	}
	for _, in := range regexUnicodeDriftInputs(tree15, tree17) {
		a, b := re15.FindAllStringSubmatchIndex(in, -1), re17.FindAllStringSubmatchIndex(in, -1)
		if !reflect.DeepEqual(a, b) {
			return true, fmt.Sprintf("input %q: %v vs %v", in, a, b)
		}
	}
	return false, ""
}

func TestRegexUnicodeVerifier(t *testing.T) {
	pass := []string{
		`(\\p)`,
		`[(?i)]`,
		`(?P<id>[0-9]+)`,
		`(?s:.)`,
		`([A-ZÀ-ÖꟜ]+)`,
		`[]p]`,
		`[^]\]]`,
		`[[:alpha:]p]`,
		`(?:a)`,
		`(?m)^a$`,
	}
	for _, pattern := range pass {
		if err := verifyFrozenRegexText(pattern); err != nil {
			t.Errorf("verifyFrozenRegexText(%q) = %v, want nil", pattern, err)
		}
	}
	fail := []string{
		`(\p{Lu}+)`,
		`\PL`,
		`(?i)(ƛ+)`,
		`(?i:x)`,
		`(?si)a`,
		`(?-i)a`,
		`[\p{L}]`,
		`[]\pL]`,
		`[[:alpha:]](?i)a`,
	}
	for _, pattern := range fail {
		if err := verifyFrozenRegexText(pattern); err == nil {
			t.Errorf("verifyFrozenRegexText(%q) = nil, want error", pattern)
		}
	}
}

// regexUnicodeEquivalenceInputs are the spike inputs for freezer equivalence.
var regexUnicodeEquivalenceInputs = []string{"aKkſßΐΐƛꟜ\nb", "Straße STRASSE", "ABCꟜ", "1Ƛ2A"}

func TestRegexUnicodeFreezerEquivalence(t *testing.T) {
	patterns := []string{
		`(a)(b(c))`,
		`a`,
		`(?P<n>a+)(?P<m>\d)?`,
		`..0|.0|..`,
		`(?m)^(\w+)$`,
		`(?s)(a.+b)`,
		`(?U)(a+)`,
		`(?i)(straße)`,
		`(?i)(ΐ)`,
		`(?i)ƛ`,
		`\p{Lu}`,
		`[^\p{L}]`,
		`(?i)[^\p{Lu}k]`,
		`(\p{Lu}+)`,
		`(\d+)\p{Lu}`,
	}
	for _, pattern := range patterns {
		frozen, err := freezeRegexPattern(pattern, maxConfigSize)
		if err != nil {
			t.Fatalf("freeze %q: %v", pattern, err)
		}
		requireRegexUnicodeFrozenEquivalent(t, pattern, frozen, regexUnicodeEquivalenceInputs)
		if strings.Contains(frozen, `\p`) || strings.Contains(frozen, `\P`) || strings.Contains(frozen, `(?i`) {
			t.Errorf("frozen %q of %q still requests a table operation", frozen, pattern)
		}
		if err := verifyFrozenRegexText(frozen); err != nil {
			t.Errorf("verifier rejects frozen %q of %q: %v", frozen, pattern, err)
		}
		requireRegexUnicodeFixedPoint(t, pattern, frozen)
	}
}

func requireRegexUnicodeFrozenEquivalent(t *testing.T, pattern, frozen string, inputs []string) {
	t.Helper()
	original, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("compile %q: %v", pattern, err)
	}
	compiled, err := regexp.Compile(frozen)
	if err != nil {
		t.Fatalf("frozen %q of %q does not compile: %v", frozen, pattern, err)
	}
	if original.NumSubexp() != compiled.NumSubexp() {
		t.Fatalf("frozen %q of %q: NumSubexp %d, want %d", frozen, pattern, compiled.NumSubexp(), original.NumSubexp())
	}
	if !reflect.DeepEqual(original.SubexpNames(), compiled.SubexpNames()) {
		t.Fatalf("frozen %q of %q: SubexpNames %q, want %q", frozen, pattern, compiled.SubexpNames(), original.SubexpNames())
	}
	for _, in := range inputs {
		a, b := original.FindAllStringSubmatchIndex(in, -1), compiled.FindAllStringSubmatchIndex(in, -1)
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("p=%q frozen=%q in=%q: %v vs %v", pattern, frozen, in, a, b)
		}
	}
}

// requireRegexUnicodeFixedPoint requires repeated freezing to reach a fixed
// point within 4 passes. One-pass idempotence is not guaranteed: the stdlib
// re-factors String output.
func requireRegexUnicodeFixedPoint(t *testing.T, pattern, frozen string) {
	t.Helper()
	current := frozen
	for pass := 0; ; pass++ {
		next, err := freezeRegexPattern(current, maxConfigSize)
		if err != nil {
			t.Fatalf("pass %d of %q: %v", pass, pattern, err)
		}
		if next == current {
			return
		}
		if pass == 4 {
			t.Fatalf("no fixed point: %q -> %q", pattern, next)
		}
		if _, err := regexp.Compile(next); err != nil {
			t.Fatalf("pass %d does not compile: %q", pass, next)
		}
		current = next
	}
}

func FuzzRegexUnicodeFreeze(f *testing.F) {
	for _, s := range []string{`(?i)(ƛ+)`, `(?i)a(?-i)b`, `(?is)^.\z`, `(?U)a+?`, `(?m)^$`, `\Qa.b\E`, `(?i)[^\p{Lu}k]`,
		`(?i)(?P<n>ΐ)|x`, `a{2,3}`, `(?:)`, `x*?|`, `(?i)\x{212A}`, `[\x{10FFFF}]`, `(?i)ſ`, `..0|.0|..`, `(?i)(ǳ)`} {
		f.Add(s, "aKkſßΐΐƛꟜ\nb")
	}
	f.Fuzz(func(t *testing.T, pattern, in string) {
		if len(pattern) > 64 || !utf8.ValidString(pattern) {
			return
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return
		}
		frozen, err := freezeRegexPattern(pattern, maxConfigSize)
		if err != nil {
			t.Fatalf("freeze %q: %v", pattern, err)
		}
		requireRegexUnicodeFrozenEquivalent(t, pattern, frozen, []string{in})
		requireRegexUnicodeFixedPoint(t, pattern, frozen)
	})
}

func TestRegexUnicodeFreezerBudget(t *testing.T) {
	_, err := freezeRegexPattern(`(\p{L}+)`, 100)
	if err == nil || !strings.Contains(err.Error(), "at least") {
		t.Fatalf("freeze with budget 100: err = %v, want lower-bound error", err)
	}
	_, err = freezeRegexPattern(strings.Repeat(`\pL`, 300), maxConfigSize)
	if err == nil {
		t.Fatal("freeze of 300 x \\pL: want size error")
	}
	frozen, err := freezeRegexPattern(`(\p{L}+)`, maxConfigSize)
	if err != nil {
		t.Fatalf("freeze (\\p{L}+): %v", err)
	}
	if len(frozen) < 1000 || len(frozen) > 64<<10 {
		t.Fatalf("frozen (\\p{L}+) is %d bytes, want about 4.6 KB", len(frozen))
	}
}

func TestRegexUnicodeWireParams(t *testing.T) {
	accept := []struct {
		raw    string
		marked bool
	}{
		{`{"pattern":"x","group":1}`, false},
		{`{"pattern":"x","group":1,"source_pattern":"(?i)x","frozen_unicode":"17.0.0"}`, true},
		{`{"pattern":"x","group":0,"source_pattern":"y","frozen_unicode":"99.0.0"}`, true},
	}
	for _, tc := range accept {
		params, marked, err := decodeRegexWireParams(json.RawMessage(tc.raw))
		if err != nil {
			t.Fatalf("decode %s: %v", tc.raw, err)
		}
		if marked != tc.marked || params.Pattern != "x" {
			t.Fatalf("decode %s: marked %v params %+v", tc.raw, marked, params)
		}
	}
	reject := []string{
		`{"pattern":"x","group":1,"source_pattern":"y"}`,
		`{"pattern":"x","group":1,"frozen_unicode":"17.0.0"}`,
		`{"pattern":"x","group":1,"source_pattern":"","frozen_unicode":"17.0.0"}`,
		`{"pattern":"x","group":1,"source_pattern":"y","frozen_unicode":""}`,
		`{"pattern":"x","group":1,"source_pattern":null,"frozen_unicode":"17.0.0"}`,
		`{"pattern":"x","group":1,"source_pattern":"y","frozen_unicode":null}`,
		`{"pattern":"x","group":1,"source_pattern":5,"frozen_unicode":"17.0.0"}`,
		`{"pattern":"x","group":1,"source_pattern":"y","frozen_unicode":{}}`,
		`{"pattern":"x","group":-1}`,
		`{"pattern":`,
		`{"pattern":"x","group":"1"}`,
	}
	for _, raw := range reject {
		if _, _, err := decodeRegexWireParams(json.RawMessage(raw)); err == nil {
			t.Errorf("decode %s: want error", raw)
		}
	}
}

// TestRegexUnicodeRegexParamsUnchanged keeps the exported RegexParams shape
// (L10): unkeyed literals still compile.
func TestRegexUnicodeRegexParamsUnchanged(t *testing.T) {
	params := RegexParams{"x", 1}
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"pattern":"x","group":1}` {
		t.Fatalf("RegexParams marshal = %s", data)
	}
}

func TestRegexUnicodeTypedCompileErrors(t *testing.T) {
	_, err := compileRegexWithTimeout(`[invalid`, time.Second)
	var compileErr *regexCompileError
	if !errors.As(err, &compileErr) || compileErr.timeout {
		t.Fatalf("compile [invalid: err = %v, want *regexCompileError with timeout=false", err)
	}

	release := make(chan struct{})
	previous := regexCompileFunc
	regexCompileFunc = func(pattern string) (*regexp.Regexp, error) {
		<-release
		return regexp.Compile(pattern)
	}
	t.Cleanup(func() { regexCompileFunc = previous })
	defer close(release)

	_, err = compileRegexWithTimeout(`abc`, 10*time.Millisecond)
	compileErr = nil
	if !errors.As(err, &compileErr) || !compileErr.timeout {
		t.Fatalf("blocked compile: err = %v, want *regexCompileError with timeout=true", err)
	}

	previousDeadline := regexCompileDeadline
	regexCompileDeadline = 10 * time.Millisecond
	t.Cleanup(func() { regexCompileDeadline = previousDeadline })
	_, err = ReconstructTransformer(TransformerRegexExtract, json.RawMessage(`{"pattern":"abc","group":0}`))
	compileErr = nil
	if !errors.As(err, &compileErr) || !compileErr.timeout {
		t.Fatalf("ReconstructTransformer with blocked compile: err = %v, want typed timeout", err)
	}
}

func TestRegexUnicodeIsRegexTransformerSpec(t *testing.T) {
	cases := []struct {
		spec TransformerSpec
		want bool
	}{
		{TransformerSpec{ID: TransformerRegexExtract}, true},
		{TransformerSpec{Name: "regex_extract_int"}, true},
		{TransformerSpec{ID: TransformerToLower, Name: "to_lower", Alias: "regex_extract"}, false},
	}
	for _, tc := range cases {
		if got := isRegexTransformerSpec(tc.spec); got != tc.want {
			t.Errorf("isRegexTransformerSpec(%+v) = %v, want %v", tc.spec, got, tc.want)
		}
	}
}
