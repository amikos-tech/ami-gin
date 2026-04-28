---
phase: 19
slug: simd-dependency-decision-integration-strategy
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-27
updated: 2026-04-27
---

# Phase 19 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` artifact checks + shell checks |
| **Config file** | `go.mod` |
| **Quick run command** | `go test ./... -run TestPhase19SIMDStrategyArtifact` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~1 second for focused artifact checks; full suite depends on local machine |

---

## Sampling Rate

- **After every task commit:** Run the focused shell check listed above.
- **After every plan wave:** Run `go test ./...`.
- **Before `$gsd-verify-work`:** Run focused shell check and `go test ./...`.
- **Max feedback latency:** 60 seconds for focused checks.

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 19-01-01 | 01 | 1 | SIMD-01 | T19-01 | Dependency source, license/NOTICE posture, and pinning are explicit before adoption. | artifact-go | `go test ./... -run TestPhase19SIMDStrategyArtifact` | `phase19_validation_test.go` | covered |
| 19-01-02 | 01 | 1 | SIMD-02 | T19-02 | Runtime loading strategy documents env override, bootstrap delegation, platform set, asset labels, and tier-2 platform fallback. | artifact-go | `go test ./... -run TestPhase19SIMDStrategyArtifact` | `phase19_validation_test.go` | covered |
| 19-01-03 | 01 | 1 | SIMD-03 | T19-03 | Build-tag and API strategy preserve default stdlib behavior, make fallback explicit, and include the success branch using `WithParser`. | artifact-go | `go test ./... -run TestPhase19SIMDStrategyArtifact` | `phase19_validation_test.go` | covered |
| 19-01-04 | 01 | 1 | SIMD-03 | T19-04 | Stop/fallback table distinguishes correctness-breaking blockers from soft operational platform issues. | artifact-go | `go test ./... -run TestPhase19SIMDStrategyArtifact` | `phase19_validation_test.go` | covered |

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements:

- `phase19_validation_test.go` codifies the strategy artifact checks in the repository test suite.
- Shell `test` and `grep` remain useful as one-off verification commands.
- `go test ./...` remains the sanity check that planning-only changes did not disturb the repository.
- No new test framework or external service is required.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Upstream `pure-simdjson` license and release state remains acceptable at implementation time | SIMD-01/SIMD-02 | External upstream state can change after Phase 19 | During Phase 21, re-check upstream `v0.1.4` LICENSE, NOTICE, release assets, tag commit `0f53f3f2e8bb9608d6b79211ffc5fc7b53298617`, `windows-amd64-msvc` asset naming, and bootstrap docs before adding the dependency |

---

## Validation Sign-Off

- [x] All tasks have automated verify commands or Wave 0 dependencies.
- [x] Sampling continuity: no 3 consecutive tasks without automated verify.
- [x] Wave 0 covers all missing references.
- [x] No watch-mode flags.
- [x] Feedback latency target < 60s for focused checks.
- [x] `nyquist_compliant: true` set in frontmatter.

**Approval:** automated audit 2026-04-27

## Validation Audit 2026-04-27

| Metric | Count |
|--------|-------|
| Gaps found | 4 |
| Resolved | 4 |
| Escalated | 0 |

Resolved by adding `phase19_validation_test.go`, which covers SIMD-01, SIMD-02, and both SIMD-03 validation rows through durable Go artifact checks.
