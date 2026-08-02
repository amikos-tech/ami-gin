# Phase 22 SIMD Benchmark Report

Source artifact: [`22-SIMD-BENCHMARK-RESULTS.md`](./22-SIMD-BENCHMARK-RESULTS.md)

## Parity Gate

The correctness gate passed: stdlib and SIMD produced **identical encoded indexes and query results for documents that ingest without a parser-layer error**. The tested exclusion is malformed-input failure-layer attribution: the pinned trailing-number case is rejected by both paths but is attributed to the numeric layer by stdlib and the parser layer by SIMD.

That exclusion does not change committed index state. No in-scope encoded-byte or query-result mismatch occurred, so correctness does not force the Phase 19 HARD-stop deferral.

## Evidence Boundary

The observations below come only from the controlled, same-process `COUNT=10` smoke capture in the source artifact. The run used all four checked-in Phase 20 fixtures, explicitly cleared both external-tier variables, and analyzed one unstitched output with the approved pinned `benchstat -col /parser`.

The measurements are steady-state: the benchmark iteration counts amortize one-time warm-up. `B/op` and `allocs/op` cover the Go heap only; pure-simdjson's native buffers are not visible, so neither parser arm's `B/op` represents total process memory. Input throughput is based on JSON document bytes, not encoded-index bytes. The raw benchmark labels this `MB/s`; benchstat normalizes the displayed rates to MiB/s.

No single-run number is authoritative. The controlled ten-sample comparison is the decision source. Any future CI artifact is shared-runner-noisy trend data only and cannot replace this snapshot. No performance regression threshold or gate is defined.

## Observed Results

All deltas are SIMD relative to stdlib. Positive time, bytes, and allocation deltas are worse; negative throughput deltas are lower throughput.

| Fixture | Time: stdlib → SIMD | CPU delta | Input throughput: stdlib → SIMD | Throughput delta |
|---|---:|---:|---:|---:|
| nested-high-cardinality | 5.418 → 7.831 ms/op | +44.53% | 8.221 → 5.689 MiB/s | -30.80% |
| mixed-type-arrays | 3.140 → 5.768 ms/op | +83.68% | 7.124 → 3.881 MiB/s | -45.52% |
| number-heavy | 2.302 → 4.123 ms/op | +79.09% | 13.881 → 7.749 MiB/s | -44.18% |
| combined | 12.23 → 18.87 ms/op | +54.28% | 8.082 → 5.240 MiB/s | -35.16% |

| Fixture | Go-heap B/op: stdlib → SIMD | B/op delta | allocs/op: stdlib → SIMD | Allocation delta |
|---|---:|---:|---:|---:|
| nested-high-cardinality | 4.163 → 4.933 MiB | +18.50% | 69.33k → 99.86k | +44.05% |
| mixed-type-arrays | 2.436 → 3.325 MiB | +36.48% | 43.39k → 77.49k | +78.58% |
| number-heavy | 1.785 → 2.366 MiB | +32.55% | 38.46k → 56.89k | +47.94% |
| combined | 8.248 → 10.492 MiB | +27.20% | 148.1k → 231.1k | +56.11% |

Benchstat reports `n=10` and `p=0.000` for every comparison. Time and throughput variability was bounded but non-zero:

| Fixture | stdlib variation | SIMD variation |
|---|---:|---:|
| nested-high-cardinality | ±7% | ±4% |
| mixed-type-arrays | ±2% | ±3% |
| number-heavy | ±5% | ±1% |
| combined | ±3% | ±3% |

Go-heap bytes and allocation counts rounded to ±0% variation in the benchstat view.

## Interpretation

### nested-high-cardinality

Observed: SIMD took 44.53% more time, processed input 30.80% more slowly, allocated 18.50% more Go-heap bytes, and performed 44.05% more Go allocations. The time samples had the widest stdlib spread in the capture (±7%), but the paired difference remained larger than that variation.

Inference: this fixture shows no CPU, throughput, or Go-allocation advantage for the SIMD arm. The Go-heap result cannot be extended to native or total memory.

### mixed-type-arrays

Observed: SIMD took 83.68% more time and input throughput fell 45.52%. Go-heap bytes rose 36.48% and allocation count rose 78.58%, with ±2% stdlib and ±3% SIMD time variation.

Inference: mixed arrays are the clearest loss in this workload. The direction is consistent across all four metrics rather than trading CPU for fewer allocations.

### number-heavy

Observed: SIMD took 79.09% more time and input throughput fell 44.18%. Go-heap bytes rose 32.55% and allocation count rose 47.94%, with ±5% stdlib and ±1% SIMD time variation.

Inference: the number-heavy fixture provides no evidence that SIMD tape parsing offsets the surrounding typed-sink and index-build costs. The result does not isolate why; it only measures the complete build path.

### combined

Observed: SIMD took 54.28% more time and input throughput fell 35.16%. Go-heap bytes rose 27.20% and allocation count rose 56.11%, with ±3% variation for both arms.

Inference: the largest checked-in fixture preserves the same direction as the three component fixtures. Within this bounded smoke tier, combining shapes does not reveal a crossover or an allocation-for-throughput trade.

## Cross-Fixture Assessment

Observed: SIMD was slower and had lower input throughput on every fixture. It also used more Go-heap bytes and made more Go allocations on every fixture. Benchstat's geomeans were +64.57% time, -39.22% throughput, +28.50% Go-heap bytes, and +56.12% allocations.

Inference: correctness parity is strong enough to keep the adapter technically viable, but the controlled workloads provide no performance reason to ship SIMD as a v1.3 performance option. Because the evidence is smoke-tier-only, it does not prove that every possible consumer workload loses. It does show that an affirmative performance claim would be unsupported, and optional external measurements should be treated as follow-up evidence rather than substituted into this committed decision.

## Recommendation

Defer the SIMD path as a shippable performance option for v1.3. Keep stdlib as the default and retain the parity/CI evidence, but require new controlled profiling or a materially different representative workload before presenting the SIMD adapter as a performance benefit. This is an evidence decision, not a regression-threshold failure: parity passed, while the measured CPU, throughput, and Go-allocation results all favored stdlib.

Recommendation class: **defer**
