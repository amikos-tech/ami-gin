# Roadmap: GIN Index

## Milestones

- ✅ **v0.1.0 OSS Launch** — Phases 01-05 (shipped pre-v1.0)
- ✅ **v1.0 Query & Index Quality** — Phases 06-12 (shipped 2026-04-21) — see [`milestones/v1.0-ROADMAP.md`](./milestones/v1.0-ROADMAP.md)
- ✅ **v1.1 Performance, Observability & Experimentation** — Phases 13-15 (functionally complete 2026-04-22; PRs #29 and #30 merged)
- ✅ **v1.2 Ingest Correctness & Per-Document Isolation** — Phases 16-18 (shipped 2026-04-27) — see [`milestones/v1.2-ROADMAP.md`](./milestones/v1.2-ROADMAP.md)
- 🚧 **v1.3 Performance Evidence & Positioning** — Phases 19-25 (planned 2026-04-27)
- ⏸️ **v1.4 SIMD JSON Path** — Phases 26-27 (preview only; deferred pending `pure-simdjson` license/tag/distribution resolution)

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

### 🚧 v1.3 Performance Evidence & Positioning (Phases 19-25)

- [ ] **Phase 19: Row-Level Pruning Positioning** — Promote row-level pruning (`rg=1`) as a supported usage pattern across README, CLI/docs, and experimentation guidance without changing the pruning-first API model.
- [ ] **Phase 20: Developer Quality Gates & Janitorial Clarity** — Add local pre-push quality gates and close the low-risk Phase 06 code clarity backlog.
- [ ] **Phase 21: Realistic Benchmark Dataset Foundation** — Activate SEED-001 with fixture governance, dataset acquisition rules, and smoke-scale benchmark inputs.
- [ ] **Phase 22: Encode CPU Profiling** — Measure `writeOrderedStrings` CPU/allocation cost on realistic UUID, timestamp, and mixed JSON workloads before adding encode strategy API.
- [ ] **Phase 23: Ingest Hotspot Profiling** — Measure `NormalizePath`, bloom hashing, and array wildcard staging costs before implementing ingestion optimizations.
- [ ] **Phase 24: Encode Strategy Option** — If Phase 22 justifies it, add `WithEncodeStrategy(Auto|RawOnly|FrontCodedOnly)` with explicit version/test behavior.
- [ ] **Phase 25: Measurement-Backed Ingest Optimizations** — If Phase 23 justifies it, implement the selected `NormalizePath`, bloom allocation, and/or `[*]` opt-out improvements.

### ⏸️ v1.4 SIMD JSON Path (Phases 26-27) — PREVIEW / DEFERRED

- [ ] **Phase 26: SIMD Parser Adapter** — Same-package SIMD parser behind `//go:build simdjson`, opt-in via `WithParser(...)`, preserving exact-int semantics and default stdlib behavior.
- [ ] **Phase 27: SIMD Validation, Datasets & CI** — Validate and operationalize the SIMD path once external dependency blockers are resolved.

## Phase Details

### Phase 19: Row-Level Pruning Positioning

**Goal:** Make it clear that GIN Index supports both grouped pruning and row-level pruning when callers choose one document per row group, without renaming the API or implying row-level storage/search semantics.
**Depends on:** Nothing
**Requirements:** POS-01, POS-02
**Success Criteria:**
1. README/product copy explains row groups as a caller-chosen granularity and states that `rg=1` enables row-level pruning.
2. CLI/docs terminology consistently describes grouped and single-row-group pruning without suggesting full row-level document storage.
3. At least one example or experiment note demonstrates the row-level mental model using existing APIs.
**Plans:** TBD

### Phase 20: Developer Quality Gates & Janitorial Clarity

**Goal:** Improve contributor safety with local pre-push checks and close low-risk clarity issues that reduce future maintenance friction.
**Depends on:** Nothing
**Requirements:** QG-01, CLAR-01
**Success Criteria:**
1. A lightweight pre-push quality gate runs native repo checks such as `make lint` and `make test`, with clear missing-tool behavior.
2. The gate is documented and does not require contributors to use a specific shell beyond documented prerequisites.
3. Phase 06 clarity backlog is closed with comments or small test cleanups only; no behavior changes.
**Plans:** TBD

### Phase 21: Realistic Benchmark Dataset Foundation

**Goal:** Turn SEED-001 into usable test/benchmark infrastructure with size limits, provenance notes, and smoke-scale fixtures that exercise realistic JSON shapes.
**Depends on:** Nothing
**Requirements:** DATA-01, DATA-02, DATA-03
**Success Criteria:**
1. Dataset policy defines whether fixtures are vendored, generated, or downloaded, including license/NOTICE handling and size limits.
2. Smoke fixtures cover at least nested/high-cardinality, mixed-type array, and number-heavy cases.
3. Benchmarks can run in a default smoke mode without network access or large downloads.
**Plans:** TBD

### Phase 22: Encode CPU Profiling

**Goal:** Decide whether the dual encode pass in ordered string serialization is worth optimizing by measuring real CPU/allocation impact first.
**Depends on:** Phase 21
**Requirements:** PROF-01, PROF-02
**Success Criteria:**
1. Benchmarks cover UUID-heavy, timestamp/log-style, and mixed JSON workloads using Phase 21 fixture infrastructure.
2. A profiling report quantifies encode CPU share, allocation share, and whether `writeOrderedStrings` is material on realistic inputs.
3. The report makes an explicit go/no-go recommendation for Phase 24.
**Plans:** TBD

### Phase 23: Ingest Hotspot Profiling

**Goal:** Decide which ingestion optimizations are worth implementing by measuring candidate hotspots before adding API or internal complexity.
**Depends on:** Phase 21
**Requirements:** PROF-03, PROF-04, PROF-05
**Success Criteria:**
1. Profiling quantifies `walkJSON` / `NormalizePath` cost on builder-generated canonical paths.
2. Profiling quantifies bloom `AddString` allocation and hash-buffer cost on representative ingestion workloads.
3. Profiling quantifies array wildcard double-staging cost on long-array fixtures.
4. The report ranks candidates and recommends which, if any, should be implemented in Phase 25.
**Plans:** TBD

### Phase 24: Encode Strategy Option

**Goal:** Add an explicit encode strategy knob only if Phase 22 shows that skipping the dual encode pass is worth API surface area.
**Depends on:** Phase 22
**Requirements:** ENC-01, ENC-02, ENC-03
**Success Criteria:**
1. `WithEncodeStrategy(Auto|RawOnly|FrontCodedOnly)` is additive and defaults to current behavior.
2. Raw-only and front-coded-only strategies have deterministic format-version behavior and round-trip tests.
3. Benchmarks prove the option improves the measured workload without weakening decode or query semantics.
**Plans:** TBD

### Phase 25: Measurement-Backed Ingest Optimizations

**Goal:** Implement only the ingestion optimizations justified by Phase 23, preserving correctness and existing query semantics.
**Depends on:** Phase 23
**Requirements:** ING-01, ING-02, ING-03
**Success Criteria:**
1. `NormalizePath` fast-path, bloom allocation cleanup, and/or wildcard opt-out are implemented only if Phase 23 recommends them.
2. Any public wildcard opt-out API is explicit, backwards-compatible by default, and covered by query behavior tests.
3. Benchmarks demonstrate measured improvements on the triggering workload and no false-negative pruning regressions.
**Plans:** TBD

### Phase 26: SIMD Parser Adapter

**Goal:** Land an opt-in same-package SIMD parser implementation behind the Phase 13 seam once external dependency blockers are resolved.
**Depends on:** Phase 13
**Blocked on:** upstream `pure-simdjson` LICENSE file, version tag, and shared-library distribution decision
**Requirements:** Deferred
**Plans:** TBD

### Phase 27: SIMD Validation, Datasets & CI

**Goal:** Validate and operationalize the SIMD path with reproducible corpora, distribution guidance, and CI coverage after Phase 26.
**Depends on:** Phase 26
**Blocked on:** Phase 26 and final shared-library distribution contract
**Requirements:** Deferred
**Plans:** TBD

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
| 19. Row-Level Pruning Positioning | v1.3 | 0/- | Planned | - |
| 20. Developer Quality Gates & Janitorial Clarity | v1.3 | 0/- | Planned | - |
| 21. Realistic Benchmark Dataset Foundation | v1.3 | 0/- | Planned | - |
| 22. Encode CPU Profiling | v1.3 | 0/- | Planned | - |
| 23. Ingest Hotspot Profiling | v1.3 | 0/- | Planned | - |
| 24. Encode Strategy Option | v1.3 | 0/- | Planned | - |
| 25. Measurement-Backed Ingest Optimizations | v1.3 | 0/- | Planned | - |
| 26. SIMD Parser Adapter | v1.4 preview | 0/- | Deferred | - |
| 27. SIMD Validation, Datasets & CI | v1.4 preview | 0/- | Deferred | - |

---
*v1.3 planned 2026-04-27 from backlog and SEED-001. Priority order: usefulness/positioning first, then developer safety, then benchmark infrastructure, then profiling, then only measurement-backed optimization/API work. SIMD remains deferred to v1.4 pending external dependency blockers.*
