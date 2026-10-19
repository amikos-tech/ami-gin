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

## Milestone: v1.3 — SIMD-First Performance

**Shipped:** 2026-10-19
**Phases:** 4 (19-22) | **Plans:** 16 | **Tasks:** 32
**Timeline:** 2026-04-27 → 2026-10-19 (Phases 19-21 by 2026-07-23; Phase 22 plans 01-07 on 2026-08-01; Plan 22-08 waited on an admin ruleset change until 2026-10-19)

### What Was Built

- **SIMD strategy** (Phase 19): `pure-simdjson` pin, MIT/NOTICE posture, upstream loading delegation, `NewSIMDParser()` behind `//go:build simdjson`, hard/soft stop policy.
- **Benchmark dataset foundation** (Phase 20): governed deterministic JSONL fixtures, offline Build/Encode/Query benchmarks, opt-in local external tier.
- **SIMD adapter** (Phase 21): typed scalar routes, exact-int semantics, last-key-wins traversal, default-graph isolation, cleanup failures that poison the builder exactly once.
- **Validation and CI** (Phase 22): parity hard gate, five-seed differential fuzz, drift-checked deployment guide, five-platform required/advisory matrix with native cache and explicit-path replay, dual `govulncheck`, controlled 80-sample benchmark, Main ruleset bound to the two required legs.

### What Worked

- **Parity first, performance second.** Byte-identical index and equal query results were the hard gate; the benchmark was allowed to say "no". It did, and the project kept stdlib as the default without debate.
- **Decide on evidence, not thresholds.** `COUNT=10` controlled captures were authoritative and shared-runner output was trend-only. The defer decision rests on one committed document.
- **Required/advisory CI split.** Two required race legs gate merge; three advisory legs record platform evidence. The windows leg failed once on the PR run and later passed on main without blocking anything.
- **Quick tasks absorbed real production work.** Five releases (v1.1.1 to v1.5.0) shipped from issue-driven quick tasks while the milestone's last plan waited.

### What Was Inefficient

- **Phase 22 stalled 2.5 months on one admin action.** Plan 22-08 needed a ruleset change the agent was not allowed to make. The todo was filed on 2026-08-02 and acted on 2026-10-19.
- **The SIMD performance bet did not pay.** Three phases of adapter work produced a parity-proven path that is slower than stdlib on every fixture. A smaller spike before Phase 21 might have surfaced that earlier, though parity and CI assets remain useful.
- **Phases 23-25 were planned into the milestone but never had a chance.** They added 18 requirements that stayed `Planned` for six months and were deferred at close.
- **Planning state drifted from releases.** STATE.md stopped at 2026-09-14 while v1.3.0, v1.4.0 and v1.5.0 shipped. The planning milestone name `v1.3` now collides with the release tag `v1.3.0`.
- **Toolchain pin masked a green gate.** Running `make security-scan` under `GOTOOLCHAIN=go1.25.5` reported eight already-fixed stdlib advisories; CI on Go 1.26 was clean.

### Patterns Established

- **Qualified parity claim.** "Identical encoded indexes and query results for documents that ingest without a parser-layer error" is the exact wording in tests, docs and evidence; the malformed-attribution asymmetry is the named exclusion.
- **External policy is inspected read-only, mutated under separate authority.** Ruleset changes live in a todo and are verified after the fact.
- **Drift guards for pinned versions.** `check-simd-docs` derives the effective module version via `go list` and fails lint on stale references.
- **Evidence documents have one recommendation class.** `ship | defer | narrow`, no threshold, traceable to raw samples.

### Key Lessons

1. **Name planning milestones so they cannot collide with release tags.** Use a codename or the next real release version.
2. **Keep a milestone to the phases that can start now.** Park follow-on phases in the backlog from day one instead of carrying unstarted requirements to close.
3. **Spike the performance claim before building the adapter.** A one-day paired benchmark on a throwaway adapter would have set expectations for Phases 21-22.
4. **Admin-gated checkpoints need an owner and a date.** A blocking human-action with no deadline stalls the whole phase.
5. **Run gates on the CI toolchain version.** Pin the local toolchain to match CI, not to the go.mod floor.

### Cost Observations

- Model mix: quality profile (opus-class orchestration, sonnet executors and verifiers); no explicit budget tracking
- Sessions: Phase 19-22 execution sessions plus ~25 issue-driven quick tasks between 2026-07-22 and 2026-10-06
- Notable: The controlled benchmark and the five-platform CI matrix were the highest-value artifacts; they settled the SIMD question with evidence rather than opinion.

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Phases | Plans | Key Change |
|-----------|--------|-------|------------|
| v0.1.0 (OSS launch) | 5 | — | Treat planning dir as private (`.planning/` gitignored, force-add for commits) |
| v1.0 (Query & Index Quality) | 7 | 19 | Added evidence reconciliation as a numbered phase; external PR review for doc-only changes |
| v1.2 (Ingest Correctness) | 3 | 12 | Established per-document ingest atomicity, unified failure modes, and structured ingest error reporting |
| v1.3 (SIMD-First Performance) | 4 | 16 | Evidence-first performance work: parity hard gate, controlled COUNT=10 benchmarks, required/advisory CI; outcome was a documented defer |

### Cumulative Quality

| Milestone | Go LOC | Operators | Transformers | Wire version | Test files |
|-----------|--------|-----------|--------------|--------------|-----------|
| v0.1.0 | ~12,000 | 12 | 5 | v8 | ~8 |
| v1.0 | ~25,500 | 12 | 13 + 3 helpers | v9 | ~15 |
| v1.2 | ~36,000 | 12 | 13 + 3 helpers | v9 | ~25 |
| v1.3 | ~47,600 | 12 | 13 + 3 helpers | v11 | 65 |

### Top Lessons (Verified Across Milestones)

1. **Keep planning artifacts versioned locally, force-add to commits.** The `.planning/` gitignore pattern has survived two milestones without friction.
2. **Squash merge with rich PR bodies.** Clean main trunk + durable per-plan history inside GitHub PRs works for both code and docs-only branches.
