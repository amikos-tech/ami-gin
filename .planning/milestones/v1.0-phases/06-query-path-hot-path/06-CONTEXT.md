# Phase 06: Query Path Hot Path - Context

**Gathered:** 2026-04-14
**Status:** Ready for planning

<domain>
## Phase Boundary

Make query-time path resolution fast and deterministic by replacing per-predicate linear path scans with indexed lookup and by treating supported equivalent JSONPath spellings as the same canonical path. This phase covers the public path contract, stored path representation, and benchmark proof for EQ, CONTAINS, and REGEX lookups. It does not add new query operators, change the predicate model, or tighten unresolved-path behavior beyond canonicalizing supported spellings.

</domain>

<decisions>
## Implementation Decisions

### Canonical path contract
- **D-01:** Supported equivalent JSONPath spellings must resolve transparently at query time. For supported inputs, callers should be able to use forms such as `$.foo`, `$['foo']`, and `$["foo"]` interchangeably.
- **D-02:** The index should store and surface one canonical path spelling everywhere. `PathDirectory`, serialized path names, CLI/info output, and tests should converge on the canonical form instead of preserving mixed source spellings.

### Benchmark strategy
- **D-03:** PATH-03 should use a realistic blended benchmark family: wide path-count stress is the primary lookup pressure, but the same fixture family should also cover equivalent-spelling lookups and the EQ, CONTAINS, and REGEX operators.
- **D-04:** Benchmark fixtures should prefer a recognizable public log-style corpus as the base shape when practical, then widen or reshape it synthetically as needed to create the high path-count scenarios Phase 06 needs.

### Query fallback semantics
- **D-05:** After applying canonicalization for supported spellings, unresolved or unknown paths should keep the current safe fallback behavior for this phase: no explicit query-time error, and no pruning beyond what the index can prove safely.

### the agent's Discretion
- Exact lookup data structure and caching strategy, as long as path resolution becomes constant or logarithmic time and preserves the canonical path contract.
- Exact canonicalization implementation point(s) between builder, query lookup, and serialization, as long as the public representation is stable and consistent.
- Exact public benchmark corpus choice, fixture-preparation flow, and synthetic widening mechanics, as long as the benchmark story remains credible and scoped to the hot path.
- Exact benchmark names, helper layout, and regression thresholds.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and acceptance
- `.planning/ROADMAP.md` — Phase 06 goal, success criteria, and milestone ordering constraints.
- `.planning/REQUIREMENTS.md` — `PATH-01`, `PATH-02`, and `PATH-03`, which define the required lookup and benchmark outcomes.
- `.planning/PROJECT.md` — milestone-level constraints: preserve pruning-first semantics, protect correctness, back claims with benchmarks, and avoid gratuitous API churn.
- `.planning/STATE.md` — current milestone state and the carry-forward decision to prioritize low-risk/high-signal wins first.

### Existing architecture and test conventions
- `.planning/codebase/ARCHITECTURE.md` — current query/build/serialization layering, path lookup flow, and unknown-path fallback behavior.
- `.planning/codebase/CONVENTIONS.md` — package, error-handling, and hot-path coding conventions that Phase 06 should stay consistent with.
- `.planning/codebase/TESTING.md` — benchmark/test organization and helper patterns used by the existing suite.
- `README.md` — current public JSONPath support and benchmark positioning that should remain consistent with the canonical path contract.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `jsonpath.go`: `ValidateJSONPath()`, `ParseJSONPath()`, and `NormalizePath()` already exist, so Phase 06 can build on an established canonicalization primitive rather than inventing a new path grammar.
- `benchmark_test.go`: operator-focused query benchmarks already exist and can be extended instead of creating a separate benchmark harness.
- `gin_test.go` and `integration_property_test.go`: existing JSONPath validation, normalization, wildcard, and serialization-adjacent tests give a natural place to harden the canonical path contract.

### Established Patterns
- `query.go` currently centralizes path resolution in `findPath()`, which makes it the main hot-path integration point for replacing linear scans.
- `builder.go` currently materializes one concrete path name into `PathDirectory`, and `Finalize()` sorts paths before assigning IDs. Any canonical public representation needs to account for that build-time path pipeline.
- The query layer currently treats unknown paths conservatively by returning `AllRGs()` rather than failing, and this behavior is intentionally preserved for Phase 06.
- The repo is a single-package Go library with performance work concentrated in `benchmark_test.go`, so the phase should fit existing benchmark and test patterns rather than introducing a new subsystem.

### Integration Points
- `query.go`: replace or bypass `findPath()` linear scans in predicate evaluation.
- `builder.go`: canonicalize or normalize stored path names before `PathDirectory` and dependent indexes are finalized.
- `serialize.go`: ensure canonical path names round-trip cleanly through encode/decode once the stored representation changes.
- `benchmark_test.go`, `gin_test.go`, and `integration_property_test.go`: add lookup regression tests and wide-path EQ/CONTAINS/REGEX benchmarks tied to the new contract.

</code_context>

<specifics>
## Specific Ideas

- Use a recognizable public log-style dataset if practical so benchmark claims read as credible to readers outside the project.
- Synthetic widening of that public corpus is acceptable when needed to create the high path-count lookup pressure that Phase 06 specifically targets.
- A dataset choice similar in spirit to Apache-style logs is acceptable, but fixture realism should not turn this phase into a data-ingestion project of its own.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 06-query-path-hot-path*
*Context gathered: 2026-04-14*
