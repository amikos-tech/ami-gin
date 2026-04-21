# Phase 13: Parser Seam Extraction - Context

**Gathered:** 2026-04-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Pure refactor. Extract the JSON-parse boundary from `builder.go` into a pluggable `Parser` interface with a `stdlibParser` default that wraps today's `json.Decoder.UseNumber()` logic verbatim. **Zero behavior change.** The parity harness (byte-identical encoded index + identical `Evaluate` across all operators) is the merge gate.

No SIMD parser in this phase. No telemetry wiring in this phase. No CLI changes in this phase. The seam exists so Phase 14 can read `Parser.Name()` as a telemetry attribute and v1.2 can land a SIMD parser without touching builder internals.

**Carrying forward from prior work:**
- Explicit-number ingest + int64 classifier (Phase 07) stays load-bearing — the classifier lives in `builder.go`, not in parsers. Pitfall #1 defense.
- `BuilderOption` convention (Phase 06+): nil arguments return errors (`WithCodec` precedent at `builder.go:105`).
- PROJECT.md constraint: "avoid gratuitous API churn" → exported surface stays minimal.

</domain>

<decisions>
## Implementation Decisions

### Interface Shape

- **D-01: ParserSink exposes `*documentBuildState` as an opaque handle.** Sink methods take `state *documentBuildState` as the first arg; parsers never touch its fields. Rationale: minimal diff from today's `b.stageX(path, tok, state)` call chain (just swap receiver), zero new allocations per AddDocument, keeps the int64 classifier load-bearing in `builder.go` (Pitfall #1 fix for SIMD v1.2). `documentBuildState` stays package-private; the "leak" is internal to `package gin` only.

  Sink method set (initial):
  ```
  BeginDocument(rgID int) *documentBuildState
  StageScalar(state *documentBuildState, path string, token any) error
  StageJSONNumber(state *documentBuildState, path, raw string) error
  StageNativeNumeric(state *documentBuildState, path string, v any) error
  StageMaterialized(state *documentBuildState, path string, value any, allowTransform bool) error
  ShouldBufferForTransform(path string) bool
  ```

- **D-02: Exported surface is narrower than the ROADMAP criterion.** Only `Parser` (interface) and `WithParser(Parser) BuilderOption` (function) are exported. `parserSink` (interface) and `stdlibParser` (struct) stay unexported. Rationale: every parser we plan to ship lives inside `package gin` (stdlib today, SIMD v1.2 via `//go:build simdjson` same-package file). Exporting the sink is non-breaking if a future use case demands it; removing an export is breaking. Ship minimal now.

  **Deviation from ROADMAP success criterion #3:** roadmap lists `ParserSink` and `stdlibParser` as public surface. This phase ships them package-private instead. The criterion is still met in spirit — the seam is additive and no existing signatures change — but the literal public-surface enumeration needs revising when the phase completes. Flag this during verification.

### Buffering Hook

- **D-03: Sink owns when-to-buffer, parser owns how-to-buffer.** The sink exposes `ShouldBufferForTransform(path string) bool` which delegates to `b.config.representations(path)`. Parser calls it at each path visit. If true, parser decodes the whole subtree into a Go value using its own decoder semantics (stdlib uses `json.Decoder.Decode(&value)`; SIMD v1.2 will use `Element.AsInterface()` or equivalent), then calls `sink.StageMaterialized(state, path, value, true)`. If false, parser keeps streaming through `StageScalar` / `StageJSONNumber`. Mirrors today's `decodeTransformedValue` control flow exactly.

### Parity Harness

- **D-04: Golden bytes at `testdata/parity-golden/`.** Generate `Encode()` output from the v1.0 tag before the PR lands, commit the byte blobs, assert `stdlibParser` reproduces them byte-identically. Zero dead code in the repo; goldens are the permanent pinning reference. Regeneration procedure documented in `testdata/parity-golden/README.md` so future serialization-format bumps (post-v9) are reproducible.

- **D-05: Parity coverage spans four dimensions.**
  1. **Authored fixtures** — hand-crafted corpus covering int64 boundaries (`MaxInt64`, `-MaxInt64`, values near `1<<53`), null vs missing, deep nesting, unicode keys, empty arrays, large strings.
  2. **Gopter generators** — reuse existing `GenJSONDocument`, `GenTestDocs`, `GenTestDocsWithNulls`, `GenMixedTypeDocs` from `generators_test.go` for fuzz coverage with shrinker-driven failure reports.
  3. **Full Evaluate matrix** — for each of the 12 operators (`EQ`, `NE`, `GT`, `GTE`, `LT`, `LTE`, `IN`, `NIN`, `IsNull`, `IsNotNull`, `Contains`, `Regex`), exercise one matching and one pruning predicate against the same index built through `stdlibParser`; compare to goldens.
  4. **Transformer-driven paths** — at least one fixture with `WithISODateTransformer` and `WithToLowerTransformer` registered, ensuring the `ShouldBufferForTransform` branch is covered.

  Harness lives in `parser_parity_test.go` (new) — the file the ROADMAP criterion #2 explicitly names.

### Parser.Name() + WithParser Validation

- **D-06: Name() cached once at NewBuilder.** After the options loop, `NewBuilder` calls `parser.Name()` and stores the result on `b.parserName string`. Phase 14 reads the cached string. `WithParser(nil)` returns `errors.New("parser cannot be nil")` (WithCodec precedent). Empty `Name()` is also rejected at NewBuilder (`errors.New("parser name cannot be empty")`) — guards telemetry-attribute integrity. Default parser returns `"stdlib"`.

- **D-07: No error wrapping at AddDocument.** Parsers wrap errors with their own context (e.g., `"stdlib parser: read JSON token: ..."`); `AddDocument` returns `parser.Parse(...)` errors verbatim. Phase 14 adds parser name as a structured telemetry attribute on error events, not in the error string. Keeps error messages stable across parser swaps and avoids duplicate noise when telemetry is enabled.

### Claude's Discretion

- **Name() convention beyond "stdlib"** — not decided here. `stdlibParser` returns `"stdlib"`; future parsers choose their own identifiers (e.g., SIMD v1.2 may return `"pure-simdjson"` or `"pure-simdjson/v0.x"`). No format mandate.
- **`ensureDecoderEOF` placement** — inside `stdlibParser.Parse`, wrapping with the same `"failed to parse JSON"` error today's code emits. Parser-internal detail; not part of the sink contract.
- **Error sub-classes** — if parse errors surface patterns during implementation (e.g., corrupted UTF-8 vs. trailing content), planner may introduce typed sentinels. Not a requirement.
- **Benchmark coverage** — existing `benchmark_test.go` already exercises the builder hot path; running it pre-merge to confirm no regression is implicit in the merge gate. Planner decides whether to add a dedicated `BenchmarkAddDocumentThroughParser` vs relying on existing benchmarks.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents (researcher, planner) MUST read these before acting.**

### Phase specification
- `.planning/ROADMAP.md` §Phase 13 — goal, success criteria (4 items), depends-on
- `.planning/REQUIREMENTS.md` §Parser Seam — PARSER-01 definition
- `.planning/PROJECT.md` §Constraints — "avoid gratuitous API churn" informs D-02

### Research (v1.1 milestone)
- `.planning/research/SUMMARY.md` — TL;DR, phase ordering rationale (A → {B, C} → D), proposed phase split
- `.planning/research/ARCHITECTURE.md` §Pattern 1 (lines 57-173) — concrete `Parser` / `ParserSink` interface sketch; integration points at `builder.go:115, 287, 322`; new-files table
- `.planning/research/PITFALLS.md` §Pitfall 1 — SIMD int demotion (the reason the int64 classifier stays in builder.go); §Pitfall 11 — why the seam must land as a standalone behavior-neutral PR
- `.planning/research/FEATURES.md` — TS-SIMD-1, TS-SIMD-2, TS-SIMD-4 (parity infrastructure this phase delivers)
- `.planning/research/STACK.md` — current dependency inventory (no new deps in this phase)

### Code anchors (current tree)
- `builder.go:115` — `NewBuilder`, where default `parser = stdlibParser{}` lands after options loop; also where `b.parserName = parser.Name()` caching happens (D-06)
- `builder.go:287` — `AddDocument`, call site that swaps from `b.parseAndStageDocument(jsonDoc, pos)` to `b.parser.Parse(jsonDoc, pos, b)` plus `mergeDocumentState` (unchanged)
- `builder.go:322` — `parseAndStageDocument`, whose body moves verbatim into `stdlibParser.Parse`
- `builder.go:345` — `stageStreamValue`, the recursive streaming routine; becomes a method on `stdlibParser` (or a package-level helper) with `sink` threaded through
- `builder.go:438` — `decodeTransformedValue`, whose config-lookup logic becomes `sink.ShouldBufferForTransform(path)` (D-03)
- `builder.go:473-654` — `stageScalarToken`, `stageMaterializedValue`, `stageJSONNumberLiteral`, `stageNativeNumeric`, `stageCompanionRepresentations` — the five staging methods that become sink-interface methods (minus `stageCompanionRepresentations`, which is called from within `stageMaterializedValue` and stays private)
- `builder.go:105` — `WithCodec`, the nil-check BuilderOption precedent D-06 follows
- `generators_test.go:87, 276, 303, 343, 371` — `GenJSONDocument`, `GenTestDocs`, `GenTestDocsWithNulls`, `GenMixedTypeDocs` — gopter generators the parity harness reuses (D-05)
- `integration_property_test.go` — property-based test pattern the parity harness can mirror
- `gin_test.go` — existing Evaluate test matrices (EQ, NE, GT, ..., Regex) whose predicate set feeds D-05 item #3

### Prior-phase evidence
- `.planning/milestones/v1.0-phases/07-builder-parsing-numeric-fidelity/` — BUILD-03 contract (exact-int64 fidelity); the parity harness must preserve this invariant
- `.planning/milestones/v1.0-phases/10-serialization-compaction/` — v9 serialization format; golden bytes in D-04 are v9-encoded

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **Gopter generators** (`generators_test.go`): 11+ generators covering JSON shapes, RG sets, numeric ranges. `GenJSONDocument` (maxDepth), `GenTestDocs` (maxCount), `GenTestDocsWithNulls`, `GenMixedTypeDocs` — reused verbatim in parity harness corpus (D-05 item #2).
- **`integration_property_test.go`** — existing shape-mirror property-test pattern (build index, encode, decode, evaluate, compare) is the template parity_test extends.
- **`gin_test.go`** Evaluate matrices — predicate-per-operator fixtures already written; parity harness reuses their inputs with goldens recomputed.
- **`ensureDecoderEOF`** (`builder.go:336`) — trailing-content guard; stays inside `stdlibParser.Parse` (Claude's discretion).
- **`normalizeWalkPath`** (`builder.go:307`) — path canonicalization; called from inside each stage* method, no changes needed.

### Established Patterns
- **BuilderOption functional-options pattern** (`builder.go:103`): `WithCodec`, `WithParser` both return `func(b *GINBuilder) error`. Nil → error. Last-wins for duplicates (implicit from option-order application). No duplicate-detection.
- **`errors.Wrap` / `errors.Errorf`** from `github.com/pkg/errors` — stdlibParser's internal errors follow this (error class: parser-internal, not wrapped at seam per D-07).
- **Encode/Decode determinism** — v9 serialized format is stable; golden bytes are viable because builds from the same inputs produce the same bytes.
- **`MustNewX` for builder internals** — `MustNewRGSet`, `MustNewBloomFilter` used where nil-return would be a programming bug. Parser construction is not a hot path; constructor uses `New*` with error return.

### Integration Points
- **`NewBuilder` (`builder.go:115`)** — add parser default after options loop: `if b.parser == nil { b.parser = stdlibParser{} }`; then `b.parserName = b.parser.Name()` with empty-name check.
- **`AddDocument` (`builder.go:287`)** — replace direct `b.parseAndStageDocument(jsonDoc, pos)` call with `b.parser.Parse(jsonDoc, pos, b)`. Merge call (`mergeDocumentState`) stays in AddDocument; parser doesn't see merge.
- **`GINBuilder` struct** — add two fields: `parser Parser`, `parserName string`.
- **Phase 14 hook** — Phase 14 reads `b.parserName` as the `parser.name` telemetry attribute on build events. Out of scope for this phase; only commit is: the string is available on the builder.

### Constraints enabled by the existing architecture
- Single-threaded builder (documented in `ARCHITECTURE.md`) — parser can hold per-Parse state without sync primitives.
- `*documentBuildState` already private; exposing as opaque handle on sink methods is package-internal only.
- `v9` format already checked in and serialization is deterministic — enables the golden-bytes strategy without a new determinism contract.

</code_context>

<specifics>
## Specific Ideas

- **Sink method count as acceptance.** The sink should expose 6 methods exactly: `BeginDocument`, `StageScalar`, `StageJSONNumber`, `StageNativeNumeric`, `StageMaterialized`, `ShouldBufferForTransform`. Planner flags any proposal to add a 7th for user review.
- **Default parser as zero value.** `stdlibParser{}` has no fields. `NewBuilder` can test `if b.parser == nil` because the struct has no identity beyond its methods. Avoid pointer receivers unless state is introduced.
- **`parser_parity_test.go` layout.** One file containing: (a) golden-bytes loader helper, (b) fixture corpus (numeric edges, nulls, unicode, nested, transformer paths), (c) gopter property test asserting `Encode(stdlib) == goldens`, (d) 12-operator Evaluate matrix.
- **Goldens regeneration procedure.** Add a `testdata/parity-golden/README.md` documenting: "To regenerate: `git checkout v1.0 && go test -run TestGenerateParityGoldens -tags=regenerate` (writes fresh blobs), then `git checkout -` and commit." Planner decides whether to gate regeneration behind a build tag.

</specifics>

<deferred>
## Deferred Ideas

- **Export `ParserSink`** — if v1.2+ introduces a legitimate need for a third-party parser, export it then. Non-breaking at that point.
- **Parser.Name() format standard** (e.g., `vendor/lib/version`) — not decided; defer until SIMD adds a second parser and gives us two data points.
- **Parser-level benchmarks** — dedicated `BenchmarkAddDocumentThroughParser` vs. relying on existing builder benchmarks. Planner's call.
- **`EvaluateContext` / `BuildFromParquetContext`** — context-aware API variants. Scheduled for Phase 14 (OBS-07), not this phase.
- **Typed parse-error sentinels** — only introduce if patterns emerge during implementation.

</deferred>

---

*Phase: 13-parser-seam-extraction*
*Context gathered: 2026-04-21*
