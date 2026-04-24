# Phase 5: Release - Context

**Gathered:** 2026-04-13
**Status:** Ready for planning

<domain>
## Phase Boundary

Make the library publicly consumable at `v0.1.0` with an automated GitHub release path for future semver tags. This phase covers tag-driven release automation, the first public GitHub Release, and README-known-limitations messaging. It does not add new library features, expand into packaged CLI distribution, or introduce a separate manual/bootstrap release process.

</domain>

<decisions>
## Implementation Decisions

### Release cut path
- **D-01:** `v0.1.0` should be cut through the same tag-push GitHub Actions/GoReleaser flow that future `vX.Y.Z` releases will use.
- **D-02:** Do not create a one-off bootstrap/manual release path for the first public tag.
- **D-03:** A dry-run or preflight on the exact target SHA is acceptable before pushing the real tag, but the actual public release must still be the production tag-triggered flow.

### Public release surface
- **D-04:** Phase 5 is library-first. The supported public release contract is the Go module, not packaged CLI artifacts.
- **D-05:** `cmd/gin-index` may remain in the repository as a utility/source-install surface, but it should not be positioned as a first-class release artifact or expand GoReleaser scope in this phase.

### Release notes style
- **D-06:** The `v0.1.0` GitHub Release should use a short maintainer-written preface followed by grouped, generated GoReleaser changelog sections.
- **D-07:** The preface should stay concise, explain that this is the first public OSS release, and avoid duplicating README tutorial content.
- **D-08:** Generated notes should be grouped into adopter-readable buckets such as `Features`, `Fixes`, `Docs`, `CI/Release`, and `Dependencies`; the exact regex/category wiring is implementation detail.

### Known limitations presentation
- **D-09:** `README.md` should add a visible `Known limitations` section rather than moving release-critical caveats into a separate doc.
- **D-10:** That section should be brief: one framing sentence plus three bullets covering OR/AND composites, index merge, and query-time transformers.
- **D-11:** The limitations copy should read as intentional `v0.1.0` scope boundaries, not as promises that those capabilities are part of this phase.

### the agent's Discretion
- Exact GoReleaser changelog regex/group configuration, as long as it produces grouped generated notes aligned with the categories above.
- Exact mechanics of the pre-tag rehearsal or dry-run before `v0.1.0` is pushed.
- Exact release workflow filename/job naming and whether the short preface is supplied via GoReleaser templating or a brief post-automation GitHub Release body edit.
- Exact placement of the README limitations section, as long as it remains visible in `README.md` without cluttering the first screen.

</decisions>

<specifics>
## Specific Ideas

- Use a short human preface above generated changelog sections for `v0.1.0`; keep it to roughly 3-5 lines.
- Use the local reference release process in `/Users/tazarov/experiments/telia/tclr/tclr-v2/scripts/release.sh` as inspiration for pre-tag safety checks: verify a clean working tree, review commits since the last tag, and preview the release before pushing the real tag.
- Do **not** copy TCLR's release-branch model into this repo; the chosen release path here remains direct tag-driven automation.
- Keep the CLI visible as a repo utility only, not as the headline public release surface.

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and acceptance
- `.planning/ROADMAP.md` — Phase 5 goal and success criteria for automated release setup, `v0.1.0`, and README limitations.
- `.planning/REQUIREMENTS.md` — `REL-01` through `REL-03`, which define the required outputs for this phase.
- `.planning/PROJECT.md` — core value, active release-facing work, and the explicit out-of-scope limitations that must be documented rather than implemented.

### Prior phase constraints
- `.planning/phases/03-ci-pipeline/03-CONTEXT.md` — locks GitHub Actions as the orchestration layer and establishes CI conventions the release workflow should align with.
- `.planning/phases/04-contributor-experience/04-CONTEXT.md` — locks the minimal adopter-focused README and the current public-doc surface expectations.

### Existing repo surfaces this phase must update or align with
- `README.md` — current adopter-facing landing page that must gain known limitations without bloating the top of the document.
- `CONTRIBUTING.md` — contributor-local workflow guidance; release automation should not turn this into a release runbook.
- `.github/workflows/ci.yml` — current GitHub Actions conventions, job ecosystem, and Go/toolchain posture that the release workflow should stay consistent with.
- `cmd/gin-index/main.go` — confirms the CLI exists so planners can intentionally keep it out of the supported release contract for this phase.

### Release research and pitfalls
- `.planning/research/SUMMARY.md` — overall OSS launch sequencing and release-automation rationale.
- `.planning/research/ARCHITECTURE.md` — recommended tag-triggered `release.yml` pattern and the `fetch-depth: 0` changelog requirement.
- `.planning/research/STACK.md` — GoReleaser library-only configuration guidance and recommended action versions.
- `.planning/research/PITFALLS.md` — broken-tag permanence, changelog-history pitfalls, and release safety hazards.

### Local cross-repo reference
- `/Users/tazarov/experiments/telia/tclr/tclr-v2/scripts/release.sh` — local reference script mentioned during discussion; use only as inspiration for release preflight checks, not for branching strategy.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `README.md`: already has a minimal adopter-facing structure and badge row; Phase 5 should add limitations without redesigning the document.
- `.github/workflows/ci.yml`: already establishes the repo's GitHub Actions conventions, pinned action majors, and `GOTOOLCHAIN: local` posture.
- `CONTRIBUTING.md`: already centralizes local contributor commands, which helps keep release orchestration out of `make`.
- `cmd/gin-index/main.go`: proves the CLI is real and source-installable, which makes it a conscious scope choice to keep it out of packaged release support.

### Established Patterns
- GitHub Actions is the repo's automation control plane; `make` is the local contributor interface, not the public release mechanism.
- Root-level markdown files are the public documentation surface, and Phase 4 already locked `README.md` into a minimal adopter-focused style.
- The repo currently has no release workflow, no `.goreleaser.yml`, and no published tags, so Phase 5 is greenfield release automation work rather than adaptation of an existing release pipeline.

### Integration Points
- New `.github/workflows/release.yml` tag-triggered workflow.
- New `.goreleaser.yml` library-only release configuration.
- `README.md` known-limitations section and any small release-facing wording adjustments.
- Git tags on the release commit as the trigger that creates the GitHub Release and enables `go get github.com/amikos-tech/ami-gin@v0.1.0`.

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 05-release*
*Context gathered: 2026-04-13*
