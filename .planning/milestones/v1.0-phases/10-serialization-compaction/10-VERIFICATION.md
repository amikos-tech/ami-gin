---
phase: 10-serialization-compaction
verified: 2026-04-17T14:28:39Z
status: passed
score: 9/9 must-haves verified
overrides_applied: 0
---

# Phase 10: Serialization Compaction Verification Report

**Phase Goal:** Compact path-directory names and string term payloads in the wire format, keep the v9 boundary explicit, preserve eager decode/query semantics, and back the change with benchmark evidence.
**Verified:** 2026-04-17T14:28:39Z
**Status:** passed
**Re-verification:** Yes — final pass after adding explicit front-coded block and entry count guards.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | The wire format has an explicit v9 boundary and keeps rebuild-only compatibility behavior. | ✓ VERIFIED | `gin.go:12-28` sets `Version = uint16(9)` and documents the Phase 10 boundary; `serialize_security_test.go:285-359` keeps version-mismatch and legacy-rejection coverage current, including explicit rejection of version `8`. |
| 2 | Ordered-string compaction preserves caller order instead of re-sorting section payloads. | ✓ VERIFIED | `prefix.go:51-81` keeps the sorted helper for lexical callers while `CompressInOrder` preserves section order for path and term payloads. |
| 3 | Path-directory names are compacted once per section and remain bound to the same path metadata after decode. | ✓ VERIFIED | `serialize.go:600-661` writes one ordered-string payload for path names before metadata; `gin_test.go:1163-1224` verifies `PathID`, `PathName`, `ObservedTypes`, `Cardinality`, `Mode`, and `Flags` remain aligned. |
| 4 | Classic string terms are compacted once per path and still align with their RG bitmaps after decode. | ✓ VERIFIED | `serialize.go:708-773` switches classic term storage to ordered-string payloads; `gin_test.go:1226-1313` verifies exact term order plus `RGBitmaps[i]` parity. |
| 5 | Adaptive promoted terms are compacted once per path and preserve promoted-term plus bucket bitmap alignment after decode. | ✓ VERIFIED | `serialize.go:776-873` compacts adaptive promoted terms while leaving bucket RG bitmaps explicit; `gin_test.go:1315-1434` verifies promoted-term and bucket alignment. |
| 6 | Ordered-string decoding fails closed on unknown modes, corruption, mismatched payload modes, impossible block counts, impossible entry totals, and truncation. | ✓ VERIFIED | `serialize.go:500-583` rejects unknown modes, oversized front-coded block counts, oversized per-block entry totals, and raw/front-coded count mismatches with `ErrInvalidFormat`; `serialize_security_test.go:633-759` exercises compact corruption, mode/payload mismatch, and oversized-count regressions; `serialize_security_test.go` also keeps truncated adaptive-term rejection current. |
| 7 | Representation-bearing indexes preserve both raw-path and alias-path query behavior after compact round-trip decode. | ✓ VERIFIED | `serialize_security_test.go:607-631` verifies raw and alias predicates over the same source field before and after uncompressed v9 round-trip decode. |
| 8 | Phase 10 benchmark evidence covers mixed, high-prefix, and random-like fixtures with raw-wire, zstd, encode/decode, and post-decode query reporting. | ✓ VERIFIED | `benchmark_test.go:1238-1485` defines the three fixture families, exact legacy-vs-compact accounting helpers, and `BenchmarkPhase10SerializationCompaction` with `Size`, `Encode`, `Decode`, and `QueryAfterDecode` leaves. |
| 9 | The current tree passes targeted phase regressions, the benchmark harness/evidence commands, and the full repository suite. | ✓ VERIFIED | Fresh runs on HEAD passed: targeted Phase 10 tests, `BenchmarkPhase10SerializationCompaction` with `-benchtime=1x -count=1`, `BenchmarkPhase10SerializationCompaction` with `-benchtime=1s -count=3 -benchmem`, and `go test ./... -count=1`. |

**Score:** 9/9 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `gin.go` | explicit Phase 10 wire-format version boundary | ✓ VERIFIED | `gin.go:12-28` documents and enforces v9. |
| `prefix.go` | order-preserving front-coding entry point | ✓ VERIFIED | `prefix.go:63-81` exposes `CompressInOrder`. |
| `serialize.go` | deterministic raw-vs-front-coded ordered-string sections with fail-closed decoding | ✓ VERIFIED | `serialize.go:420-583` implements block-size selection, raw-on-tie mode choice, and guarded decoding; `serialize.go:600-873` wires it into path, classic, and adaptive sections. |
| `gin_test.go` | round-trip coverage for compact path/classic/adaptive sections | ✓ VERIFIED | `gin_test.go:1163-1434` verifies ordering and RG/metadata binding. |
| `serialize_security_test.go` | corruption, compatibility, parity, and oversized-count coverage | ✓ VERIFIED | `serialize_security_test.go:285-359,607-759` covers version gating, alias parity, corruption, mode mismatch, oversized counts, and raw-on-tie fallback. |
| `benchmark_test.go` | representative benchmark matrix plus exact raw baseline accounting | ✓ VERIFIED | `benchmark_test.go:1238-1485` implements Mixed, HighPrefix, and RandomLike fixtures with exact legacy/compact/zstd metrics and query-after-decode probes. |
| `10-REVIEW.md` | advisory phase code review result | ✓ VERIFIED | Review completed with `status: clean`. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Phase 10 targeted round-trip and hardening regressions | `go test ./... -run 'Test(PathDirectoryCompactionRoundTrip|StringIndexCompactionRoundTrip|AdaptiveStringIndexCompactionRoundTrip|DecodeRejectsCompactPathSectionCorruption|DecodeRejectsCompactTermSectionCorruption|DecodeRejectsOrderedStringModePayloadMismatch|ReadOrderedStringsRejectsFrontCodedOversized(Block|Entry)Count|WriteOrderedStringsPrefersRawOnTie|DecodeVersionMismatch|DecodeLegacyRejected|DecodeRepresentationAliasParity|DecodeRejectsTruncatedAdaptiveTerm)' -count=1` | `ok github.com/amikos-tech/ami-gin 0.286s` | ✓ PASS |
| Benchmark harness smoke | `go test ./... -run '^$' -bench 'BenchmarkPhase10SerializationCompaction' -benchtime=1x -count=1` | Passed and reported stable size metrics for Mixed, HighPrefix, and RandomLike fixtures. | ✓ PASS |
| Benchmark timing evidence | `go test ./... -run '^$' -bench 'BenchmarkPhase10SerializationCompaction' -benchtime=1s -count=3 -benchmem` | Passed on HEAD. Fresh size metrics remained `Mixed=2.748%`, `HighPrefix=0.3407%`, `RandomLike=-0.04050%` raw-wire savings, with encode/decode/query timings emitted for every fixture leaf. | ✓ PASS |
| Full repository regression suite | `go test ./... -count=1` | `ok github.com/amikos-tech/ami-gin 85.942s`; `ok github.com/amikos-tech/ami-gin/cmd/gin-index 1.545s` | ✓ PASS |

### Requirements Coverage

| Requirement | Description | Status | Evidence |
| --- | --- | --- | --- |
| `SIZE-01` | Path directory serialization uses compact path-name encoding. | ✓ SATISFIED | `serialize.go:600-661` compacts path names with `writeOrderedStrings`; `gin_test.go:1163-1224` verifies path-directory round-trip fidelity. |
| `SIZE-02` | String term serialization uses compact term encoding instead of per-term raw bytes. | ✓ SATISFIED | `serialize.go:708-873` compacts classic and adaptive promoted terms; `gin_test.go:1226-1434` verifies term order and RG alignment for both section types. |
| `SIZE-03` | Compact encoding introduces explicit format-version handling and round-trip coverage for legacy/new behavior. | ✓ SATISFIED | `gin.go:12-28` defines the v9 boundary; `serialize_security_test.go:285-359,607-759` verifies strict version rejection, compact corruption handling, raw/alias parity, and deterministic ordered-string behavior. |

### Gaps Summary

None. The remaining hardening gap discovered during closeout review was fixed in `serialize.go:516-573`, covered by `serialize_security_test.go:699-759`, and re-verified on the current tree.

---

_Verified: 2026-04-17T14:28:39Z_  
_Verifier: Codex (phase closeout verification)_
