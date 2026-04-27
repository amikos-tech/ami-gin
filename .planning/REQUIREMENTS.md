# Requirements: GIN Index v1.3 SIMD-First Performance

**Defined:** 2026-04-27
**Updated:** 2026-04-27
**Core Value:** Deliver SIMD parser integration as soon as possible, while preserving correctness, default stdlib behavior, exact numeric semantics, and operational clarity for consumers.

## Scope Note

SIMD is the highest-impact performance lever and is now the top priority. The first phase exists to clear the dependency and distribution blockers quickly, not to delay SIMD behind unrelated work. Dataset infrastructure follows immediately because it is required to validate SIMD impact. Lower-impact backlog items remain in v1.3 but run after SIMD decision, implementation, and validation.

## Requirements

### SIMD Unblock And Strategy

- [ ] **SIMD-01**: The project has an explicit decision on the SIMD dependency source, license/NOTICE obligations, version/tag pinning, and whether the dependency is acceptable for this repository.
- [ ] **SIMD-02**: The project has an explicit shared-library distribution/loading strategy, including unsupported-platform behavior and release guidance.
- [ ] **SIMD-03**: The implementation plan specifies build tags, default stdlib behavior, opt-in API shape, CI expectations, and a stop/fallback path if blockers remain unresolved.

### Benchmark Dataset Foundation

- [ ] **DATA-01**: The project has a documented realistic JSON fixture policy covering source, license/NOTICE handling, size limits, and offline default behavior.
- [ ] **DATA-02**: Smoke-scale fixtures cover nested/high-cardinality, mixed-type array, and number-heavy JSON shapes suitable for SIMD and stdlib benchmark comparison.
- [ ] **DATA-03**: SEED-001 is activated into benchmark infrastructure without requiring network access for default tests.

### SIMD Parser Adapter

- [ ] **SIMD-04**: Callers can explicitly select a same-package SIMD parser through the existing parser seam without changing the default stdlib path.
- [ ] **SIMD-05**: Default builds remain stdlib-only with no SIMD dependency or runtime shared-library requirement unless the build tag and parser selection are explicit.
- [ ] **SIMD-06**: SIMD parsing preserves Phase 07 exact-int numeric semantics and does not silently coerce overflow-sensitive values to `float64`.
- [ ] **SIMD-07**: The parser sink exposes typed scalar fast paths where needed so SIMD scalar leaves do not round-trip through `any`.

### SIMD Validation And Operations

- [ ] **SIMD-08**: SIMD and stdlib paths produce identical encoded indexes and query results across authored parity fixtures and realistic benchmark fixtures.
- [ ] **SIMD-09**: Benchmarks report stdlib vs SIMD CPU, allocation, and bytes/op deltas on realistic fixtures.
- [ ] **SIMD-10**: CI covers default builds and `-tags simdjson` builds with explicit behavior when platform or shared-library requirements are unmet.
- [ ] **SIMD-11**: Runtime loading and release/distribution guidance tells consumers how to enable SIMD safely.

### Positioning

- [ ] **POS-01**: Users can understand from README/docs that GIN Index supports both grouped pruning and row-level pruning when callers choose `rg=1`, without implying the library is a row-level document store or search engine.
- [ ] **POS-02**: CLI and experimentation docs use consistent terminology for row groups, single-row groups, pruning, and row-level use cases.

### Developer Safety And Clarity

- [ ] **QG-01**: Contributors can enable a local pre-push quality gate that runs native repo checks such as `make lint` and `make test`, fails clearly when required tools are missing, and is documented for opt-in use.
- [ ] **CLAR-01**: Phase 06 clarity backlog is closed with non-behavioral comments or test cleanup around path reference validation and benchmark fixture brittleness.

### Follow-On Profiling And Measurement-Backed Implementation

- [ ] **PROF-01**: Encode benchmarks measure `writeOrderedStrings` CPU and allocation cost on UUID-heavy, timestamp/log-style, and mixed JSON workloads.
- [ ] **PROF-02**: A profiling report makes an explicit go/no-go recommendation for `WithEncodeStrategy` based on measured encode cost.
- [ ] **PROF-03**: Ingest profiling quantifies `walkJSON` / `NormalizePath` cost on builder-generated canonical paths.
- [ ] **PROF-04**: Ingest profiling quantifies bloom `AddString` allocation/hash-buffer cost on representative JSONL workloads.
- [ ] **PROF-05**: Ingest profiling quantifies array wildcard double-staging cost on long-array fixtures.
- [ ] **ENC-01**: If profiling justifies it, callers can configure `WithEncodeStrategy(Auto|RawOnly|FrontCodedOnly)` without changing default behavior.
- [ ] **ENC-02**: Encode strategy behavior has explicit round-trip, format-version, and query-semantics coverage.
- [ ] **ENC-03**: Benchmarks prove the encode strategy option improves the measured triggering workload.
- [ ] **ING-01**: If profiling justifies it, `walkJSON` can skip `NormalizePath` for builder-generated canonical paths without changing accepted JSONPath semantics.
- [ ] **ING-02**: If profiling justifies it, bloom insertion avoids measured per-insert allocation overhead without changing bloom membership semantics.
- [ ] **ING-03**: If profiling justifies it, callers can opt out of implicit `[*]` wildcard staging with backwards-compatible defaults and query behavior tests.

## Future Requirements

- `ValidateDocument` dry-run API remains future scope until there is a concrete consumer.

## Out of Scope

- Making SIMD the default parser in v1.3.
- Weakening exact numeric semantics to fit the SIMD parser.
- Requiring SIMD dependencies for default builds.
- Changing the public pruning API naming.
- Building a row-level document store, ranked retrieval, or query service.
- Implementing lower-priority performance optimizations before SIMD decision/implementation/validation work.

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| SIMD-01 | Phase 19 | Planned |
| SIMD-02 | Phase 19 | Planned |
| SIMD-03 | Phase 19 | Planned |
| DATA-01 | Phase 20 | Planned |
| DATA-02 | Phase 20 | Planned |
| DATA-03 | Phase 20 | Planned |
| SIMD-04 | Phase 21 | Planned |
| SIMD-05 | Phase 21 | Planned |
| SIMD-06 | Phase 21 | Planned |
| SIMD-07 | Phase 21 | Planned |
| SIMD-08 | Phase 22 | Planned |
| SIMD-09 | Phase 22 | Planned |
| SIMD-10 | Phase 22 | Planned |
| SIMD-11 | Phase 22 | Planned |
| POS-01 | Phase 23 | Planned |
| POS-02 | Phase 23 | Planned |
| QG-01 | Phase 24 | Planned |
| CLAR-01 | Phase 24 | Planned |
| PROF-01 | Phase 25 | Planned |
| PROF-02 | Phase 25 | Planned |
| PROF-03 | Phase 25 | Planned |
| PROF-04 | Phase 25 | Planned |
| PROF-05 | Phase 25 | Planned |
| ENC-01 | Phase 25 | Planned |
| ENC-02 | Phase 25 | Planned |
| ENC-03 | Phase 25 | Planned |
| ING-01 | Phase 25 | Planned |
| ING-02 | Phase 25 | Planned |
| ING-03 | Phase 25 | Planned |

**Coverage:**
- Requirements total: 29
- Mapped to phases: 29
- Unmapped: 0

---
*Requirements updated: 2026-04-27 for milestone v1.3 SIMD-First Performance.*
