---
phase: 21-simd-parser-adapter
plan: 01
subsystem: parser
tags: [go, parser, simdjson, numeric-fidelity, tdd]

requires:
  - phase: 13-parser-seam-extraction
    provides: Parser and package-private parserSink staging seam
  - phase: 16-add-document-atomicity
    provides: validate-before-commit AddDocument flow and staged error provenance
  - phase: 19-simd-dependency-decision-integration-strategy
    provides: locked typed-sink and exact-number routing decisions
provides:
  - Five typed scalar methods on parserSink and GINBuilder
  - Non-coercing StageFloat64 routing with numeric hard/soft provenance
  - Committed-document tests for typed scalar semantics and atomic rejection
  - Buildable simd-numeric-parity fixture with an isolated stdlib golden
affects: [21-03-simd-adapter, 22-simd-validation-benchmarks-ci]

tech-stack:
  added: []
  patterns:
    - Typed parser leaves funnel into existing builder staging primitives
    - Parser sink tests commit through AddDocument before comparing encoded indexes
    - Stage callback errors retain their ingest layer across Parser and AddDocument

key-files:
  created:
    - parser_simd_test.go
    - testdata/parity-golden/simd-numeric-parity.bin
  modified:
    - parser.go
    - parser_sink.go
    - parser_test.go
    - parser_parity_fixtures_test.go

key-decisions:
  - "StageString and StageBool reuse stageScalarToken; StageInt64 and StageUint64 reuse stageNativeNumeric."
  - "StageFloat64 bypasses stageNativeNumeric so lexeme-classified whole floats remain floats."
  - "The SIMD numeric golden keeps each numeric class on a separate canonical path."

patterns-established:
  - "Committed typed-sink comparison: Parser -> AddDocument -> Finalize -> Encode."
  - "Numeric callback provenance: every hard StageFloat64 exit passes through tagStageError."

requirements-completed: [SIMD-06, SIMD-07]

duration: 11min
completed: 2026-07-22
---

# Phase 21 Plan 01: Typed Parser Sink and Numeric Parity Summary

**Five exact typed scalar routes with non-coercing float staging, committed-state regression coverage, and a deterministic numeric parity golden**

## Performance

- **Duration:** 11 min
- **Started:** 2026-07-22T18:14:12Z
- **Completed:** 2026-07-22T18:24:50Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- Extended `parserSink`, `GINBuilder`, and the recording test sink with string, bool, int64, uint64, and float64 methods.
- Proved exact int64, whole-float non-coercion, uint64 overflow, and NaN/Inf behavior through real `AddDocument` commits and atomic rejection checks.
- Added a one-document numeric fixture whose five numeric classes use separate paths, plus a 604-byte stdlib golden without changing existing goldens.

## TDD Cycle

- **RED:** `62da99b` added committed-state typed-sink tests; the focused suite failed because all five typed methods were absent.
- **GREEN:** `d1db7f6` added the minimal typed wrappers, non-coercing float path, parser contract updates, and recording-sink methods; all focused tests passed.
- **REFACTOR:** No separate refactor was needed; the implementation already delegates to the existing staging primitives with one bespoke float path.

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement and prove the expanded typed sink contract**
   - `62da99b` — failing behavior tests
   - `d1db7f6` — typed sink implementation
2. **Task 2: Add a buildable numeric parity fixture**
   - `34e754f` — fixture and isolated golden

## Files Created/Modified

- `parser.go` — documents raw-text and exact typed numeric routes plus live error-provenance behavior.
- `parser_sink.go` — defines and implements the five typed scalar methods.
- `parser_test.go` — keeps `recordingSink` aligned with the expanded contract.
- `parser_simd_test.go` — exercises committed typed scalar equivalence and numeric rejection semantics without a native SIMD runtime.
- `parser_parity_fixtures_test.go` — registers the buildable `simd-numeric-parity` fixture.
- `testdata/parity-golden/simd-numeric-parity.bin` — stores the deterministic stdlib reference bytes.

## Verification

- `go test -run '^TestTypedSink' -count=1 .` — passed.
- `go test -run '^TestParserParity_AuthoredFixtures/simd-numeric-parity$' -count=1 .` — passed.
- `go build ./... && go vet ./...` — passed.
- `make test` — passed 1,038 tests; one pre-existing test skipped because `testdata/test.parquet` is unavailable.
- Existing golden diff excluding `simd-numeric-parity.bin` — clean before and after regeneration.

## Decisions Made

- Kept all typed wrappers thin so builder internals remain the single source of staging truth.
- Gave `StageFloat64` a direct `stagedNumericValue{isInt:false}` route because the generic native path deliberately folds whole floats.
- Excluded oversized uint64 and greater-than-uint64 BIGINT cases from the byte golden; their hard/soft policy is covered by focused behavior tests or later tagged parser validation.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- Phase context/review artifacts were absent from the dispatched worktree; the orchestrator authorized read-only loading from the primary checkout before implementation.
- The read-only GSD initialization query normalized `.planning/config.json`; the file was restored to `HEAD` before any task commit and remained unmodified.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 21-03 can consume the five typed sink methods for the tagged SIMD adapter.
- Phase 22 can reuse `simd-numeric-parity` as the stdlib byte reference for tagged parser parity.
- No blockers remain for this plan.

## Self-Check: PASSED

- All six created or modified implementation artifacts exist.
- Task commits `62da99b`, `d1db7f6`, and `34e754f` exist on the assigned branch.
- `.planning/STATE.md`, `.planning/ROADMAP.md`, and `.planning/config.json` remain unchanged.

---
*Phase: 21-simd-parser-adapter*
*Completed: 2026-07-22*
