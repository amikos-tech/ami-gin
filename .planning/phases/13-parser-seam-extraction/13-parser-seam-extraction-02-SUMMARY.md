---
phase: 13-parser-seam-extraction
plan: 02
subsystem: api
tags: [parser, builder, benchmark, goldens, testing]
requires:
  - phase: 13
    provides: additive parser seam surface
provides:
  - AddDocument wired through the parser seam
  - runtime guards for parser contract violations
  - committed authored parity goldens plus focused benchmark evidence
affects: [builder, parser, telemetry, parity-tests, phase-14]
tech-stack:
  added: []
  patterns: [default parser fast path with custom-parser guard path, build-tagged golden regeneration, focused benchmark gating]
key-files:
  created:
    - parser_parity_fixtures_test.go
    - parity_goldens_test.go
    - testdata/parity-golden/README.md
    - testdata/parity-golden/int64-boundaries.bin
    - testdata/parity-golden/nulls-and-missing.bin
    - testdata/parity-golden/deep-nested.bin
    - testdata/parity-golden/unicode-keys.bin
    - testdata/parity-golden/empty-arrays.bin
    - testdata/parity-golden/large-strings.bin
    - testdata/parity-golden/transformers-iso-date-and-lower.bin
    - .planning/phases/13-parser-seam-extraction/13-02-BASELINE.txt
    - .planning/phases/13-parser-seam-extraction/13-02-BENCH.md
    - .planning/phases/13-parser-seam-extraction/13-parser-seam-extraction-02-SUMMARY.md
  modified:
    - builder.go
    - parser_stdlib.go
    - parser_test.go
    - gin_test.go
key-decisions:
  - "Default stdlibParser path uses a concrete helper inside AddDocument so the seam stays fast while custom parsers still go through b.parser.Parse with runtime guards."
  - "Benchmark evidence is captured with focused GOMAXPROCS=1 probes because the broad parent benchmarks were scheduler-noisy in this runtime."
  - "Parity goldens are stored as compressed Encode() outputs on disk; README documents the GINc wrapper and inner GIN\\x01 header."
patterns-established:
  - "Guarded parser dispatch pattern: trusted stdlib path can use a concrete fast path, while custom parser implementations pay the full contract-validation cost."
  - "Parity artifact pattern: shared fixture corpus in an untagged *_test.go file plus a build-tagged regenerator that writes committed goldens."
requirements-completed: [PARSER-01]
duration: 29 min
completed: 2026-04-21
---

# Phase 13 Plan 02: Parser Wire-Up Summary

**AddDocument now routes through the parser seam with committed parity artifacts and no representative seam-path benchmark regression**

## Performance

- **Duration:** 29 min
- **Started:** 2026-04-21T10:49:11Z
- **Completed:** 2026-04-21T11:18:30Z
- **Tasks:** 4
- **Files modified:** 16

## Accomplishments

- Wired `NewBuilder` and `AddDocument` through the parser seam, including exact parser-name caching and runtime guards for skipped or mismatched `BeginDocument` calls.
- Added the plan-02 regression tests, updated the old explicit-parser assertion to the new seam contract, and kept `go test ./...` green after the hot-path switch.
- Captured focused v1.0 benchmark baselines, wrote the benchmark delta artifact, and generated the seven authored parity goldens plus the shared fixture corpus for plan 13-03.

## Task Commits

Plan 02 landed as one implementation commit because the hot-path switch, the guard tests, the benchmark artifacts, and the generated parity goldens are one reviewable unit.

1. **Tasks 1-4: NewBuilder/AddDocument wire-up, guard tests, benchmark artifacts, and parity goldens** - `2a563f3` (refactor/test)

## Files Created/Modified

- `builder.go` - Defaults `b.parser`, caches `parserName`, routes the trusted stdlib path directly, and keeps the runtime guard path for non-stdlib parsers.
- `parser_stdlib.go` - Splits the trusted concrete parse helper from the generic sink-contract path.
- `parser_test.go` - Adds the wire-up, error-string, and BeginDocument guard regressions for the seam.
- `gin_test.go` - Re-targets the explicit-parser contract test from the old builder internals to the new builder-plus-stdlib-parser split.
- `parser_parity_fixtures_test.go`, `parity_goldens_test.go`, `testdata/parity-golden/*` - Define the authored corpus and the committed byte-level goldens consumed by plan 13-03.
- `.planning/phases/13-parser-seam-extraction/13-02-BASELINE.txt`, [13-02-BENCH.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/13-parser-seam-extraction/13-02-BENCH.md) - Preserve the benchmark evidence that the seam path stayed allocation-flat and wall-clock-stable under the focused probe.

## Decisions Made

- Kept the runtime BeginDocument guards on the generic custom-parser path, but restored a local-state helper for the trusted `stdlibParser` path so the default hot path stays benchmark-flat.
- Recorded the benchmark gate with focused `GOMAXPROCS=1` probes after the broad parent benchmark runs proved too noisy to be decision-quality in this runtime.
- Kept the authored parity fixtures in a shared untagged test file so both the regenerator and the plan-03 assertion harness can consume the exact same corpus.

## Deviations from Plan

### Auto-fixed Issues

**1. [Performance gate] Special-cased the trusted stdlib path inside AddDocument**
- **Found during:** Plan 02 benchmark gate
- **Issue:** The first generic seam wire-up kept the right behavior but introduced benchmark drift on the default `AddDocument` path.
- **Fix:** Added a concrete stdlib helper for the trusted default parser while preserving `b.parser.Parse` plus runtime guards for custom parsers.
- **Files modified:** `builder.go`, `parser_stdlib.go`
- **Verification:** Focused benchmark probes recorded in `13-02-BENCH.md`; `go test ./... -count=1`
- **Committed in:** `2a563f3`

**2. [Benchmark methodology] Replaced noisy broad runs with focused seam-path probes**
- **Found during:** Plan 02 benchmark gate
- **Issue:** The default multi-threaded parent benchmark runs were dominated by scheduler/GC noise in this runtime and were not stable enough to act as a merge gate.
- **Fix:** Captured comparable `v1.0` and post-wire-up measurements with `GOMAXPROCS=1`, `-count=3`, `BenchmarkAddDocument`, and the explicit-number phase-07 subbenchmarks.
- **Files modified:** `.planning/phases/13-parser-seam-extraction/13-02-BASELINE.txt`, `.planning/phases/13-parser-seam-extraction/13-02-BENCH.md`
- **Verification:** Median deltas in `13-02-BENCH.md`
- **Committed in:** `2a563f3`

---

**Total deviations:** 2 auto-fixed
**Impact on plan:** No scope expansion. Both deviations were required to keep the default seam path performant and the benchmark evidence trustworthy.

## Issues Encountered

- The generated parity blobs are compressed `Encode()` outputs, so the on-disk files begin with `GINc`, not raw `GIN\x01`. The README was corrected to describe the real transport wrapper and the inner index header accurately.

## User Setup Required

None - plan 02 is repository-local refactor, benchmark, and fixture work only.

## Next Phase Readiness

- Plan 13-03 can consume the committed authored goldens and the shared fixture corpus immediately; no bootstrap regeneration step is left.
- The parser seam now exposes `parserName` for phase 14 telemetry work without altering the public query/build entry points.
- The remaining phase work is purely proof: authored-fixture parity assertions, determinism canary, and the Evaluate matrix.

## Self-Check: PASSED

- `go test -run "TestNewBuilderDefaultsToStdlibParser|TestNewBuilderRejectsEmptyParserName|TestBuilderParserNameReachable|TestWithParserAcceptsCustomParser|TestAddDocumentRoundTripsThroughParser|TestAddDocumentReturnsParserErrorVerbatim|TestAddDocumentDefaultParserErrorStringsPreserved|TestAddDocumentRejectsParserSkippingBeginDocument|TestAddDocumentRejectsBeginDocumentRGIDMismatch|TestNumericIndexPreservesInt64Exactness" -count=1 -v .`
- `go test ./... -count=1`
- `go test -tags regenerate_goldens -run TestRegenerateParityGoldens -count=1 .`
- Focused benchmark evidence recorded in [13-02-BENCH.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/13-parser-seam-extraction/13-02-BENCH.md)

---
*Phase: 13-parser-seam-extraction*
*Completed: 2026-04-21*
