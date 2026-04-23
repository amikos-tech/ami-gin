---
phase: 16
slug: adddocument-atomicity-lucene-contract
status: ready
nyquist_compliant: true
wave_0_complete: false
created: 2026-04-23
---

# Phase 16 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` plus `github.com/leanovate/gopter` |
| **Config file** | `.golangci.yml` for lint; no separate Go test config |
| **Quick run command** | `go test -run 'Test(AddDocument|ValidateStagedPaths|RunMerge|Atomicity)' ./...` |
| **Full suite command** | `make test` |
| **Estimated runtime** | Focused tests under 60s; full suite up to 30m timeout |

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
| 16-01-01 | 01 | 1 | ATOMIC-02 | T16-01 | Validator rejects lossy mixed numeric promotion before merge mutates shared state | unit | `go test -run TestValidateStagedPaths ./...` | Partial | pending |
| 16-01-02 | 01 | 1 | ATOMIC-02 | T16-04 | Functions marked `MUST_BE_CHECKED_BY_VALIDATOR` have no `error` return | static | `make lint` | Missing | pending |
| 16-02-01 | 02 | 1 | ATOMIC-03 | T16-02 | Recovered merge panic becomes `tragicErr` and later `AddDocument` is refused | unit | `go test -run 'Test(RunMergeWithRecover|AddDocumentRefusesAfterTragic)' ./...` | Missing | pending |
| 16-02-02 | 02 | 1 | ATOMIC-03 | T16-03 | Recovery logging uses the silent-by-default logger seam and avoids INFO-level raw value leakage | unit | `go test -run 'Test.*Tragic.*Log|Test.*Observability.*Policy' ./...` | Missing | pending |
| 16-03-01 | 03 | 2 | ATOMIC-03 | T16-01 | Public user-input failure catalog leaves `tragicErr == nil` | unit matrix | `go test -run TestAddDocumentPublicFailuresDoNotSetTragicErr ./...` | Missing | pending |
| 16-03-02 | 03 | 2 | ATOMIC-01 | T16-01 | Failed documents do not change byte-encoded finalized index compared with clean subset | property | `go test -run TestAddDocumentAtomicity ./...` | Missing | pending |
| 16-03-03 | 03 | 2 | ATOMIC-01 | T16-01 | Clean corpus encodes deterministically across independent builds | unit sanity | `go test -run TestAddDocumentAtomicityEncodeDeterminism ./...` | Missing | pending |
| 16-04-01 | 04 | 2 | ATOMIC-02 | T16-04 | CI executes the marker/signature check, not only local lint | static/CI | `make lint` plus inspect `.github/workflows/ci.yml` for the marker check path | Missing | pending |

*Status: pending / green / red / flaky*

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

- [ ] `atomicity_test.go` - new tests for ATOMIC-01 and public failure catalog coverage for ATOMIC-03.
- [ ] Focused validator tests - extend existing numeric-promotion coverage for ATOMIC-02.
- [ ] Recovery tests - direct `runMergeWithRecover` panic conversion plus builder refusal after tragedy.
- [ ] Marker check - Makefile lint and CI workflow enforce no `error` return on marker functions.

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

**Approval:** approved 2026-04-23 for planning
