# Requirements: GIN Index v1.0 Query & Index Quality

**Defined:** 2026-04-14
**Core Value:** Material pruning quality and hot-path efficiency gains without turning the library into a heavyweight database or document store

## Requirements

### Query Hot Path

- [x] **PATH-01**: Predicate evaluation resolves indexed paths without linearly scanning `PathDirectory` — validated in Phase 06
- [x] **PATH-02**: Equivalent supported JSONPath spellings resolve through a canonical path form — validated in Phase 06
- [x] **PATH-03**: Benchmarks cover EQ, CONTAINS, and REGEX queries across wide path counts and guard against regression — validated in Phase 06

### Builder Parsing & Numeric Fidelity

- [ ] **BUILD-01**: The primary ingest path no longer relies on `json.Unmarshal(..., &any)` for full-document decoding
- [ ] **BUILD-02**: Integer-vs-float classification is based on explicit number parsing rather than generic `float64` decoding
- [ ] **BUILD-03**: Integers within the supported range are indexed without losing precision before stats or bitmap decisions are made
- [ ] **BUILD-04**: Unsupported or unrepresentable numeric values fail safely with an explicit error instead of silent mis-indexing
- [ ] **BUILD-05**: Benchmarks capture ingest/build latency and allocation changes before and after the parser redesign

### Adaptive High-Cardinality Indexing

- [x] **HCARD-01**: High-cardinality string paths can retain exact row-group bitmaps for frequent terms instead of degrading entirely to bloom-only — validated in Phase 08
- [x] **HCARD-02**: Hot-term selection is frequency-driven and configurable at build time — validated in Phase 08
- [x] **HCARD-03**: Non-hot terms on adaptive paths still use a compact fallback with no false negatives — validated in Phase 08
- [x] **HCARD-04**: Index metadata surfaces whether a path is exact, bloom-only, or adaptive-hybrid — validated in Phase 08
- [x] **HCARD-05**: Benchmarks and fixtures quantify pruning improvement and size impact for realistic high-cardinality distributions — validated in Phase 08

### Derived Representations

- [ ] **DERIVE-01**: Configuration can declare derived indexes that preserve raw indexing and add transformed/index-friendly representations alongside it
- [ ] **DERIVE-02**: Derived representations are queryable through explicit, deterministic path names or aliases
- [ ] **DERIVE-03**: Serialization persists derived-index metadata so encoded indexes round-trip without custom rebuild logic
- [ ] **DERIVE-04**: Tests and examples cover at least date/time, normalized text, and extracted-subfield derived indexing patterns

### Serialization Compaction

- [x] **SIZE-01**: Path directory serialization uses prefix compression or an equivalent compact representation — validated in Phase 10
- [x] **SIZE-02**: String term serialization uses prefix compression or block compaction instead of raw repeated strings — validated in Phase 10
- [x] **SIZE-03**: Compact encoding introduces explicit format-version handling with round-trip coverage for legacy and new index formats — validated in Phase 10

## Out of Scope

| Feature | Reason |
|---------|--------|
| Full binary JSON / document-store representation | This milestone is about pruning quality, not row-level retrieval |
| BM25 or ranked text retrieval | Frequency is only used for pruning/index layout decisions |
| A new boolean query language | Existing predicate composition is sufficient for this milestone |
| Multi-file index merge | Valuable, but lower impact than single-index pruning quality |
| Serving the index as a remote service | Operational infrastructure is not the current bottleneck |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| PATH-01 | Phase 06 | Complete |
| PATH-02 | Phase 06 | Complete |
| PATH-03 | Phase 06 | Complete |
| BUILD-01 | Phase 12 | Pending |
| BUILD-02 | Phase 12 | Pending |
| BUILD-03 | Phase 12 | Pending |
| BUILD-04 | Phase 12 | Pending |
| BUILD-05 | Phase 12 | Pending |
| HCARD-01 | Phase 08 | Complete |
| HCARD-02 | Phase 08 | Complete |
| HCARD-03 | Phase 08 | Complete |
| HCARD-04 | Phase 08 | Complete |
| HCARD-05 | Phase 08 | Complete |
| DERIVE-01 | Phase 12 | Pending |
| DERIVE-02 | Phase 12 | Pending |
| DERIVE-03 | Phase 12 | Pending |
| DERIVE-04 | Phase 12 | Pending |
| SIZE-01 | Phase 10 | Complete |
| SIZE-02 | Phase 10 | Complete |
| SIZE-03 | Phase 10 | Complete |

**Coverage:**
- Requirements total: 20
- Checked off: 11
- Mapped to phases: 20
- Unmapped: 0

---
*Requirements defined: 2026-04-14*
*Last updated: 2026-04-20 after milestone gap planning*
