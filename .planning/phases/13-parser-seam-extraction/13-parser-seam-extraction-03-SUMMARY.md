---
phase: 13-parser-seam-extraction
plan: 03
status: completed
subsystem: test
tags: [parser, parity, gopter, benchmark, merge-gate]
requires:
  - phase: 13
    provides: parser seam wire-up and committed authored goldens
provides:
  - always-on parser parity harness
  - benchmark delta artifact preserved with an accepted-risk disposition
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
  - "Accepted the transformer-heavy benchmark noise as residual risk in 13-SECURITY.md instead of reopening implementation work that kept allocs flat and correctness/parity green."
patterns-established:
  - "Merge-gate pattern: always-on golden parity + gopter determinism canary + Evaluate matrix."
  - "Benchmark risk-acceptance pattern: preserve the failing artifact, document the residual risk, and reconcile roadmap/state after human review."
requirements-completed: [PARSER-01]
duration: completed-after-benchmark-risk-acceptance
completed: 2026-04-21
---

# Phase 13 Plan 03: Parity Harness Summary

**The parity harness is landed, the repo test/lint gate is green, and Phase 13 is closed with the residual benchmark noise accepted in the phase security record**

## Performance

- **Started:** 2026-04-21T11:18:30Z
- **Completed:** 2026-04-21
- **Tasks:** 3
- **Status:** Complete

## Accomplishments

- Added `parser_parity_test.go` as an always-on merge gate with the 7 authored golden fixtures, three gopter determinism properties, and the 24-case Evaluate matrix.
- Kept the parser-name assertions lint-clean by introducing the package-local `stdlibParserName` constant and reusing it in `stdlibParser.Name()` and the parser tests.
- Verified the full repo gate with `make lint`, focused parity runs, `TestNumericIndexPreservesInt64Exactness`, and `make test` before capturing the benchmark artifact in [13-03-BENCH.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/13-parser-seam-extraction/13-03-BENCH.md).
- Closed the phase after the benchmark drift was explicitly accepted as residual risk in [13-SECURITY.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/13-parser-seam-extraction/13-SECURITY.md).

## Task Commits

1. **Tasks 1-2: parity harness, determinism canary, Evaluate matrix, and parser-name lint cleanup** - `88c3371` (test/refactor)
2. **Phase security acceptance: benchmark noise accepted as residual risk** - `6e2abae` (docs)

## Files Created/Modified

- `parser_parity_test.go` - Adds the always-on authored-golden assertions, determinism canary, and 12-operator Evaluate matrix.
- `parser_stdlib.go`, `parser_test.go` - Share the package-local `stdlibParserName` constant so parser-name checks stay aligned and lint-clean.
- [13-03-BENCH.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/13-parser-seam-extraction/13-03-BENCH.md) - Records the benchmark evidence that was later accepted as residual risk.
- [13-SECURITY.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/13-parser-seam-extraction/13-SECURITY.md), `.planning/ROADMAP.md`, `.planning/STATE.md` - Record the accepted benchmark-risk disposition and close the phase in the planning trackers.

## Decisions Made

- Kept the golden parity test non-skippable: missing goldens are a hard failure, not a bootstrap skip path.
- Accepted the residual transformer-heavy benchmark drift after review because allocs stayed flat and the parity/correctness gates remained green.
- Preserved the narrower D-02 API interpretation from the earlier plans: `Parser` and `WithParser` are exported, while `parserSink` and `stdlibParser` remain package-private.

## Deviations from Plan

### Benchmark Risk Acceptance

- **Original plan:** Finalize Phase 13 once the full suite, lint, and the exact-anchored benchmark gate all cleared.
- **Observed outcome:** The code/test work for plan 13-03 completed, but the isolated explicit-number transformer-heavy benchmark probes remained slightly above the `+2%` wall-clock threshold on this machine.
- **Resolution:** The benchmark drift was accepted as residual risk and recorded in [13-SECURITY.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/13-parser-seam-extraction/13-SECURITY.md), so the phase is now closed without changing the benchmark artifact itself.

### ROADMAP deviation (D-02), narrower than originally drafted

Success-criterion #3 lists `Parser`, `ParserSink`, `WithParser`, and `stdlibParser` as public surface. This phase ships `Parser` and `WithParser` exported, but `ParserSink` and `stdlibParser` remain package-private. Parser's documentation continues to treat external third-party parser implementations as deferred until `ParserSink` is exported. This is non-breaking to widen later, and is flagged for human confirmation during verification.

## Issues Encountered

- The benchmark gate is close to the machine's noise floor for the transformer-heavy explicit-number workload. Isolated reruns reduced the drift for `BenchmarkAddDocument` and the int-only probes, but the transformer-heavy probes still fluctuated around and above the threshold.

## User Setup Required

None.

## Next Phase Readiness

- Phase 14 can start under the normal workflow; Phase 13 is complete and no longer blocks the milestone DAG.
- The parser seam is now guarded by an always-on parity harness, so any future benchmark follow-up can focus strictly on performance evidence rather than behavioral correctness.

## Self-Check: PASSED

- `make lint`
- `go test -run "TestParserParity_AuthoredFixtures|TestParserSeam_DeterministicAcrossRuns|TestParserParity_EvaluateMatrix" -count=1 -v .`
- `go test -run TestNumericIndexPreservesInt64Exactness -count=1 -v .`
- `make test`
- Focused benchmark evidence recorded in [13-03-BENCH.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/13-parser-seam-extraction/13-03-BENCH.md) and accepted as residual risk in [13-SECURITY.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/13-parser-seam-extraction/13-SECURITY.md)

---
*Phase: 13-parser-seam-extraction*
*Status: Complete*
