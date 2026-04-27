---
phase: 4
slug: contributor-experience
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-04-12
---

# Phase 4 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` smoke plus exact `rg` / `yq` / `jq` assertions for docs, config, and GitHub metadata |
| **Config file** | `Makefile`, `README.md`, `.github/dependabot.yml` |
| **Quick run command** | `go test ./... -run TestQueryEQ -count=1 && make help` |
| **Full suite command** | `go test -v && make help` |
| **Estimated runtime** | quick run <15s, full suite ~30s |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -run TestQueryEQ -count=1 && make help`
- **After every plan wave:** Run `go test -v && make help`
- **Before `/gsd-verify-work`:** Full suite must be green and all `rg` / `yq` / GitHub checks below must pass
- **Max feedback latency:** 30 seconds for repo-local smoke checks; GitHub-side Dependabot verification occurs only after merge

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 04-01-01 | 01 | 1 | CONTR-01 | T-04-01 | `CONTRIBUTING.md` exposes the literal `make` contributor workflow via fenced `bash` blocks, tool notes, and explicit markdown links to `README.md` and `SECURITY.md` | static + smoke | `test -f CONTRIBUTING.md && rg -n '^# Contributing$' CONTRIBUTING.md && rg -n '^## Development Setup$' CONTRIBUTING.md && rg -n '^## Local Checks$' CONTRIBUTING.md && rg -n '^## Pull Request Workflow$' CONTRIBUTING.md && rg -n '^## Security$' CONTRIBUTING.md && test "$(rg -n '^```bash$' CONTRIBUTING.md | wc -l | tr -d ' ')" -ge 2 && setup_section="$(sed -n '/^## Development Setup$/,/^## Local Checks$/p' CONTRIBUTING.md)" && printf '%s\n' "$setup_section" | rg -n 'Go.*installed locally|installed locally.*Go|need Go' && printf '%s\n' "$setup_section" | rg -n 'golangci-lint' && printf '%s\n' "$setup_section" | rg -n 'govulncheck' && printf '%s\n' "$setup_section" | rg -n 'gotestsum' && first_setup_cmd="$(printf '%s\n' "$setup_section" | awk '/^```bash$/{in_block=1; next} in_block && /^```$/{exit} in_block {print; exit}')" && test "$first_setup_cmd" = "make help" && expected_local_checks="$(cat <<'EOF'\nmake build\nmake test\nmake integration-test\nmake lint\nmake lint-fix\nmake security-scan\nmake clean\nEOF\n)" && local_checks_block="$(sed -n '/^## Local Checks$/,/^## Pull Request Workflow$/p' CONTRIBUTING.md | awk '/^```bash$/{in_block=1; next} in_block && /^```$/{exit} in_block {print}')" && test "$local_checks_block" = "$expected_local_checks" && pr_section="$(sed -n '/^## Pull Request Workflow$/,/^## Security$/p' CONTRIBUTING.md)" && printf '%s\n' "$pr_section" | rg -n 'branch from main' && printf '%s\n' "$pr_section" | rg -n 'run local checks' && printf '%s\n' "$pr_section" | rg -n 'open PR' && rg -n '\[README\.md\]\(README\.md\)' CONTRIBUTING.md && rg -n '\[SECURITY\.md\]\(SECURITY\.md\)' CONTRIBUTING.md` | ❌ W0 | ⬜ pending |
| 04-01-02 | 01 | 1 | CONTR-02 | T-04-02 | `SECURITY.md` gives reporters a private path and forbids public disclosure | static | `test -f SECURITY.md && rg -n '^# Security Policy$' SECURITY.md && rg -n '^## Reporting a Vulnerability$' SECURITY.md && rg -n '^## Preferred Channel$' SECURITY.md && rg -n '^## Fallback Contact$' SECURITY.md && rg -n '^## Disclosure Expectations$' SECURITY.md && rg -n '^## Supported Versions$' SECURITY.md && preferred_section="$(sed -n '/^## Preferred Channel$/,/^## Fallback Contact$/p' SECURITY.md)" && printf '%s\n' "$preferred_section" | rg -n 'GitHub private vulnerability reporting.*first|first.*GitHub private vulnerability reporting' && rg -n 'security@amikos\\.tech' SECURITY.md && rg -n 'public issues' SECURITY.md && rg -n 'public pull requests' SECURITY.md && rg -n 'public discussions' SECURITY.md && disclosure_section="$(sed -n '/^## Disclosure Expectations$/,/^## Supported Versions$/p' SECURITY.md)" && printf '%s\n' "$disclosure_section" | rg -n '5 business days' && printf '%s\n' "$disclosure_section" | rg -n 'follow-up' && printf '%s\n' "$disclosure_section" | rg -n 'status updates' && printf '%s\n' "$disclosure_section" | rg -n 'triage progresses' && rg -n 'latest development line' SECURITY.md && rg -n 'main' SECURITY.md && rg -n 'latest tagged release' SECURITY.md && ! rg -n 'bug bounty|bounty|reward' SECURITY.md` | ❌ W0 | ⬜ pending |
| 04-01-03 | 01 | 1 | CONTR-03 | T-04-03 | README shows exactly the CI, Go Reference, and MIT badges in the first non-empty badge row and links to the new docs | static | `rg -n '^# GIN Index$' README.md && badge_row="$(awk 'NR==1{next} NF{print; exit}' README.md)" && printf '%s\n' "$badge_row" | rg '\[!\[CI\]\(https://github\.com/amikos-tech/ami-gin/actions/workflows/ci\.yml/badge\.svg\?branch=main&event=push\)\]\(https://github\.com/amikos-tech/ami-gin/actions/workflows/ci\.yml\)' && printf '%s\n' "$badge_row" | rg '\[!\[Go Reference\]\(https://pkg\.go\.dev/badge/github\.com/amikos-tech/ami-gin\.svg\)\]\(https://pkg\.go\.dev/github\.com/amikos-tech/ami-gin\)' && printf '%s\n' "$badge_row" | rg '\[!\[License: MIT\]\(https://img\.shields\.io/badge/License-MIT-yellow\.svg\)\]\(LICENSE\)' && test "$(printf '%s\n' "$badge_row" | rg -o '\[!\[' | wc -l | tr -d ' ')" = "3" && rg -n '^See \[CONTRIBUTING\.md\]\(CONTRIBUTING\.md\) for local contributor workflows and \[SECURITY\.md\]\(SECURITY\.md\) for disclosure guidance\.$' README.md && ! sed -n '1,8p' README.md | rg -n 'Coverage|Codecov|Go Report Card|OpenSSF|Stars|Downloads'` | ✅ | ⬜ pending |
| 04-02-01 | 02 | 2 | CONTR-04 | T-04-04 / T-04-05 | `.github/dependabot.yml` watches only the root Go module weekly and groups minor/patch updates, validated structurally with `yq` | static | `test -f .github/dependabot.yml && yq eval '.' .github/dependabot.yml > /dev/null && test "$(yq eval '.version' .github/dependabot.yml)" = "2" && test "$(yq eval '.updates | length' .github/dependabot.yml)" = "1" && test "$(yq eval '.updates[0].\"package-ecosystem\"' .github/dependabot.yml)" = "gomod" && test "$(yq eval '.updates[0].directory' .github/dependabot.yml)" = "/" && test "$(yq eval '.updates[0].schedule.interval' .github/dependabot.yml)" = "weekly" && test "$(yq eval '.updates[0].schedule.day' .github/dependabot.yml)" = "monday" && test "$(yq eval '.updates[0].schedule.time' .github/dependabot.yml)" = "07:00" && test "$(yq eval '.updates[0].schedule.timezone' .github/dependabot.yml)" = "UTC" && test "$(yq eval '.updates[0].\"open-pull-requests-limit\"' .github/dependabot.yml)" = "5" && test "$(yq eval '.updates[0].labels | length' .github/dependabot.yml)" = "2" && test "$(yq eval '.updates[0].labels[0]' .github/dependabot.yml)" = "dependencies" && test "$(yq eval '.updates[0].labels[1]' .github/dependabot.yml)" = "go" && test "$(yq eval '.updates[0].groups.\"gomod-minor-and-patch\".patterns[0]' .github/dependabot.yml)" = "*" && test "$(yq eval '.updates[0].groups.\"gomod-minor-and-patch\".\"update-types\" | length' .github/dependabot.yml)" = "2" && test "$(yq eval '.updates[0].groups.\"gomod-minor-and-patch\".\"update-types\"[0]' .github/dependabot.yml)" = "minor" && test "$(yq eval '.updates[0].groups.\"gomod-minor-and-patch\".\"update-types\"[1]' .github/dependabot.yml)" = "patch"` | ❌ W0 | ⬜ pending |
| 04-02-02 | 02 | 2 | CONTR-04 | T-04-06 | GitHub confirms `.github/dependabot.yml` is readable from `main` before any PR-observation check runs | external-state | `gh api repos/amikos-tech/ami-gin/contents/.github/dependabot.yml?ref=main --jq .path` | N/A external | ⬜ pending |
| 04-02-03 | 02 | 2 | CONTR-04 | T-04-06 | Final acceptance only happens after a real Dependabot PR targets `main` | external-state | `gh pr list --repo amikos-tech/ami-gin --state all --limit 50 --json number,title,headRefName,baseRefName,createdAt | jq -e 'map(select(.baseRefName == "main" and (.headRefName | startswith("dependabot/")))) | length > 0' >/dev/null` | N/A external | ⬜ pending |

*Status: ⬜ pending - ✅ green - ❌ red - ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `CONTRIBUTING.md` and `SECURITY.md` must exist before README links and badge/doc assertions can pass
- [ ] `.github/dependabot.yml` must exist on the working branch before post-merge GitHub verification can begin
- [ ] The execution branch containing `.github/dependabot.yml` must be merged into `main` before `04-02-02` can pass

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Merge the Phase 4 branch to `main` and confirm the Dependabot config is readable from the default branch (`04-02-02`) | CONTR-04 | Dependabot acts on default-branch config, so a human-controlled merge must happen before GitHub-side verification | Merge the PR containing `.github/dependabot.yml`, run `gh api repos/amikos-tech/ami-gin/contents/.github/dependabot.yml?ref=main --jq .path`, and resume with the exact signal `dependabot-config-merged` once the command returns `.github/dependabot.yml` |
| Observe the first Dependabot PR only after the config merge checkpoint (`04-02-02`) is satisfied; this is the external PR-observation checkpoint (`04-02-03`) | CONTR-04 | GitHub PR creation is asynchronous external state and may lag minutes to hours after merge | After `dependabot-config-merged`, run the `gh pr list --repo amikos-tech/ami-gin --state all --limit 50 --json number,title,headRefName,baseRefName,createdAt | jq -e 'map(select(.baseRefName == "main" and (.headRefName | startswith("dependabot/")))) | length > 0' >/dev/null` check from task `04-02-03`; resume with `dependabot-pr-observed` only when it succeeds, otherwise wait/retry and only investigate settings after 24 hours |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s for repo-local checks
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
