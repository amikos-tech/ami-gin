# Domain Pitfalls: OSS Go Library Infrastructure

**Domain:** Go library open-source CI/CD and tooling infrastructure
**Researched:** 2026-03-24

## Critical Pitfalls

Mistakes that cause CI failures, broken releases, or security incidents.

### Pitfall 1: Go Toolchain Auto-Upgrade in CI Matrix

**What goes wrong:** CI matrix specifies Go 1.25 and 1.26, but Go 1.25 automatically upgrades itself to 1.26 because `go.mod` contains a `toolchain` directive for a newer version. All matrix entries effectively test the same Go version.
**Why it happens:** Go 1.21+ introduced automatic toolchain management. When `go.mod` says `toolchain go1.25.5`, older Go versions fetch and install 1.25.5 (or later), silently overriding the matrix version.
**Consequences:** False confidence that the library works on Go 1.25 when it's actually only tested on 1.26. Users on Go 1.25 may hit build failures that CI never caught.
**Prevention:** Set `GOTOOLCHAIN: local` as an environment variable in all CI jobs. This tells Go to use the locally installed version and never auto-upgrade.
**Detection:** Check CI logs for "downloading go1.X.X" messages during the setup-go step.
**Source:** [brandur.org -- Go version CI matrix](https://brandur.org/fragments/go-version-matrix)

### Pitfall 2: Module Path Mismatch After Rename

**What goes wrong:** Changing `go.mod` module path from `github.com/amikos-tech/gin-index` to `github.com/amikos-tech/ami-gin` without updating all internal imports. Code compiles locally (cached) but fails in CI (clean environment).
**Why it happens:** Go's module system requires all import paths to exactly match the module path in `go.mod`. A rename requires updating every `import` statement in every `.go` file, every `_test.go` file, and the `.golangci.yml` gci prefix config.
**Consequences:** Build failure in CI. If partially renamed and tagged, consumers get confusing import errors.
**Prevention:**
1. Use `find . -name '*.go' | xargs sed -i 's|github.com/amikos-tech/gin-index|github.com/amikos-tech/ami-gin|g'`
2. Update `.golangci.yml` gci prefix section
3. Run `go build ./...` and `go test ./...` to verify
4. Do the rename in a single commit, before any other changes
**Detection:** `go build ./...` will fail immediately if any import is mismatched.

### Pitfall 3: Trivy/Third-Party Action Supply Chain Risk

**What goes wrong:** A GitHub Actions dependency is compromised, leaking secrets or injecting malicious code into the CI pipeline.
**Why it happens:** Third-party actions are just Git repos. Tag references can be reassigned. In March 2026, `aquasecurity/trivy-action` was compromised for ~12 hours with a credential stealer injected into all tags (0.0.1 through 0.34.2).
**Consequences:** CI secrets (GITHUB_TOKEN, codecov tokens) leaked. Potential supply chain injection into releases.
**Prevention:**
1. Prefer official/first-party actions (actions/*, golang/*, golangci/*).
2. For critical actions, pin to commit SHA instead of tag (e.g., `actions/checkout@abcdef123` instead of `@v6`). Dependabot can still update SHA pins.
3. Minimize permissions in workflow files (`permissions: contents: read`).
4. For this project: use govulncheck (Go team official) instead of Trivy.
**Detection:** GitHub's secret scanning may alert on leaked tokens. Monitor GitHub Security Advisories for action compromises.
**Source:** [TheHackerNews -- Trivy Action compromise](https://thehackernews.com/2026/03/trivy-security-scanner-github-actions.html)

### Pitfall 4: Publishing Tag Before CI Passes

**What goes wrong:** A maintainer pushes a version tag to a commit where CI has not passed. The Go module proxy caches the tag immediately. A broken version is permanently published.
**Why it happens:** Go module proxy (proxy.golang.org) fetches and caches module versions when anyone runs `go get`. Once cached, a version cannot be unpublished or replaced.
**Consequences:** Users get a broken version. The only fix is to publish a new patch version (e.g., v0.1.1 to fix v0.1.0).
**Prevention:**
1. Never push tags directly. Create a tag only after CI passes on main.
2. Consider a release workflow that verifies CI status before creating the GitHub Release.
3. Use `v0.x.x` versioning initially -- pre-1.0 versions carry an implicit "may break" signal.
**Detection:** `go get github.com/amikos-tech/ami-gin@v0.1.0` will fail with build errors if the tagged version is broken.

## Moderate Pitfalls

### Pitfall 5: golangci-lint v2 Config Migration Errors

**What goes wrong:** The existing `.golangci.yml` uses v2 format but was not fully migrated. Adding new linters or settings using v1 syntax causes golangci-lint to silently ignore them or error.
**Prevention:** Run `golangci-lint migrate` on the existing config to verify it's fully v2 compliant. Key changes in v2: `gosimple` and `stylecheck` merged into `staticcheck`; `gci`, `gofmt`, `goimports` moved to `formatters` section; `enable-all`/`disable-all` replaced by `linters.default`.
**Source:** [golangci-lint migration guide](https://golangci-lint.run/docs/product/migration-guide/)

### Pitfall 6: Coverage Threshold Blocking All PRs

**What goes wrong:** Setting a hard coverage threshold (e.g., 80%) in Codecov config before measuring the actual baseline. All PRs are blocked by failing coverage checks.
**Prevention:** Measure current coverage first. Set Codecov's `project` threshold at current_coverage - 2%. Use `patch` coverage to ensure new code is tested without penalizing existing uncovered code.

### Pitfall 7: GoReleaser Changelog Missing Full History

**What goes wrong:** The release workflow uses `actions/checkout` with the default `fetch-depth: 1` (shallow clone). GoReleaser generates an empty or incomplete changelog because git history is missing.
**Prevention:** Set `fetch-depth: 0` in the checkout step of the release workflow. This fetches full history. Only needed in the release workflow -- shallow clone is fine for CI.

### Pitfall 8: `go.mod` Patch Version Confusing Contributors

**What goes wrong:** The `go.mod` file says `go 1.25.5` (with patch version). Contributors with Go 1.25.4 or 1.25.3 may get confusing errors or automatic toolchain downloads.
**Prevention:** Use `go 1.25` (major.minor only) in the `go` directive. The patch version in `go` directive is non-standard and adds no value for a library.

### Pitfall 9: Dependabot PR Noise

**What goes wrong:** Dependabot creates individual PRs for every transitive dependency update (especially AWS SDK, which has many subpackages). This creates PR noise and review fatigue.
**Prevention:** Group related dependencies in `dependabot.yml`. For this project, group all `github.com/aws/*` packages into a single PR. Keep `github-actions` ecosystem updates separate for visibility.

## Minor Pitfalls

### Pitfall 10: README Badges Pointing to Wrong Module Path

**What goes wrong:** Badges in README reference `github.com/amikos-tech/gin-index` instead of `github.com/amikos-tech/ami-gin`. pkg.go.dev badge shows 404.
**Prevention:** Add badges only after the module path rename is complete. Template the correct module path.

### Pitfall 11: `gotestsum@latest` Non-Reproducible Installs

**What goes wrong:** Makefile target `gotestsum-bin` uses `go install gotest.tools/gotestsum@latest`. Different developers/CI runs may get different versions, causing inconsistent JUnit output format.
**Prevention:** Pin to a specific version: `go install gotest.tools/gotestsum@v1.12.0` (or whichever is current). Or use the `go run` pattern to avoid polluting GOPATH: `go run gotest.tools/gotestsum@v1.12.0`.

### Pitfall 12: Missing `-race` Flag in CI Tests

**What goes wrong:** Tests pass in CI without race detection. A data race in bitmap operations or builder code ships to users.
**Prevention:** Always include `-race` flag in `go test` CI steps. Note: this increases test time by ~2-10x, which is acceptable for a library with ~30s test suite.

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Module path rename | Incomplete import update | Single-commit rename, verify with `go build ./...` |
| CI workflow creation | Toolchain auto-upgrade in matrix | `GOTOOLCHAIN: local` env var |
| golangci-lint upgrade | v1/v2 config mixing | Run `golangci-lint migrate`, test locally first |
| First version tag | Broken tag cached by proxy | Tag only after CI passes, use v0.x.x |
| GoReleaser setup | Empty changelog | `fetch-depth: 0` in release workflow checkout |
| Dependabot enablement | AWS SDK PR flood | Group `github.com/aws/*` packages |
| Security scanning | Using compromised action | Use govulncheck (official), not Trivy action |
| Coverage reporting | Hard threshold blocks PRs | Measure baseline first, set threshold below it |

## Sources

- [Go version CI matrix pitfall](https://brandur.org/fragments/go-version-matrix) -- GOTOOLCHAIN: local
- [Trivy supply chain compromise](https://thehackernews.com/2026/03/trivy-security-scanner-github-actions.html) -- March 2026
- [golangci-lint v2 migration guide](https://golangci-lint.run/docs/product/migration-guide/)
- [GoReleaser library cookbook](https://goreleaser.com/cookbooks/release-a-library/)
- [Go module proxy behavior](https://go.dev/ref/mod#module-proxy)
