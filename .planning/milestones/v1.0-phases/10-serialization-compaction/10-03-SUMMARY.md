---
phase: 10-serialization-compaction
plan: 03
subsystem: testing
tags: [benchmarks, serialization, compaction, performance]
requires:
  - phase: 10-serialization-compaction
    provides: compact v9 ordered-string sections plus hardening coverage from 10-01 and 10-02
provides:
  - phase 10 fixture matrix for mixed, high-prefix, and random-like data shapes
  - exact legacy raw-wire versus compact raw-wire versus default zstd reporting
  - encode/decode/query-after-decode timing evidence for the compact format
affects: [phase-verification, release-notes, future-benchmarking]
tech-stack:
  added: []
  patterns: [fixture-backed raw-wire accounting, benchmark metrics on subbench leaves, query-after-decode smoke probes]
key-files:
  created: [.planning/phases/10-serialization-compaction/10-03-SUMMARY.md]
  modified: [benchmark_test.go]
key-decisions:
  - "Computed legacy raw-wire bytes exactly from the old per-string layout for the changed sections instead of preserving a second serializer."
  - "Reported negative or flat compaction results directly so the benchmark matrix shows where front coding does not help."
  - "Attached raw-path, alias, and adaptive query smoke probes to the benchmark family instead of treating size and query evidence as unrelated harnesses."
patterns-established:
  - "Benchmark leaves report size metrics and timing evidence together so one command captures both claims."
  - "Phase-local wire-format benchmarks should expose representative no-win cases instead of collapsing them into an averaged success number."
requirements-completed: [SIZE-01, SIZE-02, SIZE-03]
duration: 6min
completed: 2026-04-17
---

# Phase 10 Plan 03: Benchmark Evidence Summary

**Fixture-backed raw/zstd size accounting plus encode, decode, and post-decode query evidence for the v9 compact wire format**

## Performance

- **Duration:** 6 min
- **Started:** 2026-04-17T17:04:12+03:00
- **Completed:** 2026-04-17T17:10:37+03:00
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments

- Added `buildPhase10MixedFixture`, `buildPhase10HighPrefixFixture`, and `buildPhase10RandomLikeFixture` plus a single `BenchmarkPhase10SerializationCompaction` family with fixture-local `Size`, `Encode`, `Decode`, and `QueryAfterDecode` leaves.
- Reported exact `legacy_raw_bytes`, `compact_raw_bytes`, `default_zstd_bytes`, and `bytes_saved_pct` metrics by reconstructing only the old raw-string layout for the sections changed in Phase 10.
- Included concrete post-decode probes for raw-path equality on every fixture, a representation-aware alias probe on the mixed fixture, and an adaptive/high-cardinality probe on the high-prefix fixture.

## Task Commits

1. **Task 1: Add Phase 10 fixture families and exact raw-wire baseline reporting** - `e893acd` (test)
2. **Task 2: Measure encode/decode costs with reproducible timing and post-decode query smoke** - `2f19aa4` (test; refined the mixed fixture so the representative case actually exercises string-section savings instead of being dominated by bitmap overhead)

## Files Created/Modified

- `benchmark_test.go` - adds the Phase 10 benchmark fixtures, exact legacy-versus-compact accounting helpers, and encode/decode/query-after-decode benchmark branches

## Decisions Made

- Used `EncodeWithLevel(..., CompressionNone)` for exact raw-wire accounting and `Encode(...)` for default compressed reporting so both raw and zstd views remain visible.
- Kept the benchmark repo-local and single-package by deriving legacy raw-wire bytes from current in-memory structures rather than maintaining a parallel legacy serializer implementation.
- Let the benchmark output show that high-prefix data wins slightly on raw bytes while mixed/random-like fixtures stay effectively flat-to-negative, matching the phase research rather than papering over it.

## Verification Evidence

- Benchmark harness smoke passed:
  `go test ./... -run '^$' -bench 'BenchmarkPhase10SerializationCompaction' -benchtime=1x -count=1`
- Benchmark timing evidence passed:
  `go test ./... -run '^$' -bench 'BenchmarkPhase10SerializationCompaction' -benchtime=1s -count=3 -benchmem`

Representative output from the timing run:

- **Mixed**
  `legacy_raw_bytes=65294`, `compact_raw_bytes=63500`, `default_zstd_bytes=3848`, `bytes_saved_pct=2.748`
  encode: ~15.1-19.1 ms/op, decode: ~2.22-3.00 ms/op, query-after-decode: raw ~1.58-2.00 µs/op, alias ~1.58-1.75 µs/op
- **HighPrefix**
  `legacy_raw_bytes=31702`, `compact_raw_bytes=31594`, `default_zstd_bytes=1344`, `bytes_saved_pct=0.3407`
  encode: ~10.7-10.9 ms/op, decode: ~1.26-1.38 ms/op, query-after-decode adaptive probe: ~1.23-1.46 µs/op
- **RandomLike**
  `legacy_raw_bytes=37037`, `compact_raw_bytes=37052`, `default_zstd_bytes=3581`, `bytes_saved_pct=-0.04050`
  encode: ~9.63-11.48 ms/op, decode: ~1.34-1.62 ms/op, query-after-decode raw probe: ~1.18-1.31 µs/op

## Deviations from Plan

None. The follow-up mixed-fixture refinement stayed within the plan’s discretion and improved the representative-case signal without hiding the random-like no-win case.

## Issues Encountered

- The first benchmark draft missed a `bytes` import for the exact compact-payload accounting helper. That was corrected before the first successful smoke run; no benchmark semantics changed afterward.

## User Setup Required

None.

## Next Phase Readiness

- Phase 10 now has implementation, hardening, and evidence coverage across all three plans.
- Remaining work is orchestration-only: update shared tracking, run final phase verification, and mark the phase complete.
- `.planning/ROADMAP.md` and `.planning/STATE.md` remain intentionally uncommitted shared artifacts while final phase orchestration finishes.

## Self-Check

PASSED

- `FOUND: e893acd`
- `FOUND: 2f19aa4`
- `FOUND: .planning/phases/10-serialization-compaction/10-03-SUMMARY.md`
- `FOUND: go test ./... -run '^$' -bench 'BenchmarkPhase10SerializationCompaction' -benchtime=1s -count=3 -benchmem`

---
*Phase: 10-serialization-compaction*
*Completed: 2026-04-17*
