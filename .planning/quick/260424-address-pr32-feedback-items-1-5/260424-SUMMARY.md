---
status: complete
completed: 2026-04-24
branch: phase/17-failure-mode-taxonomy-unification
commits:
  - a9b5cb2  # item 1: drop redundant normalize calls
  - 65a8880  # item 2: changelog
  - b944691  # item 3: parseFloat doc comment
  - aa7876a  # item 4: dead-code group check
  - 9807748  # item 5: soft-skip remap invariant comment
---

# Quick Task 260424: Address PR #32 Feedback (Items 1-5) — SUMMARY

## What Was Delivered

Five small, non-blocking cleanups from Claude's review of PR #32 (Phase 17 — Failure-Mode Taxonomy Unification). Each lands in its own atomic commit.

### Item 1: Drop redundant `normalizeIngestFailureMode` calls

- **Commit:** `a9b5cb2` — `refactor(builder): drop redundant normalizeIngestFailureMode calls`
- **Files:** `builder.go` (5 call sites replaced)
- **Change:** `NewBuilder` already normalizes `config.ParserFailureMode` and `config.NumericFailureMode` at construction time. Replaced the downstream `normalizeIngestFailureMode(b.config.X) == IngestFailureSoft` checks with direct `b.config.X == IngestFailureSoft`.
- **Impact:** Behavior identical; fewer redundant calls in hot paths.

### Item 2: Expand CHANGELOG.md Unreleased

- **Commit:** `65a8880` — `docs(changelog): document phase 17 public API additions`
- **Files:** `CHANGELOG.md`
- **Change:** Added entries for `WithParserFailureMode`, `WithNumericFailureMode`, `NumSoftSkippedDocuments`, `NumSoftSkippedRepresentations`, and the deprecated `SoftSkippedDocuments` alias.
- **Impact:** Callers can adopt the new API surface without reading the code.

### Item 3: Document `parseFloat` scientific-notation limitation

- **Commit:** `b944691` — `docs(transformer-registry): note parseFloat accepts plain decimal only`
- **Files:** `transformer_registry.go`
- **Change:** Added a doc comment on `parseFloat` noting that it accepts plain decimal only (no scientific notation, Inf, NaN, or hex literals) and that unsupported captures cause the document to be skipped via `ok=false`.
- **Impact:** Surface limitation that was previously implicit.

### Item 4: Remove dead-code `p.Group < 0` check

- **Commit:** `aa7876a` — `refactor(transformer-registry): drop unreachable negative-group guard`
- **Files:** `transformer_registry.go` (2 closure edits)
- **Change:** `RegexExtract` and `RegexExtractInt` reject negative groups at construction time, so the `p.Group < 0 || ...` check inside each closure is unreachable. Simplified to `len(matches) <= p.Group`.
- **Impact:** No behavior change; clearer control flow.

### Item 5: Document soft-skip parser-remap invariant

- **Commit:** `9807748` — `docs(builder): document the soft-skip parser remap invariant`
- **Files:** `builder.go`
- **Change:** Added a comment in `AddDocument` explaining that bare `errSkipDocument` sentinels only originate from the parser path today, and that any future non-parser skip site must wrap in a typed `softSkipDocumentError` with explicit kind to avoid misclassification.
- **Impact:** Makes the implicit contract explicit for future contributors.

## Item 6 (Review) — NOT ADDRESSED (Disagreed)

The reviewer's sixth observation — that a custom `parserSink` implementation wrapping stage errors with `fmt.Errorf(...%w)` could defeat `isStageCallbackError` — was researched and disproven. `isStageCallbackError` uses `errors.As`, which walks error chains through both stdlib `%w` and `github.com/pkg/errors.Wrap`. No code change was necessary.

## Follow-up Surfaced During Task 3

While documenting the `parseFloat` limitation, a latent behavioral divergence was discovered (not part of the reviewer's original observation):

- The **public** `RegexExtractInt` in `transformers.go:145` uses `strconv.ParseFloat`, which **accepts** scientific notation like `"1.5e3"`.
- The **registry-reconstructed** `RegexExtractInt` in `transformer_registry.go:155` uses the restricted `parseFloat`, which **rejects** scientific notation.

This means an index built in-memory and queried immediately may accept a capture that the same index (once serialized and reloaded) would reject. Out-of-scope for this task — flagged here for a follow-up decision (the cleanest fix is replacing `parseFloat` with `strconv.ParseFloat` in the registry path, guarded against `NaN`/`Inf`).

## Verification

- `go build ./...` — clean.
- `go test ./...` — all packages pass (`ami-gin`, `cmd/gin-index`, `examples/failure-modes`, `telemetry`).
- `golangci-lint run` — 0 issues.
- Targeted: `go test -run 'TestNumericFailureModeSoft|TestSoftSkippedDocumentsAreObservable|TestRegex|TestRegisteredTransformer'` — passing.

## Out of Scope

- Fixing the `parseFloat` divergence (flagged as follow-up).
- Refactoring `softSkipKindOther` to a typed parser-specific sentinel (reviewer's stronger suggestion for item 5).
- Any other phase 17 work.
