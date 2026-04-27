---
phase: 5
slug: release
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-04-13
---

# Phase 5 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` smoke plus exact `rg` / `gh` / `git` assertions for release config, docs, tag-triggered workflow behavior, and consumer installability |
| **Config file** | `.goreleaser.yml`, `.github/workflows/release.yml`, `README.md`, `Makefile` |
| **Quick run command** | `go test ./... -run TestQueryEQ -count=1 && make help` |
| **Full suite command** | `go test -v && make help` |
| **Estimated runtime** | quick run <15s, full suite ~30s; release rehearsal and GitHub verification are slower external checkpoints |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -run TestQueryEQ -count=1 && make help`
- **After release-config tasks:** Run the quick smoke plus the exact `rg` checks below for `.goreleaser.yml`, `.github/workflows/release.yml`, and `README.md`
- **After the release-preflight task:** Run `goreleaser check` and a snapshot rehearsal before any real tag is pushed
- **Before `/gsd-verify-work`:** Full suite must be green, static file checks must pass, and the tag-triggered release plus clean-environment `go get` validation must be complete
- **Max feedback latency:** 30 seconds for repo-local smoke checks; GitHub/tag verification occurs only at the human-gated release checkpoint

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 05-01-01 | 01 | 1 | REL-02 | T-05-01 | `.github/workflows/release.yml` is a dedicated semver-tag workflow with `contents: write`, `fetch-depth: 0`, aligned Go setup, and a GoReleaser action step that runs `release --clean` | static + smoke | `test -f .github/workflows/release.yml && rg -n '^name: Release$' .github/workflows/release.yml && rg -n '^on:$' .github/workflows/release.yml && rg -n '^\\s+push:$' .github/workflows/release.yml && rg -n \"^\\s+tags:\\s*$\" .github/workflows/release.yml && rg -n \"- 'v\\*'\" .github/workflows/release.yml && rg -n '^permissions:$' .github/workflows/release.yml && rg -n '^\\s+contents: write$' .github/workflows/release.yml && rg -n 'uses: actions/checkout@v6' .github/workflows/release.yml && rg -n 'fetch-depth: 0' .github/workflows/release.yml && rg -n 'uses: actions/setup-go@v6' .github/workflows/release.yml && rg -n 'uses: goreleaser/goreleaser-action@v7' .github/workflows/release.yml && rg -n 'args: release --clean' .github/workflows/release.yml && go test ./... -run TestQueryEQ -count=1 && make help` | ❌ W0 | ⬜ pending |
| 05-01-02 | 01 | 1 | REL-02 | T-05-02 | `.goreleaser.yml` stays library-only, groups changelog output into adopter-readable sections, and preserves a short human preface without shipping CLI artifacts | static + config | `test -f .goreleaser.yml && rg -n '^builds:$' .goreleaser.yml && rg -n '^\\s+-\\s+skip: true$' .goreleaser.yml && rg -n '^changelog:$' .goreleaser.yml && rg -n '^\\s+groups:$' .goreleaser.yml && rg -n 'Features|Fixes|Docs|CI/Release|Dependencies' .goreleaser.yml && rg -n '^release:$' .goreleaser.yml && rg -n 'owner:\\s*amikos-tech' .goreleaser.yml && rg -n 'name:\\s*ami-gin' .goreleaser.yml && rg -n 'header:' .goreleaser.yml && goreleaser check` | ❌ W0 | ⬜ pending |
| 05-01-03 | 01 | 1 | REL-03 | T-05-03 | `README.md` documents the locked `Known limitations` scope with exactly one section and exactly three bullets for the release-critical caveats | static | `rg -n '^## Known limitations$' README.md && limits_block=\"$(sed -n '/^## Known limitations$/,/^## /p' README.md)\" && test \"$(printf '%s\n' \"$limits_block\" | rg -n '^-' | wc -l | tr -d ' ')\" = '3' && printf '%s\n' \"$limits_block\" | rg -n 'OR/AND composites' && printf '%s\n' \"$limits_block\" | rg -n 'index merge' && printf '%s\n' \"$limits_block\" | rg -n 'query-time transformers'` | ✅ | ⬜ pending |
| 05-02-01 | 02 | 2 | REL-01, REL-02 | T-05-04 / T-05-05 | The release candidate SHA is rehearsed with a clean tree and GoReleaser validation before the real tag is pushed, so the first public release is not blind or ad hoc | static + rehearsal | `git diff --quiet && goreleaser check && goreleaser release --snapshot && git describe --tags --abbrev=0 2>/dev/null || true` | N/A checkpoint | ⬜ pending |
| 05-02-02 | 02 | 2 | REL-01 | T-05-06 | Before `v0.1.0` is pushed, the repository is publicly reachable at `https://github.com/amikos-tech/ami-gin` so unauthenticated consumers can actually satisfy the release contract | external-state | `gh repo view amikos-tech/ami-gin --json isPrivate,url && curl -I https://github.com/amikos-tech/ami-gin` | N/A external | ⬜ pending |
| 05-02-03 | 02 | 2 | REL-01 | T-05-07 | After `v0.1.0` is pushed, GitHub runs the tag-triggered workflow successfully, a GitHub Release named `v0.1.0` exists, and a clean consumer install works | external-state | `gh run list --workflow release.yml --limit 1 --json headBranch,headSha,event,status,conclusion,displayTitle && gh release view v0.1.0 && tmpdir=\"$(mktemp -d)\" && cd \"$tmpdir\" && go mod init release-check && GOPROXY=https://proxy.golang.org go get github.com/amikos-tech/ami-gin@v0.1.0` | N/A external | ⬜ pending |

*Status: ⬜ pending - ✅ green - ❌ red - ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `.github/workflows/release.yml` must exist before tag-trigger and workflow-shape checks can pass
- [ ] `.goreleaser.yml` must exist before `goreleaser check` and snapshot rehearsal can run
- [ ] `README.md` must contain `## Known limitations` before REL-03 validation can pass
- [ ] A GoReleaser CLI must be available to the executor environment before tasks `05-01-02` and `05-02-01`
- [ ] GitHub CLI auth or equivalent repository access must be available before tasks `05-02-02` and `05-02-03`
- [ ] The repository must be publicly reachable before the real `v0.1.0` tag is pushed, otherwise the clean `go get` acceptance check cannot pass
- [ ] The release-prep branch must be merged to `main` before the real `v0.1.0` tag is pushed

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Make the repository publicly reachable before the real release tag is pushed | REL-01 | The public GitHub URL currently returns `404`, so a private repository can create a release while still failing the actual public-consumption goal | Before pushing `v0.1.0`, confirm the repository is public or otherwise unauthenticatedly reachable at `https://github.com/amikos-tech/ami-gin`, then verify `gh repo view amikos-tech/ami-gin --json isPrivate,url` reports `isPrivate: false` or an equivalent public-access signal |
| Merge release-prep work to `main` and confirm the default-branch CI state before tagging | REL-01 | The public tag must come from the intended default-branch state, and Phase 4 still leaves a human checkpoint in `STATE.md` | Merge the Phase 5 prep PR, then verify the latest `CI` run on `main` completed successfully before pushing `v0.1.0` |
| Push the real `v0.1.0` tag only after rehearsal succeeds | REL-01, REL-02 | Tag creation and public release publication are irreversible enough that they should stay explicitly human-gated | After `goreleaser check` and snapshot rehearsal pass on the target SHA, push `v0.1.0` and observe the tag-triggered workflow to completion |
| Validate clean consumer install after the release exists | REL-01 | The public contract is the Go module, so an external install check is the final truth for the release | In a fresh temp directory with a clean module cache, run `go mod init release-check` and `GOPROXY=https://proxy.golang.org go get github.com/amikos-tech/ami-gin@v0.1.0` after the GitHub Release is visible |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s for repo-local checks
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
