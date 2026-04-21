# Phase 06: Query Path Hot Path - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-14
**Phase:** 06-query-path-hot-path
**Areas discussed:** Canonical public lookup, Stored path representation, Benchmark shape, Benchmark data source, Query fallback semantics

---

## Canonical public lookup

| Option | Description | Selected |
|--------|-------------|----------|
| Transparent equivalence | Treat supported alternate spellings such as `$.foo`, `$['foo']`, and `$["foo"]` as the same lookup path automatically. | ✓ |
| Canonical form required | Only the canonical spelling is guaranteed to resolve; callers normalize first. | |
| Partial equivalence only | Support the obvious field-quote variants, but do not promise broader canonical behavior yet. | |

**User's choice:** Transparent equivalence
**Notes:** The public query contract should make supported equivalent spellings interchangeable rather than pushing normalization onto callers.

---

## Stored path representation

| Option | Description | Selected |
|--------|-------------|----------|
| Canonical everywhere | Store and surface one canonical spelling in `PathDirectory`, serialization, CLI/info output, and tests. | ✓ |
| Canonical lookup only | Normalize queries internally, but preserve build-time spelling for storage and display. | |
| Canonical plus alias metadata | Use one canonical stored form but keep original or alias metadata for debugging/display. | |

**User's choice:** Canonical everywhere
**Notes:** Phase 06 should converge on a single public path representation, not just an internal lookup alias layer.

---

## Benchmark shape

| Option | Description | Selected |
|--------|-------------|----------|
| Wide path count only | Stress lookup with many unique indexed paths and measure EQ, CONTAINS, and REGEX against that. | |
| Mixed spelling coverage | Focus on canonical-vs-bracket spelling lookups and ensure equivalent spellings do not regress performance. | |
| Realistic blend | Primary stress is wide path counts, but also include equivalent-spelling cases and cover EQ, CONTAINS, and REGEX on the same fixture family. | ✓ |

**User's choice:** Realistic blend
**Notes:** The benchmark story should prove both hot-path improvement and spelling consistency without collapsing into a single synthetic micro-case.

---

## Benchmark data source

| Option | Description | Selected |
|--------|-------------|----------|
| Public corpus + synthetic expansion | Start from a recognizable public/log-style dataset, then expand or reshape it to create the high path-count scenarios the phase needs. | ✓ |
| Public corpus only | Stay fully tied to a real public dataset, even if it limits edge-case stress. | |
| Synthetic only | Use generated fixtures only and explain realism in docs rather than through the source data. | |

**User's choice:** Public corpus + synthetic expansion
**Notes:** The user explicitly asked for a recognizable public dataset, mentioning Apache logs as the kind of corpus that would make benchmark claims more credible.

---

## Query fallback semantics

| Option | Description | Selected |
|--------|-------------|----------|
| Keep current safe fallback | Normalize supported spellings, but keep unknown/unresolved paths as non-errors that do not prune. | ✓ |
| Stricter for invalid supported paths only | Keep unknown-path fallback, but tighten behavior around non-canonical or unsupported valid-looking paths. | |
| Stricter query contract | Move toward explicit errors or warnings for unresolved paths. | |

**User's choice:** Keep current safe fallback
**Notes:** Phase 06 should stay focused on lookup performance and canonicalization rather than expanding into a stricter query API contract.

---

## the agent's Discretion

- Exact lookup structure and caching strategy.
- Exact canonicalization implementation point between build, query, and serialization flows.
- Exact public corpus choice and synthetic widening mechanics for benchmarks.
- Exact benchmark naming and helper organization.

## Deferred Ideas

None.
