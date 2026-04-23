# Phase 17: Failure-Mode Taxonomy Unification - Research

**Researched:** 2026-04-23
**Domain:** Go library ingest failure routing, public API enum rename, builder atomicity, serialization compatibility
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md) [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]

### Locked Decisions [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]

## Implementation Decisions

### Public API Shape
- **D-01:** Introduce `type IngestFailureMode string` as the single mode type for parser, transformer, and numeric ingest failures.
- **D-02:** Export only `IngestFailureHard` and `IngestFailureSoft` as the new public constants. Remove the old public `TransformerFailureMode`, `TransformerFailureStrict`, and `TransformerFailureSoft` names rather than keeping compatibility aliases. This is the intended breaking change.
- **D-03:** Keep `WithTransformerFailureMode(...)` as the transformer-specific option name, but change its parameter type to `IngestFailureMode`. Add `WithParserFailureMode(mode IngestFailureMode)` and `WithNumericFailureMode(mode IngestFailureMode)` as config options on `GINConfig`.
- **D-04:** Defaults are hard for every layer. Empty mode normalizes to `IngestFailureHard`; invalid values return validation errors at option/config validation time.
- **D-05:** Add a root `CHANGELOG.md` if no changelog exists, with an Unreleased breaking-change note: `TransformerFailureMode` / `TransformerFailureStrict` / `TransformerFailureSoft` were replaced by `IngestFailureMode` / `IngestFailureHard` / `IngestFailureSoft`. Keep the migration note one line plus a small before/after snippet if space is useful.

### Soft-Skip Semantics
- **D-06:** `Soft` means "skip the whole failed document silently and return nil" at every configured ingest layer. This intentionally supersedes the current transformer soft behavior, which only skips the companion representation while still admitting the raw document.
- **D-07:** A soft-skipped document must not mutate durable builder state: no path/index changes, no `numDocs` increment, no `nextPos` advancement, no `docIDToPos`/`posToDocID` mapping, and no max row-group update. Subsequent accepted documents continue to pack densely into row groups just as Phase 15's experiment CLI already does after skipped input lines.
- **D-08:** If more than one soft-capable failure could apply to a document, the first failure encountered wins and skips the document. Phase 17 does not need multi-error collection; Phase 18's `IngestError` work can add richer reporting for hard failures.
- **D-09:** Soft parser mode covers ordinary `Parser.Parse` errors from malformed JSON/trailing content/token reads. Parser contract violations detected after `Parse` returns, such as missing `BeginDocument`, multiple `BeginDocument` calls, or row-group ID mismatch, remain hard regardless of parser failure mode because they indicate parser implementation bugs, not user document failures.
- **D-10:** Soft numeric mode covers numeric literal parse failures, non-finite/unsupported native numeric values, and validator-rejected mixed numeric promotion. It does not cover validator-missed merge panics; those remain tragic via Phase 16's `runMergeWithRecover` path.
- **D-11:** Soft transformer mode skips the document when a configured transformer rejects a value. It should not leave the raw value indexed without its required derived companion representation.

### Serialization And Compatibility
- **D-12:** Do not bump the binary serialization `Version` solely for parser/numeric failure modes. Parser and numeric modes are builder-time routing options and do not need to be encoded into finalized index config.
- **D-13:** Transformer failure mode remains serialized in representation metadata because it is already part of transformer specs. Preserve v9 compatibility for existing encoded indexes by accepting the legacy serialized tokens `strict` and `soft_fail` during decode and normalizing them into the new `IngestFailureMode` values.
- **D-14:** If the implementation can preserve existing v9 transformer metadata writes without compromising the public API, prefer no format bump. If the planner chooses to write new on-wire enum strings instead, it must explicitly bump `Version`, update version history, and add format-version tests. The preferred planning default is no format bump.

### Testing And Examples
- **D-15:** Extend the existing transformer hard/soft tests into a per-layer matrix: parser hard/soft, transformer hard/soft, numeric hard/soft. Soft cases assert nil error and no mutation for the skipped document; hard cases assert current behavior is preserved.
- **D-16:** Reuse Phase 16's public failure catalog where possible. It already covers malformed JSON, trailing JSON, transformer rejection, malformed numeric, non-finite numeric, mixed numeric promotion, parser sink contract errors, and capacity/docID gate errors.
- **D-17:** Add `examples/failure-modes/main.go` with two clear configurations: one hard/rejecting config and one soft/skipping config. The example should use fixed documents and print predictable counts/results, not rely on timing, random data, or environment.
- **D-18:** Add a regression test proving soft skips preserve the Phase 16 atomicity oracle shape for at least one parser failure, one transformer failure, and one numeric failure. Full 1000-document property coverage can stay in Phase 16 unless the planner finds a cheap reuse path.

### Claude's Discretion [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]
- Exact internal helper names for the skip signal, normalization functions, and layer checks.
- Whether soft routing is represented as an internal sentinel error, an `AddDocument` control branch, or a small result type, as long as public behavior matches the decisions above.
- Exact test table organization and fixture names.
- Exact wording of hard-mode error messages until Phase 18 wraps them in `IngestError`.

### Deferred Ideas (OUT OF SCOPE) [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]
- Structured `IngestError` with `Path`, `Layer`, `Cause`, and `Value` - Phase 18.
- Experiment CLI grouping by failure layer and structured samples - Phase 18.
- Telemetry counters for skipped documents or tragic events - future/on-demand.
- `ValidateDocument` dry-run API - future milestone with a real consumer.
- Snapshot/restore atomicity - reserve strategy only if validate-before-mutate stops being sufficient.
</user_constraints>

<phase_requirements>
## Phase Requirements [CITED: .planning/REQUIREMENTS.md]

| ID | Description | Research Support |
|----|-------------|------------------|
| FAIL-01 | Unified `IngestFailureMode` type (`Hard`/`Soft`) replaces the existing `TransformerFailureMode` constants and extends to parser and numeric-promotion layers; CHANGELOG flags the breaking change. [CITED: .planning/REQUIREMENTS.md] | Rename and validation touchpoints are `gin.go:281-323`, `gin.go:421-449`, `gin.go:750-817`, and `transformer_registry.go:47-52`; docs touchpoint is a new root `CHANGELOG.md` because no changelog file exists. [VERIFIED: rg; VERIFIED: gin.go; VERIFIED: transformer_registry.go; VERIFIED: find . -maxdepth 1 -iname '*changelog*'] |
| FAIL-02 | Add `WithParserFailureMode(mode)` and `WithNumericFailureMode(mode)`; defaults are `Hard`; `Soft` skips the failing document and returns no error. [CITED: .planning/REQUIREMENTS.md] | Parser routing belongs at `AddDocument` around `b.parser.Parse`; transformer routing belongs at `stageCompanionRepresentations`; numeric routing belongs at `stageJSONNumberLiteral`, `stageNativeNumeric`, `stageNumericObservation`, and `validateStagedPaths`. [VERIFIED: builder.go:315-359; VERIFIED: builder.go:533-566; VERIFIED: builder.go:587-684; VERIFIED: builder.go:775-792] |
</phase_requirements>

## Summary

Phase 17 should be implemented as a narrow routing/API phase on top of the Phase 16 atomicity contract. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: .planning/phases/16-adddocument-atomicity-lucene-contract/16-LEARNINGS.md] The builder already stages parser, transformer, and numeric observations into `documentBuildState` before `mergeDocumentState` updates durable builder state, so soft skips can be implemented by returning from `AddDocument` before `mergeDocumentState` and by ensuring staging-layer failures produce a document-skip control result rather than a committed partial document. [VERIFIED: builder.go:315-359; VERIFIED: builder.go:712-739]

The largest behavior change is transformer soft mode. [VERIFIED: builder.go:533-547; VERIFIED: transformers_test.go:538-576] Current transformer soft mode skips only the derived companion and still indexes the raw rejected document, while the locked Phase 17 decision requires skipping the whole document with no durable mutation. [VERIFIED: builder.go:543-544; VERIFIED: transformers_test.go:557-575; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] Parser and numeric hard behavior already returns errors before durable commit, so their soft mode is mostly a routing change from "return error" to "return nil before commit". [VERIFIED: parser_stdlib.go:21-32; VERIFIED: builder.go:337-359; VERIFIED: builder.go:556-592; VERIFIED: builder.go:775-792]

**Primary recommendation:** Use a private `skipDocument` sentinel or equivalent internal result to route soft parser, transformer, and numeric failures to an early nil return from `AddDocument`, keep hard defaults, do not change `Version`, and preserve v9 transformer wire tokens through private encode/decode mapping. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: gin.go:17-31; VERIFIED: serialize.go:112-125; VERIFIED: serialize.go:1590-1641]

## Project Constraints (from CLAUDE.md / AGENTS.md)

- Build with `go build ./...`, test with `go test -v` or the Makefile `test` target, and use `go run ./examples/basic/main.go` as the existing example execution pattern. [CITED: CLAUDE.md; CITED: AGENTS.md; VERIFIED: Makefile]
- Preserve the Builder -> Index -> Query -> Serialize architecture: `AddDocument` ingests JSON, `Finalize` produces an immutable index, `Evaluate` queries row groups, and `Encode`/`Decode` handle zstd-compressed binary serialization. [CITED: CLAUDE.md; CITED: AGENTS.md; VERIFIED: builder.go; VERIFIED: gin.go; VERIFIED: query.go; VERIFIED: serialize.go]
- Keep JSONPath support limited to the existing supported subset and do not expand parser/query semantics in this phase. [CITED: CLAUDE.md; CITED: AGENTS.md; VERIFIED: jsonpath.go]
- Follow the existing functional-option pattern for new config knobs and use the existing `GINConfig.validate()` validation path for this phase's enum validation. [CITED: CLAUDE.md; VERIFIED: gin.go:374-393; VERIFIED: gin.go:642-652; VERIFIED: gin.go:750-817]
- Use `github.com/pkg/errors` for new errors and wrapping; avoid `fmt.Errorf("%w")` for new library errors. [CITED: CLAUDE.md; CITED: AGENTS.md; VERIFIED: go.mod; VERIFIED: builder.go; VERIFIED: gin.go]
- Keep Makefile-required targets intact: `test`, `integration-test`, `lint`, `lint-fix`, `security-scan`, `clean`, and `help` are already present. [CITED: CLAUDE.md; CITED: AGENTS.md; VERIFIED: Makefile]
- Do not include internal repository or company information in commit messages, PRs, or artifacts; notify the user if a required change would need it. [CITED: AGENTS.md]

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|--------------|----------------|-----------|
| Public failure-mode API rename | API / Backend library | Docs | The exported enum and options live in package `gin`, while the breaking-change notice belongs in `CHANGELOG.md`. [VERIFIED: gin.go:281-323; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] |
| Parser failure routing | API / Backend library | Parser seam | `AddDocument` calls `b.parser.Parse` and currently returns parser errors before contract checks and commit. [VERIFIED: builder.go:315-359; VERIFIED: parser.go:22-39] |
| Transformer failure routing | API / Backend library | Serialization metadata | `stageCompanionRepresentations` handles transformer rejection and `TransformerSpec.FailureMode` is serialized. [VERIFIED: builder.go:533-547; VERIFIED: transformer_registry.go:47-52; VERIFIED: serialize.go:1590-1641] |
| Numeric failure routing | API / Backend library | Validator/merge boundary | Numeric parse and promotion failures occur in staging and validation before merge; merge recovery remains tragic. [VERIFIED: builder.go:556-684; VERIFIED: builder.go:775-792; VERIFIED: builder.go:741-761] |
| Serialized compatibility | Storage / Binary format | API / Backend library | Transformer failure mode appears in serialized config and representation metadata; parser/numeric modes are builder-time choices and should not enter `SerializedConfig`. [VERIFIED: serialize.go:112-125; VERIFIED: serialize.go:1590-1641; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] |
| Example behavior | CLI / Example binary | API / Backend library | Existing examples are standalone Go `main.go` programs using `gin.NewConfig` and `gin.NewBuilder`. [VERIFIED: find examples -maxdepth 2 -type f -name main.go; VERIFIED: examples/transformers/main.go] |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go module `github.com/amikos-tech/ami-gin` | `go 1.25.5` in `go.mod`; local toolchain `go1.26.2 darwin/arm64` available. [VERIFIED: go.mod; VERIFIED: go version] | Implement the library API, builder routing, tests, and examples. [VERIFIED: go.mod; VERIFIED: rg --files] | This phase changes existing Go package code and should not introduce a new runtime layer. [CITED: .planning/PROJECT.md; VERIFIED: gin.go; VERIFIED: builder.go] |
| `github.com/pkg/errors` | v0.9.1. [VERIFIED: go list -m] | Error creation and wrapping. [VERIFIED: go.mod; VERIFIED: builder.go; VERIFIED: gin.go] | Project instructions require this package for errors and existing code already uses it throughout the touched files. [CITED: CLAUDE.md; VERIFIED: builder.go; VERIFIED: gin.go] |
| `encoding/json` stdlib | Go stdlib from active toolchain. [VERIFIED: go version; VERIFIED: parser_stdlib.go:21-32] | Default parser, `json.Number`, and serialized config JSON payloads. [VERIFIED: parser_stdlib.go; VERIFIED: serialize.go:1590-1669] | Existing parser and config serialization paths already depend on it; no parser implementation change is in scope. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: parser_stdlib.go; VERIFIED: serialize.go] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/leanovate/gopter` | v0.2.11. [VERIFIED: go list -m] | Existing property-test framework for atomicity coverage. [VERIFIED: go.mod; VERIFIED: atomicity_test.go:489-525] | Reuse only if a cheap regression property is added; locked decisions say full new 1000-document property coverage can stay in Phase 16. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: atomicity_test.go] |
| `gotest.tools/gotestsum` | v1.13.0 in Makefile and installed locally. [VERIFIED: Makefile; VERIFIED: gotestsum --version] | Makefile test runner with JUnit output. [VERIFIED: Makefile] | Use through `make test`; direct `go test` is better for focused phase checks. [VERIFIED: Makefile; VERIFIED: narrow go test run 2026-04-23] |
| `golangci-lint` | Local version 2.11.4. [VERIFIED: golangci-lint --version] | Lint gate after code changes. [VERIFIED: Makefile] | Run after implementation because enum rename touches many compile surfaces. [VERIFIED: rg TransformerFailure; VERIFIED: Makefile] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Private skip sentinel/result | Snapshot/restore builder state | Snapshot/restore is explicitly deferred; staging-before-commit already supports early skip. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: builder.go:315-359; VERIFIED: builder.go:712-739] |
| Preserving v9 wire tokens | Bump `Version` and write new enum tokens | A version bump requires version-history edits and format-version tests; Phase 17 context prefers no bump if existing transformer metadata writes can be preserved. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: gin.go:17-31; VERIFIED: serialize_security_test.go:587-614] |
| Existing parser seam | Parser implementation rewrite | Parser implementation changes beyond failure routing are out of scope. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: parser.go; VERIFIED: parser_stdlib.go] |

**Installation:**

```bash
# No new dependencies are recommended for this phase. [VERIFIED: go.mod; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]
go mod tidy
```

**Version verification:**

```bash
go list -m -u -f '{{.Path}} {{.Version}}{{if .Update}} update={{.Update.Version}}{{end}}' \
  github.com/pkg/errors github.com/leanovate/gopter \
  github.com/RoaringBitmap/roaring/v2 github.com/ohler55/ojg github.com/klauspost/compress
```

Verified output showed the existing required versions without suggested updates for those modules on 2026-04-23. [VERIFIED: go list -m -u]

## Architecture Patterns

### System Architecture Diagram

```text
AddDocument(docID, jsonDoc)
  -> tragic/capacity/docID gate
  -> compute dense position for new docID
  -> parser.Parse(jsonDoc, pos, builder sink)
       -> ordinary parser error?
            hard: return error
            soft parser mode: return nil before commit
       -> parser contract guard?
            always hard: return error
  -> staged document state
       -> stageCompanionRepresentations
            transformer rejects?
              hard: return error
              soft transformer mode: skip whole document
       -> stageJSONNumberLiteral / stageNativeNumeric / stageNumericObservation
            numeric parse, non-finite, unsupported native numeric, or lossy promotion?
              hard: return error
              soft numeric mode: skip whole document
  -> validateStagedPaths
       -> validator-rejected numeric promotion?
            hard: return error
            soft numeric mode: skip whole document
  -> runMergeWithRecover(mergeStagedPaths)
       -> recovered merge panic?
            always tragic: set tragicErr and return error
  -> durable bookkeeping
       docIDToPos / posToDocID / nextPos / maxRGID / numDocs
```

This diagram follows the existing `AddDocument`, parser sink, staging, validation, merge, recovery, and bookkeeping flow. [VERIFIED: builder.go:315-359; VERIFIED: parser_sink.go:21-60; VERIFIED: builder.go:533-684; VERIFIED: builder.go:712-761]

### Recommended Project Structure

```text
.
├── gin.go                         # IngestFailureMode type, config fields/options, validation
├── builder.go                     # soft-skip routing at parser/staging/validation sites
├── transformer_registry.go         # TransformerSpec.FailureMode type update
├── serialize.go                   # v9 wire-token compatibility for transformer metadata
├── failure_modes_test.go           # new per-layer hard/soft matrix, if planner chooses one file
├── transformers_test.go            # rewrite current transformer soft-fail behavior expectations
├── parser_test.go                  # parser ordinary-error and contract-error coverage
├── atomicity_test.go               # reuse public failure catalog helpers and atomicity shape
├── serialize_security_test.go      # round-trip and legacy-token decode coverage
├── examples/failure-modes/main.go  # predictable hard vs soft example
└── CHANGELOG.md                    # one-line breaking migration note
```

These files are the current touchpoints or missing deliverables for Phase 17. [VERIFIED: rg TransformerFailure; VERIFIED: find examples; VERIFIED: find . -maxdepth 1 -iname '*changelog*'; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]

### Pattern 1: Unified Mode Type With Layer-Specific Config Fields

**What:** Replace `TransformerFailureMode` with `IngestFailureMode`, keep `WithTransformerFailureMode` as the transformer option, and add parser/numeric config options on `GINConfig`. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: gin.go:281-323; VERIFIED: gin.go:352-372]

**When to use:** Use for all public failure-mode knobs in this phase. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]

**Implementation guidance:**

```go
// Sketch based on existing gin.go option patterns, not existing code. [ASSUMED implementation sketch; VERIFIED: gin.go:281-323; VERIFIED: gin.go:642-652]
type IngestFailureMode string

const (
    IngestFailureHard IngestFailureMode = "hard"
    IngestFailureSoft IngestFailureMode = "soft"
)

func WithParserFailureMode(mode IngestFailureMode) ConfigOption {
    return func(c *GINConfig) error {
        normalized, err := normalizeIngestFailureMode(mode)
        if err != nil {
            return errors.Wrap(err, "parser failure mode")
        }
        c.ParserFailureMode = normalized
        return nil
    }
}
```

**Planning note:** If the planner wants no format bump and public string values `hard`/`soft`, add private transformer wire-token mapping for `strict`/`soft_fail`; if the planner chooses constant values `strict`/`soft_fail`, wire preservation is simpler but the public string representation remains legacy-shaped. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; ASSUMED implementation tradeoff]

### Pattern 2: Soft Skip As Internal Control Flow Before Commit

**What:** Convert soft-capable layer failures into an internal skip signal and let `AddDocument` return nil before `mergeDocumentState`. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: builder.go:315-359; VERIFIED: builder.go:712-727]

**When to use:** Use when `Parser.Parse`, transformer rejection, numeric parse/classification, or numeric validation fails and the relevant layer mode is `IngestFailureSoft`. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: builder.go:337-359; VERIFIED: builder.go:533-566; VERIFIED: builder.go:587-684; VERIFIED: builder.go:775-792]

**Example:**

```go
// Sketch based on AddDocument's current parser branch. [ASSUMED implementation sketch; VERIFIED: builder.go:337-339]
if err := b.parser.Parse(jsonDoc, pos, b); err != nil {
    if normalizeIngestFailureMode(b.config.ParserFailureMode) == IngestFailureSoft {
        return nil
    }
    return err
}
```

**Important boundary:** Do not apply parser soft mode to missing/repeated `BeginDocument` or row-group mismatch checks, because Phase 17 locks those as hard parser-contract violations. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: builder.go:341-358; VERIFIED: parser_test.go:337-397]

### Pattern 3: Numeric Soft Routing Around Both Staging And Validation

**What:** Numeric mode must cover parse/classification failures and validator-rejected mixed promotion. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]

**When to use:** Use for errors from `stageJSONNumberLiteral`, `stageNativeNumeric`, `stageNumericObservation`, and `validateStagedPaths`; do not use for `runMergeWithRecover` results. [VERIFIED: builder.go:556-592; VERIFIED: builder.go:612-684; VERIFIED: builder.go:730-761; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]

**Example:**

```go
// Sketch showing the distinction between numeric skip and tragic failure. [ASSUMED implementation sketch; VERIFIED: builder.go:730-739; VERIFIED: builder.go:741-761]
if err := b.validateStagedPaths(state); err != nil {
    if isNumericFailure(err) && normalizeIngestFailureMode(b.config.NumericFailureMode) == IngestFailureSoft {
        return nil
    }
    return err
}
if err := runMergeWithRecover(b.config.Logger, func() { b.mergeStagedPaths(state) }); err != nil {
    b.tragicErr = err
    return err
}
```

**Planning note:** The cleanest implementation may be a typed internal sentinel such as `skipDocumentErr{layer: ingestLayerNumeric}` from staging functions, but Phase 18 owns exported `IngestError` and layer fields. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; ASSUMED implementation sketch]

### Pattern 4: Preserve v9 Wire Compatibility For Transformer Metadata

**What:** Keep parser and numeric modes out of `SerializedConfig`, but keep transformer failure metadata decodable and writable without a `Version` bump. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: serialize.go:112-125; VERIFIED: gin.go:17-31]

**When to use:** Use whenever `writeConfig`, `readConfig`, or representation metadata normalizes `TransformerSpec.FailureMode`. [VERIFIED: serialize.go:1590-1641; VERIFIED: serialize.go:1666-1735; VERIFIED: serialize.go:1810-1845]

**Example:**

```go
// Sketch: private wire helpers keep v9 payload spelling stable. [ASSUMED implementation sketch; VERIFIED: serialize.go:1623-1625; VERIFIED: serialize.go:1829-1831]
func transformerFailureModeWireToken(mode IngestFailureMode) string {
    switch normalizeIngestFailureMode(mode) {
    case IngestFailureSoft:
        return "soft_fail"
    default:
        return "strict"
    }
}
```

**Planning note:** Add tests for decoding legacy `strict` and `soft_fail` tokens into the new type and for round-tripping without `Version` changes. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: serialize_security_test.go:587-614; VERIFIED: gin.go:31]

### Anti-Patterns to Avoid

- **Compatibility aliases for old public names:** The phase explicitly removes `TransformerFailureMode`, `TransformerFailureStrict`, and `TransformerFailureSoft`. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]
- **Layer-local transformer soft skip:** Current soft transformer behavior indexes the raw document and skips only the companion; Phase 17 must skip the whole document. [VERIFIED: builder.go:543-544; VERIFIED: transformers_test.go:557-575; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]
- **Swallowing tragic merge recovery:** `runMergeWithRecover` results close the builder through `tragicErr`; soft mode must not return nil for this path. [VERIFIED: builder.go:730-761; VERIFIED: gin_test.go:540-570; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]
- **Adding parser/numeric modes to `SerializedConfig`:** Parser and numeric modes are builder-time routing choices and should not change finalized index config. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: serialize.go:112-125]
- **Expanding to structured errors or telemetry counters:** Those are deferred to Phase 18 or later. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Atomic rollback for skipped documents | Snapshot/restore of `GINBuilder` durable maps and indexes | Existing staged-state early return before `mergeDocumentState` | Snapshot/restore is explicitly deferred, and current staging already isolates failures before durable bookkeeping. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: builder.go:100-109; VERIFIED: builder.go:315-359; VERIFIED: builder.go:712-727] |
| Structured failure classification | Exported `IngestError` or public layer enum | Internal sentinel/result only | Phase 18 owns structured `IngestError` with `Path`, `Layer`, `Cause`, and `Value`. [CITED: .planning/REQUIREMENTS.md; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] |
| Parser behavior changes | New parser implementation or custom JSON walker | Existing `Parser` seam and `stdlibParser` | Parser implementation changes beyond routing are out of scope. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: parser.go; VERIFIED: parser_stdlib.go] |
| New serialization format | New v10 payload for parser/numeric modes | Preserve v9 and private wire mapping | Phase 17 context says do not bump solely for parser/numeric modes and prefers no format bump for transformer metadata if possible. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: gin.go:17-31] |
| New logging/metrics for skipped docs | Counters or telemetry events | No-op silent skip | Telemetry counters for skipped documents are deferred. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] |

**Key insight:** Phase 16 already made failed ingest calls atomic by construction, so Phase 17 should route failures to the existing pre-commit boundary rather than inventing rollback machinery. [VERIFIED: .planning/phases/16-adddocument-atomicity-lucene-contract/16-LEARNINGS.md; VERIFIED: builder.go:712-739]

## Runtime State Inventory

| Category | Items Found | Action Required |
|----------|-------------|-----------------|
| Stored data | Committed parity-golden `.bin` files exist under `testdata/parity-golden/`, including a transformer fixture; user-owned `.gin` indexes outside the repo may also contain transformer `failure_mode` tokens. [VERIFIED: find testdata/parity-golden; VERIFIED: parser_parity_fixtures_test.go:72-90; VERIFIED: serialize.go:1590-1641] | Preserve decode support for legacy `strict`/`soft_fail` tokens; avoid a v9 wire change unless explicitly bumping `Version`; regenerate goldens only if bytes intentionally change. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: gin.go:17-31; VERIFIED: parity_goldens_test.go:11-40] |
| Live service config | None found for this rename: `.github` and `.claude` had no matches for failure-mode symbols or tokens. [VERIFIED: rg .github .claude returned no matches] | No service configuration migration required. [VERIFIED: rg .github .claude returned no matches] |
| OS-registered state | None found: no plist, systemd service, Dockerfile, compose, or process-manager config files were found in the scanned repo depth. [VERIFIED: find . -maxdepth 4 for plist/service/ecosystem/Dockerfile] | No OS registration update required. [VERIFIED: find command] |
| Secrets/env vars | None found: no `.env*`, SOPS, or secret files were found in the scanned repo depth. [VERIFIED: find . -maxdepth 4 for .env/secret/sops] | No secret or environment-variable rename required. [VERIFIED: find command] |
| Build artifacts | Ignored `coverage.out` and `unit.xml` exist, and neither contains old failure-mode symbols; no build/install directories carrying the old name were found. [VERIFIED: ls coverage.out unit.xml; VERIFIED: git check-ignore; VERIFIED: rg coverage.out unit.xml returned no matches; VERIFIED: find build/dist/bin/egg-info] | No artifact migration required; normal `make clean` can remove test outputs if desired. [VERIFIED: Makefile] |

## Common Pitfalls

### Pitfall 1: Treating Transformer Soft As Companion-Only

**What goes wrong:** A soft transformer rejection could still index the raw value while omitting the required derived companion. [VERIFIED: builder.go:543-544; VERIFIED: transformers_test.go:557-575]

**Why it happens:** Current code continues the transformer loop when `TransformerFailureSoft` is configured. [VERIFIED: builder.go:540-547]

**How to avoid:** Route transformer rejection to the same whole-document skip path used by parser and numeric soft mode. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]

**Warning signs:** `TestBuilderSoftFailSkipsCompanionWhenConfigured` still expects `Header.NumDocs == 2` after the phase, or raw `EQ("$.email", 42)` still matches the rejected document. [VERIFIED: transformers_test.go:565-570]

### Pitfall 2: Softening Parser Contract Violations

**What goes wrong:** A buggy parser that skips `BeginDocument`, calls it twice, or uses the wrong row-group ID would be silently ignored. [VERIFIED: builder.go:341-358; VERIFIED: parser_test.go:337-397]

**Why it happens:** Parser errors and parser contract checks are adjacent in `AddDocument`, but Phase 17 only softens ordinary `Parser.Parse` errors. [VERIFIED: builder.go:337-359; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]

**How to avoid:** Apply parser soft mode only to the direct `b.parser.Parse(...)` error branch and leave post-Parse contract checks hard. [VERIFIED: builder.go:337-359; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]

**Warning signs:** Tests for `did not call BeginDocument`, duplicate `BeginDocument`, or row-group mismatch start expecting nil. [VERIFIED: parser_test.go:337-397]

### Pitfall 3: Missing Validator-Rejected Numeric Promotion

**What goes wrong:** Numeric soft mode may cover malformed literals and non-finite native numerics but still return an error for mixed numeric promotion rejected by `validateStagedPaths`. [VERIFIED: builder.go:556-592; VERIFIED: builder.go:775-792; VERIFIED: gin_test.go:3328-3395]

**Why it happens:** Mixed-promotion rejection can happen during `stageNumericObservation` and again during validation replay. [VERIFIED: builder.go:612-684; VERIFIED: builder.go:775-792]

**How to avoid:** Route numeric soft failures at both staging and `validateStagedPaths`; do not route `runMergeWithRecover` as soft. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: builder.go:730-761]

**Warning signs:** Soft numeric tests pass for malformed numeric literals but fail for a seed integer `9007199254740993` followed by `1.5`. [VERIFIED: atomicity_test.go:295-305; VERIFIED: gin_test.go:3373-3395]

### Pitfall 4: Accidental Wire-Format Change Without Version Bump

**What goes wrong:** New indexes may write different transformer failure-mode strings while still claiming `Version = 9`. [VERIFIED: gin.go:17-31; VERIFIED: serialize.go:1590-1641]

**Why it happens:** `TransformerSpec.FailureMode` is marshaled into serialized config and representation metadata. [VERIFIED: transformer_registry.go:47-52; VERIFIED: serialize.go:1623-1625; VERIFIED: serialize.go:1829-1831]

**How to avoid:** Preserve legacy transformer wire tokens `strict` and `soft_fail` or intentionally bump `Version` with tests. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]

**Warning signs:** `Version` remains 9 while serialized config JSON starts containing `hard` or `soft` for transformer failure metadata. [VERIFIED: gin.go:31; VERIFIED: serialize.go:1590-1641]

### Pitfall 5: Breaking Struct-Literal Defaults

**What goes wrong:** A caller using a zero-value `GINConfig` or legacy struct literal could fail validation because parser/numeric mode fields are empty. [VERIFIED: gin_test.go:266-278; VERIFIED: gin.go:750-817]

**Why it happens:** `GINConfig.validate()` currently accepts some zero sentinel values for struct-literal callers. [VERIFIED: gin.go:750-775; VERIFIED: gin_test.go:266-278]

**How to avoid:** Normalize empty failure modes to `IngestFailureHard` at validation and use sites; set hard defaults in `DefaultConfig`. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: gin.go:655-670]

**Warning signs:** `TestNewBuilderAllowsLegacyConfigLiteralWhenAdaptiveDisabled` or equivalent struct-literal tests fail after adding fields. [VERIFIED: gin_test.go:266-278]

## Code Examples

Verified patterns from current code and recommended sketches:

### Existing Transformer Failure Branch

```go
transformed, ok := registration.FieldTransformer(prepared)
if !ok {
    if normalizeTransformerFailureMode(registration.Transformer.FailureMode) == TransformerFailureSoft {
        continue
    }
    return errors.Errorf("companion transformer %q on %s failed to produce a value", registration.Alias, canonicalPath)
}
```

This is current code and must change from companion-only soft continuation to whole-document skip. [VERIFIED: builder.go:540-547; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]

### Existing Parser Error Branch

```go
if err := b.parser.Parse(jsonDoc, pos, b); err != nil {
    return err
}
```

This is the narrow parser branch where soft parser mode should return nil; the subsequent parser contract checks should remain hard. [VERIFIED: builder.go:337-359; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]

### Existing Validation And Tragic Recovery Boundary

```go
if err := b.validateStagedPaths(state); err != nil {
    return err
}
if err := runMergeWithRecover(b.config.Logger, func() { b.mergeStagedPaths(state) }); err != nil {
    b.tragicErr = err
    return err
}
```

Numeric soft mode should be allowed at the validation error return, but not at the recovered merge panic return. [VERIFIED: builder.go:730-739; VERIFIED: builder.go:741-761; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]

### Recommended Skip Sentinel

```go
// Sketch only. Phase 18 owns exported structured errors. [ASSUMED implementation sketch; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]
var errSkipDocument = errors.New("skip document")

func isSkipDocument(err error) bool {
    return errors.Cause(err) == errSkipDocument
}
```

Use a package-private signal or equivalent result type so public hard-mode error strings remain stable until Phase 18. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: github.com/pkg/errors v0.9.1 in go.mod]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Transformer-only failure mode with `TransformerFailureStrict`/`TransformerFailureSoft`. | Unified `IngestFailureMode` across parser, transformer, and numeric layers. | Phase 17 planned on 2026-04-23. [CITED: .planning/ROADMAP.md; CITED: .planning/REQUIREMENTS.md] | Public breaking rename and simpler mental model. [CITED: .planning/PROJECT.md; CITED: .planning/STATE.md] |
| Transformer soft skips only companion representation. | Soft at any layer skips the whole failed document and returns nil. | Phase 17 planned on 2026-04-23. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] | Prevents raw-without-derived partial indexing. [VERIFIED: transformers_test.go:557-575; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] |
| Merge-layer user errors could poison builder before Phase 16. | Validator-backed commit and `tragicErr` separation make ordinary failures isolated. | Phase 16 completed on 2026-04-23. [VERIFIED: .planning/STATE.md; VERIFIED: .planning/phases/16-adddocument-atomicity-lucene-contract/16-LEARNINGS.md] | Phase 17 can implement soft skips by early return instead of rollback. [VERIFIED: builder.go:712-739] |
| Transformer failure mode strings are part of v9 metadata. | Keep v9 metadata tokens or explicitly bump format if changing them. | v8 introduced explicit companion transformer failure modes; v9 remains current. [VERIFIED: gin.go:17-31] | Avoid accidental format drift. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] |

**Deprecated/outdated:**

- `TransformerFailureMode`, `TransformerFailureStrict`, and `TransformerFailureSoft` should be removed from the public API in this phase. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: gin.go:281-285]
- `TestBuilderSoftFailSkipsCompanionWhenConfigured` reflects obsolete soft semantics and should be rewritten or replaced. [VERIFIED: transformers_test.go:538-576; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Public `IngestFailureMode` string literals should be `hard`/`soft` with private transformer wire-token mapping, rather than exposing legacy `strict`/`soft_fail` as constant values. [ASSUMED implementation tradeoff] | Architecture Patterns / Open Questions | If the planner chooses legacy constant values instead, implementation is simpler but printed/string-literal API remains less aligned with the new mental model. |
| A2 | A private sentinel error is an acceptable implementation shape for soft document skip. [ASSUMED implementation sketch] | Architecture Patterns / Code Examples | If the planner chooses a result type instead, call signatures and tests shift, but public behavior remains the same. |
| A3 | Code examples for new enum/options and wire helpers are implementation sketches based on existing patterns, not existing code. [ASSUMED implementation sketch] | Architecture Patterns / Code Examples | If final helper signatures differ, planner tasks should still preserve the same public behavior and compatibility checks. |
| A4 | Proposed test names and a new `failure_modes_test.go` file are organizational recommendations. [ASSUMED test names; ASSUMED file organization] | Validation Architecture | If the planner extends existing test files instead, requirement coverage remains valid but commands should be renamed to match final tests. |
| A5 | The research remains valid for local code topology until 2026-05-23. [ASSUMED validity window] | Metadata | If the code changes before Phase 17 planning, symbol discovery and line references should be refreshed. |

## Open Questions (RESOLVED)

1. **RESOLVED: Exact public string values for `IngestFailureMode`.**
   - What we know: The public names are locked as `IngestFailureHard` and `IngestFailureSoft`, legacy public names are removed, and legacy transformer wire tokens must decode. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]
   - Resolution: Use public string values `hard` and `soft` for `IngestFailureHard` and `IngestFailureSoft`. Preserve v9 compatibility by mapping transformer metadata to and from legacy wire tokens `strict` and `soft_fail` privately in serialization code. [SELECTED BY PLAN: .planning/phases/17-failure-mode-taxonomy-unification/17-01-PLAN.md and 17-02-PLAN.md]

2. **RESOLVED: Single new `failure_modes_test.go` vs extending existing test files.**
   - What we know: Existing relevant tests are split across `parser_test.go`, `transformers_test.go`, `gin_test.go`, `atomicity_test.go`, and `serialize_security_test.go`. [VERIFIED: rg; VERIFIED: parser_test.go; VERIFIED: transformers_test.go; VERIFIED: atomicity_test.go; VERIFIED: serialize_security_test.go]
   - Resolution: Add a focused `failure_modes_test.go` for cross-layer hard/soft semantics and soft atomicity shape, keep serialization compatibility tests in `serialize_security_test.go`, and update obsolete transformer soft expectations in `transformers_test.go`. [SELECTED BY PLAN: .planning/phases/17-failure-mode-taxonomy-unification/17-01-PLAN.md, 17-02-PLAN.md, and 17-03-PLAN.md]

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|-------------|-----------|---------|----------|
| Go toolchain | Build, tests, examples | yes | `go1.26.2 darwin/arm64`; module declares `go 1.25.5`. [VERIFIED: go version; VERIFIED: go.mod] | Use local `GOTOOLCHAIN=auto` behavior, already configured. [VERIFIED: go env GOTOOLCHAIN] |
| `make` | Full project test/lint/security gates | yes | `/usr/bin/make`. [VERIFIED: command -v make] | Direct `go test`/`go build` for focused checks. [VERIFIED: Makefile] |
| `gotestsum` | `make test` | yes | v1.13.0. [VERIFIED: gotestsum --version; VERIFIED: Makefile] | `make test` installs v1.13.0 if needed. [VERIFIED: Makefile] |
| `golangci-lint` | `make lint` | yes | 2.11.4. [VERIFIED: golangci-lint --version] | No full fallback for lint; planner should keep as required gate. [VERIFIED: Makefile] |
| `govulncheck` | `make security-scan` | yes | Go `go1.26.2` reported. [VERIFIED: govulncheck -version] | Security scan can be run after implementation. [VERIFIED: Makefile] |
| `rg` | Static symbol checks | yes | `/opt/homebrew/bin/rg`. [VERIFIED: command -v rg] | Use `grep` only if needed. [VERIFIED: command -v rg] |
| `awk` | Existing validator marker lint | yes | `/usr/bin/awk`. [VERIFIED: command -v awk] | None needed. [VERIFIED: Makefile] |

**Missing dependencies with no fallback:** None found for this phase. [VERIFIED: environment audit commands]

**Missing dependencies with fallback:** None found for this phase. [VERIFIED: environment audit commands]

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` with gopter v0.2.11 for existing property tests. [VERIFIED: go.mod; VERIFIED: atomicity_test.go] |
| Config file | `go.mod`, `Makefile`, `.golangci.yml`. [VERIFIED: rg --files] |
| Quick run command | `go test ./... -run 'Test(IngestFailureMode|FailureMode|RepresentationFailureMode|AddDocumentPublicFailuresDoNotSetTragicErr)' -count=1` [ASSUMED test names; VERIFIED: existing test infrastructure] |
| Full suite command | `make test && make lint && go build ./...` [VERIFIED: Makefile; CITED: CLAUDE.md] |

### Phase Requirements -> Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| FAIL-01 | Old public type/constants are removed; new `IngestFailureMode`, `IngestFailureHard`, and `IngestFailureSoft` compile and validate; CHANGELOG notes breaking migration. [CITED: .planning/REQUIREMENTS.md; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] | unit + static | `go test ./... -run 'TestIngestFailureMode' -count=1` and `! rg -n 'TransformerFailureMode|TransformerFailureStrict|TransformerFailureSoft' --glob '*.go'` [ASSUMED test names; VERIFIED: rg current symbols] | no for new focused test; `CHANGELOG.md` absent. [VERIFIED: find . -maxdepth 1 -iname '*changelog*'] |
| FAIL-01 | Transformer failure-mode metadata round-trips and legacy `strict`/`soft_fail` tokens decode into the new type without bumping `Version`. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] | serialization unit | `go test ./... -run 'Test(RepresentationFailureModeRoundTrip|DecodeLegacyTransformerFailureModeTokens)' -count=1` [ASSUMED new test name; VERIFIED: serialize_security_test.go:587-614] | partial; `serialize_security_test.go` exists. [VERIFIED: serialize_security_test.go] |
| FAIL-02 | Parser hard returns current parse errors; parser soft returns nil and skips without durable mutation; parser contract violations remain hard. [CITED: .planning/REQUIREMENTS.md; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] | unit matrix | `go test ./... -run 'TestParserFailureMode' -count=1` [ASSUMED test name] | partial; `parser_test.go` exists. [VERIFIED: parser_test.go] |
| FAIL-02 | Transformer hard returns current rejection error; transformer soft returns nil and skips the whole document. [CITED: .planning/REQUIREMENTS.md; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] | unit matrix | `go test ./... -run 'TestTransformerFailureMode' -count=1` [ASSUMED test name] | partial; `transformers_test.go` exists but current soft expectation is obsolete. [VERIFIED: transformers_test.go:538-576] |
| FAIL-02 | Numeric hard returns current parse/promotion errors; numeric soft returns nil and skips without durable mutation for malformed literal, non-finite native numeric, and validator-rejected promotion. [CITED: .planning/REQUIREMENTS.md; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] | unit matrix | `go test ./... -run 'TestNumericFailureMode' -count=1` [ASSUMED test name] | partial; fixtures exist in `atomicity_test.go` and `gin_test.go`. [VERIFIED: atomicity_test.go:178-194; VERIFIED: gin_test.go:3328-3395] |
| FAIL-02 | Example demonstrates hard rejecting config and soft skipping config with predictable output. [CITED: .planning/ROADMAP.md; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] | example smoke | `go run ./examples/failure-modes/main.go` [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] | no; directory absent. [VERIFIED: find examples -maxdepth 2 -type f -name main.go] |

### Sampling Rate

- **Per task commit:** Run the focused command for touched layer plus `go test ./... -run 'TestAddDocumentPublicFailuresDoNotSetTragicErr' -count=1` when touching builder routing. [VERIFIED: atomicity_test.go:230-385]
- **Per wave merge:** Run `go test ./... -run 'Test(IngestFailureMode|FailureMode|RepresentationFailureMode|AddDocumentPublicFailuresDoNotSetTragicErr|AddDocumentAtomicityEncodeDeterminism)' -count=1`. [ASSUMED test names; VERIFIED: atomicity_test.go:129-167]
- **Phase gate:** Run `make test && make lint && go build ./...`; run `go run ./examples/failure-modes/main.go`. [VERIFIED: Makefile; CITED: .planning/ROADMAP.md]

### Wave 0 Gaps

- [ ] `failure_modes_test.go` - covers parser/transformer/numeric hard-soft matrix for FAIL-01 and FAIL-02. [ASSUMED file organization; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]
- [ ] `serialize_security_test.go` new legacy-token decode case - covers v9 compatibility for FAIL-01. [VERIFIED: serialize_security_test.go:587-614; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]
- [ ] `examples/failure-modes/main.go` - covers roadmap success criterion 4. [CITED: .planning/ROADMAP.md; VERIFIED: examples directory listing]
- [ ] `CHANGELOG.md` - covers FAIL-01 breaking migration note. [CITED: .planning/REQUIREMENTS.md; VERIFIED: no root changelog found]

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|------------------|
| V2 Authentication | no | No authentication surface is touched by this library phase. [VERIFIED: phase scope in .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] |
| V3 Session Management | no | No session surface is touched by this library phase. [VERIFIED: phase scope in .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] |
| V4 Access Control | no | No access-control surface is touched by this library phase. [VERIFIED: phase scope in .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] |
| V5 Input Validation | yes | Keep hard defaults, validate mode enums at options/config validation time, and preserve parser contract hard errors. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: gin.go:301-307; VERIFIED: builder.go:341-358] |
| V6 Cryptography | no | No cryptography is changed; existing zstd serialization and hashing are outside this phase's change surface. [VERIFIED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: go.mod] |

### Known Threat Patterns for Go Ingest Failure Routing

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Silent data loss through accidental soft defaults | Tampering / Repudiation | Default every layer to hard and require explicit soft options. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] |
| False negatives from partially indexed rejected documents | Tampering | Whole-document soft skip before durable merge and tests that no raw path/docID/bookkeeping state advances. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: builder.go:712-727; VERIFIED: atomicity_test.go:205-219] |
| Swallowing internal invariant panics as soft skips | Tampering / Denial of Service | Keep `runMergeWithRecover` tragic and never route it through soft failure modes. [VERIFIED: builder.go:730-761; VERIFIED: gin_test.go:540-570; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md] |
| Wire-format confusion from changed enum tokens | Tampering | Preserve v9 tokens or explicitly bump `Version` and add format tests. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: gin.go:17-31; VERIFIED: serialize.go:1590-1641] |
| Raw input leakage in logs while handling failures | Information Disclosure | Do not add logging for soft skips; existing tragic recovery tests already prevent raw panic value logging. [CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md; VERIFIED: gin_test.go:486-522] |

## Sources

### Primary (HIGH confidence)

- `.planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md` - locked Phase 17 decisions, scope, soft-skip semantics, serialization compatibility, tests, and examples. [CITED: file read]
- `.planning/REQUIREMENTS.md` - FAIL-01 and FAIL-02 requirements and Phase 18 boundary. [CITED: file read]
- `.planning/ROADMAP.md` - Phase 17 goal and success criteria. [CITED: file read]
- `.planning/PROJECT.md` and `.planning/STATE.md` - v1.2 strategy, priorities, and breaking-rename decision. [CITED: file read]
- `.planning/phases/16-adddocument-atomicity-lucene-contract/16-CONTEXT.md` and `16-LEARNINGS.md` - dependency contract and implementation lessons. [CITED: file read]
- `gin.go`, `builder.go`, `parser.go`, `parser_stdlib.go`, `parser_sink.go`, `transformer_registry.go`, `serialize.go` - current code touchpoints and behavior. [VERIFIED: source reads]
- `atomicity_test.go`, `parser_test.go`, `transformers_test.go`, `gin_test.go`, `serialize_security_test.go`, `phase07_review_test.go` - current behavior tests and reusable fixtures. [VERIFIED: source reads]
- `go.mod`, `Makefile`, `.planning/config.json`, `CLAUDE.md`, `AGENTS.md` - environment, project constraints, and validation setup. [VERIFIED: source reads]

### Secondary (MEDIUM confidence)

- Narrow test run on 2026-04-23: `go test ./... -run 'Test(AddDocumentDefaultParserErrorStringsPreserved|BuilderFailsWhenCompanionTransformFails|BuilderSoftFailSkipsCompanionWhenConfigured|AddDocumentRejectsUnsupportedNumberWithoutPartialMutation|MixedNumericPathRejectsLossyPromotionLeavesBuilderUsable|RepresentationFailureModeRoundTrip|AddDocumentPublicFailuresDoNotSetTragicErr)$' -count=1` passed. [VERIFIED: command output]
- Environment audit commands for Go, make, gotestsum, golangci-lint, govulncheck, rg, and awk. [VERIFIED: command output]
- Runtime-state inventory commands for git-tracked data, workflows, secrets/env files, OS registration configs, and build artifacts. [VERIFIED: command output]

### Tertiary (LOW confidence)

- None. [VERIFIED: sources above]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - no new dependencies are recommended, versions were verified from `go.mod`, `go list -m`, and local tools. [VERIFIED: go.mod; VERIFIED: go list -m; VERIFIED: environment audit]
- Architecture: HIGH - all routing touchpoints are verified in current source and constrained by Phase 17 context. [VERIFIED: builder.go; VERIFIED: gin.go; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]
- Pitfalls: HIGH - each pitfall maps to current code/tests or locked decisions. [VERIFIED: source reads; CITED: .planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md]

**Research date:** 2026-04-23
**Valid until:** 2026-05-23 for local code topology; re-run symbol and test discovery if Phase 17 is delayed past that date. [ASSUMED validity window]
