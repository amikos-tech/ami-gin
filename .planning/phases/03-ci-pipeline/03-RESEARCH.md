# Phase 3: CI Pipeline - Research

**Researched:** 2026-04-11 [VERIFIED: local date]
**Domain:** GitHub Actions CI for a single-module Go library with coverage and security reporting [VERIFIED: .planning/ROADMAP.md] [VERIFIED: go.mod]
**Confidence:** MEDIUM [VERIFIED: research synthesis]

## Revision Note (2026-04-11)

This research file originally explored a Codecov-based interpretation of CI-04. That guidance is superseded by `.planning/phases/03-ci-pipeline/03-CONTEXT.md`, `.planning/ROADMAP.md`, and `.planning/REQUIREMENTS.md`, which are the authoritative inputs for Phase 3 execution.

For Phase 3, CI-04 means:
- upload `coverage.out` and `unit.xml` as GitHub Actions artifacts
- write the coverage summary to `$GITHUB_STEP_SUMMARY`
- do not add Codecov, a coverage badge, or any external coverage gate in this phase

Any remaining Codecov references below are historical research notes only and must not override the GitHub-native CI-04 contract above.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Use a hybrid model: keep `make` targets as the contributor-facing local interface, but keep CI-native orchestration in workflow YAML.
- **D-02:** Expand the local command surface to include the repo-standard `security-scan` and `integration-test` targets, but keep matrix wiring, schedules, SARIF upload, artifact upload, and coverage-summary logic in GitHub Actions rather than hiding them behind `make`.
- **D-03:** Use the default GitHub runner group for CI jobs. Do not target custom or self-hosted runner groups in this phase.
- **D-04:** CI is security-enforced in Phase 3. Required checks should block merge when they fail.
- **D-05:** Any `govulncheck` finding surfaced on a pull request should block merge; this is not limited to high or critical findings.
- **D-06:** Lint and build remain required checks alongside the test matrix.
- **D-07:** Run `go test -race` in a two-version matrix on Go `1.25` and `1.26`.
- **D-08:** Run build, lint, and security jobs once on a canonical Go `1.26` job rather than duplicating them across the full matrix.
- **D-09:** Set `GOTOOLCHAIN: local` in all CI jobs so matrix coverage is not bypassed by automatic toolchain upgrades.
- **D-10:** Keep coverage reporting GitHub-native in this phase: upload `coverage.out` and `unit.xml` as GitHub Actions artifacts and publish a coverage summary in the GitHub job output.
- **D-11:** Publish GitHub artifacts for `coverage.out` and `unit.xml` so maintainers can inspect raw CI outputs directly from GitHub.
- **D-12:** Do not introduce an external coverage service or any additional required coverage status in Phase 3. [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md]

### Claude's Discretion
The planner and implementer can choose:
- exact workflow file split (single CI workflow vs CI + scheduled security workflow)
- exact job names and artifact retention settings
- exact artifact retention settings and coverage-summary formatting, as long as reporting stays GitHub-native and compatible with the required PR gates
- exact mechanism for making PR `govulncheck` findings blocking while still publishing scheduled Code Scanning results weekly [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md]

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope. [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md]
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CI-01 | GitHub Actions CI workflow runs test matrix, lint, and build verification on PR and push to main [VERIFIED: .planning/REQUIREMENTS.md] | Use `ci.yml` with one Go test matrix job and separate canonical `lint` and `build` jobs; wire those job names into the existing `Main` ruleset after the first green run [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305] [CITED: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/troubleshooting-rules] |
| CI-02 | golangci-lint v2 config upgraded with gosec, errorlint, goconst, unconvert, unparam, nilerr, and prealloc linters [VERIFIED: .planning/REQUIREMENTS.md] | Extend the existing `.golangci.yml` `linters.enable` list; all seven linters are present in current golangci-lint docs and the repo already runs config version `2` [VERIFIED: .golangci.yml] [CITED: https://golangci-lint.run/docs/linters/] |
| CI-03 | govulncheck security scanning runs on a weekly schedule via GitHub Actions [VERIFIED: .planning/REQUIREMENTS.md] | Use a scheduled SARIF workflow because `govulncheck-action` only fails on vulnerabilities in `text` mode, while `sarif` mode succeeds and must be paired with `upload-sarif` for Code Scanning [CITED: https://github.com/golang/govulncheck-action] [CITED: https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/integrate-with-existing-tools/uploading-a-sarif-file-to-github?learn=code_security_integration] |
| CI-04 | gotestsum test outputs (`coverage.out` and `unit.xml`) are uploaded as GitHub Actions artifacts and coverage is summarized in GitHub job output [VERIFIED: .planning/REQUIREMENTS.md] | Keep `coverage.out` and `unit.xml` generation in the test job, upload them as GitHub Actions artifacts, and append the coverage summary to `$GITHUB_STEP_SUMMARY` on the canonical test leg [VERIFIED: Makefile] [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md] [CITED: https://github.com/actions/upload-artifact] |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- Contributor-facing commands must remain discoverable through `make`; this phase must leave or add the repo-standard `test`, `integration-test`, `lint`, `lint-fix`, `security-scan`, `clean`, and `help` targets. [VERIFIED: CLAUDE.md]
- If this phase touches Go error handling, use `github.com/pkg/errors` instead of `fmt.Errorf("%w")`. [VERIFIED: CLAUDE.md]
- If this phase introduces or edits Go config structs or constructors, follow the repository's functional-options plus validator/defaults conventions. [VERIFIED: CLAUDE.md]

## Summary

This repository currently has no Go CI workflow, no README CI badge, and no local `integration-test` or `security-scan` targets, but the baseline code quality is healthy enough to stand up CI immediately: `go test ./...`, `golangci-lint run`, `govulncheck ./...`, and `make test` all passed locally in this session, and `make test` already emits both `coverage.out` and `unit.xml`. [VERIFIED: .github/workflows] [VERIFIED: Makefile] [VERIFIED: local command go test ./...] [VERIFIED: local command golangci-lint run] [VERIFIED: local command govulncheck ./...] [VERIFIED: local command make test]

The clean implementation shape is a two-workflow design. `ci.yml` should run on `pull_request` and `push` to `main`, with a two-entry Go test matrix for `1.25` and `1.26`, plus single-run `lint`, `build`, and blocking `govulncheck` jobs on Go `1.26`. A separate `security.yml` should run weekly and manually, execute `govulncheck` in `sarif` mode, and upload the SARIF file to GitHub Code Scanning. This split is justified by official action behavior: `govulncheck-action` only fails the job in `text` mode, while `json` and `sarif` modes return success even when vulnerabilities are found. [CITED: https://docs.github.com/en/actions/tutorials/build-and-test-code/go] [CITED: https://github.com/golang/govulncheck-action] [CITED: https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/integrate-with-existing-tools/uploading-a-sarif-file-to-github?learn=code_security_integration]

The planner also needs to account for two non-code service prerequisites. First, `main` already has an active `Main` ruleset, but it currently lacks required status checks, so workflow YAML alone will not enforce merge gates. Second, the repository is still private and GitHub Code Security is disabled, so weekly SARIF uploads cannot surface in Code Scanning until the repo is made public or GitHub Code Security is enabled. [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305] [VERIFIED: gh api repos/amikos-tech/ami-gin] [VERIFIED: gh api repos/amikos-tech/ami-gin/code-scanning/default-setup] [CITED: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/troubleshooting-rules] [CITED: https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/integrate-with-existing-tools/uploading-a-sarif-file-to-github?learn=code_security_integration]

**Primary recommendation:** Implement `ci.yml` plus `security.yml`, use `govulncheck` `text` mode for PR/push blocking and `sarif` mode for weekly Code Scanning, keep coverage reporting GitHub-native through `coverage.out` / `unit.xml` artifacts plus `$GITHUB_STEP_SUMMARY`, and finish the phase by updating the existing `Main` ruleset to require the final CI job names. [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md] [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305] [CITED: https://github.com/golang/govulncheck-action]

## Standard Stack

### Core

| Library / Action | Version | Purpose | Why Standard |
|------------------|---------|---------|--------------|
| `actions/checkout` | `v6` major, latest release `v6.0.2` on 2026-01-09 [VERIFIED: gh release view actions/checkout] | Check out repo contents in every job [CITED: https://github.com/actions/checkout] | Official GitHub checkout action; current README usage is `@v6` and it is the default base step for other official examples. [CITED: https://github.com/actions/checkout] |
| `actions/setup-go` | `v6` major, latest release `v6.4.0` on 2026-03-30 [VERIFIED: gh release view actions/setup-go] | Install exact Go versions and enable built-in module caching [CITED: https://github.com/actions/setup-go] | Official GitHub action; supports `1.25`, `1.25.x`, `1.26`, `stable`, and built-in caching, which matches this phase's matrix needs. [CITED: https://github.com/actions/setup-go] |
| `golangci/golangci-lint-action` | `v9` major, latest release `v9.2.0` on 2025-12-02 [VERIFIED: gh release view golangci/golangci-lint-action] | Run `golangci-lint` with a pinned linter version [CITED: https://github.com/golangci/golangci-lint-action] | Official action from the golangci-lint authors; current README example uses `@v9` with linter `v2.11` and recommends a separate parallel job. [CITED: https://github.com/golangci/golangci-lint-action] |
| `golang/govulncheck-action` | `v1` major, latest release `v1.0.4` on 2024-10-02 [VERIFIED: gh release view golang/govulncheck-action] | Dependency and symbol-level vulnerability scanning [CITED: https://github.com/golang/govulncheck-action] | This is the official Go-team action, and project state already locks it in as the Trivy replacement for this repo. [VERIFIED: .planning/STATE.md] [CITED: https://github.com/golang/govulncheck-action] |
| `github/codeql-action/upload-sarif` | `v4` supported latest major [CITED: https://github.com/github/codeql-action] | Upload weekly `govulncheck.sarif` files into GitHub Code Scanning [CITED: https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/integrate-with-existing-tools/uploading-a-sarif-file-to-github?learn=code_security_integration] | GitHub documents this as the standard SARIF upload path and requires `security-events: write` for advanced code-scanning workflows. [CITED: https://github.com/github/codeql-action] [CITED: https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/integrate-with-existing-tools/uploading-a-sarif-file-to-github?learn=code_security_integration] |
| GitHub Actions artifacts + `$GITHUB_STEP_SUMMARY` | Built-in GitHub Actions primitives [VERIFIED: research synthesis] | Publish `coverage.out`, `unit.xml`, and the human-readable coverage summary without an external service [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md] [CITED: https://github.com/actions/upload-artifact] | This matches the locked GitHub-native CI-04 decision and avoids extra secrets, badges, or third-party merge gates in Phase 3. [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md] |
| `actions/upload-artifact` | `v7` major, latest release `v7.0.1` on 2026-04-10 [VERIFIED: gh release view actions/upload-artifact] | Publish `coverage.out` and `unit.xml` as downloadable CI artifacts [CITED: https://github.com/actions/upload-artifact] | Official GitHub artifact action; the current README usage examples have been bumped to `v7`. [VERIFIED: gh release view actions/upload-artifact v7.0.1] [CITED: https://github.com/actions/upload-artifact] |

### Supporting

| Library / Tool | Version | Purpose | When to Use |
|----------------|---------|---------|-------------|
| `gotest.tools/gotestsum` | `v1.13.0` published 2025-09-11 [VERIFIED: go list -m gotest.tools/gotestsum@latest] | Produce `unit.xml` and concise test logs [VERIFIED: Makefile] | Reuse for the test job so CI preserves the current `unit.xml` artifact contract instead of inventing a custom JUnit exporter. [VERIFIED: Makefile] |
| `actionlint` | `v1.7.11` installed locally [VERIFIED: local command actionlint -version] | Static validation of GitHub Actions workflow YAML [VERIFIED: local command actionlint -version] | Use for local and CI-side workflow validation because this phase changes YAML, permissions, and event wiring more than Go code. [VERIFIED: local command actionlint .github/workflows/*.yml] |
| Existing GitHub ruleset `Main` | Active repository ruleset `14266305` [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305] | Merge-gate enforcement for required checks [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305] | Update this existing ruleset instead of creating parallel branch-protection logic, because the repository is already using rulesets on `main`. [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Split `ci.yml` + `security.yml` [CITED: https://github.com/golang/govulncheck-action] | Single `ci.yml` with both text-mode and SARIF-mode jobs [CITED: https://github.com/golang/govulncheck-action] | A single file is possible, but two workflows keep PR gating and scheduled Code Scanning concerns separate, which is clearer because `sarif` mode does not fail on findings. [CITED: https://github.com/golang/govulncheck-action] |
| GitHub-native coverage artifacts + `$GITHUB_STEP_SUMMARY` [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md] | External coverage service in Phase 3 [SUPERSEDED by revision note] | The GitHub-native path satisfies CI-04 directly and avoids introducing additional auth, badges, or required checks before the repo is publicly launched. [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md] |
| Official actions (`setup-go`, `golangci-lint-action`, `upload-sarif`) [CITED: https://github.com/actions/setup-go] [CITED: https://github.com/golangci/golangci-lint-action] [CITED: https://github.com/github/codeql-action] | Shell scripts that curl binaries and post results manually [ASSUMED] | The official actions already solve version resolution, caching, and platform-specific details; shell scripts add maintenance burden without meeting any phase requirement better. [CITED: https://github.com/actions/setup-go] [CITED: https://github.com/golangci/golangci-lint-action] [CITED: https://github.com/github/codeql-action] |

**Local tool install:**
```bash
go install gotest.tools/gotestsum@v1.13.0
go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
```
[VERIFIED: go list -m gotest.tools/gotestsum@latest] [VERIFIED: go list -m golang.org/x/vuln@latest]

**Version verification:**
- `actions/checkout`: latest release `v6.0.2` published 2026-01-09. [VERIFIED: gh release view actions/checkout]
- `actions/setup-go`: latest release `v6.4.0` published 2026-03-30. [VERIFIED: gh release view actions/setup-go]
- `golangci/golangci-lint-action`: latest release `v9.2.0` published 2025-12-02. [VERIFIED: gh release view golangci/golangci-lint-action]
- `golang/govulncheck-action`: latest release `v1.0.4` published 2024-10-02. [VERIFIED: gh release view golang/govulncheck-action]
- `actions/upload-artifact`: latest release `v7.0.1` published 2026-04-10. [VERIFIED: gh release view actions/upload-artifact]

## Architecture Patterns

### Recommended Project Structure

```text
.github/
└── workflows/
    ├── ci.yml          # PR + push gates: test matrix, lint, build, blocking govulncheck
    └── security.yml    # Weekly + manual SARIF upload to GitHub Code Scanning
Makefile               # Local build/test/lint/security targets
.golangci.yml          # Single source of truth for lint policy
README.md              # CI badge only; coverage stays in GitHub artifacts + job summary
```
[VERIFIED: current repo tree] [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md]

### Pattern 1: Split Blocking and Reporting Security Paths

**What:** Use `govulncheck` twice with two different output modes: `text` in PR/push CI so vulnerabilities fail the job, and `sarif` in a weekly workflow so the results can be uploaded to Code Scanning. [CITED: https://github.com/golang/govulncheck-action]

**When to use:** Always for this phase, because the user locked in both blocking PR findings and weekly Code Scanning visibility. [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md]

**Example:**
```yaml
# Source: https://github.com/golang/govulncheck-action
# Source: https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/integrate-with-existing-tools/uploading-a-sarif-file-to-github?learn=code_security_integration
jobs:
  govulncheck:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v6
      - uses: golang/govulncheck-action@v1
        with:
          go-version-input: "1.26"
          go-package: ./...
          output-format: text

  govulncheck-code-scanning:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      security-events: write
    steps:
      - uses: actions/checkout@v6
      - uses: golang/govulncheck-action@v1
        with:
          go-version-input: "1.26"
          go-package: ./...
          output-format: sarif
          output-file: govulncheck.sarif
      - uses: github/codeql-action/upload-sarif@v4
        with:
          sarif_file: govulncheck.sarif
          category: govulncheck
```

### Pattern 2: Matrix Only the Expensive Test Job

**What:** Put the Go-version matrix only on the `test` job and keep `lint`, `build`, and blocking `govulncheck` as single canonical jobs on Go `1.26`. [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md]

**When to use:** For this repository, because the user explicitly scoped duplication to the test job only. [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md]

**Example:**
```yaml
# Source: https://docs.github.com/en/actions/tutorials/build-and-test-code/go
# Source: https://github.com/actions/setup-go
env:
  GOTOOLCHAIN: local

jobs:
  test:
    strategy:
      fail-fast: false
      matrix:
        go-version: ["1.25", "1.26"]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v6
        with:
          go-version: ${{ matrix.go-version }}
          cache-dependency-path: go.sum
      - run: go version
      - run: gotestsum --junitfile unit.xml -- -race -coverprofile=coverage.out ./...
```

### Pattern 3: Keep Local Verbs in `make`, but Keep CI Semantics in YAML

**What:** Add the missing local `integration-test` and `security-scan` targets, but keep matrix expansion, schedule triggers, artifact upload, coverage-summary logic, and ruleset enforcement in workflow YAML. [VERIFIED: Makefile] [VERIFIED: CLAUDE.md] [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md]

**When to use:** For this phase specifically, because the local interface is locked to `make`, while CI-native behavior is intentionally not hidden behind shell targets. [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md]

**Example:**
```make
# Source: repository Makefile pattern + phase decision D-01/D-02
.PHONY: integration-test
integration-test: test

.PHONY: security-scan
security-scan:
	govulncheck ./...
```
[VERIFIED: Makefile] [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md]

### Pattern 4: Update the Existing `Main` Ruleset After the First Green Run

**What:** Reuse the existing active ruleset on `main`, and add the final CI job names as required status checks after the workflow has run once and established the exact check contexts. [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305] [CITED: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/troubleshooting-rules]

**When to use:** Always here, because the repository already has a ruleset and currently has no required checks configured. [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305]

### Anti-Patterns to Avoid

- **Single SARIF-mode security gate:** `govulncheck-action` returns success in `sarif` mode even when vulnerabilities exist, so a SARIF-only PR job will silently stop blocking merges. [CITED: https://github.com/golang/govulncheck-action]
- **Workflow YAML without ruleset updates:** this repository already uses rulesets, and the current `Main` ruleset does not require any status checks, so commits can still merge without CI until the ruleset is updated. [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305]
- **Blindly calling `make test` in CI:** the current `test` target installs `gotestsum@latest` and does not add `-race`, which makes CI both time-dependent and insufficient for CI-01 unless the workflow adds pinning and race flags itself. [VERIFIED: Makefile]
- **Adding any external coverage gate now:** the user explicitly locked CI-04 to GitHub-native artifacts plus `$GITHUB_STEP_SUMMARY`, so an external coverage service or required coverage status would contradict the phase boundary. [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md]
- **Leaving `GOTOOLCHAIN` unset:** official `setup-go` can resolve versions from caches, manifests, and direct downloads, so leaving automatic toolchain selection enabled would undercut the user's fixed `1.25` / `1.26` matrix intent. [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md] [CITED: https://github.com/actions/setup-go]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Vulnerability scanning | Custom `go list`/CVE grep scripts [ASSUMED] | `golang/govulncheck-action@v1` [CITED: https://github.com/golang/govulncheck-action] | The official action already wraps the Go vulnerability tooling and understands blocking vs SARIF output modes. [CITED: https://github.com/golang/govulncheck-action] |
| SARIF ingestion | Raw REST calls to the Code Scanning API [ASSUMED] | `github/codeql-action/upload-sarif@v4` [CITED: https://github.com/github/codeql-action] | GitHub documents `upload-sarif` as the standard GitHub Actions path for third-party SARIF uploads. [CITED: https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/integrate-with-existing-tools/uploading-a-sarif-file-to-github?learn=code_security_integration] |
| Coverage reporting surface | Static SVG badge files or custom HTML summaries [ASSUMED] | GitHub Actions artifacts + `$GITHUB_STEP_SUMMARY` [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md] [CITED: https://github.com/actions/upload-artifact] | The locked CI-04 contract is already satisfied inside GitHub; this phase does not need a custom badge pipeline or external coverage host. [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md] |
| JUnit export | Custom parser around `go test -json` [ASSUMED] | `gotestsum --junitfile unit.xml` [VERIFIED: Makefile] | The repository already uses `gotestsum` for `unit.xml`, so keeping that contract avoids unnecessary churn. [VERIFIED: Makefile] |
| Merge-gate enforcement | Ad hoc bot comments or scripts that tell humans to wait [ASSUMED] | GitHub rulesets required status checks [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305] | Real merge blocking lives in repository rules, not in workflow comments or README instructions. [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305] |

**Key insight:** The hard parts of this phase are platform semantics, not YAML syntax. GitHub already provides the primitives for matrix execution, SARIF ingestion, artifacts, badges, and required checks, so the safest plan is to compose those primitives rather than replace them. [CITED: https://docs.github.com/en/actions/tutorials/build-and-test-code/go] [CITED: https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/integrate-with-existing-tools/uploading-a-sarif-file-to-github?learn=code_security_integration] [CITED: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/troubleshooting-rules]

## Common Pitfalls

### Pitfall 1: Assuming a Workflow Automatically Becomes a Merge Gate
**What goes wrong:** CI jobs run, but PRs are still mergeable because no required status checks were added to the active ruleset. [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305]
**Why it happens:** GitHub workflow files and GitHub rulesets are separate configuration planes. [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305] [CITED: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/troubleshooting-rules]
**How to avoid:** After the first successful workflow run, capture the exact job names and add them to the existing `Main` ruleset as required status checks. [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305] [CITED: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/troubleshooting-rules]
**Warning signs:** `gh api repos/amikos-tech/ami-gin/rulesets/14266305` shows no required-status-check rule, or PRs remain mergeable when CI is red. [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305]

### Pitfall 2: Using `sarif` Mode for the PR Security Gate
**What goes wrong:** PRs show a green security job even when vulnerabilities are present. [CITED: https://github.com/golang/govulncheck-action]
**Why it happens:** The official action explicitly states that `json` and `sarif` output modes return success even when vulnerabilities are detected. [CITED: https://github.com/golang/govulncheck-action]
**How to avoid:** Use `output-format: text` for PR/push blocking and reserve `output-format: sarif` for scheduled Code Scanning uploads. [CITED: https://github.com/golang/govulncheck-action]
**Warning signs:** A `govulncheck.sarif` artifact exists, but the job status is green for a known-vulnerable commit. [CITED: https://github.com/golang/govulncheck-action]

### Pitfall 3: Planning Code Scanning Without Enabling the Platform Feature
**What goes wrong:** The scheduled workflow can generate SARIF, but upload attempts fail or GitHub refuses to show results. [VERIFIED: gh api repos/amikos-tech/ami-gin/code-scanning/default-setup]
**Why it happens:** GitHub Code Scanning is available for public GitHub.com repositories, and for private organization repositories only when GitHub Code Security is enabled; this repository is private and has `code_security` disabled right now. [VERIFIED: gh api repos/amikos-tech/ami-gin] [VERIFIED: gh api repos/amikos-tech/ami-gin/code-scanning/default-setup] [CITED: https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/integrate-with-existing-tools/uploading-a-sarif-file-to-github?learn=code_security_integration]
**How to avoid:** Treat either “make the repo public” or “enable GitHub Code Security” as a prerequisite task inside the phase plan before claiming CI-03 complete. [VERIFIED: gh api repos/amikos-tech/ami-gin] [CITED: https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/integrate-with-existing-tools/uploading-a-sarif-file-to-github?learn=code_security_integration]
**Warning signs:** GitHub returns `Code Security must be enabled for this repository to use code scanning`. [VERIFIED: gh api repos/amikos-tech/ami-gin/code-scanning/default-setup]

### Pitfall 4: Misreading the Current Coverage Baseline
**What goes wrong:** Maintainers think the current baseline is about 71% because that is what `gotestsum` prints, but the actual file-based `go tool cover -func=coverage.out` total is 48.4% in this session. [VERIFIED: local command make test]
**Why it happens:** The current `Makefile` emits coverage through `gotestsum`, and the console summary is not the same number as the final total reported from the saved `coverage.out` file. [VERIFIED: Makefile] [VERIFIED: local command make test]
**How to avoid:** Base Phase 3 reporting on the uploaded `coverage.out` artifact and the explicit `$GITHUB_STEP_SUMMARY` output, not on ad hoc interpretations of the console log. [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md] [VERIFIED: local command make test]
**Warning signs:** `coverage.out` exists, `go tool cover -func=coverage.out` reports materially less coverage than the job log, and someone proposes a threshold anyway. [VERIFIED: local command make test]

## Code Examples

Verified patterns from official sources:

### Matrix Test Job with Explicit Go Versions and Cached Modules

```yaml
# Source: https://github.com/actions/setup-go
# Source: https://docs.github.com/en/actions/tutorials/build-and-test-code/go
name: CI

on:
  pull_request:
  push:
    branches: [main]

env:
  GOTOOLCHAIN: local

jobs:
  test:
    strategy:
      fail-fast: false
      matrix:
        go-version: ["1.25", "1.26"]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v6
        with:
          go-version: ${{ matrix.go-version }}
          cache-dependency-path: go.sum
      - run: gotestsum --format short-verbose --junitfile unit.xml -- -race -coverprofile=coverage.out ./...
      - uses: actions/upload-artifact@v7
        with:
          name: test-artifacts-go${{ matrix.go-version }}
          path: |
            unit.xml
            coverage.out
```

### Weekly SARIF Upload Job

```yaml
# Source: https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows
# Source: https://github.com/golang/govulncheck-action
# Source: https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/integrate-with-existing-tools/uploading-a-sarif-file-to-github?learn=code_security_integration
name: Security

on:
  workflow_dispatch:
  schedule:
    - cron: "23 7 * * 1"

jobs:
  govulncheck-code-scanning:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      security-events: write
    steps:
      - uses: actions/checkout@v6
      - uses: golang/govulncheck-action@v1
        with:
          go-version-input: "1.26"
          go-package: ./...
          output-format: sarif
          output-file: govulncheck.sarif
      - uses: github/codeql-action/upload-sarif@v4
        with:
          sarif_file: govulncheck.sarif
```

### GitHub Actions Badge Pattern

```markdown
[![CI](https://github.com/amikos-tech/ami-gin/actions/workflows/ci.yml/badge.svg?branch=main&event=push)](https://github.com/amikos-tech/ami-gin/actions/workflows/ci.yml)
```
[CITED: https://docs.github.com/en/actions/how-tos/monitor-workflows/add-a-status-badge]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| GitHub Docs examples still showing `actions/checkout@v5`, `actions/setup-go@v5`, and `actions/upload-artifact@v4` [CITED: https://docs.github.com/en/actions/tutorials/build-and-test-code/go] | Official action repos now document `checkout@v6`, `setup-go@v6`, and `upload-artifact@v7` [CITED: https://github.com/actions/checkout] [CITED: https://github.com/actions/setup-go] [CITED: https://github.com/actions/upload-artifact] | `checkout` `v6.0.2` on 2026-01-09, `setup-go` `v6.4.0` on 2026-03-30, `upload-artifact` `v7.0.1` on 2026-04-10 [VERIFIED: gh release view actions/checkout] [VERIFIED: gh release view actions/setup-go] [VERIFIED: gh release view actions/upload-artifact] | Prefer official action repo READMEs and release metadata over older GitHub Docs snippets when pinning versions for this phase. [CITED: https://github.com/actions/checkout] [CITED: https://github.com/actions/setup-go] [CITED: https://github.com/actions/upload-artifact] |
| Single `govulncheck` job expected to both block PRs and feed Code Scanning [ASSUMED] | Split `text` and `sarif` executions because only `text` mode fails on findings [CITED: https://github.com/golang/govulncheck-action] | This behavior is documented in the current action README [CITED: https://github.com/golang/govulncheck-action] | The phase architecture should reflect output-mode semantics, not just event triggers. [CITED: https://github.com/golang/govulncheck-action] |
| External coverage uploads considered part of CI-04 [SUPERSEDED by revision note] | CI-04 is now explicitly GitHub-native: artifacts plus `$GITHUB_STEP_SUMMARY` [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md] | Locked during the discuss/review cycle for Phase 3 [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md] | Remove external coverage-service assumptions from execution and validation; they no longer define success for this phase. [VERIFIED: planning synthesis] |

**Deprecated/outdated:**
- Relying on GitHub Docs action version snippets as the source of truth for pins is outdated for this phase because the official action repositories have already moved ahead. [CITED: https://docs.github.com/en/actions/tutorials/build-and-test-code/go] [CITED: https://github.com/actions/checkout] [CITED: https://github.com/actions/setup-go] [CITED: https://github.com/actions/upload-artifact]

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Shell-script replacements for official actions would add maintenance burden without solving a phase requirement better. [ASSUMED] | Standard Stack / Alternatives Considered | Low - the implementation could still use shell steps, but it would be harder to maintain. |
| A2 | A single SARIF-only job is a common intended design people may try first. [ASSUMED] | State of the Art | Low - the core documented behavior that SARIF mode succeeds on findings is still verified. |
| A3 | Static SVG badge files or bot-comment merge gates are plausible hand-rolled alternatives someone could reach for. [ASSUMED] | Don't Hand-Roll | Low - they are examples, not required plan steps. |

## Open Questions (RESOLVED)

1. **Will the repository be public before CI-03 is considered complete, or will GitHub Code Security be enabled on the private repo first?**
   - What we know: the repo is private, `security_and_analysis.code_security` is `disabled`, and GitHub currently returns `Code Security must be enabled for this repository to use code scanning`. [VERIFIED: gh api repos/amikos-tech/ami-gin] [VERIFIED: gh api repos/amikos-tech/ami-gin/code-scanning/default-setup]
   - Resolution: planning treats this as a blocking external prerequisite with two valid completion paths: either make the repository public, or keep it private and enable GitHub Code Security before claiming CI-03 complete. The plan must gate final verification on one of those states being true. [VERIFIED: planning synthesis]

2. **How should Phase 3 present coverage results without creating a second reporting surface?**
   - What we know: the current requirements and accepted plans define CI-04 as GitHub-native artifact upload plus `$GITHUB_STEP_SUMMARY`, and they explicitly reject adding an external coverage service in this phase. [VERIFIED: .planning/REQUIREMENTS.md] [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md]
   - Resolution: keep `coverage.out` and `unit.xml` as GitHub Actions artifacts, append `go tool cover -func=coverage.out | tail -1` to `$GITHUB_STEP_SUMMARY` on the canonical test leg, and defer any additional badge/service decisions to later phases. [VERIFIED: planning synthesis]

3. **What exact check names should be added to the `Main` ruleset?**
   - What we know: GitHub rulesets key required status checks by check name, and the repository already has an active ruleset with no required checks today. [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305] [CITED: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/troubleshooting-rules]
   - Resolution: fix the workflow job names to the stable contexts `test (1.25)`, `test (1.26)`, `lint`, `build`, and `govulncheck`, then update ruleset `14266305` only after a PR-triggered CI run has emitted those exact contexts. [VERIFIED: planning synthesis]

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go CLI | Local verification and coverage generation [VERIFIED: Makefile] | ✓ [VERIFIED: local command go version] | `go1.26.2` [VERIFIED: local command go version] | CI can still install `1.25` and `1.26` via `setup-go`. [CITED: https://github.com/actions/setup-go] |
| GNU Make | Contributor-facing `make` targets [VERIFIED: CLAUDE.md] | ✓ [VERIFIED: local command make --version] | `3.81` [VERIFIED: local command make --version] | Direct `go` commands exist, but phase decisions prefer `make` locally. [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md] |
| `golangci-lint` | Local parity for CI-02 [VERIFIED: .golangci.yml] | ✓ [VERIFIED: local command golangci-lint version] | `2.11.4` [VERIFIED: local command golangci-lint version] | CI action can install `v2.11` itself. [CITED: https://github.com/golangci/golangci-lint-action] |
| `govulncheck` | Local parity for blocking security job [VERIFIED: phase requirement CI-03] | ✓ [VERIFIED: local command govulncheck -version] | module `golang.org/x/vuln v1.1.4` latest, local scanner DB updated 2026-04-08 [VERIFIED: go list -m golang.org/x/vuln@latest] [VERIFIED: local command govulncheck -version] | CI action can install and run it. [CITED: https://github.com/golang/govulncheck-action] |
| `gotestsum` | `unit.xml` generation and terse test logs [VERIFIED: Makefile] | ✓ [VERIFIED: local command gotestsum --version] | `v1.13.0` [VERIFIED: local command gotestsum --version] | Raw `go test -json` is possible, but would drop current artifact shape unless reworked. [VERIFIED: Makefile] |
| `actionlint` | Local workflow validation [VERIFIED: local command actionlint -version] | ✓ [VERIFIED: local command actionlint -version] | `v1.7.11` [VERIFIED: local command actionlint -version] | GitHub will catch syntax/runtime issues later, but that is a slower feedback loop. [VERIFIED: local environment] |
| GitHub CLI authenticated access | Inspect and update rulesets / repo settings [VERIFIED: gh auth status] | ✓ [VERIFIED: gh auth status] | `gh 2.89.0` [VERIFIED: local command gh --version] | GitHub web UI can make the same changes manually. [VERIFIED: platform capability] |
| GitHub Code Scanning availability | Weekly SARIF uploads for CI-03 [VERIFIED: phase requirement CI-03] | ✗ for current repo state [VERIFIED: gh api repos/amikos-tech/ami-gin/code-scanning/default-setup] | repo is private and `code_security` is disabled [VERIFIED: gh api repos/amikos-tech/ami-gin] | Make the repo public or enable GitHub Code Security before claiming the requirement complete. [CITED: https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/integrate-with-existing-tools/uploading-a-sarif-file-to-github?learn=code_security_integration] |
| GitHub Actions artifacts and summaries | GitHub-native CI-04 reporting [VERIFIED: phase requirement CI-04] | ✓ once workflows are created [VERIFIED: planning synthesis] | Uses built-in artifact upload + `$GITHUB_STEP_SUMMARY` | No extra service auth required; implement directly in `ci.yml`. [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md] |

**Missing dependencies with no fallback:**
- GitHub Code Scanning cannot be satisfied on the current private repo until either the repo is public or GitHub Code Security is enabled. [VERIFIED: gh api repos/amikos-tech/ami-gin/code-scanning/default-setup] [CITED: https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/integrate-with-existing-tools/uploading-a-sarif-file-to-github?learn=code_security_integration]

**Missing dependencies with fallback:**
- None for CI-04; the GitHub-native coverage path does not require an external service secret. [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md]

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` + `gotestsum v1.13.0` for CI-style output. [VERIFIED: .planning/codebase/TESTING.md] [VERIFIED: go list -m gotest.tools/gotestsum@latest] |
| Config file | No dedicated test config file; the effective config lives in `Makefile` and `.golangci.yml`. [VERIFIED: Makefile] [VERIFIED: .golangci.yml] |
| Quick run command | `actionlint .github/workflows/*.yml && go test ./...` [VERIFIED: local command actionlint .github/workflows/*.yml] [VERIFIED: local command go test ./...] |
| Full suite command | `actionlint .github/workflows/*.yml && golangci-lint run && make test && govulncheck ./... && go test -race ./...` [VERIFIED: local command actionlint .github/workflows/*.yml] [VERIFIED: local command golangci-lint run] [VERIFIED: local command make test] [VERIFIED: local command govulncheck ./...] |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CI-01 | PR and `main` push run the test matrix plus lint and build jobs [VERIFIED: .planning/REQUIREMENTS.md] | static + manual GitHub smoke [VERIFIED: validation design] | `actionlint .github/workflows/ci.yml` [VERIFIED: local command actionlint -version] | ❌ Wave 0 - `.github/workflows/ci.yml` does not exist yet. [VERIFIED: .github/workflows] |
| CI-02 | `.golangci.yml` enables the required extra linters and still runs cleanly [VERIFIED: .planning/REQUIREMENTS.md] | static + smoke [VERIFIED: validation design] | `golangci-lint run` [VERIFIED: local command golangci-lint run] | ✅ `.golangci.yml` exists, but it needs more linters added. [VERIFIED: .golangci.yml] |
| CI-03 | Weekly `govulncheck` schedule produces SARIF and uploads it to Code Scanning [VERIFIED: .planning/REQUIREMENTS.md] | static + manual GitHub smoke [VERIFIED: validation design] | `actionlint .github/workflows/security.yml` [VERIFIED: local command actionlint -version] | ❌ Wave 0 - `.github/workflows/security.yml` does not exist yet. [VERIFIED: .github/workflows] |
| CI-04 | gotestsum outputs `coverage.out` and `unit.xml`, uploads them as GitHub Actions artifacts, and writes a coverage summary to the GitHub job output [VERIFIED: .planning/REQUIREMENTS.md] | smoke + manual GitHub check [VERIFIED: validation design] | `make test && test -f coverage.out && test -f unit.xml && go tool cover -func=coverage.out | tail -1` [VERIFIED: local command make test] | ✅ `README.md` exists; coverage reporting is satisfied in workflow output rather than by an external badge. [VERIFIED: README.md] |

### Sampling Rate

- **Per task commit:** `actionlint .github/workflows/*.yml && go test ./...` [VERIFIED: local command actionlint .github/workflows/*.yml] [VERIFIED: local command go test ./...]
- **Per wave merge:** `actionlint .github/workflows/*.yml && golangci-lint run && make test && govulncheck ./...` [VERIFIED: local command actionlint .github/workflows/*.yml] [VERIFIED: local command golangci-lint run] [VERIFIED: local command make test] [VERIFIED: local command govulncheck ./...]
- **Phase gate:** Run the full suite above, then verify one PR-triggered run, one `main` push run, one weekly/manual security run, the README badges, and the ruleset required checks in GitHub. [VERIFIED: phase success criteria] [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305]

### Wave 0 Gaps

- [ ] `.github/workflows/ci.yml` - missing; needed for CI-01 static validation. [VERIFIED: .github/workflows]
- [ ] `.github/workflows/security.yml` - missing; needed for CI-03 static validation. [VERIFIED: .github/workflows]
- [ ] Ruleset update - existing `Main` ruleset has no required status checks yet. [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305]
- [ ] GitHub Code Scanning enablement or repo visibility change - current private/disabled state blocks CI-03 completion. [VERIFIED: gh api repos/amikos-tech/ami-gin] [VERIFIED: gh api repos/amikos-tech/ami-gin/code-scanning/default-setup]
- [ ] Explicit race-run verification - `go test -race ./...` has not been confirmed green in this session and should be treated as a deliberate verification step during implementation. [VERIFIED: local session observation]

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes [VERIFIED: phase scope] | Use GitHub-provided `GITHUB_TOKEN` permissions only; Phase 3 should not add a second credential path for coverage reporting. [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md] [CITED: https://github.com/github/codeql-action] |
| V3 Session Management | no [VERIFIED: phase scope] | This phase does not add application sessions; keep scope on CI credentials only. [VERIFIED: phase scope] |
| V4 Access Control | yes [VERIFIED: phase scope] | Enforce required status checks in the existing `Main` ruleset, and keep job permissions minimal (`contents: read`, `security-events: write` only where needed). [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305] [CITED: https://github.com/github/codeql-action] |
| V5 Input Validation | no direct app-surface change [VERIFIED: phase scope] | The phase changes CI config, not request parsing; validation here is workflow linting and linter configuration quality. [VERIFIED: phase scope] |
| V6 Cryptography | yes [VERIFIED: phase scope] | Use platform-managed GitHub credentials where the phase needs auth; do not invent custom signing, encryption, or credential transport for CI integrations. [CITED: https://github.com/github/codeql-action] |

### Known Threat Patterns for GitHub Actions + Go CI

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Over-broad workflow permissions | Elevation of Privilege | Set default job permissions to `contents: read` and add `security-events: write` only for the SARIF upload job. [CITED: https://github.com/github/codeql-action] |
| Supply-chain drift from `@latest` tool installs | Tampering | Pin action majors and pin `gotestsum` when CI invokes it; do not let `make test` install arbitrary future versions in the CI path. [VERIFIED: Makefile] [VERIFIED: gh release view actions/checkout] [VERIFIED: go list -m gotest.tools/gotestsum@latest] |
| Non-blocking vulnerability findings due to wrong output mode | Repudiation | Use `govulncheck` `text` mode for PR/push gates and `sarif` only for reporting workflows. [CITED: https://github.com/golang/govulncheck-action] |
| Merge without successful CI despite existing workflow files | Elevation of Privilege | Add required status checks to the active `Main` ruleset after the first green run. [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305] [CITED: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/troubleshooting-rules] |
| Confusing or duplicated coverage surfaces | Information Disclosure | Keep Phase 3 coverage reporting GitHub-native so there is one canonical location for raw artifacts and the human-readable summary. [VERIFIED: .planning/phases/03-ci-pipeline/03-CONTEXT.md] |

## Sources

### Primary (HIGH confidence)

- Local repo files: `Makefile`, `.golangci.yml`, `README.md`, `.github/workflows/claude.yml`, `.github/workflows/claude-code-review.yml`, `go.mod`, `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md`, `.planning/PROJECT.md`, `.planning/STATE.md`, `.planning/codebase/TESTING.md`. [VERIFIED: repo reads]
- GitHub Docs - Building and testing Go: https://docs.github.com/en/actions/tutorials/build-and-test-code/go [CITED: https://docs.github.com/en/actions/tutorials/build-and-test-code/go]
- GitHub Docs - Events that trigger workflows (`schedule`): https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows [CITED: https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows]
- GitHub Docs - Add a workflow status badge: https://docs.github.com/en/actions/how-tos/monitor-workflows/add-a-status-badge [CITED: https://docs.github.com/en/actions/how-tos/monitor-workflows/add-a-status-badge]
- GitHub Docs - Uploading a SARIF file to GitHub: https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/integrate-with-existing-tools/uploading-a-sarif-file-to-github?learn=code_security_integration [CITED: https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/integrate-with-existing-tools/uploading-a-sarif-file-to-github?learn=code_security_integration]
- GitHub Docs - Control workflow concurrency: https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency [CITED: https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency]
- GitHub Docs - Troubleshooting rules: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/troubleshooting-rules [CITED: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/troubleshooting-rules]
- Official action repos: `actions/checkout`, `actions/setup-go`, `actions/upload-artifact`, `golangci/golangci-lint-action`, `golang/govulncheck-action`, `github/codeql-action`. [CITED: https://github.com/actions/checkout] [CITED: https://github.com/actions/setup-go] [CITED: https://github.com/actions/upload-artifact] [CITED: https://github.com/golangci/golangci-lint-action] [CITED: https://github.com/golang/govulncheck-action] [CITED: https://github.com/github/codeql-action]

### Secondary (MEDIUM confidence)

- `gh` API observations for repo state, rulesets, secrets, and code-scanning availability. [VERIFIED: gh auth status] [VERIFIED: gh api repos/amikos-tech/ami-gin] [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305] [VERIFIED: gh secret list -R amikos-tech/ami-gin]

### Tertiary (LOW confidence)

- None. All non-local ecosystem claims above were backed by official docs or official repositories. [VERIFIED: research synthesis]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - action versions, Go version syntax, SARIF upload behavior, and linter names were verified from official repos/docs plus local release metadata. [VERIFIED: gh release view actions/checkout] [CITED: https://github.com/actions/setup-go] [CITED: https://github.com/golang/govulncheck-action] [CITED: https://golangci-lint.run/docs/linters/]
- Architecture: HIGH - the split workflow recommendation is directly driven by official `govulncheck-action` semantics and current repo state from `gh api`. [CITED: https://github.com/golang/govulncheck-action] [VERIFIED: gh api repos/amikos-tech/ami-gin/rulesets/14266305]
- Pitfalls: MEDIUM - the platform-level hazards are verified, but some implementation-specific failure modes still depend on final job naming and repo-visibility timing. [VERIFIED: gh api repos/amikos-tech/ami-gin] [CITED: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/troubleshooting-rules]

**Research date:** 2026-04-11 [VERIFIED: local date]
**Valid until:** 2026-05-11 for local repo facts, or earlier if GitHub Actions major versions, repo visibility, or Code Security settings change. [VERIFIED: research synthesis]
