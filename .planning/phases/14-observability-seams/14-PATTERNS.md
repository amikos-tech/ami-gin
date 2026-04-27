# Phase 14: Observability Seams - Pattern Map

**Mapped:** 2026-04-22
**Files analyzed:** 11 planned touchpoints (6 new, 5 modified)
**Analogs found:** 11 / 11

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `logging/logger.go` (NEW) | repo-owned logging contract | request-response | `../go-wand/pkg/logging/logger.go` | exact shape, narrower payload type |
| `logging/noop.go` (NEW) | silent default logger | request-response | `../go-wand/pkg/logging/noop.go` | exact |
| `logging/slogadapter/slog.go` (NEW) | `slog` adapter | request-response | `../go-wand/pkg/logging/slogadapter/slog.go` | exact |
| `logging/stdadapter/std.go` (NEW) | stdlib `log` adapter | request-response | `../go-wand/pkg/logging/stdadapter/std.go` | exact |
| `telemetry/telemetry.go` (NEW) | local-provider `Signals` container | request-response | `../go-wand/pkg/telemetry/telemetry.go` | exact |
| `telemetry/boundary.go` (NEW) | coarse boundary helper | request-response | `../go-wand/pkg/telemetry/boundary.go` | exact |
| `gin.go` (MODIFIED) | config carrier + defaults | request-response | `gin.go` self-modify + `../go-wand/pkg/logging/noop.go` defaulting | self + reference |
| `query.go` (MODIFIED) | query boundary + legacy logger migration | hot-path request-response | `query.go` self-modify + `../go-wand/pkg/logging/doc.go` logging rules | self + policy analog |
| `serialize.go` (MODIFIED) | raw serialization boundaries | request-response | `serialize.go` self-modify + `../go-wand/pkg/telemetry/boundary.go` | mixed |
| `parquet.go` / `s3.go` (MODIFIED) | build/parquet public boundary migration | request-response + I/O | `parquet.go` / `s3.go` self-modify + `../go-wand/docs/telemetry.md` context ownership | self + policy analog |
| `benchmark_test.go`, `gin_test.go`, `parquet_test.go` (MODIFIED) | perf gates + policy tests | test/benchmark | existing repo benchmark/test patterns | exact |

## Pattern Assignments

### `logging/logger.go`

**Role:** defines the repo-owned logging contract used by the library core and adapters.

**Primary analog:** `../go-wand/pkg/logging/logger.go`

Carry over:

- local `Level` enum
- tiny `Logger` interface
- helper functions that normalize nil/noop callers

Required adaptation for this repo:

- use `Log(Level, string, ...Attr)` rather than `...any` to satisfy `OBS-01`
- keep `Enabled(Level) bool`
- reserve INFO for narrow operational events only, per `14-CONTEXT.md`

**Secondary analog:** existing config-option surfaces in `gin.go`

Why: the logger contract will be threaded through `GINConfig`, so the option naming and validation should match current repo style rather than go-wand's boundary-option style.

### `logging/noop.go`

**Role:** shared silent logger default.

**Primary analog:** `../go-wand/pkg/logging/noop.go`

Preserve:

- one shared zero-state noop logger
- `Default(logger)` helper collapsing nil to noop

Repo-specific rule:

- `DefaultConfig()` in `gin.go` should wire this in directly so the library is silent by default, not merely tolerant of nil logger fields.

### `logging/slogadapter/slog.go`

**Role:** adapts `*slog.Logger` into the repo-owned logger contract.

**Primary analog:** `../go-wand/pkg/logging/slogadapter/slog.go`

Preserve:

- context-free emission through `context.Background()`
- nil input collapses to noop
- thin level mapping

Adapt:

- map local `Attr` values to `slog.Attr` without exposing `slog` from the core API
- avoid variadic key/value normalization because this repo's logger contract is typed

### `logging/stdadapter/std.go`

**Role:** adapts stdlib `*log.Logger` into the repo-owned logger contract.

**Primary analog:** `../go-wand/pkg/logging/stdadapter/std.go`

Preserve:

- nil input collapses to noop
- bracketed severity prefixes for stdlib logs
- drop debug when the backend cannot route it naturally

Phase-14-specific use:

- this adapter becomes the compatibility path for callers who today use `SetAdaptiveInvariantLogger(log.New(...))`

### `telemetry/telemetry.go`

**Role:** local `Signals` container carrying tracer/meter providers and optional shutdown hook.

**Primary analog:** `../go-wand/pkg/telemetry/telemetry.go`

Preserve:

- `Disabled()`
- `Enabled()`
- noop fallback providers for `Tracer(...)` and `Meter(...)`
- caller-owned shutdown lifecycle

Adapt:

- keep the file limited to OTel API + noop packages only
- do not add `FromEnv(...)` here in Phase 14 because of the root-module dependency constraint

### `telemetry/boundary.go`

**Role:** coarse wrapper for one boundary operation.

**Primary analog:** `../go-wand/pkg/telemetry/boundary.go`

Preserve:

- context normalization
- span start/end lifecycle
- generic success/error attr handling
- duration/failure metrics

Adapt:

- use this only at the boundaries named in the roadmap (`Evaluate`, `Encode`, `Decode`, `BuildFromParquet`)
- keep returned attrs compatible with the frozen vocabulary required by this repo

### `gin.go`

**Role:** observability carrier, config options, and defaults.

**Primary analog:** `gin.go` itself

Relevant existing patterns:

- `GINConfig` is the existing additive carrier (`gin.go:350`)
- config options return `errors.New` / `errors.Wrap` style validation
- `DefaultConfig()` is where silent defaults belong (`gin.go:648`)

What to preserve:

- additive `ConfigOption` surface
- no breaking signature changes
- nil-safe rebuild logic for `GINIndex.Config`

### `query.go`

**Role:** largest behavioral migration in the phase.

**Primary analog:** `query.go` itself

Relevant existing anchors:

- package-global legacy logger at `query.go:13-33`
- public boundary at `query.go:36`
- failure-open invariant fallback in `adaptiveInvariantAllRGs(...)`

Reference policy analogs:

- `../go-wand/pkg/logging/doc.go`
- `../go-wand/docs/logging.md`
- `../go-wand/docs/telemetry.md`

Required pattern:

- replace globals with config-carried logger/signals
- add `EvaluateContext` as additive sibling
- keep `Evaluate` as compatibility wrapper
- instrument once around the overall query evaluation, never inside the row-group loop

### `serialize.go`

**Role:** raw serialization is the one boundary family without a config carrier.

**Primary analog:** `serialize.go` itself

Reference analog:

- `../go-wand/pkg/telemetry/boundary.go`

Required pattern:

- add `EncodeContext` / `DecodeContext`
- keep existing top-level functions as wrappers
- emit only coarse attributes such as operation/status/error type and maybe encoded size/status when low-cardinality

### `parquet.go` and `s3.go`

**Role:** public build path and context composition.

**Primary analogs:** `parquet.go` and `s3.go` themselves

Key in-tree anchors:

- `BuildFromParquet(...)` / `BuildFromParquetReader(...)` in `parquet.go:108-112`
- S3 build wrapper in `s3.go:207`
- existing internal timeout use in `s3.go`

Reference policy analog:

- `../go-wand/docs/telemetry.md` "CLI Ownership" and "Library APIs"

Required pattern:

- add context-aware build siblings without breaking the old public API
- compose caller context with existing timeout wrappers instead of resetting to `context.Background()`

### `benchmark_test.go`, `gin_test.go`, `parquet_test.go`

**Role:** enforce the phase's merge gates.

**Primary analogs:** existing benchmark and table-driven test style in this repo

Useful in-tree anchors:

- `BenchmarkScaleRowGroups` and `BenchmarkE2EBuildQuerySerialize` in `benchmark_test.go`
- legacy invariant logger tests around `gin_test.go:1065+`
- parquet boundary tests in `parquet_test.go`

Required additions:

- disabled logging/tracing perf gate
- default-silence tests
- allowlist policy tests
- compatibility-wrapper tests
- no-legacy-logger regression tests

## Implementation Notes That Should Shape Planning

- `GINIndex.Config` already survives `Finalize()` and `Decode()`, so query observability does not need a new carrier.
- `Encode` / `Decode` have no carrier today, so their context siblings should not be forced into the same plan as config-carried surfaces.
- The current repo has no nested `go.mod`, so any OTLP exporter bootstrap in Phase 14 would create new packaging complexity. Plans should not depend on it.
- The old `SetAdaptiveInvariantLogger` tests are the best template for proving migration correctness, but they must be rewritten around the new logger contract and silent defaults.
