---
phase: 08-adaptive-high-cardinality-indexing
reviewed: 2026-04-15T20:14:35Z
depth: standard
files_reviewed: 11
files_reviewed_list:
  - README.md
  - benchmark_test.go
  - builder.go
  - cmd/gin-index/main.go
  - cmd/gin-index/main_test.go
  - gin.go
  - gin_test.go
  - integration_property_test.go
  - query.go
  - serialize.go
  - serialize_security_test.go
findings:
  critical: 0
  warning: 4
  info: 0
  total: 4
status: issues_found
---

# Phase 08: Code Review Report

**Reviewed:** 2026-04-15T20:14:35Z
**Depth:** standard
**Files Reviewed:** 11
**Status:** issues_found

## Summary

Reviewed the current branch state for the phase-scoped files after commit `42be820`, with emphasis on adaptive high-cardinality behavior, CLI regressions, serialization safety, and test coverage quality. No critical or obvious security issues remain in the reviewed files, and `go test ./...` passes on the current branch, but four warning-level issues are still open.

The main risks are user-facing: the CLI's documented glob workflow does not work, exact `int64` predicates are silently rounded in CLI parsing, numeric-path cardinality metadata is wrong, and the property suite still masks setup/encode failures that should fail the run.

## Warnings

### WR-01: CLI glob inputs are treated as literal files

**File:** `cmd/gin-index/main.go:593-611`, `cmd/gin-index/main.go:638-656`, `README.md:527-529`
**Issue:** `resolveParquetFiles` and `resolveIndexFiles` return immediately when the input string ends with `.parquet` or `.gin`. Local glob inputs like `./data/*.parquet` and `./data/*.gin` also satisfy that suffix check, so the CLI never expands them and instead treats the pattern as a single literal path. That directly contradicts the documented examples in the README.
**Fix:**
```go
if !gin.IsS3Path(path) && strings.ContainsAny(path, "*?[") {
	matches, err := filepath.Glob(path)
	// filter matches by suffix here
}
if strings.HasSuffix(path, ".parquet") {
	return []string{path}, nil
}
```
Add a regression test that exercises `resolveParquetFiles("./tmp/*.parquet")` and `resolveIndexFiles("./tmp/*.gin")`.

### WR-02: CLI predicate parsing loses exact int64 values

**File:** `cmd/gin-index/main.go:744-763`
**Issue:** `parseValue` converts every successfully parsed integer to `float64`, and `parseValueList` uses `json.Unmarshal` into `[]any`, which also materializes numbers as `float64`. The core library now supports exact `int64` equality/range matching, but the CLI cannot express those predicates reliably for values above `2^53`. A query like `$.score = 9223372036854775807` will be rounded before it reaches `Evaluate`, and large `IN`/`NOT IN` lists have the same problem.
**Fix:**
```go
if i, err := strconv.ParseInt(s, 10, 64); err == nil {
	return i
}
```
For list parsing, decode with `json.Decoder.UseNumber()` and convert each `json.Number` to `int64` when possible, falling back to `float64` only when needed. Add CLI tests for single-value and `IN` exact-`int64` predicates.

### WR-03: Numeric paths always report cardinality 0

**File:** `builder.go:808-810`, `builder.go:826-878`, `builder.go:988-1008`
**Issue:** Only `addStringTerm` updates the per-path HyperLogLog. Numeric observations update bloom and min/max state, but never feed `pd.hll`, so `Finalize()` emits `PathEntry.Cardinality == 0` for numeric fields. Repro on the current branch: indexing `{"num":1}`, `{"num":2}`, `{"num":3}` reports `$ 0` and `$.num 0`. That makes `gin-index info` and any metadata consumers inaccurate for numeric paths.
**Fix:**
```go
if observation.isInt {
	pd.hll.AddString(strconv.FormatInt(observation.intVal, 10))
} else {
	pd.hll.AddString(strconv.FormatFloat(observation.floatVal, 'f', -1, 64))
}
```
Add a unit test asserting that a numeric path's `PathEntry.Cardinality` is non-zero and tracks distinct numeric values.

### WR-04: Property tests still hide setup and encode failures

**File:** `integration_property_test.go:29-31`, `integration_property_test.go:63-73`
**Issue:** Multiple properties ignore `NewBuilder` and `AddDocument` errors, and `TestPropertyIntegrationSerializationPreservesQueries` returns `true` when `Encode` fails. That means the property suite can stay green precisely when index construction or serialization regresses, which weakens the main safety net for the adaptive/numeric work added in this phase.
**Fix:**
```go
builder, err := NewBuilder(DefaultConfig(), numRGs)
if err != nil {
	return false
}
for i, doc := range docs {
	if err := builder.AddDocument(DocID(i), doc.JSON); err != nil {
		return false
	}
}
encoded, err := Encode(original)
if err != nil {
	return false
}
```
Apply the same pattern consistently across the property file so generator/setup failures fail the property instead of being ignored.

---

_Reviewed: 2026-04-15T20:14:35Z_
_Reviewer: Codex (gsd-code-reviewer)_
_Depth: standard_
