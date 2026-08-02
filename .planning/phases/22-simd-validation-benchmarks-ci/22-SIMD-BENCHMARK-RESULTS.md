# Phase 22 SIMD Benchmark Results

## Evidence Scope

This document is the authoritative controlled snapshot for the checked-in Phase 20 smoke tier. It contains one paired `COUNT=10` benchmark output and one analysis by the exact approved x/perf pseudo-version. It contains no external-corpus measurement, shared-runner result, native binary, cache contents, configuration diff content, or full environment dump.

- Capture window: 2026-08-01T10:02:07Z to 2026-08-01T10:04:14Z
- Repository revision: `2d41929d076659c6789a73c1e71668c246baab20`
- Source state before timing: `git status --short` produced no output; benchmark source, tests, fixtures, `go.mod`, `go.sum`, and `Makefile` were clean
- Preserved planning-only difference: none; no planning diff fingerprint was applicable
- Raw output SHA-256: `f58cae953f78c70d10c319cf50f59bd70ff7f23b79444b99dcfcc76b4bdd2f85`
- Raw output size: 13,853 bytes
- Sample cardinality: 80 smoke lines, exactly 10 per parser arm for each of four fixtures; 0 external-tier lines

## Parity Gate

Parity passed before measurement with the external tier explicitly unavailable:

```bash
env -u GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL -u GIN_PHASE20_SIMDJSON_DIR AMI_GIN_SIMD_REQUIRED=1 go test -v -tags simdjson -run '^(TestSIMDParserGoldenAuthoredFixtures|TestSIMDParserPhase20Parity|TestSIMDParserEvaluateParity|TestSIMDParserMalformedTrailingNumericKnownPolicyAsymmetry|FuzzParserParity)$' -count=1 .
```

The command passed all 12 authored golden fixtures, all four Phase 20 byte/query differentials, the 24-case Evaluate matrix, the known malformed trailing-number failure-layer attribution test, and all five committed `FuzzParserParity` seeds. No encoded-index or query-result mismatch occurred for documents that ingested without a parser-layer error, so the Phase 19 HARD stop did not trigger.

## Machine and Toolchain

| Field | Controlled value |
|---|---|
| OS | macOS 26.4.1 (build 25E253), Darwin 25.4.0 |
| Architecture | `darwin/arm64` |
| CPU | Apple M3 Max, 16 physical / 16 logical CPUs |
| Memory | 128 GiB |
| Go | `go1.26.5 darwin/arm64` |
| Module baseline | `go 1.25.5` |
| Native module | `github.com/amikos-tech/pure-simdjson v0.1.7` |
| Native module sum | `h1:JKnxejXIkLqRsT0m6NotcIJ5px0BZX8BLx8jl4qbToM=` |
| Benchmark analysis | `golang.org/x/perf/cmd/benchstat@v0.0.0-20260709024250-82a0b07e230d` |

The benchmark set `AMI_GIN_SIMD_REQUIRED=1`. `PURE_SIMDJSON_LIB_PATH`, `PURE_SIMDJSON_BINARY_MIRROR`, `PURE_SIMDJSON_DISABLE_GH_FALLBACK`, `PURE_SIMDJSON_CACHE_DIR`, and `PURE_SIMDJSON_WARN_LEAKS` were unset, so no explicit loading override was supplied. The upstream default resolver loaded the effective v0.1.7 native module. The analysis remained ephemeral: x/perf was not added to `go.mod`, and `go.mod`/`go.sum` stayed unchanged.

## Power, Thermal, and Background-Load Controls

- Power before and after the run: AC power, internal battery 100% and charged.
- Thermal state before and after: `pmset -g therm` reported no thermal warning and no performance warning.
- A transient CPU-heavy local metrics process was detected during preflight, so measurement was held until it exited. No benchmark command was started during the failed control snapshots.
- Accepted pre-run snapshot at 2026-08-01T10:01:59Z: 68.69% CPU idle; `ps` found no other Go test, Go tool, benchmark, linter, vulnerability scan, or process at or above 80% of one core.
- Post-run snapshot at 2026-08-01T10:04:53Z: 76.29% CPU idle; no other Go test, Go tool, benchmark, linter, or vulnerability scan was active.

## Checked-In Fixtures

All fixtures are `synthesized-from-shape`, are committed under `testdata/phase20`, and require no network access.

| Fixture | File | Documents | Input bytes | SHA-256 |
|---|---|---:|---:|---|
| nested-high-cardinality | `nested_high_cardinality.jsonl` | 96 | 46,800 | `d9cde09c630b3fab55bab42306989a8eeecdf45be665a7f41996bf19874efa28` |
| mixed-type-arrays | `mixed_type_arrays.jsonl` | 96 | 23,558 | `a970be3e593333770f1500f1d59b0c6117f4c7c193c404ab5173d62d0355c86e` |
| number-heavy | `number_heavy.jsonl` | 96 | 33,609 | `288163f69c56c557b1f3559f095b4bfc4efe07bb6518681c6cc7b373a0b51ec2` |
| combined | `combined.jsonl` | 288 | 103,967 | `cdf6990ae28694b0a961a4243792ac15790c37709c9fa1759ece270fd36810bf` |

## Metric Semantics

- `ns/op` is elapsed CPU-facing benchmark time per complete index build/finalize operation.
- `MB/s` is input JSON throughput derived from the sum of document bytes processed per iteration. It is not encoded-index throughput.
- `B/op` counts Go-heap allocation bytes only. pure-simdjson's native parser buffers are not visible to Go's allocator metrics, so `B/op` is not total process memory.
- `allocs/op` counts Go-heap allocations per operation.
- These are steady-state measurements. Spike 003 observed `b.N` of 61–312 and found first-iteration cost below run-to-run noise; this capture used `b.N` of 61–541, likewise amortizing any one-time warm-up. No cold-start claim is made.
- No single sample or `COUNT=1` result is authoritative. Spike 003 observed a 3× single-run error; only this one controlled `COUNT=10` paired snapshot is used for the recommendation.

## Controlled Command

Both external-tier variables were explicitly removed from the process environment:

```bash
env -u GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL -u GIN_PHASE20_SIMDJSON_DIR AMI_GIN_SIMD_REQUIRED=1 make bench-simd COUNT=10 BENCHTIME=1s
```

The Make target expanded to:

```text
go test -tags simdjson -run '^$' -bench '^BenchmarkSIMDTypedSinkIngest$' -benchmem -benchtime=1s -timeout=30m -count=10 .
```

## Raw Paired Output

```text
go test -tags simdjson -run '^$' -bench '^BenchmarkSIMDTypedSinkIngest$' -benchmem -benchtime=1s -timeout=30m -count=10 .
goos: darwin
goarch: arm64
pkg: github.com/amikos-tech/ami-gin
cpu: Apple M3 Max
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=stdlib-16         	     228	   5574060 ns/op	   8.38 MB/s	 4365027 B/op	   69326 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=stdlib-16         	     220	   5403490 ns/op	   8.64 MB/s	 4364999 B/op	   69326 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=stdlib-16         	     220	   5280994 ns/op	   8.84 MB/s	 4364992 B/op	   69326 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=stdlib-16         	     234	   4925197 ns/op	   9.48 MB/s	 4364982 B/op	   69326 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=stdlib-16         	     244	   5415305 ns/op	   8.62 MB/s	 4365006 B/op	   69326 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=stdlib-16         	     236	   5054434 ns/op	   9.24 MB/s	 4364988 B/op	   69326 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=stdlib-16         	     224	   5420547 ns/op	   8.62 MB/s	 4364983 B/op	   69326 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=stdlib-16         	     218	   5470649 ns/op	   8.54 MB/s	 4364990 B/op	   69326 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=stdlib-16         	     224	   5626847 ns/op	   8.30 MB/s	 4364986 B/op	   69326 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=stdlib-16         	     244	   5639808 ns/op	   8.28 MB/s	 4365000 B/op	   69326 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=simd-16           	     150	   7888108 ns/op	   5.92 MB/s	 5172216 B/op	   99863 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=simd-16           	     147	   8544928 ns/op	   5.47 MB/s	 5172140 B/op	   99863 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=simd-16           	     142	   8138014 ns/op	   5.74 MB/s	 5172170 B/op	   99863 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=simd-16           	     151	   7730829 ns/op	   6.04 MB/s	 5172338 B/op	   99863 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=simd-16           	     156	   7732983 ns/op	   6.04 MB/s	 5172320 B/op	   99863 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=simd-16           	     153	   7824922 ns/op	   5.97 MB/s	 5172371 B/op	   99864 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=simd-16           	     154	   7836236 ns/op	   5.96 MB/s	 5172207 B/op	   99863 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=simd-16           	     150	   7744778 ns/op	   6.03 MB/s	 5172376 B/op	   99864 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=simd-16           	     153	   7820283 ns/op	   5.97 MB/s	 5172322 B/op	   99864 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality/parser=simd-16           	     154	   7949792 ns/op	   5.87 MB/s	 5172371 B/op	   99863 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=stdlib-16               	     376	   3202323 ns/op	   7.33 MB/s	 2554700 B/op	   43393 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=stdlib-16               	     373	   3188666 ns/op	   7.36 MB/s	 2554688 B/op	   43393 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=stdlib-16               	     387	   3193708 ns/op	   7.35 MB/s	 2554782 B/op	   43393 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=stdlib-16               	     374	   3177085 ns/op	   7.38 MB/s	 2554786 B/op	   43393 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=stdlib-16               	     385	   3112712 ns/op	   7.54 MB/s	 2554699 B/op	   43393 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=stdlib-16               	     381	   3136683 ns/op	   7.48 MB/s	 2554786 B/op	   43393 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=stdlib-16               	     380	   3128910 ns/op	   7.50 MB/s	 2554680 B/op	   43393 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=stdlib-16               	     382	   3120240 ns/op	   7.52 MB/s	 2554761 B/op	   43393 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=stdlib-16               	     385	   3143995 ns/op	   7.46 MB/s	 2554719 B/op	   43393 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=stdlib-16               	     384	   3100843 ns/op	   7.57 MB/s	 2554691 B/op	   43393 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=simd-16                 	     207	   5842679 ns/op	   4.02 MB/s	 3486667 B/op	   77490 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=simd-16                 	     208	   5836869 ns/op	   4.02 MB/s	 3486692 B/op	   77490 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=simd-16                 	     205	   5706443 ns/op	   4.11 MB/s	 3486736 B/op	   77490 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=simd-16                 	     192	   6284807 ns/op	   3.73 MB/s	 3486736 B/op	   77490 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=simd-16                 	     198	   5839735 ns/op	   4.02 MB/s	 3486696 B/op	   77490 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=simd-16                 	     204	   5723577 ns/op	   4.10 MB/s	 3486612 B/op	   77490 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=simd-16                 	     210	   5589281 ns/op	   4.20 MB/s	 3486620 B/op	   77490 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=simd-16                 	     211	   5604244 ns/op	   4.19 MB/s	 3486625 B/op	   77490 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=simd-16                 	     206	   5812482 ns/op	   4.04 MB/s	 3486633 B/op	   77490 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays/parser=simd-16                 	     217	   5586233 ns/op	   4.20 MB/s	 3486586 B/op	   77490 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=stdlib-16                    	     530	   2288058 ns/op	  14.65 MB/s	 1871573 B/op	   38456 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=stdlib-16                    	     520	   2293799 ns/op	  14.61 MB/s	 1871585 B/op	   38456 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=stdlib-16                    	     529	   2321234 ns/op	  14.44 MB/s	 1871585 B/op	   38456 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=stdlib-16                    	     536	   2243042 ns/op	  14.94 MB/s	 1871568 B/op	   38456 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=stdlib-16                    	     534	   2271216 ns/op	  14.76 MB/s	 1871580 B/op	   38456 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=stdlib-16                    	     541	   2277597 ns/op	  14.71 MB/s	 1871581 B/op	   38456 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=stdlib-16                    	     531	   2311017 ns/op	  14.50 MB/s	 1871590 B/op	   38456 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=stdlib-16                    	     381	   3562016 ns/op	   9.41 MB/s	 1871614 B/op	   38456 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=stdlib-16                    	     499	   2420296 ns/op	  13.85 MB/s	 1871622 B/op	   38456 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=stdlib-16                    	     511	   2315815 ns/op	  14.47 MB/s	 1871603 B/op	   38456 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=simd-16                      	     295	   4067999 ns/op	   8.24 MB/s	 2480753 B/op	   56892 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=simd-16                      	     297	   3899085 ns/op	   8.60 MB/s	 2480710 B/op	   56892 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=simd-16                      	     297	   4556579 ns/op	   7.35 MB/s	 2480811 B/op	   56892 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=simd-16                      	     265	   4107364 ns/op	   8.16 MB/s	 2480777 B/op	   56892 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=simd-16                      	     279	   4132715 ns/op	   8.11 MB/s	 2480693 B/op	   56892 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=simd-16                      	     298	   4170038 ns/op	   8.04 MB/s	 2480685 B/op	   56892 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=simd-16                      	     294	   4094349 ns/op	   8.19 MB/s	 2480781 B/op	   56892 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=simd-16                      	     292	   4120178 ns/op	   8.13 MB/s	 2480746 B/op	   56892 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=simd-16                      	     292	   4158202 ns/op	   8.06 MB/s	 2480722 B/op	   56892 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=number-heavy/parser=simd-16                      	     286	   4126730 ns/op	   8.12 MB/s	 2480713 B/op	   56892 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=stdlib-16                        	      90	  12322268 ns/op	   8.41 MB/s	 8648489 B/op	  148052 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=stdlib-16                        	      94	  12264348 ns/op	   8.45 MB/s	 8648613 B/op	  148053 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=stdlib-16                        	     100	  11834080 ns/op	   8.76 MB/s	 8648506 B/op	  148053 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=stdlib-16                        	      94	  12082224 ns/op	   8.58 MB/s	 8648442 B/op	  148052 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=stdlib-16                        	     100	  12199460 ns/op	   8.50 MB/s	 8648653 B/op	  148053 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=stdlib-16                        	     100	  12357540 ns/op	   8.39 MB/s	 8648458 B/op	  148052 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=stdlib-16                        	     100	  12038373 ns/op	   8.61 MB/s	 8648584 B/op	  148053 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=stdlib-16                        	     100	  11824290 ns/op	   8.77 MB/s	 8648230 B/op	  148051 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=stdlib-16                        	      98	  12362273 ns/op	   8.39 MB/s	 8648376 B/op	  148052 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=stdlib-16                        	      99	  12417079 ns/op	   8.35 MB/s	 8648521 B/op	  148052 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=simd-16                          	      60	  18698795 ns/op	   5.54 MB/s	11001151 B/op	  231128 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=simd-16                          	      70	  18336533 ns/op	   5.65 MB/s	11000685 B/op	  231127 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=simd-16                          	      69	  18193441 ns/op	   5.70 MB/s	11000635 B/op	  231126 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=simd-16                          	      67	  19040060 ns/op	   5.45 MB/s	11001139 B/op	  231129 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=simd-16                          	      66	  19468155 ns/op	   5.33 MB/s	11001410 B/op	  231129 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=simd-16                          	      62	  19341460 ns/op	   5.36 MB/s	11001047 B/op	  231128 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=simd-16                          	      61	  18852015 ns/op	   5.50 MB/s	11001176 B/op	  231128 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=simd-16                          	      66	  18890814 ns/op	   5.49 MB/s	11001296 B/op	  231129 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=simd-16                          	      62	  18987293 ns/op	   5.46 MB/s	11001652 B/op	  231130 allocs/op
BenchmarkSIMDTypedSinkIngest/tier=smoke/fixture=combined/parser=simd-16                          	      67	  18678593 ns/op	   5.55 MB/s	11001040 B/op	  231128 allocs/op
PASS
ok  	github.com/amikos-tech/ami-gin	126.181s
```

## Pinned Benchstat Analysis

Exact approved command against the single output above:

```bash
go run golang.org/x/perf/cmd/benchstat@v0.0.0-20260709024250-82a0b07e230d -col /parser /tmp/ami-gin-22-06.dKGbG7/bench-simd-count10.txt
```

The temporary path is execution-local and not a reproducibility dependency; the complete input is preserved above. Full captured command output:

```text
go: downloading golang.org/x/perf v0.0.0-20260709024250-82a0b07e230d
goos: darwin
goarch: arm64
pkg: github.com/amikos-tech/ami-gin
cpu: Apple M3 Max
                                                                  │   stdlib    │                simd                 │
                                                                  │   sec/op    │   sec/op     vs base                │
SIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality-16   5.418m ± 7%   7.831m ± 4%  +44.53% (p=0.000 n=10)
SIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays-16         3.140m ± 2%   5.768m ± 3%  +83.68% (p=0.000 n=10)
SIMDTypedSinkIngest/tier=smoke/fixture=number-heavy-16              2.302m ± 5%   4.123m ± 1%  +79.09% (p=0.000 n=10)
SIMDTypedSinkIngest/tier=smoke/fixture=combined-16                  12.23m ± 3%   18.87m ± 3%  +54.28% (p=0.000 n=10)
geomean                                                             4.679m        7.700m       +64.57%

                                                                  │    stdlib     │                 simd                 │
                                                                  │      B/s      │     B/s       vs base                │
SIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality-16    8.221Mi ± 7%   5.689Mi ± 4%  -30.80% (p=0.000 n=10)
SIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays-16          7.124Mi ± 2%   3.881Mi ± 3%  -45.52% (p=0.000 n=10)
SIMDTypedSinkIngest/tier=smoke/fixture=number-heavy-16              13.881Mi ± 5%   7.749Mi ± 1%  -44.18% (p=0.000 n=10)
SIMDTypedSinkIngest/tier=smoke/fixture=combined-16                   8.082Mi ± 3%   5.240Mi ± 3%  -35.16% (p=0.000 n=10)
geomean                                                              9.003Mi        5.472Mi       -39.22%

                                                                  │    stdlib    │                 simd                  │
                                                                  │     B/op     │     B/op       vs base                │
SIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality-16   4.163Mi ± 0%    4.933Mi ± 0%  +18.50% (p=0.000 n=10)
SIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays-16         2.436Mi ± 0%    3.325Mi ± 0%  +36.48% (p=0.000 n=10)
SIMDTypedSinkIngest/tier=smoke/fixture=number-heavy-16              1.785Mi ± 0%    2.366Mi ± 0%  +32.55% (p=0.000 n=10)
SIMDTypedSinkIngest/tier=smoke/fixture=combined-16                  8.248Mi ± 0%   10.492Mi ± 0%  +27.20% (p=0.000 n=10)
geomean                                                             3.496Mi         4.492Mi       +28.50%

                                                                  │   stdlib    │                simd                 │
                                                                  │  allocs/op  │  allocs/op   vs base                │
SIMDTypedSinkIngest/tier=smoke/fixture=nested-high-cardinality-16   69.33k ± 0%   99.86k ± 0%  +44.05% (p=0.000 n=10)
SIMDTypedSinkIngest/tier=smoke/fixture=mixed-type-arrays-16         43.39k ± 0%   77.49k ± 0%  +78.58% (p=0.000 n=10)
SIMDTypedSinkIngest/tier=smoke/fixture=number-heavy-16              38.46k ± 0%   56.89k ± 0%  +47.94% (p=0.000 n=10)
SIMDTypedSinkIngest/tier=smoke/fixture=combined-16                  148.1k ± 0%   231.1k ± 0%  +56.11% (p=0.000 n=10)
geomean                                                             64.33k        100.4k       +56.12%
```

No regression threshold or performance gate is defined by this evidence.
