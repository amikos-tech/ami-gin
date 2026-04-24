---
status: complete
completed: 2026-04-24
branch: phase/18-structured-ingesterror-cli-integration
commit: this commit
---

# Quick Task 260424f: Phase 18 Re-Review Follow-Ups — SUMMARY

## Delivered

- `aborted:tragic` is now a real structured experiment status. Continue-mode tragic aborts preserve the accumulated failure groups, emit a report, and still return exit code `1`.
- `runExperiment` gained an overridable builder seam for tests, and the CLI now has an end-to-end tragic JSON test proving the loop stops after the first builder-closing failure instead of feeding a poisoned builder.
- CHANGELOG wording now reflects the accessor-based `IngestError` API, and the CLI additions are documented (`unknown` grouping plus `aborted:tragic`).
- SECURITY citations for T18-05 and T18-10 were realigned to the actual guard/cap enforcement lines.
- Helper comments were added for the non-obvious path remap and tragic-abort policy, the layer-order comment now distinguishes pinned `unknown` from future lexical layers, and nil-receiver accessors are covered in tests.

## Verification

- Focused `cmd/gin-index` tragic/continue/grouping tests: pass
- Focused root ingest-error tests: pass
- `go test ./...`: pass

## Notes

- The review doc at `.planning/phases/18-structured-ingesterror-cli-integration/18-REVIEW.md` was already dirty before this pass and was not modified by the implementation work.
