# Phase 09: Derived Representations - Research

**Researched:** 2026-04-16
**Domain:** Additive raw-plus-derived indexing, alias-aware query routing, and explicit representation metadata in Go
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md and discussion log)

Everything in this block is copied or restated from `.planning/phases/09-derived-representations/09-CONTEXT.md` and `.planning/phases/09-derived-representations/09-DISCUSSION-LOG.md`. [CITED: .planning/phases/09-derived-representations/09-CONTEXT.md, .planning/phases/09-derived-representations/09-DISCUSSION-LOG.md]

### Locked Decisions

#### Public transformer API
- **D-01:** Keep `Transformer` as the public API term. Do not expose `Derived` terminology in the user-facing configuration surface.
- **D-02:** Replace the current replacement-only transformer behavior with additive raw-plus-companion indexing.
- **D-03:** The public config surface uses explicit helper-style registration such as `gin.WithISODateTransformer("$.created_at", "epoch_ms")`.
- **D-04:** Multiple transformers may be registered for the same source path.
- **D-05:** Multiple transformers on one source path are sibling representations derived from the raw source value, not a chain.
- **D-06:** Phase 09 does not introduce implicit ordering or transform-on-transform behavior.

#### Query routing and alias discovery
- **D-07:** Raw-path queries remain the default behavior.
- **D-08:** Companion representations are selected explicitly at query time via a typed wrapper such as `gin.As(alias, value)` rather than by exposing internal derived storage paths directly.
- **D-09:** Internal storage may use a reserved namespace such as `__derived`, but that namespace is not the public query contract.
- **D-10:** Expose a minimal read-only introspection surface on `GINIndex` so callers and diagnostics tooling can discover available aliases and transformer kinds per source path.

#### Failure semantics
- **D-11:** Derivation is strict by default. If a registered transformer cannot produce its companion value for a document, `AddDocument()` fails for that document.
- **D-12:** If soft-fail behavior is ever added later, it must be an explicit opt-in policy rather than silent best-effort omission.

#### Serialized metadata
- **D-13:** Encode/decode must persist explicit representation metadata for every alias registration rather than reconstructing source-to-alias relationships from naming conventions.
- **D-14:** Serialized representation metadata should include the source path, alias, transformer kind/ID, parameters, and any internal target information needed for deterministic round-tripping.
- **D-15:** Convention-based inference is intentionally out of scope because the public query surface is being decoupled from internal storage layout.

### Agent Discretion
- Exact internal type names used for representation metadata structures, as long as the public API keeps `Transformer` terminology.
- Exact wrapper/value type used by `gin.As(alias, value)`, as long as raw-path queries stay default and alias selection stays explicit.
- Exact read-only introspection method names and return shape, as long as aliases and transformer kinds are discoverable.
- Exact hidden target-path format, as long as it is not the public query contract.

### Deferred / Out of Scope
- Transform chaining / ordered pipelines.
- Query-time transformers.
- Convention-based metadata reconstruction.
- Public `__derived` query strings as the primary API.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DERIVE-01 | Configuration can declare derived indexes that preserve raw indexing and add transformed/index-friendly representations alongside it. [CITED: .planning/REQUIREMENTS.md] | `Summary`, `Standard Stack`, and `Architecture Patterns` show the current config can only store one transformer per canonical path and must move to additive per-source registrations. [VERIFIED: gin.go, serialize.go, .planning/phases/09-derived-representations/09-CONTEXT.md] |
| DERIVE-02 | Derived representations are queryable through explicit, deterministic path names or aliases. [CITED: .planning/REQUIREMENTS.md] | `Summary`, `Architecture Patterns`, and `Common Pitfalls` show the current query surface only resolves raw canonical paths and that explicit alias routing should piggyback on `Predicate.Value` via `gin.As(alias, value)` to avoid a second public path namespace. [VERIFIED: query.go, gin.go, .planning/phases/09-derived-representations/09-DISCUSSION-LOG.md] |
| DERIVE-03 | Serialization persists derived-index metadata so encoded indexes round-trip without custom rebuild logic. [CITED: .planning/REQUIREMENTS.md] | `Summary`, `Architecture Patterns`, and `Security Domain` show the current serialized config only persists one `TransformerSpec` per path and needs a new representation metadata contract plus explicit version evolution. [VERIFIED: serialize.go, transformer_registry.go, gin.go, .planning/phases/09-derived-representations/09-CONTEXT.md] |
| DERIVE-04 | Tests and examples cover at least date/time, normalized text, and extracted-subfield derived indexing patterns. [CITED: .planning/REQUIREMENTS.md] | `Project Constraints`, `Common Pitfalls`, and `Validation Architecture` point to the exact docs/examples/tests that currently demonstrate replacement-only semantics and must be updated to additive alias-aware behavior. [VERIFIED: README.md, examples/transformers/main.go, examples/transformers-advanced/main.go, transformers_test.go] |
</phase_requirements>

## Summary

Phase 09 is not just a new helper API. The current implementation bakes replacement-only semantics into all three major surfaces: configuration, builder behavior, and query routing. `GINConfig` stores `fieldTransformers map[string]FieldTransformer` and `transformerSpecs map[string]TransformerSpec`, both keyed by canonical path, so each source path can have only one registered transformer and one serializable spec. [VERIFIED: gin.go, serialize.go]

The builder then replaces the source value at that same canonical path instead of adding a sibling representation. In `decodeTransformedValue()` and `stageMaterializedValue()`, if a transformer exists for the path, the transformed value is staged under the original path and the raw source value is not also indexed as a separate queryable companion. If the transformer returns `ok=false`, the builder silently falls back to indexing the raw value rather than failing the document. That directly conflicts with D-02 and D-11. [VERIFIED: builder.go, .planning/phases/09-derived-representations/09-CONTEXT.md]

The query layer is equally path-only today. `Predicate` is just `{Path string, Operator, Value any}`, `EQ()/GTE()/...` accept `path string`, and `findPath()` resolves only a canonical path string through `idx.pathLookup`. There is no alias resolution surface and no representation metadata lookup. [VERIFIED: gin.go, query.go]

**Primary recommendation:** keep the existing pathID-keyed indexes intact, but treat every companion representation as its own hidden internal target path and add an explicit representation metadata layer on top:

1. Add additive transformer registrations per source path, each with `{source path, alias, transformer id/params, hidden target path}`.
2. Keep raw source paths in `pathLookup` as the default query contract.
3. Add a second derived lookup such as `representationLookup[sourcePath][alias] -> targetPathID`.
4. Route `gin.As(alias, value)` through `Predicate.Value`, not `Predicate.Path`, so existing helper signatures remain viable.
5. Make builder staging additive: raw path always indexes, every registered companion indexes from the raw source value, and any companion failure aborts the document.
6. Persist explicit representation metadata in the encoded index and bump the wire version because current `Version = 6` does not describe this new relationship layer. [VERIFIED: gin.go, builder.go, query.go, serialize.go][CITED: .planning/phases/09-derived-representations/09-CONTEXT.md, .planning/phases/09-derived-representations/09-DISCUSSION-LOG.md]

This design minimizes churn in the core string/numeric/null/trigram structures because they already key off `pathID`. The new work lives at the representation binding layer: config registration, builder fan-out, query resolution, explicit metadata, and docs/tests. [VERIFIED: gin.go, builder.go, query.go, serialize.go]

## Project Constraints

- Follow the repository's functional-options config pattern; new public registration helpers should remain `WithX...` style options, not a separate builder DSL. [CITED: AGENTS.md][VERIFIED: gin.go]
- Keep using `github.com/pkg/errors` for new errors and wrappers. [CITED: AGENTS.md][VERIFIED: go.mod, builder.go, serialize.go]
- Preserve canonical supported JSONPath behavior from Phase 06 for public source-path handling. Any new representation lookup must start from the canonical raw source path. [CITED: .planning/phases/06-query-path-hot-path/06-CONTEXT.md][VERIFIED: jsonpath.go, gin.go, query.go]
- Preserve the transactional `AddDocument()` model from Phase 07. Strict companion failure should abort the document before merge, not partially add some representations. [CITED: .planning/phases/07-builder-parsing-numeric-fidelity/07-CONTEXT.md][VERIFIED: builder.go]
- Keep additive configuration and explicit format evolution. The project already treats binary-version changes as deliberate and test-backed. [CITED: .planning/PROJECT.md, .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md][VERIFIED: gin.go, serialize.go, serialize_security_test.go]
- Keep docs/examples/test verification in plain Go commands and repo-local artifacts. [CITED: AGENTS.md][VERIFIED: README.md, examples/transformers/main.go, examples/transformers-advanced/main.go]
- The milestone already locked "derived indexing augments raw indexing" and "serialization compaction comes later", so Phase 09 should not try to compress the new metadata aggressively yet. [CITED: .planning/PROJECT.md, .planning/STATE.md]

## Standard Stack

### Core

- `pathID`-keyed existing index sections (`StringIndexes`, `NumericIndexes`, `NullIndexes`, `TrigramIndexes`, `StringLengthIndexes`) should remain the storage substrate for raw and companion paths. [VERIFIED: gin.go]
- Explicit representation metadata should be a new additive structure on `GINIndex`, for example `[]RepresentationEntry` plus a rebuilt `representationLookup` map keyed by canonical raw path and alias. [ASSUMED][VERIFIED: gin.go, query.go]
- `TransformerSpec` / `TransformerID` / `ReconstructTransformer()` should remain the serializable transformer vocabulary for built-in companion registrations because they already encode stable kind-and-params reconstruction. [VERIFIED: transformer_registry.go, serialize.go]
- The existing transactional builder path from Phase 07 should fan out raw + companions from one parsed source value rather than re-parsing or chaining through derived outputs. [VERIFIED: builder.go][CITED: .planning/phases/07-builder-parsing-numeric-fidelity/07-CONTEXT.md]
- `Predicate.Value any` is the least disruptive place for explicit alias selection because helper signatures already take `(path string, value any)` and raw-path default behavior remains unchanged. [ASSUMED][VERIFIED: gin.go, query.go, .planning/phases/09-derived-representations/09-DISCUSSION-LOG.md]

### Supporting

- Hidden internal target paths can reuse the existing path canonicalization and `PathDirectory` machinery as long as they are never the public query contract. [ASSUMED][VERIFIED: gin.go, builder.go, query.go]
- Existing transformer examples/tests (`README.md`, `examples/transformers*.go`, `transformers_test.go`) are the right harness for DERIVE-04 because they already exercise date/time, normalized text, and extracted-subfield patterns. [VERIFIED: README.md, examples/transformers/main.go, examples/transformers-advanced/main.go, transformers_test.go]
- `serialize_security_test.go` is the right place for malformed metadata, version-bump, and round-trip guards because the repo already keeps binary hardening coverage there. [VERIFIED: serialize_security_test.go]

### Alternatives Locked Out by Context

- Do not expose a public `__derived` path convention and ask callers to type it. [CITED: .planning/phases/09-derived-representations/09-DISCUSSION-LOG.md]
- Do not infer raw-to-companion relationships from internal path strings during decode. [CITED: .planning/phases/09-derived-representations/09-CONTEXT.md]
- Do not chain multiple transformers on one source path. Every companion must derive from the same raw source value. [CITED: .planning/phases/09-derived-representations/09-CONTEXT.md]
- Do not silently skip failed companion derivations in the default path. [CITED: .planning/phases/09-derived-representations/09-CONTEXT.md]

## Architecture Patterns

### Pattern 1: Additive representation bindings, not one-transformer-per-path maps

**What:** Replace the current one-entry-per-canonical-path config maps with an additive registration model such as `map[string][]RepresentationSpec` or an ordered slice grouped by source path. [ASSUMED]

**When to use:** Every source path with zero, one, or many companion representations. [CITED: .planning/phases/09-derived-representations/09-CONTEXT.md]

**Why:** Today `WithRegisteredTransformer()` and `WithFieldTransformer()` overwrite the same `fieldTransformers[canonicalPath]` and `transformerSpecs[canonicalPath]` slot, so registering two companions on `$.email` would drop one. [VERIFIED: gin.go, serialize.go]

**Example (current collision):**

```go
if c.fieldTransformers == nil {
	c.fieldTransformers = make(map[string]FieldTransformer)
}
if c.transformerSpecs == nil {
	c.transformerSpecs = make(map[string]TransformerSpec)
}
c.fieldTransformers[canonicalPath] = fn
c.transformerSpecs[canonicalPath] = NewTransformerSpec(canonicalPath, id, params)
```

Source: `gin.go`. [VERIFIED: gin.go]

### Pattern 2: Raw path stays public; alias routing rides inside `Predicate.Value`

**What:** Keep `Predicate.Path` as the canonical raw source path and encode alias choice in a typed value wrapper such as:

```go
gin.EQ("$.created_at", gin.As("epoch_ms", july2024))
gin.EQ("$.email", gin.As("lower", "alice@example.com"))
```

[ASSUMED]

**When to use:** Any query targeting a companion representation. Raw queries remain `gin.EQ("$.email", "Alice@Example.COM")`. [CITED: .planning/phases/09-derived-representations/09-DISCUSSION-LOG.md]

**Why:** Existing helper signatures already accept `value any`, but they do not accept a typed path target. Putting alias selection in `Predicate.Path` would force a second public path API or helper-signature churn. [VERIFIED: gin.go, query.go]

**Example (current helper shape):**

```go
func EQ(path string, value any) Predicate {
	return Predicate{Path: path, Operator: OpEQ, Value: value}
}
```

Source: `query.go`. [VERIFIED: query.go]

### Pattern 3: Hidden target paths plus explicit metadata

**What:** Materialize each companion representation as its own hidden internal pathID so the existing indexes keep working, but persist a new explicit representation-metadata section that maps:

`source canonical path + alias + transformer id/params + hidden target path/pathID`

[ASSUMED]

**When to use:** Build, query-resolution, encode/decode, and introspection surfaces. [CITED: .planning/phases/09-derived-representations/09-CONTEXT.md]

**Why:** The current engine assumes every index section is keyed by `pathID`, and that assumption is worth reusing. What is missing is the public-to-hidden binding layer, not a new index dimension. [VERIFIED: gin.go, serialize.go]

**Anti-pattern to avoid:** Rebuilding alias relationships by parsing hidden path strings after decode. [CITED: .planning/phases/09-derived-representations/09-CONTEXT.md]

### Pattern 4: Additive builder fan-out from one raw source value

**What:** For a source path with companion registrations, stage:

1. The raw source value under the raw canonical path.
2. One sibling staged value per registered companion alias.
3. No transform-on-transform chaining.

[ASSUMED]

**When to use:** Both stream-driven scalar staging and the subtree-materialization path used when a transformer exists at the current canonical path. [VERIFIED: builder.go]

**Why:** Today `decodeTransformedValue()` and `stageMaterializedValue()` replace the raw source value and silently tolerate transformer `ok=false` by indexing the original value anyway. Phase 09 needs additive siblings plus strict failure instead. [VERIFIED: builder.go][CITED: .planning/phases/09-derived-representations/09-CONTEXT.md]

**Example (current replacement-only branch):**

```go
if transformed, ok := transformer(prepareTransformerValue(value)); ok {
	value = transformed
}
```

Source: `builder.go`. [VERIFIED: builder.go]

### Pattern 5: Explicit format evolution and read-only introspection

**What:** Add a new versioned metadata contract and a minimal `GINIndex` read-only API that answers questions like "what aliases exist for `$.email`?" and "what transformer kind is alias `lower`?" without exposing hidden storage paths directly. [ASSUMED]

**When to use:** `Encode()/Decode()`, diagnostics, README/examples, and any dynamic query-building use case. [CITED: .planning/phases/09-derived-representations/09-DISCUSSION-LOG.md]

**Why:** Current config serialization writes only `[]TransformerSpec`, which is insufficient for multiple aliases per source path and insufficient to describe raw-to-companion relationships explicitly after decode. [VERIFIED: serialize.go, transformer_registry.go]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Companion storage dimension | A second bespoke per-index alias dimension | Existing pathID-keyed index maps plus hidden target paths and explicit metadata. [ASSUMED][VERIFIED: gin.go] | Reuses the current core data structures instead of duplicating all query/index code. |
| Query alias contract | Public `__derived` path strings or implicit type guessing | `gin.As(alias, value)` or equivalent typed wrapper in `Predicate.Value`. [ASSUMED][CITED: .planning/phases/09-derived-representations/09-DISCUSSION-LOG.md] | Keeps raw path default intact and avoids same-type ambiguity. |
| Transformer serialization vocabulary | Ad hoc stringly typed kind names | Existing `TransformerID`, `TransformerSpec`, and `ReconstructTransformer()`. [VERIFIED: transformer_registry.go] | Built-in transformer kinds and params are already stable and reconstructible. |
| Binary hardening | New one-off fuzz harness | Existing `serialize_security_test.go` patterns. [VERIFIED: serialize_security_test.go] | The repo already keeps decode bounds/version tests there. |
| Example coverage | New standalone demo tree | Existing `README.md`, `examples/transformers/main.go`, and `examples/transformers-advanced/main.go`. [VERIFIED: README.md, examples/transformers/main.go, examples/transformers-advanced/main.go] | Those files already demonstrate the exact feature families DERIVE-04 requires. |

## Common Pitfalls

### Pitfall 1: Treating multiple registrations as an ordered pipeline

**What goes wrong:** `WithToLowerTransformer("$.email", "lower")` and `WithEmailDomainTransformer("$.email", "domain")` accidentally chain so `domain` receives the lowercased output or vice versa. [ASSUMED]

**Why it happens:** The current builder replacement branch is path-local and single-valued; a naive "loop over transformers and keep mutating `value`" implementation would accidentally create order semantics. [VERIFIED: builder.go]

**How to avoid:** Capture the raw prepared source value once, apply every companion transformer to that same raw prepared value, and stage siblings independently. [CITED: .planning/phases/09-derived-representations/09-CONTEXT.md]

### Pitfall 2: Silent fallback when a companion transform cannot be produced

**What goes wrong:** A bad date or regex extract simply omits or replaces the companion silently, and the document still indexes. [VERIFIED: builder.go]

**Why it happens:** Current transformer handling treats `ok=false` as "use the raw original value" instead of an error. [VERIFIED: builder.go]

**How to avoid:** For registered companion aliases, convert `ok=false` into an explicit `AddDocument()` failure with source path and alias context before merge. [CITED: .planning/phases/09-derived-representations/09-CONTEXT.md]

### Pitfall 3: Public API leak of hidden target paths

**What goes wrong:** Docs/examples start teaching users to query `$.email.__derived.lower` or similar internal target strings. [ASSUMED]

**Why it happens:** Hidden internal target paths are convenient for the engine and tempting to expose because `findPath()` already resolves path strings. [VERIFIED: query.go]

**How to avoid:** Keep `findPath()` focused on internal path resolution only after the alias binding layer has already mapped raw path + alias to a target pathID. Public docs and helpers should only mention raw paths and aliases. [ASSUMED][CITED: .planning/phases/09-derived-representations/09-DISCUSSION-LOG.md]

### Pitfall 4: Same-type ambiguity without explicit alias selection

**What goes wrong:** Raw `$.email` and companion `lower` are both strings, so a query like `EQ("$.email", "alice@example.com")` becomes ambiguous between raw and lowered behavior. [ASSUMED]

**Why it happens:** Raw and companion representations can share Go types even when their semantics differ. [CITED: .planning/phases/09-derived-representations/09-DISCUSSION-LOG.md]

**How to avoid:** Keep raw-path queries raw by default and require explicit alias selection for companion representations. [CITED: .planning/phases/09-derived-representations/09-CONTEXT.md]

### Pitfall 5: Opaque custom transformers and encode/decode mismatch

**What goes wrong:** A custom `WithFieldTransformer(path, fn)` companion appears to work in-memory but cannot be reconstructed after `Decode()`. [VERIFIED: gin.go, serialize.go]

**Why it happens:** Only registered transformers carry `TransformerSpec`; opaque function values are not serializable today. [VERIFIED: gin.go, serialize.go, transformer_registry.go]

**How to avoid:** Phase 09 must take an explicit stance: either add an explicit alias-bearing custom-transformer API with a documented "not serializable" restriction or fail fast when encode/decode would otherwise promise unsupported reconstruction. This cannot stay implicit. [ASSUMED]

### Pitfall 6: README/examples still teaching replacement-only semantics

**What goes wrong:** The code ships additive alias routing but the docs still show `WithFieldTransformer("$.created_at", gin.ISODateToEpochMs)` and query the raw path as though it had been replaced. [VERIFIED: README.md, examples/transformers/main.go, examples/transformers-advanced/main.go]

**Why it happens:** Existing docs/examples are entirely written around replacement-only behavior. [VERIFIED: README.md, examples/transformers/main.go, examples/transformers-advanced/main.go]

**How to avoid:** Reserve a plan for docs/example/test updates after the core routing/metadata work lands, and ensure date/time, normalized text, and extracted-subfield examples all demonstrate explicit alias queries. [CITED: .planning/REQUIREMENTS.md]

## Code Examples

### Current config only allows one transformer/spec per canonical path

```go
fieldTransformers       map[string]FieldTransformer // path -> transformer
transformerSpecs        map[string]TransformerSpec  // path -> spec for serialization
```

Source: `gin.go`. [VERIFIED: gin.go]

### Current helper registration overwrites the per-path slot

```go
c.fieldTransformers[canonicalPath] = fn
c.transformerSpecs[canonicalPath] = NewTransformerSpec(canonicalPath, id, params)
```

Source: `gin.go`. [VERIFIED: gin.go]

### Current builder replaces instead of adding a sibling companion

```go
if allowTransform && b.config.fieldTransformers != nil {
	if transformer, ok := b.config.fieldTransformers[canonicalPath]; ok {
		if transformed, ok := transformer(prepareTransformerValue(value)); ok {
			value = transformed
		}
	}
}
```

Source: `builder.go`. [VERIFIED: builder.go]

### Current stream transformer path silently falls back to the raw value

```go
if transformed, ok := transformer(prepareTransformerValue(value)); ok {
	return transformed, true, nil
}
return value, true, nil
```

Source: `builder.go`. [VERIFIED: builder.go]

### Current query surface has no alias target channel

```go
type Predicate struct {
	Path     string
	Operator Operator
	Value    any
}

func EQ(path string, value any) Predicate {
	return Predicate{Path: path, Operator: OpEQ, Value: value}
}
```

Source: `gin.go`, `query.go`. [VERIFIED: gin.go, query.go]

### Current serialized config is insufficient for multiple aliases per source path

```go
type SerializedConfig struct {
	...
	Transformers            []TransformerSpec `json:"transformers,omitempty"`
}
```

Source: `serialize.go`. [VERIFIED: serialize.go]

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `gin.As(alias, value)` should travel through `Predicate.Value` rather than replacing `Predicate.Path` or changing all helper signatures. | Summary / Pattern 2 | Planner may need a broader API edit if maintainers prefer an explicit path-target type instead. |
| A2 | Hidden target paths plus explicit metadata are lower risk than adding an alias dimension to every index section. | Summary / Pattern 3 | Planner may need deeper refactors across all index structures if pathID reuse proves too constraining. |
| A3 | Phase 09 must explicitly define the fate of opaque custom transformers because current `WithFieldTransformer(path, fn)` is not reconstructible after decode. | Common Pitfalls | Planner may leave a hidden compatibility cliff if it focuses only on built-in registered transformers. |

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` plus repo-local examples |
| Config file | none - standard Go toolchain via `go.mod` |
| Quick run command | `go test ./... -run 'Test(Transformer|Representation|Alias|Decode.*Representation|Query.*Alias)' -count=1` |
| Full suite command | `go test ./... -count=1` |
| Estimated runtime | ~20-40 seconds depending on example/test growth |

### Phase Requirements -> Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DERIVE-01 | Raw indexing survives while one source path fans out to one-or-more companion aliases | unit + integration | `go test ./... -run 'Test(RepresentationConfigFansOutRawAndCompanions|BuilderIndexesRawAndCompanionPaths)' -count=1` | ❌ Wave 0 |
| DERIVE-02 | Alias-aware queries are explicit and deterministic while raw path queries remain raw by default | unit | `go test ./... -run 'Test(QueryAliasRoutingUsesAsWrapper|RawPathQueriesRemainDefault)' -count=1` | ❌ Wave 0 |
| DERIVE-03 | Encode/decode preserves representation metadata and alias routing without convention-based rebuild logic | unit + serialization | `go test ./... -run 'Test(RepresentationMetadataRoundTrip|DecodeRepresentationAliasParity)' -count=1` | ❌ Wave 0 |
| DERIVE-04 | Date/time, normalized text, and extracted-subfield examples/docs/tests all show additive alias-aware behavior | unit + example smoke | `go test ./... -run 'Test(DateTransformerAliasCoverage|LowercaseAliasCoverage|RegexExtractAliasCoverage)' -count=1` and `go run ./examples/transformers/main.go` and `go run ./examples/transformers-advanced/main.go` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./... -run 'Test(Transformer|Representation|Alias|Decode.*Representation|Query.*Alias)' -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** `go test ./... -count=1` plus `go run ./examples/transformers/main.go` and `go run ./examples/transformers-advanced/main.go`

### Wave 0 Gaps

- `gin.go` / `query.go` need a new alias-target wrapper type and read-only representation metadata surface. [VERIFIED: gin.go, query.go]
- `builder.go` needs additive raw-plus-companion staging with strict failure semantics. [VERIFIED: builder.go]
- `serialize.go` needs explicit representation metadata and a version bump beyond the current v6 format. [VERIFIED: gin.go, serialize.go]
- `transformers_test.go` needs additive alias-aware coverage instead of only replacement-only transformer behavior. [VERIFIED: transformers_test.go]
- `README.md`, `examples/transformers/main.go`, and `examples/transformers-advanced/main.go` need to stop teaching replacement-only semantics. [VERIFIED: README.md, examples/transformers/main.go, examples/transformers-advanced/main.go]

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|------------------|
| V1 Architecture / Design | yes | Keep the public raw-path contract separate from hidden target paths and persist representation metadata explicitly. [ASSUMED] |
| V5 Input Validation | yes | Companion transform failures must be explicit and path/alias-aware; no silent fallback/omission in the default mode. [VERIFIED: builder.go][CITED: .planning/phases/09-derived-representations/09-CONTEXT.md] |
| V10 Malicious Code / Data | yes | Decode must bounds-check any new representation metadata sections and reject malformed alias/path relationships. [VERIFIED: serialize_security_test.go][ASSUMED] |

### Known Threat Patterns for this Phase

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Alias metadata is inferred from hidden path strings and decodes inconsistently | Tampering / Integrity | Persist explicit representation metadata with source path, alias, transformer id/params, and hidden target info. [CITED: .planning/phases/09-derived-representations/09-CONTEXT.md] |
| Companion transform fails but the document still partially indexes | Tampering | Convert companion `ok=false` / transform errors into explicit `AddDocument()` failure before merge. [CITED: .planning/phases/09-derived-representations/09-CONTEXT.md][VERIFIED: builder.go] |
| Public query API accidentally exposes hidden target paths | Information Disclosure / Integrity | Keep `gin.As(alias, value)` explicit and restrict introspection to read-only metadata instead of raw hidden path names. [CITED: .planning/phases/09-derived-representations/09-DISCUSSION-LOG.md] |
| New representation metadata section accepts oversized or inconsistent counts | Denial of Service | Add explicit max bounds and malformed-section tests beside existing decode hardening cases. [VERIFIED: serialize_security_test.go][ASSUMED] |

## Sources

### Primary (HIGH confidence)

- `AGENTS.md`
- `.planning/ROADMAP.md`
- `.planning/REQUIREMENTS.md`
- `.planning/PROJECT.md`
- `.planning/STATE.md`
- `.planning/phases/09-derived-representations/09-CONTEXT.md`
- `.planning/phases/09-derived-representations/09-DISCUSSION-LOG.md`
- `.planning/phases/06-query-path-hot-path/06-CONTEXT.md`
- `.planning/phases/07-builder-parsing-numeric-fidelity/07-CONTEXT.md`
- `.planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md`
- `gin.go`
- `builder.go`
- `query.go`
- `serialize.go`
- `transformer_registry.go`
- `transformers.go`
- `transformers_test.go`
- `serialize_security_test.go`
- `README.md`
- `examples/transformers/main.go`
- `examples/transformers-advanced/main.go`

### Secondary (MEDIUM confidence)

- `.planning/phases/06-query-path-hot-path/06-RESEARCH.md`
- `.planning/phases/07-builder-parsing-numeric-fidelity/07-RESEARCH.md`
- `.planning/phases/08-adaptive-high-cardinality-indexing/08-RESEARCH.md`
- `gin_test.go`
- `integration_property_test.go`

## Metadata

**Confidence breakdown:**
- additive config + hidden target path recommendation: HIGH
- `Predicate.Value` alias wrapper recommendation: HIGH
- opaque custom-transformer handling requirement: MEDIUM

**Research date:** 2026-04-16
**Valid until:** 2026-05-16
