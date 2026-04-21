---
phase: 07-builder-parsing-numeric-fidelity
plan: 02
subsystem: testing
tags: [benchmark, parser, performance, allocations, transformers]
requires:
  - phase: 07-01
    provides: transactional explicit-number builder and exact-int numeric semantics
provides:
  - in-repo legacy parser benchmark control path
  - deterministic Phase 07 benchmark fixtures for int-only, mixed-safe, wide-flat, and transformer-heavy documents
  - Phase 07 benchmark families for add-document, build, and finalize parser deltas
affects: [08-adaptive-high-cardinality-indexing, 09-derived-representations, 10-serialization-compaction]
tech-stack:
  added: []
  patterns: [in-repo benchmark control path, parser mode benchmark labeling, deterministic fixture generation]
key-files:
  created: []
  modified: [benchmark_test.go]
key-decisions:
  - "Keep the legacy control path local to benchmark_test.go so BUILD-05 stays reproducible without reviving production code."
  - "Benchmark both wide-flat and transformer-heavy fixtures so Phase 07 measures the two review-identified slow paths directly."
patterns-established:
  - "Benchmark labels use parser=/docs=/shape= segments for historical comparisons."
  - "Parser delta benchmarks reuse the same fixtures and doc counts across legacy and explicit modes."
requirements-completed: [BUILD-05]
duration: 8min
completed: 2026-04-15
---

# Phase 07: Builder Parsing & Numeric Fidelity Summary

**Reproducible parser-delta benchmarks with an in-repo legacy control and deterministic fixture families for Phase 07**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-15T11:05:25Z
- **Completed:** 2026-04-15T11:13:09Z
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments
- Added the benchmark-only `benchmarkAddDocumentLegacy` control path and kept it local to `benchmark_test.go`.
- Added deterministic fixture generators for `shape=int-only`, `shape=mixed-safe`, `shape=wide-flat`, and `shape=transformer-heavy` plus the required `parser=` and `docs=` labels.
- Landed `BenchmarkAddDocumentPhase07`, `BenchmarkBuildPhase07`, and `BenchmarkFinalizePhase07`, then verified them with `go test ./... -run '^$' -bench 'Benchmark(AddDocumentPhase07|BuildPhase07|FinalizePhase07)' -benchtime=1x -count=1`.

## Task Commits

1. **Task 1: Add deterministic Phase 07 fixtures and a benchmark-only legacy parser control** - `c0b6afb` (test)
2. **Task 2: Add legacy-vs-explicit ingest/build benchmark families with explicit naming and alloc reporting** - `c0b6afb` (test)

## Files Created/Modified
- `benchmark_test.go` - deterministic Phase 07 fixtures, benchmark-only legacy ingest control, and Phase 07 benchmark families for add-document, build, and finalize deltas

## Decisions Made
- Reused the current builder for both modes and changed only the ingest path so the benchmark deltas stay attributable to parser behavior rather than fixture drift.
- Preserved the control path inside the repo instead of relying on historical benchmark notes or old git checkouts.
- Measured both wide-document and transformer-heavy shapes explicitly because those were the main review concerns about the new staging parser.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `BUILD-05` is satisfied with a same-branch benchmark harness and reproducible parser mode labels.
- The current benchmark output shows the explicit parser is still more allocation-heavy on the wide-flat and transformer-heavy build paths, which gives Phase 08+ a concrete baseline for future optimization work.

---
*Phase: 07-builder-parsing-numeric-fidelity*
*Completed: 2026-04-15*
