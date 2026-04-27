---
phase: 15
reviewers: [gemini, claude]
reviewed_at: 2026-04-22T15:05:31Z
plans_reviewed: [15-01-PLAN.md, 15-02-PLAN.md, 15-03-PLAN.md]
---

# Cross-AI Plan Review — Phase 15: Experimentation CLI

## Gemini Review

# Cross-AI Plan Review

## 1. Summary
The provided plans outline a well-structured, phased approach to implementing the `experiment` CLI. They correctly isolate the stdin streaming logic, cleanly separate text and JSON reporting through a shared intermediate model, and include strict policy enforcement tests to prevent dependency bloat. The plans successfully meet the phase goals without violating the "pruning-first" and standard library constraints. However, there is a critical oversight regarding how the bounded-memory stdin spooling interacts with the `--sample` flag, alongside minor omissions in resource management for the temporary file.

## 2. Strengths
- **Architecture**: Breaking the execution into `experiment.go` (CLI logic) and `experiment_output.go` (reporting) neatly separates concerns and ensures text and JSON outputs remain perfectly aligned.
- **Executable Charters**: The addition of `experiment_policy_test.go` in Plan 15-03 is excellent. It programmatically enforces the charter (no `cobra`, `bubbletea`, or TTY detection) to prevent silent scope creep over time.
- **Robust Validation**: Building an in-process string larger than 64 KiB for `TestRunExperimentLargeLineNoTruncation` guarantees `bufio.Scanner` default limitations are avoided without relying on massive committed fixture files.
- **Observability Discipline**: Safely routing the existing logging seam to `stderr` ensures diagnostics never corrupt the `stdout` machine-readable JSON output.

## 3. Concerns
- **[HIGH] Unbounded Disk Spooling with `--sample`**: Plan 15-01 dictates spooling `stdin` to a temporary file to count lines before initializing `GINBuilder`. Plan 15-03 introduces `--sample N`. If a user pipes a massive dataset and passes `--sample 10`, the current plan implies spooling the entire stream to disk just to count lines, which defeats the purpose of a fast sample.
- **[MEDIUM] Resource Leaks**: Plan 15-01 specifies creating a temporary file for stdin spooling but omits explicit cleanup instructions. Failing to `defer os.Remove(...)` will leak disk space across successive CLI runs.
- **[LOW] Invalid JSON in Pass 1 vs. Pass 2**: Pass 1 counts raw `\n` characters, while Pass 2 parses JSON. Malformed or empty lines will cause `estimatedRGs` to be slightly larger than the actual number of ingested documents. The plan should explicitly acknowledge that `GINBuilder` safely tolerates an overallocated `numRGs` capacity.
- **[LOW] Dependency Ordering (Rework)**: `--sample N` is introduced in Plan 15-03, but it fundamentally impacts the two-pass ingest logic introduced in Plan 15-01. This will require the agent to heavily refactor the ingest loop in wave 3.

## 4. Suggestions
- **Optimize Stdin Spooling (Pass 1 Bypass)**: Update the ingest logic so that if `--sample N` is provided, Pass 1 (counting/spooling) is completely skipped. Initialize `GINBuilder` with `max(1, ceil(N / rgSize))` and stream Pass 2 directly.
- **Explicit Temp File Cleanup**: Explicitly mandate `defer os.Remove(tmpFile.Name())` and proper `.Close()` calls for the stdin temporary file in Plan 15-01.
- **Move `--sample` to Plan 15-01**: Consider moving the implementation of `--sample N` from Plan 15-03 to Plan 15-01. This allows the core streaming and sizing loop to be written correctly the first time, avoiding rework in wave 3.
- **Empty Line Handling**: Explicitly state how empty lines are handled in Plan 15-01 (they should be treated as invalid JSON or skipped, and must not increment the successfully ingested `documents` counter).

## 5. Risk Assessment
**MEDIUM**. The core logic, test strategy, and phase constraint adherence are sound. However, the performance and disk-space bottleneck introduced by the interaction between `stdin` temp-file spooling and the `--sample` flag needs to be resolved before execution begins. Adding explicit file cleanup and short-circuiting Pass 1 when `--sample` is set will lower the overall risk to LOW.

---

## Claude Review

# Phase 15 Plan Review: Experimentation CLI

## Summary

The three plans collectively achieve the Phase 15 goal cleanly: they introduce `gin-index experiment` as a JSONL-first subcommand with bounded-memory ingest, a shared report model driving both text and JSON output, an inline predicate tester, a readable sidecar write path, log-level wiring through the existing Phase 14 seam, sample/error-tolerance modes, and an executable charter guard. The plan split (foundation -> rich features -> semantics & guardrails) mirrors the research's recommended split and preserves TDD discipline. Scope is well-contained, dependencies on Phase 13/14 are correctly narrowed (`--log-level` yes, `--parser` no), and the two-pass ingest strategy is the right answer to `GINBuilder`'s upfront-`numRGs` constraint. A handful of specification gaps and edge-case ambiguities are worth tightening before execution, but none are structural.

`★ Insight ─────────────────────────────────────`
- The "report-first rendering" decision (Pattern 2) is the single most valuable call in this plan: text and JSON diverging is the canonical failure mode of CLIs like this, and routing both through one struct makes the golden test load-bearing.
- Two-pass ingest with temp-file spooling for stdin is an unusually honest choice — it accepts the disk trade-off rather than faking bounded memory with a buffered-bytes heuristic. Worth preserving across refactors.
- Executable charter tests (`experiment_policy_test.go`) are more valuable than README prose for resisting scope creep; the `--parser`-not-exposed test in particular is a nice "negative-surface" guard.
`─────────────────────────────────────────────────`

---

## Plan 15-01 — Foundation

### Strengths
- Correct minimal surface: only `--rg-size` is introduced in Wave 1; later flags layer cleanly.
- The stdin-aware runner signature `runExperiment(args, stdin, stdout, stderr)` avoids `os.Stdin` leakage and keeps tests in-process. Exactly right.
- Two-pass ingest contract is stated explicitly with the right justification (builder requires `numRGs` up front).
- Large-line test (>64 KiB) is made a first-class merge gate, which is the failure mode most likely to regress silently.
- Test file split (dedicated `experiment_test.go` rather than growing `main_test.go`) matches project conventions.

### Concerns
- **MEDIUM — Empty-line / EOF newline semantics are underspecified.** The research says "empty lines should be treated as invalid JSON input rather than silently skipped unless the line is the final trailing newline-only case." The plan references this research but doesn't lock it as acceptance criteria. A later refactor could easily change either behavior and still pass the listed tests. Recommend an explicit acceptance criterion and a test case.
- **MEDIUM — Row-group counter in the summary is ambiguous.** Summary says "Row Groups" but the plan never specifies whether this is `idx.Header.NumRowGroups` (the pre-sized capacity from `ceil(rawLineCount/rgSize)`) or the number of *actually used* RGs (`ceil(documents/rgSize)`). These diverge when `--sample` lands in Plan 3 or when continue mode skips lines. Locking this now prevents Plan 3 rework.
- **MEDIUM — "Counting pass" line-count rule not stated.** Does an empty trailing line count toward `rawLineCount`? Does a final line without `\n` count? If the counting pass and the ingest pass disagree, `estimatedRGs` can be off-by-one vs actual `rgID` values, which may trip `AddDocument` bounds in the builder.
- **LOW — Directory-input rejection referenced in research but not in acceptance criteria.** Research calls out "input path exists but is a directory: fail with a direct error." Should be an acceptance criterion or test.
- **LOW — `--rg-size` validation test name is listed in VALIDATION.md as `TestRunExperimentRejectsInvalidRGSize` but Task 2's verify command pattern doesn't include it.** Only Task 3's verify does. Minor, but worth aligning.
- **LOW — No explicit guard on extremely large `--rg-size`** (e.g., `MaxInt` overflow when computing `ceil(rawLineCount/rgSize)`). Not critical but worth a sentence.

### Suggestions
- Add an acceptance criterion to Task 2 pinning the empty-line and trailing-newline semantics, plus a test case.
- Define "Row Groups" in the summary explicitly as "allocated row-group capacity" or "used row groups" — and prefer the latter, derived from `idx.Header.NumRowGroups` after `Finalize()`, so it remains meaningful under sampling.
- Make the line-counting rule identical between pass 1 and pass 2 (same `ReadBytes('\n')` loop, same empty-line handling) so estimates never drift.

### Risk Assessment: **LOW**
Foundation plan is straightforward, reuses established patterns, and has well-chosen merge-gate tests. The ambiguities above are specification tightness, not structural risk.

---

## Plan 15-02 — Rich Features

### Strengths
- Single shared `experimentReport` model with explicit struct marshaling (not `map[string]any`) is the right call and is pinned in acceptance criteria.
- Bloom occupancy estimate formula is given literally (`1 - exp(-k*n/m)`) with explicit field sources, which removes interpretation ambiguity.
- `--log-level` uses `logging/slogadapter` with a clear rationale (preserves debug filtering, unlike `stdadapter`) — good research-to-decision traceability.
- Sidecar round-trip test verifies *pruning ratio stability*, not just file existence. That's a substantive correctness check, not a smoke test.
- JSON golden test is treated as a schema lock, not a string match.

### Concerns
- **HIGH — Sidecar path semantics conflict with `gin.ReadSidecar`.** The research notes `gin.ReadSidecar(...)` derives its path as `parquetFile + ".gin"`, but the plan's `-o out.gin` takes an arbitrary output path. The roundtrip test works around this by picking a basename like `<tmp>/experiment-artifact` and loading via `gin.ReadSidecar(<tmp>/experiment-artifact)`, which implicitly requires the output file to be named exactly `<basename>.gin`. If a user passes `-o /tmp/foo.bin`, the output is not loadable via `gin.ReadSidecar`. Either (a) validate/enforce a `.gin` suffix, (b) document that `-o` writes raw encoded bytes independent of the sidecar naming convention and recommend `gin.Decode` for reloading, or (c) accept a basename and append `.gin`. The current plan leaves this silently inconsistent.
- **MEDIUM — `pruning_ratio` divide-by-zero edge is spelled out, but `idx.Header.NumRowGroups == 0` case needs coherent JSON output.** The plan says `0` when RGs are 0 — good — but that's semantically odd (a `pruning_ratio: 0.0` with zero RGs is indistinguishable from "nothing pruned on a non-empty index"). Consider emitting `null` (requires pointer field) or adding a `row_groups` count in the predicate block so consumers can disambiguate.
- **MEDIUM — `-o` permissions policy has a subtle hole for stdin input.** Plan says "use `defaultLocalArtifactMode` when input is stdin." But what if the output path already exists with stricter permissions? `writeLocalIndexFile` behavior under pre-existing targets isn't specified here. Worth an explicit acceptance criterion.
- **MEDIUM — `--log-level debug` may emit no events today** (research calls this out), but there's no test verifying that `info` vs `debug` behavioral-equivalence is intentional rather than a bug. Add a regression-friendly note in the logger test so future log-site additions aren't accidentally suppressed.
- **LOW — Task 1's verify command references `TestRunExperimentJSONGolden` but the test is only written in Task 3.** Minor TDD ordering — the automated verify for Task 1 will fail until Task 3 lands. This is probably intentional under TDD-first, but if waves/tasks are run out of strict order it confuses.
- **LOW — `experimentPathRow` field list not fully specified.** The JSON schema in research has `bucket_count` and `representations` but the plan doesn't state where these come from. `representations` in particular needs a decision: always-empty array vs deriving from derived-companion metadata.
- **LOW — No explicit test that the JSON schema omits `predicate_test` when `--test` is absent.** Research specifies the rule; the golden test should cover both presence and absence.

### Suggestions
- Pick one sidecar story and lock it: easiest is "validate `-o` ends with `.gin`; reject otherwise," and make the roundtrip test pass the exact same path to `gin.ReadSidecar` by stripping the suffix. Document this limitation in usage text so users don't expect arbitrary names.
- Add a JSON golden test fixture for the "no predicate" case as well as the "with predicate" case.
- Nail down `representations` and `bucket_count` sources explicitly in Task 1 acceptance criteria so they don't silently become `null`/`0` in the schema.
- Add one line to the log-level test asserting that `--log-level off` produces *zero* stderr bytes attributable to the library (not just that stdout is clean).

### Risk Assessment: **MEDIUM**
The sidecar/`ReadSidecar` naming conflict is the standout issue — it's the kind of thing that ships green on test day and gets reported as a usability bug week one. Everything else is tightening.

---

## Plan 15-03 — Sample, Error Tolerance, Charter Guards

### Strengths
- Counter semantics are stated explicitly and distinguish `documents` (successful) from `processed_lines` (raw), which is the right distinction for sample mode.
- Continue mode explicitly records *counters only*, not an unbounded error list — good defense against pathological inputs.
- Policy-guard tests are exhaustive across the four real drift vectors (deps, imports, TTY logic, `--parser` exposure) and made executable rather than documented.
- Test split keeps malformed fixtures in-memory — avoids committing garbage JSONL.

### Concerns
- **MEDIUM — Interaction between `--sample N` and the two-pass counting strategy is unspecified.** Pass 1 counts `rawLineCount`; pass 2 stops early when `documents == N`. That means `estimatedRGs` is sized for the full input, but only `N` documents are ingested, leaving many empty RGs in the builder. Does `GINBuilder.Finalize()` tolerate unused pre-allocated RGs? If yes, this works but `idx.Header.NumRowGroups` will overstate the actual data. If not, we need Pass-2 to re-plan `numRGs` or to cap `estimatedRGs` at `ceil(N/rgSize)` when `--sample` is set. Needs an explicit decision.
- **MEDIUM — `--on-error continue` + malformed-line counting in pass 1.** Pass 1 (line counting) doesn't validate JSON — it just counts newlines. Pass 2 discovers the malformed line. So the same line is counted in `rawLineCount` but skipped during ingest. This is fine for RG estimation (you end up with fewer used RGs than allocated), but it compounds with the sample-mode concern above. Worth a single sentence in the plan locking "estimatedRGs is an upper bound; empty RGs are tolerated."
- **MEDIUM — Policy test forbidden-list maintenance.** The list (`cobra`, `urfave/cli`, `bubbletea`, `lipgloss`, `fatih/color`, `chzyer/readline`, `isatty`, `term.IsTerminal`) is enumerated. This is fine for Phase 15 but will rot — a future `spf13/pflag` dependency or `mattn/go-isatty` would slip past. Either acknowledge this as "best-effort static deny-list" or broaden the check (e.g., grep for any `NoColor`/`ANSI`/`ESC\\[` literal).
- **MEDIUM — Parser-flag guard grep may produce false positives.** "Assert usage/help strings do not mention `--parser`" — if the help text mentions anything like "parse error" or "JSON parser" in passing, the grep needs to be anchored. Recommend matching `\\b--parser\\b` or the flag registration pattern rather than the substring.
- **LOW — No test for the "empty input" edge case** called out in research ("still print a coherent zero-document report rather than panic in `Finalize()`"). Worth one test.
- **LOW — No test for malformed `--on-error` value rejection**, only for the two valid values. Quick addition.
- **LOW — Counter surfacing in text mode is not pinned to a field order.** Research says this is the-agent's-discretion territory, but a text golden-ish assertion would prevent accidental rearrangement.

### Suggestions
- Add an explicit acceptance criterion: "when `--sample N` is set, `estimatedRGs` in pass 2 is capped at `max(1, ceil(N/rgSize))` so allocated RGs track actual ingest." Or explicitly document that over-allocation is acceptable and verify `Finalize()` tolerates it with a test.
- Broaden the TTY-logic policy test to grep for ANSI escape literals (`\\x1b[`, `\\033[`) as an extra line of defense against colour creep.
- Anchor the `--parser` assertion with a regex boundary check.
- Add a `TestRunExperimentEmptyInput` test covering zero-line input.
- Consider adding `TestRunExperimentRejectsInvalidOnErrorValue` explicitly.

### Risk Assessment: **MEDIUM**
Sample/error-tolerance interaction with the two-pass strategy is the most likely place for subtle correctness bugs. The policy guards are good but brittle to future dependency churn — worth planning for maintenance.

---

## Cross-Plan Observations

### Dependency Ordering
Clean. Plan 01 delivers only foundation flags; Plan 02 extends with features; Plan 03 adds semantics and guardrails. Each plan's `depends_on` is correctly declared, and Plan 03's tests don't presume Plan 02 tests are absent. No circular or latent dependencies.

### Scope Discipline
Very good. D-13 (`--log-level` in scope) and D-14 (`--parser` deferred) are both honored throughout, and the charter test in Plan 03 makes the latter enforceable. The deferred items (Phase 999.6 messaging, `--parser` exposure) are correctly held out.

### Security Posture
STRIDE registers are reasonable for a CLI of this shape. Key gaps:
- No mention of **symlink or TOCTOU handling on `-o`** — `writeLocalIndexFile` presumably owns this, but Phase 15 should inherit the assumption explicitly.
- **Predicate string injection into logs** is correctly banned (T-15-04), but there's no positive test asserting the frozen attr vocabulary from Phase 14 is respected when `--log-level info|debug` is on with a malicious-looking predicate.
- **Temp-file cleanup on stdin path** is not explicitly required in Plan 01. If the command panics or early-exits between pass 1 and pass 2, the temp file leaks into `TMPDIR`. Add a `defer os.Remove(...)` expectation.

### Performance
Not a Phase 15 concern per the charter, and the plan correctly avoids performance-gated merge criteria. Two-pass ingest doubles I/O cost vs a one-pass ingest, but that's a locked trade-off and the research justifies it.

### Achievement of Phase Goals
The six Phase 15 success criteria from ROADMAP.md are each mapped to at least one plan task and one automated test. Coverage check:
1. File path + stdin summary — ✅ Plan 01
2. >64 KiB line ingest — ✅ Plan 01 (merge gate)
3. `--test` + `--json` — ✅ Plan 02
4. `-o` sidecar roundtrip — ✅ Plan 02 (with caveat above)
5. `--sample` + `--on-error` — ✅ Plan 03
6. No new deps + charter guard — ✅ Plan 03

---

## Overall Risk Assessment: **LOW-MEDIUM**

**Justification:** The plan split is sound, scope is tight, and the hardest correctness question (two-pass ingest for stdin) is answered explicitly rather than papered over. The residual risk sits in three specific spots: the sidecar path/`ReadSidecar` naming mismatch (Plan 02), the `--sample`/`estimatedRGs` interaction (Plan 03), and a handful of under-specified edge cases around empty lines, trailing newlines, and empty input. Each is a one-paragraph tightening, not a replan. Once these are pinned in acceptance criteria, this is a low-risk execution phase.

### Recommended Pre-Execution Tightening (priority order)
1. **Resolve sidecar naming** in Plan 02 — validate `-o` suffix or document a non-`ReadSidecar` reload path.
2. **Lock `--sample` × `estimatedRGs` behavior** in Plan 03 — cap at `ceil(N/rgSize)` or document over-allocation tolerance.
3. **Define "Row Groups" summary field** in Plan 01 — used vs allocated, and how it reports under sampling.
4. **Pin empty-line / trailing-newline semantics** in Plan 01 acceptance criteria, with a test.
5. **Add empty-input and invalid-`--on-error` tests** in Plan 03.
6. **Anchor the `--parser` policy grep** in Plan 03 to avoid false positives.

---

## Consensus Summary

The two reviewers agree the overall plan shape is strong: the work is split sensibly across foundation, rich-output, and semantics/guardrail waves; the shared report model is the right core abstraction; and the Phase 15 scope discipline is intact. The disagreement is not about direction, but about specification tightness around a few edge cases that should be resolved before execution.

### Agreed Strengths

- **Wave split and dependency ordering are sound.** Both reviewers treat the staged progression from ingest foundation to richer output to final CLI semantics as the right execution shape.
- **One shared report model is the right design anchor.** Gemini called out the clean separation between command logic and reporting; Claude explicitly highlighted "report-first rendering" as the most valuable decision in the phase.
- **The bounded-memory/two-pass stdin strategy is the right baseline.** Gemini accepted the architecture but wants the `--sample` interaction tightened; Claude explicitly called the two-pass approach the right answer to the builder's upfront row-group requirement.
- **Guardrails are strong.** Both reviewers praised the executable charter tests and the stderr-only observability design.

### Agreed Concerns

- **`--sample` needs a locked interaction with two-pass sizing.**
  - Gemini: treat full stdin spooling under `--sample N` as the main unresolved risk and consider bypassing pass 1.
  - Claude: keep the two-pass design, but explicitly cap or define `estimatedRGs` under sampling, or document that over-allocation is tolerated.
  - Action: choose one behavior and pin it in acceptance criteria and tests.
- **Temp-file lifecycle and sizing semantics need to be explicit.**
  - Gemini flagged missing cleanup for the stdin spool file.
  - Claude independently flagged temp-file cleanup and the need to document when pass-1 line counts are only an upper bound.
  - Action: require `defer os.Remove(...)`, explicit close behavior, and a statement about tolerated over-allocation if that remains the design.
- **Edge-case semantics are still under-specified.**
  - Claude raised empty-line, trailing-newline, empty-input, invalid-`--on-error`, and row-group-summary ambiguity issues.
  - Gemini also called out empty-line handling and phase ordering/rework risk.
  - Action: promote these from implied research notes into acceptance criteria and concrete tests.

### Divergent Views

- **How to handle `--sample` with stdin.**
  - Gemini recommends short-circuiting pass 1 entirely when `--sample` is set so the command can behave like a true fast sample on large piped input.
  - Claude is comfortable preserving the two-pass design if the allocated-vs-used RG behavior is defined precisely.
- **Sidecar naming/loadability.**
  - Claude raised a specific mismatch between arbitrary `-o` paths and `gin.ReadSidecar(...)` expectations.
  - Gemini did not surface this issue.
  - Action: verify whether Phase 15 wants arbitrary encoded output or a strict `.gin` sidecar contract, then encode that in the plan and tests.

### Recommended Next Action

Iterate the plans with:

```bash
$gsd-plan-phase 15 --reviews
```

Before execution, tighten these items in the plan text:

1. Lock `--sample` versus two-pass sizing behavior.
2. Require temp-file cleanup and define over-allocation semantics.
3. Resolve the `-o`/`gin.ReadSidecar` naming contract.
4. Add explicit tests for empty input, empty/trailing lines, and invalid `--on-error`.
5. Define what the summary's "row groups" count actually reports under sampling and skipped-line scenarios.
