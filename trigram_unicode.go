package gin

import (
	"sort"
	"unicode"
	"unicode/utf8"
)

// trigramUnicodeVersion is the Unicode version of the frozen tables that
// TrigramIndex uses to lowercase text and to drop whitespace-only and
// punctuation-or-symbol-only grams (#99). The grams must not depend on the
// Go toolchain's unicode package: an index built with one toolchain and
// searched with another would otherwise derive grams that Add never stored
// and prune matching row groups.
//
// Upgrade policy: a freshly built index writes no trigram_unicode_version
// config key, and an absent key means this version. A future table upgrade
// must ADD a second table set next to this one and keep this one, so that
// existing indexes keep their meaning. Readers of ami-gin <= v1.5.x ignore
// unknown config fields and cannot apply the unknown-version fallback, so
// every reader must be upgraded before any writer emits a new token.
const trigramUnicodeVersion = "15.0.0"

// trigramUnicodeTokenSupported reports whether this reader has the tables for
// a serialized trigram_unicode_version token. "" means absent, which is
// trigramUnicodeVersion.
func trigramUnicodeTokenSupported(token string) bool {
	return token == "" || token == trigramUnicodeVersion
}

// trigramUnicodeWireToken returns the token to serialize: absent for the
// frozen version, any other token verbatim.
func trigramUnicodeWireToken(token string) string {
	if token == trigramUnicodeVersion {
		return ""
	}
	return token
}

// The directive needs a POSIX env; see gen_trigram_unicode.go for Windows
// and for what to do when a Go release changes unicode.Version.
//
//go:generate env GOTOOLCHAIN=go1.26.9 go run gen_trigram_unicode.go

// trigramLowerRune lowercases r with the frozen trigramLowerRanges table. It
// is the simple per-rune mapping of unicode.ToLower at trigramUnicodeVersion.
// Locale special cases (unicode.SpecialCase) are not applied: strings.ToLower
// never applied them, and SpecialCase falls back to the toolchain tables.
func trigramLowerRune(r rune) rune {
	if r < utf8.RuneSelf {
		if 'A' <= r && r <= 'Z' {
			return r + 'a' - 'A'
		}
		return r
	}
	u := uint32(r)
	i := sort.Search(len(trigramLowerRanges), func(i int) bool { return trigramLowerRanges[i].Hi >= u })
	if i < len(trigramLowerRanges) {
		e := trigramLowerRanges[i]
		if u >= e.Lo && (u-e.Lo)%e.Stride == 0 {
			return r + e.Delta
		}
	}
	return r
}

// trigramLowerRunes returns the frozen lowercase runes of s. Each invalid
// UTF-8 byte becomes U+FFFD, exactly like []rune(strings.ToLower(s)).
func trigramLowerRunes(s string) []rune {
	out := []rune(s)
	for i, r := range out {
		out[i] = trigramLowerRune(r)
	}
	return out
}

// trigramIsSpace is unicode.IsSpace at trigramUnicodeVersion.
func trigramIsSpace(r rune) bool {
	if uint32(r) < utf8.RuneSelf {
		return r == ' ' || '\t' <= r && r <= '\r'
	}
	return unicode.Is(trigramSpaceTable, r)
}

// trigramASCIIPunctOrSymbol is trigramPunctOrSymbolTable below utf8.RuneSelf
// as a bitmask. It is built from the frozen table, so it cannot drift from
// it, and spares ASCII text the range scan in unicode.Is.
var trigramASCIIPunctOrSymbol = func() (mask [2]uint64) {
	for r := rune(0); r < utf8.RuneSelf; r++ {
		if unicode.Is(trigramPunctOrSymbolTable, r) {
			mask[r>>6] |= 1 << (r & 63)
		}
	}
	return mask
}()

// trigramIsPunctOrSymbol is unicode.IsPunct || unicode.IsSymbol at
// trigramUnicodeVersion.
func trigramIsPunctOrSymbol(r rune) bool {
	if uint32(r) < utf8.RuneSelf {
		return trigramASCIIPunctOrSymbol[r>>6]&(1<<(r&63)) != 0
	}
	return unicode.Is(trigramPunctOrSymbolTable, r)
}
