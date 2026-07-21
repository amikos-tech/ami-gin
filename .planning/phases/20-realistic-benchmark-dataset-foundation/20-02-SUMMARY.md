---
phase: 20-realistic-benchmark-dataset-foundation
plan: 02
subsystem: testing
tags: [benchmarks, jsonl, offline, local-input]

requires:
  - phase: 20-01
    provides: deterministic raw JSONL smoke fixtures and loader
provides:
  - Offline Build, Encode, and Query benchmark leaves for all Phase-20 fixtures
  - Explicitly gated, schema-independent local external benchmark tier
  - Contributor commands for offline and manual-local benchmark runs
affects: [21-simd-parser-adapter, 22-simd-validation-benchmarks-ci]

tech-stack:
  added: []
  patterns: [raw JSONL benchmark setup, exact opt-in environment gate, data-derived IsNotNull query]

key-files:
  created: []
  modified:
    - benchmark_test.go
    - README.md

key-decisions:
  - "External data is enabled only when GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL is exactly 1."
  - "The local Query leaf derives an IsNotNull predicate from the first observed scalar path."

patterns-established:
  - "Phase-20 smoke benchmark setup uses committed raw documents outside timed Encode and Query loops."
  - "Optional local benchmark tiers stay registered and skip with setup guidance when disabled."

requirements-completed: [DATA-01, DATA-03]

duration: 6min
completed: 2026-07-21
---

# Phase 20: Realistic Benchmark Dataset Foundation Summary

**Phase-20 JSONL fixtures now drive offline Build, Encode, and Query benchmarks, with a visible local-only tier that discovers arbitrary JSON input deterministically.**

## Performance

- **Duration:** 6 min
- **Started:** 2026-07-21T08:49:00Z
- **Completed:** 2026-07-21T08:55:33Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Added four offline smoke fixture branches, each with Build, Encode, and Query action leaves.
- Added an unconditionally registered external local-example tier that skips unless its exact explicit gate is enabled.
- Documented no-download smoke and manual-local workflows, including provenance and future redistribution guidance.

## Task Commits

1. **Task 1: Add offline smoke and explicitly gated local benchmark tiers** - `a41a03e` (test)
2. **Task 2: Document the offline and manual-local benchmark workflows** - `dc95a44` (docs)

## Files Created/Modified

- `benchmark_test.go` - Adds the Phase-20 benchmark family, local discovery/gate helpers, and isolated external-tier coverage.
- `README.md` - Documents exact offline smoke and manual-local benchmark commands.

## Decisions Made

- Kept the Phase-20 implementation scoped beside Phase-11 conventions without reusing its typed corpus, gzip, or metric machinery.
- Required the local enable variable to equal `1`; all other values avoid reading the local directory variable.
- Derived the external query as `IsNotNull` on the first deterministic supported scalar path, keeping local fixtures schema-independent.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - the default benchmark is offline. The optional external tier requires a local directory only when a contributor deliberately sets both documented environment variables.

## Next Phase Readiness

Phase 21 can add the opt-in SIMD parser independently. Phase 22 can reuse the established smoke benchmark leaves and raw fixture loader for parser comparisons.

## Self-Check: PASSED

- `go test ./... -run '^TestPhase20' -count=1` passed.
- The offline smoke benchmark command ran all four fixtures and their Build, Encode, and Query leaves.
- The disabled external benchmark command listed and skipped its Build, Encode, and Query leaves successfully.
- `go test ./... -count=1` and `go build ./...` passed.

---
*Phase: 20-realistic-benchmark-dataset-foundation*
*Completed: 2026-07-21*
