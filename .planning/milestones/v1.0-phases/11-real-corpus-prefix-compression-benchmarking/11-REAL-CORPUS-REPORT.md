# Phase 11 Real-Corpus Report

Source artifact: [`11-BENCHMARK-RESULTS.md`](./11-BENCHMARK-RESULTS.md)

## Dataset

| Field | Value |
|---|---|
| Dataset | `common-pile/github_archive` |
| Dataset revision | `93d90fbdbc8f06c1fab72e74d5270dc897e1a090` |
| Fixture origin | `synthesized-from-shape` for smoke; pinned local snapshot for subset/large |
| Snapshot layout | `gharchive/v0/documents/*.jsonl.gz` |
| Projections | `projection=structured`, `projection=text-heavy` |

The smoke tier uses the checked-in synthesized fixture from [`testdata/phase11/README.md`](../../../testdata/phase11/README.md). Subset and large reuse the pinned snapshot root recorded in [`11-BENCHMARK-RESULTS.md`](./11-BENCHMARK-RESULTS.md).

## Tier Matrix

| Tier | Projection | docs_indexed | shards_loaded | legacy_raw_bytes | compact_raw_bytes | default_zstd_bytes | bytes_saved_pct | Encode B/op | Encode allocs/op |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Smoke | `projection=structured` | 640 | 1 | 109973 | 102185 | 9851 | 7.082 | 588485392 | 14956 |
| Smoke | `projection=text-heavy` | 640 | 1 | 373928 | 323220 | 27063 | 13.56 | 590973320 | 40449 |
| Subset | `projection=structured` | 60466 | 4 | 2597618 | 2597468 | 711466 | 0.005775 | 607195776 | 199802 |
| Subset | `projection=text-heavy` | 60466 | 4 | 32932864 | 32932813 | 7950132 | 0.0001549 | 810211968 | 3565898 |
| Large | `projection=structured` | 486239 | 32 | 15741241 | 15740813 | 5369211 | 0.002719 | 676288200 | 354918 |
| Large | `projection=text-heavy` | 486239 | 32 | 194558569 | 194555530 | 59741681 | 0.001562 | 1933895464 | 12037885 |

`docs_indexed` grows strictly across tiers (`640 < 60466 < 486239`), so the subset/large comparisons come from materially larger corpus slices rather than repeated smoke measurements.

## Helps

| Tier | Projection | Raw bytes saved | Why it qualifies |
|---|---|---:|---|
| Smoke | `projection=structured` | 7788 | Repeated nested metadata (`repo`, `license`, `license_type`) still benefits from prefix-compacted ordered strings on the bounded fixture. |
| Smoke | `projection=text-heavy` | 50708 | The synthetic smoke fixture keeps repeated URL/source scaffolding alongside unique text, so compaction still removes a visible amount of raw string payload. |

Command-backed evidence:
- Smoke structured: `legacy_raw_bytes=109973`, `compact_raw_bytes=102185`, `bytes_saved_pct=7.082`
- Smoke text-heavy: `legacy_raw_bytes=373928`, `compact_raw_bytes=323220`, `bytes_saved_pct=13.56`

Inference from `default_zstd_bytes`: even in the tiers with visible raw wins, zstd already dominates final artifact size (`9851` and `27063` bytes respectively), so the raw prefix-compaction win is real but modest once the final compressed artifact is considered.

## Flat / No-Win

| Tier | Projection | Raw bytes saved | bytes_saved_pct | Interpretation |
|---|---|---:|---:|---|
| Subset | `projection=structured` | 150 | 0.005775 | Effectively flat on real external data. |
| Subset | `projection=text-heavy` | 51 | 0.0001549 | Functionally no win. |
| Large | `projection=structured` | 428 | 0.002719 | Functionally flat at larger scale. |
| Large | `projection=text-heavy` | 3039 | 0.001562 | Still flat relative to a 194 MB raw artifact. |

Command-backed evidence from [`11-BENCHMARK-RESULTS.md`](./11-BENCHMARK-RESULTS.md):
- Subset structured and text-heavy both stay below `0.006%` raw savings.
- Large structured and text-heavy both stay below `0.003%` raw savings.
- Large text-heavy `Decode` is absent by design because the standard `Decode()` path rejects the `194555530`-byte decompressed payload against the 64 MiB safety cap; the benchmark intentionally skips that leaf instead of weakening the production limit.

On these external tiers, `default_zstd_bytes` largely masks the already-tiny raw-string savings. That conclusion is partly inferential because Phase 11 does not encode a legacy-zstd baseline, but the observed raw savings are so close to zero that additional format work would not plausibly move the final artifact enough to matter.

## Recommendation

Do not pursue further serialization-format work now for prefix compaction alone; revisit later only if a different real corpus or a more repetition-heavy production workload shows materially larger raw savings than the near-zero subset/large deltas recorded in [`11-BENCHMARK-RESULTS.md`](./11-BENCHMARK-RESULTS.md).
