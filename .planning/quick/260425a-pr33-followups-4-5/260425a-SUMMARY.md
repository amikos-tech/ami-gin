---
status: complete
created: 2026-04-25
completed: 2026-04-25
branch: phase/18-structured-ingesterror-cli-integration
pr: 33
commits:
  - 5ec60bb
  - 54b7e34
---

# Quick Task 260425a: PR #33 Follow-Ups (Items 4 and 5) — Summary

## Outcome

Both PR #33 review observations addressed.

## Changes

### Task 1 — `+hard-ingest` directive tightening (`5ec60bb`)

- `ingest_error_guard_test.go:hasHardIngestDirective` now requires the canonical standalone form: `// +hard-ingest` as its own trimmed comment line. Substring matches with trailing text (`// +hard-ingest validate ...`) and near-identical tokens (`+hard-ingest-foo`) are rejected.
- `builder.go:1009` migrated to canonical form: `// +hard-ingest` on its own line, followed by a separate descriptive doc line for `validateStagedPaths`.
- Doc comment on the guard updated to call out the canonical-form requirement.
- Edge cases verified in a scratch module: 3 detection cases, 4 rejection cases — all pass.

### Task 2 — `withExperimentDefaultConfig` safety comment (`54b7e34`)

- Added the matching `// This helper overrides a package-global seam; callers must not run in parallel while the override is active.` comment to `cmd/gin-index/experiment_test.go:withExperimentDefaultConfig` for symmetry with `withExperimentBuilderFactory`.

## Verification

- `go test -run TestHardIngestFunctionsDoNotReturnPlainErrors .` — PASS
- `go test ./cmd/gin-index` — PASS
- `go test .` — PASS
- `go vet ./...` — clean

## Notes

Both items were classified as low-risk hygiene fixes by the PR feedback analysis. Item 4 was originally marked "defer" but proved trivial enough to land alongside item 5. Future contributors writing a new hard-ingest site now get a compile-failing test if they use the wrong directive shape.
