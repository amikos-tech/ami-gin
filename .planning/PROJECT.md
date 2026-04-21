# GIN Index

## Current State

- **Shipped:** `v1.0` Query & Index Quality (2026-04-21)
- **Tag:** `v1.0` on `main`
- **Scope delivered:** canonical JSONPath hot path, explicit-number builder ingest, adaptive high-cardinality indexing, additive derived representations, v9 compact serialization, real-corpus benchmarking, and a reconciled milestone evidence chain
- **Library size:** ~25,500 LOC Go, 12 operators, 13 built-in transformers (+3 CIDR/subnet helpers), Parquet + S3 integrations
- **Next milestone:** v1.1 Performance, Observability & Experimentation — defined (see below)

## Current Milestone: v1.1 Performance, Observability & Experimentation

**Goal:** Prepare the codebase for a future SIMD-accelerated JSON ingest path, add observability/logging primitives that surface index internals and hot-path costs, and ship a small CLI that builds an index from a JSONL file for experimentation and teaching.

**Target themes:**
- **Parser seam (preparation for SIMD)** — extract the JSON-parse boundary from the builder into a pluggable `Parser` interface with a `stdlibParser` default (wrapping today's `json.Decoder.UseNumber()`). Pure refactor with a parity-test harness. Keeps the door open for a SIMD parser in v1.2 without touching builder internals.
- **Observability & logging** — add logger/tracer interfaces inspired by `github.com/amikos-tech/go-wand`'s `pkg/logging` (PR #114) and `pkg/telemetry` (PR #115) patterns. Structured events for index build, query evaluation, serialization; zero-cost when disabled; no global OTel mutation; migrate the existing `adaptiveInvariantLogger *log.Logger` to the new interface.
- **Experimentation CLI** — new `experiment` subcommand accepts JSONL (file or stdin), builds an index, emits a per-path summary table (types, cardinality, mode, bloom occupancy, hot terms). Optional inline predicate tester, JSON output mode, streaming ingest with bounded memory.

**Deferred to v1.2:** SIMD parser implementation (`pure-simdjson` adapter, benchmarks, CI matrix). Blocked on upstream LICENSE, version tag, and shared-library distribution decision.

**Active seeds:** SEED-001 (simdjson test datasets) — deferred to v1.2 alongside the SIMD parser impl.

## What This Is

GIN Index is a Generalized Inverted Index for JSON data, designed for row-group pruning in columnar storage (Parquet). It enables fast predicate evaluation to determine which row groups MAY contain matching documents — filling the gap between a full scan and standing up a database.

As of `v1.0`, the library has a canonical hot-path lookup, exact-int numeric semantics, adaptive high-cardinality string pruning, queryable derived representations alongside raw indexing, and compact prefix-encoded serialized layout.

## Core Value

Material pruning quality and hot-path efficiency gains without turning the library into a heavyweight database or document store.

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

### Active

- **v1.1 — PARSER-01 validated in Phase 13; OBS-01..08 and CLI-01..08 remain in progress.** See `.planning/REQUIREMENTS.md` for the full list and current status across phases 13-15.

### Out of Scope

- Full row-level binary JSON or `VARIANT`-style document storage
- BM25, document scoring, or any ranked retrieval semantics
- Distributed serving, sharding, or a long-running query service
- A new composite query DSL beyond the current predicate model
- Multi-index merge across files in this milestone

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
- Field transformers now support raw-plus-derived companion representations with explicit alias routing
- Prefix-compressed path and term dictionary encoding is now part of the shipped serialized format, with real-corpus impact documented in Phase 11

## Constraints

- **Preserve pruning-first scope**: this remains a pruning index, not a row-level search engine
- **Protect correctness**: new optimizations must not introduce false negatives in row-group selection
- **Benchmark-backed changes**: hot-path and size claims must be supported by benchmarks or fixture-based measurements
- **Explicit format evolution**: any serialized format change must have clear version behavior and tests
- **Avoid gratuitous API churn**: prefer additive configuration and compatibility where practical

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Continue phase numbering at 06 | Preserve roadmap continuity after the completed `v0.1.0` milestone | ✓ |
| Prioritize low-risk/high-signal wins first | Path lookup and build-path fixes should land before structural index changes | ✓ |
| Make adaptive high-cardinality indexing frequency-driven | Hot values are where exact bitmaps recover the most pruning value | ✓ |
| Derived indexing augments raw indexing | Raw semantics remain available while optimized representations stay queryable | ✓ |
| Leave serialization compaction until last | Compaction should follow functional changes so the encoded layout stabilizes once | ✓ |

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
*Last updated: 2026-04-21 — Phase 13 closed; Phase 14 pending*
