---
phase: 07-builder-parsing-numeric-fidelity
plan: 01
subsystem: indexing
tags: [json, parser, int64, serialization, transformers]
requires: []
provides:
  - transactional AddDocument staging with explicit numeric parsing
  - exact-int numeric metadata and int-aware query evaluation
  - regression coverage for atomic failure, decode parity, and transformer numeric paths
affects: [07-02, 08-adaptive-high-cardinality-indexing, 09-derived-representations, 10-serialization-compaction]
tech-stack:
  added: []
  patterns: [transactional document staging, exact-int numeric mode, transformer subtree materialization]
key-files:
  created: []
  modified: [builder.go, gin.go, query.go, serialize.go, gin_test.go, transformers_test.go, transformer_registry_test.go, serialize_security_test.go]
key-decisions:
  - "Keep AddDocument transactional by staging per-document observations and merging only after parse/validation succeeds."
  - "Store int-only path stats as exact int64 values and promote to float mode only when every integer remains exact inside float64."
  - "Preserve existing transformer input expectations by normalizing transformer-targeted subtrees before applying the transformer, then classify transformed outputs explicitly."
patterns-established:
  - "Parser path: stream objects by default, materialize only transformer-targeted subtrees and array items that need both indexed and wildcard paths."
  - "Numeric mode: ValueType 0 = int-only, ValueType 1 = float-or-mixed across build, query, and serialization."
requirements-completed: [BUILD-01, BUILD-02, BUILD-03, BUILD-04]
duration: 32min
completed: 2026-04-15
---

# Phase 07: Builder Parsing & Numeric Fidelity Summary

**Transactional explicit-number ingest with exact-int path semantics, guarded mixed-mode promotion, and decode-parity regressions**

## Performance

- **Duration:** 32 min
- **Started:** 2026-04-15T10:40:22Z
- **Completed:** 2026-04-15T11:12:36Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments
- Replaced eager `json.Unmarshal(..., &any)` ingest with transactional per-document staging driven by `json.Decoder` and `UseNumber()`.
- Added exact `int64` path storage/query behavior plus explicit rejection for lossy mixed integer/decimal promotion.
- Extended regression coverage to lock atomic failure, decode parity, transformer numeric compatibility, numeric transformer config round-trip behavior, and the new numeric decode bounds layout.

## Task Commits

Execution landed as one tightly-coupled implementation commit plus a small verification follow-up fix because the parser, numeric mode, and regression work shared the same core builder changes:

1. **Task 1: Replace eager generic decode with transactional explicit-number staging** - `cb5b7bf` (feat)
2. **Task 2: Add exact-int numeric mode and reject lossy mixed-path promotion** - `cb5b7bf` (feat)
3. **Task 3: Add regression coverage for atomic failure, transformer compatibility, and decode parity** - `cb5b7bf` (feat)
4. **Post-verification compatibility fix: preserve int-only `GlobalMin` / `GlobalMax` expectations and update numeric decode bounds coverage** - `fc813f9` (fix)

## Files Created/Modified
- `builder.go` - transactional document staging, explicit numeric classification, transformer-aware subtree materialization, and merge-on-success ingest
- `gin.go` - exact-int numeric metadata on `NumericIndex` and `RGNumericStat`
- `query.go` - int-aware equality and range evaluation for int-only numeric paths
- `serialize.go` - exact-int numeric field encode/decode support with format version bump
- `gin_test.go` - atomic failure, exact `int64` fidelity, mixed-promotion rejection, and decode-parity regressions
- `transformers_test.go` - transformer numeric compatibility and transformed exact-int decode parity coverage
- `transformer_registry_test.go` - numeric registered-transformer config/query round-trip coverage
- `serialize_security_test.go` - numeric index decode bounds coverage updated for the new int metadata layout

## Decisions Made
- Preserve the existing transformer contract by converting transformer input subtrees to legacy-style scalar shapes before running the transformer, but always classify transformed outputs through the new explicit numeric path.
- Keep the streaming parser path sparse by materializing array items only where both indexed and wildcard paths must be staged from the same value.
- Bump the binary format version when persisting new int-only numeric metadata so decode semantics stay explicit.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Int-only numeric indexes dropped legacy float globals after the exact-int refactor**
- **Found during:** Full-suite verification after Task 3
- **Issue:** Existing transformer/date tests still read `GlobalMin` / `GlobalMax`, and the numeric decode bounds test was still writing the pre-Phase-07 binary layout.
- **Fix:** Populate float global min/max alongside exact int globals for int-only paths during finalize, and update the numeric decode bounds test to the new encoded layout.
- **Files modified:** `builder.go`, `serialize_security_test.go`
- **Verification:** `go test ./... -run 'Test(DecodeBoundsNumericRGs|DateTransformerIntegration)' -count=1` and `go test ./... -count=1`
- **Committed in:** `fc813f9` (fix)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Compatibility fix only. No scope creep and no change to the Phase 07 requirements.

## Issues Encountered

- The stale milestone branch required a one-time sync/merge from `main` before execution could start.
- The subagent execution path did not return completion signals in this runtime, so the plan was executed inline under the orchestrator.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Builder, query, and serialization layers now agree on explicit numeric mode semantics, so benchmark deltas in Plan `07-02` can measure the new parser path directly.
- The final benchmark work can stay isolated to `benchmark_test.go`; no additional production changes are required for `BUILD-05`.

---
*Phase: 07-builder-parsing-numeric-fidelity*
*Completed: 2026-04-15*
