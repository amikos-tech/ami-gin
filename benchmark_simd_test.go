//go:build simdjson

package gin

import "testing"

func BenchmarkSIMDTypedSinkIngest(b *testing.B) {
	runSIMDTypedSinkIngestBenchmarks(b)
}
