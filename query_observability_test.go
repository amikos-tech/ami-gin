package gin_test

import (
	"context"
	"log/slog"
	"testing"

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
func buildAdaptiveInvariantIndex(t *testing.T, numRGs int) *gin.GINIndex {
	t.Helper()
	idx := gin.NewGINIndex()
	idx.Header.NumRowGroups = uint32(numRGs)
	idx.PathDirectory = []gin.PathEntry{{
		PathID:   0,
		PathName: "$.field",
		Mode:     gin.PathModeAdaptiveHybrid,
	}}
	// pathLookup is unexported; use the exported SetPathLookup if available,
	// or use a builder + Finalize roundtrip that seeds the lookup table.
	// Since pathLookup is unexported we set it through the only available
	// public path: build via the builder and then patch the path directory.
	// For now, construct it via builder so pathLookup is seeded correctly.
	_ = idx
	b, err := gin.NewBuilder(gin.DefaultConfig(), numRGs)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	for i := 0; i < numRGs; i++ {
		if err := b.AddDocument(gin.DocID(i), []byte(`{"field":"cold"}`)); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	built := b.Finalize()
	// Force the path to adaptive-hybrid mode so evaluateEQ triggers adaptiveInvariantAllRGs.
	built.PathDirectory[0].Mode = gin.PathModeAdaptiveHybrid
	// Remove the adaptive index so lookupAdaptiveStringMatch returns ok=false.
	delete(built.AdaptiveStringIndexes, 0)
	// The bloom filter must pass so we reach the adaptive lookup.
	built.GlobalBloom.AddString("$.field=hot")
	return built
}
