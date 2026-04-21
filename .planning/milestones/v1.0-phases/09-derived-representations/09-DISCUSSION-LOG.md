# Phase 09: Derived Representations - Discussion Log

**Date:** 2026-04-16
**Status:** Completed

## Summary

Discussion focused on four gray areas:
- public API shape for additive transformer registration
- query targeting for companion representations
- failure behavior when derivation cannot produce a companion value
- explicit metadata required for encode/decode round-tripping

The user preferred high-DX `WithXTransformer(...)` helper names, additive raw-plus-companion semantics, and explicit query alias selection without exposing internal reserved paths publicly.

## Decision Log

### 1. Registration API

**Question:** How should Phase 09 expose additive raw-plus-derived indexing in the public config surface?

**Outcome:**
- Keep `Transformer` as the public API term.
- Replace the current replacement-only behavior with additive sibling representations.
- Use explicit helper-style registration with alias arguments.
- Support multiple transformers on one source path with no implicit ordering.

**Locked example:**

```go
config, _ := gin.NewConfig(
    gin.WithISODateTransformer("$.created_at", "epoch_ms"),
    gin.WithToLowerTransformer("$.email", "lower"),
    gin.WithEmailDomainTransformer("$.email", "domain"),
)
```

**Rejected directions:**
- generic `WithDerived(...)` as the primary public surface
- keeping replacement-only transformer semantics
- hidden ordering or chain semantics implied by registration order

### 2. Query targeting and alias discovery

**Question:** How should callers query companion representations without exposing internal storage paths?

**Outcome:**
- Raw-path queries stay the default.
- Companion representations are selected explicitly with a typed wrapper such as `gin.As(alias, value)`.
- Internal storage may use a reserved namespace such as `__derived`, but that is not the public query contract.
- Add a minimal read-only introspection API on `GINIndex` to discover aliases and transformer kinds for diagnostics and dynamic query building.

**Why this was chosen:**
- The user did not want callers to remember a public `__derived` path.
- Implicit polymorphism on `any` would be ambiguous because raw and companion representations can share the same Go type.

**Rejected directions:**
- forcing callers to query public `__derived` paths directly
- relying purely on value type guessing
- exposing a mutable registry-style metadata API

### 3. Failure behavior

**Question:** What should happen when a registered transformer cannot produce a companion value for a document?

**Outcome:**
- Strict by default: `AddDocument()` fails for the document.
- Soft-fail behavior, if added later, must be explicit rather than silent omission.

**Research used during discussion:**
- Elasticsearch/OpenSearch multi-fields informed the additive raw-plus-companion model.
- Elasticsearch/OpenSearch malformed-field and ingest failure defaults informed the strict-by-default failure posture.

### 4. Serialized relationship metadata

**Question:** How should raw-to-alias relationships survive encode/decode?

**Outcome:**
- Persist explicit representation metadata for every alias registration.
- Do not reconstruct relationships from naming conventions.

**Required metadata direction:**
- source path
- alias
- transformer kind/ID
- transformer params
- any internal target information needed for deterministic round-tripping

## User Preferences Captured

- Public API should optimize for intentional helper names and autocomplete discoverability.
- `Transformer` is preferred public terminology over `Derived`.
- Public query DX should not expose internal reserved paths.
- Hidden ordering rules in a flat config API are undesirable.
- Strict defaults are acceptable when they preserve clarity and data quality.

## Out of Scope Items Explicitly Avoided

- transform chaining / ordered transform pipelines
- query-time transformers
- convention-based metadata reconstruction
- public `__derived` query strings as the primary API

---

*Phase: 09-derived-representations*
*Discussion completed: 2026-04-16*
