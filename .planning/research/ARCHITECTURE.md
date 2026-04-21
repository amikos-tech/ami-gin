# Architecture Research — v1.1 Performance, Observability & Experimentation

**Domain:** Integration design for three additive features (SIMD JSON parser, telemetry seams, experimentation CLI) layered onto the v1.0 `ami-gin` Go library.
**Researched:** 2026-04-21
**Confidence:** HIGH for existing-code integration points (direct file reads); MEDIUM for `pure-simdjson` lifecycle details (verified via GitHub source read, not yet vendored); MEDIUM for `go-wand` telemetry pattern (verified via GitHub source read, to be adapted).

---

## System Overview — Where the Three Features Plug In

The existing v1.0 library is a single flat `package gin` with a builder-then-immutable-index pipeline. The v1.1 changes are **additive seams**, not restructures:

```
                         ┌──────────────────────────────────────────────────┐
                         │              cmd/gin-index (CLI)                 │
                         │  build │ query │ info │ extract │ experiment ★  │
                         └──────────────────┬───────────────────────────────┘
                                            │ uses
                         ┌──────────────────▼───────────────────────────────┐
                         │                  package gin                     │
                         │                                                  │
   ┌───── JSON bytes ───►│  AddDocument(docID, jsonDoc)                    │
   │                     │         │                                        │
   │  ┌──────────────┐   │         ▼                                        │
   │  │ Parser ★     │◄──┼── parseAndStageDocument(doc, rgID)              │
   │  │ (interface)  │   │         │  emits staged paths/values              │
   │  │              │   │         ▼                                        │
   │  │ • Stdlib     │   │    stageStreamValue / stageMaterializedValue    │
   │  │   (default)  │   │         │                                        │
   │  │ • SIMD ★     │   │         ▼                                        │
   │  │   (build tag │   │    mergeDocumentState → pathData[path]          │
   │  │   + opt-in)  │   │         │                                        │
   │  └──────────────┘   │         ▼                                        │
   │                     │    Finalize() ──► GINIndex (immutable)          │
   │                     │         │                                        │
   │                     │         ▼                                        │
   │                     │    Evaluate(predicates) ──► RGSet                │
   │                     │                                                  │
   │  ┌──────────────┐   │   emits events (build / query / serialize)      │
   │  │ Telemetry ★  │◄──┼──────────────┐                                  │
   │  │ (interface)  │   │              │                                  │
   │  │ • Noop       │   │   Encode/Decode                                  │
   │  │   (default)  │   │                                                  │
   │  │ • slog       │   └──────────────────────────────────────────────────┘
   │  │ • OTel       │
   │  └──────────────┘
   │
   └── ★ = new in v1.1
```

Three injection points, three contracts, zero surgery on existing hot paths when the features are disabled (default state).

---

## Component Responsibilities

| Component | New / Modified | File | Responsibility |
|-----------|----------------|------|----------------|
| `Parser` interface | **new** | `parser.go` (new) | Abstracts document-to-staged-events translation |
| `stdlibParser` | **new** | `parser_stdlib.go` (new) | Default parser — today's `json.Decoder.UseNumber()` logic moved behind the interface |
| `simdParser` | **new** | `parser_simd.go` (new, build-tagged `//go:build simdjson`) | Wraps `pure-simdjson` element traversal |
| `AddDocument` | **modified** | `builder.go:287` | Delegates to injected `Parser` rather than calling `parseAndStageDocument` directly |
| `parseAndStageDocument` | **modified** | `builder.go:322` | Becomes the `stdlibParser.Parse` body verbatim, no signature change |
| `Telemetry` interface | **new** | `telemetry.go` (new) | Single narrow event sink; default noop is zero-cost |
| Builder event emission | **modified** | `builder.go:Finalize` + a small number of merge/poison call sites | Emits structured events via injected telemetry |
| Query event emission | **modified** | `query.go:Evaluate`, per-operator evaluators | Emits pruning-decision events |
| Serialize event emission | **modified** | `serialize.go:Encode`/`Decode` | Emits compression/decode parity events |
| `experiment` subcommand | **new** | `cmd/gin-index/experiment.go` (new) | JSONL → builder → info-style summary |
| `SetAdaptiveInvariantLogger` | **kept** | `query.go:24` | Existing `*log.Logger` seam stays; will be wired into the telemetry adapter as a back-compat path |

---

## Architectural Patterns (per feature)

### Pattern 1: Parser as Functional-Option Seam (not build tag alone)

**What:** `Parser` is a plain interface injected via `BuilderOption`. The default wiring in `NewBuilder` assigns `stdlibParser{}` if none is provided. The SIMD implementation is additionally guarded by a Go build tag so `pure-simdjson` (a Rust shared library loaded at runtime via `purego`) is not a compile-time dependency of plain consumers.

**When to use:** Every time a builder is constructed. The choice must be explicit (`WithParser(...)`) so library consumers never accidentally depend on a shared library they didn't ask for.

**Trade-offs:**
- **Pro:** Consumers of `ami-gin` have zero new external deps unless they opt in with `-tags simdjson` *and* pass `WithParser(simdgin.NewSIMDParser())`.
- **Pro:** Keeps the exact-int contract intact — the interface emits staged observations, not raw floats, so the parser boundary preserves numeric fidelity.
- **Con:** Two parsers to test. Mitigated by running the existing builder test suite through both parsers (table-driven `parser_parity_test.go`).
- **Con:** The SIMD path requires a `libpure_simdjson.{so,dylib,dll}` in a discoverable path at runtime (verified in `pure-simdjson/library_loading.go`).

**Proposed interface:**

```go
// parser.go (new, no build tags)
package gin

// Parser translates one JSON document into staged per-path observations,
// writing them into the supplied state via the builder-provided sink.
// Implementations MUST preserve exact-int semantics: integers outside the
// float64-exact range [-2^53, 2^53] must be reported as isInt=true through
// stageNumericObservation rather than promoted to float64.
type Parser interface {
    // Name returns a stable identifier for telemetry/debug (e.g. "stdlib",
    // "pure-simdjson/v0.x").
    Name() string

    // Parse walks jsonDoc and stages observations into state for the given
    // rgID. It MUST call sink methods on the builder, not mutate builder
    // state directly. Errors from the sink are returned unwrapped.
    Parse(jsonDoc []byte, rgID int, sink ParserSink) error
}

// ParserSink is the narrow contract a Parser uses to publish observations.
// It is intentionally smaller than the private staging API so alternative
// parsers cannot reach into the builder's internals.
type ParserSink interface {
    BeginDocument(rgID int) *documentBuildState
    StageScalar(state *documentBuildState, path string, token any) error
    StageJSONNumber(state *documentBuildState, path, raw string) error
    StageNativeNumeric(state *documentBuildState, path string, v any) error
    StageMaterialized(state *documentBuildState, path string, value any, allowTransform bool) error
    DecodeTransformed(state *documentBuildState, path string, value any) error
}
```

**Wiring:**

```go
// builder.go — modified
func WithParser(p Parser) BuilderOption {
    return func(b *GINBuilder) error {
        if p == nil {
            return errors.New("parser cannot be nil")
        }
        b.parser = p
        return nil
    }
}

// NewBuilder: default parser assignment after options loop.
if b.parser == nil {
    b.parser = stdlibParser{} // existing behaviour, no surprise
}

// AddDocument: replace the direct call with parser dispatch.
state := newDocumentBuildState(pos)
if err := b.parser.Parse(jsonDoc, pos, b); err != nil {
    return errors.Wrap(err, "parser failed")
}
return b.mergeDocumentState(docID, pos, exists, state)
```

The SIMD implementation lives in a separate file with `//go:build simdjson`, so `go build ./...` without the tag does not import `pure-simdjson`:

```go
// parser_simd.go (new)
//go:build simdjson

package gin

import purejson "github.com/amikos-tech/pure-simdjson"

type simdParser struct { p *purejson.Parser }

func NewSIMDParser() (Parser, error) {
    p, err := purejson.NewParser()
    if err != nil {
        return nil, errors.Wrap(err, "initialize pure-simdjson; shared library must be discoverable (set PUREJSON_LIBRARY or place libpure_simdjson on loader path)")
    }
    return &simdParser{p: p}, nil
}

func (s *simdParser) Name() string { return "pure-simdjson" }
func (s *simdParser) Parse(jsonDoc []byte, rgID int, sink ParserSink) error { /* walk via purejson Doc/Element */ }
```

> Note: `pure-simdjson` is not pure Go — it is a Rust library loaded via `purego`. The `//go:build simdjson` tag keeps the import out of default builds; the CGo-free nature means no `CGO_ENABLED=1` requirement, but a shared library MUST be present at runtime. The roadmap will need a distribution story (vendored `.so`/`.dylib`/`.dll` in `libs/` like go-wand, or env-configurable path). Verified from `pure-simdjson/parser.go:NewParser` and `library_loading.go`.

### Pattern 2: Telemetry as Noop-Default Narrow Interface

**What:** A single `Telemetry` interface with one `Event(ctx, name, attrs...)` method plus `Enabled(level)` for cheap guards. The default is `noopTelemetry{}` — its `Event` is a no-op method on a value receiver that the Go compiler inlines away. Adapters ship for `slog.Logger` (stdlib, zero new deps) and optionally for OTel traces/metrics (separate sub-package to avoid pulling OTel into the core module).

**When to use:** Wire once on the `GINIndex`/`GINBuilder` via `WithTelemetry(...)`. Every hot-path emission site guards with `if t.Enabled(LevelDebug) { … }` to skip attribute materialization when disabled.

**Trade-offs:**
- **Pro:** Zero-allocation when unset (verified pattern — same shape as `go-wand/pkg/logging`: `Enabled(Level) bool` gate before `Log(...)`).
- **Pro:** `slog` adapter gives structured logs with zero new dependencies.
- **Pro:** OTel adapter is optional and lives in `telemetry/otel` sub-package — consumers who don't want OTel don't pay for it.
- **Con:** Event *attribute lists* still allocate unless gated. Mitigation: every emission site is `if t.Enabled(Lx)` guarded.
- **Con:** Two parallel logger patterns existed (the private `adaptiveInvariantLogger` at `query.go:17`). The new telemetry seam **does not break it** — the adaptive-invariant logger stays for back-compat, and the telemetry adapter routes `LevelError`/`LevelWarn` into it when both are set.

**Proposed interface:**

```go
// telemetry.go (new)
package gin

import "context"

type Level uint8

const (
    LevelDebug Level = iota
    LevelInfo
    LevelWarn
    LevelError
)

// Event names are stable, namespaced strings. The set is closed per version
// and documented in telemetry.go so consumers can write collectors.
const (
    EventBuilderDocumentAdded       = "gin.builder.document_added"
    EventBuilderMergePoisoned       = "gin.builder.merge_poisoned"
    EventBuilderFinalizeCompleted   = "gin.builder.finalize_completed"
    EventBuilderAdaptivePromotion   = "gin.builder.adaptive_promotion"
    EventQueryPredicateEvaluated    = "gin.query.predicate_evaluated"
    EventQueryBloomHit              = "gin.query.bloom_hit"
    EventQueryBloomMiss             = "gin.query.bloom_miss"
    EventQueryTrigramShortCircuit   = "gin.query.trigram_short_circuit"
    EventQueryRowGroupsPruned       = "gin.query.row_groups_pruned"
    EventSerializeEncoded           = "gin.serialize.encoded"
    EventSerializeDecoded           = "gin.serialize.decoded"
    EventSerializeDecodeParity      = "gin.serialize.decode_parity"
)

// Telemetry is the single emission contract. Implementations MUST be safe
// for concurrent use and MUST NOT retain the attrs slice beyond the call.
type Telemetry interface {
    Enabled(level Level) bool
    Event(ctx context.Context, level Level, name string, attrs ...Attr)
}

// Attr is a key/value pair. Using a concrete struct (not ...any) avoids the
// per-call slice allocation that slog-style variadic APIs incur.
type Attr struct {
    Key   string
    Value any
}

// NoopTelemetry is the zero-cost default.
type NoopTelemetry struct{}
func (NoopTelemetry) Enabled(Level) bool { return false }
func (NoopTelemetry) Event(context.Context, Level, string, ...Attr) {}
```

**Emission sites (concrete file:function references):**

| Site | Level | Event | Attrs |
|------|-------|-------|-------|
| `builder.go:AddDocument` end (line 304) | Debug | `builder.document_added` | `doc_id`, `rg_id`, `paths_staged` |
| `builder.go:mergeDocumentState` poison branch (line 763) | Error | `builder.merge_poisoned` | `cause` |
| `builder.go:Finalize` end (line 1129) | Info | `builder.finalize_completed` | `num_paths`, `num_docs`, `num_rgs`, `parser_name` |
| `builder.go:buildAdaptiveStringIndex` (line 210) | Debug | `builder.adaptive_promotion` | `path`, `promoted_terms`, `bucket_count` |
| `query.go:Evaluate` per-predicate loop (line 42) | Debug | `query.predicate_evaluated` | `path`, `op`, `candidate_count`, `elapsed_ns` |
| `query.go:evaluateEQ` bloom path (existing bloom-hit logic) | Debug | `query.bloom_hit`/`.bloom_miss` | `path`, `value_hash` |
| `query.go:evaluateRegex` trigram short-circuit (regex.go integration) | Debug | `query.trigram_short_circuit` | `path`, `literals_extracted` |
| `query.go:Evaluate` after final intersect (line 49) | Info | `query.row_groups_pruned` | `total_rgs`, `matched_rgs`, `pruning_pct` |
| `serialize.go:Encode` end | Info | `serialize.encoded` | `bytes`, `zstd_level`, `elapsed_ns` |
| `serialize.go:Decode` end | Info | `serialize.decoded` | `bytes`, `version`, `elapsed_ns` |
| Decode parity check sites | Warn if mismatch | `serialize.decode_parity` | `check_name`, `expected`, `actual` |

**Wiring:**

```go
// On GINConfig (so both builder and the resulting index carry it through)
func WithTelemetry(t Telemetry) ConfigOption { /* ... */ }

// Default in DefaultConfig: NoopTelemetry{}
// Adapter: telemetry_slog.go provides NewSlogTelemetry(*slog.Logger) Telemetry
// Optional sub-package: telemetry/otel provides NewOTelTelemetry(trace.Tracer, metric.Meter)
```

### Pattern 3: `experiment` as a New Subcommand in the Existing CLI

**What:** Add a fifth subcommand (`experiment`) to `cmd/gin-index/main.go`. Reuse the existing `writeIndexInfo`/`formatPathInfo` helpers for the summary. Data flow: JSONL file → line-by-line stream read → `builder.AddDocument(pos, line)` → `Finalize()` → `writeIndexInfo`. No Parquet involvement.

**When to use:** Consumers want to see "what would my index look like if I indexed this JSONL file?" — teaching, ad-hoc tuning, benchmarking against small corpora.

**Trade-offs:**
- **Pro:** Zero new binary; reuses `parsePredicate`, `writeIndexInfo`, `formatPathInfo`, `describeTypes` (`cmd/gin-index/main.go:445`, `:461`, `:873`).
- **Pro:** Naturally exercises both new seams — CLI flags can toggle `--parser=simd` (when built with the tag) and `--log-level=debug` (wires a `slog` telemetry adapter to stderr).
- **Con:** Slightly inflates the Parquet-focused CLI. Mitigated: documented as "experimentation only, not for production pipelines."
- **Rejected alternative:** A separate `cmd/gin-experiment/main.go` binary. Rejected because (a) it would duplicate `writeIndexInfo`/predicate parsing, (b) it fragments the shipped-binary story, and (c) the existing CLI already mixes build+query+info — one more related verb is not a coherence violation.

**Proposed subcommand skeleton:**

```go
// cmd/gin-index/experiment.go (new)
func cmdExperiment(args []string) {
    if code := runExperiment(args, os.Stdout, os.Stderr); code != 0 {
        os.Exit(code)
    }
}

func runExperiment(args []string, stdout, stderr io.Writer) int {
    fs := flag.NewFlagSet("experiment", flag.ContinueOnError)
    fs.SetOutput(stderr)
    numRGs := fs.Int("rgs", 64, "Number of simulated row groups (documents are round-robin assigned)")
    parser := fs.String("parser", "stdlib", "Parser: stdlib | simd (simd requires -tags simdjson build)")
    logLevel := fs.String("log-level", "off", "Telemetry: off | info | debug")
    outJSON := fs.Bool("json", false, "Emit summary as JSON instead of human-readable")
    if err := fs.Parse(args); err != nil { return 1 }

    if fs.NArg() != 1 {
        fmt.Fprintln(stderr, "Usage: gin-index experiment [flags] <file.jsonl>")
        return 1
    }

    tel := buildTelemetryFromFlag(*logLevel, stderr)
    cfg := gin.DefaultConfig()
    if err := gin.WithTelemetry(tel)(&cfg); err != nil { /* ... */ }

    parserImpl, err := resolveParser(*parser) // returns error with build-tag hint if simd unavailable
    if err != nil { /* ... */ }

    b, err := gin.NewBuilder(cfg, *numRGs, gin.WithParser(parserImpl))
    // stream JSONL → AddDocument (round-robin rgID = lineno % numRGs)
    // Finalize, then writeIndexInfo / or JSON emit.
}
```

Register in `main` (`cmd/gin-index/main.go:30`) adjacent to existing dispatches.

---

## Data Flow

### Modified Document Ingestion Flow (v1.1)

```
AddDocument(docID, jsonDoc)
    │
    ▼
b.parser.Parse(jsonDoc, pos, sink=b)            ← NEW seam
    │    (stdlib parser: same json.Decoder.UseNumber() logic as v1.0)
    │    (simd parser:   pure-simdjson Doc/Element traversal)
    ▼
sink.Stage*(state, path, ...)                   ← calls existing private methods
    │    • stageScalarToken / stageMaterializedValue / stageJSONNumberLiteral
    │    • stageCompanionRepresentations (transformers)
    ▼
mergeDocumentState(docID, pos, exists, state)   ← unchanged
    │
    ▼
telemetry.Event(ctx, Debug, "builder.document_added", ...)  ← NEW emission
```

### Modified Query Flow (v1.1)

```
Evaluate(predicates)
    │
    ▼
for each predicate:
    evaluatePredicate(p)  ← unchanged dispatch
         │
         ├─► evaluateEQ → bloom check → string/adaptive index lookup
         │       │
         │       └─► telemetry.Event(..., "query.bloom_hit" | "query.bloom_miss")
         │
         ├─► evaluateRegex → trigram literal extraction
         │       │
         │       └─► telemetry.Event(..., "query.trigram_short_circuit")
         │
         └─► telemetry.Event(..., "query.predicate_evaluated", elapsed_ns=...)
    │
    ▼
result = intersect all
    │
    ▼
telemetry.Event(ctx, Info, "query.row_groups_pruned", matched=..., total=...)
```

### New Experimentation Flow

```
JSONL file
    │  line-by-line (bufio.Scanner with larger buffer than default 64 KiB)
    ▼
for lineno, line := range scan(file):
    builder.AddDocument(pos=lineno % numRGs, line)
    │  (parser and telemetry seams engaged as configured)
    ▼
builder.Finalize() → GINIndex
    │
    ▼
writeIndexInfo(stdout, idx)   ← reuses cmd/gin-index/main.go:445
```

---

## Integration Points

### Modified Files

| File | Change | Why |
|------|--------|-----|
| `builder.go:115` (`NewBuilder`) | Add `b.parser` default assignment | Parser seam default |
| `builder.go:287` (`AddDocument`) | Replace `parseAndStageDocument` call with `b.parser.Parse(..., b)` | Parser dispatch |
| `builder.go:322` (`parseAndStageDocument`) | Move body into `stdlibParser.Parse`; leave a thin wrapper for internal callers | Preserves behaviour |
| `builder.go:Finalize` (`:1129`) | Add telemetry event at end | Observability |
| `query.go:36` (`Evaluate`) | Thread `ctx` (add `EvaluateCtx` overload) and emit per-predicate + final events | Observability |
| `query.go:52` (`evaluatePredicate`) | Per-operator emissions hook here or inside each evaluator | Observability |
| `serialize.go:Encode/Decode` | Emit serialize events + hook zstd-level into info attrs | Observability |
| `cmd/gin-index/main.go:30` (main dispatch) | Add `experiment` case | New CLI verb |
| `gin.go:648` (`DefaultConfig`) | Add `telemetry: NoopTelemetry{}` default | Zero-cost default |
| `gin.go:367` (`ConfigOption`) | Add `WithTelemetry` | Config seam |

### New Files

| File | Purpose |
|------|---------|
| `parser.go` | `Parser` interface, `ParserSink` interface, `stdlibParser` struct (no build tag) |
| `parser_simd.go` | `simdParser` struct, `NewSIMDParser()` — build tag `//go:build simdjson` |
| `parser_parity_test.go` | Runs the full builder test corpus through both parsers (skips simd without tag) |
| `telemetry.go` | `Telemetry` interface, `Level`, `Attr`, event-name constants, `NoopTelemetry` |
| `telemetry_slog.go` | `NewSlogTelemetry(*slog.Logger) Telemetry` adapter (stdlib only) |
| `telemetry/otel/` (sub-pkg) | Optional OTel adapter — keeps OTel out of core `go.mod` |
| `cmd/gin-index/experiment.go` | `cmdExperiment` + `runExperiment` |
| `examples/experiment/main.go` | Example wiring for the new CLI verb |

### External Services / Runtime

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| `pure-simdjson` shared library | `purego` at runtime | Requires `libpure_simdjson.{so,dylib,dll}` at a discoverable path. Set `PUREJSON_LIBRARY` env or ship in `libs/` (mirror go-wand). **Not loaded unless `-tags simdjson` build + `WithParser(NewSIMDParser())`.** |
| `log/slog` (stdlib) | Direct via `telemetry_slog.go` adapter | No new module deps |
| `go.opentelemetry.io/otel` | Separate sub-package `telemetry/otel` | Keeps core `go.mod` clean for consumers who don't want OTel |

---

## Build Order (phase dependencies)

The three features are not equally independent. Proposed order:

1. **Phase A — Parser seam (no simd yet).** Introduce `Parser`/`ParserSink` interfaces + `stdlibParser` extraction. Zero behaviour change; parity tests must be green against the existing test suite. **Unblocks** Phases B and C. **Does not block telemetry.**
2. **Phase B — Telemetry seam.** Introduce `Telemetry`/`NoopTelemetry` + `WithTelemetry` + `slog` adapter + the ~10 emission sites listed above. Independently shippable. **Can run in parallel with Phase A once A's interface is merged**, but benefits from A's `Parser.Name()` being available as an attr.
3. **Phase C — SIMD parser implementation.** `parser_simd.go` behind `//go:build simdjson` + benchmarks comparing against the stdlib baseline on corpora seeded from SEED-001 (simdjson example datasets). **Requires Phase A merged.** Needs a distribution decision (vendored library vs. user-provided path) — flag for PITFALLS.
4. **Phase D — Experimentation CLI.** `cmd/gin-index/experiment.go` reusing Phase B's telemetry (via `--log-level` flag) and Phase A's parser selector (via `--parser` flag; SIMD option gated by build tag). **Requires B and A merged, benefits from C.**

Dependency graph:

```
A (parser seam) ──┬──► C (simd impl)
                  │
                  └──► D (experiment CLI) ◄──── B (telemetry)
                                          ◄──── C (optional)

B (telemetry)  ─────────────────────────► D
```

---

## Anti-Patterns

### Anti-Pattern 1: Making SIMD the default

**What people do:** Set `simdParser` as the default when `-tags simdjson` is present, or worse, drop stdlib entirely.
**Why it's wrong:** Consumers upgrading `ami-gin` would silently acquire a Rust shared-library runtime dependency. Breaks "no new dependencies by default" expectation.
**Do this instead:** Default remains `stdlibParser`. SIMD must be opted in twice: build tag AND `WithParser(NewSIMDParser())`. Document the library-loading story in the README next to the opt-in example.

### Anti-Pattern 2: Variadic `...any` telemetry attributes

**What people do:** Design `Event(name string, attrs ...any)` slog-style.
**Why it's wrong:** Every call allocates a slice even when telemetry is disabled, unless every call-site is wrapped in `if t.Enabled(...)`. Easy to miss sites → hot-path regression.
**Do this instead:** Use typed `Attr` structs and a variadic `...Attr` parameter (still allocates a slice, but typed), **always** wrap emission sites in `if t.Enabled(level)` — the `Enabled` return type is `bool`, easily inlined for `NoopTelemetry`.

### Anti-Pattern 3: Threading `context.Context` through everything

**What people do:** Add `ctx` to `AddDocument`, `Evaluate`, `Finalize`, `Encode`, `Decode` signatures.
**Why it's wrong:** Breaking API change for a library that explicitly forbids API churn (PROJECT.md constraint: "Avoid gratuitous API churn").
**Do this instead:** Add **new** `*Ctx` variants (`AddDocumentCtx`, `EvaluateCtx`) that accept `ctx`. The old methods delegate with `context.Background()`. Telemetry remains functional without a caller-supplied context (default to `context.Background()` internally).

### Anti-Pattern 4: A free-standing `cmd/gin-experiment` binary

**What people do:** Ship a second binary to keep the Parquet CLI "clean."
**Why it's wrong:** Duplicates `parsePredicate`, `writeIndexInfo`, `formatPathInfo`, `describeTypes`. Two binaries to version, test, release.
**Do this instead:** One subcommand in `cmd/gin-index/main.go`. The CLI's verb surface is already mixed (build/query/info/extract); `experiment` fits.

### Anti-Pattern 5: Making the stdlib parser a wrapper around the SIMD one

**What people do:** Invert the dependency so `stdlibParser` delegates to `simdParser` when the tag is set.
**Why it's wrong:** Default consumers become indirectly coupled to the simdjson shared-library contract. Build-tag discipline evaporates.
**Do this instead:** `stdlibParser` is the original `parseAndStageDocument` logic, **exactly**. `simdParser` is an independent implementation in a tag-gated file.

---

## Scaling Considerations

| Scale | Architectural Adjustments |
|-------|--------------------------|
| Small (thousands of docs/file) | Stdlib parser is more than adequate; SIMD wins are <1.5× on small docs due to shared-library call overhead. Default path is the right path. |
| Medium (100k-1M docs/file) | SIMD parser becomes worthwhile; measured win expected 3-8× per simdjson's published numbers on long nested docs. Telemetry at `Info` only, `Debug` disabled. |
| Large (10M+ docs, bulk rebuilds) | Telemetry MUST be `Warn` or `Error` only on hot paths — event emissions even via `NoopTelemetry` add a predictable branch mispredict cost under PGO; guard all emission sites with `if t.Enabled(level)` check. |

### Scaling Priorities

1. **First bottleneck (hot path):** `json.Decoder.UseNumber()` token-by-token parsing — SIMD parser addresses this directly.
2. **Second bottleneck (observability overhead):** Attribute allocation at emission sites — `Attr` struct + `Enabled` gating addresses this.
3. **Third bottleneck (query-side):** Telemetry emission at `query.go:Evaluate` if callers log `Debug`; mitigated by Noop default.

---

## Sources

- Existing code: `builder.go`, `query.go`, `gin.go`, `cmd/gin-index/main.go` (read directly, current tree on branch `gsd/phase-12-milestone-evidence-reconciliation`).
- [pure-simdjson — amikos-tech/pure-simdjson](https://github.com/amikos-tech/pure-simdjson) — `parser.go`, `element.go`, `iterator.go`, `.gitmodules`, `Cargo.toml` inspected via `gh api`.
- [go-wand — amikos-tech/go-wand](https://github.com/amikos-tech/go-wand) — `pkg/logging/logger.go` and `pkg/telemetry/telemetry.go` inspected directly; adopted `Level`/`Enabled`/noop-default shape.
- [go-logr/logr](https://github.com/go-logr/logr) — cross-reference for the `Enabled()`-gate + level-aware interface pattern.
- [simdjson-go (minio)](https://github.com/minio/simdjson-go) — confirms the int64/uint64/float64 distinction simdjson preserves; relevant to exact-int contract preservation in the `simdParser` → `stageJSONNumberLiteral` path.
- `CLAUDE.md` (project architecture section) — confirms flat-package, functional-options, builder-then-immutable-index invariants.
- `.planning/PROJECT.md` — confirms v1.1 milestone scope, seed inclusion (SEED-001), constraints on API stability.

---
*Architecture research for: v1.1 Performance, Observability & Experimentation milestone*
*Researched: 2026-04-21*
