package telemetry

import (
	"context"
	"time"

	pkgerrors "github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
)

// BoundaryConfig describes one coarse boundary operation. The shared helper
// owns lifecycle and generic failure handling only; package-owned success
// counters and classifier callbacks remain outside it.
type BoundaryConfig struct {
	Scope         string
	Operation     string
	ExtraAttrs    []attribute.KeyValue
	ClassifyError func(error) string
}

// RunBoundaryOperation runs fn inside one coarse telemetry boundary.
// The helper owns span start/end, duration recording, and failure counting.
// fn receives the normalized context (never nil). A nil fn is a no-op call.
// If fn panics, RunBoundaryOperation finalizes telemetry then re-panics.
func RunBoundaryOperation(
	ctx context.Context,
	signals Signals,
	cfg BoundaryConfig,
	fn func(context.Context) error,
) error {
	if ctx == nil {
		ctx = context.Background()
	}

	ctx, span := signals.Tracer(cfg.Scope).Start(ctx, cfg.Operation)
	started := time.Now()

	var err error
	var recovered any

	defer func() {
		if r := recover(); r != nil {
			recovered = r
			err = errorFromPanic(r)
		}

		errType := ""
		if err != nil {
			errType = classifyError(cfg, err)
		}

		attrs := buildAttrs(cfg, err, errType)

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, errType)
		} else {
			span.SetStatus(codes.Ok, "")
		}
		span.SetAttributes(attrs...)
		span.End()

		recordDuration(ctx, signals, cfg.Scope, started, attrs...)
		if err != nil {
			addFailureCount(ctx, signals, cfg.Scope, 1, attrs...)
		}

		if recovered != nil {
			panic(recovered)
		}
	}()

	if fn != nil {
		err = fn(ctx)
	}

	return err
}

func buildAttrs(cfg BoundaryConfig, err error, errType string) []attribute.KeyValue {
	attrs := []attribute.KeyValue{attribute.String("operation", cfg.Operation)}
	attrs = append(attrs, cfg.ExtraAttrs...)

	status := "ok"
	if err != nil {
		status = "error"
		attrs = append(attrs, attribute.String("error.type", errType))
	}
	attrs = append(attrs, attribute.String("status", status))
	return attrs
}

func classifyError(cfg BoundaryConfig, err error) string {
	if cfg.ClassifyError == nil {
		return ErrorTypeOther
	}
	return normalizeErrorType(cfg.ClassifyError(err))
}

func normalizeErrorType(kind string) string {
	switch kind {
	case "config", "io", "invalid_format", "deserialization", "integrity", "not_found", "other":
		return kind
	default:
		return ErrorTypeOther
	}
}

func errorFromPanic(recovered any) error {
	if err, ok := recovered.(error); ok {
		return err
	}
	return pkgerrors.Errorf("panic: %v", recovered)
}

func recordDuration(ctx context.Context, signals Signals, scope string, started time.Time, attrs ...attribute.KeyValue) {
	histogram, err := signals.Meter(scope).Float64Histogram(
		"ami_gin.operation.duration",
		metric.WithUnit("ms"),
	)
	if err != nil {
		return
	}
	histogram.Record(ctx, float64(time.Since(started))/float64(time.Millisecond), metric.WithAttributes(attrs...))
}

func addFailureCount(ctx context.Context, signals Signals, scope string, value int64, attrs ...attribute.KeyValue) {
	if value <= 0 {
		return
	}
	counter, err := signals.Meter(scope).Int64Counter("ami_gin.operation.failures", metric.WithUnit("1"))
	if err != nil {
		return
	}
	counter.Add(ctx, value, metric.WithAttributes(attrs...))
}
