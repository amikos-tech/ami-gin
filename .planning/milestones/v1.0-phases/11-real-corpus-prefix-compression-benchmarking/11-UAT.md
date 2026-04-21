---
status: complete
phase: 11-real-corpus-prefix-compression-benchmarking
source: [11-01-SUMMARY.md, 11-02-SUMMARY.md, 11-03-SUMMARY.md]
started: 2026-04-20T12:50:40Z
updated: 2026-04-20T13:07:33Z
---

## Current Test

[testing complete]

## Tests

### 1. Default Smoke Benchmark Path
expected: Run `go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=smoke' -benchtime=1x -count=1 -benchmem`. The benchmark should execute both `projection=structured` and `projection=text-heavy` for `tier=smoke`, report `docs_indexed=640` and `shards_loaded=1.000`, and finish without requiring any `GIN_PHASE11_*` env vars or an external corpus snapshot.
result: pass

### 2. Opt-In External Tier Documentation
expected: Inspect `README.md` and `testdata/phase11/README.md`. They should pin dataset revision `93d90fbdbc8f06c1fab72e74d5270dc897e1a090`, require the `gharchive/v0/documents/*.jsonl.gz` layout, name the exact env vars `GIN_PHASE11_GITHUB_ARCHIVE_ROOT`, `GIN_PHASE11_ENABLE_SUBSET`, and `GIN_PHASE11_ENABLE_LARGE`, and make subset/large explicitly opt-in while smoke stays the default path.
result: pass

### 3. Subset External Benchmark Tier
expected: Set `GIN_PHASE11_GITHUB_ARCHIVE_ROOT` to the pinned snapshot root and run `GIN_PHASE11_ENABLE_SUBSET=1 go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=subset' -benchtime=1x -count=1 -benchmem`. The benchmark should run both projections, report `docs_indexed=60466` and `shards_loaded=4.000`, and complete on the current tree without code changes.
result: pass

### 4. Large External Benchmark Guardrail
expected: Set `GIN_PHASE11_GITHUB_ARCHIVE_ROOT` to the pinned snapshot root and run `GIN_PHASE11_ENABLE_LARGE=1 go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=large' -benchtime=1x -count=1 -benchmem`, or inspect `11-BENCHMARK-RESULTS.md`. The large tier should run with `docs_indexed=486239` and `shards_loaded=32.00`; `projection=text-heavy` should keep encode and query metrics while `Decode` is intentionally reported as `skipped` because the standard 64 MiB decode cap remains enforced.
result: pass

### 5. Results Artifact and Recommendation Report
expected: Inspect `.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-BENCHMARK-RESULTS.md` and `.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-REAL-CORPUS-REPORT.md`. The results artifact should contain smoke/subset/large sections with both projections plus pinned provenance, and the report should include `Dataset`, `Tier Matrix`, `Helps`, `Flat / No-Win`, and `Recommendation` sections concluding that further serialization-format work is not justified for prefix compaction on this corpus.
result: pass

### 6. Full Suite Regression Check
expected: Run `go test ./... -count=1`. The repository test suite should pass on the current tree after the Phase 11 benchmark and documentation changes.
result: pass

## Summary

total: 6
passed: 6
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
