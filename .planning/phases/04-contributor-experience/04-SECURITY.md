---
phase: 04
slug: contributor-experience
status: verified
threats_open: 0
asvs_level: 1
created: 2026-04-13
updated: 2026-04-13T13:04:20Z
---

# Phase 04 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Public repo docs -> outside contributors | Contributors rely on `README.md` and `CONTRIBUTING.md` for the correct local commands and workflows. | Public documentation and local workflow instructions |
| Vulnerability reporter -> maintainer intake channels | Reporters rely on `SECURITY.md` to choose a private channel instead of public disclosure. | Vulnerability details and disclosure metadata |
| Default-branch config -> GitHub Dependabot service | Dependabot only acts on the configuration once it exists on the default branch. | Dependency automation configuration |
| Dependabot PR stream -> maintainer review flow | The repository relies on generated PRs to keep dependency freshness visible and actionable. | Automated dependency update pull requests |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-04-01 | T | `CONTRIBUTING.md` command surface | mitigate | Verified the guide starts with `make help` and documents the literal `make build`, `make test`, `make integration-test`, `make lint`, `make lint-fix`, `make security-scan`, and `make clean` workflow from the current repo. | closed |
| T-04-02 | I | `SECURITY.md` reporting guidance | mitigate | Verified the policy forbids public disclosure paths, prefers GitHub private reporting, uses `security@amikos.tech` as fallback, and does not claim a bug bounty. | closed |
| T-04-03 | S | README badge row | mitigate | Verified the first badge row is limited to the real CI, Go Reference, and MIT links, followed by the contributor/security discovery sentence. | closed |
| T-04-04 | T | `.github/dependabot.yml` scope | mitigate | Verified on `origin/main` that the config contains exactly one root `gomod` update entry with the planned weekly schedule and grouping structure. | closed |
| T-04-05 | D | grouped update policy | mitigate | Verified with `yq` that minor and patch updates are grouped under `gomod-minor-and-patch` and the open PR limit is capped at `5`. | closed |
| T-04-06 | R | CONTR-04 completion claim | mitigate | Verified the config is present on `origin/main` and observed real Dependabot PRs targeting `main` via `gh pr list`, including merged PR `#10` and open grouped PR `#14`. | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

No accepted risks.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-04-13 | 6 | 6 | 0 | Codex |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-04-13
