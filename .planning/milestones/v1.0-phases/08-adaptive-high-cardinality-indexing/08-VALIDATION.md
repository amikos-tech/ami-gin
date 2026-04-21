---
phase: 08
slug: adaptive-high-cardinality-indexing
status: verified
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-15
updated: 2026-04-16T06:44:02Z
---

# Phase 08 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` and Go benchmark tooling |
| **Config file** | none - standard Go toolchain via `go.mod` |
| **Quick run command** | `go test ./... -run 'Test(AdaptivePromotesHotTermsToExactBitmaps|AdaptiveFallbackHasNoFalseNegatives|AdaptiveNegativePredicatesStayConservative|PropertyIntegrationCardinalityThreshold|AdaptiveConfigRoundTrip|AdaptivePathMetadataRoundTrip|PathInfoReportsAdaptiveMode|CLIInfoShowsAdaptiveSummary)' -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | quick run ~4s, benchmark smoke ~1s, full suite ~35s |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -run 'Test(AdaptivePromotesHotTermsToExactBitmaps|AdaptiveFallbackHasNoFalseNegatives|AdaptiveNegativePredicatesStayConservative|PropertyIntegrationCardinalityThreshold|AdaptiveConfigRoundTrip|AdaptivePathMetadataRoundTrip|PathInfoReportsAdaptiveMode|CLIInfoShowsAdaptiveSummary)' -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green and `go test ./... -run '^$' -bench 'BenchmarkAdaptiveHighCardinality' -benchtime=1x -count=1` must complete without harness errors
- **Max feedback latency:** 35 seconds for repo-local validation

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 08-01-01 | 01 | 1 | HCARD-01, HCARD-02 | T-08-01 / T-08-02 | Builder promotes only bounded hot terms to exact bitmaps using deterministic row-group coverage rules | unit + integration | `go test ./... -run 'TestAdaptivePromotesHotTermsToExactBitmaps' -count=1` | ✅ `gin_test.go` | ✅ green |
| 08-01-02 | 01 | 1 | HCARD-03 | T-08-01 / T-08-03 | Non-promoted terms fall back through deterministic buckets with no false negatives and no lossy negative inversion | integration + property | `go test ./... -run 'Test(AdaptiveFallbackHasNoFalseNegatives|AdaptiveNegativePredicatesStayConservative|PropertyIntegrationCardinalityThreshold)' -count=1` | ✅ `gin_test.go`, `integration_property_test.go` | ✅ green |
| 08-02-01 | 02 | 2 | HCARD-02, HCARD-04 | T-08-04 / T-08-05 / T-08-06 | Adaptive mode, counters, config, and mode-aware metadata survive encode/decode and appear in `gin-index info` output | unit + serialization + CLI | `go test ./... -run 'Test(AdaptiveConfigRoundTrip|AdaptivePathMetadataRoundTrip|PathInfoReportsAdaptiveMode|CLIInfoShowsAdaptiveSummary)' -count=1` | ✅ `serialize_security_test.go`, `cmd/gin-index/main_test.go` | ✅ green |
| 08-03-01 | 03 | 2 | HCARD-05 | T-08-07 / T-08-08 | Benchmarks report candidate row-group pruning improvement and bounded size growth on skewed high-cardinality data | benchmark | `go test ./... -run '^$' -bench 'BenchmarkAdaptiveHighCardinality' -benchtime=1x -count=1` | ✅ `benchmark_test.go` | ✅ green |

*Status: ⬜ pending - ✅ green - ❌ red - ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements.

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Audit 2026-04-16

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

Audit evidence:
- Confirmed State A input: existing `08-VALIDATION.md` plus executed `08-01` through `08-03` summary artifacts.
- Reviewed `08-01-PLAN.md`, `08-02-PLAN.md`, `08-03-PLAN.md`, and all three phase summaries to map `HCARD-01` through `HCARD-05` onto the implemented tasks and expected verification commands.
- Confirmed `.planning/config.json` does not set `workflow.nyquist_validation`; per GSD defaults this means Nyquist validation remains enabled because the key is absent rather than `false`.
- Cross-referenced the live tree and found all planned verification assets in place: `gin_test.go`, `integration_property_test.go`, `serialize_security_test.go`, `cmd/gin-index/main_test.go`, and `benchmark_test.go`.
- Verified focused adaptive regressions with `go test ./... -run 'Test(AdaptivePromotesHotTermsToExactBitmaps|AdaptiveFallbackHasNoFalseNegatives|AdaptiveNegativePredicatesStayConservative)' -count=1`.
- Verified the three-mode threshold property with `go test ./... -run 'TestPropertyIntegrationCardinalityThreshold' -count=1`.
- Verified serialization and CLI metadata coverage with `go test ./... -run 'Test(AdaptiveConfigRoundTrip|AdaptivePathMetadataRoundTrip|PathInfoReportsAdaptiveMode|CLIInfoShowsAdaptiveSummary)' -count=1`.
- Verified benchmark evidence with `go test ./... -run '^$' -bench 'BenchmarkAdaptiveHighCardinality' -benchtime=1x -count=1`, which reported hot-probe `candidate_rgs` of `48` for `mode=adaptive-hybrid` versus `96` for `mode=bloom-only`, and adaptive `encoded_bytes` of `18657`.
- Verified the repo-wide regression sweep with `go test ./... -count=1`, which passed in `34.609s` for `github.com/amikos-tech/ami-gin` on this machine.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 35s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-04-16
