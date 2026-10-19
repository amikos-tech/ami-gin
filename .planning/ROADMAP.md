# Roadmap: GIN Index

## Milestones

- ✅ **v0.1.0 OSS Launch** — Phases 01-05 (shipped pre-v1.0)
- ✅ **v1.0 Query & Index Quality** — Phases 06-12 (shipped 2026-04-21) — see [`milestones/v1.0-ROADMAP.md`](./milestones/v1.0-ROADMAP.md)
- ✅ **v1.1 Performance, Observability & Experimentation** — Phases 13-15 (functionally complete 2026-04-22; PRs #29 and #30 merged)
- ✅ **v1.2 Ingest Correctness & Per-Document Isolation** — Phases 16-18 (shipped 2026-04-27) — see [`milestones/v1.2-ROADMAP.md`](./milestones/v1.2-ROADMAP.md)
- ✅ **v1.3 SIMD-First Performance** — Phases 19-22 (shipped 2026-10-19; Phases 23-25 deferred to backlog) — see [`milestones/v1.3-ROADMAP.md`](./milestones/v1.3-ROADMAP.md)
- 📋 **Next milestone** — not yet defined. Candidates from the GitHub backlog: regex pruning precision (#93, #94, #95), zstd decoder heap measurement (#87), Parquet/S3 nested module split (#84, needs a v2 or deprecation decision), sparse n-gram investigation (#92).

## Phases

<details>
<summary>✅ v1.0 Query & Index Quality (Phases 06-12) — SHIPPED 2026-04-21</summary>

Full details: [`milestones/v1.0-ROADMAP.md`](./milestones/v1.0-ROADMAP.md)

</details>

<details>
<summary>✅ v1.1 Performance, Observability & Experimentation (Phases 13-15) — FUNCTIONALLY COMPLETE 2026-04-22</summary>

- [x] **Phase 13: Parser Seam Extraction** — Pluggable `Parser` interface and `stdlibParser` default.
- [x] **Phase 14: Observability Seams** — Logger, Telemetry, Signals, adapters, and context-aware APIs.
- [x] **Phase 15: Experimentation CLI** — JSONL experiment command with summaries, predicate tests, JSON mode, sidecar output, and error-tolerant modes.

</details>

<details>
<summary>✅ v1.2 Ingest Correctness & Per-Document Isolation (Phases 16-18) — SHIPPED 2026-04-27</summary>

- [x] **Phase 16: AddDocument Atomicity (Lucene contract)** — Validator-backed infallible merge, `tragicErr`, recovery, and atomicity property tests.
- [x] **Phase 17: Failure-Mode Taxonomy Unification** — Unified `IngestFailureMode` across parser, transformer, and numeric layers.
- [x] **Phase 18: Structured `IngestError` + CLI integration** — Structured per-document errors and grouped CLI failure summaries.

Full details: [`milestones/v1.2-ROADMAP.md`](./milestones/v1.2-ROADMAP.md)

</details>

<details>
<summary>✅ v1.3 SIMD-First Performance (Phases 19-22) — SHIPPED 2026-10-19</summary>

- [x] **Phase 19: SIMD Dependency Decision & Integration Strategy** — `pure-simdjson` pin, license/NOTICE posture, opt-in API shape, CI expectations, stop policy. (completed 2026-04-27)
- [x] **Phase 20: Realistic Benchmark Dataset Foundation** — SEED-001 activated as governed offline JSONL fixtures and benchmarks. (completed 2026-07-21)
- [x] **Phase 21: SIMD Parser Adapter** — Opt-in build-tagged SIMD parser behind the Phase 13 seam with exact numeric semantics. (completed 2026-07-23)
- [x] **Phase 22: SIMD Validation, Benchmarks & CI** — Parity hard gate, differential fuzz, five-platform CI, controlled benchmarks; recommendation **defer**. (completed 2026-10-19)

Phases 23-25 were planned but never started; see Backlog. Full details: [`milestones/v1.3-ROADMAP.md`](./milestones/v1.3-ROADMAP.md)

</details>

### 📋 Next milestone (not yet defined)

Run `/gsd:new-milestone` to define it. Suggested order from the 2026-10-19 backlog review: #93 → #94 → #95 (regex pruning precision), #87 (measurement only), then decide #84, then #92.

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
| 13. Parser Seam Extraction | v1.1 | 3/3 | Complete | 2026-04-21 |
| 14. Observability Seams | v1.1 | 4/4 | Complete | 2026-04-22 |
| 15. Experimentation CLI | v1.1 | 3/3 | Complete | 2026-04-22 |
| 16. AddDocument Atomicity (Lucene contract) | v1.2 | 4/4 | Complete | 2026-04-23 |
| 17. Failure-Mode Taxonomy Unification | v1.2 | 4/4 | Complete | 2026-04-23 |
| 18. Structured IngestError + CLI integration | v1.2 | 4/4 | Complete | 2026-04-24 |
| 19. SIMD Dependency Decision & Integration Strategy | v1.3 | 1/1 | Complete | 2026-04-27 |
| 20. Realistic Benchmark Dataset Foundation | v1.3 | 2/2 | Complete | 2026-07-21 |
| 21. SIMD Parser Adapter | v1.3 | 5/5 | Complete | 2026-07-23 |
| 22. SIMD Validation, Benchmarks & CI | v1.3 | 8/8 | Complete | 2026-10-19 |

## Backlog

### Phase 999.8: Row-Level Pruning Positioning (BACKLOG, deferred from v1.3 Phase 23)

**Goal:** Make it clear that GIN Index supports both grouped pruning and row-level pruning when callers choose one document per row group, without renaming the API or implying row-level storage/search semantics.
**Requirements:** POS-01, POS-02 (see [`milestones/v1.3-REQUIREMENTS.md`](./milestones/v1.3-REQUIREMENTS.md))
**Plans:** 0 plans

**Success Criteria:**

1. README/product copy explains row groups as a caller-chosen granularity and states that `rg=1` enables row-level pruning.
2. CLI/docs terminology consistently describes grouped and single-row-group pruning without suggesting full row-level document storage.
3. At least one example or experiment note demonstrates the row-level mental model using existing APIs.

### Phase 999.9: Developer Quality Gates & Janitorial Clarity (BACKLOG, deferred from v1.3 Phase 24)

**Goal:** Improve contributor safety with local pre-push checks and close low-risk clarity issues that reduce future maintenance friction.
**Requirements:** QG-01, CLAR-01
**Plans:** 0 plans

**Success Criteria:**

1. A lightweight pre-push quality gate runs native repo checks such as `make lint` and `make test`, with clear missing-tool behavior.
2. The gate is documented and does not require contributors to use a specific shell beyond documented prerequisites.
3. Phase 06 clarity backlog is closed with comments or small test cleanups only; no behavior changes.

**Note:** The Phase 22 code review (`22-REVIEW.md`, WR-01) found that golangci-lint never runs with `-tags simdjson`; fold a tagged lint pass into this phase.

### Phase 999.10: Follow-On Profiling & Measurement-Backed Optimizations (BACKLOG, deferred from v1.3 Phase 25)

**Goal:** Evaluate the remaining performance backlog and implement only improvements justified by measurements.
**Requirements:** PROF-01..05, ENC-01..03, ING-01..03
**Plans:** 0 plans

**Context:** ENC-01..03 (`WithEncodeStrategy`) are partly superseded by the `EncoderProfile` options shipped in v1.3.0 and v1.4.0 (#79, #83); re-evaluate before planning. ING-01 (`NormalizePath` fast path), ING-02 (bloom allocation cleanup) and ING-03 (wildcard opt-out) remain profiling-gated per project precedent. Issue #87 (zstd decoder retained heap) belongs here.

**Success Criteria:**

1. Profiling quantifies encode CPU, `NormalizePath`, bloom `AddString`, and wildcard staging costs on realistic fixtures.
2. Any optimization is implemented only if profiling justifies it, with benchmark proof and no false-negative pruning regressions.

---
*v1.3 closed 2026-10-19: SIMD parity and CI landed, the controlled benchmarks favored stdlib on every fixture, and the SIMD path is deferred as a performance option. Backlog Phase 999.7 (SIMD runtime fallback DX) was dropped as moot.*
