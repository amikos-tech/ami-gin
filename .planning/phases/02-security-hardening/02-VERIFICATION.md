---
phase: 02-security-hardening
verified: 2026-03-26T20:00:00Z
status: passed
score: 5/5 must-haves verified
re_verification: false
gaps: []
human_verification: []
---

# Phase 2: Security Hardening Verification Report

**Phase Goal:** Harden deserialization against crafted/corrupt inputs — strict version validation, remove legacy format fallback, add bounds checks to all stream-controlled allocations
**Verified:** 2026-03-26T20:00:00Z
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Decode() rejects binary data with Version != 3 | VERIFIED | `readHeader()` at serialize.go:308 checks `idx.Header.Version != Version`; `TestDecodeVersionMismatch` passes |
| 2 | Decode() rejects data without recognized magic bytes (no legacy fallback) | VERIFIED | `default:` case at serialize.go:214-215 returns `ErrInvalidFormat`; no "Legacy format" string exists in serialize.go; `TestDecodeLegacyRejected` passes |
| 3 | Decode() rejects crafted payloads with oversized allocations at all stream-controlled sites | VERIFIED | 10 allocation sites guarded with 7 constants and 2 header-derived checks; all 9 bounds tests pass |
| 4 | Existing round-trip serialization tests pass with no regressions | VERIFIED | Full `go test -count=1` exits 0 (34s, all property tests included) |
| 5 | ErrVersionMismatch and ErrInvalidFormat are detectable via errors.Is() | VERIFIED | Both sentinels declared at serialize.go:43-44; `TestSentinelErrors` verifies single and double wrapping |

**Score:** 5/5 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `serialize.go` | Bounds constants, sentinel errors, version validation, allocation guards | VERIFIED | Contains `ErrVersionMismatch`, `ErrInvalidFormat`, `maxRGSetSize`, `maxNumPaths`, `maxTermsPerPath`, `maxTrigramsPerPath`, `maxBloomWords`, `maxHLLRegisters` |
| `serialize.go` | All read functions guarded via `maxRGSetSize` | VERIFIED | `readRGSet` at line 99; used by all bitmap reads |
| `serialize_security_test.go` | Security-focused deserialization tests | VERIFIED | 14 test functions found: TestDecodeVersionMismatch, TestDecodeLegacyRejected, TestSentinelErrors, TestDecodeRoundTripRegression, TestDecodeBoundsRGSet, TestDecodeBoundsPathDirectory, TestDecodeBoundsStringIndexes, TestDecodeBoundsTrigramIndexes, TestDecodeBoundsDocIDMapping, TestDecodeBoundsBloomFilter, TestDecodeBoundsHLLRegisters, TestDecodeBoundsNumericRGs, TestDecodeBoundsStringLengthRGs, TestDecodeCraftedPayload |
| `gin.go` | Version constant (unchanged) | VERIFIED | `Version = uint16(3)` at gin.go:7; `MagicBytes = "GIN\x01"` unchanged |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `serialize.go:readHeader` | `gin.go:Version` | `idx.Header.Version != Version` check | WIRED | serialize.go:308: `if idx.Header.Version != Version { return errors.Wrapf(ErrVersionMismatch, ...)` |
| `serialize.go:readRGSet` | `serialize.go:maxRGSetSize` | bounds check before `make([]byte, dataLen)` | WIRED | serialize.go:99-101: `if dataLen > maxRGSetSize { return nil, errors.Wrapf(ErrInvalidFormat, ...)` followed by `make([]byte, dataLen)` at line 102 |
| `serialize.go:readDocIDMapping` | `serialize.go:Decode` | `maxDocs` parameter from `Header.NumDocs` | WIRED | Signature changed to `readDocIDMapping(r io.Reader, maxDocs uint64)`; check at serialize.go:903: `if numDocs > maxDocs` |
| `serialize_security_test.go` | `serialize.go:ErrVersionMismatch` | `errors.Is()` assertion | WIRED | 8 `stderrors.Is(..., ErrVersionMismatch)` assertions across TestDecodeVersionMismatch and TestSentinelErrors |

---

### Data-Flow Trace (Level 4)

Not applicable. This phase produces no components that render dynamic data. The artifacts are a hardened deserialization library and its tests — no UI/data rendering paths.

---

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Decode rejects wrong version | `go test -v -run TestDecodeVersionMismatch` | PASS | PASS |
| Decode rejects bad magic / legacy format | `go test -v -run TestDecodeLegacyRejected` | PASS | PASS |
| Sentinel errors unwrap correctly | `go test -v -run TestSentinelErrors` | PASS | PASS |
| All bounds checks fire | `go test -v -run TestDecodeBounds` (9 tests) | PASS (all 9) | PASS |
| Full regression suite | `go test -count=1` | PASS (34s) | PASS |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| SEC-01 | 02-01-PLAN.md | Deserialization bounds checks on all unbounded allocation sites | SATISFIED | 10 allocation sites guarded: readRGSet (maxRGSetSize), readPathDirectory (maxNumPaths), readStringIndexes (maxTermsPerPath + maxNumPaths), readStringLengthIndexes (maxRGs + maxNumPaths), readNumericIndexes (maxRGs + maxNumPaths), readNullIndexes (maxNumPaths), readTrigramIndexes (maxTrigramsPerPath + maxNumPaths), readHyperLogLogs (maxHLLRegisters + maxNumPaths), readBloomFilter (maxBloomWords), readDocIDMapping (maxDocs) |
| SEC-02 | 02-01-PLAN.md | Binary format version validation in Decode() — reject unknown versions | SATISFIED | `readHeader()` checks `idx.Header.Version != Version` and returns `ErrVersionMismatch`; legacy zstd fallback removed; default magic case returns `ErrInvalidFormat` |
| SEC-03 | 02-01-PLAN.md | Maximum size limits enforced during deserialization to prevent memory exhaustion | SATISFIED | 7 absolute size constants defined: maxRGSetSize (16MB), maxNumPaths (65535), maxTermsPerPath (1M), maxTrigramsPerPath (10M), maxBloomWords (1M words), maxHLLRegisters (65536); 3 header-derived limits: maxRGs from NumRowGroups, maxDocs from NumDocs |

**Orphaned requirements:** None. All requirements assigned to Phase 2 are accounted for in the plan and verified.

---

### Allocation Site Coverage Audit

All 15 `make()` calls in serialize.go were audited:

| Line | Size Source | Type | Bounded By | Status |
|------|-------------|------|------------|--------|
| 102 | `dataLen` (uint32) | `[]byte` | `maxRGSetSize` (line 99) | GUARDED |
| 364 | `pathLen` (uint16) | `[]byte` | uint16 max = 65535 bytes | NATURALLY BOUNDED |
| 418 | `numWords` (uint32) | `[]uint64` | `maxBloomWords` (line 415) | GUARDED |
| 475 | `numTerms` (uint32) | `[]string` | `maxTermsPerPath` (line 471) | GUARDED |
| 476 | `numTerms` (uint32) | `[]*RGSet` | same guard (line 471) | GUARDED |
| 483 | `termLen` (uint16) | `[]byte` | uint16 max = 65535 bytes | NATURALLY BOUNDED |
| 563 | `numRGs` (uint32) | `[]RGStringLengthStat` | maxRGs from Header.NumRowGroups (line 561) | GUARDED |
| 655 | `numRGs` (uint32) | `[]RGNumericStat` | maxRGs from Header.NumRowGroups (line 653) | GUARDED |
| 794 | `padLen` (uint8) | `[]byte` | uint8 max = 255 bytes | NATURALLY BOUNDED |
| 816 | `trigramLen` (uint8) | `[]byte` | uint8 max = 255 bytes | NATURALLY BOUNDED |
| 877 | `numRegisters` (uint32) | `[]uint8` | `maxHLLRegisters` (line 874) | GUARDED |
| 906 | `numDocs` (uint64) | `[]DocID` | maxDocs from Header.NumDocs (line 903) | GUARDED |
| 965 | `configLen` (uint64) | `[]byte` | `maxConfigSize` (line 961, pre-existing) | GUARDED |
| 986 | `len(sc.Transformers)` | `map` | JSON-parsed, not stream-controlled raw integer | NOT APPLICABLE |
| 987 | `len(sc.Transformers)` | `map` | JSON-parsed, not stream-controlled raw integer | NOT APPLICABLE |

All stream-controlled `make()` calls are either explicitly guarded or naturally bounded by fixed-width integer types (uint8, uint16).

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| serialize_security_test.go | 51 | `"XXXX" + "some payload data here"` string literal | Info | Test data — intentional crafted bad magic bytes for TestDecodeLegacyRejected |

No blockers or warnings. The one Info item is intentional test data, not a code smell.

---

### Human Verification Required

None. All behaviors for this phase are verifiable programmatically:
- Error returns are checkable via `errors.Is()`
- Bounds enforcement is checkable by crafting binary inputs
- Regression suite fully automated

---

### Gaps Summary

No gaps. All 5 must-have truths are verified, all 4 key links are wired, all 3 requirements (SEC-01, SEC-02, SEC-03) are satisfied, all 14 security tests pass, and the full test suite is green.

---

_Verified: 2026-03-26T20:00:00Z_
_Verifier: Claude (gsd-verifier)_
