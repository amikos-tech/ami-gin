---
phase: 22-simd-validation-benchmarks-ci
plan: 08
subsystem: ci
tags: [simdjson, github-actions, rulesets, benchmark-evidence, human-approval]

# Dependency graph
requires:
  - phase: 22-simd-validation-benchmarks-ci
    plan: 06
    provides: Controlled COUNT=10 benchmark results and the defer recommendation
  - phase: 22-simd-validation-benchmarks-ci
    plan: 07
    provides: Five stable SIMD matrix contexts and the required/advisory policy
provides:
  - External merge policy bound to exactly the two required SIMD contexts
  - Recorded remote five-leg evidence for the Plan 22-07 workflow revision
  - Human acceptance of the controlled SIMD benchmark recommendation (defer)
affects: [v1.3-close, ci-policy, simd-roadmap]

# Tech tracking
tech-stack:
  added: []
  patterns: [external-ruleset binding verified read-only after an authorized admin change]

key-files:
  created: []
  modified: []

key-decisions:
  - "The Main ruleset now requires exactly `SIMD (linux/amd64, required)` and `SIMD (darwin/arm64, required)`; the three advisory contexts stay non-blocking."
  - "The controlled benchmark recommendation class `defer` is accepted: parity passed, but every fixture favored stdlib on time, throughput, Go-heap bytes, and allocations."
  - "The security-scan segment is evaluated on a toolchain at or above go1.26.9; go1.25.5 reports fixed stdlib advisories that CI (Go 1.26) does not."

patterns-established:
  - "Ruleset mutation is a separately authorized admin action recorded in a todo; the plan checkpoint only inspects the result."

requirements-completed: [SIMD-08, SIMD-09, SIMD-10, SIMD-11]

# Metrics
duration: 25min
completed: 2026-10-19
---

# Phase 22: SIMD Validation, Benchmarks & CI Summary

**Merge policy now requires the two SIMD legs, remote five-leg evidence is recorded, and the controlled `defer` recommendation is accepted.**

## Performance

- **Duration:** ~25 min (gate ~10 min, evidence collection, ruleset verification)
- **Started:** 2026-10-19T09:15:00Z
- **Completed:** 2026-10-19T09:45:00Z
- **Tasks:** 3 completed (1 automated gate, 1 human-action, 1 human-verify)
- **Files modified:** 0 repository files (planning artifacts only)

## Accomplishments

- Full Phase 22 gate green: `go test ./...`, `AMI_GIN_SIMD_REQUIRED=1 go test -tags simdjson -race -timeout=30m ./...` (463 s), and `make security-scan` (both `govulncheck` passes, 0 vulnerabilities).
- Remote evidence recorded for PR #55 run `30734887913` at head `57972ab131360c4c5ad178c09cf74471cb3ab95c` (the Plan 22-07 workflow revision; PR merged 2026-08-02 as `c7f9b58`).
- The `Main` ruleset (id 14266305) gained a `required_status_checks` rule binding exactly the two required-tier contexts. Read-only inspection confirmed the rule and confirmed the other five rules are unchanged.
- The controlled report's single recommendation class, **defer**, was accepted by the project owner on 2026-10-19.

## Remote Matrix Outcomes

Run `30734887913` (PR #55 head `57972ab`):

| Context | Tier | Outcome |
|---------|------|---------|
| SIMD (linux/amd64, required) | required | success |
| SIMD (darwin/arm64, required) | required | success |
| SIMD (linux/arm64, advisory) | advisory | success |
| SIMD (darwin/amd64, advisory) | advisory | success |
| SIMD (windows/amd64, advisory) | advisory | failure (advisory, recorded) |

Latest main run `37498278533` at `4d573e5` (v1.5.0): all five contexts success. Each leg's log shows the resolved `PURE_SIMDJSON_LIB_PATH` under the effective-version cache directory and two tagged package passes (normal and explicit-path) with no supported-host skip lines. `AMI_GIN_SIMD_REQUIRED=1` is set job-wide in `ci.yml` and asserted by `TestSIMDWorkflowContract`.

## Ruleset Binding

```
required_status_checks:
  strict_required_status_checks_policy: false
  do_not_enforce_on_create: false
  contexts:
    - SIMD (linux/amd64, required)   (integration 15368)
    - SIMD (darwin/arm64, required)  (integration 15368)
```

No advisory SIMD context is required. Unrelated rules (deletion, non_fast_forward, required_signatures, pull_request, required_linear_history) are unchanged.

## Controlled Evidence Review

- D-04 qualified parity claim retained verbatim in `22-SIMD-BENCHMARK-REPORT.md`; the malformed attribution asymmetry remains the tested exclusion.
- All four fixtures trace to the one `COUNT=10` capture in `22-SIMD-BENCHMARK-RESULTS.md` and its exact benchstat output. Geomeans: +64.57% time, -39.22% throughput, +28.50% Go-heap bytes, +56.12% allocations, all favoring stdlib.
- Shared-runner trend artifacts are non-gating and trend-only. No regression threshold exists.
- Recommendation class: **defer**. Accepted.

## Task Commits

No task commits: this plan modifies no repository files. Planning artifacts are committed with the phase close.

## Decisions Made

- Accept `defer`: keep stdlib the default, keep the parity and CI evidence, and require new controlled profiling or a materially different representative workload before presenting SIMD as a performance benefit.
- Backlog Phase 999.7 (SIMD runtime fallback DX) is moot under `defer` and is dropped at milestone close.

## Deviations from Plan

**1. Ruleset mutation happened outside the checkpoint, under separate authority.** The plan forbids mutating rules during Task 3. The pending todo `2026-08-02-bind-required-simd-checks-in-ruleset.md` recorded that an administrator must bind the rule first. The project owner authorized that change on 2026-10-19; it was applied, then Task 3 inspected the result read-only. The todo is moved to `todos/completed/` with a resolution note.

**2. Security scan toolchain.** The first `make security-scan` pass ran under `GOTOOLCHAIN=go1.25.5` and reported eight Go standard-library advisories (GO-2026-6605..6617) fixed in go1.26.9. CI runs Go 1.26. The segment was rerun on the local go1.27.2 toolchain and passed with 0 vulnerabilities. No source change.

## Issues Encountered

The initial Plan 22-07 remote run had an advisory `windows/amd64` failure; the latest main run shows all five legs green. Advisory outcomes do not gate merge, as designed.

## User Setup Required

None beyond the admin ruleset change recorded above.

## Next Phase Readiness

Phase 22 is complete. v1.3 can close. Phases 23-25 are unplanned and move to the backlog at milestone close.
