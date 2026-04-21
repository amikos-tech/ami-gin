---
phase: 07
reviewers:
  - gemini
  - claude
reviewed_at: 2026-04-15T10:15:16Z
plans_reviewed:
  - 07-01-PLAN.md
  - 07-02-PLAN.md
reviewer_status:
  gemini: completed
  claude: failed_not_logged_in
---

# Cross-AI Plan Review - Phase 07

## Gemini Review

This review evaluates the implementation plans for **Phase 07: Builder Parsing & Numeric Fidelity**. The plans are well-aligned with the phase goals of reducing ingest overhead, ensuring `int64` fidelity, and achieving transactional document safety.

### Plan 01: Ingest Pipeline & Numeric Fidelity

**Summary**
Plan 01 delivers a major architectural upgrade to the builder, moving from eager, lossy JSON unmarshaling to a transactional, streaming parser. By introducing a document-local staging area and explicit integer classification, it addresses the "partial mutation" bug and the "float64 rounding" limitation in a single wave.

**Strengths**
- **Atomic Ingest:** The use of a `documentBuildState` accumulator is the correct architectural choice to fulfill the "no partial mutation" requirement.
- **IEEE-754 Awareness:** Explicitly checking the `[-2^53, 2^53]` boundary for float promotion prevents silent precision loss on large integers.
- **Transformer Continuity:** The "subtree materialization" strategy for paths with transformers is a pragmatic compromise that preserves the current API without sacrificing the performance of the streaming parser for the rest of the document.
- **Backward Compatibility:** Redefining `ValueType` while maintaining the float fields ensures that existing query patterns and serialized artifacts remain functional.

**Concerns**
- **Memory Pressure (MEDIUM):** While streaming reduces the allocation of `any` trees, the `documentBuildState` accumulator will still grow with document size. For extremely wide documents, this staging area might increase peak memory usage compared to the current eager walk.
- **Query Complexity (LOW):** Adding `int64` comparisons to `query.go` adds branching logic to the hot path. However, this is necessary for correctness and likely offset by the parser gains.
- **Transformer Performance (LOW):** Paths with transformers still require materialization. While this is the "correctness first" approach, it means the performance win will be less pronounced for transformer-heavy schemas.

**Suggestions**
- **Accumulator Pooling:** Consider using a `sync.Pool` for the `documentBuildState` structures to minimize garbage collection pressure during high-throughput ingest.
- **Path Lookup Optimization:** Since `normalizeWalkPath` and transformer checks happen for every field, ensure the path lookup is as fast as possible (perhaps via a small LRU or string-to-ID cache if path strings are long).

---

### Plan 02: Performance Benchmarking

**Summary**
Plan 02 provides the empirical foundation required by `BUILD-05`. By preserving a legacy control path within the benchmark file itself, it enables high-fidelity "before vs. after" comparisons that are independent of git history or external environment shifts.

**Strengths**
- **In-repo Control:** Including `benchmarkAddDocumentLegacy` is excellent for reproducibility. It allows maintainers to verify performance claims on any machine with a single `go test` command.
- **Clear Taxonomy:** The use of `parser=`, `docs=`, and `shape=` labels in sub-benchmarks makes it easy to parse results and identify exactly where the new parser excels or regresses.
- **Fixture Determinism:** Hard-coded literals (like `1<<63 - 1`) ensure the benchmarks exercise the exact edge cases the phase is designed to solve.

**Concerns**
- **Legacy Bitrot (LOW):** The legacy benchmark helper must be carefully maintained if internal builder structures change significantly, or it will cease to be a valid comparison.
- **Fixture Coverage (LOW):** Ensure the "transformer-heavy" fixture actually triggers several different types of transformers (Regex, Date, etc.) to test the materialization overhead.

**Suggestions**
- **Automated Comparison:** You might consider adding a small helper that calculates the percentage delta between the legacy and explicit paths and prints it during the benchmark run using `b.ReportMetric`.

---

### Risk Assessment: LOW

The overall risk is **LOW**. The plans are surgically targeted at the requirements and do not introduce significant new public API surface. The "Research -> Plan -> Validate" cycle is well-represented, especially the focus on atomic failure and numeric boundary conditions.

**Justification:**
- The technical approach (Decoder + Staging) is the industry standard for high-performance JSON processing in Go.
- The numeric fidelity strategy respects the limitations of the existing float-backed index while adding exactness where it matters.
- Validation is thorough, covering both functional correctness (regressions) and performance (benchmarks).

---

## Claude Review

Review could not be completed in this environment.

Failure details:
- CLI status: available
- Auth status: not logged in
- Non-interactive check output: `Not logged in - Please run /login`

---

## Consensus Summary

Cross-review confidence is limited because only one requested external reviewer completed successfully.

### Agreed Strengths

No multi-review agreement was established because Claude did not complete. The completed Gemini review highlighted these as the strongest parts of the phase plan:
- The staged `documentBuildState` approach directly addresses the atomic-ingest requirement.
- The numeric strategy preserves exact `int64` handling without broadening Phase 07 into a larger decimal redesign.
- The benchmark plan is reproducible because it keeps a legacy control path in-repo with explicit benchmark naming.

### Agreed Concerns

No multi-review agreement was established because Claude did not complete. The highest-priority concerns still worth feeding back into planning are:
- Medium: document-local staging may increase peak memory on very wide documents even if it lowers overall allocation churn.
- Low: transformer-heavy paths may see smaller ingest wins because they still require materialization.
- Low: the benchmark-only legacy helper can drift if production builder internals change without the control path being kept in sync.

### Divergent Views

None recorded. A second independent review did not complete, so there is no basis for identifying disagreement yet.

### Recommended Follow-up

- Retry the Claude review after authenticating the `claude` CLI with `/login`.
- Consider whether `documentBuildState` pooling is worth adding to the plan or at least to the research notes as a validation question.
- Make sure the benchmark fixtures exercise multiple transformer categories so the materialization cost is measurable rather than assumed.
