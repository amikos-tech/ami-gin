---
phase: 02-security-hardening
plan: 01
subsystem: security
tags: [deserialization, bounds-checking, dos-prevention, sentinel-errors]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: Module path, license, clean codebase
provides:
  - ErrVersionMismatch and ErrInvalidFormat sentinel errors for programmatic error handling
  - Bounds checks on all stream-controlled allocation sites in Decode()
  - Version validation rejecting unknown binary format versions
  - Legacy fallback branch removed (no more silent zstd re-attempt)
affects: [cli, parquet-integration, downstream-consumers]

# Tech tracking
tech-stack:
  added: []
  patterns: [bounds-check-before-make, sentinel-error-wrapping, header-derived-limits]

key-files:
  created: [serialize_security_test.go]
  modified: [serialize.go]

key-decisions:
  - "Absolute constant (16MB) for readRGSet instead of threading numRGs parameter -- avoids signature changes to 3+ callers"
  - "Header-derived bounds for readDocIDMapping, readNumericIndexes, readStringLengthIndexes -- structural anchors exist"
  - "All bounds violations wrap ErrInvalidFormat with actual/max values for debugging"

patterns-established:
  - "Bounds check pattern: read size from stream, check against constant/header, allocate"
  - "Sentinel error pattern: var ErrX = errors.New() + errors.Wrapf(ErrX, context) for errors.Is() support"

requirements-completed: [SEC-01, SEC-02, SEC-03]

# Metrics
duration: 10min
completed: 2026-03-26
---

# Phase 2 Plan 1: Deserialization Security Hardening Summary

**Bounds checks on all 10 stream-controlled allocation sites, version validation with ErrVersionMismatch/ErrInvalidFormat sentinels, legacy fallback removed**

## Performance

- **Duration:** 10 min
- **Started:** 2026-03-26T19:23:24Z
- **Completed:** 2026-03-26T19:33:24Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- All stream-controlled allocation sites in serialize.go now have bounds checks preventing memory exhaustion from crafted payloads
- Decode() rejects unknown versions with ErrVersionMismatch sentinel error
- Legacy fallback branch removed -- unrecognized magic bytes return ErrInvalidFormat
- 14 new security-focused tests covering version validation, legacy rejection, sentinel error wrapping, and bounds checks for every guarded site
- No new dependencies, no API signature changes

## Task Commits

Each task was committed atomically:

1. **Task 1: Version validation and legacy fallback removal** - `0221d5f` (fix)
   - RED: `8a1c0a9` (test: failing tests for version validation)
   - GREEN: `0221d5f` (fix: sentinel errors, version check, legacy removal)
2. **Task 2: Bounds checks on all allocation sites** - `5d93c6e` (fix)
   - RED: `43fa9aa` (test: failing tests for bounds checks)
   - GREEN: `5d93c6e` (fix: constants and guards on all sites)

## Files Created/Modified
- `serialize.go` - Added 7 bounds constants, 2 sentinel errors, version validation in readHeader, bounds checks in readRGSet, readPathDirectory, readBloomFilter, readStringIndexes, readStringLengthIndexes, readNumericIndexes, readNullIndexes, readTrigramIndexes, readHyperLogLogs, readDocIDMapping; removed legacy fallback branch
- `serialize_security_test.go` - 14 new tests: TestDecodeVersionMismatch, TestDecodeLegacyRejected, TestSentinelErrors, TestDecodeRoundTripRegression, TestDecodeBoundsRGSet, TestDecodeBoundsPathDirectory, TestDecodeBoundsStringIndexes, TestDecodeBoundsTrigramIndexes, TestDecodeBoundsDocIDMapping, TestDecodeBoundsBloomFilter, TestDecodeBoundsHLLRegisters, TestDecodeBoundsNumericRGs, TestDecodeBoundsStringLengthRGs, TestDecodeCraftedPayload

## Decisions Made
- Used absolute constant (maxRGSetSize = 16MB) for readRGSet instead of threading numRGs parameter -- avoids changing readRGSet signature and cascading to 3+ callers (readStringIndexes, readNullIndexes, readTrigramIndexes)
- Threaded maxRGs from Header.NumRowGroups into readNumericIndexes and readStringLengthIndexes since structural anchors exist
- Threaded maxDocs from Header.NumDocs into readDocIDMapping per D-02
- Added numPaths bounds checks in all 6 functions that read numPaths from stream (string, string-length, numeric, null, trigram, HLL indexes)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Import ordering flagged by gci linter (stderrors "errors" alias needs to be in stdlib group) -- fixed immediately

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Deserialization path is now hardened against crafted/corrupted .gin files
- Sentinel errors enable programmatic error handling in CLI and downstream consumers
- Ready for next phase (CI/documentation)

## Self-Check: PASSED

- serialize_security_test.go: FOUND
- serialize.go: FOUND
- 02-01-SUMMARY.md: FOUND
- Commit 8a1c0a9: FOUND
- Commit 0221d5f: FOUND
- Commit 43fa9aa: FOUND
- Commit 5d93c6e: FOUND

---
*Phase: 02-security-hardening*
*Completed: 2026-03-26*
