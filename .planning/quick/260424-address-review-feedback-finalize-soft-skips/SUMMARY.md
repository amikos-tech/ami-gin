---
status: complete
completed: 2026-04-24
---

# Quick Task 260424 Summary

Addressed review feedback around the `Finalize()` nil contract, soft-skip observability, and `ErrInvalidFormat` regression coverage.

## Changes

- Documented `Finalize()` nil-on-tragic behavior and guarded production callers in parquet builds and the experiment CLI.
- Replaced soft document skip message sniffing with explicit skip kinds carried by sentinel-wrapped errors.
- Added `NumSoftSkippedDocuments()` and `NumSoftSkippedRepresentations()` while keeping `SoftSkippedDocuments()` as a deprecated compatibility alias.
- Made `Encode(nil)` return a normal error instead of panicking through a nil index.
- Added regression tests for parser/numeric/companion soft-skip logging, nil finalized indexes, and `readConfig` invalid-format branches.

## Verification

- `go test -v -run 'Test(SoftSkippedDocumentsAreObservable|NumericFailureModeSoftLogsExplicitKind|TransformerFailureModeSoftKeepsRawDocumentAndSkipsCompanion|FinalizeAfterMergePanicClosesBuilder|ReadConfigRejects.*InvalidFormat)'`
- `go test ./...`
