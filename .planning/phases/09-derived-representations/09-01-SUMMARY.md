---
phase: 09-derived-representations
plan: 01
subsystem: indexing
tags: [derived-representations, transformers, builder, jsonpath, testing]
requires:
  - phase: 07-builder-parsing-numeric-fidelity
    provides: transactional document staging so companion failures can abort without partial merges
provides:
  - raw source paths remain indexed while companion representations stage under hidden alias paths
  - strict companion derivation failures with source-path and alias context
  - additive regression coverage for hidden representation paths and sibling non-chaining
affects: [09-02, 09-03, 10-serialization-compaction]
tech-stack:
  added: []
  patterns: [raw-plus-companion staging, reserved __derived: internal namespace, internal-path testing before public alias routing]
key-files:
  created: [.planning/phases/09-derived-representations/09-01-SUMMARY.md]
  modified: [builder.go, gin.go, jsonpath.go, gin_test.go, transformer_registry_test.go, transformers_test.go]
key-decisions:
  - "Keep raw source indexing intact and fan companions out separately instead of mutating the source value in-place."
  - "Treat companion derivation failure as a hard AddDocument error with alias+path context rather than silently falling back to raw-only indexing."
  - "Reject __derived: in public JSONPath validation while preserving it verbatim during internal path-lookup rebuilds."
patterns-established:
  - "Wave 1 tests query hidden companion paths through internal helpers; public alias routing is deferred to Wave 2."
  - "Object and array source paths keep raw subtree staging even when companion representations are registered."
requirements-completed: [DERIVE-01]
duration: 54min
completed: 2026-04-16
---

# Phase 09 Plan 01: Derived Representation Builder Semantics Summary

**Additive raw-plus-companion staging that preserves public source values, materializes hidden sibling representations, and fails companion derivation explicitly**

## Performance

- **Duration:** 54 min
- **Started:** 2026-04-16T18:30:00Z
- **Completed:** 2026-04-16T19:23:40Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Reworked builder staging so registered companions fan out from the same prepared raw source value while the public source path keeps its original indexed content.
- Added strict companion failure handling and explicit internal-path rules for `__derived:` during validation and path-lookup rebuilds.
- Migrated the transformer-focused tests from replacement semantics to additive semantics using hidden-path assertions until Wave 2 adds public alias query routing.

## Task Commits

1. **Task 1: Add additive representation registration types and alias-bearing helper APIs** - `ddaeab6` (feat)
2. **Task 2: Fan out raw-plus-companion staging and enforce strict companion failures** - `d670028` (feat)

## Files Created/Modified
- `builder.go` - stages companions under hidden alias paths, preserves raw staging, and converts companion `ok=false` into explicit errors
- `gin.go` - makes internal representation-path handling explicit during path lookup rebuild and keeps compatibility for legacy benchmark helpers
- `jsonpath.go` - rejects public `__derived:` path usage explicitly
- `gin_test.go` - covers raw-plus-companion staging, internal lookup preservation, and reserved-path validation
- `transformer_registry_test.go` - validates decoded numeric transformer behavior against hidden companion paths
- `transformers_test.go` - shifts transformer integration expectations to additive semantics and adds strict-failure/non-chaining regressions

## Decisions Made

- Used hidden `__derived:<canonical>#<alias>` paths as the Wave 1 verification surface rather than exposing any public alias-query API early.
- Preserved descendant raw indexing for object/array sources even when the parent path also has registered companions.
- Kept a small `firstRepresentation` compatibility helper only for legacy benchmark code while the builder itself now uses the full registration list.

## Verification Evidence

- Wave 1 additive builder tests passed:
  `go test ./... -run 'Test(BuilderIndexesRawAndCompanionRepresentations|RebuildPathLookupPreservesInternalRepresentationPaths|ValidateJSONPathRejectsInternalRepresentationPaths|BuilderFailsWhenCompanionTransformFails|SiblingTransformersDoNotChain)' -count=1`
- Replacement-era transformer expectations migrated and passed:
  `go test ./... -run 'Test(ConfigSerializationNumericTransformerPath|DateTransformerIntegration|DateTransformerRangeQuery|DateTransformerCanonicalConfigPath|DateTransformerDecodeCanonicalQueries|WildcardSubtreeTransformerNormalizesNestedNumbers|IPv4ToIntRangeQuery|SemVerToIntRangeQuery|ToLowerIntegration|InSubnetIntegration|TransformerNumericPathExplicitParserCompatibility|TransformerNumericDecodeParity)' -count=1`
- Repo-wide suite passed:
  `go test ./... -count=1`

## Deviations from Plan

None - the code changes stayed within the Plan 01 scope.

## Issues Encountered

- The initial `gsd-executor` subagent never returned a checkpoint or completion marker, so execution was resumed locally after validating that the branch state already contained the expected additive config layer and that no code changes had been applied by the stalled agent.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Wave 2 can build explicit alias-routing and serialization metadata on top of stable hidden companion paths already emitted by the builder.
- Existing transformer coverage now distinguishes raw-path behavior from companion-path behavior, which should make the Wave 2 alias-query API easier to land without re-litigating raw indexing semantics.
- `.planning/STATE.md` and `.planning/ROADMAP.md` were intentionally left uncommitted in this step; the orchestrator will own shared artifact progress updates after later phase gates.

## Self-Check

PASSED

- `FOUND: .planning/phases/09-derived-representations/09-01-SUMMARY.md`
- `FOUND: ddaeab6`
- `FOUND: d670028`
- `FOUND: go test ./... -count=1`

---
*Phase: 09-derived-representations*
*Completed: 2026-04-16*
