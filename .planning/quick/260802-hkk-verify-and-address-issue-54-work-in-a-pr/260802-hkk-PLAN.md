---
phase: quick-260802-hkk
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - gin.go
  - builder.go
  - parser_sink.go
  - parser_stdlib.go
  - parser_simd.go
  - parser_parity_test.go
  - parser_test.go
  - gin_test.go
  - parser_parity_simd_test.go
autonomous: true
requirements: [ISSUE-54]
tags: [performance, ingest, jsonpath, simd, tests]

must_haves:
  truths:
    - "A deeply nested JSON array stages one canonical wildcard path per nesting level rather than exponentially many numbered/wildcard variants."
    - "Supported public wildcard predicates such as EQ(\"$.orders[*].id\", \"wanted\") still select exactly the matching row group."
    - "A caller can opt into a per-document MaxStagedPaths limit and receives a clear hard IngestError before any part of an over-budget document is committed."
    - "The default zero MaxStagedPaths value remains unlimited, preserving existing caller behavior."
    - "The stdlib and explicitly opted-in SIMD parser take the same wildcard-only array-staging path."
  artifacts:
    - path: "gin.go"
      provides: "MaxStagedPaths configuration field and WithMaxStagedPaths option with zero-unlimited validation"
      contains: "WithMaxStagedPaths"
    - path: "builder.go"
      provides: "Single budget-enforced creation boundary for documentBuildState paths and wildcard-only materialized array descent"
      contains: "getOrCreateStagedPath"
    - path: "parser_sink.go"
      provides: "Error-returning MarkPresent contract so container-only paths also honor the staged-path budget"
      contains: "MarkPresent(state *documentBuildState, canonicalPath string) error"
    - path: "parser_stdlib.go"
      provides: "Default-parser wildcard-only array descent with propagated presence-stage errors"
      contains: "path+\"[*]\""
    - path: "parser_simd.go"
      provides: "SIMD-parser wildcard-only array descent with propagated presence-stage errors"
      contains: "rawPath+\"[*]\""
    - path: "gin_test.go"
      provides: "Deterministic deep-array, public-wildcard, and staged-path-budget regression coverage"
      contains: "TestNestedArraysStageOnlyWildcardPaths"
    - path: "parser_parity_simd_test.go"
      provides: "SIMD-tagged structural regression proving nested arrays use the same canonical path set"
      contains: "TestSIMDParserNestedArraysStageOnlyWildcardPaths"
  key_links:
    - from: "parser_sink.go:GINBuilder.MarkPresent"
      to: "builder.go:getOrCreateStagedPath"
      via: "all container-presence creation goes through the budget-enforced helper"
      pattern: "getOrCreateStagedPath"
    - from: "parser_stdlib.go:streamValue"
      to: "builder.go:stageMaterializedValue"
      via: "both descend into arrays only through the canonical [*] child path"
      pattern: "\[\*\]"
    - from: "parser_simd.go:walkElement"
      to: "builder.go:stageMaterializedValue"
      via: "SIMD and materialized ingestion retain the same canonical wildcard representation"
      pattern: "\[\*\]"
    - from: "gin_test.go:TestWildcardArrayQueryReturnsOnlyMatchingRowGroup"
      to: "query.go:Evaluate"
      via: "public EQ predicate over $.orders[*].id"
      pattern: "EQ\(\"\\$\.orders\[\\*\]\.id\""
---

<objective>
Verify and fix issue #54 on the existing dedicated PR branch (`fix/issue-54`): eliminate exponential staged-path amplification for nested arrays while retaining every supported public wildcard query and adding an opt-in per-document staged-path budget.

Purpose: JSON input with nested arrays must remain safe to ingest without retaining private numeric-index paths that the public JSONPath validator cannot query.
Output: Canonical wildcard-only staging in the materialized, stdlib, and SIMD walkers; a zero-disabled MaxStagedPaths guard; and deterministic regression coverage without allocation thresholds.
</objective>

<execution_context>
@/Users/tazarov/.codex/get-shit-done/workflows/execute-plan.md
@/Users/tazarov/.codex/get-shit-done/templates/summary.md
</execution_context>

<context>
@AGENTS.md
@.planning/STATE.md
@.planning/quick/260802-hkk-verify-and-address-issue-54-work-in-a-pr/260802-hkk-CONTEXT.md
@.planning/quick/260802-hkk-verify-and-address-issue-54-work-in-a-pr/260802-hkk-RESEARCH.md
@gin.go
@builder.go
@parser_sink.go
@parser_stdlib.go
@parser_simd.go
@parser_parity_test.go
@parser_test.go
@gin_test.go
@parser_parity_simd_test.go

<interfaces>
Existing contracts to preserve:

- `GINConfig` is public and configured through `ConfigOption`; `NewConfig` applies each option then calls `GINConfig.validate`. Existing zero-sentinel fields such as `PrefixBlockSize` are accepted by `validate` while negative values are rejected.
- `documentBuildState.paths` is a per-document `map[string]*stagedPathData`. Today `stageScalarToken`, `stageMaterializedValue`, `stageNumericObservation`, and `GINBuilder.MarkPresent` create entries independently through `state.getOrCreatePath`; issue #54's budget must centralize all four paths.
- `parserSink` is package-private. `GINBuilder` is its production implementation; `recordingSink` in `parser_test.go` is its test double. `AddDocument` unwraps an error tagged by `tagStageError` before considering `ParserFailureMode`, so `MarkPresent` must return a tagged stage error as well.
- The JSONPath validator supports wildcard segments (`[*]`) and rejects numeric array indexes. Numeric internal paths such as `$.items[0].label` are therefore not a public compatibility surface.
- `newIngestErrorString(IngestLayerSchema, path, value, err)` creates the established hard ingest error. Builder-only ingest routing settings are intentionally omitted from `SerializedConfig`, `writeConfig`, and `readConfig`; MaxStagedPaths follows that model and must not change the wire format.
- `newTestSIMDParser(t)` in the simdjson-tagged integration tests skips unsupported/unavailable local environments and makes supported-host loading fatal when `AMI_GIN_SIMD_REQUIRED=1`; use it for the tagged structural test rather than constructing a parallel helper.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Add a per-document staged-path budget at the single creation boundary</name>
  <files>gin.go, builder.go, parser_sink.go, parser_stdlib.go, parser_simd.go, parser_parity_test.go, parser_test.go, gin_test.go</files>
  <behavior>
    - `NewConfig(WithMaxStagedPaths(-1))` and a `GINConfig{MaxStagedPaths: -1}` passed to `NewBuilder` fail validation; zero is accepted and means unlimited.
    - A positive limit rejects the first distinct canonical path beyond the limit with an extractable `*IngestError`, `Layer() == IngestLayerSchema`, and stable text `staged path budget exceeded: limit N`.
    - The failed document leaves document counters, doc-ID mappings, and committed path data unchanged; the same document succeeds with the zero default.
    - Container presence (root/object/array) is budgeted just like scalar, numeric, and materialized path staging.
  </behavior>
  <action>
    First add the failing tests in `gin_test.go`, then implement the smallest shared boundary. Per the CONTEXT safety decision, add public `MaxStagedPaths int` to `GINConfig` adjacent to the builder-time ingest routing fields, documented as a per-document limit where zero is unlimited. Add `WithMaxStagedPaths(limit int) ConfigOption`; it rejects only negative values, accepts zero, and stores the value. Extend `GINConfig.validate` to reject negative struct-literal values too. Do not add the field to `SerializedConfig`, `writeConfig`, `readConfig`, or any documentation file: this is a runtime builder guard and the locked scope excludes unrelated documentation/format work.

    In `builder.go`, replace the per-document `documentBuildState.getOrCreatePath` use at every staging creator with one `GINBuilder` helper, named `getOrCreateStagedPath(state, canonicalPath) (*stagedPathData, error)`. It returns an existing entry without charging; before creating a new entry, if `b.config.MaxStagedPaths > 0 && len(state.paths) >= b.config.MaxStagedPaths`, return `newIngestErrorString(IngestLayerSchema, canonicalPath, "", errors.Errorf("staged path budget exceeded: limit %d", b.config.MaxStagedPaths))`; otherwise allocate the same initialized `stagedPathData` and insert it. Route `stageScalarToken`, `stageMaterializedValue`, and `stageNumericObservation` through this helper and propagate its error before mutating the returned state. Keep the builder-global `GINBuilder.getOrCreatePath` and merge code unchanged.

    Change the private `parserSink.MarkPresent` signature to return `error`. Implement `GINBuilder.MarkPresent` by calling the new helper, setting `present` only after it succeeds, and wrapping its result with `tagStageError` so `AddDocument` preserves a schema-layer hard error even when `ParserFailureMode` is soft. Propagate that error immediately from object/array presence sites in `stdlibParser.streamValue`, `simdParser.walkElement`, and test-only `stageMaterializedDocument`; update `recordingSink.MarkPresent` to record its event and return nil. Preserve all other parserSink method signatures.

    In `gin_test.go`, add `TestMaxStagedPaths` using `NewConfig`, `NewBuilder`, and a small container/array document that needs more distinct canonical paths than a limit of two. Assert `errors.As` finds `*IngestError`, its layer is schema, its canonical path is the first over-budget path, and its cause contains the exact budget text. Assert no document bookkeeping or `pathData` was committed, then prove the same JSON succeeds using `WithMaxStagedPaths(0)`. Include option-level negative validation and a direct negative config-literal validation assertion. Use existing atomicity helpers/patterns; do not use memory, allocation, or timing assertions.
  </action>
  <verify>
    <automated>go test -count=1 -run '^(TestMaxStagedPaths|TestAddDocumentRejectsUnsupportedNumberWithoutPartialMutation|TestAddDocumentAtomicity)$' .</automated>
  </verify>
  <done>`MaxStagedPaths` is an opt-in public config field with zero-unlimited and negative-value validation; all four document path-creation routes, including container presence, share one enforcing helper; exhaustion is a stable hard schema-layer IngestError that commits none of the rejected document; focused budget and existing atomicity tests pass.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Canonicalize every array walker to wildcard-only descent</name>
  <files>builder.go, parser_stdlib.go, parser_simd.go, parser_parity_test.go, parser_test.go</files>
  <behavior>
    - Materialized, stdlib, SIMD, and materialized-parser parity walkers stage each array item once at the corresponding `[*]` child path.
    - No walker emits private `[...]` numeric path variants solely to duplicate a supported wildcard path.
    - The generic stdlib parser event test records `materialized:$[*]` for an array and no longer expects `materialized:$[0]`.
    - The pre-existing stdlib/materializing-parser byte-parity suite remains green after both reference walkers adopt the same representation.
  </behavior>
  <action>
    Per the CONTEXT wildcard-behavior decision, retain the canonical wildcard path and remove only unqueryable numeric duplicates. In `builder.go:stageMaterializedValue`, change the `[]any` branch to recurse once per item at `path+"[*]"`; do not build `fmt.Sprintf("%s[%d]", ...)` variants. Apply the same single wildcard descent in the array branch of `parser_stdlib.go:streamValue`, `parser_simd.go:walkElement`, and the test-only `stageMaterializedDocument` in `parser_parity_test.go`. Keep object-key sorting, transformer buffering, scalar/numeric routing, and parent-container presence behavior intact. Keep SIMD's `fmt` import because `materializeElement` still uses indexed paths for diagnostic materialization; remove `fmt` imports only where they become unused, then run gofmt on every touched Go file.

    Update `TestStdlibParserStagesArrayIndexAndWildcardOnGenericSink` in `parser_test.go` to express the new external contract: the expected events are begin, root present, and one wildcard materialization; numeric index events are no longer expected. Do not introduce an alternate indexed representation or a public JSONPath change—numeric array indexing remains unsupported by `jsonpath.go`.
  </action>
  <verify>
    <automated>go test -count=1 -run '^(TestStdlibParserStagesArrayIndexAndWildcardOnGenericSink|TestStdlibParserBeginsDocumentBeforeStaging|TestParserParity_StdlibMatchesMaterializingParser|TestWildcardSubtreeTransformerNormalizesNestedNumbers)$' . &amp;&amp; gofmt -d builder.go parser_stdlib.go parser_simd.go parser_parity_test.go parser_test.go | test -z "$(cat)"</automated>
  </verify>
  <done>Every production array traversal plus its parity fixture recurses only through `[*]`; the parser sink event contract and byte-parity test match that representation; existing wildcard-transformer behavior remains green.</done>
</task>

<task type="auto">
  <name>Task 3: Lock down structural scaling and public wildcard behavior in stdlib and SIMD paths</name>
  <files>gin_test.go, parser_parity_simd_test.go</files>
  <action>
    Add deterministic, representation-level regressions—not allocation ceilings—to `gin_test.go`:

    1. Add `TestNestedArraysStageOnlyWildcardPaths`. Build a depth-8 JSON document with `strings.Repeat("[", depth) + "1" + strings.Repeat("]", depth)`, ingest it with the default stdlib builder, finalize it, and assert exactly `depth + 1` `PathDirectory` entries (root plus one wildcard child per nesting level). Assert none of their `PathName` values contain `"[0]"`. This count is the regression proof that work/path variants are linear after the fix, while remaining fast enough for the unfixed branch to demonstrate the defect structurally.

    2. Add `TestWildcardArrayQueryReturnsOnlyMatchingRowGroup`. Ingest two row groups with nested `orders` objects, only one containing `{ "id": "wanted" }`. Assert `Evaluate([]Predicate{EQ("$.orders[*].id", "wanted")})` returns exactly row group 0. Extend the existing `TestSingleDocumentSingleRowGroupIndexesArraySiblingAndWildcardPaths` so it retains wildcard string/numeric query checks but no longer requires numbered internals; additionally assert the old `$.items[0]...` and `$.items[1]...` lookup entries are absent. This implements the CONTEXT decision to preserve supported wildcard behavior while dropping needless internal combinations.

    In the exact-first-line-`//go:build simdjson` file `parser_parity_simd_test.go`, add `TestSIMDParserNestedArraysStageOnlyWildcardPaths`. Reuse the same depth-8 JSON construction and `newTestSIMDParser(t)`, build through `NewBuilder(DefaultConfig(), 1, WithParser(parser))`, then assert the finalized path count and absence of `"[0]"` with the same expectations as the stdlib test. The helper's existing platform/load policy makes local unavailable SIMD environments skip and `AMI_GIN_SIMD_REQUIRED=1` make supported CI failures fatal. Do not add a dependency, allocation benchmark, parser-depth policy, or unrelated docs.
  </action>
  <verify>
    <automated>go test -count=1 -run '^(TestNestedArraysStageOnlyWildcardPaths|TestWildcardArrayQueryReturnsOnlyMatchingRowGroup|TestSingleDocumentSingleRowGroupIndexesArraySiblingAndWildcardPaths|TestWildcardSubtreeTransformerNormalizesNestedNumbers)$' . &amp;&amp; go test -count=1 -tags simdjson -run '^TestSIMDParserNestedArraysStageOnlyWildcardPaths$' . &amp;&amp; go test -count=1 ./... &amp;&amp; go build ./... &amp;&amp; make lint</automated>
  </verify>
  <done>Default ingestion has deterministic linear nested-array path coverage, public wildcard predicates select only matching row groups, numeric private paths are explicitly absent, and the tagged SIMD ingestion path proves the identical structural representation; full default tests/build/lint are green.</done>
</task>

</tasks>

<source_audit>

| Source | ID | Feature / constraint | Plan coverage | Status |
| --- | --- | --- | --- | --- |
| GOAL | — | Verify and address issue #54's exponential nested-array staging in a focused PR | Tasks 1–3 | COVERED |
| REQ | ISSUE-54 | Deep nested arrays must not produce exponential work/path variants | Tasks 2–3 | COVERED |
| CONTEXT | Wildcard behavior | Preserve supported public wildcard queries; discard unnecessary internal numbered/wildcard combinations | Tasks 2–3 | COVERED |
| CONTEXT | Safety boundary | Opt-in staged-path budget, unlimited by default, with a clear ingest error | Task 1 | COVERED |
| CONTEXT | Scope | Limit PR to amplification fix plus regression/wildcard tests; no parser-depth or unrelated documentation work | Tasks 1–3 scope fences | COVERED |
| RESEARCH | — | Change materialized, stdlib, SIMD, and parity walkers together | Task 2 | COVERED |
| RESEARCH | — | Enforce budget through all creators, including MarkPresent | Task 1 | COVERED |
| RESEARCH | — | Use deterministic path counts rather than allocation thresholds | Task 3 | COVERED |
| RESEARCH | — | Keep staged-path configuration builder-only / out of serialization | Task 1 | COVERED |

No deferred idea is included, and no source item is unplanned.
</source_audit>

<threat_model>
## Trust Boundaries

| Boundary | Description |
| --- | --- |
| Untrusted JSON bytes → parser and document staging | Nested containers can attempt to create arbitrarily many distinct staging paths before validation/merge. |
| Parser implementation → `parserSink` | Both stdlib and optional native SIMD parsers request document-state mutation through this private interface. |
| Caller config → ingest availability | A low staged-path limit intentionally rejects otherwise valid documents. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
| --- | --- | --- | --- | --- |
| T-hkk-01 | Denial of Service | Array descent in materialized/stdlib/SIMD walkers | mitigate | Task 2 removes exponential numbered-plus-wildcard staging; Task 3 proves linear canonical path count for nested arrays. |
| T-hkk-02 | Denial of Service | `documentBuildState` path allocation | mitigate | Task 1 adds caller-configured `MaxStagedPaths`, charged only for new per-document canonical paths and enforced before state mutation. |
| T-hkk-03 | Tampering | Parser presence callback bypasses budget | mitigate | Task 1 makes `MarkPresent` error-returning and routes it through the same helper/tagged error flow as scalar and numeric staging. |
| T-hkk-04 | Repudiation | Query compatibility after representation change | mitigate | Task 3 asserts public wildcard query results and explicitly documents test expectations around removed private numeric paths. |
| T-hkk-SC | Tampering | Dependency supply chain | accept | No package installation or version change is part of this PR. |
</threat_model>

<verification>
Run the task-focused commands first, then run:

1. `go test -count=1 ./...`
2. `go build ./...`
3. `make lint`
4. In the configured native SIMD CI environment: `AMI_GIN_SIMD_REQUIRED=1 go test -count=1 -tags simdjson ./...`

Do not use allocation count/size thresholds as acceptance gates; the exact canonical `PathDirectory` counts are the deterministic regression proof. Keep the PR on `fix/issue-54`, stage only task-declared files, use conventional commit messages without restricted repository information, and squash merge the eventual PR per AGENTS.md.
</verification>

<success_criteria>
- Nested array staging is canonical and linear in depth across materialized, stdlib, and SIMD ingestion paths.
- `$.orders[*].id` remains a supported, working public predicate and selects the expected row groups.
- `MaxStagedPaths` is opt-in, validates negative inputs, defaults to unlimited, charges only distinct document paths, and returns an extractable hard schema-layer error without partial commit.
- Numeric index paths are no longer emitted merely to duplicate wildcard behavior.
- Deterministic stdlib and SIMD tests pass without allocation thresholds, alongside full default test/build/lint verification.
</success_criteria>

<output>
Create `.planning/quick/260802-hkk-verify-and-address-issue-54-work-in-a-pr/260802-hkk-SUMMARY.md` when done, listing commits and the default/SIMD verification results.
</output>
