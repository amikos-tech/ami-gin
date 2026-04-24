---
status: complete
phase: 18-structured-ingesterror-cli-integration
source: [18-01-SUMMARY.md, 18-02-SUMMARY.md, 18-03-SUMMARY.md, 18-04-SUMMARY.md]
started: 2026-04-24T14:09:59Z
updated: 2026-04-24T14:21:14Z
---

## Current Test
<!-- OVERWRITE each test - shows where we are -->

[testing complete]

## Tests

### 1. Public IngestError API Surface
expected: A hard per-document ingest failure can be extracted as `*gin.IngestError` via `errors.As`, and the extracted value exposes `Path`, `Layer`, `Value`, and `Err`. The human-readable error includes the layer and path, and the underlying cause is still reachable through stdlib/pkg-errors unwrapping.
result: pass
evidence: `go test ./... -run 'Test(IngestErrorWrappingContract|HardIngestFailuresReturnIngestError|ParserContractErrorsRemainNonIngestError|SoftFailureModesDoNotReturnIngestError|HardIngestFunctionsDoNotReturnPlainErrors)$' -count=1` passed, and `ingest_error.go` exports `IngestLayer{Parser,Transformer,Numeric,Schema}`, `IngestError`, and `Error`/`Unwrap`/`Cause` with the verbatim-value policy documented in the Godoc block.

### 2. Continue-Mode Text Failure Summary
expected: Running `gin-index experiment --on-error continue` on mixed valid and invalid input completes instead of aborting and prints a `Failures:` section grouped by layer. Each group shows a count and bounded sample lines with `line`, `input_index`, `path`, `value`, and the structured message.
result: pass
evidence: `go test ./cmd/gin-index -run 'TestRunExperiment(OnErrorContinueIngestFailuresText|OnErrorContinueIngestFailuresJSON|HundredDocsKnownIngestFailuresJSON)$' -count=1` passed, and a live `go run ./cmd/gin-index experiment --on-error continue <mixed-jsonl>` run completed with `Status: partial`, `Documents: 4`, `Skipped Lines: 1`, and a `Failures:` block showing `parser: 1` plus `line 4 input_index 3 ... value "{bad json"`.

### 3. Continue-Mode JSON Failure Summary
expected: Running the same experiment with `--json` returns one report object whose `summary.failures` field groups hard ingest failures in deterministic order (`parser`, `transformer`, `numeric`, `schema`, then lexical unknowns), with counts and at most 3 samples per layer, while accepted-document counts and row-group counts still reflect only successfully indexed documents.
result: pass
evidence: `go test ./cmd/gin-index -run 'TestRunExperiment(OnErrorContinueIngestFailuresText|OnErrorContinueIngestFailuresJSON|HundredDocsKnownIngestFailuresJSON)$' -count=1` passed, including the deterministic 100-line fixture coverage, and a live `go run ./cmd/gin-index experiment --json --on-error continue <mixed-jsonl>` run returned one report object with `summary.failures[0].layer == "parser"`, `count == 1`, one structured sample, `documents == 4`, and `row_groups == 4`.

### 4. Public Docs and Release Note
expected: The public docs state that `IngestError` is exported, document the `parser`/`transformer`/`numeric`/`schema` layer values, and explicitly say that `Value` is verbatim and not redacted or truncated by the library so callers own redaction and output-size policy. The same policy appears in the Unreleased changelog entry.
result: pass
evidence: `rg -n 'IngestLayerParser|IngestLayerTransformer|IngestLayerNumeric|IngestLayerSchema|Value is a verbatim' ingest_error.go` returned the exported layer constants and verbatim-value Godoc, and `rg -n 'IngestError|parser|transformer|numeric|schema|verbatim|redacted|truncated' CHANGELOG.md` returned the Unreleased note documenting the same public policy and grouped CLI summaries.

## Summary

total: 4
passed: 4
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
