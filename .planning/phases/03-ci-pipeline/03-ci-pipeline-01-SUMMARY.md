---
phase: 03-ci-pipeline
plan: 01
subsystem: infra
tags: [github-actions, ci, golangci-lint, govulncheck, makefile, gotestsum]
requires: []
provides:
  - contributor-facing CI command surface in Makefile
  - expanded lint policy that runs clean on the repo
  - canonical GitHub Actions CI workflow with stable required-check names
affects: [github-actions, makefile, linting, examples, readme]
tech-stack:
  added: [gotestsum@v1.13.0, golangci/golangci-lint-action@v9, golang/govulncheck-action@v1]
  patterns: [keep contributor commands in make, keep CI orchestration in workflow yaml, use exact job IDs for merge-gate contexts]
key-files:
  created:
    - .github/workflows/ci.yml
    - .planning/phases/03-ci-pipeline/03-ci-pipeline-01-SUMMARY.md
  modified:
    - Makefile
    - .golangci.yml
    - cmd/gin-index/main.go
    - parquet.go
    - serialize.go
key-decisions:
  - "Pinned gotestsum to v1.13.0 in both Makefile and CI so test tooling does not drift over time."
  - "Kept lint, build, govulncheck, and matrix test orchestration in ci.yml while preserving make as the contributor-facing command surface."
  - "Used the exact job IDs test, lint, build, and govulncheck so GitHub ruleset enforcement can bind to stable check names later in the phase."
patterns-established:
  - "CI contract pattern: make exposes local commands, ci.yml owns runner-specific orchestration, GitHub artifacts, and GitHub-native coverage summaries."
  - "Lint remediation pattern: fix all repo entrypoints that surface under the expanded linter set, not just core-library files."
requirements-completed: [CI-01, CI-02, CI-03, CI-04]
duration: 54 min
completed: 2026-04-11
---

# Phase 03 Plan 01: CI Contract and Workflow Summary

**Pinned make targets, expanded lint enforcement, and a canonical CI workflow with stable GitHub check contexts**

## Performance

- **Duration:** 54 min
- **Started:** 2026-04-11T14:41:54Z
- **Completed:** 2026-04-11T15:35:35Z
- **Tasks:** 4
- **Files modified:** 20

## Accomplishments

- Expanded `Makefile` with pinned `gotestsum`, `integration-test`, and `security-scan` while preserving the existing contributor command surface.
- Enabled the required golangci-lint policy and cleared the resulting `errorlint`, `unconvert`, `unparam`, and `gosec` findings across library, CLI, and example entrypoints.
- Added `.github/workflows/ci.yml` with the exact `test`, `lint`, `build`, and `govulncheck` jobs, GitHub-native coverage summary wiring, and artifact upload behavior expected by later ruleset enforcement.

## Task Commits

Each task was committed atomically:

1. **Task 1: Expand the local CI command surface in Makefile** - `4053a98` (chore)
2. **Task 2: Upgrade golangci-lint policy and fix core-source findings for CI-02** - `3aaeca0` (fix)
3. **Task 3: Fix CLI and example lint findings so the expanded lint set is fully green** - `5dd4c01` (fix)
4. **Task 4: Create the PR and main CI workflow** - `b7e0207` (ci)

## Files Created/Modified

- `Makefile` - Pins `gotestsum` and adds the required `integration-test` and `security-scan` targets.
- `.golangci.yml` - Enables the required linter set and preserves the existing v2 config structure.
- `.github/workflows/ci.yml` - Defines the canonical PR/push CI workflow, matrix test job, GitHub artifact uploads, GitHub-native coverage summary, lint job, build job, and blocking `govulncheck` job.
- `cmd/gin-index/main.go` - Tightens file-permission and file-read handling to satisfy `gosec`.
- `parquet.go`, `serialize.go`, `hyperloglog.go`, `gin_test.go`, `serialize_security_test.go` - Resolve core-library and test-suite findings from the expanded lint policy.
- `examples/basic/main.go`, `examples/full/main.go`, `examples/fulltext/main.go`, `examples/nested/main.go`, `examples/null/main.go`, `examples/parquet/main.go`, `examples/range/main.go`, `examples/regex/main.go`, `examples/serialize/main.go`, `examples/transformers/main.go`, `examples/transformers-advanced/main.go` - Stop ignoring errors so sample entrypoints are clean under `gosec`.

## Decisions Made

- Pinned `gotestsum` to `v1.13.0` in both local and CI execution paths to keep test output and artifact generation reproducible.
- Let `golangci-lint-action` bootstrap lint tooling on runners instead of routing lint through `make`, which keeps local ergonomics and CI setup concerns separate.
- Used top-level `GOTOOLCHAIN: local` and the exact `test (1.25)` / `test (1.26)` matrix naming pattern so GitHub branch protection can require the intended checks without ambiguity.

## Deviations from Plan

### Auto-fixed Issues

**1. [Expanded lint scope] Cleared additional gosec findings outside the plan's original example subset**
- **Found during:** Task 3 (CLI and example lint cleanup)
- **Issue:** A full `golangci-lint run` still surfaced `G104` findings in example entrypoints beyond the narrower remediation list in the plan.
- **Fix:** Extended the cleanup to `examples/basic`, `examples/full`, `examples/fulltext`, `examples/null`, and `examples/transformers`, plus the already planned example files, so the repo-wide lint run is genuinely green.
- **Files modified:** `examples/basic/main.go`, `examples/full/main.go`, `examples/fulltext/main.go`, `examples/null/main.go`, `examples/transformers/main.go`, and the planned example files
- **Verification:** `golangci-lint run`
- **Committed in:** `5dd4c01`

**2. [Verification mismatch] Validated the real CI race command instead of the default local timeout path**
- **Found during:** Task 4 (CI workflow verification)
- **Issue:** Plain `go test -race ./...` timed out after Go's default 10-minute package timeout in an existing property-test package, even though the planned CI workflow intentionally runs with `-timeout=30m`.
- **Fix:** Verified the workflow-equivalent command with pinned `gotestsum` and `-timeout=30m`, matching the exact behavior encoded in `.github/workflows/ci.yml`.
- **Files modified:** None
- **Verification:** `$(go env GOPATH)/bin/gotestsum --format short-verbose --packages="./..." --junitfile unit.xml -- -race -coverprofile=coverage.out -timeout=30m ./...`
- **Committed in:** Not code-changing; captured here for verification traceability

---

**Total deviations:** 2 auto-fixed
**Impact on plan:** Both deviations were required to make the repo-wide lint and CI workflow claims true. No functional scope was added beyond making the planned gates executable.

## Issues Encountered

- The repo's property-test suite exceeds Go's default 10-minute timeout under `go test -race ./...`. The CI workflow already uses `-timeout=30m`, and the workflow-equivalent command passed with that setting.

## User Setup Required

None - no external service configuration is required to land this plan's repository-local changes.

## Next Phase Readiness

- Wave 1 now provides the exact CI workflow and check-context names that Plan 03 will later verify on a real PR and enforce through the GitHub ruleset.
- README and SARIF reporting work from Plan 02 can build directly on this `ci.yml` contract.
- The remaining Phase 3 work is external-enforcement verification and GitHub Code Security setup, not additional local CI wiring.

## Self-Check: PASSED

- `actionlint .github/workflows/ci.yml .github/workflows/security.yml`
- `golangci-lint run`
- `govulncheck ./...`
- `$(go env GOPATH)/bin/gotestsum --format short-verbose --packages="./..." --junitfile unit.xml -- -race -coverprofile=coverage.out -timeout=30m ./...`

---
*Phase: 03-ci-pipeline*
*Completed: 2026-04-11*
