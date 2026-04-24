# Phase 4: Contributor Experience - Context

**Gathered:** 2026-04-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Make the repository understandable, safe, and low-friction for outside contributors without requiring source spelunking. This phase delivers contributor-facing docs, security disclosure guidance, README trust badges, and automated dependency update policy. It does not add new library capabilities or release automation.

</domain>

<decisions>
## Implementation Decisions

### Contributor documentation structure
- **D-01:** Use a split documentation model: a task-first `CONTRIBUTING.md` for contributor setup and local workflows, plus a small maintainer-focused appendix or companion doc for upkeep/admin notes if needed.
- **D-02:** Keep `README.md` adopter-focused. `CONTRIBUTING.md` should cover contribution workflow and exact local commands, and link back to README sections instead of duplicating feature/tutorial content.
- **D-03:** Treat `Makefile` targets as the canonical local contributor interface. The contributor guide should document `make build`, `make test`, `make integration-test`, `make lint`, and `make security-scan`.

### Security disclosure policy
- **D-04:** Publish `security@amikos.tech` in `SECURITY.md` as the fallback vulnerability reporting address.
- **D-05:** GitHub private vulnerability reporting is the primary reporting path once the repository is public.
- **D-06:** Email remains a valid fallback for reporters who cannot or prefer not to use GitHub private vulnerability reporting.

### README badge presentation
- **D-07:** Keep a minimal badge row directly under `# GIN Index`.
- **D-08:** The Phase 4 badge set is exactly `CI`, `Go Reference`, and `MIT`; do not expand into a denser trust row in this phase.

### Dependabot policy
- **D-09:** Configure Dependabot for the root module only (`directory: "/"`) because the repo is currently a single-module Go project.
- **D-10:** Run automated dependency update checks weekly.
- **D-11:** Group minor and patch Go module updates together, and keep major version updates in separate PRs.

### the agent's Discretion
- Exact filename and placement of the maintainer-focused appendix/companion doc, as long as the root `CONTRIBUTING.md` stays task-first.
- Exact `SECURITY.md` response-time language and supported-version wording, using standard low-overhead OSS policy language.
- Exact badge ordering, link URLs, and badge style, as long as the row remains minimal and directly under the title.
- Exact Dependabot weekday/time and open-PR limit, as long as the weekly cadence and grouped minor/patch vs separate major behavior are preserved.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and acceptance
- `.planning/ROADMAP.md` — Phase 4 goal, dependency on Phase 3, and success criteria for contributor docs, badges, and dependency automation.
- `.planning/REQUIREMENTS.md` — `CONTR-01` through `CONTR-04`, which define the required outputs for this phase.
- `.planning/PROJECT.md` — core value, OSS launch context, and active documentation-related requirements.

### Prior phase constraints
- `.planning/phases/03-ci-pipeline/03-CONTEXT.md` — locks the pattern that `make` is the contributor-facing local interface while CI orchestration stays in workflow YAML.

### Existing repo surfaces this phase must update or align with
- `README.md` — current adopter-facing structure and current badge placement under the project title.
- `Makefile` — canonical local build, test, lint, and security commands to document.
- `.github/workflows/ci.yml` — GitHub-only CI behavior, including matrix testing and artifact/reporting details contributors should understand at a high level.
- `.github/workflows/security.yml` — existing scheduled security scan posture that informs `SECURITY.md` and maintainer guidance.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `README.md`: already contains install, examples, and benchmark sections that contributor docs should reference instead of rewriting.
- `Makefile`: stable command surface for local contributor workflows.
- `.github/workflows/ci.yml`: definitive source for CI-only behavior such as the Go version matrix and `-race` execution.
- `.github/workflows/security.yml`: existing security scanning workflow that sets the repo's current security-maintenance baseline.

### Established Patterns
- Contributor-facing local workflows use `make`; GitHub Actions adds orchestration and reporting details that should stay in workflow YAML.
- Root-level markdown files are the main public-facing documentation surface for the repository.
- README currently uses a compact badge presentation and should stay focused on adoption rather than maintainer policy.

### Integration Points
- New root docs: `CONTRIBUTING.md` and `SECURITY.md`.
- `README.md` top badge row and links to contributor/security docs.
- New `.github/dependabot.yml` for dependency automation.
- Optional maintainer appendix/companion doc if planners decide the split-doc approach needs a second file.

</code_context>

<specifics>
## Specific Ideas

- Use `security@amikos.tech` as the fallback contact published in `SECURITY.md`.
- Keep the first screen of `README.md` clean rather than turning it into a dense badge dashboard.
- Optimize for low contributor friction: external developers should be able to find the right local commands and reporting paths without reading source code.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 04-contributor-experience*
*Context gathered: 2026-04-12*
