---
phase: 18
slug: structured-ingesterror-cli-integration
status: green
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-24
---

# Phase 18 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` |
| **Config file** | `go.mod` |
| **Quick run command** | `go test ./... -run 'Test(IngestError|HardIngestFailures|RunExperiment.*IngestFailure|CheckIngestErrorWrapping)' -count=1` |
| **Full suite command** | `go test ./... && make lint` |
| **Estimated runtime** | ~20-60 seconds for focused tests; full runtime depends on local lint tooling |

---

## Sampling Rate

- **After every task commit:** Run the focused command listed above or the narrower command in the task.
- **After every plan wave:** Run `go test ./...`.
- **Before `$gsd-verify-work`:** Run `go test ./... && make lint`.
- **Max feedback latency:** 60 seconds for focused checks.

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 18-01-01 | 01 | 1 | IERR-01 | T18-01 | `IngestError` preserves underlying cause and structured fields without redacting `Value`. | unit | `go test ./... -run 'TestIngestError' -count=1` | yes | green |
| 18-01-02 | 01 | 1 | IERR-02 | T18-02 | Parser, transformer, numeric, and schema hard failures return `*IngestError`; parser contract/tragic failures do not. | unit | `go test ./... -run 'Test(HardIngestFailuresReturnIngestError|ParserContractErrorsRemainNonIngestError|SoftFailureModesDoNotReturnIngestError)' -count=1` | yes | green |
| 18-02-01 | 02 | 1 | IERR-02 | T18-03 | Scoped guard catches plain hard-ingest errors without blocking legitimate internal errors. | static/unit | `make lint` | yes | green |
| 18-03-01 | 03 | 2 | IERR-03 | T18-04 | CLI groups returned `IngestError`s by layer with counts and at most 3 samples per layer in JSON. | integration | `go test ./cmd/gin-index -run 'TestRunExperiment.*IngestFailure.*JSON|TestRunExperimentHundredDocsKnownIngestFailures' -count=1` | yes | green |
| 18-03-02 | 03 | 2 | IERR-03 | T18-04 | CLI text mode reports layer counts and structured samples while accepted docs remain densely packed. | integration | `go test ./cmd/gin-index -run 'TestRunExperiment.*IngestFailure.*Text|TestRunExperiment.*DensePacking' -count=1` | yes | green |
| 18-04-01 | 04 | 3 | IERR-01/IERR-03 | T18-05 | Public docs and changelog warn that `Value` is verbatim and caller-redacted. | docs/full | `go test ./... && make lint` | yes | green |

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements:

- `go test ./...` already runs root-package and CLI package tests.
- `make lint` already runs scoped validator-marker enforcement before `golangci-lint`.
- No new test framework or external service is required.

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Sign-Off

- [x] All tasks have automated verify commands or Wave 0 dependencies.
- [x] Sampling continuity: no 3 consecutive tasks without automated verify.
- [x] Wave 0 covers all missing references.
- [x] No watch-mode flags.
- [x] Feedback latency target < 60s for focused checks.
- [x] `nyquist_compliant: true` set in frontmatter.

**Approval:** approved 2026-04-24

---

## Execution Results

- `go test ./... -run 'Test(IngestErrorWrappingContract|HardIngestFailuresReturnIngestError|ParserContractErrorsRemainNonIngestError|SoftFailureModesDoNotReturnIngestError|HardIngestFunctionsDoNotReturnPlainErrors)$' -count=1` - PASS
- `go test ./cmd/gin-index -run 'TestRunExperiment(OnErrorContinue|OnErrorContinueMalformedJSONFromFile|OnErrorContinueIngestFailuresJSON|HundredDocsKnownIngestFailuresJSON|OnErrorAbort.*Ingest)' -count=1` - PASS
- `go test ./...` - PASS
- `make lint` - PASS

All Phase 18 validation rows are green. The lint gate initially reported local code findings in Phase 18 tests; those were fixed and `make lint` was rerun successfully.
