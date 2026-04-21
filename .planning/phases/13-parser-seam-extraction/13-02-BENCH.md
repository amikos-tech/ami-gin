# Phase 13 Plan 02 — Benchmark Delta (post-wire-up)

Captured: 2026-04-21
Baseline source: `.planning/phases/13-parser-seam-extraction/13-02-BASELINE.txt` (captured from `v1.0`)

## Method

The default multi-threaded benchmark harness was noisy on this machine and in this runtime, so the gate was evaluated with a focused, repeatable setup:

- `GOMAXPROCS=1`
- `-count=3`
- `BenchmarkAddDocument`
- `BenchmarkAddDocumentPhase07` explicit-number subbenchmarks that exercise the seam branch

This keeps the comparison on the same machine, with the same benchmark code, and removes scheduler/GC variance that dominated the default parallel runs.

## Before (v1.0 baseline)

```text
BenchmarkAddDocument               28251 / 28473 / 28087 ns/op   82308 B/op   734 allocs/op
Phase07 int-only docs=1000          9814 /  9829 /  9754 ns/op   37648 B/op   207 allocs/op
Phase07 int-only docs=10000         9965 / 10150 / 10089 ns/op   37648 B/op   207 allocs/op
Phase07 transformer-heavy docs=1000 33923 / 33981 / 35368 ns/op  88108 B/op   852 allocs/op
Phase07 transformer-heavy docs=10000 34020 / 34529 / 33788 ns/op 88107 B/op   852 allocs/op
```

## After (post-wire-up)

```text
BenchmarkAddDocument               27993 / 27850 / 28258 ns/op   82372 B/op   734 allocs/op
Phase07 int-only docs=1000          9798 /  9383 /  9337 ns/op   37712 B/op   207 allocs/op
Phase07 int-only docs=10000         9938 /  9939 / 10056 ns/op   37712 B/op   207 allocs/op
Phase07 transformer-heavy docs=1000 33809 / 34375 / 34055 ns/op  88174 B/op   852 allocs/op
Phase07 transformer-heavy docs=10000 33803 / 34144 / 34414 ns/op 88171 B/op   852 allocs/op
```

## Delta (median vs median)

- `BenchmarkAddDocument`: `28251 -> 27993 ns/op` (`-0.91%`), allocs/op `734 -> 734`
- `Phase07 int-only docs=1000`: `9814 -> 9383 ns/op` (`-4.39%`), allocs/op `207 -> 207`
- `Phase07 int-only docs=10000`: `10089 -> 9939 ns/op` (`-1.49%`), allocs/op `207 -> 207`
- `Phase07 transformer-heavy docs=1000`: `33981 -> 34055 ns/op` (`+0.22%`), allocs/op `852 -> 852`
- `Phase07 transformer-heavy docs=10000`: `34020 -> 34144 ns/op` (`+0.36%`), allocs/op `852 -> 852`

## Status

Within gate under the focused benchmark setup: no allocation regression and no representative seam-path benchmark exceeded the `+2%` wall-clock target.
