//go:build simdjson

// DEPTH — unplanned discovery from Q2.
//
// Q2 showed simd-fresh ~3x SLOWER than simd-shared for mixed-type-arrays and
// number-heavy, but statistically identical (+0.3%) for nested-high-cardinality
// and combined. A 3x split across fixtures with no obvious pattern is either a
// real per-construction penalty or a measurement artifact.
//
// CONVENTIONS.md: "Measure, don't infer. Sweep the parameter and print a table."
//
// This sweeps repeats and reports testing.Benchmark's chosen N, because a small
// N makes ns/op extremely noisy and would explain the split without any real
// parser effect.
package simdbenchspike

import (
	"testing"

	gin "github.com/amikos-tech/ami-gin"
)

const depthRepeats = 5

func TestDepth_FreshParserAnomaly(t *testing.T) {
	fixtures := loadFixtures(t)

	t.Logf("Repeating the shared-vs-fresh comparison %d times, reporting chosen N.", depthRepeats)
	t.Logf("If N is tiny and ns/op swings run-to-run, Q2's 3x split was noise.")
	t.Logf("")

	for _, f := range fixtures {
		t.Logf("── %s (%d docs, %d bytes)", f.name, len(f.docs), f.bytes)

		// Shared parser, reused across all repeats of this fixture.
		pShared, err := gin.NewSIMDParser()
		if err != nil {
			t.Fatalf("NewSIMDParser (shared): %v", err)
		}

		for rep := 0; rep < depthRepeats; rep++ {
			rShared := benchFixture(f, pShared)

			pFresh, err := gin.NewSIMDParser()
			if err != nil {
				t.Fatalf("NewSIMDParser (fresh): %v", err)
			}
			rFresh := benchFixture(f, pFresh)
			if err := pFresh.Close(); err != nil {
				t.Fatalf("Close (fresh): %v", err)
			}

			t.Logf("   rep %d | shared N=%-6d %10.0f ns/op | fresh N=%-6d %10.0f ns/op | fresh-vs-shared %8s",
				rep,
				rShared.N, nsPerOp(rShared),
				rFresh.N, nsPerOp(rFresh),
				pctDelta(nsPerOp(rShared), nsPerOp(rFresh)))
		}

		if err := pShared.Close(); err != nil {
			t.Fatalf("Close (shared): %v", err)
		}
		t.Logf("")
	}
}

// TestDepth_ConstructionCost isolates whether NewSIMDParser itself is the
// expensive part, independent of any parsing. Spike 001 measured ~1us warm;
// if that still holds, construction cannot explain a 3x ingest delta.
func TestDepth_ConstructionCost(t *testing.T) {
	const n = 200

	first, err := gin.NewSIMDParser()
	if err != nil {
		t.Fatalf("NewSIMDParser (cold): %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close (cold): %v", err)
	}

	res := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p, err := gin.NewSIMDParser()
			if err != nil {
				b.Fatalf("NewSIMDParser: %v", err)
			}
			if err := p.Close(); err != nil {
				b.Fatalf("Close: %v", err)
			}
		}
	})

	t.Logf("NewSIMDParser+Close: N=%d  %.0f ns/op  %.0f B/op  %.0f allocs/op",
		res.N, nsPerOp(res), bytesPerOp(res), allocsPerOp(res))
	t.Logf("(compare against the multi-millisecond ingest deltas seen in Q2)")
	_ = n
}
