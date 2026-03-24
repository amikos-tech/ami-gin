# Technology Stack: OSS Infrastructure Layer

**Project:** GIN Index -- Open Source Readiness
**Domain:** Go library OSS tooling and CI/CD infrastructure
**Researched:** 2026-03-24
**Overall Confidence:** HIGH

## Recommended Stack

### CI/CD: GitHub Actions

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `actions/checkout` | v6 | Repository checkout | Current stable (v6.0.2, Jan 2026). Uses node24 runtime. |
| `actions/setup-go` | v6 | Go toolchain installation | Current stable. Built-in module + build cache. Reads `go.mod` toolchain directive. |
| `golangci/golangci-lint-action` | v9 | Lint integration | v9.0.0 supports golangci-lint v2, node24 runtime, install-only mode. |
| `golang/govulncheck-action` | v1 | Vulnerability scanning | Official Go team action. SARIF output for Code Scanning integration. |
| `goreleaser/goreleaser-action` | v7 | Tag-triggered releases | v7.0.0 current. Handles changelog generation for library-only releases. |
| `codecov/codecov-action` | v5 | Coverage reporting | Current stable. Uploads `coverage.out` from `go test -coverprofile`. |

**Go version matrix strategy:**

Test on the two most recent Go stable releases (Go 1.25.x and Go 1.26.x). Go 1.26.1 is the latest stable release (March 6, 2026). The project currently targets Go 1.25.5 in go.mod.

**Critical:** Set `GOTOOLCHAIN: local` as an environment variable in all CI jobs. Without this, Go's automatic toolchain management (introduced in Go 1.21) will silently upgrade the Go version to match the `toolchain` directive in `go.mod`, defeating the purpose of matrix testing.

**Confidence:** HIGH -- all versions verified via web search against GitHub releases and official docs.

### Linting: golangci-lint v2

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `golangci-lint` | v2.11.4 | Linting and static analysis | Latest stable (March 22, 2026). v2 config format with `linters.default`. |

**Current state:** The project has a `.golangci.yml` using `version: "2"` format, but only enables 3 linters (dupword, gocritic, mirror) plus gci formatter. This is far below what a credible OSS library should have.

**Recommended linters to add (beyond the `standard` default set):**

| Linter | Category | Why Enable |
|--------|----------|------------|
| `gosec` | Security | AST/SSA-based security scanning. 50+ rules covering OWASP Top 10. Essential for a library that deserializes untrusted data. |
| `errorlint` | Bug prevention | Catches incorrect error wrapping and comparison. Critical given the codebase uses `pkg/errors`. |
| `goconst` | Code quality | Detects repeated string/numeric constants. Catches magic numbers in serialization code. |
| `unconvert` | Code quality | Removes unnecessary type conversions. |
| `unparam` | Code quality | Detects unused function parameters. |
| `bodyclose` | Bug prevention | Ensures HTTP response bodies are closed. Relevant for S3 client code. |
| `nilerr` | Bug prevention | Detects returning nil after checking for non-nil error. |
| `dupword` | Style | Already enabled. Catches duplicate words in comments. |
| `gocritic` | Style | Already enabled. Broad diagnostic/style/performance checks. |
| `mirror` | Style | Already enabled. Detects wrong mirror patterns. |
| `prealloc` | Performance | Suggests preallocating slices. Relevant for builder/serialization hot paths. |
| `copyloopvar` | Bug prevention | Detects loop variable capture issues. |

**Exclusion presets to enable:** `std-error-handling`, `common-false-positives`

**Module path note:** The gci formatter currently references `github.com/amikos-tech/gin-index`. This MUST be updated to `github.com/amikos-tech/ami-gin` when the module path changes.

**Confidence:** HIGH -- golangci-lint v2.11.4 verified, config format verified via official docs.

### Documentation: pkg.go.dev

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| pkg.go.dev | N/A (service) | API documentation | Automatic for any public Go module. Zero configuration needed. |
| Go Report Card | N/A (service) | Code quality badge | Industry-standard badge at goreportcard.com. Free, runs golangci-lint subset. |

**How it works:** Once the repo is public and the module path resolves, pkg.go.dev automatically indexes the package. No configuration or deployment needed.

**Badges for README:**

```markdown
[![Go Reference](https://pkg.go.dev/badge/github.com/amikos-tech/ami-gin.svg)](https://pkg.go.dev/github.com/amikos-tech/ami-gin)
[![Go Report Card](https://goreportcard.com/badge/github.com/amikos-tech/ami-gin)](https://goreportcard.com/report/github.com/amikos-tech/ami-gin)
[![CI](https://github.com/amikos-tech/ami-gin/actions/workflows/ci.yml/badge.svg)](https://github.com/amikos-tech/ami-gin/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/amikos-tech/ami-gin/branch/main/graph/badge.svg)](https://codecov.io/gh/amikos-tech/ami-gin)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
```

**Doc quality actions:**
- Ensure every exported type, function, and method has a GoDoc comment
- First sentence of package comment is critical -- it appears in pkg.go.dev search results
- Example functions (`func ExampleFoo()`) appear as runnable examples on pkg.go.dev -- the project already has 10 examples, which is excellent

**Confidence:** HIGH -- pkg.go.dev is the standard Go documentation platform with no alternatives to consider.

### Release Management: GoReleaser

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| GoReleaser | v2.14.3 | Tag-triggered releases | Latest stable (March 9, 2026). Library-only mode with `builds: skip: true`. |

**Library-only configuration** (`.goreleaser.yml`):

```yaml
builds:
  - skip: true

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"
      - "^ci:"
  groups:
    - title: "Breaking Changes"
      regexp: '^.*?!:.*$'
      order: 0
    - title: "Features"
      regexp: '^feat.*'
      order: 1
    - title: "Bug Fixes"
      regexp: '^fix.*'
      order: 2
    - title: "Performance"
      regexp: '^perf.*'
      order: 3
    - title: "Other"
      order: 999

release:
  github:
    owner: amikos-tech
    name: ami-gin
  draft: false
  prerelease: auto
```

**Why GoReleaser for a library (not just `git tag`):**
- Automated changelog generation from conventional commits (which this project already uses per CLAUDE.md)
- GitHub Release creation with categorized notes
- Consistent release process that contributors can trigger
- No binary builds needed -- `skip: true` handles this

**Alternative considered:** Manual `git tag` + GitHub Release UI. Rejected because it does not scale, is error-prone, and misses changelog generation.

**Confidence:** HIGH -- GoReleaser v2.14.3 verified, library cookbook verified via official docs.

### Security Scanning

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `govulncheck` | (via action v1) | Dependency vulnerability scanning | Official Go team tool. Symbol-level analysis (not just dependency matching). Low false positive rate. |
| `gosec` | (via golangci-lint) | Static security analysis | Integrated into golangci-lint. Catches common Go security issues in source code. |

**Why NOT Trivy for this project:**
1. Trivy's GitHub Action was compromised in March 2026 (credential stealer injected for ~12 hours across all tags). The `aquasecurity/setup-trivy` action was also compromised. This is a recent and severe supply chain incident.
2. Trivy is designed for container/filesystem scanning. For a Go library, `govulncheck` provides better analysis (symbol-level, not just dependency-level).
3. `govulncheck` is maintained by the Go team and uses the Go vulnerability database directly.

**Why NOT Dependabot security alerts alone:**
Dependabot only checks if a dependency has a known CVE. `govulncheck` goes further: it checks if your code actually *calls* the vulnerable function. This dramatically reduces false positives.

**Confidence:** HIGH -- govulncheck is the official Go security tool. Trivy compromise verified via multiple sources.

### Dependency Management

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| Dependabot | Built-in | Automated dependency updates | Zero config for Go modules. Native GitHub integration. Creates PRs for updates. |

**Dependabot config** (`.github/dependabot.yml`):

```yaml
version: 2
updates:
  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"
    groups:
      aws-sdk:
        patterns:
          - "github.com/aws/*"
    labels:
      - "dependencies"
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"
    labels:
      - "ci"
```

**Why Dependabot over Renovate:**
- Zero configuration for basic Go module updates
- Native GitHub integration (no external service or bot to install)
- Sufficient for a library with ~15 direct+indirect dependencies
- Renovate's advanced features (dashboard, complex grouping, monorepo support) are overkill for this project

**Confidence:** HIGH -- Dependabot Go module support is well-established and verified.

## Supporting Tools

### Makefile Enhancements

The current Makefile has: `build`, `test`, `lint`, `lint-fix`, `clean`, `help`. Missing targets per CLAUDE.md conventions: `integration-test`, `security-scan`.

**Add:**

| Target | Command | Purpose |
|--------|---------|---------|
| `security-scan` | `govulncheck ./...` | Run vulnerability check locally |
| `coverage` | `go tool cover -html=coverage.out` | Open coverage report in browser |
| `fmt` | `golangci-lint fmt` | Run formatters (v2 feature) |

### Pre-commit Hooks

Not recommended for initial OSS launch. Rationale: adds friction for new contributors. CI catches everything that matters. Contributors should be able to `git push` without local tooling setup beyond `go` and `golangci-lint`.

## Go Version Strategy

| Concern | Recommendation | Rationale |
|---------|---------------|-----------|
| go.mod `go` directive | `go 1.25` (minimum) | Allows users on 1.25.x to consume the library |
| CI matrix | `[1.25.x, 1.26.x]` | Test on current and previous stable release |
| `GOTOOLCHAIN` | `local` in CI | Prevents auto-upgrade that defeats matrix testing |
| Upgrade cadence | Update minimum version ~6 months after new release | Follow Go's support policy: only latest two releases are supported |

**Note:** The go.mod currently says `go 1.25.5`. This should be `go 1.25` (without patch version) to allow any Go 1.25.x to work. The patch version in the `go` directive is unusual and may confuse contributors.

**Confidence:** HIGH -- Go release schedule and toolchain behavior verified.

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| CI/CD | GitHub Actions | CircleCI, Travis CI | GitHub Actions is free for public repos, native integration, largest ecosystem |
| Linting | golangci-lint v2 | Individual linters (govet, staticcheck separately) | golangci-lint aggregates 100+ linters, single config, single CI step |
| Security | govulncheck + gosec | Trivy, Snyk | govulncheck is official Go team tool; Trivy had supply chain breach March 2026; Snyk requires account |
| Releases | GoReleaser | Manual git tag | GoReleaser automates changelog, release notes from conventional commits |
| Deps | Dependabot | Renovate | Dependabot is native to GitHub, zero install, sufficient for small dependency tree |
| Coverage | Codecov | Coveralls, local badge | Codecov is free for OSS, good PR annotations, widely recognized |
| Docs | pkg.go.dev | Custom docs site | pkg.go.dev is automatic for Go modules, no maintenance needed |

## What NOT to Use

| Tool | Why NOT |
|------|---------|
| **Trivy GitHub Action** | Supply chain compromise March 19, 2026. All tags (0.0.1 through 0.34.2) were hijacked with credential stealers for ~12 hours. Use govulncheck instead. |
| **pre-commit framework** | Adds contributor friction. CI catches the same issues. Can be added later if the project grows. |
| **golangci-lint v1 config** | The project already uses `version: "2"` format. Do not regress. |
| **`fmt.Errorf` with `%w`** | Project convention (per CLAUDE.md) uses `pkg/errors`. Maintain consistency even though the Go ecosystem is moving away from `pkg/errors`. |
| **Coveralls** | Codecov has better Go integration and is more widely used in the Go OSS ecosystem. |
| **Custom documentation site** | pkg.go.dev handles this automatically. A custom site adds maintenance burden with no benefit for a library. |
| **GoReleaser Pro** | The free/open-source version handles everything a library needs. Pro features (Docker, Homebrew, monorepo) are irrelevant. |
| **`go install gotest.tools/gotestsum@latest`** | The Makefile currently installs gotestsum at `@latest` which is non-reproducible. Pin to a specific version or use `go run gotest.tools/gotestsum@v1.12.0` pattern for reproducibility. |

## Installation

```bash
# Development tools (local)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v2.11.4
go install golang.org/x/vuln/cmd/govulncheck@latest

# GoReleaser (for release management, optional locally)
brew install goreleaser
# or: go install github.com/goreleaser/goreleaser/v2@v2.14.3
```

CI installs these via their respective GitHub Actions -- no manual install needed for contributors beyond `go` and `golangci-lint`.

## Sources

- [golangci-lint releases](https://github.com/golangci/golangci-lint/releases) -- v2.11.4 verified
- [golangci-lint v2 config docs](https://golangci-lint.run/docs/configuration/file/)
- [golangci-lint v2 announcement](https://ldez.github.io/blog/2025/03/23/golangci-lint-v2/)
- [golangci-lint-action](https://github.com/golangci/golangci-lint-action) -- v9 verified
- [GoReleaser releases](https://github.com/goreleaser/goreleaser/releases) -- v2.14.3 verified
- [GoReleaser library cookbook](https://goreleaser.com/cookbooks/release-a-library/)
- [GoReleaser changelog config](https://goreleaser.com/customization/changelog/)
- [goreleaser-action](https://github.com/goreleaser/goreleaser-action) -- v7 verified
- [actions/setup-go](https://github.com/actions/setup-go) -- v6 verified
- [actions/checkout](https://github.com/actions/checkout) -- v6 verified
- [govulncheck-action](https://github.com/golang/govulncheck-action) -- v1 verified
- [codecov-action](https://github.com/codecov/codecov-action) -- v5 verified
- [Go version CI matrix pitfall](https://brandur.org/fragments/go-version-matrix) -- GOTOOLCHAIN: local
- [Go 1.26 release](https://go.dev/blog/go1.26) -- Feb 10, 2026
- [Go 1.25.8 release](https://go.dev/doc/devel/release) -- March 5, 2026
- [Trivy supply chain compromise](https://thehackernews.com/2026/03/trivy-security-scanner-github-actions.html) -- March 19, 2026
- [Trivy compromise analysis (CrowdStrike)](https://www.crowdstrike.com/en-us/blog/from-scanner-to-stealer-inside-the-trivy-action-supply-chain-compromise/)
- [pkg.go.dev badge](https://pkg.go.dev/badge/)
- [Dependabot Go support](https://github.blog/changelog/2025-12-09-dependabot-dgs-for-go/)
