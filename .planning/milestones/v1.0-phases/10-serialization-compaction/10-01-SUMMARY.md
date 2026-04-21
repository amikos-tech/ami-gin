---
phase: 10-serialization-compaction
plan: 01
subsystem: serialization
tags: [serialization, compaction, front-coding, regression-tests]
requires:
  - phase: 08-adaptive-high-cardinality-indexing
    provides: adaptive string sections whose promoted terms must keep bitmap alignment
  - phase: 09-derived-representations
    provides: representation-bearing paths whose serialized layout must survive round-trip decode
provides:
  - v9 wire-format boundary for Phase 10 compaction
  - order-preserving ordered-string sections for path names, classic terms, and adaptive promoted terms
  - baseline round-trip and security coverage for the new compact section layout
affects: [10-02, 10-03, serialization-security, benchmarks]
tech-stack:
  added: []
  patterns: [names-first path directory payloads, per-section ordered-string framing, raw-on-tie deterministic fallback]
key-files:
  created: [.planning/phases/10-serialization-compaction/10-01-SUMMARY.md]
  modified: [gin.go, prefix.go, serialize.go, gin_test.go, serialize_security_test.go]
key-decisions:
  - "Kept compaction local to ordered-string payloads inside existing sections instead of introducing a cross-section dictionary or offset layer."
  - "Serialized path names as a single ordered-string stream ahead of per-entry metadata so decode can preserve PathID-to-metadata binding while validating mode bytes against the decoded path name."
  - "Updated legacy security fixtures to emit the new ordered-string framing rather than weakening the v9 strict reader contract."
patterns-established:
  - "Compact string sections always preserve caller order and choose raw mode on equal-size payloads."
  - "Phase-local wire-format changes must update both semantic round-trip tests and handcrafted malformed-payload fixtures in the security suite."
requirements-completed: [SIZE-01, SIZE-02, SIZE-03]
duration: 26min
completed: 2026-04-17
---

# Phase 10 Plan 01: Compact Wire Layout Summary

**v9 ordered-string sections now compact path names, classic terms, and adaptive promoted terms without changing decoded query semantics**

## Performance

- **Duration:** 26 min
- **Started:** 2026-04-17T16:27:00+03:00
- **Completed:** 2026-04-17T16:53:14+03:00
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- Bumped the wire format to v9 and added order-preserving ordered-string helpers with explicit raw/front-coded mode markers and raw-on-tie determinism.
- Refactored the path directory, classic string indexes, and adaptive promoted-term sections to write one ordered-string payload per section while keeping bitmap and metadata alignment intact after decode.
- Added baseline round-trip tests for path-directory, classic-term, and adaptive-term compaction, and updated handcrafted malformed-payload tests to the new compact framing.

## Task Commits

1. **Task 1: Introduce order-preserving compact-string helpers and bump the wire format version** - `2208490` (feat)
2. **Task 2: Apply compact ordered-string encoding to path, classic-term, and adaptive-term sections** - `c6de450` (feat)

## Files Created/Modified

- `gin.go` - bumps the binary format to v9 and centralizes the default prefix block size constant
- `prefix.go` - adds `CompressInOrder(...)` for caller-order-preserving front coding
- `serialize.go` - writes and reads compact ordered-string payloads for path names, classic terms, and adaptive promoted terms
- `gin_test.go` - adds `TestPathDirectoryCompactionRoundTrip`, `TestStringIndexCompactionRoundTrip`, and `TestAdaptiveStringIndexCompactionRoundTrip`
- `serialize_security_test.go` - rewrites handcrafted malformed-payload fixtures to the v9 ordered-string section framing

## Decisions Made

- Chose a names-first path-directory section layout so path names are decoded once, then rebound to fixed-width metadata in directory order.
- Reused the same ordered-string framing across path, classic, and adaptive sections to keep the v9 surface area narrow and testable.
- Kept zero-length string slices explicit via the existing raw/front-coded modes instead of inventing a third empty-section encoding.

## Verification Evidence

- Phase 10 compaction round-trip tests passed:
  `go test ./... -run 'Test(PathDirectoryCompactionRoundTrip|StringIndexCompactionRoundTrip|AdaptiveStringIndexCompactionRoundTrip)' -count=1`
- Updated malformed-layout regression cases passed:
  `go test ./... -run 'Test(DecodeRejectsUnknownPathMode|DecodeRejectsInvalidAdaptiveSections|DecodeAdaptiveSectionDerivesPathEntryCounts|DecodeRejectsTruncatedAdaptiveTerm|DecodeRejectsDuplicatePathSectionsAcrossReaders|DecodeRejectsOutOfOrderPathDirectoryIDs)' -count=1`
- Repo-wide suite passed:
  `go test ./... -count=1`

## Deviations from Plan

None in scope. The only extra work was updating pre-existing handcrafted security fixtures so they exercised the new ordered-string framing instead of the removed inline raw-string layout.

## Issues Encountered

- The first full-suite run exposed several security tests that manually constructed old path/term payloads. Those fixtures were migrated to the v9 compact section shape so the suite continued testing corruption and ordering failures against the actual on-wire format.

## User Setup Required

None.

## Next Phase Readiness

- `10-02` can now harden the compact format against corruption, version mismatch, and mode/payload disagreement using the new ordered-string section helpers.
- `10-03` can benchmark the final v9 compact sections directly; the baseline round-trip coverage is in place.
- `.planning/STATE.md` remains intentionally uncommitted shared state while the phase orchestrator advances execution.

## Self-Check

PASSED

- `FOUND: 2208490`
- `FOUND: c6de450`
- `FOUND: .planning/phases/10-serialization-compaction/10-01-SUMMARY.md`
- `FOUND: go test ./... -count=1`

---
*Phase: 10-serialization-compaction*
*Completed: 2026-04-17*
