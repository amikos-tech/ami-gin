---
phase: 10-serialization-compaction
plan: 02
subsystem: testing
tags: [serialization, security, compatibility, ordered-strings]
requires:
  - phase: 10-serialization-compaction
    provides: v9 ordered-string sections from 10-01
provides:
  - compact-section corruption rejection with ErrInvalidFormat normalization
  - front-coded ordered-string block and entry count rejection before oversized allocation
  - explicit rebuild-only version rejection for v8 and older payloads
  - representation alias parity coverage across compact-format round trips
affects: [10-03, phase-verification, serialization-security]
tech-stack:
  added: []
  patterns: [fail-closed ordered-string decode, mode/payload mismatch coverage, compact-format parity checks]
key-files:
  created: [.planning/phases/10-serialization-compaction/10-02-SUMMARY.md]
  modified: [serialize.go, serialize_security_test.go]
key-decisions:
  - "Normalized malformed ordered-string reads to ErrInvalidFormat so corrupted compact sections fail closed instead of leaking raw EOFs."
  - "Rejected impossible front-coded block and entry counts before allocating decoded ordered-string structures."
  - "Extended version mismatch coverage to explicitly reject version 8 as the immediate pre-Phase-10 payload."
  - "Kept raw-path and alias-path parity assertions on the compact round-trip itself instead of relying on pre-Phase-10 representation tests."
patterns-established:
  - "Compact-section readers wrap truncated or malformed payloads with ErrInvalidFormat before Decode re-wraps section context."
  - "Front-coded ordered-string readers must bound both block count and per-block entry totals against the caller's expected decoded count."
  - "Handcrafted wire-fixture tests must model the current section framing exactly, including ordered-string subpayloads."
requirements-completed: [SIZE-01, SIZE-02, SIZE-03]
duration: 11min
completed: 2026-04-17
---

# Phase 10 Plan 02: Compact Format Hardening Summary

**Fail-closed compact-section decoding plus explicit v8 rejection and raw/alias parity checks for the v9 wire format**

## Performance

- **Duration:** 11 min
- **Started:** 2026-04-17T16:53:14+03:00
- **Completed:** 2026-04-17T17:04:12+03:00
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Added corruption coverage for compact path and compact term sections, explicit ordered-string mode/payload mismatch coverage, and deterministic raw-on-tie regression coverage.
- Hardened `readOrderedStrings(...)` so malformed raw/front-coded payloads surface as `ErrInvalidFormat` instead of raw EOF-class errors.
- Added explicit front-coded block-count and entry-count guards so impossible ordered-string payloads are rejected before oversized per-block allocations.
- Expanded compatibility/parity coverage so version `8` is explicitly rejected and representation-bearing indexes preserve both raw-path and alias-path query results after compact round-trip decode.

## Task Commits

1. **Task 1: Lock corruption rejection and deterministic ordered-string mode semantics** - `421ec89` (fix)
2. **Task 2: Lock rebuild-only compatibility and representation alias parity on the compact format** - `421ec89` (fix; landed with Task 1 because the new hardening tests and the compatibility/parity assertions exercised the same security-test surface)

## Files Created/Modified

- `serialize.go` - wraps malformed ordered-string reads as `ErrInvalidFormat` and rejects oversized front-coded block/entry counts before allocation
- `serialize_security_test.go` - adds compact corruption, mismatch, deterministic tie-break, oversized-count, v8 rejection, and compact alias-parity coverage

## Decisions Made

- Treated malformed ordered-string payloads as format corruption at the helper level so section-level Decode errors remain sentinel-classified after outer context wrapping.
- Treated front-coded block counts and per-block entry totals as untrusted size fields and bounded them against `expectedCount` before decoding or allocating block entries.
- Used compact-format uncompressed round trips for alias parity so the test exercises the actual v9 section layout instead of zstd-specific framing.
- Preserved the existing `TestDecodeVersionMismatch` anchor and extended it with the immediate previous version rather than introducing a separate weaker compatibility test.

## Verification Evidence

- Compact corruption, mismatch, tie-break, version, and alias parity tests passed:
  `go test ./... -run 'Test(DecodeRejectsCompactPathSectionCorruption|DecodeRejectsCompactTermSectionCorruption|DecodeRejectsOrderedStringModePayloadMismatch|ReadOrderedStringsRejectsFrontCodedOversized(Block|Entry)Count|WriteOrderedStringsPrefersRawOnTie|DecodeVersionMismatch|DecodeLegacyRejected|DecodeRepresentationAliasParity)' -count=1`
- Updated truncation expectation passed:
  `go test ./... -run 'TestDecodeRejectsTruncatedAdaptiveTerm' -count=1`
- Repo-wide suite passed:
  `go test ./... -count=1`

## Deviations from Plan

None. The production change stayed narrow: only `readOrderedStrings(...)` needed hardening after the new corruption tests exposed raw EOF propagation.

## Issues Encountered

- The first targeted red pass showed the compact reader still relied on decoded-count mismatch after allocation for impossible front-coded block and entry totals. Bounding those counts in `readOrderedStrings(...)` closed the remaining Phase 10 hardening gap without widening the v9 reader surface.

## User Setup Required

None.

## Next Phase Readiness

- The compact wire format now has explicit corruption, mismatch, version, and parity coverage, so `10-03` can focus strictly on size/timing/query benchmark evidence.
- `.planning/ROADMAP.md` and `.planning/STATE.md` remain intentionally uncommitted shared artifacts while wave 2 continues.

## Self-Check

PASSED

- `FOUND: 421ec89`
- `FOUND: .planning/phases/10-serialization-compaction/10-02-SUMMARY.md`
- `FOUND: go test ./... -count=1`

---
*Phase: 10-serialization-compaction*
*Completed: 2026-04-17*
