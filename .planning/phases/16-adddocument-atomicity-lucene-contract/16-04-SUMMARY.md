---
phase: 16-adddocument-atomicity-lucene-contract
plan: 04
subsystem: lint
tags: [lint, ci, static-policy, validator-marker]

requires:
  - phase: 16-adddocument-atomicity-lucene-contract
    provides: Marker-protected infallible merge signatures from plan 16-01
provides:
  - Local `make check-validator-markers` policy target
  - `make lint` dependency on the validator marker policy
  - GitHub CI lint-job execution of the same marker policy
affects: [16-adddocument-atomicity-lucene-contract, phase17, phase18]

tech-stack:
  added: []
  patterns:
    - POSIX awk static marker policy in Makefile
    - Explicit CI lint-job step for non-golangci static policy

key-files:
  created:
    - .planning/phases/16-adddocument-atomicity-lucene-contract/16-04-SUMMARY.md
    - .planning/phases/16-adddocument-atomicity-lucene-contract/deferred-items.md
  modified:
    - Makefile
    - .github/workflows/ci.yml

key-decisions:
  - "Implemented marker enforcement with POSIX awk in Makefile rather than a custom analyzer or golangci plugin."
  - "Kept the existing golangci-lint GitHub Action path and added a separate marker-check step in the lint job."
  - "The unowned gin_test.go goconst finding discovered during plan verification was resolved during the Wave 2 integration gate."

patterns-established:
  - "Marker lines must directly precede a function declaration."
  - "The three expected merge-layer marker names are positively enforced, not only checked when present."

requirements-completed: [ATOMIC-02]

duration: 7min
completed: 2026-04-23
---

# Phase 16 Plan 04: Static Validator Marker Enforcement Summary

**Local and CI lint enforcement for merge-layer validator markers using a POSIX awk policy target**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-23T09:45:46Z
- **Completed:** 2026-04-23T09:52:59Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- Added `make check-validator-markers`, enforcing direct marker placement, expected marker names, exact marker count, and no `error` return on marked signatures.
- Wired `make lint` through the marker policy before `golangci-lint run`.
- Added a `Check validator markers` step to the GitHub CI lint job while preserving the existing golangci action and pinned version.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add local marker/signature check to Makefile lint** - `a6c047e` (feat)
2. **Task 2: Run marker/signature check in GitHub CI lint job** - `70445e2` (ci)

**Plan metadata:** captured in the final `docs(16-04)` metadata commit

## Files Created/Modified

- `Makefile` - Added the marker policy target, made `lint` depend on it, and updated lint help text.
- `.github/workflows/ci.yml` - Added the explicit CI lint-job marker check step.
- `.planning/phases/16-adddocument-atomicity-lucene-contract/deferred-items.md` - Records that the lint blocker discovered during verification was resolved during Wave 2 integration.
- `.planning/phases/16-adddocument-atomicity-lucene-contract/16-04-SUMMARY.md` - Captures execution results and verification evidence.

## Decisions Made

- Used a Makefile-embedded POSIX awk check to match the plan's shell/awk/grep-style tooling constraint.
- Added a separate CI run step instead of changing the existing `golangci/golangci-lint-action@v9` invocation, minimizing CI behavior changes.
- Left `builder.go` and `gin_test.go` untouched for plan 16-04; concurrent plan ownership stayed respected.

## Deviations from Plan

### Integration Issues

**1. [Resolved] `make lint` initially blocked by goconst finding**
- **Found during:** Task 1 verification and plan-level verification
- **Issue:** `make lint` reaches `golangci-lint run` and fails on `gin_test.go:3298` because the string `unsupported mixed numeric promotion at $.score` appears three times.
- **Handling:** The plan executor left `gin_test.go` untouched to respect concurrent file ownership. The orchestrator resolved it after Wave 2 by extracting a shared test constant.
- **Files modified:** `gin_test.go`, `.planning/phases/16-adddocument-atomicity-lucene-contract/deferred-items.md`
- **Verification:** `make check-validator-markers` passes; `make lint` passes after the integration fix.
- **Committed in:** post-plan Wave 2 integration fix

---

**Total deviations:** 0 auto-fixed; 1 integration issue resolved after plan completion.
**Impact on plan:** Static marker enforcement is implemented locally and in CI, and full lint is green after the Wave 2 integration fix.

## Issues Encountered

- `make lint` initially failed on an unowned `gin_test.go` goconst issue; this was resolved during the Wave 2 integration gate.
- Concurrent plan 16-02 commits landed during execution; plan 16-04 staging was kept limited to its owned files after verifying commit scopes.

## User Setup Required

None - no external service configuration required.

## Verification

- `make check-validator-markers` - PASS
- Temp-copy negative checks for a missing marker and an `error` return on a marked function - PASS
- `rg -n 'Check validator markers|make check-validator-markers' .github/workflows/ci.yml` - PASS
- `make lint` - PASS after the Wave 2 integration fix

## Known Stubs

None.

## Next Phase Readiness

The marker/signature policy is active in local lint and the CI lint job. Wave 3 can rely on `make lint` running both the marker policy and golangci-lint successfully.

## Self-Check: PASSED

- Created files exist: `16-04-SUMMARY.md`, `deferred-items.md`
- Task commits found: `a6c047e`, `70445e2`
- Stub scan found no blocking placeholder patterns in plan-created or plan-modified files.

---
*Phase: 16-adddocument-atomicity-lucene-contract*
*Completed: 2026-04-23*
