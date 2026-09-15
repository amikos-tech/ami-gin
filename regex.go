package gin

import (
	"regexp/syntax"
)

const maxLiteralExpansion = 100 // Limit Cartesian product explosion

// ExtractLiterals extracts literal strings from a regex pattern that can be used
// for trigram-based candidate selection. Returns a slice of literal alternatives:
// every match of the pattern contains at least one of them as a substring.
// For patterns like "foo|bar", returns ["foo", "bar"].
// For patterns like "(error|warn)_msg", returns ["error_msg", "warn_msg"] (combined).
// For patterns like "foo.*bar", returns ["foo", "bar"] (fragments, each required).
// The list does not say which entries are alternatives and which are all
// required, so callers must treat it as OR. Case-folded literals ((?i)) come
// back in one case; compare case-insensitively. Returns nil when no literal is
// guaranteed or when the expansion exceeds maxLiteralExpansion, so the caller
// cannot prune.
func ExtractLiterals(pattern string) ([]string, error) {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil, err
	}
	re = re.Simplify()
	return extractLiterals(re).literals, nil
}

// literalSet is the literal evidence one regex node contributes.
//
// literals: every match of the node contains at least one entry as a substring.
// An empty slice means the node proves nothing.
//
// whole: the node matches exactly the entries, so a concatenation may glue them
// to its neighbours. A fragmentary set (whole == false) breaks the product:
// gluing a fragment to a neighbour would demand a substring that a real match
// need not contain, and the trigram search would then prune a matching row group.
type literalSet struct {
	literals []string
	whole    bool
}

func extractLiterals(re *syntax.Regexp) literalSet {
	switch re.Op {
	case syntax.OpLiteral:
		return literalSet{literals: []string{string(re.Rune)}, whole: true}

	case syntax.OpConcat:
		return extractConcatLiterals(re.Sub)

	case syntax.OpAlternate:
		// A branch without evidence admits matches the other branches do not
		// describe, so the whole alternation proves nothing.
		out := literalSet{whole: true}
		for _, sub := range re.Sub {
			branch := extractLiterals(sub)
			if len(branch.literals) == 0 {
				return literalSet{}
			}
			out.literals = append(out.literals, branch.literals...)
			out.whole = out.whole && branch.whole
			if len(out.literals) > maxLiteralExpansion {
				return literalSet{}
			}
		}
		return out

	case syntax.OpCapture:
		if len(re.Sub) > 0 {
			return extractLiterals(re.Sub[0])
		}
		return literalSet{}

	case syntax.OpPlus:
		// At least one occurrence is required, so its literals are contained,
		// but the product must not glue through a repetition ("ab+c" matches
		// "abbc", which holds no "abc"). Simplify expands OpRepeat before this.
		if len(re.Sub) == 0 {
			return literalSet{}
		}
		return literalSet{literals: extractLiterals(re.Sub[0]).literals}

	default:
		// OpStar, OpQuest: optional, cannot prune on it.
		return literalSet{}
	}
}

// extractConcatLiterals multiplies consecutive whole nodes into combined
// literals ("(error|warn)_msg" -> "error_msg", "warn_msg"). A node that is
// fragmentary or proves nothing ends the run; the runs and the fragments become
// separate required groups, returned as one flat list. The list is whole only
// when every node was whole.
func extractConcatLiterals(subs []*syntax.Regexp) literalSet {
	var groups []string
	product := []string{""}
	whole := true

	flush := func() {
		for _, p := range product {
			if p != "" {
				groups = append(groups, p)
			}
		}
		product = []string{""}
	}

	for _, sub := range subs {
		part := extractLiterals(sub)
		if part.whole {
			next := make([]string, 0, len(product)*len(part.literals))
			for _, prefix := range product {
				for _, lit := range part.literals {
					next = append(next, prefix+lit)
				}
			}
			if len(next) > maxLiteralExpansion {
				return literalSet{}
			}
			product = next
			continue
		}
		whole = false
		flush()
		groups = append(groups, part.literals...)
		if len(groups) > maxLiteralExpansion {
			return literalSet{}
		}
	}
	flush()
	return literalSet{literals: groups, whole: whole}
}

type RegexLiteralInfo struct {
	Literals    []string // Extracted literal strings
	HasWildcard bool     // Pattern contains unbounded wildcards
	MinLength   int      // Minimum length of any literal
}

// AnalyzeRegex extracts literals and metadata from a regex pattern
func AnalyzeRegex(pattern string) (*RegexLiteralInfo, error) {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil, err
	}
	re = re.Simplify()

	info := &RegexLiteralInfo{
		Literals:    extractLiterals(re).literals,
		HasWildcard: hasUnboundedWildcard(re),
	}

	// Calculate minimum literal length
	info.MinLength = -1
	for _, lit := range info.Literals {
		if info.MinLength < 0 || len(lit) < info.MinLength {
			info.MinLength = len(lit)
		}
	}
	if info.MinLength < 0 {
		info.MinLength = 0
	}

	return info, nil
}

func hasUnboundedWildcard(re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpStar, syntax.OpPlus:
		// Check if it's .* or .+
		if len(re.Sub) > 0 {
			sub := re.Sub[0]
			if sub.Op == syntax.OpAnyChar || sub.Op == syntax.OpAnyCharNotNL {
				return true
			}
		}
	case syntax.OpConcat, syntax.OpAlternate, syntax.OpCapture:
		for _, sub := range re.Sub {
			if hasUnboundedWildcard(sub) {
				return true
			}
		}
	}
	return false
}
