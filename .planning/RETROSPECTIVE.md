# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — Query & Index Quality

**Shipped:** 2026-04-21
**Phases:** 7 (06-12) | **Plans:** 19 | **PRs:** 9 merged (Phase 6, 7, 8, 9, 10, 11, 12, deps bump, docs)
**Timeline:** 2026-04-14 → 2026-04-21 (7 days)

### What Was Built

- **Query hot path** (Phase 06): canonical JSONPath lookup resolves variant spellings through a single stored path name; immutable hot-path map eliminates linear `PathDirectory` scans in fresh and decoded indexes.
- **Builder numeric fidelity** (Phase 07): transactional explicit-number ingest via `json.Decoder.UseNumber()`; exact-`int64` semantics preserved end-to-end with guarded mixed-mode promotion; in-repo legacy control path for reproducible benchmark deltas.
- **Adaptive high-cardinality indexing** (Phase 08): bounded exact hot-term bitmaps + deterministic bucket fallback replace all-or-nothing bloom-only degradation; mode-aware `gin-index info` surfaces exact/bloom/adaptive per path.
- **Derived representations** (Phase 09): additive raw-plus-companion staging with explicit alias routing (`gin.As(alias, value)`), deterministic introspection, and v7 representation metadata round-trip parity.
- **Serialization compaction** (Phase 10): v9 ordered-string sections compact path names, classic terms, and adaptive promoted terms via prefix-block encoding; fail-closed decoding with explicit v8 rejection.
- **Real-corpus validation** (Phase 11): env-gated benchmark family over smoke/subset/large corpora including the text-heavy flat no-win case; evidence-backed recommendation report + README reproduction guidance.
- **Milestone evidence reconciliation** (Phase 12): reconstructed Phase 07/09 verification artifacts from current-tree commands, reconciled REQUIREMENTS.md to 20/20, refreshed `v1.0-MILESTONE-AUDIT.md` to a passed state.

### What Worked

- **Phase 12 as a reconciliation phase.** Treating "milestone close evidence debt" as its own numbered phase (rather than a bolt-on pre-close checklist) produced durable artifacts: real VERIFICATION.md files for 07/09 that reviewers could replay against the code, not summary prose.
- **Dual-encode with post-hoc `WithEncodeStrategy` deferral.** Shipping Phase 10 with the dual-encode pass *plus* Phase 11 benchmark evidence made the "add a knob" question (Phase 999.4/999.5) a data-driven decision instead of a speculative one.
- **External PR review loop.** Running `/pr-review-toolkit:review-pr` on the docs-only Phase 12 branch caught 4 real evidence-drift issues (wrong line ranges, bad plan count, absolute paths) that a surface-level read would have missed. The same pattern would have been overkill on a code-only branch.
- **Squash merges + detailed PR bodies.** Seven squash-merged phase commits on main with rich PR bodies keeps `git log main` readable while preserving per-plan history inside the PR thread.

### What Was Inefficient

- **Phase 07 validation ambiguity lingered.** The gap between "validation done" and "validation debt closed" persisted across several phases before Phase 12 forced explicit closure. Future phases should close the VALIDATION.md state at phase-close, not at milestone close.
- **Audit tool false positives.** `audit-open` flagged 3 shipped quick-tasks as `[missing]` because its heuristic doesn't recognize directory-only completion markers. Acknowledge-and-proceed worked, but the tool should match either commit-linked or summary-file completion signals.
- **"10 phases" CLI miscount in MILESTONES.md.** `milestone.complete` counted all phase dirs (including prior v0.1.0 phases 01-05 and 999.x backlog) rather than only v1.0 phases. Had to fix by hand.

### Patterns Established

- **Evidence reconciliation phase.** When a milestone close audit finds missing VERIFICATION.md / ambiguous VALIDATION.md / stale REQUIREMENTS.md traceability, add a dedicated reconciliation phase rather than patching artifacts ad-hoc.
- **Blockquote notes for cross-doc arithmetic.** When two docs count the same thing differently (STATE.md `completed_phases: 7` vs audit `phases: 6/6`), leave a blockquote in both pointing at the scope difference. Beats trying to force one number to win.
- **Research-first vendor evaluations.** The 2026-04-20 sweep (Lucene BlockTree, Tantivy SSTable, RocksDB, LevelDB, Badger, Roaring, Bleve/Vellum, Lasch VLDB 2020) before Phase 10 proved the "knob, not heuristic" precedent and let us ship a simple dual-encode path without over-engineering. Do this whenever a structural index choice comes up.
- **Plan-level derived companion aliases in public API.** Public surface uses `gin.As("ipv4_int", "192.168.1.1")`; internal encoding is `__derived:$.ip#ipv4_int`. This split kept all DERIVE-* requirements reviewable without exposing internal path syntax.

### Key Lessons

1. **Close validation debt at phase close, not milestone close.** Drift between VERIFICATION and VALIDATION compounds — Phase 12 existed because of unresolved Phase 07 validation state.
2. **Factual verification is the right review for doc-only PRs.** Spawn a verification agent (cross-check claims against code) rather than running code-focused review agents on planning artifacts — it turns "subjective doc review" into a falsifiable pass.
3. **Benchmark evidence must include the boring cases.** Phase 11's text-heavy no-win corpus was the most important result — without it, the compaction recommendation would have overclaimed value.
4. **Acknowledge false-positive audit flags with a reason.** When the audit tool drift is real, document the acknowledgment in STATE.md (with commit SHAs that prove the work shipped). Future-us shouldn't have to re-derive the "why it's OK" each time.

### Cost Observations

- Model mix: Opus-weighted (Opus 4.7) for planning/execution/verification; no explicit budget tracking
- Sessions: ~14 (one per phase + sub-plans + evidence reconciliation + ship/close)
- Notable: Phase 12 (evidence reconciliation) was the most leverage per token — three short plans produced artifacts that unblocked the whole milestone close.

---

## Milestone: v1.2 — Ingest Correctness & Per-Document Isolation

**Shipped:** 2026-04-27
**Phases:** 3 (16-18) | **Plans:** 12 | **Tasks:** 27 including one integration lint deviation
**Timeline:** 2026-04-23 → 2026-04-25

### What Was Built

- **AddDocument atomicity** (Phase 16): validator-backed merge path, `tragicErr` terminal state, merge-panic recovery, public failure catalog, and full-vs-clean encoded-byte property coverage.
- **Failure-mode taxonomy** (Phase 17): unified `IngestFailureMode` API across parser, transformer, and numeric failures; hard defaults; whole-document soft skips; v9 transformer wire-token compatibility.
- **Structured ingest errors** (Phase 18): exported `IngestError` with path, layer, cause, and verbatim value; hard-ingest behavior matrix; AST guard against plain hard-ingest errors.
- **CLI failure reporting** (Phase 18): `gin-index experiment --on-error continue` grouped structured failures in text and JSON, with deterministic layer ordering and bounded samples.

### What Worked

- **Validate-before-mutate matched the existing builder shape.** The existing staged-document architecture made the Lucene-style contract achievable without snapshot/restore machinery.
- **Property tests made atomicity falsifiable.** Comparing encoded full-vs-clean corpora caught the exact behavior the milestone cared about instead of asserting incidental internal counters.
- **Behavior matrix plus AST guard kept `IngestError` focused.** Tests verified both what became structured and what deliberately stayed outside the public document-failure type.
- **Quick follow-up loops worked for review feedback.** The post-Phase 18 quick tasks tightened guard semantics and tragic reporting without reopening the milestone shape.

### What Was Inefficient

- **The local `gsd-sdk` lacked documented query commands.** Several workflow steps had to be executed by direct artifact inspection and manual document updates.
- **Milestone audit was reconstructed late.** Phase verification and UAT artifacts were strong, but the milestone-level audit file did not exist until close.
- **Review follow-ups accumulated as many quick tasks.** The feedback was useful, but closure required several small passes after the main phase execution.

### Patterns Established

- **Per-document failure contract.** User-input ingest failures should be isolated and observable; only internal invariants should close the builder.
- **Unified failure vocabulary.** Failure-mode knobs should describe behavior across layers, not implementation origin.
- **Caller-owned redaction.** Library errors can carry verbatim value context while documentation makes logging policy explicit.
- **Guard public guarantees with executable structure checks.** AST tests are a good fit for narrow "do not return plain errors here" policies.

### Key Lessons

1. **Milestone audits should be created before close, not during close.** Phase verification does not replace a current milestone audit artifact.
2. **Soft failure semantics need whole-document language everywhere.** Partial staging is the confusing failure mode; tests and docs should state that soft skips discard the whole failed document.
3. **CLI JSON schema changes should stay nested under existing report objects.** Adding `summary.failures` avoided a report-shape break while still exposing structured failures.
4. **Keep deferred performance ideas in 999.x until profiling exists.** v1.2 stayed correctness-focused because allocation cleanup and wildcard opt-out were parked explicitly.

### Cost Observations

- Model mix: quality profile; no explicit budget tracking
- Sessions: main Phase 16-18 work plus several quick review follow-ups
- Notable: The most valuable checks were full-vs-clean property testing, per-layer behavior matrices, and focused AST enforcement.

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Phases | Plans | Key Change |
|-----------|--------|-------|------------|
| v0.1.0 (OSS launch) | 5 | — | Treat planning dir as private (`.planning/` gitignored, force-add for commits) |
| v1.0 (Query & Index Quality) | 7 | 19 | Added evidence reconciliation as a numbered phase; external PR review for doc-only changes |
| v1.2 (Ingest Correctness) | 3 | 12 | Established per-document ingest atomicity, unified failure modes, and structured ingest error reporting |

### Cumulative Quality

| Milestone | Go LOC | Operators | Transformers | Wire version | Test files |
|-----------|--------|-----------|--------------|--------------|-----------|
| v0.1.0 | ~12,000 | 12 | 5 | v8 | ~8 |
| v1.0 | ~25,500 | 12 | 13 + 3 helpers | v9 | ~15 |
| v1.2 | ~36,000 | 12 | 13 + 3 helpers | v9 | ~25 |

### Top Lessons (Verified Across Milestones)

1. **Keep planning artifacts versioned locally, force-add to commits.** The `.planning/` gitignore pattern has survived two milestones without friction.
2. **Squash merge with rich PR bodies.** Clean main trunk + durable per-plan history inside GitHub PRs works for both code and docs-only branches.
