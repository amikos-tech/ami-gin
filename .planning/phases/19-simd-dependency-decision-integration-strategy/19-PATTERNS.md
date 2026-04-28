# Phase 19: SIMD Dependency Decision & Integration Strategy - Pattern Map

**Generated:** 2026-04-27
**Purpose:** Map Phase 19 planning deliverables to existing project patterns so execution stays documentation-only and downstream SIMD implementation starts from known seams.

## Files To Create Or Modify

| Target | Role | Closest Analog | Notes |
|--------|------|----------------|-------|
| `.planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md` | New phase strategy artifact | `.planning/phases/18-structured-ingesterror-cli-integration/18-VALIDATION.md` and phase context artifacts | Durable planning evidence with exact strings and grep-verifiable decisions |
| `.planning/STATE.md` | Planning state update | Existing `Current Position` and `Accumulated Context` sections | Mark Phase 19 strategy complete and Phase 21 as next implementation target after execution |

## Existing Code Anchors For Downstream Phases

| Anchor | Role | Phase 21/22 Use |
|--------|------|-----------------|
| `parser.go` | Exported `Parser` + `WithParser` seam | `NewSIMDParser` returns `Parser`; callers pass it to `WithParser` |
| `parser_sink.go` | Package-private sink contract | Same-package `parser_simd.go` can feed `StageJSONNumber`, `StageNativeNumeric`, and `MarkPresent` |
| `parser_stdlib.go` | Default parser traversal precedent | SIMD walker mirrors root/object/array present-marking and sink staging semantics |
| `builder.go` | Parser default, parser-name caching, AddDocument parser boundary | Default builds remain `stdlibParser{}`; SIMD construction is explicit |
| `parser_parity_test.go` | Existing parser parity harness | Phase 22 extends parity to SIMD encoded bytes and query results |
| `.github/workflows/ci.yml` | Existing CI matrix | Phase 22 adds `-tags simdjson` jobs without altering default job behavior |

## Concrete Pattern Excerpts

### Existing Opt-In Parser Seam

`parser.go` establishes the public surface that Phase 21 must reuse:

```go
type Parser interface {
	Name() string
	Parse(jsonDoc []byte, rgID int, sink parserSink) error
}
```

`WithParser` already rejects nil parsers and follows the `BuilderOption` pattern:

```go
func WithParser(p Parser) BuilderOption
```

### Existing Default Parser Behavior

`builder.go` defaults the parser after options:

```go
if b.parser == nil {
	b.parser = stdlibParser{}
}
```

This is the default behavior Phase 21 must preserve for non-SIMD builds.

### Existing Sink Contract

`parser_sink.go` already exposes the methods a SIMD parser needs inside the same package:

```go
StageJSONNumber(state *documentBuildState, canonicalPath, raw string) error
StageNativeNumeric(state *documentBuildState, canonicalPath string, v any) error
StageMaterialized(state *documentBuildState, path string, value any, allowTransform bool) error
```

This lets SIMD preserve Phase 07 numeric semantics without exporting `parserSink`.

## Pattern Decisions For Phase 19 Execution

- Keep Phase 19 output in `.planning/` only.
- Do not update `go.mod`, `go.sum`, root source files, README, CHANGELOG, NOTICE, or `.github/workflows/ci.yml`.
- Make the strategy artifact self-contained enough that Phase 21 can implement without reading the full discussion log.
- Include exact grep-verifiable strings in the strategy artifact: `SIMD-01`, `SIMD-02`, `SIMD-03`, `github.com/amikos-tech/pure-simdjson v0.1.4`, `NewSIMDParser() (Parser, error)`, `//go:build simdjson`, and `PURE_SIMDJSON_LIB_PATH`.

## Non-Patterns

- Do not create a new parser package. Phase 13 locked same-package integration because `parserSink` is package-private.
- Do not add a `WithSIMDParser()` convenience option in v1.3. It duplicates `WithParser` and adds API surface without a concrete need.
- Do not encode upstream bootstrap behavior locally. Delegate to `pure-simdjson`; wrap only construction errors and document deployment recipes.
- Do not convert the stop condition into a large runbook. The switch table is the intended Phase 19 depth.
