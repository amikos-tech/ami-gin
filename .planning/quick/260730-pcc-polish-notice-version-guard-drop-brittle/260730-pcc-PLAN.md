---
phase: quick-260730-pcc
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - notice_version_guard_test.go
  - Makefile
autonomous: true
requirements: [POLISH-01, POLISH-02, POLISH-03]

must_haves:
  truths:
    - "The `uses a bytewise locale` subtest no longer exists; rewriting `export LC_ALL=C` as an equivalent shell form does not fail the suite"
    - "A missing LC_ALL=C is still caught, by the behavioral `invisible version character is escaped` subtest"
    - "A reader of the Makefile learns that the version scan assumes pure-simdjson is the only versioned attribution in NOTICE.md"
    - "A guard failure prints only NOTICE.md lines mentioning pure-simdjson or containing a semantic-version token, with line numbers and byte escapes"
    - "All four mutation cases (ZWSP heading, historical v0.1.2 line, bare v0.0.9 line, some-other-lib v2.3.4 line) are still rejected"
    - "The failure dump for the bare `v0.0.9` mutation still shows the offending line"
    - "NOTICE.md is byte-identical to HEAD after all work"
  artifacts:
    - path: "notice_version_guard_test.go"
      provides: "Behavioral NOTICE guard test suite without the source-substring assertion"
      contains: "invisible version character is escaped"
    - path: "Makefile"
      provides: "check-notice-version target with single-attribution comment and narrowed failure dump"
      contains: "check-notice-version"
  key_links:
    - from: "Makefile fail_notice"
      to: "NOTICE.md"
      via: "filtered sed dump"
      pattern: "sed -n -E .*pure-simdjson"
    - from: "notice_version_guard_test.go"
      to: "make check-notice-version"
      via: "exec.Command under LC_ALL=en_US.UTF-8"
      pattern: "LC_ALL=en_US.UTF-8"
---

<objective>
Apply three polish findings from the PR review of the NOTICE version guard: drop a brittle
source-assertion test, document the single-attribution constraint in the Makefile, and narrow
the failure dump so it prints only relevant NOTICE.md lines.

Purpose: The guard's detection behavior is correct and must not change. These edits remove a
false-failure trap (a test that asserts Makefile source text rather than guard behavior), record
a non-obvious scan assumption for future contributors, and stop the diagnostic from dumping the
whole file.

Output: Edited `notice_version_guard_test.go` (one subtest removed) and `Makefile` (two-line
comment added, `fail_notice` dump narrowed). No change to what the guard catches.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@CLAUDE.md

@Makefile
@notice_version_guard_test.go
@NOTICE.md

<constraints>
- Makefile recipe lines are backslash-continued inside ONE shell invocation running under
  `set -eu`. Every edited line must keep its trailing ` \` continuation and leading tab exactly,
  or the recipe breaks.
- This repo is macOS/BSD tooling. `sed -i '' '0,/re/'` does NOT work for mutation testing.
  Use `perl -0pi -e` or `python3` for scratch-file mutations.
- CLAUDE.md: minimal comments, self-explanatory code. Fix 2 is ONE or TWO lines, not a block.
- All mutation testing happens on scratch copies OUTSIDE the repo. NOTICE.md must end
  byte-identical to HEAD.
</constraints>

<verified_facts>
Established during planning — do not re-derive:

1. Removing lines 59-64 of `notice_version_guard_test.go` leaves NO symbol or import unused.
   `readTestFile`, `repositoryRoot`, and the `strings` import are all still referenced by the
   `CI runs the dedicated guard before golangci-lint` subtest and by the file's helpers.
   `go build` / `go vet` is the gate that confirms this.

2. `fail_notice` is defined at Makefile line 85, BEFORE `version_pattern` is assigned at line 101.
   Shell functions resolve variables at CALL time, and every `fail_notice` call site is after
   line 101, so `$$version_pattern` is safe to reference inside the function body under `set -eu`.
   The function already relies on this same property for `$$expected_version`.

3. The current dump line (Makefile line 89) is `sed -n '=;l' NOTICE.md >&2` — it carries NO
   `LC_ALL=C` prefix already, so there is nothing to strip for that part of Fix 3.

4. The narrowed filter below was executed against a scratch NOTICE.md carrying all four mutations
   on macOS BSD sed. It printed every offending line (including the bare `v0.0.9` and the ZWSP
   heading, escaped as `\342\200\213`) with correct absolute line numbers and byte escapes, and
   reduced output from 66 lines to 25. Lines matching BOTH alternatives print once, not twice.

       sed -n -E "/pure-simdjson|$$version_pattern/{=;l;}" NOTICE.md >&2
</verified_facts>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Delete the brittle bytewise-locale subtest</name>
  <files>notice_version_guard_test.go</files>
  <action>
    Remove the entire `t.Run("uses a bytewise locale", ...)` subtest — currently lines 59-64,
    from `t.Run("uses a bytewise locale", func(t *testing.T) {` through its closing `})`, plus the
    blank line that separated it from the following subtest so no double blank line remains.

    Rationale to preserve in the SUMMARY: the subtest asserted an exact Makefile source substring
    (`"check-notice-version:\n\t@set -eu; \\\n\texport LC_ALL=C; \\"`) rather than guard behavior.
    Rewriting `export LC_ALL=C` as the behaviorally identical `LC_ALL=C; export LC_ALL` still
    correctly rejects the ZWSP mutation yet fails that assertion. It is also redundant: the
    `invisible version character is escaped` subtest runs the guard under `LC_ALL=en_US.UTF-8`
    and already catches a missing `LC_ALL=C` — that is exactly how the original bug surfaced.

    Do NOT touch any other subtest, helper, or import. Per verified fact 1, no symbol becomes
    unused; if `go vet` disagrees, remove only what it names and report the discrepancy rather
    than deleting proactively.
  </action>
  <verify>
    <automated>go vet ./... &amp;&amp; ! grep -q 'uses a bytewise locale' notice_version_guard_test.go &amp;&amp; grep -q 'invisible version character is escaped' notice_version_guard_test.go</automated>
  </verify>
  <done>The `uses a bytewise locale` subtest is gone, the other six subtests are untouched, the package compiles, and `go vet ./...` reports no unused symbols or imports.</done>
</task>

<task type="auto">
  <name>Task 2: Document the single-attribution constraint and narrow the failure dump</name>
  <files>Makefile</files>
  <action>
    Two independent edits to the `check-notice-version` target. Preserve every leading tab and
    trailing ` \` continuation exactly — the recipe is one `set -eu` shell invocation.

    Fix 2 — insert a comment immediately above the `.PHONY: check-notice-version` line (currently
    line 81), in the same style as the existing comment above `.PHONY: simd-isolation-check` at
    line 7. TWO lines maximum, no block, no decoration. Record that the version scan treats
    pure-simdjson as the ONLY versioned attribution allowed in NOTICE.md, and that adding a second
    vendored dependency with its own version requires widening the scan. Suggested wording:

        # The version scan treats pure-simdjson as the only versioned attribution allowed in NOTICE.md.
        # Adding a second vendored dependency with its own version requires widening the scan.

    Fix 3 — narrow the `fail_notice` dump. Replace the body of the readable branch, currently

        sed -n '=;l' NOTICE.md >&2 || printf '%s\n' 'NOTICE.md context could not be read' >&2; \

    with the filtered form from verified fact 4:

        sed -n -E "/pure-simdjson|$$version_pattern/{=;l;}" NOTICE.md >&2 || printf '%s\n' 'NOTICE.md context could not be read' >&2; \

    Note the quoting change: the sed expression moves from single to DOUBLE quotes so
    `$$version_pattern` expands. Make turns `$$` into a literal `$` for the shell. Keep the
    `|| printf ...` fallback, the `>&2` redirect, and the trailing ` \` unchanged.

    Reusing `$$version_pattern` (rather than inlining a literal) keeps the dump filter from
    drifting away from the scan pattern. Per verified fact 2 this is safe under `set -eu`.

    CRITICAL: do NOT narrow to a `/pure-simdjson/`-only filter. That was the old behavior and it
    hid exactly the version-only offending lines the file-wide sweep now detects — a bare `v0.0.9`
    line, or `Historical note: previously pinned v0.1.2`. The `pure-simdjson` OR semver-token
    alternation is required. Byte-escaped output (`l`) and line numbers (`=`) must both survive.

    Also update the surrounding intent: the preceding `printf` at line 87 announces
    "NOTICE.md content (line number followed by byte-escaped content):". Since the dump is now
    filtered, adjust that string to say the context is filtered — e.g.
    "NOTICE.md pure-simdjson and version lines (line number followed by byte-escaped content):" —
    so the diagnostic does not claim to show the whole file.

    Leave the unreadable-file branch, the `go list` resolution, the required-line loop, and the
    awk drift scan completely untouched.
  </action>
  <verify>
    <automated>make check-notice-version &amp;&amp; grep -q 'only versioned attribution' Makefile &amp;&amp; grep -q 'sed -n -E' Makefile &amp;&amp; ! grep -q "sed -n '=;l'" Makefile</automated>
  </verify>
  <done>`make check-notice-version` passes on the clean tree, the two-line constraint comment sits directly above `.PHONY: check-notice-version`, the dump uses the alternation filter, and no unfiltered whole-file `sed -n '=;l'` remains.</done>
</task>

<task type="auto">
  <name>Task 3: Run the mutation battery on scratch copies and confirm NOTICE.md is untouched</name>
  <files>(no files modified — verification only)</files>
  <action>
    Prove the guard's detection behavior is unchanged and that Fix 3 still surfaces the
    version-only offenders.

    Work entirely in a scratch directory OUTSIDE the repo (use the session scratchpad, never the
    working tree). For each case: copy `Makefile`, `NOTICE.md`, `go.mod`, and `go.sum` into a
    fresh scratch dir, apply the mutation to the scratch `NOTICE.md`, then run
    `make --no-print-directory -C <scratchdir> check-notice-version` and assert a NON-ZERO exit.

    Use `perl -0pi -e` or `python3` for mutations — BSD `sed -i '' '0,/re/'` does not work here.

    Four cases, all must be REJECTED:
      (a) ZWSP inserted into the heading version: `## pure-simdjson v0.1.7` becomes
          `## pure-simdjson v<U+200B>0.1.7`, i.e. a single U+200B ZERO WIDTH SPACE between the
          `v` and the `0`. Do NOT paste a literal invisible character into a command — emit it
          from an escape, e.g. python `"v\u200b0.1.7"` or perl `"v\x{200b}0.1.7"`. The existing
          `invisible version character is escaped` subtest in notice_version_guard_test.go builds
          the same payload via the Go escape `"v\u200b0.1.7"` if you need a reference.
      (b) appended line: `Historical note: previously pinned v0.1.2 before the current pin.`
      (c) appended bare line: `v0.0.9`
      (d) appended line: `Bundled some-other-lib v2.3.4 under Apache-2.0.`

    For case (c) specifically, capture the guard's stderr and assert the failure dump still SHOWS
    the offending bare `v0.0.9` line — this is the regression Fix 3 must not introduce. Also
    confirm the dump is genuinely narrower than the old whole-file output (fewer lines than
    `sed -n '=;l' NOTICE.md | wc -l` on the same scratch file).

    Then run the full gates from the repo root:
      - `LC_ALL=en_US.UTF-8 go test ./...`
      - `make check-notice-version`
      - `make lint`

    Finally assert the working tree NOTICE.md was never mutated: `git diff --quiet NOTICE.md`
    must exit 0. If it does not, restore with `git checkout -- NOTICE.md` and report that the
    mutation battery leaked into the repo.
  </action>
  <verify>
    <automated>LC_ALL=en_US.UTF-8 go test ./... &amp;&amp; make check-notice-version &amp;&amp; make lint &amp;&amp; git diff --quiet NOTICE.md</automated>
  </verify>
  <done>All four mutations exit non-zero on scratch copies; the case (c) dump contains `v0.0.9` and is shorter than the old whole-file dump; `LC_ALL=en_US.UTF-8 go test ./...`, `make check-notice-version`, and `make lint` all pass; `git diff --quiet NOTICE.md` exits 0.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| repo files → make recipe shell | NOTICE.md content is interpolated into shell diagnostics; content is developer-authored and version-controlled, not attacker-supplied |
| none new | No network, no package installs, no user input added by this change |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-pcc-01 | Information disclosure | `fail_notice` NOTICE.md dump | accept | Narrowing the dump strictly REDUCES output; NOTICE.md is a public, committed file with no secrets |
| T-pcc-02 | Tampering | `check-notice-version` detection coverage | mitigate | Task 3 mutation battery re-proves all four rejection cases after the edit; Fix 3 explicitly forbids reverting to the `/pure-simdjson/`-only filter that would blind the version-only cases |
| T-pcc-03 | Tampering | Makefile recipe continuation structure | mitigate | Edits preserve trailing ` \` and leading tabs; `make check-notice-version` on a clean tree is the gate in Task 2 |
| T-pcc-SC | Tampering | npm/pip/cargo installs | accept | No package-manager installs in this change; no new dependencies |
</threat_model>

<verification>
1. `LC_ALL=en_US.UTF-8 go test ./...` passes.
2. `make check-notice-version` passes on the clean tree.
3. `make lint` passes.
4. Mutation battery REJECTS all four: (a) ZWSP heading, (b) `Historical note: previously pinned v0.1.2 before the current pin.`, (c) bare `v0.0.9`, (d) `Bundled some-other-lib v2.3.4 under Apache-2.0.`
5. The case (c) failure dump SHOWS the offending bare `v0.0.9` line.
6. `git diff --quiet NOTICE.md` exits 0 — NOTICE.md is byte-identical to HEAD.
7. All mutation testing was performed on scratch copies outside the repository.
</verification>

<success_criteria>
- `uses a bytewise locale` subtest deleted; no unused imports or helpers introduced.
- Rewriting `export LC_ALL=C` as `LC_ALL=C; export LC_ALL` would no longer fail any test.
- Makefile carries a one-or-two-line comment above `.PHONY: check-notice-version` recording the
  single-attribution constraint.
- `fail_notice` dumps only pure-simdjson lines and semver-token lines, with `=` line numbers and
  `l` byte escapes intact.
- Guard detection behavior is unchanged: all four mutation cases still rejected.
- NOTICE.md unchanged in the working tree.
</success_criteria>

<output>
Create `.planning/quick/260730-pcc-polish-notice-version-guard-drop-brittle/260730-pcc-SUMMARY.md` when done
</output>
