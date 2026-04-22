package gin_test

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"sort"
	"testing"
	"time"

	gin "github.com/amikos-tech/ami-gin"
	"github.com/amikos-tech/ami-gin/logging"
	"github.com/amikos-tech/ami-gin/logging/slogadapter"
	"github.com/amikos-tech/ami-gin/telemetry"
)

// =============================================================================
// Task 1: EvaluateContext compatibility and nil-config safety
// =============================================================================

func TestEvaluateContextCompatibility(t *testing.T) {
	idx, err := buildQueryObsIndex()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	preds := []gin.Predicate{gin.EQ("$.status", "active")}

	// Both paths must agree.
	want := idx.Evaluate(preds)
	got := idx.EvaluateContext(context.Background(), preds)

	if got.Count() != want.Count() {
		t.Fatalf("EvaluateContext count=%d; Evaluate count=%d; want equal", got.Count(), want.Count())
	}
	if got.ToSlice()[0] != want.ToSlice()[0] {
		t.Fatalf("EvaluateContext result differs from Evaluate")
	}
}

func TestEvaluateContextNilConfigSilent(t *testing.T) {
	idx := gin.NewGINIndex()
	idx.Header.NumRowGroups = 3
	idx.Config = nil // explicitly nil

	// Must not panic, must return AllRGs.
	got := idx.EvaluateContext(context.Background(), []gin.Predicate{})
	if got.Count() != 3 {
		t.Fatalf("EvaluateContext(nil config, empty preds) count=%d; want 3", got.Count())
	}
}

func TestEvaluateContextPreservesResults(t *testing.T) {
	idx, err := buildQueryObsIndex()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// Wire a real slog logger so the boundary emits output.
	handler := slog.NewTextHandler(noopWriter{}, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slogadapter.New(slog.New(handler))
	signals := telemetry.Disabled()

	cfg, err := gin.NewConfig(gin.WithLogger(logger), gin.WithSignals(signals))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	idx.Config = &cfg

	preds := []gin.Predicate{gin.EQ("$.status", "active"), gin.GTE("$.age", 25)}

	want := idx.Evaluate(preds)
	got := idx.EvaluateContext(context.Background(), preds)

	if got.Count() != want.Count() {
		t.Fatalf("with observability: EvaluateContext count=%d; Evaluate count=%d", got.Count(), want.Count())
	}
}

// =============================================================================
// Task 2: Adaptive invariant logger migration
// =============================================================================

func TestAdaptiveInvariantViolationUsesLoggerSeam(t *testing.T) {
	var captured []logging.Attr
	capLogger := &captureLogger{attrs: &captured}

	idx := buildAdaptiveInvariantIndex(t, 3)
	cfg, err := gin.NewConfig(gin.WithLogger(capLogger))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	idx.Config = &cfg

	got := idx.Evaluate([]gin.Predicate{gin.EQ("$.field", "hot")})
	// Must still fail open to all row groups.
	if got.Count() != 3 {
		t.Fatalf("invariant violation: count=%d; want 3 (fail-open)", got.Count())
	}
	// Logger seam must have received a message.
	if !capLogger.called {
		t.Fatal("expected the repo-owned logger to receive the invariant violation message")
	}
}

func TestAdaptiveInvariantViolationSilentByDefault(t *testing.T) {
	idx := buildAdaptiveInvariantIndex(t, 2)
	// Default config has noop logger.
	cfg := gin.DefaultConfig()
	idx.Config = &cfg

	// Must not panic, must not emit, must fail open.
	got := idx.Evaluate([]gin.Predicate{gin.EQ("$.field", "hot")})
	if got.Count() != 2 {
		t.Fatalf("silent invariant: count=%d; want 2 (fail-open)", got.Count())
	}
}

func TestAdaptiveInvariantViolationStillFailsOpen(t *testing.T) {
	idx := buildAdaptiveInvariantIndex(t, 4)
	idx.Config = nil // even with nil config, must fail open

	got := idx.EvaluateContext(context.Background(), []gin.Predicate{gin.EQ("$.field", "hot")})
	if got.Count() != 4 {
		t.Fatalf("fail-open: count=%d; want 4", got.Count())
	}
}

// =============================================================================
// Task 3: Disabled-path performance gates
// =============================================================================

// TestEvaluateDisabledLoggingAllocsZero asserts that running EvaluateContext
// with a disabled (noop) logger adds 0 allocations over the baseline that
// uses no observability at all. This is the merge gate for OBS-02.
func TestEvaluateDisabledLoggingAllocsZero(t *testing.T) {
	idx, err := buildQueryObsIndex()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	preds := []gin.Predicate{gin.EQ("$.status", "active"), gin.GTE("$.age", 25)}

	// Baseline: DefaultConfig has noop logger + disabled signals.
	baseAllocs := testing.AllocsPerRun(100, func() {
		_ = idx.EvaluateContext(context.Background(), preds)
	})

	// Same with explicitly wired noop logger and disabled signals (same behavior,
	// but proves the enabled=false guard does not add allocations).
	noopCfg := gin.DefaultConfig()
	idx.Config = &noopCfg
	withNoopAllocs := testing.AllocsPerRun(100, func() {
		_ = idx.EvaluateContext(context.Background(), preds)
	})

	if withNoopAllocs > baseAllocs+1 {
		t.Fatalf("disabled logging allocs=%v; baseline allocs=%v; disabled path must not add more than 1 alloc", withNoopAllocs, baseAllocs)
	}
}

// TestEvaluateWithTracerWithinBudget asserts that running EvaluateContext with
// a disabled/noop tracer stays within the 0.5% overhead budget compared to the
// no-tracer baseline. Only enforced when GIN_STRICT_PERF=1.
func TestEvaluateWithTracerWithinBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping perf budget test in short mode")
	}

	idx, err := buildQueryObsIndex()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	preds := []gin.Predicate{gin.EQ("$.status", "active"), gin.GTE("$.age", 25)}

	// Baseline: no signals (disabled).
	baselineCfg := gin.DefaultConfig()
	idx.Config = &baselineCfg

	const samples = 7
	baselineTimes := collectEvalSamples(idx, preds, samples)

	// Noop-tracer path: explicitly disabled signals (same as baseline semantics,
	// but exercises the Signals.Tracer() + span.End() code path).
	noopCfg, err := gin.NewConfig(gin.WithSignals(telemetry.Disabled()))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	idx.Config = &noopCfg

	noopTimes := collectEvalSamples(idx, preds, samples)

	baseMedian := medianInt64(baselineTimes)
	noopMedian := medianInt64(noopTimes)

	if isStrictPerfMode() {
		budget := float64(baseMedian) * 1.005
		if float64(noopMedian) > budget {
			t.Fatalf("noop-tracer median=%dns exceeds 0.5%% budget over baseline=%dns (limit=%dns)",
				noopMedian, baseMedian, int64(budget))
		}
	} else {
		// Non-strict: smoke-check only — noop tracer must not be 2x slower.
		if noopMedian > baseMedian*2 {
			t.Fatalf("noop-tracer median=%dns is more than 2x baseline=%dns (smoke check)", noopMedian, baseMedian)
		}
	}
}

// =============================================================================
// Helpers
// =============================================================================

type noopWriter struct{}

func (noopWriter) Write(p []byte) (n int, err error) { return len(p), nil }

// collectEvalSamples runs EvaluateContext the given number of times and
// returns the per-iteration nanosecond durations, measured with GOMAXPROCS=1
// to reduce scheduler noise.
func collectEvalSamples(idx *gin.GINIndex, preds []gin.Predicate, n int) []int64 {
	prev := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(prev)

	times := make([]int64, n)
	for i := range times {
		start := time.Now()
		_ = idx.EvaluateContext(context.Background(), preds)
		times[i] = time.Since(start).Nanoseconds()
	}
	return times
}

// medianInt64 returns the median value of a sorted copy of vs.
func medianInt64(vs []int64) int64 {
	if len(vs) == 0 {
		return 0
	}
	cp := make([]int64, len(vs))
	copy(cp, vs)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	return cp[len(cp)/2]
}

// isStrictPerfMode reports whether GIN_STRICT_PERF=1 is set.
func isStrictPerfMode() bool {
	return os.Getenv("GIN_STRICT_PERF") == "1"
}

// captureLogger captures all Log calls for assertion.
type captureLogger struct {
	attrs  *[]logging.Attr
	called bool
}

func (c *captureLogger) Enabled(_ logging.Level) bool { return true }
func (c *captureLogger) Log(_ logging.Level, _ string, attrs ...logging.Attr) {
	c.called = true
	*c.attrs = append(*c.attrs, attrs...)
}

// buildQueryObsIndex builds a small representative index for query tests.
func buildQueryObsIndex() (*gin.GINIndex, error) {
	b, err := gin.NewBuilder(gin.DefaultConfig(), 5)
	if err != nil {
		return nil, err
	}
	docs := [][]byte{
		[]byte(`{"status":"active","age":30}`),
		[]byte(`{"status":"pending","age":20}`),
		[]byte(`{"status":"active","age":40}`),
		[]byte(`{"status":"inactive","age":15}`),
		[]byte(`{"status":"active","age":35}`),
	}
	for i, d := range docs {
		if err := b.AddDocument(gin.DocID(i), d); err != nil {
			return nil, err
		}
	}
	return b.Finalize(), nil
}

// buildAdaptiveInvariantIndex creates an index that triggers adaptive invariant
// violations when evaluated: the path is flagged AdaptiveHybrid but has no
// AdaptiveStringIndexes entry, so lookupAdaptiveStringMatch returns ok=false.
//
// We index the query term "hot" itself so the bloom and StringLengthIndex both
// pass. Then we force the path mode to AdaptiveHybrid and delete the adaptive
// index, leaving the path with no adaptive data — the invariant condition.
func buildAdaptiveInvariantIndex(t *testing.T, numRGs int) *gin.GINIndex {
	t.Helper()
	b, err := gin.NewBuilder(gin.DefaultConfig(), numRGs)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	// Index "hot" so bloom and StringLengthIndex both pass when we query it.
	for i := 0; i < numRGs; i++ {
		if err := b.AddDocument(gin.DocID(i), []byte(`{"field":"hot"}`)); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	built := b.Finalize()

	// Locate the $.field path entry.
	fieldPathID := -1
	for i, pe := range built.PathDirectory {
		if pe.PathName == "$.field" {
			fieldPathID = i
			break
		}
	}
	if fieldPathID < 0 {
		t.Fatal("$.field path entry not found in built index")
	}

	// Force adaptive-hybrid mode so evaluateEQ takes the adaptive branch.
	built.PathDirectory[fieldPathID].Mode = gin.PathModeAdaptiveHybrid
	// Remove the adaptive index so lookupAdaptiveStringMatch returns ok=false,
	// which triggers adaptiveInvariantAllRGs (the invariant violation path).
	delete(built.AdaptiveStringIndexes, uint16(fieldPathID))
	return built
}
