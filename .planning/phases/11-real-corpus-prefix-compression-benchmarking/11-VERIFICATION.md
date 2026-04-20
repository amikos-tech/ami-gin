---
phase: 11-real-corpus-prefix-compression-benchmarking
verified: 2026-04-20T14:16:26Z
status: passed
score: 6/6 must-haves verified
overrides_applied: 0
---

# Phase 11: Real-Corpus Prefix Compression Benchmarking Verification Report

**Phase Goal:** Validate Phase 10's real-world payoff on representative external corpora without reopening serialization-format scope.
**Verified:** 2026-04-20T14:16:26Z
**Status:** passed
**Re-verification:** No — initial closeout verification before ship

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | Phase 11 ships a benchmark harness that covers smoke, subset, and large tiers, keeps subset/large opt-in, and validates the expected external shard layout. | ✓ VERIFIED | `benchmark_test.go:1672-1681` defines the smoke fixture path, env vars, shard layout, and tier sizes; `benchmark_test.go:2332-2357` keeps subset/large opt-in and requires `gharchive/v0/documents/*.jsonl.gz`; `benchmark_test.go:2359-2437` wires tier and projection coverage into `BenchmarkPhase11RealCorpus`. |
| 2 | Fresh benchmark runs on the current tree passed for smoke, subset, and large, with both projections reporting the expected document and shard counts. | ✓ VERIFIED | Fresh commands passed on HEAD: smoke reported `docs_indexed=640` and `shards_loaded=1.000`; subset reported `docs_indexed=60466` and `shards_loaded=4.000`; large reported `docs_indexed=486239` and `shards_loaded=32.00`. |
| 3 | The checked-in results artifact records both raw serialized deltas and final compressed artifact size across all tiers, and explicitly preserves the large text-heavy decode-cap behavior instead of weakening the guardrail. | ✓ VERIFIED | `11-BENCHMARK-RESULTS.md:3-153` records `legacy_raw_bytes`, `compact_raw_bytes`, `default_zstd_bytes`, `docs_indexed`, and `shards_loaded` for smoke/subset/large; `11-BENCHMARK-RESULTS.md:134-153` marks large text-heavy `Decode` as skipped because the default path rejects the `194555530`-byte decompressed payload against the 64 MiB cap. |
| 4 | The final write-up clearly separates where prefix compaction helps from where it is effectively flat and closes with a recommendation against more format work for this corpus. | ✓ VERIFIED | `11-REAL-CORPUS-REPORT.md:5-61` contains `Dataset`, `Tier Matrix`, `Helps`, `Flat / No-Win`, and `Recommendation`; `11-REAL-CORPUS-REPORT.md:47-61` concludes the subset/large deltas are too small to justify more serialization-format work now. |
| 5 | The user-facing workflow documentation pins the dataset revision, records the snapshot layout, and keeps smoke as the default path while documenting the opt-in subset/large workflow. | ✓ VERIFIED | `README.md:590-632` documents the smoke command, exact env vars, pinned `common-pile/github_archive` revision, and the 4-shard/32-shard opt-in tiers; `testdata/phase11/README.md:3-11` pins the same dataset revision and shard layout for the checked-in smoke fixture. |
| 6 | The repository still passes a fresh full regression run after the Phase 11 benchmark and documentation additions. | ✓ VERIFIED | Fresh `go test ./... -count=1` passed on HEAD: `ok github.com/amikos-tech/ami-gin 104.903s`; `ok github.com/amikos-tech/ami-gin/cmd/gin-index 1.229s`. |

**Score:** 6/6 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `benchmark_test.go` | Tiered real-corpus benchmark harness with smoke/subset/large coverage and decode-skip guardrail | ✓ VERIFIED | `benchmark_test.go:1672-1681,2332-2437` defines the tier matrix, opt-in env handling, metric reporting, and guarded large decode skip. |
| `testdata/phase11/README.md` | Pinned smoke fixture provenance and external layout contract | ✓ VERIFIED | `testdata/phase11/README.md:3-11` records dataset revision, synthesized origin, shard layout, and smoke row count. |
| `README.md` | Reproducible smoke-first workflow plus opt-in external tier instructions | ✓ VERIFIED | `README.md:590-632` documents the exact commands, env vars, shard expectations, and links to the results/report artifacts. |
| `11-BENCHMARK-RESULTS.md` | Checked-in raw benchmark evidence for smoke, subset, and large tiers | ✓ VERIFIED | `11-BENCHMARK-RESULTS.md:3-153` records all three tiers, both projections, and the decode-skip note. |
| `11-REAL-CORPUS-REPORT.md` | Interpretive report grounded in the raw results artifact | ✓ VERIFIED | `11-REAL-CORPUS-REPORT.md:5-61` summarizes the corpus, metrics, help/no-win cases, and recommendation. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Default smoke benchmark path | `go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=smoke' -benchtime=1x -count=1 -benchmem` | Passed on HEAD; both projections ran and reported `docs_indexed=640`, `shards_loaded=1.000`. | ✓ PASS |
| Opt-in subset external tier | `GIN_PHASE11_GITHUB_ARCHIVE_ROOT=/Users/tazarov/.cache/huggingface/hub/datasets--common-pile--github_archive/snapshots/93d90fbdbc8f06c1fab72e74d5270dc897e1a090 GIN_PHASE11_ENABLE_SUBSET=1 go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=subset' -benchtime=1x -count=1 -benchmem` | Passed on HEAD; both projections ran and reported `docs_indexed=60466`, `shards_loaded=4.000`. | ✓ PASS |
| Opt-in large external tier | `GIN_PHASE11_GITHUB_ARCHIVE_ROOT=/Users/tazarov/.cache/huggingface/hub/datasets--common-pile--github_archive/snapshots/93d90fbdbc8f06c1fab72e74d5270dc897e1a090 GIN_PHASE11_ENABLE_LARGE=1 go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=large' -benchtime=1x -count=1 -benchmem` | Passed on HEAD; both projections ran and reported `docs_indexed=486239`, `shards_loaded=32.00`. The text-heavy branch emitted `Size`, `Encode`, and `QueryAfterDecode` only, confirming the intended decode skip on the current tree. | ✓ PASS |
| Report structure and recommendation sections | `rg -n '^## (Dataset|Tier Matrix|Helps|Flat / No-Win|Recommendation)$' .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-REAL-CORPUS-REPORT.md` | Returned all five required headings. | ✓ PASS |
| README workflow contract | `rg -n 'GIN_PHASE11_GITHUB_ARCHIVE_ROOT|GIN_PHASE11_ENABLE_SUBSET|GIN_PHASE11_ENABLE_LARGE|BenchmarkPhase11RealCorpus/tier=smoke|common-pile/github_archive|gharchive/v0/documents/\*\.jsonl\.gz|11-BENCHMARK-RESULTS\.md|11-REAL-CORPUS-REPORT\.md|opt-in only' README.md` | Returned the smoke command, the three env vars, the pinned dataset, the shard layout, the opt-in wording, and links to both checked-in artifacts. | ✓ PASS |
| Full repository regression suite | `go test ./... -count=1` | `ok github.com/amikos-tech/ami-gin 104.903s`; `ok github.com/amikos-tech/ami-gin/cmd/gin-index 1.229s` | ✓ PASS |

### Success Criteria Coverage

| Roadmap Criterion | Status | Evidence |
| --- | --- | --- |
| External coverage includes at least one realistic log-style corpus with repeated paths and repeated string terms | ✓ SATISFIED | `benchmark_test.go:1672-1761,2359-2437` benchmarks the pinned `common-pile/github_archive` shape across structured and text-heavy projections; fresh smoke/subset/large runs passed. |
| Benchmark plan uses practical dataset scales instead of only tiny synthetic fixtures | ✓ SATISFIED | Smoke, subset, and large tiers are defined in `benchmark_test.go:1728-1731` and exercised by fresh commands with `640`, `60466`, and `486239` docs respectively. |
| Results report raw serialized deltas and final encoded artifact size | ✓ SATISFIED | `11-BENCHMARK-RESULTS.md:21-153` records `legacy_raw_bytes`, `compact_raw_bytes`, and `default_zstd_bytes` for every tier/projection pair. |
| Final write-up makes explicit where compaction helps, where it is flat, and whether further work is justified | ✓ SATISFIED | `11-REAL-CORPUS-REPORT.md:30-61` separates `Helps` from `Flat / No-Win` and ends with a clear recommendation against more format work now. |

### Human Verification Required

None.

### Gaps Summary

None. Phase 11 achieves the roadmap goal on the current branch: the real-corpus benchmark harness is reproducible and tiered, the checked-in artifacts capture the raw and compressed evidence, the large decode guardrail remains intact, the final write-up is explicit about help versus no-win cases, and the repository still passes a fresh full regression run.

---

_Verified: 2026-04-20T14:16:26Z_  
_Verifier: Codex (phase closeout verification)_
