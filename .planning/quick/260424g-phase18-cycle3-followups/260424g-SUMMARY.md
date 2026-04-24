---
status: complete
completed: 2026-04-24
branch: phase/18-structured-ingesterror-cli-integration
commit: this commit
---

# Quick Task 260424g: Phase 18 Cycle 3 Follow-Ups — SUMMARY

## Delivered

- Tragic builder-closing failures now bypass the normal per-layer sample cap, so the operationally important triggering sample is always preserved in the emitted report.
- The tragic JSON CLI test now asserts report shape and sample presence/line number, and a matching tragic text-mode test now covers the human-facing rendering branch.
- SECURITY now uses `Value()` consistently and the drifting T18-09/T18-10/T18-11 rows were switched to symbol-based references to avoid recurring line-number rot.
- `remapCompanionIngestErrorPath` godoc now documents the `errors.As`-driven in-place mutation plus its no-op cases, and `experimentTragicAbortError.Unwrap()` now explicitly documents that only the builder-closing error remains in the chain.

## Verification

- Focused `cmd/gin-index` cycle-3 tragic/grouping tests: pass
- Focused root ingest-error tests: pass
- `go test ./...`: pass

## Notes

- The review doc at `.planning/phases/18-structured-ingesterror-cli-integration/18-REVIEW.md` remains separately dirty and was not edited as part of the implementation work.
