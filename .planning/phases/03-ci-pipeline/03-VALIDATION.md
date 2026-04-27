---
phase: 3
slug: ci-pipeline
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-04-11
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` plus `gotestsum` for CI-style output |
| **Config file** | `Makefile` and `.golangci.yml` |
| **Quick run command** | `actionlint .github/workflows/*.yml && go test ./... -run TestQueryEQ -count=1` |
| **Full suite command** | `actionlint .github/workflows/*.yml && golangci-lint run && govulncheck ./... && $(go env GOPATH)/bin/gotestsum --format short-verbose --packages="./..." --junitfile unit.xml -- -race -coverprofile=coverage.out -timeout=30m ./...` |
| **Estimated runtime** | quick run <30s, full suite ~1500s |

---

## Sampling Rate

- **After every task commit:** Run `actionlint .github/workflows/*.yml && go test ./... -run TestQueryEQ -count=1`
- **After every plan wave:** Run `actionlint .github/workflows/*.yml && golangci-lint run && govulncheck ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds for task smoke, ~1500 seconds for wave-end full suite

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 03-01-01 | 01 | 1 | CI-01 | T-03-01 | Makefile exposes the required local CI verbs, pins `gotestsum`, and runs the literal `govulncheck ./...` command | static + smoke | `make help && make clean && make integration-test && make security-scan` | ✅ | ⬜ pending |
| 03-01-02 | 01 | 1 | CI-02 | T-03-02 | golangci-lint v2 enables the required linter set while the core library/test findings from `errorlint`, `unconvert`, and `unparam` are fixed | static + smoke | `set -euo pipefail; for linter in dupword errorlint goconst gocritic gosec mirror nilerr prealloc unconvert unparam; do rg -n "^[[:space:]]+- ${linter}$" .golangci.yml; done; rg -n '^version: "2"$' .golangci.yml; rg -n 'prefix\(github.com/amikos-tech/ami-gin\)' .golangci.yml; rg -n 'path: _test\\\.go' .golangci.yml; rg -n 'path: examples/' .golangci.yml; rg -n 'errors.Is\(err, io.EOF\)' parquet.go; rg -n 'errors.Is\(err, io.EOF\)' serialize.go; golangci-lint run --disable-all --enable errorlint --enable unconvert --enable unparam ./...` | ✅ | ⬜ pending |
| 03-01-03 | 01 | 1 | CI-02 | T-03-02 | CLI/example lint findings are remediated without broad suppressions and the repo passes the full expanded lint gate | static + smoke | `set -euo pipefail; rg -n '0600' cmd/gin-index/main.go; golangci-lint run ./cmd/gin-index/... ./examples/...; golangci-lint run` | ✅ | ⬜ pending |
| 03-01-04 | 01 | 1 | CI-01, CI-03, CI-04 | T-03-03 | `ci.yml` creates the PR/push gate with default GitHub-hosted runners, matrix test jobs, canonical single-run jobs, explicit lint bootstrap, GitHub artifact upload, and a GitHub-native coverage summary on the 1.26 leg | static + smoke | `set -euo pipefail; actionlint .github/workflows/ci.yml; rg -n '^name: CI$' .github/workflows/ci.yml; rg -n 'pull_request:' .github/workflows/ci.yml; rg -n 'push:' .github/workflows/ci.yml; rg -n 'branches: \[main\]' .github/workflows/ci.yml; test "$(rg -c '^    runs-on: ubuntu-latest$' .github/workflows/ci.yml)" = "4"; ! rg -n 'self-hosted|^[[:space:]]*group:' .github/workflows/ci.yml; rg -n 'repo-checkout: false' .github/workflows/ci.yml; rg -n 'output-format: text' .github/workflows/ci.yml; rg -n 'name: Coverage Report' .github/workflows/ci.yml; rg -n 'GITHUB_STEP_SUMMARY' .github/workflows/ci.yml; rg -n 'test-artifacts-go\$\{\{ matrix\.go-version \}\}' .github/workflows/ci.yml; ! rg -n 'codecov|Codecov' .github/workflows/ci.yml` | ❌ W0 | ⬜ pending |
| 03-02-01 | 02 | 1 | CI-03 | T-03-06 | `security.yml` provides the weekly/manual SARIF path without weakening the blocking text-mode PR gate | static | `set -euo pipefail; actionlint .github/workflows/security.yml; rg -n '^name: Security$' .github/workflows/security.yml; rg -n 'workflow_dispatch:' .github/workflows/security.yml; rg -n 'cron: "23 7 \* \* 1"' .github/workflows/security.yml; rg -n 'GOTOOLCHAIN: local' .github/workflows/security.yml; rg -n 'security-events: write' .github/workflows/security.yml; rg -n 'repo-checkout: false' .github/workflows/security.yml; rg -n 'output-format: sarif' .github/workflows/security.yml; rg -n 'output-file: govulncheck\.sarif' .github/workflows/security.yml; rg -n 'github/codeql-action/upload-sarif@v4' .github/workflows/security.yml; rg -n 'category: govulncheck' .github/workflows/security.yml; ! rg -n 'codecov|Codecov' .github/workflows/security.yml` | ❌ W0 | ⬜ pending |
| 03-02-02 | 02 | 1 | CI-01 | T-03-07 | README advertises the canonical `ci.yml` badge URL exactly once so the public CI signal stays stable | static | `rg -n 'actions/workflows/ci.yml/badge.svg\\?branch=main&event=push' README.md` | ✅ | ⬜ pending |
| 03-03-01 | 03 | 2 | CI-03 | T-03-10 | Private/public prerequisite checks are exact: repo visibility and Code Security enabled state when private are validated before SARIF and Code Scanning claims | external-state | `gh api repos/amikos-tech/ami-gin --jq '.private, .security_and_analysis.code_security.status'` | N/A external | ⬜ pending |
| 03-03-02 | 03 | 2 | CI-01, CI-04 | T-03-09 | A real `pull_request` CI run emits the exact contexts used later by the ruleset and uploads the expected gotestsum artifacts for both matrix legs | external-state | `set -euo pipefail; gh pr view --json baseRefName,state,headRefOid; gh api repos/amikos-tech/ami-gin/commits/$(gh pr view --json headRefOid -q .headRefOid)/check-runs | rg '"name":"test \(1\.25\)"'; gh api repos/amikos-tech/ami-gin/commits/$(gh pr view --json headRefOid -q .headRefOid)/check-runs | rg '"name":"test \(1\.26\)"'; gh api repos/amikos-tech/ami-gin/commits/$(gh pr view --json headRefOid -q .headRefOid)/check-runs | rg '"name":"lint"'; gh api repos/amikos-tech/ami-gin/commits/$(gh pr view --json headRefOid -q .headRefOid)/check-runs | rg '"name":"build"'; gh api repos/amikos-tech/ami-gin/commits/$(gh pr view --json headRefOid -q .headRefOid)/check-runs | rg '"name":"govulncheck"'; gh api repos/amikos-tech/ami-gin/actions/runs/$(gh run list --workflow ci.yml --event pull_request --branch $(git branch --show-current) --limit 1 --json databaseId --jq '.[0].databaseId')/artifacts | rg '"name":"test-artifacts-go1\.25"'; gh api repos/amikos-tech/ami-gin/actions/runs/$(gh run list --workflow ci.yml --event pull_request --branch $(git branch --show-current) --limit 1 --json databaseId --jq '.[0].databaseId')/artifacts | rg '"name":"test-artifacts-go1\.26"'` | N/A external | ⬜ pending |
| 03-03-03 | 03 | 2 | CI-01, CI-03, CI-04 | T-03-08 | After merge to `main`, the merge-triggered `CI` push run succeeds on the default branch, its gotestsum artifacts exist on the default branch run, the default-branch `Security` workflow succeeds through a real `workflow_dispatch` run, GitHub shows a visible main-branch govulncheck Code Scanning analysis, and the Main ruleset preserves the pre-existing protections while requiring only the exact PR-proven CI contexts | external-state | `set -euo pipefail; gh run list --workflow ci.yml --branch main --limit 1 --json workflowName,event,status,conclusion,headBranch,databaseId; gh api repos/amikos-tech/ami-gin/actions/runs/$(gh run list --workflow ci.yml --branch main --limit 1 --json databaseId --jq '.[0].databaseId')/artifacts | rg '"name":"test-artifacts-go1\.25"'; gh api repos/amikos-tech/ami-gin/actions/runs/$(gh run list --workflow ci.yml --branch main --limit 1 --json databaseId --jq '.[0].databaseId')/artifacts | rg '"name":"test-artifacts-go1\.26"'; gh run list --workflow security.yml --branch main --limit 1 --json workflowName,event,status,conclusion,headBranch; gh api repos/amikos-tech/ami-gin/code-scanning/analyses --paginate --jq '.[] | select(.ref == "refs/heads/main") | .ref' | rg '^refs/heads/main$'; gh api repos/amikos-tech/ami-gin/code-scanning/analyses --paginate --jq '.[] | select(.ref == "refs/heads/main") | (.tool.name // .tool.driver.name // "")' | rg '^govulncheck$'; gh api repos/amikos-tech/ami-gin/rulesets/14266305` | N/A external | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `.github/workflows/ci.yml` — create PR/push CI workflow before any workflow lint gate, PR context check, or ruleset update can pass
- [ ] `.github/workflows/security.yml` — create weekly/manual SARIF workflow before CI-03 can be validated on the default branch
- [ ] Pull request to `main` from the execution branch — required before ruleset 14266305 can safely require the exact `ci.yml` contexts
- [ ] Merge of that PR to `main` — required before `workflow_dispatch` verification of `security.yml` on the default branch
- [ ] GitHub Code Scanning enablement or repo visibility change — current private/disabled state blocks SARIF results from surfacing
- [ ] Explicit CI-equivalent `gotestsum` verification — treat this as a deliberate phase check, not an assumption from current `make test`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Pull request from Task `03-03-02` is reviewed and merged to `main` before Task `03-03-03` resumes | CI-01, CI-03, CI-04 | The default-branch `push` and `workflow_dispatch` verification paths both depend on post-merge repository state | Review the PR opened in Task `03-03-02`, merge it into `main`, then resume with the exact signal `merged-to-main`; Task `03-03-03` performs the main-branch artifact and Code Scanning verification after resume |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [x] Feedback latency < 30s for task smoke
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
