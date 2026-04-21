---
phase: 06-query-path-hot-path
reviewed: 2026-04-14T14:21:36Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - benchmark_test.go
  - builder.go
  - gin.go
  - gin_test.go
  - jsonpath.go
  - query.go
  - serialize.go
  - serialize_security_test.go
  - transformer_registry_test.go
  - transformers_test.go
findings:
  critical: 3
  warning: 0
  info: 0
  total: 3
status: issues_found
---

# Phase 06: Code Review Report

**Reviewed:** 2026-04-14T14:21:36Z
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found

## Summary

Reviewed the phase scope with emphasis on the canonical path lookup changes in `gin.go` and the serializer hardening tests in `serialize_security_test.go`. The new `rebuildPathLookup()` guard correctly rejects out-of-order path IDs, but the decoder still has three hostile-input gaps in `serialize.go`: one compressed-input memory exhaustion path, one unbounded header allocation path, and one malformed bloom-filter crash path.

## Critical Issues

### CR-01: Compressed Decode Still Trusts Hostile Zstd Frame Sizes

**File:** `serialize.go:213-221`
**Issue:** `Decode()` creates a zstd decoder with default limits and immediately calls `DecodeAll(data[4:], nil)`. That means a crafted compressed index can force decompression of a very large frame before any of the format-level bounds checks run. The later `maxRGSetSize`, `maxTermsPerPath`, and related guards do not help once the process has already allocated the decompressed buffer.
**Fix:**
```go
const maxDecodedIndexSize = 64 << 20 // example, choose a repo-level ceiling

decoder, err := zstd.NewReader(nil, zstd.WithDecoderMaxMemory(maxDecodedIndexSize))
if err != nil {
	return nil, errors.Wrap(err, "create zstd decoder")
}
decompressed, err = decoder.DecodeAll(data[4:], nil)
if err != nil {
	return nil, errors.Wrap(err, "decompress data")
}
```
Also add a regression test that feeds `Decode()` a compressed frame whose declared decoded size exceeds the allowed ceiling and asserts `ErrInvalidFormat` (or the chosen wrapped error).

### CR-02: Header Counts Are Unbounded Before Slice Allocation

**File:** `serialize.go:327-336, 549-576, 634-668, 911-927`
**Issue:** `readHeader()` accepts arbitrary `NumRowGroups` and `NumDocs` values, and the downstream readers trust those counts as allocation ceilings. A crafted header can therefore drive large allocations in `readStringLengthIndexes()`, `readNumericIndexes()`, or `readDocIDMapping()` even though the file is malformed. The post-fix tests cover oversized section-local counts, but they do not cover oversized header counts, which remain the controlling upper bound.
**Fix:**
```go
const (
	maxNumRowGroups = 1 << 20
	maxNumDocs      = 1 << 30
)

if idx.Header.NumRowGroups == 0 || idx.Header.NumRowGroups > maxNumRowGroups {
	return errors.Wrapf(ErrInvalidFormat, "row group count %d exceeds max %d", idx.Header.NumRowGroups, maxNumRowGroups)
}
if idx.Header.NumDocs > maxNumDocs {
	return errors.Wrapf(ErrInvalidFormat, "doc count %d exceeds max %d", idx.Header.NumDocs, maxNumDocs)
}
if uint64(idx.Header.NumRowGroups) > uint64(^uint(0)>>1) {
	return errors.Wrap(ErrInvalidFormat, "row group count exceeds platform int range")
}
```
Apply the guard immediately after reading the header, before any other section reader runs, and add regression tests that corrupt `NumRowGroups` and `NumDocs` in an uncompressed payload.

### CR-03: Malformed Bloom Metadata Can Panic Queries After Decode

**File:** `serialize.go:415-437`
**Issue:** `readBloomFilter()` only caps `numWords`; it does not validate that `numBits > 0`, `numHashes > 0`, or that `numWords` matches `numBits`. Because it reconstructs the bloom filter with `BloomFilterFromBits()` instead of the validating constructor, a crafted payload can produce a decoded index that later panics in `MayContain()` due to modulo-by-zero or out-of-range bit access when a query hits the bloom fast path.
**Fix:**
```go
if numBits == 0 {
	return nil, errors.Wrap(ErrInvalidFormat, "bloom filter numBits must be > 0")
}
if numHashes == 0 {
	return nil, errors.Wrap(ErrInvalidFormat, "bloom filter numHashes must be > 0")
}
expectedWords := (numBits + 63) / 64
if numWords != expectedWords {
	return nil, errors.Wrapf(ErrInvalidFormat, "bloom filter word count %d does not match numBits %d", numWords, numBits)
}
```
Prefer reconstructing via `NewBloomFilter(numBits, numHashes)` and then copying the validated words in, and add security tests for zero `numBits`, zero `numHashes`, and mismatched `numWords`.

---

_Reviewed: 2026-04-14T14:21:36Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
