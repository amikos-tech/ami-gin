# Phase 4: Contributor Experience - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-12
**Phase:** 04-contributor-experience
**Areas discussed:** Contributor guide shape, Security disclosure path, Badge row and README placement, Dependabot policy

---

## Contributor guide shape

| Option | Description | Selected |
|--------|-------------|----------|
| Lean single-file quickstart | Root `CONTRIBUTING.md` stays short and focused only on contributor setup and commands | |
| Full all-in-one contributor + maintainer handbook | One larger document covers contributor tasks plus maintainer/admin process | |
| Split approach | Task-first `CONTRIBUTING.md` plus a small maintainer appendix or companion doc | ✓ |

**User's choice:** Split approach: task-first `CONTRIBUTING.md` plus small maintainer note/appendix.
**Notes:** Preserve Phase 3's `make`-first contributor workflow and keep README adopter-focused rather than turning it into process documentation.

---

## Security disclosure path

| Option | Description | Selected |
|--------|-------------|----------|
| GitHub private vulnerability reporting only | Use GitHub's private reporting flow as the sole disclosure path after launch | |
| Dedicated security email only | Use a mailbox/alias as the only public vulnerability disclosure channel | |
| GitHub private vulnerability reporting + dedicated security email fallback | GitHub is primary once public, with email as fallback for non-GitHub or pre-launch reporting | ✓ |
| Dedicated security email + `security.txt` on `amikos.tech` | Publish email in repo docs and additionally expose it via website `security.txt` | |

**User's choice:** GitHub private vulnerability reporting + dedicated security email fallback.
**Notes:** Fallback contact accepted as `security@amikos.tech`.

---

## Badge row and README placement

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal top row under `H1` | Keep a compact badge row directly below `# GIN Index` with only the required badges | ✓ |
| Expanded trust row under `H1` | Add a denser header badge row with broader trust/community signals | |
| Two-tier split | Keep required badges at top and move secondary badges or trust links lower in the README | |
| Deferred badge row after description | Move badges below the one-line description to keep the title area quieter | |

**User's choice:** Minimal top row under `H1`.
**Notes:** Phase 4 badge set should remain exactly `CI`, `Go Reference`, and `MIT`.

---

## Dependabot policy

| Option | Description | Selected |
|--------|-------------|----------|
| Daily, wildcard, ungrouped | Aggressive freshness with frequent independent PRs across any future module paths | |
| Weekly, root-only, grouped minor/patch, separate majors | Low-noise weekly cadence for the root module with semver-sensitive PR behavior | ✓ |
| Weekly, root-only, family groups | Weekly cadence with more targeted grouping, such as keeping AWS SDK updates together | |
| Monthly, broad grouped updates | Lowest churn, but slower update adoption and larger PR jumps | |

**User's choice:** Weekly, root-only, grouped minor/patch updates with separate majors.
**Notes:** The repo is currently single-module, so root-only scope fits the current structure.

---

## the agent's Discretion

- Exact maintainer appendix filename and placement
- Exact security policy response-time wording
- Exact badge order/formatting details
- Exact Dependabot schedule day/time and open-PR limit

## Deferred Ideas

None.
