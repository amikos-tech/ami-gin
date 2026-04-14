---
phase: 06-query-path-hot-path
plan: 01
subsystem: library
tags: [jsonpath, canonicalization, lookup, serialization, transformers]
requires: []
provides:
  - Canonical query/config path normalization for supported JSONPath spellings
  - Immutable derived `pathLookup` rebuilt during finalize and decode
  - Regression coverage for fresh versus decoded canonical lookup behavior
affects: [query-evaluation, serialization, transformer-config]
tech-stack:
  added: []
  patterns:
    - Public query/config paths validate before canonicalization
    - Derived lookup state rebuilds from persisted PathDirectory and stays read-only
key-files:
  created: []
  modified:
    - builder.go
    - gin.go
    - jsonpath.go
    - query.go
    - serialize.go
    - gin_test.go
    - transformers_test.go
    - transformer_registry_test.go
key-decisions:
  - "Keep the supported JSONPath surface unchanged by validating public inputs before canonicalization."
  - "Rebuild `pathLookup` only in `Finalize()` and `Decode()`, and reject canonical collisions during rebuild."
  - "Preserve safe no-pruning behavior for unsupported or missing paths."
patterns-established:
  - "Canonical public path handling: `canonicalizeSupportedPath()` for query/config boundaries, `NormalizePath()` for internal builder paths."
  - "Immutable lookup rebuild: derive `pathLookup` from `PathDirectory` once, then treat it as read-only."
requirements-completed: [PATH-01, PATH-02]
duration: 7m
completed: 2026-04-14
---

# Phase 06 Plan 01: Query Path Hot Path Summary

**Canonical JSONPath lookup now resolves supported spelling variants through one stored path name and one immutable hot-path map in fresh and decoded indexes.**

## Performance

- **Duration:** 7m
- **Started:** 2026-04-14T13:44:10Z
- **Completed:** 2026-04-14T13:51:21Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments

- Added `canonicalizeSupportedPath()` and applied it at the public query/config boundary without widening supported JSONPath syntax.
- Canonicalized builder storage and decoded config state so FTS and transformer bindings use the same canonical path keys.
- Replaced linear query-time path scans with derived immutable lookup state and rejected duplicate canonical paths during decode.
- Added regression coverage for supported-spelling parity, unsupported-path fallback, decode parity, and canonical config round-trips.

## Task Commits

Each task was committed atomically:

1. **Task 1: Canonicalize supported query/config spellings and lock the query boundary matrix**
   - `5233228` `test(06-01): add failing canonical path coverage`
   - `ae1a34c` `feat(06-01): canonicalize supported path config inputs`
2. **Task 2: Build immutable derived lookup state and make duplicate-canonical-path handling explicit**
   - `481dba4` `test(06-01): add failing lookup rebuild coverage`
   - `c7ddc7f` `feat(06-01): rebuild immutable canonical path lookup`
3. **Task 3: Add fresh-vs-decoded regression coverage for canonical lookup and config round-trips**
   - `0e75655` `test(06-01): add canonical path decode regressions`

## Files Created/Modified

- `builder.go` - Normalizes builder traversal paths before transformer lookup, path creation, trigrams, and bloom keys.
- `gin.go` - Canonicalizes config entry points and adds immutable derived `pathLookup` rebuild support.
- `jsonpath.go` - Adds `canonicalizeSupportedPath()` for validated public-path normalization.
- `query.go` - Resolves paths through canonicalization plus `pathLookup` instead of scanning `PathDirectory`.
- `serialize.go` - Canonicalizes decoded config paths and rebuilds derived lookup state during decode.
- `gin_test.go` - Covers supported/unsupported path matrices, builder collapse, duplicate collisions, lookup fallback, and decode parity.
- `transformers_test.go` - Covers canonical transformer path binding and decoded canonical query parity.
- `transformer_registry_test.go` - Covers canonical serialized config keys and decoded FTS query parity.

## Decisions Made

- Canonicalization stays split by trust boundary: public query/config inputs must pass `ValidateJSONPath`, while internal builder traversal still uses permissive `NormalizePath`.
- Duplicate canonical path names are treated as invalid serialized state and rejected with `ErrInvalidFormat` during lookup rebuild.
- `pathLookup` remains non-serialized derived state to avoid binary format churn in this phase.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- PATH-01 and PATH-02 are covered for fresh and decoded indexes.
- Query hot-path benchmarks in Plan 06-02 can now measure canonical lookup performance against the new derived map.

## Self-Check: PASSED

- Summary file exists at `.planning/phases/06-query-path-hot-path/06-query-path-hot-path-01-SUMMARY.md`.
- Verified task commits `5233228`, `ae1a34c`, `481dba4`, `c7ddc7f`, and `0e75655` exist in git history.
