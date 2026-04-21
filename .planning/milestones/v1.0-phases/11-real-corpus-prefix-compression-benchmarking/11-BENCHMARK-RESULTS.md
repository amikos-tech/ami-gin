# Phase 11 Benchmark Results

## Benchmark Configuration

Dataset revision: 93d90fbdbc8f06c1fab72e74d5270dc897e1a090
Root path: `/Users/tazarov/.cache/huggingface/hub/datasets--common-pile--github_archive/snapshots/93d90fbdbc8f06c1fab72e74d5270dc897e1a090`
Snapshot layout: `gharchive/v0/documents/*.jsonl.gz`
GIN config: default config, no extra transformers, no derived aliases
Row-group packing: 128 docs per row group
Tier growth check: `640 < 60466 < 486239` docs indexed across Smoke, Subset, and Large

## Smoke

Command: `go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=smoke' -benchtime=1x -count=1 -benchmem`

Env vars: `(none)`

Effective shard count: 1.000
Effective document count: 640.0

### projection=structured

| Leaf | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| Encode | 43937542 | 588485392 | 14956 |
| Decode | 1785375 | 67927888 | 30735 |
| QueryAfterDecode | 12333 | 1136 | 46 |

| Metric | Value |
|---|---:|
| legacy_raw_bytes | 109973 |
| compact_raw_bytes | 102185 |
| default_zstd_bytes | 9851 |
| legacy_string_payload_bytes | 19541 |
| compact_string_payload_bytes | 11753 |
| bytes_saved_pct | 7.082 |
| docs_indexed | 640.0 |
| shards_loaded | 1.000 |

### projection=text-heavy

| Leaf | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| Encode | 15097875 | 590973320 | 40449 |
| Decode | 4585917 | 69624712 | 87168 |
| QueryAfterDecode | 3958 | 832 | 37 |

| Metric | Value |
|---|---:|
| legacy_raw_bytes | 373928 |
| compact_raw_bytes | 323220 |
| default_zstd_bytes | 27063 |
| legacy_string_payload_bytes | 183133 |
| compact_string_payload_bytes | 132425 |
| bytes_saved_pct | 13.56 |
| docs_indexed | 640.0 |
| shards_loaded | 1.000 |

## Subset

Command: `GIN_PHASE11_ENABLE_SUBSET=1 go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=subset' -benchtime=1x -count=1 -benchmem`

Env vars: `GIN_PHASE11_GITHUB_ARCHIVE_ROOT=/Users/tazarov/.cache/huggingface/hub/datasets--common-pile--github_archive/snapshots/93d90fbdbc8f06c1fab72e74d5270dc897e1a090`, `GIN_PHASE11_ENABLE_SUBSET=1`

Effective shard count: 4.000
Effective document count: 60466

### projection=structured

| Leaf | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| Encode | 139874833 | 607195776 | 199802 |
| Decode | 18469250 | 79257312 | 432346 |
| QueryAfterDecode | 6208 | 2080 | 46 |

| Metric | Value |
|---|---:|
| legacy_raw_bytes | 2597618 |
| compact_raw_bytes | 2597468 |
| default_zstd_bytes | 711466 |
| legacy_string_payload_bytes | 1974 |
| compact_string_payload_bytes | 1824 |
| bytes_saved_pct | 0.005775 |
| docs_indexed | 60466 |
| shards_loaded | 4.000 |

### projection=text-heavy

| Leaf | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| Encode | 1688391166 | 810211968 | 3565898 |
| Decode | 381781000 | 279255056 | 8031199 |
| QueryAfterDecode | 8292 | 2864 | 37 |

| Metric | Value |
|---|---:|
| legacy_raw_bytes | 32932864 |
| compact_raw_bytes | 32932813 |
| default_zstd_bytes | 7950132 |
| legacy_string_payload_bytes | 2675 |
| compact_string_payload_bytes | 2624 |
| bytes_saved_pct | 0.0001549 |
| docs_indexed | 60466 |
| shards_loaded | 4.000 |

## Large

Command: `GIN_PHASE11_ENABLE_LARGE=1 go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=large' -benchtime=1x -count=1 -benchmem`

Env vars: `GIN_PHASE11_GITHUB_ARCHIVE_ROOT=/Users/tazarov/.cache/huggingface/hub/datasets--common-pile--github_archive/snapshots/93d90fbdbc8f06c1fab72e74d5270dc897e1a090`, `GIN_PHASE11_ENABLE_LARGE=1`

Effective shard count: 32.00
Effective document count: 486239

### projection=structured

| Leaf | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| Encode | 803489334 | 676288200 | 354918 |
| Decode | 59799375 | 111427672 | 697146 |
| QueryAfterDecode | 16834 | 9312 | 46 |

| Metric | Value |
|---|---:|
| legacy_raw_bytes | 15741241 |
| compact_raw_bytes | 15740813 |
| default_zstd_bytes | 5369211 |
| legacy_string_payload_bytes | 3483 |
| compact_string_payload_bytes | 3055 |
| bytes_saved_pct | 0.002719 |
| docs_indexed | 486239 |
| shards_loaded | 32.00 |

### projection=text-heavy

| Leaf | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| Encode | 13448826541 | 1933895464 | 12037885 |
| Decode | skipped | skipped | skipped |
| QueryAfterDecode | 64875 | 17200 | 37 |

| Metric | Value |
|---|---:|
| legacy_raw_bytes | 194558569 |
| compact_raw_bytes | 194555530 |
| default_zstd_bytes | 59741681 |
| legacy_string_payload_bytes | 19068 |
| compact_string_payload_bytes | 16029 |
| bytes_saved_pct | 0.001562 |
| docs_indexed | 486239 |
| shards_loaded | 32.00 |

Note: `Decode` is intentionally skipped for this branch. The default `Decode()` path rejects the 194555530-byte decompressed payload against the 64 MiB safety cap, so the benchmark records size, encode, and query-after-decode evidence without weakening the production guardrail.
