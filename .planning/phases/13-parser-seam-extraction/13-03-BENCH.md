# Phase 13 Plan 03 — Merge-Gate Benchmark Delta

Captured: 2026-04-21
Source commit: 88c3371
Baseline source: `.planning/phases/13-parser-seam-extraction/13-02-BASELINE.txt` (v1.0 focused baseline, committed by Plan 02)
Benchmark regex: `^BenchmarkAddDocument$|^BenchmarkAddDocumentPhase07/parser=explicit-number/docs=(1000|10000)/shape=(int-only|transformer-heavy)$`

## PR Paste

| Benchmark | Baseline ns/op | Final ns/op | Delta | Baseline B/op | Final B/op | Baseline allocs/op | Final allocs/op | Gate |
|-----------|----------------|-------------|-------|---------------|------------|--------------------|-----------------|------|
| `BenchmarkAddDocument` | `28251` | `28632.5` | `+1.35%` | `82308` | `82372` | `734` | `734` | PASS |
| `Phase07 explicit-number docs=1000 shape=int-only` | `9814` | `9645.5` | `-1.72%` | `37648` | `37712` | `207` | `207` | PASS |
| `Phase07 explicit-number docs=10000 shape=int-only` | `10089` | `10269` | `+1.78%` | `37648` | `37712` | `207` | `207` | PASS |
| `Phase07 explicit-number docs=1000 shape=transformer-heavy` | `33981` | `34726.5` | `+2.19%` | `88108` | `88172` | `852` | `852` | FAIL |
| `Phase07 explicit-number docs=10000 shape=transformer-heavy` | `34020` | `34746.5` | `+2.14%` | `88107` | `88171` | `852` | `852` | FAIL |

## Notes

- The first combined exact-anchored `-count=10` pass across both benchmark families drifted materially higher than the committed phase-02 focused benchmark artifact.
- To reduce cross-benchmark heating and make the comparison closer to the committed baseline methodology, the final numbers above come from isolated `GOMAXPROCS=1` reruns:
  - `^BenchmarkAddDocument$`
  - `^BenchmarkAddDocumentPhase07/parser=explicit-number/docs=(1000|10000)/shape=(int-only|transformer-heavy)$`
- A secondary transformer-only rerun was even noisier for `docs=1000/shape=transformer-heavy`, which reinforces that this machine is close to the threshold for that workload. The raw output is preserved below.
- `allocs/op` stayed flat in every case. The `+64 B/op` drift was already present in the Plan 02 evidence and remains unchanged here.

## Verdict

Needs review. The parser parity harness and full repo test/lint gate are green, and the isolated `BenchmarkAddDocument` plus both explicit-number int-only probes cleared the wall-clock threshold. The two transformer-heavy explicit-number subbenchmarks did not clear the `<= +2%` ns/op gate consistently on this machine, so Phase 13 is not auto-finalized in `.planning/ROADMAP.md` or `.planning/STATE.md`.

## Paste: raw `BenchmarkAddDocument` output

```text
goos: darwin
goarch: arm64
pkg: github.com/amikos-tech/ami-gin
cpu: Apple M3 Max
BenchmarkAddDocument 	   37724	     28376 ns/op	   82372 B/op	     734 allocs/op
BenchmarkAddDocument 	   42112	     28100 ns/op	   82372 B/op	     734 allocs/op
BenchmarkAddDocument 	   40810	     28747 ns/op	   82372 B/op	     734 allocs/op
BenchmarkAddDocument 	   42200	     30761 ns/op	   82372 B/op	     734 allocs/op
BenchmarkAddDocument 	   41361	     29538 ns/op	   82372 B/op	     734 allocs/op
BenchmarkAddDocument 	   42480	     28930 ns/op	   82372 B/op	     734 allocs/op
BenchmarkAddDocument 	   41373	     28431 ns/op	   82372 B/op	     734 allocs/op
BenchmarkAddDocument 	   42524	     28680 ns/op	   82372 B/op	     734 allocs/op
BenchmarkAddDocument 	   42394	     28585 ns/op	   82372 B/op	     734 allocs/op
BenchmarkAddDocument 	   42036	     28367 ns/op	   82372 B/op	     734 allocs/op
PASS
ok  	github.com/amikos-tech/ami-gin	15.331s
```

## Paste: raw `BenchmarkAddDocumentPhase07` output

```text
goos: darwin
goarch: arm64
pkg: github.com/amikos-tech/ami-gin
cpu: Apple M3 Max
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=int-only         	  107392	     10102 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=int-only         	  121873	      9722 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=int-only         	  119602	      9910 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=int-only         	  125691	      9573 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=int-only         	  126496	      9679 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=int-only         	  124712	      9612 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=int-only         	  126810	      9584 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=int-only         	  127294	      9601 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=int-only         	  124740	      9485 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=int-only         	  126410	      9966 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=int-only        	  117960	     10268 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=int-only        	  116378	     10190 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=int-only        	  120172	     10547 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=int-only        	  119250	     10157 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=int-only        	  117823	     10270 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=int-only        	  118915	     10237 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=int-only        	  112062	     10316 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=int-only        	  117103	     10334 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=int-only        	  119542	     10138 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=int-only        	  119401	     10829 ns/op	   37712 B/op	     207 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   34438	     34809 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   35085	     34488 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   33242	     35444 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   34880	     34217 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   34988	     34749 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   33547	     35072 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   34802	     34550 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   33856	     34409 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   35048	     34904 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   34552	     34704 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   35374	     34899 ns/op	   88171 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   34020	     34396 ns/op	   88171 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   35035	     34183 ns/op	   88171 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   35209	     34278 ns/op	   88171 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   34946	     34429 ns/op	   88171 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   34413	     34594 ns/op	   88171 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   34868	     35304 ns/op	   88171 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   31551	     36743 ns/op	   88171 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   34650	     36139 ns/op	   88171 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   35122	     37076 ns/op	   88172 B/op	     852 allocs/op
PASS
ok  	github.com/amikos-tech/ami-gin	59.894s
```

## Secondary Noise Probe: transformer-only rerun

```text
goos: darwin
goarch: arm64
pkg: github.com/amikos-tech/ami-gin
cpu: Apple M3 Max
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   33822	     34466 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   33289	     36214 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   34119	     34923 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   33902	     36092 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   33207	     36464 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   33176	     36734 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   34897	     36617 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   34509	     36408 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   31784	     36437 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=1000/shape=transformer-heavy         	   33007	     35615 ns/op	   88172 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   34212	     35464 ns/op	   88171 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   34093	     35289 ns/op	   88171 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   34664	     34834 ns/op	   88171 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   34752	     34817 ns/op	   88171 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   34910	     34452 ns/op	   88171 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   32664	     34084 ns/op	   88171 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   33777	     34322 ns/op	   88171 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   34951	     34688 ns/op	   88171 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   35143	     34257 ns/op	   88171 B/op	     852 allocs/op
BenchmarkAddDocumentPhase07/parser=explicit-number/docs=10000/shape=transformer-heavy        	   35554	     34308 ns/op	   88171 B/op	     852 allocs/op
PASS
ok  	github.com/amikos-tech/ami-gin	33.971s
```
