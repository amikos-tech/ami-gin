//go:build simdjson

// Follow-up probes triggered by two surprises in the first pass:
//
//	Q1 warm construction came back at ~1µs, which contradicts the assumption
//	that per-iteration parser construction would be too slow to fuzz.
//	Q2 passed vacuously — the >uint64 BIGINT payload did NOT poison the
//	builder, so the original test never exercised the tragic path it claimed to.
//
// parser_simd.go:133-158 shows parserLifecycleError arises ONLY when the native
// document close fails, which requires an injected fake. So the real question
// is not "does the parser recover from poisoning" but "can any public-API input
// poison it at all".
package simdfuzzspike

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	gin "github.com/amikos-tech/ami-gin"
)

// adversarialInputs targets the places a native parser is most likely to
// diverge or destabilise: depth limits, truncation, encoding, non-object roots.
// NOTE: array nesting is capped at maxSafeArrayDepth because Q5 proved
// stageMaterializedValue is O(2^depth) for nested arrays (builder.go:619-627
// recurses twice per element: once as [i], once as [*]). The original 2000-deep
// input hung Q4b for 600s. Any real fuzz harness needs this same guard.
const maxSafeArrayDepth = 12

func adversarialInputs() [][]byte {
	deep := strings.Repeat(`{"a":`, 200) + `1` + strings.Repeat(`}`, 200)
	deeper := strings.Repeat(`[`, maxSafeArrayDepth) + `1` + strings.Repeat(`]`, maxSafeArrayDepth)
	wide := `{"a":[` + strings.TrimSuffix(strings.Repeat(`1,`, 5000), `,`) + `]}`

	return [][]byte{
		nil,
		[]byte(``),
		[]byte(`   `),
		[]byte(`{`),
		[]byte(`{"a"`),
		[]byte(`{"a":`),
		[]byte(`null`),
		[]byte(`123`),
		[]byte(`"bare string"`),
		[]byte(`[1,2,3]`),
		[]byte(`{"a":NaN}`),
		[]byte(`{"a":Infinity}`),
		[]byte(`{"a":-0.0}`),
		[]byte(`{"a":1e-400}`),
		[]byte(`{"a":00}`),
		[]byte(`{"a":.5}`),
		[]byte(`{"a":1.}`),
		[]byte(`{"a":0x10}`),
		[]byte("{\"a\":\"\xff\xfe invalid utf8\"}"),
		[]byte("{\"a\":\"nul\x00byte\"}"),
		[]byte(`{"a":"\ud800"}`),
		[]byte(deep),
		[]byte(deeper),
		[]byte(wide),
		bytes.Repeat([]byte(`{"k":"v"},`), 1000),
	}
}

// TestQ2b_NoPublicInputPoisonsBuilder is the honest replacement for Q2: sweep
// every seed plus adversarial input through ONE reused parser and check whether
// any of them produces a tragic builder or leaves the parser unusable.
func TestQ2b_NoPublicInputPoisonsBuilder(t *testing.T) {
	cfg := softConfig(t)

	parser, err := gin.NewSIMDParser()
	if err != nil {
		t.Fatalf("NewSIMDParser: %v", err)
	}
	t.Cleanup(func() {
		if err := parser.Close(); err != nil {
			t.Errorf("Close parser: %v", err)
		}
	})

	canary := []byte(`{"a":1,"b":"x"}`)
	reference, _, _ := buildEncode(cfg, parser, canary)
	if len(reference) == 0 {
		t.Fatalf("canary produced no bytes; harness is wrong")
	}

	corpus := append(seedCorpus(t), adversarialInputs()...)

	tragicCount := 0
	unusableAfter := 0
	for i, doc := range corpus {
		_, addErr, finalizeNil := buildEncode(cfg, parser, doc)

		tragic := finalizeNil || (addErr != nil && strings.Contains(addErr.Error(), "tragic"))
		if tragic {
			tragicCount++
			t.Logf("Q2b TRAGIC at input %d (%d bytes): %v", i, len(doc), addErr)
		}

		// After every single input, is the parser still usable?
		after, _, _ := buildEncode(cfg, parser, canary)
		if !bytes.Equal(reference, after) {
			unusableAfter++
			t.Errorf("Q2b parser unusable/non-deterministic after input %d (%d bytes): canary %d -> %d bytes",
				i, len(doc), len(reference), len(after))
		}
	}

	t.Logf("Q2b swept %d inputs (%d seeds + %d adversarial)", len(corpus), len(corpus)-len(adversarialInputs()), len(adversarialInputs()))
	t.Logf("Q2b inputs that poisoned the builder: %d", tragicCount)
	t.Logf("Q2b inputs that left the parser unusable: %d", unusableAfter)
	if tragicCount == 0 && unusableAfter == 0 {
		t.Logf("Q2b VERDICT: no public-API input reaches the tragic path; fuzz harness need not handle parser recovery")
	}
}

// TestQ1b_PerIterationConstruction checks whether the ~1µs warm construction
// from Q1 holds under sustained construct/use/close churn, and whether such
// parsers are functionally independent.
func TestQ1b_PerIterationConstruction(t *testing.T) {
	if testing.Short() {
		t.Skip("churn probe")
	}
	cfg := softConfig(t)
	canary := []byte(`{"a":1,"b":"x"}`)

	warm, err := gin.NewSIMDParser()
	if err != nil {
		t.Fatalf("NewSIMDParser (prime): %v", err)
	}
	reference, _, _ := buildEncode(cfg, warm, canary)
	if err := warm.Close(); err != nil {
		t.Fatalf("Close prime parser: %v", err)
	}

	const iterations = 5000
	start := time.Now()
	mismatches := 0
	for i := 0; i < iterations; i++ {
		p, err := gin.NewSIMDParser()
		if err != nil {
			t.Fatalf("NewSIMDParser at iteration %d: %v", i, err)
		}
		enc, _, _ := buildEncode(cfg, p, canary)
		if !bytes.Equal(reference, enc) {
			mismatches++
		}
		if err := p.Close(); err != nil {
			t.Fatalf("Close at iteration %d: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	t.Logf("Q1b construct+build+close x%d in %v (%.0f/sec, %v per iteration incl. build)",
		iterations, elapsed, float64(iterations)/elapsed.Seconds(), elapsed/iterations)
	t.Logf("Q1b canary mismatches across independently-constructed parsers: %d", mismatches)
	if mismatches != 0 {
		t.Errorf("Q1b VERDICT: independently-constructed parsers are NOT equivalent")
	} else {
		t.Logf("Q1b VERDICT: per-iteration construction is viable and deterministic")
	}
}

// TestQ4b_SeedCorpusDifferential runs the differential over the fixed seed
// corpus deterministically, so day-one yield is measured before fuzzing adds
// mutation noise.
func TestQ4b_SeedCorpusDifferential(t *testing.T) {
	cfg := softConfig(t)

	parser, err := gin.NewSIMDParser()
	if err != nil {
		t.Fatalf("NewSIMDParser: %v", err)
	}
	t.Cleanup(func() {
		if err := parser.Close(); err != nil {
			t.Errorf("Close parser: %v", err)
		}
	})

	corpus := append(seedCorpus(t), adversarialInputs()...)

	var agree, bothReject, asymmetry, byteDivergence int
	for i, doc := range corpus {
		stdEnc, stdErr, stdNil := buildEncode(cfg, nil, doc)
		simdEnc, simdErr, simdNil := buildEncode(cfg, parser, doc)

		stdOK := !stdNil && len(stdEnc) > 0
		simdOK := !simdNil && len(simdEnc) > 0

		switch {
		case stdOK && simdOK:
			if bytes.Equal(stdEnc, simdEnc) {
				agree++
			} else {
				byteDivergence++
				t.Errorf("Q4b BYTE DIVERGENCE at input %d (%d bytes): stdlib=%d simd=%d\n  doc=%s",
					i, len(doc), len(stdEnc), len(simdEnc), truncate(doc))
			}
		case stdOK != simdOK:
			asymmetry++
			t.Logf("Q4b ASYMMETRY at input %d: stdlibOK=%v simdOK=%v\n  doc=%s\n  stdErr=%v\n  simdErr=%v",
				i, stdOK, simdOK, truncate(doc), stdErr, simdErr)
		default:
			bothReject++
		}
	}

	t.Logf("Q4b over %d inputs: agree=%d bothReject=%d asymmetry=%d byteDivergence=%d",
		len(corpus), agree, bothReject, asymmetry, byteDivergence)
}

func truncate(b []byte) string {
	const max = 120
	if len(b) <= max {
		return fmt.Sprintf("%q", b)
	}
	return fmt.Sprintf("%q...(%d bytes total)", b[:max], len(b))
}
