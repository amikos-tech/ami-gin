# Roadmap: GIN Index OSS Launch

## Overview

The GIN Index library has a production-quality codebase but lacks the infrastructure layer that signals credibility to external adopters. This roadmap takes the project from a private repo with a broken module path and no CI to a credible public open-source release with a working `go get` path, secure deserialization, green CI, contributor docs, and a versioned release.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Foundation** - Establish legal and module identity (license, module path, internal artifact removal)
- [ ] **Phase 2: Security Hardening** - Fix deserialization vulnerabilities before CI validates the codebase
- [ ] **Phase 3: CI Pipeline** - Add automated test matrix, lint, coverage, and security scanning
- [ ] **Phase 4: Contributor Experience** - Add contributor docs, README badges, and automated dependency updates
- [ ] **Phase 5: Release** - Configure release automation and publish v0.1.0

## Phase Details

### Phase 1: Foundation
**Goal**: The repository is legally usable and `go get github.com/amikos-tech/ami-gin` works
**Depends on**: Nothing (first phase)
**Requirements**: FOUND-01, FOUND-02, FOUND-03, FOUND-04
**Success Criteria** (what must be TRUE):
  1. MIT LICENSE file exists at the repository root
  2. `go get github.com/amikos-tech/ami-gin` resolves the correct module
  3. `go build ./...` and `go test ./...` pass in a clean environment with the new module path
  4. No internal references (Kiba Team, gin-index-prd.md) remain in any tracked file
  5. `.planning/` directory contains no sensitive internal references
**Plans**: TBD

### Phase 2: Security Hardening
**Goal**: The deserialization path is safe to expose to untrusted inputs
**Depends on**: Phase 1
**Requirements**: SEC-01, SEC-02, SEC-03
**Success Criteria** (what must be TRUE):
  1. All 5 unbounded allocation sites in `Decode()` have explicit bounds checks that return an error on overflow
  2. `Decode()` rejects binary data with an unknown or future version number
  3. Deserialization of a crafted payload exceeding size limits returns an error rather than exhausting memory
  4. Existing deserialization tests pass with no regressions
**Plans**: TBD

### Phase 3: CI Pipeline
**Goal**: Every pull request and push to main is automatically tested, linted, and security-scanned
**Depends on**: Phase 2
**Requirements**: CI-01, CI-02, CI-03, CI-04
**Success Criteria** (what must be TRUE):
  1. A CI badge in the README reflects live pass/fail status from GitHub Actions
  2. Pushing to main or opening a PR triggers parallel test, lint, and security jobs
  3. Tests run with `-race` across Go 1.25 and 1.26 matrix entries
  4. govulncheck runs on a weekly schedule and results appear in GitHub Code Scanning
  5. Test coverage is reported to Codecov and a coverage badge is visible in the README
**Plans**: TBD

### Phase 4: Contributor Experience
**Goal**: Any developer can understand, build, test, and contribute without reading source code
**Depends on**: Phase 3
**Requirements**: CONTR-01, CONTR-02, CONTR-03, CONTR-04
**Success Criteria** (what must be TRUE):
  1. CONTRIBUTING.md documents exact commands to build, test, lint, and run security scans locally
  2. SECURITY.md documents a vulnerability disclosure policy and contact method
  3. README displays CI status, Go Reference, and MIT license badges that link to live targets
  4. Dependabot is configured and opens its first automated dependency update PR
**Plans**: TBD

### Phase 5: Release
**Goal**: The library is publicly consumable at a stable semver version with an automated release process
**Depends on**: Phase 4
**Requirements**: REL-01, REL-02, REL-03
**Success Criteria** (what must be TRUE):
  1. `v0.1.0` tag exists and `go get github.com/amikos-tech/ami-gin@v0.1.0` installs the library
  2. A GitHub Release for `v0.1.0` exists with a generated changelog
  3. Pushing a future `vX.Y.Z` tag automatically triggers GoReleaser and creates a GitHub Release
  4. README documents known limitations (OR/AND composites, index merge, query-time transformers)
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 0/? | Not started | - |
| 2. Security Hardening | 0/? | Not started | - |
| 3. CI Pipeline | 0/? | Not started | - |
| 4. Contributor Experience | 0/? | Not started | - |
| 5. Release | 0/? | Not started | - |
