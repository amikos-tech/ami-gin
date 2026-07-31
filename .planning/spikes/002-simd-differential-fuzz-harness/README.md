---
spike: 002
name: simd-differential-fuzz-harness
type: standard
validates: "Given ami-gin's public builder API under -tags simdjson, when stdlib and SIMD arms are driven differentially, then parser construction cost, parser reuse across poisoned builders, handle stability at scale, and day-one divergence yield are known well enough to design the Phase 22 D-02 fuzz harness"
verdict: VALIDATED
related: [001]
tags: [simd, phase-22, fuzzing, parity, parser-lifecycle, darwin-arm64, complexity]
---

# Spike 002: SIMD Differential Fuzz Harness

## What This Validates

Given ami-gin's **public** builder API (`NewBuilder` / `WithParser` / `AddDocument` /
`Finalize` / `Encode`) built with `-tags simdjson`, when stdlib and SIMD arms are driven
differentially over seeds, adversarial inputs, and Go native fuzzing, then:

1. **Q1** — what `NewSIMDParser()` actually costs, and whether per-iteration construction is viable.
2. **Q2** — whether a reused `CloseableParser` survives a poisoned (tragic) builder.
3. **Q3** — whether one parser stays stable and leak-free across thousands of builder cycles.
4. **Q4** — day-one divergence yield: does differential fuzzing find zero divergences or many?

De-risks Phase 22 decision **D-02** (differential fuzz target) before planning.
See `.planning/phases/22-simd-validation-benchmarks-ci/22-CONTEXT.md`.

## Research

No new upstream research needed — Spike 001 already captured the `pure-simdjson` API
verbatim. This spike is about **ami-gin's own** behaviour under fuzzing, so the relevant
reading was in-tree:

- `builder.go:390-426` — `AddDocument`'s tragic-poisoning path. A parser **panic** during
  parse (`:414`) or an `isParserLifecycleError` (`:423`) sets `b.tragicErr`; every later
  `AddDocument`/`Finalize` refuses with *"discard and rebuild"*.
- `parser.go:40-43` — `isParserLifecycleError` matches only `*parserLifecycleError`.
- `parser_simd.go:133-162` — `finishSIMDDocument` constructs a `parserLifecycleError`
  **only when the native document close fails**. This is the load-bearing discovery for Q2:
  the tragic path is not reachable from ordinary JSON input, only from an injected fake.
- `parser.go:94-101` — `GINBuilder` never closes a supplied `CloseableParser`; ownership
  stays with the caller.

**Convention followed (Spike 001):** own module (`module simdfuzzspike` + local `go.mod`),
so the repo's `go.mod`-of-record is never mutated. Deviation from 001: this spike needs
ami-gin itself, so it uses `replace github.com/amikos-tech/ami-gin => ../../..`. That works
because the entire differential runs on the **public** API — no unexported access needed.

**Caveat:** the spike module resolves its own dependency versions, so a few transitive deps
(roaring v2.24.0, ojg v1.28.2, parquet-go v0.30.1) are newer than the repo's. This does not
affect any conclusion, because both arms of every comparison use the *same* ami-gin build —
the differential is stdlib-vs-SIMD, not spike-vs-repo.

## How to Run

```bash
cd .planning/spikes/002-simd-differential-fuzz-harness

# Q1, Q2 — fast
PURE_SIMDJSON_WARN_LEAKS=1 go test -tags simdjson -run 'TestQ1_|TestQ2_' -v

# Q1b, Q2b — the honest follow-ups
PURE_SIMDJSON_WARN_LEAKS=1 go test -tags simdjson -run 'TestQ1b_|TestQ2b_' -v

# Q3, Q4b — scale + deterministic differential
PURE_SIMDJSON_WARN_LEAKS=1 go test -tags simdjson -run 'TestQ3_|TestQ4b_' -v

# Q5 — the unplanned complexity finding
go test -tags simdjson -run 'TestQ5_' -v

# Q4 — actual fuzzing
PURE_SIMDJSON_WARN_LEAKS=1 go test -tags simdjson -run '^$' -fuzz='^FuzzParserParity$' -fuzztime=60s
```

## What to Expect

Every test passes. The interesting output is the `t.Logf` measurements, not the verdicts.
`TestQ5_NestingDepthScaling` prints a table where the stdlib column roughly **doubles per
nesting level** for arrays and stays linear for objects.

## Investigation Trail

**Pass 1 — Q1 and Q2 as originally framed.**
Q1 returned `cold=5.77ms, warm_mean=1.11µs`. That *contradicted* the premise of the spike:
the assumption was that per-iteration `NewSIMDParser()` would be too slow to fuzz. It isn't —
the native library loads once per process and subsequent construction is ~1µs.

Q2 passed, but on inspection **it passed vacuously**. The `>uint64` BIGINT payload chosen as
the "poison" produced `addErr=<nil>, finalizeNil=false, tragic=false` — it did not poison
anything. That is consistent with `routeSIMDWellFormedFallback` (`parser_simd.go:60`)
reconciling well-formed oversized integers, which already supersedes Phase 21's D-05 framing.
So Q2 proved "the parser is reusable across builders" but **not** the claim in its own name.

**Pass 2 — reframing Q2.**
Reading `parser_simd.go:133-162` showed `parserLifecycleError` arises *only* when the native
document close fails. That cannot be induced through the public API; it needs a package-internal
fake — which Phase 21's `parser_simd_lifecycle_test.go` already does. So the answerable question
is not "does the parser recover from poisoning" but **"can any public-API input poison it at
all"**. `TestQ2b_NoPublicInputPoisonsBuilder` sweeps 88 inputs (63 seeds + 25 adversarial),
re-checking a canary after every single one.

**Pass 3 — Q4b hung for 600 seconds.**
The deterministic differential over the same 88 inputs that Q2b had swept in 0.26s ran for
610s and died. The crash dump was a tower of `builder.go:621 stageMaterializedValue` frames
with the canonical path growing ~3 bytes per frame. Q2b had only exercised the SIMD arm;
Q4b added the stdlib arm — but the real variable was a 2000-deep nested-array input.

**Pass 4 — Q5, measuring instead of guessing.**
`TestQ5_NestingDepthScaling` sweeps nesting depth for arrays and objects on both arms:

| depth | stdlib | simd |
|---|---|---|
| 8 | 17.8ms | 18.4ms |
| 10 | 60.0ms | 62.6ms |
| 12 | 246.5ms | 257.0ms |
| 14 | 966.6ms | 1.197s |
| 16 | 2.804s | 2.868s |
| 18 | TIMEOUT (>3s) | TIMEOUT (>3s) |

Nested **objects** at depth 512 cost 14.9ms — linear. So the blow-up is arrays only, and
**both arms are identical within noise**.

Mechanism confirmed by code, not inferred — `builder.go:619-627`:

```go
case []any:
    for i, item := range v {
        b.stageMaterializedValue(fmt.Sprintf("%s[%d]", path, i), item, state, true)  // [i]
        b.stageMaterializedValue(path+"[*]", item, state, true)                      // [*]
    }
```

Every array element recurses **twice** — once under `[i]`, once under `[*]`. Nested arrays of
depth *d* therefore cost exactly **2^d**. Objects recurse once per key, hence linear. A
**37-byte** input (`[[[…1…]]]` at depth 18) exceeds three seconds of CPU.

**Pass 5 — fuzzing, and an A/B on the guard.**
First 60s run with a depth-12 guard: 180,277 execs, 21 new interesting, but **30 of the 60
seconds sat at 0 execs/sec** — 16 workers all starved on slow inputs, exactly as Q5 predicts
for depth 12 (~246ms/doc). Tightening to depth ≤ 8 and ≤ 4096 bytes: 181,235 execs in *45*
seconds and **85 new interesting** (corpus 72 → 157). Same throughput in 3/4 the time, 4× the
exploration.

Zero crashers in either run. No `testdata/fuzz` corpus was persisted, and no
`PURE_SIMDJSON_WARN_LEAKS` finalizer warnings appeared on stderr in any run.

## Results

**VERDICT: VALIDATED** — the D-02 harness is viable, with one mandatory guard nobody had accounted for.

| Q | Question | Answer |
|---|---|---|
| Q1 | Parser construction cost | cold 5.77ms (one-time native load), warm **1.11µs** mean / 6.04µs worst |
| Q1b | Per-iteration construction viable? | **Yes** — 5,000 construct+build+close cycles in 896ms (5,581/sec), 0 mismatches. Construction is not the bottleneck; the build/encode is. |
| Q2 | Parser survives poisoned builder? | **Question was malformed.** No public-API input reaches the tragic path. |
| Q2b | Can any public input poison the builder? | **No.** 88 inputs (63 seeds + 25 adversarial): 0 poisoned, 0 left the parser unusable. |
| Q3 | Stability / leaks at scale | 25,200 builder cycles on **one** parser in 17.7s (1,420/sec): 0 failures, 0 output drift, 0 leak warnings. |
| Q4b | Deterministic differential yield | 88 inputs: **agree=88, asymmetry=0, byteDivergence=0.** |
| Q4 | Fuzzed divergence yield | **~361,000 executions across two runs, zero crashers, zero byte divergences.** |
| Q5 | *(unplanned)* nesting-depth cost | `stageMaterializedValue` is **O(2^depth)** for nested arrays on **both** arms. Objects linear. |

### Surprises

1. **Per-iteration parser construction is cheap (~1µs), not expensive.** The design fork the
   spike was commissioned to resolve turned out not to be a fork at all — both harness shapes
   work. Reuse is still preferable (fewer moving parts), but nothing forces it.

2. **The tragic/poisoning path is unreachable from fuzz input.** It requires a native
   *close* failure, which needs an injected fake. Phase 21's lifecycle tests already own that
   case. The fuzz harness needs no recovery logic whatsoever.

3. **`stageMaterializedValue` is exponential in array nesting depth — on the DEFAULT path.**
   This is not a SIMD issue and not a parity issue (both arms match within noise), which is
   why it is a *finding* rather than a Phase 22 blocker. But 37 bytes of untrusted JSON
   costing >3s of CPU is an algorithmic-complexity concern for the default ingest path.

4. **The depth guard is a throughput parameter, not a correctness checkbox.** Depth 12
   "works" but starves the fuzzer to 0 execs/sec for half the run. Depth 8 quadrupled corpus
   discovery. Anyone writing this harness without Q5's table would pick 12 or higher and
   silently get a fuzzer that barely fuzzes.

### Signal for Phase 22 D-02

- Harness shape is free: **reuse one parser** (simplest), constructed before `f.Fuzz`, closed
  in `f.Cleanup`. No poisoned-parser recovery needed.
- **Mandatory input guard:** skip inputs with array nesting depth > **8** or length > **4096
  bytes**. Without it the fuzzer appears to hang; with a loose value it silently under-fuzzes.
- Classify outcomes three ways — *both ingest* (assert byte equality; a mismatch is the Phase 19
  HARD stop), *exactly one ingests* (the D-04 documented-exclusion class; record, do not fail),
  *both reject* (agreement).
- Day-one expectation is **zero divergences**. D-02 is a regression tripwire and a discovery
  net, not a bug hunt — which matches the CONTEXT framing that Phase 22 pins a passing invariant.
- Seed corpus of ~63 entries (authored numeric/structural edges + 12 lines per Phase 20 fixture)
  produced 51 baseline-coverage units and grew to 157 under fuzzing.

### Out of scope for Phase 22 (backlog candidate)

The O(2^depth) array behaviour is a pre-existing default-path robustness issue, unrelated to
SIMD. It deserves its own backlog item — the repo already has `serialize_security_test.go` and
a `security.yml`, so untrusted-input hardening has precedent. Phase 22 should **not** absorb it;
that would be scope creep into a phase whose job is evidence.
