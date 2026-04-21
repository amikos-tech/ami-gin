---
phase: 07
phase_name: "builder-parsing-numeric-fidelity"
project: "GIN Index — v1.0 Query & Index Quality"
generated: "2026-04-17T00:00:00Z"
counts:
  decisions: 5
  lessons: 4
  patterns: 5
  surprises: 4
missing_artifacts:
  - "07-VERIFICATION.md"
  - "07-UAT.md"
---

# Phase 07 Learnings: builder-parsing-numeric-fidelity

## Decisions

### Document Ingest Stays Transactional
`AddDocument()` was kept transactional by staging per-document observations and merging them only after parse and validation succeed.

**Rationale:** This closes the atomic-failure gap so unsupported numeric values cannot partially mutate builder state.
**Source:** 07-builder-parsing-numeric-fidelity-01-SUMMARY.md

---

### Int-Only Numeric Paths Stay Exact
Int-only path stats were stored as exact `int64` values, and promotion to float mode was allowed only when every integer stayed exact inside `float64`.

**Rationale:** Numeric fidelity needed to become an explicit contract instead of remaining an accident of generic float decoding.
**Source:** 07-builder-parsing-numeric-fidelity-01-SUMMARY.md

---

### Transformer Semantics Were Preserved Through Explicit Parsing
Transformer-targeted subtrees were normalized to the legacy input shapes before applying the transformer, and transformed outputs were then reclassified through the explicit numeric path.

**Rationale:** The parser refactor could not break existing transformer expectations while tightening numeric semantics.
**Source:** 07-builder-parsing-numeric-fidelity-01-SUMMARY.md

---

### Numeric Format Evolution Was Made Explicit
The binary format version was bumped when the new int-only numeric metadata was added.

**Rationale:** Decode behavior needed to remain explicit once exact-int numeric fields became part of persisted index state.
**Source:** 07-builder-parsing-numeric-fidelity-01-SUMMARY.md

---

### Legacy Benchmark Control Remains In-Repo and Benchmark-Only
The pre-Phase-07 ingest path was kept as a benchmark-only control inside `benchmark_test.go` instead of reviving it in production code or relying on historical notes.

**Rationale:** BUILD-05 needed reproducible same-branch parser comparisons without reintroducing obsolete runtime behavior.
**Source:** 07-builder-parsing-numeric-fidelity-02-SUMMARY.md

---

## Lessons

### Exact-Int Refactors Can Break Legacy Float Expectations
Even after adding exact-int fields, older callers and tests can still depend on float globals and pre-refactor encoded layouts.

**Context:** Full-suite verification exposed that int-only numeric indexes had dropped legacy `GlobalMin` / `GlobalMax` expectations and that the numeric decode bounds test still assumed the old binary layout.
**Source:** 07-builder-parsing-numeric-fidelity-01-SUMMARY.md

---

### Performance Tradeoffs Became Visible, Not Solved
The explicit parser improved correctness and control flow, but it still showed higher allocation pressure on the wide-flat and transformer-heavy build paths.

**Context:** The final benchmark harness established a reproducible baseline showing those shapes remained the main optimization targets for future work.
**Source:** 07-builder-parsing-numeric-fidelity-02-SUMMARY.md

---

### Execution Prep Matters on Long-Lived Branches
Older milestone branches may need a sync or merge before phase work can begin cleanly.

**Context:** Execution hit a stale branch state and required a one-time sync from `main` before the Phase 07 implementation could proceed.
**Source:** 07-builder-parsing-numeric-fidelity-01-SUMMARY.md

---

### Inline Execution Is the Safe Fallback When Orchestration Stalls
If the subagent path does not return reliable completion signals, the work should continue inline under the orchestrator.

**Context:** The Phase 07 execution path did not produce completion signals in this runtime, so the implementation was completed directly in the orchestrator session.
**Source:** 07-builder-parsing-numeric-fidelity-01-SUMMARY.md

---

## Patterns

### Sparse Streaming Parse Pattern
Stream JSON objects by default and materialize only transformer-targeted subtrees and array items that must feed both indexed and wildcard paths.

**When to use:** Use this when you need explicit parsing and transactional staging without falling back to whole-document `any` decoding.
**Source:** 07-builder-parsing-numeric-fidelity-01-SUMMARY.md

---

### Merge-On-Success Document Staging
Collect document-local observations in a staging structure and merge them into shared builder state only after the document has fully parsed and validated.

**When to use:** Use this when late validation failures must leave no partial state behind.
**Source:** 07-01-PLAN.md

---

### Explicit Numeric Mode Contract
Treat numeric mode as a stable end-to-end contract: `ValueType 0 = int-only`, `ValueType 1 = float-or-mixed` across build, query, and serialization.

**When to use:** Use this when numeric semantics need to survive code-path boundaries and encode/decode round trips.
**Source:** 07-builder-parsing-numeric-fidelity-01-SUMMARY.md

---

### Parser Benchmark Labeling Convention
Benchmark labels use `parser=/docs=/shape=` segments so parser mode, document count, and fixture shape stay obvious in historical comparisons.

**When to use:** Use this when benchmark output needs to remain mechanically comparable over time.
**Source:** 07-builder-parsing-numeric-fidelity-02-SUMMARY.md

---

### Shared-Fixture Parser Delta Benchmarking
Run legacy and explicit parser benchmarks against the same deterministic fixtures and doc counts.

**When to use:** Use this when benchmark deltas should be attributable to ingest-path changes instead of fixture drift.
**Source:** 07-builder-parsing-numeric-fidelity-02-SUMMARY.md

---

## Surprises

### Exact-Int Support Broke Old Float Globals
After the exact-int refactor landed, int-only numeric indexes no longer satisfied some legacy expectations around float globals.

**Impact:** A compatibility fix was needed to populate float global min/max alongside the new exact-int globals and to update decode-bounds coverage to the new layout.
**Source:** 07-builder-parsing-numeric-fidelity-01-SUMMARY.md

---

### The Starting Branch Was Staler Than Expected
The phase could not start cleanly from the existing milestone branch state.

**Impact:** A one-time sync or merge from `main` was required before implementation could begin.
**Source:** 07-builder-parsing-numeric-fidelity-01-SUMMARY.md

---

### The Subagent Path Was Not Reliable in This Runtime
The execution workflow did not receive completion signals back from the subagent path.

**Impact:** The implementation had to be completed inline instead of relying on the normal delegated execution loop.
**Source:** 07-builder-parsing-numeric-fidelity-01-SUMMARY.md

---

### Correctness Work Still Left Costly Shapes
The new parser path remained noticeably more allocation-heavy on wide-flat and transformer-heavy workloads.

**Impact:** Phase 07 finished with a correctness and measurement win, but it also established concrete optimization targets for later phases rather than eliminating those costs immediately.
**Source:** 07-builder-parsing-numeric-fidelity-02-SUMMARY.md
