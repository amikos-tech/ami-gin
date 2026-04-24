---
phase: 18
reviewers: [gemini, claude]
reviewed_at: 2026-04-24T11:42:47Z
plans_reviewed:
  - 18-01-PLAN.md
  - 18-02-PLAN.md
  - 18-03-PLAN.md
  - 18-04-PLAN.md
---

# Cross-AI Plan Review - Phase 18

## Gemini Review

# Cross-AI Plan Review

## Summary
The provided implementation plans are exceptionally well-structured, logically sequenced, and highly detailed. They successfully translate the Phase 18 requirements into executable tasks with clear boundaries, especially regarding what constitutes a user-facing `IngestError` versus an internal, tragic, or soft error. The division of work into API Definition (18-01), Guard/Enforcement (18-02), CLI Reporting (18-03), and Documentation/Verification (18-04) ensures a safe, iterative, and highly testable progression.

## Strengths
- **Clear Error Boundaries:** Explicitly and correctly distinguishes between user-document hard failures (which get wrapped in `IngestError`) and parser contract/tragic errors (which must remain unchanged to avoid breaking existing recovery flows).
- **Error Ecosystem Compatibility:** Wisely mandates the inclusion of the `Cause() error` method to maintain seamless compatibility with `github.com/pkg/errors`, which is the project's established standard error library.
- **Pragmatic Enforcement:** The decision to use a scoped static guard in the `Makefile` (targeting specific ingest files) rather than a noisy repo-wide ban on `errors.New`/`errors.Wrap` is a smart, low-friction approach.
- **CLI Output Safety:** Explicitly caps CLI failure samples at 3 per layer to prevent unbounded memory growth and log spam. It also strictly preserves the dense row-group packing logic.
- **Comprehensive Testing:** The 100-document fixture with known failure ratios is an excellent end-to-end acceptance test that proves both the aggregation logic and the JSON output format.

## Concerns
- **CLI Accumulator Determinism (LOW):** Plan 18-03 suggests accumulating failures in a `map[gin.IngestLayer]*experimentFailureGroup` and later requires deterministic output order. Since Go map iteration is randomized, the implementation will need to explicitly sort the resulting slice or use a slice-based ordered accumulator from the start.
- **Static Guard Brittleness (LOW):** The `check-ingest-error-wrapping` target in Plan 18-02 relies on strict string matching via `awk` or `rg` for specific error message strings (e.g., `"parse numeric at %s"`). While pragmatic, this could become brittle and require manual `Makefile` updates if developers slightly tweak error strings in the future.
- **`fmt.Sprint` on Complex Values (LOW):** Plan 18-01 suggests using `fmt.Sprint(value)` for the `Value` field. While perfectly fine for scalars and basic tokens, if `value` happens to be a deeply nested structure (e.g., an unsupported map type), the output could be large. This is mostly mitigated by the fact that it's targeted at native numerics and scalar tokens.

## Suggestions
- **Specify Sorting Logic in 18-03:** Explicitly instruct the agent to sort the aggregated failure groups before appending them to `report.Summary.Failures`. A predefined priority order (`parser`, `transformer`, `numeric`, `schema`, then lexical) will ensure the JSON/text output is fully deterministic for testing.
- **POSIX-Compliant Guard:** For the `Makefile` guard in 18-02, ensure that any `awk` or `rg` (ripgrep) commands are standard enough to run smoothly across both macOS and Linux environments to prevent CI pipeline failures.
- **Guard Alternative:** If the `Makefile` string-matching proves too brittle during execution, empower the agent to replace it with a lightweight AST-based Go test (e.g., `golang.org/x/tools/go/analysis`) that specifically checks returns in `builder.go`, though the `Makefile` approach is fine for a first pass.

## Risk Assessment
**LOW**

The plans are highly rigorous, deeply integrated with the project's existing architectural context, and defensively designed. The test strategies - especially the comprehensive behavioral test matrix and the targeted 100-doc CLI fixture - provide high confidence that the changes will meet the milestone's success criteria without introducing regressions to atomicity or soft-mode behavior.

---

## Claude Review

# Cross-AI Plan Review - Phase 18: Structured IngestError + CLI integration

**Reviewer:** Claude Opus 4.7
**Date:** 2026-04-24
**Plans reviewed:** 18-01, 18-02, 18-03, 18-04

## Overall Summary

The four plans form a coherent, well-decomposed implementation of IERR-01..03 that closely mirrors the research findings and context decisions. The dependency DAG (01 -> {02, 03} -> 04) is correct and enables parallel execution of the guard/test work and the CLI work once the public API lands. Scope is appropriately tight: plans don't drift into redaction policies, failure-code enums, or soft-mode observability that the context explicitly defers. The main weaknesses are (a) one gap around `validateStagedPaths`-level numeric rejection wrapping, (b) a static guard that regresses on historical strings rather than structurally enforcing the invariant, and (c) a few under-specified test-ordering details in the 100-doc fixture. None are blocking; the phase is safely executable.

**Overall Risk: LOW-MEDIUM**

## Plan 18-01: Public API + Builder Wrapping

### Summary
Creates `ingest_error.go` with `IngestLayer`/`IngestError` and wraps hard failures at six sites in `builder.go`. Preserves the critical AddDocument branch ordering (skipDocument -> stageCallback -> parser-soft -> parser-hard). Tests cover extraction through `errors.Wrap`.

### Strengths
- Correctly preserves `isStageCallbackError` unwrapping before parser wrapping - prevents numeric/transformer errors being mislabeled as parser (flagged as the top risk in research).
- `Err` field name (not `Cause`) is the right call given `Cause() error` method shadowing with `pkg/errors`.
- Nil-safe `Error()`/`Unwrap()`/`Cause()` methods.
- Tests verify both `errors.As` through stdlib wrap AND `errors.Cause` through `pkg/errors` chain.
- Schema-layer mapping to `stageScalarToken`/`stageMaterializedValue` default branches is defensible - it surfaces "unsupported value shape" without inventing a new concept.
- Transformer site uses `canonicalPath` not `registration.TargetPath`, matching D-10.

### Concerns
- **[MEDIUM] Missing `validateStagedPaths` wrapping site.** Per Phase 16, mixed numeric promotion rejection was hoisted from `mergeNumericObservation` into `validateStagedPaths`. The plan's Task 2 says "In `stageNumericObservation`, hard unsupported mixed promotion failures return numeric-layer `IngestError`" - but Phase 16's work moved that rejection boundary to the validator. The plan should explicitly cover wrapping at the validator-rejection site (`validateStagedPaths` or wherever Phase 16 hoisted the check), not just `stageNumericObservation`. Without this, the `numeric_mixed_promotion` test case could pass against the pre-hoist code path but miss the real one.
- **[MEDIUM] Task 2 is unusually large.** Six distinct call sites with different `Value` formatting rules (raw literal, `fmt.Sprint`, custom `formatStagedNumericValue`), plus three new helpers, plus three new tests. A single failing acceptance criterion forces re-running the whole task. Consider splitting into (a) helpers + parser + transformer sites, (b) numeric + schema sites + validator site, (c) test matrix.
- **[LOW] `formatStagedNumericValue` is under-specified.** Research says "prefer adding a formatter for `stagedNumericValue`" but the plan introduces it without specifying output format or whether it needs tests of its own. What does `formatStagedNumericValue({int64: 9007199254740993})` produce? `"9007199254740993"`? What about the observation that triggers promotion - is it the new offending value or the prior state? This affects the test assertion `Value "not-empty"`.
- **[LOW] Double-wrap risk.** `newIngestError(..., errors.Wrap(err, "parse numeric"))` wraps an already-wrapped error. The resulting `IngestError.Err` chain becomes `wrap(original)`, and `IngestError.Error()` prepends another layer. Messages like `"ingest numeric failure at $.score: parse numeric: invalid syntax"` are fine but worth checking one golden assertion to lock the shape.
- **[LOW] Parser `Value: string(jsonDoc)` for large documents.** No size cap mentioned. A 1MB malformed doc pins 1MB into every error object, which the CLI may then copy into samples. Research notes the verbatim policy, but a `const maxIngestErrorValueBytes = 4096` with truncation marker would be defensible without violating "library does not redact" (truncation != redaction).

### Suggestions
1. Add explicit wrapping task for the validator-rejection site hoisted in Phase 16.
2. Split Task 2 into two or three tasks.
3. Specify `formatStagedNumericValue` output format with one example in `<action>`.
4. Add one golden assertion on `IngestError.Error()` output to lock the format.
5. Consider a truncation policy for `Value` length (non-blocking for Phase 18 but cheap now).

## Plan 18-02: Behavior Matrix + Static Guard

### Summary
Hardens the Plan 18-01 test matrix with named subtests and adds a scoped Makefile guard `check-ingest-error-wrapping` wired into `make lint`.

### Strengths
- Correctly avoids repo-wide grep (matches research recommendation).
- Explicitly allowlists parser contract strings, tragic state strings, and `blank JSONL line` - the research's biggest pitfall avoided.
- `lint: check-validator-markers check-ingest-error-wrapping` ordering matches the existing precedent.
- Behavior matrix asserts "builder remains usable after hard public failure" - reinforces the Phase 16 atomicity contract.

### Concerns
- **[HIGH] Guard is a historical-string blocker, not a structural invariant.** The guard pattern-matches on strings like `"parse numeric at %s"` that are being *removed* in 18-01. It enforces "don't bring these back" but does nothing to catch a *new* hard-ingest return with different wording (e.g., a future contributor adding `return errors.Errorf("field %s exceeds schema bound", path)` to a new stage helper). The real invariant - "hard ingest returns go through `newIngestError`" - is not expressible as a string scan. This is acknowledged in research ("If the guard becomes brittle, a focused Go test ... is preferable") but the plan chose both the test matrix AND the brittle guard. Consider: is the guard earning its keep, or does the behavioral matrix + `go vet`-level discipline suffice?
- **[MEDIUM] Tight coupling to 18-01 test names.** Task 1 requires subtest names like `parser_unknown_path` to exist. If 18-01 executor names them slightly differently (`parser_no_path` etc.), Task 1 fails on acceptance. Consider specifying the subtest names in 18-01's `<acceptance_criteria>` instead of only here.
- **[LOW] `errSkipDocument` is in the guard allowlist but is an internal sentinel.** Fine, but the allowlist growing over time is a signal the guard is the wrong shape.

### Suggestions
1. Consider dropping the Makefile guard in favor of a single Go test that enumerates all `builder.go` top-level error-returning functions and asserts via AST/reflection that specific ingest sites produce `*IngestError`. Higher signal, lower false-positive surface.
2. If keeping the guard: move the expected subtest names into 18-01's acceptance criteria to prevent hand-off drift.
3. Add a comment in `Makefile` explaining the guard's scope and its intentional blindspots (the research context is only in `.planning/`).

## Plan 18-03: CLI Aggregation + 100-Doc Test

### Summary
Extends `experimentSummary` with `Failures []experimentFailureGroup`, adds `recordExperimentIngestFailure` helper, renders text + JSON output, and adds the roadmap-required 100-doc fixture.

### Strengths
- Preserves single-object JSON shape (D-14) and keeps `line N: err` stderr output for backwards compatibility with existing tests.
- Dense packing assertion is explicit: `summary.row_groups == 9` for 90 docs / rg-size 10.
- Deterministic group ordering (parser, transformer, numeric, schema, then lexical) prevents golden-test flakes.
- `InputIndex: lineNumber - 1` matches the 0-based convention and keeps failed lines off accepted-doc positions.
- `experimentDefaultConfig` test hook pattern is idiomatic Go and keeps prod CLI flags unchanged.

### Concerns
- **[MEDIUM] 100-doc fixture ordering is under-specified.** The plan says "first seeding a large integer score (9007199254740993) in a valid line and later adding records that trigger mixed numeric promotion." But:
  - If the large-int seed line is at position 50 and the failing float lines are at positions 20, 30, 40, the first float ingests fine (no prior int), the second/third also fine, and only post-50 floats would fail promotion. Test breaks.
  - The plan doesn't say *which* of the 90 valid lines carries the seed, or which 3 lines are the float-triggers.
  - Recommend explicit ordering: line 1 = seed large int with `email` valid, then 89 safe mixed, with failures interleaved at fixed positions (e.g., 2, 25, 50 parser; 10, 30, 60, 80 transformer; 40, 70, 90 numeric - all *after* line 1).
- **[MEDIUM] `experimentDefaultConfig` override is global package state.** If tests run in parallel (`t.Parallel()`), the override races. Plan uses `t.Cleanup` but doesn't forbid `t.Parallel()` in these tests. Add a note or guard.
- **[LOW] Text format `line %d input_index %d path %q value %q: %s` has no length cap.** A 1MB `Value` (see 18-01 concern) would dump into stderr/stdout here. CLI tolerability matters more than library correctness - the 3-sample cap helps but per-sample size is unbounded.
- **[LOW] `summary.documents == 90` asserts exactly 90 - but the 3 numeric failures depend on prior promotion state. If the promotion-trigger pattern doesn't fire as expected (see ordering concern above), the test gets `documents: 93` and fails loudly, which is actually the right behavior. Good defense in depth, but the ordering concern remains.
- **[LOW] No test for `--on-error abort` path.** Abort mode should still return the `*IngestError` to the caller via stderr. Plan focuses on continue mode (correctly per spec), but one abort-mode sanity test would catch regressions to the plain-error return path there.

### Suggestions
1. Pin the fixture with explicit line numbers for each failure type - either in the plan or as a fixture file.
2. Explicitly document non-parallel constraint on tests using `experimentDefaultConfig` override.
3. Add a sample-length truncation (e.g., 256 bytes) in `recordExperimentIngestFailure` - keeps samples scannable.
4. Add one abort-mode test asserting `*IngestError` is returned from `runExperiment` / visible on stderr.

## Plan 18-04: Docs + Final Verification

### Summary
Adds Godoc to `ingest_error.go`, CHANGELOG entry, runs final test+lint, updates `18-VALIDATION.md` with Execution Results.

### Strengths
- Narrow, unambiguous scope.
- Verification commands match research's "Suggested Test Commands" verbatim.
- Fallback for local `golangci-lint` absence is reasonable.

### Concerns
- **[LOW] Godoc exact-sentence requirement is brittle.** "`Value is a verbatim string representation of the offending input or value; the library does not redact it.`" - punctuation mismatches will fail acceptance. Worth loosening to semantic match or specifying the grep pattern.
- **[LOW] CHANGELOG entry doesn't mention the `make check-ingest-error-wrapping` addition.** Minor - CHANGELOG is user-facing and this is a dev-facing change, so omission is defensible. Flag for awareness only.

### Suggestions
1. Loosen the exact-sentence acceptance to a substring match (e.g., "contains `verbatim` and `does not redact`").
2. Consider whether CHANGELOG should note the new `make` target for contributors.

## Cross-Cutting Concerns

### Dependency & Ordering
- **DAG is correct:** 18-01 must land before 18-02 (guard depends on new helpers) and 18-03 (CLI uses `IngestError`). 18-04 strictly depends on 01/02/03. Parallel execution of 18-02 and 18-03 after 18-01 is safe - they touch disjoint files.
- **Implicit coupling:** Subtest names in 18-01 are re-asserted in 18-02. Should be explicit in 18-01 acceptance.

### Security
- **Information disclosure** is accepted policy (D-08), documented in 18-04. No net-new risk beyond what the phase chose.
- **No input-length bounds** on `Value` - minor DoS surface if a caller logs `IngestError.Error()` on untrusted gigabyte JSON. Low severity; mitigation is a 4KB truncation in the helper.

### Performance
- **Hot path impact:** Wrapping hard ingest errors adds allocations per failing document only. On the success path, no change. Acceptable per project priorities (correctness > perf).
- **CLI failure aggregation:** Bounded by 3 samples x 4 layers = 12 sample objects max. Unbounded count accumulator is fine.

### Scope Discipline
- Plans resist the temptation to add: failure-code enums, sentinel errors, soft-skip observer API, `ValidateDocument`, redaction hooks. All correctly deferred per context.
- No over-engineering observed.

### Goal Achievement
Cross-checking against Phase 18 success criteria:

| # | Success Criterion | Coverage |
|---|---|---|
| 1 | `IngestError` with Path/Layer/Cause/Value | Yes, 18-01 (naming: `Err` not `Cause`, method `Cause()` bridges) |
| 2 | `errors.As` round-trip | Yes, 18-01 Task 1 + 18-02 matrix |
| 3 | All ingest sites wrapped + grep/test guard | Partial: 18-01 (validator-rejection site gap) + 18-02 (guard is cosmetic) |
| 4 | CLI grouped by Layer in text + JSON | Yes, 18-03 |
| 5 | 100-doc test with 3/4/3 counts | Yes, 18-03 (ordering under-specified) |

## Risk Assessment

**LOW-MEDIUM**

**Justification:**
- Architecture is sound, dependencies are correct, test coverage is thorough.
- Primary risks are implementation-detail gaps (validator-site wrapping, fixture ordering) that will surface as test failures during execution rather than latent bugs.
- The static guard being cosmetic is a long-term concern but doesn't block Phase 18 correctness - the behavioral matrix is the real enforcement.
- No security, scope-creep, or architectural concerns.
- `Value` length cap is the one concrete missing piece; all others are clarifications.

**Recommended pre-execution fixes (in priority order):**
1. Add explicit `validateStagedPaths` wrapping task in 18-01.
2. Pin 100-doc fixture line ordering in 18-03.
3. Clarify `formatStagedNumericValue` output format in 18-01.
4. Decide: keep the Makefile guard, replace with an AST-based Go test, or drop it.
5. Add 4KB `Value` truncation in `newIngestError` helper.

None are blocking - the phase is executable as written.

---

## Consensus Summary

Both reviewers agree the phase is well decomposed, the dependency ordering is correct, and the plans stay disciplined about the hard-failure boundary: `IngestError` for user-document hard failures, with parser-contract, tragic, and soft-mode behavior left intact. Both also see the CLI aggregation and end-to-end test strategy as strong, especially the grouped failure reporting and the success-path preservation around dense row-group packing.

### Agreed Strengths
- The 18-01 -> 18-02/18-03 -> 18-04 dependency graph is correct and supports safe staged execution.
- The plans preserve the critical error-classification boundaries and avoid scope creep into deferred areas like redaction policy, failure-code enums, and soft-mode observability.
- Compatibility with `github.com/pkg/errors` and `errors.As` is handled deliberately rather than as an afterthought.
- The CLI work is careful about deterministic reporting, bounded samples, and preserving dense packing of accepted documents.
- The phase is backed by strong verification strategy, including the behavior matrix and the 100-document fixture.

### Agreed Concerns
- Determinism needs to be pinned more explicitly in 18-03. Gemini calls out map-order sorting for failure groups; Claude calls out fixture line ordering for numeric promotion. Both point to avoidable test flake if ordering rules stay implicit.
- The Makefile guard in 18-02 is useful as a first pass but brittle as a long-term invariant. Both reviewers warn that string-matching on historical error text can miss future regressions or require noisy maintenance.
- Some value-formatting details remain under-specified. Gemini flags large/complex `fmt.Sprint` output; Claude wants `formatStagedNumericValue` clarified and notes that large `Value` strings can spill into CLI samples.

### Divergent Views
- Claude specifically flags a medium-risk gap around the `validateStagedPaths` numeric rejection site. Gemini does not call that out, so this looks like the highest-value single plan clarification before execution.
- Gemini is comfortable keeping the current Makefile guard approach if it stays pragmatic; Claude is much more skeptical and would prefer replacing or dropping it in favor of stronger behavioral or AST-based enforcement.
- Claude recommends adding an explicit size cap or truncation policy for `Value`; Gemini treats large formatted values as a low-severity edge case.
