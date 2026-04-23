# Requirements: GIN Index v1.2 Ingest Correctness & Per-Document Isolation

**Defined:** 2026-04-23
**Core Value:** Bring `AddDocument` in line with the Lucene per-document contract — a failed ingest leaves the builder consistent and usable; only genuinely unrecoverable internal-invariant violations close the builder. Make the failure observable to callers through a unified failure-mode taxonomy and a structured error type.

## Scope Note

v1.2 is correctness-first. The current `AddDocument` (`builder.go:304`) collapses every merge failure into a "poisoned" builder, even for class-1 failures (malformed input, schema mismatch, transformer rejection) that mature inverted-index libraries (Lucene, Elasticsearch, Tantivy) treat as per-document and isolated.

The fix is structural, not cosmetic: extend the existing two-phase `validateStagedPaths` / `mergeStagedPaths` pattern (`builder.go:724`, `builder.go:743`) so the merge step becomes infallible by construction. The "tragic" path (renamed from `poisonErr`) survives but narrows to genuinely unrecoverable internal-invariant violations.

Once per-document failure is a first-class concept, the milestone unifies the failure-mode taxonomy across parser, transformer, and numeric layers, and exposes a structured `IngestError` so callers can act on failures programmatically.

## Requirements

### Atomicity (Lucene contract)

- [ ] **ATOMIC-01**: `AddDocument` returning a non-tragic error leaves the builder in a state indistinguishable from never having received the failed call. Verified by an atomicity property test that ingests a corpus, interleaves guaranteed-failing documents, and asserts the encoded index is byte-identical to the same corpus without the failures.
- [ ] **ATOMIC-02**: `mergeStagedPaths` and `mergeNumericObservation` become infallible — `validateStagedPaths` is extended to fully simulate every reason these merge functions could fail, against the *real* `pathData` state (not a fresh preview). The merge-layer error returns are removed and a compile-time check enforces the new signatures.
- [ ] **ATOMIC-03**: `tragicErr` (renamed from `poisonErr` at `builder.go:34`) is reserved for internal-invariant violations only; no user-input failure mode reaches it. A `recover()`-in-merge belt-and-suspenders converts any reachable panic to `tragicErr`. A unit-test allowlist enforces that `tragicErr` stays nil across the full public failure-mode catalog.

### Failure-Mode Taxonomy

- [ ] **FAIL-01**: Unified `IngestFailureMode` type (`Hard`/`Soft`) replaces the existing `TransformerFailureMode` constants (deliberate breaking rename for clarity over convenience) and extends to parser and numeric-promotion layers. CHANGELOG flags this as a breaking API change with a one-line migration note.
- [ ] **FAIL-02**: New config knobs `WithParserFailureMode(mode)` and `WithNumericFailureMode(mode)` added; default `Hard` for both, preserving current behavior. `Soft` mode skips the failing document at the configured layer and returns no error to the caller.

### Structured IngestError

- [ ] **IERR-01**: Exported `IngestError` type carries `Path` (the JSONPath where the failure happened), `Layer` (parser / transformer / numeric / schema), `Cause` (the wrapped underlying error), and `Value` (verbatim string repr of the offending value — caller redacts as needed; the library does not redact). `errors.As`-friendly extraction tested.
- [ ] **IERR-02**: All ingest-error sites (parser, transformer, numeric promotion) wrap their underlying error in `IngestError` with the four fields populated. Round-trip extraction via `errors.As` is covered by a per-layer test matrix.
- [ ] **IERR-03**: `gin-index experiment --on-error continue` (shipped in Phase 15) reports per-document failures grouped by `Layer` plus a sample of the first N `IngestError`s with structured fields, in both text and `--json` output modes. Golden-tested.

## Out of Scope (deferred to a future milestone)

| Feature | Reason |
|---------|--------|
| `ValidateDocument` dry-run API | Powerful capability that becomes possible once `AddDocument` is atomic; deserves its own milestone with a real consumer in mind, not landed speculatively |
| Builder snapshot / restore for batch ingestion | Over-engineered without a user request; the validate-before-mutate strategy makes per-document atomicity sufficient for the foreseeable use cases |
| Snapshot-and-restore atomicity (Strategy A from brainstorming) | Held in reserve only if a future failure mode genuinely cannot be pre-validated; not built now |
| Bloom `AddString` allocation cleanup | Perf-shaped; routed to backlog (new 999.x entry) per the project's "profile before optimizing" precedent (999.5) |
| Per-path opt-out for `[*]` array wildcard | Disconnected from the correctness story; routed to backlog |
| All other v1.1 follow-ons (perf, transformer registry expansion, etc.) | Outside the correctness theme |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| ATOMIC-01 | Phase 16 | Planned |
| ATOMIC-02 | Phase 16 | Planned |
| ATOMIC-03 | Phase 16 | Planned |
| FAIL-01 | Phase 17 | Planned |
| FAIL-02 | Phase 17 | Planned |
| IERR-01 | Phase 18 | Planned |
| IERR-02 | Phase 18 | Planned |
| IERR-03 | Phase 18 | Planned |

**Coverage:**
- Requirements total: 8
- Mapped to phases: 8
- Unmapped: 0

---
*Requirements defined: 2026-04-23 for milestone v1.2 Ingest Correctness & Per-Document Isolation*
*Architectural strategy: validate-before-mutate (Strategy C from brainstorming), with Lucene's per-document contract as the target — see `.planning/research/v1.2-atomicity-precedents.md` if generated, or the brainstorming transcript for industry-precedent grounding (Lucene IndexWriter, Tantivy, RocksDB, PostgreSQL GIN, Bleve).*
*v1.1 (Phases 13–15) requirements live in MILESTONES.md and ROADMAP.md historical sections.*
