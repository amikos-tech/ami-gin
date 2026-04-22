---
phase: 15
plan: 02
subsystem: cli
tags: [cli, json, predicate, sidecar, logging, report]
dependency_graph:
  requires:
    - 15-01 (experiment command foundation)
  provides:
    - shared experiment report model for text and JSON output
    - inline predicate testing with count-only output
    - readable `.gin` sidecar writing
    - stderr-only log-level wiring for experiment runs
  affects:
    - cmd/gin-index/experiment.go
    - cmd/gin-index/experiment_output.go
    - cmd/gin-index/experiment_test.go
tech_stack:
  added: []
  patterns:
    - explicit report structs with stable JSON field tags
    - text rendering that reuses writeIndexInfo via a shallow header override
    - parsePredicate + EvaluateContext inline predicate execution
    - slog text handler bridged through logging/slogadapter
key_files:
  created:
    - cmd/gin-index/experiment_output.go
  modified:
    - cmd/gin-index/experiment.go
    - cmd/gin-index/experiment_test.go
decisions:
  - "JSON output is struct-backed and indented, not map-backed, so field names and omission behavior stay deterministic."
  - "Predicate reporting stays count-only: matched, pruned, and pruning_ratio are computed from used row groups rather than builder capacity."
  - "Experiment-side logger wiring stays CLI-owned and stderr-only by adapting slog into the repo logger contract instead of printing ad hoc diagnostics."
metrics:
  completed: "2026-04-22"
  tasks_completed: 3
  files_changed: 3
---

# Phase 15 Plan 02: Rich Experiment Output Summary

Wave 2 shipped: the `experiment` command now has one shared report model for text and JSON, an inline predicate tester, optional `.gin` sidecar writing, and CLI-owned `--log-level` wiring through the Phase 14 observability seam.

## What Was Built

### Task 1: Shared report model for text and JSON output

- Added `cmd/gin-index/experiment_output.go` with explicit report structs for:
  - `source`
  - `summary`
  - `paths`
  - optional `predicate_test`
- Added `collectExperimentPathRows(...)` so JSON mode and text mode share the same path metadata source.
- Added `estimateBloomOccupancy(...)` using the configured bloom-filter dimensions and per-path cardinality estimate.
- Refactored the command loop to build one report value, then render either text or JSON from it.

### Task 2: Predicate testing, sidecar writing, and log-level wiring

- Added `--test` using the existing `parsePredicate(...)` path and `EvaluateContext(...)`.
- Added `-o out.gin` support with `.gin` suffix validation, `gin.Encode(...)`, and the existing local artifact write helpers.
- Added `--log-level off|info|debug`, wired through `logging/slogadapter` and a `slog.TextHandler` that writes only to `stderr`.
- Preserved the user-facing report stream on `stdout` for both text and JSON output.

### Task 3: Rich-output regression coverage

- Extended `cmd/gin-index/experiment_test.go` with focused tests for:
  - JSON schema and omission rules
  - text predicate reporting
  - JSON predicate reporting
  - sidecar roundtrip stability through `gin.ReadSidecar(...)`
  - non-`.gin` output rejection
  - stderr-only log output behavior

## Commits

| Commit | Purpose |
|--------|---------|
| `3afc8d6` | add the shared report model and JSON/text renderers |
| `a9d23cd` | add predicate testing, sidecar output, and log-level handling |
| `a8c960d` | add rich experiment output regression tests |

## Verification

- `go test ./cmd/gin-index -run 'Test(RunExperimentJSONGolden|RunExperimentPredicateReport(Text|JSON)|RunExperimentWritesSidecarRoundTrip|RunExperimentRejectsNonGinOutput|RunExperimentLogLevelWritesOnlyToStderr)$' -count=1`
- `go test ./cmd/gin-index -count=1`
- `go test ./...`

## Deviations from Plan

None.

## Known Stubs

- `--sample` and `--on-error` are intentionally deferred to Plan 15-03.
- The executable charter/policy guards are intentionally deferred to Plan 15-03.

## Threat Flags

None.

## Self-Check

Files created:
- cmd/gin-index/experiment_output.go — FOUND

Files modified:
- cmd/gin-index/experiment.go — FOUND
- cmd/gin-index/experiment_test.go — FOUND

Verification checks:
- JSON schema test passes: PASS
- text predicate report test passes: PASS
- JSON predicate report test passes: PASS
- sidecar roundtrip test passes: PASS
- non-`.gin` output rejection test passes: PASS
- stderr-only log-level behavior test passes: PASS
- full `cmd/gin-index` suite passes: PASS
- full `go test ./...` suite passes: PASS

## Self-Check: PASSED
