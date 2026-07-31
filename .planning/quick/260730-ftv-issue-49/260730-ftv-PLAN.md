---
phase: quick-260730-ftv
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - .github/workflows/ci.yml
  - docs/simd-deployment.md
autonomous: true
requirements:
  - ISSUE-49
must_haves:
  truths:
    - "The SIMD CI bootstrap command always uses the pure-simdjson version selected by go.mod, so a dependency bump requires no separate workflow pin edit"
    - "The workflow keeps the Go template and computed module/cmd@version argument safely quoted while preserving the existing verified native-library fetch"
    - "The deployment guide describes the 1,023/1,024 boundary as NewSIMDParser's current default configuration, not as a universal release-specific package limit"
  artifacts:
    - path: ".github/workflows/ci.yml"
      provides: "SIMD bootstrap version derivation from the active Go module graph"
      contains: "go list -m -f '{{.Version}}' github.com/amikos-tech/pure-simdjson"
    - path: "docs/simd-deployment.md"
      provides: "Accurately scoped default SIMD nesting-limit guidance"
      contains: "NewSIMDParser"
  key_links:
    - from: "go.mod"
      to: ".github/workflows/ci.yml"
      via: "go list reads the selected pure-simdjson module version"
      pattern: "go list -m -f '\\{\\{\\.Version\\}\\}' github\\.com/amikos-tech/pure-simdjson"
    - from: ".github/workflows/ci.yml"
      to: "github.com/amikos-tech/pure-simdjson/cmd/pure-simdjson-bootstrap"
      via: "the derived version variable is interpolated inside one quoted package@version argument"
      pattern: "pure-simdjson-bootstrap@\\$\\{pure_simdjson_version\\}"
    - from: "docs/simd-deployment.md"
      to: "parser_simd_integration_test.go"
      via: "documentation preserves the tagged test's 1,023-accepted/1,024-rejected default boundary"
      pattern: "ErrDepthLimitExceeded"
---

<objective>
Resolve issue #49 with two narrowly scoped edits: remove the operational
`pure-simdjson` bootstrap version pin from CI by deriving it from `go.mod`, and
correct the remaining nesting-limit wording so it describes the adapter's
default configuration accurately.

Purpose: Make `go.mod` the single operational version source while preserving
useful, behavior-backed deployment guidance.
Output: Updated `.github/workflows/ci.yml` and
`docs/simd-deployment.md`; no production code, dependency-file, or test-file
changes.
</objective>

<execution_context>
@/Users/tazarov/.codex/get-shit-done/workflows/execute-plan.md
@/Users/tazarov/.codex/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/quick/260730-ftv-issue-49/260730-ftv-CONTEXT.md
@.planning/quick/260730-ftv-issue-49/260730-ftv-RESEARCH.md
@go.mod
@.github/workflows/ci.yml
@docs/simd-deployment.md
@parser_simd.go
@parser_simd_integration_test.go

For decision traceability, the three locked decision groups in CONTEXT.md are
aliased here as D-01 (CI version source), D-02 (single-source scope), and D-03
(existing documentation work).
</context>

<source_audit>

| Source | ID | Feature or constraint | Task | Status |
|--------|----|-----------------------|------|--------|
| GOAL | — | Eliminate operational version drift and correct the remaining nesting-limit wording | 1, 2 | COVERED |
| REQ | ISSUE-49 | Make `go.mod` the operational source of truth for the SIMD bootstrap | 1 | COVERED |
| RESEARCH | — | Use the verified two-line `go list` plus quoted `go run` pattern | 1 | COVERED |
| RESEARCH | — | Scope the 1,023/1,024 boundary to `NewSIMDParser`'s default and remove the release literal | 2 | COVERED |
| RESEARCH | — | Add no production code, dependency, drift guard, or new test file | 1, 2 | COVERED |
| CONTEXT | D-01 | Derive the bootstrap version from `go.mod` with `go list`; no `tools.go` or duplicate pin | 1 | COVERED |
| CONTEXT | D-02 | Remove operational duplicate pins while retaining meaningful verified behavioral guidance | 1, 2 | COVERED |
| CONTEXT | D-03 | Treat the prior general cleanup as complete and revisit only the deployment nesting statement | 2 | COVERED |

This quick task has no ROADMAP `phase_req_ids`; `ISSUE-49` is its direct
requirement. CONTEXT.md records no deferred ideas.
</source_audit>

<execution_baseline>
Before Task 1 makes any source edit, require a clean non-planning worktree with
`test -z "$(git status --porcelain=v1 --untracked-files=all -- . ':(exclude).planning/**')"`; if it is not clean, stop and preserve the existing
changes rather than folding them into this task. Then run
`! git show-ref --verify --quiet refs/gsd/baselines/quick-260730-ftv &amp;&amp; git update-ref refs/gsd/baselines/quick-260730-ftv HEAD`
to require a fresh ref and record the starting `HEAD`. Keep that local baseline
ref until the final whole-diff verification succeeds. This makes the final
scope check cover task commits plus staged and unstaged changes instead of
observing only the last task's working tree.
</execution_baseline>

<tasks>

<task type="auto">
  <name>Task 1: Derive the SIMD bootstrap version from go.mod</name>
  <files>.github/workflows/ci.yml</files>
  <action>
    In the `simd` job's existing `Install verified native SIMD library` step,
    replace the hard-coded one-line `run` command with the researched two-line
    YAML block. Assign `pure_simdjson_version` from
    `go list -m -f '{{.Version}}' github.com/amikos-tech/pure-simdjson`, then
    invoke `go run` with the entire
    `github.com/amikos-tech/pure-simdjson/cmd/pure-simdjson-bootstrap@${pure_simdjson_version}`
    argument double-quoted and pass `fetch` unchanged. Preserve the step name,
    environment, job order, and verified artifact-fetch behavior.

    This implements D-01 and the operational portion of D-02. Do not edit
    `go.mod` or `go.sum`, add `tools.go`, add an empty-value guard, introduce a
    drift-check script/test, or retain any literal bootstrap version in this
    workflow; the existing Bash `-e` behavior already stops when `go list`
    fails.
  </action>
  <verify>
    <automated>git show-ref --verify --quiet refs/gsd/baselines/quick-260730-ftv &amp;&amp; actionlint .github/workflows/ci.yml &amp;&amp; rg -nF "pure_simdjson_version=\"\$(go list -m -f '{{.Version}}' github.com/amikos-tech/pure-simdjson)\"" .github/workflows/ci.yml &amp;&amp; rg -nF 'go run "github.com/amikos-tech/pure-simdjson/cmd/pure-simdjson-bootstrap@${pure_simdjson_version}" fetch' .github/workflows/ci.yml &amp;&amp; test "$(rg -v '^[[:space:]]*#' .github/workflows/ci.yml | rg -c 'pure-simdjson-bootstrap@')" -eq 1 &amp;&amp; pure_simdjson_version="$(go list -m -f '{{.Version}}' github.com/amikos-tech/pure-simdjson)" &amp;&amp; bootstrap_version_output="$(go run "github.com/amikos-tech/pure-simdjson/cmd/pure-simdjson-bootstrap@${pure_simdjson_version}" version)" &amp;&amp; rg -Fxq "module: ${pure_simdjson_version}" &lt;&lt;&lt;"${bootstrap_version_output}"</automated>
  </verify>
  <done>
    The workflow is valid and contains exactly one bootstrap invocation: the
    required quoted fetch command fed by the required `go list` assignment.
    The derived command's `version` output exactly matches the selected
    `pure-simdjson` module version, and neither `go.mod` nor `go.sum` changes.
  </done>
</task>

<task type="auto">
  <name>Task 2: Scope the documented nesting boundary to the adapter default</name>
  <files>docs/simd-deployment.md</files>
  <action>
    Replace only the first paragraph under `## Nesting limit`. Per D-02 and
    D-03, state that `NewSIMDParser` currently uses `pure-simdjson`'s default
    maximum depth; that this configuration accepts at most 1,023 nested
    array/object containers and returns `ErrDepthLimitExceeded` at depth 1,024;
    and that the stdlib decoder's 10,000-container syntax limit means documents
    between those boundaries can be accepted by the default parser but rejected
    by the SIMD parser. Keep the explicit parser-parity warning, but remove the
    `v0.1.7` literal because the boundary describes the adapter's default rather
    than an immutable package-wide release limit.

    Leave the following failure-mode/remediation paragraphs, the rest of
    `docs/simd-deployment.md`, `parser_simd.go`, and
    `parser_simd_integration_test.go` unchanged. Do not repeat the general
    version-reference cleanup already completed before this task and do not
    expose `WithMaxDepth` or change the adapter limit.
  </action>
  <verify>
    <automated>nesting_text="$(awk '/^## Nesting limit$/{capture=1} /^## Numeric limits$/{capture=0} capture' docs/simd-deployment.md | tr '\n' ' ')" &amp;&amp; rg -q 'NewSIMDParser.*default maximum depth.*1,023.*ErrDepthLimitExceeded.*1,024.*10,000-container' &lt;&lt;&lt;"${nesting_text}" &amp;&amp; ! rg -q 'pure-simdjson` v[0-9]' &lt;&lt;&lt;"${nesting_text}" &amp;&amp; go test -tags simdjson -run '^TestSIMDParserNestingDepthBoundaryContract$' -count=1 . &amp;&amp; git diff --check -- .github/workflows/ci.yml docs/simd-deployment.md</automated>
  </verify>
  <done>
    The nesting section accurately scopes the tested 1,023/1,024 boundary to
    `NewSIMDParser`'s current default, retains the 10,000-container stdlib
    comparison and operator warning, contains no release literal, and the
    existing boundary contract test passes.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| `go.mod` → CI shell | Repository-controlled dependency metadata becomes an executable bootstrap package version |
| CI runner → module/artifact network | The existing versioned Go command and verified bootstrap fetch retrieve external inputs |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-quick260730ftv-01 | Tampering | `go.mod`-to-bootstrap version handoff | mitigate | Resolve the selected module version with `go list`, interpolate only that value into the fixed bootstrap package path, quote the complete argument, and smoke-test the derived command |
| T-quick260730ftv-02 | Tampering | CI workflow syntax or shell parsing | mitigate | Preserve the existing step environment, use the researched Bash-compatible quoting, and require `actionlint` before completion |
| T-quick260730ftv-03 | Tampering | External module/native artifact retrieval | accept | This task introduces no package or version; it continues using the existing direct Go requirement and the existing bootstrap's verified artifact-fetch path |
</threat_model>

<verification>
  <automated>baseline_ref=refs/gsd/baselines/quick-260730-ftv &amp;&amp; git show-ref --verify --quiet "${baseline_ref}" &amp;&amp; actual_changed="$({ git diff --name-only "${baseline_ref}" --; git ls-files --others --exclude-standard; } | rg -v '^\.planning/' | LC_ALL=C sort -u)" &amp;&amp; expected_changed="$(printf '%s\n' '.github/workflows/ci.yml' 'docs/simd-deployment.md' | LC_ALL=C sort)" &amp;&amp; test "${actual_changed}" = "${expected_changed}" &amp;&amp; git diff --check "${baseline_ref}" -- &amp;&amp; git update-ref -d "${baseline_ref}"</automated>

Run both task-level automated checks, then run the whole-diff command above.
It compares every tracked change since the pre-edit baseline plus every
untracked, non-ignored path against exactly the two intended implementation
files; `.planning/` is excluded because the plan and execution artifacts are
not implementation scope. Before running it, review
`git diff refs/gsd/baselines/quick-260730-ftv --`; then the automated command
deletes the temporary baseline ref only after the exact-file comparison and
whitespace check pass. Confirm the workflow retains its existing environment
and the documentation edit is confined to the first nesting-limit paragraph.
</verification>

<success_criteria>
- A future `pure-simdjson` version change in `go.mod` automatically changes the
  bootstrap command version used by the SIMD CI job.
- The workflow passes `actionlint`, and a locally derived versioned bootstrap
  command executes successfully.
- The deployment guide attributes the tested 1,023/1,024 boundary to
  `NewSIMDParser`'s current default configuration without a release literal.
- The existing tagged nesting-boundary test passes.
- Only `.github/workflows/ci.yml` and `docs/simd-deployment.md` are modified;
  no Go code, dependency files, tests, guards, or unrelated documentation are
  added or changed.
</success_criteria>

<output>
Create
`.planning/quick/260730-ftv-issue-49/260730-ftv-SUMMARY.md`
and update `.planning/STATE.md` after implementation and verification.
</output>
