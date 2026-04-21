# Project Research Summary — v1.1 Performance, Observability & Experimentation

**Project:** GIN Index (`github.com/amikos-tech/ami-gin`)
**Domain:** Additive milestone on an existing pure-Go pruning-index library (builder → immutable index → query)
**Researched:** 2026-04-21
**Confidence:** HIGH (existing code + two sibling repos `pure-simdjson` and `go-wand` inspected directly; OTel/slog patterns well-documented)

## TL;DR

- **Three additive themes, three seams:** a `Parser` interface (SIMD opt-in), a `Telemetry`/`Logger` interface (noop by default), and a new `experiment` subcommand in the existing CLI. None touches query correctness or the v1.0 exact-int contract when disabled.
- **Core integration pattern is "lift from go-wand":** `go-wand`'s `pkg/logging` (backend-neutral `Logger` with `Enabled(Level) bool`) and `pkg/telemetry` (`Signals` container carrying OTel providers, never global mutation) are proven in-house templates and should be adopted near-verbatim. Phase 07 (PR #114) and Phase 08 (PR #115) are the literal blueprints.
- **SIMD is not "pure Go":** `amikos-tech/pure-simdjson` is a `purego` FFI binding to a Rust-built C shared library. Zero CGo, yes — but `libpure_simdjson.{so,dylib,dll}` must be present at runtime. Must be gated behind `//go:build simdjson` AND explicit `WithParser(NewSIMDParser())` opt-in to preserve v1.0's zero-dependency default.
- **Top blockers to resolve before Phase 1 starts:** (1) `pure-simdjson` has **no LICENSE file** and **no tags**, (2) shared-library distribution mechanism is undecided (vendor vs. cargo-build vs. user-provided path), (3) integer-fidelity parity test must land with the SIMD PR — simdjson-family parsers silently demote integers that overflow `uint64` to float64.
- **Phase order is settled across all four research dimensions:** Parser-seam refactor (no behavior change) → Telemetry seam (independent) → SIMD impl (depends on parser seam) → Experiment CLI (consumes the other two). Observability can run parallel with the parser-seam refactor once the interface is merged.

---

## Key Findings

### Recommended Stack Additions

All v1.0 dependencies (Go 1.25.5, roaring/v2, xxhash/v2, klauspost/compress, parquet-go, ojg, aws-sdk-go-v2, gopter, pkg/errors) remain unchanged. v1.1 adds:

**SIMD path (opt-in, build-tag gated):**
- `github.com/amikos-tech/pure-simdjson` — pinned to a commit SHA (no tags exist yet; `^0.1.x` is implied by the ABI doc). License file MUST land upstream before we declare the dep.
- `github.com/ebitengine/purego` v0.10.0 — transitive, Apache-2.0. FFI plumbing, no direct import.

**Observability (core imports — interfaces + noop only, pulled into main module):**
- `go.opentelemetry.io/otel` v1.43.0
- `go.opentelemetry.io/otel/metric` v1.43.0 + `metric/noop`
- `go.opentelemetry.io/otel/trace` v1.43.0 + `trace/noop`

**Observability (optional, only in a `pkg/telemetry` sub-package for the `FromEnv` helper):**
- `go.opentelemetry.io/otel/sdk`, `otel/sdk/metric`
- `go.opentelemetry.io/otel/exporters/otlp/otlp{trace,metric}http`

**Experimentation CLI:** zero new dependencies. Stays on stdlib `flag`, `text/tabwriter`, `bufio`, `encoding/json`.

All licenses compatible with ami-gin's MIT (OTel is Apache-2.0, purego is Apache-2.0, upstream simdjson is Apache-2.0 / MIT dual).

### Expected Features (table stakes vs differentiators)

Summarized from FEATURES.md; the full matrix is there.

**Theme 1 — SIMD JSON (must-have):**
- Parser interface + `encoding/json.UseNumber` default adapter (TS-SIMD-1/2)
- `pure-simdjson` adapter preserving exact-int semantics (TS-SIMD-3)
- Parser-choice-invariant behavior: identical path set, types, null bitmaps across parsers (TS-SIMD-4)
- Benchmarks vs baseline on SEED-001 corpus (TS-SIMD-6)

**Theme 2 — Observability (must-have, directly mirrors go-wand):**
- Backend-neutral `Logger` interface with `Enabled(Level) bool` + `Log(Level, msg, kv...)` (TS-OBS-1)
- Noop default (TS-OBS-2), `Enabled` gate before expensive payload assembly (TS-OBS-3)
- `slog` adapter in its own sub-package (TS-OBS-5); optional `zap` adapter only on request (TS-OBS-6)
- Boundary-level `WithXxxLogger` option carriers — no global state (TS-OBS-7)
- Published structured-key vocabulary + safe-metadata policy (TS-OBS-8/9)

**Theme 2 — Observability (differentiator):**
- `Signals` container carrying `TracerProvider` + `MeterProvider` + `Shutdown` (D-OBS-1/2)
- `FromEnv(ctx, serviceName) (Signals, error)` OTLP bootstrap (D-OBS-3) — CLI-only
- Frozen operation + metric + error-type vocabulary (D-OBS-4/5/7)
- Context-aware API variants `EvaluateContext`, `BuildFromParquetContext` (D-OBS-6)

**Theme 3 — Experimentation CLI (must-have):**
- New `experiment` subcommand (TS-CLI-1), synthetic row-group assignment (TS-CLI-2)
- Per-path summary table: types, cardinality estimate, mode, bloom occupancy, promoted hot terms (TS-CLI-3)
- Build stats, optional `-o out.gin` write (TS-CLI-4/5)
- Streaming JSONL, bounded memory, error-tolerant mode behind a flag (TS-CLI-6/7)

**Theme 3 — Experimentation CLI (differentiator):**
- Inline predicate tester (`--test '$.status = "error"'`) showing pruning ratio (D-CLI-1)
- JSON output mode for piping into jq/CI (D-CLI-6)
- Sample-mode flag `--sample N` (D-CLI-4)

**Defer past v1.1:** REPL/TUI, parallel builder shards, projection filter (D-CLI-5), two-file diff (D-CLI-3), second-parser shim (D-SIMD-4), auto-color output.

### Integration Map (concrete file:function touchpoints)

Build order and exact surgery sites (from ARCHITECTURE.md):

**Phase A — Parser seam (zero behavior change, pure refactor):**
- New file `parser.go` — `Parser` interface, `ParserSink` interface, `stdlibParser` struct
- `builder.go:115` (`NewBuilder`) — default `b.parser = stdlibParser{}` after options loop
- `builder.go:287` (`AddDocument`) — replace direct `parseAndStageDocument` call with `b.parser.Parse(..., b)`
- `builder.go:322` (`parseAndStageDocument`) — body becomes `stdlibParser.Parse` verbatim
- New `parser_parity_test.go` — runs existing corpus through both parsers

**Phase B — Telemetry seam (independent of SIMD, can run parallel with A after A's interface merges):**
- New files `telemetry.go` (interface + `NoopTelemetry` + event constants), `telemetry_slog.go` (stdlib adapter)
- `gin.go:648` (`DefaultConfig`) — `telemetry: NoopTelemetry{}` default
- `gin.go:367` (`ConfigOption`) — add `WithTelemetry`
- `builder.go:Finalize` (`:1129`) — emit `builder.finalize_completed`
- `builder.go:763` (merge poison branch) — emit `builder.merge_poisoned`
- `query.go:36` (`Evaluate`) — add `EvaluateCtx` overload, per-predicate + final events
- `serialize.go:Encode/Decode` — emit serialize events, bytes + level + elapsed
- Optional sub-package `telemetry/otel/` — wraps a `tracer.Tracer` + `metric.Meter` in the `Telemetry` interface
- **Debt migration:** the existing `adaptiveInvariantLogger *log.Logger` at `query.go:17` is routed through the new interface; no dual convention

**Phase C — SIMD implementation (requires A merged):**
- New file `parser_simd.go` with `//go:build simdjson` — wraps `purejson.Parser` / `Doc` / `Element`
- MUST stage via `ParserSink.StageJSONNumber` / `StageNativeNumeric` so exact-int classification stays in `builder.go`'s classifier, not in the SIMD walker
- Distribution: ship `libpure_simdjson.{so,dylib,dll}` in a `libs/` directory following go-wand's pattern OR require `PUREJSON_LIBRARY` env
- CI matrix: add `-tags simdjson` job on linux/amd64 + linux/arm64 + darwin/arm64 + windows/amd64, all with `CGO_ENABLED=0`

**Phase D — Experiment CLI (requires A+B, benefits from C):**
- New file `cmd/gin-index/experiment.go` — `cmdExperiment` / `runExperiment`
- `cmd/gin-index/main.go:30` — register `experiment` case in dispatch
- Reuses `writeIndexInfo`, `formatPathInfo`, `describeTypes`, `parsePredicate`
- Flags: `--rgs N`, `--parser stdlib|simd`, `--log-level off|info|debug`, `--json`, `--test '$...'`, `--sample N`, `--on-error continue|abort`

### Watch Out For (top pitfalls)

Consolidated from PITFALLS.md — ordered by blast radius.

1. **SIMD silently demotes large integers to float64 (Pitfall #1, CRITICAL).** simdjson classifies numeric tokens by decoded value fit, not source text. Integers that overflow `uint64` return as `float64` with no overflow flag, erasing v1.0's BUILD-03 contract. **Prevention:** a `TestSIMDParserPreservesExactInt64` parity test on the Phase 07 corpus MUST land in the same PR as the SIMD dependency. Route SIMD values through `ParserSink.StageJSONNumber` (with raw source text) so the existing classifier in `builder.go:598-750` stays the single source of truth.
2. **SIMD breaks CGo-free / ARM64 cross-compile (Pitfall #2, CRITICAL).** Though `purego` avoids the CGo toolchain, the shared library must exist on every target platform. **Prevention:** CI matrix on amd64/arm64 × linux/darwin/windows × CGO=0 must land in Phase C. Verify with `go list -deps -f '{{.CgoFiles}}'`.
3. **Observability allocates on the disabled hot path (Pitfall #4, CRITICAL).** The ergonomic `logger.Info("msg", "k", v)` form allocates even when handler is nil. At query rates (millions of RG decisions), this turns "zero-cost when disabled" into a measurable regression. **Prevention:** use typed `Attr` structs, `...Attr` variadic (not `...any`), always wrap emission sites in `if t.Enabled(level)`, assert 0 allocs/op in `BenchmarkEvaluateDisabledLogging`.
4. **Forcing `*slog.Logger` into the public API (Pitfall #5, CRITICAL).** Couples consumers to stdlib slog, forces existing `adaptiveInvariantLogger *log.Logger` users to maintain two conventions. **Prevention:** public API exposes only the local `Logger` interface. Ship `slogadapter`, `stdadapter` in sub-packages. Migrate `adaptiveInvariantLogger` in the same phase — one convention.
5. **Telemetry emits predicate values (PII risk, Pitfall #6).** Values at INFO leak user data into operators' logs. **Prevention:** values at DEBUG only; span/metric attributes follow a closed allowlist (operation, predicate_op, path_mode, status, error.type); hard ban on path values, JSON field values, query text, doc IDs, row-group IDs, term IDs.
6. **Tracing spans explode on the hot path (Pitfall #7).** One span per predicate per RG = 50k spans per query. **Prevention:** coarse boundary spans only (Evaluate, Encode, Decode, BuildFromParquet); predicate outcomes as span events on the parent. `BenchmarkEvaluateWithTracer` within 0.5% of `BenchmarkEvaluateNoTracer` when tracer is off.
7. **CLI scope-creeps into a REPL / TUI (Pitfall #8).** Charter: "accept JSONL on stdin or a path, build an index with default config, emit a structured summary." Anything beyond that is a separate phase. **Prevention:** JSON output is the stable default, plain text opt-in; no color, no TTY detection, no interactive mode in v1.1; flag list reviewed against charter at phase close.

---

## Open Questions (resolve at phase-0 or before)

1. **`pure-simdjson` LICENSE file** — GitHub API returns `"license":null`. Must be added (MIT to match ami-gin) before the dependency is declared stable. **Blocker for Phase C merge.**
2. **`pure-simdjson` version tag** — no tags exist; pin to commit SHA with `go mod edit -require=github.com/amikos-tech/pure-simdjson@<sha>` until 0.1.0 ships. Re-pin to the tag once published.
3. **Shared-library distribution** — three candidates: (a) vendor prebuilt `.so`/`.dylib`/`.dll` in `libs/` mirroring go-wand, (b) `cargo build --release` as a release-time step with per-platform artifacts in GitHub Releases, (c) require users to provide `PUREJSON_LIBRARY` env pointing to their own build. Needs a Phase-0 decision before Phase C starts. Leaning toward (a)+(c) combination: ship artifacts for common platforms, fall back to env for others.
4. **OTel sub-package location** — two options: (a) `pkg/telemetry/otel` inside the main module (pulls OTel API types onto every consumer's `go.sum` even if unused), (b) a **separate module** at `github.com/amikos-tech/ami-gin/telemetry/otel` with its own `go.mod`. Option (b) is cleaner but operationally heavier. Recommend (a) since OTel API packages are interface-only with noop fallbacks (verified — no SDK pulled into core).
5. **Tracing support in v1.1 or logging-only?** Research suggests full `Signals` (traces + metrics) since the cost is one interface-call per boundary when noop. Recommend: land logging + metrics + traces together in Phase B, skip context-aware variants (D-OBS-6) to reduce surface area.
6. **Migration of `adaptiveInvariantLogger`** — in-phase (Phase B) with the rest of the telemetry work, not parallel. Keeping dual conventions even briefly creates the exact tech debt PITFALLS #5 warns about.
7. **`EvaluateContext` / `BuildFromParquetContext` — additive variants vs ctx threading** — the additive path avoids API breakage (PROJECT.md "Avoid gratuitous API churn"). Old methods wrap with `context.Background()`. Settle in Phase B.

---

## Proposed Phase Split (feeds the roadmapper)

### Phase A: Parser Seam Extraction (pure refactor)
**Rationale:** Extract the parser boundary with zero behavior change before adding any new parser. Keeps the surgery visible and reviewable.
**Delivers:** `Parser` / `ParserSink` interfaces, `stdlibParser` with current `parseAndStageDocument` logic verbatim, `WithParser(p)` option, parity test harness.
**Addresses:** TS-SIMD-1, TS-SIMD-2, TS-SIMD-4 (parity infrastructure).
**Avoids:** Pitfall #11 (mixing concerns) by making this a standalone, behavior-neutral PR.
**Size:** S (1-2 days). No new dependencies.

### Phase B: Observability Seams (logging + telemetry + slog adapter)
**Rationale:** Can run parallel with Phase A once A's `Parser.Name()` is available as an attribute. Foundational for measuring Phase C's impact and for Phase D's `--log-level` flag. Also the phase where `adaptiveInvariantLogger` migrates.
**Delivers:** `Logger` interface + `NoopLogger`, `slogadapter` + `stdadapter` sub-packages, `Telemetry`/`Signals` container carrying OTel tracer + meter providers, boundary `WithXxxLogger` options, frozen metric + operation vocabulary, `FromEnv` OTLP bootstrap (CLI helper), context-aware API variants.
**Addresses:** TS-OBS-1 through TS-OBS-10, D-OBS-1/2/3/4/5/7.
**Avoids:** Pitfalls #4 (disabled-path allocs — `0 allocs/op` merge gate), #5 (no `*slog.Logger` in public API), #6 (PII allowlist), #7 (coarse spans only).
**Size:** M (1 week). New deps: OTel v1.43.0 API packages + noop.

### Phase C: SIMD Parser Implementation
**Rationale:** Requires Phase A merged; benefits from Phase B (parser-choice events, capability-fallback warnings). Largest risk surface — isolated under a build tag so default consumers are unaffected.
**Delivers:** `parser_simd.go` (build-tagged `//go:build simdjson`), `NewSIMDParser()`, int64/uint64/float64 fidelity parity test, benchmark suite on SEED-001 simdjson example datasets, CI matrix amd64/arm64 × linux/darwin/windows × CGO=0, shared-library distribution artifacts.
**Addresses:** TS-SIMD-3, TS-SIMD-5, TS-SIMD-6, D-SIMD-2 (capability fallback logged via Phase B).
**Avoids:** Pitfalls #1 (exact-int parity test as merge gate), #2 (CI matrix), #3 (`-race` clean; `TestConcurrentBuildersAreIndependent`), #12 (`toolchain` pin in `go.mod`).
**Size:** L (1-2 weeks). New deps: `pure-simdjson` (pinned SHA), `purego` (transitive). **Blocked by:** pure-simdjson LICENSE, distribution decision.
**Research flag:** YES — `/gsd-research-phase` for distribution mechanism and prebuilt-artifact pipeline.

### Phase D: Experimentation CLI
**Rationale:** Consumes A (parser flag) and B (log-level flag). Ships last, lowest risk, most visible to new evaluators. Strict charter to avoid scope creep.
**Delivers:** `experiment` subcommand, per-path summary table (reusing `writeIndexInfo`), build stats, JSON output mode, inline predicate tester, sample-mode, streaming JSONL with bounded memory, error-tolerant mode behind a flag.
**Addresses:** TS-CLI-1 through TS-CLI-7, D-CLI-1, D-CLI-4, D-CLI-6.
**Avoids:** Pitfalls #8 (charter-approved flag list), #9 (`bufio.Reader.ReadBytes` or `Scanner` with sized buffer; streaming; stdin `-`), #10 (zero color).
**Size:** M (3-5 days). No new dependencies.

### Phase Ordering Rationale

- **A → {B, C} → D** is the dependency DAG. B and C can be in flight in parallel after A merges; both must land before D.
- A is a pure refactor with a parity test as the merge gate — smallest possible change, validates the seam abstraction.
- B lands before C so SIMD capability-fallback and parser-selection events have a home; also so Phase C benchmarks can use the telemetry vocabulary.
- C is the highest-risk phase (numeric fidelity, platform portability, shared library distribution) — isolating it behind a build tag means a post-merge rollback of SIMD does not touch A/B/D.
- D ships last because its UX is easier to iterate once the underlying seams are stable.

### Research Flags

Phases needing `/gsd-research-phase` during planning:
- **Phase C (SIMD):** distribution mechanism (vendored vs. cargo-build vs. env), ABI version-check story, prebuilt-artifact pipeline in GitHub Releases. Also: live verification that `pure-simdjson`'s Rust crate handles `TypeBigInt` / `ErrPrecisionLoss` paths we're depending on.

Phases with well-documented patterns (skip deeper research):
- **Phase A (parser seam):** straight refactor of existing `parseAndStageDocument`.
- **Phase B (telemetry):** go-wand PRs #114/#115 are line-by-line templates.
- **Phase D (CLI):** stdlib `flag` + `text/tabwriter` patterns are well-established; charter keeps scope bounded.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | pure-simdjson and go-wand source inspected directly via `gh api`. OTel versions confirmed against go-wand's pinned deps. |
| Features | MEDIUM-HIGH | go-wand PRs #114/#115 are fresh, concrete templates. SIMD parser landscape well-surveyed. CLI UX is a design choice, not an industry template. |
| Architecture | HIGH | Existing-code integration points (builder.go, query.go, serialize.go line numbers) verified. MEDIUM for pure-simdjson lifecycle details (not yet vendored) and go-wand telemetry pattern (to-be-adapted, not copied). |
| Pitfalls | HIGH | SIMD numeric drift and slog allocation traps (multiple sources). MEDIUM for OTel span-explosion figures (single summary, widely cited). LOW for claims specific to `amikos-tech/pure-simdjson` (library surface only reachable via source inspection, not external docs — treat as upstream-unknown until Phase C starts). |

**Gaps to address during planning:**
- `pure-simdjson` public-facing docs are thin; rely on source inspection until 0.1.0 releases with proper README.
- No existing benchmark baseline for the builder hot path with telemetry off — need a `BenchmarkEvaluateDisabledLogging` baseline recorded in Phase B.
- Distribution mechanism for the native library is an unresolved design question blocking Phase C.
- simdjson example datasets (SEED-001) need to be vendored under `testdata/simdjson-examples/` with a NOTICE.md before benchmarks can be reproducible.

---

## Sources

Aggregated from the four research files; full citations in the underlying docs.

- Local code: `builder.go`, `query.go`, `gin.go`, `serialize.go`, `cmd/gin-index/main.go`, `go.mod` (current tree on `gsd/phase-12-milestone-evidence-reconciliation`)
- `github.com/amikos-tech/pure-simdjson` — `parser.go`, `element.go`, `iterator.go`, `library_loading.go`, `Cargo.toml`, `go.mod`, `Makefile` inspected via `gh api`, verified 2026-04-21
- `github.com/amikos-tech/go-wand` — `pkg/logging/{logger.go,noop.go,doc.go}`, `pkg/logging/slogadapter/slog.go`, `pkg/telemetry/{telemetry.go,attrs.go,doc.go,env.go}`, `cmd/go-wand/main.go`, `internal/indexcli/app.go` inspected directly, verified 2026-04-21
- `github.com/amikos-tech/go-wand` PR #114 (logging, merged 2026-04-16) and PR #115 (telemetry, merged 2026-04-17)
- OpenTelemetry Go API v1.43.0 — confirmed from go-wand's pinned deps
- `github.com/minio/simdjson-go` — cross-reference for number parsing and portability pitfalls (issues #13, #30; minio/minio #9003)
- Go `log/slog` docs — `LogAttrs`, `Enabled`, `DiscardHandler` (Go issue #62005)
- `go-logr/logr` — logging interface pattern for libraries
- OpenTelemetry Go best practices — span cardinality (official docs)
- Phase 07 verification — exact-int contract (`.planning/milestones/v1.0-phases/07-builder-parsing-numeric-fidelity/`)
- SEED-001 — simdjson example datasets (`.planning/seeds/SEED-001-simdjson-test-datasets.md`)
- `.planning/PROJECT.md` — v1.1 milestone scope, constraints, seed inclusion

---

*Synthesis for: v1.1 Performance, Observability & Experimentation milestone*
*Synthesized: 2026-04-21*
