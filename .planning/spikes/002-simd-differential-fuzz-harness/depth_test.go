//go:build simdjson

// Q5 — unplanned probe. Q4b hung for 600s on a 2000-deep nested-array input
// that Q2b (SIMD arm only) had swept in 0.26s. The crash stack was a tower of
// builder.go:621 stageMaterializedValue frames with the canonical path growing
// ~3 bytes per level, which is the signature of array dual emission ([i] AND
// [*]) branching once per nesting level.
//
// This measures scaling directly instead of guessing, and checks whether the
// stdlib and SIMD arms differ (parser_simd.go has reachesSIMDNestingDepthLimit;
// parser_stdlib.go appears to have no equivalent guard).
package simdfuzzspike

import (
	"strings"
	"testing"
	"time"

	gin "github.com/amikos-tech/ami-gin"
)

func nestedArray(depth int) []byte {
	return []byte(strings.Repeat(`[`, depth) + `1` + strings.Repeat(`]`, depth))
}

func nestedObject(depth int) []byte {
	return []byte(strings.Repeat(`{"a":`, depth) + `1` + strings.Repeat(`}`, depth))
}

func timeArm(t *testing.T, cfg gin.GINConfig, parser gin.Parser, doc []byte, budget time.Duration) (time.Duration, bool, int) {
	t.Helper()
	type result struct {
		d time.Duration
		n int
	}
	ch := make(chan result, 1)
	go func() {
		start := time.Now()
		enc, _, _ := buildEncode(cfg, parser, doc)
		ch <- result{time.Since(start), len(enc)}
	}()
	select {
	case r := <-ch:
		return r.d, true, r.n
	case <-time.After(budget):
		return budget, false, -1
	}
}

func TestQ5_NestingDepthScaling(t *testing.T) {
	cfg := softConfig(t)

	parser, err := gin.NewSIMDParser()
	if err != nil {
		t.Fatalf("NewSIMDParser: %v", err)
	}
	t.Cleanup(func() {
		if err := parser.Close(); err != nil {
			t.Errorf("Close parser: %v", err)
		}
	})

	const budget = 3 * time.Second

	t.Log("=== nested ARRAYS: [[[...1...]]] ===")
	t.Log("depth | stdlib          | simd")
	stdlibBlown := 0
	for depth := 2; depth <= 26; depth += 2 {
		doc := nestedArray(depth)

		stdD, stdDone, stdN := timeArm(t, cfg, nil, doc, budget)
		simdD, simdDone, simdN := timeArm(t, cfg, parser, doc, budget)

		stdStr := formatArm(stdD, stdDone, stdN)
		simdStr := formatArm(simdD, simdDone, simdN)
		t.Logf("%5d | %-15s | %s", depth, stdStr, simdStr)

		if !stdDone {
			stdlibBlown = depth
			t.Logf("Q5 stdlib arm exceeded %v at array depth %d — stopping array sweep", budget, depth)
			break
		}
	}

	t.Log("=== nested OBJECTS: {\"a\":{\"a\":...}} ===")
	t.Log("depth | stdlib          | simd")
	for _, depth := range []int{2, 8, 32, 128, 512} {
		doc := nestedObject(depth)

		stdD, stdDone, stdN := timeArm(t, cfg, nil, doc, budget)
		simdD, simdDone, simdN := timeArm(t, cfg, parser, doc, budget)

		t.Logf("%5d | %-15s | %s", depth, formatArm(stdD, stdDone, stdN), formatArm(simdD, simdDone, simdN))
		if !stdDone {
			break
		}
	}

	if stdlibBlown > 0 {
		t.Logf("Q5 VERDICT: nested-ARRAY cost is non-linear on the stdlib arm; it blew a %v budget at depth %d (input is %d bytes)",
			budget, stdlibBlown, len(nestedArray(stdlibBlown)))
	} else {
		t.Logf("Q5 VERDICT: no blow-up observed within the swept range")
	}
}

func formatArm(d time.Duration, done bool, n int) string {
	if !done {
		return "TIMEOUT"
	}
	if n == 0 {
		return "rejected"
	}
	return d.String()
}
