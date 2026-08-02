//go:build simdjson

package gin

import "testing"

type simdTypedSinkBenchmarkArm struct {
	name   string
	parser Parser
}

func BenchmarkSIMDTypedSinkIngest(b *testing.B) {
	runSIMDTypedSinkIngestBenchmarks(b)
}

func runSIMDTypedSinkIngestBenchmarks(b *testing.B) {
	b.Helper()

	// This benchmark is steady-state: parser construction and fixture discovery
	// are outside leaf timers, and testing.B amortizes warm-up. Spike 003 found no
	// order or shared-vs-fresh bias, so one caller-owned SIMD parser is reused
	// sequentially. Reported B/op covers Go-heap allocations only; native parser
	// buffers are excluded.
	simdParser := newTestSIMDParser(b)
	arms := []simdTypedSinkBenchmarkArm{
		{name: "stdlib", parser: stdlibParser{}},
		{name: "simd", parser: simdParser},
	}

	for _, fixture := range phase20SmokeFixtures {
		docs, err := phase20LoadRawJSONL(fixture.path)
		if err != nil {
			b.Fatalf("phase20LoadRawJSONL(%q) error = %v", fixture.path, err)
		}
		inputBytes := simdTypedSinkInputBytes(docs)
		for _, arm := range arms {
			arm := arm
			b.Run("tier=smoke/fixture="+fixture.name+"/parser="+arm.name, func(b *testing.B) {
				benchmarkSIMDTypedSinkIngest(b, docs, inputBytes, arm.parser)
			})
		}
	}

	b.Run("tier=external/fixture=local-example", func(b *testing.B) {
		fixture := phase20ExternalBenchmarkFixture(b)
		inputBytes := simdTypedSinkInputBytes(fixture.docs)
		for _, arm := range arms {
			arm := arm
			b.Run("parser="+arm.name, func(b *testing.B) {
				benchmarkSIMDTypedSinkIngest(b, fixture.docs, inputBytes, arm.parser)
			})
		}
	})
}

func benchmarkSIMDTypedSinkIngest(b *testing.B, docs [][]byte, inputBytes int64, parser Parser) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(inputBytes)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := phase20BuildBenchmarkIndex(docs, WithParser(parser)); err != nil {
			b.Fatalf("phase20BuildBenchmarkIndex() error = %v", err)
		}
	}
}

func simdTypedSinkInputBytes(docs [][]byte) int64 {
	var total int64
	for _, doc := range docs {
		total += int64(len(doc))
	}
	return total
}
