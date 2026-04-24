---
phase: 14
plan: 01
subsystem: observability
tags: [observability, logging, telemetry, config, adapters, go]
dependency_graph:
  requires: []
  provides:
    - logging.Logger interface with typed Attr and frozen vocab
    - logging.NewNoop / logging.Default helpers
    - logging/slogadapter.New(*slog.Logger) logging.Logger
    - logging/stdadapter.New(*log.Logger) logging.Logger
    - telemetry.Signals container with Disabled/NewSignals
    - telemetry.RunBoundaryOperation coarse boundary helper
    - GINConfig.Logger and GINConfig.Signals runtime-only fields
    - WithLogger / WithSignals config options
    - normalizeObservability shared helper (gin.go + serialize.go)
  affects:
    - gin.go (GINConfig, DefaultConfig, WithLogger, WithSignals)
    - serialize.go (readConfig now calls normalizeObservability)
tech_stack:
  added:
    - go.opentelemetry.io/otel v1.43.0 (API only)
    - go.opentelemetry.io/otel/trace v1.43.0 (API only)
    - go.opentelemetry.io/otel/metric v1.43.0 (API only)
    - go.opentelemetry.io/otel/trace/noop (subpackage, no separate dep)
    - go.opentelemetry.io/otel/metric/noop (subpackage, no separate dep)
  patterns:
    - typed Attr surface (Attr{Key, Value}) instead of variadic any
    - explicit enabled flag in Signals for deterministic Enabled() semantics
    - normalizeObservability shared helper called at DefaultConfig and readConfig
key_files:
  created:
    - logging/logger.go
    - logging/attrs.go
    - logging/noop.go
    - logging/doc.go
    - logging/slogadapter/slog.go
    - logging/stdadapter/std.go
    - telemetry/telemetry.go
    - telemetry/boundary.go
    - telemetry/doc.go
    - observability_test.go
  modified:
    - gin.go
    - serialize.go
    - go.mod
    - go.sum
decisions:
  - "Used typed Attr{Key, Value} struct rather than variadic any to enforce frozen vocabulary at the type level and prevent future callers from passing raw dynamic values."
  - "Signals.Enabled() backed by an explicit bool field set only by NewSignals, not by type assertions on provider types. This satisfies D-08 without relying on nil-provider inference."
  - "normalizeObservability called from both DefaultConfig() and readConfig() to ensure all config paths (fresh and decoded) install silent defaults before any boundary code runs."
  - "go.opentelemetry.io/otel/trace/noop and metric/noop accessed as sub-packages of the existing API modules — no separate go get needed."
metrics:
  duration: 6m
  completed: "2026-04-22"
  tasks_completed: 4
  files_changed: 14
---

# Phase 14 Plan 01: Observability Foundation Summary

Repo-owned `logging` and `telemetry` seams established; `GINConfig` wired with silent defaults and no on-wire serialization of runtime observability state.

## What Was Built

### logging package

- `logging/logger.go`: `Level` enum, typed `Logger` interface (`Enabled(Level) bool`, `Log(Level, string, ...Attr)`), and package-level `Debug`/`Info`/`Warn`/`Error` helpers. `Debug` guards on `Enabled` before calling `Log`.
- `logging/attrs.go`: `Attr{Key, Value}` struct; five frozen INFO-level key constants (`operation`, `predicate_op`, `path_mode`, `status`, `error.type`); bounded `PathMode*` string constants (`exact`, `bloom-only`, `adaptive-hybrid`); constructor helpers `AttrOperation`, `AttrPredicateOp`, `AttrPathMode`, `AttrStatus`, `AttrErrorType`; `normalizeErrorType` collapses unknown kinds to `other`.
- `logging/noop.go`: shared zero-state `noopLogger`; `NewNoop()` and `Default(Logger) Logger`.
- `logging/doc.go`: documents context-free contract, adapter split, and INFO-level safe-metadata restrictions.

### logging adapter subpackages

- `logging/slogadapter/slog.go`: `New(*slog.Logger) logging.Logger`; nil collapses to noop; maps `Attr` to `slog.Attr`; uses `LogAttrs` for efficient structured emission.
- `logging/stdadapter/std.go`: `New(*log.Logger) logging.Logger`; nil collapses to noop; emits `[INFO]`/`[WARN]`/`[ERROR]` severity prefixes; drops debug (no native severity routing in stdlib).

### telemetry package

- `telemetry/telemetry.go`: `Signals` struct with `TracerProvider`, `MeterProvider`, unexported `shutdown` hook, and an explicit `enabled` bool (set only by `NewSignals`). `Disabled()` returns the zero value. `Enabled()` is backed by the flag, not type assertions. Noop tracer/meter providers used as fallback.
- `telemetry/boundary.go`: `RunBoundaryOperation` — owns span start/end, duration histogram, and failure counter. `BoundaryConfig` carries scope, operation, extra attrs, and optional error classifier. Panic-safe via recover+re-panic.
- `telemetry/doc.go`: documents fail-open model, local-provider-only dependency rule, and OTLP/SDK out-of-scope statement.

### GINConfig wiring (gin.go + serialize.go)

- `GINConfig` gains `Logger logging.Logger` and `Signals telemetry.Signals` as runtime-only fields (not in `SerializedConfig`).
- `DefaultConfig()` calls `normalizeObservability` which sets `logging.NewNoop()` if Logger is nil and `telemetry.Disabled()` if Signals is not enabled.
- `WithLogger(nil)` returns an error (`"logger cannot be nil"`); `WithSignals` accepts any value.
- `readConfig` in `serialize.go` calls `normalizeObservability` after config reconstruction so decoded indexes are safe without requiring callers to patch the config.
- `configLogger(*GINConfig) logging.Logger` and `configSignals(*GINConfig) telemetry.Signals` nil-safe accessor helpers for future boundary code.

## TDD Gate Compliance

| Gate | Commit |
|------|--------|
| RED (test) | `1e40824` — failing tests for all 4 tasks committed before implementation |
| GREEN (feat) | `7571a35` — implementation making all tests pass |
| REFACTOR | None needed — code is clean as written |

## Deviations from Plan

### Auto-fixed Issues

None — plan executed exactly as written.

### Notes

- The existing `adaptiveInvariantLogger *log.Logger` in `query.go` is pre-existing code outside Plan 14-01's `files_modified` scope. Its migration to the new logging seam is D-13/D-14 work belonging to a later 14-x plan. Documented as deferred, not a deviation.
- `Signals.Enabled()` uses an explicit `bool` flag rather than go-wand's nil-provider type assertion. This was a deliberate local adaptation per the research recommendation (A1 in 14-RESEARCH.md) to give deterministic semantics without relying on provider type assertions.

## Known Stubs

None — the foundation package surfaces compile and behave correctly. No data is stubbed to UI or user-visible output.

## Threat Flags

None — this plan creates no new network endpoints, auth paths, file access patterns, or schema changes at trust boundaries.

## Self-Check

Files created/committed:
- logging/logger.go — FOUND
- logging/attrs.go — FOUND
- logging/noop.go — FOUND
- logging/doc.go — FOUND
- logging/slogadapter/slog.go — FOUND
- logging/stdadapter/std.go — FOUND
- telemetry/telemetry.go — FOUND
- telemetry/boundary.go — FOUND
- telemetry/doc.go — FOUND
- observability_test.go — FOUND
- gin.go (modified) — FOUND
- serialize.go (modified) — FOUND

Commits:
- 1e40824 — FOUND
- 7571a35 — FOUND

## Self-Check: PASSED
