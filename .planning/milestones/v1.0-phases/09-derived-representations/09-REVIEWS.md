---
phase: "09"
reviewers: [gemini, claude]
reviewed_at: 2026-04-16T18:36:46Z
plans_reviewed:
  - 09-01-PLAN.md
  - 09-02-PLAN.md
  - 09-03-PLAN.md
---

# Cross-AI Plan Review — Phase 09

## Gemini Review

This is a review of implementation plans **09-01**, **09-02**, and **09-03** for Phase 09 of the GIN Index project.

## Summary
Phase 09 is a foundational shift from replacement-only transformers to an additive "multi-field" indexing model. The plans provide a highly detailed and technically sound roadmap to achieve this by introducing hidden internal target paths and an explicit representation metadata layer (v7 format). The strategy preserves the existing `pathID`-keyed storage substrate while decoupling the public query contract from the physical storage layout. The plans rigorously adhere to the "strict by default" failure posture and "explicit over implicit" query routing requested by the user.

## Strengths
*   **Collision-Safe Internal Layout:** The use of the `__derived:` prefix for internal target paths effectively separates derived storage from valid JSONPaths (which must start with `$`), preventing name collisions while reusing the existing `PathDirectory` machinery.
*   **Surgical Integration:** By routing alias selection through `Predicate.Value` using a typed wrapper (`gin.As`), the plan avoids a massive refactor of the query helper API signatures while maintaining backward compatibility for raw-path queries.
*   **Transactional Integrity:** Leveraging the transactional `AddDocument` model from Phase 07 to ensure strict failure (aborting merge if any transform fails) ensures the index remains a source of truth for all declared representations.
*   **Explicit Serialization Contract:** The decision to move representation metadata into a dedicated v7 binary section rather than overloading the existing JSON-based `SerializedConfig` protects the format from "naming convention" brittle-ness and simplifies round-trip validation.
*   **Migration Path:** Turning `WithFieldTransformer` into a "compatibility trap" error ensures users are forced to move to the safer aliased API rather than having their code silently break or change behavior.

## Concerns
*   **Breaking Change for Custom Transformers** (**Severity: LOW**): The plan explicitly breaks existing `WithFieldTransformer` usage. While this is the correct architectural choice for a v1.0-milestone transition, it is a high-friction point for users upgrading from v0.1.x. *Mitigated by the explicit error message implementation in Plan 01.*
*   **Serialization Failure for Custom Transforms** (**Severity: MEDIUM**): Users who heavily rely on custom Go functions for transforms will find their indexes can no longer be encoded. This is a functional regression from the v0.1.x "replacement-only" model where even if the transform wasn't serializable, the *result* was already in the index. *Mitigated by explicit documentation and the `Serializable: false` check.*
*   **Internal Path Hardcoding** (**Severity: LOW**): There is a small risk that power users might discover the `__derived:` prefix and hardcode queries using it. *Mitigated by Plan 03's focus on teaching `gin.As` exclusively.*

## Suggestions
*   **Diagnostic Introspection:** In Plan 02 Task 1, ensure the `Representations()` method provides enough info (like the `TransformerID`) to help users debug why a specific transform might be failing for certain documents.
*   **Reserved Prefix Guard:** Consider adding a guard in `ValidateJSONPath` to reject any path starting with `__derived:` if it doesn't come from the internal builder, ensuring the reserved namespace stays truly internal.
*   **Transformer Registry Growth:** As more built-in transformers are added to the registry to replace custom function needs (to solve the serialization issue), ensure the `v7` metadata section remains extensible.

## Risk Assessment
**Overall Risk: LOW**

The implementation is well-sequenced across three waves, with clear "must-have" truths and artifacts. By reusing the core `pathID` logic and focusing the new work on the "binding layer" (Config -> Builder -> Query Resolution), the plans minimize the surface area for logic errors in the complex Roaring bitmap and numeric pruning code. The primary risk is user friction regarding custom transformer serialization, which is a known and accepted trade-off for the v1.0 quality bar.

---

## Claude Review

`★ Insight ─────────────────────────────────────`
This is a well-researched phase with strong planning artifacts — the CONTEXT.md + DISCUSSION-LOG.md + RESEARCH.md chain shows systematic requirement capture. The main technical risks are architectural: the hidden internal path format interacts with existing validation machinery in ways the plans don't explicitly resolve, and the breaking API change creates a compile-broken gap between Plan 01 and Plan 03.
`─────────────────────────────────────────────────`

---

## Phase 09 Plan Review: Derived Representations

### 1. Summary

The three-plan sequence is logically sound and well-sequenced: config layer first, then query/serialize layer, then public docs/tests. The research is thorough, the acceptance criteria are specific and testable, and the threat model covers the genuinely risky boundaries. However, two structural issues create significant execution risk. First, the breaking signature change to all 13 built-in helper functions in Plan 01 will immediately break the full existing test suite, which won't be repaired until Plan 03 — leaving a multi-plan window where `go test ./... -count=1` fails. Second, the hidden `__derived:<path>#<alias>` format for companion paths is not a valid JSONPath string, and the existing `rebuildPathLookup()` function runs every PathDirectory entry through `NormalizePath()` (backed by the `ojg/jp` parser) — this will likely silently corrupt or error on decode, which is a correctness gap that the plans don't address. Both issues are fixable but need explicit handling before execution begins.

---

### 2. Strengths

- **Plan sequencing is correct.** Wave 1 (config + builder), Wave 2 (query + serialize), Wave 3 (docs + acceptance) is the right execution order with no hidden cyclic dependency.
- **Acceptance criteria are machine-verifiable.** Every task has exact type/function names as acceptance criteria, making automated validation straightforward.
- **Threat model is accurate and complete.** The STRIDE register correctly identifies the three most dangerous boundaries: overwrite-silent-drop, transform chaining, and partial-merge on failure.
- **Explicit handling of opaque custom transformers.** Making `WithFieldTransformer(path, fn)` a hard error and providing `WithCustomTransformer(path, alias, fn)` with `Serializable: false` is the right design; Pitfall 5 in the research is directly addressed.
- **Version bump is explicit.** v6 -> v7 with a dedicated representation-metadata payload is the right choice; existing rejection semantics keep the contract clean.
- **TDD anchors are specific.** Named test functions in acceptance criteria prevent vague "add tests" tasks.
- **`Predicate.Value` as the alias channel** is the lowest-churn approach — existing operator handlers are unchanged, and raw-path default behavior requires zero callsite updates for non-companion queries.

---

### 3. Concerns

**[HIGH] — Hidden `__derived:` paths are not valid JSONPath; `rebuildPathLookup()` will fail on decode**

`rebuildPathLookup()` calls `NormalizePath(entry.PathName)` on every PathDirectory entry. `NormalizePath` uses `ojg/jp` internally. The string `__derived:$.email#lower` is not a valid JSONPath expression — the parser will either return it unchanged (breaking canonical deduplication) or produce an error. Either outcome breaks decode correctness for any index with companion representations. The plans never address how `__derived:` prefixed paths bypass or are exempted from JSONPath normalization/validation. The executor will discover this at runtime, but there's no guidance in the plan.

*Suggested fix: Plan 01 Task 2 should explicitly state that the builder must register companion paths as raw string keys in `pathData` and that `rebuildPathLookup()` must skip JSONPath normalization for paths starting with the `__derived:` prefix.*

---

**[HIGH] — Breaking API change creates a compile-broken gap between Plan 01 and Plan 03**

Plan 01 Task 1 changes all 13 built-in helper signatures from single-arg to two-arg (e.g., `WithISODateTransformer(path string)` -> `WithISODateTransformer(path, alias string)`) and makes `WithFieldTransformer(path, fn)` return an error. The existing test suite has ~15 tests in `transformers_test.go` alone that use these old signatures (`TestDateTransformerIntegration`, `TestToLowerIntegration`, `TestIPv4ToIntRangeQuery`, etc.). These tests will fail to **compile** after Plan 01 executes. The full suite `go test ./... -count=1` (required at wave merge) will be broken until Plan 03 updates them — spanning two plan boundaries. Plan 02 also depends on the same test file.

*Suggested fix: Plan 01 Task 2 (or a new task in Plan 01) should explicitly update all existing `transformers_test.go` tests to the new API as part of the same wave. Alternatively, Plan 03 should be flagged as a dependency blocker for the Plan 01 wave merge verification.*

---

**[MEDIUM] — `Finalize()` doesn't populate `representationLookup`; only decode path is specified**

Plan 02 Task 2 describes rebuilding `representationLookup` from decoded metadata after `Decode()`. But `Finalize()` also produces a `GINIndex` that needs `representationLookup` populated for in-memory queries to work without round-tripping through encode/decode. The plan has no task or acceptance criterion for populating `representationLookup` inside `Finalize()`. Any test that builds an index, calls `Finalize()`, and immediately queries an alias will fail.

*Suggested fix: Plan 02 Task 1 or Task 2 should include an explicit acceptance criterion: `GINIndex.representationLookup` is populated by `Finalize()` as well as by `readRepresentations()`.*

---

**[MEDIUM] — No coverage of the hidden path in `validatePathReferences()` / CLI `info` output**

`validatePathReferences()` checks every string index, numeric index, null index, etc. against PathDirectory entries. If companion paths (`__derived:...`) appear in PathDirectory but not in these index maps (because a specific doc had no string/numeric companions), the validation will pass. But if a path ends up in one of those maps, the reverse check requires a PathDirectory entry. The plans don't specify how `validatePathReferences()` treats hidden companion paths, and the CLI `info` command will surface them directly in output, which would confuse users who were told never to type `__derived:`.

---

**[MEDIUM] — Array/object source path fan-out path is underspecified**

Plan 01 Task 2 says "Keep arrays/objects working through the existing subtree materialization path when a registration exists at the current canonical source path." But today, when a transformer is registered on a path, `decodeTransformedValue()` eagerly decodes the entire subtree and returns a materialized value — skipping stream descent. With additive companions, the plan needs to: (a) decode the full subtree once, (b) stage raw under the source path, (c) apply each companion to the raw prepared value. Step (b) is new behavior — raw subtree staging was previously suppressed by the transformer. The mechanics for this exact case (transformer on an object path) are not spelled out and have a realistic chance of being missed.

---

**[LOW] — `serialize.go` new section position is ambiguous**

Plan 02 Task 2 says to add representation metadata "alongside the config payload" and "around the existing config trailer." Whether the new section comes *before* or *after* the config JSON payload matters for the binary format contract. The acceptance criteria don't pin down the section order, and the executor will make an arbitrary choice that becomes a permanent v7 format decision.

---

**[LOW] — No benchmark coverage for companion path overhead**

The project constraint requires benchmark-backed claims for hot-path and size changes. Every document with companion registrations now writes 1+N paths instead of 1 path to the PathDirectory and all associated index maps. For a source path with 3 companions, that's 4 paths. There's no plan to measure AddDocument overhead or PathDirectory size growth. This is lower risk if the project doesn't claim perf neutrality, but the phase goal language ("hot-path efficiency") implies it matters.

---

**[LOW] — DERIVE-04 decode parity is underspecified**

Plan 03 Task 2 acceptance criteria say "at least one of those tests compares alias-query results before and after Encode()/Decode()" — but there are three required pattern families. If only one of three tests exercises decode parity, the other two might silently break after a serialization change. Consider requiring decode parity for all three acceptance tests.

---

### 4. Suggestions

- **Before Plan 01 Task 1 executes**, audit every call site in `gin_test.go`, `transformers_test.go`, `serialize_security_test.go`, and `examples/` that uses `WithFieldTransformer` or any single-arg built-in helper. Add updating those sites to Plan 01 Task 1's scope, or add a Plan 01 Task 3 specifically for existing test/example migration. This prevents the compile-broken window.

- **Add an explicit rule to `rebuildPathLookup()`** (and document it in Plan 01 Task 2): paths prefixed with `__derived:` are internal companion paths and must bypass the `NormalizePath`/JSONPath canonicalization step. Add a test for this in `gin_test.go` to lock the behavior.

- **Add a `Finalize()` requirement to Plan 02**: after constructing the index from `pathData`, `Finalize()` should populate `idx.representationLookup` from the builder's registration data — the same way it populates `idx.pathLookup`. Validate this with an in-memory (non-round-trip) alias query test.

- **Pin the representation metadata section position** in Plan 02 Task 2 (e.g., "write representation metadata immediately after the config payload"). Lock this with a byte-offset regression test similar to the existing `TestDecodeRejectsUnknownPathMode` approach.

- **Add a `Representations()` filter for hidden paths in `info` CLI output** — or at minimum add a note to Plan 02 Task 1 that the `info` output should suppress `__derived:` prefixed paths. This prevents user confusion when they run `gin-index info` on a file built with companions.

---

### 5. Risk Assessment

**Overall Risk: HIGH**

The two HIGH concerns together create a scenario where Plan 01 execution produces a codebase that doesn't compile and doesn't correctly decode its own output. Either issue alone would be a significant setback; together they represent a likely execution failure without pre-plan intervention. Both are clearly fixable — the issues are identifiable in advance — but they require explicit plan amendments before the executor starts work. The LOW/MEDIUM concerns are implementation details that a careful executor would handle, but the HIGH concerns need to be in the plan text itself to be reliable.

---

## Consensus Summary

The phase direction is strong. Both reviewers agree the core architecture fits the codebase: keep raw-path queries as the default public contract, add explicit alias targeting through `gin.As(...)`, preserve transactional builder semantics, and persist explicit representation metadata instead of inferring behavior from naming conventions.

### Agreed Strengths

- Reusing the existing `pathID`-keyed storage layer is the right move; the new work belongs in the representation binding layer rather than in a second indexing dimension.
- Routing companion selection through `Predicate.Value` with `gin.As(...)` preserves the current query helper surface and keeps raw-path behavior stable.
- Strict additive builder semantics are correct for this project: raw values stay indexed, companion failures abort the document, and transform chaining stays out of scope.
- A dedicated versioned representation-metadata section is the right serialization direction for deterministic round-tripping.

### Agreed Concerns

- The migration story needs to be explicit. Both reviews call out user friction around helper-signature changes, custom transformer behavior, and the need for strong docs plus diagnostics.
- The reserved internal namespace needs hardening. Both reviews warn against callers or tools depending on hidden `__derived:` paths as if they were part of the public API.

### Divergent Views

- Claude identifies execution blockers that Gemini does not: hidden `__derived:` paths likely conflict with existing JSONPath normalization during lookup rebuild, the Plan 01 helper-signature change creates a compile-broken gap before later plans update tests/examples, and `Finalize()` needs explicit alias-lookup population for in-memory queries.
- Gemini is more comfortable with the current sequencing and rates the phase low risk overall, focusing on ergonomics and guardrails such as richer `Representations()` diagnostics, reserved-prefix validation, and keeping the transformer registry extensible.

### Recommended Follow-Ups Before Execution

- Amend Plan 01 to define how internal `__derived:` paths bypass JSONPath normalization and validation during lookup rebuild.
- Pull migration of existing transformer tests/examples into the first execution wave, or make the wave-level verification expectations explicit so the repo is not left compile-broken between plans.
- Amend Plan 02 so `Finalize()` populates alias lookup tables in-memory, not only after `Decode()`.
- Pin the representation-metadata section order in the v7 wire format and decide whether CLI/info output should suppress or abstract hidden companion paths.
