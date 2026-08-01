---
phase: 22-simd-validation-benchmarks-ci
plan: 05
subsystem: benchmarking
tags: [simdjson, go-benchmark, typed-sink, benchmark-hygiene, make]

# Dependency graph
requires:
  - phase: 22-simd-validation-benchmarks-ci
    plan: 02
    provides: Required-host construction policy plus byte-identical realistic and query parity
  - phase: 22-simd-validation-benchmarks-ci
    plan: 03
    provides: Bounded deterministic SIMD differential seed replay
  - phase: 22-simd-validation-benchmarks-ci
    plan: 04
    provides: Executable SIMD deployment guidance and check-simd-docs Make contract
  - phase: 20-realistic-benchmark-dataset-foundation
    provides: Offline smoke fixtures, bounded external loader, and shared row-group packing helper
provides:
  - Same-process paired stdlib/SIMD typed-sink ingest benchmark over all Phase 20 fixture tiers
  - Per-arm steady-state ns/op, input MB/s, Go-heap B/op, and allocs/op metrics
  - Narrow bench-simd Make target with preserved BENCHTIME, BENCH_TIMEOUT, and COUNT controls
affects: [22-06, 22-07, SIMD-09, benchmark-evidence]

# Tech tracking
tech-stack:
  added: []
  patterns: [same-process paired benchmarks, shared sequential parser ownership, setup-outside-timing, key-value benchmark names]

key-files:
  created:
    - benchmark_simd_test.go
  modified:
    - Makefile

key-decisions:
  - "The paired benchmark explicitly selects stdlibParser and one caller-owned SIMD parser through the same Phase 20 build helper and row-group packing."
  - "Input throughput is the sum of JSON document bytes per iteration; B/op is labeled as Go-heap-only and excludes native parser buffers."
  - "Optional external discovery remains inside one dedicated parent and uses only GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL and GIN_PHASE20_SIMDJSON_DIR."

patterns-established:
  - "Paired benchmark leaves: tier=<tier>/fixture=<name>/parser=stdlib|simd keeps both arms adjacent and benchstat-ready in one output file."
  - "Steady-state boundary: construct the shared parser and load fixtures before leaf ResetTimer, then measure only build/finalize work."

requirements-completed: [SIMD-09]

# Metrics
duration: 8 min
completed: 2026-08-01
---

# Phase 22 Plan 05: Paired SIMD Typed-Sink Benchmark Summary

**A same-process stdlib/SIMD ingest benchmark with realistic offline fixtures, input-throughput and Go-heap metrics, plus one narrowly anchored contributor command**

## Performance

- **Duration:** 8 min
- **Started:** 2026-08-01T09:32:03Z
- **Completed:** 2026-08-01T09:40:03Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Added `BenchmarkSIMDTypedSinkIngest` with adjacent stdlib and SIMD leaves for all four checked-in Phase 20 fixtures in one tagged process.
- Reused one caller-owned SIMD parser sequentially, preserving Spike 003's no-bias lifecycle result while keeping parser construction and fixture I/O outside timed leaf regions.
- Reported `ns/op`, input-JSON `MB/s`, Go-heap `B/op`, and `allocs/op` for every arm through `ReportAllocs`, `SetBytes`, and `ResetTimer`.
- Kept the optional bounded external corpus behind the exact Phase 20 opt-in variables and isolated its disabled skip to a dedicated subtree.
- Added `make bench-simd` with the SIMD tag, empty test selector, anchored benchmark selector, benchmark memory metrics, and existing duration/timeout/count overrides.

## Task Commits

Each task was committed atomically:

1. **Task 1 RED: Specify the paired typed-sink benchmark** - `b33dad5` (test)
2. **Task 1 GREEN: Implement the paired typed-sink benchmark** - `d6b9f0d` (feat)
3. **Task 2: Add the scoped bench-simd Make target** - `7ba2838` (chore)

**Plan metadata:** committed with this summary.

## Files Created/Modified

- `benchmark_simd_test.go` - Tagged paired benchmark, shared sequential parser lifecycle, input-byte accounting, smoke fixture leaves, and optional external subtree.
- `Makefile` - Anchored `bench-simd` target and contributor help for smoke/default and external opt-in tiers.

## Decisions Made

- Used explicit `stdlibParser{}` and the shared SIMD parser as options to the same `phase20BuildBenchmarkIndex` helper so both arms retain identical builder configuration and row-group packing.
- Counted throughput from the loaded JSON document bytes, not encoded-index bytes or JSONL delimiters, matching the ingest payload processed each iteration.
- Retained a single parser across fixture leaves because Spike 003 measured no order or shared-vs-fresh bias; no artificial warm-up or per-fixture construction was added.
- Kept `COUNT ?= 1` for contributor smoke runs. The authoritative `COUNT=10` capture remains reserved for plan 22-06.

## Verification Evidence

- The pre-benchmark hard gate passed for authored goldens, four Phase 20 byte/query differentials, the shared Evaluate matrix, malformed failure-layer attribution, and all committed `FuzzParserParity` seeds.
- The direct benchmark and `make bench-simd COUNT=1 BENCHTIME=100ms` each produced exactly eight smoke leaves: four fixtures times `parser=stdlib|simd`.
- Every smoke leaf exposed `ns/op`, `MB/s`, `B/op`, and `allocs/op`; source checks proved `SetBytes` receives the sum of input document lengths.
- With both external variables unset, the dedicated `tier=external/fixture=local-example` parent skipped while every smoke result completed.
- Make contract checks proved the selector is anchored, `BENCHTIME=1s`, `BENCH_TIMEOUT=30m`, and `COUNT=1` remain defaults, all three controls are overridable, and required targets plus `check-simd-docs` remain present.
- `go test ./...`, `make simd-isolation-check`, and `TestSIMDConstructorCallPolicy` passed after both task commits.
- `go.mod` and `go.sum` are unchanged; no dependency, production parser, fixture, threshold, or persistent benchmark tool was added.

## TDD Gate Compliance

- Task 1 RED commit `b33dad5` failed on the intentionally missing paired benchmark runner.
- Task 1 GREEN commit `d6b9f0d` followed RED and passed parity, benchmark, metric, offline-tier, default-suite, and isolation gates.
- No refactor commit was needed after GREEN.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - checked-in smoke fixtures run offline. The local external tier remains optional and uses the existing Phase 20 variables documented by `make help`.

## Known Stubs

None.

## Next Phase Readiness

- Plan 22-07 can call the narrow target for a non-gating trend artifact without pulling in the full tagged benchmark suite.
- Plan 22-06 can perform the controlled `COUNT=10` capture against the paired output and synthesize the evidence-backed ship/defer/narrow recommendation.
- Correctness remains green and no performance conclusion is asserted from this plan's `COUNT=1` smoke runs.

## Self-Check: PASSED

- Both implementation files and this summary exist; task commits `b33dad5`, `d6b9f0d`, and `7ba2838` are present in git history.
- Every task acceptance criterion and plan-level verification command passed after the final task commit.
- Stub and threat-surface scans found no incomplete goal behavior or unplanned trust boundary; T-22-04 remains bounded by the existing Phase 20 loader and opt-in variables.

---
*Phase: 22-simd-validation-benchmarks-ci*
*Completed: 2026-08-01*
