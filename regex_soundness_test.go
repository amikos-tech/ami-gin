package gin

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// regexKeepsMatch builds a one-row-group index holding value and reports
// whether Regex(pattern) keeps that row group.
func regexKeepsMatch(t *testing.T, pattern, value string) bool {
	t.Helper()
	builder, err := NewBuilder(DefaultConfig(), 1)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	if err := builder.AddDocument(0, []byte(fmt.Sprintf(`{"v":%q}`, value))); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	idx := builder.Finalize()
	return !idx.Evaluate([]Predicate{Regex("$.v", pattern)}).IsEmpty()
}

// TestRegexNeverPrunesAMatch pins issue #68: every row is a regexp match that
// v1.1.0 pruned because a composed literal demanded a substring the match
// does not hold.
func TestRegexNeverPrunesAMatch(t *testing.T) {
	cases := []struct{ pattern, value string }{
		{`(foo|)bar`, "xbarx"},
		{`(foo|[0-9]+)bar`, "12bar"},
		{`(a|b*)cde`, "bcde"},
		{`ab+c`, "abbc"},
		{`(ab.*|cd)ef`, "abXXef"},
		{`(foo|x?)bar`, "zbar"},
		{`x(ab)+yzw`, "xababyzw"},
		{`foo(bar|baz)?qux`, "fooqux"},
		{`^check`, "checkout"},
		{`(?i)Checkout`, "CHECKOUT-v2"},
	}
	for _, c := range cases {
		t.Run(c.pattern, func(t *testing.T) {
			if !regexp.MustCompile(c.pattern).MatchString(c.value) {
				t.Fatalf("test bug: %q does not match %q", c.pattern, c.value)
			}
			if !regexKeepsMatch(t, c.pattern, c.value) {
				t.Errorf("Regex(%q) pruned the row group holding %q", c.pattern, c.value)
			}
		})
	}
}

// TestRegexStillPrunes guards the fix against the trivial "always AllRGs" answer.
func TestRegexStillPrunes(t *testing.T) {
	cases := []struct{ pattern, value string }{
		{`(foo|bar)baz`, "foobar"},
		{`(foo|[0-9]+)bar`, "foobaz"},
		{`error.*timeout`, "warning only"},
		{`Toyota|Tesla`, "Ford Mustang"},
	}
	for _, c := range cases {
		t.Run(c.pattern, func(t *testing.T) {
			if regexp.MustCompile(c.pattern).MatchString(c.value) {
				t.Fatalf("test bug: %q matches %q", c.pattern, c.value)
			}
			if regexKeepsMatch(t, c.pattern, c.value) {
				t.Errorf("Regex(%q) kept %q, expected a prune", c.pattern, c.value)
			}
		})
	}
}

// genRegexPattern draws patterns from a small grammar over the alphabet "abc":
// literals, classes, wildcards, anchors, alternation with possibly empty
// branches, optional, star, plus, bounded repeats and groups.
func genRegexPattern(depth int) gopter.Gen {
	literal := gen.SliceOfN(3, gen.OneConstOf("a", "b", "c")).Map(func(parts []string) string {
		return strings.Join(parts, "")
	})
	shortLiteral := gen.OneConstOf("a", "b", "c", "ab", "bc")
	atom := gen.OneGenOf(literal, literal, shortLiteral,
		gen.OneConstOf("[ab]", "[^c]", ".", "\\d", "^", "$", "\\b", ""))
	if depth == 0 {
		return atom
	}
	sub := genRegexPattern(depth - 1)
	return gen.OneGenOf(
		atom,
		gopter.CombineGens(sub, sub).Map(func(v []interface{}) string { return v[0].(string) + v[1].(string) }),
		gopter.CombineGens(sub, sub).Map(func(v []interface{}) string { return "(" + v[0].(string) + "|" + v[1].(string) + ")" }),
		sub.Map(func(s string) string { return "(" + s + ")?" }),
		sub.Map(func(s string) string { return "(" + s + ")*" }),
		sub.Map(func(s string) string { return "(" + s + ")+" }),
		sub.Map(func(s string) string { return "(" + s + "){0,2}" }),
		sub.Map(func(s string) string { return "(" + s + "){2,}" }),
		sub.Map(func(s string) string { return "(?i:" + s + ")" }),
	)
}

// TestPropertyRegexIsSuperset: for every generated pattern and value, a regexp
// match implies the row group survives Regex. Values are drawn over a slightly
// larger alphabet so decoys exist too.
func TestPropertyRegexIsSuperset(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParametersWithBudgets(400, 60))

	genValue := gen.SliceOfN(8, gen.OneConstOf("a", "b", "c", "x", "A", "1", " ")).Map(func(parts []string) string {
		return strings.Join(parts, "")
	})

	properties.Property("regexp match implies Regex keeps the row group", prop.ForAll(
		func(pattern string, values []string) (bool, error) {
			re, err := regexp.Compile(pattern)
			if err != nil {
				// The grammar can repeat an anchor; regexp rejects that and so
				// does syntax.Parse, so there is nothing to compare.
				return true, nil //nolint:nilerr // rejected pattern is vacuously sound
			}
			builder, err := NewBuilder(DefaultConfig(), len(values))
			if err != nil {
				return false, err
			}
			for rg, v := range values {
				if err := builder.AddDocument(DocID(rg), []byte(fmt.Sprintf(`{"v":%q}`, v))); err != nil {
					return false, err
				}
			}
			idx := builder.Finalize()
			kept := idx.Evaluate([]Predicate{Regex("$.v", pattern)})
			for rg, v := range values {
				if re.MatchString(v) && !kept.IsSet(rg) {
					lits, _ := ExtractLiterals(pattern)
					return false, fmt.Errorf("Regex(%q) pruned row group %d holding %q; literals %q", pattern, rg, v, lits)
				}
			}
			return true, nil
		},
		genRegexPattern(3),
		gen.SliceOfN(6, genValue),
	))

	properties.TestingRun(t)
}
