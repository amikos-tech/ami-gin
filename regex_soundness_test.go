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

// TestRegexNeverPrunesAMatch pins issue #68. The first seven rows are regexp
// matches that v1.1.0 pruned because a composed literal demanded a substring
// the match does not hold. The last three guard behaviour that already held.
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
		{`(?i)sss`, "ſſſ"}, // #70: long s folds with s, ToLower keeps it
		{`(?i)µµµ`, "μμμ"}, // #70: micro sign folds with Greek mu
		{`(?i)kkk`, "KKK"}, // Kelvin sign lowers to k
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
		{`(foo|[0-9]+)baz`, "foobar"},
		{`error.*timeout`, "warning only"},
		{`Toyota|Tesla`, "Ford Mustang"},
		{`(abc.*|cde)fgh`, "xyzxyz"}, // fragments still prune
		{`(?i)Checkout`, "cart"},
		{`(?i)kkk`, "cart"}, // Kelvin orbit is safe under ToLower, still prunes
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

// TestRegexExpansionCap pins that exceeding maxLiteralExpansion yields nothing
// rather than a cut list; a cut list drops alternatives and prunes their matches.
func TestRegexExpansionCap(t *testing.T) {
	over := strings.Repeat("(aaa|bbb|ccc)", 5) // 243 combinations
	lits, err := ExtractLiterals(over)
	if err != nil || lits != nil {
		t.Fatalf("ExtractLiterals(%q) = %q, %v; want nil", over, lits, err)
	}
	if !regexKeepsMatch(t, over, strings.Repeat("ccc", 5)) {
		t.Errorf("Regex(%q) pruned a match after exceeding the cap", over)
	}

	under := strings.Repeat("(aaa|bbb|ccc)", 4) // 81 combinations
	lits, err = ExtractLiterals(under)
	if err != nil || len(lits) != 81 {
		t.Fatalf("ExtractLiterals(%q) returned %d literals, %v; want 81", under, len(lits), err)
	}
	if regexKeepsMatch(t, under, "ababab") {
		t.Errorf("Regex(%q) kept a non-match below the cap", under)
	}

	fragments := strings.Repeat("ab.*", maxLiteralExpansion+1)
	lits, err = ExtractLiterals(fragments)
	if err != nil || lits != nil {
		t.Fatalf("ExtractLiterals(%q) = %d literals, %v; want nil", fragments, len(lits), err)
	}
}

// genRegexPattern draws patterns from a small grammar over the alphabet "abc":
// literals, classes, wildcards, anchors, alternation with possibly empty
// branches, optional, star, plus, bounded repeats and groups.
func genRegexPattern(depth int) gopter.Gen {
	literal := gen.SliceOfN(3, gen.OneConstOf("a", "b", "c")).Map(func(parts []string) string {
		return strings.Join(parts, "")
	})
	shortLiteral := gen.OneConstOf("a", "b", "c", "ab", "bc", "sss", "kkk")
	atom := gen.OneGenOf(literal, literal, literal, literal, shortLiteral,
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

	genValue := gen.SliceOfN(8, gen.OneConstOf("a", "b", "c", "x", "A", "1", " ", "ſ", "K")).Map(func(parts []string) string {
		return strings.Join(parts, "")
	})

	properties.Property("regexp match implies Regex keeps the row group", prop.ForAll(
		func(pattern string, values []string) (bool, error) {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return false, err
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

	properties.Property("regexp match implies a literal is a substring", prop.ForAll(
		func(pattern string, values []string) (bool, error) {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return false, err
			}
			lits, err := ExtractLiterals(pattern)
			if err != nil {
				return false, err
			}
			for _, v := range values {
				if !re.MatchString(v) || len(lits) == 0 {
					continue
				}
				if !containsAnyFold(v, lits) {
					return false, fmt.Errorf("ExtractLiterals(%q) = %q, none in match %q", pattern, lits, v)
				}
			}
			return true, nil
		},
		genRegexPattern(3),
		gen.SliceOfN(6, genValue),
	))

	properties.TestingRun(t)
}

func containsAnyFold(value string, lits []string) bool {
	lower := strings.ToLower(value)
	for _, lit := range lits {
		if strings.Contains(lower, strings.ToLower(lit)) {
			return true
		}
	}
	return false
}
