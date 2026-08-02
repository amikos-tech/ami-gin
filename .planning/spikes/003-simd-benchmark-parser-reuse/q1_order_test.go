//go:build simdjson

// Q1 — Order dependence.
//
// Given one shared SIMD parser (Plan 22-05's mandate), when the same fixture
// runs FIRST through a fresh parser versus LAST after the other three fixtures
// have already passed through it, then per-fixture ns/op, B/op and allocs/op
// should be unchanged.
//
// If they differ, Plan 22-06's committed per-fixture evidence misattributes
// cost, and D-12 forbids any threshold that would catch it.
package simdbenchspike

import (
	"testing"

	gin "github.com/amikos-tech/ami-gin"
)

func TestQ1_OrderDependence(t *testing.T) {
	fixtures := loadFixtures(t)

	t.Logf("fixture sizes:")
	for _, f := range fixtures {
		t.Logf("  %-24s %3d docs, %6d bytes", f.name, len(f.docs), f.bytes)
	}

	type row struct {
		name              string
		firstNs, lastNs   float64
		firstBpo, lastBpo float64
		firstApo, lastApo float64
		rssAfterWarm      int64
	}
	rows := make([]row, 0, len(fixtures))

	for _, target := range fixtures {
		// --- Arm A: target runs FIRST through a brand-new parser.
		pFirst, err := gin.NewSIMDParser()
		if err != nil {
			t.Fatalf("NewSIMDParser (first-arm, %s): %v", target.name, err)
		}
		resFirst := benchFixture(target, pFirst)
		if err := pFirst.Close(); err != nil {
			t.Fatalf("Close (first-arm): %v", err)
		}

		// --- Arm B: target runs LAST, after the other three fixtures have
		// gone through the SAME parser. This is the state Plan 22-05 actually
		// produces for every fixture except the first one.
		pLast, err := gin.NewSIMDParser()
		if err != nil {
			t.Fatalf("NewSIMDParser (last-arm, %s): %v", target.name, err)
		}
		rssBefore := rssKB(t)
		for _, warm := range fixtures {
			if warm.name == target.name {
				continue
			}
			if _, err := buildIndex(warm.docs, pLast); err != nil {
				t.Fatalf("warm-up buildIndex(%s): %v", warm.name, err)
			}
		}
		rssAfter := rssKB(t)
		resLast := benchFixture(target, pLast)
		if err := pLast.Close(); err != nil {
			t.Fatalf("Close (last-arm): %v", err)
		}

		rows = append(rows, row{
			name:     target.name,
			firstNs:  nsPerOp(resFirst), lastNs: nsPerOp(resLast),
			firstBpo: bytesPerOp(resFirst), lastBpo: bytesPerOp(resLast),
			firstApo: allocsPerOp(resFirst), lastApo: allocsPerOp(resLast),
			rssAfterWarm: rssAfter - rssBefore,
		})
	}

	t.Logf("")
	t.Logf("Q1 — same fixture measured FIRST (fresh parser) vs LAST (after 3 others):")
	t.Logf("%-24s | %14s %14s %8s | %12s %12s %8s | %10s",
		"fixture", "ns/op first", "ns/op last", "delta", "B/op first", "B/op last", "delta", "warmRSS")
	for _, r := range rows {
		t.Logf("%-24s | %14.0f %14.0f %8s | %12.0f %12.0f %8s | %10s",
			r.name,
			r.firstNs, r.lastNs, pctDelta(r.firstNs, r.lastNs),
			r.firstBpo, r.lastBpo, pctDelta(r.firstBpo, r.lastBpo),
			mib(r.rssAfterWarm))
	}

	t.Logf("")
	t.Logf("allocs/op (Go heap only — native buffer growth is invisible here):")
	for _, r := range rows {
		t.Logf("  %-24s first=%.0f last=%.0f (%s)",
			r.name, r.firstApo, r.lastApo, pctDelta(r.firstApo, r.lastApo))
	}
}
