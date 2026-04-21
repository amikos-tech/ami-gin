# Roadmap: GIN Index

## Milestones

- ✅ **v0.1.0 OSS Launch** — Phases 01-05 (shipped pre-v1.0)
- ✅ **v1.0 Query & Index Quality** — Phases 06-12 (shipped 2026-04-21) — see [`milestones/v1.0-ROADMAP.md`](./milestones/v1.0-ROADMAP.md)
- 📋 **v1.1** — To be planned via `/gsd-new-milestone`

## Phases

<details>
<summary>✅ v1.0 Query & Index Quality (Phases 06-12) — SHIPPED 2026-04-21</summary>

- [x] Phase 06: Query Path Hot Path (2/2 plans) — completed 2026-04-14
- [x] Phase 07: Builder Parsing & Numeric Fidelity (2/2 plans) — completed 2026-04-15
- [x] Phase 08: Adaptive High-Cardinality Indexing (3/3 plans) — completed 2026-04-15
- [x] Phase 09: Derived Representations (3/3 plans) — completed 2026-04-16
- [x] Phase 10: Serialization Compaction (3/3 plans) — completed 2026-04-17
- [x] Phase 11: Real-Corpus Prefix Compression Benchmarking (3/3 plans) — completed 2026-04-20
- [x] Phase 12: Milestone Evidence Reconciliation (3/3 plans) — completed 2026-04-21

Full details: [`milestones/v1.0-ROADMAP.md`](./milestones/v1.0-ROADMAP.md)

</details>

### 📋 v1.1 (To Be Defined)

No phases yet. Run `/gsd-new-milestone` to seed the next milestone from requirements and research.

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 06. Query Path Hot Path | v1.0 | 2/2 | Complete | 2026-04-14 |
| 07. Builder Parsing & Numeric Fidelity | v1.0 | 2/2 | Complete | 2026-04-15 |
| 08. Adaptive High-Cardinality Indexing | v1.0 | 3/3 | Complete | 2026-04-15 |
| 09. Derived Representations | v1.0 | 3/3 | Complete | 2026-04-16 |
| 10. Serialization Compaction | v1.0 | 3/3 | Complete | 2026-04-17 |
| 11. Real-Corpus Prefix Compression Benchmarking | v1.0 | 3/3 | Complete | 2026-04-20 |
| 12. Milestone Evidence Reconciliation | v1.0 | 3/3 | Complete | 2026-04-21 |

---
*v1.0 shipped 2026-04-21. Prior milestone v0.1.0 completed the OSS launch (phases 01-05).*

## Backlog

### Phase 999.1: Lefthook Pre-Push Quality Gates (BACKLOG)

**Goal:** Add `lefthook`-based pre-push quality gates, modeled on `/Users/tazarov/experiments/telia/tclr/tclr-v2/lefthook.yml`, to block pushes when required local validation fails or required tools are missing. Scope the future implementation around this repo's native checks such as `make lint` and `make test`, with room for selective changed-package execution if that keeps hook latency reasonable.
**Requirements:** TBD
**Plans:** 0 plans

Plans:
- [ ] TBD (promote with /gsd-review-backlog when ready)

### Phase 999.2: walkJSON NormalizePath Fast-Path (BACKLOG)

**Goal:** Add a fast-path in `walkJSON` to skip `NormalizePath` when the path is already in canonical form (builder-generated paths never contain bracket-quoted fields). This avoids `jp.ParseString` overhead on every recursive call during ingestion.
**Requirements:** Profile ingestion to confirm `NormalizePath` is a measurable hotspot before implementing.
**Plans:** 0 plans

### Phase 999.3: Minor Code Clarity in Phase 06 (BACKLOG)

**Goal:** Address non-blocking observations from Phase 06 review: (a) add comment on `findPath` bounds check explaining it guards against corruption; (b) reorder or comment `validatePathReferences` to clarify it reads the original directory; (c) make benchmark fixture path count assertion less brittle.
**Requirements:** None — cosmetic improvements only.
**Plans:** 0 plans

### Phase 999.4: WithEncodeStrategy Config Option (BACKLOG)

**Goal:** Expose `WithEncodeStrategy(Auto|RawOnly|FrontCodedOnly)` ConfigOption for ordered string sections so callers can declare data shape per-index and skip the dual-encode pass on known-random paths (UUIDs, hashes). Implementation sketch: new `EncodeStrategy` uint8 iota, field on `GINConfig`, option in `gin.go`, threaded through to `writeOrderedStrings` in `serialize.go:440`. Follows RocksDB's `block_restart_interval` precedent — industry-standard "knob, not heuristic" pattern confirmed by 2026-04-20 research sweep across Lucene BlockTree, Tantivy SSTable, PostgreSQL GIN, RocksDB/LevelDB, Roaring Bitmaps, Bleve/Vellum, Badger, and Lasch VLDB 2020. Zero libraries use upfront sampling to choose front-coded vs raw.
**Requirements:** Blocked on profiling data from Phase 999.5 justifying the API surface. Originates from PR #23 review feedback (non-blocking).
**Plans:** 0 plans

Plans:
- [ ] TBD (promote with /gsd-review-backlog when ready)

### Phase 999.5: Profile Encode CPU on Real Workloads (BACKLOG)

**Goal:** Measure encode CPU and allocations for `writeOrderedStrings` on representative workloads (UUID-heavy paths, log-style timestamp paths, mixed JSON corpora) before committing to the Phase 999.4 API surface. Per Roaring Bitmaps' measure-first philosophy and the PR #23 reviewer's own framing — "if encode performance ever shows up in profiling" — we want profiling data to justify whether the dual-encode cost in `serialize.go:456-474` is a real bottleneck or a speculative optimization. Deliverable: a benchmark + flamegraph report showing encode CPU share on realistic fixtures, decision record on whether 999.4 is worth shipping.
**Requirements:** TBD — define workload fixtures (UUID, timestamp, mixed) during planning.
**Plans:** 0 plans

Plans:
- [ ] TBD (promote with /gsd-review-backlog when ready)
