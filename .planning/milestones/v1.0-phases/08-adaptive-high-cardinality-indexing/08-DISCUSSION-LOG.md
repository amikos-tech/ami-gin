# Phase 08: Adaptive High-Cardinality Indexing - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in `08-CONTEXT.md` — this log preserves the alternatives considered.

**Date:** 2026-04-15
**Phase:** 08-adaptive-high-cardinality-indexing
**Areas discussed:** Promotion policy, Long-tail fallback behavior, Adaptive negative predicates, Config surface, Metadata and CLI visibility

---

## Promotion policy

| Option | Description | Selected |
|--------|-------------|----------|
| Top-K by row-group frequency | Promote the top K terms with the highest row-group frequency and cap exact storage that way | |
| Row-group coverage band | Promote terms based on a minimum/maximum row-group coverage window | |
| Hybrid: min RG frequency + max coverage ceiling + top-K cap | Promote only terms that are hot enough, reject overly broad terms, and cap the promoted set per path | ✓ |
| Byte-budget / marginal-gain promotion | Promote terms by estimated pruning gain per byte of stored bitmap cost | |

**User's choice:** Hybrid: min RG frequency + max coverage ceiling + top-K cap
**Notes:** Promotion should be frequency-driven and bounded, not naive top-K alone. Row-group coverage matters more than raw document count for pruning quality.

---

## Long-tail fallback behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Current `GlobalBloom -> AllRGs` fallback | Keep today’s bloom-only conservative fallback for non-promoted values | |
| `GlobalBloom -> per-RG string-length stats` | Use existing string-length stats as the main long-tail narrowing mechanism | |
| `GlobalBloom -> per-path Bloom narrowing` | Add a path-local Bloom-based narrowing scheme for non-promoted values | |
| `GlobalBloom -> fixed hash-bucket RG bitmaps` | Use bounded hash buckets that map long-tail values to conservative RG bitmaps | ✓ |

**User's choice:** `GlobalBloom -> fixed hash-bucket RG bitmaps`
**Notes:** Long-tail fallback should keep producing `RGSet` candidates in the same general shape as today’s exact indexes, rather than collapsing to `AllRGs()` after a bloom hit.

---

## Adaptive negative predicates

| Option | Description | Selected |
|--------|-------------|----------|
| Keep Phase 08 scoped to positive membership pruning | `EQ` / `IN` get adaptive behavior; `NE` / `NIN` stay conservative on adaptive paths unless exact promotion applies | ✓ |
| Add dedicated conservative `NE` / `NIN` handling | Extend adaptive-path semantics to negative predicates in Phase 08 | |
| Let the agent decide | Leave this boundary to implementation-time judgment | |

**User's choice:** Keep Phase 08 scoped to positive membership pruning
**Notes:** This keeps correctness risk lower and avoids broadening adaptive query semantics beyond what the phase needs.

---

## Config surface

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal global knobs | Keep `CardinalityThreshold` as the trigger and add only a small number of global adaptive knobs | ✓ |
| Richer global policy struct | Introduce a broader named adaptive policy object with several tunable heuristics | |
| Sparse path-level overrides | Add path-specific adaptive-policy overrides in Phase 08 | |

**User's choice:** Minimal global knobs
**Notes:** Adaptive indexing should stay additive to the existing flat `GINConfig` surface rather than introducing a larger policy DSL or path-level override system in this phase.

---

## Metadata and CLI visibility

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal label-only | Expose only exact vs bloom-only vs adaptive mode labels | |
| Moderate hybrid stats/counters | Expose mode plus compact hybrid summary counters such as promoted-term count and related adaptive stats | ✓ |
| Detailed diagnostic surface | Expose a heavier diagnostic/tuning surface with more internal detail | |

**User's choice:** Moderate hybrid stats/counters
**Notes:** Adaptive paths should be visible and explainable in `gin-index info`, but Phase 08 should not grow a heavyweight diagnostic artifact.

---

## the agent's Discretion

- Exact default values and naming for the new global adaptive knobs
- Exact bucket-count/hash layout for long-tail fallback
- Exact metadata field layout and CLI formatting for the moderate adaptive summary
- Exact benchmark fixture mix and reporting shape for HCARD-05

## Deferred Ideas

- Path-level adaptive overrides as a later addition if benchmark data proves global defaults are insufficient
- A broader adaptive-policy DSL beyond the narrow Phase 08 knobs
- A full diagnostic metadata/CLI surface with raw promoted values or deep tuning output
