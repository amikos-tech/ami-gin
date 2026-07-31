//go:build simdjson

// FOLLOW-UP — reframing the question Q2 actually answered.
//
// Q2 was supposed to be about parser-reuse bias. It found none, but surfaced
// something far more consequential: on the Phase 20 smoke tier, SIMD is
// 1.45x-1.84x SLOWER than stdlib and allocates 18-36% more Go heap.
//
// Phase 20 fixtures are JSONL with small (~250-500 byte) records. simdjson's
// advantage is throughput on large buffers; per-document FFI overhead through
// purego is a fixed cost paid once per Parse. That predicts a document-size
// crossover.
//
// CONVENTIONS.md: "A/B the parameters a spike recommends." Reporting
// "SIMD is slower" is weak; reporting "SIMD loses below ~X KB/doc and wins
// above it" is actionable for Plan 22-06's ship/defer/narrow decision.
package simdbenchspike

import (
	"fmt"
	"strings"
	"testing"

	gin "github.com/amikos-tech/ami-gin"
)

// synthDoc builds a flat JSON object of roughly targetBytes, mixing strings and
// numbers so both parsers exercise scalar staging rather than one shape only.
func synthDoc(targetBytes int) []byte {
	var b strings.Builder
	b.WriteString(`{"id":"doc-0"`)
	for i := 0; b.Len() < targetBytes; i++ {
		if i%2 == 0 {
			fmt.Fprintf(&b, `,"s%d":"value-%d-padding-abcdefghijklmnop"`, i, i)
		} else {
			fmt.Fprintf(&b, `,"n%d":%d.%d`, i, i*7919, i%997)
		}
	}
	b.WriteString("}")
	return []byte(b.String())
}

// docSizes sweeps three orders of magnitude around the Phase 20 record size.
var docSizes = []int{256, 1024, 4096, 16384, 65536, 262144}

// totalBytesPerCase keeps total ingested volume roughly constant across sizes,
// so the comparison isolates per-document overhead rather than total work.
const totalBytesPerCase = 2 << 20 // 2 MiB

func TestFollowup_DocumentSizeCrossover(t *testing.T) {
	p, err := gin.NewSIMDParser()
	if err != nil {
		t.Fatalf("NewSIMDParser: %v", err)
	}
	defer func() {
		if err := p.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}()

	t.Logf("Constant ~%d KiB total volume per case; only document size varies.", totalBytesPerCase/1024)
	t.Logf("%-12s %8s | %14s %14s | %10s | %12s %12s | %8s",
		"doc size", "docs", "stdlib ns/op", "simd ns/op", "simd/std", "stdlib B/op", "simd B/op", "B ratio")

	for _, size := range docSizes {
		doc := synthDoc(size)
		actual := len(doc)
		count := totalBytesPerCase / actual
		if count < 4 {
			count = 4
		}
		docs := make([][]byte, count)
		for i := range docs {
			docs[i] = doc
		}
		f := fixture{name: fmt.Sprintf("synth-%d", size), docs: docs, bytes: int64(actual * count)}

		rStd := benchFixture(f, nil)
		rSimd := benchFixture(f, p)

		stdNs, simdNs := nsPerOp(rStd), nsPerOp(rSimd)
		stdB, simdB := bytesPerOp(rStd), bytesPerOp(rSimd)

		marker := ""
		if simdNs < stdNs {
			marker = "  ← SIMD WINS"
		}
		t.Logf("%-12d %8d | %14.0f %14.0f | %9.2fx | %12.0f %12.0f | %7.2fx%s",
			actual, count, stdNs, simdNs, simdNs/stdNs, stdB, simdB, simdB/stdB, marker)
	}

	t.Logf("")
	t.Logf("Phase 20 smoke records are ~250-500 bytes/doc — the far-left end of this sweep.")
}
