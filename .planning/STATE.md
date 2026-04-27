---
gsd_state_version: 1.0
milestone: v1.3
milestone_name: SIMD-First Performance
status: planning
stopped_at: Phase 19 SIMD dependency decision ready for discussion
last_updated: "2026-04-27T00:00:00Z"
last_activity: "2026-04-27 - Reprioritized v1.3 milestone to SIMD-first"
progress:
  total_phases: 7
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
  percent: 0
---

# Project State

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-04-27)

**Core value:** Material pruning quality and hot-path efficiency gains without turning the library into a heavyweight database or document store
**Current focus:** v1.3 SIMD-First Performance

## Current Position

Phase: 19
Plan: Not started
Status: Ready to discuss Phase 19 SIMD dependency decision
Last activity: 2026-04-27 - v1.3 milestone reprioritized to SIMD-first

Progress: [----------] 0% for v1.3 (0/7 phases complete)

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
| 17 | 4 | Complete (4/4 plans complete) |
| 18 | 4 | Complete (4/4 plans complete) |

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
- **Numbering = Option B**: v1.2 took phases 16–18 (chronological); SIMD is now first-class v1.3 scope starting at Phase 19.
- **16-01 pre-check result**: `validateStagedPaths` already covered both lossy mixed numeric promotion directions before merge signature edits; `builder.go` validator logic was left unchanged.
- **16-01 test isolation**: focused validator tests seed staged numeric observations directly because `stageJSONNumberLiteral` already rejects these lossy promotions before `validateStagedPaths` can be isolated.
- **16-02 tragic recovery**: `runMergeWithRecover` wraps only `mergeStagedPaths`; recovered merge panics set `tragicErr`, log through the logger seam with `error.type` and `panic_type`, and skip document bookkeeping.
- **16-04 marker enforcement**: local and CI marker checks now enforce the merge-layer validator marker policy; the Wave 2 integration fix resolved the `gin_test.go` goconst finding and `make lint` is green.
- **16-03 atomicity proof**: `atomicity_test.go` uses a bounded 1000-document full-vs-clean property with deterministic 10% failing slots and encoded-byte equality; the public failure catalog asserts user-input failures leave `tragicErr` nil.
- **17 planning resolution**: public `IngestFailureMode` string values are planned as `hard` and `soft`, while transformer serialization preserves legacy v9 wire tokens `strict` and `soft_fail` through private mapping.
- **17 test organization**: Phase 17 plans use a focused `failure_modes_test.go` for cross-layer hard/soft semantics, targeted serialization tests in `serialize_security_test.go`, and a rewrite of the obsolete transformer soft expectation in `transformers_test.go`.
- **17 completion**: Phase 17 verified 15/15 must-haves on 2026-04-23. Public `IngestFailureMode` API, parser/numeric config knobs, whole-document soft skips, v9 transformer wire-token compatibility, changelog note, and deterministic failure-modes example are complete.
- **18-01 completion**: Public `IngestLayer`/`IngestError` API and builder hard-failure wrapping are complete. Parser, transformer, numeric, schema, and validator-replayed numeric failures are extractable with `errors.As`; parser contract, tragic/internal, and soft-mode paths stay non-`IngestError`.
- **18-02 completion**: The hard ingest behavior matrix now asserts extraction through an outer `errors.Wrap`, builder usability after public hard failures, and explicit non-`IngestError` exceptions. A focused stdlib AST guard protects named hard-ingest functions against direct plain error returns.
- **18-03 completion**: `gin-index experiment --on-error continue` now reports grouped structured `IngestError` failures in text and JSON summaries. Failure groups are deterministic (`parser`, `transformer`, `numeric`, `schema`, then lexical unknowns), samples are capped at 3 per layer, and the 100-line fixture asserts 3 parser, 4 transformer, and 3 numeric failures with 90 accepted documents / 9 row groups.
- **18-04 completion**: Public API docs and CHANGELOG now state `IngestError.Value` is verbatim, not redacted, and not truncated by the library; `18-VALIDATION.md` records green focused Phase 18 tests, full `go test ./...`, and `make lint`.
- **18 verification**: Phase 18 verification passed 16/16 must-haves on 2026-04-24. Advisory code review is clean with 0 findings, focused root/CLI tests passed, full `go test ./...` passed, and `make lint` passed.

### Roadmap Evolution

- v1.1 functionally complete 2026-04-22 (PRs #29 and #30); milestone not formally closed via `/gsd-complete-milestone` but advanced into v1.2.
- v1.2 milestone opened 2026-04-23 with 8 requirements (ATOMIC-01..03, FAIL-01..02, IERR-01..03).
- Phase split derived from brainstorming session:
  - Phase 16: AddDocument Atomicity (Lucene contract) — ATOMIC-01..03
  - Phase 17: Failure-Mode Taxonomy Unification — FAIL-01..02
  - Phase 18: Structured IngestError + CLI integration — IERR-01..03
- DAG: 16 → 17 → 18 (strict sequence; Phases 17 and 18 only become possible because Phase 16 makes per-document failure first-class).
- Phase 16 completed 2026-04-23 with ATOMIC-01, ATOMIC-02, and ATOMIC-03 fully covered.
- Phase 17 completed 2026-04-23 with 4/4 plans complete, verification passed, and FAIL-01/FAIL-02 satisfied.
- 100% requirement coverage — no orphans
- v1.3 milestone planned 2026-04-27 from backlog and SEED-001, then reprioritized SIMD-first:
  - Phase 19: SIMD Dependency Decision & Integration Strategy — SIMD-01..03
  - Phase 20: Realistic Benchmark Dataset Foundation — DATA-01..03
  - Phase 21: SIMD Parser Adapter — SIMD-04..07
  - Phase 22: SIMD Validation, Benchmarks & CI — SIMD-08..11
  - Phase 23: Row-Level Pruning Positioning — POS-01..02
  - Phase 24: Developer Quality Gates & Janitorial Clarity — QG-01, CLAR-01
  - Phase 25: Follow-On Profiling & Measurement-Backed Optimizations — PROF-01..05, ENC-01..03, ING-01..03

### Pending Todos

- Discuss and plan Phase 19: SIMD Dependency Decision & Integration Strategy.

### Blockers/Concerns

- The validator becoming the single point of truth for "what can fail" introduces an invariant that future contributors must respect. Mitigation is captured in Phase 16 plans: `// MUST_BE_CHECKED_BY_VALIDATOR` markers plus local and CI checks for merge-layer error returns.
- Phase 16 integration gate is green: `make test`, `make lint`, and `go build ./...` passed after all four plans.
- Phase 17 integration gate is green: `go test ./...`, `make test`, `make lint`, and `go build ./...` passed. Advisory review warnings are documented in `17-REVIEW.md` and residual risks in `17-VERIFICATION.md`.
- `bloom.AddString`, `hll.AddString`, `trigram.Add`, and `RGSet.Set` are presumed infallible — explicit audit is included in 16-01.
- v1.3 SIMD blockers (`pure-simdjson` LICENSE / tag / distribution) are now the first thing to resolve in Phase 19.

### Quick Tasks Completed (v1.1, retained for reference)

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260422-lv1 | PR #29 review feedback: predicate_op attr fix, telemetry.ErrorTypeOther promotion + go mod tidy, allocs test rename, parser-name info-leak test wiring | 2026-04-22 | 8679f64 | [260422-lv1-...](./quick/260422-lv1-address-pr-29-feedback-fix-attr-vocab-ti/) |
| 260422-ur4 | PR #30 review feedback: json.Valid abort validator, trim mutation contract doc, name-based $.status lookup, shared typeNames helper | 2026-04-22 | 231275d | [260422-ur4-...](./quick/260422-ur4-address-pr-30-feedback-items-1-4-replace/) |

### Quick Tasks Completed (v1.2)

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260424 | Phase 17 review feedback: Finalize nil caller guards, explicit soft-skip kinds/counters, nil Encode handling, and ErrInvalidFormat coverage | 2026-04-24 | this commit | [260424-address-review-feedback-finalize-soft-skips](./quick/260424-address-review-feedback-finalize-soft-skips/) |
| 260424b | Phase 17 follow-up review: ErrNilIndex sentinel, builder Err propagation, soft-skip cleanup, and guard coverage | 2026-04-24 | this commit | [260424-follow-up-review-finalize-sentinels](./quick/260424-follow-up-review-finalize-sentinels/) |
| 260424c | Phase 17 janitorial review suggestions: inline experiment finalize trampoline and document soft-skip fallback invariant | 2026-04-24 | this commit | [260424-janitorial-review-suggestions](./quick/260424-janitorial-review-suggestions/) |
| 260424d | PR #32 review feedback items 1-5: drop redundant normalize calls, expand CHANGELOG, document parseFloat decimal-only, remove dead-code Group guard, document soft-skip parser remap invariant | 2026-04-24 | 9807748 | [260424-address-pr32-feedback-items-1-5](./quick/260424-address-pr32-feedback-items-1-5/) |
| 260424e | Phase 18 review follow-ups: source-path remap, unknown CLI failures, tragic continue abort, unexported IngestError accessors, schema/sample-cap/guard coverage | 2026-04-24 | this commit | [260424e-phase18-review-followups](./quick/260424e-phase18-review-followups/) |
| 260424f | Phase 18 re-review follow-ups: structured tragic status/reporting, end-to-end tragic CLI coverage, changelog/security refresh, helper docs, and nil-safe accessor tests | 2026-04-24 | this commit | [260424f-phase18-rereview-followups](./quick/260424f-phase18-rereview-followups/) |
| 260424g | Phase 18 cycle 3 follow-ups: tragic sample-cap bypass, stronger tragic CLI assertions, text tragic coverage, SECURITY accessor/citation refresh, and remap godoc clarification | 2026-04-24 | this commit | [260424g-phase18-cycle3-followups](./quick/260424g-phase18-cycle3-followups/) |
| 260424h | Phase 18 nit follow-ups: tragic wrap ordering comment, explicit Unwrap policy test, stronger tragic text-mode info count, and companion remap godoc clarification | 2026-04-24 | 72eea02 | [260424h-phase18-nit-followups](./quick/260424h-phase18-nit-followups/) |
| 260425a | PR #33 follow-ups items 4 and 5: tighten +hard-ingest guard to require canonical standalone form and mirror no-parallel safety comment on withExperimentDefaultConfig | 2026-04-25 | 54b7e34 | [260425a-pr33-followups-4-5](./quick/260425a-pr33-followups-4-5/) |

## Deferred Items

Items deferred to current or later milestones:

| Category | Item | Status | Note |
|----------|------|--------|------|
| feature | `ValidateDocument` dry-run API | future | Architectural prerequisite (Phase 16 atomicity) landed in v1.2; landing the API is a separate milestone with a real consumer |
| feature | Snapshot-and-restore atomicity (Strategy A) | reserve | Held in case a future failure mode cannot be pre-validated |
| feature | Bloom `AddString` allocation cleanup | 999.x | Perf-shaped; profile before optimizing per project precedent |
| feature | Per-path `[*]` array wildcard opt-out | 999.x | Disconnected from correctness theme |
| bug | `RegexExtractInt` registry `parseFloat` round-trip divergence | 999.x | Reconstructed transformer rejects scientific notation while public API (`transformers.go:134`) accepts it; fix is to swap `parseFloat` for `strconv.ParseFloat` in `transformer_registry.go` with `NaN`/`Inf` guard and add a serialize→reload round-trip test. Surfaced during PR #32 review follow-up (quick task 260424d). |
| feature | `zap` logger adapter | on-demand | Ship `slog`/`stdlib` only; add `zap` on explicit user request |
| feature | Two-file index diff (CLI) | on-demand | Low value vs. complexity; wait for user signal |
| feature | Experimentation CLI REPL/TUI | out-of-scope | Charter excludes interactive modes |

## Session Continuity

Last session: 2026-04-24T14:46:00Z
Stopped at: Phase 18 verified
Resume file: .planning/phases/18-structured-ingesterror-cli-integration/18-VERIFICATION.md

**Next step:** `$gsd-discuss-phase 19` to resolve SIMD dependency and integration strategy, or `$gsd-plan-phase 19` to plan directly.
