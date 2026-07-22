# Phase 20: Realistic Benchmark Dataset Foundation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md - this log preserves the alternatives considered.

**Date:** 2026-07-21
**Phase:** 20-realistic-benchmark-dataset-foundation
**Areas discussed:** Dataset policy source, Smoke fixture shape, Benchmark integration

---

## Dataset Policy Source

| Option | Description | Selected |
|--------|-------------|----------|
| Synthesized smoke only | Checked-in generated fixtures shaped after simdjson examples; no upstream JSON rows vendored. | |
| Hybrid | Checked-in synthesized smoke fixtures plus documented opt-in path for exact upstream simdjson examples. | yes |
| Vendor exact upstream samples | Commit selected upstream JSON example files and handle license/NOTICE directly. | |
| Other | User-provided policy. | |

**User's choice:** Hybrid.
**Notes:** Default fixture data should be synthesized and checked in. Exact upstream simdjson examples can be supported as optional external/local benchmark inputs, not required for default tests or benchmarks.

---

## Smoke Fixture Shape

| Option | Description | Selected |
|--------|-------------|----------|
| Three focused JSONL fixtures | Separate checked-in smoke files for nested/high-cardinality, mixed-type arrays, and number-heavy data. | |
| Three focused plus one combined | The three focused fixtures plus one mixed end-to-end smoke fixture. | yes |
| Generated-only smoke | No committed JSONL; tests and benchmarks generate data at runtime. | |
| Other | User-provided shape policy. | |

**User's choice:** Three focused plus one combined.
**Notes:** The focused files keep failures easy to diagnose. The combined file gives one realistic mixed stream for end-to-end smoke benchmarks.

---

## Benchmark Integration

| Option | Description | Selected |
|--------|-------------|----------|
| Extend Phase 11 tier pattern | Add Phase 20 smoke/subset-style benchmark plumbing with default offline smoke and optional external simdjson examples. | yes |
| Extend Phase 13 parity pattern | Make these fixtures feed parser parity/golden checks first, so Phase 22 can compare stdlib vs SIMD byte-for-byte. | |
| Use both patterns | Benchmarks use Phase 11-style tiers, and Phase 22 later plugs the same fixtures into parser parity. | |
| New standalone layer | Separate Phase 20 benchmark/fixture system, not tied closely to Phase 11 or Phase 13. | |

**User's choice:** Extend Phase 11 tier pattern.
**Notes:** Keep the mental model simple: small fixture always runs; bigger or exact upstream data only runs when env vars are set. Phase 20 should prepare data that later SIMD work can reuse, but it does not need to wire SIMD parity itself.

---

## the agent's Discretion

- Exact fixture row counts and byte caps.
- Exact fixture helper names and benchmark projection names.
- Whether a small deterministic generator is worth adding.

## Deferred Ideas

- Vendoring exact upstream simdjson JSON examples by default.
- SIMD-vs-stdlib parity/CI wiring, which belongs to Phase 22.
