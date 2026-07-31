//go:build simdjson

// Spike 002 — differential fuzz harness viability for Phase 22 D-02.
//
// Deliberately built on ami-gin's PUBLIC API only (NewBuilder / WithParser /
// AddDocument / Finalize / Encode), so this lives in its own module and never
// touches the repo's go.mod-of-record — the Spike 001 convention.
package simdfuzzspike

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gin "github.com/amikos-tech/ami-gin"
)

const repoRoot = "../../.."

// Tightened after the first 60s run stalled at 0 execs/sec for 30 of 60
// seconds. Q5: array depth 12 already costs ~246ms per document, so a depth-12
// guard still starves the fuzzer. Depth 8 is ~18ms.
const (
	fuzzMaxArrayDepth = 8
	fuzzMaxInputBytes = 4096
)

// softConfig mirrors the soft-failure fixture Phase 21 used, because that is
// where the known stdlib/SIMD asymmetry lives.
func softConfig(t testing.TB) gin.GINConfig {
	t.Helper()
	cfg := gin.DefaultConfig()
	if err := gin.WithParserFailureMode(gin.IngestFailureSoft)(&cfg); err != nil {
		t.Fatalf("WithParserFailureMode: %v", err)
	}
	if err := gin.WithNumericFailureMode(gin.IngestFailureSoft)(&cfg); err != nil {
		t.Fatalf("WithNumericFailureMode: %v", err)
	}
	return cfg
}

// buildEncode runs one document through a FRESH builder and returns encoded
// bytes. A nil parser selects the default stdlib path.
func buildEncode(cfg gin.GINConfig, parser gin.Parser, doc []byte) (encoded []byte, addErr error, finalizeNil bool) {
	var opts []gin.BuilderOption
	if parser != nil {
		opts = append(opts, gin.WithParser(parser))
	}
	b, err := gin.NewBuilder(cfg, 4, opts...)
	if err != nil {
		return nil, err, false
	}
	addErr = b.AddDocument(gin.DocID(0), doc)
	idx := b.Finalize()
	if idx == nil {
		return nil, addErr, true
	}
	encoded, encErr := gin.Encode(idx)
	if encErr != nil && addErr == nil {
		addErr = encErr
	}
	return encoded, addErr, false
}

// ---------------------------------------------------------------------------
// Q1 — what does NewSIMDParser() actually cost?
// ---------------------------------------------------------------------------

func TestQ1_ConstructionCost(t *testing.T) {
	const n = 20

	start := time.Now()
	first, err := gin.NewSIMDParser()
	if err != nil {
		t.Fatalf("NewSIMDParser (cold): %v", err)
	}
	cold := time.Since(start)
	if err := first.Close(); err != nil {
		t.Errorf("Close cold parser: %v", err)
	}

	warm := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		s := time.Now()
		p, err := gin.NewSIMDParser()
		if err != nil {
			t.Fatalf("NewSIMDParser (warm %d): %v", i, err)
		}
		warm = append(warm, time.Since(s))
		if err := p.Close(); err != nil {
			t.Errorf("Close warm parser %d: %v", i, err)
		}
	}

	var total time.Duration
	worst := time.Duration(0)
	for _, d := range warm {
		total += d
		if d > worst {
			worst = d
		}
	}
	mean := total / time.Duration(len(warm))

	t.Logf("Q1 construction cost: cold=%v  warm_mean=%v  warm_worst=%v  (n=%d)", cold, mean, worst, n)
	t.Logf("Q1 implied ceiling if constructed per fuzz iteration: ~%.0f iterations/sec", float64(time.Second)/float64(mean))
}

// ---------------------------------------------------------------------------
// Q2 — does a reused CloseableParser survive a poisoned (tragic) builder?
// ---------------------------------------------------------------------------

// poisonPayload is a >uint64 BIGINT, which Phase 21 documented as failing the
// entire SIMD Parse(). Whether that is merely an error or a *lifecycle* error
// (builder.go:423 isParserLifecycleError -> tragicErr) is exactly the unknown.
const poisonPayload = `{"n":179769313486231570000000000000000000000000000000000000000000000000000000000000000}`

func TestQ2_ParserSurvivesPoisonedBuilder(t *testing.T) {
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

	// 1. Establish the parser works.
	before, addErr, nilIdx := buildEncode(cfg, parser, []byte(`{"a":1,"b":"x"}`))
	t.Logf("Q2 baseline: addErr=%v finalizeNil=%v encodedLen=%d", addErr, nilIdx, len(before))
	if len(before) == 0 {
		t.Fatalf("Q2 baseline produced no encoded bytes; harness is wrong")
	}

	// 2. Attempt to poison a builder with the same parser instance.
	_, poisonErr, poisonNil := buildEncode(cfg, parser, []byte(poisonPayload))
	t.Logf("Q2 poison attempt: addErr=%v finalizeNil=%v", poisonErr, poisonNil)
	tragic := poisonNil || (poisonErr != nil && strings.Contains(poisonErr.Error(), "tragic"))
	t.Logf("Q2 builder went tragic: %v", tragic)

	// 3. THE QUESTION: is the parser still usable with a fresh builder?
	after, addErr2, nilIdx2 := buildEncode(cfg, parser, []byte(`{"a":1,"b":"x"}`))
	t.Logf("Q2 after-poison reuse: addErr=%v finalizeNil=%v encodedLen=%d", addErr2, nilIdx2, len(after))

	if len(after) == 0 {
		t.Fatalf("Q2 VERDICT: parser is NOT reusable after a poisoned builder — harness must construct per iteration")
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("Q2 VERDICT: parser reusable but NOT deterministic after poisoning (%d vs %d bytes)", len(before), len(after))
	}
	t.Logf("Q2 VERDICT: parser survives a poisoned builder and stays byte-deterministic — reuse-across-iterations is viable")
}

// ---------------------------------------------------------------------------
// Q3 — handle leaks / stability across many iterations with ONE parser
// ---------------------------------------------------------------------------

func TestQ3_ParserReuseAtScale(t *testing.T) {
	if testing.Short() {
		t.Skip("scale probe")
	}
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

	docs := seedCorpus(t)
	const rounds = 400

	start := time.Now()
	var reference []byte
	failures := 0
	for r := 0; r < rounds; r++ {
		for _, d := range docs {
			enc, _, _ := buildEncode(cfg, parser, d)
			if len(enc) == 0 {
				failures++
			}
		}
		if r == 0 {
			reference, _, _ = buildEncode(cfg, parser, []byte(`{"a":1,"b":"x"}`))
		}
	}
	elapsed := time.Since(start)
	iterations := rounds * len(docs)

	final, _, _ := buildEncode(cfg, parser, []byte(`{"a":1,"b":"x"}`))
	drifted := !bytes.Equal(reference, final)

	t.Logf("Q3 reuse at scale: %d builder cycles on ONE parser in %v (%.0f/sec)", iterations, elapsed, float64(iterations)/elapsed.Seconds())
	t.Logf("Q3 empty-encode failures: %d", failures)
	t.Logf("Q3 output drifted after %d cycles: %v", iterations, drifted)
	if drifted {
		t.Errorf("Q3 VERDICT: parser output drifted under sustained reuse")
	}
	t.Logf("Q3 note: run with PURE_SIMDJSON_WARN_LEAKS=1 and check stderr for finalizer leak warnings")
}

// ---------------------------------------------------------------------------
// Q4 — day-one divergence yield
// ---------------------------------------------------------------------------

// seedCorpus draws from the Phase 20 checked-in fixtures plus hand-authored
// numeric/structural edges mirroring the authored parity fixtures.
func seedCorpus(t testing.TB) [][]byte {
	t.Helper()
	seeds := [][]byte{
		[]byte(`{"a":1}`),
		[]byte(`{"a":1.0}`),
		[]byte(`{"a":1e18}`),
		[]byte(`{"a":1.5}`),
		[]byte(`{"a":9007199254740993}`),
		[]byte(`{"a":18446744073709551615}`),
		[]byte(`{"a":-9223372036854775808}`),
		[]byte(poisonPayload),
		[]byte(`{"name":}`),
		[]byte(`{"name":"bob","overflow":1e400}`),
		[]byte(`{"s":"","n":null,"b":true,"arr":[],"o":{}}`),
		[]byte(`{"arr":[1,"two",false,null,{"k":"v"},[]]}`),
		[]byte(`{"dup":1,"dup":2}`),
		[]byte(`{"nested":{"deep":{"deeper":{"x":[1,2,3]}}}}`),
		[]byte(`{"unicode":"é中😀"}`),
	}

	pattern := filepath.Join(repoRoot, "testdata", "phase20", "*.jsonl")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob phase20 fixtures: %v", err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(filepath.Clean(f))
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		lines := bytes.Split(raw, []byte("\n"))
		// Cap per file so the seed corpus stays a corpus, not a dataset.
		for i, ln := range lines {
			if i >= 12 {
				break
			}
			if len(bytes.TrimSpace(ln)) == 0 {
				continue
			}
			seeds = append(seeds, ln)
		}
	}
	return seeds
}

func FuzzParserParity(f *testing.F) {
	cfg := softConfig(f)

	for _, s := range seedCorpus(f) {
		f.Add(s)
	}

	parser, err := gin.NewSIMDParser()
	if err != nil {
		f.Fatalf("NewSIMDParser: %v", err)
	}
	f.Cleanup(func() {
		if err := parser.Close(); err != nil {
			f.Errorf("Close parser: %v", err)
		}
	})

	f.Fuzz(func(t *testing.T, doc []byte) {
		// MANDATORY GUARD (Q5): stageMaterializedValue is O(2^depth) for nested
		// arrays because builder.go:619-627 recurses twice per element ([i] and
		// [*]). Without this, the fuzzer generates deep arrays and appears to
		// hang — a 37-byte depth-18 input exceeds 3 seconds on BOTH arms.
		if len(doc) > fuzzMaxInputBytes || arrayNestingDepth(doc) > fuzzMaxArrayDepth {
			return
		}

		stdEnc, stdErr, stdNil := buildEncode(cfg, nil, doc)
		simdEnc, simdErr, simdNil := buildEncode(cfg, parser, doc)

		stdOK := !stdNil && len(stdEnc) > 0
		simdOK := !simdNil && len(simdEnc) > 0

		switch {
		case stdOK && simdOK:
			// The real parity claim. A mismatch here is a Phase 19 HARD stop.
			if !bytes.Equal(stdEnc, simdEnc) {
				t.Fatalf("BYTE DIVERGENCE (hard-stop class)\ndoc: %q\nstdlib: %d bytes\nsimd:   %d bytes", doc, len(stdEnc), len(simdEnc))
			}
		case stdOK != simdOK:
			// The D-04 asymmetry class: one side ingests, the other rejects.
			// Recorded, not failed — this is the documented exclusion.
			// D-04 documented-exclusion class: one side ingests, the other
			// rejects. Not a parity break. Counted deterministically by Q4b.
			_, _ = stdErr, simdErr
			return
		default:
			// Both rejected. Agreement.
		}
	})
}

// arrayNestingDepth returns the maximum '[' nesting depth, ignoring brackets
// inside strings. Cheap syntactic guard — see Q5.
func arrayNestingDepth(doc []byte) int {
	depth, max := 0, 0
	inString, escaped := false, false
	for _, c := range doc {
		switch {
		case escaped:
			escaped = false
		case c == '\\' && inString:
			escaped = true
		case c == '"':
			inString = !inString
		case inString:
			// skip
		case c == '[':
			depth++
			if depth > max {
				max = depth
			}
		case c == ']':
			depth--
		}
	}
	return max
}
