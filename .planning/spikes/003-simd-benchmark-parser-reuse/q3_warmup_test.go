//go:build simdjson

// Q3 — Cold vs steady-state.
//
// Given a fresh SIMD parser, when per-iteration ingest cost is swept from
// iteration 1 upward, then the plateau point shows how many iterations of
// warm-up exist and whether testing's b.N averaging amortizes it to nothing.
//
// This decides what Plan 22-06 should call authoritative: if warm-up is
// confined to the first iteration or two, then b.N (measured at 61-312 in the
// depth test) makes every committed number a steady-state number, and the
// evidence document should say so explicitly.
package simdbenchspike

import (
	"sort"
	"testing"
	"time"

	gin "github.com/amikos-tech/ami-gin"
)

const (
	warmupShow  = 12
	warmupTotal = 150
)

func TestQ3_ColdVsSteadyState(t *testing.T) {
	fixtures := loadFixtures(t)

	for _, f := range fixtures {
		p, err := gin.NewSIMDParser()
		if err != nil {
			t.Fatalf("NewSIMDParser (%s): %v", f.name, err)
		}
		rssStart := rssKB(t)

		samples := make([]float64, 0, warmupTotal)
		for i := 0; i < warmupTotal; i++ {
			start := time.Now()
			if _, err := buildIndex(f.docs, p); err != nil {
				t.Fatalf("buildIndex(%s) iter %d: %v", f.name, i, err)
			}
			samples = append(samples, float64(time.Since(start).Nanoseconds()))
		}
		rssEnd := rssKB(t)
		if err := p.Close(); err != nil {
			t.Fatalf("Close (%s): %v", f.name, err)
		}

		// Steady-state reference: median of the last half.
		tail := append([]float64(nil), samples[len(samples)/2:]...)
		sort.Float64s(tail)
		steady := tail[len(tail)/2]

		t.Logf("── %s (%d docs, %d bytes)  steady-state median = %.0f ns",
			f.name, len(f.docs), f.bytes, steady)

		for i := 0; i < warmupShow && i < len(samples); i++ {
			t.Logf("   iter %-3d %10.0f ns  %8s vs steady", i+1, samples[i], pctDelta(steady, samples[i]))
		}

		// How many leading iterations sit more than 20% above steady state?
		warmIters := 0
		for _, s := range samples {
			if s > steady*1.20 {
				warmIters++
				continue
			}
			break
		}
		t.Logf("   → leading iterations >20%% above steady state: %d", warmIters)
		t.Logf("   → RSS growth over %d iterations: %s", warmupTotal, mib(rssEnd-rssStart))
		t.Logf("")
	}

	t.Logf("Interpretation: testing.Benchmark chose N=61..312 for these fixtures")
	t.Logf("(see TestDepth_FreshParserAnomaly). Any warm-up confined to the first")
	t.Logf("few iterations is therefore amortized to near-zero in reported ns/op.")
}
