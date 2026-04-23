---
phase: 16-adddocument-atomicity-lucene-contract
plan: 03
subsystem: testing
tags: [atomicity, property-testing, gopter, failure-catalog]

requires:
  - phase: 16-adddocument-atomicity-lucene-contract
    provides: Tragic recovery, infallible merge signatures, and marker enforcement from plans 16-01, 16-02, and 16-04
provides:
  - Byte-identical AddDocument atomicity property over 1000 attempted documents
  - Full public non-tragic failure catalog coverage for AddDocument
  - Encode determinism sanity test for clean corpora with non-contiguous DocIDMapping
affects: [phase17, phase18, adddocument, ingest-errors]

tech-stack:
  added: []
  patterns:
    - bounded gopter property budget for heavy ingest properties
    - full-vs-clean encoded-byte atomicity oracle
    - public failure catalog asserts tragicErr stays nil

key-files:
  created:
    - atomicity_test.go
    - .planning/phases/16-adddocument-atomicity-lucene-contract/16-03-SUMMARY.md
  modified: []

key-decisions:
  - "Used a serializable registered email-domain transformer under alias strict for atomicity tests so Encode can include representation metadata."
  - "Kept the atomicity oracle byte-based: full attempted ingest and clean-only ingest must produce identical encoded output."

patterns-established:
  - "Heavy AddDocument properties use propertyTestParametersWithBudgets(50, 10)."
  - "Failure-catalog tests assert both returned errors and unchanged builder bookkeeping before checking continued usability."

requirements-completed: [ATOMIC-01, ATOMIC-03]

duration: 12min
completed: 2026-04-23
---

# Phase 16 Plan 03: Public Failure Catalog and Atomicity Property Summary

**AddDocument failed-document isolation is now pinned by byte-identical full-vs-clean encoded output and a tragic-nil public failure catalog**

## Performance

- **Duration:** 12 min
- **Started:** 2026-04-23T10:03:44Z
- **Completed:** 2026-04-23T10:15:34Z
- **Tasks:** 3
- **Files modified:** 1

## Accomplishments

- Added `TestAddDocumentAtomicityEncodeDeterminism`, proving repeated clean builds encode identically and preserve non-contiguous `DocIDMapping`.
- Added `TestAddDocumentPublicFailuresDoNotSetTragicErr`, covering parser, stage, transformer, numeric, pre-parser gate, parser-contract, uint overflow, and unsupported-number regression failures.
- Added `TestAddDocumentAtomicity`, a bounded gopter property with 1000 attempted docs, deterministic 10% failing slots, parser/transformer/numeric failure intents, and `bytes.Equal` over encoded full-vs-clean builds.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add encode determinism sanity and shared atomicity helpers** - `6e0a936` (test)
2. **Task 2: Add tragic-nil public failure catalog coverage** - `d621567` (test)
3. **Task 3: Add the gopter full-vs-clean AddDocument atomicity property** - `ffe45e5` (test)

**Plan metadata:** captured in the final `docs(16-03)` metadata commit

## Files Created/Modified

- `atomicity_test.go` - New encode determinism sanity, failure catalog, typed failure-intent generators, and full-vs-clean atomicity property.
- `.planning/phases/16-adddocument-atomicity-lucene-contract/16-03-SUMMARY.md` - Captures execution results and verification evidence.

## Decisions Made

- Used `WithEmailDomainTransformer("$.email", "strict")` rather than a runtime-only custom transformer so the strict companion representation remains serializable during `Encode`.
- Kept the property's oracle at the serialized byte level, including `Header.NumRowGroups`, path/index payloads, and `DocIDMapping`.
- Preserved original successful `DocID` values in `cleanOnly`; clean-only builds never compact successful document IDs.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Switched strict test transformer to a serializable registered transformer**
- **Found during:** Task 1 (Add encode determinism sanity and shared atomicity helpers)
- **Issue:** The initial helper used `WithCustomTransformer`, and `Encode` rejected the runtime-only representation with `representation strict on $.email is not serializable`.
- **Fix:** Replaced it with the registered email-domain transformer under alias `strict`, which still rejects non-string and missing-`@` inputs while preserving serializable metadata.
- **Files modified:** `atomicity_test.go`
- **Verification:** `go test -run TestAddDocumentAtomicityEncodeDeterminism ./... -count=1`
- **Committed in:** `6e0a936`

**2. [Rule 3 - Blocking] Fixed lint blockers in the atomicity property test**
- **Found during:** Task 3 (Add the gopter full-vs-clean AddDocument atomicity property)
- **Issue:** `make lint` flagged De Morgan simplifications and an unused helper parameter introduced by the new test helpers.
- **Fix:** Factored the failure-count expression into a boolean while keeping the acceptance expression present, and removed the unnecessary helper parameter.
- **Files modified:** `atomicity_test.go`
- **Verification:** `make lint`; `go test -run 'Test(AddDocumentAtomicity|AddDocumentPublicFailuresDoNotSetTragicErr)' ./... -count=1`
- **Committed in:** `ffe45e5`

---

**Total deviations:** 2 auto-fixed (2 blocking)
**Impact on plan:** Both fixes preserved the planned behavioral contract and kept the implementation scoped to the owned test file.

## Issues Encountered

- `gsd-sdk query` is unavailable in this checkout, so SUMMARY, STATE, ROADMAP, and REQUIREMENTS updates were applied directly.

## User Setup Required

None - no external service configuration required.

## Verification

- `go test -run TestAddDocumentAtomicityEncodeDeterminism ./... -count=1` - PASS
- `go test -run 'Test(AddDocumentPublicFailuresDoNotSetTragicErr|AddDocumentRejectsUnsupportedNumberWithoutPartialMutation)$' ./... -count=1` - PASS
- `go test -run 'TestAddDocumentAtomicity(EncodeDeterminism)?$' ./... -count=1` - PASS
- `go test -run 'Test(AddDocumentAtomicity|AddDocumentPublicFailuresDoNotSetTragicErr)' ./... -count=1` - PASS
- `make test` - PASS, 871 tests, 1 skipped
- `make lint` - PASS, 0 issues
- `go build ./...` - PASS

## Known Stubs

None.

## Threat Flags

None - the plan added tests only and introduced no new network, auth, file-access, or trust-boundary runtime surface.

## Next Phase Readiness

Phase 16 is complete. Phase 17 can build the unified failure-mode taxonomy on top of a tested AddDocument contract: ordinary public failures are non-tragic, failed documents are isolated, and clean-vs-full finalized bytes match.

## Self-Check: PASSED

- Created files exist: `atomicity_test.go`, `16-03-SUMMARY.md`
- Task commits found: `6e0a936`, `d621567`, `ffe45e5`
- Stub scan found no TODO/FIXME/placeholder markers in plan-created files.
- Required verification commands passed.

---
*Phase: 16-adddocument-atomicity-lucene-contract*
*Completed: 2026-04-23*
