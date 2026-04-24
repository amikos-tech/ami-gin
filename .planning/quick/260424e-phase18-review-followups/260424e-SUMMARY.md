---
status: complete
completed: 2026-04-24
branch: phase/18-structured-ingesterror-cli-integration
commit: this commit
---

# Quick Task 260424e: Phase 18 Review Follow-Ups — SUMMARY

## Delivered

- Fixed the derived-path leak by remapping companion-representation ingest failures from `__derived:...` back to the source canonical path before returning them.
- Tightened `IngestError` into an accessor-based API (`Path()`, `Layer()`, `Value()`) and documented both the verbatim-value contract and the stable `Error()` string format.
- Made CLI experiment failure grouping lossless for continue mode by introducing an `unknown` failure bucket for non-`IngestError` lines and preserving first-seen sample order up to the 3-sample cap.
- Added tragic-builder abort handling for continue mode so the CLI stops after the first fatal builder closure instead of emitting repeated `builder closed` noise.
- Rewrote the hard-ingest AST guard to auto-discover `stage*` methods plus `+hard-ingest` surfaces and catch direct plain-error construction, `fmt.Errorf`, `errors.WithMessage`, `errors.WithStack`, composite-literal `IngestError`, and `return err` from tainted assignments.

## Test Coverage Added or Tightened

- Companion transformer matrix for unsupported transformed types (`complex128`, struct) with source-path assertions.
- CLI deterministic ordering now covers `schema` and `unknown` buckets.
- CLI hundred-doc fixture now exercises schema failures end-to-end.
- Sample-cap behavior is asserted with 10 failures in one layer and strict first-three ordering.
- Non-finite numeric hard failure now uses `requireIngestError` like the sibling numeric cases.
- Tragic continue-mode abort logic is covered via the extracted line-error handler.

## Verification

- `go test` — pass
- `go test ./cmd/gin-index` — pass
- `go test ./...` — pass

## Notes

- Review suggestions that are still not implemented after this pass are the lower-priority text/assertion ergonomics items (`experimentDefaultConfig` factory threading, sentinel `errors.Is` coverage, regex-based text assertions).
