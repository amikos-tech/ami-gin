# Stack Research — v1.1 Performance, Observability & Experimentation

**Domain:** Go library (pure-Go pruning index for Parquet) — additive milestone
**Researched:** 2026-04-21
**Confidence:** HIGH (pure-simdjson, go-wand patterns verified by direct source inspection via `gh api`)

## Scope Guard

This research covers **only the three new capability areas** for v1.1. v1.0's validated stack (Go 1.25.5, roaring/v2, xxhash/v2, klauspost/compress, parquet-go, ojg, aws-sdk-go-v2, gopter, pkg/errors) is unchanged and is **not re-researched**.

---

## 1. SIMD JSON Builder Path — `github.com/amikos-tech/pure-simdjson`

### Recommended

| Dependency | Version | Purpose | Why |
|------------|---------|---------|-----|
| `github.com/amikos-tech/pure-simdjson` | **`main` / untagged** (no tags exist yet; use a pinned commit SHA) | Opt-in SIMD JSON parsing path for `GINBuilder.AddDocument` | Preserves distinct `int64`/`uint64`/`float64` numeric classification natively — aligns with v1.0's exact-int guarantee from Phase 07 |
| `github.com/ebitengine/purego` | `v0.10.0` (transitive) | FFI plumbing under pure-simdjson (dlopen of the Rust-built shared library) | Pulled automatically; no direct import |

### Critical Discovery — Not Pure Go, Not CGo

Despite the name, `pure-simdjson` is **not a pure-Go JSON parser**. It is a `purego`-based binding to a **Rust-built C shared library** (`libpure_simdjson.{so,dylib,dll}`), produced via `cbindgen` from a Rust crate that wraps the C++ simdjson library.

Evidence:
- `Cargo.toml` at repo root (`crate-type = ["cdylib", "staticlib", "rlib"]`)
- `library_loading.go`, `library_unix.go`, `library_windows.go` — dlopen logic
- `go.mod` requires `github.com/ebitengine/purego v0.10.0`
- Build recipe: `cargo build --release` must produce the shared library before `go test ./...` works
- Parser API (`parser.go`) uses `ffi.ParserHandle`, `library.bindings.ParserParse(handle, data)`

**Consequences for ami-gin:**
- **Not strictly CGo** — `purego` avoids the Go CGo toolchain, so cross-compilation from a pure-Go source POV still works
- **But requires a native library at runtime** — breaks the "pure-Go dependency chain" claim for any consumer opting into the SIMD path
- **Build-time Rust toolchain dependency** — contributors who touch this path need `cargo` installed; CI needs a matrix with prebuilt `.so`/`.dylib`/`.dll` artifacts or an on-the-fly cargo build step
- **Dual artifact distribution** — releases must ship the native library alongside (or consumers must build it themselves)

### API Surface (verified from source)

Package name is `purejson` (not `simdjson`). Core types:

```go
parser, err := purejson.NewParser()            // dlopens the library, verifies ABI
doc, err := parser.Parse(jsonBytes)            // returns *Doc; one live Doc per Parser
defer doc.Close()
root := doc.Root()                             // Element view

// Typed accessors preserving int64/uint64/float64 split:
kind := root.Type()                            // TypeInt64|TypeUint64|TypeFloat64|TypeString|...
s, err := elem.GetString()
i, err := elem.GetInt64()                      // ErrPrecisionLoss if BIGINT classification
u, err := elem.GetUint64()
f, err := elem.GetFloat64()                    // ErrPrecisionLoss for non-exact ints

// Iteration:
obj, err := elem.AsObject()
it := obj.Iter()                               // ObjectIter with .Next() .Key() .Value() .Err()
arr, err := elem.AsArray()
ait := arr.Iter()                              // ArrayIter with .Next() .Value() .Err()
```

**Integration point with v1.0 builder:** The current `parseAndStageDocument` walks `any`-typed decoded maps/slices from `encoding/json` + `UseNumber()`. The purejson path must be a **parallel walker over `purejson.Element`** that feeds the same `stageMaterializedValue`/`stageCompanionRepresentations` calls. Do **not** round-trip purejson → `any` → builder (that defeats the whole point and loses the int64/uint64 distinction).

**Exact-int preservation (CRITICAL):** purejson's `TypeInt64`/`TypeUint64` return path is **stronger** than our current `json.Number.Int64()` probe because simdjson does the classification at parse time and surfaces `ErrPrecisionLoss` explicitly. The builder's numeric-fidelity contract (Phase 07) is *easier* to honor here than with `encoding/json`.

### What NOT to Add

| Avoid | Why | Instead |
|-------|-----|---------|
| `github.com/minio/simdjson-go` | Pure Go, but does NOT preserve the int64/uint64/float64 split the way we need; weaker numeric semantics | `pure-simdjson` |
| `github.com/valyala/fastjson` | Mutates the input buffer; its `Value` type reuses memory across calls in ways that conflict with our builder's staging | Keep `encoding/json` as the default; pure-simdjson as opt-in |
| `github.com/goccy/go-json` | Drop-in `encoding/json` replacement; gains are modest (~2x) and don't justify a new dependency unless the baseline has been benchmarked against it first | Benchmark only; do not adopt without evidence |
| CGo binding to simdjson proper | Heavier toolchain requirement (C++ compiler), same runtime-library shipping problem, no advantage over the Rust-wrapped variant | `pure-simdjson` via `purego` |

### License Check

- **`github.com/amikos-tech/pure-simdjson`**: **no `LICENSE` file at repo root** as of 2026-04-20 push. GitHub API returns `"license":null`. Both repos live in the same `amikos-tech` org as `ami-gin`, so the license decision is coordinated, but **the dependency MUST have a license file before ami-gin takes it as a declared dependency** (OSS hygiene, Go module consumers need a clear license).
- Transitive: `ebitengine/purego` is **Apache-2.0** (MIT-compatible).
- Rust crate inside pure-simdjson wraps upstream simdjson (Apache-2.0 / MIT dual-licensed) — compatible with ami-gin's MIT.

**Action required before declaring this dependency stable:** add a `LICENSE` (MIT to match ami-gin) to `amikos-tech/pure-simdjson`. Track as an explicit roadmap prerequisite, not a silent assumption.

### Versioning Risk

- No tags exist yet in `pure-simdjson`. The ABI contract doc (`docs/ffi-contract.md`) references `^0.1.x`, implying a 0.1 tag is imminent. Pin to a specific commit SHA in `go.mod` (`go mod edit -require=github.com/amikos-tech/pure-simdjson@<sha>`) until a real tag ships.
- purejson enforces an ABI version check at `NewParser()`; mismatched shared-library versions fail fast with `ErrABIMismatch`. Good — exposes version drift immediately.

---

## 2. Observability / Logging Primitives — Mirror `go-wand`'s Pattern

### Recommended (direct dependencies)

| Dependency | Version | Purpose | Why |
|------------|---------|---------|-----|
| `go.opentelemetry.io/otel` | `v1.43.0` | Trace + metric interface types only (we import the API, not the SDK) | Zero-cost when disabled via `tracenoop` / `metricnoop`; matches go-wand's Signals pattern; industry standard |
| `go.opentelemetry.io/otel/metric` | `v1.43.0` | `metric.MeterProvider` interface | Same |
| `go.opentelemetry.io/otel/metric/noop` | `v1.43.0` | Package-local no-op meter provider | Default fallback when metrics are not wired |
| `go.opentelemetry.io/otel/trace` | `v1.43.0` | `trace.TracerProvider` interface | Same |
| `go.opentelemetry.io/otel/trace/noop` | `v1.43.0` | Package-local no-op tracer provider | Default fallback when tracing is not wired |

### Recommended (indirect, only in adapter subpackages)

| Dependency | Version | Where | Why |
|------------|---------|-------|-----|
| `go.opentelemetry.io/otel/sdk` | `v1.43.0` | **Optional** `pkg/telemetry` env-bootstrap helper (`FromEnv`) | Only built in when callers import the helper; core packages never pull SDK |
| `go.opentelemetry.io/otel/sdk/metric` | `v1.43.0` | Same | Same |
| `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp` | `v1.43.0` | Same | Same |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` | `v1.43.0` | Same | Same |

### Recommended Pattern — Lift Directly from go-wand

Proven in `amikos-tech/go-wand/pkg/logging` + `pkg/telemetry`. Files verified: `logger.go`, `noop.go`, `doc.go`, `telemetry.go`, `attrs.go`, `env.go`.

**Logging seam (`internal/obs` or `pkg/logging`):**

```go
// Library-owned tiny interface — no backend types in core packages
type Logger interface {
    Enabled(Level) bool
    Log(Level, string, ...any)
}
```

- Levels: `LevelDebug`, `LevelInfo`, `LevelWarn`, `LevelError` (matches slog conventions)
- Package-level helpers `Debug/Info/Warn/Error` that check `Enabled()` first for Debug (avoids payload-construction cost)
- `NewNoop()` returns a shared no-op instance; `Default(logger)` normalises `nil` to noop
- **Adapters in separate subpackages** so importing the interface does not pull backend deps:
  - `pkg/logging/slogadapter` → wraps `*slog.Logger` (stdlib `log/slog`)
  - `pkg/logging/stdadapter` → wraps `*log.Logger`
  - Optional: `pkg/logging/zapadapter` only if a concrete user asks for it — avoid unless requested

**Telemetry seam (`pkg/telemetry`):**

```go
type Signals struct {
    TracerProvider trace.TracerProvider
    MeterProvider  metric.MeterProvider
    shutdown       func(context.Context) error
}

func Disabled() Signals                                    // zero-value, nil providers
func NewSignals(tp trace.TracerProvider, mp metric.MeterProvider, shutdown func(context.Context) error) Signals
func (s Signals) Enabled() bool
func (s Signals) Tracer(scope string) trace.Tracer         // falls back to tracenoop
func (s Signals) Meter(scope string) metric.Meter          // falls back to metricnoop
func (s Signals) Shutdown(ctx context.Context) error
```

**Zero-cost-when-disabled proof** (directly from go-wand `pkg/telemetry/telemetry.go`):
1. Nil-provider check is one pointer compare per `Tracer()`/`Meter()` call at the boundary
2. No-op providers are package-level vars reused on every call (no allocation)
3. Span start on a no-op tracer is a constant-time, non-sampling path in the OTEL SDK
4. Counter `.Add()` on a noop meter is a no-op function call — the compiler devirtualizes if the provider type is known, otherwise it is one interface call + an empty body
5. Boundary APIs accept `Signals` by value; `Disabled()` is the zero value — no allocations, no heap

**Fixed attribute vocabulary** (mirror go-wand `attrs.go`): freeze a small set of metric names and attribute keys up front to avoid cardinality blow-up. Proposed for ami-gin:

```
Metric names (all prefixed gin_index.*):
  gin_index.build.duration        (ms)
  gin_index.build.documents       (1)
  gin_index.build.rowgroups       (1)
  gin_index.build.bytes_out       (By)
  gin_index.query.duration        (ms)
  gin_index.query.rowgroups_in    (1)
  gin_index.query.rowgroups_out   (1)  // after pruning
  gin_index.query.pruning_ratio   (1)
  gin_index.serialize.duration    (ms)
  gin_index.serialize.bytes       (By)

Allowed attributes (low-cardinality only):
  operation       (string: "build"|"query"|"serialize"|...)
  predicate_op    (string: "EQ"|"GT"|"Contains"|"Regex"|...)
  path_mode       (string: "classic"|"adaptive"|"bloom_only")
  status          (string: "ok"|"error")
  error.type      (closed set: "config"|"io"|"format"|"integrity"|"other")
```

**Hard bans** (from go-wand's doc.go — good discipline to copy): never attribute path values, JSON field values, query text, document IDs, row-group IDs, or term IDs. That would explode cardinality and leak data.

### What NOT to Add

| Avoid | Why | Instead |
|-------|-----|---------|
| `go.uber.org/zap` as a core dependency | Forces a logger choice on every consumer; inflates binary size | Ship `zapadapter` only if explicitly requested, in a separate subpackage |
| `github.com/rs/zerolog` | Same reasoning — opinionated backend | Stay backend-neutral with the tiny `Logger` interface |
| `github.com/sirupsen/logrus` | Mature but slower; maintenance-only mode | N/A |
| Setting `otel.SetTracerProvider` / `SetMeterProvider` | Mutates process-global state; libraries **must not** do this (from go-wand's explicit contract) | Accept providers via `Signals` only; callers decide global registration |
| A repo-owned `Event` / `Span` DSL on top of OTEL | Adds a pointless abstraction layer; OTEL is already the abstraction | Use `trace.Tracer`/`metric.Meter` directly at boundary |
| Logging inside hot paths (`evaluateEQ`, `walkJSON`) | Even cheap log calls add overhead; violates v1.0's benchmark-backed constraint | Keep hot paths logger-free; only log at build/query *boundary* entry and exit |

### License Check

All OTEL packages: **Apache-2.0** (compatible with ami-gin's MIT).

---

## 3. Experimentation CLI (JSONL → Index Summary)

### Recommended

| Addition | Version | Purpose | Why |
|----------|---------|---------|-----|
| **stdlib `flag`** | n/a | Subcommand flag parsing | Already used in go-wand's `internal/indexcli/app.go`; zero new deps; the existing `cmd/gin-index/main.go` likely already uses it |
| **stdlib `text/tabwriter`** | n/a | Aligned columnar output for the summary report | Already in stdlib; no deps; proven for table output |
| **stdlib `encoding/json`** + `bufio.Scanner` | n/a | JSONL streaming ingest | Already used; unchanged |

### CLI Framework Decision

**Stay on stdlib `flag`. Do not add cobra, urfave/cli, or kong.**

Evidence from go-wand (`internal/indexcli/app.go`):
- Subcommand dispatch via `switch args[0]` (manual)
- Per-subcommand `flag.FlagSet` created inline
- Usage printed manually via helper functions
- Cobra is pulled in `pure-simdjson` only as a transitive indirect; go-wand does not use cobra in its CLI at all

This is the **minimal viable surface** and matches the "radically simple" CLAUDE.md directive. A new `experiment` subcommand fits inside the existing `cmd/gin-index/main.go` dispatcher with ~60 LOC of new code.

### Pretty-Printing — No New Dependency

| Need | Solution | Why |
|------|----------|-----|
| Aligned columnar tables | `text/tabwriter` (stdlib) | Zero deps; enough for path summaries, mode column, cardinality estimate, pruning candidates |
| Percentages / histograms | Custom sprintf formatters | Trivial; no lib needed |
| Color output | **Do not add** (`fatih/color` or `pterm`) | Terminal detection logic, TTY edge cases, and an extra dep for a teaching/experiment CLI is scope creep. If contrast is needed later, revisit. |
| JSON output mode | `encoding/json` with `MarshalIndent` | Already a dep; add `--output json` flag for machine-readable output |

### What NOT to Add

| Avoid | Why | Instead |
|-------|-----|---------|
| `github.com/spf13/cobra` | Pulls in pflag + mousetrap; overkill for 4-5 subcommands that already work on `flag` | stdlib `flag` with manual dispatch |
| `github.com/urfave/cli/v2` | Same reasoning; forces restructuring existing subcommands | stdlib `flag` |
| `github.com/alecthomas/kong` | Elegant for struct-based CLIs, but no existing use in the org; adopting it means rewriting `build`/`query`/`info`/`extract` for consistency | stdlib `flag` |
| `github.com/olekukonko/tablewriter` | Nicer tables, but `text/tabwriter` already ships in stdlib | `text/tabwriter` |
| `github.com/fatih/color` | TTY-detection subtleties; a library summary CLI doesn't need color | Plain text |
| `github.com/pterm/pterm` | Heavyweight TUI library | Not needed |

### License Check

All stdlib — n/a.

---

## SEED-001 — simdjson Example Datasets (Test Corpus, Not Code Dependencies)

The seed calls for **using the simdjson project's example JSON files** as a real-data benchmark corpus (`twitter.json`, `github_events.json`, geographic/numeric/nested payloads). Status:

- **Source:** `https://github.com/simdjson/simdjson/tree/master/jsonexamples`
- **License:** simdjson is **Apache-2.0** — example files inherit that; fine to vendor selectively.
- **Integration:** these are **test fixtures**, not runtime dependencies. They go under `testdata/simdjson-examples/` with a license-preservation `NOTICE.md`. Size budget: keep under ~5 MB total (select small/medium files, document oversize ones as opt-in downloads gated on `TESTDATA_CORPUS_PATH` like Phase 11's external corpus pattern).
- Feeds both the simdjson-vs-encoding/json benchmark suite and the experimentation CLI's worked examples.

---

## Installation Snippet (for roadmap planning)

```bash
# Phase: SIMD integration (opt-in build tag preferred, e.g., -tags simdjson)
go get github.com/amikos-tech/pure-simdjson@<pinned-sha>
# Plus: cargo build --release inside pure-simdjson (or shipped prebuilt binaries)

# Phase: Observability
go get go.opentelemetry.io/otel@v1.43.0
go get go.opentelemetry.io/otel/metric@v1.43.0
go get go.opentelemetry.io/otel/metric/noop@v1.43.0
go get go.opentelemetry.io/otel/trace@v1.43.0
go get go.opentelemetry.io/otel/trace/noop@v1.43.0
# Optional (only for FromEnv helper subpackage):
go get go.opentelemetry.io/otel/sdk@v1.43.0
go get go.opentelemetry.io/otel/sdk/metric@v1.43.0
go get go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp@v1.43.0
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@v1.43.0

# Phase: Experimentation CLI
# (no new deps)
```

---

## Build-Tag Strategy for SIMD

Recommend gating pure-simdjson behind a build tag (`//go:build simdjson`) so:
- The default `go build ./...` for consumers pulls **no** `purego` / native-library dependencies
- The SIMD path is opt-in, consistent with the stated "opt-in builder parser" goal in the milestone definition
- CI runs both matrices: `go test ./...` (default, `encoding/json`) and `go test -tags simdjson ./...` (SIMD, requires native lib)

This preserves v1.0's pure-Go default posture while enabling the performance path for users willing to ship a native library.

---

## Compatibility Matrix

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| ami-gin Go 1.25.5 | pure-simdjson Go 1.24 | OK — lower minimum is fine |
| ami-gin klauspost/compress v1.18.5 | go-wand v1.18.3 | OK — same minor, forward-compatible |
| ami-gin parquet-go v0.29.0 | go-wand v0.26.4 | OK — no API change we depend on |
| ami-gin aws-sdk-go-v2 v1.41.6 | go-wand v1.41.1 | OK — patch-level |
| OTEL v1.43.0 | Go 1.25.5 | OK — OTEL supports Go 1.22+ |

---

## Sources

- `github.com/amikos-tech/pure-simdjson` — direct source inspection via `gh api repos/amikos-tech/pure-simdjson/contents/...` (files: `go.mod`, `parser.go`, `purejson.go`, `doc.go`, `element.go`, `iterator.go`, `Cargo.toml`, `Makefile`) — verified 2026-04-21
- `github.com/amikos-tech/go-wand` (private, org-accessible) — direct source inspection of `pkg/logging/{logger.go,noop.go,doc.go}`, `pkg/logging/slogadapter/slog.go`, `pkg/telemetry/{telemetry.go,attrs.go,doc.go,env.go}`, `cmd/go-wand/main.go`, `internal/indexcli/app.go`, `go.mod` — verified 2026-04-21
- OpenTelemetry Go API: `go.opentelemetry.io/otel` v1.43.0 — version confirmed from go-wand's pinned deps (HIGH confidence — go-wand is a production library with the pattern actively used)
- `ebitengine/purego` v0.10.0 — pinned as direct dep of pure-simdjson
- simdjson example datasets: `https://github.com/simdjson/simdjson/tree/master/jsonexamples` (per SEED-001)
- v1.0 baseline stack: `/Users/tazarov/experiments/amikos/custom-gin/go.mod` (current tree, post Phase 12)

---
*Stack research for: v1.1 Performance, Observability & Experimentation*
*Researched: 2026-04-21*
