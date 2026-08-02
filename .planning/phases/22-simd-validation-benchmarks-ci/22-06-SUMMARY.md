---
phase: 22-simd-validation-benchmarks-ci
plan: 06
subsystem: benchmarking
tags: [simdjson, benchstat, controlled-benchmark, parity, performance-evidence]

# Dependency graph
requires:
  - phase: 22-simd-validation-benchmarks-ci
    plan: 05
    provides: Same-process paired stdlib/SIMD typed-sink benchmark and bench-simd target
  - phase: 22-simd-validation-benchmarks-ci
    plan: 07
    provides: Completed CPU-intensive CI implementation suites and non-gating trend contract
provides:
  - Controlled COUNT=10 paired smoke benchmark record with parity, environment, and source-state controls
  - Exact pinned benchstat parser projection over one 80-sample output
  - Neutral per-fixture interpretation and evidence-backed defer recommendation
affects: [22-08, SIMD-08, SIMD-09, SIMD shipping decision]

# Tech tracking
tech-stack:
  added: []
  patterns: [parity-first performance evidence, one-output paired analysis, raw-results/report split, controlled benchmark hygiene]

key-files:
  created:
    - .planning/phases/22-simd-validation-benchmarks-ci/22-SIMD-BENCHMARK-RESULTS.md
    - .planning/phases/22-simd-validation-benchmarks-ci/22-SIMD-BENCHMARK-REPORT.md
  modified: []

key-decisions:
  - "Defer the SIMD path as a shippable v1.3 performance option: parity passed, but every controlled smoke fixture favored stdlib for time, throughput, Go-heap bytes, and allocation count."
  - "Treat the committed COUNT=10 snapshot as authoritative; future shared-runner CI output remains noisy trend evidence only."
  - "Interpret B/op as Go-heap-only and define no regression threshold."

patterns-established:
  - "Controlled evidence: parity first, clean benchmark-relevant source, AC/no-warning/load controls, explicitly cleared external tier, one paired COUNT=10 output."
  - "Recommendation reports observed per-fixture values separately from bounded inference and ends in exactly one stop-table class."

requirements-completed: [SIMD-08, SIMD-09]

# Metrics
duration: 12 min
completed: 2026-08-01
---

# Phase 22 Plan 06: Controlled SIMD Benchmark Evidence Summary

**A clean 80-sample paired capture found byte/query parity but uniformly worse SIMD ingest time, throughput, Go-heap bytes, and allocation counts, supporting a defer recommendation**

## Performance

- **Duration:** 12 min
- **Started:** 2026-08-01T09:58:14Z
- **Completed:** 2026-08-01T10:10:31Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Re-ran authored golden, Phase 20 byte/query, Evaluate-matrix, malformed-attribution, and fuzz-seed parity before timing; the Phase 19 HARD stop did not trigger.
- Captured one controlled `COUNT=10`, `BENCHTIME=1s` output with external inputs explicitly unavailable: exactly 10 stdlib and 10 SIMD samples for each of four checked-in smoke fixtures, 80 lines total.
- Recorded the exact source revision, clean benchmark surface, Go/native versions, fixture identities and hashes, AC/thermal/load controls, relevant native-loading state, and raw-output fingerprint without disclosing a full environment or cache.
- Analyzed the single paired output with the approved ephemeral `golang.org/x/perf/cmd/benchstat@v0.0.0-20260709024250-82a0b07e230d -col /parser`; x/perf remained outside `go.mod`.
- Produced a neutral report that separates observations from inference and recommends deferring the SIMD performance option: time was 44.53–83.68% higher, input throughput 30.80–45.52% lower, Go-heap bytes 18.50–36.48% higher, and allocation counts 44.05–78.58% higher across the four fixtures.

## Task Commits

Each task was committed atomically:

1. **Task 1: Capture the controlled paired benchmark record** - `d8eaeb9` (docs)
2. **Task 2: Synthesize the parity-first shipping recommendation** - `2ae0460` (docs)
3. **Task 1 verification fix: Remove trailing evidence whitespace** - `fb6c598` (fix)

**Plan metadata:** committed with this summary and project tracking.

## Files Created/Modified

- `.planning/phases/22-simd-validation-benchmarks-ci/22-SIMD-BENCHMARK-RESULTS.md` - Controlled metadata, fixture provenance, parity gate, all 80 raw samples, metric semantics, and full pinned benchstat output.
- `.planning/phases/22-simd-validation-benchmarks-ci/22-SIMD-BENCHMARK-REPORT.md` - Raw-source link, parity boundary, per-fixture observed deltas and inference, noise policy, and one defer recommendation.

## Decisions Made

- **Defer the SIMD path as a shippable v1.3 performance option.** Correctness parity is green, but the complete controlled smoke matrix shows no CPU, throughput, or Go-allocation advantage.
- **Keep the committed `COUNT=10` snapshot authoritative.** A `COUNT=1` result is known to be unreliable, and shared-runner CI output remains trend-only.
- **Do not introduce a performance threshold.** The decision follows the observed workload evidence rather than a brittle numeric gate.
- **Keep memory language bounded.** `B/op` covers the Go heap only and cannot support a total-process-memory claim.

## Verification Evidence

- The pre-measurement and final parity commands both passed all scoped authored/realistic/query/malformed/fuzz cases with `AMI_GIN_SIMD_REQUIRED=1`.
- Source and dependency checks proved the benchmark-relevant tree was clean at revision `2d41929d076659c6789a73c1e71668c246baab20`, and `go.mod`/`go.sum` remained unchanged after pinned benchstat execution.
- Power was AC at 100% charge; `pmset -g therm` reported no thermal or performance warning before or after capture. Measurement waited for a transient CPU-heavy metrics process to exit.
- Artifact gates found exactly 80 smoke benchmark lines, 10 per fixture/parser arm, zero external-tier lines, all four metrics, one raw-report link, and exactly one final `Recommendation class: **defer**`.
- Final `git diff --check` across both evidence files passed.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed a trailing blank line from the raw evidence file**
- **Found during:** Overall artifact verification after Task 2
- **Issue:** `git show --check` identified one extra blank line at EOF in the Task 1 evidence commit.
- **Fix:** Removed only the trailing blank line; all 80 samples, metadata, and pinned analysis remained byte-for-byte unchanged otherwise.
- **Files modified:** `.planning/phases/22-simd-validation-benchmarks-ci/22-SIMD-BENCHMARK-RESULTS.md`
- **Verification:** Final-range `git diff --check`, cardinality checks, and the repeated parity gate passed.
- **Committed in:** `fb6c598`

**2. [Rule 1 - Bug] Corrected stale next-plan session guidance**
- **Found during:** Plan metadata self-check
- **Issue:** The state handlers advanced the current position to Plan 08 but left the prose next-step pointer on Plan 06.
- **Fix:** Updated the session guidance to execute Phase 22 Plan 08.
- **Files modified:** `.planning/STATE.md`
- **Verification:** Current position and next-step guidance both name Plan 08.
- **Committed in:** Plan metadata commit

---

**Total deviations:** 2 auto-fixed (2 bugs).
**Impact on plan:** One whitespace-only correction and one state-pointer correction; no numeric evidence, recommendation, scope, or benchmark command changed.

## Issues Encountered

- A local metrics process briefly consumed one CPU core during control preflight. The benchmark was not started until that process exited and the accepted load snapshot contained no other Go/test/benchmark or high-CPU process.
- The controlled result matched the measured prior direction rather than the hoped-for performance benefit. This is an accepted evidence outcome, not a phase failure.

## User Setup Required

None - the authoritative smoke fixtures are checked in and the analysis tool was invoked ephemerally at the approved exact version.

## Known Stubs

None.

## Threat Flags

None - this plan added evidence documents only. It introduced no network endpoint, authentication path, file-access behavior, schema boundary, persistent dependency, or executable artifact.

## Next Phase Readiness

- Plan 22-08 can review the controlled defer recommendation alongside remote five-platform CI results and required-check bindings.
- Correctness evidence remains green; the recommendation defers only the performance-shipping claim, not the documented parity result.
- No blocker remains for the final Phase 22 verification plan.

## Self-Check: PASSED

- Both evidence documents and this summary exist.
- Task commits `d8eaeb9`, `2ae0460`, and `fb6c598` are present in git history.
- Repeated parity, 80-sample cardinality, no-external, report-link, one-recommendation, dependency-diff, stub, and whitespace gates passed.
- No unexpected deletion, untracked generated file, persistent x/perf dependency, known stub, or unplanned threat surface remains.
