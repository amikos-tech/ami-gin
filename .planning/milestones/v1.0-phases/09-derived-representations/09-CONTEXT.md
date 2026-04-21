# Phase 09: Derived Representations - Context

**Gathered:** 2026-04-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Keep raw field values queryable while adding transformed companion representations that are first-class indexed targets. This phase covers the public transformer registration shape, explicit query routing to companion aliases, strict failure behavior for derivation, and explicit serialized metadata so raw-plus-derived relationships survive encode/decode. It does not add implicit transform chaining, query-time transformers, or convention-based alias inference.

</domain>

<decisions>
## Implementation Decisions

### Public transformer API
- **D-01:** Keep `Transformer` as the public API term. Do not expose `Derived` terminology in the user-facing configuration surface.
- **D-02:** Replace the current replacement-only transformer behavior with additive raw-plus-companion indexing.
- **D-03:** The public config surface uses explicit helper-style registration such as `gin.WithISODateTransformer("$.created_at", "epoch_ms")`.
- **D-04:** Multiple transformers may be registered for the same source path.
- **D-05:** Multiple transformers on one source path are sibling representations derived from the raw source value, not a chain.
- **D-06:** Phase 09 does not introduce implicit ordering or transform-on-transform behavior.

### Query routing and alias discovery
- **D-07:** Raw-path queries remain the default behavior.
- **D-08:** Companion representations are selected explicitly at query time via a typed wrapper such as `gin.As(alias, value)` rather than by exposing internal derived storage paths directly.
- **D-09:** Internal storage may use a reserved namespace such as `__derived`, but that namespace is not the public query contract.
- **D-10:** Expose a minimal read-only introspection surface on `GINIndex` so callers and diagnostics tooling can discover available aliases and transformer kinds per source path.

### Failure semantics
- **D-11:** Derivation is strict by default. If a registered transformer cannot produce its companion value for a document, `AddDocument()` fails for that document.
- **D-12:** If soft-fail behavior is ever added later, it must be an explicit opt-in policy rather than silent best-effort omission.

### Serialized metadata
- **D-13:** Encode/decode must persist explicit representation metadata for every alias registration rather than reconstructing source-to-alias relationships from naming conventions.
- **D-14:** Serialized representation metadata should include the source path, alias, transformer kind/ID, parameters, and any internal target information needed for deterministic round-tripping.
- **D-15:** Convention-based inference is intentionally out of scope because the public query surface is being decoupled from internal storage layout.

### the agent's Discretion
- Exact internal type names used for the new representation metadata structures, as long as the public API keeps `Transformer` terminology.
- Exact shape and naming of the typed query wrapper, as long as raw-path queries stay default and alias selection stays explicit.
- Exact read-only introspection method set on `GINIndex`, as long as aliases and transformer kinds are discoverable for diagnostics and dynamic query building.
- Exact internal reserved-path layout, as long as it is hidden from normal query DX and remains compatible with explicit serialized metadata.

</decisions>

<specifics>
## Specific Ideas

- The preferred registration style is:

```go
config, _ := gin.NewConfig(
    gin.WithISODateTransformer("$.created_at", "epoch_ms"),
    gin.WithToLowerTransformer("$.email", "lower"),
    gin.WithEmailDomainTransformer("$.email", "domain"),
)
```

- The public DX should not force callers to remember or type a reserved internal path such as `__derived`.
- Querying a companion representation should look like “same source path, explicit alias selection” rather than “different public path string”.
- The user prefers flat, intentional `WithXTransformer(...)` helpers over a generic `WithDerived(...)` or representation-spec-first public API.
- Research reference only: the strict-by-default failure decision aligns with the default malformed-field / ingest-failure posture used by Elasticsearch and OpenSearch, while the additive alias model aligns with their multi-field pattern.

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and acceptance
- `.planning/ROADMAP.md` — Phase 09 goal, success criteria, dependency on Phase 07, and milestone sequencing.
- `.planning/REQUIREMENTS.md` — `DERIVE-01` through `DERIVE-04`, which define additive companion indexing, explicit query targets, metadata round-tripping, and example/test coverage.
- `.planning/PROJECT.md` — milestone constraints and the locked project-level decision that derived indexing augments raw indexing.
- `.planning/STATE.md` — carry-forward milestone context and the explicit note that derived representations are additive and serialization compaction comes later.

### Prior phase constraints that still apply
- `.planning/phases/06-query-path-hot-path/06-CONTEXT.md` — canonical supported JSONPath behavior, safe unresolved-path fallback, and stable public path contract constraints.
- `.planning/phases/07-builder-parsing-numeric-fidelity/07-CONTEXT.md` — builder/parser compatibility expectations and the constraint to avoid redesigning derived representations before Phase 09.
- `.planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md` — additive configuration expectations, explicit metadata/versioning discipline, and the convention that correctness and format evolution stay conservative.

### Existing implementation surfaces
- `gin.go` — current `FieldTransformer`, `WithXTransformer` helpers, `GINConfig`, and index metadata structures that Phase 09 will replace or extend.
- `builder.go` — current replacement-only transformer application points in `decodeTransformedValue` and `stageMaterializedValue`.
- `query.go` — current path lookup and predicate evaluation flow where explicit alias routing will be introduced.
- `serialize.go` — current config serialization and decode reconstruction logic that must evolve from flat transformer specs to explicit representation metadata.
- `transformer_registry.go` — current transformer IDs/specs and reconstruction path that will underpin additive companion registration.
- `transformers.go` — built-in transformer behaviors that remain the semantic building blocks of the new additive API.
- `transformers_test.go` — compatibility, serialization, and behavior coverage that should expand for raw-plus-companion semantics.
- `jsonpath.go` — current supported JSONPath grammar and canonicalization rules that constrain public query naming.

### Public docs and examples
- `README.md` — current transformer documentation that will need to shift from replacement-only semantics to raw-plus-companion semantics.
- `examples/transformers/main.go` — current date-transform example that should evolve to demonstrate raw-plus-companion indexing.
- `examples/transformers-advanced/main.go` — current advanced transformer examples that should cover alias-based companion querying.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `transformers.go`: built-in date, text, regex, duration, and normalization transforms already exist and should remain the semantic basis of Phase 09.
- `transformer_registry.go`: current transformer IDs, parameter structs, and `ReconstructTransformer` logic provide a natural base for explicit representation metadata.
- `jsonpath.go`: canonical supported JSONPath normalization already exists and should continue to govern source-path handling.
- `transformers_test.go` plus `examples/transformers*.go`: existing transformer coverage and examples give a direct harness for proving additive raw-plus-companion behavior.

### Established Patterns
- Public configuration uses `NewConfig(...ConfigOption)` and intentional `WithX...` helper names rather than generic builder objects.
- Current transformer behavior is replacement-only and keyed by canonical source path; Phase 09 must intentionally break that behavior in favor of additive siblings.
- Query lookup currently assumes a path string resolves to one canonical index path; explicit alias routing must preserve deterministic lookup without exposing internal storage paths.
- Serialization already prefers explicit versioned metadata over heuristics; Phase 09 should extend that discipline rather than hiding meaning in path conventions.

### Integration Points
- `gin.go`: replace the single-transformer-per-path config maps with structures that support multiple alias registrations on one source path.
- `builder.go`: stop overwriting the raw indexed value and instead materialize raw plus zero-or-more companion representations from the same source input.
- `query.go`: add explicit alias-aware routing while preserving plain raw-path queries as the default.
- `serialize.go`: persist and reconstruct representation metadata explicitly for deterministic encode/decode round-trips.
- `README.md`, `examples/transformers/main.go`, and `examples/transformers-advanced/main.go`: update docs/examples to demonstrate additive semantics and explicit alias querying.

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 09-derived-representations*
*Context gathered: 2026-04-16*
