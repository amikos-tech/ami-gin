---
phase: 15-experimentation-cli
verified: 2026-04-22T16:38:37Z
status: passed
score: 6/6 must-haves verified
overrides_applied: 0
re_verification: false
---

# Phase 15: Experimentation CLI Verification Report

**Phase Goal:** A new `gin-index experiment` subcommand that turns a JSONL file or stdin stream into a built index plus a human- or JSON-readable per-path summary, with inline predicate testing and optional sidecar output.
**Verified:** 2026-04-22T16:38:37Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `gin-index experiment <file>` and `gin-index experiment -` both work and produce summary + path table output | VERIFIED | `cmd/gin-index/main.go` dispatches `experiment`; `cmd/gin-index/experiment.go` exposes `runExperiment(args, stdin, stdout, stderr)`; `TestRunExperimentFromFile` and `TestRunExperimentFromStdin` pass |
| 2 | Streaming JSONL ingest handles large lines and final non-newline records without full-file buffering | VERIFIED | `cmd/gin-index/experiment.go` uses `bufio.NewReader(...).ReadBytes('\n')`; `TestRunExperimentLargeLineNoTruncation` and `TestRunExperimentFinalLineWithoutTrailingNewline` pass |
| 3 | `--json` emits a stable schema and `--test` reports matched/pruned/pruning_ratio counts without RG ID leakage | VERIFIED | `cmd/gin-index/experiment_output.go` defines struct-backed `experimentReport`; `TestRunExperimentJSONGolden`, `TestRunExperimentPredicateReportText`, and `TestRunExperimentPredicateReportJSON` pass |
| 4 | `-o out.gin` writes a readable sidecar and round-trips through `gin.ReadSidecar(...)` | VERIFIED | `cmd/gin-index/experiment.go` writes encoded output via `writeLocalIndexFile(...)`; `TestRunExperimentWritesSidecarRoundTrip` passes |
| 5 | `--sample` and `--on-error continue|abort` have exact counter semantics and used-row-group math | VERIFIED | `cmd/gin-index/experiment.go` validates `--sample` and `--on-error`, supports direct-stream sampled runs, and maintains processed/skipped/error counters; `TestRunExperimentOnErrorAbort`, `TestRunExperimentOnErrorContinue`, `TestRunExperimentSampleLimit`, and `TestRunExperimentRejectsInvalidOnErrorValue` pass |
| 6 | The command stays within the phase charter: no CLI frameworks, no TTY/color logic, no `--parser` exposure | VERIFIED | `cmd/gin-index/experiment_policy_test.go` enforces forbidden dependency/import/TTY/parser-flag checks; the targeted policy suite passes |

**Score:** 6/6 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/gin-index/experiment.go` | command runner, ingest orchestration, flags, sidecar writing, log-level wiring | VERIFIED | Supports `--json`, `--test`, `-o`, `--log-level`, `--sample`, and `--on-error` |
| `cmd/gin-index/experiment_output.go` | shared report structs and text/JSON rendering | VERIFIED | Defines `experimentReport`, `collectExperimentPathRows`, `estimateBloomOccupancy`, `writeExperimentText`, and `writeExperimentJSON` |
| `cmd/gin-index/experiment_test.go` | end-to-end command regression coverage | VERIFIED | Contains foundation, rich-output, sampling, and malformed-input coverage |
| `cmd/gin-index/experiment_policy_test.go` | executable charter guards | VERIFIED | Checks forbidden dependencies/imports/TTY logic and parser-flag exposure |
| `cmd/gin-index/main.go` | dispatcher + usage integration | VERIFIED | `case "experiment":` present and usage includes file/stdin examples |
| `cmd/gin-index/main_test.go` | parse-failure helper integration | VERIFIED | `experiment` included in shared parse-failure coverage |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Rich-output regression slice | `go test ./cmd/gin-index -run 'Test(RunExperimentJSONGolden|RunExperimentPredicateReport(Text|JSON)|RunExperimentWritesSidecarRoundTrip|RunExperimentRejectsNonGinOutput|RunExperimentLogLevelWritesOnlyToStderr)$' -count=1` | PASS | PASS |
| Sampling/error-handling slice | `go test ./cmd/gin-index -run 'Test(RunExperimentOnError(Abort|Continue)|RunExperimentSampleLimit|RunExperimentRejectsInvalidOnErrorValue)$' -count=1` | PASS | PASS |
| Policy guard slice | `go test ./cmd/gin-index -run 'TestExperiment(CommandHasNoForbidden(Dependencies|Imports|TTYLogic)|UsageDoesNotExposeParserFlag)$' -count=1` | PASS | PASS |
| Full command package | `go test ./cmd/gin-index -count=1` | PASS | PASS |
| Full repository | `go test ./...` | PASS | PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|----------------|-------------|--------|----------|
| CLI-01 | 15-01 | `experiment` subcommand accepts file path or `-` | SATISFIED | dispatcher + usage wiring; file/stdin tests pass |
| CLI-02 | 15-01, 15-02 | per-path summary reuses existing info rendering and shared report model | SATISFIED | `writeExperimentText(...)` uses `writeIndexInfo(...)`; JSON report shares same path rows |
| CLI-03 | 15-01, 15-03 | streaming ingest, large-line support, sample/on-error semantics | SATISFIED | `ReadBytes('\n')` ingest; large-line/sample/on-error tests pass |
| CLI-04 | 15-02 | optional sidecar write | SATISFIED | sidecar roundtrip test passes |
| CLI-05 | 15-02 | stable JSON schema | SATISFIED | JSON golden test passes |
| CLI-06 | 15-02 | inline predicate tester | SATISFIED | text/JSON predicate tests pass |
| CLI-07 | 15-03 | line-level abort/continue handling | SATISFIED | abort/continue tests pass |
| CLI-08 | 15-03 | sample limit and charter guardrails | SATISFIED | sample-limit and policy tests pass |

### Gaps Summary

No gaps. All Phase 15 roadmap truths are implemented and covered by automated verification. The command remains stdlib-first, bounded-memory for non-sampled paths, direct-stream for sampled runs, and protected against CLI drift through executable policy tests.

---

_Verified: 2026-04-22T16:38:37Z_
_Verifier: Codex_
