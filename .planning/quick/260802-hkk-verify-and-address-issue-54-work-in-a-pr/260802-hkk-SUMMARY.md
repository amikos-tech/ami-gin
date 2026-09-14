---
phase: quick-260802-hkk
plan: 01
subsystem: ingest
tags: [performance, ingest, jsonpath, simd, tests]
requires:
  - phase: 21
    provides: optional SIMD parser seam and lifecycle test support
provides:
  - Canonical wildcard-only array staging across materialized, stdlib, and SIMD walkers
  - Opt-in per-document staged-path budget with atomic schema-layer failures
affects: [array queries, parser parity, SIMD ingest]
tech-stack:
  added: []
  patterns:
    - A single builder helper owns all document-path allocation and budget enforcement.
    - Array descendants use public canonical wildcard paths rather than private numeric variants.
key-files:
  created: []
  modified:
    - gin.go
    - builder.go
    - parser_sink.go
    - parser_stdlib.go
    - parser_simd.go
    - gin_test.go
    - parser_parity_simd_test.go
key-decisions:
  - "Keep MaxStagedPaths runtime-only and zero-unlimited; do not serialize it."
  - "Retain wildcard query behavior while removing unsupported numeric array-path staging."
patterns-established:
  - "All parser presence callbacks return errors so path-allocation safeguards cannot be bypassed."
requirements-completed: [ISSUE-54]
duration: 5min
completed: 2026-08-02
---

# Quick Task 260802-hkk: Canonical array staging Summary

**Nested arrays now stage one wildcard path per depth in every parser, with an optional hard per-document path budget and preserved public wildcard queries.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-08-02T10:32:28Z
- **Completed:** 2026-08-02T10:37:19Z
- **Tasks:** 3
- **Files modified:** 13

## Accomplishments

- Added `MaxStagedPaths` and `WithMaxStagedPaths`, enforcing limits before staged-state mutation and returning extractable schema-layer `IngestError`s.
- Canonicalized materialized, stdlib, SIMD, and parity array traversal to stage only `[*]` descendants.
- Added deterministic deep-array and public wildcard-query regressions, refreshed representation goldens, and covered the SIMD equivalent.

## Task Commits

1. **Task 1: Add a per-document staged-path budget** - `71eb155` (feat)
2. **Task 2: Canonicalize every array walker to wildcard-only descent** - `0933669` (test), `0a9d371` (fix)
3. **Task 3: Lock down structural scaling and public wildcard behavior** - `b44ac8e` (test)
4. **Rule 1 follow-up: Align SIMD numeric-path expectations** - `852db60` (test)
5. **Rule 1 follow-up: Remove dead staged-path helper** - `404ad73` (fix)

## Files Created/Modified

- `gin.go` - Builder-only staged-path configuration and validation.
- `builder.go` - Shared budgeted staging boundary and wildcard-only materialized array descent.
- `parser_sink.go`, `parser_stdlib.go`, `parser_simd.go` - Error-propagating presence callbacks and canonical array traversal.
- `gin_test.go`, `parser_parity_simd_test.go`, `parser_simd_integration_test.go` - Budget, structural, wildcard-query, and canonical SIMD-path coverage.
- `testdata/parity-golden/*.bin` - Updated byte-parity fixtures for the canonical representation.

## Verification

- `go test -count=1 -run '^(TestMaxStagedPaths|TestAddDocumentRejectsUnsupportedNumberWithoutPartialMutation|TestAddDocumentAtomicity)$' .` — passed
- `go test -count=1 -run '^(TestStdlibParserStagesArrayIndexAndWildcardOnGenericSink|TestStdlibParserBeginsDocumentBeforeStaging|TestParserParity_StdlibMatchesMaterializingParser|TestWildcardSubtreeTransformerNormalizesNestedNumbers)$' .` — passed
- Focused structural default and SIMD tests — passed
- `go test -count=1 ./...` — passed
- `go build ./...` — passed
- `make lint` — passed after the post-review dead-helper fix (`404ad73`)
- `AMI_GIN_SIMD_REQUIRED=1 go test -count=1 -tags simdjson ./...` — passed

## Decisions Made

- The staged-path budget applies to new canonical paths only; repeats do not consume budget.
- Numeric array paths remain available in diagnostics where useful, but are no longer staged or queryable index paths.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Test compatibility] Refreshed stale representation expectations.**
- **Found during:** Task 3 verification
- **Issue:** Byte-parity goldens and two SIMD numeric-routing assertions retained the removed numeric array-path representation.
- **Fix:** Regenerated the affected parity goldens and updated the SIMD test expectations to canonical wildcard paths.
- **Files modified:** `testdata/parity-golden/*.bin`, `parser_simd_integration_test.go`
- **Verification:** Full default and required SIMD test suites passed.
- **Committed in:** `b44ac8e`, `852db60`

**2. [Rule 1 - Code quality] Removed the obsolete unbudgeted document-path helper.**
- **Found during:** Post-plan code review
- **Issue:** `documentBuildState.getOrCreatePath` remained only as a test helper after production staging centralized on `GINBuilder.getOrCreateStagedPath`, causing `unparam` lint failure.
- **Fix:** Removed the helper and made the two validator-preflight tests use the budget-enforced builder boundary.
- **Files modified:** `builder.go`, `gin_test.go`
- **Verification:** `make lint` and focused staged-path/validator/wildcard tests passed.
- **Committed in:** `404ad73`

**Total deviations:** 2 auto-fixed (Rule 1)

## Known Stubs

None.

## Next Steps

The `fix/issue-54` branch is ready for PR review and eventual squash merge.

## Self-Check: PASSED

- Summary and all listed implementation files exist.
- All six task and follow-up commits are present in the repository history.
