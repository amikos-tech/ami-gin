---
phase: 22-simd-validation-benchmarks-ci
plan: 07
subsystem: ci
tags: [simdjson, github-actions, matrix, cache, benchmark-trend, govulncheck]

# Dependency graph
requires:
  - phase: 22-simd-validation-benchmarks-ci
    plan: 03
    provides: Bounded SIMD differential corpus and stable malformed-input exclusion
  - phase: 22-simd-validation-benchmarks-ci
    plan: 04
    provides: Executable check-simd-docs contract and deployment guidance
  - phase: 22-simd-validation-benchmarks-ci
    plan: 05
    provides: Narrow paired bench-simd target and text benchmark output
provides:
  - Exact five-platform SIMD CI matrix with two required race legs and three advisory non-race legs
  - Effective-version and platform-specific native cache plus validated explicit-path replay
  - Main-or-dispatch non-gating text-only benchmark trend artifact with noise disclaimer
  - Default and simdjson-tagged govulncheck call-graph coverage
affects: [22-06, 22-08, SIMD-09, SIMD-10, SIMD-11, ci-policy]

# Tech tracking
tech-stack:
  added: [actions/cache@v5, ephemeral golang.org/x/perf benchstat]
  patterns: [explicit matrix include policy, fail-closed cache-path resolution, event-guarded advisory evidence, dependency-free workflow contract]

key-files:
  created:
    - simd_workflow_contract_test.go
  modified:
    - .github/workflows/ci.yml
    - Makefile

key-decisions:
  - "Required-host failure policy and leak warnings are job-wide, while runner.temp-derived cache paths are step-local because GitHub does not expose the runner context to job-level env."
  - "workflow_dispatch intentionally exposes the full CI workflow to manual runs; the benchmark trend job alone is additionally guarded to dispatch or push-main."
  - "The second SIMD pass consumes one test-f-validated cache file through PURE_SIMDJSON_LIB_PATH and never invokes bootstrap or another download path."

patterns-established:
  - "Workflow contract scope: inspect the whole workflow only for triggers, least privilege, and preserved jobs; use ciJobSection and workflowStepSection for job-local policy."
  - "Shared-runner evidence: COUNT=1 text artifacts are non-gating and explicitly noisy; controlled committed Phase 22 evidence remains authoritative."

requirements-completed: [SIMD-09, SIMD-10, SIMD-11]

# Metrics
duration: 9 min
completed: 2026-08-01
---

# Phase 22 Plan 07: Five-Platform SIMD CI and Operational Evidence Summary

**Five supported SIMD platforms now execute under an exact required/advisory policy, replay through a validated native-cache path, and emit clearly non-authoritative trend text off the PR path**

## Performance

- **Duration:** 9 min
- **Started:** 2026-08-01T09:44:40Z
- **Completed:** 2026-08-01T09:53:32Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Replaced the single Linux SIMD job with five exact include rows: required linux/amd64 and darwin/arm64 race legs plus advisory linux/arm64, darwin/amd64, and windows/amd64 non-race legs, all with required construction and fail-fast disabled.
- Added exact effective-version/runner cache isolation with no restore prefixes, upstream-owned bootstrap, and a fail-closed resolver that validates the matrix-specific cached library before a focused explicit-path parity replay.
- Preserved default test, lint, build/isolation, and vulnerability jobs while adding the SIMD documentation contract before golangci-lint.
- Added a Linux/amd64 benchmark trend job limited to push-main or workflow dispatch, non-gating by construction, with cleared external-corpus variables, pinned benchstat, a shared-runner-noisy disclaimer, and only two text outputs.
- Extended the canonical security target to scan both default and `simdjson`-tagged Go call graphs without loading the native library.
- Added one dependency-free Go contract covering exact matrix rows, cache semantics, explicit loading, least privilege, triggers, artifact boundaries, lint order, and Make security policy.

## Task Commits

Each task followed RED/GREEN TDD and was committed atomically:

1. **Task 1 RED: Specify the five-platform matrix and cache contract** - `34789c0` (test)
2. **Task 1 GREEN: Enforce the five-platform SIMD matrix** - `11254c5` (feat)
3. **Task 2 RED: Specify explicit loading, trend, docs, and security policy** - `afc556e` (test)
4. **Task 2 GREEN: Prove loading and publish non-gating trend data** - `28a6156` (feat)

**Plan metadata:** committed with this summary and project tracking.

## Files Created/Modified

- `simd_workflow_contract_test.go` - Exact matrix parser plus workflow, step, artifact, least-privilege, and Make-policy regression tests.
- `.github/workflows/ci.yml` - Five-platform SIMD matrix, exact cache contract, explicit-path smoke, docs guard, and guarded benchmark trend job.
- `Makefile` - Default plus tagged govulncheck scans and updated contributor help.

## Decisions Made

- Used one `matrix.include` with stable `SIMD (<goos>/<goarch>, <tier>)` names so repository rules can bind only the two required checks in Plan 22-08.
- Kept the CI-required flag and leak warning at job scope, but placed the dynamic cache directory on each consuming step after `actionlint` proved `runner.temp` is unavailable to job-level `env`.
- Resolved the explicit library from the effective module version, matrix OS/architecture, and upstream-confirmed on-disk filename; the focused replay has no bootstrap, mirror, checksum, fallback, or downloader seam.
- Kept all manually triggered default jobs intact and added an event guard only to `simd-benchmark-trend`, matching GitHub's workflow-level dispatch model.
- Used `actions/cache@v5`, matching the current official action major and the repository's Node 24-era action majors.

## Verification Evidence

- `go test -run '^TestSIMDWorkflowContract$' .` passed all matrix/cache, explicit-path/docs, trend-artifact, and security-scan subtests.
- `make check-simd-docs` passed and the lint-job contract proves it runs before golangci-lint.
- `go test ./...` passed across the root package, CLI, examples, logging adapters, and telemetry.
- `make simd-isolation-check` passed `go build ./...`, default dependency isolation, and `go vet ./...`.
- `actionlint .github/workflows/ci.yml` passed after the dynamic cache variable was scoped to valid step contexts.
- A corrected exact multiline `rg` check and `make -n security-scan` proved the canonical scan order is default first, then one `-tags simdjson` invocation.
- Final workflow scan found exactly five matrix runners, two required rows, three advisory rows, and no restore key, schedule, threshold, or regression gate.

## TDD Gate Compliance

- Task 1 RED commit `34789c0` failed because the original workflow had no `matrix.include`; GREEN commit `11254c5` made the exact five-row and cache contract pass.
- Task 2 RED commit `afc556e` failed on the absent explicit-path resolver, trend job, workflow dispatch, docs order, and tagged security scan; GREEN commit `28a6156` made the full contract pass.
- No refactor commits were needed after either GREEN gate.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Moved runner-derived cache paths to valid workflow contexts**
- **Found during:** Task 2 GREEN verification
- **Issue:** `actionlint` showed that `${{ runner.temp }}` is unavailable in job-level `env`, so the initial workflow would have been rejected even though the textual contract passed.
- **Fix:** Kept literal enforcement variables job-wide and moved `PURE_SIMDJSON_CACHE_DIR` to the bootstrap, resolver, test, explicit-path, and benchmark steps that consume it.
- **Files modified:** `.github/workflows/ci.yml`
- **Verification:** `actionlint .github/workflows/ci.yml` and the full workflow contract both pass.
- **Committed in:** `28a6156`

---

**Total deviations:** 1 auto-fixed (1 bug).
**Impact on plan:** The fix preserves the exact cache path and five-row policy while making the workflow valid on GitHub Actions. No scope expansion.

## Issues Encountered

- The plan's literal multiline `rg` verifier escaped only two of the three dots in `./...`, so it could not match the required Make recipe. The corrected execution check escaped all three dots and passed; the executable Go contract independently verifies the same exact two-command order.

## User Setup Required

None - no external service configuration is required. Plan 22-08 owns remote matrix and repository-ruleset verification.

## Known Stubs

None.

## Next Phase Readiness

- Plan 22-06 can capture controlled `COUNT=10` benchmark evidence against the now-canonical `bench-simd` workflow and Make target.
- Plan 22-08 can verify the two stable required check names, three advisory outcomes, remote five-leg execution, and the controlled evidence approval.
- Required/advisory behavior in YAML does not itself mutate repository rules; that external binding remains deliberately deferred to the blocking Plan 22-08 checkpoint.

## Self-Check: PASSED

- The summary, workflow, workflow contract, and Makefile all exist, and task commits `34789c0`, `11254c5`, `afc556e`, and `28a6156` are present in git history.
- The final workflow contract passed with exactly five rows, the 2/3 required/advisory split, and no restore key, schedule, threshold, or regression gate.
- Stub and threat-surface scans found no incomplete behavior or unplanned trust boundary; T-22-01, T-22-02, and T-22-04 are covered by the planned workflow and Make contracts.

---
*Phase: 22-simd-validation-benchmarks-ci*
*Completed: 2026-08-01*
