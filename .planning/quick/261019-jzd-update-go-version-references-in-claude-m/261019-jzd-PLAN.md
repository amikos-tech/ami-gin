---
phase: quick-261019-jzd
plan: 01
type: execute
wave: 1
depends_on: []
files_modified: [CLAUDE.md]
autonomous: true
requirements: [QUICK-261019-jzd]
must_haves:
  truths:
    - "CLAUDE.md states Go 1.26.0 everywhere it previously stated 1.25.5"
    - "CLAUDE.md contains no remaining 1.25 references"
    - "The change is one signed commit on branch chore/go-126-claude-md"
  artifacts:
    - path: "CLAUDE.md"
      provides: "Go version references matching the go.mod floor"
      contains: "1.26.0"
  key_links:
    - from: "CLAUDE.md"
      to: "go.mod"
      via: "Go version floor (go 1.26.0, line 3)"
      pattern: "1\\.26\\.0"
---

<objective>
Update the four stale "Go 1.25.5" references in CLAUDE.md to match the go.mod floor `go 1.26.0` (raised in PR #97 by OpenTelemetry v1.47.0).

Purpose: Keep project instructions consistent with go.mod.
Output: CLAUDE.md edited, one signed docs commit on `chore/go-126-claude-md`.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@CLAUDE.md
</context>

<tasks>

<task type="auto">
  <name>Task 1: Replace the four Go 1.25.5 references in CLAUDE.md and commit</name>
  <files>CLAUDE.md</files>
  <action>
Confirm the current branch is `chore/go-126-claude-md` (`git branch --show-current`). Never commit on main.

Edit CLAUDE.md with the Edit tool. Replace exactly these four lines, one per edit, keeping the leading "- " and all other text unchanged:

1. Line 184: `- **Go version**: 1.25.5 (already current)` becomes `- **Go version**: 1.26.0 (go.mod floor; raised in #97 by OpenTelemetry v1.47.0)`
2. Line 192: `- Go 1.25.5 - Entire codebase (library + CLI tool)` becomes `- Go 1.26.0 - Entire codebase (library + CLI tool)`
3. Line 195: `- Go toolchain 1.25.5 (specified in \`go.mod\`)` becomes `- Go toolchain 1.26.0 (specified in \`go.mod\`)` (backticks around go.mod stay literal in the file)
4. Line 244: `- Go 1.25.5+` becomes `- Go 1.26.0+`

Do not touch any other file (not .planning/PROJECT.md, not go.mod). Do not open a pull request; the orchestrator does that.

After verification passes, stage only CLAUDE.md and commit with a signed commit: `git add CLAUDE.md && git commit -S -m "docs(claude): update Go version references to 1.26.0"`. The message must be exactly that subject, with no body and no "Generated with Claude Code" trailer.
  </action>
  <verify>
    <automated>test "$(grep -c '1\.25' CLAUDE.md)" = "0" && test "$(grep -c '1\.26\.0' CLAUDE.md)" = "4" && test "$(git diff --name-only HEAD~1 HEAD)" = "CLAUDE.md" && test "$(git log -1 --format=%s)" = "docs(claude): update Go version references to 1.26.0" && git log -1 --format=%G? | grep -qv N && test "$(git branch --show-current)" = "chore/go-126-claude-md"</automated>
  </verify>
  <done>`grep -c "1\.25" CLAUDE.md` prints 0 and `grep -c "1\.26\.0" CLAUDE.md` prints 4. One signed commit with subject `docs(claude): update Go version references to 1.26.0` exists on `chore/go-126-claude-md`, touching only CLAUDE.md.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| none | Documentation-only edit to a markdown file; no code, input, or dependency changes |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-quick-01 | Tampering | commit on main | mitigate | Task checks the branch is chore/go-126-claude-md before committing; verify asserts it |
| T-quick-02 | Repudiation | unsigned commit | mitigate | Commit uses `git commit -S`; verify checks signature status is not N |
</threat_model>

<verification>
- `grep -c "1\.25" CLAUDE.md` prints 0
- `grep -c "1\.26\.0" CLAUDE.md` prints 4
- `git show --stat HEAD` lists only CLAUDE.md
</verification>

<success_criteria>
CLAUDE.md Go version references match go.mod `go 1.26.0`, committed once, signed, on the feature branch, with no PR opened by the plan.
</success_criteria>

<output>
Create `.planning/quick/261019-jzd-update-go-version-references-in-claude-m/261019-jzd-SUMMARY.md` when done
</output>
