---
phase: quick-260730-kny
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - Makefile
  - .github/workflows/ci.yml
  - notice_version_guard_test.go
autonomous: true
requirements:
  - KNY-01
  - KNY-02
  - KNY-03
  - KNY-04
  - KNY-05
  - KNY-06
must_haves:
  truths:
    - "Every CI run explicitly executes the NOTICE version guard before the Go linter."
    - "Every complete semantic-version token on a pure-simdjson NOTICE reference, including an added stale reference outside the four canonical lines, must equal the selected effective module version."
    - "The guard does not turn failed reads or searches into empty counts, uses a replace directive's replacement version when present, and shows escaped context for invisible NOTICE mismatches."
    - "Focused automated regressions prove aligned, stale-anywhere, replacement-version, invisible-character, and CI-wiring cases."
  artifacts:
    - path: "Makefile"
      provides: "Fail-closed effective-version NOTICE guard"
      contains: "check-notice-version"
    - path: ".github/workflows/ci.yml"
      provides: "Explicit CI NOTICE version-gate step"
      contains: "make check-notice-version"
    - path: "notice_version_guard_test.go"
      provides: "Isolated executable regression tests for the Make guard"
      contains: "TestNoticeVersionGuard"
  key_links:
    - from: "go.mod replace directive"
      to: "Makefile check-notice-version"
      via: "go list -m template chooses .Replace.Version when .Replace is populated"
      pattern: "\.Replace"
    - from: "NOTICE.md"
      to: "Makefile check-notice-version"
      via: "full-file scan of pure-simdjson version-bearing references"
      pattern: "pure-simdjson"
    - from: ".github/workflows/ci.yml"
      to: "Makefile check-notice-version"
      via: "dedicated lint-job run step"
      pattern: "make check-notice-version"
    - from: "notice_version_guard_test.go"
      to: "Makefile check-notice-version"
      via: "temporary repository copies invoke the real make target"
      pattern: "check-notice-version"
---

<objective>
Close the review findings in the pure-simdjson NOTICE version guard: execute it
directly in CI, make it fail closed for stale version references anywhere in
NOTICE.md, and cover edge cases with a durable isolated regression suite.

Purpose: Legal-notice version drift must be impossible to merge unnoticed,
even when a Go module replacement selects the effective version or an
invisible character hides a malformed NOTICE reference.
Output: A hardened Make target, an explicit CI gate, and focused Go regression
coverage with no dependency, NOTICE-content, or parser behavior change.
</objective>

<execution_context>
@/Users/tazarov/.codex/get-shit-done/workflows/execute-plan.md
@/Users/tazarov/.codex/get-shit-done/templates/summary.md
</execution_context>

<context>
@AGENTS.md
@.planning/STATE.md
@.planning/quick/260730-ije-address-stale-pure-simdjson-version-refe/260730-ije-SUMMARY.md
@Makefile
@NOTICE.md
@.github/workflows/ci.yml
@go.mod
@validator_marker_test.go

The preceding quick task made `go.mod` authoritative and created
`check-notice-version`, but its guard currently reaches CI only indirectly via
the local `lint` target. CI invokes `make check-validator-markers` and then
`golangci-lint` directly, so it never executes `check-notice-version`.

Use the project test style shown in `validator_marker_test.go`: package-level
Go tests with `t.TempDir`, standard-library file I/O, precise `t.Fatalf`
diagnostics, and no dependency additions. The new test may invoke `make` only
inside a temporary copy containing the minimal target inputs; it must never
edit the tracked repository, run `go mod tidy`, or fetch/install anything.

<interfaces>
Existing Make target contract:

    make check-notice-version

It exits zero only when the effective pure-simdjson module version and all
required NOTICE pins agree. Preserve this target name and keep it usable as a
standalone check.

Existing CI lint-job contract:

    - name: Check validator markers
      run: make check-validator-markers

Insert the NOTICE step in the same `lint` job before `golangci-lint`; do not
replace either existing check with a broad `make lint` invocation because that
would alter the CI linter command and its established configuration.

Go module template data:

    Module.Version
    Module.Replace.Version

When `Module.Replace` is non-nil, its version is the effective selected
dependency version for the guard. A local replacement has no semantic version;
the guard must reject that empty/non-semver value with a specific diagnostic
rather than silently falling back to the original requirement version.
</interfaces>
</context>

<source_audit>

| Source | ID | Feature or constraint | Task | Status | Notes |
|--------|----|-----------------------|------|--------|-------|
| GOAL | — | Address NOTICE version-guard review findings atomically | 1, 2, 3 | COVERED | Direct quick-task description |
| REQ | KNY-01 | CI must explicitly execute the guard | 3 | COVERED | Dedicated workflow step, not an implicit local prerequisite |
| REQ | KNY-02 | Any stale pure-simdjson version anywhere in NOTICE must fail | 2 | COVERED | Full relevant-reference scan plus stale-extra-line regression |
| REQ | KNY-03 | Remove error-swallowing count/search logic | 2 | COVERED | No `|| true` around guard reads/searches/counts; command failures propagate |
| REQ | KNY-04 | Use the effective replacement version when `replace` applies | 2 | COVERED | Select `Module.Replace.Version`; local replacement fails clearly |
| REQ | KNY-05 | Diagnostics reveal invisible NOTICE mismatches | 2 | COVERED | Emit line-numbered `LC_ALL=C sed -n l` context on every NOTICE mismatch |
| REQ | KNY-06 | Add focused regression coverage where repository patterns support it | 1, 3 | COVERED | Root Go test invokes real target in isolated copies and asserts CI wiring |
| RESEARCH | — | No research artifact | — | N/A | Quick-task instruction explicitly prohibits a research phase |
| CONTEXT | — | No CONTEXT.md or D-XX decisions supplied | — | N/A | Direct review findings are mapped to KNY requirements above |

No source item is deferred or unplanned.
</source_audit>

<execution_baseline>
Before implementation, preserve the user's existing `.planning/config.json`
modification and any other unrelated work. Confirm that the only planned
non-planning files changed by this task are `Makefile`,
`.github/workflows/ci.yml`, and `notice_version_guard_test.go`; do not reset,
stage, or modify unrelated paths.
</execution_baseline>

<tasks>

<task type="auto">
  <name>Task 1: Define isolated regressions for the NOTICE guard and CI contract</name>
  <files>notice_version_guard_test.go</files>
  <action>
    Create `notice_version_guard_test.go` in package `gin`, following the
    standard-library and `t.TempDir` conventions in `validator_marker_test.go`.
    Add `TestNoticeVersionGuard` with named subtests and helpers that copy only
    `Makefile`, `NOTICE.md`, `go.mod`, and `go.sum` into a temporary directory,
    then execute `make --no-print-directory -C <temp> check-notice-version`
    with `exec.Command`. Capture combined output and include it in failures;
    never mutate tracked files or rely on the current process working directory
    after setup.

    Write the following executable behavioral contract before hardening the
    recipe. The aligned copied inputs pass. Appending a separate line such as
    `Supplemental pure-simdjson v0.0.0 reference.` causes the target to fail
    and names both `NOTICE.md` and the expected effective version. A copied
    `go.mod` whose pure-simdjson requirement is replaced by the same module at
    a different complete semantic version, with all canonical NOTICE pins
    changed to that replacement version, passes; this proves the guard selects
    `Replace.Version`, not the original requirement. A copied NOTICE with a
    zero-width Unicode character inserted into one otherwise-valid version
    fails and its diagnostic includes escaped bytes produced by `LC_ALL=C sed
    -n l` (for example `\\342\\200\\213`), so the hidden mismatch is visible.

    Add a focused CI-contract subtest that reads `.github/workflows/ci.yml` and
    verifies the `lint` job has a dedicated `run: make check-notice-version`
    step after checkout/setup and before the `golangci-lint` action. It must
    reject merely finding the command in unrelated YAML text or relying on the
    Makefile's `lint` prerequisite. Do not test external GitHub Actions
    behavior or add a YAML dependency; assert the stable local step ordering
    with exact line/section parsing appropriate to the current workflow.
  </action>
  <verify>
    <automated>go test -run '^TestNoticeVersionGuard$' -count=1 .; test -f notice_version_guard_test.go; git diff --check -- notice_version_guard_test.go</automated>
  </verify>
  <done>
    A root-level test file specifies and executes aligned, stale-anywhere,
    effective-replacement, invisible-character, and CI-step ordering behavior
    entirely through isolated copies and local file inspection. It initially
    exposes the current guard/CI findings until the remaining tasks are
    implemented.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Make the NOTICE version guard fail closed on effective full-file references</name>
  <files>Makefile</files>
  <behavior>
    - An aligned current NOTICE passes `make check-notice-version`.
    - A stale complete version on any line that references pure-simdjson fails, even when it is outside the four canonical pin lines.
    - A versioned module replacement supplies the comparison version; an unversioned local replacement fails with an actionable non-semver diagnostic.
    - Missing, unreadable, malformed, or invisibly corrupted NOTICE input fails without a swallowed command error and prints escaped, line-numbered context.
  </behavior>
  <action>
    Refactor only the `check-notice-version` recipe to make Task 1 pass while
    retaining the existing target name, its standalone use, the four exact
    canonical line-shape requirements, and the normal `lint` prerequisite.

    Derive `expected_version` from one `go list -m` template that chooses
    `.Replace.Version` when `.Replace` exists and `.Version` otherwise. Do not
    parse `go.mod`, hard-code a release, or fall back to the original version
    when a replacement is local/unversioned. Validate the resulting value as a
    complete Go semantic-version token and fail with a diagnostic that says
    whether it came from an unusable replacement value.

    Retain exact-one checks for the dependency/tree line, heading, LICENSE
    permalink, and NOTICE permalink, then separately scan the entire
    `NOTICE.md` for every line that both names `pure-simdjson` and contains a
    complete version token. Extract and compare every such token to
    `expected_version`, so a stale supplementary attribution cannot evade the
    four canonical shape checks. Do not constrain the full-file scan to a
    presumed count; canonical-count checks and all-reference comparisons have
    distinct purposes.

    Remove every `|| true` and other error-swallowing search/count pattern from
    this target. Use an `awk`-based or equivalently fail-closed extraction that
    preserves command errors under `set -eu`; distinguish no relevant lines,
    malformed canonical lines, and version drift in explicit diagnostics.
    Whenever NOTICE validation fails, print line-numbered matching/relevant
    context through `LC_ALL=C sed -n 'l'` (or an equivalent byte-escaping
    command) in addition to the expected version. This must make whitespace,
    zero-width Unicode, CRLF, and other non-printing mismatches inspectable.
    Keep `NOTICE.md`, `go.mod`, `go.sum`, dependency selection, and unrelated
    Make targets unchanged.
  </action>
  <verify>
    <automated>go test -run '^TestNoticeVersionGuard$' -count=1 . &amp;&amp; make check-notice-version &amp;&amp; ! rg -n '\|\|[[:space:]]*true' Makefile &amp;&amp; ! rg -n 'pure-simdjson[^[:cntrl:]]*v[0-9]+\.[0-9]+\.[0-9]+' Makefile &amp;&amp; rg -nF '{{with .Replace}}{{.Version}}{{else}}{{.Version}}{{end}}' Makefile &amp;&amp; rg -n "LC_ALL=C.*sed -n.*l" Makefile &amp;&amp; git diff --check -- Makefile</automated>
  </verify>
  <done>
    The real Make target uses the effective module version, rejects stale
    pure-simdjson version references anywhere in NOTICE, propagates underlying
    command failures, preserves canonical pin checks, and emits diagnostics
    that expose invisible corrupted characters. All guard regression subtests
    pass.
  </done>
</task>

<task type="auto">
  <name>Task 3: Run the NOTICE guard explicitly in CI</name>
  <files>.github/workflows/ci.yml</files>
  <action>
    In the existing `lint` job, add a clearly named `Check NOTICE version
    alignment` step with exactly `run: make check-notice-version`. Place it
    after the current validator-marker check and before the
    `golangci/golangci-lint-action` step, so the guard runs as an independent
    required CI gate even though CI intentionally invokes golangci-lint
    directly. Preserve all runner, Go setup, action versions, permissions,
    matrix, and non-lint jobs unchanged.

    Re-run the Task 1 CI-contract regression after this edit. Do not replace
    the two discrete lint checks with `make lint`: the latter would introduce
    the known repository-wide lint baseline into a different CI command path
    and would not meet the review requirement for an explicit guard step.
  </action>
  <verify>
    <automated>go test -run '^TestNoticeVersionGuard$' -count=1 . &amp;&amp; actionlint .github/workflows/ci.yml &amp;&amp; git diff --check -- .github/workflows/ci.yml</automated>
  </verify>
  <done>
    The CI lint job visibly and independently invokes `make
    check-notice-version` before golangci-lint, the focused contract test
    proves the exact placement, and the workflow remains syntactically valid.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| `go.mod` module graph and optional `replace` → Make recipe | Repository dependency selection becomes the legal-notice version authority. |
| `NOTICE.md` human-maintained text → Make recipe | Untrusted/manual legal text is parsed and compared before it can be merged. |
| GitHub Actions workflow → Make target | CI configuration must actually invoke the guard instead of merely documenting it locally. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-quick260730kny-01 | Tampering | `NOTICE.md` version references | mitigate | Scan every pure-simdjson version-bearing reference, compare complete tokens against the effective module version, and regression-test an added stale line. |
| T-quick260730kny-02 | Tampering | `go.mod` replacement resolution | mitigate | Select `.Replace.Version` when present; reject a blank or malformed local replacement rather than accepting the original version. |
| T-quick260730kny-03 | Repudiation | Guard diagnostics | mitigate | Emit expected version plus line-numbered, byte-escaped NOTICE context on all validation failures. |
| T-quick260730kny-04 | Elevation of privilege | CI quality gate bypass | mitigate | Add a dedicated lint-job command and assert its placement in an executable regression test. |
| T-quick260730kny-SC | Tampering | No package installation | accept | This task adds no dependencies or package-manager operations; the existing Go toolchain and Make command are used only against repository-controlled temporary copies. |
</threat_model>

<verification>
  <automated>go test -run '^TestNoticeVersionGuard$' -count=1 . &amp;&amp; make check-notice-version &amp;&amp; make check-validator-markers &amp;&amp; actionlint .github/workflows/ci.yml &amp;&amp; git diff --check -- Makefile .github/workflows/ci.yml notice_version_guard_test.go &amp;&amp; git diff --name-only -- Makefile .github/workflows/ci.yml notice_version_guard_test.go | sort -u | diff -u &lt;(printf '%s\n' '.github/workflows/ci.yml' 'Makefile' 'notice_version_guard_test.go' | sort) -</automated>

Run the focused suite first because it exercises the real Make target in
temporary copies. Then run the standalone guard, marker guard, workflow
syntax validation, whitespace check, and exact intended-file scope check. If
`actionlint` is unavailable locally, install neither a new dependency nor a
binary; use the repository's configured CI/actionlint validation mechanism and
record the unavailable command rather than hiding it.
</verification>

<success_criteria>
- CI's lint job contains and executes a separate `make check-notice-version`
  step before golangci-lint.
- The guard derives the actual effective pure-simdjson version from the Go
  module graph, including versioned replacements, and rejects an unversioned
  local replacement explicitly.
- Every version-bearing pure-simdjson reference anywhere in `NOTICE.md` must
  use that exact complete semantic version; missing, malformed, stale, and
  invisible-character variants fail closed with inspectable diagnostics.
- No failed recipe search/read is converted to an empty successful count.
- `notice_version_guard_test.go` executes the aligned, stale-anywhere,
  replacement, invisible-character, and CI-ordering regressions without
  changing tracked NOTICE or module files.
</success_criteria>

<output>
Create
`.planning/quick/260730-kny-address-notice-version-guard-review-find/260730-kny-SUMMARY.md`
after implementation and verification.
</output>
