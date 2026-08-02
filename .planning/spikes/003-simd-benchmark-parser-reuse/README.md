---
spike: 003
name: simd-benchmark-parser-reuse
type: standard
validates: "Given Plan 22-05's one-shared-SIMD-parser mandate, when the same Phase 20 fixtures are measured under different parser lifecycles and orderings, then per-fixture ns/op, B/op and allocs/op are unbiased and the stdlib comparison is fair"
verdict: VALIDATED
related: [001, 002]
tags: [simd, phase-22, benchmark, parser-lifecycle, measurement-validity, darwin-arm64, performance]
---

# Spike 003: SIMD Benchmark Parser-Reuse Bias

Commissioned from the Phase 22 cross-AI review (`22-REVIEWS.md`), where Codex
flagged that Plan 22-05 reuses one SIMD parser across all four Phase 20 fixtures
while the stdlib arm has no equivalent persistent state — with D-12 forbidding
any regression threshold that would catch skewed evidence.

## What This Validates

**Given** Plan 22-05's mandate (`22-05-PLAN.md:95`: one caller-owned SIMD parser
constructed before any timed region, reused across all four smoke fixtures),
**when** the same fixtures are measured under different parser lifecycles and
orderings, **then** per-fixture `ns/op`, `B/op` and `allocs/op` should be
unbiased, and the SIMD-vs-stdlib comparison should be fair.

Three questions, risk-ordered:

- **Q1 — Order dependence.** Does a fixture measure differently when run first
  through a fresh parser vs. last after three others?
- **Q2 — Shared vs. per-fixture parser.** Is the SIMD↔stdlib delta stable across
  parser lifecycles?
- **Q3 — Cold vs. steady-state.** Where does per-iteration cost plateau, and does
  `b.N` amortize warm-up away?

## Research

Read before writing code, per CONVENTIONS.md ("capture verbatim API from the
module cache"). Source: `$GOMODCACHE/github.com/amikos-tech/pure-simdjson@v0.1.7`.

| Finding | Location | Consequence for this spike |
|---|---|---|
| `Parser` holds `handle ffi.ParserHandle`; `newParserWithConfig` "allocates a **reusable** native parser" | `parser.go:21-46` | Buffer retention across `Parse` was plausible — the premise of the concern |
| Native allocation telemetry exists: `NativeAllocStatsReset` / `NativeAllocStatsSnapshot` → `TotalAllocBytes`, `AllocCount`, `LiveBytes` | `benchmark_native_alloc_test.go` | **Package-internal.** Exported `Parser` surface is only `Parse`/`Close` (`parser.go:66,113`) |
| Go's `B/op` counts Go-heap allocations only | — | Native buffer growth is **invisible** to `B/op`; process RSS is the only externally available signal |

**Chosen approach:** isolated Go module (`module simdbenchspike`) with
`replace github.com/amikos-tech/ami-gin => ../../..`, driving only the public
API — reusing Spike 002's module scaffolding rather than rebuilding it.
`harness_test.go` mirrors `benchmark_test.go`'s `phase20LoadRawJSONL`,
`phase20BuildBenchmarkIndex` and `phase20SmokeFixtures` (those live in a
package-internal `_test.go` file and cannot be imported). Measurement uses
`testing.Benchmark()` driven from ordinary `Test` functions so orderings are
controlled precisely within one process and results print as tables.

**Gotcha:** `parser_simd.go` is `//go:build simdjson`; every command needs
`-tags simdjson`, which applies to the ami-gin dependency too.

## How to Run

```bash
cd .planning/spikes/003-simd-benchmark-parser-reuse

# Q1 — order dependence (~14s)
PURE_SIMDJSON_WARN_LEAKS=1 go test -tags simdjson -run 'TestQ1_' -v

# Q2 — shared vs fresh vs stdlib, with vacuity guard (~21s)
PURE_SIMDJSON_WARN_LEAKS=1 go test -tags simdjson -run 'TestQ2_' -v

# Depth — the fresh-parser anomaly, 5 repeats x 4 fixtures (~68s)
PURE_SIMDJSON_WARN_LEAKS=1 go test -tags simdjson -run 'TestDepth_' -v

# Q3 — cold vs steady-state sweep (~6s)
PURE_SIMDJSON_WARN_LEAKS=1 go test -tags simdjson -run 'TestQ3_' -v

# Follow-up — document-size crossover sweep (~25s)
PURE_SIMDJSON_WARN_LEAKS=1 go test -tags simdjson -run 'TestFollowup_' -v

# Validity — byte-identical output across arms (<1s)
PURE_SIMDJSON_WARN_LEAKS=1 go test -tags simdjson -run 'TestValidity_' -v
```

## What to Expect

Tables on stdout. This is a fact-finding spike (benchmark numbers), so stdout
verification is correct per CONVENTIONS.md — no UI.

## Investigation Trail

**1. Q1 came back clean — suspiciously so.** Every fixture measured within
±0.9% `ns/op` and ±0.0% `B/op` between first-position and last-position, with
warm RSS growth of 0.0–0.2 MiB. Identical `allocs/op` to the single allocation.

CONVENTIONS.md warns: *"Distrust a test that passes on the first try. Verify the
precondition actually fired."* Numbers that identical are exactly what a
silently-disengaged `WithParser` would produce. Rather than believe Q1, a
**vacuity guard** was built into Q2: if SIMD lands within 2% of stdlib on all
four fixtures, `t.Fatalf` — because then neither test proves anything.

**2. The guard passed (0/4 within 2%) — and exposed the real story.** SIMD is
**1.45×–1.84× slower** than stdlib and allocates **18–36% more Go heap** with
~44% more `allocs/op`. Q1's negative result is therefore trustworthy, but the
commissioning concern turned out to be the least interesting thing here.

**3. Q2 also threw an anomaly: `simd-fresh` 3× slower than `simd-shared`** for
`mixed-type-arrays` (17.1ms vs 5.6ms) and `number-heavy` (11.1ms vs 4.0ms), but
statistically identical (+0.3%) for the other two. A 3× split across fixtures
with no pattern is either a real per-construction penalty or an artifact.

**4. `depth_test.go` killed the anomaly.** 5 repeats × 4 fixtures = 20 paired
comparisons: fresh-vs-shared within **±2.5% every single time**, with healthy
`N` (61–312, not a noisy small-N artifact). `TestDepth_ConstructionCost`
independently measured `NewSIMDParser`+`Close` at **828 ns/op, 216 B/op,
10 allocs/op** — four orders of magnitude below the multi-millisecond deltas,
so construction cannot explain a 3× swing. The Q2 outlier was transient machine
noise. *This is itself a Phase 22 finding: a single `COUNT=1` run can be 3× wrong.*

**5. Q3 found no measurable warm-up.** Iteration 1 sat +0.0% to +20.2% above the
steady-state median; only `number-heavy` had a single iteration above the 20%
threshold, and run-to-run variance (±12%) swamps any warm-up signal. No
monotonic curve — just noise around the median.

**6. Follow-up: hunting the crossover — and not finding one.** Hypothesis: Phase
20 records are small (~250–500 B), and simdjson wins on large buffers, so
per-document FFI overhead through purego should dominate and there should be a
document-size crossover. Swept 265 B → 262 KB per document at constant ~2 MiB
total volume. **No crossover.** SIMD stayed 1.24×–1.51× slower across three
orders of magnitude, with `B/op` flat at 1.11×–1.16×. The hypothesis was wrong:
cost tracks *total volume*, not document count — consistent with an FFI crossing
per scalar leaf rather than per document.

**7. Validity guard.** A "SIMD is slower" claim is worthless if the SIMD arm was
doing less work. `validity_test.go` asserts byte-identical encoded indexes across
both arms on all four fixtures — they match exactly (10920/7478/6432/18309
bytes). The comparison is apples-to-apples.

## Results

**Verdict: VALIDATED** — the commissioning concern is disproven; a larger finding
replaced it.

### Q1 — Order dependence: NONE

| fixture | ns/op first | ns/op last | delta | B/op delta | warm RSS |
|---|---|---|---|---|---|
| nested-high-cardinality | 7,507,238 | 7,489,483 | −0.2% | +0.0% | 0.2 MiB |
| mixed-type-arrays | 5,570,553 | 5,569,345 | −0.0% | −0.0% | 0.0 MiB |
| number-heavy | 3,905,745 | 3,940,238 | +0.9% | +0.0% | 0.0 MiB |
| combined | 18,563,646 | 18,598,054 | +0.2% | +0.0% | 0.0 MiB |

### Q2 — Parser lifecycle: NO EFFECT; but SIMD loses to stdlib

| fixture | stdlib ns/op | simd ns/op | simd/std | stdlib B/op | simd B/op | B ratio |
|---|---|---|---|---|---|---|
| nested-high-cardinality | 5,081,889 | 7,572,236 | **1.49×** | 4,365,005 | 5,172,465 | 1.18× |
| mixed-type-arrays | 3,039,151 | 5,585,398 | **1.84×** | 2,554,624 | 3,486,902 | 1.36× |
| number-heavy | 2,410,759 | 3,956,377 | **1.64×** | 1,871,587 | 2,480,923 | 1.33× |
| combined | 12,798,981 | 18,554,089 | **1.45×** | 8,647,720 | 11,001,993 | 1.27× |

### Q3 — Cold vs steady-state: warm-up not measurable above noise

`b.N` lands at 61–312 for these fixtures, so any first-iteration cost is
amortized to near-zero. **Every committed number is already a steady-state
number.**

### Follow-up — document size: no crossover

| doc size | docs | stdlib ns/op | simd ns/op | simd/std | B ratio |
|---|---|---|---|---|---|
| 265 B | 7913 | 244,799,617 | 367,493,528 | 1.50× | 1.16× |
| 1,031 B | 2034 | 254,966,229 | 340,935,833 | 1.34× | 1.15× |
| 4,115 B | 509 | 253,887,292 | 314,205,833 | 1.24× | 1.16× |
| 16,400 B | 127 | 213,232,517 | 285,676,740 | 1.34× | 1.15× |
| 65,566 B | 31 | 226,810,117 | 341,445,681 | 1.51× | 1.14× |
| 262,172 B | 7 | 285,891,708 | 374,781,222 | 1.31× | 1.11× |

### Answers to the commissioned questions

1. **Order dependence?** No. ≤0.9% `ns/op`, 0.0% `B/op`.
2. **Shared vs. per-fixture parser?** No difference (±2.5% over 20 paired runs).
   Plan 22-05's shared-parser mandate is **sound as written** and needs no change.
3. **Cold or steady-state?** Steady-state, already — `b.N` amortizes warm-up.
   Plan 22-06 should *label* it steady-state rather than change anything.

### Surprises

- **SIMD is uniformly slower in ami-gin's ingest path** (1.24×–1.84×) and
  allocates more Go heap (1.11×–1.36×), on byte-identical output, across every
  fixture and every document size tested. This bears directly on Plan 22-06's
  ship/defer/narrow decision.
- **`B/op` cannot see native memory.** Go's allocator metrics count only the Go
  heap; pure-simdjson's native buffers are invisible. Any "SIMD uses less memory"
  claim from `B/op` would be a measurement artifact. Here SIMD uses *more* Go
  heap anyway, so the conclusion is unaffected — but the caveat must be recorded
  before someone reads a favourable `B/op` as a memory win.
- **A single `COUNT=1` run produced a 3× wrong number.** Directly justifies Plan
  22-06's `COUNT=10` requirement and the "shared-runner-noisy, trend only"
  disclaimer on the CI artifact (D-11).
- **The parity gate passes on all four Phase 20 fixtures** — byte-identical
  encoded indexes. Corroborates what Plan 22-02 sets out to prove.

### Limitations

- Single machine (darwin/arm64 laptop), one session. Thermal/background state is
  the most likely confounder; the depth test's 20 paired repeats mitigate but do
  not eliminate it.
- Measures **full index build** (`NewBuilder` → `AddDocument`×N → `Finalize`),
  which is what Plan 22-05 measures. Parse is only a fraction of that, so the
  SIMD parse+staging path in isolation is likely worse than the 1.2×–1.8× totals.
- Does **not** isolate *why* SIMD is slower. The per-scalar-FFI-crossing theory
  fits the volume-tracking evidence but was not directly instrumented — that
  would need the internal `NativeAllocStats` API or a profile.
