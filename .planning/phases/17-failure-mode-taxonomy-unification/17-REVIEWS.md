---
phase: 17
reviewers: [gemini, claude]
reviewed_at: 2026-04-23T15:35:14Z
plans_reviewed:
  - .planning/phases/17-failure-mode-taxonomy-unification/17-01-PLAN.md
  - .planning/phases/17-failure-mode-taxonomy-unification/17-02-PLAN.md
  - .planning/phases/17-failure-mode-taxonomy-unification/17-03-PLAN.md
  - .planning/phases/17-failure-mode-taxonomy-unification/17-04-PLAN.md
---

# Cross-AI Plan Review - Phase 17

## Gemini Review

This review evaluates the implementation plans for **Phase 17: Failure-Mode Taxonomy Unification**. The plans are highly mature, technically grounded in the Phase 16 atomicity contract, and rigorously follow the project's "correctness-first" priority.

### Summary
Phase 17 successfully unifies the mental model for ingest failures across the library. By replacing the specialized `TransformerFailureMode` with a general `IngestFailureMode` and extending it to parser and numeric layers, the library gains a consistent "Hard vs. Soft" routing strategy. The architectural choice to implement "Soft" mode as a silent whole-document skip before durable commit-while preserving the binary v9 format for transformers-is an excellent balance of API modernization and backward compatibility.

### Strengths
- **Surgical Breaking Change:** The plans embrace the breaking rename (`TransformerFailureMode` -> `IngestFailureMode`) for long-term clarity while providing a clear one-line migration path in the `CHANGELOG.md`.
- **Wire-Format Preservation:** Map-on-write/normalize-on-read strategy in Plan 17-02 avoids a binary version bump while allowing the public API symbols to change, which is a sophisticated way to handle legacy compatibility.
- **Lucene-Contract Continuity:** Plan 17-03 uses an internal `errSkipDocument` sentinel to ensure that soft-skipped documents exit the ingest flow before mutating any durable builder state (bitmaps, counts, or position tracking), maintaining the atomicity guarantees from Phase 16.
- **Deterministic Examples:** The inclusion of a fixed-input example with exact expected string output (Plan 17-04) ensures the behavior is easily verifiable by both humans and automated smoke tests.
- **Fail-Fast Configuration:** Every plan includes validation logic in `GINConfig.validate()` and functional options to prevent invalid mode strings from reaching the builder runtime.

### Concerns
- **Transformer Semantics Shift (MEDIUM):** The plan changes transformer "Soft" mode from "skip only the companion" to "skip the whole document." While this is the intended goal of the phase, it is a significant behavioral change for existing users of that specific feature.
    - *Mitigation:* This is well-documented in the plans and the `CHANGELOG.md` task.
- **Parser Contract Boundary (LOW):** There is a risk that a developer might accidentally soften a "Tragic" error or a parser contract violation (like missing `BeginDocument`).
    - *Mitigation:* Plan 17-03 explicitly identifies these as "Hard-Only" and creates a specific test (`TestParserFailureModeSoftKeepsContractViolationsHard`) to guard this boundary.
- **DocID Density (LOW):** If many documents are soft-skipped, the mapping between `DocID` and internal `position` could become sparse if not handled correctly.
    - *Mitigation:* Plan 17-03 and the example in Plan 17-04 explicitly verify "dense row-group packing," ensuring that skipped documents do not consume internal position slots.

### Suggestions
- **Internal Error Provenance:** In Plan 17-03, when implementing the `errSkipDocument` routing, consider using a private wrapper type or a specific builder field (as mentioned in the "Provenance" task) to ensure that only errors *at the current layer* are softened. This prevents a "Soft Parser" setting from accidentally swallowing a "Hard Transformer" error that bubbled up.
- **Example Clarity:** In the `examples/failure-modes/main.go`, consider adding a brief comment explaining *why* a specific document is expected to fail (e.g., "DocID 1 fails because email 42 is an integer, triggering transformer rejection").

### Risk Assessment: LOW
The risk is low because the plans build directly on the "validate-before-mutate" infrastructure established in Phase 16. The most complex part-maintaining v9 serialization compatibility while changing public types-is handled with specific mapping helpers and regression tests. The dependency ordering (API -> Serialization -> Builder -> Docs) is correct and minimizes rework.

**Verdict: Approved for execution.**

---

## Claude Review

# Cross-AI Plan Review: Phase 17 Failure-Mode Taxonomy Unification

` Insight `
- This phase is interesting because it deliberately breaks a public API for clarity, which is unusual in a library moving toward OSS credibility - the trade-off is worth examining.
- The locked decision to have `Soft` at *any* layer skip the whole document (not just the companion) is a behavior-change masquerading as a rename; the plan treats this well but it deserves extra scrutiny.
- Validate-before-mutate architecture from Phase 16 is doing a lot of heavy lifting here - soft-skip routing is essentially "return before commit," which is a surprisingly cheap way to get atomicity.
``

---

## Overall Summary

The four plans cover Phase 17 comprehensively: API rename (17-01), v9 wire-format compatibility (17-02), soft-skip routing through parser/transformer/numeric layers (17-03), and documentation/example integration (17-04). The wave ordering (17-01 -> {17-02, 17-03} in parallel -> 17-04) is sensible, and each plan has clear `must_haves`, threat mapping, and automated acceptance criteria. The most technically demanding work is concentrated in 17-03, where the soft-skip control flow has to thread through parser, staging, validation, and merge without accidentally routing tragic panics or hard stage errors. The plans show strong awareness of Phase 16's atomicity contract and carry forward its encoded-byte oracle pattern as a regression guard.

Risk is **LOW-to-MEDIUM**: the design is sound and locked decisions are specific, but 17-03's parser-error provenance mechanism (distinguishing "parser returned a parser error" from "parser returned an error bubbled up from a staging sink") introduces subtle control-flow complexity that is easy to get wrong and hard to test exhaustively.

---

## Plan 17-01: Unified Public Failure-Mode API

### Strengths

- **Clean breaking rename with no compatibility aliases** - aligns with the locked "clarity over convenience" decision and avoids the deprecation-zombie problem.
- **Struct-literal compatibility is explicitly addressed** via normalization in `NewBuilder` after `config.validate()`, preserving the `TestNewBuilderAllowsLegacyConfigLiteralWhenAdaptiveDisabled` contract.
- **Separation of public API tokens (`hard`/`soft`) from legacy wire tokens (`strict`/`soft_fail`)** is done via private helpers, keeping the public surface clean while 17-02 handles compatibility.
- **Tight acceptance criteria with static `rg` checks** enforce that no old symbols leak back in - this is the right tool for a rename.
- **Task 2's default-and-validation test** pins all four corners: defaults, valid soft config, empty-mode normalization for struct literals, and invalid-value rejection at each layer.

### Concerns

- **[MEDIUM] `normalizeTransformerFailureMode` vs `normalizeIngestFailureMode` ambiguity.** The plan keeps *both* helpers - `normalizeTransformerFailureMode` must accept legacy wire tokens (`strict`, `soft_fail`) because it is called from serialization decode paths, while `normalizeIngestFailureMode` should only accept the public values. The plan mentions this but does not make it explicit that `validateTransformerFailureMode` and `validateIngestFailureMode` have *different* accept sets. A future contributor could easily collapse them, breaking v9 decode. Consider a comment on each helper clarifying the asymmetric accept sets, or a test that pins `validateIngestFailureMode(IngestFailureMode("strict"))` returns an error while `validateTransformerFailureMode(IngestFailureMode("strict"))` returns nil.

- **[LOW] `WithParserFailureMode` and `WithNumericFailureMode` error wrapping inconsistency.** The sketch in the context wraps with `errors.Wrap(err, "parser failure mode")`, but the task only requires "returns a github.com/pkg/errors error for invalid values." Whether the error string contains `"invalid ingest failure mode"` (which Task 2's test asserts) depends on the implementation choice - if the planner wraps with `errors.Wrap`, the wrapped message prefixes but still includes the substring. Worth being explicit.

- **[LOW] `phase09_review_test.go` is listed in `files_modified` but not explained.** The plan should note why this file needs mechanical updates (presumably uses the old constants).

### Suggestions

- Add an explicit test `TestValidateIngestFailureModeRejectsLegacyTokens` that asserts the public validator rejects `strict`/`soft_fail`, so the asymmetry with the transformer metadata validator is pinned.
- Consider whether `WithTransformerFailureMode` should accept empty strings (normalizing to hard) or reject them at the option level. The current pattern accepts empty via normalization, but this is unusual - most functional options reject explicitly meaningless input.
- Task 1 and Task 2 both touch `failure_modes_test.go` and `gin.go`; consider whether they should be one task or if there's clear TDD benefit to splitting (Task 2 is entirely tests, so it's a reasonable split).

### Risk: **LOW**

Rename-and-add is mechanical; the main risks (static symbol leaks, struct-literal breakage) are caught by the acceptance gates.

---

## Plan 17-02: v9 Serialization Compatibility

### Strengths

- **Explicit "do not bump Version" guard via `rg -n 'const Version = 9'`** - a simple but effective regression gate.
- **Copy-slice-before-marshal pattern** in `writeRepresentations` avoids mutating the live builder config, which is a subtle correctness concern the plan gets right.
- **Hand-built `SerializedConfig` test payload** proves decode compatibility without depending on a pre-existing v9 golden file, which is more robust than relying on fixtures.
- **Three complementary tests** (round-trip, legacy-decode, wire-token-regression) triangulate the behavior from both write and read sides.

### Concerns

- **[MEDIUM] No test for the "write path outputs legacy tokens" direction.** `TestTransformerFailureModeWireTokensStayV9` inspects encoded output for the `soft_fail` string, but the assertion `does not include "soft" as the transformer failure_mode` is fragile - the substring `soft` appears inside `soft_fail`, so a naive string match will trip over itself. The test needs to parse the JSON payload and inspect the `failure_mode` field value exactly, not substring-search the raw bytes. Worth making this explicit in the task.

- **[MEDIUM] Parity-golden files.** Phase 13/16 used committed parity goldens under `testdata/parity-golden/`. If any golden contains a transformer with failure-mode metadata, the wire-token-preservation guarantee means goldens should remain byte-identical. The plan does not mention regenerating or verifying goldens. If goldens *do* change byte-wise, that is a red flag that wire tokens drifted; if they don't, the plan is silently correct. Consider adding a sanity check: `make test` or equivalent should exercise the parity golden suite and stay green without regeneration.

- **[LOW] `writeRepresentations` copy pattern not shown in detail.** The task says "marshal a copied slice of representations, not the original slice," but this is load-bearing - if the planner copies the slice but not the embedded `TransformerSpec`, the mutation still leaks. Consider spelling this out: deep-copy the `Transformer` field explicitly.

- **[LOW] No test for unknown transformer failure-mode tokens.** What happens if a malicious v9 payload contains `failure_mode: "panic"`? The current `validateTransformerFailureMode` would reject it at decode time, which is correct, but this is not pinned by a test.

### Suggestions

- Replace the `does not include "soft"` check with a structured JSON-field assertion, e.g., parse the encoded config payload, iterate `Transformers`, and assert `spec.FailureMode == "soft_fail"` exactly.
- Add a `TestReadConfigRejectsUnknownTransformerFailureMode` using the hand-built `SerializedConfig` pattern, to pin the security posture against crafted indexes.
- Note in the plan whether `testdata/parity-golden/` needs re-verification and what "green" means there.

### Risk: **LOW-to-MEDIUM**

Wire-format compatibility is a forever contract - the tests are good but a substring-based assertion could hide drift.

---

## Plan 17-03: Soft-Skip Routing (Most Complex Plan)

### Strengths

- **The `errSkipDocument` sentinel approach** is the right choice: simple, uses existing `errors.Cause` machinery, and keeps the diff minimal.
- **Explicit preservation of Phase 16's full-vs-clean encoded-byte oracle** as a regression test for soft skips - this is exactly the correct way to prove atomicity holds under the new semantics.
- **Parser contract violations remain hard** (D-09) is enforced by a dedicated test, which guards against a subtle footgun.
- **`TestNumericFailureModeSoftKeepsMergeRecoveryTragic`** directly tests the most dangerous anti-pattern - letting soft mode swallow a tragic panic.
- **Transformer soft-mode test rewrite** (`TestBuilderSoftFailSkipsCompanionWhenConfigured` -> `TestBuilderSoftFailSkipsDocumentWhenConfigured`) documents the intentional behavior change cleanly.

### Concerns

- **[HIGH] Parser-error provenance mechanism is underspecified and fragile.** The plan introduces a `currentDocStageErr` field to distinguish "ordinary parser error from `Parse` itself" from "stage error bubbled up through the parser sink." The control flow is:
  ```
  b.parser.Parse(...) returns err
  -> was err caused by a StageX sink call returning an error?
     -> yes: do not soften, return original err
     -> no: apply parser soft mode
  ```
  But the stdlib parser may not propagate sink errors identically to future parsers, and the provenance flag has to be reset at every entry/exit point of `AddDocument`, including panics. If the flag is set during staging but the parser catches the sink error and returns its own wrapped error, the provenance is lost. **Consider whether the cleaner design is to have staging functions return `errSkipDocument` directly when numeric/transformer soft is configured, and let parser-mode-soft apply only to errors that are NOT `errSkipDocument` and NOT from the public failure catalog's staging sites.** In other words: parser soft should be "if the error has no other classification, treat as parser error." This inverts the check and sidesteps the provenance-flag lifecycle issue.

- **[MEDIUM] Order-of-checks in `AddDocument` parser branch matters and is fragile.** The task spells out a 4-step order:
  1. `isSkipDocument(err)` -> nil
  2. Stage error -> return original
  3. Parser soft -> nil
  4. Otherwise -> return err

  Step 1 (skip-document from staging) must come before step 2 (stage hard error) because a soft stage failure that was converted to `errSkipDocument` should skip, not surface. But what if a stage function returns a *non-skip* error (hard-mode numeric failure) that the parser wraps? The provenance field is needed precisely because steps 2 and 3 are ambiguous. This deserves a dedicated test: `TestParserHardDoesNotSwallowNumericHardErrorFromSink` or similar.

- **[MEDIUM] `currentDocStageErr` field is reset on entry to `AddDocument` but what about re-entrancy?** The builder is documented as single-threaded, but the defensive reset pattern should mirror `currentDocState` and `beginDocumentCalls` exactly. If a panic happens between reset and `Parse`, the defer-based cleanup from Phase 16 needs to cover this field too.

- **[MEDIUM] `TestSoftFailureModesMatchCleanCorpus` corpus design.** The task specifies five documents including "one transformer-rejected document" and "one validator-rejected mixed numeric promotion document." But validator-rejected promotion requires a *prior* accepted document with a specific numeric type, so the ordering matters: the accepted seed doc must come before the validator-failing doc in *both* the full and clean corpora, with the same DocID. The plan should spell out the DocIDs used in the clean-only corpus to make this unambiguous.

- **[MEDIUM] Soft mode with derived raw indexing (D-11).** The locked decision says "soft transformer mode should not leave the raw value indexed without its required derived companion representation." The implementation in `stageCompanionRepresentations` returns `errSkipDocument` on rejection, which unwinds the whole document. But what if the raw value was already staged in `documentBuildState` *before* `stageCompanionRepresentations` ran? The task says "skip before durable mutation" - and since staging is in `documentBuildState` (not durable), this works. But it's worth a test: a transformer that rejects on the *second* field of a document, where the first field was already staged. Assert no paths from either field appear in the finalized index.

- **[LOW] First-failure-wins (D-08) is asserted in prose but not test-pinned.** Consider a test where a document has both a malformed numeric literal and a transformer-rejectable value; assert the error/skip reflects whichever one is encountered first deterministically.

### Suggestions

- **Invert the parser-error provenance logic.** Instead of tracking "was this error from a stage sink?", track whether staging explicitly returned `errSkipDocument`. Let staging hard-errors propagate through the parser sink unchanged and be handled by the parser-error branch: if the error is `errSkipDocument`, skip; if it matches `isStageHardError`-by-origin (e.g., a tagged sentinel), return as-is; otherwise apply parser soft. This way, the provenance is carried by error *value*, not builder *state*.
- Add `TestParserHardDoesNotSwallowNumericHardErrorFromSink` and `TestTransformerSoftUnwindsPartiallyStagedDocument` as explicit pinning tests.
- Make the clean-corpus DocID mapping in `TestSoftFailureModesMatchCleanCorpus` explicit (e.g., clean corpus uses DocIDs 0, 2, 4 to match the accepted docs in the full corpus).
- Consider moving `runMergeWithRecover` error handling into a `commitStagedPaths` comment that says "// never soft - tragic only" as a durable signal to future maintainers.

### Risk: **MEDIUM**

The error-provenance mechanism is the single concentrated risk in Phase 17. If it goes wrong, soft parser mode could either swallow hard staging errors (correctness regression) or fail to soften legitimate parse errors (UX regression). The atomicity oracle will catch *some* of this but not all cases.

---

## Plan 17-04: Changelog & Example

### Strengths

- **Deterministic example with fixed DocIDs and fixed expected output lines** - follows the existing example conventions and makes the smoke test trivially verifiable.
- **Example covers three soft modes in one run** (parser `not-json`, transformer `{"email":42}`, numeric `1.5` after `MaxInt+1`), demonstrating the unified mental model end-to-end.
- **Full phase integration verification** (`make test && make lint && go build ./...`) at the end is the correct exit gate.
- **Forbidden-content guard** (`rg -n 'https?://|/Users/' CHANGELOG.md`) enforces the AGENTS.md policy on internal references.

### Concerns

- **[MEDIUM] Example expected output is brittle.** The hard-config stops after "1 indexed document" - but that's only true if `DocID(0)` with `score=9007199254740993` (a large int) succeeds in hard mode. With `WithEmailDomainTransformer` hard, DocID(0) has a valid email and should index cleanly. DocID(1) is `{"email":42}` - transformer rejects. So the hard example stops at DocID(1), having indexed DocID(0). The count of "1" is correct *assuming the numeric promotion validation does not reject DocID(0)* - which it shouldn't, because there's no prior float observation. But then DocID(2) would promote `$.score` to float and reject. If the hard example ingested documents *in order*, it would stop at DocID(1) with the transformer error, never seeing the numeric issue. The exact expected string assumes this ordering and counting - worth spelling out in the plan what "indexed" means (finalized vs. accepted by AddDocument).
- **[LOW] The soft count of 2.** With the full 5-doc corpus in soft mode, why is the answer 2 and not 3? Let me walk through:
  - DocID(0): email=alice, score=9007199254740993 (big int) -> accepted
  - DocID(1): email=42 -> transformer rejects -> soft skip
  - DocID(2): email=bob, score=1.5 -> seeds float; previously int. Numeric validator rejects mixed promotion -> soft numeric skip
  - DocID(3): `not-json` -> parser error -> soft parser skip
  - DocID(4): email=carol, score=9007199254740992 -> accepts (int still)

  So accepted = {0, 4} = 2 documents. That checks out, and the row-groups `[0 1]` for `example.com` are correct (DocIDs 0 and 4 map to dense positions 0 and 1). Good. But this is non-obvious from the plan - a comment in the example explaining why the count is 2 would help.

- **[LOW] `rg` regex in verify uses `\\[0 1\\]` which would match literal backslashes.** The rg pattern should be `\[0 1\]` unshell-escaped, or the command should use single quotes consistently. Worth spot-checking.

- **[LOW] No Wave 3 lint/test sampling rule.** The plan ends with a full `make test && make lint && go build ./...`, which is correct for the phase gate. But 17-04 is the only plan that exercises the integrated system - if something breaks between 17-02 and 17-03 that only shows up with the example running, the failure mode is "Wave 3 task 2 fails and we go back to untangle."

### Suggestions

- Add a brief comment in the example explaining the expected counts and why (hard stops at DocID(1); soft accepts {0, 4}).
- Replace substring-based stdout matching in `verify` with an exact-line match to avoid false positives.
- Consider adding an integration test `TestExampleFailureModesOutput` that runs `go run ./examples/failure-modes/main.go` and asserts the exact output, so regressions are caught in CI, not just manually.

### Risk: **LOW**

Documentation and example work is mostly mechanical; the main risk is the example output drifting from what the plan specifies.

---

## Cross-Cutting Observations

### Dependency Ordering

- **17-01 -> {17-02, 17-03} -> 17-04** is correct. 17-02 and 17-03 can run in parallel because they touch disjoint surfaces (serialize.go vs builder.go) after 17-01's API is in place.
- **One cross-dependency not called out:** 17-03's `TestSoftFailureModesMatchCleanCorpus` uses `Encode` to compare bytes. If 17-02 has not yet landed the wire-token preservation, encoded bytes may differ between the full-corpus build (containing the transformer with soft failure mode) and the clean-only build - because both would use the new token spelling and be byte-equal. Actually this works either way (both sides use whatever spelling is current), so it's not a blocker. But if 17-03 lands before 17-02, the wire format temporarily writes `soft` instead of `soft_fail`, which is a v9 compatibility break until 17-02 merges. Consider making 17-02 a hard prerequisite for 17-03, or ensuring the wire-token helper exists as a stub before 17-03's tests run.

### Missing Edge Cases

- **What happens in soft parser mode when `b.tragicErr != nil`?** The tragic gate at the top of `AddDocument` returns early with a wrapped tragic error. Soft mode should *not* soften this - it's tragic, not per-document. None of the plans explicitly test `AddDocument` with soft parser mode after a tragic panic. Worth one test.
- **Soft mode and position allocation.** The plan says soft-skipped documents do not advance `nextPos`. But the position is computed *before* parsing (`pos = b.nextPos` if not in `docIDToPos`). If the parser soft-skips, `nextPos` stays put. Fine. But what if the *same* `DocID` is re-ingested later with valid content? Then `docIDToPos` does not have it, `pos = b.nextPos`, accepted. Works. Worth a test.
- **Soft mode on retry semantics.** If caller retries the same DocID with corrected content after a soft skip, does it succeed? Implicit in the design, explicit in no test.

### Scope Discipline

- The plans resist scope creep well: no telemetry for soft skips, no `IngestError` (that's Phase 18), no `ValidateDocument`. Good.
- One area of mild creep: the `currentDocStageErr` field is a non-trivial addition to `GINBuilder` that exists solely to support parser-mode provenance. This is the "structural addition for a routing decision" that the suggested invert-the-logic approach would eliminate.

### Security Posture

- Threat models are well-developed and consistent across plans (T-17-01 through T-17-05).
- No new attack surface introduced: soft mode is a caller-configured skip with no logging, so information disclosure risk is nil.
- One potential concern: if a caller configures soft mode on all three layers, a malicious input stream can be *silently* dropped at ingest. The caller sees no error and may conclude the index is complete. This is the caller's choice (documented in the example), but it's worth noting that the `experiment --on-error continue` CLI (Phase 18) will be the primary tool for surfacing this - there is no Phase 17 counter.

### Performance Implications

- **Zero perf work, correctly.** The project priority is correctness -> usefulness -> performance. No benchmarks needed for a routing change.
- **One minor perf note:** `normalizeIngestFailureMode` is called on every parser/numeric/transformer failure path. If it's `if mode == "" { return IngestFailureHard }`, the cost is nil. If it's a switch, negligible. Not worth benchmarking.

---

## Overall Risk Assessment: **LOW-to-MEDIUM**

**Justification:** The phase has a tight scope, well-locked decisions, strong regression guards (Phase 16's encoded-byte oracle is the star), and good threat-model coverage. The primary risk is concentrated in Plan 17-03's parser-error provenance mechanism, which introduces builder state (`currentDocStageErr`) to support a control-flow decision that could be handled more cleanly by carrying provenance in the error value itself. This is fixable in planning - I'd recommend inverting the provenance logic before execution rather than discovering the fragility during implementation. The other plans (17-01, 17-02, 17-04) are low-risk mechanical work with appropriate test coverage.

### Top Three Actions Before Execution

1. **Revisit the parser-error provenance design in 17-03 Task 1.** Strongly consider carrying provenance in error values (sentinel or tagged error) rather than a builder state field. This eliminates the lifecycle-management risk.
2. **Tighten 17-02 Task 2's wire-token assertion.** Replace substring matching with structured JSON-field inspection to avoid `soft`-matches-`soft_fail` confusion.
3. **Add an explicit test for the "soft mode on all three layers, malicious input stream" scenario** to pin the silent-drop semantics as documented behavior. This also serves as a natural input to Phase 18's CLI grouped-failure summary work.

---

## Consensus Summary

Both reviewers consider the Phase 17 plans substantially ready and aligned with the v1.2 correctness goal. Gemini rated overall risk LOW; Claude rated overall risk LOW-to-MEDIUM because Plan 17-03 concentrates subtle control-flow risk around parser versus staging error provenance.

### Agreed Strengths

- The unified `IngestFailureMode` API is a clear, intentional breaking change that improves the public mental model.
- The plans preserve the Phase 16 validate-before-mutate atomicity contract by routing soft skips before durable builder mutation.
- v9 transformer metadata compatibility is correctly treated as a format contract, with no parser/numeric config added to finalized index serialization.
- Hard defaults and invalid-mode validation preserve current behavior unless callers explicitly opt into soft skips.
- The deterministic hard-vs-soft example is useful as both documentation and a smoke test.

### Agreed Concerns

- Transformer soft mode changes behavior from companion-only skip to whole-document skip. This is intended, but it must be clearly documented and tested so rejected raw values never leak into the index.
- Parser soft mode must not soften parser contract violations, tragic builder state, or hard staging errors that bubble through parser callbacks.
- Soft skips must preserve dense position packing, `DocID` retry behavior, and no-mutation guarantees across parser, transformer, and numeric failures.
- The example output depends on document ordering and accepted-document counts; comments and exact output assertions should make that behavior obvious.

### Additional High-Priority Follow-Up

Claude identified one HIGH concern: the proposed `currentDocStageErr` provenance mechanism in 17-03 may be fragile. Before execution, revise the plan to prefer error-carried provenance where possible, or add explicit lifecycle/reset tests proving a builder field cannot leak across documents, panics, or retries.

### Planning Adjustments To Consider

- Add a public-validator test that rejects legacy serialized tokens (`strict`, `soft_fail`) while transformer metadata decode still accepts them.
- In 17-02, assert transformer wire tokens by parsing serialized config fields exactly instead of substring matching, since `soft` is contained in `soft_fail`.
- Add decode rejection coverage for unknown transformer failure-mode tokens.
- Make the clean-corpus `DocID` mapping explicit in `TestSoftFailureModesMatchCleanCorpus`.
- Add a regression test where transformer soft rejection occurs after earlier fields were staged, proving the whole staged document is discarded.
- Add a retry test showing a soft-skipped `DocID` can later be accepted without consuming an internal position.

### Divergent Views

Gemini approved the plans for execution with LOW risk and treated parser provenance as a general implementation detail. Claude was stricter, classifying Plan 17-03 as MEDIUM risk and recommending a design adjustment before execution. The actionable difference is to feed Claude's 17-03 provenance feedback back into planning before running `$gsd-plan-phase 17 --reviews` or execution.
