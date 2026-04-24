---
phase: 15
slug: experimentation-cli
status: verified
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-22
updated: 2026-04-22T16:54:55Z
---

# Phase 15 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Source of record: `15-01-PLAN.md`, `15-02-PLAN.md`, `15-03-PLAN.md`, the corresponding summary artifacts, and the current-tree audit evidence below.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` plus `.planning/` artifact inspection |
| **Config file** | `go.mod` (no extra framework config) |
| **Quick run command** | `go test ./cmd/gin-index -run 'Test(RunExperiment|Experiment)' -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | quick run ~1s, full suite ~45s |

---

## Sampling Rate

- **After every task commit:** Run the task-specific command from the verification map below.
- **After every plan wave:** Run `go test ./cmd/gin-index -run 'Test(RunExperiment|Experiment)' -count=1`.
- **Before `$gsd-verify-work`:** `go test ./... -count=1` must be green.
- **Max feedback latency:** 45 seconds for repo-local validation.

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 15-01-01 | 01 | 1 | CLI-01 | T-15-01 / T-15-02 | Command accepts a file path or `-`, rejects invalid `--rg-size`, and rejects directory inputs before ingest | unit | `go test ./cmd/gin-index -run 'Test(RunExperimentFrom(File|Stdin)|RunExperimentRejectsInvalidRGSize|RunExperimentRejectsDirectoryInput)$' -count=1` | ✅ `cmd/gin-index/experiment.go`, ✅ `cmd/gin-index/experiment_test.go` | ✅ green |
| 15-01-02 | 01 | 1 | CLI-02, CLI-03 | T-15-01 / T-15-04a / T-15-04b | Streaming ingest handles large lines without truncation, accepts a final line without a trailing newline, handles empty input coherently, and prints the summary before the path table | unit | `go test ./cmd/gin-index -run 'Test(RunExperimentFinalLineWithoutTrailingNewline|RunExperimentEmptyInput|RunExperimentLargeLineNoTruncation|RunExperimentTextOutputOrder)$' -count=1` | ✅ `cmd/gin-index/experiment.go`, ✅ `cmd/gin-index/experiment_test.go` | ✅ green |
| 15-02-01 | 02 | 2 | CLI-04, CLI-05 | T-15-05 / T-15-06 | JSON schema is stable, `predicate_test` is omitted when unset, and sidecar output is readable via the `.gin` contract | unit | `go test ./cmd/gin-index -run 'Test(RunExperimentJSONGolden|RunExperimentWritesSidecarRoundTrip|RunExperimentRejectsNonGinOutput)$' -count=1` | ✅ `cmd/gin-index/experiment.go`, ✅ `cmd/gin-index/experiment_output.go`, ✅ `cmd/gin-index/experiment_test.go` | ✅ green |
| 15-02-02 | 02 | 2 | CLI-06 | T-15-04 | Predicate test reports matched/pruned counts and ratio without leaking RG IDs | unit | `go test ./cmd/gin-index -run 'Test(RunExperimentPredicateReport(Text|JSON)|RunExperimentLogLevelWritesOnlyToStderr)$' -count=1` | ✅ `cmd/gin-index/experiment.go`, ✅ `cmd/gin-index/experiment_output.go`, ✅ `cmd/gin-index/experiment_test.go` | ✅ green |
| 15-03-01 | 03 | 3 | CLI-07 | T-15-07 / T-15-08 | Continue mode streams `line N` diagnostics to `stderr`; abort mode exits on the first bad line; invalid `--on-error` values fail before ingest | unit | `go test ./cmd/gin-index -run 'Test(RunExperimentOnError(Abort|Continue)|RunExperimentRejectsInvalidOnErrorValue)$' -count=1` | ✅ `cmd/gin-index/experiment.go`, ✅ `cmd/gin-index/experiment_test.go` | ✅ green |
| 15-03-02 | 03 | 3 | CLI-08 | T-15-08 | Sample limit caps successfully ingested documents, not raw lines | unit | `go test ./cmd/gin-index -run 'TestRunExperimentSampleLimit$' -count=1` | ✅ `cmd/gin-index/experiment.go`, ✅ `cmd/gin-index/experiment_test.go` | ✅ green |
| 15-03-03 | 03 | 3 | CLI-01..08 | T-15-09 | No forbidden CLI deps, TUI imports, colour or TTY detection logic, or `--parser` exposure land in the phase | unit | `go test ./cmd/gin-index -run 'TestExperiment(CommandHasNoForbidden(Dependencies|Imports|TTYLogic)|UsageDoesNotExposeParserFlag)$' -count=1` | ✅ `cmd/gin-index/experiment_policy_test.go`, ✅ `cmd/gin-index/main.go` | ✅ green |

*Status: ⬜ pending - ✅ green - ❌ red - ⚠ accepted/manual review*

---

## Wave 0 Requirements

- [x] Existing `go test` infrastructure already covers `cmd/gin-index`
- [x] Existing CLI test style (`bytes.Buffer`, `t.TempDir`, subprocess parse helper) is sufficient
- [x] Dedicated experiment command tests exist in `cmd/gin-index/experiment_test.go`
- [x] Charter and policy guard tests exist in `cmd/gin-index/experiment_policy_test.go`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Human readability of text summary | CLI-02 | Layout quality is easier to assess with one sample run than with string-fragment assertions alone | Create a temporary JSONL fixture with 2-3 documents, run `go run ./cmd/gin-index experiment <fixture>` and confirm the order is summary -> path table -> optional predicate block |

---

## Validation Audit 2026-04-22

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

Audit evidence:
- Confirmed State A input: existing `15-VALIDATION.md`, all three Phase 15 plan files, all three Phase 15 summary artifacts, and `15-VERIFICATION.md` were present before the audit began.
- Confirmed Nyquist validation remains enabled by default because `.planning/config.json` is absent and the local GSD config template sets `workflow.nyquist_validation` to `true` when unspecified.
- Re-read `15-01-PLAN.md`, `15-02-PLAN.md`, `15-03-PLAN.md`, `15-01-SUMMARY.md`, `15-02-SUMMARY.md`, `15-03-SUMMARY.md`, and `15-VERIFICATION.md` to rebuild the Phase 15 requirement-to-task map for CLI-01 through CLI-08.
- Cross-referenced the live proof surface in `cmd/gin-index/main.go`, `cmd/gin-index/main_test.go`, `cmd/gin-index/experiment.go`, `cmd/gin-index/experiment_output.go`, `cmd/gin-index/experiment_test.go`, and `cmd/gin-index/experiment_policy_test.go`; no missing product-test files were found.
- Re-ran targeted experiment and policy coverage with `go test ./cmd/gin-index -run 'Test(RunExperiment|Experiment)' -count=1`, which passed with `ok github.com/amikos-tech/ami-gin/cmd/gin-index 0.358s`.
- Re-ran the repo-wide regression sweep with `go test ./... -count=1`, which passed in `42.950s` for `github.com/amikos-tech/ami-gin`, `0.675s` for `github.com/amikos-tech/ami-gin/cmd/gin-index`, and `0.313s` for `github.com/amikos-tech/ami-gin/telemetry`.
- No COVERED/PARTIAL/MISSING gaps remained after the live cross-reference, so the gap-fix user gate and `gsd-nyquist-auditor` spawn path were not needed for Phase 15.
- Closed the stale validation-contract gap without adding new product tests: the pre-execution draft table now reflects the shipped test files, green commands, and the verified state of the completed phase.
- No new test files were required for Phase 15. Existing command tests, policy guard tests, and the repo-wide green suite already satisfy the completed phase.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 45s for repo-local checks
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-04-22
