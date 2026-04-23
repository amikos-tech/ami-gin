---
status: complete
phase: 15-experimentation-cli
source: [15-01-SUMMARY.md, 15-02-SUMMARY.md, 15-03-SUMMARY.md]
started: 2026-04-22T17:02:47Z
updated: 2026-04-22T17:33:01Z
---

## Current Test

[testing complete]

## Tests

### 1. File Input Experiment Report
expected: Create a small JSONL file with three records and run `go run ./cmd/gin-index experiment <path-to-jsonl>`. The command should exit successfully, print `Experiment Summary:` before `GIN Index Info:`, report the input path plus `Documents: 3`, `Row Groups: 3`, `RG Size: 1`, and list the indexed paths in the final `Paths:` section.
result: pass

### 2. Stdin Input Experiment Report
expected: Pipe the same three-record JSONL into `go run ./cmd/gin-index experiment -`. The command should consume stdin successfully, report `Input: -`, keep the summary-before-info-table ordering, and render the indexed paths without requiring a temp file argument.
result: pass

### 3. JSON Predicate Report
expected: Run `go run ./cmd/gin-index experiment --json --rg-size 2 --test '$.status = "error"' <path-to-jsonl>` against a file containing two `status=\"ok\"` records and one `status=\"error\"` record. The command should emit valid JSON with top-level `source`, `summary`, `paths`, and `predicate_test` sections; `summary.documents` should be `3`, `summary.row_groups` should be `2`, and `predicate_test` should report the canonical predicate plus `matched: 1`, `pruned: 1`, and `pruning_ratio: 0.5`.
result: pass

### 4. Sidecar Output
expected: Run `go run ./cmd/gin-index experiment -o <output>.gin <path-to-jsonl>`. The command should succeed, mention `Sidecar Path:` in the summary, and leave a readable `.gin` file at the requested path.
result: pass

### 5. Stderr-Only Log Output
expected: Run `go run ./cmd/gin-index experiment --log-level info --test '$.status = "error"' <path-to-jsonl>` while capturing stdout and stderr separately. The normal report should stay on stdout, and the evaluation log line (for example `evaluate completed`) should appear only on stderr.
result: pass

### 6. Sample Limit
expected: Run `go run ./cmd/gin-index experiment --sample 2 <path-to-three-record-jsonl>`. The command should stop after two successful documents, report `Documents: 2`, `Row Groups: 2`, `Sample Limit: 2`, and `Processed Lines: 2`, and the indexed paths should reflect only the sampled records.
result: pass

### 7. Abort on Malformed Input
expected: Run `go run ./cmd/gin-index experiment --on-error abort <path-to-jsonl-with-a-bad-line>`. The command should emit a `line N:` parse diagnostic to stderr, exit non-zero immediately, and avoid printing the normal experiment summary to stdout.
result: pass

### 8. Continue on Malformed Input
expected: Run `go run ./cmd/gin-index experiment --on-error continue --sample 2 <path-to-jsonl-with-a-bad-line>`. The command should emit a `line N:` parse diagnostic to stderr, continue processing until two valid documents are ingested, and print a summary showing `Documents: 2`, `Sample Limit: 2`, `Processed Lines: 3`, `Skipped Lines: 1`, and `Error Count: 1`.
result: pass

## Summary

total: 8
passed: 8
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
