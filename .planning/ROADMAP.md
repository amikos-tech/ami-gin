# Roadmap: GIN Index v1.0 Query & Index Quality

## Overview

`v0.1.0` proved the library can ship as a clean open-source package. The next milestone focuses on the index internals themselves: removing avoidable query-path overhead, reducing build-time JSON parsing cost, recovering pruning quality on high-cardinality string paths, supporting raw-plus-derived representations, and compacting the serialized layout once the functional shape stabilizes.

## Phases

**Phase Numbering:**
- Phase numbering continues from the completed `v0.1.0` milestone
- Phases `01` through `05` are complete and archived
- This milestone starts at `Phase 06`

- [ ] **Phase 06: Query Path Hot Path** - Remove linear path scans and canonicalize supported JSONPath lookup
- [ ] **Phase 07: Builder Parsing & Numeric Fidelity** - Lower ingest overhead and make number handling explicit and safe
- [ ] **Phase 08: Adaptive High-Cardinality Indexing** - Recover exact pruning for hot values without exploding index size
- [ ] **Phase 09: Derived Representations** - Add raw-plus-derived indexing instead of replacement-only transformers
- [ ] **Phase 10: Serialization Compaction** - Shrink encoded path and term dictionaries once functional layout stabilizes

## Phase Details

### Phase 06: Query Path Hot Path
**Goal**: Query evaluation resolves indexed paths in constant or logarithmic time and treats equivalent supported JSONPath spellings consistently
**Depends on**: Previous milestone complete
**Requirements**: PATH-01, PATH-02, PATH-03
**Success Criteria** (what must be TRUE):
  1. `findPath()` no longer linearly scans `PathDirectory` for every predicate
  2. Equivalent supported paths such as `$.foo` and `$['foo']` resolve through the same canonical lookup path
  3. EQ, CONTAINS, and REGEX benchmarks include high path-count fixtures and show measurable lookup improvement or no regression
  4. Existing query, JSONPath, and serialization tests continue to pass
**Plans:** TBD

### Phase 07: Builder Parsing & Numeric Fidelity
**Goal**: Build-time ingest gets cheaper and numeric semantics stop depending on generic `float64` JSON decoding
**Depends on**: Phase 06
**Requirements**: BUILD-01, BUILD-02, BUILD-03, BUILD-04, BUILD-05
**Success Criteria** (what must be TRUE):
  1. The primary ingest path no longer uses `json.Unmarshal(..., &any)` as its full-document representation step
  2. Integer and float classification comes from explicit number parsing
  3. Integers within the supported range are indexed without pre-index rounding loss
  4. Unsupported numeric values return an explicit error instead of being silently mis-indexed
  5. Benchmarks report ingest/build latency and allocation deltas for the new parser path
**Plans:** TBD

### Phase 08: Adaptive High-Cardinality Indexing
**Goal**: High-cardinality string paths keep exact pruning power for hot values while retaining compact fallback behavior for the long tail
**Depends on**: Phase 07
**Requirements**: HCARD-01, HCARD-02, HCARD-03, HCARD-04, HCARD-05
**Success Criteria** (what must be TRUE):
  1. Builder tracks enough per-path frequency information to promote frequent values to exact row-group bitmaps
  2. Adaptive behavior is configurable with sensible defaults for hot-term thresholds and caps
  3. Query evaluation uses exact bitmaps for promoted terms and conservative compact fallback for non-hot terms with no false negatives
  4. Path metadata and CLI/info output distinguish exact, bloom-only, and adaptive-hybrid paths
  5. Benchmarks and fixtures show improved pruning effectiveness on realistic high-cardinality datasets with bounded size growth
**Plans:** TBD

### Phase 09: Derived Representations
**Goal**: Raw values remain queryable while derived representations become first-class indexed companions
**Depends on**: Phase 07
**Requirements**: DERIVE-01, DERIVE-02, DERIVE-03, DERIVE-04
**Success Criteria** (what must be TRUE):
  1. Configuration can declare derived representations without dropping the raw indexed value
  2. Builder emits stable path/alias metadata for derived indexes that survives encode/decode
  3. Queries can target derived representations explicitly without ambiguous lookup behavior
  4. Tests and examples cover date/time, normalized text, and extracted-subfield derived patterns
**Plans:** TBD

### Phase 10: Serialization Compaction
**Goal**: Encoded indexes become meaningfully smaller and stay explicitly versioned after the functional layout changes land
**Depends on**: Phase 08, Phase 09
**Requirements**: SIZE-01, SIZE-02, SIZE-03
**Success Criteria** (what must be TRUE):
  1. Path directory encoding no longer stores every path string as raw repeated bytes
  2. String term encoding no longer stores every term as raw repeated bytes
  3. Format-version handling is explicit and covered by round-trip tests for legacy and compact formats
  4. Size benchmarks show a clear encoded-size reduction on representative fixtures without query regressions
**Plans:** TBD

## Progress

**Execution Order:**
Phases execute in numeric order: `06 → 07 → 08 → 09 → 10`

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 06. Query Path Hot Path | 0/0 | Not started | - |
| 07. Builder Parsing & Numeric Fidelity | 0/0 | Not started | - |
| 08. Adaptive High-Cardinality Indexing | 0/0 | Not started | - |
| 09. Derived Representations | 0/0 | Not started | - |
| 10. Serialization Compaction | 0/0 | Not started | - |

---
*Previous milestone note: phases `01` through `05` completed the OSS launch and `v0.1.0` release. This roadmap is the next milestone and intentionally continues numbering from `06`.*
