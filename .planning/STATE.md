---
gsd_state_version: 1.0
milestone: v1.2
milestone_name: Ingest Correctness & Per-Document Isolation
status: executing
stopped_at: Phase 17 Wave 1 executing
last_updated: "2026-04-23T16:01:54.000Z"
last_activity: 2026-04-23
progress:
  total_phases: 15
  completed_phases: 8
  total_plans: 28
  completed_plans: 23
  percent: 82
---

# Project State

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-04-23)

**Core value:** Material pruning quality and hot-path efficiency gains without turning the library into a heavyweight database or document store
**Current focus:** Phase 17 — failure-mode-taxonomy-unification

## Current Position

Phase: 17
Plan: 17-01 executing
Status: Phase 17 execution in progress; 4 plans across 3 waves
Last activity: 2026-04-23

Progress: [----------] 0% for Phase 17 (0/4 plans executed; 4/4 plans planned and checker-verified)

## Performance Metrics

**Velocity (v1.0, shipped):**

- Total plans completed in v1.0: 19
- Milestone duration: 2026-04-14 → 2026-04-21

**Velocity (v1.1, shipped):**

- Total plans completed in v1.1: 10
- Milestone duration: 2026-04-21 → 2026-04-22

**By Phase (v1.1):**

| Phase | Plans | Status |
|-------|-------|--------|
| 13 | 3 | Complete (PR #29) |
| 14 | 4 | Complete (PR #29) |
| 15 | 3 | Complete (PR #30) |

**v1.2 projection:**

| Phase | Plans | Status |
|-------|-------|--------|
| 16 | 4 | Complete (4/4 plans complete) |
| 17 | 4 | Ready to execute (0/4 plans complete) |
| 18 | TBD | Planned (defining) |

## Accumulated Context

### Decisions

Decisions are logged in `PROJECT.md`.
Key decisions shaping v1.2 (from brainstorming, 2026-04-23):

- **Atomicity strategy = C (validate-before-mutate)** with Lucene's per-document contract as the target. `mergeStagedPaths` becomes infallible; `validateStagedPaths` extended to cover every merge failure mode against the real `pathData` state.
- **`poisonErr` → `tragicErr`** (renamed and narrowed). Reserved for internal-invariant violations; user-input failures are isolated per-document.
- **`recover()` in merge belt-and-suspenders** confirmed by user — safety net even after panic audit. Trade-off: extra defense vs marginal complexity.
- **`TransformerFailureMode` rename to `IngestFailureMode`** confirmed as deliberate breaking change ("clarity over convenience"). Requires CHANGELOG flag.
- **`IngestError.Value` not redacted by library** — callers redact themselves ("cleaner DX with less surprises").
- **Industry-precedent grounded**: Lucene IndexWriter's per-document isolation contract is the target. Tantivy/Bleve/PostgreSQL GIN/RocksDB also reviewed; Lucene is the closest analog.
- **Scope explicitly tight (Shape 1 from brainstorming)**: 3 phases — atomicity, failure-mode taxonomy, structured IngestError + CLI. No `ValidateDocument` dry-run, no snapshot/restore, no perf items.
- **Numbering = Option B**: v1.2 takes phases 16–18 (chronological); SIMD renumbered to v1.3 phases 19–20.
- **16-01 pre-check result**: `validateStagedPaths` already covered both lossy mixed numeric promotion directions before merge signature edits; `builder.go` validator logic was left unchanged.
- **16-01 test isolation**: focused validator tests seed staged numeric observations directly because `stageJSONNumberLiteral` already rejects these lossy promotions before `validateStagedPaths` can be isolated.
- **16-02 tragic recovery**: `runMergeWithRecover` wraps only `mergeStagedPaths`; recovered merge panics set `tragicErr`, log through the logger seam with `error.type` and `panic_type`, and skip document bookkeeping.
- **16-04 marker enforcement**: local and CI marker checks now enforce the merge-layer validator marker policy; the Wave 2 integration fix resolved the `gin_test.go` goconst finding and `make lint` is green.
- **16-03 atomicity proof**: `atomicity_test.go` uses a bounded 1000-document full-vs-clean property with deterministic 10% failing slots and encoded-byte equality; the public failure catalog asserts user-input failures leave `tragicErr` nil.
- **17 planning resolution**: public `IngestFailureMode` string values are planned as `hard` and `soft`, while transformer serialization preserves legacy v9 wire tokens `strict` and `soft_fail` through private mapping.
- **17 test organization**: Phase 17 plans use a focused `failure_modes_test.go` for cross-layer hard/soft semantics, targeted serialization tests in `serialize_security_test.go`, and a rewrite of the obsolete transformer soft expectation in `transformers_test.go`.

### Roadmap Evolution

- v1.1 functionally complete 2026-04-22 (PRs #29 and #30); milestone not formally closed via `/gsd-complete-milestone` but advanced into v1.2.
- v1.2 milestone opened 2026-04-23 with 8 requirements (ATOMIC-01..03, FAIL-01..02, IERR-01..03).
- Phase split derived from brainstorming session:
  - Phase 16: AddDocument Atomicity (Lucene contract) — ATOMIC-01..03
  - Phase 17: Failure-Mode Taxonomy Unification — FAIL-01..02
  - Phase 18: Structured IngestError + CLI integration — IERR-01..03
- DAG: 16 → 17 → 18 (strict sequence; Phases 17 and 18 only become possible because Phase 16 makes per-document failure first-class).
- Phase 16 completed 2026-04-23 with ATOMIC-01, ATOMIC-02, and ATOMIC-03 fully covered.
- Phase 17 planned 2026-04-23 with 4 plans across 3 waves; plan checker passed after resolving research open questions.
- v1.3 (was v1.2) SIMD work renumbered: Phases 16/17 → 19/20. Same scope, blocked on the same upstream items.
- 100% requirement coverage — no orphans

### Pending Todos

- Execute Phase 17 failure-mode taxonomy unification
- Add new 999.x backlog entries for the perf items considered and deferred during v1.2 brainstorming (bloom AddString allocation cleanup; per-path `[*]` opt-out)

### Blockers/Concerns

- The validator becoming the single point of truth for "what can fail" introduces an invariant that future contributors must respect. Mitigation is captured in Phase 16 plans: `// MUST_BE_CHECKED_BY_VALIDATOR` markers plus local and CI checks for merge-layer error returns.
- Phase 16 integration gate is green: `make test`, `make lint`, and `go build ./...` passed after all four plans.
- `bloom.AddString`, `hll.AddString`, `trigram.Add`, and `RGSet.Set` are presumed infallible — explicit audit is included in 16-01.
- v1.3 SIMD blockers (`pure-simdjson` LICENSE / tag / distribution) remain unresolved and do not gate v1.2.

### Quick Tasks Completed (v1.1, retained for reference)

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260422-lv1 | PR #29 review feedback: predicate_op attr fix, telemetry.ErrorTypeOther promotion + go mod tidy, allocs test rename, parser-name info-leak test wiring | 2026-04-22 | 8679f64 | [260422-lv1-...](./quick/260422-lv1-address-pr-29-feedback-fix-attr-vocab-ti/) |
| 260422-ur4 | PR #30 review feedback: json.Valid abort validator, trim mutation contract doc, name-based $.status lookup, shared typeNames helper | 2026-04-22 | 231275d | [260422-ur4-...](./quick/260422-ur4-address-pr-30-feedback-items-1-4-replace/) |

## Deferred Items

Items deferred to v1.3 or later:

| Category | Item | Status | Note |
|----------|------|--------|------|
| requirement | pure-simdjson parser implementation | v1.3 (was v1.2) | Blocked on upstream LICENSE file, version tag, shared-library distribution decision |
| requirement | SIMD parser benchmarks vs stdlib | v1.3 | Depends on SIMD implementation |
| requirement | CI matrix for `-tags simdjson` builds | v1.3 | Depends on SIMD implementation |
| seed | SEED-001-simdjson-test-datasets | v1.3 | Activate alongside SIMD parser implementation |
| feature | `ValidateDocument` dry-run API | future | Architectural prerequisite (Phase 16 atomicity) lands in v1.2; landing the API is a separate milestone with a real consumer |
| feature | Snapshot-and-restore atomicity (Strategy A) | reserve | Held in case a future failure mode cannot be pre-validated |
| feature | Bloom `AddString` allocation cleanup | 999.x | Perf-shaped; profile before optimizing per project precedent |
| feature | Per-path `[*]` array wildcard opt-out | 999.x | Disconnected from correctness theme |
| feature | `zap` logger adapter | on-demand | Ship `slog`/`stdlib` only; add `zap` on explicit user request |
| feature | Two-file index diff (CLI) | on-demand | Low value vs. complexity; wait for user signal |
| feature | Experimentation CLI REPL/TUI | out-of-scope | Charter excludes interactive modes |

## Session Continuity

Last session: 2026-04-23T15:02:00.000Z
Stopped at: Phase 17 planned
Resume file: .planning/phases/17-failure-mode-taxonomy-unification/17-01-PLAN.md

**Next step:** Execute Phase 17 failure-mode taxonomy unification.
