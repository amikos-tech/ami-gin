# Milestones

## v1.0 Query & Index Quality (Shipped: 2026-04-21)

**Phases completed:** 7 phases (06-12), 19 plans, 43 tasks
**Known deferred items at close:** 4 (see STATE.md Deferred Items)
**Git tag:** `v1.0`

**Key accomplishments:**

- Canonical JSONPath lookup now resolves supported spelling variants through one stored path name and one immutable hot-path map in fresh and decoded indexes.
- Fixed-width wide-path benchmark proof with shared log-style fixtures, explicit lookup attribution, and EQ/CONTAINS/REGEX spelling variants
- Transactional explicit-number ingest with exact-int path semantics, guarded mixed-mode promotion, and decode-parity regressions
- Reproducible parser-delta benchmarks with an in-repo legacy control and deterministic fixture families for Phase 07
- Adaptive high-cardinality string paths now keep bounded exact hot-term bitmaps and deterministic bucket fallbacks instead of collapsing directly to bloom-only behavior.
- Versioned adaptive serialization, mode-aware `gin-index info`, and README docs now make the adaptive-hybrid high-cardinality behavior explicit end to end.
- Deterministic skewed high-cardinality benchmarks proving adaptive hot-value pruning recovery with direct candidate and encoded-size metrics
- Additive raw-plus-companion staging that preserves public source values, materializes hidden sibling representations, and fails companion derivation explicitly
- Explicit alias routing, deterministic representation introspection, and v7 representation metadata round-trip parity
- Alias-aware docs/examples plus DERIVE-04 acceptance coverage for date, normalized text, and extracted-subfield companions
- v9 ordered-string sections now compact path names, classic terms, and adaptive promoted terms without changing decoded query semantics
- Fail-closed compact-section decoding plus explicit v8 rejection and raw/alias parity checks for the v9 wire format
- Fixture-backed raw/zstd size accounting plus encode, decode, and post-decode query evidence for the v9 compact wire format
- Phase 11 now has a reproducible smoke corpus plus an env-gated real-corpus benchmark family that contrasts structured metadata against text-heavy payloads
- Phase 11 now has pinned smoke, subset, and large benchmark evidence, including the large text-heavy flat/no-win case and its decode-cap behavior
- Phase 11 now closes with an evidence-backed recommendation report and a README workflow that keeps smoke default while documenting the opt-in external corpus path
- Phase 07 proof surface rebuilt with a verified validation audit, a requirement-mapped verification report, and fresh parser/benchmark/full-suite evidence
- Phase 09 now has a current-tree verification report that ties DERIVE-01 through DERIVE-04 to rerun tests, CLI/docs smoke, and runnable example output
- BUILD and DERIVE requirement evidence now points to Phases 07 and 09, and the v1.0 milestone audit reruns cleanly against fresh current-tree command results

---
