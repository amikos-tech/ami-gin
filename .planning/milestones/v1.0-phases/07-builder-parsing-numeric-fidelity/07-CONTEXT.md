# Phase 07: Builder Parsing & Numeric Fidelity - Context

**Gathered:** 2026-04-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Lower build-time ingest cost and make number handling explicit and safe during index construction. This phase covers the builder-side parsing path, numeric classification, failure behavior for unsupported numbers, and benchmark proof for ingest/build costs. It does not add new query operators, widen the public numeric/query surface beyond exact `int64` integers plus the existing decimal behavior, or redesign derived representations ahead of Phase 09.

</domain>

<decisions>
## Implementation Decisions

### Numeric support boundary
- **D-01:** Phase 07 keeps the current decimal/query surface while adding exact `int64` fidelity for integers. This phase does not broaden into big-int or exact-decimal semantics.

### Unsupported-number failure policy
- **D-02:** If a numeric value cannot be represented safely under the Phase 07 rules, `AddDocument()` should fail that document immediately with explicit error context rather than partially indexing the document or silently degrading pruning.

### Mixed numeric semantics per path
- **D-03:** Paths that contain both integers and decimals remain one shared numeric domain. Integers must be parsed exactly before any widening or stats decisions are made, but mixed paths are still queryable as numeric fields.

### Transformer compatibility
- **D-04:** The parser redesign should preserve the current transformer/config contract. Existing field transformers should continue to work without user-facing config or query changes in this phase.

### the agent's Discretion
- Exact parser implementation strategy replacing `json.Unmarshal(..., &any)`, as long as the ingest path stops depending on generic `float64` JSON decoding.
- Exact internal numeric representation and widening rules between parse-time exact integers and the existing float-backed numeric indexes, as long as the locked support boundary and failure semantics are preserved.
- Exact benchmark shape, fixture mix, and reporting style for ingest/build latency and allocation deltas.
- Exact error message wording and attached numeric/path context, as long as failures are explicit and actionable.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and acceptance
- `.planning/ROADMAP.md` — Phase 07 goal, success criteria, and milestone ordering constraints.
- `.planning/REQUIREMENTS.md` — `BUILD-01` through `BUILD-05`, which define the parser, numeric-fidelity, failure, and benchmark outcomes.
- `.planning/PROJECT.md` — milestone-level constraints: preserve pruning-first scope, protect correctness, back claims with benchmarks, and avoid gratuitous API churn.
- `.planning/STATE.md` — current milestone state and the explicit concern that Phase 07 needs a design for integer fidelity against the current float-centric structures.

### Prior phase and architecture context
- `.planning/phases/06-query-path-hot-path/06-CONTEXT.md` — carry-forward constraints around canonical path behavior, preserved safe fallback semantics, and benchmark credibility.
- `.planning/codebase/ARCHITECTURE.md` — current build/query/serialization layering and where parsing changes connect to the rest of the index.
- `.planning/codebase/CONVENTIONS.md` — package, error-handling, and hot-path coding conventions to preserve while changing parsing internals.
- `.planning/codebase/STRUCTURE.md` — locations of builder, query, transformer, and benchmark code that Phase 07 will touch.
- `.planning/codebase/STACK.md` — runtime/toolchain context and current dependency surface for parser/benchmark work.
- `.planning/codebase/TESTING.md` — benchmark/test organization and existing performance/property-testing patterns to extend for `BUILD-05`.

### Existing implementation surfaces
- `builder.go` — current `AddDocument()`, `walkJSON()`, transformer application point, numeric type switch, and finalize pipeline.
- `gin.go` — `NumericIndex`, `RGNumericStat`, and config structures that constrain Phase 07’s internal numeric model.
- `query.go` — current numeric query behavior and float-backed evaluation path that Phase 07 must remain compatible with.
- `benchmark_test.go` — existing builder/query benchmark harness to extend for ingest/build latency and allocation comparisons.
- `transformers.go` — built-in transformer behavior that Phase 07 must preserve.
- `transformers_test.go` — existing transformer compatibility and numeric-query expectations that should remain valid after the parser redesign.
- `transformer_registry.go` — serialized transformer configuration contract that should remain stable in this phase.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `builder.go`: `AddDocument()`, `walkJSON()`, `normalizeWalkPath()`, and `Finalize()` already centralize the ingest pipeline, so the parser redesign can stay localized instead of spreading through the package.
- `benchmark_test.go`: existing `BenchmarkAddDocument`, `BenchmarkAddDocumentBatch`, `BenchmarkFinalize`, and `BenchmarkBuilderMemory` provide a direct place to capture before/after latency and allocation deltas for `BUILD-05`.
- `transformers.go`, `transformers_test.go`, and `transformer_registry.go`: current transformer implementations and round-trip coverage provide a compatibility harness for the “preserve transformer contract” decision.

### Established Patterns
- The library is a single-package root Go module, so Phase 07 should extend the existing builder/query/benchmark files instead of introducing a new subsystem.
- The current builder path applies field transformers before type classification in `walkJSON()`, so parser changes must preserve that observable behavior unless a later phase deliberately revisits it.
- Numeric indexing is currently float-backed through `NumericIndex` and `RGNumericStat`, which means exact integer parsing likely needs an internal bridge to today’s query/index structures rather than a public numeric model rewrite.
- Error handling across the repo uses `github.com/pkg/errors`, and hot-path code prefers explicit guard behavior over silent semantic changes.

### Integration Points
- `builder.go`: replace the full-document generic decode path and make numeric parsing explicit before type-switch classification and stats updates.
- `gin.go`: adjust or extend numeric build-time metadata only as needed to preserve exact integer fidelity within the locked support boundary.
- `query.go`: keep numeric query behavior compatible with the existing decimal surface after builder-side parsing changes land.
- `benchmark_test.go`: add parser-focused ingest/build benchmarks and allocation reporting tied directly to Phase 07 success criteria.
- `transformers_test.go` and related tests: prove that transformer behavior and numeric-query expectations still hold after the parser redesign.

</code_context>

<specifics>
## Specific Ideas

- Keep the parser redesign internal to Phase 07 rather than turning it into a public numeric API expansion.
- Preserve current decimal behavior while making integer parsing and unsafe-number failures explicit.
- No specific requirements beyond the locked decisions above — standard implementation choices are acceptable where not constrained.

</specifics>

<deferred>
## Deferred Ideas

- Evaluate `github.com/lemire/constmap` or a similar immutable lookup structure for `pathLookup` as a separate query-path optimization experiment. This is structurally relevant to immutable path lookup, but it is out of scope for Phase 07 because this phase is builder parsing and numeric fidelity, not query lookup performance.

</deferred>

---

*Phase: 07-builder-parsing-numeric-fidelity*
*Context gathered: 2026-04-15*
