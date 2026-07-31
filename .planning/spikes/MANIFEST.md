# Spike Manifest

## Idea

De-risk the SIMD workstream (Phases 21-22) by empirically verifying behavior before
planning — first `pure-simdjson`'s own API surface (Spike 001), then ami-gin's
differential parity harness under fuzzing (Spike 002).

## Requirements

Design decisions that emerged / were confirmed during spiking. Feed into phase planning.

### From Spike 001 (Phase 21)

- **SIMD floats MUST route through the non-coercing `StageFloat64` (D-01), never the coercing `stageNativeNumeric`.** pure-simdjson's `Type()` is lexeme-based (`1.0`/`1e18` → Float64), so parity with stdlib holds only if floats are staged as floats without whole-number→int folding.
- **Numeric routing MUST be driven by `Element.Type()`, not by opportunistically trying typed accessors** — `GetInt64` on a float element returns `ErrWrongType`, and `GetFloat64` on a >2^53 int returns `ErrPrecisionLoss`.
- **A >uint64 BIGINT fails the entire SIMD `Parse()`** (`BIGINT_ERROR`), unlike the stdlib path which parses the doc and rejects only the field. **Superseded in-tree:** `routeSIMDWellFormedFallback` (`parser_simd.go:60`) reconciles well-formed oversized integers, and Spike 002 confirmed empirically that they no longer diverge.
- **The `//go:build simdjson` tag is ami-gin's own convention** — upstream needs no tag and loads the native lib at runtime via `purego` (no CGo).

### From Spike 002 (Phase 22)

- **The differential fuzz target MUST guard input array-nesting depth (≤ 8) and total length (≤ 4096 bytes).** `stageMaterializedValue` is O(2^depth) for nested arrays on *both* parser arms; without the guard the fuzzer stalls at 0 execs/sec, and with a loose guard (depth 12) it silently under-fuzzes — depth 8 quadrupled corpus discovery in 3/4 the wall time.
- **The fuzz harness needs no poisoned-parser recovery logic.** The builder's tragic path requires a native document-close failure, which is unreachable from public-API input and is already covered by Phase 21's injected-fake lifecycle tests.
- **Reuse one `CloseableParser` across fuzz iterations**, constructed before `f.Fuzz` and closed in `f.Cleanup`. Per-iteration construction also works (~1µs warm) but buys nothing.
- **Classify differential outcomes three ways:** both ingest → assert byte equality (a mismatch is the Phase 19 HARD stop); exactly one ingests → the D-04 documented-exclusion class, record without failing; both reject → agreement.
- **`stageMaterializedValue`'s O(2^depth) array-nesting cost is a pre-existing default-path robustness issue**, not a SIMD or parity issue — both arms match within noise. It belongs in the backlog, NOT in Phase 22.

## Spikes

| # | Name | Type | Validates | Verdict | Tags |
|---|------|------|-----------|---------|------|
| 001 | pure-simdjson-api-verification | standard | Number Type() classification, iteration API/order, native bootstrap vs Phase 21 D-02/D-03 assumptions | ✓ VALIDATED (with BIGINT divergence finding) | simd, phase-21, numeric-parity, pure-simdjson, darwin-arm64 |
| 002 | simd-differential-fuzz-harness | standard | Parser construction cost, poisoned-builder reuse, handle stability at scale, and day-one divergence yield vs Phase 22 D-02 assumptions | ✓ VALIDATED (with O(2^depth) array-nesting finding) | simd, phase-22, fuzzing, parity, parser-lifecycle, darwin-arm64, complexity |
