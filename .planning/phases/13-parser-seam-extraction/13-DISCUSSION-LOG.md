# Phase 13: Parser Seam Extraction - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in `13-CONTEXT.md` — this log preserves the alternatives considered.

**Date:** 2026-04-21
**Phase:** 13-parser-seam-extraction
**Areas discussed:** ParserSink contract shape, Parity harness scope, Transformer subtree buffering, Parser.Name() + WithParser validation

---

## ParserSink Contract Shape

| Option | Description | Selected |
|--------|-------------|----------|
| Expose `*documentBuildState` | Research sketch; sink methods take `state` as first arg. Minimal diff from today's call chain. Zero new allocs/doc. Preserves int64 classifier in builder.go (Pitfall #1). | ✓ |
| Hide state in sink | `docSink` wrapper holds state; parser calls `sink.StageScalar(path, tok)` without state. Adds +1 alloc/AddDocument and a `ShouldBufferSubtree`-style method to replace state-passing. | |
| Parser returns `*documentBuildState` | `Parse(doc, rgID, sink) (*documentBuildState, error)`. Symmetric to Option A with extra return value; `*documentBuildState` appears in two signatures instead of one. | |

**User's choice:** Option A (expose `*documentBuildState`).
**Notes:** User expressed reservation about API surface expansion. Follow-up agreed to narrow exported surface (see D-02): export only `Parser` + `WithParser`; keep `parserSink` and `stdlibParser` unexported. Every parser in the v1.1→v1.2 roadmap lives inside `package gin`, so exporting the sink is premature. Adding exports later is non-breaking; removing is breaking.

---

## Parity Harness — Reference Mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| Golden bytes only | Generate `Encode()` output from v1.0 before PR; commit to `testdata/parity-golden/`. Zero dead code, permanent pinning. | ✓ |
| In-test legacy copy | Keep verbatim `parseAndStageDocument` chain in `parser_parity_test.go`. Run both paths per doc. ~100 LOC dead code until SIMD lands. | |
| Hybrid (goldens + property equivalence) | Goldens + property-based determinism checks on fresh generated docs. More test code, no dead code. | |

**User's choice:** Golden bytes only.

---

## Parity Harness — Coverage

| Option | Description | Selected |
|--------|-------------|----------|
| Authored fixtures (numeric edges, nulls, unicode, nested) | Small hand-crafted corpus covering int64 boundaries, deep nesting, null vs missing, unicode keys, empty arrays, large strings. | ✓ |
| Existing gopter generators | Reuse `GenJSONDocument`, `GenTestDocs`, `GenTestDocsWithNulls`, `GenMixedTypeDocs`. | ✓ |
| Full Evaluate matrix (all 12 operators) | Match + prune predicate per operator (EQ, NE, GT, GTE, LT, LTE, IN, NIN, IsNull, IsNotNull, Contains, Regex). | ✓ |
| Transformer-driven paths | Exercise `decodeTransformedValue` subtree-buffering branch with at least one transformer registration. | ✓ |

**User's choice:** All four dimensions (multi-select).

---

## Transformer Subtree Buffering

| Option | Description | Selected |
|--------|-------------|----------|
| Sink method `ShouldBufferForTransform(path) bool` | One new sink method. Parser asks before each path visit; buffers its own way and calls `StageMaterialized` when true. Mirrors today's `decodeTransformedValue` logic exactly. | ✓ |
| `RepresentedPaths() map[string]bool` upfront | Sink returns path set once at Parse start; parser caches. Cheaper per-visit but leaks a map shape and adds cache-invalidation risk. | |
| Fold into `StageMaterialized` | Parser always buffers subtrees; sink decides transform routing. Dead-simple API but defeats streaming and adds allocation regression. | |

**User's choice:** Sink method `ShouldBufferForTransform(path) bool`.

---

## Parser.Name() + WithParser Validation

| Option | Description | Selected |
|--------|-------------|----------|
| Cache at NewBuilder, nil-check WithParser | WithParser(nil) errors; NewBuilder calls Name() once and stores. Empty Name() also rejected. Matches WithCodec precedent. | ✓ |
| Call Name() on demand, nil-check only | No caching; parsers trusted to return const string cheaply. No empty-name check. | |
| Cache, allow WithParser(nil) to fall through | Silent fallthrough on nil. Deviates from WithCodec precedent; footgun. | |

**User's choice:** Cache at NewBuilder, nil-check WithParser.

---

## Error Wrapping at AddDocument

| Option | Description | Selected |
|--------|-------------|----------|
| No wrap; parser owns its error context | AddDocument returns parser error verbatim. Phase 14 adds parser name as telemetry attribute, not in error string. | ✓ |
| Wrap with parser name at AddDocument | `errors.Wrapf(err, "parser %q", name)` at the seam. Visible even without telemetry; changes v1.0 error text. | |
| Wrap only on sink-mismatch errors | Most errors pass through; only sink-contract violations get wrapped. Adds classification logic at the seam. | |

**User's choice:** No wrap; parser owns its error context.

---

## Claude's Discretion

Areas the user deferred to planner/implementation judgment:
- `Parser.Name()` naming format beyond "stdlib" — defer until SIMD adds a second data point
- `ensureDecoderEOF` placement — parser-internal; stays inside `stdlibParser.Parse`
- Typed parse-error sentinels — only if patterns emerge during implementation
- Dedicated `BenchmarkAddDocumentThroughParser` vs relying on existing builder benchmarks
- Goldens-regeneration build-tag gating (vs always-runnable test)

## Deferred Ideas

- **Export `ParserSink`** — revisit only if a third-party parser use case materializes post-v1.2.
- **Name() format standard** (e.g., `vendor/lib/version`) — defer pending SIMD data point.
- **`EvaluateContext` / `BuildFromParquetContext`** — scheduled for Phase 14 (OBS-07), explicitly out of scope here.
