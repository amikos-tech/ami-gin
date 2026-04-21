---
phase: 13
reviewers: [gemini, codex]
reviewed_at: 2026-04-21T09:30:50Z
plans_reviewed: [13-01-PLAN.md, 13-02-PLAN.md, 13-03-PLAN.md]
---

# Cross-AI Plan Review — Phase 13: Parser Seam Extraction

## Gemini Review

# Phase 13: Parser Seam Extraction — Implementation Plan Review

This review covers implementation plans **13-01-PLAN.md**, **13-02-PLAN.md**, and **13-03-PLAN.md**. These plans detail the extraction of the JSON parsing logic from the `GINBuilder` into a pluggable `Parser` interface, preparing the codebase for SIMD acceleration in `v1.2` while maintaining strict `v1.0` parity.

---

### 1. Summary

The proposed plans provide a comprehensive, low-risk, and highly structured approach to a behavior-neutral refactor. By partitioning the work into an **Additive Wave** (13-01), a **Wiring/Cleanup Wave** (13-02), and a **Parity/Validation Wave** (13-03), the strategy ensures that the codebase remains stable and verifiable at every step. The plans demonstrate a deep understanding of the performance requirements and the subtle numeric fidelity constraints (BUILD-03) that are critical to the project's success. The parity harness in 13-03 is particularly impressive, providing four independent dimensions of verification that effectively "freeze" current behavior against any potential drift.

---

### 2. Strengths

- **Strategic Pitfall Defense:** The decision to keep the `int64` classifier logic within `builder.go` (Pitfall #1) is a critical foresight. It ensures that future SIMD parsers (which may only provide raw source text for numbers) still pass through a single, authoritative source of truth for numeric fidelity.
- **Performance Awareness:** The use of zero-value structs with value receivers for the `stdlibParser` and the "stashed state" pattern for `*documentBuildState` ensures that the indirection of the interface call introduces negligible overhead and zero new heap allocations on the `AddDocument` hot path.
- **Rigorous Verification:** The parity harness covers not just byte-level output, but also functional operator results (12-operator matrix) and structural determinism (gopter), making it nearly impossible for the refactor to introduce silent regressions.
- **Minimal API Surface:** Adhering to the "avoid gratuitous API churn" constraint by keeping the `parserSink` and `stdlibParser` package-private shows high engineering discipline.
- **Clear Documentation:** The inclusion of a README for goldens regeneration and the gated build-tag approach for the generator tool is excellent for long-term maintainability.

---

### 3. Concerns

- **Benchmark Baseline Continuity (MEDIUM):** Task 3 in Plan 13-02 relies on a baseline captured before the refactor. If the environment changes between Plan 01 and 02, or if the baseline wasn't captured at the absolute start, the measurements might be misleading. The fallback (regenerating from `v1.0` tag) is valid but adds friction.
- **`b.currentDocState` Lifetime (LOW):** While the builder is single-threaded, stashing the state on the builder between `BeginDocument` (called by parser) and the end of `Parse` (read by `AddDocument`) relies on the parser always calling `BeginDocument` first and the builder not being used concurrently. This is a safe assumption given the current architecture but should be clearly commented (as the plan specifies).
- **Dev-Tool Dependency (LOW):** Verification steps in Plan 13-03 Task 2 use `xxd`. While common on Darwin/Linux, it's an external dependency not explicitly listed in the project's `STACK.md`.

---

### 4. Suggestions

- **Automated Baseline:** In Plan 13-02 Task 3, consider providing a small script or one-liner that automatically checks out the `v1.0` tag, runs the benchmark, and returns to the branch to ensure the baseline is fresh and captured in the same environment as the post-refactor run.
- **DocID Type Verification:** In Plan 13-03 Task 1, ensure the `DocID(i)` cast matches the actual type in `docid.go`. If `DocID` is an alias for `uint64`, the cast is correct; if it's a struct/interface, it may need `MustParseDocID` or similar.
- **Linting Early:** Ensure `make lint` is run specifically on the new `parser_*.go` files in Plan 13-01 to catch import ordering issues (GCI) before the wiring in Plan 13-02.

---

### 5. Risk Assessment

**Overall Risk: LOW**

**Justification:**
The refactor is purely structural. The logic being moved is being copied verbatim from a stable `v1.0` state. The inclusion of a byte-level parity test against goldens generated from the `v1.0` tag provides an absolute safety net. The most complex part of the refactor—preserving numeric fidelity—is mitigated by not moving that specific code. The performance risk is addressed by explicit benchmark gates. This is a textbook example of a safe, large-scale refactor.

---

**Verdict: APPROVED.** Proceed with the implementation of Plan 13-01.

---

## Codex Review

### Overall
The phase split is sensible and the plans are clearly grounded in the current codebase, but I see three material blockers before execution: the proposed exported `Parser` API is not actually implementable outside `package gin`, several planned tests/fixtures use transformer option signatures that do not exist, and the parity harness currently proves self-determinism more than legacy parity.

### Plan 13-01: Parser interface + stdlibParser + sink adapters

**Summary**
This is the strongest of the three plans structurally: it isolates the seam before wiring it into `AddDocument`, keeps the numeric classifier in the builder, and minimizes churn. The main issue is that the public API shape and some test details do not line up with the actual code.

**Strengths**
- Good incremental sequencing: add the seam first, keep behavior unchanged, wire it later.
- Correctly preserves the numeric classifier in `builder.go:598`, which is the critical Phase 07 invariant.
- Thin sink adapters and compile-time assertions are a good fit for this codebase.

**Concerns**
- **HIGH:** Exporting `Parser` while keeping `parserSink` unexported makes the interface unusable to external consumers. A method signature that mentions an unexported type cannot be implemented outside the package. That conflicts with the stated goal that consumers can pass `WithParser(p)`.
- **HIGH:** Multiple planned tests/fixtures call `WithISODateTransformer` and `WithToLowerTransformer` with one argument, but the actual signatures require `path, alias string` in `gin.go:508` and `gin.go:520`. As written, those tasks will not compile.
- **MEDIUM:** Task ordering is inconsistent. `parser_sink.go` depends on `GINBuilder.currentDocState`, but that field is only added in Task 4, while earlier tasks try to run `go vet`/`go build`.
- **MEDIUM:** The "opaque handle" story is not fully true because `stdlibParser` still mutates `state.getOrCreatePath(...).present` directly instead of going through the sink.

**Suggestions**
- Either export a real `ParserSink`/public sink type, or keep `Parser`/`WithParser` internal for Phase 13 and revise the roadmap before implementation.
- Fix all transformer option usage to include aliases, e.g. `WithISODateTransformer("$.created_at", "epoch_ms")`.
- Reorder the plan so the `GINBuilder` fields land before any compile/verify step that references them.
- If the six-method sink must stay, explicitly document the container-presence exception; otherwise add a sink method for it.

**Risk Assessment**
**MEDIUM-HIGH**. The refactor shape is good, but the exported API contract and compileability issues need resolution first.

### Plan 13-02: Wire `AddDocument` through `b.parser.Parse`

**Summary**
This plan achieves the actual seam activation with minimal code movement, and the dead-code cleanup is well placed after the switch. The main risk is the `currentDocState` side channel: without explicit guarding, a buggy parser can corrupt builder state.

**Strengths**
- `NewBuilder` is the right place to default the parser and cache `parserName`.
- Deleting `parseAndStageDocument`, `stageStreamValue`, and `decodeTransformedValue` only after the switch is the right order.
- Keeping the existing numeric tests, especially `TestNumericIndexPreservesInt64Exactness`, is the right regression anchor.

**Concerns**
- **HIGH:** `b.currentDocState` needs defensive handling. `mergeDocumentState` dereferences `state` immediately in `builder.go:753`. If a parser forgets to call `BeginDocument`, `AddDocument` can reuse stale state from a previous document or panic on nil.
- **MEDIUM:** The planned malformed-JSON test likely expects the wrong error text. In the current parser flow, `{"foo": bar}` fails while parsing the object value, so the stable wrapper is more likely `parse object value at $.foo` from `builder.go:364`, not `read JSON token`.
- **MEDIUM:** The benchmark gate is operationally brittle: it depends on `/tmp` state, manual `git stash/checkout`, and loose regex matching that can capture extra benchmarks.
- **LOW:** The import-cleanup note is slightly off: `fmt` is still used by `stageMaterializedValue` in `builder.go:531`.

**Suggestions**
- In `AddDocument`, set `b.currentDocState = nil` before `Parse`, then fail if it is still nil or has the wrong `rgID` after a successful parse.
- Make the default-parser error test use a top-level malformed payload if you want to assert `read JSON token`; otherwise assert the actual current wrapper.
- Tighten the benchmark command to explicit names and record baselines in a committed artifact, not `/tmp`.
- Consider making the state flow explicit in code comments; it is the highest-risk part of the seam.

**Risk Assessment**
**MEDIUM** if state validation is added; **HIGH** if not. The wiring itself is straightforward, but the parser contract needs runtime enforcement.

### Plan 13-03: Parity harness + goldens

**Summary**
This plan has the right intent and good fixture coverage, but it is the weakest merge gate as currently written. The main problem is that it can skip when goldens are missing and its randomized section checks determinism of the new path, not parity with legacy behavior.

**Strengths**
- The 7 authored fixtures are well chosen, especially the int64 and transformer cases.
- A build-tagged regenerator is the right mechanism for golden refreshes.
- Adding both byte-level checks and query-level checks is a good defense-in-depth pattern.

**Concerns**
- **HIGH:** `TestParserParity_AuthoredFixtures` skips when goldens are absent. That weakens the gate: if the goldens disappear, CI can still pass.
- **HIGH:** The gopter section is not parity. It builds twice through the same new path, and `encodeDocs` returns `nil` on internal failure, so `bytes.Equal(nil, nil)` can hide real errors.
- **HIGH:** The documented "generate from `v1.0`" procedure is not reproducible as written, because the regeneration test and fixture helper do not exist on the `v1.0` tag.
- **MEDIUM:** Some matrix expectations conflict with current pruning semantics. `Evaluate` returns `AllRGs` when path resolution fails in `query.go:36`, so cases like `IsNotNull("$.nonexistent") => []` are wrong.
- **MEDIUM:** The matrix uses `DefaultConfig()`, so exact expectations depend on adaptive/trigram heuristics rather than a deliberately pinned mode.

**Suggestions**
- Make missing goldens a hard failure in normal test runs. Use a separate bootstrap command if needed, but do not leave a permanent skip path in the merge gate.
- Replace the gopter "build twice" property with either legacy-vs-seam comparison or seam-vs-golden comparison on generated corpora.
- Make `encodeDocs` fail the property on builder/AddDocument/Encode error instead of returning `nil`.
- Fix the README so regeneration is actually reproducible, or generate goldens before Plan 13-02 deletes the legacy path.
- Pin the evaluate-matrix config and avoid nonexistent-path cases unless the expected result is explicitly `AllRGs`.

**Risk Assessment**
**HIGH**. The authored fixtures are good, but the gate is not yet strong enough to prove "zero behavior change" with confidence.

### Bottom line
The overall design is solid, but I would not approve execution as-is. The main fixes I'd require first are:

- Resolve the `Parser`/`parserSink` public API contradiction.
- Correct the transformer option signatures in the planned tests and fixtures.
- Add `currentDocState` contract checks in `AddDocument`.
- Make goldens mandatory and make the randomized parity check compare against legacy behavior, not just the new path twice.

---

## Consensus Summary

The two reviewers diverged sharply on overall verdict — **Gemini: APPROVED / LOW risk**; **Codex: NOT APPROVED / MEDIUM-HIGH to HIGH risk per plan**. The divergence itself is signal: Gemini reviewed the plans as design artifacts, while Codex cross-referenced them against the current code and found concrete implementability gaps. The Codex concerns should be verified against source before dismissing either view.

### Agreed Strengths

- **Incremental sequencing** across 13-01 (additive) → 13-02 (wiring/cleanup) → 13-03 (parity gate) is the right shape for a zero-behavior-change refactor.
- **Keeping the numeric/`int64` classifier inside the builder** (rather than pushing it into parsers) preserves the Phase 07 numeric-fidelity invariant for any future SIMD parser.
- **Byte-level golden comparison + 12-operator Evaluate matrix + property-based determinism** is the right defense-in-depth pattern for a parity harness.
- **Regression anchors** — existing tests like `TestNumericIndexPreservesInt64Exactness` are correctly identified as the key guardrails to keep green through the wiring step.
- **Build-tagged regenerator** for goldens is a good long-term maintainability choice.

### Agreed Concerns

Only one concern was raised by **both** reviewers, differing only in severity:

- **`b.currentDocState` side-channel needs explicit lifecycle handling.**
  - Gemini: LOW — "safe assumption given the current architecture but should be clearly commented".
  - Codex: HIGH — needs runtime enforcement; set to `nil` before `Parse`, fail if still nil or wrong `rgID` after a successful parse.
  - **Action:** Treat this as a required change, not just a comment. Add contract validation in `AddDocument` around the `Parse` call. Codex's stronger stance is the safer default for a parity-gated refactor.

### Divergent Views — Worth Investigating

These were raised only by Codex and would benefit from a quick evidence check before planning iterates:

1. **`Parser` interface referencing unexported `parserSink` (HIGH per Codex).** Claim: an exported `Parser` whose method signature mentions an unexported `parserSink` type cannot be implemented outside the package, breaking the `WithParser(p)` consumer story. **Verify:** read 13-01-PLAN.md's interface signature and check whether `parserSink` is intended to be public. If Codex is right, either (a) export `ParserSink`, or (b) keep `Parser`/`WithParser` internal for Phase 13 and defer external pluggability.

2. **Transformer option signatures in planned tests/fixtures (HIGH per Codex).** Claim: `WithISODateTransformer` and `WithToLowerTransformer` require `(path, alias string)` per `gin.go:508,520`, but planned test code calls them with one argument — won't compile. **Verify:** grep the plans for these option names and compare to current signatures. If true, this is a fast fix across 13-01/13-03 before execution.

3. **`TestParserParity_AuthoredFixtures` skipping when goldens are absent (HIGH per Codex).** Claim: a permanent skip path in the merge gate means missing goldens can be silently green in CI. **Verify:** read the plan's test skeleton; change `t.Skip` to `t.Fatal` (with a separate bootstrap command for first-time generation).

4. **Gopter property tests self-determinism vs. legacy parity (HIGH per Codex).** Claim: "building twice through the same new path" proves determinism, not parity; combined with `encodeDocs` returning `nil` on failure, `bytes.Equal(nil, nil)` can silently pass. **Verify:** read the property test in 13-03-PLAN.md. If accurate, the property needs to compare seam-vs-golden (or seam-vs-legacy while legacy still exists) and `encodeDocs` must fail the property on error.

5. **"Generate goldens from `v1.0` tag" reproducibility (HIGH per Codex).** Claim: the regeneration test and fixture helper do not exist on the `v1.0` tag, so the documented procedure fails. **Verify:** git-log the helper files. If correct, either generate goldens *before* 13-02 deletes the legacy path (on the refactor branch pre-deletion), or build a standalone generator that does not depend on the new helpers.

6. **Evaluate-matrix edge cases (MEDIUM per Codex).** Claim: `IsNotNull("$.nonexistent") => []` conflicts with `query.go:36`, which returns `AllRGs` on path-resolution failure. **Verify:** trace the expected-result table in 13-03 against `query.go` behavior. Nonexistent paths should expect `AllRGs`, or should be removed from the matrix.

7. **Benchmark baseline ergonomics (MEDIUM per both, different framing).** Gemini suggested a one-liner script; Codex flagged `/tmp` + manual `git stash/checkout` + loose regex matching as brittle. **Action:** commit baselines to an artifact file and pin benchmark names explicitly.

### Recommended Next Action

Given the divergence and the concrete line-number claims from Codex, iterate the plans with:

```
/gsd-plan-phase 13 --reviews
```

Focus the iteration on the seven verification items above. The reviewers agree on the refactor *shape*; disagreement is entirely on whether the current *plan text* is executable and whether the parity gate is actually a gate. Both are resolvable with targeted edits, not rearchitecture.
