# GIN Index — v1.0 Query & Index Quality

## What This Is

GIN Index has shipped `v0.1.0` and proven the core pruning model: compact sidecar bytes, row-group candidate evaluation, and broad JSON predicate support. This milestone shifts the project from open-source readiness to product quality work on the index itself: better query hot-path performance, lower build-time overhead, stronger numeric fidelity, improved pruning on high-cardinality paths, and smaller serialized artifacts.

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

- None. The v1.0 milestone requirements are fully validated as of Phase 12 evidence reconciliation.

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
*Last updated: 2026-04-21 after Phase 12 completion*
