---
phase: quick-260730-ije
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - parser_simd.go
  - CHANGELOG.md
  - Makefile
  - docs/simd-deployment.md
autonomous: true
requirements:
  - IJE-01
  - IJE-02
  - IJE-03
  - IJE-04
  - IJE-05
must_haves:
  truths:
    - "The SIMD adapter source comment and Unreleased changelog describe pure-simdjson without a stale duplicated release version"
    - "The NOTICE guard derives pure-simdjson's selected version from the Go module graph and fails when any intentional NOTICE pin differs"
    - "The normal lint target runs the NOTICE alignment guard, making version drift a required quality-gate failure"
    - "The deployment guide distinguishes upstream WithMaxDepth support from this adapter's optionless NewSIMDParser constructor"
    - "The adapter API, default 1,023/1,024 nesting boundary, dependency selection, and runtime behavior remain unchanged"
  artifacts:
    - path: "parser_simd.go"
      provides: "Version-independent explanation of the default SIMD nesting boundary"
      contains: "simdNestingDepthLimit"
    - path: "CHANGELOG.md"
      provides: "Accurate version-independent Unreleased adapter announcement"
      contains: "pure-simdjson"
    - path: "Makefile"
      provides: "Go-module-derived NOTICE pin alignment guard integrated with lint"
      contains: "check-notice-version"
    - path: "docs/simd-deployment.md"
      provides: "Accurate max-depth configurability guidance for upstream and adapter callers"
      contains: "WithMaxDepth"
  key_links:
    - from: "go.mod"
      to: "Makefile"
      via: "go list resolves the selected pure-simdjson module version"
      pattern: "go list -m -f '\\{\\{\\.Version\\}\\}' github\\.com/amikos-tech/pure-simdjson"
    - from: "Makefile"
      to: "NOTICE.md"
      via: "check-notice-version inspects every token on the four intentional version-bearing lines"
      pattern: "check-notice-version"
    - from: "Makefile lint"
      to: "check-notice-version"
      via: "lint prerequisite"
      pattern: "^lint:.*check-notice-version"
    - from: "docs/simd-deployment.md"
      to: "parser_simd.go"
      via: "documentation reflects the optionless adapter constructor and default upstream construction"
      pattern: "func NewSIMDParser\\(\\)"
---

<objective>
Remove the remaining stale `pure-simdjson` release attributions, enforce
alignment between intentional NOTICE pins and the selected Go module version,
and state the exact max-depth configuration boundary exposed by the adapter.

Purpose: Keep `go.mod` authoritative for dependency selection while preserving
accurate legal links and avoiding claims that the adapter exposes upstream
configuration options it does not accept.
Output: Version-independent source/changelog wording, a lint-integrated NOTICE
guard, and precise SIMD deployment guidance; no API, dependency, NOTICE, test,
or runtime-behavior changes.
</objective>

<execution_context>
@/Users/tazarov/.codex/get-shit-done/workflows/execute-plan.md
@/Users/tazarov/.codex/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/quick/260730-ftv-issue-49/260730-ftv-SUMMARY.md
@parser_simd.go
@CHANGELOG.md
@NOTICE.md
@Makefile
@docs/simd-deployment.md
@go.mod

The direct quick-task requirements are aliased as IJE-01 (source comment),
IJE-02 (Unreleased changelog), IJE-03 (NOTICE alignment guard), IJE-04
(max-depth configurability), and IJE-05 (tight behavioral scope).

<interfaces>
From parser_simd.go:

    const simdNestingDepthLimit = 1024
    func NewSIMDParser() (CloseableParser, error)

`NewSIMDParser` currently calls `purejson.NewParser()` without upstream parser
options. Preserve both signatures and that construction behavior.

From go.mod:

    github.com/amikos-tech/pure-simdjson v0.1.7

The implementation must derive this selected version with `go list`; the
literal above is context, not a new Makefile or prose pin.
</interfaces>
</context>

<source_audit>

| Source | ID | Feature or constraint | Task | Status | Notes |
|--------|----|-----------------------|------|--------|-------|
| GOAL | — | Eliminate stale adapter version claims, guard intentional NOTICE pins, and clarify max-depth configurability | 1, 2 | COVERED | Quick-task description |
| REQ | IJE-01 | Remove the stale `pure-simdjson v0.1.4` attribution from `parser_simd.go` without replacing it with another duplicated version | 1 | COVERED | |
| REQ | IJE-02 | Correct the Unreleased changelog adapter claim using non-stale wording | 1 | COVERED | |
| REQ | IJE-03 | Add a loud Makefile assertion that all four intentional NOTICE pin lines align with the module-graph version | 2 | COVERED | Includes every version token in tree/blob permalinks |
| REQ | IJE-04 | Explain that upstream offers `WithMaxDepth` but optionless `NewSIMDParser()` does not expose it | 1 | COVERED | |
| REQ | IJE-05 | Keep behavior, API, dependencies, NOTICE content, and unrelated files unchanged | 1, 2 | COVERED | Enforced by whole-diff gate |
| RESEARCH | — | No research artifact | — | N/A | Quick-task instruction explicitly says no research phase |
| CONTEXT | — | No CONTEXT.md or D-XX decisions supplied | — | N/A | Direct DATA requirements are mapped above as IJE-01 through IJE-05 |

No source item is deferred or unplanned.
</source_audit>

<execution_baseline>
Before Task 1 makes any implementation edit, require a clean non-planning
worktree with
`test -z "$(git status --porcelain=v1 --untracked-files=all -- . ':(exclude).planning/**')"`; if it is not clean, stop and preserve those changes.
Then run
`! git show-ref --verify --quiet refs/gsd/baselines/quick-260730-ije &amp;&amp; git update-ref refs/gsd/baselines/quick-260730-ije HEAD`
to require a fresh ref and record the starting `HEAD`. Retain the local ref
until final whole-diff verification succeeds so the scope check includes task
commits plus staged and unstaged changes.
</execution_baseline>

<tasks>

<task type="auto">
  <name>Task 1: Remove stale attribution and document the adapter depth boundary</name>
  <files>parser_simd.go, CHANGELOG.md, docs/simd-deployment.md</files>
  <action>
    Implement IJE-01 by changing only the comment above
    `simdNestingDepthLimit`: attribute the 1,024 first-rejected depth to the
    default `pure-simdjson` parser configuration, with no release number.
    Preserve the constant name/value and all executable Go code.

    Implement IJE-02 by removing `v0.1.4` from the first Unreleased changelog
    bullet and describing it as the opt-in same-package `pure-simdjson` parser
    adapter. Do not substitute `v0.1.7` or another release literal; retain the
    rest of the announcement's API, ownership, fallback, and numeric-policy
    details.

    Implement IJE-04 in the `## Nesting limit` section. Keep the tested default
    1,023-accepted/1,024-rejected boundary, stdlib comparison, failure policy,
    and remediation. Add an explicit statement that upstream
    `pure-simdjson` exposes `WithMaxDepth` at parser construction, while this
    adapter's `NewSIMDParser()` accepts no options and constructs the upstream
    parser with its defaults, so callers cannot configure maximum depth
    through this adapter. Do not add a new constructor, expose parser options,
    change the default, or edit any other documentation section. These
    prohibitions enforce IJE-05.
  </action>
  <verify>
    <automated>git show-ref --verify --quiet refs/gsd/baselines/quick-260730-ije &amp;&amp; ! rg -nF 'pure-simdjson v0.1.4' parser_simd.go CHANGELOG.md &amp;&amp; ! rg -n 'pure-simdjson[[:space:]]+v[0-9]+\.[0-9]+\.[0-9]+' parser_simd.go CHANGELOG.md &amp;&amp; printf '%s\n' 'pure-simdjson v0.1.7' | rg -q 'pure-simdjson[[:space:]]+v[0-9]+\.[0-9]+\.[0-9]+' &amp;&amp; rg -nF 'default pure-simdjson parser configuration' parser_simd.go &amp;&amp; rg -nF 'func NewSIMDParser() (CloseableParser, error)' parser_simd.go &amp;&amp; nesting_text="$(awk '/^## Nesting limit$/{capture=1; next} /^## Numeric limits$/{capture=0} capture' docs/simd-deployment.md | tr '\n' ' ')" &amp;&amp; printf '%s\n' "${nesting_text}" | rg -Fq 'WithMaxDepth' &amp;&amp; printf '%s\n' "${nesting_text}" | rg -Fq '`NewSIMDParser()` accepts no options' &amp;&amp; printf '%s\n' "${nesting_text}" | rg -Fq 'cannot configure maximum depth through this adapter' &amp;&amp; printf '%s\n' "${nesting_text}" | rg -q '1,023.*ErrDepthLimitExceeded.*1,024.*10,000-container' &amp;&amp; test -z "$(gofmt -d parser_simd.go)" &amp;&amp; go test -tags simdjson -run '^TestSIMDParserNestingDepthBoundaryContract$' -count=1 . &amp;&amp; git diff --check -- parser_simd.go CHANGELOG.md docs/simd-deployment.md</automated>
  </verify>
  <done>
    Neither source nor changelog contains the stale attribution or a
    replacement adapter version pin; the source constant and constructor are
    unchanged; the nesting guide states both upstream configurability and the
    adapter limitation; and the existing tagged boundary contract test passes.
  </done>
</task>

<task type="auto">
  <name>Task 2: Enforce NOTICE version alignment through lint</name>
  <files>Makefile</files>
  <action>
    Implement IJE-03 by adding a phony `check-notice-version` target following
    the existing `set -eu`, shell-variable, and loud stderr diagnostic style.
    Resolve `expected_version` with
    `go list -m -f '{{.Version}}' github.com/amikos-tech/pure-simdjson`; do not
    parse `go.mod` directly and do not add a second literal version.

    Use `grep` to collect exactly the four intentional version-bearing
    `pure-simdjson` lines in `NOTICE.md`: the dependency/tree link line, the
    section heading, the pinned LICENSE permalink, and the pinned NOTICE
    permalink. Fail if those lines are absent or their count changes, avoiding
    a vacuous pass. Extract every Go semantic-version token from the four
    lines—including both visible text and URL occurrences on the dependency
    line—and fail if any token is not exactly `expected_version`. The drift
    diagnostic must name `NOTICE.md`, print the expected module version, and
    show the offending pinned lines or versions.

    Add `check-notice-version` as a prerequisite of the required `lint` target
    alongside `check-validator-markers`. List it in `help` with a description
    that names NOTICE alignment with `go.mod`. Keep `NOTICE.md`, `go.mod`,
    `go.sum`, dependency selection, and all other targets unchanged per IJE-05.
  </action>
  <verify>
    <automated>set -eu
git show-ref --verify --quiet refs/gsd/baselines/quick-260730-ije
make check-notice-version
rg -nF "go list -m -f '{{.Version}}' github.com/amikos-tech/pure-simdjson" Makefile
! rg -ni 'pure[-_]simdjson[^[:cntrl:]]*v[0-9]+\.[0-9]+\.[0-9]+' Makefile
rg -n '^lint:.*check-validator-markers.*check-notice-version|^lint:.*check-notice-version.*check-validator-markers' Makefile
help_output="$(make help)"
printf '%s\n' "${help_output}" | rg -q '^[[:space:]]*check-notice-version[[:space:]]+-[[:space:]].*NOTICE.*go\.mod$'

tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT
cp Makefile go.mod go.sum NOTICE.md "${tmp_dir}/"
selected_version="$(go list -m -f '{{.Version}}' github.com/amikos-tech/pure-simdjson)"
test "$(grep -oF "${selected_version}" NOTICE.md | wc -l | tr -d '[:space:]')" -eq 5
required_lines="$(grep -nE 'pure-simdjson.*v[0-9]' NOTICE.md | cut -d: -f1)"
test "$(printf '%s\n' "${required_lines}" | sed '/^$/d' | wc -l | tr -d '[:space:]')" -eq 4
go -C "${tmp_dir}" list -m -f '{{.Version}}' github.com/amikos-tech/pure-simdjson &gt; /dev/null

for token_index in 1 2 3 4 5; do
  awk -v expected="${selected_version}" -v target="${token_index}" '
  BEGIN { seen = 0 }
  {
    rest = $0
    rendered = ""
    while ((position = index(rest, expected)) &gt; 0) {
      seen++
      rendered = rendered substr(rest, 1, position - 1)
      if (seen == target) {
        rendered = rendered "v0.0.0"
      } else {
        rendered = rendered expected
      }
      rest = substr(rest, position + length(expected))
    }
    print rendered rest
  }
  END { if (seen != 5 || target &gt; seen) exit 2 }
  ' NOTICE.md &gt; "${tmp_dir}/NOTICE.md"
  if make --no-print-directory -s -C "${tmp_dir}" check-notice-version &gt; "${tmp_dir}/drift.out" 2&gt;&amp;1; then
    printf 'NOTICE guard accepted corrupted version token %s\n' "${token_index}"
    exit 1
  fi
  rg -Fq 'NOTICE.md' "${tmp_dir}/drift.out"
  rg -Fq "${selected_version}" "${tmp_dir}/drift.out"
done

for line_number in ${required_lines}; do
  awk -v drop="${line_number}" 'NR != drop' NOTICE.md &gt; "${tmp_dir}/NOTICE.md"
  if make --no-print-directory -s -C "${tmp_dir}" check-notice-version &gt; "${tmp_dir}/missing-line.out" 2&gt;&amp;1; then
    printf 'NOTICE guard accepted deletion of required line %s\n' "${line_number}"
    exit 1
  fi
  rg -Fq 'NOTICE.md' "${tmp_dir}/missing-line.out"
  rg -Fq "${selected_version}" "${tmp_dir}/missing-line.out"
done

git diff --check -- Makefile</automated>
  </verify>
  <done>
    The guard passes against the current four NOTICE pin lines; independently
    rejects corruption of each of their five version tokens and deletion of
    each required line; uses the exact module-graph command without a literal
    dependency pin; names NOTICE and the selected version in failures; runs
    through `make lint`; appears in `make help`; and changes only Makefile.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| `go.mod` module graph → Make recipe | Repository-controlled dependency selection becomes the expected legal-notice version |
| `NOTICE.md` → lint result | Human-maintained legal text is classified and compared by a required automated gate |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-quick260730ije-01 | Tampering | NOTICE pin classification | mitigate | Require exactly the four intentional version-bearing lines and compare every extracted version token, preventing missing-line and mixed visible-link/permalink passes |
| T-quick260730ije-02 | Tampering | Module-version handoff in Makefile | mitigate | Resolve the fixed module path with `go list`, quote shell expansions, use values only for comparison/diagnostics, and fail closed when resolution fails |
| T-quick260730ije-03 | Repudiation | Optional standalone consistency target | mitigate | Wire the guard into the required `lint` target and prove both aligned and drifted cases during execution |
</threat_model>

<verification>
  <automated>make test &amp;&amp; make lint &amp;&amp; baseline_ref=refs/gsd/baselines/quick-260730-ije &amp;&amp; git show-ref --verify --quiet "${baseline_ref}" &amp;&amp; actual_changed="$({ git diff --name-only "${baseline_ref}" -- . ':(exclude).planning/**'; git ls-files --others --exclude-standard -- . ':(exclude).planning/**'; } | LC_ALL=C sort -u)" &amp;&amp; expected_changed="$(printf '%s\n' 'CHANGELOG.md' 'Makefile' 'docs/simd-deployment.md' 'parser_simd.go' | LC_ALL=C sort)" &amp;&amp; test "${actual_changed}" = "${expected_changed}" &amp;&amp; git diff --check "${baseline_ref}" -- . ':(exclude).planning/**' &amp;&amp; git update-ref -d "${baseline_ref}"</automated>

Run both task-level checks, then run the full project `make test` and
`make lint` gates followed by the whole-diff command above. Before the final
command, review
`git diff refs/gsd/baselines/quick-260730-ije -- . ':(exclude).planning/**'`.
The scope and whitespace gates compare only non-planning tracked changes since
the pre-edit baseline plus non-planning untracked, non-ignored paths against
exactly the four intended implementation files. They delete the temporary
baseline ref only after every check passes.
</verification>

<success_criteria>
- `parser_simd.go` and the Unreleased changelog no longer attribute the adapter
  to `pure-simdjson v0.1.4` or duplicate the selected replacement version.
- `make check-notice-version` derives the selected version from the Go module
  graph, validates all tokens on the four intentional NOTICE pin lines, and
  fails loudly on mixed or stale versions.
- `make lint` executes the NOTICE alignment guard and remains green with the
  current aligned `NOTICE.md`.
- The deployment guide states that upstream supports `WithMaxDepth` but this
  project's optionless `NewSIMDParser()` does not expose maximum-depth
  configuration.
- The focused tagged boundary test, full project tests, lint, whitespace, and
  exact-file scope checks pass without changes to API behavior, dependencies,
  NOTICE content, or tests.
</success_criteria>

<output>
Create
`.planning/quick/260730-ije-address-stale-pure-simdjson-version-refe/260730-ije-SUMMARY.md`
and update `.planning/STATE.md` after implementation and verification.
</output>
