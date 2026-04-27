# Roadmap: GIN Index

## Milestones

- ✅ **v0.1.0 OSS Launch** — Phases 01-05 (shipped pre-v1.0)
- ✅ **v1.0 Query & Index Quality** — Phases 06-12 (shipped 2026-04-21) — see [`milestones/v1.0-ROADMAP.md`](./milestones/v1.0-ROADMAP.md)
- ✅ **v1.1 Performance, Observability & Experimentation** — Phases 13-15 (functionally complete 2026-04-22; PRs #29 and #30 merged)
- ✅ **v1.2 Ingest Correctness & Per-Document Isolation** — Phases 16-18 (shipped 2026-04-27) — see [`milestones/v1.2-ROADMAP.md`](./milestones/v1.2-ROADMAP.md)
- 🚧 **v1.3 SIMD-First Performance** — Phases 19-25 (planned 2026-04-27)

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

### 🚧 v1.3 SIMD-First Performance (Phases 19-25)

- [x] **Phase 19: SIMD Dependency Decision & Integration Strategy** — Resolve the `pure-simdjson` license/tag/distribution questions and lock the integration approach so implementation can start immediately.
- [ ] **Phase 20: Realistic Benchmark Dataset Foundation** — Activate SEED-001 with fixture governance, dataset acquisition rules, and smoke-scale benchmark inputs for SIMD and non-SIMD performance work.
- [ ] **Phase 21: SIMD Parser Adapter** — Land the opt-in same-package SIMD parser behind the Phase 13 parser seam.
- [ ] **Phase 22: SIMD Validation, Benchmarks & CI** — Validate parity, performance, dataset handling, and build-tag CI for the SIMD path.
- [ ] **Phase 23: Row-Level Pruning Positioning** — Promote row-level pruning (`rg=1`) as a supported usage pattern across README, CLI/docs, and experimentation guidance.
- [ ] **Phase 24: Developer Quality Gates & Janitorial Clarity** — Add local pre-push quality gates and close the low-risk Phase 06 code clarity backlog.
- [ ] **Phase 25: Follow-On Profiling & Measurement-Backed Optimizations** — Profile encode and ingest hotspots, then implement only the encode strategy or ingest optimizations justified by measurements.

## Phase Details

### Phase 19: SIMD Dependency Decision & Integration Strategy

**Goal:** Make SIMD executable as soon as possible by resolving the external dependency and distribution blockers before any lower-impact backlog work.
**Depends on:** Phase 13
**Requirements:** SIMD-01, SIMD-02, SIMD-03
**Success Criteria:**
1. The milestone records a clear decision on the SIMD dependency source, license/NOTICE posture, version/tag pinning, and shared-library distribution/loading strategy.
2. The build strategy is specified before implementation: build tags, default stdlib behavior, opt-in API shape, unsupported-platform behavior, and CI expectations.
3. If a blocker remains unresolved, the phase produces an explicit fallback or stop condition rather than silently pushing SIMD behind unrelated work.
**Plans:** 1 plan

Plans:

- [x] 19-01: Finalize SIMD strategy artifact and update planning state

### Phase 20: Realistic Benchmark Dataset Foundation

**Goal:** Turn SEED-001 into usable test/benchmark infrastructure with size limits, provenance notes, and smoke-scale fixtures that exercise realistic JSON shapes needed for SIMD evaluation.
**Depends on:** Nothing
**Requirements:** DATA-01, DATA-02, DATA-03
**Success Criteria:**
1. Dataset policy defines whether fixtures are vendored, generated, or downloaded, including license/NOTICE handling and size limits.
2. Smoke fixtures cover at least nested/high-cardinality, mixed-type array, and number-heavy cases.
3. Benchmarks can run in a default smoke mode without network access or large downloads.
**Plans:** TBD

### Phase 21: SIMD Parser Adapter

**Goal:** Land an opt-in same-package SIMD parser implementation behind the existing parser seam without changing default stdlib behavior.
**Depends on:** Phase 19
**Requirements:** SIMD-04, SIMD-05, SIMD-06, SIMD-07
**Success Criteria:**
1. `parser_simd.go` behind `//go:build simdjson` adds a same-package SIMD parser constructor and `WithParser(...)` can select it explicitly.
2. Default builds remain stdlib-only with no SIMD dependency or runtime shared-library requirement.
3. SIMD numeric handling preserves Phase 07 exact-int semantics and never silently coerces overflow-sensitive values to `float64`.
4. The parser sink gains typed scalar fast paths where needed so SIMD tape tags do not round-trip through `any` for scalar leaves.
**Plans:** TBD

### Phase 22: SIMD Validation, Benchmarks & CI

**Goal:** Prove the SIMD path is correct, measurable, and operationally shippable.
**Depends on:** Phase 20, Phase 21
**Requirements:** SIMD-08, SIMD-09, SIMD-10, SIMD-11
**Success Criteria:**
1. Parity tests prove SIMD and stdlib produce identical encoded indexes and query results across authored fixtures and Phase 20 datasets.
2. Benchmarks compare stdlib vs SIMD typed-sink ingest on realistic fixtures and report CPU, allocation, and bytes/op deltas.
3. CI covers default builds and `-tags simdjson` builds with explicit skip/fail behavior when platform or shared-library requirements are unmet.
4. Runtime loading and release/distribution guidance explains how consumers enable SIMD without guesswork.
**Plans:** TBD

### Phase 23: Row-Level Pruning Positioning

**Goal:** Make it clear that GIN Index supports both grouped pruning and row-level pruning when callers choose one document per row group, without renaming the API or implying row-level storage/search semantics.
**Depends on:** Nothing
**Requirements:** POS-01, POS-02
**Success Criteria:**
1. README/product copy explains row groups as a caller-chosen granularity and states that `rg=1` enables row-level pruning.
2. CLI/docs terminology consistently describes grouped and single-row-group pruning without suggesting full row-level document storage.
3. At least one example or experiment note demonstrates the row-level mental model using existing APIs.
**Plans:** TBD

### Phase 24: Developer Quality Gates & Janitorial Clarity

**Goal:** Improve contributor safety with local pre-push checks and close low-risk clarity issues that reduce future maintenance friction.
**Depends on:** Nothing
**Requirements:** QG-01, CLAR-01
**Success Criteria:**
1. A lightweight pre-push quality gate runs native repo checks such as `make lint` and `make test`, with clear missing-tool behavior.
2. The gate is documented and does not require contributors to use a specific shell beyond documented prerequisites.
3. Phase 06 clarity backlog is closed with comments or small test cleanups only; no behavior changes.
**Plans:** TBD

### Phase 25: Follow-On Profiling & Measurement-Backed Optimizations

**Goal:** After SIMD is underway, evaluate the remaining performance backlog and implement only improvements justified by measurements.
**Depends on:** Phase 20
**Requirements:** PROF-01, PROF-02, PROF-03, PROF-04, PROF-05, ENC-01, ENC-02, ENC-03, ING-01, ING-02, ING-03
**Success Criteria:**
1. Profiling quantifies encode CPU, `NormalizePath`, bloom `AddString`, and wildcard staging costs on realistic fixtures.
2. `WithEncodeStrategy` is implemented only if encode profiling justifies the API surface.
3. `NormalizePath` fast-path, bloom allocation cleanup, and/or wildcard opt-out are implemented only if ingest profiling justifies them.
4. Any implemented optimization has benchmark proof and no false-negative pruning regressions.
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
| 19. SIMD Dependency Decision & Integration Strategy | v1.3 | 1/1 | Complete | 2026-04-27 |
| 20. Realistic Benchmark Dataset Foundation | v1.3 | 0/- | Planned | - |
| 21. SIMD Parser Adapter | v1.3 | 0/- | Planned | - |
| 22. SIMD Validation, Benchmarks & CI | v1.3 | 0/- | Planned | - |
| 23. Row-Level Pruning Positioning | v1.3 | 0/- | Planned | - |
| 24. Developer Quality Gates & Janitorial Clarity | v1.3 | 0/- | Planned | - |
| 25. Follow-On Profiling & Measurement-Backed Optimizations | v1.3 | 0/- | Planned | - |

---
*v1.3 reprioritized 2026-04-27: SIMD is the top priority. Phase 19 exists to clear the blocker as quickly as possible, Phase 21 implements the adapter, and Phase 22 validates/operationalizes it. Remaining backlog work follows SIMD.*
