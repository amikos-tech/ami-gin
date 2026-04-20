# Roadmap: GIN Index v1.0 Query & Index Quality

## Overview

`v0.1.0` proved the library can ship as a clean open-source package. The next milestone focuses on the index internals themselves: removing avoidable query-path overhead, reducing build-time JSON parsing cost, recovering pruning quality on high-cardinality string paths, supporting raw-plus-derived representations, and compacting the serialized layout once the functional shape stabilizes.

## Phases

**Phase Numbering:**
- Phase numbering continues from the completed `v0.1.0` milestone
- Phases `01` through `05` are complete and archived
- This milestone starts at `Phase 06`

- [x] **Phase 06: Query Path Hot Path** - Remove linear path scans and canonicalize supported JSONPath lookup (completed 2026-04-14)
- [x] **Phase 07: Builder Parsing & Numeric Fidelity** - Lower ingest overhead and make number handling explicit and safe (completed 2026-04-15)
- [x] **Phase 08: Adaptive High-Cardinality Indexing** - Recover exact pruning for hot values without exploding index size (completed 2026-04-15)
- [x] **Phase 09: Derived Representations** - Add raw-plus-derived indexing instead of replacement-only transformers (completed 2026-04-16)
- [x] **Phase 10: Serialization Compaction** - Shrink encoded path and term dictionaries once functional layout stabilizes (completed 2026-04-17)
- [ ] **Phase 11: Real-Corpus Prefix Compression Benchmarking** - Measure compaction payoff on representative external log-style datasets before considering any broader format work

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
**Plans:** 2/2 plans complete
Plans:
- [x] `06-01-PLAN.md` — Canonical path storage, constant-time lookup, decode rebuild, and regression coverage
- [x] `06-02-PLAN.md` — Wide-path log-style benchmark family for EQ, CONTAINS, REGEX, and equivalent spellings

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
**Plans:** 2/2 plans complete

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
**Plans:** 3/3 plans complete

### Phase 09: Derived Representations
**Goal**: Raw values remain queryable while derived representations become first-class indexed companions
**Depends on**: Phase 07
**Requirements**: DERIVE-01, DERIVE-02, DERIVE-03, DERIVE-04
**Success Criteria** (what must be TRUE):
  1. Configuration can declare derived representations without dropping the raw indexed value
  2. Builder emits stable path/alias metadata for derived indexes that survives encode/decode
  3. Queries can target derived representations explicitly without ambiguous lookup behavior
  4. Tests and examples cover date/time, normalized text, and extracted-subfield derived patterns
**Plans:** 3/3 plans complete

### Phase 10: Serialization Compaction
**Goal**: Encoded indexes become meaningfully smaller and stay explicitly versioned after the functional layout changes land
**Depends on**: Phase 08, Phase 09
**Requirements**: SIZE-01, SIZE-02, SIZE-03
**Success Criteria** (what must be TRUE):
  1. Path directory encoding no longer stores every path string as raw repeated bytes
  2. String term encoding no longer stores every term as raw repeated bytes
  3. Format-version handling is explicit and covered by round-trip tests for legacy and compact formats
  4. Size benchmarks show a clear encoded-size reduction on representative fixtures without query regressions
**Plans:** 3/3 plans complete

### Phase 11: Real-Corpus Prefix Compression Benchmarking
**Goal**: Validate Phase 10's real-world payoff on representative external corpora without expanding the serialization-change scope again
**Depends on**: Phase 10
**Requirements**: TBD
**Success Criteria** (what must be TRUE):
  1. Benchmark coverage includes at least one realistic external log-style corpus large enough to stress repeated paths and repeated string terms
  2. The benchmark plan defines practical dataset scales such as smoke, meaningful subset, and larger corpus runs instead of relying only on tiny synthetic fixtures
  3. Results report both raw serialized string-section deltas and final encoded artifact size on those corpora
  4. The final write-up makes it explicit where prefix compaction helps, where it is flat, and whether further format work is justified
**Plans:** 0 plans

Plans:
- [ ] TBD (run /gsd-plan-phase 11 to break down)

## Progress

**Execution Order:**
Phases execute in numeric order: `06 → 07 → 08 → 09 → 10 → 11`

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 06. Query Path Hot Path | 2/2 | Complete    | 2026-04-14 |
| 07. Builder Parsing & Numeric Fidelity | 2/2 | Complete    | 2026-04-15 |
| 08. Adaptive High-Cardinality Indexing | 3/3 | Complete    | 2026-04-15 |
| 09. Derived Representations | 3/3 | Complete   | 2026-04-16 |
| 10. Serialization Compaction | 3/3 | Complete    | 2026-04-17 |
| 11. Real-Corpus Prefix Compression Benchmarking | 0/0 | Not started | - |

---
*Previous milestone note: phases `01` through `05` completed the OSS launch and `v0.1.0` release. This roadmap is the next milestone and intentionally continues numbering from `06`.*

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
