---
phase: 20-realistic-benchmark-dataset-foundation
plan: 01
subsystem: testing
tags: [fixtures, benchmark, jsonl, provenance]

requires:
  - phase: 19-simd-dependency-decision-integration-strategy
    provides: opt-in SIMD roadmap and offline-default constraints
provides:
  - Deterministic, checked-in realistic JSONL smoke fixtures
  - Raw JSONL loading and integrity coverage for future benchmark work
affects: [20-02, 22-simd-validation-benchmarks-ci]

tech-stack:
  added: []
  patterns: [checked-in synthesized JSONL, raw-byte interleaving validation]

key-files:
  created:
    - testdata/phase20/generate.go
    - testdata/phase20/README.md
    - testdata/phase20/nested_high_cardinality.jsonl
    - testdata/phase20/mixed_type_arrays.jsonl
    - testdata/phase20/number_heavy.jsonl
    - testdata/phase20/combined.jsonl
  modified:
    - benchmark_test.go

key-decisions:
  - "The committed JSONL corpus is the default input; generation is refresh-only."
  - "Combined records are checked byte-for-byte against focused source records."

patterns-established:
  - "Phase 20 fixture checks enforce caps and shape without parsing README tables."
  - "Raw JSONL readers retain validated document bytes for future Builder.AddDocument use."

requirements-completed: [DATA-01, DATA-02]

duration: 7min
completed: 2026-07-21
---

# Phase 20: Realistic Benchmark Dataset Foundation Summary

**Offline, deterministic JSONL fixtures now cover nested high-cardinality data, mixed wildcard arrays, numeric boundaries, and byte-preserving combined streams.**

## Performance

- **Duration:** 7 min
- **Started:** 2026-07-21T08:40:19Z
- **Completed:** 2026-07-21T08:47:06Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments

- Added three 96-record focused fixtures and a 288-record raw round-robin combined fixture.
- Documented synthesized provenance, byte limits, offline use, and optional local simdjson inputs.
- Added raw JSONL loading and smoke integrity coverage for structure, limits, provenance markers, and interleaving drift.

## Task Commits

1. **Task 1: Generate and document the bounded synthesized JSONL corpus** - `b899c1d` (feat)
2. **Task 2: Add raw JSONL loading and fixture-integrity coverage** - `f3cda6a` (test)

## Files Created/Modified

- `testdata/phase20/generate.go` - Deterministically refreshes only the four committed fixture streams.
- `testdata/phase20/README.md` - Records provenance, limits, offline behavior, and optional-local policy.
- `testdata/phase20/*.jsonl` - Focused and raw-interleaved realistic JSONL smoke data.
- `benchmark_test.go` - Raw loader, malformed-line diagnostics, and fixture-contract tests.

## Decisions Made

- Kept fixtures checked in as the consumer source of truth; the generator is only a deterministic maintenance tool.
- Used caps and record counts as executable guards while keeping documented byte metadata out of the test contract.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Combined fixture source-kind assertion treated its descriptor name as a record label**
- **Found during:** Task 2 (Add raw JSONL loading and fixture-integrity coverage)
- **Issue:** The first integrity-test version expected every combined record to have a `combined` source kind, although the contract requires raw focused records.
- **Fix:** Excluded the synthetic combined descriptor from focused-source checks and asserted the required nested/array/number source-kind sequence during byte-for-byte interleaving validation.
- **Files modified:** `benchmark_test.go`
- **Verification:** `go test ./... -run '^TestPhase20SmokeFixtures$' -count=1`
- **Committed in:** `f3cda6a`

---

**Total deviations:** 1 auto-fixed (1 Rule 1 bug).
**Impact on plan:** The fix aligns the integrity check with the raw-record interleaving contract; no scope expanded.

## Issues Encountered

- The first focused Go test populated the local module cache by downloading an existing AWS SDK module. No repository dependency or source change was made; subsequent required checks ran successfully from cache.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Plan 20-02 can build the smoke benchmark family on the committed raw-document loader and fixture descriptors. The optional local simdjson path remains documentation-only until that plan wires its explicit opt-in benchmark tier.

## Self-Check: PASSED

- All planned artifacts exist and the task commits are present.
- Generator refresh leaves committed JSONL unchanged.
- Focused smoke-fixture and full repository tests pass.

---
*Phase: 20-realistic-benchmark-dataset-foundation*
*Completed: 2026-07-21*
