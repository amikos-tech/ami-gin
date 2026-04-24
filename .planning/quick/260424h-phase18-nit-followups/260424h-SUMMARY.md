---
status: complete
completed: 2026-04-24
branch: phase/18-structured-ingesterror-cli-integration
commit: 72eea02
---

# Quick Task 260424h: Phase 18 Nit Follow-Ups — SUMMARY

## Delivered

- Added the tragic-abort call-site ordering comment so future refactors preserve failure recording before wrapping.
- Added `TestExperimentTragicAbortErrorUnwrapExposesOnlyBuilderErr` to pin the public error-chain policy.
- Replaced the narrow text-mode negation with an exact `GIN Index Info:` count assertion.
- Expanded `remapCompanionIngestErrorPath` godoc to mention target-prefix mismatch as a no-op.

## Verification

- `go test ./cmd/gin-index -run 'Test(HandleExperimentLineErrorAbortsOnTragicBuilder|ExperimentTragicAbortErrorUnwrapExposesOnlyBuilderErr|RunExperimentOnErrorContinueTragicAbortText)'`: pass
- `go test ./...`: pass

## Notes

- The pre-existing dirty review artifact at `.planning/phases/18-structured-ingesterror-cli-integration/18-REVIEW.md` was not edited.
