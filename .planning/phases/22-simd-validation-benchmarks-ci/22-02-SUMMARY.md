---
phase: 22-simd-validation-benchmarks-ci
plan: 02
subsystem: testing
tags: [simdjson, differential-testing, ast-guard, parity]

# Dependency graph
requires:
  - phase: 22-simd-validation-benchmarks-ci
    plan: 01
    provides: Approved pure-simdjson v0.1.7 provenance for tagged Phase 22 evidence
  - phase: 20-realistic-benchmark-dataset-foundation
    provides: Four checked-in realistic JSONL fixtures and registered query predicates
  - phase: 21-simd-parser-adapter
    provides: Opt-in SIMD parser, authored parity oracle, and malformed-input attribution exclusion
provides:
  - Shared three-state SIMD test construction policy for unsupported, optional-local, and required-CI hosts
  - AST-enforced raw-constructor allowlist across every SIMD-tagged test file
  - Live encoded-byte and query parity over four Phase 20 fixtures and the shared 24-case Evaluate matrix
affects: [22-03, 22-04, 22-05, 22-06, 22-07, SIMD-08, SIMD-10]

# Tech tracking
tech-stack:
  added: []
  patterns: [testing.TB parser ownership, native-free policy seams, same-process parser differential, AST policy guard]

key-files:
  created:
    - parser_parity_phase20_simd_test.go
  modified:
    - parser_simd_integration_test.go
    - parser_parity_simd_test.go
    - parser_parity_test.go
    - benchmark_test.go

key-decisions:
  - "Supported SIMD tests use one testing.TB helper: unsupported platforms skip before construction, local load failures skip with remediation, and AMI_GIN_SIMD_REQUIRED=1 makes supported-host failures fatal."
  - "Realistic parity remains a live same-process differential; Phase 20 binary goldens are not added."
  - "The qualified parity claim covers documents that ingest without a parser-layer error; malformed-input failure-layer attribution remains the explicit tested exclusion."

patterns-established:
  - "Tagged constructor ownership: every native-dependent test obtains its parser through newTestSIMDParser, enforced by an AST guard rather than text matching."
  - "Realistic parity: stdlib and one reused SIMD parser receive identical fixture bytes/configuration, then encoded bytes and query results are compared in process."

requirements-completed: [SIMD-08, SIMD-10]

# Metrics
duration: 10 min
completed: 2026-08-01
---

# Phase 22 Plan 02: SIMD Construction and Realistic Parity Summary

**A non-skippable supported-host SIMD test policy plus byte-identical indexes and matching queries across authored fixtures, four realistic datasets, and 24 Evaluate cases**

## Performance

- **Duration:** 10 min
- **Started:** 2026-08-01T08:47:58Z
- **Completed:** 2026-08-01T08:57:57Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- Centralized every native-dependent tagged test on one `testing.TB` constructor policy with an exact five-platform allowlist, six wrapped-error sentinel classes, local remediation skips, and required-CI fatal behavior.
- Added an executable AST guard that rejects unqualified and qualified raw constructor bypasses while allowing only the sole helper call and the optional no-output compile-only Example shape.
- Proved identical encoded indexes and query results for documents that ingest without a parser-layer error across the 12 authored fixtures, all four Phase 20 fixtures, their four registered predicates, and the shared 24-case Evaluate matrix.
- Preserved malformed-input failure-layer attribution as the explicit tested exclusion and left the default dependency graph, `go.mod`, `go.sum`, and authored goldens unchanged.

## Task Commits

Each task followed RED then GREEN with atomic commits:

1. **Task 1 RED: Enforce the three-state SIMD test construction policy** - `cbf13cf` (test)
2. **Task 1 GREEN: Enforce the three-state SIMD test construction policy** - `6ba934c` (test)
3. **Task 2 RED: Pin realistic encoded-byte and query parity** - `4fc3454` (test)
4. **Task 2 GREEN: Pin realistic encoded-byte and query parity** - `6444b30` (test)

**Plan metadata:** committed with this summary.

## Files Created/Modified

- `parser_simd_integration_test.go` - Three-state constructor policy, native-free policy/lifecycle coverage, raw-call AST validator, and migrated manual-close tests.
- `parser_parity_simd_test.go` - Existing soft-skip parity now uses the shared constructor helper.
- `parser_parity_phase20_simd_test.go` - Same-process byte and query differential for four realistic fixtures plus the shared Evaluate matrix.
- `parser_parity_test.go` - Variadic parser-option build seam and reusable 24-case Evaluate oracle.
- `benchmark_test.go` - Variadic Phase 20 index builder while preserving existing callers and row-group packing.

## Decisions Made

- `AMI_GIN_SIMD_REQUIRED` is exactly the CI enforcement flag; only the value `1` enables fatal supported-host construction behavior.
- Constructor failure classes are stable symbolic labels derived with `errors.Is` from the six approved upstream sentinels; error strings are diagnostics only.
- Phase 20 parity uses live differentials and existing JSONL fixtures, avoiding duplicate binary goldens and fixture-regeneration coupling.

## Verification Evidence

- Three-state policy and AST guard: `go test -tags simdjson -run '^(TestSupportedSIMDPlatform|TestClassifySIMDLoadError|TestSIMDConstructionPolicy|TestSIMDConstructorCallPolicy)$' -count=1 .` passed.
- Required-host tagged migration: `AMI_GIN_SIMD_REQUIRED=1 go test -tags simdjson -run '^TestSIMDParser' -count=1 .` passed.
- Hard parity gate: `AMI_GIN_SIMD_REQUIRED=1 go test -tags simdjson -run '^(TestSIMDParserGoldenAuthoredFixtures|TestSIMDParserPhase20Parity|TestSIMDParserEvaluateParity|TestSIMDParserMalformedTrailingNumericKnownPolicyAsymmetry)$' -count=1 .` passed.
- Default regression suite: `go test ./...` passed.
- Dependency isolation: `make simd-isolation-check` passed.
- Dependency and oracle integrity: `go.mod`, `go.sum`, and `testdata/parity-golden/` have no Plan 22-02 changes.

## TDD Gate Compliance

- Task 1: RED `cbf13cf` preceded GREEN `6ba934c`.
- Task 2: RED `4fc3454` preceded GREEN `6444b30`.
- Both RED runs failed on the missing policy/helper seams expected by their behavior tests; both GREEN runs passed the focused and regression gates.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Known Stubs

None.

## Next Phase Readiness

- Plan 22-03 can add bounded differential fuzz coverage through the shared `testing.TB` constructor and parity seams.
- Plans 22-05 through 22-07 may proceed because the Phase 19 HARD stop did not trigger: all in-scope encoded bytes and query results matched.
- The five-platform workflow enforcement remains owned by the later CI plan; this plan supplies the required-host flag and constructor guard it will consume.

## Self-Check: PASSED

- All five created/modified files exist and the four task commits are present in git history.
- Every task acceptance command and the plan-level verification commands passed after the final production commit.
- No Phase 20 binary golden, production parser/API, dependency, CLI, fixture generator, or default behavior changed.

---
*Phase: 22-simd-validation-benchmarks-ci*
*Completed: 2026-08-01*
