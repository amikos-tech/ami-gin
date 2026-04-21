---
phase: 09-derived-representations
plan: 03
subsystem: docs
tags: [derived-representations, docs, examples, acceptance-tests]
requires:
  - phase: 09-derived-representations
    provides: explicit alias routing and v7 representation metadata from 09-02
provides:
  - public docs and runnable examples that teach additive raw-plus-companion semantics
  - acceptance coverage for date, normalized text, and extracted-subfield alias patterns
  - explicit custom-transformer serialization caveat in README
affects: [10-serialization-compaction]
tech-stack:
  added: []
  patterns: [raw-path coexistence, explicit alias queries, example-backed acceptance coverage]
key-files:
  created: [.planning/phases/09-derived-representations/09-03-SUMMARY.md]
  modified: [README.md, examples/transformers/main.go, examples/transformers-advanced/main.go, transformers_test.go]
key-decisions:
  - "Teach all built-in derived patterns through helper-style registrations plus `gin.As(alias, value)` instead of replacement-era `WithFieldTransformer` examples."
  - "Use acceptance fixtures that satisfy strict companion derivation semantics rather than implying partial-match fallback for regex companions."
  - "Document custom companion serialization as an explicit Encode-time limitation instead of burying it in implementation details."
patterns-established:
  - "Public examples demonstrate both raw-path queries and companion queries against the same source field."
  - "Each required alias pattern family proves encode/decode parity in tests before the docs point users at it."
requirements-completed: [DERIVE-04]
duration: 64min
completed: 2026-04-16
---

# Phase 09 Plan 03: Public Docs, Examples, and Acceptance Coverage Summary

**Alias-aware docs/examples plus DERIVE-04 acceptance coverage for date, normalized text, and extracted-subfield companions**

## Performance

- **Duration:** 64 min
- **Started:** 2026-04-16T20:44:00Z
- **Completed:** 2026-04-16T21:48:00Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Added end-to-end acceptance tests for date/time, normalized text, and extracted-subfield alias queries, with raw-path coexistence and before/after encode/decode parity checks.
- Rewrote the README and both transformer examples around additive helper-style registrations and explicit `gin.As(alias, value)` querying.
- Added a clear README note that `WithCustomTransformer(...)` companions are not serializable and therefore rejected by `Encode()`.

## Task Commits

1. **Task 2: Add DERIVE-04 acceptance coverage for derived alias patterns** - `dc2885f` (test)
2. **Task 1: Rewrite docs and runnable examples around additive alias-aware semantics** - `847a9b8` (docs)

## Files Created/Modified
- `transformers_test.go` - adds `TestDateTransformerAliasCoverage`, `TestNormalizedTextAliasCoverage`, and `TestRegexExtractAliasCoverage`
- `README.md` - replaces replacement-era transformer guidance with additive alias-aware docs and custom-transformer serialization caveat
- `examples/transformers/main.go` - demonstrates raw date queries plus explicit `epoch_ms` companion queries
- `examples/transformers-advanced/main.go` - demonstrates alias-aware IP, semver, normalized text, regex extract, and duration examples without hidden-path leakage

## Decisions Made

- Used the public alias wrapper consistently in README/examples even for older helper categories like IP and semver so the docs tell one coherent story.
- Removed replacement-style examples entirely rather than trying to explain both contracts side by side.
- Corrected regex-derived fixtures to ensure every indexed source value matches the configured extraction pattern, aligning the public examples with strict companion semantics.

## Verification Evidence

- DERIVE-04 acceptance coverage passed:
  `go test ./... -run 'Test(DateTransformerAliasCoverage|NormalizedTextAliasCoverage|RegexExtractAliasCoverage)' -count=1`
- README/examples no longer expose replacement-era APIs or hidden target paths:
  `rg -n 'WithFieldTransformer|__derived:' README.md examples/transformers/main.go examples/transformers-advanced/main.go`
  No matches found.
- Runnable examples passed:
  `go run ./examples/transformers/main.go`
  `go run ./examples/transformers-advanced/main.go`
- Repo-wide suite passed:
  `go test ./... -count=1`

## Deviations from Plan

None - the wave stayed within the README/example/test acceptance scope.

## Issues Encountered

- The first regex acceptance fixture included a non-matching source value, which now correctly fails indexing under the strict companion-derivation rules introduced in 09-01. The fixture was updated so the public acceptance story matches the intended contract.

## User Setup Required

None.

## Next Phase Readiness

- Phase 09 is complete: the feature now has additive builder semantics, explicit public alias routing, explicit serialization metadata, and public-facing evidence.
- Phase 10 can focus on serialization compaction without reopening the derived-representation contract.
- `.planning/STATE.md` and `.planning/ROADMAP.md` remain intentionally uncommitted shared artifacts after execution.

## Self-Check

PASSED

- `FOUND: .planning/phases/09-derived-representations/09-03-SUMMARY.md`
- `FOUND: dc2885f`
- `FOUND: 847a9b8`
- `FOUND: go test ./... -count=1`

---
*Phase: 09-derived-representations*
*Completed: 2026-04-16*
