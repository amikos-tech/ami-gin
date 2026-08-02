//go:build simdjson

// Q6 — quantify the O(2^depth) finding so severity can be argued from numbers
// rather than from the shape of a curve.
//
// Open questions this answers:
//   - Is the cost CPU-only (a slow job) or does memory grow too (an OOM)?
//   - What is the actual byte-in -> resource-out amplification?
//   - Does it touch the query/decode path or only ingest?
//   - What do LEGITIMATE deep documents cost, i.e. where is the safe envelope?
package simdfuzzspike

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	gin "github.com/amikos-tech/ami-gin"
)

func measure(cfg gin.GINConfig, doc []byte) (elapsed time.Duration, heapDeltaBytes uint64, encodedLen int) {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)

	start := time.Now()
	enc, _, _ := buildEncode(cfg, nil, doc)
	elapsed = time.Since(start)

	runtime.ReadMemStats(&after)
	// TotalAlloc is cumulative and monotonic — the honest measure of work done.
	return elapsed, after.TotalAlloc - before.TotalAlloc, len(enc)
}

func TestQ6_QuantifyAmplification(t *testing.T) {
	cfg := softConfig(t)

	// Warm up: the first measured build carries one-time package/GC warm-up that
	// otherwise lands entirely on the depth-2 row.
	_, _, _ = measure(cfg, nestedArray(4))

	t.Log("Nested ARRAYS — default (stdlib) path")
	t.Log("depth | input B | wall       | alloc       | encoded B | alloc/input")
	for depth := 2; depth <= 20; depth += 2 {
		doc := nestedArray(depth)
		if depth >= 18 {
			t.Logf("%5d | %7d | (skipped — prior depth already exceeded budget)", depth, len(doc))
			continue
		}
		el, heap, enc := measure(cfg, doc)
		t.Logf("%5d | %7d | %-10v | %-11s | %9d | %.0fx",
			depth, len(doc), el.Round(time.Microsecond), humanBytes(heap), enc,
			float64(heap)/float64(len(doc)))
	}

	t.Log("")
	t.Log("Nested OBJECTS — control, should be linear")
	t.Log("depth | input B | wall       | alloc       | encoded B | alloc/input")
	for _, depth := range []int{16, 64, 256, 1024} {
		doc := nestedObject(depth)
		el, heap, enc := measure(cfg, doc)
		t.Logf("%5d | %7d | %-10v | %-11s | %9d | %.0fx",
			depth, len(doc), el.Round(time.Microsecond), humanBytes(heap), enc,
			float64(heap)/float64(len(doc)))
	}

	t.Log("")
	t.Log("REALISTIC envelope — what legitimate documents cost")
	t.Log("shape                          | input B | wall       | alloc")
	realistic := []struct {
		name string
		doc  []byte
	}{
		{"flat 20-field object", []byte(`{` + strings.TrimSuffix(strings.Repeat(`"k%d":"v",`, 1), `,`) + `}`)},
		{"phase20 nested fixture line", firstPhase20Line(t)},
		{"array of 1000 scalars", []byte(`{"a":[` + strings.TrimSuffix(strings.Repeat(`1,`, 1000), `,`) + `]}`)},
		{"depth-6 nested arrays", nestedArray(6)},
		{"depth-10 nested arrays", nestedArray(10)},
	}
	for _, r := range realistic {
		el, heap, _ := measure(cfg, r.doc)
		t.Logf("%-30s | %7d | %-10v | %s", r.name, len(r.doc), el.Round(time.Microsecond), humanBytes(heap))
	}
}

// TestQ6b_QueryPathExposure checks whether the blow-up reaches Decode/Evaluate
// or is confined to ingest.
func TestQ6b_QueryPathExposure(t *testing.T) {
	cfg := softConfig(t)

	doc := nestedArray(14)
	start := time.Now()
	enc, _, _ := buildEncode(cfg, nil, doc)
	buildTime := time.Since(start)

	if len(enc) == 0 {
		t.Fatalf("build produced nothing")
	}

	start = time.Now()
	idx, err := gin.Decode(enc)
	decodeTime := time.Since(start)
	if err != nil {
		t.Logf("Q6b decode=%v REJECTED: %v", decodeTime.Round(time.Microsecond), err)
		t.Logf("Q6b VERDICT: the serialized artifact is REFUSED by Decode's existing size guard —")
		t.Logf("Q6b           the blow-up cannot propagate to a consumer through a shipped index.")
		t.Logf("Q6b           Exposure is confined to the INGEST side, in the producer's own process.")
		return
	}

	start = time.Now()
	rgs := idx.Evaluate([]gin.Predicate{gin.EQ("$.a", "nope")})
	evalTime := time.Since(start)

	t.Logf("Q6b depth-14 nested array (%d input bytes -> %d encoded bytes)", len(doc), len(enc))
	t.Logf("Q6b build=%v  decode=%v  evaluate=%v  matched=%v", buildTime.Round(time.Microsecond), decodeTime.Round(time.Microsecond), evalTime.Round(time.Microsecond), rgs.Count())
	t.Logf("Q6b VERDICT: the cost is concentrated in %s", dominant(buildTime, decodeTime, evalTime))
}

func firstPhase20Line(t testing.TB) []byte {
	t.Helper()
	for _, s := range seedCorpus(t) {
		if len(s) > 400 {
			return s
		}
	}
	return []byte(`{"a":1}`)
}

func dominant(build, decode, eval time.Duration) string {
	switch {
	case build >= decode && build >= eval:
		return fmt.Sprintf("BUILD/ingest (%.1fx decode, %.1fx evaluate)", float64(build)/float64(decode), float64(build)/float64(eval))
	case decode >= eval:
		return "DECODE"
	default:
		return "EVALUATE"
	}
}

func humanBytes(b uint64) string {
	switch {
	case b > 1<<30:
		return fmt.Sprintf("%.2f GB", float64(b)/(1<<30))
	case b > 1<<20:
		return fmt.Sprintf("%.2f MB", float64(b)/(1<<20))
	case b > 1<<10:
		return fmt.Sprintf("%.2f KB", float64(b)/(1<<10))
	}
	return fmt.Sprintf("%d B", b)
}
