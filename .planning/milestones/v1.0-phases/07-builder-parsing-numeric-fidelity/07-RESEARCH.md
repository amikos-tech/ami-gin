# Phase 07: Builder Parsing & Numeric Fidelity - Research

**Researched:** 2026-04-15
**Domain:** Builder-side JSON parsing, transactional ingest, and exact integer handling in Go
**Confidence:** HIGH

## User Constraints

### Numeric support boundary
- **D-01:** Phase 07 keeps the current decimal/query surface while adding exact `int64` fidelity for integers. This phase does not broaden into big-int or exact-decimal semantics. [VERIFIED: .planning/phases/07-builder-parsing-numeric-fidelity/07-CONTEXT.md]

### Unsupported-number failure policy
- **D-02:** If a numeric value cannot be represented safely under the Phase 07 rules, `AddDocument()` should fail that document immediately with explicit error context rather than partially indexing the document or silently degrading pruning. [VERIFIED: .planning/phases/07-builder-parsing-numeric-fidelity/07-CONTEXT.md]

### Mixed numeric semantics per path
- **D-03:** Paths that contain both integers and decimals remain one shared numeric domain. Integers must be parsed exactly before any widening or stats decisions are made, but mixed paths are still queryable as numeric fields. [VERIFIED: .planning/phases/07-builder-parsing-numeric-fidelity/07-CONTEXT.md]

### Transformer compatibility
- **D-04:** The parser redesign should preserve the current transformer/config contract. Existing field transformers should continue to work without user-facing config or query changes in this phase. [VERIFIED: .planning/phases/07-builder-parsing-numeric-fidelity/07-CONTEXT.md]

### the agent's Discretion
- Exact parser implementation strategy replacing `json.Unmarshal(..., &any)`, as long as the ingest path stops depending on generic `float64` JSON decoding. [VERIFIED: .planning/phases/07-builder-parsing-numeric-fidelity/07-CONTEXT.md]
- Exact internal numeric representation and widening rules between parse-time exact integers and the existing float-backed numeric indexes, as long as the locked support boundary and failure semantics are preserved. [VERIFIED: .planning/phases/07-builder-parsing-numeric-fidelity/07-CONTEXT.md]
- Exact benchmark fixture shape and reporting for `BUILD-05`. [VERIFIED: .planning/phases/07-builder-parsing-numeric-fidelity/07-CONTEXT.md]
- Exact wording of explicit numeric parse/indexing errors. [VERIFIED: .planning/phases/07-builder-parsing-numeric-fidelity/07-CONTEXT.md]

## Project Constraints

- The repo's build/test workflow is standard Go: `go build ./...`, `go test -v`, focused `go test -v -run ...`, and `go test -bench ...`, so Phase 07 verification should stay on plain Go commands. [VERIFIED: AGENTS.md; VERIFIED: .planning/codebase/TESTING.md]
- Core library code stays in the root `gin` package. Phase 07 should extend `builder.go`, `gin.go`, `query.go`, `serialize.go`, and existing `_test.go` files instead of introducing a new subsystem. [VERIFIED: AGENTS.md; VERIFIED: .planning/codebase/CONVENTIONS.md; VERIFIED: .planning/codebase/STRUCTURE.md]
- Error handling uses `github.com/pkg/errors`; Phase 07 should keep explicit wrapped errors instead of `fmt.Errorf(... %w ...)`. [VERIFIED: AGENTS.md; VERIFIED: .planning/codebase/CONVENTIONS.md]
- Query evaluation currently returns safe no-pruning results on unsupported inputs rather than surfacing runtime query errors. Phase 07 should preserve that query-side contract while making builder-side numeric failures explicit. [VERIFIED: .planning/codebase/ARCHITECTURE.md; VERIFIED: query.go]
- Transformers are configuration-driven and serializable through `TransformerSpec`; Phase 07 should preserve canonical path matching and transformer round-trip behavior. [VERIFIED: gin.go; VERIFIED: transformer_registry.go]

## Summary

Phase 07 is not just a parser swap. The current ingest path in `builder.go` does three things that directly conflict with the phase goal: it decodes the whole document through `json.Unmarshal(..., &any)`, it classifies numbers only after they have already become `float64`, and it mutates builder state eagerly while walking the document. [VERIFIED: builder.go] That means an invalid numeric value discovered late in a document can already have polluted `pathData`, bloom state, trigram state, HLLs, and document counters. [VERIFIED: builder.go]

The current numeric model is also fully float-backed after build. `NumericIndex` stores `GlobalMin`, `GlobalMax`, and per-row-group `RGNumericStat{Min, Max}` as `float64`, while query evaluation converts integral predicate values to `float64` before comparison. [VERIFIED: gin.go; VERIFIED: query.go] This is safe for small integers, but it cannot preserve exact `int64` semantics above the IEEE-754 exact integer range. [ASSUMED]

**Primary recommendation:** implement a transactional ingest pipeline that parses with `encoding/json.Decoder` plus `UseNumber()`, stages every document in a document-local accumulator, and merges into `GINBuilder` only after the full document has parsed and validated successfully. [ASSUMED] Numbers should be classified from raw numeric lexemes into either exact `int64` or finite `float64`, not from already-rounded generic `float64`. [ASSUMED] Integer-only paths should keep exact `int64` stats and query behavior. Paths that mix integers and decimals should stay queryable as one numeric domain, but promotion from exact-int mode to float mode must fail explicitly whenever an existing or staged integer would be rounded by `float64`. [ASSUMED] This is the smallest design that satisfies D-01 through D-03 without silently mis-indexing large integers. [ASSUMED]

The transformer constraint changes the parser design. Because `walkJSON()` currently applies transformers before type dispatch, a transformer registered on `$.foo` can replace an object or array before recursion. [VERIFIED: builder.go] A fully streaming walker would break that behavior unless it can materialize the current subtree on demand. The practical compromise is: stream by default, but when a registered transformer exists for the current canonical path, decode that one subtree into `any` with `UseNumber()`, apply the transformer, run explicit numeric classification over the transformed value, and stage the transformed result through the same accumulator. [ASSUMED]

For `BUILD-05`, the repo needs benchmark-proof deltas, not just a new faster code path. The cleanest way to keep "before vs after" measurements after the implementation lands is to preserve a benchmark-only copy of the legacy `json.Unmarshal(..., &any)` path inside `benchmark_test.go` and compare it against the new explicit parser on the same document fixtures and batch sizes. [ASSUMED]

## Standard Stack

### Core
| Library / Primitive | Purpose | Why it fits Phase 07 |
|---------------------|---------|----------------------|
| `encoding/json.Decoder` + `UseNumber()` | Parse JSON without generic `float64` conversion | Stops relying on `json.Unmarshal(..., &any)` and preserves raw numeric intent long enough for explicit classification. [VERIFIED: builder.go; ASSUMED] |
| Document-local staging accumulator | Keep `AddDocument()` atomic | Prevents partial builder mutation when a late numeric parse or promotion failure occurs. [ASSUMED] |
| Extended `NumericIndex` / `RGNumericStat` | Preserve exact `int64` stats for int-only paths while retaining float stats for mixed paths | Keeps query compatibility while adding exact-integer fidelity where the phase requires it. [VERIFIED: gin.go; ASSUMED] |
| Existing Go benchmark harness in `benchmark_test.go` | Report ingest/build latency and allocations | Matches repo conventions and allows legacy-vs-new comparisons with `go test -bench`. [VERIFIED: .planning/codebase/TESTING.md; VERIFIED: benchmark_test.go] |

### Supporting
| Library / Primitive | Purpose | When to use |
|---------------------|---------|-------------|
| `json.Number` | Distinguish integral and decimal JSON numbers before widening | Use for numeric tokens from the decoder and for materialized transformer subtrees. [ASSUMED] |
| `strconv.ParseInt` / `strconv.ParseFloat` | Explicit numeric classification from source text | Parse integers first when the lexeme has no decimal point or exponent; use float parsing only for decimal/exponent forms. [ASSUMED] |
| Existing transformer registry/config flow | Preserve user-facing transformer behavior | Keep `WithFieldTransformer`, registered transformers, and encode/decode config stable while the ingest internals change. [VERIFIED: gin.go; VERIFIED: transformer_registry.go] |

### Alternatives Considered
| Instead of | Could use | Tradeoff |
|------------|-----------|----------|
| Decoder + document-local staging | Keep `json.Unmarshal(..., &any)` and only add `UseNumber()`-style classification afterward | Fails `BUILD-01` and still leaves eager partial mutation problems. [VERIFIED: builder.go; VERIFIED: .planning/REQUIREMENTS.md] |
| Exact int path mode + explicit mixed-path promotion failure | Store everything in `float64` and rely on query callers to pass safe values | Violates D-01 through D-03 by silently rounding large integers or pretending mixed paths are always safe. [VERIFIED: gin.go; VERIFIED: query.go; VERIFIED: .planning/phases/07-builder-parsing-numeric-fidelity/07-CONTEXT.md] |
| Benchmark-only legacy control | Compare only the new parser against old benchmark numbers from git history | Makes `BUILD-05` less reproducible inside the repo because the same branch cannot run both paths under the same harness. [ASSUMED] |

## Architecture Patterns

### Pattern 1: Parse -> stage -> merge
**What:** `AddDocument()` computes a tentative row-group position, parses/stages the full document, and only then mutates `docIDToPos`, `pathData`, bloom state, and counters. [ASSUMED]  
**When to use:** Every document ingest. [VERIFIED: .planning/REQUIREMENTS.md]  
**Why:** D-02 requires explicit failure without partial indexing, and rollback across bloom/HLL/trigram structures is the wrong direction. [VERIFIED: .planning/phases/07-builder-parsing-numeric-fidelity/07-CONTEXT.md; ASSUMED]

### Pattern 2: Exact-int first, float only when truly needed
**What:** Track path numeric mode as `int-only` until a decimal/exponent or float-like transformer output appears; only then attempt promotion to the shared float domain. [ASSUMED]  
**When to use:** Every numeric path touched by `AddDocument()`. [VERIFIED: .planning/phases/07-builder-parsing-numeric-fidelity/07-CONTEXT.md]  
**Why:** This preserves exact `int64` semantics for integer-only paths while keeping mixed numeric paths queryable. [ASSUMED]

### Pattern 3: Transformer-triggered subtree materialization
**What:** Stay streaming by default, but decode a full subtree into `any` only when a transformer exists for the current canonical path. [ASSUMED]  
**When to use:** Paths registered in `GINConfig.fieldTransformers`. [VERIFIED: builder.go; VERIFIED: gin.go]  
**Why:** It preserves the observable "transform before type dispatch" contract without forcing the whole document back through generic decoding. [ASSUMED]

### Pattern 4: Benchmark with an in-repo legacy control
**What:** Keep a benchmark-only helper that reproduces the old `json.Unmarshal(..., &any)` ingest path, then benchmark it against the new explicit parser on the same fixtures. [ASSUMED]  
**When to use:** `BUILD-05` benchmark families. [VERIFIED: .planning/REQUIREMENTS.md]  
**Why:** It gives reproducible latency/allocation deltas after the implementation lands. [ASSUMED]

## Do Not Hand-Roll

| Problem | Do not build | Use instead | Why |
|---------|--------------|-------------|-----|
| JSON lexing | Custom JSON scanner from scratch | `encoding/json.Decoder` + `UseNumber()` | The phase needs explicit number handling, not a new parser implementation. [ASSUMED] |
| Rollback of global builder state | Reverse mutations across bloom/HLL/trigram maps | Document-local staging and merge-on-success | Rollback is fragile and unnecessary when atomic staging is available. [ASSUMED] |
| Exact-decimal math layer | Big decimal / arbitrary precision numeric engine | Existing float behavior for decimal paths, exact `int64` for integer paths | D-01 explicitly keeps the current decimal/query surface and avoids widening scope. [VERIFIED: .planning/phases/07-builder-parsing-numeric-fidelity/07-CONTEXT.md] |

## Common Pitfalls

### Pitfall 1: Partial indexing on numeric failure
**What goes wrong:** A document with a bad late-field number leaves earlier fields, bloom keys, or counters mutated even though `AddDocument()` returned an error. [VERIFIED: builder.go; ASSUMED]  
**Why it happens:** The current builder mutates global state while walking the document. [VERIFIED: builder.go]  
**How to avoid:** Stage per-document observations first, then merge once the full parse succeeds. [ASSUMED]  
**Warning signs:** A failed `AddDocument()` still changes `Header.NumDocs`, `docIDToPos`, `PathDirectory`, or query results for earlier fields. [ASSUMED]

### Pitfall 2: Silent lossy promotion on mixed paths
**What goes wrong:** A path that first sees `9007199254740993` and later sees `1.5` quietly widens to float mode and loses exact integer fidelity. [ASSUMED]  
**Why it happens:** The current numeric structures are float-backed, and converting too early hides the loss. [VERIFIED: gin.go; VERIFIED: query.go]  
**How to avoid:** Promote only when every staged and existing integer on that path is exactly representable as `float64`; otherwise return an explicit builder error with path context. [ASSUMED]  
**Warning signs:** Equality queries against large integers only match when callers pass rounded floats. [ASSUMED]

### Pitfall 3: Breaking transformer behavior while optimizing the parser
**What goes wrong:** A transformer registered on an object-valued path sees a partially parsed stream instead of the full value it expects. [VERIFIED: builder.go; ASSUMED]  
**Why it happens:** Pure streaming walkers recurse before checking whether a transformer is configured for the current path. [ASSUMED]  
**How to avoid:** Check the canonical path for a transformer before descending; if one exists, materialize exactly that subtree, transform it, and re-stage the transformed value. [ASSUMED]

### Pitfall 4: BUILD-05 without a stable "before" control
**What goes wrong:** Benchmarks show the new parser's raw numbers but cannot report deltas because the old path disappeared from the branch. [ASSUMED]  
**Why it happens:** End-state benchmarks often overwrite the control implementation. [ASSUMED]  
**How to avoid:** Preserve a benchmark-only legacy helper in `benchmark_test.go` with explicit sub-benchmark names like `parser=legacy-unmarshal` and `parser=explicit-number`. [ASSUMED]

## Code Examples

### Existing eager ingest path
```go
func (b *GINBuilder) AddDocument(docID DocID, jsonDoc []byte) error {
	var doc any
	if err := json.Unmarshal(jsonDoc, &doc); err != nil {
		return errors.Wrap(err, "failed to parse JSON")
	}

	b.walkJSON("$", doc, pos)
	return nil
}
```
Source: `builder.go`. [VERIFIED: builder.go]

### Existing float-backed numeric stats
```go
type NumericIndex struct {
	ValueType uint8
	GlobalMin float64
	GlobalMax float64
	RGStats   []RGNumericStat
}

type RGNumericStat struct {
	Min      float64
	Max      float64
	HasValue bool
}
```
Source: `gin.go`. [VERIFIED: gin.go]

### Existing query coercion to float
```go
case int64:
	return idx.evaluateEQ(pathID, entry, float64(v))
```
Source: `query.go`. [VERIFIED: query.go]

## Assumptions Log

| # | Claim | Section | Risk if wrong |
|---|-------|---------|---------------|
| A1 | Mixed paths that would require lossy `int64 -> float64` promotion should fail explicitly rather than silently widening. | Summary / Pitfalls | Planner may need a narrower supported range statement for mixed paths in tests and docs. |
| A2 | A benchmark-only copy of the legacy ingest path is acceptable technical debt because it exists solely to satisfy `BUILD-05` evidence. | Summary / Pattern 4 | Planner may need to replace it with recorded benchmark baselines if maintainers reject keeping the legacy control code. |

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` plus Go benchmark tooling |
| Config file | none - standard Go toolchain via `go.mod` |
| Quick run command | `go test ./... -run 'Test(AddDocument.*Atomic|.*Int64.*|.*Mixed.*Promotion|.*UnsupportedNumber|.*Transformer.*Numeric.*|.*Decode.*Numeric.*)' -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| BUILD-01 | Builder ingest path no longer depends on `json.Unmarshal(..., &any)` | unit + integration | `go test ./... -run 'TestAddDocument.*Parser' -count=1` | ❌ Wave 0 |
| BUILD-02 | Integer-vs-float classification comes from explicit parsing | unit | `go test ./... -run 'Test(ParseNumber|.*Mixed.*Promotion)' -count=1` | ❌ Wave 0 |
| BUILD-03 | Integer-only paths preserve exact `int64` semantics | unit + integration | `go test ./... -run 'Test.*Int64.*Exact|Test.*Decode.*Numeric.*' -count=1` | ❌ Wave 0 |
| BUILD-04 | Unsupported numeric forms fail safely with explicit errors and no partial mutation | unit + integration | `go test ./... -run 'Test(AddDocumentRejectsUnsupportedNumberWithoutPartialMutation|.*UnsupportedNumber.*)' -count=1` | ❌ Wave 0 |
| BUILD-05 | Benchmarks report ingest/build latency and allocation deltas for legacy vs explicit parser paths | benchmark | `go test ./... -run '^$' -bench 'Benchmark(AddDocumentPhase07|BuildPhase07|FinalizePhase07)' -benchtime=1x -count=1` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./... -run 'Test(AddDocument.*Atomic|.*Int64.*|.*Mixed.*Promotion|.*UnsupportedNumber|.*Transformer.*Numeric.*|.*Decode.*Numeric.*)' -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** `go test ./... -count=1` and `go test ./... -run '^$' -bench 'Benchmark(AddDocumentPhase07|BuildPhase07|FinalizePhase07)' -benchtime=1x -count=1`

### Wave 0 Gaps
- `gin_test.go` needs explicit coverage for atomic builder failure, exact `int64` equality/range queries, mixed-path promotion failure, and encode/decode parity. [VERIFIED: gin_test.go; ASSUMED]
- `transformers_test.go` needs coverage showing transformed numeric fields still work under the explicit parser path. [VERIFIED: transformers_test.go; ASSUMED]
- `benchmark_test.go` needs benchmark families that compare `parser=legacy-unmarshal` against `parser=explicit-number` on the same fixtures. [VERIFIED: benchmark_test.go; ASSUMED]

## Security Domain

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|------------------|
| V1 Architecture / Design | yes | Keep trust boundaries explicit between raw JSON bytes, staged per-document observations, and globally merged builder state. [ASSUMED] |
| V5 Input Validation | yes | Parse numeric literals explicitly, reject unsupported or lossy forms with wrapped errors, and keep query-side safe fallback behavior unchanged. [VERIFIED: query.go; ASSUMED] |
| V10 Malicious Code / Data | yes | Ensure malformed or adversarial numeric input cannot partially corrupt builder state or trigger silent rounding. [ASSUMED] |

### Known Threat Patterns for this phase
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Malformed or lossy numeric input silently corrupts pruning stats | Tampering | Explicit numeric parser, path-aware errors, and exact-int path mode with promotion checks. [ASSUMED] |
| Builder state mutates before numeric validation completes | Tampering / Repudiation | Stage document-local changes and merge only after success. [ASSUMED] |
| Transformer path takes a different numeric semantics path than the raw parser | Integrity | Route transformed values through the same explicit number classifier and staging pipeline. [ASSUMED] |

## Sources

### Primary (HIGH confidence)
- `AGENTS.md`
- `.planning/ROADMAP.md`
- `.planning/REQUIREMENTS.md`
- `.planning/STATE.md`
- `.planning/PROJECT.md`
- `.planning/phases/07-builder-parsing-numeric-fidelity/07-CONTEXT.md`
- `.planning/codebase/ARCHITECTURE.md`
- `.planning/codebase/CONVENTIONS.md`
- `.planning/codebase/TESTING.md`
- `.planning/codebase/STRUCTURE.md`
- `.planning/codebase/STACK.md`
- `builder.go`
- `gin.go`
- `query.go`
- `serialize.go`
- `benchmark_test.go`
- `transformers.go`
- `transformers_test.go`
- `transformer_registry.go`

### Secondary (MEDIUM confidence)
- `.planning/phases/06-query-path-hot-path/06-CONTEXT.md`
- `.planning/phases/06-query-path-hot-path/06-RESEARCH.md`
- `.planning/phases/06-query-path-hot-path/06-01-PLAN.md`
- `.planning/phases/06-query-path-hot-path/06-02-PLAN.md`

## Metadata

**Confidence breakdown:**
- Parser/staging recommendation: HIGH
- Numeric-mode bridge recommendation: HIGH
- Benchmark control recommendation: MEDIUM

**Research date:** 2026-04-15
**Valid until:** 2026-05-15
