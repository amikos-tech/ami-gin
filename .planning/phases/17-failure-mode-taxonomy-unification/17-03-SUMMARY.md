---
phase: 17-failure-mode-taxonomy-unification
plan: 03
subsystem: builder
tags: [builder, parser, transformer, numeric, atomicity, failure-modes]

requires:
  - phase: 16-adddocument-atomicity-lucene-contract
    provides: AddDocument staging atomicity, tragic merge recovery, and encoded-byte oracle patterns
  - phase: 17-failure-mode-taxonomy-unification
    provides: Unified IngestFailureMode API from Plan 17-01 and v9 transformer wire-token compatibility from Plan 17-02
provides:
  - Parser soft mode skips ordinary parser failures without mutating durable builder state
  - Numeric soft mode skips malformed literals, non-finite native numerics, and validator-rejected mixed promotion
  - Transformer soft mode skips whole documents instead of indexing raw values without companions
  - Error-carried parser callback provenance for hard staging failures
  - Full-vs-clean encoded-byte oracle for parser, transformer, and numeric soft skips
affects: [17-04, 18-structured-ingest-error-cli-integration, adddocument, transformers]

tech-stack:
  added: []
  patterns:
    - Private errSkipDocument sentinel for configured whole-document skips
    - Private stageCallbackError wrapper for parser callback provenance
    - Encoded full-vs-clean oracle using serializable transformer metadata

key-files:
  created:
    - .planning/phases/17-failure-mode-taxonomy-unification/17-03-SUMMARY.md
  modified:
    - builder.go
    - parser_sink.go
    - failure_modes_test.go
    - transformers_test.go
    - phase09_review_test.go

key-decisions:
  - "Parser callback provenance is carried in private error values rather than persistent builder state."
  - "Soft skips return before document bookkeeping advances; recovered merge panics remain tragic even under soft numeric mode."
  - "Raw rejected transformer values are asserted through finalized index structure because numeric EQ on a string-only path remains a conservative all-row-groups fallback."

patterns-established:
  - "Soft ingest failures use errSkipDocument and are translated to nil only at AddDocument/mergeDocumentState boundaries."
  - "Parser soft mode classifies skip sentinels and tagged hard stage errors before ordinary parser errors."
  - "Whole-document transformer soft skips discard any staged fields before durable merge."

requirements-completed: [FAIL-02]

duration: 13min
completed: 2026-04-23
---

# Phase 17 Plan 03: Soft-Skip Routing Summary

**Parser, transformer, and numeric soft failure modes now skip whole failed documents while hard staging errors, parser contracts, and tragic merge recovery remain hard**

## Performance

- **Duration:** 13 min
- **Started:** 2026-04-23T16:25:34Z
- **Completed:** 2026-04-23T16:38:17Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- Added `errSkipDocument` routing and parser callback error tagging so parser soft mode only softens ordinary parser errors.
- Routed numeric soft failures for malformed literals, non-finite native values, and mixed-promotion validation to nil whole-document skips before bookkeeping.
- Changed transformer soft mode to skip the whole document, including documents with earlier staged fields.
- Added per-layer hard/soft tests, DocID retry/dense packing coverage, all-soft silent-drop coverage, and a full-vs-clean encoded-byte oracle.

## Task Commits

Each task was committed atomically:

1. **Task 1 RED: parser and numeric soft-mode tests** - `8ac43d0` (test)
2. **Task 1 GREEN: parser and numeric soft-skip routing** - `322dfe2` (feat)
3. **Task 2: transformer whole-document soft-skip coverage** - `5dd21bc` (test)
4. **Task 2 fix: legacy Phase 09 expectation update** - `08d0c8c` (fix)

## Files Created/Modified

- `builder.go` - Adds skip/tag helpers and routes parser, transformer, and numeric soft failures before durable merge/bookkeeping.
- `parser_sink.go` - Tags non-skip parser callback staging errors so parser soft mode cannot swallow hard stage failures.
- `failure_modes_test.go` - Adds parser, transformer, numeric, retry, all-soft, partial-staging, and encoded-byte oracle coverage.
- `transformers_test.go` - Rewrites the transformer soft-fail regression to expect whole-document skip and dense row-group packing.
- `phase09_review_test.go` - Updates a legacy companion-only soft expectation to the new whole-document soft contract.

## Decisions Made

- Used error-carried provenance (`stageCallbackError`) instead of a persistent builder field.
- Kept `runMergeWithRecover` outside all soft-mode routing.
- Used finalized `NumericIndexes` absence checks for rejected raw numeric transformer values because existing query semantics conservatively return all row groups for numeric EQ on a string-only path.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated obsolete Phase 09 soft transformer expectation**
- **Found during:** Plan-level `go test ./...`
- **Issue:** `TestPhase09FinalizeOmitsNeverMaterializedRepresentations` still expected soft transformer failures to index raw documents.
- **Fix:** Updated the regression to assert all soft-rejected documents are skipped and no raw timestamp path is finalized.
- **Files modified:** `phase09_review_test.go`
- **Verification:** `go test ./...`
- **Committed in:** `08d0c8c`

---

**Total deviations:** 1 auto-fixed (Rule 1)
**Impact on plan:** The fix aligns stale coverage with the planned Phase 17 behavior change. No production scope was added.

## TDD Gate Compliance

- RED: `8ac43d0` added parser/numeric tests and the focused command failed on parser and numeric soft skips as expected.
- GREEN: `322dfe2` implemented the shared skip/provenance routing and made Task 1 verification pass.
- Task 2 coverage was committed in `5dd21bc`; transformer behavior was already enabled by the shared `errSkipDocument` routing in `322dfe2`, so there is no separate Task 2 GREEN code commit.

## Verification

Passed:

```bash
go test ./... -run 'Test(ParserFailureMode|NumericFailureMode|SoftSkippedDocIDCanBeRetriedWithoutPositionConsumption|AddDocumentPublicFailuresDoNotSetTragicErr|AddDocumentRefusesAfterRecoveredMergePanic)' -count=1
go test ./... -run 'Test(TransformerFailureMode|BuilderSoftFailSkipsDocumentWhenConfigured|TransformerFailureModeSoftDiscardsPartiallyStagedDocument|SoftFailureModesMatchCleanCorpus|AllSoftFailureModesSilentlyDropFailures|AddDocumentAtomicityEncodeDeterminism)$' -count=1
go test ./... -run 'Test(ParserFailureMode|TransformerFailureMode|NumericFailureMode|BuilderSoftFailSkipsDocumentWhenConfigured|TransformerFailureModeSoftDiscardsPartiallyStagedDocument|SoftSkippedDocIDCanBeRetriedWithoutPositionConsumption|SoftFailureModesMatchCleanCorpus|AllSoftFailureModesSilentlyDropFailures|AddDocumentPublicFailuresDoNotSetTragicErr|AddDocumentRefusesAfterRecoveredMergePanic)$' -count=1
go test ./...
```

## Known Stubs

None - no TODO, FIXME, placeholder, or intentionally empty implementation stubs were introduced. Empty slices/maps in touched tests are expected test fixtures/assertions.

## Threat Flags

None - no new network endpoint, auth path, file access pattern, or schema trust boundary was introduced.

## Issues Encountered

- The local `gsd-sdk` binary does not expose the documented `query` subcommand, so commits and summary creation used direct git/apply-patch operations.
- Existing query behavior returns all row groups for unsupported numeric EQ on a string-only path. Tests therefore assert raw rejected numeric absence via finalized numeric index state rather than using that query shape.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Plan 17-04 can document the breaking failure-mode behavior and add the hard-vs-soft example. Phase 18 can build structured `IngestError` reporting on top of the hard/soft routing boundaries now pinned here.

## Self-Check: PASSED

- Found `builder.go`, `parser_sink.go`, `failure_modes_test.go`, `transformers_test.go`, and `phase09_review_test.go`.
- Created `.planning/phases/17-failure-mode-taxonomy-unification/17-03-SUMMARY.md`.
- Verified task commits exist: `8ac43d0`, `322dfe2`, `5dd21bc`, `08d0c8c`.
- Re-ran the plan-level verification command successfully.
- Ran `go test ./...` successfully.

---
*Phase: 17-failure-mode-taxonomy-unification*
*Completed: 2026-04-23*
