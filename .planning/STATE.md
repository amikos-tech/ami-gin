---
gsd_state_version: 1.0
milestone: v1.1
milestone_name: Performance, Observability & Experimentation
status: "Phase 14 shipped - PR #29"
stopped_at: Phase 14 shipped - PR #29
last_updated: "2026-04-22T11:42:12Z"
last_activity: 2026-04-22
last_learnings_extraction:
  phase: 11
  on: 2026-04-22

progress:
  total_phases: 8
  completed_phases: 2
  total_plans: 11
  completed_plans: 7
  percent: 56
---

# Project State

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-04-21)

**Core value:** Material pruning quality and hot-path efficiency gains without turning the library into a heavyweight database or document store
**Current focus:** Phase 15 — experimentation-cli (next)

## Current Position

Phase: 14 (observability-seams) — COMPLETE
Plan: 4/4 plans complete (waves: 1 -> 2 -> 3)
Status: Phase 14 shipped - PR #29
Last activity: 2026-04-22

Progress: [######....] 56% (2/3 phases, 9/17 requirements)

## Performance Metrics

**Velocity (v1.0, shipped):**

- Total plans completed in v1.0: 19
- Milestone duration: 2026-04-14 → 2026-04-21

**By Phase (v1.0):**

| Phase | Plans | Milestone |
|-------|-------|-----------|
| 06 | 2 | v1.0 |
| 07 | 2 | v1.0 |
| 08 | 3 | v1.0 |
| 09 | 3 | v1.0 |
| 10 | 3 | v1.0 |
| 11 | 3 | v1.0 |
| 12 | 3 | v1.0 |

**v1.1 projection:**

| Phase | Plans | Status |
|-------|-------|--------|
| 13 | 3 | Complete |
| 14 | 4 | Shipped (PR #29) |
| 15 | TBD | Not started |

## Accumulated Context

### Decisions

Decisions are logged in `PROJECT.md`.
Key decisions shaping v1.1:

- Parser seam lands as a pure refactor in Phase 13 (zero behavior change, parity harness is the merge gate)
- SIMD parser implementation deferred to v1.2 — upstream blockers: `pure-simdjson` has no LICENSE file, no version tag, shared-library distribution undecided
- Observability adopts the `go-wand` `pkg/logging` + `pkg/telemetry` shape (PRs #114/#115) near-verbatim; `Signals` container carries OTel providers, library never mutates global OTel state
- `adaptiveInvariantLogger *log.Logger` migrates to the new `Logger` interface in the same phase (Phase 14) — no dual-logger convention
- Context-aware API surface is additive (`EvaluateContext`, `BuildFromParquetContext`); existing methods wrap with `context.Background()` — no breaking change per PROJECT.md "avoid gratuitous API churn" constraint
- Experimentation CLI charter: JSONL-in, summary-out — no REPL, no TUI, no color auto-detection

### Roadmap Evolution

- v1.1 milestone opened with 17 requirements (PARSER-01, OBS-01..08, CLI-01..08)
- Phase split derived from research/SUMMARY.md "Proposed Phase Split":
  - Phase 13: Parser Seam Extraction (PARSER-01)
  - Phase 14: Observability Seams (OBS-01..08)
  - Phase 15: Experimentation CLI (CLI-01..08)
- DAG: 13 → {14} → 15. Phase 14 can run in parallel with the tail of Phase 13 once the `Parser` interface merges (per SUMMARY.md). Phase 15 still requires both landed (consumes `--parser` and `--log-level` flags).
- Phase 14 is now planned as 4 executable plans:
  - 14-01 core logging/signals/config surface
  - 14-02 query boundary migration + perf gates
  - 14-03 parquet/build + raw serialization context siblings
  - 14-04 policy and phase-level verification gates
- 100% requirement coverage — no orphans

### Pending Todos

- Plan and execute Phase 15 (Experimentation CLI, CLI-01..08)
- Watch for v1.2 planning when SIMD upstream blockers are resolved (`pure-simdjson` LICENSE + tag + distribution decision)

### Blockers/Concerns

- Phase 13's benchmark noise was accepted as residual risk in `.planning/phases/13-parser-seam-extraction/13-SECURITY.md`; preserve that evidence if a future performance follow-up revisits the transformer-heavy explicit-number path.
- The pure-simdjson blockers (LICENSE, tag, distribution mechanism) remain deferred to v1.2 and do not gate Phases 13-15.

### Quick Tasks Completed (v1.0 era — retained for reference)

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260417-pvi | Phase-10 review follow-ups: T1 second-entry PrefixLen test, T2 table-driven path-directory truncation test, wrap bare io.EOF leaks in 8 serialize readers, PrefixBlockSize > MaxUint16 guard | 2026-04-17 | 8eb78f5 | [260417-pvi-phase-10-review-follow-ups-t1-subsequent](./quick/260417-pvi-phase-10-review-follow-ups-t1-subsequent/) |
| 260417-tnm | PR #23 review fixes: unexport readCompressedTerms, drop zero-value PrefixCompressor + redundant count check in ordered-string decode, short-circuit writeOrderedStrings for trivial inputs, add WithPrefixBlockSize ConfigOption, document compact-path corruption byte layout | 2026-04-17 | c28957f | [260417-tnm-address-pr-23-feedback-unexport-readcomp](./quick/260417-tnm-address-pr-23-feedback-unexport-readcomp/) |
| 260420-h1a | Unexport WriteCompressedTerms to writeCompressedTerms — PR #23 review feedback item 2; removes unused public API surface now that ReadCompressedTerms counterpart is gone | 2026-04-20 | 1e8746d | [260420-h1a-unexport-writecompressedterms-to-writeco](./quick/260420-h1a-unexport-writecompressedterms-to-writeco/) |

## Deferred Items

Items deferred to v1.2 or later:

| Category | Item | Status | Note |
|----------|------|--------|------|
| requirement | pure-simdjson parser implementation | v1.2 | Blocked on upstream LICENSE file, version tag, shared-library distribution decision |
| requirement | SIMD parser benchmarks vs stdlib | v1.2 | Depends on SIMD implementation |
| requirement | CI matrix for `-tags simdjson` builds | v1.2 | Depends on SIMD implementation |
| seed | SEED-001-simdjson-test-datasets | v1.2 | Activate alongside SIMD parser implementation |
| requirement | `zap` logger adapter | on-demand | Ship `slog`/`stdlib` only in v1.1; add `zap` on explicit user request |
| feature | Two-file index diff (CLI) | on-demand | Low value vs. complexity; wait for user signal |
| feature | Experimentation CLI REPL/TUI | out-of-scope | Charter excludes interactive modes |

Items acknowledged and deferred at v1.0 milestone close (retained for audit):

| Category | Item | Status | Note |
|----------|------|--------|------|
| quick_task | 260417-pvi-phase-10-review-follow-ups-t1-subsequent | resolved | Shipped in commit 8eb78f5; audit tool flag was false positive |
| quick_task | 260417-tnm-address-pr-23-feedback-unexport-readcomp | resolved | Shipped in commit c28957f; audit tool flag was false positive |
| quick_task | 260420-h1a-unexport-writecompressedterms-to-writeco | resolved | Shipped in commit 1e8746d; audit tool flag was false positive |

## Session Continuity

Last session: Phase 14 shipped
Stopped at: Phase 14 shipped - PR #29 opened

**Next step:** Plan Phase 15 (Experimentation CLI). `/gsd-discuss-phase 15` to discuss, then `/gsd-plan-phase 15`.

**v1.1 projection update:**

| Phase | Plans | Status |
|-------|-------|--------|
| 13 | 3 | Complete |
| 14 | 4 | Complete 2026-04-22 |
| 15 | TBD | Not started |
