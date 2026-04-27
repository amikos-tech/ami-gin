---
phase: 18-structured-ingesterror-cli-integration
plan: 01
subsystem: api
tags: [api, builder, errors, ingest, tdd]

requires:
  - phase: 16-adddocument-atomicity-lucene-contract
    provides: AddDocument validator and non-tragic per-document failure boundary
  - phase: 17-failure-mode-taxonomy-unification
    provides: hard/soft ingest failure modes for parser, transformer, and numeric layers
provides:
  - Exported IngestLayer and IngestError public API
  - Parser, transformer, numeric, and schema hard ingest failures wrapped as structured errors
  - Regression coverage for parser-contract and soft-mode non-IngestError boundaries
affects: [builder, parser, failure-modes-example, phase-18-cli-reporting]

tech-stack:
  added: []
  patterns:
    - Public error type implementing Error, Unwrap, and Cause for stdlib and github.com/pkg/errors compatibility
    - TDD red/green commits for structured ingest failure behavior

key-files:
  created:
    - ingest_error.go
    - .planning/phases/18-structured-ingesterror-cli-integration/18-01-SUMMARY.md
  modified:
    - builder.go
    - failure_modes_test.go
    - parser_test.go
    - gin_test.go
    - examples/failure-modes/main_test.go

key-decisions:
  - "IngestError.Value remains verbatim and unbounded in the library per D-08; callers own redaction and output-size policy."
  - "Parser contract errors remain non-IngestError implementation errors; ordinary parser document failures are wrapped at AddDocument after stage-callback unwrapping."
  - "Unsigned numeric overflow remains numeric-mode aware so Phase 17 numeric soft-skip behavior is preserved."

patterns-established:
  - "Structured ingest errors: use newIngestError or newIngestErrorString at hard user-document failure sites."
  - "Numeric value formatting: preserve raw JSON numeric literals when available, otherwise use integer decimal or float g-format."

requirements-completed: [IERR-01, IERR-02]

duration: 27 min
completed: 2026-04-24
---

# Phase 18 Plan 01: Public IngestError API Summary

**Structured hard ingest failures now expose path, layer, verbatim value, and cause via an exported `*IngestError` API.**

## Performance

- **Duration:** 27 min
- **Started:** 2026-04-24T12:06:02Z
- **Completed:** 2026-04-24T12:33:10Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments

- Added `IngestLayer` constants and `IngestError` with `Error()`, `Unwrap()`, and `Cause()` compatibility.
- Wrapped parser, transformer, numeric, schema, and validator-replayed mixed numeric failures as structured hard document errors.
- Preserved parser contract, tragic/internal, and Phase 17 soft-mode boundaries outside returned `*IngestError`.

## Task Commits

1. **Task 1 RED: IngestError wrapping contract test** - `036bef8` (test)
2. **Task 1 GREEN: Structured ingest error API** - `af9125e` (feat)
3. **Task 2 RED: Parser/transformer/schema hard layer tests** - `f6bb8f7` (test)
4. **Task 2 GREEN: Parser/transformer/schema wrapping** - `0ce64a9` (feat)
5. **Task 3 RED: Numeric hard and soft-boundary tests** - `0fcc0f0` (test)
6. **Task 3 GREEN: Numeric wrapping and validator replay values** - `26c76e1` (feat)

**Plan metadata:** this docs commit

## Files Created/Modified

- `ingest_error.go` - Public structured ingest error API and helper constructors/value formatting.
- `builder.go` - Hard user-document failure wrapping at parser, transformer, numeric, schema, and validator replay sites.
- `failure_modes_test.go` - Contract tests and per-layer hard/soft boundary coverage.
- `parser_test.go` - Parser hard-error expectation updated for structured parser errors.
- `gin_test.go` - Numeric promotion expectation updated for structured mixed-promotion errors.
- `examples/failure-modes/main_test.go` - Example output updated for structured transformer hard error text.

## Decisions Made

- Kept `Value` unredacted and untruncated in the library, matching the phase decision that callers own logging policy.
- Preserved stage-callback provenance before parser wrapping so numeric, transformer, and schema failures are not mislabeled as parser failures.
- Kept oversized unsigned numeric values routed through numeric handling rather than schema pre-checks so numeric soft mode continues to skip documents correctly.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated existing expectations for structured hard-error messages**
- **Found during:** Task 3 (numeric wrapping full-suite verification)
- **Issue:** Existing tests and the failure-modes example expected old opaque error strings such as `parse numeric at $.score` and transformer messages with embedded path text.
- **Fix:** Updated affected assertions to validate the new structured message shape or `errors.As` extraction while preserving behavior.
- **Files modified:** `failure_modes_test.go`, `parser_test.go`, `gin_test.go`, `examples/failure-modes/main_test.go`
- **Verification:** `go test ./...` passed.
- **Committed in:** `26c76e1`

**2. [Rule 1 - Bug] Preserved numeric soft mode for oversized unsigned values**
- **Found during:** Task 3 (numeric wrapping implementation)
- **Issue:** A literal Task 2 uint/uint64 schema pre-check would bypass existing numeric soft-skip behavior for transformer-produced oversized unsigned values.
- **Fix:** Left unsigned overflow routing in numeric staging so hard mode returns numeric `*IngestError` and soft mode still skips the document.
- **Files modified:** `builder.go`
- **Verification:** `go test ./...` passed, including existing oversized unsigned numeric soft-skip coverage.
- **Committed in:** `26c76e1`

---

**Total deviations:** 2 auto-fixed (2 Rule 1)
**Impact on plan:** Both fixes preserve the intended public structured-error contract and Phase 17 soft-mode boundary; no scope expansion.

## Issues Encountered

Full-suite verification initially failed because older tests asserted exact pre-Phase-18 string forms. Those assertions were updated as part of Task 3 after the new structured API behavior was in place.

## User Setup Required

None - no external service configuration required.

## Verification

- `go test ./... -run 'TestIngestErrorWrappingContract$' -count=1` - passed
- `go test ./... -run 'Test(HardIngestFailuresReturnIngestError|ParserContractErrorsRemainNonIngestError)$' -count=1` - passed
- `go test ./... -run 'Test(HardIngestFailuresReturnIngestError|SoftFailureModesDoNotReturnIngestError)$' -count=1` - passed
- `go test ./... -run 'Test(IngestErrorWrappingContract|HardIngestFailuresReturnIngestError|ParserContractErrorsRemainNonIngestError|SoftFailureModesDoNotReturnIngestError)$' -count=1` - passed
- `go test ./...` - passed

## Known Stubs

None.

## Next Phase Readiness

Ready for Plan 18-02 to add the broader behavior matrix and enforcement around hard ingest sites.

## Self-Check: PASSED

- Verified created/modified files exist on disk.
- Verified task commits exist in git history: `036bef8`, `af9125e`, `f6bb8f7`, `0ce64a9`, `0fcc0f0`, `26c76e1`.

---
*Phase: 18-structured-ingesterror-cli-integration*
*Completed: 2026-04-24*
