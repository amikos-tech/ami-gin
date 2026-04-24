# Phase 18: Structured IngestError + CLI integration - Research

**Researched:** 2026-04-24
**Status:** Complete

## Executive Summary

Phase 18 should be implemented as a small public error API plus targeted rewiring of hard ingest failures and the existing experiment report model. The safest path is to avoid changing Phase 17 soft-mode behavior and to keep existing parser contract/tragic failures outside `IngestError`.

The implementation should split into three work areas:

1. Add exported `IngestLayer` constants and `*IngestError` in the root `gin` package, with `Error()`, `Unwrap()`, and `Cause()`.
2. Wrap user-document hard ingest failures at their source or at the builder boundary with the best-known `Path`, `Layer`, `Value`, and underlying `Err`.
3. Extend `gin-index experiment --on-error continue` to aggregate returned `*gin.IngestError` values by layer and expose bounded samples in text and JSON output.

## Current Code Map

| Area | Files | Current behavior |
|------|-------|------------------|
| Builder entrypoint | `builder.go` | `AddDocument` calls `b.parser.Parse`, unwraps `stageCallbackError`, soft-skips configured parser failures, then validates parser contract and merges staged state. Hard parser errors currently return verbatim. |
| Parser implementation | `parser.go`, `parser_stdlib.go`, `parser_sink.go` | Parser errors are built with `github.com/pkg/errors`. Sink callback failures are tagged with `stageCallbackError` so AddDocument does not incorrectly classify numeric/transformer errors as parser errors. |
| Transformer failures | `builder.go` | `stageCompanionRepresentations` returns plain formatted errors for hard transformer failure and skips only the companion representation under soft transformer mode. |
| Numeric failures | `builder.go` | `stageJSONNumberLiteral`, `stageNativeNumeric`, and `stageNumericObservation` return hard errors or `softSkipDocumentError` depending on `NumericFailureMode`. `validateStagedPaths` can surface numeric promotion failures before merge mutation. |
| Schema/unsupported values | `builder.go` | Unsupported scalar/materialized/transformed values return plain formatted errors from staging paths. These are user-document hard failures and should be classified as `schema`. |
| CLI experiment | `cmd/gin-index/experiment.go`, `cmd/gin-index/experiment_output.go` | `buildExperimentIndex` logs `line N: err`, increments skipped/error counts, and keeps accepted docs densely packed. JSON report is a single object with `source`, `summary`, `paths`, and optional `predicate_test`. |
| Existing tests | `failure_modes_test.go`, `atomicity_test.go`, `cmd/gin-index/experiment_test.go` | Good fixtures already exist for parser, transformer, numeric, unsupported token, soft mode, dense packing, and CLI continue/abort JSON/text outputs. |
| Static guard precedent | `Makefile` | `check-validator-markers` uses scoped awk validation and is already wired into `make lint`. Phase 18 should add similarly scoped enforcement rather than broad repo grep. |

## Implementation Findings

### Public API

Add a focused root-package API:

```go
type IngestLayer string

const (
	IngestLayerParser      IngestLayer = "parser"
	IngestLayerTransformer IngestLayer = "transformer"
	IngestLayerNumeric     IngestLayer = "numeric"
	IngestLayerSchema      IngestLayer = "schema"
)

type IngestError struct {
	Path  string
	Layer IngestLayer
	Value string
	Err   error
}

func (e *IngestError) Error() string
func (e *IngestError) Unwrap() error
func (e *IngestError) Cause() error
```

`Err` is intentionally the field name, not `Cause`, because the method `Cause() error` is required for compatibility with `github.com/pkg/errors.Cause`.

Recommended location: create `ingest_error.go` in the root package to keep public error API isolated from builder internals.

### Wrapping Strategy

Use small internal helpers rather than open-coded struct construction at every site. The helpers should be unexported and keep call sites concrete:

- `newIngestError(layer IngestLayer, path string, value any, err error) error`
- `newIngestErrorString(layer IngestLayer, path string, value string, err error) error` if preserving raw numeric text is easier.
- `wrapParserIngestError(jsonDoc []byte, err error) error` for unknown-location parser failures.

Value formatting must be deterministic and not redact. For raw JSON input or numeric literals, use the verbatim string available at the error site. For Go values, `fmt.Sprint(value)` is sufficient unless a test needs `%#v` precision for a specific unsupported type. Avoid JSON marshaling for `Value`; it can fail or normalize values.

### Parser Layer

Parser failures that happen before a stable path is known should become:

- `Layer: IngestLayerParser`
- `Path: ""`
- `Value: string(jsonDoc)`
- `Err: original parser error`

Keep parser contract errors non-`IngestError`:

- missing `BeginDocument`
- multiple `BeginDocument` calls
- `BeginDocument` row-group mismatch

The current `AddDocument` parser-error branch is the right boundary for wrapping ordinary parser errors. It already distinguishes `stageCallbackError` before parser soft handling. Preserve that ordering:

1. `isSkipDocument`
2. `isStageCallbackError` -> unwrap stage callback and return it unchanged
3. parser soft mode -> nil skip
4. hard parser -> wrap as `IngestError`

### Transformer Layer

Hard transformer failures originate in `stageCompanionRepresentations` when a registered transformer returns `ok == false`.

Use source user path semantics:

- `Path: canonicalPath`
- `Layer: IngestLayerTransformer`
- `Value: fmt.Sprint(prepared)` or source `value`
- `Err: errors.Errorf("companion transformer %q failed to produce a value", registration.Alias)` or equivalent cause

Do not expose `registration.TargetPath` in `Path`. Phase 18 explicitly says internal representation paths should not leak unless source path is unavailable.

Soft transformer mode remains unchanged: increment `numSoftRepresentationSkips`, log, continue, and return nil.

### Numeric Layer

Numeric hard failures have three main source groups:

- malformed JSON number literal in `stageJSONNumberLiteral`
- unsupported/non-finite native numeric in `stageNativeNumeric`
- unsupported mixed numeric promotion in `stageNumericObservation` and `validateStagedPaths`

All hard numeric document failures should be `IngestLayerNumeric`, with the canonical JSONPath and a useful value string:

- raw JSON number literal: `Value` is the raw literal
- native numeric: `Value` is `fmt.Sprint(value)`
- mixed promotion: `Value` can be the staged numeric value string that triggered promotion, or an empty string only if the trigger value is not available. Prefer adding a formatter for `stagedNumericValue`.

Soft numeric mode must continue returning `newSoftSkipNumericDocumentError(path)` and not `IngestError`.

### Schema Layer

Phase context includes `schema` as a layer even though current code has no external schema system. The practical Phase 18 mapping is "unsupported value/type shape during ingest":

- `stageScalarToken` default branch: unsupported JSON token type
- `stageMaterializedValue` default branch: unsupported transformed value type
- unsupported value returned by a transformer and routed into `stageMaterializedValue`

These should become `IngestLayerSchema`. Tests can use `unsupportedTokenAtomicityParser` or a custom transformer returning an unsupported type to exercise this layer.

### CLI Reporting

Extend the existing single-object report. Recommended JSON shape:

```json
{
  "summary": {
    "error_count": 10,
    "failures": [
      {
        "layer": "parser",
        "count": 3,
        "samples": [
          {
            "line": 2,
            "input_index": 1,
            "path": "",
            "value": "not-json",
            "message": "read JSON token: invalid character ..."
          }
        ]
      }
    ]
  }
}
```

This preserves the single-object report shape and keeps failure diagnostics attached to `summary`, where `error_count` and `skipped_lines` already live. `input_index` should be zero-based source record index (`lineNumber - 1`) so failed input lines do not consume dense accepted-document positions.

Text output should add a compact section only when failures exist:

```text
  Failures:
    parser: 3
      line 2 input_index 1 path "" value "not-json": read JSON token: ...
    transformer: 4
    numeric: 3
```

Keep at most 3 samples per layer. Continue printing the existing `line N: err` messages to stderr for compatibility with current tests and user feedback.

### 100-Document Test Fixture

The roadmap asks for 100 docs with 10 known failures: 3 parser, 4 transformer, 3 numeric. The experiment CLI currently uses `DefaultConfig()`, so transformer failures will not occur unless the test can inject configuration. Options:

1. Add a package-level overridable variable in `cmd/gin-index/experiment.go`:
   `var experimentDefaultConfig = gin.DefaultConfig`
   and call that from `experimentConfigForLogLevel`.
2. In tests, temporarily set `experimentDefaultConfig` to return a config with a strict transformer and numeric failure mode hard.

This keeps production flags unchanged and avoids adding hidden CLI config surface just for tests.

Recommended fixture:

- 90 valid docs with `email` string and safe numeric `score`
- 3 parser failures: `not-json`
- 4 transformer failures: valid JSON with `email` numeric under `WithEmailDomainTransformer("$.email", "domain")`
- 3 numeric failures: seed one safe large integer path then use float on same path to trigger mixed promotion, or use custom parser/test helper if direct CLI fixture cannot create all numeric cases naturally

Because the CLI path should remain realistic, prefer malformed numeric JSON or mixed-promotion JSONL records that the default parser can emit. The seed-large-int plus float strategy exercises real `buildExperimentIndex`.

### Static Guard

Do not add a broad repo-wide ban on `errors.New` or `errors.Wrap`. It will create noise in configuration, decode, query, and CLI code that is outside ingest.

Recommended guard choices:

- Add a `TestHardIngestFailuresReturnIngestErrorMatrix` table that covers all required sites behaviorally.
- Add a Makefile target `check-ingest-error-wrapping` that scans a narrow set of files and functions for obvious plain returns from ingest surfaces.

The Makefile target should be conservative:

- scan `builder.go`, `parser_stdlib.go`, `parser_sink.go`, and `cmd/gin-index/experiment.go`
- permit known internal/tragedy/parser-contract messages such as `builder closed by prior tragic failure`, `did not call BeginDocument`, `called BeginDocument`, `BeginDocument rgID mismatch`, `builder tragic`, and `errExperimentAbort`
- fail only on disallowed patterns in known hard-ingest function bodies, not every error in the file

If the guard becomes brittle, a focused Go test that enumerates hard ingest failures is preferable to a noisy awk rule.

## Risks And Mitigations

| Risk | Why it matters | Mitigation |
|------|----------------|------------|
| Wrapping stage callback errors as parser errors | Would misclassify numeric/transformer/schema failures returned through parser sink callbacks. | Preserve `isStageCallbackError` branch before parser wrapping. Add tests using parser sink fixtures. |
| Accidentally changing soft-mode behavior | Phase 17 semantics are locked: soft parser/numeric skips return nil; soft transformer skips only companion representation. | Add regression tests that soft failures do not return `IngestError` and existing soft counters still behave. |
| Leaking internal representation paths | Public `Path` would expose `__derived:` targets and confuse callers. | Transformer hard-error tests must assert source path such as `$.email`, not target path. |
| CLI sample positions disturb dense packing | Failed lines must not consume accepted document row-group/doc positions. | Keep `rgID` derived from `result.ingestedDocs/rgSize`; store failed `input_index` separately from accepted doc position. Add dense packing assertion. |
| Value disclosure surprise | This phase explicitly does not redact. | Document in API comments and CHANGELOG that `Value` is verbatim and callers own redaction. |
| Static guard false positives | Broad grep would fail on legitimate internal errors. | Scope guard tightly and rely on behavior matrix for correctness. |

## Validation Architecture

> Nyquist validation plan enabled: `.planning/config.json` has `workflow.nyquist_validation` set to true.

| Property | Value |
|----------|-------|
| Framework | Go `testing` |
| Config file | `go.mod` |
| Quick run command | `go test ./... -run 'Test(IngestError|HardIngestFailures|RunExperiment.*IngestFailure|CheckIngestErrorWrapping)' -count=1` |
| Full suite command | `go test ./...` and `make lint` |
| Estimated runtime | ~20-60 seconds for focused tests, project-dependent for full suite |

### Required Automated Tests

| Requirement | Test coverage |
|-------------|---------------|
| IERR-01 | Unit tests for `IngestError.Error`, `Unwrap`, `Cause`, and `errors.As` through an outer `errors.Wrap`. |
| IERR-02 | Table-driven hard-ingest matrix for parser, transformer, numeric, and schema layers asserting `Path`, `Layer`, `Value`, and non-nil `Err`. Include parser contract tests asserting no `IngestError`. |
| IERR-03 | CLI text and JSON tests for layer counts, 3-sample cap, structured sample fields, and 100-doc/10-failure fixture with 3 parser, 4 transformer, 3 numeric failures. |

### Suggested Test Commands

```bash
go test ./... -run 'Test(IngestError|HardIngestFailuresReturnIngestError|ParserContractErrorsRemainNonIngestError|SoftFailureModesDoNotReturnIngestError)$' -count=1
go test ./cmd/gin-index -run 'TestRunExperiment(OnErrorContinue.*IngestFailure|JSONReportsGroupedIngestFailures|TextReportsGroupedIngestFailures|HundredDocsKnownIngestFailures)$' -count=1
make lint
go test ./...
```

## Planning Recommendation

Use four implementation plans:

1. Public `IngestError` API and builder wrapping for parser/transformer/numeric/schema hard failures.
2. Per-layer behavior matrix and scoped wrapping guard.
3. CLI aggregation/reporting in text and JSON, including 100-doc fixture and dense packing checks.
4. Docs/changelog and final full-suite verification.

This decomposition keeps the public library API independently testable before the CLI starts depending on it.

## RESEARCH COMPLETE
