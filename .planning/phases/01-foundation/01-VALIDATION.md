---
phase: 1
slug: foundation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-24
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | none — Go test is built-in |
| **Quick run command** | `go test -v -count=1 ./...` |
| **Full suite command** | `go test -v -count=1 ./... && go build ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go build ./...`
- **After every plan wave:** Run `go test -v -count=1 ./... && go build ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 1 | FOUND-01 | file check | `test -f LICENSE` | ❌ W0 | ⬜ pending |
| 1-01-02 | 01 | 1 | FOUND-02 | build | `go build ./...` | ✅ | ⬜ pending |
| 1-01-03 | 01 | 1 | FOUND-03 | grep | `grep -r "gin-index" --include="*.go" . \| wc -l` | ✅ | ⬜ pending |
| 1-01-04 | 01 | 1 | FOUND-04 | grep | `grep -rn "Kiba\|gin-index-prd" . --include="*.go" --include="*.md" \| grep -v .planning/` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- Existing infrastructure covers all phase requirements.

*No new test infrastructure needed — validation uses `go build`, `go test`, `grep`, and `test` commands.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `go get github.com/amikos-tech/ami-gin` resolves | FOUND-02 | Requires published module on GitHub | Push to GitHub, run `go get` from a clean GOPATH |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
