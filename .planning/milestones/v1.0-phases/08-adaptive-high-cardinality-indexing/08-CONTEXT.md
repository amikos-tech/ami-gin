# Phase 08: Adaptive High-Cardinality Indexing - Context

**Gathered:** 2026-04-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Recover exact pruning power for hot values on high-cardinality string paths without abandoning compact fallback behavior for the long tail. This phase covers adaptive promotion policy, conservative fallback behavior for non-promoted values, additive build-time configuration, metadata/CLI visibility for exact vs bloom-only vs adaptive paths, and benchmark proof that pruning improves with bounded size growth. It does not redesign derived indexing ahead of Phase 09, compact the broader binary layout ahead of Phase 10, or broaden adaptive query semantics beyond the explicitly locked string-membership scope below.

</domain>

<decisions>
## Implementation Decisions

### Hot-term promotion policy
- **D-01:** High-cardinality string paths should use a hybrid promotion rule, not naive top-K alone. Promotion is driven by row-group frequency, rejects overly broad terms with a coverage ceiling, and caps the number of promoted exact terms per path.
- **D-02:** Promotion ranking should use per-term row-group coverage already available from `RGSet` cardinality at finalize time, not raw document occurrence count.

### Long-tail fallback behavior
- **D-03:** Non-promoted values on adaptive paths should use fixed hash-bucket row-group bitmaps as the primary conservative fallback instead of falling back to `AllRGs()` after a global bloom hit.
- **D-04:** Existing string-length stats may remain an additional cheap filter, but they are not the primary adaptive fallback mechanism.

### Adaptive operator scope
- **D-05:** Phase 08 is scoped to positive string membership pruning on adaptive paths. `EQ` and `IN` should use promoted exact bitmaps plus bucket fallback for non-promoted values.
- **D-06:** `NE` and `NIN` should stay conservative on adaptive paths unless the queried value is one of the exact promoted terms. Phase 08 should not broaden lossy negative-predicate pruning semantics.

### Config surface
- **D-07:** Adaptive behavior should remain an additive, small global configuration surface in Phase 08. Keep `CardinalityThreshold` as the path-level trigger for switching from full exact indexing to adaptive behavior.
- **D-08:** Add only the global knobs needed for the chosen hybrid policy and bounded promotion behavior. Do not introduce a broader adaptive-policy DSL or path-level override system in this phase.

### Metadata and CLI visibility
- **D-09:** Path metadata and `gin-index info` must distinguish exact, bloom-only, and adaptive-hybrid paths explicitly.
- **D-10:** Adaptive paths should expose moderate summary counters rather than raw diagnostic dumps: enough to show hybrid mode, promoted hot-term count, configured threshold/cap, and useful hybrid-coverage summary, but not a heavy diagnostic surface or raw promoted values.

### the agent's Discretion
- Exact default values and naming for the new global adaptive knobs, as long as they implement the locked hybrid promotion policy and remain additive to the existing `GINConfig` shape.
- Exact hash/bucket layout, hashing choice, and bucket count strategy for long-tail fallback, as long as fallback stays conservative, bounded, and versioned explicitly.
- Exact metadata field layout and CLI presentation wording, as long as adaptive paths are clearly distinguishable and expose the locked moderate summary counters.
- Exact benchmark fixture mix and reporting format, as long as HCARD-05 demonstrates pruning improvement on realistic high-cardinality datasets with bounded size growth.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and acceptance
- `.planning/ROADMAP.md` — Phase 08 goal, success criteria, and milestone sequencing constraints.
- `.planning/REQUIREMENTS.md` — `HCARD-01` through `HCARD-05`, which define adaptive promotion, configurability, conservative fallback, metadata visibility, and benchmark proof.
- `.planning/PROJECT.md` — milestone-level constraints: preserve pruning-first scope, avoid false negatives, keep API churn additive, and back claims with benchmarks.
- `.planning/STATE.md` — current milestone state and carry-forward concern context from completed phases.

### Prior-phase constraints that still apply
- `.planning/phases/06-query-path-hot-path/06-CONTEXT.md` — canonical-path behavior, safe unresolved-path fallback, and benchmark-credibility constraints that Phase 08 must preserve.
- `.planning/phases/07-builder-parsing-numeric-fidelity/07-CONTEXT.md` — additive-config and compatibility expectations coming out of the builder/parser redesign.
- `.planning/phases/07-builder-parsing-numeric-fidelity/07-builder-parsing-numeric-fidelity-02-SUMMARY.md` — current benchmark-labeling and fixture-pattern conventions that should stay aligned when adding Phase 08 benchmarks.

### Existing implementation surfaces
- `builder.go` — current path-build data, string-term RG bitmap accumulation, HLL cardinality tracking, string-length stats, and finalize-time exact-vs-bloom-only path selection.
- `gin.go` — `PathEntry`, `StringIndex`, `GINConfig`, path flags, and immutable index structure that Phase 08 will extend.
- `query.go` — current `EQ`/`IN`/`NE` behavior for strings and the existing bloom-only conservative fallback path.
- `serialize.go` — index/config encode/decode layout and explicit format-version behavior that adaptive metadata must extend safely.
- `cmd/gin-index/main.go` — current `info` output surface that must distinguish exact, bloom-only, and adaptive paths.
- `benchmark_test.go` — existing high-cardinality fixture helpers and benchmark structure to extend for HCARD-05.
- `integration_property_test.go` — current cardinality-threshold property tests that define today’s exact-vs-bloom-only behavior and should be updated for adaptive semantics.
- `README.md` — current public description of high-cardinality behavior and config shape that Phase 08 must keep consistent.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `builder.go`: `pathBuildData.stringTerms` already captures `term -> RGSet`, and `Finalize()` already computes per-path HLL cardinality, so adaptive promotion can be decided without a second ingest structure.
- `builder.go`: `StringLengthIndex` support already exists and can remain a secondary cheap filter for adaptive long-tail lookups.
- `benchmark_test.go`: `generateHighCardinalityDocs()` and the existing scale benchmarks provide a direct starting point for HCARD-05 fixture families.

### Established Patterns
- High-cardinality behavior is currently all-or-nothing: if estimated path cardinality exceeds `CardinalityThreshold`, `Finalize()` sets `FlagBloomOnly` and omits the exact `StringIndex`.
- Query evaluation is optimized around `path -> index -> RGSet` lookups, so adaptive fallback should preferably still materialize candidate row-group sets rather than require heavyweight per-query scans.
- Config serialization currently persists only a small flat subset of `GINConfig`, so Phase 08 should stay additive and explicit instead of introducing a broad new policy layer.
- The project already treats correctness conservatively when it cannot prove pruning safely; adaptive fallback must preserve that same no-false-negative rule.

### Integration Points
- `builder.go`: replace the current path-wide exact-vs-bloom-only finalize branch with exact, bloom-only, or adaptive-hybrid path assembly and promoted-term selection.
- `query.go`: update positive string membership evaluation to use promoted exact terms plus bucket fallback on adaptive paths, while keeping negative predicates conservative.
- `gin.go` and `serialize.go`: extend path flags/config/serialized sections so adaptive metadata survives encode/decode cleanly and explicitly.
- `cmd/gin-index/main.go`: surface adaptive mode and compact hybrid summary counters in `gin-index info`.
- `benchmark_test.go`, `gin_test.go`, and `integration_property_test.go`: add regressions and fixtures for promoted-term behavior, conservative fallback semantics, metadata visibility, and pruning/size measurements.

</code_context>

<specifics>
## Specific Ideas

- The hybrid promotion rule should rank by row-group frequency, not raw document occurrence count, because row-group selectivity is what matters for pruning quality.
- Bucket fallback should preserve the current `RGSet`-shaped query path rather than introducing a separate per-query probing model.
- Adaptive metadata should be operationally useful but compact: enough to show hybrid coverage and caps, not a verbose dump of raw hot values or large diagnostics.

</specifics>

<deferred>
## Deferred Ideas

- Path-level adaptive overrides are intentionally deferred unless Phase 08 benchmarks prove that a few specific paths need exceptions beyond global defaults.
- A richer adaptive-policy DSL is deferred; the chosen Phase 08 surface should stay small and additive.
- A full diagnostic/forensic metadata surface is deferred; Phase 08 only needs moderate counters, not heavy tuning output.

</deferred>

---

*Phase: 08-adaptive-high-cardinality-indexing*
*Context gathered: 2026-04-15*
