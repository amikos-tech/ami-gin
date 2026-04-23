---
phase: 16
slug: adddocument-atomicity-lucene-contract
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-23
last_audit: 2026-04-23
---

# Phase 16 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` plus `github.com/leanovate/gopter` |
| **Config file** | `.golangci.yml` for lint; no separate Go test config |
| **Quick run command** | `go test -run 'Test(AddDocument|ValidateStagedPaths|RunMerge|Atomicity)' ./... -count=1` |
| **Full suite command** | `make test` |
| **Estimated runtime** | Focused tests under 10s; full suite ~46s with 30m timeout |

---

## Sampling Rate

- **After every task commit:** Run the focused `go test -run ... ./...` command that covers the changed behavior.
- **After every plan wave:** Run `go test ./...`; run `make lint` after marker or CI changes land.
- **Before `$gsd-verify-work`:** `make test`, `make lint`, and `go build ./...` must be green.
- **Max feedback latency:** One focused test command per task, full suite only at wave/phase gate.

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 16-01-01 | 01 | 1 | ATOMIC-02 | T16-01 | Validator rejects lossy mixed numeric promotion before merge mutates shared state | unit | `go test -run 'Test(ValidateStagedPathsRejectsLossyPromotionBeforeMerge|ValidateStagedPathsRejectsUnsafeIntIntoFloatPath|MixedNumericPathRejectsLossyPromotionLeavesBuilderUsable)$' ./... -count=1` | ✅ `gin_test.go` | ✅ green |
| 16-01-02 | 01 | 1 | ATOMIC-02 | T16-04 | Functions marked `MUST_BE_CHECKED_BY_VALIDATOR` have no `error` return | static | `make check-validator-markers && make lint` | ✅ `builder.go`, ✅ `Makefile` | ✅ green |
| 16-02-01 | 02 | 1 | ATOMIC-03 | T16-02 | Recovered merge panic becomes `tragicErr` and later `AddDocument` is refused | unit | `go test -run 'Test(RunMergeWithRecoverConvertsPanicToTragicError|AddDocumentRefusesAfterRecoveredMergePanic|AddDocumentRefusesAfterTragicFailure)$' ./... -count=1` | ✅ `builder.go`, ✅ `gin_test.go` | ✅ green |
| 16-02-02 | 02 | 1 | ATOMIC-03 | T16-03 | Recovery logging uses the silent-by-default logger seam and avoids INFO-level raw value leakage | unit | `go test -run 'TestRunMergeWithRecoverLogsThroughLoggerWithoutPanicValue$' ./... -count=1` | ✅ `builder.go`, ✅ `gin_test.go` | ✅ green |
| 16-03-01 | 03 | 2 | ATOMIC-03 | T16-01 | Public user-input failure catalog leaves `tragicErr == nil` | unit matrix | `go test -run 'Test(AddDocumentPublicFailuresDoNotSetTragicErr|AddDocumentRejectsUnsupportedNumberWithoutPartialMutation)$' ./... -count=1` | ✅ `atomicity_test.go` | ✅ green |
| 16-03-02 | 03 | 2 | ATOMIC-01 | T16-01 | Failed documents do not change byte-encoded finalized index compared with clean subset | property | `go test -run 'TestAddDocumentAtomicity$' ./... -count=1` | ✅ `atomicity_test.go` | ✅ green |
| 16-03-03 | 03 | 2 | ATOMIC-01 | T16-01 | Clean corpus encodes deterministically across independent builds | unit sanity | `go test -run 'TestAddDocumentAtomicityEncodeDeterminism$' ./... -count=1` | ✅ `atomicity_test.go` | ✅ green |
| 16-04-01 | 04 | 2 | ATOMIC-02 | T16-04 | CI executes the marker/signature check, not only local lint | static/CI | `make check-validator-markers && make lint` plus inspect `.github/workflows/ci.yml` for the marker check path | ✅ `Makefile`, ✅ `.github/workflows/ci.yml` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠ accepted/manual review*

---

## Threat Model References

| Threat Ref | Pattern | Mitigation Required In Plans |
|------------|---------|------------------------------|
| T16-01 | Failed document partially mutates builder indexes, bloom/HLL/trigram state, numeric stats, or doc bookkeeping | Validate all user-reachable failures before merge; assert byte-identical `Encode` for full-vs-clean corpora |
| T16-02 | Panic during merge crashes caller process | Recover at the `mergeStagedPaths` boundary, convert to `tragicErr`, and refuse later writes |
| T16-03 | Panic-value logging leaks raw document content | Keep default logger noop; keep recovery logging at Error level and avoid broad raw document attrs |
| T16-04 | Static invariant check exists locally but is absent from CI | Wire marker/signature check into both `make lint` and `.github/workflows/ci.yml` |

---

## Wave 0 Requirements

- [x] `atomicity_test.go` - new tests for ATOMIC-01 and public failure catalog coverage for ATOMIC-03.
- [x] Focused validator tests - extend existing numeric-promotion coverage for ATOMIC-02.
- [x] Recovery tests - direct `runMergeWithRecover` panic conversion plus builder refusal after tragedy.
- [x] Marker check - Makefile lint and CI workflow enforce no `error` return on marker functions.

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Sign-Off

- [x] All tasks have automated verify commands or explicit Wave 0 dependencies.
- [x] Sampling continuity: no three consecutive tasks without automated verify.
- [x] Wave 0 covers all missing references.
- [x] No watch-mode flags.
- [x] Feedback latency is bounded by focused Go tests per task.
- [x] `nyquist_compliant: true` set in frontmatter.

**Approval:** approved 2026-04-23 after validation audit

---

## Validation Audit 2026-04-23

| Metric | Count |
|--------|-------|
| Tasks audited | 8 |
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

Audit evidence:

- Confirmed State A input: existing `16-VALIDATION.md`, all four Phase 16 plan files, all four Phase 16 summary artifacts, and `16-VERIFICATION.md` are present.
- Confirmed Nyquist validation is enabled in `.planning/config.json` via `workflow.nyquist_validation: true`.
- Rebuilt the requirement-to-task map from `16-01-PLAN.md` through `16-04-PLAN.md` and the corresponding summaries for ATOMIC-01, ATOMIC-02, and ATOMIC-03.
- Cross-referenced shipped test coverage in `gin_test.go` and `atomicity_test.go`, plus static policy coverage in `Makefile` and `.github/workflows/ci.yml`.
- Re-ran focused verification commands for validator coverage, tragic recovery, public failure catalog coverage, atomicity encode determinism, the byte-identical atomicity property, and validator marker enforcement; all passed.
- Re-ran the phase integration gate: `make lint` passed with 0 issues, `go build ./...` passed, and `make test` passed with 871 tests and 1 skipped in 46.258s.
- No COVERED/PARTIAL/MISSING gaps remained after the live cross-reference, so the gap-fix gate and `gsd-nyquist-auditor` spawn path were not needed for Phase 16.
- No new test files were required by this audit; existing Phase 16 test files already satisfy the Nyquist coverage contract.
