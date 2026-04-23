---
phase: 16
reviewers: [gemini, claude]
reviewed_at: 2026-04-23T09:08:13Z
plans_reviewed: [16-01-PLAN.md, 16-02-PLAN.md, 16-03-PLAN.md, 16-04-PLAN.md]
---

# Cross-AI Plan Review — Phase 16

## Gemini Review

This review evaluates the implementation plans for **Phase 16: AddDocument Atomicity (Lucene contract)**. The plans provide a robust and well-grounded approach to bringing the GIN Index builder in line with industry-standard atomicity guarantees, specifically targeting the "inconsistent index after user-error" problem by adopting a strict validate-before-mutate strategy.

### Summary
The plans effectively decompose the complex task of ensuring ingest atomicity into four logical increments: signature refactoring, failure-mode isolation (tragedy vs. per-doc), comprehensive property-based verification, and static enforcement. By leveraging the existing shadow-field simulation pattern for numeric promotion and introducing a testable recovery boundary around the merge logic, the design achieves the Lucene per-document contract without requiring expensive snapshot/restore overhead. The inclusion of a determinism sanity check as a prerequisite for byte-identical atomicity testing demonstrates high engineering maturity and awareness of potential flakes in binary serialization tests.

### Strengths
*   **Validator-Merge Separation:** The use of `// MUST_BE_CHECKED_BY_VALIDATOR` markers combined with signature changes and a CI grep (Plan 16-04) is an excellent, low-overhead way to enforce the architectural invariant that merge must be infallible by construction.
*   **Safe Recovery Pattern:** Plan 16-02's `runMergeWithRecover` helper correctly narrows the scope of `recover()` to the merge phase only, preventing it from masking parser or staging bugs that should remain observable as standard errors.
*   **Security-Conscious Logging:** The decision to omit `panic_value` and raw document content from tragic recovery logs (Plan 16-02) is a proactive mitigation against accidental PII leakage in observability streams.
*   **Rigorous Verification:** Plan 16-03's approach to atomicity testing—using a generated corpus with non-contiguous DocIDs to compare against a "clean" oracle build—is the gold standard for verifying that no partial mutation occurred.

### Concerns
*   **Wording Consistency (LOW):** Plan 16-01 and 16-02 rely on exact string matching for some error assertions (e.g., `"unsupported mixed numeric promotion at $.score"`). While appropriate for this phase to minimize diff churn, any future localization or structured error work (Phase 18) will require updating these brittle assertions.
*   **CI Marker Check Portability (LOW):** Plan 16-04 uses `awk` for the marker check. While `awk` is generally POSIX-standard, subtle differences between BSD and GNU `awk` could lead to CI failures if the script relies on non-standard extensions.
*   **Performance Overhead (LOW):** Re-running numeric simulation in the validator adds a small CPU cost per document. Given the project's "correctness-first" priority and the fact that GIN is designed for row-group pruning (not high-frequency OLTP), this is acceptable but should be monitored in future benchmarks.

### Suggestions
*   **Marker Robustness:** In Plan 16-04, ensure the `awk` script handles varied whitespace or comment styles (e.g., `//MUST_BE_CHECKED...` without the space) to avoid false negatives.
*   **Tragic Error Wrapping:** In Plan 16-02 Task 2, when assigning `b.tragicErr = err`, consider ensuring that if `err` is already a "tragic" typed error from a nested caller, it isn't double-wrapped in a way that makes `errors.As` or `errors.Is` difficult later.
*   **Test Documentation:** Add a comment in `atomicity_test.go` explaining *why* the non-contiguous DocID IDs are used (to verify `DocIDMapping` integrity), as this is a subtle but important part of the atomicity proof.

### Risk Assessment: LOW
The risk is low because the plans strictly adhere to the "Strategy C" (validate-before-mutate) direction already established in the project context. The changes are largely internal to `builder.go`, preserving the public API surface. The most "dangerous" change—the introduction of `recover()`—is mitigated by limiting its scope to the narrowest possible callback and providing a dedicated test suite for the recovery path itself. The dependency order is logical, ensuring that the infrastructure (signatures and markers) is in place before the verification logic is applied.


---

## Claude Review

# Phase 16 Plan Review — AddDocument Atomicity (Lucene Contract)

## Overall Summary

The four-plan decomposition is well-grounded in the research audit and faithfully implements the locked CONTEXT decisions. The sequencing (16-01 → {16-02, 16-04} → 16-03) correctly places the structural contract first, recovery second, static enforcement in parallel, and property verification last. Scope discipline is strong: no Phase 17/18 surfaces leak in, and the merge-hoisting catalog correctly narrows to the one real user-reachable merge failure (numeric promotion). The main risks are (a) a likely **property-test runtime blow-up** in 16-03, (b) a few **integration-test gaps** where field-level manipulation stands in for end-to-end behavior, and (c) a minor **marker-check completeness gap** in 16-04.

**Overall risk: MEDIUM** — plans will deliver ATOMIC-01/02/03, but 16-03 as written may not complete within CI time budgets.

---

## Plan 16-01: Validator-Complete Numeric Promotion + No-Error Merge Signatures

### Strengths
- Targets exactly the one merge-hoisting failure mode identified by the research audit (`unsupported mixed numeric promotion at %s`).
- Preserves the shadow-field simulation pattern on `stagedPathData` rather than introducing a parallel preview type (D-01 honored).
- Converts missed-validator branches to `panic(...)` rather than silently dropping them, which keeps Plan 16-02's recovery net meaningful.
- Error string `unsupported mixed numeric promotion at $.score` is locked verbatim, minimizing Phase 18 migration churn (D-09 honored).

### Concerns

- **[LOW]** The validator may already cover both promotion directions today. Per `builder.go:632-647`, `stageNumericObservation` handles both `NumericValueTypeIntOnly` and `NumericValueTypeFloatMixed` branches with `!canRepresentIntAsExactFloat` checks, and seeds from real `b.pathData` via `seedNumericSimulation`. If so, Task 2's "hoisting work" collapses to a pure signature change plus marker addition — which is fine, but the plan's conditional wording ("If either focused test from Task 1 fails, extend `validateStagedPaths`...") leaves ambiguity. **Suggestion:** Verify before execution whether hoisting is needed or only the signature change is, and tighten the plan to reflect that.

- **[LOW]** The promotion path through `promoteNumericPathToFloat` wraps errors as `"promote numeric path %s: ..."` today (`builder.go:818-821`). After hoisting, the error comes from validator as `"unsupported mixed numeric promotion at %s"` — **this is a user-visible wording change** from one specific failure sub-path. Not breaking, but worth flagging in the summary so test authors know. The plan's acceptance criteria lock the new wording, which handles it.

- **[MEDIUM]** `stageJSONNumberLiteral` is called directly on a `documentBuildState` in the focused tests. This is an internal function; calling it from tests is fine for `package gin` tests but couples the test to an internal API surface. **Suggestion:** Consider whether an `AddDocument`-level test with a seeded doc and a rejected second doc would be more robust to internal refactors while still proving "rejected before merge".

### Risk: **LOW**

---

## Plan 16-02: Tragic Rename + Merge Recovery

### Strengths
- Separates the mechanical rename (Task 1) from the recovery helper (Task 2) — keeps diffs reviewable.
- Locks `runMergeWithRecover` scope to `mergeStagedPaths` only (D-04/D-06 honored; not `mergeDocumentState`, not `AddDocument`).
- Logger emission is silent-by-default and goes through the Phase 14 seam; explicitly forbids `panic_value` attr to prevent raw-document leakage.
- Tests both direct helper behavior and builder-level refusal.

### Concerns

- **[MEDIUM]** `TestAddDocumentRefusesAfterRecoveredMergePanic` sets `builder.tragicErr` by **calling `runMergeWithRecover` directly and assigning the result** — this proves the helper + gate, but it does NOT prove that `mergeDocumentState` actually assigns `b.tragicErr` in the real flow after the helper returns. The code path `mergeDocumentState → runMergeWithRecover → mergeStagedPaths panics → b.tragicErr = err → return err → AddDocument returns → next AddDocument refused` has no end-to-end test. CONTEXT D-06 explicitly requested an "end-to-end refusal test (new): go through a real merge path that panics (e.g., using the helper); assert subsequent `AddDocument` refused." **Suggestion:** Add a test that forces a real merge-path panic (e.g., via a test parser or an injected hook inside mergeStagedPaths, or by monkeypatching a `MustNewRGSet` to panic in a controlled test configuration).

- **[MEDIUM]** `runMergeWithRecover(nil, ...)` must be nil-safe. The test `TestRunMergeWithRecoverConvertsPanicToTragicError` passes `nil` as logger. The implementation uses `logging.Error(logger, ...)` — need to verify `logging.Default(nil)` returns a noop logger (Phase 14 contract). The test would fail confusingly if it panics in the logging call. **Suggestion:** Add explicit nil-logger assertion or switch test to use `logging.Noop` sentinel.

- **[LOW]** Using raw `logging.Attr{Key: "panic_type", Value: ...}` literal rather than a typed helper function means `panic_type` is not part of any attribute vocabulary. At Error level this is fine (the frozen allowlist is INFO-only per Phase 14 D-03), but it sets a precedent for ad-hoc Error-level attrs. **Suggestion:** Document in 16-02-SUMMARY.md that Error-level attrs are not gated by the INFO allowlist so future readers don't think this is a policy bypass.

- **[LOW]** The import path `github.com/amikos-tech/ami-gin/logging` should be verified against the actual `logging` package location. The stack module path is `github.com/amikos-tech/ami-gin` and the patterns show `"github.com/amikos-tech/ami-gin/logging"` — looks correct.

### Risk: **MEDIUM** (primarily due to the missing end-to-end recovery test)

---

## Plan 16-03: Atomicity Property + Public Failure Catalog

### Strengths
- Three-layered structure (determinism → catalog → property) correctly orders prerequisites so nondeterminism surfaces as its own failure (D-05 honored).
- Non-contiguous `docID` assignment (`DocID(i*3+1)`) is a sharp test — proves `DocIDMapping` is preserved, not just that encoded content is equivalent.
- Full public failure catalog is enumerated with explicit subtest names, matching CONTEXT D-03.
- `bytes.Equal` oracle directly matches ATOMIC-01 wording; no query-equivalence weakening.

### Concerns

- **[HIGH]** **Runtime budget.** `propertyTestParameters()` runs **1000 successful tests** in normal mode. Each test builds **two** indexes of ~1000 documents each. That's **2 million `AddDocument` calls** per property invocation, plus two `Encode` calls per iteration. Even at 10µs per doc, this is 20+ seconds just for doc ingest, and encode is not free either. The full test suite uses a 30m timeout, but this single property could easily consume a large fraction of it — or hit shrinker explosions on failure. **Suggestion:** Explicitly scope the property via `propertyTestParametersWithBudgets(50, 10)` (50 iterations normal, 10 short) since the corpus itself already provides statistical coverage within each iteration. The research's "1000 successful tests" applies to the overall budget, not to nested doc-count-per-iteration. ATOMIC-01 requires "≥1000 documents per corpus", not ≥1000 corpora.

- **[MEDIUM]** **Failure-ratio enforcement in generators.** `failingCount*10 >= len(corpus.all)` must be a hard invariant of `genAtomicityCorpus`, not a probabilistic outcome. If gopter's shrinker ever reduces the corpus while preserving the distribution, `failingCount` could drop below 10%. **Suggestion:** Construct the corpus deterministically — e.g., always place a failing doc at every 10th position — rather than relying on a generator probability. The plan hints at this but doesn't lock the mechanism.

- **[MEDIUM]** **Clean subset `numRGs`.** Both builds use `corpus.numRGs`, which equals the full attempted capacity. The clean build will have `Header.NumRowGroups == corpus.numRGs` with some RGs having zero documents. Verify that `Finalize` + `Encode` produces identical bytes when some RGs are unused in both builds (they must be, since the full build also only advances `nextPos` on success). This should work but deserves a specific test assertion or comment explaining why it works. **Suggestion:** Add an assertion that `Header.NumRowGroups` matches between both encodings as a sanity check before `bytes.Equal`.

- **[LOW]** The catalog has 12 enumerated subtests covering parser / stage / numeric / pre-parser-gate / overflow cases. This is thorough, but several rely on a **package-private test parser** that must be defined in `atomicity_test.go`. The plan doesn't specify the test parser's structure — it just says "a package-private test parser that calls `sink.StageScalar(...)`". **Suggestion:** Either (a) define the test-parser skeleton in the plan, or (b) reference `parser_test.go:337-439` as the pattern to mirror (it already has `skipBeginDocumentParser`, `doubleBeginDocumentParser`, etc.).

- **[LOW]** `genNumericPromotionFailingDoc` returns "an ordered pair or chunk" — pair semantics need careful handling when interleaved into the larger corpus, because the "clean seed" must be a clean doc and the "failing doc" must be a failing doc in the same corpus. This is subtle. **Suggestion:** Consider implementing numeric-promotion failures by **pre-seeding** one clean doc at corpus construction and then injecting failing-doc generators that only fire for paths already seeded. This removes the pair-ordering coupling.

### Risk: **MEDIUM-HIGH** (runtime budget is the blocking concern)

---

## Plan 16-04: Local + CI Marker/Signature Enforcement

### Strengths
- Correctly identifies that CI bypasses `make lint` today and adds an explicit CI step (Pitfall 4 from research mitigated).
- POSIX-compatible awk; no custom analyzer, no golangci plugin (D-04 honored).
- Wires `lint: check-validator-markers` so local `make lint` composes the check.

### Concerns

- **[MEDIUM]** **Inverse check is missing.** The awk script fails if a marked function returns `error`. But if a future refactor **removes** the `// MUST_BE_CHECKED_BY_VALIDATOR` marker from `mergeStagedPaths`, the check silently passes and the invariant is gone. **Suggestion:** Add a positive assertion that `builder.go` contains exactly 3 occurrences of `MUST_BE_CHECKED_BY_VALIDATOR`, or that the three specific function names are marked. This closes the gap.

- **[LOW]** Scope limited to `builder.go`. If a future phase splits merge functions into `builder_merge.go` (considered and deferred per CONTEXT), the check becomes silently inactive. **Suggestion:** Change the awk scan target to `*.go` at repo root (or use a `find` loop) for cheap future-proofing — marker is unique enough that false positives on other files are unlikely.

- **[LOW]** `awk` signature-line detection relies on `/^func /` immediately following the marker. Blank lines, doc comments, or build-tag comments between the marker and `func` could cause drift. Given the explicit placement rule ("immediately above these three function declarations"), this is defensible, but brittle. **Suggestion:** Tighten to require the marker on the line directly preceding `func` — reject any intervening non-blank non-whitespace lines.

### Risk: **LOW**

---

## Cross-Plan Concerns

- **[MEDIUM]** **No phase-gate full-suite run.** Each plan runs focused tests (`go test -run ...`). There is no explicit `make test` or `go test ./...` gate. Integration regressions (e.g., an existing test asserting `"builder poisoned"` that was missed during rename, or a test relying on the old merge error return path) could slip past all four plans' focused verifications. **Suggestion:** Add a final verification step after 16-03 completes that runs `make test` + `make lint` + `go build ./...`.

- **[LOW]** **Dependency frontmatter.** Plan 16-03 declares `depends_on: [16-02]` but transitively also depends on 16-01 (validator completeness). The Phase's 3-wave execution ordering makes this moot in practice, but explicit `depends_on: [16-01, 16-02]` would be more correct.

- **[LOW]** **Existing test `TestAddDocumentRejectsUnsupportedNumberWithoutPartialMutation`** (`gin_test.go:2904-2957`) asserts no partial mutation for unsupported literals — this is exactly the behavior Phase 16 strengthens. None of the plans explicitly migrate or strengthen it to add a `tragicErr == nil` assertion. **Suggestion:** Either fold its assertions into Plan 16-03's catalog or explicitly note that it continues to pass unchanged.

- **[LOW]** **Build artifacts.** Research notes that `ami-gin.test` contains old `poisonErr` strings. Not a plan concern but a cleanup item for execution.

---

## Risk Assessment

| Plan | Risk | Main Driver |
|------|------|-------------|
| 16-01 | LOW | Straightforward structural refactor; validator may already be complete |
| 16-02 | MEDIUM | Missing end-to-end recovery test; nil-logger path untested |
| 16-03 | MEDIUM-HIGH | Property-test runtime budget likely blows CI timeout |
| 16-04 | LOW | Inverse check gap (marker-removal undetected) |
| **Overall** | **MEDIUM** | 16-03 as written may not execute in the time budget; 16-02 has one testing gap |

## Top 3 Recommended Changes Before Execution

1. **16-03 Task 3:** Replace `propertyTestParameters()` with `propertyTestParametersWithBudgets(50, 10)` and add a comment documenting why (each iteration is ~2000 doc ingests + 2 encodes). Verify runtime locally before merging.
2. **16-02 Task 2:** Add an end-to-end test that forces a real panic inside `mergeStagedPaths` (not via direct helper call) and proves `b.tragicErr` is set + later `AddDocument` refused. This is what CONTEXT D-06 requested.
3. **16-04 Task 1:** Add positive-presence check: `awk 'END { exit (markers != 3) }'` alongside the signature check, so marker removal is also a lint failure.


---

## Consensus Summary

Both reviewers agree that the phase decomposition is coherent, correctly ordered, and aligned with the locked validate-before-mutate strategy. Gemini rates the plan set LOW risk, while Claude rates it MEDIUM due to execution-budget and test-coverage gaps. The feedback is complementary rather than contradictory: Gemini validates the architectural direction, while Claude identifies specific refinements to make before or during execution.

### Agreed Strengths

- The validator-before-merge architecture is the right mechanism for the Lucene-style per-document contract.
- The plan sequence is sound: make merge infallible first, add tragic recovery boundaries, then prove atomicity and enforce the invariant.
- Static marker/signature enforcement is a lightweight but useful guardrail for future contributors.
- The atomicity verification strategy is strong because it compares encoded output against a clean oracle build.

### Agreed Concerns

- Plan 16-04 marker enforcement needs more robustness. Both reviewers called out marker-check fragility; Claude specifically recommends a positive presence check so marker removal fails lint.
- Exact error-string assertions are acceptable now but brittle for later structured-error work in Phase 18.
- Numeric validation may add overhead; correctness justifies it, but property-test runtime should be controlled so CI remains reliable.

### Divergent Views

- Overall risk: Gemini rates the plan set LOW risk; Claude rates it MEDIUM, mainly because Plan 16-03 could be too expensive if it runs 1000 property iterations with ~1000 documents per corpus.
- Test sufficiency: Gemini considers the recovery and atomicity tests strong as planned; Claude asks for a more end-to-end merge-panic recovery test and a smaller property-test iteration budget.

### Priority Follow-Ups

1. Reduce the property-test iteration count while preserving >=1000 documents per corpus.
2. Add a real-flow recovery test proving `mergeDocumentState` assigns `tragicErr` after a recovered merge panic.
3. Extend the marker check to assert the expected marker count or specific marked functions, not only that marked functions avoid `error` returns.
