---
phase: 06
slug: query-path-hot-path
status: verified
threats_open: 0
asvs_level: 1
created: 2026-04-14
updated: 2026-04-14T18:05:00Z
---

# Phase 06 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Public query/config paths -> canonicalization helper | Untrusted caller-supplied JSONPath strings must be validated before they influence lookup or config binding. | User-controlled JSONPath expressions |
| Builder traversal paths -> persisted `PathDirectory` | Internal traversal strings become durable path identity used by future queries and config binding. | Canonicalized path names stored in the index |
| Decoded `PathDirectory` bytes -> derived lookup state | Deserialized path names must not silently collide after canonicalization. | Serialized path metadata from untrusted bytes |
| Synthetic Phase 06 fixture generator -> benchmark claims | The benchmark fixture must create real path-count pressure or the benchmark story becomes misleading. | Deterministic benchmark documents and path counts |
| Path-lookup control benchmark -> integrated query benchmarks | Control and integrated workloads must share the same fixture family so attribution is defensible. | Shared benchmark index shapes and query workloads |
| Benchmark output -> maintainer interpretation | Raw timings are objective, but operators must remain comparable across runs. | `go test -bench` result lines and labels |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-06-01 | T | query/config path canonicalization | mitigate | Verified `canonicalizeSupportedPath()` validates supported public JSONPath inputs before normalization, and is used at `WithFTSPaths`, `WithFieldTransformer`, `WithRegisteredTransformer`, `readConfig()`, and `findPath()`. Covered by `TestJSONPathCanonicalizeSupportedPath`, `TestJSONPathCanonicalizeUnsupportedPath`, `TestConfigSerializationCanonicalPaths`, and `TestDateTransformerCanonicalConfigPath`. | closed |
| T-06-02 | T/R | duplicate canonical names during rebuild | mitigate | Verified `rebuildPathLookup()` canonicalizes persisted `PathDirectory` entries, rejects canonical collisions with `ErrInvalidFormat`, and `Decode()` invokes the rebuild before returning. Covered by `TestRebuildPathLookupRejectsDuplicateCanonicalPaths`. | closed |
| T-06-03 | D | `findPath()` hot path | mitigate | Verified `findPath()` canonicalizes once, resolves through derived `pathLookup`, and returns `-1, nil` on invalid or missing paths so `Evaluate()` preserves the existing `AllRGs()` no-pruning fallback. Covered by `TestFindPathCanonicalLookupAndFallback` and the targeted query runs. | closed |
| T-06-04 | E | mutable derived lookup state | mitigate | Verified `GINIndex` documents immutability after `Finalize()` / `Decode()`, keeps `pathLookup` non-serialized, and rebuilds it only in `Finalize()` and `Decode()` with no lazy query-time mutation path. Confirmed structurally in `gin.go`, `builder.go`, and `serialize.go`. | closed |
| T-06-05 | T | fixture shape | mitigate | Verified `benchmark_test.go` fixes the Phase 06 workload at `4096` docs / row groups, width tiers `16/128/512/2048`, recognizable base log fields, and deterministic `extra_%04d` filler fields inside `generatePhase06WideLogDoc()`. Smoke benchmark passed on that fixture family. | closed |
| T-06-06 | R | lookup attribution | mitigate | Verified `BenchmarkPathLookup` and the integrated EQ / CONTAINS / REGEX families all use `setupPhase06WideIndex`, so attribution is measured on the same fixture/index family instead of separate synthetic setups. | closed |
| T-06-07 | D | noisy benchmark comparison | mitigate | Verified benchmark names encode `paths=`, `spelling=`, and `selectivity=` on the standard `go test -bench` harness, and the smoke run emitted the expected families for `BenchmarkPathLookup`, `BenchmarkQueryEQ`, `BenchmarkQueryContains`, and `BenchmarkQueryRegex`. | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

No accepted risks.

---

## Verification Evidence

- Reviewed `.planning/phases/06-query-path-hot-path/06-01-PLAN.md` and `06-02-PLAN.md` threat models and both summary artifacts.
- Confirmed neither `06-query-path-hot-path-01-SUMMARY.md` nor `06-query-path-hot-path-02-SUMMARY.md` contains a `## Threat Flags` section requiring carry-forward risk handling.
- `go test ./... -run 'Test(JSONPath|WithFTSPaths|DateTransformer|ConfigSerialization|TransformerRoundTrip)' -count=1`
- `go test ./... -run 'Test(.*Canonical.*Path|.*Duplicate.*Path|.*Unknown.*Path|QueryEQ)' -count=1`
- `go test ./... -run '^$' -bench 'Benchmark(PathLookup|Query(EQ|Contains|Regex))' -benchtime=1x -count=1`
- `go test ./... -count=1`

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-04-14 | 7 | 7 | 0 | Codex |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-04-14
