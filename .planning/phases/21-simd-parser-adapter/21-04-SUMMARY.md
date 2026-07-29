---
phase: 21-simd-parser-adapter
plan: 04
subsystem: parser
tags: [go, simdjson, lifecycle, error-provenance, atomicity]

requires:
  - phase: 21-03
    provides: Build-tagged pure-simdjson adapter and native document cleanup path
provides:
  - Fatal parser-lifecycle marker with multi-cause errors.Is/errors.As traversal
  - Builder poisoning before parser soft-skip and stage-error classification
  - Native-free tagged lifecycle tests for cleanup and concurrent stage failures
affects: [22-simd-validation-benchmarks-ci]

tech-stack:
  added: []
  patterns:
    - Private multi-cause lifecycle errors expose cleanup and concurrent walk failures as peer unwrap branches
    - Parser lifecycle failures terminate the builder before recoverable ingest routing
    - Native cleanup behavior is tested through injected walk and close callbacks

key-files:
  created:
    - parser_lifecycle_test.go
    - parser_simd_lifecycle_test.go
  modified:
    - parser.go
    - builder.go
    - parser_simd.go

key-decisions:
  - Failed parser cleanup is a terminal builder integrity failure regardless of ParserFailureMode
  - Cleanup and concurrent walk errors remain peer causes instead of flattening either into text
  - Tagged lifecycle regressions use native-free callbacks and leave runtime parity to Phase 22

patterns-established:
  - "Fatal parser routing: lifecycle-marker detection precedes soft-skip, stage, and parser failure-mode branches"
  - "Exactly-once cleanup: finishSIMDDocument owns one deferred close callback around one walk callback"

requirements-completed: [SIMD-04, SIMD-06, SIMD-07]

duration: 8 min
completed: 2026-07-23
---

# Phase 21 Plan 04: Parser Lifecycle Gap Closure Summary

**Fatal SIMD cleanup failures now poison the builder exactly once while preserving cleanup, hard-stage, and soft-skip causes through standard Go error traversal.**

## Performance

- **Duration:** 8 min
- **Started:** 2026-07-23T06:24:04Z
- **Completed:** 2026-07-23T06:32:58Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- Added a private multi-cause parser lifecycle error that keeps cleanup and concurrent walk failures discoverable through `errors.Is` and `errors.As`.
- Routed lifecycle failures ahead of all recoverable parser branches so soft mode cannot hide a failed native close or permit parser reuse.
- Replaced stringified cleanup handling with an exactly-once callback seam that retains hard numeric `IngestError` and soft numeric skip provenance.
- Added default and tagged native-free regressions proving zero durable mutation, zero soft-skip accounting, one stored tragic error, and no second parser dispatch.

## Task Commits

Each TDD task was committed with separate RED and GREEN outcomes:

1. **Task 1 RED: Add failing parser lifecycle tests** - `d540619` (test)
2. **Task 1 GREEN: Make parser lifecycle failures terminal** - `0739e8b` (fix)
3. **Task 2 RED: Add failing SIMD lifecycle tests** - `cdfe6ae` (test)
4. **Task 2 GREEN: Preserve SIMD cleanup provenance** - `69fe63f` (fix)

## Files Created/Modified

- `parser.go` - Defines the private multi-cause lifecycle marker and documents the fatal parser contract.
- `builder.go` - Stores lifecycle failures as the builder's first tragic error before any recoverable routing.
- `parser_lifecycle_test.go` - Covers multi-cause traversal and exact-once terminal builder behavior in default builds.
- `parser_simd.go` - Adds the exactly-once `finishSIMDDocument` cleanup combiner and wires it into the tagged adapter.
- `parser_simd_lifecycle_test.go` - Injects native-free walk and close outcomes for close-only, hard-stage, soft-stage, and panic cleanup paths.

## Verification

- `go test -run '^TestParserLifecycle' -count=1 .` — passed.
- `go test -tags simdjson -run '^TestSIMDDocumentLifecycle' -count=1 .` — passed without constructing the native parser.
- `go test ./...` — passed.
- `go test -tags simdjson ./...` — passed.
- `go build ./... && go vet ./...` — passed.
- `go build -tags simdjson ./... && go vet -tags simdjson ./...` — passed.
- `make simd-isolation-check` — passed.
- `make test` — passed 1,042 tests with one pre-existing fixture-dependent skip.

## TDD Gate Compliance

- Task 1 failed in RED because `parserLifecycleError` and its constructor did not exist, then passed after fatal routing was implemented.
- Task 2 failed in RED because `finishSIMDDocument` did not exist, then passed after the callback cleanup seam was wired.
- Both RED commits precede their corresponding GREEN implementation commits.

## Decisions Made

- Treat native document cleanup failure as an integrity boundary, not a parser-local input failure.
- Preserve cleanup and walk failures as separate unwrap branches so callers can diagnose both without parsing error text.
- Keep Phase 22's native execution, parity, benchmark, and platform validation scope unchanged.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - the SIMD backend remains opt-in through the existing build tag and parser selection path.

## Next Phase Readiness

- The two blocking Phase 21 verification gaps are closed.
- Phase 22 can validate native runtime parity, benchmarks, and platform CI without inheriting a soft-skippable cleanup failure.

## Self-Check: PASSED

- All five created or modified implementation/test files exist.
- Task commits `d540619`, `0739e8b`, `cdfe6ae`, and `69fe63f` are present.
- Default, tagged, isolation, build, vet, and repository test gates passed.
- Stub scan found no placeholders or unwired data paths in plan-owned files.
- No security-relevant surface outside the plan threat model was introduced.

---
*Phase: 21-simd-parser-adapter*
*Completed: 2026-07-23*
