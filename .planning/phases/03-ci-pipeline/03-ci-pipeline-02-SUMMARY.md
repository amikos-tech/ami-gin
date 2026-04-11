---
phase: 03-ci-pipeline
plan: 02
subsystem: infra
tags: [github-actions, govulncheck, sarif, code-scanning, readme, ci-badge]
requires: []
provides:
  - weekly govulncheck SARIF reporting workflow
  - canonical README CI badge for ci.yml
affects: [github-actions, readme, code-scanning]
tech-stack:
  added: [golang/govulncheck-action@v1, github/codeql-action/upload-sarif@v4]
  patterns: [split blocking and reporting security workflows, use canonical GitHub-hosted badge URLs]
key-files:
  created:
    - .github/workflows/security.yml
    - .planning/phases/03-ci-pipeline/03-ci-pipeline-02-SUMMARY.md
  modified:
    - README.md
key-decisions:
  - "Kept SARIF reporting in a dedicated Security workflow so scheduled reporting stays separate from blocking PR security checks."
  - "Scoped security-events: write to the govulncheck SARIF upload job while leaving workflow-level permissions at contents: read."
patterns-established:
  - "Security reporting pattern: weekly and manual govulncheck SARIF uploads live in security.yml, separate from ci.yml enforcement."
  - "README badge pattern: use the exact GitHub-hosted ci.yml badge URL once, directly under the repository title."
requirements-completed: [CI-03]
duration: 2 min
completed: 2026-04-11
---

# Phase 03 Plan 02: Security Reporting and README Badge Summary

**Weekly govulncheck SARIF reporting workflow plus the canonical GitHub Actions CI badge in README**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-11T14:41:41Z
- **Completed:** 2026-04-11T14:43:02Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Added `.github/workflows/security.yml` with weekly and manual triggers for govulncheck SARIF reporting.
- Wired SARIF upload through `github/codeql-action/upload-sarif@v4` with least-privilege permissions.
- Added the exact `ci.yml` GitHub Actions badge to the top of `README.md`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Create the scheduled SARIF security workflow** - `52c0baa` (feat)
2. **Task 2: Add the live CI badge to README** - `bc23686` (docs)

**Plan metadata:** Summary-only docs commit recorded in executor output because `STATE.md` and `ROADMAP.md` were intentionally left untouched for wave merge orchestration.

## Files Created/Modified
- `.github/workflows/security.yml` - Weekly/manual `Security` workflow that runs `govulncheck` in SARIF mode and uploads `govulncheck.sarif`.
- `README.md` - Adds the canonical GitHub-hosted CI badge immediately below `# GIN Index`.

## Decisions Made
- Kept the SARIF workflow separate from `ci.yml` so scheduled reporting cannot replace blocking PR-time security enforcement.
- Granted `security-events: write` only to `govulncheck-code-scanning`, preserving `contents: read` at the workflow level.
- Used only the GitHub-hosted CI badge in this plan and left coverage-badge work to the later prerequisite plan.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required in this plan.

## Next Phase Readiness

- `security.yml` is ready for weekly or manual execution once repository Code Scanning is enabled.
- `README.md` now exposes the exact `ci.yml` badge URL expected by downstream validation.
- Shared planning state files were intentionally not updated here because the orchestrator owns those writes after wave merge.

## Self-Check: PASSED

- Verified `.github/workflows/security.yml` exists.
- Verified `README.md` contains the exact canonical `ci.yml` badge URL.
- Verified task commits `52c0baa` and `bc23686` exist in git history.

---
*Phase: 03-ci-pipeline*
*Completed: 2026-04-11*
