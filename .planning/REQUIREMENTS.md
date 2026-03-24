# Requirements: GIN Index OSS Launch

**Defined:** 2026-03-24
**Core Value:** A credible first impression — anyone who finds the repo can immediately understand, build, test, and contribute

## v1 Requirements

### Foundation

- [ ] **FOUND-01**: Repository has an MIT LICENSE file at the root
- [ ] **FOUND-02**: Module path changed from `github.com/amikos-tech/gin-index` to `github.com/amikos-tech/ami-gin` across go.mod, all imports, golangci-lint config, and examples
- [ ] **FOUND-03**: Internal PRD (`gin-index-prd.md`) removed from the repository
- [ ] **FOUND-04**: `.planning/` contents reviewed and any sensitive internal references removed

### CI Pipeline

- [ ] **CI-01**: GitHub Actions CI workflow runs test matrix, lint, and build verification on PR and push to main
- [ ] **CI-02**: golangci-lint v2 config upgraded with gosec, errorlint, goconst, unconvert, unparam, nilerr, and prealloc linters
- [ ] **CI-03**: govulncheck security scanning runs on a weekly schedule via GitHub Actions
- [ ] **CI-04**: Test coverage reported to Codecov with badge in README

### Security Hardening

- [ ] **SEC-01**: Deserialization bounds checks added for all 5 unbounded allocation sites
- [ ] **SEC-02**: Binary format version validation added to `Decode()` — reject unknown versions
- [ ] **SEC-03**: Maximum size limits enforced during deserialization to prevent memory exhaustion

### Contributor Experience

- [ ] **CONTR-01**: CONTRIBUTING.md exists with build, test, and lint instructions
- [ ] **CONTR-02**: SECURITY.md exists with vulnerability disclosure policy
- [ ] **CONTR-03**: README has CI status, Go Reference, and MIT license badges
- [ ] **CONTR-04**: Dependabot configuration for automated Go module dependency updates

### Release

- [ ] **REL-01**: First semantic version tag `v0.1.0` pushed after all other requirements pass
- [ ] **REL-02**: GoReleaser configured for library-only mode with automated CHANGELOG generation
- [ ] **REL-03**: Known limitations documented (OR/AND composites, index merge, query-time transformers)

## v2 Requirements

### Documentation Polish

- **DOC-01**: Package doc comment (`doc.go`) for pkg.go.dev rendering
- **DOC-02**: Key workflows converted to `Example*` test functions for pkg.go.dev
- **DOC-03**: GoDoc badge in README

### Community

- **COMM-01**: GitHub issue templates (bug report, feature request)
- **COMM-02**: Pull request template
- **COMM-03**: CODE_OF_CONDUCT.md

### CI Enhancements

- **CIENH-01**: Go version matrix testing (1.25 + latest stable)
- **CIENH-02**: Benchmark regression tracking

## Out of Scope

| Feature | Reason |
|---------|--------|
| Git history rewrite (filter-repo) | Repo is private; going public creates fresh history exposure. Deleting the PRD file is sufficient |
| Windows CI | No evidence target audience (data infrastructure) uses Windows |
| CLA or governance structure | Overkill for a single-maintainer library at this stage |
| Documentation website | pkg.go.dev is sufficient; README is comprehensive |
| v1.0.0 tag | v0.1.0 signals API instability, appropriate for first public release |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| FOUND-01 | — | Pending |
| FOUND-02 | — | Pending |
| FOUND-03 | — | Pending |
| FOUND-04 | — | Pending |
| CI-01 | — | Pending |
| CI-02 | — | Pending |
| CI-03 | — | Pending |
| CI-04 | — | Pending |
| SEC-01 | — | Pending |
| SEC-02 | — | Pending |
| SEC-03 | — | Pending |
| CONTR-01 | — | Pending |
| CONTR-02 | — | Pending |
| CONTR-03 | — | Pending |
| CONTR-04 | — | Pending |
| REL-01 | — | Pending |
| REL-02 | — | Pending |
| REL-03 | — | Pending |

**Coverage:**
- v1 requirements: 18 total
- Mapped to phases: 0
- Unmapped: 18 (awaiting roadmap)

---
*Requirements defined: 2026-03-24*
*Last updated: 2026-03-24 after initial definition*
