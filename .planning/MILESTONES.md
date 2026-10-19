# Milestones

## v1.0 Query & Index Quality (Shipped: 2026-04-21)

**Phases completed:** 7 phases (06-12), 19 plans, 43 tasks
**Known deferred items at close:** 4 (see STATE.md Deferred Items)
**Git tag:** `v1.0`

**Key accomplishments:**

- Canonical JSONPath lookup now resolves supported spelling variants through one stored path name and one immutable hot-path map in fresh and decoded indexes.
- Fixed-width wide-path benchmark proof with shared log-style fixtures, explicit lookup attribution, and EQ/CONTAINS/REGEX spelling variants
- Transactional explicit-number ingest with exact-int path semantics, guarded mixed-mode promotion, and decode-parity regressions
- Reproducible parser-delta benchmarks with an in-repo legacy control and deterministic fixture families for Phase 07
- Adaptive high-cardinality string paths now keep bounded exact hot-term bitmaps and deterministic bucket fallbacks instead of collapsing directly to bloom-only behavior.
- Versioned adaptive serialization, mode-aware `gin-index info`, and README docs now make the adaptive-hybrid high-cardinality behavior explicit end to end.
- Deterministic skewed high-cardinality benchmarks proving adaptive hot-value pruning recovery with direct candidate and encoded-size metrics
- Additive raw-plus-companion staging that preserves public source values, materializes hidden sibling representations, and fails companion derivation explicitly
- Explicit alias routing, deterministic representation introspection, and v7 representation metadata round-trip parity
- Alias-aware docs/examples plus DERIVE-04 acceptance coverage for date, normalized text, and extracted-subfield companions
- v9 ordered-string sections now compact path names, classic terms, and adaptive promoted terms without changing decoded query semantics
- Fail-closed compact-section decoding plus explicit v8 rejection and raw/alias parity checks for the v9 wire format
- Fixture-backed raw/zstd size accounting plus encode, decode, and post-decode query evidence for the v9 compact wire format
- Phase 11 now has a reproducible smoke corpus plus an env-gated real-corpus benchmark family that contrasts structured metadata against text-heavy payloads
- Phase 11 now has pinned smoke, subset, and large benchmark evidence, including the large text-heavy flat/no-win case and its decode-cap behavior
- Phase 11 now closes with an evidence-backed recommendation report and a README workflow that keeps smoke default while documenting the opt-in external corpus path
- Phase 07 proof surface rebuilt with a verified validation audit, a requirement-mapped verification report, and fresh parser/benchmark/full-suite evidence
- Phase 09 now has a current-tree verification report that ties DERIVE-01 through DERIVE-04 to rerun tests, CLI/docs smoke, and runnable example output
- BUILD and DERIVE requirement evidence now points to Phases 07 and 09, and the v1.0 milestone audit reruns cleanly against fresh current-tree command results

---

## v1.1 Performance, Observability & Experimentation (Functionally complete: 2026-04-22)

**Phases completed:** 3 phases (13–15), 10 plans
**Known deferred items at close:** SIMD work (originally v1.2, renumbered to v1.3 phases 19–20 during v1.2 bootstrap)
**Git tag:** _not yet tagged — milestone not formally closed via `/gsd-complete-milestone`_

**Key accomplishments:**

- Pluggable `Parser` interface with a `stdlibParser` default (`json.Decoder.UseNumber()`) plus an always-on parity harness; the JSON-parse boundary is now extracted from the builder without any behavior change, so a SIMD parser can land later without touching builder internals.
- Backend-neutral `Logger` and `Telemetry`/`Signals` observability seams with `slog` and `stdlib log` adapters as separate sub-packages; zero allocations when disabled, ≤0.5% wall-clock overhead with a tracer supplied; the public API never exposes `*slog.Logger` or OTel SDK types directly.
- Frozen INFO-level attribute vocabulary (`operation`, `predicate_op`, `path_mode`, `status`, `error.type`); predicate values, path field names, and doc/RG/term IDs explicitly banned from INFO and asserted by an allowlist test.
- `EvaluateContext` and `BuildFromParquetContext` shipped as additive siblings; the `adaptiveInvariantLogger *log.Logger` migrated to the new `Logger` interface (single convention, no dual-logger state).
- `gin-index experiment` subcommand: JSONL ingest from a file or stdin, per-path summary table (types, cardinality, mode, bloom occupancy, hot terms), inline `--test '<predicate>'` tester, `--json` mode, sample/error-tolerant flags, optional sidecar write — no new dependencies, no REPL, no TUI.

---

## v1.2 Ingest Correctness & Per-Document Isolation (Shipped: 2026-04-27)

**Phases completed:** 3 phases (16-18), 12 plans, 27 tasks including one integration lint deviation  
**Known deferred items at close:** SIMD parser work, `ValidateDocument`, snapshot/restore reserve strategy, bloom allocation cleanup, and per-path wildcard opt-out (see STATE.md Deferred Items)  
**Git tag:** `v1.2`

**Archived artifacts:**

- [`milestones/v1.2-ROADMAP.md`](./milestones/v1.2-ROADMAP.md)
- [`milestones/v1.2-REQUIREMENTS.md`](./milestones/v1.2-REQUIREMENTS.md)
- [`milestones/v1.2-MILESTONE-AUDIT.md`](./milestones/v1.2-MILESTONE-AUDIT.md)

**Key accomplishments:**

- `AddDocument` now has per-document atomicity: ordinary public failures leave the builder usable and indistinguishable from never receiving the failed document.
- Merge-layer mutation is validator-backed and infallible by construction, with local and CI marker checks guarding the contract.
- `poisonErr` was renamed to `tragicErr` and narrowed to internal-invariant or recovered merge-panic cases.
- Public failure configuration is unified under `IngestFailureMode` with hard and soft modes across parser, transformer, and numeric layers.
- Transformer v9 wire compatibility was preserved while moving the public API to the new failure-mode vocabulary.
- Exported `IngestError` carries path, layer, cause, and verbatim value, and can be extracted through `errors.As`.
- `gin-index experiment --on-error continue` now reports grouped structured failures in text and JSON, backed by deterministic 100-line fixture coverage.
- Phase verification passed 8/8, 15/15, and 16/16 must-haves for Phases 16, 17, and 18 respectively.

---

## v1.3 SIMD-First Performance (Shipped: 2026-10-19)

**Phases completed:** 4 phases (19-22), 16 plans, 32 tasks
**Known deferred items at close:** 3 phases (23-25) with 15 requirements, 1 dormant seed, 36 quick-task directories flagged `missing` by `audit-open` (see STATE.md Deferred Items)
**Git tag:** _none for the planning milestone; releases v1.3.0, v1.4.0 and v1.5.0 shipped during it_

**Archived artifacts:**

- [`milestones/v1.3-ROADMAP.md`](./milestones/v1.3-ROADMAP.md)
- [`milestones/v1.3-REQUIREMENTS.md`](./milestones/v1.3-REQUIREMENTS.md)

**Key accomplishments:**

- `19-SIMD-STRATEGY.md` locked the `pure-simdjson` pin, MIT/NOTICE posture, upstream loading delegation, `NewSIMDParser()` behind `//go:build simdjson`, explicit `WithParser` opt-in, and the hard/soft stop policy.
- Governed, deterministic JSONL benchmark fixtures (nested high-cardinality, mixed wildcard arrays, numeric boundaries) drive offline Build, Encode, and Query benchmarks with an opt-in local external tier.
- Build-tagged SIMD adapter with typed scalar routes, exact-int semantics, last-key-wins traversal, default-graph isolation, and native cleanup failures that poison the builder exactly once.
- Byte-identical encoded indexes and equal query results across 12 authored goldens, 4 realistic fixtures and 24 Evaluate cases, plus a five-seed bounded differential fuzz target.
- Five-platform SIMD CI matrix (two required race legs, three advisory legs) with an effective-version native cache, explicit-path replay, dual `govulncheck` call graphs, and the Main ruleset bound to exactly the two required contexts.
- Controlled 80-sample paired benchmark: parity passed, but SIMD was slower on every fixture (geomean +64.57% time, +28.50% Go-heap bytes, +56.12% allocations). Recommendation class **defer** accepted; stdlib stays the default.

**Shipped outside GSD during the milestone:** v1.3.0 (bounded-memory zstd encoder profile, #79), v1.4.0 (uncached bounded-memory profile, #83), v1.5.0 (uniform `IsNull`, #89), plus precise NIN on multi-value row groups (#63, wire v11), the O(2^depth) nested-array staging fix (#54), and regex literal composition and Unicode fold fixes (#68, #70).

### Known Gaps

Deferred unplanned phases, moved to the roadmap backlog at close:

- Phase 23 Row-Level Pruning Positioning: POS-01, POS-02
- Phase 24 Developer Quality Gates & Janitorial Clarity: QG-01, CLAR-01
- Phase 25 Follow-On Profiling & Measurement-Backed Optimizations: PROF-01..05, ENC-01..03, ING-01..03 (ENC partly superseded by the shipped encoder profiles; ING-02/03 remain parked)
- Backlog Phase 999.7 SIMD runtime fallback DX: dropped, moot under the defer decision

---
