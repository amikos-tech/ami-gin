---
phase: 19
reviewers: [gemini, claude]
reviewed_at: 2026-04-27T17:41:45+03:00
plans_reviewed:
  - .planning/phases/19-simd-dependency-decision-integration-strategy/19-01-PLAN.md
---

# Cross-AI Plan Review - Phase 19

## Gemini Review

# Phase 19 Plan Review: SIMD Dependency Decision & Integration Strategy

## Summary

The plan for Phase 19 is a high-quality, documentation-first strategy that successfully resolves the first-order blockers for SIMD integration. It correctly identifies the upstream `pure-simdjson` (`v0.1.4`) as the target dependency and establishes a clear boundary: Phase 19 locks the architecture and policy, while Phase 21/22 handle the implementation. The strategy for handling native binaries, delegating to upstream bootstrap while providing explicit environment variable overrides for air-gapped or corporate environments, is pragmatic and aligns with the correctness -> usefulness -> performance priority of the project.

## Strengths

- Zero-code boundary: Strictly adheres to the phase goal by producing only documentation (`19-SIMD-STRATEGY.md`) and state updates, preventing premature implementation drift.
- Exact numeric semantics: Explicitly carries forward the Phase 07 requirement for `int64`/`uint64`/`float64` fidelity by utilizing `pure-simdjson`'s typed accessors, mitigating a high-risk technical pitfall.
- Robust fallback policy: The decision to use hard error on load plus documented fallback recipe is superior to silent degradation. It ensures operational clarity and prevents performance surprises in production while allowing callers to implement their own retry/failover logic.
- Comprehensive CI matrix: Committing to a 5-platform matrix across Linux, Darwin, and Windows on `amd64`/`arm64` ensures platform-specific ABI or numeric issues are caught in Phase 22.
- Dependency hygiene: Includes a clear plan for `NOTICE.md` updates and manual license-drift reviews during dependency bumps, which is appropriate for a project moving toward a stable release.

## Concerns

- LOW - Binary security/trust: The strategy relies on `pure-simdjson` auto-download from Cloudflare/GitHub. While the plan mentions SHA-256 verification and signing, consumers in highly regulated environments may still find auto-downloading native code problematic.
  - Mitigation: The plan already includes `PURE_SIMDJSON_LIB_PATH` and `docs/simd-deployment.md` for air-gapped use, which is sufficient.
- LOW - Build tag leaking: If the `simdjson` tag is accidentally enabled in an environment without native libraries and without a fallback, construction will fail.
  - Mitigation: Task 2's requirement for `go test ./...` without tags ensures the default path remains green and dependency-free.
- LOW - Stop condition speedup threshold: The decide-on-evidence policy for performance gains is slightly vague.
  - Mitigation: This is acceptable for a strategy phase; the actual benchmark data in Phase 22 will provide the necessary context.

## Suggestions

- Verification command enhancement: In the Task 1 `<verify>` block, consider adding a check for the exact commit SHA `0f53f3f2e8bb9608d6b79211ffc5fc7b53298617` to ensure the strategy document is as specific as the research.
- Telemetry alignment: Explicitly note in the strategy that the `Name()` return value (`pure-simdjson`) should be what is passed to the telemetry signals defined in Phase 14, ensuring the operational readiness goal is met.
- Bootstrap tooling: In `docs/simd-deployment.md` to be written in Phase 21, ensure the mention of the `pure-simdjson-bootstrap` CLI is highlighted for users who want to pre-fetch binaries during a Docker build phase to avoid runtime downloads.

## Risk Assessment: LOW

The overall risk is LOW. The plan is extremely conservative regarding the repository's default state, preserving Go-only, stdlib-first behavior, while providing a well-defined opt-in path for SIMD. By resolving the license, distribution, and API questions in a dedicated strategy phase, the project avoids the risk of re-architecting during implementation. The alignment with prior phase decisions, especially Phase 07 and Phase 13, is excellent.

---

## Claude Review

# Phase 19 Plan Review: SIMD Dependency Decision & Integration Strategy

## 1. Summary

The plan is well-scoped, single-deliverable, and tightly anchored to the upstream decisions captured in `19-CONTEXT.md`. It correctly recognizes Phase 19 as a strategy phase and resists the temptation to land code, dependency edits, or CI changes that belong in Phases 21/22. The decision coverage table maps all 13 CONTEXT decisions to artifact requirements, and the grep-based verification gives the artifact a falsifiable acceptance contract. The main weaknesses are that the verification command relies on grep for natural-language strings that could pass while the surrounding context is wrong or contradictory, some locked-strategy values are presented inconsistently between context (`windows-amd64-msvc`) and plan (`windows/amd64`), and the stop-condition runbook is intentionally minimal but lacks a clear owner/escalation pointer should a HARD trigger fire.

## 2. Strengths

- Tight scope discipline. The "Out of Scope" section in both CONTEXT and the plan explicitly forbids `go.mod`, source, CI, README, NOTICE, and CHANGELOG edits, eliminating the most common scope-creep failure mode for strategy phases.
- Exhaustive decision coverage table. Every D-01..D-13 maps to a strategy-artifact section, making it easy to audit completeness.
- Falsifiable acceptance contract. Grep-based verification for exact strings (`NewSIMDParser() (Parser, error)`, `//go:build simdjson`, `PURE_SIMDJSON_LIB_PATH`) means the artifact either matches or fails.
- Right-sized plan count. Single plan (`19-01`) with two tasks correctly reflects the research recommendation; splitting would add process without reducing risk.
- Threat model is concrete. T19-01 through T19-04 cover the four real failure surfaces: license drift, default-build pollution, silent fallback, and parity failure shipping anyway.
- Locked-strategy values prevent drift between researcher intent and planner execution.

## 3. Concerns

- MEDIUM - Platform-label inconsistency between locked values and CONTEXT/specifics. `<locked_strategy_values>` lists `windows/amd64` but CONTEXT `<specifics>` says the matrix label is `windows-amd64-msvc`, matching upstream's signed-binary naming. The strategy artifact will inherit whichever the planner copies, and Phase 22 CI will inherit the artifact. Trivial to resolve now; expensive to discover during Phase 22 CI bring-up. Suggest deciding once and using the same label string everywhere; recommend `windows-amd64-msvc` to match upstream artifact names.
- MEDIUM - Grep verification can pass with contradictory content. All acceptance criteria are positive substring checks. Nothing forbids the artifact from also stating a contradictory policy. Suggest adding at least one negative check for the small number of properties that must not appear.
- MEDIUM - Stop-condition handoff is underspecified. D-13 explicitly defers the runbook for HARD triggers. The strategy artifact will state HARD means halt Phase 21/22 and defer SIMD to v1.4, but does not say who notices, who decides, or what the deferral artifact path is. Suggest adding one sentence pointing to `/gsd-pause-work` or equivalent as the trigger response so the next agent has a path.
- LOW - `WithParser` is an existing API but the artifact does not link callers to it. The fallback recipe shows default stdlib construction, but should also show the success branch using `gin.WithParser(p)`.
- LOW - `NewSIMDParser()` return type loses concrete-type benefits. Returning `Parser` follows Phase 13 export minimization, but precludes future SIMD-specific methods without a type assertion. Acceptable given D-12, but worth flagging so Phase 21 does not try to expose SIMD-only methods.
- LOW - Task 2 runs `go test ./...` as a sanity check but the phase is documentation-only. If it fails, it likely indicates a pre-existing repository issue, not a Phase 19 regression. Keep the check, but state that interpretation.
- LOW - No cross-check that Phase 20 dependency ordering is preserved. Phase 20 can run independently; one note would prevent future planners from unnecessarily serializing it behind Phase 19.
- LOW - Status string in Task 2 is awkward. It conflates next planning target with next executable target; consider splitting.

## 4. Suggestions

- Use `windows-amd64-msvc` consistently in the strategy artifact and update `<locked_strategy_values>` to match `<specifics>` and upstream. Update acceptance criteria to include this exact string.
- Add a negative-grep acceptance criterion for at least `CGo` and `silent fallback` to prevent contradictory content from passing review.
- Expand the fallback recipe in the SIMD-03 section to include the success branch, making it a complete construction recipe rather than half a recipe.
- Add an escalation subsection under the stop table: one sentence pointing at `/gsd-pause-work` and the v1.4 backlog item path as the response to a HARD trigger.
- Add a Phase 20 independence note in the downstream Phase 22 contract section so future planners do not serialize unnecessarily.
- Soften the `go test ./...` check in Task 2's acceptance criteria by making it advisory, not blocking. Phase 19 cannot regress code it does not touch.
- Consider a single-line self-review criterion in Task 1 that the planner reads the finished artifact end-to-end before marking the task done. The grep checks ensure required strings exist; only a human read ensures the prose is coherent.

## 5. Risk Assessment

Overall risk: LOW.

The phase touches no executable code, so blast radius is limited to `.planning/`. Reversibility is trivial: the artifact can be edited or rewritten without coordination. The dependency decisions being locked, including pin, license, build tag, and API shape, are well grounded in verified upstream state and Phase 13 precedent. The largest residual risk is prose-level ambiguity that survives grep verification but trips up Phase 21/22. The plan correctly identifies and resists scope creep into Phases 20-22.

---

## Consensus Summary

Both reviewers agree that Phase 19 is well-scoped, low-risk, and correctly documentation-only. The plan strongly preserves default stdlib behavior, keeps SIMD explicitly opt-in, and carries forward the exact numeric semantics and parser-seam constraints from earlier phases.

### Agreed Strengths

- The phase boundary is clear: no product code, dependency graph, CI, README, NOTICE, CHANGELOG, or runtime-doc changes land in Phase 19.
- The dependency decision is explicit enough to unblock Phase 21: `github.com/amikos-tech/pure-simdjson v0.1.4`, MIT posture, and manual license/NOTICE review on bumps.
- The operational stance is conservative: hard construction failure, caller-owned fallback, no silent degradation, and default stdlib builds remain dependency-free.
- The downstream contracts for Phase 21 and Phase 22 are mostly actionable.

### Agreed Concerns

- Verification should be tightened. Gemini asked for commit SHA coverage; Claude asked for negative checks and contradiction prevention. Highest-value plan update: add checks for the tag commit, platform label, and prohibited contradictory language.
- Runtime/bootstrap guidance needs to be clear in Phase 21 documentation. Gemini emphasized `pure-simdjson-bootstrap`; Claude emphasized full success/fallback construction examples.
- The stop/fallback table is directionally right, but Claude identified a missing escalation handoff for HARD triggers.

### Divergent Views

- Gemini considered the vague performance speedup threshold acceptable for this phase. Claude did not focus on the threshold, instead prioritizing ambiguity and handoff risks.
- Gemini treated the 5-platform CI matrix as a clear strength. Claude agreed but flagged the exact Windows label as inconsistent (`windows/amd64` vs `windows-amd64-msvc`) and worth resolving before execution.

### Recommended Planning Updates Before Execution

1. Resolve the Windows platform label inconsistency, preferably to `windows-amd64-msvc` if the strategy is meant to mirror upstream artifact naming.
2. Add verification checks for the tag commit SHA and at least one negative check against contradictory fallback/dependency language.
3. Expand the Go recipe so it shows both successful `gin.WithParser(p)` construction and explicit stdlib fallback.
4. Add one escalation sentence for HARD stop triggers.
5. Note that Phase 20 remains independent and should not be unnecessarily blocked by Phase 19 strategy execution.
