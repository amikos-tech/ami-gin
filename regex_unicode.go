package gin

import (
	"bytes"
	"encoding/json"
	"regexp/syntax"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/pkg/errors"
)

// Regex companions and Unicode tables (#115).
//
// Go's regexp resolves \p{..}, \P{..} and case folding with the Unicode tables
// of the toolchain that compiles the pattern. A builder and a reader on
// different toolchains can then extract different companion values from the
// same raw input. The builder therefore stores a "frozen" pattern: the parsed
// tree with every Unicode class expanded to explicit ranges and every
// case-folded literal expanded to an explicit class of its fold orbit, using
// the builder's tables. A reader that compiles the frozen text matches exactly
// like the builder.

// Reserved regex params keys. The builder owns them: registration rejects
// params that already hold either key.
const (
	regexParamSourcePattern = "source_pattern"
	regexParamFrozenUnicode = "frozen_unicode"
)

// regexWireParams is the stored params layout of RegexExtract and
// RegexExtractInt. Pattern is the executable pattern (frozen text for a
// frozen spec). SourcePattern and FrozenUnicode are set only for a frozen
// spec; they are omitted for patterns the builder did not freeze, so those
// params keep their exact bytes. The exported RegexParams type is unchanged.
type regexWireParams struct {
	Pattern       string `json:"pattern"`
	Group         int    `json:"group"`
	SourcePattern string `json:"source_pattern,omitempty"`
	FrozenUnicode string `json:"frozen_unicode,omitempty"`
}

// isRegexTransformerSpec reports whether spec describes RegexExtract or
// RegexExtractInt. It matches on the transformer ID OR the name, never on the
// alias, like isLoweringSpec.
func isRegexTransformerSpec(spec TransformerSpec) bool {
	switch spec.ID {
	case TransformerRegexExtract, TransformerRegexExtractInt:
		return true
	}
	switch spec.Name {
	case transformerNames[TransformerRegexExtract], transformerNames[TransformerRegexExtractInt]:
		return true
	}
	return false
}

// regexReservedParamMembers scans the top-level members of raw in their
// order and returns the reserved keys it holds. Keys match
// case-insensitively, like encoding/json matches struct fields. A reserved
// key present more than once (also with another case, or with the same
// spelling) is an error: a map decode would keep only one of them, and which
// one would depend on map order. The flags are still set on that error.
// JSON null holds no members.
func regexReservedParamMembers(raw json.RawMessage) (source, frozen json.RawMessage, hasSource, hasFrozen bool, err error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, nil, false, false, errors.Wrap(err, "scan regex params")
	}
	if tok == nil {
		return nil, nil, false, false, nil
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, nil, false, false, errors.New("regex params must be a JSON object")
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, nil, hasSource, hasFrozen, errors.Wrap(err, "scan regex params")
		}
		key, _ := keyTok.(string)
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, nil, hasSource, hasFrozen, errors.Wrap(err, "scan regex params")
		}
		switch {
		case strings.EqualFold(key, regexParamSourcePattern):
			if hasSource {
				return nil, nil, true, hasFrozen, errors.Errorf("regex params hold %s more than once", regexParamSourcePattern)
			}
			source, hasSource = value, true
		case strings.EqualFold(key, regexParamFrozenUnicode):
			if hasFrozen {
				return nil, nil, hasSource, true, errors.Errorf("regex params hold %s more than once", regexParamFrozenUnicode)
			}
			frozen, hasFrozen = value, true
		}
	}
	return source, frozen, hasSource, hasFrozen, nil
}

// regexParamsHoldReservedKeys reports whether raw is a JSON object that holds
// source_pattern or frozen_unicode, with any value (also "" or null).
func regexParamsHoldReservedKeys(raw json.RawMessage) bool {
	_, _, hasSource, hasFrozen, _ := regexReservedParamMembers(raw)
	return hasSource || hasFrozen
}

// decodeRegexWireParams decodes stored regex params. marked reports a frozen
// spec (source_pattern or frozen_unicode present). A marked spec must hold
// both keys, each once and as a non-empty JSON string. Any frozen_unicode
// value is accepted: an unfamiliar Unicode version never disables a frozen
// pattern.
func decodeRegexWireParams(raw json.RawMessage) (regexWireParams, bool, error) {
	var params regexWireParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return regexWireParams{}, false, errors.Wrap(err, "unmarshal regex params")
	}
	if params.Group < 0 {
		return regexWireParams{}, false, errors.New("regex group must be non-negative")
	}
	source, frozen, hasSource, hasFrozen, err := regexReservedParamMembers(raw)
	if err != nil {
		return regexWireParams{}, false, err
	}
	if !hasSource && !hasFrozen {
		return params, false, nil
	}
	if !hasSource || !hasFrozen {
		return regexWireParams{}, false, errors.Errorf("incomplete frozen metadata: %s and %s must both be present", regexParamSourcePattern, regexParamFrozenUnicode)
	}
	for _, field := range []struct {
		name  string
		value json.RawMessage
	}{{regexParamSourcePattern, source}, {regexParamFrozenUnicode, frozen}} {
		var s string
		if len(field.value) == 0 || field.value[0] != '"' || json.Unmarshal(field.value, &s) != nil || s == "" {
			return regexWireParams{}, false, errors.Errorf("incomplete frozen metadata: %s must be a non-empty string", field.name)
		}
	}
	return params, true, nil
}

// regexTableSensitive reports whether pattern may depend on the Unicode
// tables of the toolchain that compiles it. It is conservative: false
// positives only cost space (a pattern is frozen without need) or pruning (a
// legacy companion degrades). It is the source rule OR the tree rule.
func regexTableSensitive(pattern string) bool {
	if regexSourceSensitive(pattern) {
		return true
	}
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return true
	}
	return regexTreeSensitive(re)
}

// regexSourceSensitive is the lexical source rule. It flags any \p or \P
// escape, and any pattern that has an i flag in some flag group (scope is not
// tracked, so (?-i) counts too) plus a code point >= 0x80, written literally
// or as an escape (\xHH, \x{..}, octal). The tree rule cannot see these cases
// alone, because syntax.Parse resolves classes with the toolchain tables
// before the tree exists.
func regexSourceSensitive(pattern string) bool {
	hasIFlag := false
	hasNonASCII := false
	for i := 0; i < len(pattern); {
		r, size := utf8.DecodeRuneInString(pattern[i:])
		if r >= utf8.RuneSelf {
			hasNonASCII = true
			i += size
			continue
		}
		switch r {
		case '\\':
			i++
			if i >= len(pattern) {
				continue
			}
			next, nextSize := utf8.DecodeRuneInString(pattern[i:])
			i += nextSize
			switch {
			case next == 'p' || next == 'P':
				return true
			case next >= utf8.RuneSelf:
				hasNonASCII = true
			case next == 'x':
				value, consumed := scanHexEscape(pattern[i:])
				i += consumed
				if value >= 0x80 {
					hasNonASCII = true
				}
			case next >= '0' && next <= '7':
				value := int(next - '0')
				for n := 0; n < 2 && i < len(pattern) && pattern[i] >= '0' && pattern[i] <= '7'; n++ {
					value = value*8 + int(pattern[i]-'0')
					i++
				}
				if value >= 0x80 {
					hasNonASCII = true
				}
			}
		case '(':
			i++
			if strings.HasPrefix(pattern[i:], "?") {
				if flags, ok := scanFlagGroup(pattern[i+1:]); ok && strings.ContainsRune(flags, 'i') {
					hasIFlag = true
				}
			}
		default:
			i++
		}
	}
	return hasIFlag && hasNonASCII
}

// scanHexEscape reads the digits after \x: either {hex...} or two hex digits.
// It returns the value (saturated) and the bytes consumed.
func scanHexEscape(s string) (int, int) {
	value := 0
	add := func(c byte) bool {
		var d int
		switch {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case c >= 'a' && c <= 'f':
			d = int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = int(c-'A') + 10
		default:
			return false
		}
		if value <= unicode.MaxRune {
			value = value*16 + d
		}
		return true
	}
	if strings.HasPrefix(s, "{") {
		i := 1
		for i < len(s) && add(s[i]) {
			i++
		}
		if i < len(s) && s[i] == '}' {
			i++
		}
		return value, i
	}
	i := 0
	for i < len(s) && i < 2 && add(s[i]) {
		i++
	}
	return value, i
}

// scanFlagGroup reads the text after "(?". It reports the flag characters and
// true when the text is a flag group: only characters from "imsU-", ended by
// ')' or ':'. "(?P<", "(?<" and a bare "(?:" are not flag groups.
func scanFlagGroup(s string) (string, bool) {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case 'i', 'm', 's', 'U', '-':
			continue
		case ')', ':':
			return s[:i], i > 0
		default:
			return "", false
		}
	}
	return "", false
}

// regexTreeSensitive is the tree rule (spike 008): a FoldCase literal with a
// non-ASCII rune (its fold orbit can grow), or a char class with a non-ASCII
// range other than a negated-ASCII tail [lo..MaxRune] with lo <= 0x80, and
// other than the two stable ASCII fold partners U+017F and U+212A.
func regexTreeSensitive(re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpLiteral:
		if re.Flags&syntax.FoldCase != 0 {
			for _, r := range re.Rune {
				if r >= utf8.RuneSelf {
					return true
				}
			}
		}
	case syntax.OpCharClass:
		for i := 0; i+1 < len(re.Rune); i += 2 {
			lo, hi := re.Rune[i], re.Rune[i+1]
			if hi < utf8.RuneSelf {
				continue
			}
			if hi == unicode.MaxRune && lo <= utf8.RuneSelf {
				continue
			}
			if lo == hi && (lo == 0x17F || lo == 0x212A) {
				continue
			}
			return true
		}
	}
	for _, sub := range re.Sub {
		if regexTreeSensitive(sub) {
			return true
		}
	}
	return false
}

// verifyFrozenRegexText checks that an executable (frozen) pattern requests
// no Unicode-table operation. It is lexical and runs before any parse: a
// backslash consumes the next rune, bracket classes are tracked so their
// content is literal, and it fails on a \p or \P escape anywhere or on an i in
// any flag group outside a class. It also fails on a \Q escape: String never
// writes one, and the quoted text would hide a later flag group from this
// scan. An escaped backslash followed by Q (\\Q) is a literal and passes.
// Explicit non-ASCII literals and ranges pass. It never parses, never
// regenerates from source_pattern and never re-freezes.
func verifyFrozenRegexText(pattern string) error {
	inClass := false
	classStart := 0
	for i := 0; i < len(pattern); {
		r, size := utf8.DecodeRuneInString(pattern[i:])
		switch {
		case r == '\\':
			i += size
			if i >= len(pattern) {
				return nil
			}
			next, nextSize := utf8.DecodeRuneInString(pattern[i:])
			if next == 'p' || next == 'P' {
				return errors.Errorf("frozen regex requests a Unicode class at offset %d", i-1)
			}
			if next == 'Q' {
				return errors.Errorf("frozen regex holds a \\Q quoted literal at offset %d", i-1)
			}
			i += nextSize
		case inClass:
			switch {
			case r == ']' && i > classStart:
				inClass = false
				i += size
			case r == '[' && strings.HasPrefix(pattern[i:], "[:"):
				if end := strings.Index(pattern[i+2:], ":]"); end >= 0 {
					i += 2 + end + 2
				} else {
					i += size
				}
			default:
				i += size
			}
		case r == '[':
			inClass = true
			i += size
			if strings.HasPrefix(pattern[i:], "^") {
				i++
			}
			// A ']' right after '[' or '[^' is a literal.
			classStart = i + 1
			if !strings.HasPrefix(pattern[i:], "]") {
				classStart = i
			}
		case r == '(' && strings.HasPrefix(pattern[i:], "(?"):
			if flags, ok := scanFlagGroup(pattern[i+2:]); ok && strings.ContainsRune(flags, 'i') {
				return errors.Errorf("frozen regex sets the case-insensitive flag at offset %d", i)
			}
			i += size
		default:
			i += size
		}
	}
	return nil
}

// freezeRegexPattern rewrites pattern so its meaning no longer depends on the
// reader's Unicode tables. syntax.Parse already expands \p classes to explicit
// ranges with this toolchain's tables; case-folded literals are expanded here
// into explicit classes of this toolchain's fold orbit. The result is
// rendered with (*syntax.Regexp).String.
//
// budget bounds the JSON-escaped length of the result. A lower bound of the
// rendered length is checked before rendering, so memory stays within a
// constant factor of budget.
//
// The text is not idempotent after one pass (the stdlib re-factors String
// output), but repeated freezing reaches a fixed point. Freeze once at
// registration and store the text; never re-freeze to compare.
func freezeRegexPattern(pattern string, budget int) (string, error) {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return "", errors.Wrap(err, "parse regex pattern")
	}
	re = freezeRegexNode(re)
	// The JSON form adds two quotes to the rendered text.
	if lower := frozenRegexLowerBound(re, budget) + 2; lower > budget {
		return "", errors.Errorf("frozen regex needs at least %d bytes, more than the limit of %d bytes", lower, budget)
	}
	frozen := re.String()
	encoded, err := json.Marshal(frozen)
	if err != nil {
		return "", errors.Wrap(err, "marshal frozen regex")
	}
	if len(encoded) > budget {
		return "", errors.Errorf("frozen regex is %d bytes, more than the limit of %d bytes", len(encoded), budget)
	}
	return frozen, nil
}

// frozenRegexLowerBound returns a lower bound of the rendered byte length of
// re, as (*syntax.Regexp).String writes it. It follows the char class layout
// of String (gaps for a class that holds 0 and MaxRune, no '-' between two
// adjacent runes) and counts each rune as at least its UTF-8 length, because
// String writes a rune either as itself (perhaps after a backslash) or as a
// longer escape. JSON encoding only adds bytes. The bound therefore never
// exceeds the real size, and a pattern whose frozen text fits is never
// rejected. It stops counting once the bound passes limit.
func frozenRegexLowerBound(re *syntax.Regexp, limit int) int {
	total := 0
	switch re.Op {
	case syntax.OpCharClass:
		total += 2
		addRange := func(lo, hi rune) {
			total += regexRuneLowerBound(lo)
			if lo != hi {
				if hi != lo+1 {
					total++
				}
				total += regexRuneLowerBound(hi)
			}
		}
		switch {
		case len(re.Rune) == 0:
			total += len(`^\x00-\x{10FFFF}`)
		case re.Rune[0] == 0 && re.Rune[len(re.Rune)-1] == unicode.MaxRune && len(re.Rune) > 2:
			total++
			for i := 1; i+1 < len(re.Rune)-1 && total <= limit; i += 2 {
				addRange(re.Rune[i]+1, re.Rune[i+1]-1)
			}
		default:
			for i := 0; i+1 < len(re.Rune) && total <= limit; i += 2 {
				addRange(re.Rune[i], re.Rune[i+1])
			}
		}
	case syntax.OpLiteral:
		for _, r := range re.Rune {
			total += regexRuneLowerBound(r)
		}
	}
	for _, sub := range re.Sub {
		if total > limit {
			return total
		}
		total += frozenRegexLowerBound(sub, limit-total)
	}
	return total
}

// regexRuneLowerBound is the fewest bytes String uses to write r: its UTF-8
// length, or 1 for a rune that has no UTF-8 form (String escapes it).
func regexRuneLowerBound(r rune) int {
	if n := utf8.RuneLen(r); n > 0 {
		return n
	}
	return 1
}

// regexFoldOrbit returns the sorted SimpleFold orbit of r with this
// toolchain's tables.
func regexFoldOrbit(r rune) []rune {
	orbit := []rune{r}
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		orbit = append(orbit, f)
	}
	sort.Slice(orbit, func(i, j int) bool { return orbit[i] < orbit[j] })
	return orbit
}

// freezeRegexNode replaces every FoldCase literal with plain literals and
// explicit fold-orbit classes. Adjacent orbit runes merge into ranges, the
// canonical form, so the text reaches a fixed point.
func freezeRegexNode(re *syntax.Regexp) *syntax.Regexp {
	for i, sub := range re.Sub {
		re.Sub[i] = freezeRegexNode(sub)
	}
	if re.Op != syntax.OpLiteral || re.Flags&syntax.FoldCase == 0 {
		return re
	}
	base := re.Flags &^ syntax.FoldCase
	var parts []*syntax.Regexp
	var run []rune
	flush := func() {
		if len(run) > 0 {
			parts = append(parts, &syntax.Regexp{Op: syntax.OpLiteral, Flags: base, Rune: run})
			run = nil
		}
	}
	for _, r := range re.Rune {
		orbit := regexFoldOrbit(r)
		if len(orbit) == 1 {
			run = append(run, r)
			continue
		}
		flush()
		class := make([]rune, 0, 2*len(orbit))
		for _, o := range orbit {
			if n := len(class); n > 0 && class[n-1]+1 == o {
				class[n-1] = o
				continue
			}
			class = append(class, o, o)
		}
		parts = append(parts, &syntax.Regexp{Op: syntax.OpCharClass, Flags: base, Rune: class})
	}
	flush()
	if len(parts) == 1 {
		return parts[0]
	}
	return &syntax.Regexp{Op: syntax.OpConcat, Flags: base, Sub: parts}
}

// freezeRegexTransformerParams runs at registration of RegexExtract and
// RegexExtractInt (never on Decode). It rejects the builder-owned keys
// source_pattern and frozen_unicode. A pattern the detector does not flag
// keeps the caller's params bytes exactly. A flagged pattern is frozen with
// this toolchain's tables, and the params become the frozen pattern plus the
// original pattern and unicode.Version.
func freezeRegexTransformerParams(params []byte) ([]byte, error) {
	if regexParamsHoldReservedKeys(params) {
		return nil, errors.Errorf("regex params must not set %s or %s: the builder owns these fields", regexParamSourcePattern, regexParamFrozenUnicode)
	}
	wire, _, err := decodeRegexWireParams(params)
	if err != nil {
		return nil, err
	}
	if !regexTableSensitive(wire.Pattern) {
		return params, nil
	}
	frozen, err := freezeRegexPattern(wire.Pattern, maxConfigSize)
	if err != nil {
		return nil, errors.Wrap(err, "freeze regex pattern")
	}
	if err := verifyFrozenRegexText(frozen); err != nil {
		return nil, errors.Wrapf(err, "frozen regex %q fails verification", frozen)
	}
	frozenParams, err := json.Marshal(regexWireParams{
		Pattern:       frozen,
		Group:         wire.Group,
		SourcePattern: wire.Pattern,
		FrozenUnicode: unicode.Version,
	})
	if err != nil {
		return nil, errors.Wrap(err, "marshal frozen regex params")
	}
	return frozenParams, nil
}

// companionDegradation records why a companion returns every row group. It is
// index-owned and never stored: every Decode classifies again.
type companionDegradation struct {
	reason DegradeReason
	detail string
	// placeholder is true when the stored pattern did not compile, so the
	// companion function is regexDegradedPlaceholder and never produces a
	// value. It is tracked apart from reason: a marked spec that fails the
	// verifier and also fails to compile reports unverified_frozen_regex.
	// NewBuilder rejects a config with such a companion.
	placeholder bool
}

// regexDegradedPlaceholder stands in for a regex companion whose stored
// pattern did not compile. It never produces a value. Query safety does not
// depend on it: the degraded marker drives the query guard.
func regexDegradedPlaceholder(any) (any, bool) { return nil, false }

// classifyRegexCompanion is the one read-side classification of a stored
// RegexExtract or RegexExtractInt spec, for config-backed and
// representation-only specs alike. It never freezes and never uses
// source_pattern: it runs the stored executable pattern.
//
// An error means invalid params (bad JSON, wrong types, negative group,
// incomplete frozen metadata, an ID that is not a regex ID); Decode fails.
// Otherwise it returns the function and, for an unsafe companion, a
// degradation:
//   - marked spec whose pattern fails verifyFrozenRegexText: unverified_frozen_regex
//   - pattern does not compile: regex_compile_failed
//   - compile timed out: regex_compile_timeout
//   - unmarked spec the detector flags: legacy_unfrozen_regex
//
// A degraded companion keeps its compiled function when the pattern
// compiled, so a decoded config reused by NewBuilder extracts as before #115.
// Otherwise the function is a placeholder that never produces a value, and
// the degradation is marked so NewBuilder rejects the config.
func classifyRegexCompanion(spec TransformerSpec) (FieldTransformer, *companionDegradation, error) {
	if spec.ID != TransformerRegexExtract && spec.ID != TransformerRegexExtractInt {
		return nil, nil, errors.Errorf("transformer name %q does not match transformer id %d", spec.Name, spec.ID)
	}
	params, marked, err := decodeRegexWireParams(spec.Params)
	if err != nil {
		return nil, nil, err
	}
	var degraded *companionDegradation
	if marked {
		if err := verifyFrozenRegexText(params.Pattern); err != nil {
			degraded = &companionDegradation{reason: DegradeReasonUnverifiedFrozenRegex, detail: err.Error()}
		}
	}
	re, err := compileRegexWithTimeout(params.Pattern, regexCompileDeadline)
	if err != nil {
		if degraded == nil {
			degraded = &companionDegradation{reason: DegradeReasonRegexCompileFailed, detail: err.Error()}
			var compileErr *regexCompileError
			if errors.As(err, &compileErr) && compileErr.timeout {
				degraded = &companionDegradation{reason: DegradeReasonRegexCompileTimeout, detail: err.Error()}
			}
		}
		degraded.placeholder = true
		return regexDegradedPlaceholder, degraded, nil
	}
	if degraded == nil && !marked && regexTableSensitive(params.Pattern) {
		degraded = &companionDegradation{
			reason: DegradeReasonLegacyUnfrozenRegex,
			detail: "stored pattern has no frozen metadata and may depend on the Unicode tables of the reader; rebuild the index to restore pruning",
		}
	}
	return regexExtractFunc(re, params.Group, spec.ID == TransformerRegexExtractInt), degraded, nil
}

// checkExecutableTransformers fails when a registration has no executable
// transformer: a decoded regex companion whose stored pattern did not compile
// (#115). The error names the source path, alias and reason. NewBuilder runs
// it; Decode does not, because a reader can still query such an index (the
// companion returns every row group).
func (c *GINConfig) checkExecutableTransformers() error {
	paths := make([]string, 0, len(c.representationTransformers))
	for path := range c.representationTransformers {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		for _, registered := range c.representationTransformers[path] {
			if registered.degraded == nil || !registered.degraded.placeholder {
				continue
			}
			return errors.Errorf("companion %s alias %q has no executable transformer (%s: %s); register the transformer again to build",
				registered.SourcePath, registered.Alias, registered.degraded.reason, registered.degraded.detail)
		}
	}
	return nil
}
