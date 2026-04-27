# Requirements: GIN Index v1.3 Performance Evidence & Positioning

**Defined:** 2026-04-27
**Core Value:** Turn the backlog into a measured, user-facing performance/readiness milestone: clarify row-level pruning positioning, improve contributor safety, build realistic benchmark data, profile before optimizing, and only then add justified performance knobs or ingestion optimizations.

## Scope Note

v1.3 prioritizes usefulness and evidence before speculative performance work. Backlog items that affect user understanding or contributor safety come first. Performance work is split into dataset/profiling phases before any API or optimization implementation. SIMD parser work stays deferred to v1.4 until external dependency blockers are resolved.

## Requirements

### Positioning

- [ ] **POS-01**: Users can understand from README/docs that GIN Index supports both grouped pruning and row-level pruning when callers choose `rg=1`, without implying the library is a row-level document store or search engine.
- [ ] **POS-02**: CLI and experimentation docs use consistent terminology for row groups, single-row groups, pruning, and row-level use cases.

### Developer Safety And Clarity

- [ ] **QG-01**: Contributors can enable a local pre-push quality gate that runs native repo checks such as `make lint` and `make test`, fails clearly when required tools are missing, and is documented for opt-in use.
- [ ] **CLAR-01**: Phase 06 clarity backlog is closed with non-behavioral comments or test cleanup around path reference validation and benchmark fixture brittleness.

### Benchmark Dataset Foundation

- [ ] **DATA-01**: The project has a documented realistic JSON fixture policy covering source, license/NOTICE handling, size limits, and offline default behavior.
- [ ] **DATA-02**: Smoke-scale fixtures cover nested/high-cardinality, mixed-type array, and number-heavy JSON shapes suitable for default benchmark runs.
- [ ] **DATA-03**: SEED-001 is activated into benchmark infrastructure without requiring network access for default tests.

### Profiling

- [ ] **PROF-01**: Encode benchmarks measure `writeOrderedStrings` CPU and allocation cost on UUID-heavy, timestamp/log-style, and mixed JSON workloads.
- [ ] **PROF-02**: A profiling report makes an explicit go/no-go recommendation for `WithEncodeStrategy` based on measured encode cost.
- [ ] **PROF-03**: Ingest profiling quantifies `walkJSON` / `NormalizePath` cost on builder-generated canonical paths.
- [ ] **PROF-04**: Ingest profiling quantifies bloom `AddString` allocation/hash-buffer cost on representative JSONL workloads.
- [ ] **PROF-05**: Ingest profiling quantifies array wildcard double-staging cost on long-array fixtures.

### Measurement-Backed Implementation

- [ ] **ENC-01**: If profiling justifies it, callers can configure `WithEncodeStrategy(Auto|RawOnly|FrontCodedOnly)` without changing default behavior.
- [ ] **ENC-02**: Encode strategy behavior has explicit round-trip, format-version, and query-semantics coverage.
- [ ] **ENC-03**: Benchmarks prove the encode strategy option improves the measured triggering workload.
- [ ] **ING-01**: If profiling justifies it, `walkJSON` can skip `NormalizePath` for builder-generated canonical paths without changing accepted JSONPath semantics.
- [ ] **ING-02**: If profiling justifies it, bloom insertion avoids measured per-insert allocation overhead without changing bloom membership semantics.
- [ ] **ING-03**: If profiling justifies it, callers can opt out of implicit `[*]` wildcard staging with backwards-compatible defaults and query behavior tests.

## Future Requirements

- SIMD parser adapter and SIMD validation/CI move to v1.4 because upstream license, version tag, and shared-library distribution issues are still unresolved.
- `ValidateDocument` dry-run API remains future scope until there is a concrete consumer.

## Out of Scope

- Changing the public pruning API naming.
- Building a row-level document store, ranked retrieval, or query service.
- Implementing performance optimizations before benchmark/profiling evidence exists.
- Adding `WithEncodeStrategy` if Phase 22 recommends no-go.
- Adding wildcard opt-out API if Phase 23 recommends no-go.

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| POS-01 | Phase 19 | Planned |
| POS-02 | Phase 19 | Planned |
| QG-01 | Phase 20 | Planned |
| CLAR-01 | Phase 20 | Planned |
| DATA-01 | Phase 21 | Planned |
| DATA-02 | Phase 21 | Planned |
| DATA-03 | Phase 21 | Planned |
| PROF-01 | Phase 22 | Planned |
| PROF-02 | Phase 22 | Planned |
| PROF-03 | Phase 23 | Planned |
| PROF-04 | Phase 23 | Planned |
| PROF-05 | Phase 23 | Planned |
| ENC-01 | Phase 24 | Planned |
| ENC-02 | Phase 24 | Planned |
| ENC-03 | Phase 24 | Planned |
| ING-01 | Phase 25 | Planned |
| ING-02 | Phase 25 | Planned |
| ING-03 | Phase 25 | Planned |

**Coverage:**
- Requirements total: 18
- Mapped to phases: 18
- Unmapped: 0

---
*Requirements defined: 2026-04-27 for milestone v1.3 Performance Evidence & Positioning.*
