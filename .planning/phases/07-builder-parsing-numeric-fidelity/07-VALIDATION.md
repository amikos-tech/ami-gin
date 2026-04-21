---
phase: 07
slug: builder-parsing-numeric-fidelity
status: verified
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-15
updated: 2026-04-21T04:43:29Z
---

# Phase 07 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` and Go benchmark tooling |
| **Config file** | none - standard Go toolchain via `go.mod` |
| **Quick run command** | `go test ./... -run 'Test(AddDocumentRejectsUnsupportedNumberWithoutPartialMutation|NumericIndexPreservesInt64Exactness|MixedNumericPathRejectsLossyPromotion|IntOnlyNumericDecodeParity|TransformerNumericPathExplicitParserCompatibility|TransformerNumericDecodeParity)' -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | quick run ~2s, benchmark smoke ~70s, full suite ~40s |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -run 'Test(AddDocumentRejectsUnsupportedNumberWithoutPartialMutation|NumericIndexPreservesInt64Exactness|MixedNumericPathRejectsLossyPromotion|IntOnlyNumericDecodeParity|TransformerNumericPathExplicitParserCompatibility|TransformerNumericDecodeParity)' -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green and `go test ./... -run '^$' -bench 'Benchmark(AddDocumentPhase07|BuildPhase07|FinalizePhase07)' -benchtime=1x -count=1` must complete without harness errors
- **Max feedback latency:** 70 seconds for repo-local validation on this machine

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 07-01-01 | 01 | 1 | BUILD-01, BUILD-04 | T-07-01 / T-07-03 | Builder parses into document-local staging and rejects bad numeric input without partial mutation | unit + integration | `go test ./... -run 'Test(AddDocumentUsesExplicitParser|AddDocumentRejectsUnsupportedNumberWithoutPartialMutation)' -count=1` | ✅ `builder.go`, ✅ `gin_test.go` | ✅ green |
| 07-01-02 | 01 | 1 | BUILD-02, BUILD-03, BUILD-04 | T-07-01 / T-07-02 | Integer-only paths keep exact `int64` stats/query behavior and mixed paths fail on lossy promotion | unit + integration | `go test ./... -run 'Test(NumericIndexPreservesInt64Exactness|MixedNumericPathRejectsLossyPromotion|IntOnlyNumericDecodeParity)' -count=1` | ✅ `gin.go`, ✅ `query.go`, ✅ `serialize.go`, ✅ `gin_test.go` | ✅ green |
| 07-01-03 | 01 | 1 | BUILD-01, BUILD-02, BUILD-03, BUILD-04 | T-07-01 / T-07-02 / T-07-03 | Transformer-registered paths still use the same numeric classifier and decode/query parity as raw paths | unit + integration | `go test ./... -run 'Test(TransformerNumericPathExplicitParserCompatibility|TransformerNumericDecodeParity)' -count=1` | ✅ `builder.go`, ✅ `transformers_test.go`, ✅ `transformer_registry_test.go` | ✅ green |
| 07-02-01 | 02 | 2 | BUILD-05 | T-07-05 / T-07-06 / T-07-07 | Benchmarks compare `parser=legacy-unmarshal` and `parser=explicit-number` on the same fixtures and report alloc deltas | benchmark | `go test ./... -run '^$' -bench 'Benchmark(AddDocumentPhase07|BuildPhase07|FinalizePhase07)' -benchtime=1x -count=1` | ✅ `benchmark_test.go` | ✅ green |

*Status: ⬜ pending - ✅ green - ❌ red - ⚠ flaky*

---

## Wave 0 Requirements

- [x] `gin_test.go` - regression coverage exists for explicit parser usage, atomic failure, exact `int64` semantics, mixed-path promotion failure, and decode parity
- [x] `transformers_test.go` - transformed numeric path compatibility coverage exists for the explicit parser
- [x] `benchmark_test.go` - legacy-vs-explicit parser benchmark families exist for ingest/build latency and allocation deltas

---

## Manual-Only Verifications

All Phase 07 behaviors remain automatable with repo-local Go tests and benchmark smoke runs.

---

## Validation Audit 2026-04-21

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 1 |
| Escalated | 0 |

Audit evidence:
- Re-ran the current-tree targeted regression command exactly as required by Phase 12 and it passed: `go test ./... -run 'Test(AddDocumentRejectsUnsupportedNumberWithoutPartialMutation|NumericIndexPreservesInt64Exactness|MixedNumericPathRejectsLossyPromotion|IntOnlyNumericDecodeParity|TransformerNumericPathExplicitParserCompatibility|TransformerNumericDecodeParity)' -count=1` produced `ok github.com/amikos-tech/ami-gin 0.751s` and `ok github.com/amikos-tech/ami-gin/cmd/gin-index 1.065s [no tests to run]`.
- Re-ran the Phase 07 benchmark smoke harness exactly as required and it passed: `go test ./... -run '^$' -bench 'Benchmark(AddDocumentPhase07|BuildPhase07|FinalizePhase07)' -benchtime=1x -count=1` produced full parser-mode coverage for `docs=100`, `1000`, and `10000` across `shape=int-only`, `shape=mixed-safe`, `shape=wide-flat`, and `shape=transformer-heavy`, then exited `PASS` with `ok github.com/amikos-tech/ami-gin 69.774s`.
- Benchmark smoke summary from the current tree: `BenchmarkAddDocumentPhase07` shows explicit-number remains competitive on simple int-only ingest (`docs=10000/shape=int-only`: `25.125µs`, `37648 B/op`, `207 allocs/op` vs legacy `28.625µs`, `34448 B/op`, `128 allocs/op`); `BenchmarkBuildPhase07` still exposes higher wide-flat staging cost for the explicit parser (`docs=10000/shape=wide-flat`: `16.225s`, `9.53 GB`, `185661665 allocs/op` vs legacy `13.164s`, `5.80 GB`, `113391664 allocs/op`); `BenchmarkFinalizePhase07` confirms BUILD-05 coverage includes finalize deltas as well (`docs=10000/shape=wide-flat`: explicit `146.9ms` vs legacy `179.8ms`).
- Re-ran the repo-wide regression suite cited by the milestone audit and it passed: `go test ./... -count=1` produced `ok github.com/amikos-tech/ami-gin 38.735s` and `ok github.com/amikos-tech/ami-gin/cmd/gin-index 1.672s`.
- Confirmed the shipped ingest path is still explicit-number and transactional on the current tree: `AddDocument()` stages through `parseAndStageDocument()` and only merges via `mergeDocumentState()` after parse success (`builder.go:287-305`), the parser uses `json.NewDecoder(...).UseNumber()` with `documentBuildState` (`builder.go:322-345`), and transformer subtree materialization is only reached from the representation branch rather than a whole-document fallback decode (`builder.go:497-598`).
- Confirmed exact-int semantics remain wired end-to-end on the current tree: numeric mode fields live in `gin.go:204-220`, lossless-promotion enforcement uses `maxExactFloatInt := int64(1 << 53)` in `builder.go:654-750`, int-only query comparisons are handled in `query.go:276-303` and `query.go:333-499`, and encode/decode persists the exact-int fields in `serialize.go:1066-1175`.
- Confirmed the live regression assets match the validation contract: `gin_test.go:2933-3233` covers atomic failure, exact `int64` fidelity, lossy-promotion rejection, and decode parity; `transformers_test.go:1274-1365` covers explicit-parser transformer compatibility and numeric decode parity; `benchmark_test.go:206-229,549-565,972-1065` defines the Phase 07 benchmark matrix and benchmark-only legacy control path.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 310s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-04-21
