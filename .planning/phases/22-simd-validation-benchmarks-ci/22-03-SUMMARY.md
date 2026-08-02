---
phase: 22-simd-validation-benchmarks-ci
plan: 03
subsystem: testing
tags: [simdjson, fuzzing, differential-testing, corpus, input-bounds]

# Dependency graph
requires:
  - phase: 22-simd-validation-benchmarks-ci
    plan: 02
    provides: Shared SIMD test construction policy and same-process encoded-byte parity seams
  - phase: 20-realistic-benchmark-dataset-foundation
    provides: Deterministic nested and mixed-array JSONL fixtures
  - phase: 21-simd-parser-adapter
    provides: Parser lifecycle contract and malformed-input attribution oracle
provides:
  - Five reviewed standard Go fuzz seeds spanning authored, realistic, and known-exclusion inputs
  - Pre-arm 4096-byte and depth-8 array bounds with string-aware scanning
  - Builder-state three-way parity classification with stable non-fatal one-sided diagnostics
affects: [22-04, 22-07, SIMD-08]

# Tech tracking
tech-stack:
  added: []
  patterns: [standard Go fuzz corpus, sequential shared-parser differential, injected test-arm seam, builder-state outcome classification]

key-files:
  created:
    - parser_parity_fuzz_simd_test.go
    - testdata/fuzz/FuzzParserParity/authored-int-boundaries
    - testdata/fuzz/FuzzParserParity/authored-transformer-nested
    - testdata/fuzz/FuzzParserParity/phase20-nested
    - testdata/fuzz/FuzzParserParity/phase20-mixed-array
    - testdata/fuzz/FuzzParserParity/known-malformed-layer-asymmetry
  modified: []

key-decisions:
  - "Fuzz parity uses hard parser failures plus soft numeric failures so the existing malformed trailing-number attribution difference remains observable without treating either arm as committed."
  - "Committed state is derived from fresh-builder numDocs bookkeeping; encoding occurs only after exactly one document commits."
  - "Unexpected one-sided commits remain non-fatal but always use the stable SIMD_FUZZ_OUTCOME class=unexpected_one_sided_commit diagnostic with both arm outcomes."

patterns-established:
  - "Bound before arms: reject inputs over 4096 bytes or array depth 8 before invoking either parser/build closure."
  - "Deterministic discovery net: ordinary tagged go test replays committed standard corpus seeds; timed fuzzing remains manual."

requirements-completed: [SIMD-08]

# Metrics
duration: 8 min
completed: 2026-08-01
---

# Phase 22 Plan 03: Bounded SIMD Differential Fuzzing Summary

**A deterministic five-seed SIMD differential with pre-arm resource bounds, builder-backed commit classification, and an explicit malformed-attribution exclusion**

## Performance

- **Duration:** 8 min
- **Started:** 2026-08-01T09:03:09Z
- **Completed:** 2026-08-01T09:11:32Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- Committed exactly five standard `go test fuzz v1` seeds: two verbatim authored documents, two verbatim Phase 20 JSONL records, and the existing `1e400 garbage` exclusion.
- Added a string/escape-aware array-depth scanner and inclusive 4096-byte/depth-8 predicate, with an injected seam proving rejected inputs return before both arms.
- Implemented a sequential differential that constructs one caller-owned SIMD parser before `f.Fuzz`, creates fresh builders per arm, and derives commit/soft-skip/error state from builder bookkeeping.
- Kept byte divergence fatal, made one-sided commits durably visible under one stable structured prefix, and classified the exact parser-error-versus-numeric-soft-skip case separately.

## Task Commits

Each task was committed atomically:

1. **Task 1: Commit a bounded standard Go fuzz seed corpus** - `e6870a1` (test)
2. **Task 2 RED: Specify bounded three-way differential behavior** - `781fdc1` (test)
3. **Task 2 GREEN: Implement bounded SIMD parity fuzzing** - `31a8044` (feat)

**Plan metadata:** committed with this summary.

## Files Created/Modified

- `parser_parity_fuzz_simd_test.go` - Tagged bounds, build outcomes, classifier, standard-corpus validator, deterministic tests, and `FuzzParserParity`.
- `testdata/fuzz/FuzzParserParity/authored-int-boundaries` - Exact authored large-integer seed.
- `testdata/fuzz/FuzzParserParity/authored-transformer-nested` - Exact authored buffered transformer/nested-array seed.
- `testdata/fuzz/FuzzParserParity/phase20-nested` - First deterministic nested high-cardinality JSONL record.
- `testdata/fuzz/FuzzParserParity/phase20-mixed-array` - First deterministic mixed-type array JSONL record.
- `testdata/fuzz/FuzzParserParity/known-malformed-layer-asymmetry` - Exact malformed trailing-number attribution seed.

## Decisions Made

- The known exclusion requires both the exact `1e400 garbage` bytes and the established stdlib numeric-soft-skip/SIMD parser-error shape; other neither-committed outcomes remain ordinary rejection agreement.
- The fuzz target does not load corpus files itself. Go discovers and replays them automatically; the test-only AST decoder exists solely to validate standard format, provenance, and bounds.
- One-sided commit records quote error text so embedded newlines cannot break the one-record-per-line diagnostic contract.

## Verification Evidence

- Focused tagged scanner, boundary, early-return, classifier, corpus, known-asymmetry, and seed replay command passed with `AMI_GIN_SIMD_REQUIRED=1`.
- Verbose classifier evidence included `SIMD_FUZZ_OUTCOME class=unexpected_one_sided_commit` with both stdlib and SIMD outcomes.
- All five `FuzzParserParity` corpus subtests passed without `-fuzz`.
- Full default `go test ./...` passed.
- `make simd-isolation-check` passed (`go build ./...` and `go vet ./...`).
- Final diff contains only the tagged harness and the five named seed files; production parsers/builders, CI schedules, dependencies, source fixtures, and parity goldens are unchanged.

## TDD Gate Compliance

- Task 2 RED commit `781fdc1` failed on the intentionally missing scanner, bounds, outcome, classifier, and corpus-decoder seams.
- Task 2 GREEN commit `31a8044` followed RED and passed every focused behavior plus the full default and isolation gates.
- No refactor commit was needed after GREEN.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Known Stubs

None.

## Next Phase Readiness

- Plan 22-04 can compile-check and guard the documented SIMD consumer path while reusing deterministic `FuzzParserParity` seed replay.
- The Phase 19 HARD stop did not trigger: every committed corpus input produced byte-identical indexes when both arms committed.
- Timed fuzzing remains manual; no timed CI fuzz job or parser registry was added.

## Self-Check: PASSED

- All six created files exist, all three task commits are present, and the standard corpus validator proves the required 2 authored / 2 Phase 20 / 1 known-exclusion composition.
- Every task acceptance criterion and the plan-level verification command passed after the final production commit.
- Stub and threat-surface scans found no incomplete behavior or new production trust boundary; T-22-03 and T-22-04 mitigations are executable in the tagged test suite.

---
*Phase: 22-simd-validation-benchmarks-ci*
*Completed: 2026-08-01*
