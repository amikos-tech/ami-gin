//go:build simdjson

// Q2 — Shared vs per-fixture parser, against the stdlib reference.
//
// Given the same fixture set, when run with (a) one shared SIMD parser
// [Plan 22-05's mandate], (b) a fresh SIMD parser per fixture, and (c) the
// stdlib default path, then the SIMD-vs-stdlib delta should be stable across
// both SIMD parser lifecycles.
//
// This test ALSO carries Q1's vacuity guard: if the stdlib and SIMD arms are
// numerically indistinguishable, WithParser never engaged and Q1's negative
// result proves nothing. CONVENTIONS.md: "Distrust a test that passes on the
// first try. Verify the precondition actually fired."
package simdbenchspike

import (
	"math"
	"testing"

	gin "github.com/amikos-tech/ami-gin"
)

func TestQ2_ParserLifecycle(t *testing.T) {
	fixtures := loadFixtures(t)

	stdlib := make(map[string]testing.BenchmarkResult, len(fixtures))
	shared := make(map[string]testing.BenchmarkResult, len(fixtures))
	fresh := make(map[string]testing.BenchmarkResult, len(fixtures))

	// --- Arm C: stdlib reference (nil parser), no persistent state at all.
	for _, f := range fixtures {
		stdlib[f.name] = benchFixture(f, nil)
	}

	// --- Arm A: ONE shared SIMD parser across all fixtures (Plan 22-05:95).
	pShared, err := gin.NewSIMDParser()
	if err != nil {
		t.Fatalf("NewSIMDParser (shared): %v", err)
	}
	rssBeforeShared := rssKB(t)
	for _, f := range fixtures {
		shared[f.name] = benchFixture(f, pShared)
	}
	rssAfterShared := rssKB(t)
	if err := pShared.Close(); err != nil {
		t.Fatalf("Close (shared): %v", err)
	}

	// --- Arm B: a FRESH SIMD parser per fixture.
	for _, f := range fixtures {
		p, err := gin.NewSIMDParser()
		if err != nil {
			t.Fatalf("NewSIMDParser (fresh, %s): %v", f.name, err)
		}
		fresh[f.name] = benchFixture(f, p)
		if err := p.Close(); err != nil {
			t.Fatalf("Close (fresh, %s): %v", f.name, err)
		}
	}

	t.Logf("Q2 — ns/op by parser lifecycle:")
	t.Logf("%-24s | %13s %13s %13s | %10s %10s",
		"fixture", "stdlib", "simd-shared", "simd-fresh", "simd/std", "shr-vs-frsh")
	for _, f := range fixtures {
		s := nsPerOp(stdlib[f.name])
		sh := nsPerOp(shared[f.name])
		fr := nsPerOp(fresh[f.name])
		t.Logf("%-24s | %13.0f %13.0f %13.0f | %9.2fx %10s",
			f.name, s, sh, fr, sh/s, pctDelta(fr, sh))
	}

	t.Logf("")
	t.Logf("Q2 — B/op by parser lifecycle (GO HEAP ONLY):")
	t.Logf("%-24s | %13s %13s %13s | %10s",
		"fixture", "stdlib", "simd-shared", "simd-fresh", "simd/std")
	for _, f := range fixtures {
		s := bytesPerOp(stdlib[f.name])
		sh := bytesPerOp(shared[f.name])
		fr := bytesPerOp(fresh[f.name])
		t.Logf("%-24s | %13.0f %13.0f %13.0f | %9.2fx",
			f.name, s, sh, fr, sh/s)
	}

	t.Logf("")
	t.Logf("Q2 — allocs/op by parser lifecycle (GO HEAP ONLY):")
	for _, f := range fixtures {
		t.Logf("  %-24s stdlib=%.0f shared=%.0f fresh=%.0f",
			f.name, allocsPerOp(stdlib[f.name]), allocsPerOp(shared[f.name]), allocsPerOp(fresh[f.name]))
	}

	t.Logf("")
	t.Logf("shared-arm RSS growth across all 4 fixtures: %s", mib(rssAfterShared-rssBeforeShared))

	// ---------------------------------------------------------------------
	// VACUITY GUARD. If SIMD and stdlib are indistinguishable on every
	// fixture, WithParser did not engage and neither Q1 nor Q2 means anything.
	// ---------------------------------------------------------------------
	indistinguishable := 0
	for _, f := range fixtures {
		s := nsPerOp(stdlib[f.name])
		sh := nsPerOp(shared[f.name])
		if s == 0 {
			continue
		}
		if math.Abs(sh-s)/s < 0.02 {
			indistinguishable++
		}
	}
	if indistinguishable == len(fixtures) {
		t.Fatalf("VACUOUS: SIMD arm is within 2%% of stdlib on all %d fixtures — "+
			"WithParser almost certainly did not engage; Q1's negative result is meaningless",
			len(fixtures))
	}
	t.Logf("vacuity guard: %d/%d fixtures within 2%% of stdlib (need < %d to be meaningful)",
		indistinguishable, len(fixtures), len(fixtures))
}
