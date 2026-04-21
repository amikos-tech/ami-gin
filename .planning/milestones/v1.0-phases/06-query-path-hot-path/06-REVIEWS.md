---
phase: 6
reviewers: [gemini, opencode]
reviewers_attempted: [gemini, codex, opencode]
reviewers_failed: [codex]
reviewed_at: 2026-04-14T15:25:00Z
plans_reviewed: [06-01-PLAN.md, 06-02-PLAN.md]
---

# Cross-AI Plan Review — Phase 06

## Gemini Review

# Plan Review: Phase 06 — Query Path Hot Path

## 1. Summary
The proposed plans for Phase 06 are high-quality, technically sound, and strictly aligned with the project's "pruning-first" philosophy. Plan 06-01 effectively solves the O(n) bottleneck by introducing a derived lookup map that is reconstructed during `Finalize` and `Decode`, thereby avoiding a breaking change to the serialization format. Plan 06-02 provides the necessary empirical rigour to validate these changes against high-cardinality path scenarios. The strategy of "canonicalize at the edge" (in the builder and query entry points) ensures that the internal index remains a "source of truth" with zero ambiguity.

---

## 2. Strengths
*   **Zero Format Churn**: Rebuilding the `pathLookup` map during `Decode` is a clever way to achieve O(1) lookups without incrementing the serialization version or increasing disk footprint.
*   **Canonical Integrity**: By forcing normalization inside `builder.go` before any internal IDs are assigned, the plan ensures that the `PathDirectory` itself becomes canonical. This simplifies serialization and debugging (CLI output will always show the same spelling).
*   **Defensive Query Boundary**: The decision to create a query-specific normalization helper that validates the supported subset (`$`, `$.field`, `$[*]`) prevents "feature creep" where the index might accidentally return results for unsupported JSONPath syntax.
*   **Realistic Benchmarking**: Using a log-style corpus with synthetic widening (Task 2 of Plan 06-02) directly targets the primary risk of GIN indexes: performance degradation in "wide" schemas with thousands of unique paths.

---

## 3. Concerns
*   **Thread Safety (MEDIUM)**: The `GINIndex` struct will now hold a `pathLookup map`. While `Finalize` and `Decode` are typically called during initialization, if a `GINIndex` is used concurrently in a high-traffic query environment, the map must be strictly read-only after construction.
    *   *Risk*: If any "lazy loading" or "just-in-time" rebuilding were introduced later, it would cause race conditions.
*   **Memory Overhead (LOW)**: For an index with 10,000 paths, a `map[string]uint16` adds roughly 500KB–1MB of heap overhead. This is negligible given the performance gains, but should be noted for extremely memory-constrained environments.
*   **Normalization Edge Cases (LOW)**: JSONPath allows for escaped characters in bracket notation (e.g., `$['field.with.dot']`).
    *   *Risk*: If `NormalizePath` isn't robust, `$.foo` and `$['foo']` might converge, but more complex valid paths might be mangled.

---

## 4. Suggestions
*   **Explicit Immutable Contract**: Ensure that the `pathLookup` map is never modified after `Finalize` or `Decode` returns. Consider adding a comment in `gin.go` explicitly stating that the `GINIndex` is immutable once returned to the user.
*   **Warm-up the Map in Tests**: In the regression coverage (06-01, Task 3), explicitly test a "round-trip" where an index is built, encoded to a byte slice, decoded, and *then* queried with an alternative spelling. This ensures the `Decode` rebuild logic is fully exercised.
*   **Benchmark Baseline**: In Plan 06-02, ensure you run the benchmarks against the *current* v0.1.0 code before applying Phase 06 changes to establish a "before" baseline. The synthetic widening should reveal the point at which linear scans start to hurt (usually >100 paths).
*   **Validation Logic**: Ensure the query-facing helper in `jsonpath.go` returns a clear error or a "not supported" signal if a user tries to use an index/slice (e.g., `$[0]`) that isn't in the supported subset, rather than just failing the lookup.

---

## 5. Risk Assessment
**Overall Risk: LOW**

**Justification**:
The plan is highly surgical. It optimizes a known hot path (`findPath`) using a standard computer science trade-off (memory for speed) without changing the core pruning logic or the binary format. The dependencies are well-managed, and the success criteria are measurable via the provided benchmark plan. The most significant risk—breaking the serialization format—has been explicitly avoided.

---

## Codex Review

**Status: FAILED** — Two attempts (14:35 UTC and 15:07 UTC). Both `codex exec` invocations reached sleeping (S) state and remained there with zero bytes written. Host had 4-9 concurrent `codex --yolo` sessions competing for resources; appears to be an environment/concurrency issue rather than a content problem. Killed both tasks. A fresh attempt from a clean terminal session (outside this Claude harness) is the recommended next try.

---

## OpenCode Review

## Plan 06-01: Canonical path storage + fast lookup pipeline

**Summary**

This plan is directionally correct and matches the phase goal well: canonicalize supported JSONPath spellings at the storage/query boundary, replace linear scans with a derived lookup map, and keep the wire format stable by rebuilding the lookup after `Finalize()` and `Decode()`. It is appropriately scoped for PATH-01 and PATH-02. The main risks are around making sure canonicalization happens at every path entry point, preserving current behavior for unsupported/internal paths, and avoiding accidental collisions or duplicate canonical entries during finalize.

**Strengths**

- Targets the real hot path: `findPath()` linear scanning.
- Uses a derived in-memory structure instead of changing serialization format.
- Correctly identifies that both `Finalize()` and `Decode()` must rebuild lookup state.
- Keeps unsupported/unknown-path behavior stable, which aligns with D-05.
- Separates internal normalization from query-facing validation, reducing the risk of broadening supported syntax.
- Regression test coverage includes encoded/decoded parity, which is easy to miss.

**Concerns**

- HIGH: The plan does not explicitly say how duplicate canonical paths are handled if older indexes or mixed builder inputs produce multiple stored spellings that normalize to the same canonical string. A silent overwrite in `pathLookup` could create non-deterministic behavior.
- MEDIUM: Canonicalizing in `builder.go` before transformer lookup and FTS matching is likely right, but it can subtly change behavior if any existing config keys were written against non-canonical spellings and currently rely on exact string matching.
- MEDIUM: The plan says internal array paths like `$.items[0]` and `$.items[*]` remain distinct, but it does not explicitly state how the query-facing helper rejects or preserves unsupported index syntax so query validation does not accidentally start accepting internal-only path forms.
- MEDIUM: `uint16` in `pathLookup map[string]uint16` mirrors current `PathID`, but the plan should confirm that the project already guarantees path counts fit that range and that canonicalization cannot increase collision pressure.
- LOW: Test coverage mentions absent path fallback, but not malformed query paths that should still be rejected or handled exactly as they are today.

**Suggestions**

- Define duplicate-canonical-path behavior explicitly during finalize/decode:
  - If duplicates are impossible after builder changes, assert that in tests.
  - If decoded legacy data can contain duplicates, pick a deterministic rule or fail clearly.
- Add tests for query-path handling boundaries:
  - `$.foo`
  - `$['foo']`
  - `$["foo"]`
  - unsupported inputs such as `$.items[0]`, recursive descent, slices, filters
- Add one test that verifies transformer and/or FTS path config still works when configured with equivalent supported spellings.
- Make the rebuild helper the single source of truth for `pathLookup` construction so finalize/decode cannot drift.
- Document in code that `pathLookup` is derived, non-serialized state and must always be rebuilt after deserialization.

**Risk Assessment**

**MEDIUM**. The approach is solid and minimal, but correctness depends on handling canonical duplicates and preserving current validation boundaries. Those are manageable with a few explicit tests and a clearly defined rebuild policy.

---

## Plan 06-02: Wide-path benchmarks

**Summary**

This is a good benchmark companion plan for 06-01. It exercises the intended regression surface by combining wide path-count fixtures with operator coverage across EQ, CONTAINS, and REGEX, and it aligns with the requirement to benchmark realistic lookup behavior rather than only synthetic microcases. The main weakness is that it still needs tighter definition around benchmark shape, control variables, and what specifically is being measured so results are attributable to path lookup improvements rather than unrelated trigram or regex costs.

**Strengths**

- Covers all required operators for PATH-03.
- Uses multiple width tiers, which is important for exposing path-scan scaling behavior.
- Includes equivalent-spelling lookup cases, directly validating D-01 and D-03.
- Blends recognizable log-like shapes with synthetic widening, which matches D-04.
- Keeps scope limited to `benchmark_test.go`, avoiding unnecessary production-code churn.

**Concerns**

- HIGH: The plan does not explicitly isolate path-resolution cost from total query cost. For CONTAINS and especially REGEX, trigram work or regex analysis may dominate and mask whether path lookup improved.
- MEDIUM: "At least 3 width tiers" is good, but the plan does not specify document count, row-group count, path fanout, or match selectivity. Without stable parameters, benchmark output may be noisy or hard to compare over time.
- MEDIUM: Equivalent-spelling coverage is mentioned, but not whether it is benchmarked as a same-index/same-data paired case against canonical spelling to make regression obvious.
- MEDIUM: There is no explicit baseline benchmark for narrow path-count workloads, so a regression on common small indexes could be missed.
- LOW: The naming target `BenchmarkQuery(EQ|Contains|Regex)` is helpful, but the plan does not say whether sub-benchmarks will encode width tier and spelling variant clearly enough for CI/history comparisons.

**Suggestions**

- Add benchmark structure that isolates the variable under test:
  - Same corpus, same predicate semantics, only vary path count and path spelling.
- Include one narrow control tier alongside wide tiers, such as `16` or `32` paths, to catch regressions on small indexes.
- Fix benchmark parameters explicitly:
  - number of documents
  - row groups
  - paths per document
  - match/non-match selectivity
  - regex pattern shape
- For CONTAINS and REGEX, choose patterns where path lookup remains material; otherwise add a dedicated micro-benchmark for `findPath()`/predicate path resolution if the integrated benchmark is too noisy.
- Use sub-benchmark names that encode operator, width, and spelling, for example:
  - `EQ/paths=512/spelling=canonical`
  - `EQ/paths=512/spelling=bracket`
- State whether benchmark success means "improvement or no regression within noise threshold," even if enforcement remains manual.

**Risk Assessment**

**MEDIUM**. The benchmark direction is good, but without tighter control of workload shape and attribution, results may be ambiguous and fail to prove the hot-path improvement convincingly.

---

## Overall Assessment

Both plans are well aligned with the phase goal and stay within scope. Plan 06-01 is the stronger of the two and should deliver the intended behavior if duplicate canonical path handling and validation boundaries are made explicit. Plan 06-02 is valuable, but it needs more benchmark design discipline so the data clearly supports the performance claim rather than mixing multiple costs together. Overall phase risk is **MEDIUM**: the implementation approach is sound, and the remaining risks are mostly edge-case correctness and benchmark quality, not architectural failure.

---

## Consensus Summary

Two reviewers returned successful responses (Gemini and OpenCode). Codex failed on both attempts due to host load (not a content issue). The two successful reviews converge on the plan direction but diverge meaningfully on risk rating and on which correctness concerns matter most — Gemini sees this as LOW risk with polish items, OpenCode sees it as MEDIUM with two HIGH-severity gaps that need explicit treatment.

### Agreed Strengths

- **Zero format churn via rebuild-on-Decode** — both reviewers flag the derived-lookup approach avoiding a serialization version bump as the right architectural call.
- **Canonicalization at the boundary (builder in, query in)** — both agree this is the correct integration point.
- **Defensive query-boundary validation** — validate-then-normalize ordering prevents accidental widening of the supported JSONPath surface.
- **Operator coverage + wide-path fixtures in 06-02** — both agree the benchmark strategy targets the right risk profile.
- **Decoded-index parity testing** — both call out the `Encode → Decode → query` round-trip as easy-to-miss and important.

### Agreed Concerns

- **Query-boundary helper must explicitly reject unsupported syntax** (Gemini LOW, OpenCode MEDIUM): both reviewers want explicit handling of `$[0]`, recursive descent, slices, and filters — and want a test that locks it down. Gemini framed this as "clear error signal"; OpenCode framed it as "don't let internal-only path forms leak into the query surface."
- **Thread-safety / immutability contract on `pathLookup`** (Gemini MEDIUM, OpenCode implicit): Gemini explicitly flagged the race-condition risk if lazy rebuild is ever introduced. OpenCode's "document that `pathLookup` is derived, non-serialized state" overlaps with the same concern.

### New Concerns From OpenCode (Gemini did not flag)

- **HIGH — Duplicate canonical paths after Finalize/Decode**: What happens if two stored path spellings normalize to the same canonical string? Plan does not define the rule. Silent map overwrite would be non-deterministic. Needs either an assertion (duplicates are impossible) or an explicit tie-breaker. This is the single most important gap in the current plan.
- **HIGH — Benchmark attribution for CONTAINS / REGEX**: Trigram and regex-literal extraction work could dominate query time and mask the actual path-lookup improvement. PATH-03 claims "measurable lookup improvement" but the benchmark as-designed may not attribute improvement to path resolution specifically. Needs either tighter fixture design or a dedicated `findPath`-focused micro-benchmark as a control.
- **MEDIUM — Transformer / FTS config key compatibility**: If any existing config keys were written using non-canonical spellings, running every path through `NormalizePath()` before transformer/FTS lookup could silently change behavior. Worth a test.
- **MEDIUM — `uint16` collision / capacity**: `pathLookup map[string]uint16` inherits the `PathID` range. Plan should confirm that canonicalization cannot produce more distinct canonical paths than the project already tolerates.
- **MEDIUM — Benchmark parameters not fixed**: Width tiers are defined but doc count, RG count, selectivity, and regex pattern shape are not, which will make historical comparisons noisy.

### Divergent Views

- **Overall risk rating**: Gemini says **LOW** (surgical optimization with no format change); OpenCode says **MEDIUM** (two HIGH-severity correctness/attribution gaps). The delta is almost entirely about whether duplicate canonical path handling and benchmark attribution are treated as blocking.
- **Focus axis**: Gemini weighs *future* risk (lazy-init regressions, memory overhead at 10K paths) and polish (immutability comments, baseline capture). OpenCode weighs *present* correctness (duplicate rule, attribution, config-key compatibility) and benchmark discipline (fixed parameters, control tier, sub-benchmark naming).

### Suggestions Worth Incorporating (consolidated)

Priority order — most impactful first:

1. **[HIGH, OpenCode]** Define duplicate-canonical-path behavior during `Finalize()`/`Decode()`. Either assert it cannot happen after builder canonicalization (and cover with a test that exercises mixed-spelling input) or pick a deterministic tie-breaker and document it.
2. **[HIGH, OpenCode]** Add a dedicated `findPath()`-focused micro-benchmark or tighten the CONTAINS/REGEX fixtures so path-resolution improvement is attributable rather than lost inside trigram/regex cost.
3. **[MEDIUM, both]** Add an explicit query-boundary test matrix: `$.foo`, `$['foo']`, `$["foo"]`, `$.items[0]`, recursive descent, slices, filters. Supported forms converge; unsupported forms stay rejected.
4. **[MEDIUM, OpenCode]** Fix benchmark parameters explicitly (doc count, RG count, paths/doc, selectivity, regex pattern shape) so results are reproducible across runs.
5. **[MEDIUM, OpenCode]** Add a narrow control tier (16 or 32 paths) alongside the wide tiers to catch small-index regressions.
6. **[MEDIUM, OpenCode]** Adopt sub-benchmark naming that encodes operator + width + spelling, e.g. `BenchmarkQueryEQ/paths=512/spelling=canonical`.
7. **[MEDIUM, Gemini]** `Encode → Decode → query-with-alternative-spelling` round-trip test in 06-01 Task 3.
8. **[LOW, Gemini]** Baseline capture against `v0.1.0` before Phase 06 lands, so success criterion #3's "measurable improvement" has a numeric anchor.
9. **[LOW, both]** Immutability comment on `GINIndex` / `pathLookup` and/or a code comment that `pathLookup` is derived, non-serialized, and must be rebuilt after deserialization.
10. **[LOW, OpenCode]** Test that transformer and FTS-path configs still bind correctly when user supplies equivalent supported spellings.

### Blocking vs. Non-Blocking

Using the stricter (OpenCode) rating:

- **Blocking before Wave 1 execution**: #1 (duplicate-path rule), #3 (query-boundary matrix test).
- **Blocking before Wave 2 execution**: #2 (attribution), #4 (fixed parameters).
- **Nice-to-have polish**: #5–#10.

---

*Generated by /gsd-review --phase 06 (gemini + codex + opencode over multiple invocations) at 2026-04-14T15:25:00Z. Codex failed on 2 attempts due to host concurrency; Gemini and OpenCode succeeded.*
