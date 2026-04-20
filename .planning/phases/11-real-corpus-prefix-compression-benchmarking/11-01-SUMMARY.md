---
phase: 11-real-corpus-prefix-compression-benchmarking
plan: 01
subsystem: benchmarking
tags: [benchmarks, jsonl, corpora, prefix-compaction]
requires:
  - phase: 10-serialization-compaction
    provides: Phase 10 size-accounting helpers and compact ordered-string serializer behavior
provides:
  - checked-in synthesized smoke corpus for Phase 11
  - env-gated `BenchmarkPhase11RealCorpus` smoke/subset/large benchmark tree
  - structured and text-heavy projections over the same corpus shape
affects: [11-02, 11-03, README]
tech-stack:
  added: []
  patterns: [synthesized-from-shape fixture provenance, env-gated external shard discovery, projection-stable corpus benchmarking]
key-files:
  created: [.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-01-SUMMARY.md, testdata/phase11/README.md, testdata/phase11/github_archive_smoke.jsonl]
  modified: [benchmark_test.go]
key-decisions:
  - "Used a synthesized-from-shape smoke fixture instead of direct upstream rows because redistributable row-level licensing was not explicit for this repository."
  - "Kept subset and large tiers opt-in behind env vars while still registering stable `tier=` and `projection=` benchmark branches."
  - "Built both projections from the same corpus shape so Phase 11 can compare repeated metadata wins against text-heavy flat/no-win cases without introducing a second corpus."
patterns-established:
  - "Real-corpus benchmark tiers use a checked-in smoke file plus deterministic external shard counts from `gharchive/v0/documents/*.jsonl.gz`."
  - "Phase 11 benchmark metrics extend Phase 10 accounting names instead of introducing a second size vocabulary."
requirements-completed: []
duration: 15min
completed: 2026-04-20
---

# Phase 11 Plan 01: Smoke Corpus and Benchmark Structure Summary

**Phase 11 now has a reproducible smoke corpus plus an env-gated real-corpus benchmark family that contrasts structured metadata against text-heavy payloads**

## Performance

- **Duration:** 15 min
- **Started:** 2026-04-20T14:40:00+03:00
- **Completed:** 2026-04-20T14:54:43+03:00
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Added `testdata/phase11/github_archive_smoke.jsonl` as a deterministic synthesized-from-shape NDJSON fixture with 640 rows and nested `metadata.repo/url/license/license_type` fields.
- Added `testdata/phase11/README.md` with pinned dataset revision `93d90fbdbc8f06c1fab72e74d5270dc897e1a090`, explicit fixture origin, and the required external shard layout.
- Added `BenchmarkPhase11RealCorpus` to `benchmark_test.go` with smoke/subset/large tiers, structured/text-heavy projections, env-gated external shard discovery, and Phase 10-style size metrics.

## Task Commits

1. **Task 1: Add the checked-in smoke fixture and provenance note** - `6c29195` (feat)
2. **Task 2: Add deterministic tier and projection loading with env-gated skip semantics** - `6c29195` (feat)

## Files Created/Modified

- `benchmark_test.go` - adds Phase 11 corpus loaders, projection builders, external shard validation, and the `BenchmarkPhase11RealCorpus` family
- `testdata/phase11/github_archive_smoke.jsonl` - synthesized smoke corpus matching the locked `common-pile/github_archive` record shape
- `testdata/phase11/README.md` - pinned revision, provenance, smoke corpus counts, and external layout requirements

## Decisions Made

- Used a synthesized-from-shape smoke corpus rather than redistributing direct upstream rows.
- Reused Phase 10 metric names and benchmark structure so the real-corpus work stays comparable with prior serializer evidence.
- Registered subset and large tiers unconditionally, but made them skip unless their opt-in env vars are explicitly set.

## Verification Evidence

- Helper red/green tests passed:
  `go test ./... -run 'TestPhase11(DiscoverExternalShardsRejectsMissingLayout|LoadSmokeFixture)' -count=1`
- Default smoke benchmark and skip semantics passed:
  `go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus' -benchtime=1x -count=1`
- Invalid external root fails with the required env-var and layout hint:
  `GIN_PHASE11_ENABLE_SUBSET=1 GIN_PHASE11_GITHUB_ARCHIVE_ROOT=/definitely-missing go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=subset' -benchtime=1x -count=1`
- Repo-wide suite passed:
  `go test ./... -count=1`

## Deviations from Plan

None in scope. The only judgment call was choosing a synthesized-from-shape smoke fixture, which was already permitted by the plan when direct redistribution safety was unclear.

## Issues Encountered

- `go test -bench` does not fail when a benchmark is missing, so the red step used helper tests for shard validation and smoke-fixture loading instead of relying on a missing-benchmark failure.
- The first invalid-root error mentioned the env var but omitted the expected shard layout, so the loader error contract was tightened before final verification.

## User Setup Required

`11-02` needs a local `common-pile/github_archive` snapshot rooted so `${GIN_PHASE11_GITHUB_ARCHIVE_ROOT}/gharchive/v0/documents/*.jsonl.gz` exists at revision `93d90fbdbc8f06c1fab72e74d5270dc897e1a090`.

## Next Phase Readiness

- `11-02` can extend the checked-in benchmark output into a results artifact without reworking the benchmark tree again.
- The only remaining blocker for `11-02` is acquiring a local snapshot root for the opt-in subset and large tiers.

## Self-Check

PASSED

- `FOUND: 6c29195`
- `FOUND: testdata/phase11/github_archive_smoke.jsonl`
- `FOUND: testdata/phase11/README.md`
- `FOUND: BenchmarkPhase11RealCorpus`
- `FOUND: go test ./... -count=1`

---
*Phase: 11-real-corpus-prefix-compression-benchmarking*
*Completed: 2026-04-20*
