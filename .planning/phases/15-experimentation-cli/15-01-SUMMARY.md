---
phase: 15
plan: 01
subsystem: cli
tags: [cli, experiment, jsonl, streaming, stdin, text-report]
dependency_graph:
  requires: []
  provides:
    - experiment subcommand registration and usage examples
    - stdin-aware experiment runner surface
    - bounded-memory JSONL ingest with two-pass RG sizing
    - foundation experiment command tests
  affects:
    - cmd/gin-index/main.go
    - cmd/gin-index/experiment.go
    - cmd/gin-index/main_test.go
    - cmd/gin-index/experiment_test.go
tech_stack:
  added: []
  patterns:
    - stdlib flag.NewFlagSet command wiring
    - bufio.Reader.ReadBytes streaming ingest
    - temp-file spooling for stdin replay
    - writeIndexInfo reuse with used-row-group reporting
key_files:
  created:
    - cmd/gin-index/experiment.go
    - cmd/gin-index/experiment_test.go
  modified:
    - cmd/gin-index/main.go
    - cmd/gin-index/main_test.go
decisions:
  - "The command keeps a two-pass contract because GINBuilder requires row-group capacity up front; stdin satisfies that contract by spooling to a temp file rather than buffering the whole stream in memory."
  - "Text output reuses writeIndexInfo by rendering through a shallow index copy with NumRowGroups rewritten to used row groups, so human-facing output does not report preallocated builder capacity."
  - "Blank JSONL lines are rejected immediately as invalid input, while a final non-newline-terminated document is accepted."
metrics:
  completed: "2026-04-22"
  tasks_completed: 3
  files_changed: 4
---

# Phase 15 Plan 01: Experiment Command Foundation Summary

Phase 15 foundation shipped: the CLI now exposes `gin-index experiment`, accepts JSONL from a file path or `-`, builds via bounded-memory line reads, and prints a run summary before the existing per-path info table.

## What Was Built

### Task 1: Command registration and stdin-aware runner surface

- Added `case "experiment": cmdExperiment(args)` to `cmd/gin-index/main.go`.
- Extended `printUsage()` with both file-path and stdin examples.
- Created `cmd/gin-index/experiment.go` with `cmdExperiment(args)` and `runExperiment(args, stdin, stdout, stderr)`.
- Extended the parse-failure helper in `cmd/gin-index/main_test.go` so `experiment` participates in the shared CLI parse-error coverage.

### Task 2: Streaming ingest, synthetic row groups, and base text report

- Implemented a two-pass input contract:
  - local files are counted, then reopened for ingest
  - stdin is spooled to a temp file during the count pass, then reopened for the build pass
- Both passes use `bufio.NewReader(...).ReadBytes('\n')`, so large lines are preserved and final lines without a trailing newline are still ingested.
- Real ingest trims only `\n` / `\r\n`, rejects blank JSONL lines, derives synthetic row groups from successful ingest count, and writes a summary block followed by the reused `GIN Index Info:` path table.
- User-facing row-group output reports used row groups rather than builder capacity.

### Task 3: Focused experiment foundation tests

- Added `cmd/gin-index/experiment_test.go` with direct `runExperiment(...)` coverage for:
  - file input
  - stdin input
  - final line without trailing newline
  - empty input
  - line larger than 64 KiB
  - summary-before-path-table ordering
  - invalid `--rg-size`
  - directory input rejection

## Commits

| Commit | Purpose |
|--------|---------|
| `d4e7c45` | add parse-failure coverage for `experiment` |
| `340ca34` | register the `experiment` command surface and usage text |
| `8a5a169` | implement two-pass streaming ingest and base text reporting |
| `888b918` | add focused experiment foundation tests |

## Verification

- `go test ./cmd/gin-index -run 'Test(RunCommandsReturnParseFailureCode|RunExperimentFromFile|RunExperimentFromStdin|RunExperimentFinalLineWithoutTrailingNewline|RunExperimentEmptyInput|RunExperimentLargeLineNoTruncation|RunExperimentTextOutputOrder|RunExperimentRejectsInvalidRGSize|RunExperimentRejectsDirectoryInput)$' -count=1`
- `go test ./...`
- Manual layout check:
  - `go run ./cmd/gin-index experiment <temp-jsonl>`
  - confirmed output order is `Experiment Summary:` followed by `GIN Index Info:`

## Deviations from Plan

None.

## Known Stubs

- `--json`, `--test`, `-o`, and `--log-level` are intentionally deferred to Plan 15-02.
- `--sample` and `--on-error` are intentionally deferred to Plan 15-03.

## Threat Flags

None.

## Self-Check

Files created:
- cmd/gin-index/experiment.go — FOUND
- cmd/gin-index/experiment_test.go — FOUND

Files modified:
- cmd/gin-index/main.go — FOUND
- cmd/gin-index/main_test.go — FOUND

Verification checks:
- parse-failure coverage passes: PASS
- file-input experiment run passes: PASS
- stdin experiment run passes: PASS
- final non-newline record passes: PASS
- empty-input summary passes: PASS
- >64 KiB line passes: PASS
- text output order passes: PASS
- invalid `--rg-size` rejection passes: PASS
- directory-input rejection passes: PASS
- full `go test ./...` passes: PASS
- manual summary/path-table layout check passes: PASS

## Self-Check: PASSED
