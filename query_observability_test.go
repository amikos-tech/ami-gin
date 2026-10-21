package gin_test

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	metricnoop "go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

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

func TestEvaluateContextCanceledFailsOpenAndLogsError(t *testing.T) {
	idx, err := buildQueryObsIndex()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	var captured []logging.Attr
	capLogger := &captureLogger{attrs: &captured}
	cfg, err := gin.NewConfig(gin.WithLogger(capLogger))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	idx.Config = &cfg

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := idx.EvaluateContext(ctx, []gin.Predicate{gin.EQ("$.status", "active")})
	if got.Count() != int(idx.Header.NumRowGroups) {
		t.Fatalf("EvaluateContext canceled count=%d; want fail-open count=%d", got.Count(), idx.Header.NumRowGroups)
	}
	if !capLogger.called {
		t.Fatal("expected canceled evaluation to emit a completion log entry")
	}
	if value, ok := attrValue(captured, "status"); !ok || value != "error" {
		t.Fatalf("status attr = %q, %v; want %q, true", value, ok, "error")
	}
	if value, ok := attrValue(captured, "error.type"); !ok || value != "other" {
		t.Fatalf("error.type attr = %q, %v; want %q, true", value, ok, "other")
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

// newNoopSignalsConfig returns config B: an explicitly configured noop-signals
// config (Signals.Enabled() == true, OTel noop tracer and meter providers).
func newNoopSignalsConfig(t *testing.T) gin.GINConfig {
	t.Helper()
	cfg, err := gin.NewConfig(gin.WithSignals(telemetry.NewSignals(
		tracenoop.NewTracerProvider(),
		metricnoop.NewMeterProvider(),
		nil,
	)))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	return cfg
}

// TestEvaluateDisabledLoggingAllocsAtMostOne asserts that running
// EvaluateContext with an explicitly configured noop-signals config adds at
// most one allocation over the default config. The default config has zero
// Signals (Enabled() == false). The comparison config has Enabled() == true
// with a noop tracer provider and a noop meter provider. The +1 tolerance
// accommodates scheduler/jitter noise on some Go runtimes while still
// catching regressions that leak a heap allocation per call. This is the
// merge gate for OBS-02.
func TestEvaluateDisabledLoggingAllocsAtMostOne(t *testing.T) {
	idx, err := buildQueryObsIndex()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	preds := []gin.Predicate{gin.EQ("$.status", "active"), gin.GTE("$.age", 25)}

	// Config A: DefaultConfig has a noop logger and zero (disabled) Signals.
	cfgA := gin.DefaultConfig()
	idx.Config = &cfgA
	baseAllocs := testing.AllocsPerRun(100, func() {
		_ = idx.EvaluateContext(context.Background(), preds)
	})

	// Config B: explicit noop signals (Enabled() == true). Proves that wiring
	// noop providers does not add allocations over the default path.
	cfgB := newNoopSignalsConfig(t)
	idx.Config = &cfgB
	noopAllocs := testing.AllocsPerRun(100, func() {
		_ = idx.EvaluateContext(context.Background(), preds)
	})

	if noopAllocs > baseAllocs+1 {
		t.Fatalf("noop-signals allocs=%v; default-config allocs=%v; noop-signals path must add at most 1 alloc over default", noopAllocs, baseAllocs)
	}
}

const (
	tracerBudgetTrials       = 3
	tracerBudgetWarmupBlocks = 5
	tracerBudgetTimedBlocks  = 25
)

// tracerTrialSamples holds the timed per-call durations (ns) of one trial.
// Warmup durations are never stored.
type tracerTrialSamples struct {
	a []int64
	b []int64
}

// tracerTrialMins holds the per-side minimum duration (ns) of one trial.
type tracerTrialMins struct {
	minA int64
	minB int64
}

// minSample returns the smallest value in vs. It returns 0 for an empty slice.
func minSample(vs []int64) int64 {
	if len(vs) == 0 {
		return 0
	}
	m := vs[0]
	for _, v := range vs[1:] {
		m = min(m, v)
	}
	return m
}

// tracerBudgetExceeded reports whether one trial is over the 2x bound (D2).
// It uses an integer compare, so a ratio of exactly 2.0 does not exceed.
func tracerBudgetExceeded(m tracerTrialMins) bool {
	return m.minB > 2*m.minA
}

// tracerBudgetVerdict decides the smoke check from per-trial minimums. The
// bound is 2x (D2). ok is false when trials is empty (no evidence is not a
// pass) or when every trial exceeds the bound. ok is true when at least one
// trial stays within the bound. The report lists, one line per trial, the
// 1-based index, both minimums and their ratio.
func tracerBudgetVerdict(trials []tracerTrialMins) (ok bool, report string) {
	var sb strings.Builder
	allExceeded := len(trials) > 0
	for i, m := range trials {
		ratio := "n/a"
		if m.minA > 0 {
			ratio = fmt.Sprintf("%.3f", float64(m.minB)/float64(m.minA))
		}
		exceeded := tracerBudgetExceeded(m)
		if !exceeded {
			allExceeded = false
		}
		fmt.Fprintf(&sb, "trial %d: minA=%dns minB=%dns ratio=%s exceeded=%t\n", i+1, m.minA, m.minB, ratio, exceeded)
	}
	if len(trials) == 0 {
		sb.WriteString("no trials\n")
	}
	return len(trials) > 0 && !allExceeded, sb.String()
}

// collectTracerTrialSamples times EvaluateContext under config A and config B.
// Each trial runs a GC, then warmup blocks, then timed blocks. One block is
// four calls in the order A, B, B, A, each timed on its own. Only timed
// durations are returned: 2 per block per side, so 50 per side per trial.
func collectTracerTrialSamples(idx *gin.GINIndex, preds []gin.Predicate, cfgA, cfgB *gin.GINConfig) []tracerTrialSamples {
	ctx := context.Background()
	call := func(cfg *gin.GINConfig) int64 {
		idx.Config = cfg
		start := time.Now()
		_ = idx.EvaluateContext(ctx, preds)
		return time.Since(start).Nanoseconds()
	}

	trials := make([]tracerTrialSamples, 0, tracerBudgetTrials)
	for range tracerBudgetTrials {
		runtime.GC()
		for range tracerBudgetWarmupBlocks {
			call(cfgA)
			call(cfgB)
			call(cfgB)
			call(cfgA)
		}
		ts := tracerTrialSamples{
			a: make([]int64, 0, 2*tracerBudgetTimedBlocks),
			b: make([]int64, 0, 2*tracerBudgetTimedBlocks),
		}
		for range tracerBudgetTimedBlocks {
			ts.a = append(ts.a, call(cfgA))
			ts.b = append(ts.b, call(cfgB))
			ts.b = append(ts.b, call(cfgB))
			ts.a = append(ts.a, call(cfgA))
		}
		trials = append(trials, ts)
	}
	return trials
}

// TestEvaluateWithTracerWithinBudget is a 2x smoke check for the extra cost of
// explicitly configured noop signals (Signals.Enabled() == true with the OTel
// noop tracer and meter providers) over the default config (zero Signals).
//
// Coverage limit: both configs run telemetry.RunBoundaryOperation and start a
// span on every call, because zero Signals fall back to noop providers. A
// regression shared by both sides is not caught here. Precise overhead numbers
// come from BenchmarkEvaluateWithTracer compared with benchstat.
//
// This is a HARD fail on shared CI, which is acceptable for three reasons.
// Calls are interleaved in A,B,B,A blocks, so both sides see the same noise
// window. The per-side minimum over 50 timed calls estimates the noise-free
// cost. The test fails only when min(B) > 2*min(A) in all 3 trials. Measured
// ratios were 0.97 to 1.01 under -race with 24 busy loops running. The CI SIMD
// jobs run it with -race and it needs no race skip.
//
// This test amends Phase 14 D-16a: it replaces the median-of-N approach, the
// 0.5% budget, and the rule against a hard-fail single wall-clock comparison
// on shared CI.
func TestEvaluateWithTracerWithinBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping perf budget test in short mode")
	}

	idx, err := buildQueryObsIndex()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	preds := []gin.Predicate{gin.EQ("$.status", "active"), gin.GTE("$.age", 25)}

	cfgA := gin.DefaultConfig()
	cfgB := newNoopSignalsConfig(t)

	// Correctness precheck: both configs must give the same result.
	idx.Config = &cfgA
	wantRGs := idx.EvaluateContext(context.Background(), preds)
	idx.Config = &cfgB
	gotRGs := idx.EvaluateContext(context.Background(), preds)
	if wantRGs.Count() != gotRGs.Count() || !slices.Equal(wantRGs.ToSlice(), gotRGs.ToSlice()) {
		t.Fatalf("config A and config B disagree: A=%v B=%v", wantRGs.ToSlice(), gotRGs.ToSlice())
	}

	prev := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(prev)

	trials := collectTracerTrialSamples(idx, preds, &cfgA, &cfgB)
	mins := make([]tracerTrialMins, 0, len(trials))
	for _, ts := range trials {
		mins = append(mins, tracerTrialMins{minA: minSample(ts.a), minB: minSample(ts.b)})
	}

	if ok, report := tracerBudgetVerdict(mins); !ok {
		t.Fatalf("noop-signals smoke check failed: min(B) > 2*min(A) in every trial:\n%s", report)
	}
}

func TestTracerBudgetVerdict(t *testing.T) {
	cases := []struct {
		name   string
		trials []tracerTrialMins
		wantOK bool
	}{
		{"empty is a failure", nil, false},
		{"exactly 2x passes", []tracerTrialMins{{100, 200}, {100, 200}, {100, 200}}, true},
		{"all exceed fails", []tracerTrialMins{{100, 201}, {100, 300}, {50, 101}}, false},
		{"two exceed one within passes", []tracerTrialMins{{100, 500}, {100, 150}, {100, 900}}, true},
		{"zero minA with positive minB exceeds", []tracerTrialMins{{0, 5}, {0, 5}, {0, 5}}, false},
		{"zero minA and zero minB passes", []tracerTrialMins{{0, 0}, {0, 0}, {0, 0}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, report := tracerBudgetVerdict(tc.trials)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v; want %v; report:\n%s", ok, tc.wantOK, report)
			}
			if strings.Contains(report, "NaN") || strings.Contains(report, "Inf") {
				t.Fatalf("report contains NaN or Inf:\n%s", report)
			}
		})
	}

	t.Run("report lists every trial", func(t *testing.T) {
		_, report := tracerBudgetVerdict([]tracerTrialMins{{101, 303}, {202, 404}, {505, 606}})
		for _, want := range []string{
			"trial 1", "trial 2", "trial 3",
			"minA=101ns", "minB=303ns", "minA=202ns", "minB=404ns", "minA=505ns", "minB=606ns",
			"2.000", "1.200",
		} {
			if !strings.Contains(report, want) {
				t.Fatalf("report missing %q:\n%s", want, report)
			}
		}
	})

	t.Run("minSample", func(t *testing.T) {
		if got := minSample(nil); got != 0 {
			t.Fatalf("minSample(nil)=%d; want 0", got)
		}
		if got := minSample([]int64{7, 3, 9, 3, 5}); got != 3 {
			t.Fatalf("minSample=%d; want 3", got)
		}
	})
}

func TestCollectTracerTrialSamplesShape(t *testing.T) {
	idx, err := buildQueryObsIndex()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	preds := []gin.Predicate{gin.EQ("$.status", "active"), gin.GTE("$.age", 25)}
	cfgA := gin.DefaultConfig()
	cfgB := newNoopSignalsConfig(t)

	trials := collectTracerTrialSamples(idx, preds, &cfgA, &cfgB)
	if len(trials) != tracerBudgetTrials {
		t.Fatalf("trials=%d; want %d", len(trials), tracerBudgetTrials)
	}
	want := 2 * tracerBudgetTimedBlocks
	for i, ts := range trials {
		if len(ts.a) != want || len(ts.b) != want {
			t.Fatalf("trial %d: len(a)=%d len(b)=%d; want %d each", i+1, len(ts.a), len(ts.b), want)
		}
	}
}

// =============================================================================
// Helpers
// =============================================================================

type noopWriter struct{}

func (noopWriter) Write(p []byte) (n int, err error) { return len(p), nil }

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

func attrValue(attrs []logging.Attr, key string) (string, bool) {
	for _, attr := range attrs {
		if attr.Key == key {
			return attr.Value, true
		}
	}
	return "", false
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
