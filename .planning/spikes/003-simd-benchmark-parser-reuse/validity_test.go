//go:build simdjson

// VALIDITY GUARD for the performance findings.
//
// The Q2/follow-up conclusion ("SIMD is 1.2x-1.5x slower and allocates more")
// is only meaningful if both arms are doing the SAME work. If the SIMD arm
// silently produced a smaller or different index — dropped documents, skipped
// paths — it would be doing LESS work and the comparison would be worthless.
//
// Phase 22's own parity gate (SIMD-08, D-04) requires byte-identical encoded
// indexes. Asserting it here proves the benchmark comparison is apples to
// apples, using the same oracle Plan 22-02 will use.
package simdbenchspike

import (
	"bytes"
	"testing"

	gin "github.com/amikos-tech/ami-gin"
)

func TestValidity_ByteIdenticalAcrossArms(t *testing.T) {
	fixtures := loadFixtures(t)

	p, err := gin.NewSIMDParser()
	if err != nil {
		t.Fatalf("NewSIMDParser: %v", err)
	}
	defer func() {
		if err := p.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}()

	for _, f := range fixtures {
		stdIdx, err := buildIndex(f.docs, nil)
		if err != nil {
			t.Fatalf("stdlib buildIndex(%s): %v", f.name, err)
		}
		simdIdx, err := buildIndex(f.docs, p)
		if err != nil {
			t.Fatalf("simd buildIndex(%s): %v", f.name, err)
		}

		stdEnc, err := gin.Encode(stdIdx)
		if err != nil {
			t.Fatalf("Encode stdlib(%s): %v", f.name, err)
		}
		simdEnc, err := gin.Encode(simdIdx)
		if err != nil {
			t.Fatalf("Encode simd(%s): %v", f.name, err)
		}

		if !bytes.Equal(stdEnc, simdEnc) {
			t.Fatalf("%s: encoded indexes DIFFER (stdlib=%d bytes, simd=%d bytes) — "+
				"the performance comparison is not apples-to-apples",
				f.name, len(stdEnc), len(simdEnc))
		}
		t.Logf("%-24s byte-identical across arms (%d encoded bytes, %d docs)",
			f.name, len(stdEnc), len(f.docs))
	}
	t.Logf("")
	t.Logf("Both arms produce identical output ⇒ the ns/op and B/op deltas reflect")
	t.Logf("cost of the same work, not a difference in work performed.")
}
