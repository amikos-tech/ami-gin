package gin

import (
	"regexp/syntax"
	"unicode"
)

const maxLiteralExpansion = 100 // Limit Cartesian product explosion

// ExtractLiterals extracts literal strings from a regex pattern for
// trigram-based candidate selection. Every match of the pattern contains at
// least one returned literal as a substring, so callers must treat the list
// as OR. A case-folded literal run ((?i)) is dropped entirely — contributing
// no literal — when any rune in it has a unicode.SimpleFold orbit member
// whose unicode.ToLower differs from that rune's own ToLower; the drop
// applies to the whole merged literal node, not just the offending rune (see
// foldsUnderToLower). Returns nil when no literal is guaranteed or the
// expansion exceeds maxLiteralExpansion.
//
//	"foo|bar"          -> ["foo", "bar"]
//	"(error|warn)_msg" -> ["error_msg", "warn_msg"]
//	"foo.*bar"         -> ["foo", "bar"]
func ExtractLiterals(pattern string) ([]string, error) {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil, err
	}
	re = re.Simplify()
	return extractLiterals(re).literals, nil
}

// literalSet is the literal evidence one regex node contributes: every match
// of the node contains at least one entry as a substring (empty = no evidence).
// whole means the node matches exactly the entries, so a concatenation may glue
// them to its neighbours; gluing a fragment would demand a substring a real
// match need not hold.
type literalSet struct {
	literals []string
	whole    bool
}

func extractLiterals(re *syntax.Regexp) literalSet {
	switch re.Op {
	case syntax.OpLiteral:
		if re.Flags&syntax.FoldCase != 0 && !foldsUnderToLower(re.Rune) {
			return literalSet{}
		}
		return literalSet{literals: []string{string(re.Rune)}, whole: true}

	case syntax.OpConcat:
		return extractConcatLiterals(re.Sub)

	case syntax.OpAlternate:
		// A branch without evidence admits matches the others do not describe.
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
		return extractLiterals(re.Sub[0])

	case syntax.OpPlus:
		// One occurrence is required, but never glue through a repetition:
		// "ab+c" matches "abbc", which holds no "abc". Simplify expands OpRepeat.
		return literalSet{literals: extractLiterals(re.Sub[0]).literals}

	default:
		// OpStar, OpQuest, classes, anchors: nothing is guaranteed.
		return literalSet{}
	}
}

// extractConcatLiterals cross-joins consecutive whole nodes into one run
// ("(error|warn)_msg" -> "error_msg", "warn_msg"). A fragmentary or empty node
// ends the run; finished runs and fragments are returned as one flat list.
func extractConcatLiterals(subs []*syntax.Regexp) literalSet {
	var groups, run []string
	whole := true

	for _, sub := range subs {
		part := extractLiterals(sub)
		if part.whole {
			run = crossJoin(run, part.literals)
			if len(run) > maxLiteralExpansion {
				return literalSet{}
			}
			continue
		}
		whole = false
		groups = append(groups, run...)
		groups = append(groups, part.literals...)
		run = nil
		if len(groups) > maxLiteralExpansion {
			return literalSet{}
		}
	}
	return literalSet{literals: append(groups, run...), whole: whole}
}

// foldsUnderToLower reports whether strings.ToLower, which TrigramIndex applies
// to both sides, equates every rune in a (?i) literal with its whole fold orbit.
// "s" fails: regexp folds it with "ſ" (long s), which ToLower keeps as is.
func foldsUnderToLower(runes []rune) bool {
	for _, r := range runes {
		lower := unicode.ToLower(r)
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			if unicode.ToLower(f) != lower {
				return false
			}
		}
	}
	return true
}

func crossJoin(prefixes, suffixes []string) []string {
	if len(prefixes) == 0 {
		return suffixes
	}
	out := make([]string, 0, len(prefixes)*len(suffixes))
	for _, prefix := range prefixes {
		for _, suffix := range suffixes {
			out = append(out, prefix+suffix)
		}
	}
	return out
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
