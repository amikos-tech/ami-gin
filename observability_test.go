package gin_test

import (
	"bytes"
	"context"
	"log"
	"log/slog"
	"testing"

	gin "github.com/amikos-tech/ami-gin"
	"github.com/amikos-tech/ami-gin/logging"
	"github.com/amikos-tech/ami-gin/logging/slogadapter"
	"github.com/amikos-tech/ami-gin/logging/stdadapter"
	"github.com/amikos-tech/ami-gin/telemetry"
)

// --------------------------------------------------------------------------
// Task 1: Logging core tests
// --------------------------------------------------------------------------

func TestDefaultConfigObservabilityDefaults(t *testing.T) {
	cfg := gin.DefaultConfig()

	// Logger field must be set to a noop-compatible logger (not nil).
	if cfg.Logger == nil {
		t.Fatal("DefaultConfig().Logger must not be nil")
	}

	// Noop logger must not be enabled for any level.
	for _, level := range []logging.Level{
		logging.LevelDebug, logging.LevelInfo, logging.LevelWarn, logging.LevelError,
	} {
		if cfg.Logger.Enabled(level) {
			t.Errorf("DefaultConfig().Logger.Enabled(%v) = true; want false", level)
		}
	}

	// Signals must report disabled.
	if cfg.Signals.Enabled() {
		t.Fatal("DefaultConfig().Signals.Enabled() = true; want false")
	}
}

func TestWithLoggerRejectsNil(t *testing.T) {
	_, err := gin.NewConfig(gin.WithLogger(nil))
	if err == nil {
		t.Fatal("WithLogger(nil) must return an error")
	}
}

func TestLoggingAttrErrorTypeUnknownFallsBackToOther(t *testing.T) {
	a := logging.AttrErrorType("definitely_unknown_kind_xyz")
	if a.Value != "other" {
		t.Errorf("AttrErrorType(unknown).Value = %q; want %q", a.Value, "other")
	}
}

// --------------------------------------------------------------------------
// Task 2: Adapter tests
// --------------------------------------------------------------------------

func TestSlogAdapterNilFallsBackToNoop(t *testing.T) {
	l := slogadapter.New(nil)
	for _, level := range []logging.Level{
		logging.LevelDebug, logging.LevelInfo, logging.LevelWarn, logging.LevelError,
	} {
		if l.Enabled(level) {
			t.Errorf("slogadapter.New(nil).Enabled(%v) = true; want false", level)
		}
	}
	// Must not panic.
	l.Log(logging.LevelInfo, "test msg", logging.AttrOperation("op"))
}

func TestStdAdapterNilFallsBackToNoop(t *testing.T) {
	l := stdadapter.New(nil)
	for _, level := range []logging.Level{
		logging.LevelDebug, logging.LevelInfo, logging.LevelWarn, logging.LevelError,
	} {
		if l.Enabled(level) {
			t.Errorf("stdadapter.New(nil).Enabled(%v) = true; want false", level)
		}
	}
	// Must not panic.
	l.Log(logging.LevelInfo, "test msg", logging.AttrOperation("op"))
}

func TestStdAdapterPrefixesSeverity(t *testing.T) {
	var buf bytes.Buffer
	stdl := log.New(&buf, "", 0)
	l := stdadapter.New(stdl)

	l.Log(logging.LevelInfo, "hello", logging.AttrOperation("op"))
	got := buf.String()
	if len(got) == 0 {
		t.Fatal("stdadapter should have emitted output")
	}
	if got[:6] != "[INFO]" {
		t.Errorf("expected output to start with [INFO], got: %q", got)
	}
}

func TestSlogAdapterForwardsToSlog(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	sl := slog.New(handler)
	l := slogadapter.New(sl)

	if !l.Enabled(logging.LevelInfo) {
		t.Fatal("slogadapter backed by debug-level handler must be enabled for LevelInfo")
	}
	l.Log(logging.LevelInfo, "test", logging.AttrOperation("test-op"))
	if buf.Len() == 0 {
		t.Fatal("slogadapter must forward log entries to the underlying slog.Logger")
	}
}

// --------------------------------------------------------------------------
// Task 3: Telemetry / Signals tests
// --------------------------------------------------------------------------

func TestSignalsDisabledDefaults(t *testing.T) {
	s := telemetry.Disabled()
	if s.Enabled() {
		t.Fatal("telemetry.Disabled().Enabled() = true; want false")
	}
	// Must not panic on Tracer/Meter access.
	_ = s.Tracer("test-scope")
	_ = s.Meter("test-scope")
}

func TestSignalsEnabledSemantics(t *testing.T) {
	// Zero value must report disabled.
	var zero telemetry.Signals
	if zero.Enabled() {
		t.Fatal("zero-value Signals.Enabled() = true; want false")
	}

	// Disabled() must report disabled.
	if telemetry.Disabled().Enabled() {
		t.Fatal("Disabled().Enabled() = true; want false")
	}

	// NewSignals with non-nil providers must report enabled.
	s := telemetry.NewSignals(nil, nil, nil)
	// NewSignals with all nil providers: enabled is based on explicit construction.
	// Per plan D: "NewSignals(...) reports true once the runtime is intentionally constructed"
	if !s.Enabled() {
		t.Fatal("NewSignals(...) must report Enabled() = true")
	}
}

func TestSignalsShutdownNilContextNoop(t *testing.T) {
	s := telemetry.Disabled()
	// Must not panic with nil context.
	if err := s.Shutdown(nil); err != nil {
		t.Fatalf("Disabled().Shutdown(nil) returned error: %v", err)
	}
}

func TestRunBoundaryOperationNoop(t *testing.T) {
	s := telemetry.Disabled()
	called := false
	err := telemetry.RunBoundaryOperation(nil, s, telemetry.BoundaryConfig{
		Scope:     "test-scope",
		Operation: "test.op",
	}, func(_ context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("RunBoundaryOperation noop returned error: %v", err)
	}
	if !called {
		t.Fatal("RunBoundaryOperation must call the provided fn")
	}
}

// --------------------------------------------------------------------------
// Task 4: Config round-trip tests
// --------------------------------------------------------------------------

func TestConfigRoundTripObservabilityDefaults(t *testing.T) {
	idx, err := buildSmallIndex()
	if err != nil {
		t.Fatalf("build index: %v", err)
	}

	data, err := gin.Encode(idx)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded, err := gin.Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.Config == nil {
		// No config in index is acceptable; query paths must be nil-safe.
		return
	}

	// After decode, logger must be noop (not nil, not panicking).
	if decoded.Config.Logger == nil {
		t.Fatal("decoded Config.Logger must not be nil")
	}
	// Signals must be disabled after decode.
	if decoded.Config.Signals.Enabled() {
		t.Fatal("decoded Config.Signals must be disabled")
	}
}

// buildSmallIndex is a test helper that creates a minimal GIN index.
func buildSmallIndex() (*gin.GINIndex, error) {
	b, err := gin.NewBuilder(gin.DefaultConfig(), 10)
	if err != nil {
		return nil, err
	}
	if err := b.AddDocument(0, []byte(`{"name":"alice"}`)); err != nil {
		return nil, err
	}
	return b.Finalize(), nil
}
