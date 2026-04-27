# Phase 18: Structured IngestError + CLI integration - Context

**Gathered:** 2026-04-24
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 18 makes returned per-document ingest failures actionable. It adds an exported structured `IngestError` for hard document failures and surfaces those failures in the existing `gin-index experiment --on-error continue` text and JSON reports.

This phase does not change Phase 17 soft-mode semantics, does not add a `ValidateDocument` dry-run API, does not add snapshot/restore behavior, and does not introduce performance work.

</domain>

<decisions>
## Implementation Decisions

### Public IngestError API
- **D-01:** Add an exported concrete `*IngestError` type with public fields `Path`, `Layer`, `Value`, and `Err`.
- **D-02:** `Err` is the underlying cause field. Do not name the field `Cause`, because the type also exposes `Cause() error` for `github.com/pkg/errors` compatibility.
- **D-03:** `IngestError` implements `Error() string`, `Unwrap() error`, and `Cause() error`; callers must be able to extract it with `errors.As` from anywhere in the wrap chain.
- **D-04:** Add an exported `IngestLayer` string-like taxonomy with the layer values `parser`, `transformer`, `numeric`, and `schema`.
- **D-05:** Do not add per-layer sentinel errors or `errors.Is` classification in this phase. The stable classification surface is `IngestError.Layer`.

### Layer, Path, And Value Semantics
- **D-06:** Use best-known user-location semantics. `Path` is the canonical JSONPath when the failing site knows it.
- **D-07:** Parser failures that happen before a stable JSONPath is known use an empty `Path`. Use `$` only for known root-level failures, not as a placeholder for "unknown".
- **D-08:** `Value` is a verbatim string representation of the offending input or value at the error site. The library does not redact; callers own redaction.
- **D-09:** Parser contract errors such as missing `BeginDocument`, multiple `BeginDocument` calls, or row-group ID mismatch remain hard non-`IngestError` implementation errors. They are not user document failures.
- **D-10:** Transformer failures should report the source user path rather than the derived representation target path. Internal representation paths should not leak into the public failure location unless the source user path is unavailable.

### CLI Failure Reporting
- **D-11:** `gin-index experiment --on-error continue` reports returned hard `IngestError`s grouped by `layer` only. Do not introduce a failure-code enum in Phase 18.
- **D-12:** Each layer group keeps at most 3 structured samples.
- **D-13:** Each sample includes line information and structured ingest fields where available: `line`, `input_index`, `path`, `value`, and `message`.
- **D-14:** The CLI keeps the existing single-object JSON report shape. Do not switch to NDJSON/event-stream output and do not introduce SARIF-style diagnostics.
- **D-15:** Failed-line samples must not be assigned row-group/doc positions that would disturb dense packing of accepted documents.

### Enforcement And Test Matrix
- **D-16:** Use a scoped guard plus table-driven behavior tests. Avoid a broad repo grep because non-ingest subsystems legitimately use plain `errors.New` / `errors.Wrap`.
- **D-17:** Scope the static guard to ingest surfaces only: builder ingest paths, parser ingest paths, and the experiment CLI ingest-validation flow.
- **D-18:** The guard must explicitly allow tragic/internal builder errors and parser contract errors to remain non-`IngestError`.
- **D-19:** Add per-layer tests for parser, transformer, numeric, and schema/unsupported-value hard document failures. Each test asserts `errors.As` extraction and that `Path`, `Layer`, `Value`, and `Err` are populated according to the locked semantics.
- **D-20:** Add CLI tests for grouped layer counts, 3-sample caps, and structured sample fields in both text and JSON modes.

### Soft Mode Boundary
- **D-21:** Phase 18 is hard-error-only. `IngestError` is for returned hard per-document failures.
- **D-22:** Preserve Phase 17 soft semantics: configured soft failures skip and return nil.
- **D-23:** Existing soft counters/logs remain the soft-mode surface. Do not add CLI soft-skip samples, a library soft-skip observer API, or a changed soft return contract in this phase.

### the agent's Discretion
- Exact formatting of `IngestError.Error()` as long as it is Go-idiomatic, stable, and includes enough context for humans without replacing structured fields.
- Exact helper names for wrapping ingest failures and formatting `Value`.
- Exact JSON field nesting for CLI failure groups, as long as it preserves the layer-only grouping, 3-sample cap, and structured sample fields.
- Exact implementation of the scoped guard: Makefile `rg`/`awk`, a focused test, or equivalent, as long as it is low-noise and scoped to ingest surfaces.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Scope And Requirements
- `.planning/ROADMAP.md` - Phase 18 goal and success criteria for exported `IngestError`, full ingest-site wrapping, and CLI grouped reporting.
- `.planning/REQUIREMENTS.md` - IERR-01, IERR-02, and IERR-03 define the remaining v1.2 requirements.
- `.planning/PROJECT.md` - v1.2 milestone strategy, correctness-first priority order, non-redacted `IngestError.Value`, and out-of-scope list.
- `.planning/STATE.md` - Current project state and accumulated v1.2 decisions.

### Prior Phase Context
- `.planning/phases/16-adddocument-atomicity-lucene-contract/16-CONTEXT.md` - Atomicity contract, tragic vs public failure catalogs, merge recovery boundary, and the requirement that user-input failures do not close the builder.
- `.planning/phases/17-failure-mode-taxonomy-unification/17-CONTEXT.md` - `IngestFailureMode` hard/soft taxonomy, per-layer failure routing, whole-document soft-skip semantics, and deferred Phase 18 error work.
- `.planning/phases/15-experimentation-cli/15-CONTEXT.md` - Existing `gin-index experiment` JSONL ingest, `--on-error continue|abort`, JSON/text report model, and dense packing after skipped lines.
- `.planning/phases/13-parser-seam-extraction/13-CONTEXT.md` - Parser seam contract, parser sink behavior, and current parser error routing.

### Current Code Anchors
- `builder.go` - `AddDocument`, `stageCompanionRepresentations`, numeric staging, soft skip helpers, tragic error handling, and commit boundary.
- `parser.go` and `parser_stdlib.go` - Parser contract and ordinary parser failure paths.
- `gin.go` - `IngestFailureMode`, config options, default hard modes, and validation patterns.
- `cmd/gin-index/experiment.go` - JSONL ingest loop, `--on-error continue|abort`, line processing, dense row-group packing, and build/finalize flow.
- `cmd/gin-index/experiment_output.go` - Existing text and JSON report structures to extend with failure groups.
- `failure_modes_test.go` and `atomicity_test.go` - Existing per-layer failure fixtures and atomicity/public failure catalog coverage.
- `Makefile` - Existing scoped enforcement pattern for validator markers; use as precedent for a low-noise guard.
- `CHANGELOG.md` - Existing Unreleased failure-mode notes; Phase 18 may need an additive note for the new structured error surface.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `softSkipDocumentError` and `stageCallbackError` in `builder.go` already implement `Error()`, `Unwrap()`, and `Cause()`, which is the local pattern for error types compatible with both Go wrapping and `github.com/pkg/errors`.
- Existing `IngestFailureMode` and hard/soft config plumbing in `gin.go` provide the layer-routing context that `IngestError.Layer` should align with.
- Existing CLI report structs in `cmd/gin-index/experiment_output.go` can be extended without changing the single-object JSON report style.
- Existing `failure_modes_test.go` and `atomicity_test.go` fixtures already exercise parser, transformer, numeric, soft skip, and public failure catalog behavior.

### Established Patterns
- The repo uses `github.com/pkg/errors` for new errors and wrapping; public error types should remain compatible with `errors.As`, `errors.Unwrap`, and `pkg/errors.Cause`.
- Ingest stages into per-document state before durable merge. Hard document failures return before mutating durable builder state; tragic/internal failures close the builder.
- CLI `experiment` keeps accepted documents densely packed into synthetic row groups and treats rejected input lines separately from accepted document positions.
- Static policy checks should be scoped and low-noise, following the existing `check-validator-markers` Makefile precedent rather than adding broad lint churn.

### Integration Points
- Add the public `IngestError` and `IngestLayer` API in the root package.
- Wrap hard parser, transformer, numeric, and schema/unsupported-value document failures at the sites that currently return plain errors.
- Preserve parser contract and tragic/internal failures outside `IngestError`.
- Extend experiment build results and reports with failure groups and samples.
- Add focused guard/test coverage for ingest surfaces and CLI JSON/text golden behavior.

</code_context>

<specifics>
## Specific Ideas

- Keep the API small: `Layer` is enough classification for Phase 18; do not add failure codes or sentinel errors until a real caller needs finer machine routing.
- Prefer truthful missing data over fake precision: parser failures with no known path should leave `Path` empty, not pretend the failure happened at `$`.
- CLI samples should help debugging without becoming a second API taxonomy: line, input index, path, value, and message are enough.
- Soft mode should remain conceptually separate from returned hard failures. Mixing nil-on-soft with returned `IngestError` would make the failure API harder to reason about.

</specifics>

<deferred>
## Deferred Ideas

- Failure-code enum for CLI/API grouping - defer until callers need machine routing finer than `Layer`.
- `errors.Is` layer sentinels such as `ErrIngestNumeric` - defer; `IngestError.Layer` is the Phase 18 classification surface.
- Library soft-skip observer or sampled soft-skip event API - defer; existing counters/logs remain the surface.
- CLI soft-skip samples - defer to a future explicit observability/auditability phase.
- `ValidateDocument` dry-run API - remains future milestone scope.

</deferred>

---

*Phase: 18-structured-ingesterror-cli-integration*
*Context gathered: 2026-04-24*
