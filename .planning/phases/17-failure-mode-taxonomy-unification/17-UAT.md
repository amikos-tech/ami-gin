---
status: complete
phase: 17-failure-mode-taxonomy-unification
source: [17-01-SUMMARY.md, 17-02-SUMMARY.md, 17-03-SUMMARY.md, 17-04-SUMMARY.md]
started: 2026-04-23T18:13:40Z
updated: 2026-04-23T18:16:04Z
---

## Current Test
<!-- OVERWRITE each test - shows where we are -->

[testing complete]

## Tests

### 1. Public Failure-Mode API Surface
expected: Caller-facing failure configuration now uses `IngestFailureMode`, `IngestFailureHard`, and `IngestFailureSoft`; `WithParserFailureMode`, `WithNumericFailureMode`, and `WithTransformerFailureMode` all use that type; parser/numeric defaults are `hard`; and the old public transformer-only symbols are no longer the public API.
result: pass
evidence: `rg -n "type IngestFailureMode|IngestFailureHard|IngestFailureSoft|WithParserFailureMode|WithNumericFailureMode|WithTransformerFailureMode|ParserFailureMode|NumericFailureMode" gin.go transformer_registry.go` showed the unified type/options in `gin.go`, and the old public transformer-only type/constants were absent outside the preserved `WithTransformerFailureMode` option name.

### 2. Migration Note in CHANGELOG
expected: `CHANGELOG.md` contains an Unreleased breaking note mapping `TransformerFailureMode` / `TransformerFailureStrict` / `TransformerFailureSoft` to `IngestFailureMode` / `IngestFailureHard` / `IngestFailureSoft`, with a before/after `WithTransformerFailureMode(...)` snippet.
result: pass
evidence: `sed -n '1,40p' CHANGELOG.md` showed the Unreleased rename note and before/after snippet.

### 3. v9 Transformer Wire Compatibility
expected: Transformer metadata still round-trips legacy v9 `strict` / `soft_fail` wire tokens, unknown transformer failure-mode tokens are rejected, and parser/numeric failure-mode knobs remain absent from serialized config.
result: pass
evidence: `go test ./... -run 'Test(...RepresentationFailureMode|DecodeLegacyTransformerFailureModeTokens|ReadConfigRejectsUnknownTransformerFailureMode|TransformerFailureModeWireTokensStayV9...)$' -count=1` passed, and `rg -n 'const Version = 9|Version\\s*=\\s*9' gin.go` returned `31: Version = 9`.

### 4. Soft Whole-Document Skip Behavior
expected: With soft parser, numeric, or transformer modes enabled, failing documents are dropped as whole documents without partial staged data or row-group holes, retrying a skipped document keeps dense packing, and full-attempt vs clean-only corpora encode identically.
result: pass
evidence: `go test ./... -run 'Test(...ParserFailureMode|TransformerFailureMode|NumericFailureMode|BuilderSoftFailSkipsDocumentWhenConfigured|TransformerFailureModeSoftDiscardsPartiallyStagedDocument|SoftSkippedDocIDCanBeRetriedWithoutPositionConsumption|SoftFailureModesMatchCleanCorpus|AllSoftFailureModesSilentlyDropFailures...)$' -count=1` passed.

### 5. Deterministic Hard-vs-Soft Example
expected: Running `go run ./examples/failure-modes/main.go` prints the hard-stop error after 1 indexed document, then `soft: indexed 2 documents`, then `soft: email-domain example.com row groups [0 1]` exactly in that order.
result: pass
evidence: `go test ./examples/failure-modes -run 'TestFailureModesExampleOutput$' -count=1` passed and `go run ./examples/failure-modes/main.go` printed the expected three lines in order.

## Summary

total: 5
passed: 5
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
