# Phase 16: AddDocument Atomicity (Lucene contract) - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in `16-CONTEXT.md` — this log preserves the alternatives considered.

**Date:** 2026-04-23
**Phase:** 16-adddocument-atomicity-lucene-contract
**Areas discussed:** Validator simulation strategy, Merge-layer failure catalog, Contract enforcement mechanism, Atomicity property test & existing-test migration, Tragic-recover observability, Pre-parser error placement, Error-message wording

---

## Validator Simulation Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Extend shadow-fields pattern | Add new `*Sim*` fields to `stagedPathData` per new merge-reason; staging re-seeds from real `pathData` on first touch. Incremental, consistent with existing numeric-promotion code. | ✓ |
| Typed pathPreview clone | Throwaway `pathPreview` struct mirroring `pathBuildData`; validator clones lazily, shared simulation logic. Clean separation but refactor cost and parallel-shape maintenance. | |
| Dry-run shared helper with commit flag | Merge-layer helpers take `commit bool`. Maximum DRY, but conditional writes hide the mutation site — exactly what this phase wants to make obvious. | |

**User's choice:** Extend shadow-fields pattern (Recommended).
**Notes:** Consistent with the existing `numericSim*`/`seedNumericSimulation` pattern at `builder.go:671`; narrow scope suits today's 1-item merge-hoisting catalog.

---

## Merge-Layer Failure Catalog

| Option | Description | Selected |
|--------|-------------|----------|
| Narrow merge catalog, broad tragic-stays-nil catalog | Merge-hoisting = numeric promotion only; `MUST_BE_CHECKED_BY_VALIDATOR` marker on merge fns; tragic-stays-nil test iterates the full public failure-mode catalog including pre-merge sites. Mirrors Lucene's two-catalog structure. | ✓ |
| Unified 'atomicity catalog' | One flat table across parse/stage/validate/merge. Single source of truth, but conflates contracts Lucene explicitly keeps separate (per-doc exception vs tragic event). | |
| Merge catalog only; defer broader taxonomy to Phase 17 | Phase 16 hoists only numeric promotion; tragic-stays-nil test minimal; Phase 17 owns broader. Conflicts with ATOMIC-03 explicit scope. | |

**User's choice:** Narrow merge catalog + broad tragic-stays-nil catalog (Recommended, reframed under Lucene lens).
**Notes:** User explicitly requested the Lucene lens. The Lucene `IndexWriter` maintains two separate catalogs (per-doc exceptions vs `tragicEvent`) with different routing semantics; merging them conflates the contracts. Option A mirrors that structure.

---

## Contract Enforcement Mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| Compiler + marker + grep + merge-scoped recover | Drop `error` from merge signatures (compiler-enforced); `// MUST_BE_CHECKED_BY_VALIDATOR` marker; CI grep rejects `error` on marked functions; `recover()` wraps `mergeStagedPaths` only. | ✓ |
| Compiler + merge-only file + file-level rule + mergeDocumentState recover | Move merge fns into `builder_merge.go` with file-level no-error rule; `recover()` wraps `mergeDocumentState`. More churn; broader recover scope than ATOMIC-03 specifies. | |
| Compiler-only minimalism | No marker, no grep, no reorganization; rely on compiler + review. Violates ATOMIC-03's explicit CI-grep requirement. | |

**User's choice:** Compiler + marker + grep + merge-scoped recover (Recommended).
**Notes:** Matches ATOMIC-03 wording precisely ("`recover()`-in-merge"). Marker is the future-proofing layer; compiler catches today's reintroduction; grep catches tomorrow's new merge-layer function that forgets the marker.

---

## Atomicity Property Test Shape

| Option | Description | Selected |
|--------|-------------|----------|
| Typed failure-intent generators + bytes.Equal on Encode | 3 generators (ParserMalformedDoc, HardTransformerRejectingDoc, NumericPromotionFailingDoc); compose with existing clean-doc generator; `bytes.Equal` on encoded output; encode-determinism sanity test prerequisite. | ✓ |
| Reuse existing generators + structural decode compare | Reuse `property_test.go` generators with byte-corruption post-processing; decode + struct-equal. Weaker guarantee than roadmap's "byte-identical" wording; random corruption can yield valid JSON. | |
| Typed generators + decode compare hybrid | Typed generators but decode-compare assertion. Violates roadmap's literal "byte-identical encoded" wording. | |

**User's choice:** Typed failure-intent generators + `bytes.Equal` on `Encode` (Recommended).
**Notes:** Encode-determinism sanity test runs FIRST to isolate encoder non-determinism from atomicity violations in failure diagnosis.

---

## Existing-Test Migration (`gin_test.go:435`)

| Option | Description | Selected |
|--------|-------------|----------|
| Rename-migrate gate test + testable recover helper + trigger test | Rename field + message; factor `runMergeWithRecover(func())`; add unit test for recover path; add end-to-end refusal test. | ✓ |
| Rename-only migration + minimal recover coverage | Rename only; recover coverage via `export_test.go` inline panic trigger. No helper refactor. Smaller diff but tighter coupling. | |
| Replace field-level gate test with end-to-end only | Delete field-level test; one end-to-end test covers gate + trigger. Couples gate to trigger plumbing. | |

**User's choice:** Rename-migrate gate test + testable recover helper + trigger test (Recommended).
**Notes:** Helper factoring gives the recover path a direct testable seam without exporting internals through `export_test.go`.

---

## Tragic Recover() + Phase 14 Logger Seam

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, Logger at Error level | Single `Logger.Error` call at the recover site; silent by default; opt-in visible. Zero new surface. | ✓ |
| Yes, Logger + Telemetry counter | Log + `gin_builder_tragic_total` counter. More signals; more surface to maintain. | |
| Silent, defer to Phase 18 | No logging; callers log on receiving tragic error. Missed signal for the single most log-worthy builder event. | |

**User's choice:** Logger at Error level only (Recommended).
**Notes:** Silent-by-default semantics of Phase 14 preserved; Telemetry counter deferred to Phase 17/18.

---

## Pre-Parser Error Placement

| Option | Description | Selected |
|--------|-------------|----------|
| Broad tragic-stays-nil catalog + 'pre-stage atomic' doc note | All four (position-exceeds-numRGs, BeginDocument not-called / wrong-count, rgID-mismatch) listed in tragic-stays-nil test; documented as atomic-by-construction (pre-stage). | ✓ |
| Split: position → tragic-stays-nil; sink-contract → tragic candidate | Sink-contract treated as internal invariant. Buggy custom parser would close builder for everyone. | |
| All four out of scope of both catalogs | Don't mention. Reduces exhaustiveness claim. | |

**User's choice:** Broad catalog + documented as pre-stage atomic (Recommended).

---

## Error-Message Wording for Hoisted Numeric-Promotion Failure

| Option | Description | Selected |
|--------|-------------|----------|
| Mirror today's exact wording | `"unsupported mixed numeric promotion at %s"` — zero churn; Phase 18 migrates to structured `IngestError`. | ✓ |
| Restructure wording now for clarity | More informative ("cannot promote int min/max to float64 because …"). Breaks test assertions; Phase 18 rewrites again. | |
| Add a layer prefix ("numeric: …") | Smooths Phase 18 grep-based migration. Mild churn; Phase 18 can design `Layer` constant more deliberately. | |

**User's choice:** Mirror today's exact wording (Recommended).

---

## Claude's Discretion

- Exact wording of the new `tragicErr` wrap message at `AddDocument:305` (replacing `"builder poisoned by prior merge failure; discard and rebuild"`).
- Exact structure of `Logger.Error` attributes at the recover site (beyond `panic_value`).
- Name of the `runMergeWithRecover` helper.
- Exact CI grep shell invocation.

## Deferred Ideas

- Structured `IngestError` type → Phase 18 (IERR-01..03).
- `IngestFailureMode` (`Hard`/`Soft`) taxonomy → Phase 17 (FAIL-01..02).
- Telemetry counter for tragic events.
- Signals seam emission on tragic.
- Layer-prefix on error wording.
- Merge-only file + file-level "no error returns" rule.
- `recover()` wrapping `mergeDocumentState` (broader net).
- `ValidateDocument` dry-run public API (milestone-level deferral).
