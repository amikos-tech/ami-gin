---
phase: 18-structured-ingesterror-cli-integration
plan: 03
subsystem: cli
tags: [cli, json, text-report, experiment, diagnostics, tdd]

requires:
  - phase: 18-structured-ingesterror-cli-integration
    provides: Public IngestError API and hard ingest wrapping from Plans 18-01 and 18-02
provides:
  - Experiment CLI failure aggregation grouped by IngestError layer
  - Text and JSON summary output for bounded structured failure samples
  - Deterministic 100-line JSONL fixture asserting parser, transformer, and numeric failure counts
affects: [experiment-cli, ingest-error-reporting, phase-18-docs]

tech-stack:
  added: []
  patterns:
    - Bounded-by-count CLI diagnostics with verbatim values
    - Deterministic layer ordering: parser, transformer, numeric, schema, then lexical unknowns
    - Package-level test hook for CLI default GIN config, restored with t.Cleanup and not used from parallel tests

key-files:
  created:
    - .planning/phases/18-structured-ingesterror-cli-integration/18-03-SUMMARY.md
  modified:
    - cmd/gin-index/experiment.go
    - cmd/gin-index/experiment_output.go
    - cmd/gin-index/experiment_test.go

key-decisions:
  - "Attached grouped failures to experiment summary to preserve the existing single-object JSON report shape."
  - "Kept IngestError.Value verbatim in CLI samples; report growth is bounded by at most 3 samples per layer rather than truncation."
  - "Added experimentDefaultConfig as a package-level test hook instead of adding user-facing CLI configuration flags."

patterns-established:
  - "recordExperimentIngestFailure extracts *gin.IngestError with errors.As and ignores non-structured errors."
  - "experimentIngestFailureGroups copies map-backed groups into deterministic output order before rendering."

requirements-completed: [IERR-03]

duration: 5 min
completed: 2026-04-24
---

# Phase 18 Plan 03: Experiment CLI Grouped IngestError Summary

**`gin-index experiment --on-error continue` now reports structured hard ingest failures grouped by layer in text and JSON summaries.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-24T12:50:16Z
- **Completed:** 2026-04-24T12:55:17Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments

- Added `summary.failures` with layer count and up to 3 structured samples per layer.
- Recorded returned `*gin.IngestError` values during continue-mode ingest without changing abort behavior or dense row-group packing.
- Added text output for failure groups and JSON tests for parser, transformer, and numeric failures.
- Added the roadmap-required deterministic 100-line fixture: 90 accepted docs, 10 failures, 9 row groups.

## Task Commits

1. **Task 1 RED: deterministic failure aggregation test** - `e684cae` (test)
2. **Task 1 GREEN: failure report structures and ordering** - `432f5dd` (feat)
3. **Task 2 RED: text/abort reporting tests** - `2e5b259` (test)
4. **Task 2 GREEN: continue-mode recording and text rendering** - `4e0abe5` (feat)
5. **Task 3 RED: JSON grouped failure tests** - `6c9bfc0` (test)
6. **Task 3 GREEN: config hook and 100-line fixture pass** - `a85f9de` (feat)

**Plan metadata:** this docs commit

## Files Created/Modified

- `cmd/gin-index/experiment.go` - Added failure accumulator, deterministic conversion, continue-mode recording, and the test-only default config hook.
- `cmd/gin-index/experiment_output.go` - Added failure group/sample structs, JSON summary field, and text `Failures:` rendering.
- `cmd/gin-index/experiment_test.go` - Added aggregation unit coverage, text/abort sanity coverage, JSON failure coverage, and the 100-line deterministic fixture.

## Decisions Made

- Failures live under `summary.failures` so JSON mode remains one report object.
- Unknown future layers sort lexically after known layers.
- The config override remains package-private and tests that mutate it do not call `t.Parallel()`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Abort sanity test moved from stdin to file input**
- **Found during:** Task 2 (abort-mode sanity coverage)
- **Issue:** Stdin abort mode validates during the spool pre-pass and returns the legacy `invalid JSON record` message before the build loop can produce `*gin.IngestError`.
- **Fix:** Kept existing stdin behavior unchanged and used file input for the abort sanity test so it exercises the build-loop structured parser failure path.
- **Files modified:** `cmd/gin-index/experiment_test.go`
- **Verification:** `go test ./cmd/gin-index -run 'TestRunExperiment(OnErrorContinue.*Text|OnErrorContinue$|OnErrorAbort.*Ingest)' -count=1` passed.
- **Committed in:** `4e0abe5`

---

**Total deviations:** 1 auto-fixed (1 Rule 1)
**Impact on plan:** Existing stdin abort semantics were preserved; file-input abort coverage verifies the intended structured ingest error path without expanding abort reporting.

## Issues Encountered

The only issue was the stdin pre-validation nuance documented above.

## User Setup Required

None - no external service configuration required.

## Verification

- `go test ./cmd/gin-index -run 'TestRunExperimentOnErrorContinue' -count=1` - passed
- `go test ./cmd/gin-index -run 'TestRunExperiment(OnErrorContinue.*Text|OnErrorContinue$|OnErrorAbort.*Ingest)' -count=1` - passed
- `go test ./cmd/gin-index -run 'TestRunExperiment(OnErrorContinueIngestFailuresJSON|HundredDocsKnownIngestFailuresJSON)$' -count=1` - passed
- `go test ./cmd/gin-index -count=1` - passed

## Known Stubs

None. Stub scan found no placeholder/TODO/FIXME patterns introduced by this plan; nil and empty-string matches were ordinary control-flow checks and assertions.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: information-disclosure | `cmd/gin-index/experiment.go` | CLI samples copy untrusted `IngestError.Value` verbatim into reports by design; this is accepted by T18-08 and bounded to 3 samples per layer. |

## Next Phase Readiness

Ready for Plan 18-04 to add public docs/changelog notes and run the final phase verification.

## Self-Check: PASSED

- Verified modified files exist on disk.
- Verified task commits exist in git history: `e684cae`, `432f5dd`, `2e5b259`, `4e0abe5`, `6c9bfc0`, `a85f9de`.
- Verified no unexpected file deletions were included in task commits.

---
*Phase: 18-structured-ingesterror-cli-integration*
*Completed: 2026-04-24*
