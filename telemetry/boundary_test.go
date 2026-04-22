package telemetry

import (
	"context"
	stderrors "errors"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

type recordedSpan struct {
	name       string
	scope      string
	ended      bool
	status     codes.Code
	statusDesc string
	attrs      []attribute.KeyValue
	errors     []error
}

type histogramMeasurement struct {
	scope string
	name  string
	value float64
	attrs []attribute.KeyValue
}

type counterMeasurement struct {
	scope string
	name  string
	value int64
	attrs []attribute.KeyValue
}

type boundaryRecorder struct {
	spans      []*recordedSpan
	histograms []histogramMeasurement
	counters   []counterMeasurement
}

func TestRunBoundaryOperationRecordsSuccess(t *testing.T) {
	signals, rec := newRecordingSignals()

	err := RunBoundaryOperation(context.Background(), signals, BoundaryConfig{
		Scope:     "test-scope",
		Operation: "test.success",
	}, func(context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("RunBoundaryOperation() error = %v", err)
	}

	if len(rec.spans) != 1 {
		t.Fatalf("recorded spans = %d; want 1", len(rec.spans))
	}
	span := rec.spans[0]
	if span.scope != "test-scope" {
		t.Fatalf("span scope = %q; want %q", span.scope, "test-scope")
	}
	if span.name != "test.success" {
		t.Fatalf("span name = %q; want %q", span.name, "test.success")
	}
	if !span.ended {
		t.Fatal("span was not ended")
	}
	if span.status != codes.Ok {
		t.Fatalf("span status = %v; want %v", span.status, codes.Ok)
	}
	if len(span.errors) != 0 {
		t.Fatalf("recorded span errors = %d; want 0", len(span.errors))
	}
	if !hasAttr(span.attrs, "operation", "test.success") {
		t.Fatal("span attrs missing operation=test.success")
	}
	if !hasAttr(span.attrs, "status", "ok") {
		t.Fatal("span attrs missing status=ok")
	}
	if len(rec.histograms) != 1 {
		t.Fatalf("histogram measurements = %d; want 1", len(rec.histograms))
	}
	if rec.histograms[0].name != "ami_gin.operation.duration" {
		t.Fatalf("histogram name = %q; want %q", rec.histograms[0].name, "ami_gin.operation.duration")
	}
	if !hasAttr(rec.histograms[0].attrs, "status", "ok") {
		t.Fatal("duration attrs missing status=ok")
	}
	if len(rec.counters) != 0 {
		t.Fatalf("failure counter measurements = %d; want 0", len(rec.counters))
	}
}

func TestRunBoundaryOperationRecordsErrorAndFailureCount(t *testing.T) {
	signals, rec := newRecordingSignals()
	wantErr := stderrors.New("boom")

	err := RunBoundaryOperation(context.Background(), signals, BoundaryConfig{
		Scope:     "test-scope",
		Operation: "test.error",
		ClassifyError: func(error) string {
			return "io"
		},
	}, func(context.Context) error {
		return wantErr
	})
	if !stderrors.Is(err, wantErr) {
		t.Fatalf("RunBoundaryOperation() error = %v; want %v", err, wantErr)
	}

	if len(rec.spans) != 1 {
		t.Fatalf("recorded spans = %d; want 1", len(rec.spans))
	}
	span := rec.spans[0]
	if span.status != codes.Error {
		t.Fatalf("span status = %v; want %v", span.status, codes.Error)
	}
	if span.statusDesc != "io" {
		t.Fatalf("span status description = %q; want %q", span.statusDesc, "io")
	}
	if len(span.errors) != 1 || !stderrors.Is(span.errors[0], wantErr) {
		t.Fatalf("recorded span errors = %v; want [%v]", span.errors, wantErr)
	}
	if !hasAttr(span.attrs, "status", "error") {
		t.Fatal("span attrs missing status=error")
	}
	if !hasAttr(span.attrs, "error.type", "io") {
		t.Fatal("span attrs missing error.type=io")
	}
	if len(rec.histograms) != 1 {
		t.Fatalf("histogram measurements = %d; want 1", len(rec.histograms))
	}
	if len(rec.counters) != 1 {
		t.Fatalf("failure counter measurements = %d; want 1", len(rec.counters))
	}
	if rec.counters[0].name != "ami_gin.operation.failures" {
		t.Fatalf("counter name = %q; want %q", rec.counters[0].name, "ami_gin.operation.failures")
	}
	if rec.counters[0].value != 1 {
		t.Fatalf("counter value = %d; want 1", rec.counters[0].value)
	}
	if !hasAttr(rec.counters[0].attrs, "error.type", "io") {
		t.Fatal("counter attrs missing error.type=io")
	}
}

func TestRunBoundaryOperationFinalizesBeforeRepanic(t *testing.T) {
	signals, rec := newRecordingSignals()
	wantPanic := stderrors.New("boom")

	defer func() {
		recovered := recover()
		if recovered != wantPanic {
			t.Fatalf("recover() = %v; want %v", recovered, wantPanic)
		}
		if len(rec.spans) != 1 {
			t.Fatalf("recorded spans = %d; want 1", len(rec.spans))
		}
		span := rec.spans[0]
		if !span.ended {
			t.Fatal("span was not ended before re-panic")
		}
		if span.status != codes.Error {
			t.Fatalf("span status = %v; want %v", span.status, codes.Error)
		}
		if len(span.errors) != 1 || !stderrors.Is(span.errors[0], wantPanic) {
			t.Fatalf("recorded span errors = %v; want [%v]", span.errors, wantPanic)
		}
		if !hasAttr(span.attrs, "status", "error") {
			t.Fatal("span attrs missing status=error")
		}
		if !hasAttr(span.attrs, "error.type", "config") {
			t.Fatal("span attrs missing error.type=config")
		}
		if len(rec.histograms) != 1 {
			t.Fatalf("histogram measurements = %d; want 1", len(rec.histograms))
		}
		if len(rec.counters) != 1 {
			t.Fatalf("failure counter measurements = %d; want 1", len(rec.counters))
		}
	}()

	_ = RunBoundaryOperation(context.Background(), signals, BoundaryConfig{
		Scope:     "test-scope",
		Operation: "test.panic",
		ClassifyError: func(error) string {
			return "config"
		},
	}, func(context.Context) error {
		panic(wantPanic)
	})
}

func newRecordingSignals() (Signals, *boundaryRecorder) {
	rec := &boundaryRecorder{}
	return NewSignals(
		&recordingTracerProvider{
			TracerProvider: trace.NewNoopTracerProvider(),
			recorder:       rec,
		},
		&recordingMeterProvider{
			MeterProvider: metricnoop.NewMeterProvider(),
			recorder:      rec,
		},
		nil,
	), rec
}

type recordingTracerProvider struct {
	trace.TracerProvider
	recorder *boundaryRecorder
}

func (p *recordingTracerProvider) Tracer(name string, options ...trace.TracerOption) trace.Tracer {
	return &recordingTracer{
		Tracer:   p.TracerProvider.Tracer(name, options...),
		scope:    name,
		provider: p,
		recorder: p.recorder,
	}
}

type recordingTracer struct {
	trace.Tracer
	scope    string
	provider trace.TracerProvider
	recorder *boundaryRecorder
}

func (t *recordingTracer) Start(ctx context.Context, spanName string, _ ...trace.SpanStartOption) (context.Context, trace.Span) {
	span := &recordingSpan{
		Span:     trace.SpanFromContext(ctx),
		recorded: &recordedSpan{name: spanName, scope: t.scope},
		provider: t.provider,
	}
	t.recorder.spans = append(t.recorder.spans, span.recorded)
	return trace.ContextWithSpan(ctx, span), span
}

type recordingSpan struct {
	trace.Span
	recorded *recordedSpan
	provider trace.TracerProvider
}

func (s *recordingSpan) End(_ ...trace.SpanEndOption) {
	s.recorded.ended = true
}

func (s *recordingSpan) IsRecording() bool {
	return true
}

func (s *recordingSpan) RecordError(err error, _ ...trace.EventOption) {
	if err != nil {
		s.recorded.errors = append(s.recorded.errors, err)
	}
}

func (s *recordingSpan) SetStatus(code codes.Code, description string) {
	s.recorded.status = code
	s.recorded.statusDesc = description
}

func (s *recordingSpan) SetAttributes(kv ...attribute.KeyValue) {
	s.recorded.attrs = append(s.recorded.attrs, kv...)
}

func (s *recordingSpan) TracerProvider() trace.TracerProvider {
	return s.provider
}

type recordingMeterProvider struct {
	metric.MeterProvider
	recorder *boundaryRecorder
}

func (p *recordingMeterProvider) Meter(name string, opts ...metric.MeterOption) metric.Meter {
	return &recordingMeter{
		Meter:    p.MeterProvider.Meter(name, opts...),
		scope:    name,
		recorder: p.recorder,
	}
}

type recordingMeter struct {
	metric.Meter
	scope    string
	recorder *boundaryRecorder
}

func (m *recordingMeter) Float64Histogram(name string, _ ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	return &recordingHistogram{
		name:     name,
		scope:    m.scope,
		recorder: m.recorder,
	}, nil
}

func (m *recordingMeter) Int64Counter(name string, _ ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	return &recordingCounter{
		name:     name,
		scope:    m.scope,
		recorder: m.recorder,
	}, nil
}

type recordingHistogram struct {
	metric.Float64Histogram
	name     string
	scope    string
	recorder *boundaryRecorder
}

func (h *recordingHistogram) Record(_ context.Context, incr float64, options ...metric.RecordOption) {
	cfg := metric.NewRecordConfig(options)
	set := cfg.Attributes()
	h.recorder.histograms = append(h.recorder.histograms, histogramMeasurement{
		scope: h.scope,
		name:  h.name,
		value: incr,
		attrs: (&set).ToSlice(),
	})
}

func (h *recordingHistogram) Enabled(context.Context) bool {
	return true
}

type recordingCounter struct {
	metric.Int64Counter
	name     string
	scope    string
	recorder *boundaryRecorder
}

func (c *recordingCounter) Add(_ context.Context, incr int64, options ...metric.AddOption) {
	cfg := metric.NewAddConfig(options)
	set := cfg.Attributes()
	c.recorder.counters = append(c.recorder.counters, counterMeasurement{
		scope: c.scope,
		name:  c.name,
		value: incr,
		attrs: (&set).ToSlice(),
	})
}

func (c *recordingCounter) Enabled(context.Context) bool {
	return true
}

func hasAttr(attrs []attribute.KeyValue, key, want string) bool {
	for _, attr := range attrs {
		if string(attr.Key) == key && attr.Value.AsString() == want {
			return true
		}
	}
	return false
}
