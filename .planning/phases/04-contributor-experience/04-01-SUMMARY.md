---
phase: 04-contributor-experience
plan: 01
subsystem: docs
tags: [contributing, security-policy, readme, badges, documentation]
requires:
  - phase: 03-ci-pipeline
    provides: canonical ci.yml badge pattern and make-based contributor workflow surface
provides:
  - task-first contributor guide at the repository root
  - coordinated disclosure policy with private and email reporting paths
  - minimal three-badge README trust row with contributor and security doc links
affects: [readme, contributor-docs, security-policy, contributor-experience]
tech-stack:
  added: []
  patterns:
    - task-first contributor documentation
    - minimal trust badges under the README title
key-files:
  created:
    - CONTRIBUTING.md
    - SECURITY.md
    - .planning/phases/04-contributor-experience/04-01-SUMMARY.md
  modified:
    - README.md
key-decisions:
  - "Kept CONTRIBUTING.md focused on the existing Makefile targets and surfaced `make help` as the discovery step."
  - "Used conditional GitHub private vulnerability reporting language with `security@amikos.tech` as the stable fallback."
  - "Locked the README trust row to exactly CI, Go Reference, and MIT, then added a single discovery sentence below it."
patterns-established:
  - "Contributor guidance pattern: root CONTRIBUTING.md documents literal make targets and links back to README for product usage."
  - "Security policy pattern: SECURITY.md names private GitHub reporting when available, preserves email fallback, and forbids public disclosure."
requirements-completed: [CONTR-01, CONTR-02, CONTR-03]
duration: 13 min
completed: 2026-04-12
---

# Phase 04 Plan 01 Summary

**Task-first contributor docs, coordinated disclosure guidance, and a three-badge README trust row for the public repository surface**

## Performance

- **Duration:** 13 min
- **Started:** 2026-04-12T19:52:00Z
- **Completed:** 2026-04-12T20:05:36Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments
- Added a root `CONTRIBUTING.md` that starts with `make help` and documents the exact local contributor commands.
- Added a root `SECURITY.md` with a private-first disclosure policy, fallback contact, and supported-version guidance.
- Extended the README header to the exact CI, Go Reference, and MIT badge set and linked contributors to the new docs.

## Task Commits

Each task was committed atomically:

1. **Task 1: Create a task-first contributor guide with explicit `make help` discovery and fenced command blocks** - `7f5f258` (docs)
2. **Task 2: Publish a real security disclosure policy without inventing a bug bounty program** - `f558bdd` (docs)
3. **Task 3: Extend the README trust row to exactly CI, Go Reference, and MIT with less brittle verification** - `e1d1ebb` (docs)

**Plan metadata:** Summary-only docs commit recorded after this summary file is written.

## Files Created/Modified
- `CONTRIBUTING.md` - Root contributor workflow guide for setup, checks, PR flow, and security routing.
- `SECURITY.md` - Root coordinated disclosure policy with private GitHub reporting guidance and email fallback.
- `README.md` - Minimal three-badge trust row and a single discovery sentence for contributor and security docs.
- `.planning/phases/04-contributor-experience/04-01-SUMMARY.md` - Execution summary for Phase 04 Plan 01.

## Decisions Made
- Kept the contributor guide aligned to the existing Makefile contract instead of introducing alternate command surfaces.
- Used exact low-overhead disclosure wording with private GitHub reporting preferred when available and `security@amikos.tech` as fallback.
- Preserved the README product narrative and limited the trust row to the locked three-badge set.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Wave 1 completed the documentation surfaces required before dependency automation work begins.
- Wave 2 can now add the root-only Dependabot configuration and pause at the planned merge checkpoint.

## Self-Check: PASSED

- Verified all `CONTRIBUTING.md` acceptance criteria with exact `rg`, `sed`, and `awk` checks.
- Verified all `SECURITY.md` acceptance criteria, including fallback email, no-public-disclosure language, and no bounty wording.
- Verified the README badge row contains exactly the three required badges plus the discovery sentence.
- Verified `go test ./... -run TestQueryEQ -count=1` and `make help` both succeed.

---
*Phase: 04-contributor-experience*
*Completed: 2026-04-12*
