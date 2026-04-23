# Changelog

## Unreleased

- `IngestFailureMode` / `IngestFailureHard` / `IngestFailureSoft` are the preferred failure-mode names. Deprecated source-compatible aliases `TransformerFailureMode` / `TransformerFailureStrict` / `TransformerFailureSoft` remain available for pre-phase-17 callers.
- Companion transformer soft mode is representation-scoped: `WithTransformerFailureMode(gin.IngestFailureSoft)` skips only the derived alias when a transformer returns `ok=false`; it does not drop the source document.

  Before: `gin.WithTransformerFailureMode(gin.TransformerFailureSoft)`

  After: `gin.WithTransformerFailureMode(gin.IngestFailureSoft)`
