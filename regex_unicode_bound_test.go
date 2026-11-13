package gin

import (
	"encoding/json"
	"regexp/syntax"
	"runtime"
	"strings"
	"testing"
)

// TestRegexUnicodeFreezerLowerBoundIsLower: the bound never exceeds the real
// JSON length, so a pattern whose frozen text fits is never rejected early.
func TestRegexUnicodeFreezerLowerBoundIsLower(t *testing.T) {
	patterns := []string{
		`(\p{L}+)`, `(\pL+)`, `\P{Lu}`, `[^a]`, `[^\x00-\x{10FFFF}]`, `[ab]`, `[a-c]`, `[\-a]`,
		`(?i)(ƛ+)`, `(?i)k`, `[\x00-\x1f]`, `[^\p{Greek}]`, `\p{Han}+`, `(\d+)\p{Lu}`,
		`[\x{D7FF}-\x{E000}]`, `x{2,5}|y*`, `(?s).`, strings.Repeat(`\pL`, 3),
	}
	for _, pattern := range patterns {
		re, err := syntax.Parse(pattern, syntax.Perl)
		if err != nil {
			t.Fatalf("parse %q: %v", pattern, err)
		}
		re = freezeRegexNode(re)
		encoded, err := json.Marshal(re.String())
		if err != nil {
			t.Fatal(err)
		}
		if lower := frozenRegexLowerBound(re, maxConfigSize) + 2; lower > len(encoded) {
			t.Fatalf("%q: lower bound %d > real JSON length %d", pattern, lower, len(encoded))
		}
	}
}

// TestRegexUnicodeFreezerStopsEarly: 300 x \pL renders to more than 1 MiB.
// Registration fails on the lower-bound check, before String renders the
// full expansion, so the bytes allocated stay well below the rendered size.
func TestRegexUnicodeFreezerStopsEarly(t *testing.T) {
	pattern := strings.Repeat(`\pL`, 300)
	_, err := NewConfig(WithRegexExtractTransformer("$.name", "big", "("+pattern+")", 1))
	if err == nil || !strings.Contains(err.Error(), "needs at least") {
		t.Fatalf("NewConfig with 300 x \\pL: err = %v, want the early lower-bound error", err)
	}

	// Rendering text longer than maxConfigSize allocates at least
	// maxConfigSize bytes more than the parse itself.
	parseAlloc := regexUnicodeAllocatedBytes(func() {
		re, err := syntax.Parse(pattern, syntax.Perl)
		if err != nil {
			t.Fatal(err)
		}
		_ = freezeRegexNode(re)
	})
	freezeAlloc := regexUnicodeAllocatedBytes(func() {
		_, err = freezeRegexPattern(pattern, maxConfigSize)
	})
	if err == nil || !strings.Contains(err.Error(), "needs at least") {
		t.Fatalf("freeze 300 x \\pL: err = %v, want the early lower-bound error", err)
	}
	if freezeAlloc >= parseAlloc+maxConfigSize {
		t.Fatalf("freeze 300 x \\pL allocated %d bytes, parse alone %d bytes: the full expansion was rendered", freezeAlloc, parseAlloc)
	}
}

func regexUnicodeAllocatedBytes(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}
