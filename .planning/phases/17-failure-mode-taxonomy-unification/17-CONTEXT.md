# Phase 17: Failure-Mode Taxonomy Unification - Context

**Gathered:** 2026-04-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Provide one ingest failure-mode model across parser, transformer, and numeric layers. This phase replaces the transformer-only `TransformerFailureMode` API with a unified `IngestFailureMode` model, adds parser and numeric failure-mode config knobs, preserves hard-by-default behavior, and makes soft mode skip a failed document without returning an error.

This phase owns FAIL-01 and FAIL-02 only. It does not add the structured `IngestError` type or the experiment CLI grouped failure summary; those belong to Phase 18. It does not add `ValidateDocument`, snapshot/restore, performance work, telemetry counters, or parser implementation changes beyond failure routing.

Carrying forward from prior phases:
- Phase 16 made non-tragic ingest failures atomic by construction; soft skips must preserve that same "as if the failed call never happened" builder state.
- `tragicErr` remains reserved for internal invariant violations. Soft mode must never swallow a validator-missed merge panic or any other tragic condition.
- The deliberate `TransformerFailureMode` to `IngestFailureMode` rename is already locked by PROJECT.md and STATE.md as a breaking API change for clarity over convenience.
- Phase 18 owns `IngestError.Path`, `Layer`, `Cause`, and `Value`; Phase 17 should keep existing hard-mode error strings as stable as practical and only route them.

</domain>

<decisions>
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

### the agent's Discretion
- Exact internal helper names for the skip signal, normalization functions, and layer checks.
- Whether soft routing is represented as an internal sentinel error, an `AddDocument` control branch, or a small result type, as long as public behavior matches the decisions above.
- Exact test table organization and fixture names.
- Exact wording of hard-mode error messages until Phase 18 wraps them in `IngestError`.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Scope And Requirements
- `.planning/ROADMAP.md` - Phase 17 goal, dependency on Phase 16, and the four success criteria for unified failure modes.
- `.planning/REQUIREMENTS.md` - FAIL-01 and FAIL-02; confirms Phase 18 owns IERR-01 through IERR-03.
- `.planning/PROJECT.md` - v1.2 milestone strategy, deliberate breaking rename, correctness-first priority order, and out-of-scope list.
- `.planning/STATE.md` - Current v1.2 decisions and Phase 16 completion state.

### Prior Phase Context
- `.planning/phases/13-parser-seam-extraction/13-CONTEXT.md` - Parser seam rules, package-private `parserSink`, and the decision that AddDocument returns parser errors verbatim.
- `.planning/phases/15-experimentation-cli/15-CONTEXT.md` - Existing CLI `--on-error continue` behavior and dense row-group packing after skipped lines.
- `.planning/phases/16-adddocument-atomicity-lucene-contract/16-CONTEXT.md` - Atomicity contract, tragic vs public failure catalogs, and merge recovery boundary.
- `.planning/phases/16-adddocument-atomicity-lucene-contract/16-LEARNINGS.md` - Validator-backed commit pattern, public failure catalog guard, and encoded-byte oracle lessons.

### Current Code Anchors
- `gin.go:281` - Existing `TransformerFailureMode` type and constants to replace.
- `gin.go:323` - Existing `WithTransformerFailureMode` option to retarget to `IngestFailureMode`.
- `gin.go:352` - `GINConfig`, where parser and numeric failure-mode fields belong.
- `gin.go:655` - `DefaultConfig()`, where hard defaults should be set or normalized.
- `gin.go:750` - `GINConfig.validate()`, where mode validation belongs.
- `transformer_registry.go:47` - `TransformerSpec.FailureMode`, the serialized representation metadata field.
- `serialize.go:112` - `SerializedConfig`; parser/numeric modes should not be added unless planning deliberately changes format behavior.
- `serialize.go:1590` and `serialize.go:1641` - Config write/read paths and transformer failure-mode normalization.
- `builder.go:315` - `AddDocument`, the control point for parser soft skips and for preserving doc bookkeeping.
- `builder.go:533` - `stageCompanionRepresentations`, the transformer failure routing site.
- `builder.go:556` and `builder.go:587` - JSON/native numeric parse routing sites.
- `builder.go:730` and `builder.go:775` - Validator/commit boundary; numeric soft mode must happen before merge and must not swallow tragic recovery.
- `parser_stdlib.go:21` - Default parser ordinary error path.
- `parser.go:22` - Parser contract; sink contract violations are implementation bugs and stay hard.
- `transformers_test.go:538` - Existing transformer soft-fail behavior test to rewrite under unified soft document-skip semantics.
- `atomicity_test.go:230` - Public failure catalog tests to extend/reuse for per-layer hard/soft coverage.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- Existing transformer option plumbing in `gin.go` can be generalized: normalize/validate mode, resolve options, store mode on representation metadata.
- Existing Phase 16 atomicity helpers and failure catalog in `atomicity_test.go` provide ready-made fixtures for parser, transformer, and numeric failures.
- Existing experiment CLI skipped-line handling gives a precedent for accepted-document row-group packing after failures.

### Established Patterns
- Public config uses functional options and validates early.
- Builder ingest stages into `documentBuildState` before commit; skipped documents should exit before `mergeDocumentState` mutates durable state.
- Serialization is explicit and versioned; format changes require an intentional version-history update and tests.
- Error handling uses `github.com/pkg/errors`.

### Integration Points
- Public API changes land mainly in `gin.go` and `transformer_registry.go`.
- Parser soft routing integrates at `AddDocument` around `b.parser.Parse(...)`.
- Transformer soft routing integrates in `stageCompanionRepresentations(...)`.
- Numeric soft routing integrates in `stageJSONNumberLiteral(...)`, `stageNativeNumeric(...)`, and validator-rejected promotion paths.
- Documentation/example work lands in `CHANGELOG.md` and `examples/failure-modes/main.go`.

</code_context>

<specifics>
## Specific Ideas

- Keep the user-facing mental model simple: hard rejects and returns the failure; soft skips the failed document and returns nil.
- Prefer preserving binary v9 compatibility over changing serialized enum spelling in this phase.
- The example should make the behavior obvious from counts: hard config stops on the first bad document; soft config completes with only valid documents indexed.

</specifics>

<deferred>
## Deferred Ideas

- Structured `IngestError` with `Path`, `Layer`, `Cause`, and `Value` - Phase 18.
- Experiment CLI grouping by failure layer and structured samples - Phase 18.
- Telemetry counters for skipped documents or tragic events - future/on-demand.
- `ValidateDocument` dry-run API - future milestone with a real consumer.
- Snapshot/restore atomicity - reserve strategy only if validate-before-mutate stops being sufficient.

</deferred>

---

*Phase: 17-failure-mode-taxonomy-unification*
*Context gathered: 2026-04-23*
