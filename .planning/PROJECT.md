# GIN Index

## Current State

- **Shipped:** `v1.0` Query & Index Quality (2026-04-21); `v1.1` Performance, Observability & Experimentation (functionally complete 2026-04-22, PRs #29 and #30 merged); `v1.2` Ingest Correctness & Per-Document Isolation (2026-04-27)
- **Tag:** `v1.0` on `main`; `v1.2` created during milestone close
- **Scope delivered (v1.0):** canonical JSONPath hot path, explicit-number builder ingest, adaptive high-cardinality indexing, additive derived representations, v9 compact serialization, real-corpus benchmarking, and a reconciled milestone evidence chain
- **Scope delivered (v1.1):** pluggable Parser interface + parity harness, observability seams (Logger/Telemetry/Signals with slog and stdlib adapters), and a new `gin-index experiment` JSONL CLI
- **Library size:** ~25,500 LOC Go, 12 operators, 13 built-in transformers (+3 CIDR/subnet helpers), Parquet + S3 integrations
- **Current milestone:** v1.3 Performance Evidence & Positioning — Phases 19-25 planned from backlog and SEED-001

## Current Milestone: v1.3 Performance Evidence & Positioning

**Goal:** Convert the backlog and SEED-001 into a prioritized milestone that improves user-facing positioning and contributor safety first, then builds realistic benchmark evidence before adding any performance API or optimization.

**Target themes:**
- **User-facing positioning first** — clarify that GIN Index supports row-level pruning when callers choose `rg=1`, while preserving the pruning-first product scope and avoiding row-level storage/search claims.
- **Contributor safety and maintainability** — add opt-in local pre-push quality gates and close low-risk Phase 06 clarity items before deeper performance work.
- **Realistic benchmark foundation** — activate SEED-001 into governed fixture infrastructure with offline smoke fixtures and clear license/size policy.
- **Measure before optimizing** — profile encode CPU, `NormalizePath`, bloom hashing/allocation, and wildcard staging before adding knobs or optimizations.
- **Implementation only after evidence** — add `WithEncodeStrategy` and selected ingest optimizations only if profiling justifies the extra API or internal complexity.

**Deferred beyond v1.3:**
- SIMD parser implementation and validation — moved to v1.4 preview until upstream LICENSE, version tag, and shared-library distribution blockers are resolved.
- `ValidateDocument` dry-run API — remains future scope until there is a concrete consumer.

## What This Is

GIN Index is a Generalized Inverted Index for JSON data, designed for row-group pruning in columnar storage (Parquet). It enables fast predicate evaluation to determine which row groups MAY contain matching documents — filling the gap between a full scan and standing up a database.

As of `v1.1`, the library has a canonical hot-path lookup, exact-int numeric semantics, adaptive high-cardinality string pruning, queryable derived representations alongside raw indexing, compact prefix-encoded serialized layout, a pluggable parser seam, backend-neutral observability, and a streaming JSONL experimentation CLI.

## Core Value

Material pruning quality and hot-path efficiency gains without turning the library into a heavyweight database or document store.

## Project Priorities

In order: **correctness → usefulness → performance**. A perf bottleneck only gets pulled forward when it blocks correctness or usefulness; otherwise perf items live in the 999.x backlog gated by profiling data (see 999.5 precedent).

## Requirements

### Validated

- ✓ Core index structures (string, numeric, null, trigram, bloom, HLL) — existing
- ✓ Query evaluation with 12 operators (EQ, NE, GT, GTE, LT, LTE, IN, NIN, IsNull, IsNotNull, Contains, Regex) — existing
- ✓ Binary serialization with zstd compression — existing
- ✓ Field transformers for pre-index value transformation — existing
- ✓ CLI tool for Parquet file operations — existing
- ✓ Property-based tests and benchmarks — existing
- ✓ MIT LICENSE, public module path, and release automation — completed in `v0.1.0`
- ✓ Deserialization hardening and CI/security workflows — completed in `v0.1.0`
- ✓ Canonical supported JSONPath lookup and constant-time path resolution — validated in Phase 06
- ✓ Reduce builder ingest cost and preserve numeric intent during parsing/indexing — validated in Phase 07: builder-parsing-numeric-fidelity
- ✓ Replace all-or-nothing bloom-only fallback with adaptive high-cardinality hybrid indexing — validated in Phase 08: adaptive-high-cardinality-indexing
- ✓ Support raw-plus-derived index representations instead of transformer replacement only — validated in Phase 09: derived-representations
- ✓ Compact serialized path and term dictionaries using the existing prefix-compression direction — validated in Phase 10: serialization-compaction
- ✓ Pluggable Parser seam with `stdlibParser` default — validated in Phase 13
- ✓ Observability seams (Logger/Telemetry/Signals, slog/stdlib adapters, frozen attribute vocabulary, `EvaluateContext`/`BuildFromParquetContext`) — validated in Phase 14
- ✓ Streaming JSONL `experiment` CLI subcommand with summary, predicate test, JSON mode, sample/error-tolerant flags — validated in Phase 15
- ✓ AddDocument atomicity with validator-backed infallible merge, `tragicErr` recovery, marker enforcement, and full-vs-clean encoded property coverage — validated in Phase 16
- ✓ Unified ingest failure-mode taxonomy with `IngestFailureMode`, parser/numeric config knobs, v9 transformer metadata compatibility, whole-document soft skips, changelog migration note, and deterministic failure-modes example — validated in Phase 17

### Active

- **v1.3 Performance Evidence & Positioning.** POS-01..02, QG-01, CLAR-01, DATA-01..03, PROF-01..05, ENC-01..03, and ING-01..03 are planned across Phases 19-25.

### Out of Scope

- Full row-level binary JSON or `VARIANT`-style document storage
- BM25, document scoring, or any ranked retrieval semantics
- Distributed serving, sharding, or a long-running query service
- A new composite query DSL beyond the current predicate model
- Multi-index merge across files in this milestone
- `ValidateDocument` dry-run API in v1.2 — deferred to a future milestone with a real consumer

## Context

- `v0.1.0` is tagged on `main`; the OSS launch milestone is complete enough to move on
- Phase 06 completed canonical path lookup, decode parity guards, and fixed-width benchmark coverage for EQ, CONTAINS, REGEX, and direct path lookup
- Phase 07 completed the streaming JSON ingest path and explicit numeric-fidelity handling
- Phase 08 completed adaptive high-cardinality string indexing with exact hot-term pruning, bounded tail fallback, and benchmark evidence
- Phase 09 completed additive derived representations with explicit alias routing, metadata round-trip, and public example coverage
- Phase 10 completed serialization compaction for path and term dictionaries with explicit format-version coverage
- Phase 11 completed real-corpus prefix-compression benchmarking and the final compaction recommendation
- Phase 12 reconciled missing Phase 07/09 verification artifacts, aligned the requirements ledger, and refreshed the v1.0 milestone audit to a passed state
- Phase 13 completed the parser seam and always-on parity harness; residual benchmark noise was accepted as documented in `.planning/phases/13-parser-seam-extraction/13-SECURITY.md`
- Phase 14 completed the observability seams and migrated the `adaptiveInvariantLogger` to the unified `Logger` interface
- Phase 15 completed the `gin-index experiment` JSONL subcommand
- v1.1 functionally complete 2026-04-22 (PRs #29 and #30); not formally closed via `/gsd-complete-milestone` but advanced into v1.2
- v1.2 opened 2026-04-23 with brainstorming-locked design: validate-before-mutate atomicity strategy, Lucene per-document contract target, deliberate `TransformerFailureMode` → `IngestFailureMode` rename, `IngestError.Value` not redacted by library
- Phase 16 completed AddDocument atomicity on 2026-04-23: ordinary public failures are non-tragic, failed documents are isolated by encoded-byte property tests, and marker checks enforce the validator/merge contract locally and in CI
- Phase 17 completed failure-mode taxonomy unification on 2026-04-23: public `IngestFailureMode` replaces the old transformer-only taxonomy, parser/numeric/transformer soft modes skip whole documents without durable mutation, v9 transformer wire tokens stay compatible, and the hard-vs-soft example is regression-tested
- Phase 18 completed structured `IngestError` + CLI integration on 2026-04-24: public structured errors cover parser/transformer/numeric/schema hard document failures, hard-ingest sites are guarded by behavior and AST tests, and the experiment CLI reports grouped structured failures in text and JSON modes
- v1.2 shipped and archived on 2026-04-27 with 8/8 requirements complete and milestone archives in `.planning/milestones/`
- v1.3 planned on 2026-04-27 from backlog and SEED-001. It prioritizes row-level pruning messaging, quality gates, realistic benchmark data, profiling, and measurement-backed performance work. SIMD parser work is deferred to v1.4 preview due unresolved external dependency blockers.
- Field transformers now support raw-plus-derived companion representations with explicit alias routing
- Prefix-compressed path and term dictionary encoding is now part of the shipped serialized format, with real-corpus impact documented in Phase 11

## Constraints

- **Project priorities (in order):** correctness → usefulness → performance. A perf bottleneck only gets pulled forward when it blocks correctness or usefulness.
- **Preserve pruning-first scope**: this remains a pruning index, not a row-level search engine
- **Protect correctness**: new optimizations must not introduce false negatives in row-group selection
- **Benchmark-backed changes**: hot-path and size claims must be supported by benchmarks or fixture-based measurements
- **Explicit format evolution**: any serialized format change must have clear version behavior and tests
- **Avoid gratuitous API churn**: prefer additive configuration and compatibility where practical; deliberate breaking changes (such as v1.2's `TransformerFailureMode` → `IngestFailureMode` rename) must be flagged in the CHANGELOG with a migration note

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Continue phase numbering at 06 | Preserve roadmap continuity after the completed `v0.1.0` milestone | Done |
| Prioritize low-risk/high-signal wins first | Path lookup and build-path fixes should land before structural index changes | Done |
| Make adaptive high-cardinality indexing frequency-driven | Hot values are where exact bitmaps recover the most pruning value | Done |
| Derived indexing augments raw indexing | Raw semantics remain available while optimized representations stay queryable | Done |
| Leave serialization compaction until last | Compaction should follow functional changes so the encoded layout stabilizes once | Done |
| Extract parser seam as a pure refactor before SIMD work | Land the seam in v1.1; allow SIMD to land in a later milestone without touching builder internals | Done |
| Adopt validate-before-mutate atomicity (Strategy C) for v1.2 | Smallest diff that delivers the Lucene per-document contract; leverages existing two-phase architecture | Done in Phase 16 |
| Rename `TransformerFailureMode` → `IngestFailureMode` (breaking) | Clarity over convenience; one mental model across parser/transformer/numeric layers | Done in Phase 17 |
| Renumber SIMD work out of v1.3 to v1.4 preview | v1.3 now focuses on backlog/benchmark readiness while SIMD remains blocked on upstream dependency decisions | Planned |
| Prioritize evidence before performance APIs in v1.3 | Avoid speculative knobs or optimizations; benchmark and profile first, then implement only justified changes | Planned |
| Move SIMD work to v1.4 preview | External `pure-simdjson` blockers are unresolved; v1.3 can still improve performance readiness with datasets and profiling | Planned |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition**:
1. Requirements validated? Move them to Validated with the phase reference
2. New constraints or tradeoffs discovered? Add them to Context or Constraints
3. Any scope cuts? Move them to Out of Scope with a reason
4. Any durable design choice? Add it to Key Decisions

**After each milestone**:
1. Re-check the core value against what shipped
2. Archive completed requirements into the next milestone's baseline
3. Refresh Context to reflect the new starting point

---
*Last updated: 2026-04-27 — v1.3 Performance Evidence & Positioning planned from backlog and SEED-001.*
