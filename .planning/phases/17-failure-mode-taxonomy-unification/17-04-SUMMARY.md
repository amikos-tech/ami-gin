---
phase: 17-failure-mode-taxonomy-unification
plan: 04
subsystem: docs-examples
tags: [docs, examples, integration, failure-modes, go]

requires:
  - phase: 17-failure-mode-taxonomy-unification
    provides: Unified IngestFailureMode API, v9 transformer wire-token compatibility, and soft-skip routing from Plans 17-01 through 17-03
provides:
  - Root CHANGELOG breaking rename note for TransformerFailureMode to IngestFailureMode
  - Deterministic hard-vs-soft ingest failure-mode example
  - Exact output regression test for the failure-modes example
  - Final Phase 17 integration verification across focused tests, full tests, lint, and build
affects: [18-structured-ingest-error-cli-integration, docs, examples, release-notes]

tech-stack:
  added: []
  patterns:
    - Deterministic example output asserted by go run smoke test
    - Changelog entry limited to public API migration symbols
    - Lint-only integration fixes committed separately from planned docs/example work

key-files:
  created:
    - CHANGELOG.md
    - examples/failure-modes/main.go
    - examples/failure-modes/main_test.go
    - .planning/phases/17-failure-mode-taxonomy-unification/17-04-SUMMARY.md
  modified:
    - builder.go
    - failure_modes_test.go
    - serialize_security_test.go

key-decisions:
  - "The changelog documents only the public breaking rename and focused transformer option before/after snippet."
  - "The example uses fixed public sample documents and exact three-line output to demonstrate hard rejection and configured soft skips."
  - "Integrated lint blockers from prior Phase 17 work were fixed minimally because make lint is a required 17-04 exit gate."

patterns-established:
  - "Public examples that document ingest error behavior should have exact stdout regression tests."
  - "Final integration lint blockers should be isolated in a separate fix commit and documented as deviations."

requirements-completed: [FAIL-01, FAIL-02]

duration: 9min
completed: 2026-04-23
---

# Phase 17 Plan 04: Changelog and Failure-Modes Example Summary

**Breaking failure-mode rename documented with a deterministic hard-vs-soft ingest example and full integration gates passing**

## Performance

- **Duration:** 9 min
- **Started:** 2026-04-23T16:42:43Z
- **Completed:** 2026-04-23T16:51:40Z
- **Tasks:** 2 planned tasks plus 1 integration lint deviation
- **Files modified:** 7

## Accomplishments

- Added a root `CHANGELOG.md` with the required Unreleased breaking rename note for `TransformerFailureMode` to `IngestFailureMode`.
- Added `examples/failure-modes/main.go`, demonstrating hard rejection on transformer failure and soft parser/numeric/transformer skipping with dense row groups.
- Added `examples/failure-modes/main_test.go`, which executes `go run .` and asserts exact stdout plus empty stderr.
- Ran the final Phase 17 verification gates after all integration fixes.

## Task Commits

Each planned task was committed atomically:

1. **Task 1: Add breaking rename migration note** - `ad9e703` (docs)
2. **Task 2: Add deterministic hard-vs-soft example** - `0ddcf7f` (feat)
3. **Integration lint deviation** - `f26ea43` (fix)

## Files Created/Modified

- `CHANGELOG.md` - Adds the Unreleased breaking-change migration note and before/after snippet.
- `examples/failure-modes/main.go` - Runnable failure-mode example with fixed attempted documents and deterministic output.
- `examples/failure-modes/main_test.go` - Exact output regression test for the example binary.
- `builder.go` - Lint-only sentinel check cleanup using `errors.Is`.
- `failure_modes_test.go` - Lint-only removal of an unused test parser.
- `serialize_security_test.go` - Lint-only reuse of the lower alias constant in serialization tests.
- `.planning/phases/17-failure-mode-taxonomy-unification/17-04-SUMMARY.md` - This execution summary.

## Decisions Made

- Kept `CHANGELOG.md` limited to the one required public API migration note and snippet.
- Used `go run .` from the example test so CI checks the same execution path users run.
- Fixed unrelated integrated lint blockers only after they blocked the required plan-level `make lint` gate.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Resolved integrated lint blockers outside the planned write list**
- **Found during:** Plan-level `make lint`
- **Issue:** `make lint` failed on existing Phase 17 files: an error comparison in `builder.go`, repeated `"lower"` literals in `serialize_security_test.go`, and an unused parser in `failure_modes_test.go`.
- **Fix:** Replaced the sentinel comparison with `errors.Is`, introduced a reusable `lowerAlias` constant in serialization tests, and removed the unused parser type.
- **Files modified:** `builder.go`, `serialize_security_test.go`, `failure_modes_test.go`
- **Verification:** `go test ./... -run 'Test(NumericFailureMode|DecodeLegacyTransformerFailureModeTokens|ReadConfigRejectsUnknownTransformerFailureMode|TransformerFailureModeWireTokensStayV9|RepresentationFailureModeRoundTrip|SoftFailureModesMatchCleanCorpus)' -count=1`, `make test`, `make lint`, `go build ./...`
- **Committed in:** `f26ea43`

---

**Total deviations:** 1 auto-fixed (Rule 3)
**Impact on plan:** The fixes were lint-only or test cleanup needed to satisfy the required integration gate. No public API, example output, or serialized behavior changed.

## Verification

Passed:

```bash
go test ./examples/failure-modes -run 'TestFailureModesExampleOutput$' -count=1
go run ./examples/failure-modes/main.go
go test ./... -run 'Test(IngestFailureMode|ParserFailureMode|TransformerFailureMode|NumericFailureMode|RepresentationFailureMode|DecodeLegacyTransformerFailureModeTokens|TransformerFailureModeWireTokensStayV9|SoftFailureModesMatchCleanCorpus|AddDocumentPublicFailuresDoNotSetTragicErr)' -count=1
make test
make lint
go build ./...
```

Example output:

```text
hard: stopped after 1 indexed document: companion transformer "domain" on $.email failed to produce a value
soft: indexed 2 documents
soft: email-domain example.com row groups [0 1]
```

## Known Stubs

None - no TODO, FIXME, placeholder, or intentional empty implementation stubs were introduced. The stub scan found only an existing serialization corruption-test comment describing an empty suffix payload.

## Threat Flags

None - no network endpoint, auth path, file access pattern, or schema trust boundary was introduced. The example uses fixed public sample documents and prints counts/results only.

## Authentication Gates

None.

## Issues Encountered

- The local `gsd-sdk` binary does not expose the documented `query` subcommands, so execution used direct git and file operations.
- `make lint` surfaced integrated blockers from prior Phase 17 work; they were fixed in the separate deviation commit listed above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Phase 17 is integrated for the failure-mode taxonomy work: the API rename is documented, the hard/soft example is executable and regression-tested, and the full test/lint/build gates are green. Phase 18 can build structured `IngestError` reporting on top of the behavior now documented here.

## Self-Check: PASSED

- Found `CHANGELOG.md`.
- Found `examples/failure-modes/main.go`.
- Found `examples/failure-modes/main_test.go`.
- Found `.planning/phases/17-failure-mode-taxonomy-unification/17-04-SUMMARY.md`.
- Verified commits exist: `ad9e703`, `0ddcf7f`, `f26ea43`.
- Re-ran all plan-level verification commands successfully.

---
*Phase: 17-failure-mode-taxonomy-unification*
*Completed: 2026-04-23*
