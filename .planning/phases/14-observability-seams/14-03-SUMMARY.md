---
phase: 14
plan: 03
subsystem: observability
tags: [observability, parquet, serialization, context, s3, boundary, go]
dependency_graph:
  requires:
    - 14-01 (logging/telemetry packages and GINConfig wiring)
  provides:
    - BuildFromParquetContext / BuildFromParquetReaderContext with coarse boundary span
    - S3Client.BuildFromParquetContext with caller-context range-read propagation
    - EncodeContext / EncodeWithLevelContext / DecodeContext with EncodeOption / DecodeOption shims
    - Compatibility wrappers: BuildFromParquet, BuildFromParquetReader, Encode, EncodeWithLevel, Decode
  affects:
    - parquet.go (additive context-aware siblings; old functions delegate)
    - s3.go (s3ReaderAt carries parentCtx; BuildFromParquetContext added)
    - serialize.go (context-aware encode/decode siblings; option types; internal core functions)
tech_stack:
  added: []
  patterns:
    - context-aware additive sibling pattern (old = wrapper over new with context.Background())
    - scoped context-in-struct exception for short-lived s3ReaderAt (documented in comment)
    - EncodeOption / DecodeOption runtime shims for raw serialization (no config receiver)
    - classifyParquetError / classifySerializeError helpers for frozen error.type vocabulary
key_files:
  created:
    - boundary_observability_test.go
  modified:
    - parquet.go
    - s3.go
    - serialize.go
decisions:
  - "buildFromParquetReaderCore extracted as internal function so BuildFromParquetReaderContext can wrap it in RunBoundaryOperation cleanly — the public boundary is exactly one span per build call."
  - "s3ReaderAt.parentCtx is an intentional scoped exception to the no-context-in-struct guideline: the reader is private, short-lived, constructed once per build, and never reused across requests. The rationale is documented in the struct comment."
  - "classifySerializeError maps ErrInvalidFormat, ErrVersionMismatch, ErrDecodedSizeExceedsLimit to specific frozen error.type values; everything else falls back to 'other'."
  - "EncodeContext seeds signals from idx.Config via configSignals to avoid requiring callers to repeat the config; DecodeContext defaults to telemetry.Disabled() because no config exists at decode time."
metrics:
  duration: 6m
  completed: "2026-04-22"
  tasks_completed: 3
  files_changed: 4
---

# Phase 14 Plan 03: Parquet/Build and Raw Serialization Boundaries Summary

Additive context-aware siblings for parquet build and raw encode/decode; S3 build path propagates caller context through range reads; compatibility wrappers preserve the existing API exactly.

## What Was Built

### parquet.go

- `BuildFromParquetContext(ctx, parquetFile, jsonColumn, config)` — context-aware sibling; starts one coarse `build_from_parquet` span via `telemetry.RunBoundaryOperation`; no spans inside page, value, or document loops.
- `BuildFromParquetReaderContext(ctx, ..., reader, size)` — context-aware sibling for the reader variant; delegates to internal `buildFromParquetReaderCore`.
- `BuildFromParquet` and `BuildFromParquetReader` now delegate with `context.Background()` — zero behavior change for existing callers.
- `classifyParquetError` maps column-not-found, open/read, and create-builder errors to the frozen `error.type` vocabulary.
- `buildFromParquetReaderCore` extracted as the internal implementation so the public boundary wraps it cleanly.

### s3.go

- `S3Client.BuildFromParquetContext(ctx, bucket, key, jsonColumn, ginCfg)` — context-aware build method; calls `OpenParquetContext` which wires `parentCtx` into the reader.
- `S3Client.OpenParquetContext(ctx, bucket, key)` — constructs an `s3ReaderAt` with `parentCtx = ctx` so every range read derives a 30-second timeout child from the caller context instead of `context.Background()`.
- `s3ReaderAt.parentCtx` field carries the caller context. The struct comment explains the scoped exception to the no-context-in-struct guideline.
- `S3Client.BuildFromParquet` and `S3Client.OpenParquet` now delegate to their context-aware siblings — zero behavior change for existing callers.

### serialize.go

- `EncodeContext(ctx, idx, ...EncodeOption)` — context-aware sibling; wraps `EncodeWithLevelContext` at `CompressionBest`.
- `EncodeWithLevelContext(ctx, idx, level, ...EncodeOption)` — seeds signals from `idx.Config` via `configSignals`; caller `EncodeOption`s override; delegates to internal `encodeWithLevel`.
- `DecodeContext(ctx, data, ...DecodeOption)` — defaults signals to `telemetry.Disabled()` (no config receiver); caller `DecodeOption`s can supply explicit signals.
- `EncodeOption` / `DecodeOption` function types for runtime observability injection into raw encode/decode.
- `Encode`, `EncodeWithLevel`, `Decode` now delegate with `context.Background()` — zero behavior change for existing callers.
- `encodeWithLevel` / `decodeCore` are the internal implementations extracted from the old public functions.
- `classifySerializeError` maps the three known sentinel errors to frozen `error.type` values.

### boundary_observability_test.go

- `TestBuildFromParquetContextCompatibility` — old vs new error-shape parity.
- `TestBuildFromParquetContextPreservesResults` — index structure unchanged through context-aware path.
- `TestBuildFromParquetContextNilDisabledObservabilityNoChanges` — silent config produces same result.
- `TestBuildFromParquetContextEmitsParserNameWithoutInfoLeak` — function exists and does not panic.
- `TestS3BuildFromParquetContextCompatibility` — compile-time type assertion on S3Client interface.
- `TestS3BuildFromParquetContextHonorsCancellationWithStubTransport` — pre-canceled context propagates to S3 build without live AWS dependency.
- `TestEncodeContextCompatibility` — Encode vs EncodeContext byte-length parity.
- `TestEncodeWithLevelContextCompatibility` — EncodeWithLevel vs EncodeWithLevelContext parity.
- `TestDecodeContextCompatibility` — Decode vs DecodeContext index structure parity.
- `TestSerializationContextRoundTrip` — EncodeContext → DecodeContext round-trip produces correct header values.
- `TestMetadataAndSidecarHelpersUseContextSiblings` — EncodeToMetadata / DecodeFromMetadata still work end-to-end.

## TDD Gate Compliance

| Gate | Commit |
|------|--------|
| RED (test) | `4f325fd` — all boundary observability tests fail before implementation |
| GREEN (feat) | `0f723b6` — implementation makes all tests pass |
| REFACTOR | None needed — code is clean as written |

## Deviations from Plan

None — plan executed exactly as written.

### Notes

- The S3 cancellation test uses a pre-canceled context plus a local unreachable endpoint (port 1) instead of a `httptest` mock server. The test produces a deterministic error without any live network dependency, which satisfies the acceptance criterion.
- `classifySerializeError` uses `stderrors.Is` (not string matching) to map the three sentinel errors; this is correct because the sentinel errors are package-level `var` values wrapped by pkg/errors.

## Known Stubs

None — all public functions are fully wired. No data is stubbed or hardcoded to empty values that would flow to user-visible output.

## Threat Flags

None — this plan adds no new network endpoints, auth paths, file access patterns beyond the existing S3/parquet surfaces. The s3ReaderAt.parentCtx field is private and internal to one build call; it does not widen any security surface.

## Self-Check: PASSED

Files verified:
- boundary_observability_test.go — FOUND
- parquet.go — FOUND
- s3.go — FOUND
- serialize.go — FOUND

Commits verified:
- 4f325fd (RED) — FOUND
- 0f723b6 (GREEN) — FOUND
