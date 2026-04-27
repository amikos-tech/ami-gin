# Phase 3: CI Pipeline - Context

**Gathered:** 2026-04-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Add a GitHub Actions CI pipeline so every pull request and push to `main` is automatically tested, linted, built, security-scanned, and reported through a CI badge plus GitHub-native test artifacts. This phase covers workflow design and CI integrations only; contributor docs and broader repo polish stay in later phases.

</domain>

<decisions>
## Implementation Decisions

### CI entrypoints
- **D-01:** Use a hybrid model: keep `make` targets as the contributor-facing local interface, but keep CI-native orchestration in workflow YAML.
- **D-02:** Expand the local command surface to include the repo-standard `security-scan` and `integration-test` targets, but keep matrix wiring, schedules, SARIF upload, artifact upload, and coverage-summary logic in GitHub Actions rather than hiding them behind `make`.
- **D-03:** Use the default GitHub runner group for CI jobs. Do not target custom or self-hosted runner groups in this phase.

### Merge gates
- **D-04:** CI is security-enforced in Phase 3. Required checks should block merge when they fail.
- **D-05:** Any `govulncheck` finding surfaced on a pull request should block merge; this is not limited to high or critical findings.
- **D-06:** Lint and build remain required checks alongside the test matrix.

### Matrix scope
- **D-07:** Run `go test -race` in a two-version matrix on Go `1.25` and `1.26`.
- **D-08:** Run build, lint, and security jobs once on a canonical Go `1.26` job rather than duplicating them across the full matrix.
- **D-09:** Set `GOTOOLCHAIN: local` in all CI jobs so matrix coverage is not bypassed by automatic toolchain upgrades.

### Coverage reporting
- **D-10:** Keep coverage reporting GitHub-native in this phase: upload `coverage.out` and `unit.xml` as GitHub Actions artifacts and publish a coverage summary in the GitHub job output.
- **D-11:** Publish GitHub artifacts for `coverage.out` and `unit.xml` so maintainers can inspect raw CI outputs directly from GitHub.
- **D-12:** Do not introduce an external coverage service or any additional required coverage status in Phase 3.

### the agent's Discretion
The planner and implementer can choose:
- exact workflow file split (single CI workflow vs CI + scheduled security workflow)
- exact job names and artifact retention settings
- exact artifact retention settings and coverage-summary formatting, as long as reporting stays GitHub-native and compatible with the required PR gates
- exact mechanism for making PR `govulncheck` findings blocking while still publishing scheduled Code Scanning results weekly

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and acceptance
- `.planning/ROADMAP.md` §Phase 3: CI Pipeline — goal, success criteria, and required outcomes for test matrix, security scanning, and badges
- `.planning/REQUIREMENTS.md` §CI Pipeline — CI-01 through CI-04 requirements that must be satisfied in this phase
- `.planning/PROJECT.md` §Core Value and §Active — open-source readiness standard and current CI-related launch requirements

### Prior decisions that constrain CI
- `.planning/STATE.md` §Accumulated Context — carry forward the decisions to use `golang/govulncheck-action@v1` and set `GOTOOLCHAIN: local` in all CI jobs

### Existing repo interfaces
- `Makefile` — current local `build`, `test`, `lint`, and artifact-producing test flow that CI should align with for contributor-facing commands
- `.golangci.yml` — existing golangci-lint v2 configuration that Phase 3 extends for CI-02
- `go.mod` — current Go version baseline (`1.25.5`) that informs the `1.25` and `1.26` CI matrix
- `README.md` — integration point for the CI badge required by this phase

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `Makefile`: already provides `build`, `test`, `lint`, `lint-fix`, `clean`, and `help`; `make test` emits `coverage.out` and `unit.xml`
- `.golangci.yml`: existing lint policy is already centralized and should stay the single source of linter configuration
- `README.md`: existing user-facing documentation page where the CI badge will be added

### Established Patterns
- The repo is a single Go module with a root library package plus `cmd/gin-index` and `examples/*`; `go test ./...` is the broad validation command and currently passes locally
- Local developer workflows already lean on `make`, but current GitHub Actions coverage is limited to Claude automation, so there is no existing Go CI pipeline to preserve
- Security scanning and coverage reporting need GitHub-native behavior for SARIF, schedules, required checks, and external uploads; those concerns should stay visible in workflow YAML

### Integration Points
- `.github/workflows/`: add the PR/push CI workflow and the scheduled security scanning workflow(s)
- `Makefile`: add the missing repo-standard targets needed for local parity with CI
- `README.md`: add the live CI badge once workflows are in place; keep coverage reporting inside GitHub Actions artifacts and job output

</code_context>

<specifics>
## Specific Ideas

No stylistic preferences beyond:
- keep local contributor commands simple and discoverable through `make`
- keep the PR check wall clean by matrixing only the expensive compatibility-sensitive test job
- avoid custom runner-group coupling in this phase

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 03-ci-pipeline*
*Context gathered: 2026-04-11*
