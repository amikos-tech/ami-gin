package gin

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp/syntax"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/pkg/errors"
)

// regexUnicodeVerifierBypasses are patterns where an earlier verifier scan
// ended a bracket class at a different ']' than regexp/syntax and then did not
// see a later flag group.
var regexUnicodeVerifierBypasses = []string{
	`[]](?i)(ƛ+)`,
	`[^]](?i)(ƛ+)`,
	`[]-a](?i)x`,
	`[^]-a](?i)x`,
	`[A-[:](?i)(ƛ+):]`,
	`[A-[:](?i:ƛ):]`,
}

// TestRegexUnicodeClassifyMarkedClassBoundary: a marked spec whose pattern
// applies (?i) after a class with a leading ']' or a '[' range endpoint is
// unverified, so the companion returns every row group.
func TestRegexUnicodeClassifyMarkedClassBoundary(t *testing.T) {
	specs := make([]regexUnicodeRawSpec, 0, len(regexUnicodeVerifierBypasses)+1)
	want := make(map[string]DegradeReason, len(regexUnicodeVerifierBypasses))
	for i, pattern := range regexUnicodeVerifierBypasses {
		params, err := json.Marshal(regexWireParams{Pattern: pattern, Group: 0, SourcePattern: `(?i)(ƛ+)`, FrozenUnicode: "17.0.0"})
		if err != nil {
			t.Fatal(err)
		}
		alias := "b" + string(rune('0'+i))
		specs = append(specs, regexUnicodeRawSpec{"$.name", alias, TransformerRegexExtract, string(params)})
		want["$.name#"+alias] = DegradeReasonUnverifiedFrozenRegex
	}
	specs = append(specs, regexUnicodeSafeNum)
	idx := mustDecodeRegexUnicode(t, regexUnicodeRawIndex(t, specs...))
	requireRegexUnicodeDegraded(t, idx, want)
	for i := range regexUnicodeVerifierBypasses {
		requireRows(t, idx, EQ("$.name", As("b"+string(rune('0'+i)), "]ƛ")), []int{0, 1, 2, 3})
	}
	requireRows(t, idx, EQ("$.code", As("num", float64(8))), []int{1})
}

// regexUnicodeAppliesTableOp is the differential oracle for the verifier. It
// asks regexp/syntax, not a scanner, whether a pattern that parses applies a
// \p or \P class or a flag group that holds i:
//   - a \p or \P class makes the parse fail with ErrInvalidEscape when
//     UnicodeGroups is off;
//   - for each "(?" flag run that holds an i, the i is replaced with Z. Only
//     a flag group that Go applies then fails with ErrInvalidPerlOp. Inside a
//     class or a \Q...\E block the Z is a literal (at most an invalid range).
//
// It never looks at FoldCase in the tree, so a legitimate [Aa] or [ƛꟜ]
// class that the parser turns into a FoldCase literal is not reported.
func regexUnicodeAppliesTableOp(pattern string) (string, bool) {
	var se *syntax.Error
	if _, err := syntax.Parse(pattern, syntax.Perl&^syntax.UnicodeGroups); errors.As(err, &se) && se.Code == syntax.ErrInvalidEscape {
		return "Unicode class", true
	}
	for j := 0; j+2 < len(pattern); j++ {
		if !strings.HasPrefix(pattern[j:], "(?") {
			continue
		}
		k := j + 2
		for k < len(pattern) && pattern[k] != 'i' && strings.IndexByte("msU-", pattern[k]) >= 0 {
			k++
		}
		if k >= len(pattern) || pattern[k] != 'i' {
			continue
		}
		mutated := pattern[:k] + "Z" + pattern[k+1:]
		if _, err := syntax.Parse(mutated, syntax.Perl); errors.As(err, &se) && se.Code == syntax.ErrInvalidPerlOp {
			return fmt.Sprintf("flag group with i at offset %d", j), true
		}
	}
	return "", false
}

// checkRegexUnicodeVerifier fails when the verifier passes a pattern that Go
// parses and that applies a Unicode-table operation.
func checkRegexUnicodeVerifier(t *testing.T, pattern string) (checked, flagged bool) {
	t.Helper()
	if _, err := syntax.Parse(pattern, syntax.Perl); err != nil {
		return false, false
	}
	what, applied := regexUnicodeAppliesTableOp(pattern)
	if applied && verifyFrozenRegexText(pattern) == nil {
		t.Errorf("verifier passes %q, but Go applies a %s", pattern, what)
	}
	return true, applied
}

// regexUnicodeVerifierPieces build the differential patterns. They hold the
// class, escape and flag-group tokens where a lexical scan can disagree with
// the parser.
var regexUnicodeVerifierPieces = []string{
	`[`, `]`, `^`, `-`, `:`, `[:alpha:]`, `[:`, `:]`, `\`, `\\`, `\Q`, `\E`,
	`(?i)`, `(?i:`, `(?s)`, `(?-i)`, `(?`, `(`, `)`, `?`, `i`, `a`, `A`, `ƛ`,
	`\p{Lu}`, `\pL`, `\d`, `\x{5d}`, `\x5d`, `\135`, `[]`, `[^`, `]-`, `|`, `*`,
}

func randomRegexUnicodeVerifierPattern(r *rand.Rand) string {
	var b strings.Builder
	for n := 1 + r.Intn(10); n > 0; n-- {
		b.WriteString(regexUnicodeVerifierPieces[r.Intn(len(regexUnicodeVerifierPieces))])
	}
	return b.String()
}

// TestRegexUnicodeVerifierDifferential is a bounded, deterministic run of the
// verifier against the parser oracle.
func TestRegexUnicodeVerifierDifferential(t *testing.T) {
	for _, pattern := range regexUnicodeVerifierBypasses {
		if _, applied := regexUnicodeAppliesTableOp(pattern); !applied {
			t.Fatalf("oracle misses the bypass %q", pattern)
		}
		checkRegexUnicodeVerifier(t, pattern)
	}
	for _, pattern := range []string{`[Aa]`, `[ƛꟜ]`, `[(?i)]`, `\Q(?i)\E`, `[i-i]`, `[(?i-i)]`, `[A-[(?i)]`} {
		if what, applied := regexUnicodeAppliesTableOp(pattern); applied {
			t.Fatalf("oracle reports %s for %q", what, pattern)
		}
	}
	r := rand.New(rand.NewSource(115))
	iterations := 100000
	if testing.Short() {
		iterations = 20000
	}
	checked, flagged := 0, 0
	for n := 0; n < iterations; n++ {
		c, f := checkRegexUnicodeVerifier(t, randomRegexUnicodeVerifierPattern(r))
		if c {
			checked++
		}
		if f {
			flagged++
		}
		if t.Failed() {
			return
		}
	}
	if checked < iterations/20 || flagged < iterations/100 {
		t.Fatalf("weak run: %d of %d patterns parse, %d apply a table operation", checked, iterations, flagged)
	}
}

func FuzzRegexUnicodeVerifier(f *testing.F) {
	for _, s := range regexUnicodeVerifierBypasses {
		f.Add(s)
	}
	for _, s := range []string{`[]p]`, `[^]\]]`, `[[:alpha:]](?i)a`, `\Q[\E(?i)x`, `(\\Q[a])`, `[\x{5d}(?i)]`, `[\135-[:](?i)x:]`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, pattern string) {
		if len(pattern) > 64 || !utf8.ValidString(pattern) {
			return
		}
		checkRegexUnicodeVerifier(t, pattern)
	})
}
