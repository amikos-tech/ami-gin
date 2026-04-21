# Phase 08: Adaptive High-Cardinality Indexing - Research

**Researched:** 2026-04-15  
**Domain:** Adaptive string indexing for high-cardinality row-group pruning  
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

Everything in this block is copied from `.planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md`. [CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md]

### Locked Decisions

#### Hot-term promotion policy
- **D-01:** High-cardinality string paths should use a hybrid promotion rule, not naive top-K alone. Promotion is driven by row-group frequency, rejects overly broad terms with a coverage ceiling, and caps the number of promoted exact terms per path.
- **D-02:** Promotion ranking should use per-term row-group coverage already available from `RGSet` cardinality at finalize time, not raw document occurrence count.

#### Long-tail fallback behavior
- **D-03:** Non-promoted values on adaptive paths should use fixed hash-bucket row-group bitmaps as the primary conservative fallback instead of falling back to `AllRGs()` after a global bloom hit.
- **D-04:** Existing string-length stats may remain an additional cheap filter, but they are not the primary adaptive fallback mechanism.

#### Adaptive operator scope
- **D-05:** Phase 08 is scoped to positive string membership pruning on adaptive paths. `EQ` and `IN` should use promoted exact bitmaps plus bucket fallback for non-promoted values.
- **D-06:** `NE` and `NIN` should stay conservative on adaptive paths unless the queried value is one of the exact promoted terms. Phase 08 should not broaden lossy negative-predicate pruning semantics.

#### Config surface
- **D-07:** Adaptive behavior should remain an additive, small global configuration surface in Phase 08. Keep `CardinalityThreshold` as the path-level trigger for switching from full exact indexing to adaptive behavior.
- **D-08:** Add only the global knobs needed for the chosen hybrid policy and bounded promotion behavior. Do not introduce a broader adaptive-policy DSL or path-level override system in this phase.

#### Metadata and CLI visibility
- **D-09:** Path metadata and `gin-index info` must distinguish exact, bloom-only, and adaptive-hybrid paths explicitly.
- **D-10:** Adaptive paths should expose moderate summary counters rather than raw diagnostic dumps: enough to show hybrid mode, promoted hot-term count, configured threshold/cap, and useful hybrid-coverage summary, but not a heavy diagnostic surface or raw promoted values.

### Claude's Discretion
- Exact default values and naming for the new global adaptive knobs, as long as they implement the locked hybrid promotion policy and remain additive to the existing `GINConfig` shape.
- Exact hash/bucket layout, hashing choice, and bucket count strategy for long-tail fallback, as long as fallback stays conservative, bounded, and versioned explicitly.
- Exact metadata field layout and CLI presentation wording, as long as adaptive paths are clearly distinguishable and expose the locked moderate summary counters.
- Exact benchmark fixture mix and reporting format, as long as HCARD-05 demonstrates pruning improvement on realistic high-cardinality datasets with bounded size growth.

### Deferred Ideas (OUT OF SCOPE)
- Path-level adaptive overrides are intentionally deferred unless Phase 08 benchmarks prove that a few specific paths need exceptions beyond global defaults.
- A richer adaptive-policy DSL is deferred; the chosen Phase 08 surface should stay small and additive.
- A full diagnostic/forensic metadata surface is deferred; Phase 08 only needs moderate counters, not heavy tuning output.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| HCARD-01 | High-cardinality string paths can retain exact row-group bitmaps for frequent terms instead of degrading entirely to bloom-only. [CITED: .planning/REQUIREMENTS.md] | `Standard Stack`, `Architecture Patterns`, and `Code Examples` define reuse of existing `stringTerms -> RGSet` data plus a new adaptive mode. [VERIFIED: builder.go, query.go, gin.go] |
| HCARD-02 | Hot-term selection is frequency-driven and configurable at build time. [CITED: .planning/REQUIREMENTS.md] | `Standard Stack` and `Common Pitfalls` show that `RGSet.Count()` is the right ranking metric and that new knobs must round-trip through config/metadata. [VERIFIED: builder.go, bitmap.go, serialize.go][CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md] |
| HCARD-03 | Non-hot terms on adaptive paths still use a compact fallback with no false negatives. [CITED: .planning/REQUIREMENTS.md] | `Architecture Patterns`, `Don't Hand-Roll`, and `Security Domain` specify bucket RG bitmaps plus property-test coverage for superset semantics. [VERIFIED: query.go, bitmap.go, integration_property_test.go][CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md] |
| HCARD-04 | Index metadata surfaces whether a path is exact, bloom-only, or adaptive-hybrid. [CITED: .planning/REQUIREMENTS.md] | `Architecture Patterns`, `Common Pitfalls`, and `Validation Architecture` call out new flags/metadata plus CLI info changes and round-trip tests. [VERIFIED: gin.go, serialize.go, cmd/gin-index/main.go] |
| HCARD-05 | Benchmarks and fixtures quantify pruning improvement and size impact for realistic high-cardinality distributions. [CITED: .planning/REQUIREMENTS.md] | `Common Pitfalls`, `Validation Architecture`, and `Environment Availability` identify the current uniform fixture gap and the benchmark harness to extend. [VERIFIED: benchmark_test.go, Makefile, Phase 07 summary] |
</phase_requirements>

## Summary

Phase 08 can be implemented without a second ingest pass because the builder already keeps per-path `stringTerms map[string]*RGSet`, HyperLogLog cardinality, bloom inserts, and string-length stats until `Finalize()`. [VERIFIED: builder.go]

The current gap is finalize-time layout and query semantics: once estimated cardinality exceeds `CardinalityThreshold`, `Finalize()` sets `FlagBloomOnly`, drops the `StringIndex`, and string `EQ` on that path returns `AllRGs()` after the bloom and length checks. [VERIFIED: builder.go, query.go]

The cleanest plan is a third string-path mode, `adaptive-hybrid`, that keeps exact bitmaps only for promoted hot terms and stores fixed hash-bucket RG bitmaps for the long tail; this matches the locked policy, preserves no-false-negative behavior, and limits final index growth to `promoted terms + fixed buckets` instead of `all terms`. [CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md][VERIFIED: builder.go, bitmap.go, query.go]

**Primary recommendation:** Implement adaptive paths as a new serialized/string-query mode keyed by a new path flag plus an adaptive per-path structure that stores promoted exact terms, bucket RG bitmaps, and compact summary counters; keep existing exact and bloom-only modes unchanged. [CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md][VERIFIED: gin.go, query.go, serialize.go]

## Project Constraints (from CLAUDE.md)

- Preserve the current API surface and prefer additive configuration over breaking API changes. [CITED: CLAUDE.md]
- Keep the repo scoped as a pruning index, not a row-level search engine or database. [CITED: CLAUDE.md, .planning/PROJECT.md]
- Follow the existing functional-options pattern for configuration helpers and constructors. [CITED: CLAUDE.md][VERIFIED: gin.go]
- Use `github.com/pkg/errors` for new errors and wrapping; do not introduce `fmt.Errorf(... %w ...)` in new phase code. [CITED: CLAUDE.md][VERIFIED: go.mod]
- Keep JSONPath behavior aligned with the supported canonical forms from Phase 06. [CITED: .planning/phases/06-query-path-hot-path/06-CONTEXT.md][VERIFIED: jsonpath.go, gin.go, query.go]
- Maintain explicit, testable format evolution whenever serialized layout changes. [CITED: .planning/PROJECT.md][VERIFIED: serialize.go, serialize_security_test.go]
- Keep benchmark claims evidence-based and aligned with existing in-repo benchmark conventions. [CITED: .planning/PROJECT.md, .planning/phases/07-builder-parsing-numeric-fidelity/07-builder-parsing-numeric-fidelity-02-SUMMARY.md][VERIFIED: benchmark_test.go]
- Preserve the required Makefile contract: `test`, `integration-test`, `lint`, `lint-fix`, `security-scan`, `clean`, and `help`. [CITED: CLAUDE.md][VERIFIED: Makefile]

## Standard Stack

### Core

- `pathBuildData.stringTerms map[string]*RGSet` should remain the source of promotion candidates because it already records `term -> row-group bitmap` during ingest and gives finalize-time row-group coverage through `RGSet.Count()`. [VERIFIED: builder.go, bitmap.go][CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md]
- `RGSet` backed by `github.com/RoaringBitmap/roaring/v2 v2.17.0` should back both promoted exact terms and bucket bitmaps because the wrapper already supports `Clone`, `Union`, `Intersect`, `Invert`, and binary serialization. [VERIFIED: go.mod, bitmap.go, serialize.go]
- The existing `BloomFilter` backed by `github.com/cespare/xxhash/v2 v2.3.0` should remain the first-stage negative check for all string `EQ` and `IN` lookups. [VERIFIED: go.mod, bloom.go, builder.go, query.go]
- `StringLengthIndex` should remain the secondary cheap reject path on adaptive string queries because `evaluateEQ()` already checks global min/max string lengths before mode-specific logic. [VERIFIED: query.go, builder.go, gin_test.go]
- `HyperLogLog` should remain the path-level trigger for entering adaptive/bloom-only logic because `Finalize()` already uses `pd.hll.Estimate()` against `CardinalityThreshold`. [VERIFIED: builder.go, gin.go]

### Supporting

- Reuse `xxhash` for bucket hashing instead of adding a new hash dependency because the repo already depends on it in both Bloom and HLL code paths. [VERIFIED: go.mod, bloom.go, hyperloglog.go]
- Keep `PathEntry.Flags` as the coarse public mode carrier and add a new adaptive-only companion structure for counters and bucket data rather than overloading `FlagBloomOnly`. [VERIFIED: gin.go, query.go][CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md]
- Extend the existing benchmark and property-test harnesses rather than creating a new subsystem; the repo already concentrates performance work in `benchmark_test.go` and correctness work in `gin_test.go`, `integration_property_test.go`, and `serialize_security_test.go`. [VERIFIED: benchmark_test.go, gin_test.go, integration_property_test.go, serialize_security_test.go]

### Alternatives Locked Out by Context

- Do not use naive top-K promotion; the selection rule is row-group-frequency-driven with a coverage ceiling and a per-path cap. [CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md]
- Do not use `AllRGs()` as the long-tail positive fallback after a bloom hit on adaptive paths; use fixed hash-bucket RG bitmaps instead. [CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md]
- Do not expand adaptive negative pruning beyond promoted exact terms in this phase. [CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md]

**Dependency note:** No new third-party package is required for the recommended design; the repo already contains the bitmap, hash, bloom, and HLL primitives Phase 08 needs. [VERIFIED: go.mod, bitmap.go, bloom.go, hyperloglog.go]

## Architecture Patterns

### Recommended Project Structure

```text
builder.go                  # finalize-time mode selection and bucket construction
gin.go                      # new flags, adaptive structs, config knobs/defaults
query.go                    # adaptive EQ/IN dispatch and conservative NE/NIN handling
serialize.go                # adaptive metadata/bucket encode-decode and versioning
cmd/gin-index/main.go       # mode-aware info output
gin_test.go                 # unit tests for promotion/mode semantics
integration_property_test.go # no-false-negative adaptive properties
serialize_security_test.go  # bounds/version coverage for new wire sections
benchmark_test.go           # skewed high-cardinality fixtures and pruning/size benchmarks
```

This phase spans finalize-time layout, query semantics, persistence, visibility, and proof; treating any of those as “follow-up cleanup” would leave HCARD-03 through HCARD-05 partially unplanned. [VERIFIED: builder.go, query.go, serialize.go, cmd/gin-index/main.go, benchmark_test.go][CITED: .planning/REQUIREMENTS.md]

### Pattern 1: Three-Mode String Path Layout

**What:** Keep three explicit modes for string paths: `exact`, `adaptive-hybrid`, and `bloom-only`. [CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md]

**When to use:** Use `exact` when `CardinalityThreshold` is not exceeded, `adaptive-hybrid` when the path is high-cardinality and at least one term qualifies for promotion, and `bloom-only` when the path is high-cardinality but no term qualifies under the configured promotion rule. [VERIFIED: builder.go][CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md]

**Why:** The current finalize branch is binary and drops all exact string pruning once a path crosses the threshold. [VERIFIED: builder.go]

**Example:**

```go
// Source: builder.go (current branch point that Phase 08 should replace)
cardinality := uint32(pd.hll.Estimate())
flags := uint8(0)
if cardinality > b.config.CardinalityThreshold {
	flags |= FlagBloomOnly
}
```

Use this existing branch point to compute promoted terms from `pd.stringTerms`, decide mode, and materialize either a full `StringIndex`, an adaptive structure, or pure bloom-only metadata. [VERIFIED: builder.go, bitmap.go][CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md]

### Pattern 2: Positive String Query Resolution Chain

**What:** Keep the existing fast reject order and insert adaptive dispatch only after bloom and string-length gates: `bloom reject -> length reject -> exact/adaptive/bloom mode`. [VERIFIED: query.go, gin_test.go]

**When to use:** Apply this chain to positive string membership operators only in Phase 08: `EQ` and `IN`. [CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md]

**Why:** The repo already centralizes string equality pruning in `evaluateEQ()`, and `evaluateIN()` is just a union of `evaluateEQ()` results. [VERIFIED: query.go]

**Example:**

```go
// Source: query.go (current insertion point for adaptive dispatch)
if !idx.GlobalBloom.MayContainString(bloomKey) {
	return NoRGs(numRGs)
}

if sli, ok := idx.StringLengthIndexes[uint16(pathID)]; ok {
	queryLen := uint32(len(v))
	if queryLen < sli.GlobalMin || queryLen > sli.GlobalMax {
		return NoRGs(numRGs)
	}
}

if entry.Flags&FlagBloomOnly != 0 {
	return AllRGs(numRGs)
}
```

Replace the `FlagBloomOnly -> AllRGs()` branch with: promoted exact lookup if present, otherwise deterministic bucket lookup, and keep pure bloom-only behavior only for genuinely non-adaptive high-cardinality paths. [VERIFIED: query.go][CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md]

### Pattern 3: Adaptive Data in Its Own Serialized Section

**What:** Store adaptive per-path data in its own encoded section keyed by `pathID` instead of trying to overload `StringIndex` or `PathEntry` for bucket data. [VERIFIED: serialize.go, gin.go]

**When to use:** Use this pattern for promoted exact terms, bucket RG bitmaps, and compact summary counters that must survive `Encode()`/`Decode()` and power `gin-index info`. [VERIFIED: serialize.go, cmd/gin-index/main.go][CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md]

**Why:** The current wire layout is section-based (`PathDirectory`, `BloomFilter`, `StringIndexes`, `StringLengthIndexes`, `NumericIndexes`, and so on), and the CLI currently has no other source of mode/counter data. [VERIFIED: serialize.go, cmd/gin-index/main.go]

**Example:**

```go
// Source: serialize.go (current sectioned wire pattern)
if err := writeStringIndexes(&buf, idx); err != nil {
	return nil, errors.Wrap(err, "write string indexes")
}

if err := writeStringLengthIndexes(&buf, idx); err != nil {
	return nil, errors.Wrap(err, "write string length indexes")
}
```

Follow this pattern with a dedicated adaptive writer/reader pair and matching bounds validation tests; do not hide bucket data inside opaque JSON config alone. [VERIFIED: serialize.go, serialize_security_test.go]

### Anti-Patterns to Avoid

- **Do not rank hot terms by document occurrence count.** The locked policy is row-group coverage, and the builder already has that metric in `RGSet.Count()`. [CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md][VERIFIED: builder.go, bitmap.go]
- **Do not set both the bloom-only flag and the new adaptive-mode flag on the same path.** The current query code treats `FlagBloomOnly` as “return `AllRGs()` after prefilters,” so mixed semantics would be ambiguous and brittle. [VERIFIED: query.go, gin.go]
- **Do not invert bucket-derived positives for `NE`/`NIN`.** Current negative operators are implemented as `present ∩ invert(EQ/IN)`, which is only sound when `EQ/IN` is exact. [VERIFIED: query.go][CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md]
- **Do not dump raw promoted values in metadata or CLI output.** The locked scope asks for compact summary counters, not a diagnostic surface. [CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Exact or bucket row-group set operations | A custom bitset implementation | `RGSet` on top of Roaring bitmaps. [VERIFIED: bitmap.go, go.mod] | The repo already relies on `Union`, `Intersect`, `Invert`, `Clone`, and roaring serialization semantics. [VERIFIED: bitmap.go, serialize.go] |
| Bucket hashing | A new hash package or ad-hoc string hashing | Existing `xxhash` usage. [VERIFIED: bloom.go, hyperloglog.go, go.mod] | It is already the repo’s non-cryptographic hash primitive for Bloom and HLL code paths. [VERIFIED: bloom.go, hyperloglog.go] |
| Path cardinality trigger | An exact distinct counter in finalize | Existing `HyperLogLog` estimate. [VERIFIED: builder.go, hyperloglog.go] | `Finalize()` already uses HLL to decide the current high-cardinality mode; Phase 08 should refine the branch, not replace the trigger. [VERIFIED: builder.go] |
| Impossible-string rejection | New ad-hoc length heuristics | Existing `StringLengthIndex`. [VERIFIED: builder.go, query.go] | It already gives a no-false-negative prefilter and round-trips through serialization. [VERIFIED: gin_test.go, integration_property_test.go, serialize.go] |
| Benchmark harness | A new external benchmark framework | `benchmark_test.go` plus the Phase 07 naming/fixture conventions. [VERIFIED: benchmark_test.go][CITED: .planning/phases/07-builder-parsing-numeric-fidelity/07-builder-parsing-numeric-fidelity-02-SUMMARY.md] | Existing in-repo benchmarks already cover build/query/finalize/size flows and keep evidence reproducible on-branch. [VERIFIED: benchmark_test.go, Makefile] |

**Key insight:** Phase 08 is primarily a layout-and-dispatch change on top of existing primitives, not a greenfield indexing subsystem. [VERIFIED: builder.go, query.go, serialize.go]

## Common Pitfalls

### Pitfall 1: Unsound Negative Predicate Pruning

**What goes wrong:** If adaptive `EQ` returns a bucket superset and `NE` keeps using `present ∩ invert(EQ)`, false negatives appear immediately. [VERIFIED: query.go]

**Why it happens:** Current `NE` and `NIN` are exactness-dependent wrappers around `evaluateEQ()` and `evaluateIN()`. [VERIFIED: query.go]

**How to avoid:** Only allow negative pruning on adaptive paths when the queried term is a promoted exact term; otherwise return the conservative present-set behavior for that path. [CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md][VERIFIED: query.go]

**Warning signs:** Tail-value `NE` or `NIN` tests return fewer row groups than a naive document scan. [CITED: .planning/REQUIREMENTS.md]

### Pitfall 2: Metadata That Cannot Survive Decode

**What goes wrong:** New adaptive knobs or counters exist at build time but disappear after `Encode()`/`Decode()`, leaving `gin-index info` unable to describe the loaded index accurately. [VERIFIED: serialize.go, cmd/gin-index/main.go]

**Why it happens:** The current serialized JSON config omits `CardinalityThreshold` entirely and only persists a flat subset of `GINConfig`, while CLI info reads from the decoded index rather than the original builder. [VERIFIED: serialize.go, builder.go, cmd/gin-index/main.go]

**How to avoid:** Persist every new adaptive knob or summary field explicitly either in a new config payload field or in an adaptive metadata section, and add round-trip tests plus CLI info assertions. [VERIFIED: serialize.go, gin_test.go][CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md]

**Warning signs:** A rebuilt index and a decoded index print different adaptive settings or path modes for the same data. [VERIFIED: cmd/gin-index/main.go, serialize.go]

### Pitfall 3: Benchmarks That Miss the Hot-Head/Long-Tail Shape

**What goes wrong:** The benchmark suite reports finalize or size numbers but does not prove recovered pruning because the fixture distribution is uniform. [VERIFIED: benchmark_test.go]

**Why it happens:** `generateHighCardinalityDocs()` currently uses `i % cardinality`, which creates evenly repeated values instead of a skewed hot head with a long tail. [VERIFIED: benchmark_test.go]

**How to avoid:** Add deterministic skewed fixtures such as “hot head + unique tail” or Zipf-like distributions and benchmark promoted hot queries, non-promoted tail queries, mixed `IN`, encoded size, and candidate-RG counts. [CITED: .planning/REQUIREMENTS.md, .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md][VERIFIED: benchmark_test.go]

**Warning signs:** HCARD-05 can show size changes but cannot show that hot values prune better than pure bloom-only. [CITED: .planning/REQUIREMENTS.md]

### Pitfall 4: Decode Hardening Regressions

**What goes wrong:** A new adaptive wire section can permit oversized bucket counts, term counts, or RG-set allocations if it is added without bounds checks. [VERIFIED: serialize.go, serialize_security_test.go]

**Why it happens:** Existing decode safety relies on explicit max constants and targeted negative tests for every serialized structure. [VERIFIED: serialize.go, serialize_security_test.go]

**How to avoid:** Add max constants for adaptive paths, promoted terms, and bucket counts, validate them in readers, and extend `serialize_security_test.go` for malformed adaptive payloads and version behavior. [VERIFIED: serialize.go, serialize_security_test.go]

**Warning signs:** `Decode()` accepts absurd adaptive counts or allocates based on untrusted lengths. [VERIFIED: serialize.go]

### Pitfall 5: Overclaiming Build-Memory Improvements

**What goes wrong:** The plan can imply Phase 08 reduces builder memory, even though the builder currently materializes `stringTerms` for every observed string term before `Finalize()` decides what to keep. [VERIFIED: builder.go]

**Why it happens:** The simplest Phase 08 implementation reuses current ingest state and only changes finalized output. [VERIFIED: builder.go]

**How to avoid:** Scope the phase claims to final index size and pruning quality unless benchmarks show builder memory is also addressed; if build-memory reduction matters later, treat it as follow-on work. [CITED: .planning/ROADMAP.md, .planning/REQUIREMENTS.md][VERIFIED: builder.go]

**Warning signs:** Benchmarks improve encoded size but `BenchmarkFinalize` or builder-allocation numbers remain flat or worse on extreme high-cardinality inputs. [VERIFIED: benchmark_test.go]

## Code Examples

Verified patterns from the current repo are below. [VERIFIED: builder.go, query.go, serialize.go]

### Existing Term Accumulation Already Provides Promotion Inputs

```go
// Source: builder.go
func (b *GINBuilder) addStringTerm(pd *pathBuildData, term string, rgID int, path string) {
	pd.hll.AddString(term)

	if _, ok := pd.stringTerms[term]; !ok {
		pd.stringTerms[term] = MustNewRGSet(b.numRGs)
	}
	pd.stringTerms[term].Set(rgID)

	b.bloom.AddString(path + "=" + term)
	b.addStringLengthStat(pd, len(term), rgID)
}
```

This is why Phase 08 can choose hot terms at finalize time without a second ingest structure. [VERIFIED: builder.go]

### Existing String Query Fast Path Is the Right Adaptive Hook Point

```go
// Source: query.go
if !idx.GlobalBloom.MayContainString(bloomKey) {
	return NoRGs(numRGs)
}

if sli, ok := idx.StringLengthIndexes[uint16(pathID)]; ok {
	queryLen := uint32(len(v))
	if queryLen < sli.GlobalMin || queryLen > sli.GlobalMax {
		return NoRGs(numRGs)
	}
}
```

Adaptive mode should reuse these two cheap rejects before any promoted-term or bucket lookup. [VERIFIED: query.go, gin_test.go]

### Existing Wire Format Is Section-Oriented and Defensive

```go
// Source: serialize.go
if err := writePathDirectory(&buf, idx); err != nil {
	return nil, errors.Wrap(err, "write path directory")
}
if err := writeBloomFilter(&buf, idx.GlobalBloom); err != nil {
	return nil, errors.Wrap(err, "write bloom filter")
}
if err := writeStringIndexes(&buf, idx); err != nil {
	return nil, errors.Wrap(err, "write string indexes")
}
```

Phase 08 should follow the same dedicated-section pattern and add matching max-count validation in the reader and security tests. [VERIFIED: serialize.go, serialize_security_test.go]

## State of the Art

| Old Approach | Current Recommended Approach | When Changed | Impact |
|--------------|------------------------------|--------------|--------|
| High-cardinality string paths are a binary choice between full exact indexing and `FlagBloomOnly`. [VERIFIED: builder.go] | Use explicit `exact`, `adaptive-hybrid`, and `bloom-only` modes. [CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md] | Phase 08 | Hot values keep exact pruning without forcing exact storage for the long tail. [CITED: .planning/ROADMAP.md, .planning/REQUIREMENTS.md] |
| A bloom hit on a bloom-only string path returns `AllRGs()`. [VERIFIED: query.go] | A bloom hit on an adaptive path should return a promoted exact bitmap if present, otherwise the deterministic bucket bitmap. [CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md] | Phase 08 | Positive tail queries stay conservative but materially narrower than `AllRGs()` when buckets are selective. [CITED: .planning/REQUIREMENTS.md] |
| Current CLI info only prints path name, id, types, and cardinality. [VERIFIED: cmd/gin-index/main.go] | CLI info should add mode plus moderate adaptive summary counters. [CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md] | Phase 08 | HCARD-04 becomes externally visible and debuggable without exposing raw promoted terms. [CITED: .planning/REQUIREMENTS.md] |

**Deprecated/outdated for this phase:**

- “High cardinality = bloom-only” is the current repo behavior, but it is explicitly insufficient for Phase 08 planning because HCARD-01 through HCARD-05 require a hybrid mode. [VERIFIED: builder.go, README.md][CITED: .planning/REQUIREMENTS.md, .planning/ROADMAP.md]

## Assumptions Log

All material claims in this document were verified from repo files or cited from planning artifacts; no user confirmation is required before planning. [VERIFIED: repo file audit]

## Open Questions

1. **What exact default values should the new adaptive knobs ship with?** [CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md]
   What we know: the surface must stay global, additive, frequency-driven, and bounded by a promotion cap plus a coverage ceiling. [CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md]
   What's unclear: the final numeric defaults for hot-term cap, minimum RG frequency, maximum RG coverage, and bucket count. [CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md]
   Recommendation: choose provisional defaults only after the new skewed HCARD-05 benchmarks exist, and lock them with benchmark evidence instead of intuition. [CITED: .planning/PROJECT.md, .planning/REQUIREMENTS.md][VERIFIED: benchmark_test.go]

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|-------------|-----------|---------|----------|
| Go toolchain | Build, test, benchmarks | ✓ [VERIFIED: `go version`] | `go1.26.2` installed, module targets `go 1.25.5`. [VERIFIED: `go version`, go.mod] | — |
| `make` | Standard repo commands | ✓ [VERIFIED: `make --version`] | GNU Make 3.81. [VERIFIED: `make --version`] | Run the underlying `go` commands directly. [VERIFIED: Makefile] |
| `gotestsum` | `make test` | ✓ [VERIFIED: `gotestsum --version`] | `v1.13.0`. [VERIFIED: `gotestsum --version`, Makefile] | `go test ./...` for local verification. [VERIFIED: Makefile] |
| `golangci-lint` | `make lint` | ✓ [VERIFIED: `golangci-lint version`] | `2.11.4`. [VERIFIED: `golangci-lint version`] | No equivalent repo wrapper; planner should still use `make lint` because the tool is available. [VERIFIED: Makefile] |
| `govulncheck` | `make security-scan` | ✓ [VERIFIED: `govulncheck -version`] | Scanner present and vuln DB reachable on 2026-04-15. [VERIFIED: `govulncheck -version`] | — |

**Missing dependencies with no fallback:** None. [VERIFIED: environment audit]

**Missing dependencies with fallback:** None. [VERIFIED: environment audit]

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` plus `github.com/leanovate/gopter v0.2.11` for property tests. [VERIFIED: go.mod, integration_property_test.go] |
| Config file | None; test orchestration lives in `Makefile` and direct `go test` commands. [VERIFIED: Makefile] |
| Quick run command | `go test -run 'TestQueryEQ|TestQueryIN|TestQueryNIN|TestBloomFastPath|TestStringLengthIndex|TestPropertyIntegrationCardinalityThreshold|TestSerializeRoundTrip|TestStringLengthIndexSerialization' ./...` passed on 2026-04-15. [VERIFIED: command run] |
| Full suite command | `make test` for the repo-standard path, or `go test -v ./...` for direct execution. [VERIFIED: Makefile, AGENTS.md] |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| HCARD-01 | Promoted hot terms stay exact on high-cardinality paths. [CITED: .planning/REQUIREMENTS.md] | unit | `go test -run TestAdaptivePromotesHotTerms ./...` | ❌ Wave 0 |
| HCARD-02 | Promotion policy is frequency-driven and build-time configurable. [CITED: .planning/REQUIREMENTS.md] | unit | `go test -run TestAdaptiveConfigAndPromotionPolicy ./...` | ❌ Wave 0 |
| HCARD-03 | Tail fallback is compact and has no false negatives for `EQ`/`IN`; `NE`/`NIN` remain conservative. [CITED: .planning/REQUIREMENTS.md] | property + unit | `go test -run 'TestAdaptive(QuerySemantics|NegativePredicates)|TestPropertyAdaptiveNoFalseNegatives' ./...` | ❌ Wave 0 |
| HCARD-04 | Path metadata and CLI info expose exact vs bloom-only vs adaptive-hybrid. [CITED: .planning/REQUIREMENTS.md] | unit + integration | `go test -run 'TestAdaptiveMetadataRoundTrip|TestInfoShowsAdaptiveMode' ./...` | ❌ Wave 0 |
| HCARD-05 | Benchmarks quantify pruning and encoded-size impact on realistic high-cardinality distributions. [CITED: .planning/REQUIREMENTS.md] | benchmark | `go test -run '^$' -bench 'BenchmarkAdaptiveHighCardinality' -benchmem -benchtime=1x ./...` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** Run the targeted adaptive tests for the code path touched plus the existing string fast-path smoke set above. [VERIFIED: query.go, gin_test.go, command run]
- **Per wave merge:** Run `go test ./...` or `make test`, then `make lint` before closing the wave. [VERIFIED: Makefile]
- **Phase gate:** Full suite green, adaptive benchmark family present, and benchmark output recorded for pruning effectiveness plus size growth. [CITED: .planning/REQUIREMENTS.md, .planning/PROJECT.md]

### Wave 0 Gaps

- [ ] Add unit tests for promoted exact hits, non-promoted bucket hits, zero-promoted fallback to bloom-only, and negative-operator conservatism. [VERIFIED: gin_test.go, query.go][CITED: .planning/REQUIREMENTS.md]
- [ ] Add property tests that compare adaptive query results to a naive per-document scan and assert “no false negatives, exact only when promoted.” [VERIFIED: integration_property_test.go][CITED: .planning/REQUIREMENTS.md]
- [ ] Add serialization and security tests for adaptive path metadata, adaptive section bounds, and explicit version behavior if the wire format changes. [VERIFIED: serialize.go, serialize_security_test.go]
- [ ] Add CLI tests for `gin-index info` mode/counter output because the `cmd/gin-index` package currently has no tests. [VERIFIED: cmd/gin-index/main.go, `go test -list . ./...` output]
- [ ] Add new skewed high-cardinality benchmarks; the current `generateHighCardinalityDocs()` fixture is uniform and cannot prove hot-value recovery. [VERIFIED: benchmark_test.go]

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no. [VERIFIED: repo scope in README.md and CLI surface] | Not part of this library’s threat surface. [VERIFIED: README.md, cmd/gin-index/main.go] |
| V3 Session Management | no. [VERIFIED: repo scope in README.md and CLI surface] | Not part of this library’s threat surface. [VERIFIED: README.md, cmd/gin-index/main.go] |
| V4 Access Control | no. [VERIFIED: repo scope in README.md and CLI surface] | Not part of this library’s threat surface. [VERIFIED: README.md, cmd/gin-index/main.go] |
| V5 Input Validation | yes. [VERIFIED: jsonpath.go, serialize.go, serialize_security_test.go] | Preserve strict JSONPath validation and decode bounds checks for every new adaptive field or section. [VERIFIED: jsonpath.go, serialize.go, serialize_security_test.go] |
| V6 Cryptography | no for security controls. [VERIFIED: bloom.go, hyperloglog.go] | `xxhash` is a performance hash for Bloom/HLL/buckets, not a security boundary; do not treat bucket hashing as cryptographic protection. [VERIFIED: bloom.go, hyperloglog.go] |

### Known Threat Patterns for This Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Corrupt adaptive wire payload causing oversized allocation or panic. [VERIFIED: serialize.go, serialize_security_test.go] | Denial of Service | Add explicit max constants, validate counts before allocation, and extend `serialize_security_test.go`. [VERIFIED: serialize.go, serialize_security_test.go] |
| Bucket-based pruning accidentally dropping real matches. [CITED: .planning/REQUIREMENTS.md] | Tampering | Treat bucket lookup as a conservative superset and prove it with property tests against a naive scan. [CITED: .planning/REQUIREMENTS.md][VERIFIED: integration_property_test.go] |
| Negative predicate inversion on non-exact adaptive hits. [VERIFIED: query.go] | Tampering | Keep `NE`/`NIN` conservative unless the term is promoted exact. [CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md] |
| Canonical path mismatch between built and decoded indexes hiding adaptive metadata or query results. [VERIFIED: gin.go, jsonpath.go, query.go] | Tampering | Preserve Phase 06 canonicalization on lookup and decode rebuild. [CITED: .planning/phases/06-query-path-hot-path/06-CONTEXT.md][VERIFIED: gin.go, jsonpath.go] |

## Sources

### Primary (HIGH confidence)

- `.planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md` - locked Phase 08 policy, scope, and metadata expectations. [VERIFIED: file read]
- `.planning/REQUIREMENTS.md` - HCARD-01 through HCARD-05 acceptance criteria. [VERIFIED: file read]
- `.planning/ROADMAP.md` and `.planning/PROJECT.md` - milestone constraints, success criteria, and sequencing. [VERIFIED: file read]
- `builder.go`, `gin.go`, `query.go`, `serialize.go`, `cmd/gin-index/main.go` - current implementation surfaces and insertion points. [VERIFIED: file read]
- `benchmark_test.go`, `gin_test.go`, `integration_property_test.go`, `serialize_security_test.go` - current fixture quality, safety checks, and regression hooks. [VERIFIED: file read]
- `go.mod`, `go list -m`, `go version`, `gotestsum --version`, `golangci-lint version`, `govulncheck -version`, `make --version` - dependency and environment verification. [VERIFIED: command run]
- `CLAUDE.md`, `AGENTS.md`, and `Makefile` - project conventions and required local commands. [VERIFIED: file read]

### Secondary (MEDIUM confidence)

- None. [VERIFIED: research scope stayed repo-grounded]

### Tertiary (LOW confidence)

- None. [VERIFIED: research scope stayed repo-grounded]

## Metadata

**Confidence breakdown:**

- Standard stack: HIGH - the needed primitives already exist in-repo and were verified directly from code and the module graph. [VERIFIED: go.mod, builder.go, bitmap.go, bloom.go, hyperloglog.go]
- Architecture: HIGH - Phase 08 design space is tightly constrained by locked context decisions and current code insertion points. [CITED: .planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md][VERIFIED: builder.go, query.go, serialize.go]
- Pitfalls: HIGH - each major risk is already visible in current code paths, tests, or benchmark fixtures. [VERIFIED: query.go, serialize.go, benchmark_test.go, integration_property_test.go]

**Research date:** 2026-04-15  
**Valid until:** 2026-05-15 for repo-internal structure; revisit earlier if Phase 08 design decisions change materially.
