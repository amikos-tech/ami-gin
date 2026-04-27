---
phase: 2
slug: security-hardening
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-26
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | none — standard Go test infrastructure |
| **Quick run command** | `go test -v -run TestDecode` |
| **Full suite command** | `go test -v` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -v -run TestDecode`
- **After every plan wave:** Run `go test -v`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 02-01-01 | 01 | 1 | SEC-02 | unit | `go test -v -run TestDecodeVersionMismatch` | ❌ W0 | ⬜ pending |
| 02-01-02 | 01 | 1 | SEC-02 | unit | `go test -v -run TestDecodeLegacyFallbackRemoved` | ❌ W0 | ⬜ pending |
| 02-02-01 | 02 | 1 | SEC-01 | unit | `go test -v -run TestDecodeBoundsCheck` | ❌ W0 | ⬜ pending |
| 02-02-02 | 02 | 1 | SEC-03 | unit | `go test -v -run TestDecodeCraftedPayload` | ❌ W0 | ⬜ pending |
| 02-02-03 | 02 | 1 | SEC-01 | regression | `go test -v` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Security-specific decode tests for version mismatch, bounds overflow, and crafted payloads
- [ ] Existing `serialize_test.go` infrastructure covers regression testing

*Existing infrastructure covers regression requirements. New tests needed for security-specific scenarios.*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
