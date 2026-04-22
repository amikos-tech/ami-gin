// Package telemetry provides the local Signals container and coarse boundary
// helper for github.com/amikos-tech/ami-gin.
//
// # Ownership Model
//
// This package is fail-open and caller-owned. The library never mutates global
// OTel state (no otel.SetTracerProvider, otel.SetMeterProvider, or
// global.SetLoggerProvider calls). Callers construct Signals, pass them into
// GINConfig or raw encode/decode options, and own the provider shutdown
// lifecycle.
//
// # Local Providers Only
//
// This package depends only on the OTel API and noop packages
// (go.opentelemetry.io/otel, go.opentelemetry.io/otel/trace,
// go.opentelemetry.io/otel/metric). OTLP exporter/bootstrap helpers and the
// OTel SDK are explicitly out of scope for the root module. CLI bootstrap
// wiring belongs to a CLI-owned module boundary.
//
// # Disabled Path
//
// Disabled() and the zero-value Signals are silent and zero-cost. Noop tracer
// and meter providers are used for all operations when signals are disabled.
package telemetry
