---
phase: 16-adddocument-atomicity-lucene-contract
plan: 01
subsystem: builder
tags: [builder, atomicity, validator, numeric]

requires: []
provides:
  - Validator regression coverage for both lossy mixed numeric promotion directions
  - Marker-protected infallible merge signatures for numeric merge paths
  - Internal invariant panic wording for validator-missed mixed numeric promotion
affects: [16-02, 16-03, 16-04, phase17, phase18]

tech-stack:
  added: []
  patterns:
    - validate-before-mutate numeric promotion gate
    - MUST_BE_CHECKED_BY_VALIDATOR merge marker
    - merge-layer invariant panic for missed validator cases

key-files:
  created:
    - .planning/phases/16-adddocument-atomicity-lucene-contract/16-01-SUMMARY.md
  modified:
    - builder.go
    - gin_test.go

key-decisions:
  - "Focused pre-check showed validateStagedPaths already covered both mixed numeric promotion directions before signature edits; the validator body was left unchanged."
  - "Validator tests seed staged numeric observations directly because stageJSONNumberLiteral already rejects these lossy promotions before validateStagedPaths can be isolated."

patterns-established:
  - "Merge-layer functions marked MUST_BE_CHECKED_BY_VALIDATOR do not return error."
  - "Missed mixed-promotion validation in merge panics with validator-missed wording."

requirements-completed: [ATOMIC-02]

duration: 5min
completed: 2026-04-23
---

# Phase 16 Plan 01: Validator-Complete Numeric Promotion Summary

**Validator-backed mixed numeric promotion rejection with marker-protected infallible merge signatures**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-23T09:35:28Z
- **Completed:** 2026-04-23T09:40:14Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Added focused validator coverage for unsafe int-to-float and float-path unsafe-int promotion cases.
- Proved rejected mixed numeric promotion leaves the builder usable for a later valid document.
- Removed `error` returns from `mergeStagedPaths`, `mergeNumericObservation`, and `promoteNumericPathToFloat`, with validator markers immediately above each function.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add focused validator tests for lossy numeric promotion hoisting** - `515c10f` (test)
2. **Task 2: Make merge-layer numeric functions infallible by signature** - `e2304c1` (refactor)

**Plan metadata:** captured in the final `docs(16-01)` metadata commit

## Files Created/Modified

- `builder.go` - Added merge validator markers, removed merge-layer `error` returns, and converted missed mixed-promotion checks to invariant panics.
- `gin_test.go` - Added focused validator and post-rejection usability coverage.
- `.planning/phases/16-adddocument-atomicity-lucene-contract/16-01-SUMMARY.md` - Captures execution results and verification evidence.

## Decisions Made

- Focused pre-check result: `validateStagedPaths` already covered both promotion directions before signature edits, so no validator body changes were made.
- The direct validator tests seed `staged.numericValues` manually to isolate `validateStagedPaths`, because `stageJSONNumberLiteral` already rejects these lossy promotions against real `b.pathData`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Adjusted validator unit-test staging shape**
- **Found during:** Task 1 (Add focused validator tests for lossy numeric promotion hoisting)
- **Issue:** The plan-directed call to `stageJSONNumberLiteral` returned `unsupported mixed numeric promotion at $.score` before `validateStagedPaths` ran, so it could not prove the validator path directly.
- **Fix:** Seeded the staged numeric observations directly in the test state, then called `validateStagedPaths`.
- **Files modified:** `gin_test.go`
- **Verification:** `go test -run 'Test(ValidateStagedPathsRejectsLossyPromotionBeforeMerge|ValidateStagedPathsRejectsUnsafeIntIntoFloatPath)$' ./... -count=1`
- **Committed in:** `515c10f`

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** The behavioral goal and acceptance criteria are preserved; the adjustment only isolates the intended validator function under current staging behavior.

## Issues Encountered

- `stageJSONNumberLiteral` already catches the tested lossy promotion against existing builder state. This confirmed the validator simulation was already complete once staged observations are present, and required only the test-shape adjustment above.

## User Setup Required

None - no external service configuration required.

## Verification

- `go test -run 'Test(ValidateStagedPathsRejectsLossyPromotionBeforeMerge|ValidateStagedPathsRejectsUnsafeIntIntoFloatPath|MixedNumericPathRejectsLossyPromotionLeavesBuilderUsable)$' ./... -count=1` - PASS
- `rg -n 'func .*mergeStagedPaths.*error|func .*mergeNumericObservation.*error|func .*promoteNumericPathToFloat.*error' builder.go` - PASS, no output
- `go test ./... -count=1` - PASS

## Known Stubs

None.

## Next Phase Readiness

Ready for 16-02. The merge functions are now marker-protected and infallible by signature; downstream recovery and tragic-state work can wrap the invariant panic surface without ordinary user-error returns in the merge layer.

## Self-Check: PASSED

- Created files exist: `16-01-SUMMARY.md`
- Task commits found: `515c10f`, `e2304c1`
- Required verification commands passed.

---
*Phase: 16-adddocument-atomicity-lucene-contract*
*Completed: 2026-04-23*
