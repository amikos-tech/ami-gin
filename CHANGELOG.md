# Changelog

## Unreleased

- Added the opt-in same-package `pure-simdjson` v0.1.4 parser adapter. Builds compiled with `-tags simdjson` can call `NewSIMDParser() (Parser, error)`, handle native construction errors, and select the parser explicitly with `WithParser(p)`; its parser name is `pure-simdjson`. Ordinary builds and the default `NewBuilder` remain stdlib-only. Integers larger than `uint64` remain an accepted difference: SIMD reports a parser-layer failure at an empty path governed by `ParserFailureMode`, while stdlib reaches path-aware numeric handling governed by `NumericFailureMode`; soft mode at either layer discards the failed document atomically. See [`docs/simd-deployment.md`](docs/simd-deployment.md) for activation, loading, integrity, fallback, and limitation details.
- The root package now exports `IngestError` for hard per-document ingest failures. Callers inspect it via `Path()`, `Layer()`, `Value()`, `Unwrap()`, and `Cause()`; `Layer()` uses `parser`, `transformer`, `numeric`, and `schema` values. `Value()` is verbatim and is not redacted or truncated by the library, so callers own redaction and output-size policy. `gin-index experiment --on-error continue` text and JSON summaries now group structured failures by layer with at most 3 samples per layer, use an `unknown` bucket for non-`IngestError` failures, and emit `aborted:tragic` when continue mode aborts on a closed builder.
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
