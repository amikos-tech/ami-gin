---
phase: 11-real-corpus-prefix-compression-benchmarking
plan: 03
subsystem: documentation
tags: [benchmarks, docs, report, huggingface]
requires:
  - phase: 11-real-corpus-prefix-compression-benchmarking
    provides: pinned benchmark evidence in 11-BENCHMARK-RESULTS.md
provides:
  - final Phase 11 recommendation report
  - README workflow for smoke, subset, and large corpus benchmarking
  - explicit guidance that default benchmarking remains smoke-only
affects: [README, phase-closeout, verify-work]
tech-stack:
  added: []
  patterns: [results-backed recommendation writing, opt-in benchmark documentation, pinned snapshot reproducibility]
key-files:
  created: [.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-03-SUMMARY.md, .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-REAL-CORPUS-REPORT.md]
  modified: [README.md]
key-decisions:
  - "Recommended no further serialization-format work now because the external subset and large tiers stayed effectively flat."
  - "Kept README reproduction guidance smoke-first and opt-in only for subset/large so normal developer runs stay lightweight."
  - "Linked the README directly to the raw results and the narrative report so future readers can inspect provenance before acting on the recommendation."
patterns-established:
  - "Phase reports cite the checked-in raw-results artifact instead of paraphrasing benchmark output from memory."
  - "Benchmark workflow docs pin the dataset revision and acquisition path whenever external corpora are part of the evidence base."
requirements-completed: []
duration: 19min
completed: 2026-04-20
---

# Phase 11 Plan 03: Report and Reproduction Guidance Summary

**Phase 11 now closes with an evidence-backed recommendation report and a README workflow that keeps smoke default while documenting the opt-in external corpus path**

## Performance

- **Duration:** 19 min
- **Started:** 2026-04-20T15:05:00+03:00
- **Completed:** 2026-04-20T15:23:46+03:00
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Wrote `11-REAL-CORPUS-REPORT.md` with explicit `Dataset`, `Tier Matrix`, `Helps`, `Flat / No-Win`, and `Recommendation` sections grounded in `11-BENCHMARK-RESULTS.md`.
- Updated `README.md` with the exact smoke command, the pinned `common-pile/github_archive` acquisition snippet, the required env vars, and tier-specific resource notes.
- Recorded the phase conclusion that Phase 10’s prefix-compaction work does not justify more format changes now for this real corpus because subset and large raw savings stayed near zero.

## Task Commits

1. **Task 1: Write the checked-in interpretive Phase 11 report** - recorded in the Phase 11 closeout docs commit with this summary
2. **Task 2: Document the opt-in workflow in README without changing the default benchmark surface** - recorded in the Phase 11 closeout docs commit with this summary

## Files Created/Modified

- `.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-REAL-CORPUS-REPORT.md` - interpretive report tied directly to the raw benchmark artifact
- `README.md` - smoke/default benchmark command plus opt-in subset/large workflow and pinned snapshot example

## Decisions Made

- Recommended against more serialization-format work now for prefix compaction alone.
- Kept the default README benchmark path smoke-only while preserving exact subset/large reproduction instructions.
- Carried forward the pinned revision and raw-results links so readers can audit the conclusion.

## Verification Evidence

- Report structure and evidence references passed:
  `rg -n '^## (Dataset|Tier Matrix|Helps|Flat / No-Win|Recommendation)$' .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-REAL-CORPUS-REPORT.md`
- README workflow and opt-in wording passed:
  `rg -n 'GIN_PHASE11_GITHUB_ARCHIVE_ROOT|GIN_PHASE11_ENABLE_SUBSET|GIN_PHASE11_ENABLE_LARGE|BenchmarkPhase11RealCorpus/tier=smoke|common-pile/github_archive|gharchive/v0/documents/\\*\\.jsonl\\.gz|11-BENCHMARK-RESULTS\\.md|11-REAL-CORPUS-REPORT\\.md|opt-in only' README.md`
- Repo-wide suite remained green on the current tree:
  `go test ./... -count=1`

## Deviations from Plan

None. The plan executed as written once `11-02` provided the raw evidence artifact.

## Issues Encountered

- None beyond the prior `11-02` decode-cap handling already captured in `11-02-SUMMARY.md`.

## User Setup Required

None. The README now documents the exact optional setup for future subset/large runs.

## Next Phase Readiness

- Phase 11 execution is complete and ready for `$gsd-verify-work 11`.
- The benchmark workflow is documented enough for future maintainers to reproduce or challenge the recommendation with a different corpus later.

## Self-Check

PASSED

- `FOUND: .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-REAL-CORPUS-REPORT.md`
- `FOUND: README.md`
- `FOUND: 11-BENCHMARK-RESULTS.md`
- `FOUND: go test ./... -count=1`

---
*Phase: 11-real-corpus-prefix-compression-benchmarking*
*Completed: 2026-04-20*
