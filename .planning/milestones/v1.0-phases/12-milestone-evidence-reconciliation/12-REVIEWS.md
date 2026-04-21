---
phase: 12
reviewers: [gemini, claude]
reviewed_at: 2026-04-20T15:58:07Z
plans_reviewed:
  - .planning/phases/12-milestone-evidence-reconciliation/12-01-PLAN.md
  - .planning/phases/12-milestone-evidence-reconciliation/12-02-PLAN.md
  - .planning/phases/12-milestone-evidence-reconciliation/12-03-PLAN.md
---

# Cross-AI Plan Review — Phase 12

## Gemini Review

# Phase 12: Milestone Evidence Reconciliation — Plan Review

## Summary
The plans for Phase 12 provide a highly disciplined approach to closing documentation and evidence debt identified in the `v1.0-MILESTONE-AUDIT.md`. By treating Phase 07 and Phase 09 as shipped features requiring retrospective synthesis, the plans avoid scope creep while ensuring that the "truth" of the repository is anchored in current-tree evidence rather than stale planning states. The two-wave execution model correctly separates independent evidence gathering from the final ledger and audit reconciliation, ensuring that `REQUIREMENTS.md` updates are strictly predicated on the existence of valid verification artifacts.

## Strengths
*   **Traceability Logic**: 12-03 correctly enforces a dependency on 12-01 and 12-02, preventing the common pitfall of marking requirements "Complete" before the supporting verification reports actually exist.
*   **Honest Validation Reporting**: The strategy for Phase 07 validation debt (Task 1 of 12-01) is exemplary; it prioritizes evidence-based closure but provides a clear path for "explicitly accepted gap" reporting if the implementation does not meet the original Nyquist compliance bar.
*   **Standardized Artifacts**: The plans rigorously adhere to the existing verification-report structures used in Phases 06, 08, 10, and 11, ensuring a consistent audit trail.
*   **Current-Tree Verification**: Instead of copy-pasting old results, the plans mandate re-running targeted tests, benchmarks (using the project-standard `-benchtime=1x` for finite fixtures), and example programs.
*   **Conservative Scoring**: The logic for refreshing the milestone audit includes updating frontmatter scores and the `gaps` block, ensuring that a "Pass" is actually earned by the new evidence.

## Concerns
*   **Benchmark Noise in Verification Reports** (Severity: **LOW**)
    *   Benchmarks in 12-01 Task 1 are necessary for `BUILD-05`, but raw benchmark output can be verbose. The agent should ensure that only the summary metrics (latency/allocs) are synthesized into the report rather than dumping the full table, to maintain readability.
*   **Manual Count Accuracy** (Severity: **LOW**)
    *   12-03 Task 1 manually sets `- Checked off: 20`. While mathematically correct based on the requirement count, a manual update is prone to being missed if the requirements table structure changes.

## Suggestions
*   **Audit "Why the Audit Fails" Clearance**: In 12-03 Task 2, explicitly ensure that the `## Why The Audit Fails` section is replaced with a success statement (e.g., `## Milestone Pass Rationale`) rather than just removing the text, to provide a positive signal for the milestone close.
*   **Verification Report Captures**: When capturing `go run` output for examples in 12-02, it is useful to include a few lines of the actual output (e.g., "expected: [0, 2]") in the `Behavioral Spot-Checks` section to prove the example logic actually worked as intended.

## Risk Assessment
**Overall Risk: LOW**

The phase is focused on documentation, traceability, and re-verifying already-shipped code. It does not modify product logic, which eliminates the risk of behavioral regressions. The primary risks (stale evidence or false-positive passes) are well-mitigated by the requirement to re-run all proof commands against the current tree and the strict ordering of the waves. The plan is highly likely to achieve the goal of making the `v1.0` milestone audit-ready.

---

## Claude Review

`★ Insight ─────────────────────────────────────`
- This phase is unusual: it ships *no product code* — only planning artifacts. That inverts normal risk: bugs become typos in `rg` patterns or stale ledger arithmetic, not runtime failures.
- The plan correctly enforces a strict ordering invariant: verification artifacts must exist *before* the requirements ledger flips checkboxes. Reversing that order recreates the same drift the audit caught.
- Wave 1's parallelism (12-01 ∥ 12-02) only works because the two tracks touch disjoint files. The dependency edge is via the `REQUIREMENTS.md` and audit refresh in Wave 2.
`─────────────────────────────────────────────────`

# Cross-AI Plan Review: Phase 12 Milestone Evidence Reconciliation

## 1. Summary

This is a **well-structured evidence-reconciliation phase** that correctly treats Phase 07 and 09 as already-shipped work and limits scope to documentation/audit closure. The three-plan / two-wave shape mirrors the dependency graph cleanly: parallel reconstruction of `07-VERIFICATION.md` (with validation-debt resolution) and `09-VERIFICATION.md`, then a single fan-in plan reconciling `REQUIREMENTS.md` and refreshing `v1.0-MILESTONE-AUDIT.md`. The plans hew tightly to existing verification-report patterns from Phases 06/08/10/11 and avoid the "lightweight placeholder" anti-pattern called out in research. The main weaknesses are mechanical: several `rg` verify commands have regex-escape bugs, the "Approval" sign-off has a subjective gate, and a few state/timestamp updates in `v1.0-MILESTONE-AUDIT.md` are underspecified.

## 2. Strengths

- **Strict ordering invariant**: 12-01/12-02 are explicit prerequisites for 12-03's ledger flip. Acceptance criteria in 12-03 Task 1 even include `test -f` guards on both verification artifacts, so the sequence cannot be silently violated.
- **Honest treatment of Phase 07 validation debt**: Plan 12-01 Task 1 explicitly forbids fake compliance and provides a documented "accepted gap" exit path that 12-03 Task 2 mirrors. This is the right shape for evidence-integrity threats T-12-04/T-12-08.
- **Reproducibility of audit evidence**: 12-03 Task 2 mandates *re-running* `go test ./... -count=1` and both example commands rather than copy-forwarding old output. Directly addresses Pitfall 4 from RESEARCH.md.
- **Pattern reuse over invention**: 12-PATTERNS.md identifies `06-VERIFICATION.md` and `11-VERIFICATION.md` as exact analogs, and the plans require the same heading set (`### Observable Truths`, `### Required Artifacts`, etc.). Eliminates evidence-format drift.
- **Requirement coverage is exhaustive**: Every `BUILD-0[1-5]` and `DERIVE-0[1-4]` ID is mentioned by name in acceptance criteria; the math checks out (11 already-checked + 5 BUILD + 4 DERIVE = 20).
- **Conservative blast radius**: Only six files modified across three plans, all in `.planning/`. Zero product code touched, so regression risk is bounded to documentation drift.

## 3. Concerns

### HIGH

- **Verify-command regex escapes are likely broken** (12-01 Task 1, 12-03 Task 2). Patterns like `Benchmark\\(AddDocumentPhase07\\|BuildPhase07\\|FinalizePhase07\\)` — once shell-unquoted — become `\(...\|...\)`, which in ripgrep's default Rust regex matches *literal* parentheses and pipes, not alternation. The verify will succeed only if the file contains the literal escaped string, which is unlikely. **Fix**: use unescaped ERE: `'Benchmark\(AddDocumentPhase07\|BuildPhase07\|FinalizePhase07\)'` won't work either; the right form is `'Benchmark\((AddDocumentPhase07|BuildPhase07|FinalizePhase07)\)'` with `\(` for literal parens and unescaped `|` for alternation.
- **Negative grep in 12-03 Task 2 may produce false negatives**. The current audit text uses `verification_status: "missing"` in YAML and `Phase \`07\` is missing \`07-VERIFICATION.md\`` in prose. The acceptance criterion checks for the prose form only. If the executor merely deletes the prose blocker but leaves the YAML `gaps:` block intact, the grep passes while the audit still self-reports as `gaps_found`. **Fix**: also assert frontmatter `status:` is no longer `gaps_found` and the YAML `gaps.requirements:` block is empty (or only contains accepted-gap entries).

### MEDIUM

- **Audit YAML frontmatter updates underspecified** (12-03 Task 2). The action mentions updating "scores and `gaps` block" but doesn't enumerate the required transitions: `audited: 2026-04-20T...Z`, `status: passed` (or equivalent), `scores.requirements: 20/20`, `scores.phases: 6/6`, `nyquist.partial_phases: []` (if 07 closed) or a documented `nyquist.accepted_gaps: [...]`. Without these, the refreshed audit could keep stale frontmatter while only updating prose.
- **"Sufficient coverage" is subjective** (12-01 Task 1). The action says set `nyquist_compliant: true` "if every mapped command is green AND coverage is sufficient." Coverage sufficiency for Nyquist is not defined here. Risk: executor sets `approved` based only on green commands, missing genuine Nyquist gaps the original audit flagged. **Fix**: enumerate the exact Nyquist coverage criteria (e.g., "every per-task verification map row has ✅ green status AND `wave_0_complete: true`") or refer to the existing `09-VALIDATION.md` audit format as the structural target.
- **No STATE.md / ROADMAP.md update step**. After Plan 12-03, STATE.md still shows `Phase 12 planning completed` and ROADMAP.md still shows Plan 0/3 complete. The executor's per-plan workflow handles this normally, but neither plan body explicitly calls it out, and the standard `update_current_position`/`update_roadmap` steps run only when not in worktree mode.
- **Phase 07 acceptance path branching not validated end-to-end**. If 12-01 Task 1 lands "accepted gap" rather than "approved", 12-01 Task 2's acceptance criterion (`Gaps Summary` `None.` OR explicit debt note) accepts both. Then 12-03 Task 2 must propagate the accepted-gap language. There's no test that the gap text is *consistent* across all three artifacts (validation, verification, audit). A copy/paste drift here would create exactly the kind of stale-evidence problem the phase is closing.

### LOW

- **Phase 12-02 Task 1 verify reruns commands but doesn't capture output**. The "Behavioral Spot-Checks" section in 12-02 Task 2 is supposed to cite results from Task 1, but Task 1's automated verify just runs the commands without writing them anywhere durable. The executor must remember to capture them inline in 09-VERIFICATION.md.
- **`RepresentationFailureModeRoundTrip` test name** appears in 12-02's targeted command list. This test was added in the 2026-04-17 worktree audit (see `09-VALIDATION.md:122`). It exists, but worth confirming it's still in the tree before the plan runs.
- **`Approval: approved 2026-04-20` is hardcoded** (12-01 Task 1 acceptance). If execution slips to 2026-04-21, the executor must remember to update; the criterion will fail otherwise.
- **README and STATE.md untouched**. README documents Phase 11 workflow; STATE.md `Pending Todos` still references `gsd-secure-phase 11`. Neither is in scope for Phase 12, but post-milestone-close review may flag them.

## 4. Suggestions

1. **Fix the ripgrep escape bugs** in 12-01 Task 1 verify, 12-03 Task 1 verify, and 12-03 Task 2 verify. Test the patterns against a sample file before committing.
2. **Add explicit YAML frontmatter mutations** to 12-03 Task 2 action: list the exact key/value transitions for `audited`, `status`, `scores.*`, and `gaps.requirements`. Add an acceptance criterion asserting `status: passed` (or your equivalent terminal state) appears in frontmatter.
3. **Strengthen the "audit no longer reports blockers" check** by greping the YAML block, not just prose: `! rg -A 20 '^gaps:' .planning/v1.0-MILESTONE-AUDIT.md | rg 'BUILD-0[1-5]|DERIVE-0[1-4]'`.
4. **Pin the Nyquist closure criteria** in 12-01 Task 1 to the structural shape of `09-VALIDATION.md`'s closed audit (per-task map all ✅ green, sign-off boxes all checked, `nyquist_compliant: true`).
5. **Add a consistency check task to 12-03** (or fold into Task 2): assert that if `07-VALIDATION.md` carries an unresolved gap, the same gap text appears verbatim in `07-VERIFICATION.md`'s `Gaps Summary` and in the refreshed audit's accepted-deviations section. A simple `rg -F` cross-file grep would suffice.
6. **Use `$(date -u +"%Y-%m-%d")` semantics** for the approval timestamp instead of hardcoding 2026-04-20, or add a note that the executor should adjust if execution slips.
7. **Add an explicit STATE.md/ROADMAP.md update note** in 12-03 confirming the executor's standard workflow steps will fire (since this is a non-worktree, non-parallel plan).
8. **Consider adding a `12-VERIFICATION.md` for Phase 12 itself** as a meta-verification that the milestone evidence reconciliation succeeded. Phases 06/08/10/11 all have one; consistency suggests Phase 12 should too.

## 5. Risk Assessment

**Overall: LOW**

Justification:
- **Blast radius is minimal**: six files in `.planning/`, zero product code, fully reversible via `git revert`.
- **Plans correctly enforce ordering**: ledger updates gated on verification-artifact existence via `test -f` checks.
- **Pattern reuse eliminates evidence-format drift**: the new artifacts will look like the existing audited ones.
- **Honest debt-handling path is preserved**: Phase 07 validation can land "accepted gap" without forcing fake compliance.

The remaining risks (broken `rg` patterns, underspecified YAML mutations, subjective sign-off gate) are mechanical and fix-once; they would cause verify-command false-positives or executor confusion, not corrupted milestone state. None are show-stoppers, but the HIGH items in §3 should be patched before execution to avoid silent drift between artifacts.

`★ Insight ─────────────────────────────────────`
- For evidence/documentation phases, the most insidious failure mode is **artifacts that pass the bar individually but disagree with each other**. Cross-artifact consistency checks (suggestion #5) are worth more than additional within-artifact assertions.
- The Phase 07 "either close or accept" branching pattern is a good model for handling sunk validation debt in any milestone — preserving honesty over green checkmarks. It's worth extracting as a reusable closure-protocol pattern.
`─────────────────────────────────────────────────`

---

## Consensus Summary

Both reviewers agree that Phase 12 is shaped correctly as an evidence-reconciliation phase rather than an implementation phase. The three-plan, two-wave split is the right dependency graph: rebuild the missing Phase 07 and Phase 09 verification artifacts independently, then reconcile the requirements ledger and milestone audit only after those artifacts exist. Both also agree that the plans make the right call by reusing the existing verification-report format, rerunning current-tree proof commands, and preserving an honest branch for Phase 07 validation debt instead of forcing a false green state.

### Agreed Strengths

- The wave ordering is correct and defensive: `12-03` is gated on `12-01` and `12-02`, so `REQUIREMENTS.md` cannot flip before `07-VERIFICATION.md` and `09-VERIFICATION.md` exist.
- The plans use current-tree evidence instead of stale summaries by rerunning targeted tests, benchmarks, and example commands before refreshing the milestone audit.
- The artifact shape is consistent with completed phases, which reduces audit-format drift and keeps milestone evidence comparable.
- Phase 07 validation debt is handled honestly, with a clear close-or-explicitly-accept branch instead of silent compliance.
- The blast radius is low because the phase is constrained to `.planning/` artifacts and does not touch product code.

### Agreed Concerns

- Verification mechanics need to be tightened so the phase cannot pass on prose-only edits. In particular, the audit refresh should validate frontmatter and blocker metadata, not just narrative strings.
- Some evidence capture is underspecified. Benchmark/example outputs and milestone-audit state transitions should be recorded explicitly enough that the final artifacts are reproducible and reviewable.
- Cross-artifact consistency is a real risk if Phase 07 lands as an accepted gap rather than a closed gap. The validation file, verification report, and milestone audit need one shared story.

### Divergent Views

- Gemini sees the phase as very low risk with only minor execution details to tighten, mostly around report readability and manual ledger arithmetic.
- Claude agrees the overall risk is still low, but it identifies concrete high-severity plan issues in the current verify patterns and in the lack of explicit audit frontmatter assertions.
- Claude suggests a possible `12-VERIFICATION.md` meta-artifact for consistency with earlier phases; Gemini does not raise that as necessary.
