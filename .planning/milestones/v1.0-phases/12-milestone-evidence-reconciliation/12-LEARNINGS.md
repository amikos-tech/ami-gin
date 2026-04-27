---
phase: 12
phase_name: "Milestone Evidence Reconciliation"
project: "GIN Index"
generated: "2026-04-22"
counts:
  decisions: 6
  lessons: 4
  patterns: 7
  surprises: 4
missing_artifacts: []
---

# Phase 12 Learnings: Milestone Evidence Reconciliation

## Decisions

### Close Phase 07 validation debt instead of carrying an accepted gap
All three required command surfaces — the targeted parser/numeric regression cluster, the `Benchmark(AddDocumentPhase07|BuildPhase07|FinalizePhase07)` smoke, and `go test ./... -count=1` — were green on the current tree on 2026-04-21, so the Phase 07 `status: verified`, `nyquist_compliant: true` close branch was taken rather than leaving an `Accepted validation gap:` line in circulation.

**Rationale:** Closure is only allowed when every command, every task row, and every sign-off checkbox lines up on the same day; the rerun met that bar, and carrying an accepted gap when one is not needed muddles downstream milestone close language.
**Source:** 12-01-PLAN.md, 12-milestone-evidence-reconciliation-01-SUMMARY.md, 12-milestone-evidence-reconciliation-03-SUMMARY.md

---

### Reuse the existing completed-phase verification-report format for Phase 07 and Phase 09
Both `07-VERIFICATION.md` and `09-VERIFICATION.md` were written using the same structure already in use by `06-VERIFICATION.md` and `11-VERIFICATION.md`: frontmatter with `phase`/`verified`/`status`/`score`/`overrides_applied` and the canonical `## Goal Achievement`, `### Observable Truths`, `### Required Artifacts`, `### Behavioral Spot-Checks`, `### Requirements Coverage`, `### Gaps Summary` sections.

**Rationale:** Feeding the milestone audit requires a consistent shape; introducing a bespoke format for rebuilt evidence would have forced the audit to branch on artifact class. Reuse preserved the audit's single read path.
**Source:** 12-01-PLAN.md, 12-02-PLAN.md, 12-milestone-evidence-reconciliation-01-SUMMARY.md, 12-milestone-evidence-reconciliation-02-SUMMARY.md

---

### Requirements ledger cites the implementing phase, not the repair phase
BUILD-01..05 rows were moved from `Phase 12 / Pending` to `Phase 07 / Complete`, and DERIVE-01..04 rows to `Phase 09 / Complete`, even though Phase 12 is the one that actually produced the verification artifacts.

**Rationale:** The ledger must reflect the phase that delivered the requirement, not the phase that repaired its evidence — otherwise future readers would think Phase 12 shipped parser/numeric/derived-representation behavior, which it did not.
**Source:** 12-03-PLAN.md, 12-milestone-evidence-reconciliation-03-SUMMARY.md

---

### Leave PATH / HCARD / SIZE mappings untouched
The reconciliation scope explicitly excluded Phase 06 / 08 / 10 rows in `REQUIREMENTS.md` — their verification artifacts already aligned with the shipped phases.

**Rationale:** Avoid "blast radius" edits on already-correct traceability. Updating only what the missing-verification blocker actually required kept the audit diff scoped and auditable.
**Source:** 12-03-PLAN.md, 12-milestone-evidence-reconciliation-03-SUMMARY.md

---

### Refresh the milestone audit in place rather than creating a new audit artifact
The refreshed `v1.0-MILESTONE-AUDIT.md` kept its heading vocabulary, frontmatter shape, and scoring schema — only the content (status, gap arrays, rationale section) flipped from `gaps_found` to `passed`. The `## Why The Audit Fails` section was renamed to `## Milestone Pass Rationale`, not replaced wholesale.

**Rationale:** A reader comparing the old and new audits needed to see it as a rerun, not a new artifact class; drift in section vocabulary would have suggested the milestone was scored under different criteria.
**Source:** 12-03-PLAN.md, 12-milestone-evidence-reconciliation-03-SUMMARY.md

---

### Summarize BUILD-05 benchmark evidence with representative deltas, not the full smoke matrix
The Phase 07 benchmark smoke output was distilled to representative latency/allocation numbers in `07-VERIFICATION.md` instead of pasting the full `Benchmark(AddDocumentPhase07|BuildPhase07|FinalizePhase07) -benchtime=1x` table.

**Rationale:** BUILD-05 is about "parser-redesign latency/allocation deltas" — the audit needs signal, not a data dump. The exact command is cited so the full table is reproducible on demand.
**Source:** 12-01-PLAN.md, 12-milestone-evidence-reconciliation-01-SUMMARY.md

---

## Lessons

### `.planning/` is gitignored in this repo, so owned artifacts must be force-staged
All three plan executions hit the same operational friction: every owned evidence artifact had to be added with `git add -f <path>` to keep the atomic task commits path-scoped and avoid the unrelated dirty shared files.

**Context:** Documented identically in all three plan summaries as an "Issues Encountered" line. This is a structural property of the repo, not a one-off — any future milestone-evidence work will hit it.
**Source:** 12-milestone-evidence-reconciliation-01-SUMMARY.md, 12-milestone-evidence-reconciliation-02-SUMMARY.md, 12-milestone-evidence-reconciliation-03-SUMMARY.md

---

### Shared planning state files (`STATE.md`, `ROADMAP.md`) must be left untouched during scoped evidence work
Each plan explicitly left the shared dirty files outside the execution scope, even though they were already modified in the working tree when the phase began.

**Context:** Scope ownership matters: a plan that edits the milestone audit should not also sweep in unrelated STATE/ROADMAP edits, because it poisons the commit diff and makes the task commit harder to audit later.
**Source:** 12-milestone-evidence-reconciliation-01-SUMMARY.md, 12-milestone-evidence-reconciliation-02-SUMMARY.md, 12-milestone-evidence-reconciliation-03-SUMMARY.md

---

### Summary-only claims are not sufficient evidence for milestone close
The Phase 07 / 09 summaries were detailed, but the milestone audit still refused to pass because the verification artifacts themselves were missing. Evidence repair required rerunning the actual commands against the current tree, not lifting claims out of summaries.

**Context:** A SUMMARY.md describes intent and outcome from the execution-day perspective; a VERIFICATION.md binds those claims to current-tree symbols and rerunnable commands. The audit consumes the latter.
**Source:** 12-01-PLAN.md, 12-02-PLAN.md, 12-VERIFICATION.md

---

### Public examples are first-class DERIVE-04 evidence, not just supplementary
`09-VERIFICATION.md` captures one representative `Observed stdout:` line from each of `go run ./examples/transformers/main.go` and `go run ./examples/transformers-advanced/main.go`, not just the exit status, because DERIVE-04 requires runnable public artifacts.

**Context:** Tests prove the internal contract; examples prove the documented public contract. For a requirement that specifically covers public example behavior, "exit 0" is insufficient — the report has to show what the example actually emitted.
**Source:** 12-02-PLAN.md, 12-milestone-evidence-reconciliation-02-SUMMARY.md, 12-VERIFICATION.md

---

## Patterns

### Current-tree evidence reconstruction
Rebuild missing verification artifacts from shipped plans + summaries + a rerun of the commands the plans originally specified. Never synthesize verification from summary prose alone.

**When to use:** Any time a completed phase needs a retroactive verification artifact to satisfy milestone close — e.g., because the phase shipped before the current verification template existed, or the artifact was lost/never produced.
**Source:** 12-01-PLAN.md, 12-02-PLAN.md, 12-VERIFICATION.md

---

### `Observed stdout:` spot-checks inside verification reports
When a requirement is satisfied by a runnable example, quote one representative output line under `### Behavioral Spot-Checks` with the `Observed stdout:` prefix, alongside the command.

**When to use:** Requirements that include "public examples" or "docs/example" coverage — especially any DERIVE-style requirement where the public alias surface is the contract.
**Source:** 12-02-PLAN.md, 12-milestone-evidence-reconciliation-02-SUMMARY.md

---

### Implementing-phase attribution in traceability ledgers
Traceability rows always cite the phase that shipped the behavior, never the phase that repaired evidence. The repair phase only shows up in plan-level source attribution inside the verification artifacts themselves.

**When to use:** Any milestone with reconciliation phases, retroactive verification passes, or deferred-evidence cleanup. Prevents the ledger from drifting into a history-of-repairs log.
**Source:** 12-03-PLAN.md, 12-milestone-evidence-reconciliation-03-SUMMARY.md

---

### Audit rerun preserves vocabulary, not just scoring
When refreshing an existing milestone audit from `gaps_found` to `passed`, keep the section headings, frontmatter shape, and scoring schema identical — only flip the content.

**When to use:** Any transition from a failed-state audit to a passed-state audit on the same milestone. Protects readers who compare artifacts across revisions.
**Source:** 12-03-PLAN.md, 12-milestone-evidence-reconciliation-03-SUMMARY.md

---

### Cross-artifact verbatim line reuse for accepted gaps
When a plan defines an `Accepted validation gap:` line, the exact same line is copied verbatim into `07-VALIDATION.md`, `07-VERIFICATION.md`, and `v1.0-MILESTONE-AUDIT.md`. If debt closes cleanly, the parallel sentence `Phase 07 validation debt: closed on current-tree evidence.` is reused verbatim in the audit under both `## Nyquist Coverage` and `## Milestone Pass Rationale`.

**When to use:** Whenever a closure or accepted-gap decision spans multiple artifacts. Verbatim reuse makes cross-artifact consistency programmatically verifiable.
**Source:** 12-01-PLAN.md, 12-03-PLAN.md, 12-VERIFICATION.md

---

### Staged ordering: artifacts first, then ledger, then audit
Plans 12-01 and 12-02 run in wave 1 and produce the missing verification artifacts; plan 12-03 runs in wave 2 and only then updates `REQUIREMENTS.md` and refreshes the audit. The dependency (`depends_on: [12-01, 12-02]`) is structural, not optional.

**When to use:** Any multi-step evidence reconciliation. The ledger must never be updated before the supporting verification exists — otherwise a partial failure leaves the ledger claiming evidence that was never produced.
**Source:** 12-03-PLAN.md, 12-milestone-evidence-reconciliation-03-SUMMARY.md

---

### `git add -f <owned-path>` for gitignored planning directories
In repos where `.planning/` is gitignored but the planning artifacts are the deliverable, every atomic task commit explicitly force-adds only the paths it owns — never `git add -A`.

**When to use:** Any project where the planning directory is gitignored but artifact commits are required; keeps commits path-scoped and surgical.
**Source:** 12-milestone-evidence-reconciliation-01-SUMMARY.md, 12-milestone-evidence-reconciliation-02-SUMMARY.md, 12-milestone-evidence-reconciliation-03-SUMMARY.md

---

## Surprises

### Phase 07 Nyquist debt closed cleanly on the rerun
Plan 12-01 was specified with two branches — a "close Phase 07 debt" branch and an "accept remaining gap" branch — because the validation state was genuinely ambiguous going in. The rerun produced clean green results, so the close branch was taken and no `Accepted validation gap:` line needed to flow downstream into 12-03.

**Impact:** Plan 12-03 executed only the clean-close rationale path (`Phase 07 validation debt: closed on current-tree evidence.`) instead of the accepted-gap copy-verbatim path, simplifying the audit language and leaving no residual debt for v1.1.
**Source:** 12-01-PLAN.md, 12-milestone-evidence-reconciliation-01-SUMMARY.md, 12-milestone-evidence-reconciliation-03-SUMMARY.md

---

### Three consecutive plans executed with zero deviations
All three Phase 12 plan summaries report "None - plan executed exactly as written" under "Deviations from Plan". Given how conditional plan 12-03 is (clean-close vs accepted-gap branches that flow through verbatim-line propagation), zero deviations is a meaningful outcome, not a truism.

**Impact:** Suggests that highly structured reconciliation plans with explicit branch handling can be executed mechanically once the upstream evidence is gathered. The branch design front-loaded the thinking so execution itself was rote.
**Source:** 12-milestone-evidence-reconciliation-01-SUMMARY.md, 12-milestone-evidence-reconciliation-02-SUMMARY.md, 12-milestone-evidence-reconciliation-03-SUMMARY.md

---

### Total phase duration was ~14 minutes across 3 plans
Plan 12-01 took 6 min, 12-02 took 3 min, 12-03 took 4m 24s — roughly 13m 24s of wall-clock work to repair two missing verification artifacts and refresh the milestone audit.

**Impact:** Evidence-reconciliation work is cheap compared to the blocker it removes — this phase alone unblocked the v1.0 milestone close. Worth the investment in structured repair plans rather than ad-hoc patching.
**Source:** 12-milestone-evidence-reconciliation-01-SUMMARY.md, 12-milestone-evidence-reconciliation-02-SUMMARY.md, 12-milestone-evidence-reconciliation-03-SUMMARY.md

---

### The `.planning/` gitignore friction was surfaced in every single summary
Rather than being noted once and forgotten, each plan's "Issues Encountered" section flags the same `git add -f` requirement. This is a structural, recurring operational cost that wasn't obvious from the initial phase setup.

**Impact:** If enough future phases hit this pattern, it argues for a Makefile target or git-tracked hook that auto-stages owned planning paths rather than relying on every plan author to remember the `-f` flag. For now it is documented across three summaries, which is enough to make it part of the extracted-learnings canon.
**Source:** 12-milestone-evidence-reconciliation-01-SUMMARY.md, 12-milestone-evidence-reconciliation-02-SUMMARY.md, 12-milestone-evidence-reconciliation-03-SUMMARY.md
