---
status: complete
phase: 06-query-path-hot-path
source: [06-query-path-hot-path-01-SUMMARY.md, 06-query-path-hot-path-02-SUMMARY.md]
started: 2026-04-14T17:36:40Z
updated: 2026-04-14T17:36:40Z
---

## Current Test

[testing complete]

## Tests

### 1. Canonical Supported JSONPath Spellings
expected: Run `go test ./... -run 'Test(JSONPathCanonicalizeSupportedPath|JSONPathCanonicalizeUnsupportedPath|BuilderCanonicalizesSupportedPathVariants|WithFTSPathsCanonicalizesEquivalentSupportedPaths|QueryEQCanonicalPathDecodeParity)' -count=1`. The targeted regression set should pass, proving `$.foo`, `$['foo']`, and `$["foo"]` resolve through the same canonical lookup path in fresh and decoded indexes without widening unsupported syntax.
result: pass

### 2. Safe Fallback and Duplicate Canonical Collision Guard
expected: Run `go test ./... -run 'Test(RebuildPathLookupRejectsDuplicateCanonicalPaths|FindPathCanonicalLookupAndFallback|EvaluateUnsupportedPathsFallback)' -count=1`. The tests should pass, proving duplicate canonical path state is rejected with `ErrInvalidFormat` while missing or unsupported public paths still return safe no-pruning results instead of false negatives.
result: pass

### 3. Canonical Transformer and Serialized Config Behavior
expected: Run `go test ./... -run 'Test(DateTransformerCanonicalConfigPath|DateTransformerDecodeCanonicalQueries|ConfigSerializationCanonicalPaths|ConfigSerializationCanonicalQueryBehavior)' -count=1`. The tests should pass, proving transformer and FTS config written with supported bracket spellings decode back to canonical keys and equivalent queries return the same row groups.
result: pass

### 4. Fixed-Parameter Wide-Path Benchmark Families
expected: Run `go test ./... -run '^$' -bench 'Benchmark(PathLookup|Query(EQ|Contains|Regex))' -benchtime=1x -count=1`. The output should include `BenchmarkQueryEQ`, `BenchmarkQueryContains`, `BenchmarkQueryRegex`, and `BenchmarkPathLookup` sub-benchmarks labeled with `paths=16|128|512|2048`, `spelling=canonical|single-quoted|double-quoted`, and operator selectivity markers (`64of4096`, `256of4096`, `512of4096`) for reproducible attribution.
result: pass

### 5. Full Suite Regression Check
expected: Run `go test ./... -count=1`. All repository tests should pass with no regressions from the Phase 06 lookup and benchmark changes.
result: pass

## Summary

total: 5
passed: 5
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
