---
phase: quick-260730-kny
plan: 01
status: complete
subsystem: tooling
tags: [make, github-actions, go-modules, notice, regression-tests]

requires:
  - phase: quick-260730-ije
    provides: Initial module-derived NOTICE alignment target
provides:
  - Fail-closed NOTICE version validation against Go module replacements
  - Full-file pure-simdjson version-reference scanning with escaped diagnostics
  - Explicit CI gating and isolated regression coverage
affects: [dependency-maintenance, ci, legal-notices]

tech-stack:
  added: []
  patterns:
    - Derive legal-notice pins from the selected module replacement when present
    - Validate canonical NOTICE shapes separately from all relevant version tokens
    - Exercise Make guards through isolated temporary repository inputs

key-files:
  created:
    - notice_version_guard_test.go
  modified:
    - Makefile
    - .github/workflows/ci.yml

key-decisions:
  - "Treat a replacement version as the effective legal-notice authority and reject unversioned local replacements."
  - "Use exact canonical-line checks plus a separate full-file scan so stale supplementary references cannot evade validation."
  - "Run the guard as a named CI step without changing the established golangci-lint command."

patterns-established:
  - "NOTICE guard failures show matching references as line-numbered, byte-escaped text."
  - "Workflow contract tests inspect only the intended job section and step ordering."

requirements-completed: [KNY-01, KNY-02, KNY-03, KNY-04, KNY-05, KNY-06]

metrics:
  duration: 7min
  completed: 2026-07-30
---

# Quick Task 260730-kny: NOTICE Guard Review Findings Summary

**The NOTICE guard now derives effective replacement versions, rejects stale pure-simdjson pins anywhere in the file with byte-visible diagnostics, and runs as a dedicated CI lint gate.**

## Performance

- **Duration:** 7 min
- **Started:** 2026-07-30T11:57:33Z
- **Completed:** 2026-07-30T12:04:30Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments

- Added isolated Make-target regressions for aligned, stale-anywhere, replacement-version, local-replacement, invisible-character, and CI-ordering cases.
- Hardened `check-notice-version` to select `.Replace.Version`, keep exact canonical pin checks, and compare every complete version token on a pure-simdjson reference.
- Added an explicit `make check-notice-version` step between the marker guard and golangci-lint in CI.

## Task Commits

1. **Task 1: Define isolated regressions for the NOTICE guard and CI contract** - `8ebc2f7` (test)
2. **Task 2: Make the NOTICE version guard fail closed on effective full-file references** - `9c07a82` (feat)
3. **Task 3: Run the NOTICE guard explicitly in CI** - `d47c4eb` (chore)
4. **Rule 2 regression: Cover unversioned local replacements** - `74a79fa` (test)

## Files Created/Modified

- `notice_version_guard_test.go` - Runs the real target in temporary copies and asserts CI step ordering.
- `Makefile` - Resolves effective versions, scans all relevant NOTICE lines, and prints byte-escaped failure context.
- `.github/workflows/ci.yml` - Adds the independent NOTICE version-alignment gate to the lint job.

## Decisions Made

- The replacement version is authoritative when Go module resolution supplies one; a local replacement fails clearly instead of falling back to the original requirement.
- Four canonical NOTICE lines retain exact-one validation, while a separate full-file scan rejects stale supplementary version references.
- The existing `golangci-lint` action command remains unchanged; CI invokes the Make guard directly beforehand.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Portability bug] Replaced escaped regex punctuation with bracket forms**
- **Found during:** Task 2
- **Issue:** Make removed the backslashes intended for `awk`, producing an invalid regular expression on the local toolchain.
- **Fix:** Used `[.]` and `[+]` in the shared ERE so both `grep` and `awk` receive the intended complete semantic-version pattern.
- **Files modified:** `Makefile`
- **Verification:** Aligned, stale-anywhere, replacement, and invisible-character regressions pass.
- **Committed in:** `9c07a82`

**2. [Rule 2 - Missing critical coverage] Added an unversioned local-replacement regression**
- **Found during:** Final verification
- **Issue:** The guard implemented the required local replacement rejection but the durable suite did not execute that branch.
- **Fix:** Added a temporary local module replacement test that requires the actionable non-semver diagnostic.
- **Files modified:** `notice_version_guard_test.go`
- **Verification:** `TestNoticeVersionGuard/unversioned_local_replacement_fails` passes.
- **Committed in:** `74a79fa`

---

**Total deviations:** 2 auto-fixed (1 Rule 1, 1 Rule 2)
**Impact on plan:** Both changes keep the guard portable and directly verify its required fail-closed replacement behavior; no dependency, NOTICE-content, or parser behavior changed.

## Issues Encountered

- The initial shared regular expression depended on backslashes that Make removed before `awk` parsed it. The portable bracket-form expression resolved the issue without weakening validation.

## Known Stubs

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Future dependency-version or replacement changes now require all canonical and supplementary pure-simdjson NOTICE references to match the effective version.
- CI visibly enforces this guard before the established Go linter action.

---
*Quick task: 260730-kny*
*Completed: 2026-07-30*

## Self-Check: PASSED

- Found all three implementation files and this summary.
- Found task commits `8ebc2f7`, `9c07a82`, `d47c4eb`, and `74a79fa`.
- Confirmed the complete implementation diff from `db578a8` contains only `Makefile`, `.github/workflows/ci.yml`, and `notice_version_guard_test.go` with no tracked-file deletions.
