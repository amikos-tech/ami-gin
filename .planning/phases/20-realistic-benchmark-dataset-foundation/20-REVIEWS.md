---
phase: 20
reviewers: [claude, opencode]
successful_reviewers: [opencode]
reviewed_at: 2026-07-21T07:55:02Z
plans_reviewed: [20-01-PLAN.md, 20-02-PLAN.md]
---

# Cross-AI Plan Review — Phase 20

## Claude Review

Claude CLI was invoked in a separate session but returned `Execution error` without a review. It is recorded here for transparency and was not used in the synthesis.

---

## OpenCode Review

## Summary

The plans are thorough and strongly aligned with Phase 20’s stated goal: create offline, provenance-documented realistic JSON fixtures and wire them into benchmark infrastructure without touching SIMD parser behavior. The requirements DATA-01, DATA-02, and DATA-03 are covered with clear traceability, explicit non-goals, and good verification commands. The main risk is over-specification: the plans prescribe a large amount of fixture shape, README parsing, subprocess benchmark validation, and exact byte synchronization, which may make execution brittle for a data-foundation phase whose value is mostly “small realistic corpus plus offline benchmark path.”

## Strengths

- Clear separation between Plan 01 and Plan 02: fixture contract first, benchmark/documentation second.
- Strong protection against scope creep: no downloader, no SIMD parser work, no CI changes, no Phase-13 golden refresh, no parser selection changes.
- Good offline-default posture: checked-in smoke fixtures are the source of truth, external data is local-only and explicitly gated.
- Good provenance handling: synthesized fixtures avoid upstream redistribution obligations while documenting future LICENSE/NOTICE review requirements.
- The combined fixture interleaving contract is precise and useful for detecting accidental concatenation.
- Numeric lexeme preservation is correctly called out, which matters for later SIMD exact-number validation.
- Benchmark leaves cover build, encode, and query, which gives Phase 22 a reusable comparison surface.
- External benchmark registration requirements are well thought out: disabled leaves should visibly skip rather than silently not exist.
- Verification commands are concrete and mostly map directly to acceptance criteria.

## Concerns

- **MEDIUM:** Plan 01 may be too brittle by requiring exact byte counts to be duplicated across descriptors, README, and test constants. Exact bytes are useful for drift detection, but they can make harmless fixture edits noisy and increase maintenance overhead.
- **MEDIUM:** Requiring `TestPhase20SmokeFixtures` to parse the README Markdown table makes prose documentation part of the executable contract. This is clever but brittle: formatting changes to documentation can break tests even if the fixture contract remains valid.
- **MEDIUM:** The fixture creation task is highly prescriptive but provides no practical mechanism for generating 96 + 96 + 96 + 288 deterministic records. Hand-authoring 576 JSONL lines is error-prone; forbidding a generator entirely may increase risk of malformed or inconsistent data.
- **MEDIUM:** Plan 02’s subprocess benchmark assertions inside `TestPhase20ExternalTier` may be expensive and flaky. Running `go test` from inside `go test` can be slow, environment-sensitive, and awkward under race/coverage/custom test flags.
- **MEDIUM:** The external benchmark aggregate over arbitrary local `*.json`, `*.jsonl`, and `*.ndjson` files may produce inconsistent query behavior unless the deterministic query is guaranteed to match the temporary/local aggregate. The plan says “same deterministic nonzero-candidate Query actions,” but external examples may not contain the smoke fixture’s expected fields.
- **LOW:** The benchmark naming pattern uses slash-separated sub-benchmarks with regex selectors. This is fine, but the README command’s external regex should be verified carefully because Go benchmark regex matching splits slash components in non-obvious ways.
- **LOW:** The plan requires errors to use `github.com/pkg/errors`, but tests and benchmark helpers may already use mixed standard errors. This is consistent with repo guidance, but implementation should avoid churn outside new helpers.
- **LOW:** The “no environment reads” requirement for smoke benchmarks is good, but testing that absence mechanically is hard. The plan relies mostly on implementation discipline.
- **LOW:** The number-heavy fixture says not to include unsupported out-of-range literals, but also includes `9007199254740993`. That value is valid JSON and int64-safe, but implementers need to ensure the current parser/index path accepts it as intended.

## Suggestions

- Relax the README parsing requirement. Prefer keeping exact fixture metadata in Go descriptors and requiring the README to contain the policy markers plus fixture paths. If Markdown table parsing stays, keep the parser intentionally tiny and tolerant of whitespace.
- Allow a small deterministic generator under `testdata/phase20/` or `internal/testdata` if it is used only to refresh committed fixtures. The committed JSONL can remain source of truth, but a generator would reduce manual drift for the 288-line interleaving contract.
- For external benchmark query behavior, use a query guaranteed by the temporary test fixture for automated tests, and allow the real local external tier to skip the Query action if no deterministic candidate can be constructed from discovered data. Alternatively, build a trivial query from the first loaded document’s observed scalar field.
- Avoid subprocess benchmark tests unless necessary. A helper-level test can prove gate behavior, discovery order, invalid input errors, and benchmark action functions. If subprocess tests remain, isolate them behind the narrowest selector and ensure they cannot recurse unexpectedly.
- Consider caps plus row counts as hard test gates, but treat exact byte counts as documentation/review gates unless there is strong precedent that fixture bytes must be immutable.
- Make the external discovery helper reject symlinks only if that matters. If not, explicitly state that local paths are trusted developer inputs to avoid over-building a security boundary.
- Add a short assertion that fixture files do not contain blank lines if the contract is “every nonblank line is valid JSON.” Skipping blanks is fine for external inputs, but committed fixtures should probably reject blanks.
- Ensure `combined.jsonl` is byte-preserving from focused files by comparing raw line bytes, not only decoded `phase20_source_kind` sequence.

## Risk Assessment

**Overall risk: MEDIUM.**

The plans are correct in direction and cover the phase goals well. The risk is not from missing requirements or unsafe behavior; it is mainly execution complexity. The fixture and benchmark contracts are detailed enough that implementation could become brittle, especially around exact byte synchronization, README table parsing, hand-authored large JSONL files, and subprocess benchmark tests. If those are simplified slightly while preserving offline fixtures, provenance, shape coverage, and smoke benchmark behavior, the phase risk drops to LOW.

---

## Consensus Summary

Only OpenCode produced a usable review; the points below are therefore a single-reviewer synthesis, not cross-reviewer consensus.

### Agreed Strengths

- The two-plan ordering is sound: establish the committed, governed fixtures before adding benchmark and documentation plumbing.
- Offline-by-default, local opt-in external data, and explicit non-goals preserve Phase 20’s intended boundary.
- The Build, Encode, and Query benchmark actions give Phase 22 a practical reuse path.

### Agreed Concerns

- Simplify brittle verification where possible: avoid coupling tests to exact Markdown formatting and duplicate byte counts.
- Confirm the external query action is data-driven or otherwise guaranteed to match arbitrary opted-in local inputs.
- Keep subprocess benchmark tests narrowly scoped; they are the highest likely source of test slowness or flakiness.

### Divergent Views

- No comparison is possible because Claude did not return a review.
