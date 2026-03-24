# Architecture Patterns: OSS CI/CD Infrastructure

**Domain:** Go library CI/CD and release infrastructure
**Researched:** 2026-03-24

## Recommended Architecture

The OSS infrastructure consists of three GitHub Actions workflows, supporting configuration files, and GitHub platform features. No application architecture changes are needed -- this is purely the infrastructure layer.

### Component Boundaries

| Component | Responsibility | Triggered By |
|-----------|---------------|--------------|
| CI workflow (`ci.yml`) | Test, lint, vet, coverage, security scan | Push to main, PRs |
| Release workflow (`release.yml`) | Changelog generation, GitHub Release creation | Tag push (`v*`) |
| Security workflow (`security.yml`) | Scheduled vulnerability scanning | Cron (weekly) + push to main |
| Dependabot config | Automated dependency update PRs | GitHub platform (weekly) |
| golangci-lint config | Lint rules, formatter settings | Used by CI workflow |
| GoReleaser config | Release artifact definitions | Used by release workflow |

### Workflow Architecture

```
PR opened/updated
    |
    v
[ci.yml] ----- matrix: [go-1.25, go-1.26] x [ubuntu-latest]
    |
    +-- Step: checkout
    +-- Step: setup-go (with GOTOOLCHAIN: local)
    +-- Step: go build ./...
    +-- Step: go test -coverprofile=coverage.out ./...
    +-- Step: golangci-lint run
    +-- Step: govulncheck ./...
    +-- Step: upload coverage to Codecov (main branch only)

Tag v* pushed
    |
    v
[release.yml]
    |
    +-- Step: checkout (with fetch-depth: 0 for changelog)
    +-- Step: setup-go
    +-- Step: goreleaser release (skip=build, changelog from commits)

Weekly cron
    |
    v
[security.yml]
    |
    +-- Step: checkout
    +-- Step: setup-go
    +-- Step: govulncheck ./... (SARIF output)
    +-- Step: upload SARIF to GitHub Code Scanning
```

## Workflow Patterns

### Pattern 1: CI Workflow with Matrix Strategy

**What:** Single workflow that runs tests, linting, and security across multiple Go versions.
**When:** Every push to main and every PR.
**Why:** Ensures the library works on both supported Go releases. Catches Go version-specific issues early.

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

env:
  GOTOOLCHAIN: local

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go-version: ['1.25.x', '1.26.x']
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v6
        with:
          go-version: ${{ matrix.go-version }}
      - run: go build ./...
      - run: go test -v -race -coverprofile=coverage.out ./...
      - name: Upload coverage
        if: matrix.go-version == '1.26.x' && github.event_name == 'push'
        uses: codecov/codecov-action@v5
        with:
          files: ./coverage.out

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v6
        with:
          go-version: '1.26.x'
      - uses: golangci/golangci-lint-action@v9
        with:
          version: v2.11

  security:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v6
        with:
          go-version: '1.26.x'
      - uses: golang/govulncheck-action@v1
        with:
          go-version-input: '1.26.x'
          go-package: './...'
```

### Pattern 2: Tag-Triggered Release

**What:** GoReleaser creates a GitHub Release with changelog when a semver tag is pushed.
**When:** `git tag v0.1.0 && git push --tags`
**Why:** Automated, consistent releases with categorized changelogs. No manual release notes.

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v6
        with:
          go-version: '1.26.x'
      - uses: goreleaser/goreleaser-action@v7
        with:
          version: '~> v2'
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### Pattern 3: Scheduled Security Scan

**What:** Weekly govulncheck with SARIF upload to GitHub Code Scanning.
**When:** Weekly cron + push to main (catches new vulns between dependency updates).
**Why:** Vulnerabilities in dependencies can appear at any time, not just when code changes.

```yaml
name: Security

on:
  schedule:
    - cron: '0 6 * * 1'  # Monday 6am UTC
  push:
    branches: [main]

permissions:
  contents: read
  security-events: write

jobs:
  govulncheck:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: golang/govulncheck-action@v1
        with:
          go-version-input: '1.26.x'
          go-package: './...'
          output-format: sarif
          output-file: govulncheck.sarif
      - uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: govulncheck.sarif
```

## Anti-Patterns to Avoid

### Anti-Pattern 1: Single Monolithic CI Job
**What:** Putting test, lint, security in one sequential job.
**Why bad:** Failure in linting blocks test results. Slow feedback. Can't parallelize.
**Instead:** Separate jobs for test (matrix), lint, and security. They run in parallel.

### Anti-Pattern 2: `@latest` for CI Tool Versions
**What:** Using `golangci-lint@latest` or `goreleaser@latest` in CI.
**Why bad:** CI breaks unpredictably when a new version is released. Non-reproducible builds.
**Instead:** Pin major versions (`v2.11` for golangci-lint, `~> v2` for goreleaser). Let Dependabot update the CI action versions.

### Anti-Pattern 3: Coverage Gate Without Baseline
**What:** Setting a hard coverage threshold (e.g., 80%) in CI before measuring the current baseline.
**Why bad:** May block all PRs if current coverage is below the threshold.
**Instead:** Measure current coverage first. Set threshold at current - 2%. Use Codecov's "patch coverage" to ensure new code is tested.

### Anti-Pattern 4: Testing Without `-race`
**What:** Running `go test ./...` without the `-race` flag.
**Why bad:** Misses data races that only manifest under concurrent execution. The library has bitmap operations and builder code that could have race conditions.
**Instead:** Always use `go test -race ./...` in CI. Note: this slightly increases test time.

### Anti-Pattern 5: `fetch-depth: 1` for Release Workflows
**What:** Using shallow clone in the release workflow.
**Why bad:** GoReleaser needs full git history to generate changelogs between tags.
**Instead:** Use `fetch-depth: 0` in the release workflow. Shallow clone (`fetch-depth: 1`) is fine for CI.

## File Layout

```
.github/
  workflows/
    ci.yml                    # Test + lint + security (PR/push)
    release.yml               # Tag-triggered GoReleaser
    security.yml              # Scheduled govulncheck
    claude.yml                # Existing: Claude bot
    claude-code-review.yml    # Existing: Claude code review
  dependabot.yml              # Dependency update config
.golangci.yml                 # Lint config (v2 format, upgraded)
.goreleaser.yml               # Release config (library-only)
LICENSE                       # Apache-2.0
CONTRIBUTING.md               # Build/test/lint instructions
```

## Scalability Considerations

| Concern | Now (launch) | At 10 contributors | At 100+ stars |
|---------|-------------|-------------------|--------------|
| CI time | ~2-3 min (matrix of 2) | Same | Same |
| Coverage | Codecov free tier | Codecov free tier | Codecov free tier (OSS) |
| Releases | Manual tag push | Manual tag push | Consider release-please for auto-tagging |
| Branch protection | Optional | Enable: require CI pass | Enable: require review + CI pass |
| Dependabot noise | Low (weekly) | Low (weekly) | Consider grouping updates |

## Sources

- [GitHub Actions: Building and testing Go](https://docs.github.com/en/actions/use-cases-and-examples/building-and-testing/building-and-testing-go)
- [golangci-lint CI integration](https://golangci-lint.run/docs/welcome/integrations/)
- [GoReleaser GitHub Actions docs](https://goreleaser.com/ci/actions/)
- [govulncheck-action docs](https://github.com/golang/govulncheck-action)
- [Go version matrix pitfall](https://brandur.org/fragments/go-version-matrix)
