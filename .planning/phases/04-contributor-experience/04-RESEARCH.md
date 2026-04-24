# Phase 4: Contributor Experience - Research

**Researched:** 2026-04-12 [VERIFIED: local date]
**Domain:** Contributor-facing repository docs, trust badges, and Dependabot version-update automation for a single-module Go OSS library [VERIFIED: .planning/ROADMAP.md] [VERIFIED: README.md] [VERIFIED: go.mod]
**Confidence:** MEDIUM-HIGH [VERIFIED: repo inspection] [VERIFIED: official GitHub docs]

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Use a split documentation model: a task-first `CONTRIBUTING.md` for contributor setup and local workflows, plus a small maintainer-focused appendix or companion doc for upkeep/admin notes if needed.
- **D-02:** Keep `README.md` adopter-focused. `CONTRIBUTING.md` should cover contribution workflow and exact local commands, and link back to README sections instead of duplicating feature/tutorial content.
- **D-03:** Treat `Makefile` targets as the canonical local contributor interface. The contributor guide should document `make build`, `make test`, `make integration-test`, `make lint`, and `make security-scan`.
- **D-04:** Publish `security@amikos.tech` in `SECURITY.md` as the fallback vulnerability reporting address.
- **D-05:** GitHub private vulnerability reporting is the primary reporting path once the repository is public.
- **D-06:** Email remains a valid fallback for reporters who cannot or prefer not to use GitHub private vulnerability reporting.
- **D-07:** Keep a minimal badge row directly under `# GIN Index`.
- **D-08:** The Phase 4 badge set is exactly `CI`, `Go Reference`, and `MIT`; do not expand into a denser trust row in this phase.
- **D-09:** Configure Dependabot for the root module only (`directory: "/"`) because the repo is currently a single-module Go project.
- **D-10:** Run automated dependency update checks weekly.
- **D-11:** Group minor and patch Go module updates together, and keep major version updates in separate PRs. [VERIFIED: .planning/phases/04-contributor-experience/04-CONTEXT.md]

### Claude's Discretion
The planner and executor can choose:
- the exact maintainer appendix filename and placement if one is needed
- the exact `SECURITY.md` response-time and supported-version wording
- the exact badge order, badge URL flavor, and link destinations as long as the row remains minimal and directly below the title
- the exact Dependabot weekday/time, labels, and open-PR limit as long as updates remain weekly, root-only, and group minor/patch separately from majors [VERIFIED: .planning/phases/04-contributor-experience/04-CONTEXT.md]

### Deferred Ideas (OUT OF SCOPE)
None - discussion stayed within phase scope. [VERIFIED: .planning/phases/04-contributor-experience/04-CONTEXT.md]
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CONTR-01 | `CONTRIBUTING.md` exists with build, test, and lint instructions [VERIFIED: .planning/REQUIREMENTS.md] | Keep `CONTRIBUTING.md` task-first and literal: document `make build`, `make test`, `make integration-test`, `make lint`, `make lint-fix`, `make security-scan`, and `make clean`, and call out any prerequisite tools that are not auto-installed by the Makefile. [VERIFIED: Makefile] |
| CONTR-02 | `SECURITY.md` exists with vulnerability disclosure policy [VERIFIED: .planning/REQUIREMENTS.md] | Use a standard coordinated-disclosure structure: preferred private GitHub reporting path when public/enabled, explicit fallback email `security@amikos.tech`, "do not file public issues for vulnerabilities", and short expectations for acknowledgement/update timing. GitHub documents that private vulnerability reporting is separate from `SECURITY.md` and only works when enabled on a public repository. [VERIFIED: .planning/phases/04-contributor-experience/04-CONTEXT.md] [CITED: https://docs.github.com/en/code-security/how-tos/report-and-fix-vulnerabilities/privately-reporting-a-security-vulnerability] |
| CONTR-03 | README has CI status, Go Reference, and MIT license badges [VERIFIED: .planning/REQUIREMENTS.md] | Preserve the existing GitHub Actions CI badge, add the standard `pkg.go.dev` Go Reference badge for `github.com/amikos-tech/ami-gin`, and add an MIT badge linking to `LICENSE`. Keep badge scope to exactly these three. README currently already has only the CI badge, so this is an additive refinement rather than a rewrite. [VERIFIED: README.md] [VERIFIED: LICENSE] |
| CONTR-04 | Dependabot configuration for automated Go module dependency updates [VERIFIED: .planning/REQUIREMENTS.md] | Add `.github/dependabot.yml` with `version: 2`, one `updates` entry for `package-ecosystem: "gomod"` and `directory: "/"`, a weekly schedule, and a `groups` block that matches all dependencies and includes only `minor` and `patch` updates. GitHub's Dependabot docs support `patterns` and `update-types` in groups, and version updates are enabled by checking in `dependabot.yml`. [VERIFIED: go.mod] [CITED: https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference?learn=dependency_version_updates&learnproduct=code-security] [CITED: https://docs.github.com/en/code-security/concepts/supply-chain-security/about-dependabot-version-updates] |
</phase_requirements>

## Project Constraints (from repo state)

- `Makefile` already exposes the required contributor commands for `build`, `test`, `integration-test`, `lint`, `lint-fix`, `security-scan`, `clean`, and `help`; Phase 4 should document these commands, not invent parallel shell entrypoints. [VERIFIED: Makefile] [VERIFIED: AGENTS.md]
- `README.md` is already long and adopter-focused. Phase 4 should add contributor/security links surgically instead of duplicating installation, examples, or architecture sections inside `CONTRIBUTING.md`. [VERIFIED: README.md] [VERIFIED: .planning/phases/04-contributor-experience/04-CONTEXT.md]
- `.github/workflows/ci.yml` and `.github/workflows/security.yml` already establish the CI and security posture that contributor docs should reference at a high level; they should not be re-explained line by line. [VERIFIED: .github/workflows/ci.yml] [VERIFIED: .github/workflows/security.yml]
- There is no existing `CONTRIBUTING.md`, `SECURITY.md`, or `.github/dependabot.yml`, so Phase 4 is additive on new repo surfaces rather than a migration of legacy docs. [VERIFIED: local command ls CONTRIBUTING.md SECURITY.md .github/dependabot.yml]
- `go list -m -u all` currently shows many outdated Go dependencies, so once a valid root-level Dependabot config lands on the default branch, the repository should have real update candidates rather than an empty queue. [VERIFIED: local command go list -m -u all]

## Summary

Phase 4 should be planned as two implementation tracks, not one monolithic docs blob. The first track is repository-facing documentation polish: add `CONTRIBUTING.md`, add `SECURITY.md`, and extend the existing minimal README badge row from one badge to three while keeping README adopter-focused. The second track is automation: add a root-only weekly `gomod` Dependabot config that groups minor/patch updates and leaves major updates separate, then verify the first Dependabot PR after the config reaches the default branch. That split mirrors the actual trust surfaces in the repository: humans read docs from the branch, but Dependabot only acts once `.github/dependabot.yml` is on the default branch. [VERIFIED: README.md] [VERIFIED: Makefile] [VERIFIED: .planning/phases/04-contributor-experience/04-CONTEXT.md]

The main planning trap is treating `SECURITY.md` and GitHub private vulnerability reporting as the same thing. GitHub explicitly documents that private vulnerability reporting is separate from the security policy file and only works when enabled on public repositories. The plan should therefore make the repo-local part explicit: `SECURITY.md` must direct reporters to private GitHub reporting when available, but must also publish the email fallback because the repo is still private today and the feature may not yet be enabled when the docs land. [VERIFIED: .planning/STATE.md] [CITED: https://docs.github.com/en/code-security/how-tos/report-and-fix-vulnerabilities/privately-reporting-a-security-vulnerability]

Dependabot also needs to be modeled as partly repo-local and partly post-merge. GitHub's docs state that version updates are enabled by checking in `dependabot.yml`, and that Dependabot raises a pull request when it identifies an outdated dependency. Because the repo already has many outdated modules, the planner can reasonably include a post-merge verification step that checks for a `dependabot/` branch PR targeting `main` rather than treating CONTR-04 as a purely future aspiration. [VERIFIED: local command go list -m -u all] [CITED: https://docs.github.com/en/code-security/concepts/supply-chain-security/about-dependabot-version-updates] [CITED: https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference?learn=dependency_version_updates&learnproduct=code-security]

**Primary recommendation:** create one autonomous plan for `CONTRIBUTING.md` + `SECURITY.md` + README badge/link updates, then a second plan for `.github/dependabot.yml` plus a merge-gated verification step that confirms the first Dependabot PR appears on the default branch. [VERIFIED: repo inspection]

## Standard Stack

### Core Repo Surfaces

| Surface | Current State | Planning Implication |
|---------|---------------|----------------------|
| `README.md` | Existing CI badge directly under `# GIN Index`; no contributor/security links; no Go Reference or MIT badges [VERIFIED: README.md] | Update in place rather than replace; keep the header compact and add only the missing badges and short links to new docs. |
| `Makefile` | Canonical local command surface with required targets already present [VERIFIED: Makefile] | `CONTRIBUTING.md` should document the literal `make` commands and note that `make test` installs `gotestsum`, while lint/security scan require their tools to be available locally. |
| `.github/workflows/ci.yml` | Existing PR/push CI gate [VERIFIED: .github/workflows/ci.yml] | README CI badge must keep pointing at this workflow; contributor docs can reference CI behavior without duplicating YAML internals. |
| `.github/workflows/security.yml` | Existing scheduled/manual govulncheck SARIF workflow [VERIFIED: .github/workflows/security.yml] | `SECURITY.md` can mention that the project maintains security scans, but disclosure guidance must stay focused on reporting rather than CI implementation details. |
| `go.mod` | Single root Go module [VERIFIED: go.mod] | Dependabot scope should be exactly one `gomod` updates entry at `directory: "/"`. |

### External Platform Contracts

| Contract | Practical Meaning for Plans | Source |
|----------|-----------------------------|--------|
| Private vulnerability reporting is separate from `SECURITY.md` and is available when enabled on public repos | `SECURITY.md` should describe the preferred GitHub route conditionally and always include the fallback email | [CITED: https://docs.github.com/en/code-security/how-tos/report-and-fix-vulnerabilities/privately-reporting-a-security-vulnerability] |
| Checking in `dependabot.yml` enables Dependabot version updates | `.github/dependabot.yml` is the repo-local trigger for CONTR-04 | [CITED: https://docs.github.com/en/code-security/concepts/supply-chain-security/about-dependabot-version-updates] |
| Dependabot groups support `patterns` and `update-types` | Minor/patch grouping can be implemented directly in config without custom tooling | [CITED: https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference?learn=dependency_version_updates&learnproduct=code-security] |

## Recommended Patterns

### Pattern 1: Task-First `CONTRIBUTING.md`

**What:** Create a short root `CONTRIBUTING.md` organized around "how to get set up", "how to run local checks", and "how to submit changes", with literal `make` commands and links back to README for product usage. [VERIFIED: README.md] [VERIFIED: Makefile]

**Why it fits this phase:** The repo already has a stable local command surface, and the user explicitly wants README to stay adopter-focused. A task-first guide reduces contributor friction without duplicating feature documentation. [VERIFIED: .planning/phases/04-contributor-experience/04-CONTEXT.md]

**Include exactly:**
- prerequisite note for Go plus locally installed `golangci-lint` / `govulncheck`
- `make build`
- `make test`
- `make integration-test`
- `make lint`
- `make lint-fix`
- `make security-scan`
- `make clean`
- a compact PR workflow (`fork/branch -> run checks -> open PR`)
- short links to `README.md` and `SECURITY.md`

### Pattern 2: Standard OSS `SECURITY.md` With Conditional GitHub Path

**What:** Add a root `SECURITY.md` that tells reporters to use GitHub private vulnerability reporting when the public repo exposes it, otherwise email `security@amikos.tech`, and never open a public issue for a vulnerability. [VERIFIED: .planning/phases/04-contributor-experience/04-CONTEXT.md] [CITED: https://docs.github.com/en/code-security/how-tos/report-and-fix-vulnerabilities/privately-reporting-a-security-vulnerability]

**Why it fits this phase:** It preserves the long-term desired primary channel without lying about repo state today. The file remains valid before and after launch.

**Include exactly:**
- preferred reporting path wording for private GitHub reporting when available
- fallback email address
- request not to disclose in public issues/discussions/PRs
- acknowledgement and follow-up timing language
- supported-version statement scoped to the latest development line until release policy matures

### Pattern 3: Minimal README Trust Row

**What:** Keep exactly three badges under `# GIN Index`: CI, Go Reference, MIT. Add short "Contributing" / "Security" doc links elsewhere in README if needed, but do not turn the header into a dashboard. [VERIFIED: README.md] [VERIFIED: .planning/phases/04-contributor-experience/04-CONTEXT.md]

**Implementation shape:**
- Preserve existing CI badge URL
- Add `pkg.go.dev` Go Reference badge for `github.com/amikos-tech/ami-gin`
- Add MIT badge linking to `LICENSE`

**When to use:** Always for this phase; the badge set is locked. [VERIFIED: .planning/phases/04-contributor-experience/04-CONTEXT.md]

### Pattern 4: Root-Only Weekly `gomod` Dependabot With Minor/Patch Grouping

**What:** Configure only the root Go module in `.github/dependabot.yml`, use a weekly schedule, and define one group that matches all dependencies with `update-types: ["minor", "patch"]`. Major updates stay ungrouped by omission. [VERIFIED: go.mod] [CITED: https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference?learn=dependency_version_updates&learnproduct=code-security]

**Why it fits this phase:** The repository is a single-module Go library today, and local inspection already shows many outdated modules, so the config should start generating real PRs once merged. [VERIFIED: local command go list -m -u all]

**Recommended shape:**
```yaml
version: 2
updates:
  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"
    groups:
      gomod-minor-and-patch:
        patterns:
          - "*"
        update-types:
          - "minor"
          - "patch"
```

### Pattern 5: Explicit Post-Merge Verification for CONTR-04

**What:** Treat "first Dependabot PR appears" as a merge-gated verification step, not a purely local edit. After `.github/dependabot.yml` reaches `main`, verify that GitHub opens a PR whose head branch starts with `dependabot/` and targets `main`. [CITED: https://docs.github.com/en/code-security/concepts/supply-chain-security/about-dependabot-version-updates]

**Why it fits this phase:** It closes the gap between "config exists in the branch" and "automation is actually alive on the repository."

## Anti-Patterns to Avoid

- **Do not duplicate README usage content into `CONTRIBUTING.md`.** The guide should route contributors to the right commands, not repeat the long feature and example narrative already in README. [VERIFIED: README.md]
- **Do not describe GitHub private vulnerability reporting as universally available today.** The repo is still private, and GitHub documents that private reporting is tied to enabled public-repo configuration. [VERIFIED: .planning/STATE.md] [CITED: https://docs.github.com/en/code-security/how-tos/report-and-fix-vulnerabilities/privately-reporting-a-security-vulnerability]
- **Do not add extra badges, coverage badges, or community badges in this phase.** The badge set is locked to three. [VERIFIED: .planning/phases/04-contributor-experience/04-CONTEXT.md]
- **Do not configure Dependabot for GitHub Actions, Docker, or additional directories in this phase.** The root `gomod` scope is a locked decision. [VERIFIED: .planning/phases/04-contributor-experience/04-CONTEXT.md]
- **Do not group major updates with minor/patch updates.** That would violate the semver-noise constraint in D-11. [VERIFIED: .planning/phases/04-contributor-experience/04-CONTEXT.md]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Contributor command surface | Custom shell scripts documented only in docs [ASSUMED] | Existing `Makefile` targets [VERIFIED: Makefile] | The commands already exist and are the locked contributor interface. |
| Security intake | Ad hoc "email us somehow" wording [ASSUMED] | Root `SECURITY.md` plus GitHub private reporting path when available [CITED: https://docs.github.com/en/code-security/how-tos/report-and-fix-vulnerabilities/privately-reporting-a-security-vulnerability] | GitHub separates the policy doc from the private-reporting feature; both need explicit treatment. |
| Dependency update grouping | Custom bots or Actions workflows [ASSUMED] | Native Dependabot `groups` configuration [CITED: https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference?learn=dependency_version_updates&learnproduct=code-security] | The platform already supports the required grouping semantics. |
| Badge generation | Custom SVGs checked into the repo [ASSUMED] | Existing GitHub Actions badge, `pkg.go.dev` badge, MIT badge linked to `LICENSE` | Static hand-rolled badges will drift and add maintenance with no product value. |

## Common Pitfalls

### Pitfall 1: Documenting Commands That Do Not Match the Actual Makefile
**What goes wrong:** Contributors copy commands from `CONTRIBUTING.md` that do not exist or omit required local tools.
**Why it happens:** Docs get written from memory instead of from the current `Makefile`. [VERIFIED: Makefile]
**How to avoid:** Make every command in the guide a literal `make` target that already exists, and separately note which external tools must be installed locally for `lint` and `security-scan`. [VERIFIED: Makefile]

### Pitfall 2: Treating `SECURITY.md` as a Substitute for GitHub Private Reporting
**What goes wrong:** The docs imply users can always click "Report a vulnerability" even when the repo is private or the feature is not enabled.
**Why it happens:** GitHub's reporting feature and the repository security policy are easy to conflate. [CITED: https://docs.github.com/en/code-security/how-tos/report-and-fix-vulnerabilities/privately-reporting-a-security-vulnerability]
**How to avoid:** Use conditional wording: GitHub private reporting is preferred when available, email is always valid as fallback. [VERIFIED: .planning/phases/04-contributor-experience/04-CONTEXT.md]

### Pitfall 3: Dependabot Config That Never Produces the Intended PR Shape
**What goes wrong:** The config watches the wrong directory, groups majors with everything else, or omits `patterns`, leading to noisy or missing PRs.
**Why it happens:** The file is small enough that subtle misconfiguration looks plausible in review.
**How to avoid:** Verify exact YAML keys for `package-ecosystem`, `directory`, `schedule.interval`, `groups.<name>.patterns`, and `groups.<name>.update-types`, then confirm a `dependabot/` PR appears after merge. [CITED: https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference?learn=dependency_version_updates&learnproduct=code-security]

### Pitfall 4: Assuming CONTR-04 Is Done Before the Config Hits the Default Branch
**What goes wrong:** The branch adds `.github/dependabot.yml`, but the phase closes without confirming that automation actually raised a PR.
**Why it happens:** Repo-local config and GitHub-side behavior happen on different timelines.
**How to avoid:** Put the post-merge verification into the plan explicitly and block final acceptance of CONTR-04 on observing the first Dependabot PR. [VERIFIED: local command go list -m -u all]

## Validation Architecture

Phase 4 is mostly docs and repository config, so validation should be fast and explicit rather than test-framework heavy. The right model is: keep a lightweight Go smoke test to catch accidental repo breakage, and verify each deliverable with exact `rg` checks plus one GitHub-side check for the first Dependabot PR after merge. [VERIFIED: repo inspection]

### Recommended Validation Contract

- **Quick run:** `go test ./... -run TestQueryEQ -count=1 && make help`
- **Wave-end full run:** `go test -v && make help`
- **Per-task checks:** exact `rg` matches for `CONTRIBUTING.md`, `SECURITY.md`, `README.md`, and `.github/dependabot.yml`
- **Manual external check:** after merge, inspect GitHub PRs for a `dependabot/` branch targeting `main`

### Task-Level Validation Shape

- `CONTRIBUTING.md`: verify the required `make` commands appear literally
- `SECURITY.md`: verify private GitHub reporting wording, fallback email, and "do not disclose publicly" guidance
- `README.md`: verify the exact three badge links and no extra badge creep
- `.github/dependabot.yml`: verify `gomod`, `directory: "/"`, weekly schedule, and grouped `minor`/`patch` updates
- GitHub post-merge state: verify at least one PR with `headRefName` starting `dependabot/` targets `main`

## RESEARCH COMPLETE

Wrote `.planning/phases/04-contributor-experience/04-RESEARCH.md`
