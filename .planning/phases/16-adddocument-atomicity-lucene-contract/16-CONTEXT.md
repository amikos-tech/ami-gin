# Phase 16: AddDocument Atomicity (Lucene contract) - Context

**Gathered:** 2026-04-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Refactor the builder's `validate → merge` pipeline so **the merge step is infallible by construction**. Every reason `mergeStagedPaths` / `mergeNumericObservation` / `promoteNumericPathToFloat` can return an error today must be pre-detected in `validateStagedPaths` against the *real* `pathData` state (not a fresh preview). Rename `poisonErr` → `tragicErr` and narrow it to internal-invariant violations only. Add a `recover()` belt-and-suspenders inside `mergeStagedPaths` that converts any reachable panic to `tragicErr`. Lock the contract with a `// MUST_BE_CHECKED_BY_VALIDATOR` comment marker + CI grep, new compile-enforced merge signatures, and a `gopter`-driven atomicity property test (`atomicity_test.go`).

This phase is **structural refactor only** inside `builder.go` — no public API changes, no CLI surface, no new exported error types. The structured `IngestError` type (Phase 18) and the `IngestFailureMode` taxonomy (Phase 17) layer on top of the contract established here.

**Carrying forward from earlier phases / project charter:**
- **Strategy C (validate-before-mutate) is locked** by REQUIREMENTS.md and PROJECT.md §Current Milestone. Strategy A (snapshot/restore) stays held in reserve.
- **Lucene per-document contract is the reference target.** Lucene maintains two separate catalogs (tragic-event vs per-document exception) with deliberately different routing semantics; we mirror that separation.
- **Priority order** `correctness → usefulness → performance` — any perf concern about the added validator cost is routed to the 999.x backlog per the 999.5 precedent.
- **Phase 13 parser seam** — parser errors return verbatim from `AddDocument`; pathData is untouched because `b.currentDocState` resets via `defer`. That behavior is preserved.
- **Phase 14 observability seam** — silent-by-default Logger is already wired; the tragic `recover()` site emits at Error level through that seam (opt-in visibility, no default-behavior regression).
- **Additive-change bias** — internal merge-layer signatures change (drop `error` return); public `AddDocument`/`Finalize` surface unchanged.

</domain>

<decisions>
## Implementation Decisions

### Validator Simulation Strategy
- **D-01:** Extend the existing **shadow-fields pattern on `stagedPathData`** (today's `numericSim*` fields seeded by `seedNumericSimulation` at `builder.go:671`). As the merge-hoisting catalog grows, add new `*Sim*` fields onto `stagedPathData`; the staging functions transparently re-seed from `b.pathData` on first touch. No typed `pathPreview` clone, no `commit-bool` shared helper. Keep the mutation site obvious.

### Failure-Mode Catalogs (two catalogs, deliberately separate)

Following Lucene's `IndexWriter` structure (per-document exceptions vs `tragicEvent` — different routing, different assurance bars).

- **D-02 — Merge-hoisting catalog (narrow):** exactly ONE failure mode reaches merge today and therefore needs hoisting into `validateStagedPaths`:
  - `"unsupported mixed numeric promotion at %s"` — fires at `mergeNumericObservation:839` and `promoteNumericPathToFloat:860,867`.
  - All other ingest errors (parser, stage, companion-transformer, JSON literal parse, unsupported type) already fire **before** merge against throwaway state (`currentDocState` reset via `defer`), so they are atomic by construction and do NOT need validator hoisting.
  - The `// MUST_BE_CHECKED_BY_VALIDATOR` marker is placed on `mergeStagedPaths`, `mergeNumericObservation`, and `promoteNumericPathToFloat` only.

- **D-03 — `tragicErr`-stays-nil catalog (broad):** the ATOMIC-03 unit test iterates the **full public failure-mode catalog** and asserts `tragicErr` stays nil throughout:
  1. Parser errors (malformed JSON, non-UTF8, trailing bytes)
  2. Unsupported JSON token type (`stageScalarToken:436`, `stageMaterializedValue:514`)
  3. Hard-mode companion transformer rejection (`stageCompanionRepresentations:531`)
  4. Malformed numeric literal (`stageJSONNumberLiteral:544`)
  5. Non-finite numeric (`parseJSONNumberLiteral:560`, `stagedNumericFromValue:586`)
  6. Validator-rejected numeric promotion (post-hoist)
  7. Pre-parser gate errors: `position >= numRGs` (`AddDocument:312`), `BeginDocument` not called / called >1x (`AddDocument:331,333`), `rgID` mismatch (`AddDocument:341`)
  8. Unsupported `uint`/`uint64` > `MaxInt64` (`stageMaterializedValue:482,493`)

### Contract Enforcement (composed)
- **D-04 — Mechanism composes four defenses:**
  1. **Compile-time signature change** — `mergeStagedPaths`, `mergeNumericObservation`, `promoteNumericPathToFloat` drop their `error` return. Go compiler rejects any future reintroduction.
  2. **`// MUST_BE_CHECKED_BY_VALIDATOR` comment marker** — placed on each of the three merge-layer functions above.
  3. **CI grep** — `make lint` (or equivalent) asserts: any function whose signature is preceded by `// MUST_BE_CHECKED_BY_VALIDATOR` MUST NOT contain `error` in its return types. Implemented as a shell/grep check in the Makefile lint target, not a golangci-lint custom rule.
  4. **`recover()` belt-and-suspenders** — wraps **`mergeStagedPaths` only** (narrowest scope that still catches all merge-reachable panics). Not `mergeDocumentState`, not `AddDocument`. Factored into a testable helper `runMergeWithRecover(func())` so the recovery path has a direct unit test (see D-06).

### Tragic Observability (Phase 14 seam integration)
- **D-07 — Logger emission at the recover site:** when `recover()` converts a panic to `tragicErr`, emit exactly ONE log event at Error level through the existing `gin.WithLogger` seam: `Logger.Error("builder tragic: recovered panic in merge", attrs={...})`. Silent for default users (Phase 14's silent-default); visible to callers who wired slog/stdlib. No Telemetry counter, no Signals emission — Logger only. Phase 18 can extend.

### Error-Message Wording
- **D-09 — Mirror today's exact wording.** The hoisted numeric-promotion failure returns the same string the merge layer produces today: `"unsupported mixed numeric promotion at %s"` (and the variant from `promoteNumericPathToFloat`). No restructuring, no layer-prefix. Phase 18 owns the structured migration to `IngestError` with a `Layer=numeric` field; Phase 16 keeps wording stable to minimize that diff and avoid test-assertion churn.

### Test Infrastructure
- **D-05 — Atomicity property test shape:**
  - File: `atomicity_test.go` (new, at repo root).
  - **Encode-determinism sanity test (prerequisite):** build the same clean corpus twice, `Encode` both, assert `bytes.Equal`. This fires FIRST so any encode non-determinism surfaces as a test failure of its own, not as a confusing atomicity-violation failure.
  - **Typed failure-intent gopter generators** (new generator family alongside existing `property_test.go` ones):
    - `genParserMalformedDoc` — produces raw bytes guaranteed to fail `parser.Parse` (non-UTF8, truncated JSON, trailing-content varieties).
    - `genHardTransformerRejectingDoc` — pairs with a registered hard-mode transformer and a value the transformer will reject.
    - `genNumericPromotionFailingDoc` — produces a doc whose numeric observation will cause `!canRepresentIntAsExactFloat` at merge/validator time.
  - **Corpus:** gopter `gen.SliceOfN(≥1000, …)` interleaving clean docs and the three failure-intent generators; verify ≥10% failing docs per generated corpus.
  - **Assertion:** `Encode(builderWithFullCorpus)` `bytes.Equal` `Encode(builderWithCleanSubsetOnly)`.

- **D-06 — Existing-test migration:**
  - **Gate test:** `gin_test.go:435` — rename field access `builder.poisonErr` → `builder.tragicErr`, update the asserted substring `"builder poisoned"` → new tragic wording (e.g., `"builder closed by prior tragic failure"`); keep the test's field-level unit-test shape.
  - **Factor `runMergeWithRecover(fn func()) error`** as a package-level helper that runs `fn` under `defer recover()` and returns a `tragicErr`-shaped error if `fn` panics.
  - **Trigger test (new):** directly invoke `runMergeWithRecover` with a closure that `panic("simulated")`; assert the returned error has tragic wording and that a subsequent `AddDocument` on the same builder is refused by the gate.
  - **End-to-end refusal test (new):** go through a real merge path that panics (e.g., using the helper); assert subsequent `AddDocument` refused.

### Pre-existing Exceptions (documented, not decisions)
- **`walkJSON` at `builder.go:358`** is test-only internal (called only from `gin_test.go:2366` and `phase07_review_test.go:16`). It bypasses `validateStagedPaths` by design. Never reachable from user input. **No validator gate needed**; Phase 16 does not modify it. Document its role so reviewers don't flag it as a missed site.

### Claude's Discretion
- Exact wording of the new `tragicErr` wrap message in `AddDocument:305` (replacing today's `"builder poisoned by prior merge failure; discard and rebuild"`). Must remain Go-idiomatic, informative, and stable for this phase's tests.
- Exact structure of the log attributes on the `Logger.Error` emission (which fields beyond `panic_value` to include).
- Naming of the `runMergeWithRecover` helper (equivalent names acceptable).
- Exact CI grep invocation (awk/grep/rg all acceptable), as long as it fails `make lint` on reintroduction of `error` returns after the marker.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Scope and Constraints
- `.planning/ROADMAP.md` §Phase 16 — Goal, dependency chain (first phase of v1.2), and the six Success Criteria items (merge-signature change, validator extension, `tragicErr` rename + zero user-reachable sites, `recover()` belt-and-suspenders, atomicity property test ≥1000 docs ≥10% failing, `MUST_BE_CHECKED_BY_VALIDATOR` marker + CI grep).
- `.planning/REQUIREMENTS.md` §Atomicity — `ATOMIC-01` (byte-identical atomicity property test), `ATOMIC-02` (infallible merge by construction, validator simulates against real `pathData`), `ATOMIC-03` (`tragicErr` narrowed; `recover()` net; public catalog test).
- `.planning/PROJECT.md` §Current Milestone v1.2 — Validate-before-mutate (Strategy C) locked; Lucene contract as reference; `IngestError` and `IngestFailureMode` deferred to Phases 17 and 18.
- `.planning/STATE.md` §Current Position — Phase 16 is first phase of v1.2.

### Prior Phase Constraints
- `.planning/phases/13-parser-seam-extraction/13-CONTEXT.md` — Parser seam exists; parser errors return verbatim from `AddDocument`; `currentDocState` resets via `defer` means pre-merge parser/stage failures are already atomic.
- `.planning/phases/14-observability-seams/14-CONTEXT.md` — `gin.WithLogger` seam + silent-by-default; `Logger.Error` at the recover site is opt-in visibility consistent with that contract.
- `.planning/phases/15-experimentation-cli/15-CONTEXT.md` §Failure Handling — CLI-level `--on-error continue|abort` semantics; Phase 16's atomicity contract is what makes `continue` mode safe at the library level.

### Current Code Anchors (builder.go)
- `builder.go:34` — `poisonErr` field (rename target → `tragicErr`).
- `builder.go:304` — `AddDocument` entry; gate at line 305–307 wraps `poisonErr`.
- `builder.go:358` — `walkJSON` (test-only internal; documented pre-existing exception, not modified).
- `builder.go:416` — `stageScalarToken` (unsupported-type error site, pre-merge).
- `builder.go:440` — `stageMaterializedValue` (unsupported-type / uint-overflow error sites, pre-merge).
- `builder.go:518` — `stageCompanionRepresentations` (hard-mode transformer rejection, pre-merge).
- `builder.go:541` — `stageJSONNumberLiteral` (malformed numeric literal, pre-merge).
- `builder.go:597` — `stageNumericObservation` (validator-visible simulator; shadow-field writer).
- `builder.go:671` — `seedNumericSimulation` (shadow-field seed from real `pathData`; pattern to extend per D-01).
- `builder.go:697` — `mergeDocumentState` (today's validate-then-merge orchestrator; merge path is where `poisonErr` is set).
- `builder.go:724` — `validateStagedPaths` (extend target).
- `builder.go:743` — `mergeStagedPaths` (new signature: no `error` return; `recover()` site).
- `builder.go:799` — `mergeNumericObservation` (new signature: no `error` return; failure-hoist source).
- `builder.go:855` — `promoteNumericPathToFloat` (new signature: no `error` return; failure-hoist source).

### Test Anchors
- `gin_test.go:435` — existing `builder.poisonErr = ...` field-level gate test (migration target per D-06).
- `property_test.go` — existing gopter doc generators; atomicity property test composes with these.
- `phase07_review_test.go:16` — `walkJSON` caller (documented pre-existing exception).

### Phase 14 Seam
- `gin.go` — `WithLogger`, default silent logger.
- `logging/slogadapter/slog.go`, `logging/stdadapter/std.go` — adapter surfaces the `Logger.Error` emission at the recover site will flow through.

### Lucene Reference (per-document contract)
- Lucene `IndexWriter` JavaDoc — *"if an Exception is hit, the index will be consistent, but this document may not have been added"* (the phrase the milestone is named after).
- Lucene `IndexWriter.tragicEvent` — closed list of causes that close the writer (I/O, OOM, disk corruption). Our tragic surface is open-ended-via-`recover()` but contract-compatible.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **Shadow-fields pattern on `stagedPathData`** (`numericSim*` fields + `seedNumericSimulation` at `builder.go:671`): already implements "simulate against real `pathData` without mutating" for numeric observations. D-01 extends this pattern rather than introducing a new one.
- **`validateStagedPaths` skeleton** (`builder.go:724`): already creates a fresh `preview` state and replays `stageNumericObservation` against it. The hoisting work is mostly confined to adding the mixed-promotion pre-check that `mergeNumericObservation:839` does today; staging functions already route to the shadow-field checker.
- **`documentBuildState` + `currentDocState` + `defer` reset** (`builder.go:319–324`): ensures parser / stage failures never leak into shared state. Phase 16 relies on this existing guarantee for D-03's broad catalog.
- **Phase 14 Logger seam**: `WithLogger`, `slogadapter`, `stdadapter` — ready for the D-07 tragic-event emission with no new adapter code.

### Established Patterns
- **Two-phase validate/merge**: Phase 16 strengthens a pattern already present (lines 698–701), not invents one.
- **`errors.Wrap` with stack capture** via `github.com/pkg/errors`: the new `tragicErr` wrap message in the AddDocument gate and the validator-rejection messages all follow the repo's existing wrap style.
- **`Must*` constructors panic on invariant violation**: `MustNewRGSet`, `MustNewBloomFilter` are already in the merge path; any panic they raise is exactly what `recover()` needs to catch.

### Integration Points
- **Rename**: `poisonErr` → `tragicErr` across the builder struct, `AddDocument` gate, `mergeDocumentState`, and existing test (`gin_test.go:435`).
- **Signature change**: `mergeStagedPaths`, `mergeNumericObservation`, `promoteNumericPathToFloat` drop their `error` return; `mergeDocumentState` stops checking their errors.
- **New helper**: `runMergeWithRecover(fn func()) error` (package-level or method on builder), wrapping `mergeStagedPaths`. Called from `mergeDocumentState`.
- **Validator extension**: `validateStagedPaths` adds the mixed-promotion pre-check (simulate the merge's int-overflow-to-float path against the shadow-fields).
- **Log call** at the recover site, through `b.config.Logger` (or whichever field holds the wired logger).
- **New file**: `atomicity_test.go` for the property test + determinism sanity test + typed failure-intent generators.
- **Makefile `lint` target**: add the `MUST_BE_CHECKED_BY_VALIDATOR` grep check.

</code_context>

<specifics>
## Specific Ideas

- The Lucene two-catalog structure (tragic vs per-document exception) is **load-bearing**, not ceremonial. The reason to keep them separate is the same reason Lucene keeps them separate: different routing (caller sees per-doc; writer closes on tragic), different assurance bars.
- The merge-hoisting catalog is narrow today **by observation of the code**, not by conservative scoping. `git grep` on `errors.` inside the merge functions shows exactly the two `mergeNumericObservation:839` and `promoteNumericPathToFloat:860,867` sites. Future merge-layer additions enter via the marker convention.
- The `recover()` net is intentionally open-ended (catches any panic) rather than enumerated (Go can't type-filter panics the way Java does). This is stricter-by-exclusion than Lucene's explicit tragic list: anything not handled by validator-hoisting is tragic.
- Encode determinism is a **prerequisite** of the atomicity property test's `bytes.Equal` shape. The sanity test that encodes the same clean corpus twice runs FIRST so any non-determinism surfaces as its own failure.

</specifics>

<deferred>
## Deferred Ideas

- **Structured `IngestError` type with `Path`/`Layer`/`Cause`/`Value` fields** — Phase 18 (IERR-01..03).
- **Unified `IngestFailureMode` (`Hard`/`Soft`) across parser / transformer / numeric layers** — Phase 17 (FAIL-01..02).
- **Telemetry counter for tragic events** (`gin_builder_tragic_total` or similar) — considered and deferred. Phase 17/18 can revisit with a structured signal.
- **Signals seam emission on tragic** — same rationale; deferred with Telemetry.
- **Layer-prefix on error wording (`"numeric: …"`)** — considered as a Phase 18 migration smoother; rejected in favor of Phase 18 designing the structured shape deliberately.
- **File-level "no error returns" convention with merge functions moved to `builder_merge.go`** — considered; rejected in favor of function-level marker to minimize diff in this phase.
- **`recover()` wrapping `mergeDocumentState` (broader net)** — considered; rejected in favor of the narrow `mergeStagedPaths`-only scope specified by ATOMIC-03.
- **`ValidateDocument` dry-run public API** — already deferred at the milestone level (PROJECT.md §Out of Scope).

</deferred>

---

*Phase: 16-adddocument-atomicity-lucene-contract*
*Context gathered: 2026-04-23*
