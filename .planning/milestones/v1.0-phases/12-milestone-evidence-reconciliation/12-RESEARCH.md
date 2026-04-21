# Phase 12: Milestone Evidence Reconciliation - Research

**Researched:** 2026-04-20
**Domain:** Verification-artifact reconstruction and requirements-ledger reconciliation for the v1.0 milestone
**Confidence:** HIGH

<user_constraints>
## User Constraints (from ROADMAP and milestone audit)

Phase 12 has no `12-CONTEXT.md`, so the locked constraints come from `.planning/ROADMAP.md`, `.planning/REQUIREMENTS.md`, and `.planning/v1.0-MILESTONE-AUDIT.md`.

### Locked Decisions

#### Scope
- This phase closes planning-evidence debt, not product-code gaps.
- The work must reconcile shipped Phase 07 and Phase 09 reality instead of reopening their implementation scope.
- The milestone-close decision must be grounded in current-tree evidence, not stale plan status.

#### Required outcomes
- `07-VERIFICATION.md` must exist and cover `BUILD-01` through `BUILD-05` against the shipped Phase 07 implementation.
- `09-VERIFICATION.md` must exist and cover `DERIVE-01` through `DERIVE-04` against the shipped Phase 09 implementation.
- `.planning/REQUIREMENTS.md` must match verified status for `PATH-*`, `BUILD-*`, `HCARD-*`, `DERIVE-*`, and `SIZE-*`.
- Phase 07 Nyquist / validation debt must be closed or explicitly accepted with updated milestone-audit evidence.
- Re-running the milestone audit must no longer fail on missing verification artifacts or stale requirement status.

#### Preferred approach implied by the audit
- Reconstruct verification from shipped plans, summaries, validation strategy, and current-tree behavior.
- Reuse existing verification-report patterns from Phases 06, 08, 10, and 11 instead of inventing a new evidence format.
- Keep the work repo-local and reproducible with the same commands already cited by the milestone audit where possible.

### Agent Discretion
- Exact wave split between Phase 07 evidence, Phase 09 evidence, and ledger/audit reconciliation.
- Whether Phase 07 validation debt is closed by refreshing `07-VALIDATION.md` to current evidence or explicitly accepted in the refreshed audit.
- Exact verification commands, as long as they remain grounded in existing tests/examples and prove the required claims.

### Deferred / Out of Scope
- Re-implementing Phase 07 parser/numeric work or Phase 09 derived-representation behavior.
- Expanding milestone scope beyond the audit blockers already identified.
- Adding new product features while reconstructing evidence.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| BUILD-01 | The primary ingest path no longer relies on `json.Unmarshal(..., &any)` for full-document decoding. | `07-01-PLAN.md` and `07-builder-parsing-numeric-fidelity-01-SUMMARY.md` claim this explicitly; Phase 12 needs a verification report that proves the shipped tree still satisfies it. |
| BUILD-02 | Integer-vs-float classification is based on explicit number parsing rather than generic `float64` decoding. | Phase 07 plans/summaries and current tests describe explicit numeric parsing and exact-int semantics; the missing artifact is phase-level verification. |
| BUILD-03 | Supported integers preserve precision before stats/bitmap decisions are made. | Phase 07 summary and tests cite exact `int64` handling; verification must anchor that to current code and current command output. |
| BUILD-04 | Unsupported or unrepresentable numeric values fail safely with explicit errors. | Phase 07 plan/summary and validation map already identify atomic-failure and unsupported-number regressions; verification must restate and prove them. |
| BUILD-05 | Benchmarks capture ingest/build latency and allocation deltas for the parser redesign. | `07-02-PLAN.md` and `07-builder-parsing-numeric-fidelity-02-SUMMARY.md` already establish the benchmark harness; verification must prove the evidence still exists and the harness runs. |
| DERIVE-01 | Derived indexes preserve raw indexing while adding additive companion representations. | Phase 09 Plan 01 and its summary already document this behavior; verification is missing. |
| DERIVE-02 | Derived representations are queryable through explicit aliases. | Phase 09 Plan 02 and its summary already document alias routing and introspection; verification is missing. |
| DERIVE-03 | Serialization persists derived-representation metadata across round trips. | Phase 09 Plan 02 and current serialization tests cover this; Phase 12 must convert it into phase-level verification evidence. |
| DERIVE-04 | Tests/examples cover date/time, normalized text, and extracted-subfield derived patterns. | Phase 09 Plan 03 and its summary already identify the acceptance tests/examples; verification must prove they remain valid on the current tree. |
</phase_requirements>

## Summary

Phase 12 is an evidence-reconciliation phase, not an implementation phase. The fastest credible path is to treat Phase 07 and Phase 09 as already-shipped work whose missing milestone evidence must be rebuilt from:

1. the original plan contracts (`07-01/02-PLAN.md`, `09-01/02/03-PLAN.md`);
2. the shipped summaries that already claim requirement completion;
3. the current validation artifacts (`07-VALIDATION.md`, `09-VALIDATION.md`);
4. the current tree’s repo-local commands (`go test ./...`, targeted regressions, and the two transformer examples);
5. the established verification-report format used by Phases 06, 08, 10, and 11.

The milestone audit already narrows the blocker list to three concrete items:

- missing `07-VERIFICATION.md`
- missing `09-VERIFICATION.md`
- stale `REQUIREMENTS.md` status for the requirements those phases delivered

That means the plan should not spend time rediscovering phase scope. It should instead create two evidence-reconstruction tracks in parallel, then a final reconciliation track that updates the requirements ledger and refreshes the milestone audit against the new evidence set.

## Project Constraints

- Stay within `.planning/` plus repo-local verification commands unless a current-tree proof requires otherwise.
- Preserve the existing verification-report style used by completed phases so the milestone audit can compare artifacts consistently.
- Do not treat missing verification docs as permission to weaken the proof bar; the replacement reports still need current-tree command evidence and file-level traceability.
- Prefer updating the stale Phase 07 validation artifact to current evidence if the commands are now green; only fall back to explicit acceptance if Nyquist closure is not supportable on the current tree.
- Keep requirement reconciliation conservative: only mark `BUILD-*` and `DERIVE-*` complete after the corresponding new phase-level verification docs exist, while also confirming `PATH-*`, `HCARD-*`, and `SIZE-*` rows still align with their existing verification reports.

## Standard Stack

### Core

- Existing phase plans and summaries under `.planning/phases/07-builder-parsing-numeric-fidelity/` and `.planning/phases/09-derived-representations/` are the main source-of-truth inputs.
- Existing verification reports from Phases 06, 08, 10, and 11 are the output-shape reference for new verification artifacts.
- Existing validation strategies (`07-VALIDATION.md`, `09-VALIDATION.md`) are the task-to-command reference for Nyquist and verification mapping.
- Repo-local Go commands remain the primary proof surface:
  - `go test ./... -count=1`
  - targeted `go test ./... -run '...' -count=1`
  - `go run ./examples/transformers/main.go`
  - `go run ./examples/transformers-advanced/main.go`

### Supporting

- `.planning/v1.0-MILESTONE-AUDIT.md` already contains the exact blocker inventory and can serve as the before/after reconciliation baseline.
- `.planning/REQUIREMENTS.md` is the single ledger that must be updated once new verification artifacts exist.
- `.planning/STATE.md` and the eventual phase summary should reflect that this is documentation/evidence closure work rather than feature development.

### Alternatives Locked Out by Context

- Do not create lightweight placeholder verification docs that merely restate summary claims. The completed-phase verification docs in this repo all include observable truths, required artifacts, behavioral spot-checks, and requirement mapping.
- Do not update `REQUIREMENTS.md` before the Phase 07 and Phase 09 verification reports exist; doing so would repeat the same stale-ledger problem in reverse.
- Do not rerun the milestone audit against stale Phase 07 validation status without first deciding whether that debt is now closable.

## Architecture Patterns

### Pattern 1: Reconstruct Phase-Level Verification from Plan + Summary + Current Tree

Use the existing completed-phase verification reports as the template:

- frontmatter with `phase`, `verified`, `status`, `score`, `overrides_applied`
- `## Goal Achievement`
- `### Observable Truths`
- `### Required Artifacts`
- `### Behavioral Spot-Checks`
- `### Requirements Coverage`
- `### Gaps Summary`

For Phase 07 and Phase 09, the missing work is not feature discovery. It is phase-level synthesis that binds existing plan commitments and current proof commands into that same report shape.

### Pattern 2: Treat Phase 07 and Phase 09 as Independent Wave-1 Tracks

The two missing verification artifacts are independent:

- Phase 07 evidence depends on its own plans, summaries, validation strategy, and parser/numeric tests.
- Phase 09 evidence depends on its own plans, summaries, validation strategy, alias/representation tests, and example programs.

They should be planned as parallel Wave 1 plans so the final ledger/audit plan can depend on both.

### Pattern 3: Use Ledger/Audit Reconciliation as the Only Wave-2 Fan-In

Once `07-VERIFICATION.md` and `09-VERIFICATION.md` exist:

- update `.planning/REQUIREMENTS.md` so `BUILD-*` and `DERIVE-*` reflect verified completion;
- confirm `PATH-*`, `HCARD-*`, and `SIZE-*` still align with existing verification artifacts;
- refresh `v1.0-MILESTONE-AUDIT.md` so the blocker list and scores reflect the new evidence set;
- if Phase 07 validation debt remains, capture the explicit acceptance rationale in the audit.

This keeps the final plan focused on milestone truth reconciliation rather than mixing evidence generation and ledger updates in the same step.

### Pattern 4: Prefer Closing the Phase 07 Validation Debt Over Carrying It Forward

`07-VALIDATION.md` is currently draft and non-compliant, but the current tree already contains the shipped tests, benchmark harness, and summaries for BUILD-01 through BUILD-05. That makes Phase 12 the right place to decide whether the validation contract can now be refreshed to a compliant state. If current commands and coverage support it, closing the debt is stronger than merely accepting it in the milestone audit.

## Don’t Hand-Roll

| Problem | Don’t Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Verification format | A new ad hoc evidence document layout | Existing Phase 06/08/10/11 verification-report structure | The milestone already has a proven verification artifact shape. |
| Requirement reconciliation | Separate side ledger or checklist | `.planning/REQUIREMENTS.md` | It is already the canonical traceability ledger. |
| Milestone closure proof | Narrative-only audit notes | Refreshed `v1.0-MILESTONE-AUDIT.md` backed by explicit command/file evidence | The current audit already established the scoring and blocker vocabulary. |
| Phase 07 debt handling | Blanket override without inspection | Refresh `07-VALIDATION.md` first, accept only if evidence still cannot support closure | This keeps Nyquist closure evidence-based. |

## Common Pitfalls

### Pitfall 1: Updating REQUIREMENTS Before Verification Exists

This recreates the same mismatch that caused the audit to fail. The plan should require verification artifacts first, ledger updates second.

### Pitfall 2: Rewriting History Instead of Verifying the Current Tree

Phase 12 should verify what shipped on the current branch, not what Phase 07 or Phase 09 originally intended in isolation. Existing summaries are inputs, not sufficient proof on their own.

### Pitfall 3: Leaving Phase 07 Validation Ambiguous

Success criterion 4 explicitly calls out Phase 07 validation debt. The phase plan must either close `07-VALIDATION.md` with current evidence or record a deliberate accepted gap in the refreshed audit.

### Pitfall 4: Refreshing the Audit Without Re-running Its Underlying Commands

The current audit cites:

- `go test ./... -count=1`
- `go run ./examples/transformers/main.go`
- `go run ./examples/transformers-advanced/main.go`

The refreshed audit should rerun current-tree commands instead of copy-forwarding old results.

## Environment Availability

- The required inputs already exist locally in `.planning/`.
- The required proof commands are repo-local and do not depend on external services.
- No new product code appears necessary for milestone closure; the work is documentation, traceability, and current-tree verification.

## Validation Architecture

Phase 12 should validate four things:

1. **Phase 07 evidence reconstruction**
   - `07-VERIFICATION.md` exists.
   - It maps `BUILD-01` through `BUILD-05` to current-tree evidence.
   - The Phase 07 targeted commands still pass on the current tree.
   - `07-VALIDATION.md` is either refreshed to a compliant state or its remaining debt is explicitly accepted in the audit.

2. **Phase 09 evidence reconstruction**
   - `09-VERIFICATION.md` exists.
   - It maps `DERIVE-01` through `DERIVE-04` to current-tree evidence.
   - The Phase 09 targeted commands and example runs still pass on the current tree.

3. **Requirements ledger reconciliation**
   - `BUILD-*` and `DERIVE-*` checkbox and traceability rows are updated from pending to verified completion.
   - Existing `PATH-*`, `HCARD-*`, and `SIZE-*` rows still match the completed-phase verification docs.

4. **Milestone audit refresh**
   - The refreshed audit no longer reports missing Phase 07/09 verification or stale requirement status as blockers.
   - The refreshed audit records the current-tree command evidence used to reach that conclusion.

Recommended quick proof surface:

- `go test ./... -run 'Test(AddDocument.*|.*Numeric.*|.*Transformer.*Numeric.*|.*Alias.*|.*Representation.*)' -count=1`
- `go run ./examples/transformers/main.go`
- `go run ./examples/transformers-advanced/main.go`
- `go test ./... -count=1`
- `rg -n 'BUILD-0[1-5]|DERIVE-0[1-4]' .planning/phases/07-builder-parsing-numeric-fidelity/07-VERIFICATION.md .planning/phases/09-derived-representations/09-VERIFICATION.md .planning/REQUIREMENTS.md`

## Recommended Plan Shape

Three plans across two waves is the cleanest structure:

### Wave 1

- **Plan 01:** Reconstruct Phase 07 verification evidence and close or explicitly resolve Phase 07 validation debt.
- **Plan 02:** Reconstruct Phase 09 verification evidence against the current tree.

### Wave 2

- **Plan 03:** Reconcile `.planning/REQUIREMENTS.md`, refresh `v1.0-MILESTONE-AUDIT.md`, and capture the final milestone-close evidence state.

This shape maximizes parallelism while keeping the final ledger/audit step dependent on both reconstructed verification artifacts.

## Security Domain

The main risks are evidence integrity risks, not product attack-surface changes:

- false-positive milestone closure from stale or incomplete proof
- requirement rows marked complete without matching verification artifacts
- audit refreshes that cite old command output instead of current-tree runs

The phase threat model should therefore focus on traceability integrity, stale-evidence detection, and conservative milestone closure.

## Sources

### Repository (HIGH confidence)

- `.planning/ROADMAP.md`
- `.planning/REQUIREMENTS.md`
- `.planning/STATE.md`
- `.planning/v1.0-MILESTONE-AUDIT.md`
- `.planning/phases/07-builder-parsing-numeric-fidelity/07-01-PLAN.md`
- `.planning/phases/07-builder-parsing-numeric-fidelity/07-02-PLAN.md`
- `.planning/phases/07-builder-parsing-numeric-fidelity/07-builder-parsing-numeric-fidelity-01-SUMMARY.md`
- `.planning/phases/07-builder-parsing-numeric-fidelity/07-builder-parsing-numeric-fidelity-02-SUMMARY.md`
- `.planning/phases/07-builder-parsing-numeric-fidelity/07-VALIDATION.md`
- `.planning/phases/09-derived-representations/09-01-PLAN.md`
- `.planning/phases/09-derived-representations/09-02-PLAN.md`
- `.planning/phases/09-derived-representations/09-03-PLAN.md`
- `.planning/phases/09-derived-representations/09-01-SUMMARY.md`
- `.planning/phases/09-derived-representations/09-02-SUMMARY.md`
- `.planning/phases/09-derived-representations/09-03-SUMMARY.md`
- `.planning/phases/09-derived-representations/09-VALIDATION.md`
- `.planning/phases/06-query-path-hot-path/06-VERIFICATION.md`
- `.planning/phases/08-adaptive-high-cardinality-indexing/08-VERIFICATION.md`
- `.planning/phases/10-serialization-compaction/10-VERIFICATION.md`
- `.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-VERIFICATION.md`
- `AGENTS.md`

## Metadata

- Phase: `12-milestone-evidence-reconciliation`
- Research mode: default plan-phase research path without `CONTEXT.md`
- Formal requirement IDs: `BUILD-01`, `BUILD-02`, `BUILD-03`, `BUILD-04`, `BUILD-05`, `DERIVE-01`, `DERIVE-02`, `DERIVE-03`, `DERIVE-04`
