# Phase 14: Observability Seams - Research

**Researched:** 2026-04-22  
**Domain:** Go observability seams for builder, query, serialization, parquet, and S3 boundaries [VERIFIED: local planning context]  
**Confidence:** MEDIUM

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions [VERIFIED: local planning context]

### Reference Model And Uniformity

- **D-01:** Phase 14 must align with `/Users/tazarov/experiments/amikos/go-wand` as the canonical observability reference model. Adopt its split seam near-verbatim: a repo-owned, context-free `Logger`; a separate `Signals` container for telemetry; boundary-only instrumentation via a shared helper; CLI-owned `FromEnv(...)`; no trace IDs injected into logs; no global OTel mutation.
- **D-02:** The official adapters in this phase are `logging/slogadapter` and `logging/stdadapter` only. Do not expose `*slog.Logger`, `*log.Logger`, or any OTel SDK/exporter type from the core public API. `zap` remains explicitly out of scope for v1.1.
- **D-03:** Freeze the low-cardinality observability vocabulary in one source file. INFO-level attributes stay on the roadmap allowlist only: `operation`, `predicate_op`, `path_mode`, `status`, `error.type`. Predicate values, path names, doc IDs, row-group IDs, term IDs, and raw user content stay banned at INFO level.
- **D-04:** `Parser.Name()` from Phase 13 may be used only in traces or guarded debug-level signals. It must not widen the frozen INFO-level attribute vocabulary.

### Dependency Boundary And Packaging

- **D-05:** Keep the go-wand telemetry API shape and behavior, but adapt exporter packaging to the stricter `ami-gin` milestone rule: the root `github.com/amikos-tech/ami-gin` module may depend on OTel API/noop packages for `Signals`, but OTLP SDK/exporter bootstrap must live outside the core module boundary so the root `go.mod` does not pick up SDK/exporter deps.
- **D-06:** `FromEnv(ctx, serviceName)` remains a CLI-owned convenience surface, modeled on go-wand. The library itself must not auto-bootstrap telemetry, mutate global providers, or hide exporter lifecycle inside package init or defaults.

### Carrier Shape Across Public APIs

- **D-07:** For build, query, and parquet paths, carry the new logger and telemetry seams through `GINConfig`, not through a separate repo-wide observability object and not through a large go-wand-style explosion of per-boundary options. This fits the existing `ami-gin` API shape better.
- **D-08:** `DefaultConfig()` wires silent defaults: a noop logger plus disabled/noop signals. Library behavior remains quiet unless the caller opts in.
- **D-09:** Query-time observability reads from `GINIndex.Config`, which already survives `Finalize()` and `Decode()`. If `idx.Config` is nil, query paths must fall back to silent/noop behavior rather than panic or assume configuration is always present.
- **D-10:** `EvaluateContext` and `BuildFromParquetContext` are additive siblings. Existing `Evaluate` and `BuildFromParquet` remain compatibility wrappers over `context.Background()` and the config-carried observability seams.

### Serialization Observability

- **D-11:** Raw serialization is the exception to the config-carrier rule. Add additive `EncodeContext` and `DecodeContext` siblings so raw encode/decode can emit coarse observability signals without globals.
- **D-12:** Existing `Encode` and `Decode` stay as compatibility wrappers over the new context-aware siblings. Planner/implementation may choose the exact additive signature, but raw serialization must not reintroduce package-global hooks or dual logging conventions.

### Migration And Compatibility

- **D-13:** Remove `SetAdaptiveInvariantLogger(*log.Logger)` in Phase 14 instead of keeping a deprecated compatibility bridge. This repo moves directly to one logging convention; no dual logger state survives the phase.
- **D-14:** The existing adaptive invariant fallback behavior stays the same: invariant violations still fail open to `AllRGs()`. Only the emission path changes, from package-global stdlib logging to the repo-owned logger seam.

### Instrumentation Scope And Performance

- **D-15:** Instrumentation stays boundary-only. Coarse operation boundaries include `Evaluate`, `Encode`, `Decode`, `BuildFromParquet`, and the related parquet/open/build surfaces. Per-predicate details, when emitted, are parent-span events rather than nested spans. Never add per-row-group spans or telemetry in the hot query loop.
- **D-16:** Zero-cost disabled behavior is a merge gate. Disabled logging must stay at 0 allocs/op on the benchmark gate, and disabled tracer wiring must remain within the roadmap's no-regression budget. Expensive debug payload assembly must be guarded before attr construction.

### the agent's Discretion [VERIFIED: local planning context]

- Exact package and file layout for the new seams, as long as the go-wand split model and the `ami-gin` dependency boundary both hold.
- Exact naming of the new config options and additive context-aware helpers.
- Exact signature design for `EncodeContext` / `DecodeContext`, since raw decode has no pre-existing config carrier.
- Exact boundary helper names, benchmark helper names, and message wording for invariant-violation logs, as long as the frozen vocabulary and no-dual-logger rule are preserved.

### Deferred Ideas (OUT OF SCOPE) [VERIFIED: local planning context]

- `zap` adapter — explicitly on-demand only, not Phase 14 scope
- Per-predicate nested spans or per-row-group telemetry — explicit non-goal
- Trace/log correlation inside the repo-owned logger seam — caller-owned only
- Broader CLI surface such as `--log-level` flags and root bootstrap wiring — Phase 15 concern, though the go-wand pattern is now the reference
- Broader public S3 context-aware API expansion beyond what Phase 14 needs to satisfy `BuildFromParquetContext`
</user_constraints>

<phase_requirements>
## Phase Requirements

Descriptions are copied from `.planning/REQUIREMENTS.md`. [VERIFIED: local planning context]

| ID | Description | Research Support |
|----|-------------|------------------|
| OBS-01 | Library exposes a minimal local `Logger` interface (`Enabled(Level) bool`, `Log(Level, msg, ...Attr)`); noop default so the library stays silent by default. | Use a repo-owned `logging` package with `logger.go`, `noop.go`, `doc.go`, plus `slogadapter` and `stdadapter`; keep the logger context-free and carry it through `GINConfig`. [VERIFIED: go-wand local source][CITED: https://opentelemetry.io/docs/concepts/instrumentation/libraries/] |
| OBS-02 | Observability is zero-cost when disabled — `BenchmarkEvaluateDisabledLogging` asserts 0 allocs/op and `BenchmarkEvaluateWithTracer` stays within 0.5% of the no-tracer baseline. | Guard every expensive payload behind `Enabled(LevelDebug)` or span recording checks, add dedicated benchmark gates in `benchmark_test.go`, and avoid inner-loop spans. [VERIFIED: go doc log/slog][VERIFIED: local code grep][CITED: https://opentelemetry.io/docs/concepts/instrumentation/libraries/] |
| OBS-03 | `slog` and `stdlib log` adapters shipped as separate sub-packages; public API never exposes `*slog.Logger` directly. | Put adapters in `logging/slogadapter` and `logging/stdadapter`; keep the root logger contract backend-neutral and do not expose `*slog.Logger`, `*log.Logger`, or OTel SDK types from root packages. [VERIFIED: go-wand local source][VERIFIED: local planning context] |
| OBS-04 | Frozen structured-attribute vocabulary (`operation`, `predicate_op`, `path_mode`, `status`, `error.type`); PII allowlist bans predicate values, path field names, doc/RG/term IDs from INFO level. | Freeze keys and operation names in one `telemetry/attrs.go` file and add allowlist tests that reject extra INFO attrs. [VERIFIED: local planning context][VERIFIED: go-wand local source] |
| OBS-05 | `Telemetry`/`Signals` container carries OTel `TracerProvider` and `MeterProvider`; the library never mutates global OTel state (no `otel.SetTracerProvider`). | Use a root-module `telemetry` package that depends only on OTel API + noop packages; keep OTLP SDK/exporter bootstrap outside the root module. [CITED: https://opentelemetry.io/docs/languages/go/instrumentation/][CITED: https://pkg.go.dev/go.opentelemetry.io/otel/trace/noop][CITED: https://pkg.go.dev/go.opentelemetry.io/otel/metric/noop][VERIFIED: local planning context] |
| OBS-06 | Boundary-only instrumentation — coarse spans on `Evaluate`, `Encode`, `Decode`, `BuildFromParquet`; per-predicate decisions emit span events on the parent span, not nested spans. | Wrap only top-level public boundaries and emit per-predicate detail as parent-span events; keep `s3ReaderAt.ReadAt`, row-group loops, and predicate loops span-free. [CITED: https://opentelemetry.io/docs/concepts/instrumentation/libraries/][VERIFIED: local code grep] |
| OBS-07 | Context-aware API variants `EvaluateContext` and `BuildFromParquetContext` added as additive siblings; existing methods wrap with `context.Background()` — no breaking change. | Add sibling wrappers for query, parquet build, and raw serialization; keep old entry points delegating to `context.Background()`. [VERIFIED: local code grep][VERIFIED: local planning context] |
| OBS-08 | Existing `adaptiveInvariantLogger *log.Logger` at `query.go:17` migrated to the new `Logger` interface in the same phase (single convention, no dual logger state). | Delete the package-global logger and setter, resolve the logger from `idx.Config`, and keep the fail-open `AllRGs()` fallback behavior unchanged. [VERIFIED: local code grep][VERIFIED: local planning context] |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- Preserve pruning-first scope: observability must not turn the library into a service framework or query engine with richer semantics. [VERIFIED: local planning context]
- Protect correctness: no observability change may introduce false negatives in row-group selection. [VERIFIED: local planning context]
- Keep the API additive where practical: prefer siblings and options over signature breaks. [VERIFIED: local planning context]
- Back hot-path claims with benchmarks or fixture-based measurements; this phase's disabled-path and no-tracer guarantees must be benchmark-gated. [VERIFIED: local planning context]
- Keep runtime-only observability fields out of the serialized wire format unless a format bump is explicitly justified and tested. [VERIFIED: local planning context][VERIFIED: local code grep]
- Follow the repo's Go conventions for new option surfaces: functional options returning `error`, `pkg/errors` for error creation/wrapping, and validation in constructors or config helpers. [VERIFIED: local planning context]
- If the phase introduces new dedicated config structs outside `GINConfig`, the preferred project pattern is `validator/v10` plus `creasty/defaults`; avoid inventing a second config style. [VERIFIED: local planning context]
- Keep `make test`, `make lint`, `make security-scan`, and `go test ./...` usable after any package or module layout changes. [VERIFIED: local planning context][VERIFIED: local environment audit]

## Summary

Current code already gives Phase 14 the right carrier for build, query, and parquet observability: `GINBuilder` stores `GINConfig` by value, `Finalize()` persists `&b.config` into `GINIndex.Config`, and `Decode()` restores `idx.Config` from `readConfig`, so build/query/parquet seams can reuse the existing config path without inventing a second repo-wide carrier. Raw serialization is the exception because `Encode`, `EncodeWithLevel`, and `Decode` are package-level functions with no config receiver today. `writeConfig` and `readConfig` explicitly whitelist serialized fields, so runtime-only logger/signals fields can stay off-wire with no format bump if they are omitted from `SerializedConfig`. [VERIFIED: local code grep]

The critical planning constraint is packaging, not API shape. The root repo is still a single Go module (`go.mod`, no nested workspace). A temp module experiment verified that a root module importing a replaced nested helper module still picks up the helper module's transitive dependencies in the root `go.mod` and `go.sum`. That means a compileable OTLP `FromEnv` helper cannot live in any package imported by the current root module if D-05 is to remain true. To keep SDK/exporter dependencies out of the root module, either Phase 14 must stop at the root-module library seams and leave actual CLI bootstrap wiring for a later CLI module split, or the phase must first carve `cmd/gin-index` into its own module boundary. [VERIFIED: local code grep][VERIFIED: temp go module experiment][VERIFIED: local planning context]

The zero-cost and frozen-vocabulary requirements are feasible with this codebase, but only if the plan stays disciplined: keep spans on public boundaries only, keep per-predicate detail as parent-span events, centralize all info-level keys in one file, make nil-config query paths collapse to noop, and add dedicated benchmark gates instead of relying on generic `go test -bench ./...` output. The existing repo already has a large `benchmark_test.go`, explicit compatibility-wrapper patterns, and a passing `go test ./...` baseline, so the missing work is observability-specific verification rather than new infrastructure. [VERIFIED: local code grep][VERIFIED: local test run][VERIFIED: go-wand local source][CITED: https://opentelemetry.io/docs/concepts/instrumentation/libraries/]

**Primary recommendation:** land the root-module library seams first (`logging`, `telemetry`, `WithLogger`, `WithSignals`, `EvaluateContext`, `BuildFromParquetContext`, `EncodeContext`, `DecodeContext`) and treat compileable OTLP `FromEnv` bootstrap as blocked on a real CLI module boundary rather than a nested helper import. [VERIFIED: temp go module experiment][VERIFIED: local planning context]

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|--------------|----------------|-----------|
| Repo-owned logger contract and adapters | Core library | — | The logger contract is part of the library boundary and must stay backend-neutral; adapters are opt-in leaves, not ownership points for library behavior. [VERIFIED: go-wand local source][VERIFIED: local planning context] |
| Signals container and boundary helper | Core library | — | The library owns span/event emission and noop fallbacks, but it must stop at API/noop packages and never own SDK/exporter lifecycle. [CITED: https://opentelemetry.io/docs/languages/go/instrumentation/][CITED: https://pkg.go.dev/go.opentelemetry.io/otel/trace/noop][CITED: https://pkg.go.dev/go.opentelemetry.io/otel/metric/noop] |
| Build/query/serialize instrumentation | Core library | Boundary I/O | Public builder/query/serialize entry points are the right coarse boundaries; inner loops and helper functions should remain dark. [VERIFIED: local code grep][CITED: https://opentelemetry.io/docs/concepts/instrumentation/libraries/] |
| Parquet and S3 orchestration observability | Boundary I/O | Core library | File/object read/write/build/load surfaces own the I/O boundary telemetry; page iteration and range-read helpers are inner mechanics and should not emit their own spans. [VERIFIED: local code grep][VERIFIED: local planning context] |
| OTLP bootstrap and provider shutdown | CLI bootstrap | — | SDK/exporter setup and shutdown belong to the process root, not the library, and cannot be imported into the current root module without violating D-05. [VERIFIED: temp go module experiment][VERIFIED: local planning context] |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `logging` (new root subpackage) | `repo-local` [VERIFIED: local planning context] | Backend-neutral, context-free logger seam with noop default and adapter leaves. | This matches the go-wand split model while keeping `*slog.Logger` and `*log.Logger` out of core APIs. [VERIFIED: go-wand local source][VERIFIED: local planning context] |
| `telemetry` (new root subpackage) | `repo-local` [VERIFIED: local planning context] | `Signals`, noop tracer/meter fallback, boundary helper, and frozen vocabulary. | This keeps the library on OTel API/noop packages only and preserves no-global-provider behavior. [CITED: https://opentelemetry.io/docs/languages/go/instrumentation/][VERIFIED: go-wand local source] |
| `go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/trace`, `go.opentelemetry.io/otel/metric` | `v1.43.0` published `2026-04-03` [VERIFIED: go list -m] | API packages for tracers, meters, and instrumentation scope names. | OTel's Go guidance says libraries should use the API packages and let applications own SDK setup. [CITED: https://opentelemetry.io/docs/languages/go/instrumentation/] |
| `go.opentelemetry.io/otel/trace/noop` and `go.opentelemetry.io/otel/metric/noop` | `v1.43.0` current package docs [VERIFIED: go list -m][CITED: https://pkg.go.dev/go.opentelemetry.io/otel/trace/noop][CITED: https://pkg.go.dev/go.opentelemetry.io/otel/metric/noop] | Zero-work tracer/meter providers for disabled paths. | Package docs confirm the noop providers do not record telemetry, which is the right disabled-path default for this phase. [CITED: https://pkg.go.dev/go.opentelemetry.io/otel/trace/noop][CITED: https://pkg.go.dev/go.opentelemetry.io/otel/metric/noop] |
| `log/slog` (stdlib) | Go `1.25.5` floor in `go.mod`; local toolchain `go1.26.2` [VERIFIED: local code grep][VERIFIED: local environment audit] | Official structured-logging backend for the first adapter. | `Logger.LogAttrs` is the efficient structured path and `Handler.Enabled` is called before arguments are processed, which is the right adapter target for disabled-path guards. [VERIFIED: go doc log/slog] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `go.opentelemetry.io/otel/sdk` | `v1.43.0` published `2026-04-03` [VERIFIED: go list -m] | SDK tracer/meter providers. | Use only in a non-root CLI/bootstrap module; never import it from any package that the root module builds. [VERIFIED: temp go module experiment][VERIFIED: local planning context] |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` | `v1.43.0` published `2026-04-03` [VERIFIED: go list -m] | OTLP/HTTP trace exporter for `FromEnv`. | Use only in the CLI bootstrap module if the CLI gets its own module boundary. [VERIFIED: temp go module experiment][VERIFIED: local planning context] |
| `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp` | `v1.43.0` published `2026-04-03` [VERIFIED: go list -m] | OTLP/HTTP metric exporter for `FromEnv`. | Use only in the CLI bootstrap module if the CLI gets its own module boundary. [VERIFIED: temp go module experiment][VERIFIED: local planning context] |
| `github.com/pkg/errors` | `v0.9.1` already required in root `go.mod` [VERIFIED: local code grep] | Error creation and wrapping. | Reuse the repo's existing error style at every new observability boundary. [VERIFIED: local planning context] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Root-module library seam only | Root-module `telemetry/env.go` importing SDK/exporters | Rejected: this directly violates D-05 because the root module would carry SDK/exporter deps. [VERIFIED: local planning context] |
| Root-module library seam only | Nested helper module imported from the root via `replace` | Rejected: a temp experiment showed the root `go.mod` still acquired the helper's transitive deps. [VERIFIED: temp go module experiment] |
| `idx.Config` as the build/query/parquet carrier | A separate repo-wide observability object | Rejected: `GINIndex.Config` already survives `Finalize()` and `Decode()`, and D-07 explicitly prefers the existing config carrier. [VERIFIED: local code grep][VERIFIED: local planning context] |
| Local raw-serialization options | Package-global encode/decode hooks | Rejected: D-11 and D-12 forbid reintroducing global state for raw serialization. [VERIFIED: local planning context] |
| Typed local `Attr` values | Literal go-wand `...any` key/value contract | Possible, but a typed `Attr` surface gives the planner a cleaner way to freeze vocabulary and police alloc regressions. [ASSUMED] |

**Installation:**
```bash
# Root module only.
go get go.opentelemetry.io/otel@v1.43.0
go get go.opentelemetry.io/otel/trace@v1.43.0
go get go.opentelemetry.io/otel/metric@v1.43.0

# Separate CLI/bootstrap module only; do NOT add these to the root module.
go get go.opentelemetry.io/otel/sdk@v1.43.0
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@v1.43.0
go get go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp@v1.43.0
```

**Version verification:** `go list -m -json` confirmed `go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/sdk`, `otlptracehttp`, and `otlpmetrichttp` are all at `v1.43.0`, published `2026-04-03T08:30:03Z`. [VERIFIED: go list -m]

## Architecture Patterns

### System Architecture Diagram

```text
Caller / future CLI root
    |
    | 1. construct logger + Signals (caller-owned)
    v
GINConfig -------------------------------------------------------------+
  - logger                                                             |
  - signals                                                            |
    |                                                                  |
    | 2a. build/query/parquet calls reuse config carrier               |
    v                                                                  |
EvaluateContext / BuildFromParquetContext / Builder.Finalize ----------+
    |
    | 3. boundary helper starts coarse span, records safe attrs
    |    operation / predicate_op / path_mode / status / error.type
    v
Core logic (query loop, builder merge/finalize, parquet orchestration)
    |
    | 4. per-predicate detail becomes parent-span events only
    |    invariant violations log via repo-owned Logger
    v
Result / RGSet / *GINIndex / error

Raw serialization path
Caller
    |
    | 1b. explicit local runtime opts because Encode/Decode have no config receiver
    v
EncodeContext / DecodeContext / EncodeWithLevelContext
    |
    | 2. same boundary helper, same frozen attrs
    v
serialize.go read/write + zstd + config/representation sections

Separate CLI bootstrap module (only if introduced)
    |
    | parse OTEL env + build SDK providers + own shutdown
    v
Signals
    |
    +--> passed into GINConfig or raw Encode/Decode options
```

The diagram above follows the current code shape: config-carried observability for build/query/parquet, explicit local runtime options for raw encode/decode, and caller-owned provider lifecycle. [VERIFIED: local code grep][VERIFIED: go-wand local source][VERIFIED: temp go module experiment]

### Recommended Project Structure

```text
logging/
├── logger.go            # Level, Logger contract, Default/Debug helpers
├── noop.go              # Shared noop logger
├── attr.go              # Local Attr type + constructors (recommended)
├── doc.go               # Contract and safe-metadata rules
├── slogadapter/
│   └── slog.go          # *slog.Logger adapter
└── stdadapter/
    └── std.go           # *log.Logger adapter

telemetry/
├── telemetry.go         # Signals, Disabled(), Tracer(), Meter()
├── boundary.go          # Shared coarse-boundary helper
├── attrs.go             # Frozen operation names + attr helpers
└── doc.go               # Ownership and non-goals

observability_runtime.go # root-package helpers: configLogger/configSignals
query.go                 # EvaluateContext + invariant logger migration
serialize.go             # EncodeContext/DecodeContext + local raw-runtime opts
parquet.go               # BuildFromParquetContext + reader-based internal core
s3.go                    # boundary wrappers only; no range-read spans
builder.go               # Finalize boundary; AddDocument error/warn hooks only

cmd/gin-index/           # keep current CLI untouched unless the phase explicitly
                         # accepts a CLI module split
```

If a compileable `FromEnv` helper must ship in this phase, the clean layout is a real CLI submodule boundary, for example `cmd/gin-index/go.mod` plus `cmd/gin-index/internal/otelbootstrap/env.go`. That split changes testing and Makefile coverage because `go test ./...` from the repo root does not traverse nested modules. [VERIFIED: local code grep][VERIFIED: temp go module experiment]

### Pattern 1: Config-Carried Observability Runtime

**What:** Add runtime-only logger/signals fields to `GINConfig`, expose `WithLogger(...)` and `WithSignals(...)` options, and resolve noop defaults through small root-package helpers instead of serializing anything new. [VERIFIED: local code grep][VERIFIED: local planning context]

**When to use:** Builder, query, and parquet entry points that already receive `GINConfig` or consume `GINIndex.Config`. [VERIFIED: local code grep]

**Example:**
```go
// Source: adapted from local gin.go + serialize.go patterns.
type GINConfig struct {
	CardinalityThreshold uint32
	// ...existing serialized fields...

	logger  logging.Logger     // runtime-only
	signals telemetry.Signals  // runtime-only
}

func WithLogger(l logging.Logger) ConfigOption {
	return func(c *GINConfig) error {
		c.logger = logging.Default(l)
		return nil
	}
}

func WithSignals(s telemetry.Signals) ConfigOption {
	return func(c *GINConfig) error {
		c.signals = s
		return nil
	}
}

func configLogger(cfg *GINConfig) logging.Logger {
	if cfg == nil {
		return logging.NewNoop()
	}
	return logging.Default(cfg.logger)
}

func configSignals(cfg *GINConfig) telemetry.Signals {
	if cfg == nil {
		return telemetry.Disabled()
	}
	if !cfg.signals.Enabled() {
		return telemetry.Disabled()
	}
	return cfg.signals
}
```

### Pattern 2: Raw Serialization Context Siblings With Local Options

**What:** Keep config-carried observability out of raw encode/decode and give serialization a tiny local runtime option surface instead. This is the narrow exception to D-07. [VERIFIED: local planning context]

**When to use:** `Encode`, `EncodeWithLevel`, and `Decode`, because they are package-level functions and may operate on hand-constructed `*GINIndex` values with nil config. [VERIFIED: local code grep]

**Example:**
```go
// Source: adapted from local serialize.go compatibility wrappers.
type EncodeOption func(*encodeRuntime)
type DecodeOption func(*decodeRuntime)

type encodeRuntime struct {
	logger  logging.Logger
	signals telemetry.Signals
}

func Encode(idx *GINIndex) ([]byte, error) {
	return EncodeContext(context.Background(), idx)
}

func EncodeContext(ctx context.Context, idx *GINIndex, opts ...EncodeOption) ([]byte, error) {
	return EncodeWithLevelContext(ctx, idx, CompressionBest, opts...)
}

func EncodeWithLevel(idx *GINIndex, level CompressionLevel) ([]byte, error) {
	return EncodeWithLevelContext(context.Background(), idx, level)
}
```

### Pattern 3: Public Boundary Span + Parent Events Only

**What:** Start one coarse span per public operation and attach per-predicate detail as events on that parent span. Log invariant violations through the repo-owned logger, but do not convert fail-open query behavior into returned errors. [CITED: https://opentelemetry.io/docs/concepts/instrumentation/libraries/][VERIFIED: local planning context][VERIFIED: local code grep]

**When to use:** `EvaluateContext`, `BuildFromParquetContext`, `EncodeContext`, `DecodeContext`, `WriteSidecar`, `ReadSidecar`, `LoadIndex`, and `Finalize()`. [VERIFIED: local code grep][VERIFIED: local planning context]

**Example:**
```go
// Source: adapted from go-wand pkg/telemetry/boundary.go + local query.go.
func (idx *GINIndex) EvaluateContext(ctx context.Context, predicates []Predicate) *RGSet {
	logger := configLogger(idx.Config)
	signals := configSignals(idx.Config)

	ctx, span := signals.Tracer("github.com/amikos-tech/ami-gin/query").Start(ctx, telemetry.OperationEvaluate)
	defer span.End()

	result := AllRGs(int(idx.Header.NumRowGroups))
	for _, p := range predicates {
		// Event attrs stay low-cardinality; values never include raw predicate values.
		span.AddEvent(telemetry.EventPredicateEvaluated,
			trace.WithAttributes(
				telemetry.OTelAttrPredicateOp(p.Operator.String()),
			),
		)
		result = result.Intersect(idx.evaluatePredicate(p))
		if result.IsEmpty() {
			break
		}
	}

	logging.Info(logger, "evaluate completed", telemetry.LogAttrOperation(telemetry.OperationEvaluate), telemetry.LogAttrStatus("ok"))
	return result
}
```

### Anti-Patterns to Avoid

- **Importing OTel SDK/exporter packages anywhere in the root module:** this breaks D-05 immediately and is not rescued by a replaced nested helper module. [VERIFIED: temp go module experiment][VERIFIED: local planning context]
- **Keeping `SetAdaptiveInvariantLogger` as a deprecated bridge:** Phase 14 explicitly forbids dual logger state. [VERIFIED: local planning context][VERIFIED: local code grep]
- **Instrumenting `s3ReaderAt.ReadAt`, per-page loops, per-predicate loops, or row-group loops with child spans:** OTel guidance recommends logs or span events for verbose detail, not span explosion. [CITED: https://opentelemetry.io/docs/concepts/instrumentation/libraries/][VERIFIED: local code grep]
- **Serializing runtime-only logger/signals fields through `SerializedConfig`:** the current wire format does not need a version bump for runtime-only observability. [VERIFIED: local code grep]
- **Scattering info-level attr strings across files:** D-03 requires one frozen vocabulary file, and a spread-out implementation will make the allowlist test brittle. [VERIFIED: local planning context]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Disabled tracer/meter fallback | Custom noop providers | `trace/noop` and `metric/noop` from OTel API packages | The package docs already guarantee providers that do not record telemetry, which is safer than a custom fake. [CITED: https://pkg.go.dev/go.opentelemetry.io/otel/trace/noop][CITED: https://pkg.go.dev/go.opentelemetry.io/otel/metric/noop] |
| Backend-specific public logging API | `*slog.Logger` or `*log.Logger` in `GINConfig` | Repo-owned `logging.Logger` + adapter subpackages | This keeps the public API backend-neutral and matches D-02. [VERIFIED: go-wand local source][VERIFIED: local planning context] |
| OTLP environment bootstrap | Custom HTTP POST code or global `otel.Set*` initialization | OTel SDK + OTLP/HTTP exporters in a separate CLI-only module | The official OTel docs already define the SDK/exporter ownership model; the library should stop at the API. [CITED: https://opentelemetry.io/docs/languages/go/instrumentation/][VERIFIED: local planning context] |
| Attribute vocabulary policing | String literals and ad-hoc checks spread across tests | One `telemetry/attrs.go` file + allowlist tests | The phase explicitly freezes the vocabulary and bans raw values at INFO. [VERIFIED: local planning context] |
| Performance gating | Stopwatch scripts or human-read benchmark output | `benchmark_test.go`, `testing.AllocsPerRun`, and paired in-process comparisons | The repo already uses Go's benchmark harness and requires benchmark-backed claims. [VERIFIED: local code grep][VERIFIED: local planning context] |

**Key insight:** the hard part in this phase is not span creation; it is preserving the root-module dependency boundary and the disabled-path performance contract at the same time. [VERIFIED: temp go module experiment][VERIFIED: local planning context]

## Common Pitfalls

### Pitfall 1: Root Module Pollution From "Helper" Bootstrap Imports
**What goes wrong:** A seemingly isolated helper package for `FromEnv` still drags OTel SDK/exporter dependencies into the root `go.mod` once the root module imports it. [VERIFIED: temp go module experiment]  
**Why it happens:** Go resolves the imported helper as part of the root module's build list; `replace` does not hide the helper's transitive dependencies from the root module. [VERIFIED: temp go module experiment]  
**How to avoid:** Keep root-module Phase 14 limited to API/noop-based library seams, or split the CLI into its own module before importing any bootstrap package. [VERIFIED: temp go module experiment][VERIFIED: local planning context]  
**Warning signs:** `go.mod` or `go.sum` in the root repo starts listing `go.opentelemetry.io/otel/sdk` or OTLP exporters. [VERIFIED: temp go module experiment]  

### Pitfall 2: Nil-Config Query Panics After Decode Or `NewGINIndex()`
**What goes wrong:** Query-time observability assumes `idx.Config` is always non-nil and panics on decoded or hand-built indexes. [VERIFIED: local code grep]  
**Why it happens:** `Decode()` assigns the result of `readConfig`, which can be nil when the config section is absent, and many tests build `NewGINIndex()` directly. [VERIFIED: local code grep]  
**How to avoid:** Centralize logger/signals fallback in helper functions that accept `*GINConfig` and collapse nil to noop defaults. [VERIFIED: local code grep][VERIFIED: local planning context]  
**Warning signs:** `idx.Config.logger` or `idx.Config.signals` is accessed directly from query/serialize/parquet code. [VERIFIED: local code grep]  

### Pitfall 3: Runtime-Only Fields Leak Into The Wire Format
**What goes wrong:** Adding logger/signals to the serialized config path causes unnecessary wire-format churn and couples decode behavior to process-local runtime state. [VERIFIED: local code grep]  
**Why it happens:** `SerializedConfig` is explicit and easy to extend accidentally when adding new `GINConfig` fields. [VERIFIED: local code grep]  
**How to avoid:** Keep runtime observability fields out of `SerializedConfig`, `writeConfig`, and `readConfig`; use runtime helper defaults instead. [VERIFIED: local code grep]  
**Warning signs:** `SerializedConfig` grows logger, tracer, meter, adapter, or endpoint fields. [VERIFIED: local code grep]  

### Pitfall 4: Span Explosion In Predicate And S3 Inner Loops
**What goes wrong:** Per-predicate, per-page, or per-range-request spans flood exporters and bury the coarse operation boundaries the phase is supposed to surface. [CITED: https://opentelemetry.io/docs/concepts/instrumentation/libraries/][VERIFIED: local code grep]  
**Why it happens:** The obvious instrumentation points are the hottest loops: predicate evaluation, parquet page reads, and S3 range reads. [VERIFIED: local code grep]  
**How to avoid:** Keep spans on public operations only and attach verbose detail as parent-span events or guarded debug logs. [CITED: https://opentelemetry.io/docs/concepts/instrumentation/libraries/][VERIFIED: local planning context]  
**Warning signs:** `trace.Tracer.Start` appears in `evaluatePredicate`, `s3ReaderAt.ReadAt`, parquet page loops, or row-group loops. [VERIFIED: local code grep]  

### Pitfall 5: Disabled-Path Attr Construction Still Allocates
**What goes wrong:** Logging and tracing are "disabled" semantically but still pay for attr or message construction in the hot path. [VERIFIED: go doc log/slog][VERIFIED: local planning context]  
**Why it happens:** `Handler.Enabled` is only helpful if expensive work happens after the enabled check, and query evaluation is on the hot path. [VERIFIED: go doc log/slog]  
**How to avoid:** Guard debug payloads before attr construction, keep info attrs tiny and boundary-only, and add explicit alloc benchmarks instead of trusting casual inspection. [VERIFIED: go doc log/slog][VERIFIED: local planning context]  
**Warning signs:** `fmt.Sprintf`, slice building, or dynamic attr assembly happens before any `Enabled` or `span.IsRecording()` guard. [VERIFIED: local planning context][ASSUMED]  

## Code Examples

Verified patterns from official and local sources:

### Additive Context Wrapper Pattern
```go
// Source: local code pattern in query.go, parquet.go, serialize.go.
func (idx *GINIndex) Evaluate(predicates []Predicate) *RGSet {
	return idx.EvaluateContext(context.Background(), predicates)
}

func BuildFromParquet(parquetFile, jsonColumn string, config GINConfig) (*GINIndex, error) {
	return BuildFromParquetContext(context.Background(), parquetFile, jsonColumn, config)
}
```
[VERIFIED: local code grep]

### Local Signals Container Pattern
```go
// Source: /Users/tazarov/experiments/amikos/go-wand/pkg/telemetry/telemetry.go
type Signals struct {
	TracerProvider trace.TracerProvider
	MeterProvider  metric.MeterProvider
	shutdown       func(context.Context) error
}

func Disabled() Signals {
	return Signals{}
}
```
[VERIFIED: go-wand local source]

### CLI-Owned Bootstrap Pattern
```go
// Source: adapted from /Users/tazarov/experiments/amikos/go-wand/cmd/go-wand/main.go
signals, err := telemetryFromEnv(context.Background(), "gin-index")
if err != nil {
	logging.Warn(logger, "telemetry bootstrap failed", telemetry.LogAttrStatus("error"))
}
defer func() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = signals.Shutdown(shutdownCtx)
}()
```
[VERIFIED: go-wand local source]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Package-global invariant logger (`adaptiveInvariantLogger *log.Logger`) | Config-carried repo-owned logger seam with adapter leaves | Phase 14 target; current global lives in `query.go` today. [VERIFIED: local code grep][VERIFIED: local planning context] | One logging convention across the repo; no package-global mutable state. [VERIFIED: local planning context] |
| Global/provider mutation in application startup examples | Local `Signals` containers passed downward by the caller | Current OTel docs and go-wand telemetry pattern. [CITED: https://opentelemetry.io/docs/languages/go/instrumentation/][VERIFIED: go-wand local source] | Avoids global state collisions and keeps the library embeddable. [CITED: https://opentelemetry.io/docs/languages/go/instrumentation/] |
| Verbose child spans for low-level detail | Public boundary spans plus span events or logs for verbose data | Current library-instrumentation guidance. [CITED: https://opentelemetry.io/docs/concepts/instrumentation/libraries/] | Keeps telemetry volume and cardinality under control. [CITED: https://opentelemetry.io/docs/concepts/instrumentation/libraries/] |
| "Nested helper module" as dependency escape hatch | Real CLI module boundary or defer the bootstrap import entirely | Verified in this research via temp module experiments on 2026-04-22. [VERIFIED: temp go module experiment] | This is the main sequencing constraint for any `FromEnv` implementation in this repo. [VERIFIED: temp go module experiment] |

**Deprecated/outdated:**
- `SetAdaptiveInvariantLogger(*log.Logger)` in `query.go` is outdated for this milestone and should be removed rather than preserved as a deprecated compatibility bridge. [VERIFIED: local code grep][VERIFIED: local planning context]
- Any plan that imports OTLP SDK/exporter packages into the root module is outdated the moment D-05 is treated as locked. [VERIFIED: local planning context][VERIFIED: temp go module experiment]

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | A typed local `logging.Attr` contract is the safest way to satisfy the zero-alloc and frozen-vocabulary gates, even though go-wand currently uses `...any`. [ASSUMED] | Standard Stack, Architecture Patterns | The implementation could spend extra design time on an unnecessary contract divergence from go-wand. |
| A2 | If the project wants a compileable `FromEnv` helper in this phase, splitting `cmd/gin-index` into its own module is an acceptable scope increase. [ASSUMED] | Summary, Recommended Project Structure | The plan could overreach Phase 14 and destabilize build/test workflows more than the milestone wants. |

## Open Questions

1. **Does Phase 14 have permission to split `cmd/gin-index` into its own Go module?**
   - What we know: importing a nested helper module from the root module does not preserve D-05; root `go.mod` still acquires transitive deps. [VERIFIED: temp go module experiment]
   - What's unclear: whether a CLI module split belongs in this phase or must wait for Phase 15. [ASSUMED]
   - Recommendation: decide this before planning begins; it changes both package layout and test orchestration. [VERIFIED: temp go module experiment][ASSUMED]

2. **Should the logger contract stay literal go-wand (`...any`) or become typed (`...Attr`)?**
   - What we know: `log/slog` exposes `LogAttrs` as the efficient structured path, and `Handler.Enabled` is called before args are processed. [VERIFIED: go doc log/slog]
   - What's unclear: whether the repo wants a precise attr type despite the small divergence from go-wand. [ASSUMED]
   - Recommendation: resolve this in the first plan so the adapters, allowlist tests, and benchmarks are all built around one contract. [ASSUMED]

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build, tests, benchmarks | ✓ | `go1.26.2` installed; repo floor is `1.25.5` [VERIFIED: local environment audit][VERIFIED: local code grep] | — |
| `make` | Repo test/lint/security targets | ✓ | `GNU Make 3.81` [VERIFIED: local environment audit] | Run underlying commands directly. [VERIFIED: local planning context] |
| `gotestsum` | `make test` | ✓ | `v1.13.0` [VERIFIED: local environment audit][VERIFIED: local code grep] | `go test ./...` |
| `golangci-lint` | `make lint` | ✓ | `2.11.4` [VERIFIED: local environment audit] | No direct fallback; install required for gate parity. [VERIFIED: local planning context] |
| `govulncheck` | `make security-scan` | ✓ | reported by local tool as `govulncheck@v0.0.0` [VERIFIED: local environment audit] | No direct fallback; install required for gate parity. [VERIFIED: local planning context] |

**Missing dependencies with no fallback:**
- None for Phase 14 research and planning. [VERIFIED: local environment audit]

**Missing dependencies with fallback:**
- None at research time. [VERIFIED: local environment audit]

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + benchmarks in `benchmark_test.go` [VERIFIED: local code grep] |
| Config file | none; repo uses `go test`, `gotestsum`, `.golangci.yml`, and `Makefile` targets [VERIFIED: local code grep] |
| Quick run command | `go test ./...` [VERIFIED: local test run] |
| Full suite command | `make test && make lint && make security-scan` [VERIFIED: local code grep][VERIFIED: local environment audit] |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| OBS-01 | `DefaultConfig()` is silent by default and the logger seam is backend-neutral | unit | `go test -run 'TestDefaultConfigObservabilityDefaults|TestNoopLogger|TestLoggerAdapters' ./...` | ❌ `observability_test.go` needed [VERIFIED: local code grep] |
| OBS-02 | disabled logging adds 0 allocs/op and disabled tracing stays within the no-regression budget | benchmark | `go test -run '^$' -bench 'BenchmarkEvaluateDisabledLogging|BenchmarkEvaluateWithTracer' -benchmem .` | ✅ `benchmark_test.go` exists; new benchmarks required [VERIFIED: local code grep] |
| OBS-03 | no public backend-specific types in core API; adapters isolated in subpackages | unit + grep | `go test -run TestPublicObservabilitySurface ./...` and `rg -n '\\*slog\\.Logger|go\\.opentelemetry\\.io/otel/sdk' .` | ❌ dedicated contract test needed [VERIFIED: local code grep] |
| OBS-04 | INFO-level attrs never escape the allowlist | unit | `go test -run TestInfoAttrAllowlist ./...` | ❌ `observability_test.go` needed [VERIFIED: local planning context] |
| OBS-05 | `Signals` stays local and no global OTel setters appear | unit + grep | `go test -run 'TestSignalsDisabled|TestSignalsShutdownNoop' ./...` and `rg -n 'otel\\.SetTracerProvider|otel\\.SetMeterProvider|global\\.SetLoggerProvider' .` | ❌ `telemetry_test.go` needed [VERIFIED: local planning context] |
| OBS-06 | spans stay on coarse public boundaries; predicate detail uses events only | unit | `go test -run 'TestEvaluateContextEmitsParentSpanEventsOnly|TestParquetBoundariesStayCoarse' ./...` | ❌ new trace-capture tests needed [VERIFIED: local planning context] |
| OBS-07 | additive context-aware wrappers preserve current behavior | unit/integration | `go test -run 'TestEvaluateContextCompatibility|TestBuildFromParquetContextCompatibility|TestEncodeContextCompatibility|TestDecodeContextCompatibility' ./...` | ❌ new compatibility tests needed [VERIFIED: local planning context] |
| OBS-08 | adaptive invariant logging migrates with no dual logger state | unit + grep | `go test -run TestAdaptiveInvariantViolationUsesConfigLogger ./...` and `rg -n 'SetAdaptiveInvariantLogger|adaptiveInvariantLogger|\\*log\\.Logger' query.go` | ❌ test needed; grep currently finds legacy state [VERIFIED: local code grep] |

### Sampling Rate

- **Per task commit:** `go test -run 'Test.*Observability|Test.*ContextCompatibility|TestAdaptiveInvariant.*' ./...` [ASSUMED]
- **Per wave merge:** `go test ./...` and the Phase 14 benchmark command. [VERIFIED: local test run][ASSUMED]
- **Phase gate:** `make test && make lint && make security-scan` plus `go test -run '^$' -bench 'BenchmarkEvaluateDisabledLogging|BenchmarkEvaluateWithTracer' -benchmem .` [VERIFIED: local code grep][ASSUMED]

### Wave 0 Gaps

- [ ] `observability_test.go` — wrapper compatibility, nil-config silence, attr allowlist, invariant logger routing. [VERIFIED: local planning context]
- [ ] `benchmark_test.go` additions — `BenchmarkEvaluateDisabledLogging` and `BenchmarkEvaluateWithTracer`. [VERIFIED: local planning context][VERIFIED: local code grep]
- [ ] `telemetry_test.go` — `Signals.Disabled()`, shutdown no-op, and no-global-setter guard. [VERIFIED: local planning context]
- [ ] If a CLI module split is in scope: `cmd/gin-index` module-aware test/lint wiring and bootstrap tests. [ASSUMED]

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no [VERIFIED: local planning context] | — |
| V3 Session Management | no [VERIFIED: local planning context] | — |
| V4 Access Control | no [VERIFIED: local planning context] | — |
| V5 Input Validation | yes [VERIFIED: local code grep] | Reuse explicit config validation, keep allowlists closed, and sanitize any future env/bootstrap parsing rather than logging raw values. [VERIFIED: local code grep][VERIFIED: local planning context] |
| V6 Cryptography | no [VERIFIED: local planning context] | Never hand-roll crypto; this phase does not need new cryptographic controls. [VERIFIED: local planning context] |

### Known Threat Patterns for this Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Predicate values or path names leaking into INFO logs | Information Disclosure | Freeze INFO keys in one file and ban raw predicate values, path field names, doc IDs, row-group IDs, term IDs, and user content from INFO-level attrs. [VERIFIED: local planning context] |
| Global OTel provider mutation contaminates tests or host processes | Tampering | Keep `Signals` local, do not call `otel.SetTracerProvider`, `otel.SetMeterProvider`, or similar global setters in library code. [CITED: https://opentelemetry.io/docs/languages/go/instrumentation/][VERIFIED: local planning context] |
| Span or metric cardinality explosion from inner-loop instrumentation | Denial of Service | Restrict spans to public boundaries and use span events or guarded logs for verbose detail. [CITED: https://opentelemetry.io/docs/concepts/instrumentation/libraries/][VERIFIED: local planning context] |
| Runtime observability fields accidentally serialized into index payloads | Tampering | Keep logger/signals out of `SerializedConfig` and verify no format version bump is needed for runtime-only fields. [VERIFIED: local code grep] |
| S3 endpoint, bucket, or key details leaking through logs | Information Disclosure | Keep object identifiers off INFO attrs and sanitize any future debug-level endpoint summaries before emission. [VERIFIED: local planning context][ASSUMED] |

## Sources

### Primary (HIGH confidence)
- Local code audit of `gin.go`, `builder.go`, `query.go`, `serialize.go`, `parquet.go`, `s3.go`, `parser.go`, `benchmark_test.go`, `Makefile`, and `go.mod` — carrier shape, wrapper candidates, serialization boundaries, current global logger state, benchmark harness, and test tooling. [VERIFIED: local code grep]
- Local go-wand reference files: `pkg/logging/{logger.go,noop.go,doc.go}`, `pkg/logging/{slogadapter,stdadapter}`, `pkg/telemetry/{telemetry.go,boundary.go,attrs.go,doc.go,env.go}`, `docs/{logging,telemetry}.md`, and `cmd/go-wand/main.go` — canonical seam model and ownership rules. [VERIFIED: go-wand local source]
- `go list -m -json` for `go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/sdk`, `otlptracehttp`, and `otlpmetrichttp` — current versions and publish timestamps. [VERIFIED: go list -m]
- `go doc log/slog.Logger.LogAttrs`, `go doc log/slog.Handler.Enabled`, and `go doc log/slog.Attr` — current stdlib logging semantics relevant to disabled-path guards and adapter design. [VERIFIED: go doc log/slog]
- Official OTel Go instrumentation docs: https://opentelemetry.io/docs/languages/go/instrumentation/ — library vs SDK ownership, no-op meter behavior, and global-provider guidance. [CITED: https://opentelemetry.io/docs/languages/go/instrumentation/]
- Official OTel library-instrumentation docs: https://opentelemetry.io/docs/concepts/instrumentation/libraries/ — public API span guidance and "events/logs instead of verbose child spans". [CITED: https://opentelemetry.io/docs/concepts/instrumentation/libraries/]
- OTel noop package docs: https://pkg.go.dev/go.opentelemetry.io/otel/trace/noop and https://pkg.go.dev/go.opentelemetry.io/otel/metric/noop — no-op tracer/meter provider behavior. [CITED: https://pkg.go.dev/go.opentelemetry.io/otel/trace/noop][CITED: https://pkg.go.dev/go.opentelemetry.io/otel/metric/noop]
- Local temp-module experiments run on 2026-04-22 — verified that importing a replaced nested helper module still pulls its transitive deps into the root `go.mod`, and that `go list ./...` at the repo root skips nested modules. [VERIFIED: temp go module experiment]

### Secondary (MEDIUM confidence)
- Context7 lookup for `/open-telemetry/opentelemetry-go` — used as a documentation index and cross-check for current OTel Go package structure. [VERIFIED: Context7 CLI]

### Tertiary (LOW confidence)
- None. All non-local ecosystem claims in this document were either cross-checked against official OTel docs or omitted. [VERIFIED: research process]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - OTel versions were verified with `go list -m`, and the library/API split is backed by current official docs. [VERIFIED: go list -m][CITED: https://opentelemetry.io/docs/languages/go/instrumentation/]
- Architecture: MEDIUM - the config-carrier and raw-wrapper recommendations are strongly grounded in current code, but the exact CLI-bootstrap path depends on whether the phase accepts a CLI module split. [VERIFIED: local code grep][VERIFIED: temp go module experiment][ASSUMED]
- Pitfalls: HIGH - the dependency-boundary pitfall was reproduced locally, and the no-global-provider / event-vs-span guidance is directly documented by OTel. [VERIFIED: temp go module experiment][CITED: https://opentelemetry.io/docs/concepts/instrumentation/libraries/][CITED: https://opentelemetry.io/docs/languages/go/instrumentation/]

**Research date:** 2026-04-22  
**Valid until:** 2026-05-22 for current OTel module versions and docs; earlier if the project decides to split `cmd/gin-index` into its own module before planning. [VERIFIED: go list -m][ASSUMED]
