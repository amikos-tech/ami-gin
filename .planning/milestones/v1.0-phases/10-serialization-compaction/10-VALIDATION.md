---
phase: 10
slug: serialization-compaction
status: verified
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-17
---

# Phase 10 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` and Go benchmark tooling |
| **Config file** | none - standard Go toolchain via `go.mod` |
| **Quick run command** | `go test ./... -run 'Test(PathDirectoryCompactionRoundTrip|StringIndexCompactionRoundTrip|AdaptiveStringIndexCompactionRoundTrip|DecodeRejectsCompactPathSectionCorruption|DecodeRejectsCompactTermSectionCorruption|DecodeRejectsOrderedStringModePayloadMismatch|ReadOrderedStringsRejectsFrontCodedOversized(Block|Entry)Count|WriteOrderedStringsPrefersRawOnTie|DecodeVersionMismatch|DecodeLegacyRejected|DecodeRepresentationAliasParity|DecodeRejectsTruncatedAdaptiveTerm)' -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | quick run ~8s, benchmark smoke ~3s, benchmark evidence ~20s, full suite ~120s |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -run 'Test(PathDirectoryCompactionRoundTrip|StringIndexCompactionRoundTrip|AdaptiveStringIndexCompactionRoundTrip|DecodeRejectsCompactPathSectionCorruption|DecodeRejectsCompactTermSectionCorruption|DecodeRejectsOrderedStringModePayloadMismatch|ReadOrderedStringsRejectsFrontCodedOversized(Block|Entry)Count|WriteOrderedStringsPrefersRawOnTie|DecodeVersionMismatch|DecodeLegacyRejected|DecodeRepresentationAliasParity|DecodeRejectsTruncatedAdaptiveTerm)' -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `$gsd-verify-work`:** Full suite must be green, `go test ./... -run '^$' -bench 'BenchmarkPhase10SerializationCompaction' -benchtime=1x -count=1` must pass as a harness smoke check, and `go test ./... -run '^$' -bench 'BenchmarkPhase10SerializationCompaction' -benchtime=1s -count=3 -benchmem` must produce the final timing evidence
- **Max feedback latency:** 120 seconds for repo-local validation

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 10-01-01 | 01 | 1 | SIZE-01, SIZE-02 | T-10-01 / T-10-02 | Compact path-directory and term encoding preserve exact order, metadata binding, and RG alignment while remaining explicitly mode-tagged | unit + serialization | `go test ./... -run 'Test(PathDirectoryCompactionRoundTrip|StringIndexCompactionRoundTrip|AdaptiveStringIndexCompactionRoundTrip)' -count=1` | ✅ `gin_test.go` | ✅ green |
| 10-02-01 | 02 | 2 | SIZE-01, SIZE-02 | T-10-04 / T-10-10 | Compact ordered-string sections reject corruption, mode/payload disagreement, oversized front-coded block and entry counts, and nondeterministic fallback behavior | unit + security | `go test ./... -run 'Test(DecodeRejectsCompactPathSectionCorruption|DecodeRejectsCompactTermSectionCorruption|DecodeRejectsOrderedStringModePayloadMismatch|ReadOrderedStringsRejectsFrontCodedOversized(Block|Entry)Count|WriteOrderedStringsPrefersRawOnTie|DecodeRejectsTruncatedAdaptiveTerm)' -count=1` | ✅ `serialize_security_test.go` | ✅ green |
| 10-02-02 | 02 | 2 | SIZE-03 | T-10-05 / T-10-06 | New wire version is explicit, legacy payloads are rejected cleanly, and compact round trips preserve representation alias semantics | unit + security | `go test ./... -run 'Test(DecodeVersionMismatch|DecodeLegacyRejected|DecodeRepresentationAliasParity)' -count=1` | ✅ `serialize_security_test.go` | ✅ green |
| 10-03-01 | 03 | 2 | SIZE-01, SIZE-02, SIZE-03 | T-10-07 / T-10-08 / T-10-09 | Representative fixtures show raw-wire and zstd-compressed size deltas with reproducible encode/decode timing and post-decode query smoke coverage | benchmark | `go test ./... -run '^$' -bench 'BenchmarkPhase10SerializationCompaction' -benchtime=1s -count=3 -benchmem` | ✅ `benchmark_test.go` | ✅ green |

*Status: ⬜ pending - ✅ green - ❌ red - ⚠ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements.

---

## Manual-Only Verifications

All phase behaviors should remain automatable with repo-local tests and benchmarks.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved
