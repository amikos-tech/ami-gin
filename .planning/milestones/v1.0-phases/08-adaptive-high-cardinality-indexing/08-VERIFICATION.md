---
phase: 08-adaptive-high-cardinality-indexing
verified: 2026-04-15T20:23:07Z
status: passed
score: 9/9 must-haves verified
overrides_applied: 0
---

# Phase 08: Adaptive High-Cardinality Indexing Verification Report

**Phase Goal:** High-cardinality string paths keep exact pruning power for hot values while retaining compact fallback behavior for the long tail
**Verified:** 2026-04-15T20:23:07Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | Builder tracks enough per-path frequency information to realize exact, bloom-only, and adaptive-hybrid string modes. | ✓ VERIFIED | `builder.go:167-235` selects promoted terms and builds adaptive buckets from `pd.stringTerms`; `builder.go:992-1021` chooses exact vs adaptive vs bloom-only in `Finalize()`. |
| 2 | Adaptive behavior is configurable with sensible defaults and validation for hot-term thresholds, caps, and buckets. | ✓ VERIFIED | `gin.go:152-163` adds adaptive config knobs; `gin.go:283-324` exposes option helpers; `gin.go:339-351` sets defaults `2/64/0.80/128`; `gin.go:373-399` validates caps, ceiling, and power-of-two buckets. |
| 3 | Hot-term promotion is driven by row-group coverage rather than raw document counts. | ✓ VERIFIED | `builder.go:173-191` ranks candidates by `rgSet.Count()`, rejects low coverage and over-broad terms, and caps the promoted set. |
| 4 | Query evaluation uses exact promoted bitmaps for hot terms and deterministic bucket fallback for the long tail without degrading to `AllRGs()` after a positive bloom hit. | ✓ VERIFIED | `query.go:78-117` does bloom reject + string-length reject + shared adaptive lookup; `query.go:131-156` routes adaptive string `EQ`; `gin_test.go:327-367` proves tail lookups stay supersets but not `AllRGs()`. |
| 5 | Adaptive `NE` and `NIN` stay conservative unless the queried value resolves to an exact-promoted term. | ✓ VERIFIED | `query.go:202-217` and `query.go:412-440` avoid inverting lossy bucket results; `gin_test.go:369-400` verifies non-promoted negatives return all present RGs while promoted negatives invert exactly. |
| 6 | Adaptive config and per-path metadata survive encode/decode under an explicit format evolution. | ✓ VERIFIED | `gin.go:10-12` bumps the binary format to version 5; `serialize.go:88-100`, `serialize.go:174-183`, `serialize.go:556-705`, and `serialize.go:1148-1223` persist adaptive config plus a dedicated adaptive section; `serialize_security_test.go:141-280` covers round-trip and malformed adaptive input. |
| 7 | Path metadata, CLI info output, and public docs distinguish exact, bloom-only, and adaptive-hybrid paths. | ✓ VERIFIED | `gin.go:61-81` stores adaptive counters; `cmd/gin-index/main.go:383-415` prints `mode=exact`, `mode=bloom-only`, and `mode=adaptive-hybrid`; `cmd/gin-index/main_test.go:260-315` locks the formatter contract; `README.md:543-571` documents the three-mode model and defaults. |
| 8 | Benchmarks and fixtures compare exact, bloom-only, and adaptive-hybrid behavior on the same realistic skewed dataset and report pruning plus encoded-size metrics. | ✓ VERIFIED | `benchmark_test.go:199-218` defines a three-mode matrix; `benchmark_test.go:241-289` builds a deterministic skewed head/tail fixture; `benchmark_test.go:1695-1748` reports `candidate_rgs` and `encoded_bytes` and fails setup unless adaptive beats bloom-only on the hot probe. |
| 9 | Focused regressions and property tests cover promotion, fallback safety, conservative negatives, serialization round-trip, and the three-mode threshold contract. | ✓ VERIFIED | `gin_test.go:190-400`, `integration_property_test.go:164-295`, and `serialize_security_test.go:141-280` provide named coverage for all phase-specific behaviors. |

**Score:** 9/9 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `gin.go` | Adaptive model, flags, path metadata, config knobs, versioning | ✓ VERIFIED | Defines `FlagAdaptiveHybrid`, `AdaptiveStringIndex`, adaptive config fields/defaults, version 5, and validates adaptive path references. |
| `builder.go` | Finalize-time adaptive promotion and bucket assembly | ✓ VERIFIED | Builds promoted exact terms and tail buckets from `pathBuildData.stringTerms` and emits three string modes in `Finalize()`. |
| `query.go` | Exact/bloom/adaptive string routing with conservative negatives | ✓ VERIFIED | Uses one adaptive lookup helper for `EQ`/`IN`/`NE`/`NIN`, with exact-vs-lossy signaling. |
| `gin_test.go` + `integration_property_test.go` | Focused regression and threshold/property coverage | ✓ VERIFIED | Contains the named adaptive regressions plus the three-mode threshold property test. |
| `serialize.go` | Adaptive config and per-path metadata persistence | ✓ VERIFIED | Writes/reads adaptive config and a dedicated adaptive string section between string and string-length sections. |
| `serialize_security_test.go` | Adaptive round-trip and malformed-input guards | ✓ VERIFIED | Covers config round-trip, path metadata round-trip, and oversized adaptive bucket rejection. |
| `cmd/gin-index/main.go` + `cmd/gin-index/main_test.go` | Mode-aware CLI info output | ✓ VERIFIED | `writeIndexInfo`/`formatPathInfo` render per-path mode and adaptive counters; tests assert the output strings directly. |
| `README.md` | Public adaptive behavior and config docs | ✓ VERIFIED | Documents the three modes, defaults, and high-cardinality pruning behavior. |
| `benchmark_test.go` | Reproducible skewed fixture benchmarks with pruning/size metrics | ✓ VERIFIED | Defines the skewed fixture family, exact/bloom/adaptive mode matrix, hot/tail probes, and metric reporting. |

### Key Link Verification

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| `pathBuildData.stringTerms` | Adaptive promotion selection | `selectAdaptivePromotedTerms()` | ✓ WIRED | `builder.go:167-202` derives promotion candidates directly from `pd.stringTerms[term].Count()`. |
| `pathBuildData.stringTerms` | Tail bucket bitmaps | `buildAdaptiveStringIndex()` | ✓ WIRED | `builder.go:205-234` unions non-promoted term RG sets into fixed buckets. |
| `adaptiveBucketIndex()` | Builder and query paths | Shared helper | ✓ WIRED | The same hash helper is called in `builder.go:230` and `query.go:93`, so build/query bucket routing stays deterministic. |
| Global bloom + string-length filters | Adaptive lookup path | `evaluateAdaptiveStringTerm()` | ✓ WIRED | `query.go:103-117` preserves bloom rejection and string-length rejection ahead of exact or bucket lookup. |
| `GINConfig` adaptive fields | Serialized config payload | `writeConfig()` / `readConfig()` | ✓ WIRED | `serialize.go:1153-1223` round-trips adaptive knobs instead of dropping them. |
| Adaptive path flags/counters | Dedicated serialized metadata section | `writeAdaptiveStringIndexes()` / `readAdaptiveStringIndexes()` | ✓ WIRED | `serialize.go:174-183`, `serialize.go:277-285`, and `serialize.go:556-705` keep adaptive data in an explicit section between string and string-length indexes. |
| `PathEntry` adaptive metadata | CLI info output | `formatPathInfo()` | ✓ WIRED | `cmd/gin-index/main.go:396-415` consumes `Flags`, `AdaptivePromotedTerms`, `AdaptiveBucketCount`, threshold, and cap. |
| Shared skewed fixture family | Reported benchmark metrics | `BenchmarkAdaptiveHighCardinality()` | ✓ WIRED | `benchmark_test.go:308-324` prepares all modes from the same fixture and `benchmark_test.go:1739-1747` reports metrics from those prepared indexes. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| --- | --- | --- | --- | --- |
| `query.go` adaptive lookup | `adaptive.Terms`, `adaptive.RGBitmaps`, `adaptive.BucketRGBitmaps` | Built from `pd.stringTerms` in `builder.go:205-234` | Yes | ✓ FLOWING |
| `cmd/gin-index/main.go` path reporting | `pe.Flags`, `pe.AdaptivePromotedTerms`, `pe.AdaptiveBucketCount`, `idx.Header.CardinalityThresh`, `idx.Config.AdaptivePromotedTermCap` | Populated by `builder.go:1004-1018` or `serialize.go:691-693` + `serialize.go:1213-1223` | Yes | ✓ FLOWING |
| `serialize.go` adaptive decode | `idx.AdaptiveStringIndexes[pathID]` and adaptive config knobs | Bytes emitted by `writeAdaptiveStringIndexes()` and `writeConfig()` | Yes | ✓ FLOWING |
| `benchmark_test.go` metrics | `hotCandidateRGs`, `tailCandidateRGs`, `encodedBytes` | `Evaluate(EQ(...))` and `Encode(idx)` on the prepared exact/bloom/adaptive indexes | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Adaptive promotion, fallback safety, and conservative negatives | `go test ./... -run 'Test(AdaptivePromotesHotTermsToExactBitmaps|AdaptiveFallbackHasNoFalseNegatives|AdaptiveNegativePredicatesStayConservative)' -count=1` | `ok github.com/amikos-tech/ami-gin 0.743s` | ✓ PASS |
| Three-mode threshold property | `go test ./... -run 'TestPropertyIntegrationCardinalityThreshold' -count=1` | `ok github.com/amikos-tech/ami-gin 7.616s` | ✓ PASS |
| Adaptive serialization and CLI info output | `go test ./... -run 'Test(AdaptiveConfigRoundTrip|AdaptivePathMetadataRoundTrip|PathInfoReportsAdaptiveMode|CLIInfoShowsAdaptiveSummary)' -count=1` | `ok github.com/amikos-tech/ami-gin 1.082s`; `ok github.com/amikos-tech/ami-gin/cmd/gin-index 2.210s` | ✓ PASS |
| Adaptive benchmark evidence | `go test ./... -run '^$' -bench 'BenchmarkAdaptiveHighCardinality' -benchtime=1x -count=1` | Hot probe: adaptive `48 candidate_rgs` vs bloom-only `96`; adaptive `18632 encoded_bytes` vs bloom-only `13743` | ✓ PASS |
| Repo-wide regression sweep | `go test ./... -count=1` | `ok github.com/amikos-tech/ami-gin 106.287s`; `ok github.com/amikos-tech/ami-gin/cmd/gin-index 1.141s` | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| `HCARD-01` | `08-01` | High-cardinality string paths can retain exact row-group bitmaps for frequent terms instead of degrading entirely to bloom-only | ✓ SATISFIED | `builder.go:992-1021` emits adaptive mode; `builder.go:205-234` stores promoted exact terms; `gin_test.go:190-259` verifies hot-term exact promotion. |
| `HCARD-02` | `08-01`, `08-02` | Hot-term selection is frequency-driven and configurable at build time | ✓ SATISFIED | `builder.go:173-195` ranks by RG coverage; `gin.go:283-399` adds additive config knobs and validation; `serialize.go:1153-1223` preserves those knobs. |
| `HCARD-03` | `08-01` | Non-hot terms on adaptive paths still use a compact fallback with no false negatives | ✓ SATISFIED | `query.go:78-117` and `query.go:131-156` route non-promoted terms into deterministic buckets; `gin_test.go:327-367` proves superset behavior without `AllRGs()`. |
| `HCARD-04` | `08-02` | Index metadata surfaces whether a path is exact, bloom-only, or adaptive-hybrid | ✓ SATISFIED | `gin.go:61-81` stores mode/counters; `cmd/gin-index/main.go:396-415` prints modes; `cmd/gin-index/main_test.go:260-315` verifies output. |
| `HCARD-05` | `08-03` | Benchmarks and fixtures quantify pruning improvement and size impact for realistic high-cardinality distributions | ✓ SATISFIED | `benchmark_test.go:241-289` defines the skewed head/tail fixture and `benchmark_test.go:1695-1748` reports `candidate_rgs` plus `encoded_bytes`; benchmark smoke confirms adaptive hot pruning beats bloom-only. |

No orphaned Phase 08 requirements were found: `HCARD-01` through `HCARD-05` are all declared across the phase plans and all map to implementation evidence on the current branch.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| `cmd/gin-index/main.go` | 593 | Local `*.parquet` / `*.gin` inputs are treated as literal files before `filepath.Glob()` | Warning | Breaks documented CLI glob workflows, but does not affect adaptive index construction, serialization, or query semantics. |
| `cmd/gin-index/main.go` | 744 | CLI integer predicates are coerced to `float64` and list parsing uses `json.Unmarshal` defaults | Warning | Exact `int64` CLI predicates above `2^53` can lose precision; unrelated to the high-cardinality string-path goal. |
| `builder.go` | 810 | Only string observations feed `pd.hll`; numeric observations update stats but not cardinality | Warning | Numeric path cardinality metadata can report `0`, but adaptive mode selection for string paths is still backed by real HLL data. |
| `integration_property_test.go` | 29 | Property tests ignore `NewBuilder`/`AddDocument` errors and `TestPropertyIntegrationSerializationPreservesQueries` returns success on `Encode` failure | Warning | Weakens one regression net, but the dedicated Phase 08 adaptive tests and full suite still passed on this branch. |

### Human Verification Required

None.

### Gaps Summary

No blocking gaps found. The Phase 08 code on the current branch delivers the roadmap goal and all five `HCARD-*` requirements: builder-side hot-term promotion exists, adaptive configuration is explicit and persisted, query routing preserves exact hot-value pruning plus conservative long-tail fallback, metadata/CLI/docs surface the three modes, and the benchmark harness demonstrates pruning recovery and encoded-size tradeoffs on a deterministic skewed dataset.

The four open warning-level review items in `08-REVIEW.md` are real advisory issues in nearby CLI/property/numeric-cardinality code, but they do not break the adaptive high-cardinality behaviors verified above and therefore do not block phase-goal achievement.

---

_Verified: 2026-04-15T20:23:07Z_
_Verifier: Claude (gsd-verifier)_
