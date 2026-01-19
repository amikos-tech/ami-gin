package gin

import (
	"regexp/syntax"
)

const maxLiteralExpansion = 100 // Limit Cartesian product explosion

// ExtractLiterals extracts literal strings from a regex pattern that can be used
// for trigram-based candidate selection. Returns a slice of literal alternatives.
// For patterns like "foo|bar", returns ["foo", "bar"].
// For patterns like "(error|warn)_msg", returns ["error_msg", "warn_msg"] (combined).
func ExtractLiterals(pattern string) ([]string, error) {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil, err
	}
	re = re.Simplify()
	return extractCombinedLiterals(re), nil
}

// extractCombinedLiterals extracts literals and combines them properly.
// For Concat(Alternation, Literal), it produces the Cartesian product.
func extractCombinedLiterals(re *syntax.Regexp) []string {
	switch re.Op {
	case syntax.OpLiteral:
		return []string{string(re.Rune)}

	case syntax.OpConcat:
		return extractConcatLiterals(re.Sub)

	case syntax.OpAlternate:
		var result []string
		for _, sub := range re.Sub {
			result = append(result, extractCombinedLiterals(sub)...)
			if len(result) > maxLiteralExpansion {
				return result[:maxLiteralExpansion]
			}
		}
		return result

	case syntax.OpCapture:
		if len(re.Sub) > 0 {
			return extractCombinedLiterals(re.Sub[0])
		}
		return nil

	case syntax.OpPlus, syntax.OpRepeat:
		// + means 1 or more, so the sub-pattern is required
		if len(re.Sub) > 0 {
			return extractCombinedLiterals(re.Sub[0])
		}
		return nil

	case syntax.OpStar, syntax.OpQuest:
		// * and ? mean 0 or more, so the sub-pattern is optional
		// We can't rely on it for pruning
		return nil

	case syntax.OpCharClass, syntax.OpAnyCharNotNL, syntax.OpAnyChar,
		syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText, syntax.OpEndText,
		syntax.OpWordBoundary, syntax.OpNoWordBoundary,
		syntax.OpEmptyMatch, syntax.OpNoMatch:
		return nil

	default:
		return nil
	}
}

// extractConcatLiterals handles concatenation by building combined strings.
// For [Literal("a"), Alternation("b","c"), Literal("d")], returns ["abd", "acd"].
func extractConcatLiterals(subs []*syntax.Regexp) []string {
	// Start with empty prefix
	results := []string{""}

	for _, sub := range subs {
		subLiterals := extractCombinedLiterals(sub)

		if len(subLiterals) == 0 {
			// Non-extractable element (wildcard, char class, etc.)
			// Flush current results and start fresh after this gap
			if len(results) == 1 && results[0] == "" {
				continue
			}
			// Keep what we have, these become separate literals
			var newResults []string
			for _, r := range results {
				if r != "" {
					newResults = append(newResults, r)
				}
			}
			results = []string{""}
			if len(newResults) > 0 {
				// Return accumulated + continue extraction
				remaining := extractConcatLiterals(subs[indexOf(subs, sub)+1:])
				return append(newResults, remaining...)
			}
			continue
		}

		// Cartesian product: combine each current result with each sub-literal
		var newResults []string
		for _, prefix := range results {
			for _, suffix := range subLiterals {
				combined := prefix + suffix
				newResults = append(newResults, combined)
				if len(newResults) > maxLiteralExpansion {
					return newResults
				}
			}
		}
		results = newResults
	}

	// Filter out empty strings
	var filtered []string
	for _, r := range results {
		if r != "" {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func indexOf(subs []*syntax.Regexp, target *syntax.Regexp) int {
	for i, s := range subs {
		if s == target {
			return i
		}
	}
	return -1
}

// RegexLiteralInfo contains extracted information from a regex pattern
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
		Literals:    extractCombinedLiterals(re),
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
