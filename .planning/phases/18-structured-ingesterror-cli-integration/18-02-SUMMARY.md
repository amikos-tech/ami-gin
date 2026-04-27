---
phase: 18-structured-ingesterror-cli-integration
plan: 02
subsystem: testing
tags: [tests, guard, ingest, errors, ast]

requires:
  - phase: 18-structured-ingesterror-cli-integration
    provides: Public IngestError API and builder hard-failure wrapping from Plan 18-01
provides:
  - Named per-layer hard ingest behavior matrix assertions
  - Focused AST guard for direct plain error returns in named hard-ingest functions
  - Explicit non-IngestError exception coverage for parser contract, tragic, recovered panic, and soft-mode paths
affects: [builder, ingest-error-contract, phase-18-cli-reporting]

tech-stack:
  added: []
  patterns:
    - Stdlib AST guard scoped to known hard-ingest functions
    - Behavior matrix extraction through github.com/pkg/errors outer wrapping

key-files:
  created:
    - ingest_error_guard_test.go
    - .planning/phases/18-structured-ingesterror-cli-integration/18-02-SUMMARY.md
  modified:
    - failure_modes_test.go

key-decisions:
  - "Used a focused Go AST test instead of a Makefile historical-string guard to reduce false positives and portability risk."
  - "Kept the AST guard scoped to the current named hard-ingest functions; new functions still require behavior-matrix coverage."

patterns-established:
  - "Hard ingest behavior tests assert errors.As through an outer github.com/pkg/errors.Wrap layer."
  - "Exception paths use a shared non-IngestError assertion so parser-contract and tragic boundaries stay explicit."

requirements-completed: [IERR-02]

duration: 3 min
completed: 2026-04-24
---

# Phase 18 Plan 02: Structured IngestError Guard Summary

**Hard ingest failures now have named behavioral coverage plus a focused AST guard against direct plain error returns in current ingest functions.**

## Performance

- **Duration:** 3 min
- **Started:** 2026-04-24T12:44:05Z
- **Completed:** 2026-04-24T12:46:30Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Hardened `TestHardIngestFailuresReturnIngestError` so every named matrix case extracts `*IngestError` through an outer `errors.Wrap`.
- Added builder-usability assertions after hard public failures where fixtures can continue with a valid document.
- Added `TestHardIngestFunctionsDoNotReturnPlainErrors`, a stdlib AST guard scoped to the seven named hard-ingest functions in `builder.go`.

## Task Commits

1. **Task 1: Harden behavior matrix and exception coverage** - `2568f3a` (test)
2. **Task 2: Replace brittle Makefile guard with focused AST enforcement test** - `fd7094c` (test)

**Plan metadata:** this docs commit

## Files Created/Modified

- `failure_modes_test.go` - Added outer-wrap extraction, builder reuse checks, and shared non-IngestError exception assertions.
- `ingest_error_guard_test.go` - Parses `builder.go` and fails direct `errors.New`, `errors.Errorf`, `errors.Wrap`, or `errors.Wrapf` returns in named hard-ingest functions unless routed through `newIngestError` helpers.

## Decisions Made

- Replaced the reviewed Makefile guard approach with a Go AST test, matching the final 18-02 plan and review consensus.
- Kept the guard intentionally narrow: it protects current hard-ingest functions without blocking legitimate non-ingest errors elsewhere.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

The tasks were marked `tdd="true"` but were test/enforcement deliverables against production code already made compliant by Plan 18-01. The added tests passed immediately; no production GREEN commit was needed.

## User Setup Required

None - no external service configuration required.

## Verification

- `go test ./... -run 'Test(HardIngestFailuresReturnIngestError|ParserContractErrorsRemainNonIngestError|SoftFailureModesDoNotReturnIngestError|ParserFailureModeSoftKeepsTragicStateHard|NumericFailureModeSoftKeepsMergeRecoveryTragic)$' -count=1` - passed
- `go test ./... -run 'TestHardIngestFunctionsDoNotReturnPlainErrors$' -count=1` - passed
- `go test ./... -run 'Test(HardIngestFailuresReturnIngestError|ParserContractErrorsRemainNonIngestError|SoftFailureModesDoNotReturnIngestError|HardIngestFunctionsDoNotReturnPlainErrors)$' -count=1` - passed

## Known Stubs

None.

## Threat Flags

None.

## Next Phase Readiness

Ready for Plan 18-03 to consume structured `IngestError` values in the experiment CLI grouped failure reporting.

## Self-Check: PASSED

- Verified created/modified files exist on disk.
- Verified task commits exist in git history: `2568f3a`, `fd7094c`.
- Verified no unexpected file deletions were included in task commits.

---
*Phase: 18-structured-ingesterror-cli-integration*
*Completed: 2026-04-24*
