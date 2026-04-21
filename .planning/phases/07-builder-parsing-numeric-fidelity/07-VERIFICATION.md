---
phase: 07-builder-parsing-numeric-fidelity
verified: 2026-04-21T04:43:29Z
status: passed
score: 5/5 must-haves verified
overrides_applied: 0
---

# Phase 07: Builder Parsing & Numeric Fidelity Verification Report

**Phase Goal:** Builder ingest uses explicit numeric parsing with transactional staging, exact-int path semantics, transformer compatibility, and reproducible parser-delta benchmark evidence.
**Verified:** 2026-04-21T04:43:29Z
**Status:** passed
**Re-verification:** Yes — evidence reconstructed from the current tree during Phase 12 closeout

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | `AddDocument()` no longer relies on eager whole-document `json.Unmarshal(..., &any)` and only mutates builder-global state after parse success. | ✓ VERIFIED | `builder.go:287-305` stages through `parseAndStageDocument()` and merges with `mergeDocumentState()` only after success; `builder.go:322-345` uses `json.NewDecoder(...).UseNumber()` and `documentBuildState`; the targeted regression command passed on HEAD. |
| 2 | Integer-only numeric paths preserve exact `int64` semantics, while mixed numeric paths fail when promotion would be lossy. | ✓ VERIFIED | `builder.go:654-750` tracks int-only vs float-or-mixed mode and rejects unsafe promotion via `maxExactFloatInt := int64(1 << 53)`; `gin.go:204-220` stores exact-int metadata; `query.go:276-303,333-499` uses int-aware equality/range evaluation; `gin_test.go:2988-3233` proves exactness, lossy-promotion rejection, and decode parity. |
| 3 | Unsupported numeric values fail safely with path-aware errors and do not leak partial document state into the builder or finalized index. | ✓ VERIFIED | `builder.go:598-607` wraps numeric parse failures with path context; `gin_test.go:2933-2986` verifies rejected documents do not mutate `docIDToPos`, `nextPos`, `pathData`, or finalized indexes; the targeted regression command passed on HEAD. |
| 4 | Transformer-configured numeric paths still flow through the same explicit classifier and preserve encode/decode parity. | ✓ VERIFIED | `builder.go:497-598` stages transformed values through `stageMaterializedValue()` and `stageCompanionRepresentations()`; `transformers_test.go:1274-1365` verifies raw child-path indexing, derived numeric queries, and encode/decode parity for numeric transformer paths; the targeted regression command passed on HEAD. |
| 5 | BUILD-05 benchmark evidence is reproducible on the current tree with an in-repo legacy control and explicit parser-mode labeling. | ✓ VERIFIED | `benchmark_test.go:206-229` defines the fixed doc-count/shape matrix; `benchmark_test.go:549-565` keeps the benchmark-only legacy control path; `benchmark_test.go:972-1065` defines `BenchmarkAddDocumentPhase07`, `BenchmarkBuildPhase07`, and `BenchmarkFinalizePhase07`; the benchmark smoke command passed on HEAD and emitted both parser modes across all shapes/doc counts. |

**Score:** 5/5 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `builder.go` | Explicit-number streaming parser with document-local staging and merge-on-success ingest | ✓ VERIFIED | `builder.go:287-345,497-760` implements transactional staging, `UseNumber()`, and merge-after-success semantics. |
| `gin.go` | Exact-int numeric metadata in the index model | ✓ VERIFIED | `gin.go:204-220` defines `ValueType`, `IntGlobalMin`, `IntGlobalMax`, `IntMin`, and `IntMax`. |
| `query.go` | Int-aware equality and range evaluation for int-only paths | ✓ VERIFIED | `query.go:276-303,333-499` routes integral predicates through exact-int comparison on int-only paths. |
| `serialize.go` | Encode/decode support for exact-int numeric metadata | ✓ VERIFIED | `serialize.go:1066-1175` writes and reads `IntGlobalMin`, `IntGlobalMax`, `IntMin`, and `IntMax`. |
| `gin_test.go` | Regression coverage for atomic failure, exact `int64` fidelity, mixed-promotion rejection, and decode parity | ✓ VERIFIED | `gin_test.go:2933-3233` contains the live Phase 07 regression set. |
| `transformers_test.go` | Transformer numeric compatibility and decode-parity coverage | ✓ VERIFIED | `transformers_test.go:1274-1365` covers transformed numeric path behavior before and after `Encode()` / `Decode()`. |
| `benchmark_test.go` | Legacy-vs-explicit Phase 07 benchmark families with explicit parser/docs/shape labels | ✓ VERIFIED | `benchmark_test.go:206-229,549-565,972-1065` defines the matrix, legacy control path, and benchmark families. |
| `07-VALIDATION.md` | Current-tree Phase 07 Nyquist state resolved honestly from fresh evidence | ✓ VERIFIED | `07-VALIDATION.md` now carries a 2026-04-21 audit with all task rows green, checked sign-off, and approved closeout. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Phase 07 targeted parser/numeric regressions | `go test ./... -run 'Test(AddDocumentRejectsUnsupportedNumberWithoutPartialMutation|NumericIndexPreservesInt64Exactness|MixedNumericPathRejectsLossyPromotion|IntOnlyNumericDecodeParity|TransformerNumericPathExplicitParserCompatibility|TransformerNumericDecodeParity)' -count=1` | Passed on HEAD: `ok github.com/amikos-tech/ami-gin 0.751s`; `ok github.com/amikos-tech/ami-gin/cmd/gin-index 1.065s [no tests to run]`. | ✓ PASS |
| Phase 07 benchmark smoke | `go test ./... -run '^$' -bench 'Benchmark(AddDocumentPhase07|BuildPhase07|FinalizePhase07)' -benchtime=1x -count=1` | Passed on HEAD: emitted both `parser=legacy-unmarshal` and `parser=explicit-number` for `docs=100`, `1000`, and `10000` across `shape=int-only`, `shape=mixed-safe`, `shape=wide-flat`, and `shape=transformer-heavy`, then exited `PASS` with `ok github.com/amikos-tech/ami-gin 69.774s`. Representative deltas: explicit-number stayed competitive on `AddDocument/int-only/docs=10000` (`25.125µs` vs `28.625µs`) while remaining heavier on `Build/wide-flat/docs=10000` (`16.225s`, `9.53 GB`, `185661665 allocs/op` vs `13.164s`, `5.80 GB`, `113391664 allocs/op`). | ✓ PASS |
| Full repository regression suite | `go test ./... -count=1` | Passed on HEAD: `ok github.com/amikos-tech/ami-gin 38.735s`; `ok github.com/amikos-tech/ami-gin/cmd/gin-index 1.672s`. | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| `BUILD-01` | `07-01-PLAN.md` | Primary ingest path no longer relies on eager whole-document `json.Unmarshal(..., &any)` | ✓ SATISFIED | `builder.go:287-345` stages via `json.Decoder.UseNumber()` and `documentBuildState`; `gin_test.go:2933-2986` proves atomic failure behavior. |
| `BUILD-02` | `07-01-PLAN.md` | Integer-vs-float classification is based on explicit number parsing rather than generic `float64` decoding | ✓ SATISFIED | `builder.go:598-620` parses numeric literals from source text; `builder.go:654-724` maintains numeric mode during staging; targeted regressions passed. |
| `BUILD-03` | `07-01-PLAN.md` | Supported integers preserve precision before stats/bitmap decisions are made | ✓ SATISFIED | `gin.go:204-220`, `query.go:276-303,333-499`, `serialize.go:1066-1175`, and `gin_test.go:2988-3233` preserve and verify exact `int64` semantics end to end. |
| `BUILD-04` | `07-01-PLAN.md` | Unsupported or unrepresentable numeric values fail safely with explicit errors | ✓ SATISFIED | `builder.go:598-607,688-702` returns path-aware errors for unsupported numerics and lossy promotion; `gin_test.go:2933-2986,3171-3195` verifies failure safety and no partial mutation. |
| `BUILD-05` | `07-02-PLAN.md` | Benchmarks capture ingest/build/finalize latency and allocation deltas for the parser redesign | ✓ SATISFIED | `benchmark_test.go:206-229,549-565,972-1065` provides the fixed benchmark matrix and legacy control; the benchmark smoke command passed on HEAD with both parser modes across all four shapes and three doc counts. |

### Gaps Summary

None.

---

_Verified: 2026-04-21T04:43:29Z_  
_Verifier: Codex (phase closeout verification)_
