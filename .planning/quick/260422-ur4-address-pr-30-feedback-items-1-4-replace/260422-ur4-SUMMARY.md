---
phase: 260422-ur4
plan: 01
subsystem: cmd/gin-index
tags:
  - pr-30-feedback
  - refactor
  - cli
  - experimentation
requires:
  - Phase 15 shipped (PR #30)
provides:
  - Lightweight json.Valid abort validator (no shadow GINBuilder)
  - Documented mutation contract for trimExperimentIndexRowGroups
  - Order-independent $.status lookup in experiment JSON golden test
  - Shared typeNames helper for type bitmask iteration
affects:
  - cmd/gin-index/experiment.go
  - cmd/gin-index/experiment_test.go
  - cmd/gin-index/main.go
  - cmd/gin-index/experiment_output.go
tech_stack:
  added: []
  patterns:
    - "encoding/json.Valid for record-level JSONL validation"
    - "Name-based lookup over array-index lookup in JSON golden tests"
key_files:
  created: []
  modified:
    - cmd/gin-index/experiment.go
    - cmd/gin-index/experiment_test.go
    - cmd/gin-index/main.go
    - cmd/gin-index/experiment_output.go
decisions:
  - "Validator uses encoding/json.Valid and preserves the original \"blank JSONL line\" error for empty records to keep existing test expectations intact"
  - "Mutation contract for trimExperimentIndexRowGroups documented inline rather than via a new API — keeps the contract exception visible at the call site"
  - "typeNames helper placed in main.go alongside describeTypes (both files in package main, no extra file needed)"
metrics:
  duration_minutes: ~5
  completed: 2026-04-22
  tasks_completed: 4
  commits: 5
requirements:
  - PR30-FEEDBACK-01
  - PR30-FEEDBACK-02
  - PR30-FEEDBACK-03
  - PR30-FEEDBACK-04
---

# Quick Task 260422-ur4: Address PR #30 Feedback Items 1–4 Summary

Swapped the stdin abort validator to `encoding/json.Valid`, documented the deliberate post-Finalize mutation in `trimExperimentIndexRowGroups`, made the experiment JSON golden test locate `$.status` by name instead of array index, and extracted the type-bitmask iteration into a shared `typeNames` helper so `describeTypes` and `collectExperimentTypes` no longer drift.

## Tasks Executed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Replace builder-based abort validator with `json.Valid` | `ae56e17` | `cmd/gin-index/experiment.go` |
| 2 | Document `trimExperimentIndexRowGroups` mutation contract | `4f3185b` | `cmd/gin-index/experiment.go` |
| 3 | Name-based `$.status` lookup in experiment JSON golden test | `54f5df4` | `cmd/gin-index/experiment_test.go` |
| 4 | Extract shared `typeNames` helper | `74c16b0` | `cmd/gin-index/main.go`, `cmd/gin-index/experiment_output.go` |
| + | Drop now-unused `config` parameter from stdin plumbing (Rule 3 follow-up) | `231275d` | `cmd/gin-index/experiment.go` |

## Verification

```bash
go build ./...                          # clean
go test ./cmd/gin-index/... -count=1    # ok  github.com/amikos-tech/ami-gin/cmd/gin-index  0.505s
golangci-lint run ./cmd/gin-index/...   # 0 issues
```

All three gates pass.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Dropped unused `config` parameter from `prepareExperimentStdin` and `prepareExperimentSource`**
- **Found during:** Final lint run after Task 4
- **Issue:** `unparam` flagged both functions once the validator no longer needed a `gin.GINConfig`. Ignoring it would have failed the project-wide lint gate.
- **Fix:** Removed the parameter from both signatures and updated the single call site in `runExperiment`. No behavior change.
- **Files modified:** `cmd/gin-index/experiment.go`
- **Commit:** `231275d`

## Key Decisions

- **Error message parity:** The new validator keeps the exact `"blank JSONL line"` error for empty records so pre-existing test assertions and stderr output remain unchanged. Malformed JSON now emits `"invalid JSON record"` — clearer than the previous builder-derived errors.
- **Mutation doc inline, not reorganized:** `trimExperimentIndexRowGroups` stays at its current call site with only a richer doc comment. Moving it into its own file or wrapping it in a new type would have leaked the exception beyond the experiment path; inline doc keeps the caveat local to the one place that uses it.
- **`typeNames` lives in `main.go`:** Both files are `package main`, so an additional types helper file was unnecessary. Placement next to `describeTypes` keeps the loop beside the older caller.

## Self-Check: PASSED

- `cmd/gin-index/experiment.go` — modified, committed in `ae56e17`, `4f3185b`, `231275d`
- `cmd/gin-index/experiment_test.go` — modified, committed in `54f5df4`
- `cmd/gin-index/main.go` — modified, committed in `74c16b0`
- `cmd/gin-index/experiment_output.go` — modified, committed in `74c16b0`
- All five commits present in `git log` on `codex/phase-15-experimentation-cli`
- `go build ./...` passes
- `go test ./cmd/gin-index/... -count=1` passes
- `golangci-lint run ./cmd/gin-index/...` reports 0 issues
