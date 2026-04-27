---
phase: 19
slug: simd-dependency-decision-integration-strategy
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-27
---

# Phase 19 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Shell checks + Go `testing` sanity |
| **Config file** | `go.mod` |
| **Quick run command** | `test -f .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md && grep -q 'SIMD-01' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md && grep -q 'SIMD-02' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md && grep -q 'SIMD-03' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md && grep -q 'github.com/amikos-tech/pure-simdjson v0.1.4' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md && grep -q 'NewSIMDParser() (Parser, error)' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md && grep -q '//go:build simdjson' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md && grep -q 'PURE_SIMDJSON_LIB_PATH' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md` |
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
| 19-01-01 | 01 | 1 | SIMD-01 | T19-01 | Dependency source, license/NOTICE posture, and pinning are explicit before adoption. | artifact | `grep -q 'github.com/amikos-tech/pure-simdjson v0.1.4' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md` | W0 | pending |
| 19-01-02 | 01 | 1 | SIMD-02 | T19-02 | Runtime loading strategy documents env override, bootstrap delegation, platform set, and tier-2 platform fallback. | artifact | `grep -q 'PURE_SIMDJSON_LIB_PATH' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md && grep -q 'linux/amd64' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md && grep -q 'windows/amd64' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md` | W0 | pending |
| 19-01-03 | 01 | 1 | SIMD-03 | T19-03 | Build-tag and API strategy preserve default stdlib behavior and make fallback explicit. | artifact | `grep -q '//go:build simdjson' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md && grep -q 'NewSIMDParser() (Parser, error)' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md && grep -q 'WithParser' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md` | W0 | pending |
| 19-01-04 | 01 | 1 | SIMD-03 | T19-04 | Stop/fallback table distinguishes correctness-breaking blockers from soft operational platform issues. | artifact | `grep -q 'HARD' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md && grep -q 'SOFT' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md` | W0 | pending |

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements:

- Shell `test` and `grep` are enough to verify the strategy artifact.
- `go test ./...` remains the sanity check that planning-only changes did not disturb the repository.
- No new test framework or external service is required.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Upstream `pure-simdjson` license and release state remains acceptable at implementation time | SIMD-01/SIMD-02 | External upstream state can change after Phase 19 | During Phase 21, re-check upstream `v0.1.4` LICENSE, NOTICE, release assets, and bootstrap docs before adding the dependency |

---

## Validation Sign-Off

- [x] All tasks have automated verify commands or Wave 0 dependencies.
- [x] Sampling continuity: no 3 consecutive tasks without automated verify.
- [x] Wave 0 covers all missing references.
- [x] No watch-mode flags.
- [x] Feedback latency target < 60s for focused checks.
- [x] `nyquist_compliant: true` set in frontmatter.

**Approval:** pending
