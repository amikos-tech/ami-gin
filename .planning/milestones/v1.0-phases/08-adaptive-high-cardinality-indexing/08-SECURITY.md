---
phase: 08
slug: adaptive-high-cardinality-indexing
status: verified
threats_open: 0
asvs_level: 1
created: 2026-04-16
updated: 2026-04-16T06:04:28Z
---

# Phase 08 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Per-term coverage -> promotion selection | Promotion must be bounded and deterministic or hot-term promotion can overfit and degrade pruning guarantees. | `pd.stringTerms` row-group coverage, threshold/cap/ceiling config |
| Term hash -> adaptive bucket bitmap | Builder and query must use the same bucket mapping or adaptive tail lookups can miss true matches. | Untrusted string terms, bucket IDs, tail `RGSet` unions |
| Adaptive query path -> negative predicate logic | Lossy bucket matches must never be inverted as if they were exact. | Query terms, promoted-match signal, present row-group bitmap |
| In-memory adaptive index -> serialized bytes | Wire-format drift or malformed sections can corrupt adaptive behavior after decode. | Promoted terms, bucket counts, adaptive `RGSet` bitmaps, config knobs |
| Adaptive metadata -> CLI output | Operators need accurate mode and counter reporting to reason about pruning behavior. | Path flags, promoted-term counts, bucket counts, threshold/cap metadata |
| Fixture/docs/benchmark output -> maintainer decisions | Unrealistic fixtures or incomplete metrics can mislead future tuning and rollback decisions. | Deterministic skewed fixture data, `candidate_rgs`, `encoded_bytes`, README guidance |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-08-01 | T/I | adaptive bucket routing | mitigate | Verified one shared deterministic bucket helper in [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:138) and [query.go](/Users/tazarov/experiments/amikos/custom-gin/query.go:93). Builder sends non-promoted terms into bucket unions at [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:226), and query reuses the same helper in `lookupAdaptiveStringMatch()` at [query.go](/Users/tazarov/experiments/amikos/custom-gin/query.go:78). Coverage is locked by `TestAdaptiveFallbackHasNoFalseNegatives` in [gin_test.go](/Users/tazarov/experiments/amikos/custom-gin/gin_test.go:327). | closed |
| T-08-02 | D | promoted exact term set | mitigate | Verified adaptive knobs, defaults, and validation in [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:152), [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:283), and [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:339). Promotion ranking uses `RGSet.Count()`, minimum RG coverage, coverage ceiling, and promoted-term cap in [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:167). Regression coverage is anchored by `TestAdaptivePromotesHotTermsToExactBitmaps` and oversized-setting rejection in [gin_test.go](/Users/tazarov/experiments/amikos/custom-gin/gin_test.go:190). | closed |
| T-08-03 | T/I | adaptive negative predicates | mitigate | Verified adaptive positives flow through `evaluateAdaptiveStringTerm()` at [query.go](/Users/tazarov/experiments/amikos/custom-gin/query.go:103), while `evaluateNE()` and `evaluateNIN()` only invert exact-promoted results and otherwise return all present row groups at [query.go](/Users/tazarov/experiments/amikos/custom-gin/query.go:202) and [query.go](/Users/tazarov/experiments/amikos/custom-gin/query.go:412). Locked by `TestAdaptiveNegativePredicatesStayConservative` in [gin_test.go](/Users/tazarov/experiments/amikos/custom-gin/gin_test.go:369). | closed |
| T-08-04 | T/R | adaptive encode/decode | mitigate | Verified explicit version bump to 5 in [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:12), adaptive section placement between string and string-length sections in [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:174), dedicated adaptive read/write helpers and bounds checks in [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:556), and adaptive config round-trip in [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:1148). Locked by `TestAdaptiveConfigRoundTrip`, `TestAdaptivePathMetadataRoundTrip`, and malformed-input rejection in [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:141). | closed |
| T-08-05 | R | CLI path reporting | mitigate | Verified `writeIndexInfo()` and `formatPathInfo()` emit explicit `mode=exact`, `mode=bloom-only`, and `mode=adaptive-hybrid`, with adaptive counters, at [cmd/gin-index/main.go](/Users/tazarov/experiments/amikos/custom-gin/cmd/gin-index/main.go:383). Locked by `TestPathInfoReportsAdaptiveMode` and `TestCLIInfoShowsAdaptiveSummary` in [cmd/gin-index/main_test.go](/Users/tazarov/experiments/amikos/custom-gin/cmd/gin-index/main_test.go:260). | closed |
| T-08-06 | I | README / config docs | mitigate | Verified the README documents the additive knobs and three-mode behavior at [README.md](/Users/tazarov/experiments/amikos/custom-gin/README.md:551) and [README.md](/Users/tazarov/experiments/amikos/custom-gin/README.md:563), including `adaptive-hybrid`, `bloom-only`, and hot-term exact pruning language. | closed |
| T-08-07 | R | benchmark fixture realism | mitigate | Verified the benchmark harness uses one deterministic skewed head-tail fixture with 32 hot values and a 10k+ long tail in [benchmark_test.go](/Users/tazarov/experiments/amikos/custom-gin/benchmark_test.go:241), and prepares exact, bloom-only, and adaptive-hybrid modes from that same fixture in [benchmark_test.go](/Users/tazarov/experiments/amikos/custom-gin/benchmark_test.go:308). | closed |
| T-08-08 | I | benchmark interpretation | mitigate | Verified `BenchmarkAdaptiveHighCardinality` reports `candidate_rgs` and `encoded_bytes` for hot and tail probes, and fails setup unless adaptive hot pruning beats bloom-only, at [benchmark_test.go](/Users/tazarov/experiments/amikos/custom-gin/benchmark_test.go:1695). This keeps performance evidence tied to pruning behavior instead of raw latency only. | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

No accepted risks.

---

## Verification Evidence

- Read the threat models in [08-01-PLAN.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/08-adaptive-high-cardinality-indexing/08-01-PLAN.md:195), [08-02-PLAN.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/08-adaptive-high-cardinality-indexing/08-02-PLAN.md:179), and [08-03-PLAN.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/08-adaptive-high-cardinality-indexing/08-03-PLAN.md:134).
- Confirmed the phase summary files contain no `## Threat Flags` carry-forward sections: [08-01-SUMMARY.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/08-adaptive-high-cardinality-indexing/08-01-SUMMARY.md:1), [08-02-SUMMARY.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/08-adaptive-high-cardinality-indexing/08-02-SUMMARY.md:1), [08-03-SUMMARY.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/08-adaptive-high-cardinality-indexing/08-03-SUMMARY.md:1).
- `go test ./... -run 'Test(AdaptivePromotesHotTermsToExactBitmaps|AdaptiveFallbackHasNoFalseNegatives|AdaptiveNegativePredicatesStayConservative|TestPropertyIntegrationCardinalityThreshold)' -count=1`
- `go test ./... -run 'Test(AdaptiveConfigRoundTrip|AdaptivePathMetadataRoundTrip|PathInfoReportsAdaptiveMode|CLIInfoShowsAdaptiveSummary)' -count=1`
- `go test ./... -run '^$' -bench 'BenchmarkAdaptiveHighCardinality' -benchtime=1x -count=1`
- `go test ./... -count=1`

Benchmark snapshot from the audit run:

- `mode=exact/probe=hot-value`: `48 candidate_rgs`, `26466 encoded_bytes`
- `mode=bloom-only/probe=hot-value`: `96 candidate_rgs`, `13734 encoded_bytes`
- `mode=adaptive-hybrid/probe=hot-value`: `48 candidate_rgs`, `18659 encoded_bytes`
- `mode=adaptive-hybrid/probe=tail-value`: `2 candidate_rgs`, `18659 encoded_bytes`

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-04-16 | 8 | 8 | 0 | Codex |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-04-16
