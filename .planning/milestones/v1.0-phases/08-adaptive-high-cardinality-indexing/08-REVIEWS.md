---
phase: 8
reviewers: [gemini, claude]
reviewed_at: 2026-04-15T18:20:39Z
plans_reviewed:
  - 08-01-PLAN.md
  - 08-02-PLAN.md
  - 08-03-PLAN.md
---

# Cross-AI Plan Review — Phase 08

## Gemini Review

# Phase 08: Adaptive High-Cardinality Indexing — Plan Review

The proposed plans provide a robust and systematic approach to implementing frequency-driven adaptive indexing. By introducing a "middle ground" between full exact indexing and lossy bloom-only fallback, the design recovers significant pruning power for hot values in high-cardinality datasets while maintaining strict correctness for the long tail.

### Summary
The plan set is comprehensive, covering the core algorithmic logic, serialized persistence, operational visibility, and empirical validation. It correctly identifies the technical risks—specifically the risk of false negatives in negative predicates—and mitigates them by enforcing conservatism on non-promoted terms. The separation of concerns across three waves (Core, Persistence/Visibility, and Benchmarks) is logical and ensures that functional correctness is established before surfacing the feature to users or measuring its performance.

### Strengths
- **Algorithmic Soundness**: Ranking promotion candidates by row-group coverage (`RGSet.Count()`) rather than raw document frequency is the correct choice for a pruning index, as it directly correlates with selectivity.
- **Correctness First**: The plan explicitly forbids inverting bucket-based positive results for `NE`/`NIN` queries, preventing a major class of potential false-negative bugs.
- **Efficient Reuse**: The design leverages existing builder data (`pd.stringTerms`, HLL, and Bloom) to make promotion decisions in a single pass without adding ingest-time overhead.
- **Operational Visibility**: Adding specific mode indicators (`mode=adaptive-hybrid`) and counters to `gin-index info` ensures that users can verify the index's internal layout without needing specialized diagnostic tools.
- **Robust Validation**: Updating the `CardinalityThreshold` property test to handle three modes instead of two ensures that the fundamental contract of the library (no false negatives) is maintained across the structural shift.

### Concerns
- **Binary Versioning (LOW)**: The plan mentions a version bump "if the binary layout changes." Given that a new serialized section for adaptive indexes is being added, a version bump to `5` is almost certainly required. The plan should commit to this bump to ensure `Decode` remains defensive against older index files.
- **Bucket Count Scalability (LOW)**: While fixed hash-buckets are compact, very small bucket counts on extremely high-cardinality paths could lead to "hot buckets" that effectively disable pruning for the long tail. However, since this is a fallback for data that previously had *no* pruning (bloom-only), any improvement is a net win.
- **CLI Test Coverage (MEDIUM)**: The plan notes that `cmd/gin-index` currently has no tests. Adding `main_test.go` and asserting against stdout is essential to lock in the mode-reporting contract, especially for automated tooling that might parse CLI output.

### Suggestions
- **Promotion Ceiling**: Ensure the "coverage ceiling" (Decision D-01) is strictly enforced to prevent promoting terms like "empty string" or "null-equivalent" that appear in almost every row group, as storing exact bitmaps for these provides zero pruning benefit.
- **Benchmark Skew**: For the skewed fixtures in Wave 3, consider using a Zipfian distribution. It is the most realistic model for "power-law" distributions typically found in log IDs, user IDs, and tag cardinalities.
- **Config Defaults**: Provisionally set the `promoted-term cap` to a moderate value (e.g., 32 or 64) and the `bucket count` to a power of two (e.g., 128 or 256) to ensure a good balance between size and pruning recovery out of the box.

### Risk Assessment: LOW
The overall risk is low. The architecture is additive and preserves the established "no-false-negative" property of the library. The most complex logic (promotion and bucket hashing) is localized to the `Finalize` and `Query` dispatch paths, and the plan includes sufficient property-based and unit testing to catch regressions early.

- **Missing Edge Cases**: Handled via property tests and conservative negative-operator logic.
- **Dependency Ordering**: Correct (Core -> Persistence -> Benchmarks).
- **Security**: Handled via bounds checks on new serialized sections and max-count constants.
- **Performance**: Validated via explicit metrics in Wave 3.

---

## Claude Review

# Phase 08 Plan Review: Adaptive High-Cardinality Indexing

## Plan 08-01: Core Adaptive-Hybrid Structures, Finalize, and Query Semantics

### Summary

This is the heaviest plan of the phase — it introduces the entire adaptive-hybrid concept across three core files (`gin.go`, `builder.go`, `query.go`) and their test coverage in a single plan with three TDD tasks. The plan is well-grounded in the existing codebase: it correctly identifies `pathBuildData.stringTerms` as the promotion source, the `Finalize()` cardinality branch as the insertion point, and the bloom→length→mode chain in `evaluateEQ()` as the query hook. The task decomposition (structures+finalize → query routing → test coverage) is logical and dependency-ordered.

### Strengths

- **Correct identification of existing primitives.** The plan reuses `RGSet.Count()` for promotion ranking, `xxhash` for bucket hashing, and preserves `StringLengthIndex` as a secondary filter — no unnecessary new dependencies.
- **Three-mode model is clean.** Explicit `exact` / `adaptive-hybrid` / `bloom-only` modes with a new flag (`FlagAdaptiveHybrid`) avoids the ambiguity of overloading `FlagBloomOnly`.
- **Conservative negative semantics.** The explicit requirement that `NE`/`NIN` stay conservative for non-promoted terms is the right call and directly prevents the most dangerous correctness bug (inverting lossy bucket results).
- **Task 1 acceptance criteria are precise.** Checking for the deterministic bucket helper, promotion cap, and coverage ceiling enforcement gives the executor clear success gates.
- **Threat model is focused.** T-08-01 through T-08-03 are the real risks, and the mitigations are concrete.

### Concerns

- **HIGH: Task 2 creates tests that Task 3 also creates.** Task 2's acceptance criteria require `TestAdaptivePromotesHotTermsToExactBitmaps`, `TestAdaptiveFallbackHasNoFalseNegatives`, and `TestAdaptiveNegativePredicatesStayConservative` to exist in `gin_test.go`. Task 3's acceptance criteria require the same test names plus property test updates. This creates a sequencing ambiguity: if the executor writes these tests in Task 2 as required, Task 3 becomes partially redundant or must "enhance" them without clear scope. The plan should either (a) put all three test names in Task 2 and make Task 3 purely about property test updates, or (b) defer the test names to Task 3 and have Task 2's TDD use differently-named intermediate tests.

- **MEDIUM: No explicit default values for adaptive config knobs.** The plan says "add additive global config knobs on `GINConfig` for the hybrid policy" but leaves default values entirely to the executor. The research doc notes this as an open question, but Plan 01 needs at least provisional defaults to be testable. The TDD cycle in Task 1 needs concrete values to assert against. Suggestion: specify placeholder defaults (e.g., `AdaptiveHotTermCap: 100`, `AdaptiveCoverageCeiling: 0.8`, `AdaptiveBucketCount: 64`, `AdaptiveMinRGCoverage: 2`) and note they can be tuned based on Plan 03 benchmarks.

- **MEDIUM: Bucket count strategy is unspecified.** The plan says "a fixed bucket count" but doesn't indicate whether this is a global constant, a config knob, or derived from cardinality. Since the bucket count directly determines fallback selectivity and index size, this needs at least a guiding principle (e.g., "fixed global default like 64, configurable via `GINConfig`").

- **MEDIUM: `evaluateNE()` and `evaluateNIN()` rework needs more detail.** The current implementations delegate to `evaluateEQ()`/`evaluateIN()` and invert. The plan says "return the conservative present-row-group bitmap instead of a lossy inversion" for non-promoted terms but doesn't specify where the exact-vs-bucket detection happens. Since `evaluateEQ()` currently returns an `*RGSet` with no signal about whether it came from an exact lookup or a bucket fallback, the executor will need to either: (a) add a return flag/wrapper indicating resolution type, or (b) duplicate the promoted-term check in `evaluateNE()`. The plan should acknowledge this design choice explicitly.

- **LOW: TDD for Task 1 may be awkward.** Task 1 is marked `tdd="true"` and modifies `gin.go` and `builder.go`, but the test targets (`TestAdaptivePromotesHotTermsToExactBitmaps`, `TestPropertyIntegrationCardinalityThreshold`) are in `gin_test.go` and `integration_property_test.go` — files that belong to Tasks 2/3. This means Task 1's RED phase would write tests in files listed under later tasks. Not a blocker, but the file assignments across tasks are slightly inconsistent.

### Suggestions

- **Split test ownership clearly.** Move `TestAdaptivePromotesHotTermsToExactBitmaps` to Task 1 (as the TDD test for finalize behavior), keep `TestAdaptiveFallbackHasNoFalseNegatives` and `TestAdaptiveNegativePredicatesStayConservative` in Task 2 (as TDD for query routing), and reserve Task 3 purely for property-test updates and skewed-fixture regressions.
- **Specify provisional config defaults and the bucket-count strategy** so the executor's TDD assertions have concrete targets.
- **Add a note about `evaluateNE`/`evaluateNIN` detection strategy** — specifically whether adaptive EQ will return a tagged result or whether the caller re-checks for promoted status.

### Risk Assessment: **MEDIUM**

The core design is sound and well-anchored in existing code. The main risk is ambiguous test ownership between Tasks 2 and 3, which could lead to redundant work or missing coverage depending on how the executor interprets the overlap. The unspecified defaults and NE/NIN detection strategy add moderate implementation ambiguity but are solvable.

---

## Plan 08-02: Serialization, CLI Visibility, and README

### Summary

This plan handles the "make it durable and visible" concerns: persisting adaptive config and per-path metadata through encode/decode, updating the CLI `info` command, and updating public documentation. It correctly depends on Plan 01 and targets the right files. The three tasks (serialization → CLI → README) are well-ordered and independent enough to execute cleanly.

### Strengths

- **Explicit version bump requirement.** The acceptance criterion "If the binary layout changes, the plan leaves an exact version bump" is exactly right. The current `Version = uint16(4)` must increment if the wire format gains a new section, and the plan calls this out explicitly.
- **CLI output contract is testable.** Requiring exact strings `mode=exact`, `mode=bloom-only`, `mode=adaptive-hybrid` in output makes the acceptance criteria unambiguous and grep-verifiable.
- **Adding `cmd/gin-index/main_test.go`.** The CLI currently has zero tests. This plan creates the file and adds focused tests — good hygiene.
- **README task is appropriately scoped.** It updates the three sections that reference high-cardinality behavior without turning into a doc rewrite.

### Concerns

- **HIGH: Wire format section ordering is fragile.** The current `Encode()`/`Decode()` chain writes sections in a fixed order without section headers or length-prefixed framing. Adding a new `writeAdaptiveIndexes()`/`readAdaptiveIndexes()` section means the section must go in a specific position in the chain, and the position must be consistent between encoder and decoder. The plan doesn't specify where in the write/read sequence the adaptive section goes. If the executor places it after string indexes but before numeric indexes, old decoders will fail on the version-bumped format. The plan should specify either: (a) exact position in the section chain, or (b) that the version bump is sufficient to reject old decoders and the position can be anywhere consistent.

- **MEDIUM: `SerializedConfig` adaptive fields may not be enough.** The plan says to extend `SerializedConfig` with adaptive knobs. But `SerializedConfig` is a JSON blob written/read as a single chunk at the end of the binary. The adaptive **per-path** data (promoted terms, bucket bitmaps) needs its own binary section — the plan correctly notes "explicit adaptive section helpers rather than smuggling adaptive state through existing string-index code paths." However, the distinction between config-level fields (knobs like `AdaptiveHotTermCap`) and per-path index data (the actual promoted terms and bucket bitmaps) could be clearer to prevent the executor from trying to JSON-serialize per-path bitmap data.

- **MEDIUM: `TestPathInfoReportsAdaptiveMode` and `TestCLIInfoShowsAdaptiveSummary` need an index fixture.** These tests need to build an index with adaptive paths, encode it, write it to a temp file, and then invoke `infoSingleFile()`. The current `cmd/gin-index/main.go` reads from files or S3, so the test needs to either (a) write a temp `.gin` file and invoke the CLI function, or (b) refactor `infoSingleFile()` to accept an `*GINIndex` directly. The plan doesn't address this test plumbing, and the executor may waste time figuring out the cleanest approach.

- **LOW: README task acceptance criteria use `rg` (ripgrep).** The verify command is `rg -n 'adaptive-hybrid|promoted|bucket|coverage ceiling|high-cardinality' README.md`. This works but is a weak verify — it checks for keyword presence, not semantic correctness. Acceptable for a docs task, but worth noting.

### Suggestions

- **Specify adaptive section position** in the encode/decode chain — logically after `writeStringIndexes` and before `writeStringLengthIndexes`, or after all existing sections but before `writeConfig`. Either works, but it needs to be explicit.
- **Clarify the config vs. per-path data split:** `SerializedConfig` gets the global knobs; the new adaptive binary section gets the per-path promoted terms and bucket bitmaps. State this explicitly.
- **Add a note about CLI test plumbing** — suggest either a temp-file approach or refactoring `infoSingleFile` to accept a pre-loaded `*GINIndex` to keep tests fast and filesystem-independent.

### Risk Assessment: **MEDIUM**

The wire format change is the highest-risk element. If the section ordering or version bump isn't handled precisely, decode compatibility breaks silently or noisily. The plan correctly identifies the need for explicit format evolution but could be more prescriptive about the mechanical details.

---

## Plan 08-03: Benchmarks and Fixtures

### Summary

A focused plan that creates the evidence base for HCARD-05. Two tasks: (1) create skewed fixtures that expose hot-value recovery, and (2) report pruning and size metrics. This is the cleanest plan of the three — well-scoped, no cross-cutting concerns, clear acceptance criteria.

### Strengths

- **Skewed fixtures address the real gap.** The existing `generateHighCardinalityDocs()` uses `i % cardinality` which is uniform — it cannot prove hot-value recovery. The plan explicitly requires "hot head + long tail" fixtures.
- **`b.ReportMetric` for `candidate_rgs` and `encoded_bytes`.** This is the right approach — Go benchmark infrastructure supports custom metrics, and these two numbers are the actual proof points for HCARD-05.
- **Same fixture across all three modes.** This is critical for apples-to-apples comparison and the plan mandates it.
- **Phase 07 naming convention alignment.** Using `mode=` and `shape=` segments keeps benchmark history comparable.

### Concerns

- **MEDIUM: No explicit fixture parameters.** The plan says "hot head and long tail" but doesn't specify concrete numbers: how many total unique values, what fraction is "hot," how many row groups, how many docs per RG. Without these, two different executors might produce very different fixtures. Suggestion: specify a concrete fixture like "20,000 unique values, top 50 appear in 80% of 100 row groups, remaining 19,950 appear in 1-2 row groups each."

- **MEDIUM: "Mode=exact" benchmark may be misleading.** If the fixture has 20,000 unique values and the default `CardinalityThreshold` is 10,000, then "mode=exact" requires lowering the threshold or raising it above 20,000. The plan says "changing only builder config knobs" but doesn't specify *which* knobs to change. For exact mode, you'd need `CardinalityThreshold > cardinality` (store all terms). For bloom-only, you'd need adaptive disabled. For adaptive, you'd use the new defaults. This config matrix should be explicit.

- **LOW: No assertion that adaptive actually beats bloom-only.** The benchmarks report metrics but don't fail if adaptive pruning isn't better. This is arguably correct (benchmarks shouldn't be assertions), but the plan could add a test (not a benchmark) that asserts `adaptiveResult.Count() < bloomOnlyResult.Count()` for a hot-value probe to lock the pruning improvement as a regression.

- **LOW: Wave 2 depends on Plan 01.** The benchmark needs the adaptive builder/query code to exist. If Plan 01 has issues, Plan 03 blocks. This is already captured in `depends_on: [08-01]` but worth noting that Plan 03 cannot be parallelized with Plan 01.

### Suggestions

- **Specify concrete fixture parameters** (unique count, hot fraction, RG count, docs per RG) so the benchmark is reproducible and comparable.
- **Specify the config knob matrix** for the three compared modes.
- **Consider adding one non-benchmark regression** that asserts `adaptiveHotResult.Count() < bloomOnlyHotResult.Count()` to lock the pruning improvement claim.

### Risk Assessment: **LOW**

This is the lowest-risk plan. It only adds test files, doesn't change production code, and the worst case is that the benchmarks are less informative than ideal. The concerns are about specificity, not correctness.

---

## Cross-Plan Assessment

### Requirement Coverage

| Requirement | Plan 01 | Plan 02 | Plan 03 | Coverage |
|-------------|---------|---------|---------|----------|
| HCARD-01 | ✅ (core structures + finalize) | — | — | Complete |
| HCARD-02 | ✅ (config knobs + promotion) | ✅ (config serialization) | — | Complete |
| HCARD-03 | ✅ (bucket fallback + NE/NIN) | — | — | Complete |
| HCARD-04 | — | ✅ (CLI + metadata roundtrip) | — | Complete |
| HCARD-05 | — | — | ✅ (benchmarks) | Complete |

All five requirements are covered. No orphans.

### Dependency Ordering

```
Plan 01 (Wave 1) ──┬──> Plan 02 (Wave 2)
                    └──> Plan 03 (Wave 2)
```

Plans 02 and 03 are independent of each other and can theoretically execute in parallel. This is correct.

### Missing Concerns

1. **`rebuildPathLookup()` and adaptive paths.** After decode, `rebuildPathLookup()` validates path references for all index maps (`StringIndexes`, `NumericIndexes`, etc.). The new `AdaptiveStringIndexes` map must be included in `validatePathReferences()`. Neither Plan 01 nor Plan 02 explicitly mentions this, though Plan 02's serialization task would naturally handle it. Worth calling out.

2. **`GINIndex` initialization.** `NewGINIndex()` initializes all maps. The new `AdaptiveStringIndexes map[uint16]*AdaptiveStringIndex` must be initialized there. Plan 01 would handle this, but it's not in the acceptance criteria.

3. **Bloom filter interaction with adaptive paths.** During build, `addStringTerm()` inserts `path + "=" + term` into the global bloom for *all* terms, including those that will become non-promoted on adaptive paths. This is correct (bloom is a prefilter), but the plan should confirm that bloom insertions continue unchanged for adaptive paths. The plan doesn't explicitly mention this, but the current code already does it before finalize mode selection, so it should be fine.

### Overall Phase Risk: **MEDIUM**

The plans are well-structured and well-grounded in the codebase. The primary risks are:
- Test ownership ambiguity between Plan 01 Tasks 2/3 (fixable with clearer task boundaries)
- Wire format section positioning in Plan 02 (needs one explicit sentence)
- Unspecified provisional defaults requiring executor judgment (acceptable but adds variance)

None of these are blockers. An experienced executor should be able to resolve all of them within the spirit of the plan and the locked CONTEXT.md decisions.

---

## Consensus Summary

Both reviewers agree the phase is directionally strong: the three-plan split is coherent, the adaptive-hybrid design fits the existing codebase, and the plan consistently protects the no-false-negative contract by keeping negative predicates conservative for non-promoted terms.

### Agreed Strengths

- Promotion based on row-group coverage is the right pruning metric and correctly reuses existing `RGSet` and builder data.
- The plan preserves correctness by avoiding lossy `NE` / `NIN` inversion for bucket-backed fallback paths.
- The phase decomposition is sensible: core adaptive behavior first, then persistence and visibility, then benchmark proof.
- Visibility requirements are strong: both reviewers liked explicit mode reporting such as `mode=adaptive-hybrid` and compact summary counters.

### Agreed Concerns

- Serialization details need to be more explicit. Both reviewers called out the adaptive wire-format change as a place where the plan should commit to a concrete version bump and make encode/decode handling unambiguous.
- Adaptive defaults are underspecified. Both reviewers wanted provisional values or clearer guidance for promoted-term caps, bucket count, and related hybrid-policy knobs so implementation and TDD do not rely on ad hoc executor choices.
- CLI verification should be locked down. Both reviewers highlighted the lack of existing `cmd/gin-index` tests and wanted the mode-reporting contract made explicit through targeted test coverage.

### Divergent Views

- Claude is materially more skeptical about plan mechanics. It calls out overlapping test ownership between Plan 08-01 Tasks 2 and 3, missing detail around `evaluateNE()` / `evaluateNIN()` detection flow, and a few integration hooks such as `rebuildPathLookup()` and `NewGINIndex()` initialization.
- Gemini is more comfortable with the current structure and rates the overall phase risk as low, focusing mainly on versioning clarity, bucket-count sizing, and practical tuning suggestions like coverage-ceiling enforcement and Zipf-like benchmark skew.

### Recommended Follow-Ups Before Execution

- Make the format evolution explicit in Plan 08-02: commit to the version bump and state where the adaptive section sits in the encode/decode contract.
- Add provisional adaptive knob defaults and describe the bucket-count strategy in Plan 08-01.
- Clarify test ownership between Plan 08-01 Tasks 2 and 3 so the executor does not duplicate or fragment the adaptive regression suite.
- Keep the CLI plan explicit about how `cmd/gin-index/main_test.go` will exercise `info` output for adaptive paths.
