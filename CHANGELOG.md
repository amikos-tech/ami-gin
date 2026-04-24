# Changelog

## Unreleased

- `IngestFailureMode` / `IngestFailureHard` / `IngestFailureSoft` are the preferred failure-mode names. Deprecated source-compatible aliases `TransformerFailureMode` / `TransformerFailureStrict` / `TransformerFailureSoft` remain available for pre-phase-17 callers.
- Companion transformer soft mode is representation-scoped: `WithTransformerFailureMode(gin.IngestFailureSoft)` skips only the derived alias when a transformer returns `ok=false`; it does not drop the source document.

  Before: `gin.WithTransformerFailureMode(gin.TransformerFailureSoft)`

  After: `gin.WithTransformerFailureMode(gin.IngestFailureSoft)`

- New config options extend the taxonomy to parser and numeric failures:
  - `WithParserFailureMode(mode)` — drop documents that fail JSON parsing under `IngestFailureSoft`; surface the error under `IngestFailureHard` (default).
  - `WithNumericFailureMode(mode)` — drop documents that fail numeric coercion or hit unsupported mixed-numeric promotion under `IngestFailureSoft`; surface the error under `IngestFailureHard` (default).
- New builder observability:
  - `NumSoftSkippedDocuments()` returns the count of documents dropped by soft parser or numeric failures.
  - `NumSoftSkippedRepresentations()` returns the count of companion representations skipped by soft transformer failures.
  - `SoftSkippedDocuments()` is a deprecated alias for `NumSoftSkippedDocuments()`; prefer the new name.
