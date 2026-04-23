---
phase: 15
plan: 03
subsystem: cli
tags: [cli, sampling, error-handling, policy, guardrails, jsonl]
dependency_graph:
  requires:
    - 15-01 (experiment command foundation)
    - 15-02 (shared report model and rich output features)
  provides:
    - sample-limited direct-stream ingest
    - abort/continue malformed-line semantics with counters
    - executable charter guards for CLI drift
  affects:
    - cmd/gin-index/experiment.go
    - cmd/gin-index/experiment_output.go
    - cmd/gin-index/experiment_test.go
    - cmd/gin-index/experiment_policy_test.go
tech_stack:
  added: []
  patterns:
    - single-pass sampled stdin/file ingest
    - immediate `line N:` stderr diagnostics with counter-only tracking
    - static source/go.mod scans as command-charter guardrails
key_files:
  created:
    - cmd/gin-index/experiment_policy_test.go
  modified:
    - cmd/gin-index/experiment.go
    - cmd/gin-index/experiment_output.go
    - cmd/gin-index/experiment_test.go
decisions:
  - "Sampled runs bypass the original count/spool pass entirely and size builder capacity from the sample limit so stdin sampling can stop without consuming the full stream."
  - "Abort-mode line failures emit `line N:` once and return non-zero without a second generic error wrapper."
  - "Continue-mode tracking retains only counters; no unbounded in-memory error list is accumulated."
metrics:
  completed: "2026-04-22"
  tasks_completed: 3
  files_changed: 4
---

# Phase 15 Plan 03: Sampling, Error Handling, and Policy Summary

Wave 3 shipped: the `experiment` command now supports sample-limited direct streaming, exact abort/continue malformed-line semantics with stable counters, and CI-enforced policy tests that block CLI framework, TTY, colour, and parser-flag drift.

## What Was Built

### Task 1: Sample mode and abort/continue ingest semantics

- Added `--sample N` with direct-stream behavior for both file and stdin inputs.
- Added `--on-error abort|continue`, defaulting to `abort`.
- Continue mode emits `line N:` diagnostics to `stderr`, increments counters, and keeps ingesting.
- Abort mode emits `line N:` diagnostics and exits immediately with a non-zero code.
- Summary counters now capture `processed_lines`, `skipped_lines`, and `error_count`, and both text and JSON output derive row-group math from used row groups rather than builder capacity.

### Task 2: End-to-end malformed-input and sampling tests

- Added tests for:
  - abort-mode malformed input
  - continue-mode malformed input in both text and JSON output
  - sampled stdin execution that must not over-read past the sampled documents
  - invalid `--on-error` validation

### Task 3: Executable charter guards

- Added `cmd/gin-index/experiment_policy_test.go` with guards for:
  - forbidden CLI/TUI/color/readline dependencies in `go.mod`
  - forbidden imports in production `cmd/gin-index/*.go`
  - forbidden TTY/ANSI/color logic in production command code
  - accidental `--parser` exposure in usage output or flag registration

## Commits

| Commit | Purpose |
|--------|---------|
| `a4d12c6` | add sampled and error-tolerant experiment ingest |
| `84f5c25` | add end-to-end coverage for error handling and sampling |
| `a13f4b2` | add executable experiment policy guards |

## Verification

- `go test ./cmd/gin-index -run 'Test(RunExperimentOnError(Abort|Continue)|RunExperimentSampleLimit|RunExperimentRejectsInvalidOnErrorValue)$' -count=1`
- `go test ./cmd/gin-index -run 'TestExperiment(CommandHasNoForbidden(Dependencies|Imports|TTYLogic)|UsageDoesNotExposeParserFlag)$' -count=1`
- `go test ./cmd/gin-index -count=1`
- `go test ./...`

## Deviations from Plan

None.

## Known Stubs

None. This plan closes the remaining Phase 15 roadmap scope.

## Threat Flags

None.

## Self-Check

Files created:
- cmd/gin-index/experiment_policy_test.go — FOUND

Files modified:
- cmd/gin-index/experiment.go — FOUND
- cmd/gin-index/experiment_output.go — FOUND
- cmd/gin-index/experiment_test.go — FOUND

Verification checks:
- abort-mode malformed-line test passes: PASS
- continue-mode malformed-line tests pass: PASS
- sample-limit direct-stream test passes: PASS
- invalid `--on-error` validation test passes: PASS
- forbidden dependency/import/TTY tests pass: PASS
- parser-flag exposure test passes: PASS
- full `cmd/gin-index` suite passes: PASS
- full `go test ./...` suite passes: PASS

## Self-Check: PASSED
