# Phase 14: Observability Seams - Context

**Gathered:** 2026-04-22
**Status:** Ready for planning

<domain>
## Phase Boundary

Make index build, query evaluation, and serialization observable through a backend-neutral logging seam and a `Signals`-style telemetry seam, while staying silent and effectively zero-cost when disabled.

This phase includes:
- a repo-owned `Logger` contract plus `slog` and stdlib adapters
- a repo-owned `Signals` container for local tracer/meter providers
- boundary-only instrumentation for build/query/serialize/parquet surfaces
- additive context-aware siblings for the public APIs the roadmap names
- migration of the existing `adaptiveInvariantLogger` into the new logging seam

This phase does not include:
- hot-loop spans or per-row-group tracing
- trace/log correlation inside the core logging seam
- a `zap` adapter
- CLI UX work beyond the reusable telemetry/bootstrap pattern that Phase 15 can consume

</domain>

<decisions>
## Implementation Decisions

### Reference Model And Uniformity

- **D-01:** Phase 14 must align with `/Users/tazarov/experiments/amikos/go-wand` as the canonical observability reference model. Adopt its split seam near-verbatim: a repo-owned, context-free `Logger`; a separate `Signals` container for telemetry; boundary-only instrumentation via a shared helper; CLI-owned `FromEnv(...)`; no trace IDs injected into logs; no global OTel mutation.
- **D-02:** The official adapters in this phase are `logging/slogadapter` and `logging/stdadapter` only. Do not expose `*slog.Logger`, `*log.Logger`, or any OTel SDK/exporter type from the core public API. `zap` remains explicitly out of scope for v1.1.
- **D-03:** Freeze the low-cardinality observability vocabulary in one source file. INFO-level attributes stay on the roadmap allowlist only: `operation`, `predicate_op`, `path_mode`, `status`, `error.type`. Predicate values, path names, doc IDs, row-group IDs, term IDs, and raw user content stay banned at INFO level.
- **D-03a:** `path_mode` is a bounded value enum, not a free-form string. Phase 14 uses the existing `PathMode.String()` labels only: `exact`, `bloom-only`, and `adaptive-hybrid`. Tests should enforce both the key allowlist and the bounded `path_mode` value set where INFO-level attrs are emitted.
- **D-04:** `Parser.Name()` from Phase 13 may be used only in traces or guarded debug-level signals. It must not widen the frozen INFO-level attribute vocabulary.
- **D-04a:** `Parser.Name()` attribution belongs to the build/parquet side of the phase, not the query hot path. Phase 14 must add one explicit build-boundary test proving parser identity is reachable from traces or debug-only observability without appearing in INFO-level log attrs.

### Dependency Boundary And Packaging

- **D-05:** Keep the go-wand telemetry API shape and behavior, but adapt exporter packaging to the stricter `ami-gin` milestone rule: the root `github.com/amikos-tech/ami-gin` module may depend on OTel API/noop packages for `Signals`, but OTLP SDK/exporter bootstrap must live outside the core module boundary so the root `go.mod` does not pick up SDK/exporter deps.
- **D-06:** `FromEnv(ctx, serviceName)` remains a CLI-owned convenience surface, modeled on go-wand. The library itself must not auto-bootstrap telemetry, mutate global providers, or hide exporter lifecycle inside package init or defaults.

### Carrier Shape Across Public APIs

- **D-07:** For build, query, and parquet paths, carry the new logger and telemetry seams through `GINConfig`, not through a separate repo-wide observability object and not through a large go-wand-style explosion of per-boundary options. This fits the existing `ami-gin` API shape better.
- **D-08:** `DefaultConfig()` wires silent defaults: a noop logger plus disabled/noop signals. Library behavior remains quiet unless the caller opts in.
- **D-08a:** Decoded configs must be repaired at decode time, not just tolerated later. `readConfig(...)` (or one shared normalization helper called by it) must restore noop logger + disabled signals before the config is returned to `Decode()`, so decoded indexes are safe even before any boundary helper runs.
- **D-09:** Query-time observability reads from `GINIndex.Config`, which already survives `Finalize()` and `Decode()`. If `idx.Config` is nil, query paths must fall back to silent/noop behavior rather than panic or assume configuration is always present.
- **D-10:** `EvaluateContext` and `BuildFromParquetContext` are additive siblings. Existing `Evaluate` and `BuildFromParquet` remain compatibility wrappers over `context.Background()` and the config-carried observability seams.
- **D-10a:** The S3-backed parquet build path may use a short-lived internal `s3ReaderAt` that stores the caller context and derives timeout child contexts from it for range reads. This is an allowed internal exception to the general "do not store context in structs" guideline because the reader is per-build, never shared, and exists only to compose cancellation into the `io.ReaderAt` contract.

### Serialization Observability

- **D-11:** Raw serialization is the exception to the config-carrier rule. Add additive `EncodeContext` and `DecodeContext` siblings so raw encode/decode can emit coarse observability signals without globals.
- **D-11a:** Raw serialization follows the research Pattern 2 contract exactly: `EncodeContext(ctx, idx, opts ...EncodeOption)`, `EncodeWithLevelContext(ctx, idx, level, opts ...EncodeOption)`, and `DecodeContext(ctx, data, opts ...DecodeOption)`. Old wrappers delegate with `context.Background()`, preserving the existing `EncodeWithLevel` surface instead of bypassing observability for configurable compression.
- **D-12:** Existing `Encode` and `Decode` stay as compatibility wrappers over the new context-aware siblings. Planner/implementation may choose the exact additive signature, but raw serialization must not reintroduce package-global hooks or dual logging conventions.
- **D-12a:** Encode/decode runtime options are observability-only shims, not a second general-purpose config surface. They may carry logger/signals-style runtime inputs for raw serialization, but must not evolve into a duplicate of `GINConfig`.

### Migration And Compatibility

- **D-13:** Remove `SetAdaptiveInvariantLogger(*log.Logger)` in Phase 14 instead of keeping a deprecated compatibility bridge. This repo moves directly to one logging convention; no dual logger state survives the phase.
- **D-14:** The existing adaptive invariant fallback behavior stays the same: invariant violations still fail open to `AllRGs()`. Only the emission path changes, from package-global stdlib logging to the repo-owned logger seam.

### Instrumentation Scope And Performance

- **D-15:** Instrumentation stays boundary-only. Coarse operation boundaries include `Evaluate`, `Encode`, `Decode`, `BuildFromParquet`, and the related parquet/open/build surfaces. Per-predicate details, when emitted, are parent-span events rather than nested spans. Never add per-row-group spans or telemetry in the hot query loop.
- **D-16:** Zero-cost disabled behavior is a merge gate. Disabled logging must stay at 0 allocs/op on the benchmark gate, and disabled tracer wiring must remain within the roadmap's no-regression budget. Expensive debug payload assembly must be guarded before attr construction.
- **D-16a:** The 0.5% disabled-tracer overhead budget must be measured with a noise-controlled median-of-N helper on a normalized runtime (`GOMAXPROCS(1)` plus repeated samples). Do not hard-fail a single wall-clock comparison on generic shared CI; if a strict budget gate is needed, run it on a dedicated normalized path.

### the agent's Discretion

- Exact package and file layout for the new seams, as long as the go-wand split model and the `ami-gin` dependency boundary both hold.
- Exact naming of the new config options and additive context-aware helpers.
- Exact signature design for `EncodeContext` / `DecodeContext`, since raw decode has no pre-existing config carrier.
- Exact boundary helper names, benchmark helper names, and message wording for invariant-violation logs, as long as the frozen vocabulary and no-dual-logger rule are preserved.

</decisions>

<specifics>
## Specific Ideas

- Use `/Users/tazarov/experiments/amikos/go-wand` as the canonical implementation reference for telemetry and logging shape. Deviate only when `ami-gin`'s existing API surface or milestone dependency constraint makes a literal copy incorrect.
- Keep the same separation of concerns as go-wand: logging is context-free and caller-pluggable; telemetry is local-provider based and caller-owned; command-root bootstrap/shutdown is not hidden inside library code.
- Reuse the go-wand CLI ownership pattern in a later phase: bootstrap one shared `Signals` instance at the CLI root, pass it downward once, and shut it down once on exit while keeping command output on `stdout` and diagnostics on `stderr`.
- Because Phase 13 already caches `b.parserName`, build-path observability can expose parser identity in traces/debug signals without touching parser implementations again.

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Requirements And Constraints
- `.planning/ROADMAP.md` §Phase 14 — goal, success criteria, and dependency on Phase 13
- `.planning/REQUIREMENTS.md` §Observability — `OBS-01` through `OBS-08`
- `.planning/PROJECT.md` §Current Milestone / Constraints — additive API preference and benchmark-backed change bar
- `.planning/STATE.md` §Accumulated Context — milestone-level decisions already carried forward into Phase 14

### Research Guidance
- `.planning/research/SUMMARY.md` §Theme 2, §Phase B, and §Open Questions 4-7 — go-wand blueprint, context-aware API notes, and dependency-shape tension
- `.planning/research/FEATURES.md` §Theme 2 — must-have logging/telemetry features plus anti-features
- `.planning/research/PITFALLS.md` — disabled-path allocation trap, public `slog` API trap, and span-explosion guardrails

### go-wand Alignment Blueprint
- `../go-wand/pkg/logging/logger.go` — repo-owned `Logger` contract
- `../go-wand/pkg/logging/noop.go` — noop/default logger semantics
- `../go-wand/pkg/logging/doc.go` — context-free logger rules and adapter split
- `../go-wand/pkg/logging/slogadapter/slog.go` — `slog` adapter behavior
- `../go-wand/pkg/logging/stdadapter/std.go` — stdlib adapter behavior and severity prefixes
- `../go-wand/pkg/telemetry/telemetry.go` — `Signals`, `Disabled()`, and local-provider semantics
- `../go-wand/pkg/telemetry/boundary.go` — shared boundary helper pattern
- `../go-wand/pkg/telemetry/attrs.go` — frozen attr/metric/error-type vocabulary pattern
- `../go-wand/pkg/telemetry/doc.go` — telemetry ownership and explicit non-goals
- `../go-wand/docs/logging.md` — safe metadata policy and logging vocabulary discipline
- `../go-wand/docs/telemetry.md` — CLI ownership, coarse coverage, and correlation stance
- `../go-wand/cmd/go-wand/main.go` — root-owned logger + telemetry bootstrap/shutdown pattern

### Current Code Anchors
- `query.go` — current package-global `adaptiveInvariantLogger` and invariant fallback sites to migrate
- `builder.go` — `GINBuilder.parserName`, already cached in Phase 13 for build-path observability
- `gin.go` — `GINConfig`, `DefaultConfig()`, and `GINIndex.Config` as the selected config carrier
- `serialize.go` — current raw `Encode` / `Decode` boundaries that need additive context-aware siblings
- `parquet.go` — current public parquet build path and the `BuildFromParquetContext` addition point
- `s3.go` — existing internal context/timeouts that public context-aware build paths must compose with
- `cmd/gin-index/main.go` — current CLI root that Phase 15 can align to the go-wand bootstrap/shutdown pattern

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`GINIndex.Config` persistence**: finalized and decoded indexes already carry a config pointer, which makes query-time observability reuse feasible without inventing a second carrier.
- **`GINBuilder.parserName`**: Phase 13 already caches parser identity once at builder construction time. Observability can reuse this without new parser work.
- **Existing S3 timeouts**: `s3.go` already uses internal `context.WithTimeout` wrappers, so public context-aware build APIs can compose with existing network discipline rather than replace it.

### Established Patterns
- **Additive configuration**: this repo consistently prefers additive config/options and compatibility wrappers over breaking signature changes. That supports `EvaluateContext`, `BuildFromParquetContext`, `EncodeContext`, and `DecodeContext`.
- **Top-level serialization functions**: `Encode` and `Decode` are package-level functions, not methods, so raw serialization has no natural config carrier today. That is why serialization needs its own additive context-aware siblings.
- **Nil-config tolerance**: tests and security fixtures frequently construct `NewGINIndex()` directly. Query observability must treat missing config as silent/noop, not as programmer error.

### Integration Points
- **`gin.go`**: extend `GINConfig` and `DefaultConfig()` with logger/signals defaults that remain silent when unset.
- **`query.go`**: replace the package-global invariant logger, add `EvaluateContext`, and wire boundary-only logging/telemetry through `idx.Config`.
- **`parquet.go`**: add `BuildFromParquetContext`; keep the legacy function as a `context.Background()` wrapper.
- **`serialize.go`**: add context-aware raw serialization entry points without introducing package-global observability state.

</code_context>

<deferred>
## Deferred Ideas

- `zap` adapter — explicitly on-demand only, not Phase 14 scope
- Per-predicate nested spans or per-row-group telemetry — explicit non-goal
- Trace/log correlation inside the repo-owned logger seam — caller-owned only
- Broader CLI surface such as `--log-level` flags and root bootstrap wiring — Phase 15 concern, though the go-wand pattern is now the reference
- Broader public S3 context-aware API expansion beyond what Phase 14 needs to satisfy `BuildFromParquetContext`

</deferred>

---

*Phase: 14-observability-seams*
*Context gathered: 2026-04-22*
