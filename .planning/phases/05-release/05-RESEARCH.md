# Phase 5: Release - Research

**Researched:** 2026-04-13 [VERIFIED: local date]
**Domain:** Tag-driven GitHub release automation for a public Go library with GoReleaser, first-release safeguards, and README-known-limitations messaging [VERIFIED: .planning/ROADMAP.md] [VERIFIED: .planning/phases/05-release/05-CONTEXT.md] [VERIFIED: README.md] [VERIFIED: .github/workflows/ci.yml]
**Confidence:** HIGH for workflow shape and GoReleaser capabilities; MEDIUM-HIGH for the first-release operational cut because it depends on live GitHub state at execution time [VERIFIED: repo inspection] [CITED: https://goreleaser.com/resources/cookbooks/release-a-library/] [CITED: https://goreleaser.com/customization/ci/actions/] [CITED: https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax]

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** `v0.1.0` must be cut through the same tag-push GitHub Actions/GoReleaser flow that future `vX.Y.Z` releases will use.
- **D-02:** Do not create a one-off bootstrap or manual-only release path for the first public tag.
- **D-03:** A dry-run or preflight on the exact target SHA is acceptable before the real tag is pushed, but the public release still has to be the production tag-triggered flow.
- **D-04:** Phase 5 is library-first. The supported public release contract is the Go module, not packaged CLI artifacts.
- **D-05:** `cmd/gin-index` may stay in the repo as a utility/source-install surface, but it must not become a first-class packaged release artifact in this phase.
- **D-06:** The `v0.1.0` GitHub Release should use a short maintainer-written preface followed by grouped, generated GoReleaser changelog sections.
- **D-07:** The preface should stay concise, explain that this is the first public OSS release, and avoid duplicating README tutorial content.
- **D-08:** Generated notes should be grouped into adopter-readable buckets such as `Features`, `Fixes`, `Docs`, `CI/Release`, and `Dependencies`; the exact regex/category wiring is implementation detail.
- **D-09:** `README.md` should gain a visible `Known limitations` section rather than moving release-critical caveats into a separate doc.
- **D-10:** That section should stay brief: one framing sentence plus exactly three bullets covering OR/AND composites, index merge, and query-time transformers.
- **D-11:** The limitations copy should read as intentional `v0.1.0` scope boundaries, not as promises that those capabilities are part of this phase. [VERIFIED: .planning/phases/05-release/05-CONTEXT.md]

### Claude's Discretion
The planner and executor can choose:
- the exact GoReleaser changelog regexes and ordering, as long as the release notes resolve into the locked adopter-readable buckets
- the exact preflight mechanism on the target SHA, as long as it remains a rehearsal for the same tag-driven release path rather than a separate publishing route
- whether the short maintainer preface is supplied directly in `.goreleaser.yml` `release.header` or by an equivalent release-body mechanism that still preserves GoReleaser-generated notes
- the exact placement of the `Known limitations` section in `README.md`, as long as it stays visible and does not bloat the first screen [VERIFIED: .planning/phases/05-release/05-CONTEXT.md] [CITED: https://goreleaser.com/customization/publish/scm/]

### Deferred Ideas (OUT OF SCOPE)
- Packaged CLI artifacts, Homebrew/Scoop distribution, or any public release contract broader than `go get github.com/amikos-tech/ami-gin`
- A release-branch model, version-bump tooling, or a custom maintainer release script that becomes the authoritative publish path
- Implementing the documented limitations themselves in this phase [VERIFIED: .planning/phases/05-release/05-CONTEXT.md] [VERIFIED: .planning/PROJECT.md]
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REL-01 | First semantic version tag `v0.1.0` pushed after all other requirements pass [VERIFIED: .planning/REQUIREMENTS.md] | Treat the tag push as a merge-gated external checkpoint, not just a file edit. The repo currently has no tags, so the first public tag will publish the entire history delta. Plan a rehearsal on the exact target SHA, then push the real `v0.1.0` tag only after `main` is green and Phase 4 is actually settled on the default branch. [VERIFIED: git tag --list] [VERIFIED: git log --oneline -20] [VERIFIED: .planning/STATE.md] |
| REL-02 | GoReleaser configured for library-only mode with automated CHANGELOG generation [VERIFIED: .planning/REQUIREMENTS.md] | Use a tag-triggered `.github/workflows/release.yml` plus `.goreleaser.yml` with `builds: - skip: true` for library-only releases, `fetch-depth: 0` for changelog history, `goreleaser/goreleaser-action@v7`, and `contents: write` so the workflow can create the GitHub Release. GoReleaser supports grouped changelog regexes and release-body headers, which matches the maintainer-preface-plus-generated-notes requirement. [CITED: https://goreleaser.com/resources/cookbooks/release-a-library/] [CITED: https://goreleaser.com/customization/ci/actions/] [CITED: https://goreleaser.com/customization/publish/changelog/] [CITED: https://goreleaser.com/customization/publish/scm/] [CITED: https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax] |
| REL-03 | Known limitations documented (OR/AND composites, index merge, query-time transformers) [VERIFIED: .planning/REQUIREMENTS.md] | Update `README.md` in place with a visible `Known limitations` section containing one framing sentence and exactly three bullets. Keep it on the adopter-facing surface rather than a new doc because the limitation list is part of the public release contract for `v0.1.0`. [VERIFIED: README.md] [VERIFIED: .planning/phases/05-release/05-CONTEXT.md] |
</phase_requirements>

## Project Constraints (from repo state)

- `.github/workflows/ci.yml` already defines the repo's GitHub Actions posture with `actions/checkout@v6`, `actions/setup-go@v6`, and `GOTOOLCHAIN: local`; the new release workflow should align with that style instead of inventing a parallel automation pattern. [VERIFIED: .github/workflows/ci.yml]
- The repository currently has no `.goreleaser.yml`, no `.github/workflows/release.yml`, and no git tags, so Phase 5 is greenfield release automation work. [VERIFIED: local command test -f .goreleaser.yml] [VERIFIED: local command ls .github/workflows] [VERIFIED: git tag --list]
- `go.mod` already points at `github.com/amikos-tech/ami-gin`, which means the Go module public surface is in place; Phase 5 does not need to re-solve the module-path problem, only prove that the tagged version installs cleanly. [VERIFIED: go.mod]
- `README.md` is already long and adopter-focused. The only public-doc expansion this phase needs is a visible but compact `Known limitations` section plus any minimal release-facing wording adjustments. [VERIFIED: README.md] [VERIFIED: .planning/phases/04-contributor-experience/04-CONTEXT.md]
- The current `Makefile` intentionally exposes build/test/lint/security commands and no release target. That reinforces the locked decision that release orchestration belongs in GitHub Actions plus semver tags, not in `make`. [VERIFIED: Makefile] [VERIFIED: .planning/phases/05-release/05-CONTEXT.md]
- The current state file still says `Waiting for human action` after Phase 4 shipped as PR `#12`, so the planner should treat the actual public tag as a human-gated step that runs only after the preceding phase is merged and the default branch is in the expected state. [VERIFIED: .planning/STATE.md]
- The unauthenticated public GitHub URL `https://github.com/amikos-tech/ami-gin` currently returns `404`, so the planner must treat repository visibility/public-access readiness as a precondition for the real public `v0.1.0` cut and the `go get` success criterion. [VERIFIED: public GitHub URL fetch returned 404]
- Recent commit history is mixed: some commits use conventional prefixes like `feat:` and `build(deps):`, while others are phase/meta commits like `Phase 04: Contributor Experience`. The changelog grouping therefore needs a preview/rehearsal step instead of assuming the default buckets will read well on the first public release. [VERIFIED: git log --oneline -20]

## Summary

Phase 5 should not be planned as a single monolithic "add GoReleaser" task. The clean split is: one autonomous implementation track for repo-local release automation and public-facing README updates, and one checkpoint-heavy track for the irreversible first public tag. The repo-local track owns `.goreleaser.yml`, `.github/workflows/release.yml`, and the `README.md` limitations section. The checkpoint track owns rehearsal on the target SHA, pushing `v0.1.0`, verifying that the tag-triggered workflow creates a GitHub Release, and proving `go get github.com/amikos-tech/ami-gin@v0.1.0` works from a clean consumer environment. [VERIFIED: repo inspection] [VERIFIED: .planning/phases/05-release/05-CONTEXT.md]

The main planning trap is treating `v0.1.0` as "just another changelog." Because there are no previous tags, the first public release notes will cover the repo's entire visible history. GoReleaser groups commits by regex against the first line of each commit message, and it supports filters plus a release-body header. That means the plan should explicitly preview the generated notes before pushing the real tag, otherwise phase/meta commits or merge commits can dominate the first public release page. [VERIFIED: git tag --list] [VERIFIED: git log --oneline -20] [CITED: https://goreleaser.com/customization/publish/changelog/] [CITED: https://goreleaser.com/customization/publish/scm/]

The second planning trap is slipping into a manual bootstrap release path. The local reference `scripts/release.sh` from TCLR is useful only for the safety checks it applies before a release: clean working tree, inspect commits since the previous tag, and rehearse before the real publish step. Those are exactly the right ideas to borrow, but the publish mechanism here must still remain tag-driven GitHub Actions plus GoReleaser, with no release branches and no custom local script becoming the canonical path. [VERIFIED: /Users/tazarov/experiments/telia/tclr/tclr-v2/scripts/release.sh] [VERIFIED: .planning/phases/05-release/05-CONTEXT.md]

**Primary recommendation:** create one autonomous plan for `.goreleaser.yml`, `.github/workflows/release.yml`, and `README.md` release messaging, then a second non-autonomous or checkpoint plan for target-SHA rehearsal, `v0.1.0` tag push, GitHub Release verification, and clean-environment `go get` confirmation. That aligns with the actual risk split between repo-local edits and irreversible external-state actions. [VERIFIED: repo inspection]

Because the public repository URL currently returns `404`, that second plan should start with an explicit visibility/public-access checkpoint before the real tag is pushed. Without that, the workflow could create a private GitHub Release while still failing the public-consumability goal of the phase. [VERIFIED: public GitHub URL fetch returned 404]

## Standard Stack

### Core Repo Surfaces

| Surface | Current State | Planning Implication |
|---------|---------------|----------------------|
| `.github/workflows/ci.yml` | Stable PR/push CI with `GOTOOLCHAIN: local`, `actions/checkout@v6`, and `actions/setup-go@v6` [VERIFIED: .github/workflows/ci.yml] | Mirror the same action majors and Go setup posture in `release.yml`, but add tag trigger, `contents: write`, and `fetch-depth: 0`. |
| `.goreleaser.yml` | Missing [VERIFIED: local command test -f .goreleaser.yml] | Add a library-only config using `builds: - skip: true`, grouped changelog configuration, and GitHub release settings. |
| `.github/workflows/release.yml` | Missing [VERIFIED: local command test -f .github/workflows/release.yml] | Add a dedicated tag-triggered workflow under `.github/workflows` rather than bolting release logic onto `ci.yml`. |
| `README.md` | Adopter-focused, no `Known limitations` section yet [VERIFIED: README.md] | Update in place with a concise limitations section; avoid moving limitations to a side doc. |
| Git history and tags | No tags; mixed conventional and phase/meta commit subjects [VERIFIED: git tag --list] [VERIFIED: git log --oneline -20] | Include a changelog preview/rehearsal task before the real tag so the first public notes are reviewed rather than blindly published. |
| `cmd/gin-index/main.go` | CLI exists in-tree [VERIFIED: .planning/phases/05-release/05-CONTEXT.md] | Explicitly keep it out of release artifacts to preserve the library-first contract. |

### External Platform Contracts

| Contract | Practical Meaning for Plans | Source |
|----------|-----------------------------|--------|
| GitHub Actions `push` workflows can filter on tags via `on.push.tags` | Use a dedicated release workflow triggered by `v*` tags rather than a manual publish job | [CITED: https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax] |
| `contents: write` lets a workflow using `GITHUB_TOKEN` create a release | Release workflow permissions must be explicit so GoReleaser can publish the GitHub Release | [CITED: https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax] |
| GoReleaser Action docs require `fetch-depth: 0` and show `args: release --clean` | The workflow must fetch full git history and invoke GoReleaser in the standard tag-release path | [CITED: https://goreleaser.com/customization/ci/actions/] |
| GoReleaser library cookbook uses `builds: - skip: true` | Library-only mode is officially supported and matches the Phase 5 contract of not packaging the CLI | [CITED: https://goreleaser.com/resources/cookbooks/release-a-library/] |
| GoReleaser supports grouped changelog regexes, filters, and a release-body `header` | A short maintainer preface plus grouped generated notes can live entirely inside GoReleaser config; no separate release-notes generator is required | [CITED: https://goreleaser.com/customization/publish/changelog/] [CITED: https://goreleaser.com/customization/publish/scm/] |
| `GITHUB_TOKEN`-triggered events do not recursively trigger most workflows | If future automation mutates the repo using `GITHUB_TOKEN`, that will not fan out new `push` runs automatically; keep the publish path tag-driven by a human or non-recursive mechanism | [CITED: https://docs.github.com/en/actions/concepts/security/github_token] |

## Recommended Patterns

### Pattern 1: Dedicated Tag-Triggered Release Workflow

**What:** Add `.github/workflows/release.yml` with `on.push.tags: ['v*']`, explicit `permissions: contents: write`, checkout with `fetch-depth: 0`, Go setup, and a `goreleaser/goreleaser-action@v7` step running `release --clean`. [CITED: https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax] [CITED: https://goreleaser.com/customization/ci/actions/]

**Why it fits this phase:** It satisfies the locked "same path for first and future tags" decision, keeps release orchestration in GitHub Actions, and creates the GitHub Release directly from the semver tag.

**Implementation shape:**
- trigger only on semver tags such as `v0.1.0`
- align action majors with the existing CI workflow where possible
- give the job `contents: write`
- use `fetch-depth: 0`
- pass `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}` to the GoReleaser step

### Pattern 2: Library-Only GoReleaser Config

**What:** Add `.goreleaser.yml` with `builds: - skip: true` and a GitHub release/changelog configuration oriented around notes generation rather than binary packaging. [CITED: https://goreleaser.com/resources/cookbooks/release-a-library/]

**Why it fits this phase:** It preserves the Go module as the public release surface while still letting GoReleaser create the changelog and GitHub Release page.

**Include exactly:**
- `builds: - skip: true`
- changelog groups and filters that can classify the current mixed history into `Features`, `Fixes`, `Docs`, `CI/Release`, and `Dependencies`
- a short release-body header or equivalent maintainer preface
- GitHub owner/name wired to `amikos-tech/ami-gin`

### Pattern 3: Rehearse on the Exact Target SHA, Then Push the Real Tag

**What:** Borrow the safety ideas from the TCLR release script, but keep them as preflight checks around the same GoReleaser path rather than a separate publishing mechanism. [VERIFIED: /Users/tazarov/experiments/telia/tclr/tclr-v2/scripts/release.sh]

**Why it fits this phase:** The first public tag is irreversible enough that the plan should prove the config and changelog on the actual candidate commit before `v0.1.0` is pushed.

**Recommended shape:**
- verify a clean working tree on the target branch
- inspect commits since the last tag, or all visible history if there is no previous tag
- run `goreleaser check` once `.goreleaser.yml` exists [CITED: https://goreleaser.com/cmd/goreleaser/]
- run a snapshot rehearsal such as `goreleaser release --snapshot` on the target SHA before the real tag [CITED: https://goreleaser.com/cmd/goreleaser/]
- review the generated notes/grouping before pushing `v0.1.0`

### Pattern 4: README-Known-Limitations as Part of the Public Contract

**What:** Add a visible `Known limitations` section to `README.md` with one framing sentence plus three bullets: OR/AND composites, index merge, and query-time transformers.

**Why it fits this phase:** Those caveats are release-critical scope boundaries for `v0.1.0`, so they need to sit on the same surface that adopters read before running `go get`.

**When to use:** Always for Phase 5; do not split this into a separate limitations doc.

### Pattern 5: Human-Gated First Public Tag

**What:** Plan `v0.1.0` as a checkpoint task that requires explicit human confirmation once repo-local files are ready and `main` is green.

**Why it fits this phase:** Creating the tag and publishing the first public GitHub Release touches external state that cannot be trivially rolled back, the state file still shows unresolved human action from the preceding phase, and the repository is not currently reachable at its public GitHub URL.

**Verification shape:**
- confirm the repository is publicly reachable or otherwise ready for unauthenticated consumption
- merge release-prep work to `main`
- confirm the default-branch CI run is green
- push `v0.1.0`
- verify the tag-triggered workflow succeeds
- verify a GitHub Release exists
- verify `go get github.com/amikos-tech/ami-gin@v0.1.0` succeeds from a clean environment

## Anti-Patterns to Avoid

- **Do not invent a one-off bootstrap release path for `v0.1.0`.** That violates D-01 and D-02 and creates a first-release process that future tags do not reuse. [VERIFIED: .planning/phases/05-release/05-CONTEXT.md]
- **Do not package or market the CLI as a release artifact in this phase.** The CLI may remain source-installable in-repo, but the public release contract is the Go library/module. [VERIFIED: .planning/phases/05-release/05-CONTEXT.md]
- **Do not use shallow checkout in `release.yml`.** GoReleaser's action docs call out `fetch-depth: 0` as required for full history/changelog behavior. [CITED: https://goreleaser.com/customization/ci/actions/]
- **Do not hide the limitations list in a separate markdown file.** The release-critical caveats belong in `README.md`, where adopters actually land. [VERIFIED: README.md] [VERIFIED: .planning/phases/05-release/05-CONTEXT.md]
- **Do not assume the first changelog will read cleanly without a preview.** The current commit log contains phase/meta commits that need deliberate grouping or filtering. [VERIFIED: git log --oneline -20]
- **Do not tag from an unmerged or unverified branch.** The first Go module version should come from the intended public default-branch state, not from a branch that still depends on unresolved human checkpoints. [VERIFIED: .planning/STATE.md]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| GitHub release creation | Custom `gh release create` scripts as the canonical publish path | GoReleaser in a tag-triggered workflow | The user explicitly wants the same automated path for `v0.1.0` and future semver tags. |
| Library-only release behavior | Fake binary packaging or archive hacks for the CLI [ASSUMED] | `builds: - skip: true` in `.goreleaser.yml` | GoReleaser already supports library-only releases directly. |
| Release notes composition | Separate homegrown changelog generator unless GoReleaser fundamentally cannot express the needed output [ASSUMED] | GoReleaser `changelog.groups`, `filters`, and `release.header` | The official config surface already supports grouped notes plus a short human preface. |
| Release runbook placement | `make release` or contributor-facing release instructions [ASSUMED] | GitHub Actions release workflow plus maintainers-only checkpoint steps in the phase plan | `Makefile` is the contributor interface, not the public release mechanism. |

## Common Pitfalls

### Pitfall 1: First Public Changelog Covers the Entire History
**What goes wrong:** There is no previous tag, so the first generated changelog includes all visible history, including phase/meta commits and merges.
**Why it happens:** GoReleaser groups based on the first line of each commit message, and there is no prior semver boundary yet. [CITED: https://goreleaser.com/customization/publish/changelog/]
**How to avoid:** Add an explicit rehearsal step that previews the generated notes on the exact target SHA and adjusts grouping/filter rules before pushing `v0.1.0`. [VERIFIED: git tag --list] [VERIFIED: git log --oneline -20]

### Pitfall 2: `fetch-depth: 1` Produces Broken or Empty Release Notes
**What goes wrong:** The release workflow checks out a shallow clone, and GoReleaser lacks the history it needs to produce correct notes.
**Why it happens:** `actions/checkout` defaults to shallow history unless overridden.
**How to avoid:** Set `fetch-depth: 0` in `release.yml`. [CITED: https://goreleaser.com/customization/ci/actions/]

### Pitfall 3: Mixed Commit Subjects Do Not Fit the Desired Buckets by Default
**What goes wrong:** Current history contains `feat:`, `build(deps):`, and `Phase N:` subjects. If the regexes are too narrow, the first public release notes become noisy or misleading.
**How to avoid:** Preview the changelog, then deliberately route phase/meta commits into `Docs` or `CI/Release`, and dependency bumps into `Dependencies`, instead of accepting the default output. [VERIFIED: git log --oneline -20] [CITED: https://goreleaser.com/customization/publish/changelog/]

### Pitfall 4: A Broken `v0.1.0` Tag Becomes the Public Contract
**What goes wrong:** The first tag is pushed before the default branch is fully ready, and the broken version becomes the version that adopters try first.
**Why it happens:** Tag creation is easy, but public release state is hard to undo cleanly.
**How to avoid:** Gate the real tag behind a green default-branch run plus a successful preflight on the target SHA. [VERIFIED: .planning/STATE.md]

### Pitfall 5: The Release Workflow Becomes Contributor-Facing by Accident
**What goes wrong:** Release commands get documented in `CONTRIBUTING.md` or wired into `make`, making maintainers and contributors share an inappropriate interface.
**How to avoid:** Keep release orchestration in `.github/workflows/release.yml` and reserve any manual steps for maintainers/checkpoints inside the phase plan, not contributor docs. [VERIFIED: CONTRIBUTING.md] [VERIFIED: Makefile]

## Validation Architecture

Phase 5 needs both repo-local validation and live GitHub verification. Repo-local checks should catch structural mistakes in `.goreleaser.yml`, `release.yml`, and `README.md` quickly. External-state checks should be reserved for the irreversible steps: pushing `v0.1.0`, observing the tag-triggered workflow, confirming the GitHub Release exists, and validating `go get` from a clean environment. [VERIFIED: repo inspection]

### Recommended Validation Contract

- **Quick run:** `go test ./... -run TestQueryEQ -count=1 && make help`
- **Release-config validation:** `goreleaser check` after `.goreleaser.yml` exists [CITED: https://goreleaser.com/cmd/goreleaser/]
- **Pre-tag rehearsal:** `goreleaser release --snapshot` on the intended release SHA before the real tag [CITED: https://goreleaser.com/cmd/goreleaser/]
- **Static file checks:** exact `rg` assertions for `.github/workflows/release.yml`, `.goreleaser.yml`, and `README.md`
- **Manual external checks:** after merge and tag push, verify the tag-triggered workflow succeeds, the GitHub Release exists, and `go get github.com/amikos-tech/ami-gin@v0.1.0` works with a clean module cache

### Task-Level Validation Shape

- Release workflow task: verify tag trigger, `contents: write`, `fetch-depth: 0`, Go setup, and `goreleaser/goreleaser-action@v7` with `release --clean`
- GoReleaser config task: verify `builds: - skip: true`, grouped changelog configuration, GitHub owner/name, and a short release header/preface mechanism
- README task: verify `## Known limitations` exists with one framing sentence and exactly three bullets for the locked limitation topics
- Release checkpoint task: verify clean tree, changelog rehearsal reviewed, `v0.1.0` tag pushed from the intended SHA, GitHub Release published, and clean-environment `go get` succeeds

## RESEARCH COMPLETE

Wrote `.planning/phases/05-release/05-RESEARCH.md`
