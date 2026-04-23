# Changelog

## Unreleased

- Breaking: `TransformerFailureMode` / `TransformerFailureStrict` / `TransformerFailureSoft` were replaced by `IngestFailureMode` / `IngestFailureHard` / `IngestFailureSoft`.

  Before: `gin.WithTransformerFailureMode(gin.TransformerFailureSoft)`

  After: `gin.WithTransformerFailureMode(gin.IngestFailureSoft)`
