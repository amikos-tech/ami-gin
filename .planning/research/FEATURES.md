# Feature Landscape: OSS Infrastructure

**Domain:** Go library open-source readiness tooling
**Researched:** 2026-03-24

## Table Stakes

Features users expect from a credible open-source Go library. Missing = project feels amateur.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| CI badge in README | Shows tests pass. First thing visitors check. | Low | GitHub Actions workflow badge |
| Passing CI on main | Broken main = dead project perception | Low | Enforce via branch protection |
| LICENSE file | Legal requirement for adoption. No license = cannot use. | Low | Apache-2.0, single file |
| Go module path matches repo | `go get` must work. Current path mismatch blocks this. | Med | Module rename ripples through all imports |
| GoDoc comments on exports | pkg.go.dev renders from source. Missing docs = unusable API. | Med | 10 examples already exist, which is excellent |
| Coverage report | Shows testing discipline. Expected >70% for credible library. | Low | Already has 1.35:1 test-to-source ratio |
| CONTRIBUTING.md | Shows how to build, test, lint. Reduces contributor friction. | Low | Build/test commands already documented in CLAUDE.md |
| Semantic versioning | Go modules require semver tags. Users need version stability. | Low | Start at v0.1.0 (pre-1.0 signals API may change) |
| Automated dependency updates | Shows active maintenance even during quiet periods | Low | Dependabot config file |
| Security scanning in CI | Shows security awareness. Expected for any data-handling library. | Low | govulncheck + gosec via golangci-lint |

## Differentiators

Features that elevate the project above the baseline. Not expected, but signal quality.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Automated changelog on releases | Categorized release notes from conventional commits | Low | GoReleaser handles this |
| Coverage badge | Visual proof of test coverage in README | Low | Codecov integration |
| Go Report Card badge | Third-party code quality validation | Low | Free service, zero config |
| Branch protection rules | Prevents broken merges to main | Low | GitHub settings, no code |
| Matrix testing (Go 1.25 + 1.26) | Proves compatibility across Go versions | Low | CI matrix strategy |
| Runnable examples on pkg.go.dev | Interactive documentation. Already have 10 examples. | Done | Verify they render correctly |
| SECURITY.md | Tells users how to report vulnerabilities responsibly | Low | Standard file, no code |

## Anti-Features

Features to explicitly NOT build for OSS launch.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Custom documentation site | pkg.go.dev handles this automatically for Go libraries. Maintenance burden with no benefit. | Ensure GoDoc comments are thorough. |
| Docker image | This is a library, not a service. No one runs it in a container. | N/A |
| Pre-commit hooks | Adds friction for new contributors who just want to submit a PR. | CI catches lint/test failures. |
| Makefile-driven releases | Error-prone manual process. | GoReleaser automates everything on tag push. |
| npm/PyPI/other package manager distribution | Go modules (`go get`) is the only distribution channel needed. | Document `go get` in README. |
| Complex branch strategy | main + feature branches is sufficient. release branches add overhead. | Tag from main for releases. |
| CLA (Contributor License Agreement) | Overkill for a small OSS project. Creates adoption friction. | Apache-2.0 license includes patent grant. |

## Feature Dependencies

```
LICENSE file -> module path rename (both needed before going public)
CI workflow -> golangci-lint v2 config upgrade (lint step needs proper config)
GoReleaser config -> conventional commits (already in use per CLAUDE.md)
Coverage badge -> CI workflow (coverage.out must be generated in CI)
Branch protection -> CI workflow (require checks to pass before merge)
Dependabot config -> CI workflow (Dependabot PRs need passing CI to be useful)
pkg.go.dev indexing -> repo goes public + valid module path
```

## MVP Recommendation

**Must have for OSS launch (in order):**
1. LICENSE file (Apache-2.0) -- legal blocker
2. Module path rename to match repo -- `go get` blocker
3. CI workflow (test + lint + security scan) -- credibility baseline
4. golangci-lint v2 config upgrade -- CI depends on this
5. CONTRIBUTING.md -- contributor onboarding
6. README badges (CI, GoDoc, license) -- visual credibility
7. Dependabot config -- automated maintenance signal

**Defer after launch:**
- GoReleaser setup (can tag v0.1.0 manually first, add automation for v0.2.0)
- Codecov integration (nice to have, not blocking)
- SECURITY.md (add before v1.0.0)
- Branch protection rules (add once CI is stable)

## Sources

- Project analysis based on PROJECT.md, CONCERNS.md, existing codebase
- [pkg.go.dev about page](https://pkg.go.dev/about)
- [Go Report Card](https://goreportcard.com/)
