---
phase: 07-builder-parsing-numeric-fidelity
reviewed: 2026-04-15T11:50:50Z
depth: standard
files_reviewed: 9
files_reviewed_list:
  - benchmark_test.go
  - builder.go
  - gin.go
  - gin_test.go
  - query.go
  - serialize.go
  - serialize_security_test.go
  - transformer_registry_test.go
  - transformers_test.go
findings:
  critical: 0
  warning: 2
  info: 0
  total: 2
status: issues_found
---

# Phase 07: Code Review Report

**Reviewed:** 2026-04-15T11:50:50Z
**Depth:** standard
**Files Reviewed:** 9
**Status:** issues_found

## Summary

Reviewed the Phase 07 parser/numeric-fidelity changes across the builder, query evaluator, serializer, and the new regression/benchmark coverage. `go test ./...` passes, but two warning-level regressions remain in the new staging/query paths.

The main concerns are:
- int-only numeric indexes stop pruning valid fractional range predicates and fall back to `AllRGs`
- transformers reached through materialized subtrees still receive raw `json.Number` payloads, which breaks the “legacy transformer input shape” compatibility goal for array/wildcard paths

## Warnings

### WR-01: Int-Only Range Queries Drop Fractional Predicate Pruning

**File:** `/Users/tazarov/experiments/amikos/custom-gin/query.go:156-160,198-202,241-243,283-285`
**Issue:** The new int-only branches in `evaluateGT/GTE/LT/LTE` only accept bounds that round-trip through `toExactInt64`. Valid predicates like `GT("$.score", 1.5)` or `LTE("$.score", 1.5)` now return `AllRGs` for int-only paths instead of using the stored integer min/max stats. Before Phase 07, the float-backed numeric stats still pruned these cases. That changes `Evaluate()` output for valid predicates and there is no regression test covering non-integral bounds against int-only indexes.
**Fix:** Convert fractional bounds to the nearest integer boundary instead of falling back immediately:

```go
// Example for GT on an int-only path.
f, ok := value.(float64)
if ok && !math.IsNaN(f) && !math.IsInf(f, 0) && f != math.Trunc(f) {
	queryInt := int64(math.Floor(f))
	// compare IntMax > queryInt
}
```

Add coverage for `GT/GTE/LT/LTE` with fractional bounds on an int-only index.

### WR-02: Transformers Under Materialized Subtrees Still See Raw `json.Number`

**File:** `/Users/tazarov/experiments/amikos/custom-gin/builder.go:273-283,375-446`
**Issue:** Phase 07 added `prepareTransformerValue()` to preserve the old `json.Unmarshal`-style transformer contract, but that normalization only happens in `decodeTransformedValue()`. Array elements and other materialized subtrees are routed through `stageMaterializedValue()`, which invokes transformers with the raw `decodeAny()` result. Under `UseNumber`, that means nested transformers on paths like `$.items[*]` or `$.items[*].metrics` now receive `json.Number` / maps containing `json.Number` instead of the legacy `float64` shapes that existing custom transformers and built-ins like `NumericBucket`/`BoolNormalize` expect.
**Fix:** Normalize before every transformer invocation in `stageMaterializedValue()`, not just in `decodeTransformedValue()`:

```go
if allowTransform && b.config.fieldTransformers != nil {
	if transformer, ok := b.config.fieldTransformers[canonicalPath]; ok {
		if transformed, ok := transformer(prepareTransformerValue(value)); ok {
			value = transformed
		}
	}
}
```

Add a regression test for a transformer attached to a wildcard/array-backed path (for example `$.items[*].metrics`) with numeric descendants.

---

_Reviewed: 2026-04-15T11:50:50Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
