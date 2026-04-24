---
phase: 18-structured-ingesterror-cli-integration
verified: 2026-04-24T14:46:00Z
status: passed
score: 16/16 must-haves verified
overrides_applied: 0
---

# Phase 18: Structured IngestError + CLI Integration Verification Report

**Phase Goal:** Structured `IngestError` + CLI integration for IERR-01, IERR-02, and IERR-03.
**Verified:** 2026-04-24T14:46:00Z
**Status:** passed
**Re-verification:** No - initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Root package exports `IngestLayer` with parser, transformer, numeric, and schema values. | VERIFIED | `ingest_error.go` defines `type IngestLayer string` and all four constants. |
| 2 | Root package exports concrete `*IngestError` carrying `Path`, `Layer`, `Value`, and cause via `Err`. | VERIFIED | `ingest_error.go` defines public fields and Go docs state `Value` is verbatim, not redacted, and not truncated. |
| 3 | `*IngestError` implements `Error()`, `Unwrap()`, and `Cause()` and is `errors.As` friendly. | VERIFIED | Methods exist; `TestIngestErrorWrappingContract` asserts `errors.As`, stdlib unwrap, `pkg/errors.Cause`, and exact message formatting. |
| 4 | Hard parser, transformer, numeric, and schema document failures return `*IngestError`. | VERIFIED | `builder.go` hard user-document failure sites call `newIngestError` / `newIngestErrorString`; behavior matrix covers all four layers. |
| 5 | Phase 16 numeric-promotion validator rejection is wrapped as `IngestLayerNumeric`. | VERIFIED | `validateStagedPaths` replays staged numeric observations through `stageNumericObservation`; `numeric_mixed_promotion` test exercises validator-replayed rejection. |
| 6 | Parser contract errors, tragic/internal builder errors, and Phase 17 soft-mode skips remain non-`IngestError`. | VERIFIED | Parser contract and soft-mode tests assert non-extraction; builder `AddDocument` keeps tragic and parser-contract branches as plain implementation errors. |
| 7 | Per-layer behavior is guarded against drift. | VERIFIED | `ingest_error_guard_test.go` parses `builder.go` and fails direct plain `errors.*` returns in named hard-ingest functions. |
| 8 | `gin-index experiment --on-error continue` groups returned `*gin.IngestError` failures by layer. | VERIFIED | `recordExperimentIngestFailure` extracts `*gin.IngestError` with `errors.As` and accumulates by `Layer`. |
| 9 | CLI failure groups are deterministically ordered parser, transformer, numeric, schema, then unknown lexically. | VERIFIED | `experimentIngestLayerRank` pins known layer order; `TestExperimentIngestFailureGroupsDeterministic` asserts ordering. |
| 10 | CLI failure samples include line, input_index, path, value, and message with at most 3 samples per layer. | VERIFIED | `experimentFailureSample` has the required fields; recorder caps samples at `experimentFailureSampleLimit = 3`; JSON tests assert the cap and fields. |
| 11 | JSON output remains a single report object. | VERIFIED | `experimentReport` still has top-level `source`, `summary`, `paths`, and optional `predicate_test`; failures are nested under `summary.failures`. |
| 12 | Accepted documents remain densely packed; failed lines do not consume row-group or document positions. | VERIFIED | `buildExperimentIndex` uses `result.ingestedDocs/rgSize` before incrementing accepted documents; 100-line fixture asserts 90 documents and 9 row groups. |
| 13 | Deterministic 100-line fixture asserts 3 parser, 4 transformer, and 3 numeric failures. | VERIFIED | `TestRunExperimentHundredDocsKnownIngestFailuresJSON` checks counts, order, sample caps, messages, `documents == 90`, `error_count == 10`, and `row_groups == 9`. |
| 14 | Public API docs state `IngestError.Value` is verbatim and caller-redacted; CLI samples are bounded by count, not truncation. | VERIFIED | `ingest_error.go` doc comment states no redaction/truncation; CLI recorder comment states report growth is bounded by sample count. |
| 15 | Changelog documents exported `IngestError`, fields, layers, verbatim value policy, and CLI grouped summaries. | VERIFIED | `CHANGELOG.md` Unreleased entry covers `IngestError`, `Path`, `Layer`, `Value`, `Err`, layer names, and CLI text/JSON grouped summaries. |
| 16 | Focused Phase 18 tests, full test, and lint evidence are recorded. | VERIFIED | `18-VALIDATION.md` records focused root tests, focused CLI tests, `go test ./...`, and `make lint` as PASS. Focused checks were rerun during verification. |

**Score:** 16/16 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `ingest_error.go` | Exported structured ingest error API and helper formatting | VERIFIED | Defines `IngestLayer`, `IngestError`, `Error`, `Unwrap`, `Cause`, constructors, and `formatStagedNumericValue`. |
| `builder.go` | Hard ingest failure wrapping at parser, transformer, numeric, schema, and validator rejection sites | VERIFIED | Hard document failure branches route through `newIngestError*`; non-document parser contract/tragic paths remain outside `IngestError`. |
| `failure_modes_test.go` | Behavior matrix and exception coverage | VERIFIED | Covers parser, transformer, numeric malformed literal, numeric mixed promotion, schema unsupported token, contract exceptions, and soft-mode exceptions. |
| `ingest_error_guard_test.go` | Low-noise AST guard for current hard-ingest functions | VERIFIED | Parses `builder.go`; scans seven named hard-ingest functions for direct plain `errors.*` returns. |
| `cmd/gin-index/experiment.go` | Failure aggregation during JSONL ingest | VERIFIED | Records continue-mode `*gin.IngestError` failures and converts groups deterministically. |
| `cmd/gin-index/experiment_output.go` | Text and JSON failure group output | VERIFIED | Adds `summary.failures`, sample/group structs, and text `Failures:` rendering. |
| `cmd/gin-index/experiment_test.go` | CLI grouped failure and 100-line fixture tests | VERIFIED | Tests text output, JSON output, deterministic ordering, sample caps, and 100-line known failure counts. |
| `CHANGELOG.md` | Release-facing note | VERIFIED | Unreleased entry documents API, fields, layer values, value policy, and CLI summaries. |
| `18-VALIDATION.md` | Final validation evidence | VERIFIED | Contains `nyquist_compliant: true`, green rows, and exact command PASS results. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `builder.go AddDocument` | `ingest_error.go IngestError` | Hard parser branch returns `newIngestErrorString(IngestLayerParser, "", string(jsonDoc), err)` | WIRED | Parser soft skip and stage-callback unwrapping happen before hard parser wrapping. |
| `builder.go stageCompanionRepresentations` | `ingest_error.go IngestError` | Transformer failure returns `newIngestError(IngestLayerTransformer, canonicalPath, value, ...)` | WIRED | Uses source canonical path and does not expose derived paths. |
| `builder.go stageJSONNumberLiteral/stageNativeNumeric/stageNumericObservation` | `ingest_error.go IngestError` | Numeric parse and promotion failures return `IngestLayerNumeric` | WIRED | Raw numeric literals and staged numeric formatting preserve doc-facing strings. |
| `builder.go stageScalarToken/stageMaterializedValue` | `ingest_error.go IngestError` | Unsupported schema/value shapes return `IngestLayerSchema` | WIRED | Unsupported token and transformed value defaults are structured. |
| `builder.go validateStagedPaths` | Numeric promotion guard | Replays staged observations through `stageNumericObservation` before merge | WIRED | Existing committed path data is seeded into the preview, so validator-rejected mixed promotion returns numeric `IngestError`. |
| `cmd/gin-index/experiment.go buildExperimentIndex` | `experimentSummary.Failures` | `recordExperimentIngestFailure` and deterministic group conversion | WIRED | Continue mode records failures before skipping the bad line; abort mode does not emit grouped summaries. |
| `cmd/gin-index/experiment_output.go` | CLI text and JSON reports | `Failures []experimentFailureGroup` under `summary` plus `Failures:` text section | WIRED | JSON remains one report object; text shows layer counts and sample fields. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `builder.go` | Returned `*IngestError` fields | Actual parser/staging/validator failures in `AddDocument` and staging functions | Yes | FLOWING |
| `cmd/gin-index/experiment.go` | `result.ingestFailures` | Real `lineErr` from `validateExperimentRecord(builder, record, result.ingestedDocs/rgSize)` | Yes | FLOWING |
| `cmd/gin-index/experiment_output.go` | `report.Summary.Failures` | `experimentIngestFailureGroups(result.ingestFailures)` | Yes | FLOWING |
| `cmd/gin-index/experiment_test.go` | JSON/text failure assertions | `runExperiment` over generated JSONL fixtures | Yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Root structured error behavior and guard tests | `go test ./... -run 'Test(IngestErrorWrappingContract|HardIngestFailuresReturnIngestError|ParserContractErrorsRemainNonIngestError|SoftFailureModesDoNotReturnIngestError|HardIngestFunctionsDoNotReturnPlainErrors)$' -count=1` | exit 0 | PASS |
| CLI grouped failure tests | `go test ./cmd/gin-index -run 'TestRunExperiment(OnErrorContinue|OnErrorContinueMalformedJSONFromFile|OnErrorContinueIngestFailuresJSON|HundredDocsKnownIngestFailuresJSON|OnErrorAbort.*Ingest)' -count=1` | exit 0 | PASS |
| Anti-pattern scan | `rg` over modified source/test/docs for TODO/FIXME/placeholders/stub markers | no matches | PASS |
| Prohibited host/internal string scan | `rg` over Phase 18 artifacts and modified files | no matches | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| IERR-01 | 18-01, 18-04 | Exported `IngestError` carries `Path`, `Layer`, cause, and verbatim `Value`; `errors.As` extraction tested. | SATISFIED | API and docs in `ingest_error.go`; wrapping contract test asserts extraction and cause behavior. |
| IERR-02 | 18-01, 18-02, 18-04 | All ingest-error sites wrap underlying errors in `IngestError`; per-layer matrix and guard coverage. | SATISFIED | Builder hard sites are structured; tests cover parser, transformer, numeric, schema, soft, contract, and guard cases. |
| IERR-03 | 18-03, 18-04 | `gin-index experiment --on-error continue` reports failures grouped by layer with structured samples in text and JSON; golden-tested. | SATISFIED | CLI aggregator/output code is wired; text, JSON, deterministic order, and 100-line fixture tests exist and pass. |

No orphaned Phase 18 requirement IDs were found in `.planning/REQUIREMENTS.md`; IERR-01, IERR-02, and IERR-03 are all claimed by plans and verified.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None | - | - | - | No placeholder/stub/TODO anti-patterns found in Phase 18 modified files. |

### Human Verification Required

None. The phase goal is API, error-wiring, CLI report shape, docs, and tests; all are programmatically verifiable and covered by focused checks.

### Gaps Summary

No gaps found. Phase 18 achieves the roadmap success criteria and requirement contracts for IERR-01, IERR-02, and IERR-03.

---

_Verified: 2026-04-24T14:46:00Z_
_Verifier: Claude (gsd-verifier)_
