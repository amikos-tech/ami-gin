---
phase: 13-parser-seam-extraction
plan: 03
status: needs-review
subsystem: test
tags: [parser, parity, gopter, benchmark, merge-gate]
requires:
  - phase: 13
    provides: parser seam wire-up and committed authored goldens
provides:
  - always-on parser parity harness
  - benchmark delta artifact with an explicit review hold
affects: [parser, tests, benchmarks, planning, phase-14]
tech-stack:
  added: []
  patterns: [always-on golden parity harness, deterministic property canary, review-hold benchmark artifact]
key-files:
  created:
    - parser_parity_test.go
    - .planning/phases/13-parser-seam-extraction/13-03-BENCH.md
    - .planning/phases/13-parser-seam-extraction/13-parser-seam-extraction-03-SUMMARY.md
  modified:
    - parser_stdlib.go
    - parser_test.go
    - .planning/ROADMAP.md
    - .planning/STATE.md
key-decisions:
  - "Kept parser parity assertions always-on with no skip-on-missing gate; the authored goldens remain the byte-level source of truth."
  - "Introduced a package-local stdlibParserName constant so parser-name tests stay lint-clean without widening the public API."
  - "Recorded the benchmark evidence as Needs review instead of marking the phase complete because the transformer-heavy explicit-number probes did not clear the +2% wall-clock gate consistently."
patterns-established:
  - "Merge-gate pattern: always-on golden parity + gopter determinism canary + Evaluate matrix."
  - "Benchmark review-hold pattern: keep the plan implementation landed, but hold roadmap/state completion when the performance gate is noisy or not cleanly green."
requirements-completed: []
duration: pending-benchmark-review
completed: pending-benchmark-review
---

# Phase 13 Plan 03: Parity Harness Summary

**The parity harness is landed and the repo test/lint gate is green, but Phase 13 remains open pending human review of the benchmark artifact**

## Performance

- **Started:** 2026-04-21T11:18:30Z
- **Last updated:** 2026-04-21T11:33:25Z
- **Tasks:** 3
- **Status:** Needs review

## Accomplishments

- Added `parser_parity_test.go` as an always-on merge gate with the 7 authored golden fixtures, three gopter determinism properties, and the 24-case Evaluate matrix.
- Kept the parser-name assertions lint-clean by introducing the package-local `stdlibParserName` constant and reusing it in `stdlibParser.Name()` and the parser tests.
- Verified the full repo gate with `make lint`, focused parity runs, `TestNumericIndexPreservesInt64Exactness`, and `make test` before capturing the benchmark artifact in [13-03-BENCH.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/13-parser-seam-extraction/13-03-BENCH.md).

## Task Commits

1. **Tasks 1-2: parity harness, determinism canary, Evaluate matrix, and parser-name lint cleanup** - `88c3371` (test/refactor)

## Files Created/Modified

- `parser_parity_test.go` - Adds the always-on authored-golden assertions, determinism canary, and 12-operator Evaluate matrix.
- `parser_stdlib.go`, `parser_test.go` - Share the package-local `stdlibParserName` constant so parser-name checks stay aligned and lint-clean.
- [13-03-BENCH.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/13-parser-seam-extraction/13-03-BENCH.md) - Records the benchmark evidence and the explicit review hold.
- `.planning/ROADMAP.md`, `.planning/STATE.md` - Keep phase tracking honest by leaving Phase 13 open while the benchmark artifact is under review.

## Decisions Made

- Kept the golden parity test non-skippable: missing goldens are a hard failure, not a bootstrap skip path.
- Treated the benchmark evidence conservatively. The code/test diff is landed, but the milestone tracker is not being advanced past the performance gate without a clean result.
- Preserved the narrower D-02 API interpretation from the earlier plans: `Parser` and `WithParser` are exported, while `parserSink` and `stdlibParser` remain package-private.

## Deviations from Plan

### Benchmark Review Hold

- **Original plan:** Finalize Phase 13 once the full suite, lint, and the exact-anchored benchmark gate all cleared.
- **Actual outcome:** The code/test work for plan 13-03 is complete, but the isolated explicit-number transformer-heavy benchmark probes remained slightly above the `+2%` wall-clock threshold on this machine.
- **Reason held open:** The benchmark artifact is not cleanly green, so Phase 13 is left executing rather than silently marked complete.

### ROADMAP deviation (D-02), narrower than originally drafted

Success-criterion #3 lists `Parser`, `ParserSink`, `WithParser`, and `stdlibParser` as public surface. This phase ships `Parser` and `WithParser` exported, but `ParserSink` and `stdlibParser` remain package-private. Parser's documentation continues to treat external third-party parser implementations as deferred until `ParserSink` is exported. This is non-breaking to widen later, and is flagged for human confirmation during verification.

## Issues Encountered

- The benchmark gate is close to the machine's noise floor for the transformer-heavy explicit-number workload. Isolated reruns reduced the drift for `BenchmarkAddDocument` and the int-only probes, but the transformer-heavy probes still fluctuated around and above the threshold.

## User Setup Required

None for the landed code/test work.

Phase 13 still needs one of:
- accept the benchmark noise and close the phase with the current evidence
- adjust the benchmark methodology before closing the phase
- investigate the transformer-heavy explicit-number benchmark path further

## Next Phase Readiness

- Phase 14 should not start under the normal workflow until Phase 13's benchmark hold is resolved.
- The parser seam itself is now guarded by an always-on parity harness, so any further benchmark investigation can focus strictly on performance evidence rather than behavioral correctness.

## Self-Check: NEEDS REVIEW

- `make lint`
- `go test -run "TestParserParity_AuthoredFixtures|TestParserSeam_DeterministicAcrossRuns|TestParserParity_EvaluateMatrix" -count=1 -v .`
- `go test -run TestNumericIndexPreservesInt64Exactness -count=1 -v .`
- `make test`
- Focused benchmark evidence recorded in [13-03-BENCH.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/13-parser-seam-extraction/13-03-BENCH.md)

---
*Phase: 13-parser-seam-extraction*
*Status: Needs review*
