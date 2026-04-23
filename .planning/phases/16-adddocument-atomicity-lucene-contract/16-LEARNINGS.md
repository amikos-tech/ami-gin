---
phase: 16
phase_name: "AddDocument Atomicity (Lucene contract)"
project: "GIN Index"
generated: "2026-04-23T14:21:35Z"
counts:
  decisions: 6
  lessons: 6
  patterns: 5
  surprises: 6
missing_artifacts: []
---

# Phase 16 Learnings: AddDocument Atomicity (Lucene contract)

## Decisions

### Keep Validation As The User-Error Boundary
`validateStagedPaths` remains the user-error return point for unsupported mixed numeric promotion. The merge layer was changed to assume validation has already proven the state safe to commit.

**Rationale:** The focused pre-check showed `validateStagedPaths` already covered both lossy mixed numeric promotion directions before signature edits, so changing validator behavior was unnecessary.
**Source:** 16-01-SUMMARY.md

---

### Make Merge Functions Infallible By Signature
`mergeStagedPaths`, `mergeNumericObservation`, and `promoteNumericPathToFloat` no longer return `error`; missed validator cases panic as internal invariant failures.

**Rationale:** ATOMIC-02 required ordinary user failures to be hoisted before shared state mutation, leaving merge as a validator-backed commit step.
**Source:** 16-01-PLAN.md

---

### Reserve `tragicErr` For Internal Invariants
The terminal builder state was renamed from `poisonErr` to `tragicErr` and narrowed to internal invariant violations or recovered merge panics.

**Rationale:** Public input failures should be isolated per document, while a true internal merge invariant failure closes the builder and preserves the original cause for later refusal errors.
**Source:** 16-02-SUMMARY.md

---

### Recover Only Around `mergeStagedPaths`
`runMergeWithRecover` wraps only the merge call inside `mergeDocumentState`, not parsing, staging, validation, or the whole `AddDocument` method.

**Rationale:** The recovery surface is limited to the internal commit phase where validator-missed invariants can panic, keeping ordinary public failures on their normal error paths.
**Source:** 16-02-SUMMARY.md

---

### Use Serialized Bytes As The Atomicity Oracle
The atomicity property compares encoded output from a full attempted ingest against a clean-only ingest, preserving original successful `DocID` values.

**Rationale:** Byte-identical `Encode` output checks the serialized header, path/index payloads, and `DocIDMapping`, which is stronger than query-equivalence-only assertions.
**Source:** 16-03-SUMMARY.md

---

### Enforce Validator Markers With Makefile And CI Checks
The marker policy was implemented as a POSIX awk Makefile target wired into local `make lint`, with a separate CI lint-job step running the same target.

**Rationale:** The existing CI lint action bypassed `make lint`, so CI needed an explicit marker check without replacing the existing golangci-lint action.
**Source:** 16-04-SUMMARY.md

---

## Lessons

### Staging Can Reject Before Validator Isolation
`stageJSONNumberLiteral` already rejects lossy promotion against existing builder state before `validateStagedPaths` can be isolated in a direct unit test.

**Context:** The validator tests had to seed `staged.numericValues` manually so they could exercise `validateStagedPaths` directly.
**Source:** 16-01-SUMMARY.md

---

### Negative Source-Grep Checks Affect Test Shape
A test that wants to assert a forbidden key is absent can accidentally violate the source-level grep by spelling the forbidden marker in the test itself.

**Context:** The recovery logging test constructed the disallowed key as `"panic" + "_value"` so the assertion remained while the source-level forbidden-marker check stayed meaningful.
**Source:** 16-02-SUMMARY.md

---

### Atomicity Tests Need Serializable Transformer Metadata
Runtime-only custom transformers cannot be used in an `Encode`-based atomicity oracle when their representation metadata is not serializable.

**Context:** The initial helper used `WithCustomTransformer`, and `Encode` rejected the runtime-only representation; the test switched to the registered email-domain transformer under alias `strict`.
**Source:** 16-03-SUMMARY.md

---

### Heavy Properties Need Bounded Iteration Budgets
The full atomicity property ingests roughly 2000 documents and performs two `Encode` calls per property iteration, so iteration count must be deliberately bounded.

**Context:** `TestAddDocumentAtomicity` uses `propertyTestParametersWithBudgets(50, 10)` while keeping 1000 attempted documents per generated corpus.
**Source:** 16-03-PLAN.md

---

### Integration Gates Catch Cross-Plan Friction
Plan-local verification can pass while the later combined integration gate exposes unrelated or concurrently introduced lint issues.

**Context:** `make lint` initially failed on a `goconst` finding in `gin_test.go`; the issue was resolved during the Wave 2 integration gate by extracting a shared test constant.
**Source:** 16-04-SUMMARY.md

---

### Tooling Assumptions Need A Direct Fallback
The expected `gsd-sdk query` workflow command was unavailable in this checkout, so planning metadata and state updates had to be applied directly.

**Context:** The summaries for plans 16-02 and 16-03 both record direct updates because `gsd-sdk query` was not available.
**Source:** 16-02-SUMMARY.md

---

## Patterns

### Validator-Backed Commit Function
Use a validator to simulate every known user-reachable commit failure against real builder state, then make the commit function return no ordinary user errors.

**When to use:** Apply this when a mutable commit step must be atomic and its possible user failures can be prevalidated.
**Source:** 16-01-PLAN.md

---

### Narrow Tragic Recovery Wrapper
Wrap only the internal merge callback with `recover`, convert recovered panics to a terminal builder error, and skip document bookkeeping when recovery returns an error.

**When to use:** Use this for internal invariant boundaries where panics should not crash callers but should close the corrupted builder.
**Source:** 16-02-PLAN.md

---

### Full-Vs-Clean Encoded-Byte Oracle
Build one index from all attempted documents and another from only successful documents, then require identical encoded bytes with the same row-group count and original document IDs.

**When to use:** Use this when failed-operation isolation must be proven across all serialized state, not just observable query behavior.
**Source:** 16-03-PLAN.md

---

### Public Failure Catalog Guard
Cover each public failure category with tests that assert an error is returned, `tragicErr` stays nil, document bookkeeping does not advance, and a later valid document remains acceptable where practical.

**When to use:** Use this when a new failure taxonomy needs a regression guard that ordinary caller errors stay recoverable.
**Source:** 16-03-SUMMARY.md

---

### Lightweight Static Policy In Makefile
Use a Makefile target for project-specific source policy that golangci-lint does not express directly, then wire it into both local lint and CI.

**When to use:** Use this for small, repository-specific invariants such as marker placement, expected function names, and return-signature checks.
**Source:** 16-04-PLAN.md

---

## Surprises

### Validator Was Already Complete For Numeric Promotion
The pre-check showed `validateStagedPaths` already rejected both unsafe mixed numeric promotion directions.

**Impact:** The validator body stayed unchanged; the implementation focused on signature changes, markers, invariant panics, and tests.
**Source:** 16-01-SUMMARY.md

---

### Planned Validator Test Path Hit Earlier Staging Logic
The original plan-directed test path returned `unsupported mixed numeric promotion at $.score` from `stageJSONNumberLiteral` before reaching `validateStagedPaths`.

**Impact:** The test was reshaped to seed staged numeric observations directly, preserving the intended validator-level proof.
**Source:** 16-01-SUMMARY.md

---

### Recovery Test Contained The Forbidden Marker It Was Checking
The initial recovery logging test included the literal forbidden key while asserting that key was not logged.

**Impact:** The assertion was preserved by constructing the key string dynamically, and the grep check remained strict.
**Source:** 16-02-SUMMARY.md

---

### Runtime Transformer Metadata Blocked Encoding
The encode determinism helper first used a custom transformer, but `Encode` rejected the runtime-only companion representation.

**Impact:** The atomicity test switched to a registered transformer so the strict alias remained serializable.
**Source:** 16-03-SUMMARY.md

---

### Lint Failed Outside The Plan 16-04 Write Scope
`make lint` initially failed on repeated `unsupported mixed numeric promotion at $.score` literals in `gin_test.go`, which plan 16-04 did not own.

**Impact:** Plan 16-04 kept its owned file scope, then the orchestrator resolved the lint blocker during integration.
**Source:** 16-04-SUMMARY.md

---

### Concurrent Wave Commits Interleaved
Plan 16-02 and plan 16-04 executed concurrently and their commits interleaved.

**Impact:** Final commit scopes were checked and kept limited to the owned files for each plan.
**Source:** 16-02-SUMMARY.md
