# Changelog

## Unreleased

- `Regex` with `(?i)` no longer prunes a match that uses a non-ASCII case fold
  (#70). `(?i)sss` matches `ſſſ` (long s) but the trigram index lowercases with
  `strings.ToLower`, which keeps `ſ`. A case-folded literal is now skipped
  when any rune's `unicode.SimpleFold` orbit does not lower to one rune, so
  such patterns fall back to a full scan. ASCII and accented letters, and the
  Kelvin sign orbit of `k`, still prune. The index format is unchanged.
- `Regex` no longer prunes a row group that holds a match (#68). Literal
  extraction now tracks whether a node's literals are whole (the node matches
  exactly those strings) or fragments (every match contains one of them
  somewhere). Only whole sets are multiplied across a concatenation; an
  alternation with a branch that proves nothing yields nothing; an unbounded
  repetition (`+`, `{n,}`) contributes its literals as fragments; exceeding the
  expansion cap (100) yields nothing instead of a cut list, so such patterns
  fall back to a full scan. `ExtractLiterals("ab+c")` is now `a`, `b`, `c`
  (too short to prune) rather than `abc`; patterns such as `(error|warn)_msg`
  and `foo.*bar` are unchanged. Property tests against `regexp` over a small
  pattern grammar pin the superset.

## v1.1.0 (2026-09-14)

- Array elements are indexed only under their canonical wildcard paths (for
  example, `$.items[*].id`), rather than private numeric paths such as
  `$.items[0].id`. Rebuild indexes containing arrays after upgrading: the
  exported `PathDirectory`, `Header.NumPaths`, `gin-index info` path listing,
  and serialized bytes will intentionally change, usually becoming smaller.
- Added `GINConfig.MaxStagedPaths` and `WithMaxStagedPaths(limit)` to bound
  all JSON paths staged for each document, including internal companion
  transformer paths. The root and object or array containers count. A limit
  failure is a hard `*IngestError` from `AddDocument`, with
  `Layer() == IngestLayerResource`; it is not affected by
  `WithParserFailureMode` and never appears at `Finalize`. Both
  `gin-index build` and `gin-index experiment` expose this bound via
  `--max-staged-paths`.
- Added the opt-in same-package `pure-simdjson` parser adapter. Builds compiled with `-tags simdjson` can call `NewSIMDParser() (CloseableParser, error)`, handle native construction errors, select the parser explicitly with `WithParser(p)`, and deterministically release its native handle with `Close`; its parser name is `pure-simdjson`. Ordinary builds and the default `NewBuilder` remain stdlib-only. Valid out-of-range numeric literals now follow the same path-aware `NumericFailureMode` behavior under both parsers, while malformed JSON remains governed by `ParserFailureMode`. See [`docs/simd-deployment.md`](docs/simd-deployment.md) for activation, ownership, loading, integrity, fallback, and numeric-limit details.
- The root package now exports `IngestError` for hard per-document ingest failures. Callers inspect it via `Path()`, `Layer()`, `Value()`, `Unwrap()`, and `Cause()`; `Layer()` uses `parser`, `transformer`, `numeric`, `schema`, and `resource` values. `Value()` is verbatim for document-data failures and empty for resource failures; resource diagnostics remain in `Cause()`. Document-data values are neither redacted nor truncated by the library, so callers own redaction and output-size policy. `gin-index experiment --on-error continue` text and JSON summaries now group structured failures by layer with at most 3 samples per layer, use an `unknown` bucket for non-`IngestError` failures, and emit `aborted:tragic` when continue mode aborts on a closed builder.
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
