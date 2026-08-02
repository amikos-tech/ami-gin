# Quick Task 260802-hkk: verify and address issue #54 — Research

**Researched:** 2026-08-02  
**Scope:** Remove nested-array path amplification while preserving supported wildcard queries; add an opt-in per-document staged-path limit.  
**Confidence:** HIGH

## User Constraints (from CONTEXT.md)

### Implementation Decisions

#### Wildcard behavior
- Preserve all supported public wildcard-query behavior, such as `$.orders[*].id`.
- Internal numbered/wildcard path combinations do not need to be retained when they are unnecessary to provide that public behavior.

#### Safety boundary
- Add an opt-in, configurable staged-path budget as defense in depth.
- The default must preserve existing behavior; callers handling untrusted JSON can configure a limit and receive a clear ingest error rather than risking excessive resource use.

#### Scope
- Keep the PR focused on the amplification fix plus regression and wildcard-behavior tests.
- Do not broaden it into parser nesting-limit alignment or unrelated documentation changes.

#### the agent's Discretion
- Select the smallest compatible implementation and precise configuration/API shape.

### Specific Ideas

- Tests should demonstrate that deeply nested arrays no longer grow exponentially in work or allocations.
- Tests should demonstrate that wildcard queries continue to return the expected row groups.

## Summary

The defect is real: the array walkers visit each element twice, once at a private numbered path and once at its wildcard path. This happens in the materialized-value walker, the default stdlib parser, and the optional SIMD parser; a fourth copy exists in the in-package materialized-parser parity fixture. Each nested-array level therefore doubles the generated path variants. [CITED: https://github.com/amikos-tech/ami-gin/issues/54] [VERIFIED: codebase grep]

**Primary recommendation:** retain only `path+"[*]"` when descending into an array, in every production walker, then add a zero-disabled `MaxStagedPaths` builder-time budget enforced whenever a document state first creates a path. This preserves the supported JSONPath surface, which accepts wildcards but rejects numeric array indexes. [VERIFIED: codebase grep]

## Project Constraints (from AGENTS.md)

- Follow the Go functional-options convention: option-level validation plus `GINConfig.validate`; use `github.com/pkg/errors` for new/wrapped errors. [CITED: AGENTS.md]
- Keep the change narrow, prioritize correctness over performance, and use the documented Go build/test commands. [CITED: AGENTS.md]
- Keep restricted internal repository information out of commits, PRs, and related artifacts; use squash merge for the eventual PR. [CITED: AGENTS.md]

## Recommended Implementation

### 1. Collapse array descent to wildcard-only paths

Change each array branch to recurse once per item, at the wildcard child path. The current numbered variants cannot be addressed through the public JSONPath validator (`[0]`/`jp.Nth` is rejected), whereas `[*]` is supported at any level. [VERIFIED: codebase grep]

```go
// Array descent: use in builder.go, parser_stdlib.go, and parser_simd.go.
for each item in array {
    stage/walk(path + "[*]", item)
}
```

Apply this to:

- `builder.go:stageMaterializedValue` — used for buffered transformer subtrees and the internal `walkJSON` path. [VERIFIED: codebase grep]
- `parser_stdlib.go:streamValue` — the normal `AddDocument` path. [VERIFIED: codebase grep]
- `parser_simd.go:walkElement` — keep the explicitly opt-in SIMD parser behavior aligned with stdlib. [VERIFIED: codebase grep]
- `parser_parity_test.go:materializedParser` — test-only reference walker must match the new representation or the parity suite compares two different contracts. [VERIFIED: codebase grep]

Container presence remains correct: both parsers mark the parent container present before walking children, while `stageMaterializedValue` marks its current path present before recursing. Therefore `IsNull`/`IsNotNull` behavior for public container paths is not dependent on numbered descendants. [VERIFIED: codebase grep]

This also keeps wildcard transformers working. The existing `TestWildcardSubtreeTransformerNormalizesNestedNumbers` registers `$.items[*].metrics`; after the change that canonical wildcard path remains the one visited. [VERIFIED: codebase grep]

### 2. Add a small, builder-only path budget

Use this API shape:

```go
// MaxStagedPaths limits distinct paths staged for one document. Zero is unlimited.
MaxStagedPaths int

func WithMaxStagedPaths(limit int) ConfigOption
```

`WithMaxStagedPaths` should reject negative input; zero is accepted and means unlimited. `GINConfig.validate` must reject negative struct-literal values as well. This follows the existing public-field/functional-option validation pattern, including zero sentinels in `PrefixBlockSize` and adaptive settings. [VERIFIED: codebase grep]

Enforce the budget through one helper such as `getOrCreateStagedPath(state, canonicalPath) (*stagedPathData, error)`: return an existing path without charging; before creating a new one, fail when `MaxStagedPaths > 0 && len(state.paths) >= MaxStagedPaths`. Route all four creators through it: `MarkPresent`, scalar staging, numeric staging, and materialized staging. [HIGH — recommendation based on verified call graph]

`MarkPresent` currently creates document paths but returns no error. Change the private `parserSink.MarkPresent` contract to return `error`, and propagate it in `parser_stdlib.go`, `parser_simd.go`, `parser_parity_test.go`, and the `recordingSink` test double. This is the smallest correct boundary: otherwise container-only documents can exceed the budget silently or leave incorrect presence state. [VERIFIED: codebase grep]

On exhaustion, return `newIngestErrorString(IngestLayerSchema, canonicalPath, "", errors.Errorf("staged path budget exceeded: limit %d", limit))`. Reusing schema avoids a new public `IngestLayer` value for an ingest-shape guard, and parser staging errors already pass through `stageCallbackError` so `AddDocument` returns the original `*IngestError`. [HIGH — recommendation based on verified error flow]

Keep `MaxStagedPaths` adjacent to parser/numeric failure routing as a runtime, builder-only field. Do **not** add it to `SerializedConfig`, `writeConfig`, or `readConfig`: existing builder-only routing fields are deliberately omitted from the encoded index, so this needs no format-version change. [VERIFIED: codebase grep]

## Regression and Query Tests

Use deterministic path counts, not allocation ceilings. Go allocation totals include a large fixed index setup cost and vary by toolchain/architecture; exact count assertions prove that the representation is linear without creating flaky performance thresholds. The issue’s depth-16 measurement demonstrates why the regression test must stay modest on the unfixed branch. [CITED: https://github.com/amikos-tech/ami-gin/issues/54]

1. Add `TestNestedArraysStageOnlyWildcardPaths` in `gin_test.go`: build `strings.Repeat("[", 8) + "1" + strings.Repeat("]", 8)`, ingest it successfully with the default config, and assert exactly `depth + 1` path-directory entries (root plus one wildcard path per level), with no `"[0]"` path. Before the fix it deterministically produces the exponential private path set; after it, the assertion is structural rather than timing-sensitive. [HIGH — recommendation based on verified traversal]
2. Add an explicit public behavior test over two row groups for `EQ("$.orders[*].id", "wanted")`, including nested object fields, and assert only the matching row group is returned. Update `TestSingleDocumentSingleRowGroupIndexesArraySiblingAndWildcardPaths` to stop requiring `$.items[0]...`/`$.items[1]...` internals while retaining its wildcard scalar and numeric queries. [VERIFIED: codebase grep]
3. Add `TestMaxStagedPaths`: a small positive limit fails with `errors.As(err, *IngestError)`, `Layer() == IngestLayerSchema`, the stable budget text, and no committed document/path data; `WithMaxStagedPaths(0)` accepts the same document and `WithMaxStagedPaths(-1)` rejects configuration. The existing failed-document atomicity test is the closest assertion pattern. [VERIFIED: codebase grep]
4. Update `parser_test.go` event expectations for an array (remove the numbered `materialized:$[0]` event) and keep the materialized parser’s parity behavior synchronized. [VERIFIED: codebase grep]

Recommended commands:

```bash
go test -run '^(TestNestedArraysStageOnlyWildcardPaths|TestMaxStagedPaths|TestSingleDocumentSingleRowGroupIndexesArraySiblingAndWildcardPaths|TestWildcardSubtreeTransformerNormalizesNestedNumbers|TestStdlibParserBeginsDocumentBeforeStaging)$' .
go test ./...
go build ./...
make lint
```

Run the tagged SIMD suite in the configured native-library/CI environment as well. Local SIMD tests intentionally skip when loading is unavailable, while `AMI_GIN_SIMD_REQUIRED=1` makes supported-host loading failures fatal in CI. [VERIFIED: codebase grep]

## Integration Gotchas

- Do not fix only `stageMaterializedValue`: default and SIMD parsers each independently emit both numbered and wildcard paths, so a partial change leaves the amplification or parser parity mismatch intact. [VERIFIED: codebase grep]
- The budget is per document (`documentBuildState.paths`), not a builder/global cap; a rejected document stays unmerged because `AddDocument` merges only after parser success. [VERIFIED: codebase grep]
- Keep the error hard regardless of parser/numeric soft modes. It is an explicitly opted-in safety limit, not a parser or numeric failure; silently skipping it would violate the requested clear failure contract. [HIGH — recommendation based on user constraint and verified routing]
- Removing the stdlib numbered branch makes `fmt` unused in `parser_stdlib.go`; remove that import. `parser_simd.go` still uses `fmt` in `materializeElement`, so retain it there. [VERIFIED: codebase grep]
- No external package is needed, so no dependency or package-legitimacy work is required. [VERIFIED: codebase grep]

## Sources

- [GitHub issue #54](https://github.com/amikos-tech/ami-gin/issues/54) — reproducer, measurements, scope, and candidate directions. [CITED: https://github.com/amikos-tech/ami-gin/issues/54]
- `builder.go`, `parser_stdlib.go`, `parser_simd.go`, `parser_sink.go`, `parser_parity_test.go`, `parser_test.go`, `gin.go`, `ingest_error.go`, `jsonpath.go`, and `serialize.go` — traversal, configuration, error, validation, and persistence behavior. [VERIFIED: codebase grep]
- `AGENTS.md` and `Makefile` — project conventions and verification commands. [CITED: AGENTS.md] [VERIFIED: codebase grep]

## Confidence

| Area | Level | Basis |
|---|---|---|
| Traversal fix | HIGH | All array-emitting call sites and public JSONPath support were inspected. [VERIFIED: codebase grep] |
| Budget design | HIGH | Matches existing config/error/atomic-staging boundaries; exact public name is a recommendation. [VERIFIED: codebase grep] |
| Test plan | HIGH | Existing wildcard, transformer, parser-event, parity, and atomicity tests were located; focused baseline passed. [VERIFIED: codebase grep] |
