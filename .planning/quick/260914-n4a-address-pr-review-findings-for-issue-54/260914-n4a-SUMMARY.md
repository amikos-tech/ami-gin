---
phase: quick-260914-n4a
plan: 01
subsystem: testing
tags: [builder, docs, security-doc, cli, tests]

# Dependency graph
requires:
  - phase: quick-260808-n99
    provides: round-3 PR review fixes on fix/issue-54 (budget diagnostic, --max-staged-paths on build, docs/SECURITY wording)
provides:
  - Trimmed and corrected comments on remapCompanionIngestErrorPath, getOrCreateStagedPath, MaxStagedPaths, IngestLayerResource, experiment.go sample-cap
  - Accurate SECURITY.md T18-04/T18-05/T18-07 evidence citations (function/test names, no line numbers)
  - Budget-hint clause on the staged-path budget error message
  - Five new gin_test.go coverage cases (builder recovery, flat-array wildcard collapsing, companion child-path remap, AllRGs fallback)
  - gin-index experiment negative --max-staged-paths rejection test mirroring the build subcommand
affects: [issue-54, PR review on fix/issue-54]

# Tech tracking
tech-stack:
  added: []
  patterns: []

key-files:
  created: []
  modified:
    - builder.go
    - gin.go
    - ingest_error.go
    - SECURITY.md
    - cmd/gin-index/experiment.go
    - gin_test.go
    - cmd/gin-index/experiment_test.go

key-decisions:
  - "requireStagedPathBudgetError switched from exact match to strings.HasPrefix so the new trailing hint clause doesn't require touching the 10 existing call sites."

patterns-established: []

requirements-completed: [PR-01, PR-02, PR-03, PR-04, PR-05, PR-06, PR-07, PR-08, PR-09, PR-10]

# Metrics
duration: 12min
completed: 2026-09-14
---

# Quick Task 260914-n4a: Address PR Review Findings for Issue #54 Summary

**Closed ten round-4 PR review findings on fix/issue-54: comment trims, stale SECURITY.md citation fixes, a staged-path budget error hint, five new test cases, and a mirrored CLI negative-flag test — no behavior changes beyond the additive error message clause.**

## Performance

- **Duration:** 12 min
- **Tasks:** 3
- **Files modified:** 7

## Accomplishments
- Trimmed `remapCompanionIngestErrorPath`'s doc comment from 9 to 4 lines, dropping the inaccurate "transformer" failure-type mention
- Replaced three stale SECURITY.md line-number citations (T18-04, T18-05, T18-07) with function/test names, each verified to exist via grep
- Appended a clarifying hint to the staged-path budget error message: "(count includes the root, containers, and derived companion paths)"
- Added builder-recovery, flat-array-wildcard, and companion-child-path-remap subtests to `TestMaxStagedPaths`, plus an AllRGs fallback assertion to `TestWildcardArrayQueryReturnsOnlyMatchingRowGroup`
- Mirrored `TestRunBuildRejectsNegativeMaxStagedPaths` onto `gin-index experiment` as `TestRunExperimentRejectsNegativeMaxStagedPaths`

## Task Commits

Each task was committed atomically:

1. **Task 1: Fix over-long/inaccurate comments and stale SECURITY.md citations** - `f732330` (docs)
2. **Task 2: Append budget-message hint and add five missing gin_test.go cases** - `784a71e` (test)
3. **Task 3: Mirror negative --max-staged-paths rejection test onto experiment** - `18371ed` (test)

## Files Created/Modified
- `builder.go` - Trimmed `remapCompanionIngestErrorPath` doc comment and `getOrCreateStagedPath` prose; appended budget-hint clause to the staged-path budget error message
- `gin.go` - Trimmed `MaxStagedPaths` field comment to point at `WithMaxStagedPaths`
- `ingest_error.go` - Fixed `IngestLayerResource` doc comment wording ("rebuild with a higher limit")
- `SECURITY.md` - Replaced stale line-number citations with function/test names for T18-04, T18-05, T18-07
- `cmd/gin-index/experiment.go` - Collapsed the 6-line sample-cap comment above the failure-sample append into one line
- `gin_test.go` - Prefix-match budget helper, 3 new `TestMaxStagedPaths` subtests, AllRGs fallback assertion in `TestWildcardArrayQueryReturnsOnlyMatchingRowGroup`
- `cmd/gin-index/experiment_test.go` - New `TestRunExperimentRejectsNegativeMaxStagedPaths`

## Decisions Made
- Kept the `requireStagedPathBudgetError` change to a single prefix-match comparison rather than editing all 10 call sites, per the plan's interface note — verified all existing and new call sites still pass.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Verification

- `go build ./...` - clean
- `go test -count=1 ./...` - green except the pre-existing `TestAddDocumentDefaultParserErrorStringsPreserved/unterminated-object` failure on local Go 1.27.1 (tracked in issue #62); confirmed fully green via `GOTOOLCHAIN=go1.26.0 go test -count=1 ./...`
- `make lint` - fails to typecheck under local Go 1.27.1 due to an unrelated golangci-lint/Go export-data version mismatch (pre-existing toolchain issue, not caused by this change); confirmed 0 issues via `GOTOOLCHAIN=go1.26.0 make lint`
- `AMI_GIN_SIMD_REQUIRED=1 go test -count=1 -tags simdjson ./...` - same single pre-existing Go 1.27 failure as above, all else green

## Next Phase Readiness

All ten PR review findings from the round-4 review on `fix/issue-54` are closed. Branch is ready for the next review pass or merge.

## Self-Check: PASSED

All 3 task commit hashes (f732330, 784a71e, 18371ed) confirmed in git log. All 7 modified source files and this summary file confirmed present on disk.

---
*Phase: quick-260914-n4a*
*Completed: 2026-09-14*
