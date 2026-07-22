package gin

import (
	"bytes"
	"sync"
	"testing"
)

// TestEncodeDecodeConcurrent exercises the shared, never-Closed zstd
// encoder/decoder singletons (serialize.go:230-262) from many goroutines.
// It locks in the invariant the encoder-reuse optimization relies on:
// EncodeAll/DecodeAll on the shared codecs are safe for concurrent use.
//
// Meaningful only under the race detector: go test -race -run Concurrent .
// Correctness (not just race-freedom) is asserted three ways per iteration:
//   - concurrent Encode output is byte-identical to a single-threaded golden
//     (proves the shared encoder is not corrupted by concurrent EncodeAll),
//   - Decode succeeds,
//   - the decoded index re-encodes to the same golden (proves the shared
//     decoder returned complete, faithful data, not a truncated frame).
func TestEncodeDecodeConcurrent(t *testing.T) {
	idx := buildAdaptiveSerializationFixture(t, DefaultConfig())

	// One level per collapsed zstd mode, plus 19 which shares SpeedBestCompression
	// with 15 — so the shared map is hit concurrently at both distinct and
	// mode-colliding levels.
	levels := []CompressionLevel{
		CompressionFastest,  // 1  -> SpeedFastest
		CompressionBalanced, // 3  -> SpeedDefault
		CompressionBetter,   // 9  -> SpeedBetterCompression
		CompressionBest,     // 15 -> SpeedBestCompression
		CompressionMax,      // 19 -> SpeedBestCompression
	}

	golden := make(map[CompressionLevel][]byte, len(levels))
	for _, lvl := range levels {
		encoded, err := EncodeWithLevel(idx, lvl)
		if err != nil {
			t.Fatalf("golden EncodeWithLevel(%d): %v", lvl, err)
		}
		golden[lvl] = encoded
	}

	const (
		goroutines = 32
		iterations = 50
	)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			lvl := levels[g%len(levels)]
			want := golden[lvl]
			for i := 0; i < iterations; i++ {
				encoded, err := EncodeWithLevel(idx, lvl)
				if err != nil {
					t.Errorf("EncodeWithLevel(%d): %v", lvl, err)
					return
				}
				if !bytes.Equal(encoded, want) {
					t.Errorf("level %d: concurrent encode differs from single-threaded golden", lvl)
					return
				}

				decoded, err := Decode(encoded)
				if err != nil {
					t.Errorf("Decode(level %d): %v", lvl, err)
					return
				}
				reEncoded, err := EncodeWithLevel(decoded, lvl)
				if err != nil {
					t.Errorf("re-encode decoded index (level %d): %v", lvl, err)
					return
				}
				if !bytes.Equal(reEncoded, want) {
					t.Errorf("level %d: decode->re-encode differs from golden (faithless decode)", lvl)
					return
				}
			}
		}(g)
	}
	wg.Wait()
}
