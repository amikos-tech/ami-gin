---
task: quick-260729-mg8
verified: 2026-07-29T00:00:00Z
status: passed
score: 5/5 must-haves verified
overrides_applied: 0
---

# Quick Task 260729-mg8: Fix SIMD Parser Adapter TypeBigInt Handling Verification Report

**Task Goal:** Fix the SIMD parser adapter so pure-simdjson's new `TypeBigInt` element kind (added in v0.1.7) routes out-of-range integers through the NUMERIC failure policy at the correct JSON path, matching stdlib parser parity — unblocking dependabot PR #48.

**Verified:** 2026-07-29
**Status:** passed
**Branch:** `dependabot/go_modules/gomod-minor-and-patch-1bec9f6472` (correct — v0.1.7 lives here)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `walkElement` routes `TypeBigInt` through `sink.StageJSONNumber` at the correct canonical path | VERIFIED | `parser_simd.go:261-266` — `case purejson.TypeBigInt:` calls `element.GetBigInt()` then `sink.StageJSONNumber(state, canonicalPath, raw)`. Confirmed `StageJSONNumber` (`parser_sink.go:93-95`) routes to `b.stageJSONNumberLiteral` — the numeric-failure-policy implementation. |
| 2 | `materializeElement` routes `TypeBigInt` to `json.Number(raw)`, converging on the same numeric policy as the walk path | VERIFIED | `parser_simd.go:356-361` returns `json.Number(raw), nil`. `builder.go:593-594` (`stageMaterializedValue`) has `case json.Number: return b.stageJSONNumberLiteral(canonicalPath, v.String(), state)` — the identical function called by `StageJSONNumber`. Both paths provably converge on one implementation; cannot drift. |
| 3 | Fail-closed `default:` arms in both switches remain intact — new cases were additive, not substitutive | VERIFIED | `git show 52ded55 -- parser_simd.go` shows both hunks are pure additions (`+` lines only) inserted before the existing `case purejson.TypeInvalid:` line; `default:` arms unchanged in both switches. |
| 4 | CI SIMD job bootstrap pin matches go.mod's pure-simdjson version | VERIFIED | `.github/workflows/ci.yml:112` pins `pure-simdjson-bootstrap@v0.1.7`; `go.mod` pins `github.com/amikos-tech/pure-simdjson v0.1.7`. Exact match. |
| 5 | No scope creep — diff confined to `parser_simd.go` and `.github/workflows/ci.yml` | VERIFIED | `git diff e7b08dd..HEAD --stat -- . ':!.planning'` (baseline = pre-existing dependabot go.mod/go.sum bump commit, prior to this quick task) shows only `parser_simd.go` (+12) and `.github/workflows/ci.yml` (+1/-1) changed. |

**Score:** 5/5 truths verified

### Causal Proof (regression reproduction)

To confirm the fix is genuinely load-bearing and not coincidentally passing, the `TypeBigInt` hunks were temporarily reverted (`git apply -R` on the `52ded55` diff) and `TestSIMDParserNativeNumericFailuresUseNumericPolicy/larger-than-uint64` was re-run:

```
--- FAIL: .../larger-than-uint64/hard-numeric-soft-parser
    AddDocument error = <nil>, want *IngestError
--- FAIL: .../larger-than-uint64/soft-numeric-hard-parser
    AddDocument: ingest parser failure: unsupported pure-simdjson element type 9 at $.n
--- FAIL: .../larger-than-uint64/hard-numeric-hard-parser
    IngestError = (layer="parser", path=""), want (numeric, $.n)
```

This confirms: without the fix, element type 9 (`TypeBigInt`) falls into the `default:` arm and surfaces as `IngestLayerParser` (not `IngestLayerNumeric`, and at the wrong/empty path) — exactly the parity break described in the task goal. The working tree was restored (`git checkout -- parser_simd.go`) immediately after, and `git status --short` confirmed a clean tree.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `parser_simd.go` | `TypeBigInt` handling in `walkElement` and `materializeElement` | VERIFIED | Both cases present, additive only (see above) |
| `.github/workflows/ci.yml` | SIMD job bootstrap pinned to v0.1.7 | VERIFIED | `pure-simdjson-bootstrap@v0.1.7 fetch` on line 112, single line changed |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `parser_simd.go:261` `walkElement` TypeBigInt case | `parserSink.StageJSONNumber` | direct call with `element.GetBigInt()` raw text | WIRED | `sink.StageJSONNumber(state, canonicalPath, raw)` — grep pattern `StageJSONNumber\(state, canonicalPath` matches |
| `parser_simd.go:356` `materializeElement` TypeBigInt case | `json.Number` | `element.GetBigInt()` wrapped as `json.Number` | WIRED | `return json.Number(raw), nil` — grep pattern `json\.Number\(` matches |
| `GINBuilder.StageJSONNumber` (parser_sink.go:93) | `stageJSONNumberLiteral` | direct call | WIRED | `return tagStageError(b.stageJSONNumberLiteral(canonicalPath, raw, state))` |
| `stageMaterializedValue` `case json.Number` (builder.go:593) | `stageJSONNumberLiteral` | direct call | WIRED | Confirms walk and materialize paths converge on the identical numeric-policy function |

### Behavioral Spot-Checks / Test Execution

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build with simdjson tag | `go build -tags simdjson ./...` | exit 0 | PASS |
| Build without simdjson tag (default build unaffected) | `go build ./...` | exit 0 | PASS |
| `go vet -tags simdjson ./...` | | exit 0, no output | PASS |
| Isolation check (fail-closed build-tag boundary) | `make simd-isolation-check` | exit 0 | PASS |
| Target test, fresh (non-cached) run | `go clean -testcache && go test -tags simdjson -run TestSIMDParserNativeNumericFailuresUseNumericPolicy -v .` | `ok  github.com/amikos-tech/ami-gin  0.488s`, all 15 subtests (3 numeric cases × 4-5 modes each incl. `overflowed-exponent`, `uint64-above-int64`, `larger-than-uint64`) PASS | PASS |
| Full simdjson-tagged test suite | `go test -tags simdjson ./...` | all packages `ok` | PASS |
| Regression reproduction (revert fix) | see Causal Proof above | 3/4 `larger-than-uint64` subtests FAIL without the fix, confirming necessity | PASS (confirms fix is load-bearing) |

Native pure-simdjson v0.1.7 library was already bootstrapped locally at the provided `PURE_SIMDJSON_CACHE_DIR` path; all commands above were executed directly by the verifier, not copied from SUMMARY.md claims.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|--------------|--------|----------|
| SIMD-04 | Phase 21 (pre-existing) | Callers can explicitly select same-package SIMD parser without changing default stdlib path | SATISFIED (unaffected) | Not touched by this fix; parser seam unchanged |
| SIMD-05 | Phase 21 (pre-existing) | Default builds remain stdlib-only, no SIMD dependency unless explicit | SATISFIED (unaffected) | `go build ./...` (no tag) passes; simdjson code is entirely behind `//go:build simdjson` |
| SIMD-06 | Phase 21 (pre-existing) | SIMD parsing preserves exact-int numeric semantics, no silent float64 coercion | SATISFIED (restored) | This fix directly restores SIMD-06 compliance for the new `TypeBigInt` kind — out-of-range integers now route through numeric policy instead of silently erroring as parser-layer failures or (worse) being silently mis-typed |
| SIMD-07 | Phase 21 (pre-existing) | Parser sink exposes typed scalar fast paths so SIMD scalars don't round-trip through `any` | SATISFIED (unaffected) | `StageJSONNumber` is one of the existing typed fast-path methods on `parserSink`; reused, not modified |

These four requirements were already marked complete under Phase 21 in `REQUIREMENTS.md`; this quick task is a regression fix restoring SIMD-06 parity that the pure-simdjson v0.1.7 dependency bump broke, not new requirement work.

### Anti-Patterns Found

None. Scanned `parser_simd.go` diff (`git show 52ded55`) and `.github/workflows/ci.yml` diff — no TODO/FIXME/XXX/HACK/PLACEHOLDER markers, no stub returns, no hardcoded empty values. Both new cases follow the existing pattern of sibling numeric cases (`TypeInt64`/`TypeUint64`/`TypeFloat64`) exactly.

### Human Verification Required

None. All truths verified programmatically via direct code inspection, build, vet, isolation check, and live test execution (including a causal regression-reproduction run) against the native pure-simdjson v0.1.7 library.

### Gaps Summary

No gaps found. The fix is minimal, additive, correctly wired end-to-end (both `walkElement` and `materializeElement` converge on the identical `stageJSONNumberLiteral` numeric-policy implementation), does not touch the fail-closed `default:` safety net, and was proven load-bearing via a live revert-and-rerun regression check. The CI pin bump matches go.mod exactly. Diff scope is confined to the two files declared in the plan.

---

_Verified: 2026-07-29_
_Verifier: Claude (gsd-verifier)_
