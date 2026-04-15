---
phase: 06
slug: query-path-hot-path
status: verified
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-14
updated: 2026-04-14T18:19:53Z
---

# Phase 06 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` and Go benchmark tooling |
| **Config file** | none - standard Go toolchain via `go.mod` |
| **Quick run command** | `go test ./... -run 'Test(JSONPath|WithFTSPaths|DateTransformer|ConfigSerialization|TransformerRoundTrip|.*Canonical.*Path|.*Duplicate.*Path|.*Unknown.*Path|QueryEQ)' -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | quick run ~2s, benchmark smoke ~40s, full suite ~308s |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -run 'Test(JSONPath|WithFTSPaths|DateTransformer|ConfigSerialization|TransformerRoundTrip|.*Canonical.*Path|.*Duplicate.*Path|.*Unknown.*Path|QueryEQ)' -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green and `go test ./... -run '^$' -bench 'Benchmark(PathLookup|Query(EQ|Contains|Regex))' -benchtime=1x -count=1` must complete without harness errors
- **Max feedback latency:** 308 seconds for repo-local validation

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 06-01-01 | 01 | 1 | PATH-01 | T-06-03 / T-06-04 | Query evaluation resolves supported paths through derived immutable lookup state and preserves safe no-pruning fallback for invalid or missing paths | unit + smoke | `go test ./... -run 'Test(FindPathCanonicalLookupAndFallback|QueryEQCanonicalPathDecodeParity)' -count=1` | ✅ `gin_test.go` | ✅ green |
| 06-01-02 | 01 | 1 | PATH-02 | T-06-01 / T-06-02 | Supported spellings canonicalize to one stored/query path, unsupported forms remain rejected, config bindings survive encode/decode, and duplicate canonical collisions fail clearly | unit + integration | `go test ./... -run 'Test(JSONPath|WithFTSPathsCanonicalizesEquivalentSupportedPaths|DateTransformerCanonicalConfigPath|DateTransformerDecodeCanonicalQueries|ConfigSerializationCanonical(Paths|QueryBehavior)|RebuildPathLookupRejectsDuplicateCanonicalPaths)' -count=1` | ✅ `gin_test.go`, `transformers_test.go`, `transformer_registry_test.go` | ✅ green |
| 06-02-01 | 02 | 2 | PATH-03 | T-06-05 / T-06-06 / T-06-07 | Wide-path EQ, CONTAINS, REGEX, and path-lookup benchmarks run on one deterministic fixture family with explicit naming for reproducible comparison | benchmark | `go test ./... -run '^$' -bench 'Benchmark(PathLookup|Query(EQ|Contains|Regex))' -benchtime=1x -count=1` | ✅ `benchmark_test.go` | ✅ green |

*Status: ⬜ pending - ✅ green - ❌ red - ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements.

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Audit 2026-04-14

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

Audit evidence:
- Reviewed `06-01-PLAN.md`, `06-02-PLAN.md`, and both phase summaries to map PATH-01, PATH-02, and PATH-03 to implementation and test artifacts.
- Confirmed current coverage in `gin_test.go`, `transformers_test.go`, `transformer_registry_test.go`, and `benchmark_test.go`.
- Verified green runs for the Phase 06 targeted test sets, benchmark smoke, and the full `go test ./... -count=1` suite on the current implementation state.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 308s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-04-14
