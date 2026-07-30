# Quick Task 260730-ftv: Issue #49 - Research

**Researched:** 2026-07-30
**Domain:** Go module-version derivation in GitHub Actions and SIMD depth-limit documentation
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### CI version source
- Derive the bootstrap command version from the `pure-simdjson` module version
  recorded in `go.mod`, using `go list`.
- Do not add a `tools.go` file or retain a duplicate pin with a drift guard.
- Prefer the smallest change that makes `go.mod` the operational source of
  truth.

#### Single-source scope
- The "one file per dependency bump" requirement applies to operational pins
  that must remain synchronized for builds or CI.
- Intentional release-specific legal attribution and verified behavioral
  documentation may remain versioned when the version itself carries meaning.
- Any documentation or other reference that contradicts the selected
  `go.mod` version or the actual dependency behavior must be fixed.

#### Existing documentation work
- Treat the general stale-reference cleanup already merged in change #50 as
  complete.
- Revisit only the remaining version-specific nesting-limit statement in
  `docs/simd-deployment.md`.
- Verify that statement against the current dependency before deciding whether
  it should remain release-specific or be corrected.

### the agent's Discretion
- Exact shell variable naming and quoting in the CI workflow.
- Verification coverage needed to prove that the bootstrap version is derived
  correctly without adding redundant production code.
- Final wording of the nesting-limit statement, subject to the decisions above.

### Deferred Ideas (OUT OF SCOPE)

None recorded.
</user_constraints>

## Summary

Use a two-line workflow block: derive the active module version with
`go list -m`, then quote the complete `module/cmd@version` argument passed to
`go run`. The current graph yields `v0.1.7`; a local command smoke reported
`module: v0.1.7` and left `go.mod`/`go.sum` unchanged. [VERIFIED: `go.mod`;
local `go list -m` and derived-command smoke]

The documented 1,023/1,024 boundary is correct for this adapter's default
parser, but not as a universal v0.1.7 package limit: v0.1.7 exposes
`WithMaxDepth`, while `NewSIMDParser` passes no options. Scope the sentence to
the adapter's default and remove the version literal. [VERIFIED:
`parser_simd.go`; checksum-backed v0.1.7 source/tests; focused tagged test]

**Primary recommendation:** Change only `.github/workflows/ci.yml` and the
nesting-limit wording in `docs/simd-deployment.md`; add no production code,
dependency, drift guard, or new test file.

## Project Constraints (from AGENTS.md)

- Keep artifacts free of internal repository/company information; eventual
  integration uses squash merge. [VERIFIED: `AGENTS.md`]
- Prefer the smallest solution and plain language. [VERIFIED: `AGENTS.md`]
- No Go or Makefile change is needed. If scope expands into Go, all documented
  constructor, validation, defaults, error, build, and test conventions apply.
  [VERIFIED: `AGENTS.md`; repository inspection]

## Recommended CI Pattern

Replace the single hard-coded `run:` line in the existing “Install verified
native SIMD library” step with:

```yaml
run: |
  pure_simdjson_version="$(go list -m -f '{{.Version}}' github.com/amikos-tech/pure-simdjson)"
  go run "github.com/amikos-tech/pure-simdjson/cmd/pure-simdjson-bootstrap@${pure_simdjson_version}" fetch
```

Why this is the safest minimal form:

- `go list -m` reports the selected active-module `Version`; here it is the
  direct `v0.1.7` requirement. [CITED:
  https://pkg.go.dev/cmd/go#hdr-List_packages_or_modules] [VERIFIED: `go.mod`;
  local command]
- GitHub runs all block lines in one shell; this Ubuntu step defaults to
  `bash -e`, so a failed assignment stops before `go run`. No explicit
  `shell:` or separate empty-value guard is needed. [CITED:
  https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#jobsjob_idstepsrun]
  [VERIFIED: local `bash -e` probe]
- The quotes protect both the Go template and the final package word.
  `go run package@version` executes the exact command version outside the root
  module context. [CITED:
  https://pkg.go.dev/cmd/go#hdr-Compile_and_run_Go_program] [VERIFIED: local
  smoke]

## Pitfalls and Boundaries

| Risk | Finding | Planning consequence |
|---|---|---|
| Cold cache | `go list -m` needed module metadata: an empty cache failed with `GOPROXY=off` and succeeded with the normal proxy without repository edits. [VERIFIED: isolated `GOMODCACHE` probes] | Do not call this offline. It adds no new practical CI dependency because versioned `go run` also needs the command module unless cached. |
| Graph scope | The repository has no `go.work`, `vendor`, or `replace`; `go list` therefore returns the intended direct version. [VERIFIED: repository scan; `go env GOWORK`] | Keep the simple command. |
| Future replacement | `go run package@version` ignores root-module replacements. [CITED: https://go.dev/ref/mod#go-mod-file-replace] [CITED: https://pkg.go.dev/cmd/go#hdr-Compile_and_run_Go_program] | Revisit this coupling if a workspace, local fork, or `replace` is introduced. |
| YAML/shell | `{{.Version}}` is not a GitHub `${{ ... }}` expression, and assignment failure propagates under `bash -e`. [VERIFIED: official syntax; local probe] | Keep the exact block and quoting shown above. |

## Nesting-Limit Review

The upstream default is `1024`; its test accepts depth 1,023 and rejects 1,024
with `ErrDepthLimitExceeded`. The repository's matching adapter test passed.
Go 1.26's `encoding/json` scanner allows 10,000 containers, so the stdlib
comparison remains accurate. [VERIFIED: checksum-backed v0.1.7
`parser_options.go`/`parser_options_test.go`; focused tagged test; Go 1.26.5
`src/encoding/json/scanner.go:146-185`]

Recommended replacement for the first two sentences:

> `NewSIMDParser` currently uses `pure-simdjson`'s default maximum depth. That
> configuration accepts at most 1,023 nested array/object containers and
> returns `ErrDepthLimitExceeded` at depth 1,024. The stdlib decoder has a
> 10,000-container syntax limit, so otherwise well-formed documents between
> those boundaries can be indexed with the default parser but rejected by the
> SIMD parser.

This preserves the warning, correctly scopes it, and removes a version literal
that adds no meaning to the adapter contract. [VERIFIED:
`260730-ftv-CONTEXT.md`; repository/upstream boundary tests]

Do not broaden this task into exposing `WithMaxDepth`, changing
`simdNestingDepthLimit`, or revisiting the documentation cleanup already
completed in change #50. [VERIFIED: `260730-ftv-CONTEXT.md`]

## Scope and Minimal Verification

Touch only `.github/workflows/ci.yml:108-112` and
`docs/simd-deployment.md:185-196`. Leave `go.mod`, Go code, tests, and the
remainder of change #50's documentation cleanup alone. [VERIFIED:
`260730-ftv-CONTEXT.md`; repository inspection]

Smallest adequate verification after implementation:

```bash
actionlint .github/workflows/ci.yml

pure_simdjson_version="$(go list -m -f '{{.Version}}' github.com/amikos-tech/pure-simdjson)"
go run "github.com/amikos-tech/pure-simdjson/cmd/pure-simdjson-bootstrap@${pure_simdjson_version}" version

go test -tags simdjson -run '^TestSIMDParserNestingDepthBoundaryContract$' -count=1 .
git diff --check
```

These three checks passed during research (current workflow for `actionlint`);
the actual `fetch` remains covered by the existing SIMD CI job. No new test or
drift guard is warranted. [VERIFIED: local commands;
`.github/workflows/ci.yml:108-119`; `260730-ftv-CONTEXT.md`]

**Assumptions:** None; all implementation-relevant claims were verified.

## Sources

- [Go command: list modules](https://pkg.go.dev/cmd/go#hdr-List_packages_or_modules)
- [Go command: run a versioned package](https://pkg.go.dev/cmd/go#hdr-Compile_and_run_Go_program)
- [GitHub Actions workflow `run` and shell syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#jobsjob_idstepsrun)
- [`pure-simdjson` v0.1.7 parser options](https://github.com/amikos-tech/pure-simdjson/blob/v0.1.7/parser_options.go)
- [`pure-simdjson` v0.1.7 depth boundary test](https://github.com/amikos-tech/pure-simdjson/blob/v0.1.7/parser_options_test.go#L178-L216)
- Repository evidence: `go.mod`, `.github/workflows/ci.yml`,
  `docs/simd-deployment.md`, `parser_simd.go`, and
  `parser_simd_integration_test.go`.
