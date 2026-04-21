# Phase 12: Milestone Evidence Reconciliation - Pattern Map

**Mapped:** 2026-04-20
**Files analyzed:** 5
**Analogs found:** 5 / 5 direct

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `.planning/phases/07-builder-parsing-numeric-fidelity/07-VERIFICATION.md` | docs | transform | `.planning/phases/06-query-path-hot-path/06-VERIFICATION.md` | exact |
| `.planning/phases/09-derived-representations/09-VERIFICATION.md` | docs | transform | `.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-VERIFICATION.md` | exact |
| `.planning/phases/07-builder-parsing-numeric-fidelity/07-VALIDATION.md` | docs | transform | `.planning/phases/09-derived-representations/09-VALIDATION.md` | exact |
| `.planning/REQUIREMENTS.md` | docs | transform | `.planning/REQUIREMENTS.md` | exact |
| `.planning/v1.0-MILESTONE-AUDIT.md` | docs | transform | `.planning/v1.0-MILESTONE-AUDIT.md` | exact |

## Pattern Assignments

### `07-VERIFICATION.md` and `09-VERIFICATION.md` (phase-close verification reports)

**Primary analogs:** `06-VERIFICATION.md`, `08-VERIFICATION.md`, `10-VERIFICATION.md`, `11-VERIFICATION.md`

**Frontmatter pattern** (`06-VERIFICATION.md:1-6`):
```yaml
---
phase: 06-query-path-hot-path
verified: 2026-04-14T14:29:26Z
status: passed
score: 8/8 must-haves verified
overrides_applied: 0
---
```

Use the same frontmatter keys for new Phase 07 and Phase 09 verification reports.

**Section pattern** (`11-VERIFICATION.md`):
- `# Phase NN: ... Verification Report`
- `## Goal Achievement`
- `### Observable Truths`
- `### Required Artifacts`
- `### Behavioral Spot-Checks`
- `### Requirements Coverage`
- `### Gaps Summary`

Do not replace this with a lighter summary format. Phase 12 is closing milestone audit blockers, so the report shape must match the completed-phase verification artifacts already used by the milestone audit.

**Evidence pattern**
- Cite current-tree commands in `Behavioral Spot-Checks`
- Tie every requirement row to both an implementation artifact and a passing command
- Keep `Gaps Summary` explicit: either `None` or a narrowly scoped accepted gap

### `07-VALIDATION.md` (Nyquist validation strategy refresh)

**Primary analog:** `09-VALIDATION.md`

**Frontmatter pattern** (`09-VALIDATION.md:1-7`):
```yaml
---
phase: 09
slug: derived-representations
status: verified
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-16
updated: 2026-04-17T07:59:41Z
---
```

If Phase 07 debt is closed, refresh `07-VALIDATION.md` into this same audited shape:
- keep `Per-Task Verification Map`
- add an audit section with concrete command evidence
- finish with a checked `Validation Sign-Off`

If closure is not supportable, keep the document honest and record the explicit acceptance in the milestone audit instead of pretending Nyquist compliance.

### `.planning/REQUIREMENTS.md` (requirements ledger)

**Primary analog:** the current file itself

**Two places must stay in sync**
- requirement checklist status in each domain section
- `## Traceability` table

For Phase 12, only update requirement rows after the new Phase 07 and Phase 09 verification reports exist. The file already shows the desired style for completed requirements:

```markdown
- [x] **HCARD-01**: ...
...
| HCARD-01 | Phase 08 | Complete |
```

Match that exact checkbox/table pairing for `BUILD-*` and `DERIVE-*`, while also confirming `PATH-*`, `HCARD-*`, and `SIZE-*` remain aligned with their existing verification artifacts.

### `.planning/v1.0-MILESTONE-AUDIT.md` (milestone-close evidence)

**Primary analog:** the current file itself

**Required structure to preserve**
- YAML frontmatter with `status`, `scores`, `nyquist`, and `gaps`
- `## Result`
- `## Current-Tree Integration Evidence`
- `## Phase Status`
- `## Requirements Coverage`
- `## Cross-Phase Integration`
- `## End-to-End Flows`
- `## Nyquist Coverage`
- `## Why The Audit Fails` or the equivalent pass-state explanation

For Phase 12, keep the file as a refreshed audit artifact, not a changelog. The important pattern is that every conclusion is backed by either:
- a verification artifact
- a requirements-ledger row
- a current-tree command result

### Concrete reuse guidance

- Use `06-VERIFICATION.md` as the strongest structural analog for a requirement-heavy code phase verification report.
- Use `11-VERIFICATION.md` as the strongest analog for a documentation/evidence-heavy verification report with explicit current-tree commands.
- Use `09-VALIDATION.md` as the strongest analog for how to convert a previously incomplete validation strategy into an audited, compliant artifact.
- Use the current `v1.0-MILESTONE-AUDIT.md` headings and scoring vocabulary verbatim where possible so the refreshed report reads as the same audit rerun, not a different artifact class.
