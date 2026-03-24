# Project Research Summary

**Project:** GIN Index — OSS Infrastructure Layer
**Domain:** Open-source Go library CI/CD and tooling infrastructure
**Researched:** 2026-03-24
**Confidence:** HIGH

## Executive Summary

The GIN Index library has a mature, well-tested codebase (1.35:1 test-to-source ratio, 10 runnable examples, property-based tests, race-condition coverage) but entirely lacks the infrastructure layer that signals credibility to potential adopters. The gap is not in code quality — the code quality is good — it is in CI/CD automation, release management, security scanning, contributor onboarding, and correct module path resolution. A library without these artifacts reads as a personal experiment, not a maintained OSS project.

The Go OSS tooling ecosystem in 2026 is well-consolidated and the right choices are unambiguous. GitHub Actions is the dominant CI platform for public Go libraries (free, native, widest ecosystem). golangci-lint v2.11.4 is the standard lint aggregator. GoReleaser v2.14.3 handles changelog generation and GitHub Release creation for library-only projects without binary builds. govulncheck (official Go team tool) is the security scanner of choice — critically, the Trivy GitHub Action was supply-chain compromised in March 2026 and must be avoided. pkg.go.dev provides automatic API documentation for any public Go module at zero ongoing cost.

The two hard blockers for public launch are: (1) the module path mismatch — the repo is `ami-gin` but `go.mod` declares `gin-index`, so `go get` will not work — and (2) the absence of a LICENSE file, which makes the project legally unusable by anyone. Everything else is incremental. The module path rename is the highest-risk mechanical operation and must be done first, in a single isolated commit, before any CI or tooling is layered on top of the correct paths.

## Key Findings

### Recommended Stack

The infrastructure stack is a standard, well-proven combination for Go OSS libraries. All versions listed are current as of March 2026 and verified against official release pages.

**Core technologies:**
- `actions/checkout@v6` + `actions/setup-go@v6`: CI checkout and Go toolchain — current stable, node24 runtime, built-in module/build caching
- `golangci-lint v2.11.4` via `golangci-lint-action@v9`: Lint aggregator — v2 config format, 100+ linters, single CI step
- `goreleaser v2.14.3` via `goreleaser-action@v7`: Release automation — library-only mode (`builds: skip: true`), changelog from conventional commits
- `golang/govulncheck-action@v1`: Security scanning — official Go team, symbol-level analysis, not just dependency matching
- `codecov/codecov-action@v5`: Coverage reporting — free for OSS, PR annotations, recognized badge
- `Dependabot` (built-in GitHub): Dependency updates — zero config, native integration, weekly PRs for both Go modules and GitHub Actions
- `pkg.go.dev`: API documentation — automatic for any public Go module, zero maintenance

**Go version matrix:** Test on Go 1.25.x and 1.26.x. Go 1.26.1 is the current stable release. The `go.mod` should declare `go 1.25` (without patch version) as the minimum.

**Not recommended:** Trivy (supply chain breach March 2026), Renovate (overkill for small dependency tree), custom docs site (pkg.go.dev handles it), pre-commit hooks (contributor friction), Coveralls (Codecov has better Go integration).

### Expected Features

**Must have (table stakes — missing = project feels amateur):**
- LICENSE file (Apache-2.0) — legal blocker; no license = legally unusable
- Module path matches repository — `go get` is broken without this
- CI badge in README — first thing visitors check to assess health
- Passing CI on main branch — broken main signals dead project
- GoDoc comments on all exports — pkg.go.dev renders from source; missing = unusable API
- Coverage reporting — signals testing discipline; the existing 1.35:1 ratio suggests coverage is likely strong
- CONTRIBUTING.md — tells contributors how to build, test, lint without reading all source
- Semantic version tag — Go modules require semver; `v0.1.0` signals pre-stable API
- Automated dependency updates — signals active maintenance even during quiet periods
- Security scanning in CI — expected for any library that deserializes or processes untrusted data

**Should have (differentiators — signal quality above baseline):**
- Automated changelog on releases — categorized release notes from conventional commits via GoReleaser
- Coverage badge — visual proof of test quality in README
- Go Report Card badge — third-party quality validation, zero config
- Matrix testing (Go 1.25 + 1.26) — proves multi-version compatibility
- Branch protection rules — prevents broken merges to main
- SECURITY.md — responsible disclosure channel (add before v1.0.0)

**Defer to after initial launch:**
- Codecov coverage thresholds (measure baseline before setting gates)
- Branch protection enforcement (add once CI is stable and contributors exist)
- SECURITY.md (add before v1.0.0, not strictly needed for v0.1.0)
- GoReleaser can be added for v0.2.0 if v0.1.0 is tagged manually

**Anti-features (do not build):**
- Custom documentation site — pkg.go.dev handles this automatically
- Docker image — this is a library, not a service
- Pre-commit hooks — adds contributor friction; CI catches the same failures
- CLA — overkill for a small project; Apache-2.0 includes patent grant

### Architecture Approach

The OSS infrastructure is three GitHub Actions workflows plus supporting configuration files. No changes to the GIN Index library code itself are needed — this is purely the infrastructure layer around the existing codebase. All three workflows are independent and can be implemented incrementally; the repo can go public after any phase.

**Major components:**
1. `ci.yml` — Test, lint, vet, coverage, security scan; triggered on push to main and all PRs; matrix across Go 1.25 and 1.26; parallel jobs (test, lint, security)
2. `release.yml` — GoReleaser changelog generation and GitHub Release creation; triggered on tag push (`v*`); library-only mode (no binary builds)
3. `security.yml` — Scheduled weekly govulncheck with SARIF output uploaded to GitHub Code Scanning; also runs on push to main
4. `.github/dependabot.yml` — Weekly PRs for Go module and GitHub Actions updates; AWS SDK packages grouped to reduce noise
5. `.golangci.yml` (upgraded) — v2 config format with expanded linter set: gosec, errorlint, prealloc, nilerr, copyloopvar added to existing dupword, gocritic, mirror
6. `.goreleaser.yml` — Library-only release config with conventional commit changelog grouping
7. `LICENSE`, `CONTRIBUTING.md` — Static onboarding files

**Key anti-patterns to avoid:**
- Single monolithic CI job: separate test, lint, and security into parallel jobs
- `@latest` for tool versions in CI: pin to major versions (Dependabot updates them)
- `fetch-depth: 1` in release workflow: GoReleaser needs full git history for changelogs
- Testing without `-race`: bitmap and builder code can have data races

### Critical Pitfalls

1. **Go toolchain auto-upgrade defeats matrix testing** — When `go.mod` contains a `toolchain` directive, Go 1.21+ silently upgrades to that version, making all matrix entries test the same Go version. Fix: set `GOTOOLCHAIN: local` in all CI jobs. Without this, CI gives false confidence about Go 1.25 compatibility.

2. **Module path rename leaves stale imports** — Changing `go.mod` from `gin-index` to `ami-gin` without updating every `import` statement in every `.go` and `_test.go` file causes CI failures in clean environments. The `.golangci.yml` gci prefix must also be updated. Do the rename in a single isolated commit, verify with `go build ./...` and `go test ./...` before anything else.

3. **Trivy GitHub Action supply chain compromise (March 2026)** — All Trivy action tags (0.0.1 through 0.34.2) were compromised with credential stealers. Do not use `aquasecurity/trivy-action` or `aquasecurity/setup-trivy`. Use `golang/govulncheck-action@v1` instead.

4. **Publishing a broken tag to Go module proxy** — The Go module proxy caches versions permanently. Tagging a broken commit means users get a broken library until a new patch version is published. Tag only after CI passes on main. Use `v0.x.x` to signal pre-stable API.

5. **GoReleaser empty changelog** — Default `actions/checkout` uses `fetch-depth: 1` (shallow clone). GoReleaser needs full git history to generate changelogs between tags. Set `fetch-depth: 0` in the release workflow checkout step only.

## Implications for Roadmap

Based on cross-research synthesis, the work naturally falls into four phases with clear dependency ordering.

### Phase 1: Foundation
**Rationale:** The module path is a dependency for everything else — CI import paths, README badges, pkg.go.dev indexing, and `go get` all break without a correct module path. LICENSE is a legal blocker that prevents adoption. Both must be resolved before any other work, in isolation, to avoid compounding errors.
**Delivers:** A legally usable codebase with a working `go get` path, even before CI exists
**Addresses:** Module path mismatch (table stakes), LICENSE (legal blocker), `go.mod` `go` directive patch version cleanup
**Avoids:** Module rename pitfall (single commit, isolated, verified before layering CI on top)
**Tasks:**
- Rename module from `github.com/amikos-tech/gin-index` to `github.com/amikos-tech/ami-gin` (all `.go`, `_test.go`, `.golangci.yml`)
- Add `LICENSE` file (Apache-2.0)
- Fix `go.mod` `go` directive from `go 1.25.5` to `go 1.25`
- Verify: `go build ./...` and `go test ./...` pass in clean environment

### Phase 2: CI Pipeline
**Rationale:** CI must exist before Dependabot PRs are useful (nothing validates them), before coverage is reportable (no workflow to generate `coverage.out`), and before branch protection has checks to require. The golangci-lint config upgrade is a prerequisite for the CI lint job.
**Delivers:** Green CI badge on main, automated testing on every PR across Go 1.25 and 1.26, security scanning integrated
**Addresses:** CI badge (table stakes), passing CI on main (table stakes), security scanning (table stakes), linting upgrade
**Avoids:** Toolchain auto-upgrade pitfall (`GOTOOLCHAIN: local`), monolithic job anti-pattern (parallel jobs), testing without `-race`, `@latest` tool version non-reproducibility
**Tasks:**
- Upgrade `.golangci.yml` with expanded linter set (gosec, errorlint, prealloc, nilerr, copyloopvar)
- Create `.github/workflows/ci.yml` (test matrix, lint, security jobs in parallel)
- Create `.github/workflows/security.yml` (weekly govulncheck with SARIF upload)
- Add `security-scan` and `coverage` targets to Makefile

### Phase 3: Contributor Experience
**Rationale:** With CI in place, badges are meaningful (they display live status), Dependabot PRs can be automatically validated by CI, and a CONTRIBUTING.md has accurate commands to reference. Adding these before CI would produce a broken or misleading experience.
**Delivers:** Credible README, documented contributor workflow, automated maintenance signal, visual quality indicators
**Addresses:** CONTRIBUTING.md (table stakes), README badges (table stakes), automated dependency updates (table stakes), Go Report Card + coverage badges (differentiators)
**Avoids:** Badge 404 pitfall (module path correct from Phase 1), Dependabot AWS SDK PR flood (grouped in config)
**Tasks:**
- Add `CONTRIBUTING.md` with build/test/lint/security-scan instructions
- Update `README.md` with badges (CI, GoDoc, Go Report Card, license, coverage)
- Add `.github/dependabot.yml` (Go modules + GitHub Actions, weekly, AWS SDK grouped)
- Configure Codecov integration

### Phase 4: Release Automation
**Rationale:** Releasing is the final step. It requires: stable CI (Phase 2) to avoid publishing broken tags, correct module path (Phase 1) for `go get` to work, and complete contributor docs (Phase 3) so users know how to consume the library. GoReleaser is additive — the first tag can be manual if needed.
**Delivers:** Automated changelog and GitHub Release creation on tag push; first consumable version (`v0.1.0`) published
**Addresses:** Semantic versioning (table stakes), automated changelog (differentiator)
**Avoids:** Broken tag pitfall (CI already validates before tagging), empty changelog pitfall (`fetch-depth: 0`), broken import path in release notes
**Tasks:**
- Create `.goreleaser.yml` (library-only, conventional commit changelog grouping)
- Create `.github/workflows/release.yml` (tag-triggered, `fetch-depth: 0`)
- Add branch protection rules (require CI to pass before merge)
- Push first semver tag `v0.1.0` after CI is green

### Phase Ordering Rationale

- Phase 1 must be first: module path is a hard dependency for CI (import paths must resolve), badges (URLs must be correct), pkg.go.dev indexing, and `go get`. Doing it first in isolation minimizes blast radius.
- Phase 2 before Phase 3: badges and Dependabot are meaningless without CI. A badge pointing to a non-existent workflow shows as missing; Dependabot PRs with no CI validation cannot be trusted.
- Phase 3 before Phase 4: GoReleaser changelog references contributors, CONTRIBUTING.md should exist before the library is publicly discoverable via `go get`, and Codecov must be set up before coverage badges show real data.
- Phase 4 last: releasing is irreversible. Every preceding phase de-risks the release. Each phase is independently deployable — the repo can be made public after Phase 1 or Phase 2 without waiting for Phase 4.

### Research Flags

Phases with well-documented patterns (deeper research not needed during planning):
- **Phase 1:** Mechanical find-and-replace with known verification commands. No ambiguity.
- **Phase 2:** All action versions, workflow patterns, and golangci-lint config are fully documented in STACK.md and ARCHITECTURE.md with exact YAML. Copy-paste ready.
- **Phase 3:** Standard files with known patterns. CONTRIBUTING.md template, Dependabot config, badge URLs all documented.
- **Phase 4:** GoReleaser library-only cookbook is well-documented. Config is fully specified in STACK.md.

No phases in this project require `/gsd:research-phase` during planning. All tools are mature, all versions verified, all configurations documented in the research files.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All tool versions verified via web search against official GitHub release pages. Multiple independent sources corroborate. Actions versions checked against GitHub marketplace. |
| Features | HIGH | Based on direct analysis of existing codebase state combined with standard Go OSS expectations. No speculation — gaps are objectively observable (missing LICENSE, missing CI, etc.). |
| Architecture | HIGH | GitHub Actions workflow patterns are extensively documented by GitHub, golangci-lint, GoReleaser, and govulncheck official docs. Patterns are widely reproduced across Go OSS projects. |
| Pitfalls | HIGH | Critical pitfalls verified with authoritative sources: GOTOOLCHAIN pitfall from Brandur's documented experience, Trivy compromise from TheHackerNews + CrowdStrike analysis, Go module proxy permanence from official Go docs. |

**Overall confidence:** HIGH

### Gaps to Address

- **golangci-lint `standard` default linter set:** The exact list of linters included in the v2 `standard` preset was not found in official docs. Resolution: run `golangci-lint help linters` locally after upgrading to see the full default set. LOW impact — the additional linters recommended (gosec, errorlint, etc.) are explicitly named and enabled regardless of the default set.
- **Current coverage baseline:** The actual coverage percentage is unknown. The 1.35:1 test-to-source ratio is a positive signal but not a direct measure. Resolution: run `go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out` before setting any Codecov thresholds. MEDIUM impact — setting Codecov thresholds before measuring baseline can block all PRs (Pitfall 6).
- **`go.mod` patch version user impact:** Whether `go 1.25.5` in the `go` directive causes issues for users on Go 1.25.3 or 1.25.4 needs verification. Resolution: change to `go 1.25` (standard practice) during Phase 1 module path work. MEDIUM user-experience impact if left unfixed.
- **Codecov `.codecov.yml` configuration:** Specific Codecov config for patch vs project coverage thresholds was not researched in depth. Resolution: use Codecov defaults for initial setup, tune after baseline is established. LOW impact for v0.1.0.

## Sources

### Primary (HIGH confidence)
- [golangci-lint releases](https://github.com/golangci/golangci-lint/releases) — v2.11.4 verified
- [golangci-lint v2 config docs](https://golangci-lint.run/docs/configuration/file/) — v2 format verified
- [golangci-lint-action](https://github.com/golangci/golangci-lint-action) — v9 verified
- [GoReleaser releases](https://github.com/goreleaser/goreleaser/releases) — v2.14.3 verified
- [GoReleaser library cookbook](https://goreleaser.com/cookbooks/release-a-library/) — library-only config
- [goreleaser-action](https://github.com/goreleaser/goreleaser-action) — v7 verified
- [actions/setup-go](https://github.com/actions/setup-go) — v6 verified
- [actions/checkout](https://github.com/actions/checkout) — v6 verified
- [govulncheck-action](https://github.com/golang/govulncheck-action) — v1 verified
- [codecov-action](https://github.com/codecov/codecov-action) — v5 verified
- [Go 1.26 release](https://go.dev/blog/go1.26) — Feb 10, 2026; 1.26.1 March 6, 2026
- [GitHub Actions: Building and testing Go](https://docs.github.com/en/actions/use-cases-and-examples/building-and-testing/building-and-testing-go) — workflow patterns
- [Go module proxy behavior](https://go.dev/ref/mod#module-proxy) — version permanence

### Secondary (MEDIUM confidence)
- [brandur.org — Go version CI matrix](https://brandur.org/fragments/go-version-matrix) — GOTOOLCHAIN: local pitfall
- [golangci-lint v2 announcement](https://ldez.github.io/blog/2025/03/23/golangci-lint-v2/) — v2 migration
- [Go Report Card](https://goreportcard.com/) — badge service
- [pkg.go.dev about](https://pkg.go.dev/about) — automatic indexing behavior
- [Dependabot Go support](https://github.blog/changelog/2025-12-09-dependabot-dgs-for-go/) — Go module support

### Tertiary (verified incident reports)
- [TheHackerNews — Trivy Action compromise](https://thehackernews.com/2026/03/trivy-security-scanner-github-actions.html) — March 19, 2026 incident
- [CrowdStrike — Trivy compromise analysis](https://www.crowdstrike.com/en-us/blog/from-scanner-to-stealer-inside-the-trivy-action-supply-chain-compromise/) — technical details

---
*Research completed: 2026-03-24*
*Ready for roadmap: yes*
