package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

var (
	noopTracerProvider = tracenoop.NewTracerProvider()
	noopMeterProvider  = metricnoop.NewMeterProvider()
)

// Signals carries the local telemetry providers used by coarse boundary APIs.
// The zero value is fully disabled: providers are nil, Enabled() reports false,
// and Shutdown() is a no-op.
type Signals struct {
	TracerProvider trace.TracerProvider
	MeterProvider  metric.MeterProvider
	shutdown       func(context.Context) error
	// enabled records intentional construction via NewSignals. This field
	// separates "caller explicitly disabled telemetry" (Disabled()) from
	// "caller has not set anything yet" (zero value). Both map to the same
	// behavior, but Enabled() is backed by an explicit flag rather than
	// type assertions so callers cannot spoof enabled state with custom providers.
	enabled bool
}

// NewSignals constructs a local signal container with optional tracer and meter
// providers plus an optional shutdown hook. Passing nil providers leaves that
// signal type inactive but Enabled() still reports true because the caller
// explicitly chose to construct signals. Use Disabled() to express "no telemetry".
func NewSignals(
	tracerProvider trace.TracerProvider,
	meterProvider metric.MeterProvider,
	shutdown func(context.Context) error,
) Signals {
	return Signals{
		TracerProvider: tracerProvider,
		MeterProvider:  meterProvider,
		shutdown:       shutdown,
		enabled:        true,
	}
}

// Disabled returns the zero-value, fully disabled signal container. It carries
// nil providers, reports Enabled()==false, and treats Shutdown as a no-op.
func Disabled() Signals {
	return Signals{}
}

// Enabled reports whether this Signals container was intentionally constructed
// via NewSignals. Both the zero value and Disabled() return false.
func (s Signals) Enabled() bool {
	return s.enabled
}

// Shutdown flushes and closes configured providers when a shutdown hook exists.
// Nil contexts are normalized to context.Background(), and zero-value or
// Disabled signal containers return nil without doing any work.
func (s Signals) Shutdown(ctx context.Context) error {
	if s.shutdown == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return s.shutdown(ctx)
}

// Tracer returns a tracer for the supplied scope, falling back to a package
// local no-op provider when tracing is disabled.
func (s Signals) Tracer(scope string) trace.Tracer {
	if s.TracerProvider == nil {
		return noopTracerProvider.Tracer(scope)
	}
	return s.TracerProvider.Tracer(scope)
}

// Meter returns a meter for the supplied scope, falling back to a package
// local no-op provider when metrics are disabled.
func (s Signals) Meter(scope string) metric.Meter {
	if s.MeterProvider == nil {
		return noopMeterProvider.Meter(scope)
	}
	return s.MeterProvider.Meter(scope)
}
