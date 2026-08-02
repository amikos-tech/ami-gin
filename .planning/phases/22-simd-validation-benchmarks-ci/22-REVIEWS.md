---
phase: 22
reviewers: [codex, opencode]
reviewed_at: 2026-07-31T16:53:16Z
plans_reviewed: [22-01-PLAN.md, 22-02-PLAN.md, 22-03-PLAN.md, 22-04-PLAN.md, 22-05-PLAN.md, 22-06-PLAN.md, 22-07-PLAN.md, 22-08-PLAN.md]
---

# Cross-AI Plan Review — Phase 22: SIMD Validation, Benchmarks & CI

Reviewers were given PROJECT.md context, the ROADMAP phase section, requirements SIMD-08..11,
22-CONTEXT.md, 22-VALIDATION.md, 22-RESEARCH.md, 22-PATTERNS.md, and all eight plans.

| Reviewer | Model | Repo access | Overall risk |
|---|---|---|---|
| Codex | CLI default | yes — read source directly | HIGH until 5 blockers fixed, then MEDIUM-LOW |
| OpenCode | gpt-5.5 | plan text only | MEDIUM |

---

## Codex Review

# Overall verdict

The phase is technically strong and covers SIMD-08 through SIMD-11 well, but it is not execution-ready yet. The parity design, operational boundaries, benchmark methodology, and evidence-first shipping rule are all sound. Four issues need correction first: Plan 22-01’s checkpoint semantics, the stale validation map, Plan 22-03’s fuzz outcome classification, and Plan 22-07’s impossible workflow-test scope. Plan 22-08 also needs an explicit route for creating the remote state it expects.

Overall risk: **HIGH until those blockers are corrected; MEDIUM-LOW afterward.**

---

## [Plan 22-01 — Provenance gates](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/22-simd-validation-benchmarks-ci/22-01-PLAN.md:1)

### Summary

Good provenance policy, but the checkpoint representation does not reliably enforce the required before-execution decision.

### Strengths

- Separates the already-pinned native dependency from the ephemeral benchstat tool.
- Prevents unpinned substitution or accidental `go.mod` additions.
- Records explicit human approval and preserves upstream ownership boundaries.
- Correctly distinguishes `[SUS]` from `[SLOP]`.

### Concerns

- **[HIGH]** Both tasks use `checkpoint:human-verify` with nonstandard `gate="blocking-human"` at [line 52](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/22-simd-validation-benchmarks-ci/22-01-PLAN.md:52). GSD’s default is end-of-phase verification, while these approvals must happen before execution. `checkpoint:decision gate="blocking"` is the appropriate contract.
- **[MEDIUM]** “Before any tagged SIMD test or module download” is historically impossible: the dependency and tagged CI already exist. The guarantee should be scoped to new Phase 22 evidence commands.
- **[LOW]** The automated checks prove pins exist, but do not prove `go.mod` and `go.sum` remained byte-identical.

### Suggestions

- Convert both tasks to blocking decision checkpoints with explicit approve/reject outcomes.
- Use the standard `gate="blocking"`.
- Reword the boundary to “before Phase 22 tagged evidence or benchstat execution.”
- Record pre/post hashes or use `git diff --exit-code -- go.mod go.sum`.

### Risk Assessment

**HIGH** — the intended supply-chain gate may not function as a true prerequisite.

---

## [Plan 22-02 — Parity hard gate](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/22-simd-validation-benchmarks-ci/22-02-PLAN.md:1)

### Summary

This is the strongest plan in the set. It reuses the right helpers, avoids unnecessary goldens, and directly satisfies the core correctness requirement.

### Strengths

- Preserves authored goldens while using a live oracle for generated Phase 20 fixtures.
- Compares both parsers in one process with identical configuration.
- Covers encoded bytes, expected Evaluate results, and cross-parser query parity.
- Keeps malformed failure-layer attribution explicit.
- Preserves default-build isolation and production behavior.
- Correctly treats any in-scope byte or query difference as a hard stop.

### Concerns

- **[MEDIUM]** The three-state constructor policy is primarily exercised through a successful local construction. There are no deterministic tests for supported-local failure, required-CI fatal behavior, unsupported-platform skip, or every sentinel classification.
- **[LOW]** `assertByteIdentical` retains “golden” terminology when comparing SIMD against live stdlib bytes, which may make failures slightly misleading.

### Suggestions

- Extract pure helpers such as `supportedSIMDPlatform` and `classifySIMDLoadError`.
- Add table-driven tests for all five supported pairs, representative unsupported pairs, all six sentinels, and unknown errors.
- Inject the construction result into a small policy helper so skip/fatal behavior can be tested without breaking the native installation.

### Risk Assessment

**MEDIUM** — parity coverage is excellent; only the CI failure-policy branches are under-tested.

---

## [Plan 22-03 — Differential fuzzing](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/22-simd-validation-benchmarks-ci/22-03-PLAN.md:1)

### Summary

The resource bounds and parser lifecycle are well designed, but the proposed three-way classification does not clearly measure whether a document actually ingested.

### Strengths

- Enforces the empirically justified 4096-byte and depth-8 bounds before both arms.
- Reuses one parser sequentially with proper cleanup.
- Uses the standard Go fuzz corpus instead of custom infrastructure.
- Keeps timed fuzzing out of CI.
- Adds direct tests that over-limit inputs never reach either arm.
- Avoids absorbing the exponential nested-array problem into this phase.

### Concerns

- **[MEDIUM]** Both failure modes are configured soft at [line 126](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/22-simd-validation-benchmarks-ci/22-03-PLAN.md:126). A soft-skipped document can still produce a non-nil encoded empty index, so “encoded bytes exist” is not equivalent to “the document ingested.”
- **[MEDIUM]** The dedicated `1e400 garbage` seed will not necessarily exercise the exactly-one-ingests branch under both-soft configuration. The known asymmetry is exposed by hard parser/soft numeric policy in the existing test.
- **[MEDIUM]** Corpus verification only checks file count and headers at [line 93](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/22-simd-validation-benchmarks-ci/22-03-PLAN.md:93). An over-limit or malformed corpus entry could silently return before useful coverage.
- **[LOW]** `t.Logf` is not a durable record during ordinary successful CI runs.

### Suggestions

- Define “ingested” from actual commit state or explicit `AddDocument` outcome, not encoded-output existence.
- Unit-test the three-way classifier with synthetic outcomes independently of the fuzz engine.
- Either run the known exclusion under hard-parser/soft-numeric policy or state that the existing integration test—not the fuzz seed—pins attribution.
- Add a corpus-validation test that decodes every corpus entry, enforces bounds, and confirms the five expected source categories.

### Risk Assessment

**MEDIUM** — core parity remains protected by Plan 22-02, but the fuzz plan currently overstates what its outcome branches prove.

---

## [Plan 22-04 — Executable guidance](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/22-simd-validation-benchmarks-ci/22-04-PLAN.md:1)

### Summary

A careful and useful documentation plan with good drift prevention. Its main risk is complexity in the guard implementation.

### Strengths

- Corrects the unpinned upstream link.
- Preserves the four-variable allowlist.
- Compile-checks the consumer API without loading native code.
- Protects guide path, README/CHANGELOG links, release copy, and example synchronization.
- Clearly preserves the operator’s responsibility for explicit-path provenance.
- Avoids adding a Markdown or YAML dependency.

### Concerns

- **[MEDIUM]** Implementing the same contract in both Make shell logic and mutation-driven Go tests risks two sources of truth.
- **[MEDIUM]** A plain relative `docs/simd-deployment.md` link in generated release notes may resolve incorrectly from a release page. The plan guards text presence, not actual release-page resolution.
- **[LOW]** The guide will describe the five-platform tier policy before this repository’s remote matrix has completed. It should distinguish intended support policy from completed Phase 22 evidence until Plan 22-08.

### Suggestions

- Make `check-simd-docs` invoke one authoritative Go contract test rather than duplicating parsing logic in Make.
- Use a tag-aware absolute release link based on `{{ .Tag }}`, or another release-page-safe form.
- Phrase platform text as policy pending Phase 22 verification until the final approval is recorded.

### Risk Assessment

**LOW-MEDIUM** — technically complete, with manageable maintainability and release-link concerns.

---

## [Plan 22-05 — Paired benchmark](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/22-simd-validation-benchmarks-ci/22-05-PLAN.md:1)

### Summary

The benchmark measures the right work with the right metrics, but needs tighter wording around external-tier skipping and warm parser state.

### Strengths

- Uses one tagged process and a benchstat-compatible `parser=` dimension.
- Measures realistic Phase 20 fixtures.
- Keeps I/O and parser construction outside timing.
- Reports CPU, B/op, allocs/op, and input throughput.
- Keeps external data strictly opt-in.
- Anchors the Make target to one benchmark.

### Concerns

- **[MEDIUM]** The plan should explicitly place external discovery inside a dedicated `b.Run`. Calling `phase20ExternalBenchmarkFixture(b)` at the top benchmark level would skip the parent rather than only the external tier.
- **[MEDIUM]** One SIMD parser is reused across all fixtures. Native buffer growth from an earlier fixture may benefit later fixtures, while stdlib has no equivalent persistent state. That can bias per-fixture allocations and timing.
- **[LOW]** The benchmark semantics do not explicitly say whether the intended result is cold, warm, or steady-state ingest.

### Suggestions

- Specify the structure as `tier=external/fixture=local-example` parent followed by parser children, so only that subtree skips.
- Either create one SIMD parser per fixture outside timing or deliberately warm both arms and document that the benchmark is steady-state.
- Record parser reuse/warm-up semantics in the benchmark results document.

### Risk Assessment

**MEDIUM** — the measurement surface is good, but ambiguous state reuse could weaken the performance conclusion.

---

## [Plan 22-06 — Controlled evidence](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/22-simd-validation-benchmarks-ci/22-06-PLAN.md:1)

### Summary

The raw-results/report split is excellent, but reproducibility needs stronger environmental and artifact-integrity gates.

### Strengths

- Rechecks correctness before performance.
- Uses one paired output and one exact benchstat invocation.
- Separates raw evidence from interpretation.
- Records revision, toolchain, machine, fixtures, samples, and metric meanings.
- Makes correctness override performance.
- Requires exactly one ship/defer/narrow recommendation.
- Avoids a noisy regression threshold.

### Concerns

- **[MEDIUM]** The capture command does not explicitly clear `GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL` and `GIN_PHASE20_SIMDJSON_DIR`. Inherited local values could contaminate the authoritative smoke-only record.
- **[MEDIUM]** Recording a dirty worktree is weaker than requiring a clean one. A revision plus “dirty” does not identify the code measured.
- **[MEDIUM]** Automated verification checks only for keywords at [line 82](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/22-simd-validation-benchmarks-ci/22-06-PLAN.md:82), not four fixtures, both parser arms, ten samples, or absence of external results.
- **[LOW]** Power mode, thermal state, and other machine-load controls are not stated, despite the controlled run being authoritative.

### Suggestions

- Explicitly unset both external-tier variables for the capture.
- Require a clean worktree, or record a patch hash/diff artifact when dirty.
- Add an evidence validator checking fixture/arm/sample cardinality and rejecting `tier=external`.
- Record power source/mode and confirm no concurrent benchmark or Go process.

### Risk Assessment

**MEDIUM** — methodology is sound, but a contaminated or untraceable run could still pass the current artifact checks.

---

## [Plan 22-07 — CI matrix and artifacts](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/22-simd-validation-benchmarks-ci/22-07-PLAN.md:1)

### Summary

The operational policy is comprehensive and closely matches the locked decisions, but one acceptance requirement is internally impossible as written.

### Strengths

- Encodes the exact five runners and tier policy.
- Keeps race coverage on required legs only.
- Prevents supported-platform construction failures from silently skipping.
- Uses exact version/platform cache keys without restore prefixes.
- Proves the explicit path using the fetched cache artifact.
- Keeps benchmark trend data off PRs and non-gating.
- Restricts uploads to text and retains least-privilege permissions.
- Adds executable workflow regression coverage.

### Concerns

- **[HIGH]** Task 1 says the contract test “reads only the `simd` section” yet also asserts top-level `permissions: contents: read` and preservation of other jobs at [lines 99–108](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/22-simd-validation-benchmarks-ci/22-07-PLAN.md:99). Those facts do not exist inside `ciJobSection(..., "simd")`.
- **[MEDIUM]** The final security gate scans only the default build. No plan runs `govulncheck` against the `simdjson` build-tagged Go path.
- **[LOW]** Exact string-based workflow checks are deliberately brittle. They protect layout as well as semantics, so harmless YAML refactoring may fail the guard.
- **[LOW]** Running the complete tagged suite twice on every platform materially increases CI time. This may be acceptable, but the explicit-path pass only needs enough coverage to prove loading plus representative parsing.

### Suggestions

- Read the complete workflow for top-level permissions and job-presence assertions; use `ciJobSection` only for job-local contracts.
- Add `govulncheck -tags simdjson ./...` or an equivalent tagged security step.
- Separate semantic assertions from formatting assertions in the workflow contract.
- If D-17 permits, use a focused explicit-path construction/parity smoke instead of duplicating the full suite.

### Risk Assessment

**MEDIUM-HIGH** — the CI architecture is strong, but the current contract-test scope cannot satisfy its own acceptance criteria.

---

## [Plan 22-08 — External and human approval](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/22-simd-validation-benchmarks-ci/22-08-PLAN.md:1)

### Summary

This is the right final gate, but it assumes a completed PR run and configured repository rules without including the action that creates that external state.

### Strengths

- Runs a full automated gate before human judgment.
- Separates source-controlled workflow policy from external merge-rule binding.
- Reviews actual logs rather than trusting YAML alone.
- Keeps repository-rule inspection read-only.
- Ensures controlled evidence outranks noisy CI trends.
- Routes evidence corrections back to the producing plan.

### Concerns

- **[HIGH]** Task 2 requires a completed PR matrix after the workflow names have landed at [line 97](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/22-simd-validation-benchmarks-ci/22-08-PLAN.md:97), but no plan pushes the branch, opens/updates a PR, or otherwise creates that run.
- **[MEDIUM]** The required-check wording can be read as “the repository has exactly two required checks,” which would incorrectly reject unrelated required checks such as lint or tests. It should mean exactly two required checks among the SIMD matrix names.
- **[MEDIUM]** `make security-scan` still scans only the default tagged surface.
- **[LOW]** The plan has two separate human-verification checkpoints even though the default GSD mode batches verification at phase end. A single consolidated approval would be simpler.

### Suggestions

- Add an explicit `checkpoint:human-action` prerequisite for pushing/updating the PR when no completed run exists, or document the completed run as a hard precondition before Plan 22-08 starts.
- Clarify that only the two required-tier SIMD contexts—and none of the three advisory SIMD contexts—must be required, without constraining unrelated checks.
- Include tagged vulnerability scanning.
- Consolidate the remote-policy and benchmark-evidence review into one end-of-phase human checkpoint if the workflow remains in default verification mode.

### Risk Assessment

**HIGH** — the plan can dead-end waiting for external state that no preceding task creates.

---

# Cross-plan concerns

- **[HIGH] Checkpoint contract:** Correct Plan 22-01 before execution.
- **[MEDIUM] Validation drift:** The [per-task validation map](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/22-simd-validation-benchmarks-ci/22-VALIDATION.md:49) still maps parity/fuzz to Plan 01, benchmark evidence to Plan 02, docs to Plan 03, and CI to Plan 04. Those no longer match Plans 22-01 through 22-07, so the `nyquist_compliant: true` claim is not currently trustworthy.
- **[MEDIUM] Tagged security coverage:** Default `govulncheck ./...` does not validate the optional tagged Go call graph.
- **[LOW] Requirement attribution:** Plan 22-01 claims all four requirements despite producing only prerequisite approval. This inflates requirement coverage rather than documenting direct delivery.

# Priority corrections

1. Replace Plan 22-01’s checkpoints with standard blocking decisions.
2. Update `22-VALIDATION.md` to the actual plan, wave, and task mapping.
3. Fix Plan 22-03’s definition and testing of “ingested” versus “soft-skipped.”
4. Fix Plan 22-07’s workflow-test scope contradiction.
5. Add the missing external-run creation/precondition to Plan 22-08.
6. Harden benchmark capture by clearing external-tier variables and requiring reproducible source state.

No files were modified during this review.

---

## OpenCode Review

## Summary

The plan set is unusually thorough and mostly well-aligned with Phase 22’s purpose: turn an already-working SIMD path into enforceable correctness, benchmark, CI, and documentation evidence without changing runtime behavior. The strongest parts are the explicit correctness-first sequencing, the hard distinction between controlled benchmark evidence and noisy CI trend data, and the use of executable guards instead of prose-only guidance. Overall risk is **MEDIUM**, not because the technical design is weak, but because the execution surface is large: many test helpers, CI workflow assertions, docs guards, fuzz corpus files, benchmark evidence docs, and manual checkpoints must all line up exactly.

## Strengths

- Correctness is properly gated before benchmarks or shipping recommendations.
- The D-04 parity scope is consistently preserved across tests, docs, benchmark report, and final approval.
- The plan avoids reimplementing upstream native download/checksum/cache/ABI behavior.
- Same-process stdlib/SIMD comparison is the right oracle given both parsers coexist under `-tags simdjson`.
- The three-state SIMD construction policy directly addresses silent CI skips.
- The fuzz plan incorporates the Spike 002 CPU-exhaustion finding instead of rediscovering it.
- The benchmark design is sound: paired arms, one output file, `parser=` dimension, setup outside timing, `benchstat -col /parser`.
- The CI matrix encodes the required/advisory policy cleanly and avoids over-gating flaky/legacy platforms.
- Documentation drift is treated as executable contract, which fits the project’s existing guard pattern.
- Manual checkpoints are correctly used for things source code cannot prove: package provenance, external ruleset binding, and performance interpretation.

## Concerns

- **HIGH: Plan ordering in the roadmap list and plan dependencies are confusing.** The roadmap says Wave 4 is `22-04`, Wave 5 `22-05`, Wave 6 `22-07`, Wave 7 `22-06`, Wave 8 `22-08`, which matches dependencies, but the “Plans: 0/8” list names `22-07` before `22-06`. This is technically encoded in dependencies, but easy for an executor to misread.

- **HIGH: `22-01` blocks on human approval before “any tagged SIMD evidence command runs,” but later plans require reading summaries that do not yet exist until execution.** This is intentional, but any autonomous executor must handle missing `22-01-SUMMARY.md` as a hard stop, not as a file-read failure to work around.

- **HIGH: Plan 22-04 requires parity/fuzz gates green before changing public guidance, but its verification only checks docs/example, not the parity/fuzz commands.** It reads `22-03-SUMMARY.md`, but the plan’s own automated checks do not re-run the prior gates. That creates a stale-summary risk if files changed after `22-03`.

- **HIGH: CI workflow contract tests are dependency-free string assertions, which are brittle against harmless YAML formatting changes.** This is consistent with existing patterns, but the scope of `simd_workflow_contract_test.go` is much larger than the NOTICE guard. It may become a maintenance burden or fail on semantically equivalent YAML.

- **HIGH: Plan 22-07’s explicit-path cache path may be wrong if upstream cache layout differs from the plan’s assumed `<cache>/<version>/<goos>-<goarch>/<library>` shape.** The plan says “effective version already includes the cache directory's `v` prefix,” but `go list -m` returns versions like `v0.1.7`; any upstream layout using asset labels or normalized names differently will break all matrix legs.

- **MEDIUM: The fuzz corpus task asks for exact file names and exactly five seeds, which may be over-constrained.** Five seeds is fine for day one, but making the exact count an acceptance criterion can make future minimization/promotion awkward unless later work edits the plan.

- **MEDIUM: The “exactly-one-ingests records the D-04 exclusion” language risks accepting asymmetries beyond the known malformed trailing-number attribution case.** D-02c says record, do not fail, but the plan should ensure logs clearly distinguish the known policy asymmetry from unexpected one-sided ingestion cases so they are visible during review.

- **MEDIUM: Benchmark Plan 22-05 says external tier uses `phase20ExternalBenchmarkFixture`, while Plan 22-06 says committed evidence must not enable external tier.** This is acceptable, but the benchmark target will include an external sub-benchmark that skips by default. Make sure skipped external sub-benchmarks do not clutter or confuse the `benchstat` output used for smoke evidence.

- **MEDIUM: Plan 22-06 asks for `COUNT=10 BENCHTIME=1s`, which may be expensive and noisy on developer hardware.** It is appropriate for controlled evidence, but the plan should explicitly allow aborting and recording failure if thermal throttling, power state, or background load invalidates the run.

- **MEDIUM: `go test -tags simdjson -run '^ExampleNewSIMDParser$' .` may not compile only that example in the way intended if the example has no output and is not discovered/run as expected.** Go compiles all tests in the package regardless of `-run`, so the compile check works, but the command name may give false confidence that the example body itself is executed. The plan correctly says no native construction should execute.

- **MEDIUM: Plan 22-07 adds `workflow_dispatch` to the whole workflow, not just the benchmark job.** That may be intended, but it means all jobs can be manually triggered unless guarded. If only trend artifacts need dispatch, the workflow-level trigger is still the only valid GitHub Actions mechanism, but this should be acknowledged.

- **LOW: The plans repeatedly require no version literals, but evidence docs necessarily record concrete versions.** This is fine, but distinguish “no hardcoded version in guard/workflow logic” from “record effective version in benchmark evidence.”

- **LOW: `make security-scan` in Plan 22-08 may depend on tools or network outside the phase.** If security scanning is already standard, this is acceptable; otherwise final verification may fail for environmental reasons unrelated to SIMD.

- **LOW: Several tasks mention creating summary files but no plan defines their exact structure beyond the template reference.** Probably fine in GSD context, but summaries are used as dependency evidence, so they should record exact commands and outcomes consistently.

## Suggestions

- Add a short dependency banner to each plan summary: “Do not execute unless these summary files exist and say approved/green.”
- In Plan 22-04, add a verification step that re-runs the focused parity/fuzz command or explicitly checks current git diff does not touch parity/fuzz inputs since `22-03`.
- In Plan 22-07, validate the explicit-path cache layout locally or with a small guarded script before baking the assumed path into workflow tests.
- In the fuzz harness, log or count exactly-one-ingests cases separately as `known_malformed_layer_asymmetry` versus `unexpected_asymmetry`, even if both are non-fatal.
- In the workflow contract test, consider asserting semantic anchors rather than every formatting detail where possible, while still avoiding a YAML dependency.
- In Plan 22-06, require recording machine power/thermal context briefly, especially laptop plugged-in state and major background load.
- In Plan 22-06, explicitly exclude skipped external-tier benchmark lines from the committed smoke-tier interpretation.
- In Plan 22-08, clarify whether `make security-scan` failures caused by unavailable tools should block or be reported as environment blockers.
- Add one final “no scope creep” check before completion: no production parser files changed except already-approved test-only interactions, no CLI changes, no new persistent dependencies.

## Risk Assessment

**Overall risk: MEDIUM.**

The approach is technically sound and strongly aligned with the phase goals. The main risk is operational complexity: this phase touches correctness tests, fuzzing, benchmarks, documentation, Make targets, GitHub Actions, release notes, benchmark reports, and manual repository-rule verification. The most important correctness and security risks are addressed directly, especially silent SIMD skips, stale native cache selection, and malformed-input parity wording. The remaining risk is execution fragility: brittle workflow guards, assumed upstream cache layout, stale summary dependencies, and a large number of exact-string contracts that can fail for non-behavioral reasons.

---

## Consensus Summary

Two independent reviewers (Codex, OpenCode/gpt-5.5) assessed the eight Phase 22 plans. Both
judge the *design* sound — correctness-first sequencing, the controlled-vs-noisy evidence split,
and executable guards instead of prose are called out as strengths by both. They diverge on
readiness: OpenCode rates the phase **MEDIUM** risk (design fine, execution surface large),
while Codex rates it **HIGH until five specific blockers are corrected, MEDIUM-LOW afterward**.

Codex's review is the more actionable of the two: it read the repository source directly and
cites file:line for each finding. OpenCode reviewed plan text only.

### Agreed Strengths

- Correctness is hard-gated before any benchmark or shipping recommendation is produced.
- The D-04 qualified parity claim is carried consistently through tests, docs, benchmark report,
  and final approval — no reviewer found a place where it silently widens.
- Controlled committed evidence is cleanly separated from shared-runner noisy trend artifacts,
  and no performance regression threshold is introduced (D-12).
- Upstream download/checksum/cache/ABI behavior is referenced, not reimplemented.
- The three-state SIMD construction policy (`AMI_GIN_SIMD_REQUIRED=1`) directly closes the
  silent-CI-skip threat T-22-01.
- Bounded fuzzing incorporates the Spike 002 CPU-exhaustion finding rather than rediscovering it.
- Manual checkpoints are correctly reserved for what source cannot prove: package provenance,
  external ruleset binding, and performance interpretation.

### Agreed Concerns

Ordered by combined severity. Items marked **[verified]** were independently confirmed against
the artifacts during this review.

1. **[HIGH] [verified] `22-VALIDATION.md` is stale relative to the plan set.** Raised by Codex;
   independently confirmed. The per-task map still encodes a **4-plan / 3-wave** structure
   (task IDs `22-01-01` … `22-04-02`, mapping parity/fuzz→Plan 01, benchmark→Plan 02,
   docs→Plan 03, CI→Plan 04) against the actual **8-plan / 8-wave** phase. It also names
   `simd_contract_guard_test.go`, which the plans split into `simd_docs_guard_test.go` (22-04)
   and `simd_workflow_contract_test.go` (22-07). Consequence: the `nyquist_compliant: true`
   frontmatter claim is not currently trustworthy.

2. **[HIGH] [verified] Plan 22-08 can dead-end waiting for external state no plan creates.**
   Raised by Codex. Task 2 requires "at least one pull-request matrix run has completed", but
   `grep` across all eight plans finds no push, `gh pr create`, or equivalent action. Either add
   an explicit `checkpoint:human-action` for pushing/updating the PR, or document the completed
   run as a hard precondition before 22-08 starts.

3. **[HIGH] [verified] Plan 22-07's workflow contract test cannot satisfy its own acceptance
   criteria.** Raised by Codex. Task 1 states the test "reads only the `simd` section" yet also
   requires asserting top-level `permissions: contents: read` and the preservation of other jobs
   — facts that do not exist inside `ciJobSection(..., "simd")`. Fix: read the whole workflow for
   top-level/job-presence assertions and reserve `ciJobSection` for job-local contracts.

4. **[HIGH] [verified] Plan 22-01 uses a nonstandard checkpoint gate.** Raised by Codex. Both
   tasks use `gate="blocking-human"` (22-01:52, 22-01:74) while 22-08 uses the standard
   `gate="blocking"` (22-08:87, 22-08:112) — inconsistent within a single phase. Since these
   approvals must happen *before* execution rather than at phase end, the contract needs to be
   one GSD actually enforces.

5. **[MEDIUM-HIGH] [verified] Stale-summary risk in gate inheritance.** Raised by OpenCode,
   confirmed. Plan 22-04 publishes *public* parity guidance, but both its `<verify>` blocks run
   only docs/example/isolation commands; the parity gate is enforced solely by reading
   `22-03-SUMMARY.md` prose in `read_first`. If parity inputs changed after 22-03, 22-04 would
   ship a public correctness claim on a stale green. Plans 22-05 and 22-06 inherit the same
   pattern.

6. **[MEDIUM] Tagged security coverage gap.** Raised by Codex, echoed by OpenCode's note on
   `make security-scan`. `govulncheck ./...` validates only the default build; nothing scans the
   `-tags simdjson` call graph — precisely the optional path this phase exists to bless.

7. **[MEDIUM] Benchmark state and environment hygiene (Plans 22-05 / 22-06).** Both reviewers,
   from different angles. Codex: one SIMD parser is reused across all fixtures, so native buffer
   growth from an earlier fixture can bias later ones while stdlib has no equivalent persistent
   state; the plan never states whether the intended measurement is cold, warm, or steady-state.
   Codex also notes the capture command does not clear `GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL` /
   `GIN_PHASE20_SIMDJSON_DIR`, so inherited local values could contaminate the authoritative
   smoke-only record, and that recording a *dirty* worktree is weaker than requiring a clean one.
   OpenCode independently raises thermal/power/background-load contamination for the COUNT=10 run
   and asks that skipped external-tier lines be excluded from committed interpretation.

8. **[MEDIUM] External-tier skip placement (Plan 22-05).** Codex: calling
   `phase20ExternalBenchmarkFixture(b)` at top benchmark level would skip the *parent*, not just
   the external tier. Needs a dedicated `b.Run` subtree.

9. **[MEDIUM] Plan 22-03 overstates what its fuzz branches prove.** Codex only, but well argued:
   with both failure modes configured soft, a soft-skipped document can still yield a non-nil
   encoded empty index, so "encoded bytes exist" ≠ "the document ingested". The `1e400 garbage`
   seed therefore may not exercise the exactly-one-ingests branch at all under both-soft policy —
   the known asymmetry is exposed by hard-parser/soft-numeric policy in the *existing* test.
   Corpus verification also checks only file count and headers.

10. **[MEDIUM] Brittle exact-string workflow assertions.** Both reviewers. Consistent with the
    existing NOTICE-guard pattern, but `simd_workflow_contract_test.go` has a far larger surface
    and will fail on semantically equivalent YAML reformatting. Both suggest separating semantic
    anchors from formatting assertions.

11. **[LOW] Requirement attribution inflation.** Codex: Plan 22-01 claims all four requirements
    (SIMD-08…11) while producing only prerequisite approvals and no artifacts.

### Divergent Views

- **Overall readiness.** Codex: **HIGH** risk, five blockers must be fixed before execution.
  OpenCode: **MEDIUM**, no hard blockers — main risk is operational complexity. The divergence
  is explained by depth of access: Codex read the repo and found concrete internal contradictions
  (22-07 scope, 22-01 gate syntax, 22-08 missing precondition) that a text-only pass would miss.
  Weight Codex's blocker list accordingly.

- **Roadmap plan ordering (OpenCode HIGH — downgraded).** OpenCode flags that the ROADMAP lists
  `22-07` before `22-06`. Verified: the list is ordered by **wave** (Wave 6 = 22-07,
  Wave 7 = 22-06) and matches every `depends_on` edge exactly. This is a readability nit, not a
  defect. Note that 22-06's dependency on 22-07 is a deliberate *resource-isolation* edge — it
  keeps CPU-intensive suites out of the timing window — not a data dependency.

- **Plan 22-07 cache-path layout (OpenCode HIGH — partially valid).** OpenCode worries the
  assumed `<cache>/<version>/<goos>-<goarch>/<library>` shape may not match upstream, and that
  `go list -m` returns `v0.1.7` with a `v` prefix. The plan already anticipates the prefix
  (22-07:131) and instructs reading upstream `internal/bootstrap/cache.go` and `url.go` to
  confirm layout (22-07:91). The residual risk is real but narrower than stated: confirmation is
  a *read-the-source* step, not an executable assertion, and the path is baked into both workflow
  YAML and the contract test — so a layout mismatch breaks all five legs simultaneously.

- **`check-simd-docs` implementation.** Codex flags dual sources of truth (Make shell logic +
  mutation-driven Go tests) and recommends Make invoke one authoritative Go contract test.
  OpenCode does not raise this. Worth a decision either way.

- **Fuzz corpus seed count.** OpenCode calls "exactly five seeds" over-constrained as an
  acceptance criterion; Codex instead wants *stronger* corpus validation (decode every entry,
  enforce bounds, confirm the five source categories). These pull in opposite directions —
  Codex's position is better aligned with the phase's evidence-first posture.

### Recommended Next Step

Five items are execution blockers (#1–#4 above, plus the 22-08 precondition). They are all
plan-text corrections rather than design changes, so:

```
/gsd:plan-phase 22 --reviews
```
