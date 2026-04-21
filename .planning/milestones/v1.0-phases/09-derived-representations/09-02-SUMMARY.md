---
phase: 09-derived-representations
plan: 02
subsystem: querying
tags: [derived-representations, alias-routing, serialization, cli, testing]
requires:
  - phase: 09-derived-representations
    provides: hidden companion paths and additive raw-plus-companion staging from 09-01
provides:
  - explicit alias-aware query routing via `gin.As(alias, value)` while raw-path queries remain unchanged
  - read-only representation metadata and deterministic alias lookup on fresh and decoded indexes
  - explicit v7 representation metadata serialization with hard failures for non-serializable companions
affects: [09-03, 10-serialization-compaction]
tech-stack:
  added: []
  patterns: [explicit alias wrapper, decoded metadata rebuild, hidden-path suppression in public diagnostics]
key-files:
  created: [.planning/phases/09-derived-representations/09-02-SUMMARY.md]
  modified: [builder.go, benchmark_test.go, gin.go, query.go, serialize.go, serialize_security_test.go, gin_test.go, transformers_test.go, cmd/gin-index/main.go, cmd/gin-index/main_test.go]
key-decisions:
  - "Require explicit `gin.As(alias, value)` routing for companion queries so raw-path semantics stay the default public contract."
  - "Persist representation metadata as a dedicated v7 section instead of inferring alias bindings from hidden target-path naming."
  - "Reject non-serializable custom companions during `Encode()` rather than pretending decode can reconstruct them."
patterns-established:
  - "Fresh `Finalize()` and `Decode()` both rebuild the same alias lookup before any query evaluation."
  - "CLI/info surfaces raw paths plus alias metadata while suppressing internal `__derived:` entries."
requirements-completed: [DERIVE-02, DERIVE-03]
duration: 79min
completed: 2026-04-16
---

# Phase 09 Plan 02: Alias Routing and Representation Metadata Summary

**Explicit alias routing, deterministic representation introspection, and v7 representation metadata round-trip parity**

## Performance

- **Duration:** 79 min
- **Started:** 2026-04-16T19:24:00Z
- **Completed:** 2026-04-16T20:43:00Z
- **Tasks:** 2
- **Files modified:** 10

## Accomplishments
- Added public alias-routing primitives with `RepresentationValue` / `As(...)`, read-only `Representations(...)` introspection, and deterministic alias lookup rebuilds on both freshly finalized and decoded indexes.
- Routed predicate evaluation through explicit raw-vs-alias path resolution without exposing hidden `__derived:` target paths as a public query contract.
- Added explicit v7 representation metadata encoding/decoding, hard failures for non-serializable companions, malformed-section validation, and CLI suppression of hidden internal paths.

## Task Commits

1. **Task 1 + Task 2: Add alias routing, introspection, and explicit representation metadata round-trip support** - `8d1661e` (feat)

## Files Created/Modified
- `builder.go` - rebuilds representation lookup on fresh `Finalize()` output before any alias query runs
- `benchmark_test.go` - keeps legacy numeric benchmark comparison fixtures isolated from representation-aware config state
- `gin.go` - defines alias wrapper/introspection types, representation metadata storage, and lookup rebuild helpers
- `query.go` - resolves raw vs alias-aware predicates through explicit path/value unwrapping
- `serialize.go` - bumps format to v7 and adds dedicated representation metadata trailer read/write helpers
- `serialize_security_test.go` - locks round-trip parity, trailer ordering, and malformed metadata rejection
- `gin_test.go` - covers alias routing, raw-path defaults, introspection ordering, and fresh-finalize lookup rebuilds
- `transformers_test.go` - adds decode-parity coverage using supported exact-int companion values
- `cmd/gin-index/main.go` - hides internal `__derived:` paths from public diagnostics while rendering alias metadata on raw paths
- `cmd/gin-index/main_test.go` - anchors the CLI hidden-path suppression contract

## Decisions Made

- Kept unknown alias lookups conservative by returning the same result shape as other unknown-path queries instead of panicking.
- Stored `SourcePath`, `Alias`, `TargetPath`, `Transformer`, and `Serializable` explicitly in the representation metadata trailer to avoid name-based inference.
- Adjusted numeric decode-parity coverage to stay within the library's supported exact-integer range instead of broadening runtime numeric guarantees.

## Verification Evidence

- Alias routing and introspection tests passed:
  `go test ./... -run 'Test(QueryAliasRoutingUsesAsWrapper|RawPathQueriesRemainDefaultWithDerivedAliases|RepresentationsIntrospectionListsAliases|FinalizePopulatesRepresentationLookup|CLIInfoSuppressesInternalRepresentationPaths)' -count=1`
- Representation metadata round-trip and guardrail tests passed:
  `go test ./... -run 'Test(RepresentationMetadataRoundTrip|DecodeRepresentationAliasParity|RepresentationMetadataSectionFollowsConfig|EncodeRejectsNonSerializableRepresentation|DecodeRejectsDuplicateRepresentationAlias|DecodeRejectsRepresentationTargetPathOutOfRange|DecodeRejectsOversizedRepresentationSection)' -count=1`
- Focused numeric parity regression passed after fixture correction:
  `go test ./... -run TestTransformerNumericDecodeParity -count=1`
- Repo-wide suite passed:
  `go test ./... -count=1`

## Deviations from Plan

None - the implementation stayed within the Plan 02 scope.

## Issues Encountered

- The initial numeric parity fixture used a value larger than the supported exact-float integer bound (`1<<53`), so the test was corrected to validate decode parity within documented numeric fidelity limits rather than implying a broader guarantee.

## User Setup Required

None - no external setup or migration step required.

## Next Phase Readiness

- Phase 09-03 can now document and demonstrate the public alias-aware contract without any hidden-path leakage or decode caveat gaps.
- The read-only `Representations(...)` surface gives the examples and README a stable public explanation point for alias discovery.
- `.planning/STATE.md` and `.planning/ROADMAP.md` remain intentionally uncommitted shared artifacts while execution continues.

## Self-Check

PASSED

- `FOUND: .planning/phases/09-derived-representations/09-02-SUMMARY.md`
- `FOUND: 8d1661e`
- `FOUND: go test ./... -count=1`

---
*Phase: 09-derived-representations*
*Completed: 2026-04-16*
