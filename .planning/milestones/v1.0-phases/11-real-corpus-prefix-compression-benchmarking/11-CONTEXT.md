# Phase 11: Real-Corpus Prefix Compression Benchmarking - Context

**Gathered:** 2026-04-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Validate the real-world payoff of Phase 10's prefix-compacted string serialization on a representative external corpus without reopening the wire-format design. This phase covers corpus selection, bounded-versus-opt-in benchmark tiers, and the evidence/reporting shape needed to say clearly where compaction helps, where it is flat, and whether broader format work is justified.

</domain>

<decisions>
## Implementation Decisions

### Corpus sourcing and ownership
- **D-01:** Phase 11 should keep one small bounded fixture in-repo for reproducible smoke coverage, while larger corpus runs stay outside the repo.
- **D-02:** Larger real-corpus runs should use on-demand downloads rather than vendoring a large external dataset into the repository.
- **D-03:** Hugging Face is the preferred source class for the larger JSON/JSONL corpus used in this phase.
- **D-04:** The primary large external corpus is `common-pile/github_archive`.

### Corpus mix and comparison shape
- **D-05:** Phase 11 should use a single external corpus, not a multi-corpus comparison matrix.
- **D-06:** The "helps vs flat" conclusion should come from contrasting field families or subsets inside `common-pile/github_archive`, not from introducing a second external corpus.

### Scale tiers and activation
- **D-07:** Define three benchmark tiers: `smoke`, `subset`, and `large`.
- **D-08:** Only `smoke` belongs in the normal benchmark surface; `subset` and `large` are explicit opt-in runs.
- **D-09:** Opt-in activation should be env-var gated only.
- **D-10:** Opt-in tiers should skip cleanly when the required external-corpus env vars are absent, and only fail when the user explicitly opted in but configured the dataset path incorrectly.

### Evidence and reporting
- **D-11:** Reuse the existing Phase 10 metric style where practical, including raw string-section deltas and final encoded artifact size.
- **D-12:** Phase 11 must produce a checked-in narrative report in addition to benchmark output.
- **D-13:** The final write-up must explicitly call out where prefix compaction helps, where it is flat, and whether further format work is justified.

### the agent's Discretion
- Exact checked-in bounded fixture contents and size, as long as the smoke tier stays lightweight and reproducible.
- Exact field-family or subset slicing inside `common-pile/github_archive`, as long as the analysis can show both wins and flat/no-win cases from the same corpus.
- Exact env var names, benchmark naming, and helper layout for the opt-in `subset` and `large` tiers.
- Exact filename and structure of the checked-in Phase 11 report, as long as it records the locked evidence and recommendation outcome clearly.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and milestone constraints
- `.planning/ROADMAP.md` — Phase 11 goal, dependency on Phase 10, and success criteria for real-corpus validation.
- `.planning/PROJECT.md` — milestone-level constraints that claims must stay benchmark-backed, preserve pruning-first scope, and avoid unjustified format expansion.
- `.planning/STATE.md` — current focus note and the carry-forward todo to plan Phase 11 around representative external benchmark datasets and bounded corpus sizes.

### Prior phase decisions that constrain Phase 11
- `.planning/phases/06-query-path-hot-path/06-CONTEXT.md` — benchmark fixtures should prefer a recognizable public corpus when practical and only use bounded synthetic shaping where needed.
- `.planning/phases/10-serialization-compaction/10-CONTEXT.md` — broader proof bar, explicit raw-vs-zstd reporting, and the requirement to show where compaction is flat instead of averaging it away.
- `.planning/phases/10-serialization-compaction/10-03-SUMMARY.md` — existing benchmark-reporting pattern, metric names, and the precedent for attaching interpretation to benchmark evidence.
- `.planning/phases/10-serialization-compaction/10-VERIFICATION.md` — verified evidence bar for Phase 10 and the concrete metric/report shape that Phase 11 should extend rather than reinvent.

### Existing implementation surfaces
- `benchmark_test.go` — current Phase 10 fixture builders, metric reporting (`legacy_raw_bytes`, `compact_raw_bytes`, `default_zstd_bytes`, `bytes_saved_pct`), and benchmark subbench structure to extend.
- `serialize.go` — compact ordered-string serialization logic whose real-world payoff Phase 11 is validating.
- `prefix.go` — front-coding primitive and block-compression behavior being evaluated on real data.
- `Makefile` — current repo entry points; no benchmark-specific targets exist today, which supports the locked env-var-only opt-in design.
- `s3.go` — existing env-var configuration pattern in the repo.
- `cmd/gin-index/main_test.go` — existing test/helper env-var gating precedent.

### Dataset sourcing and external corpus choice
- `.planning/seeds/SEED-001-simdjson-test-datasets.md` — dormant dataset-infrastructure seed and the existing reasoning for using real JSON corpora during benchmark-focused phases.
- `https://huggingface.co/datasets/common-pile/github_archive` — locked primary large external corpus for Phase 11; data is exposed as `gharchive/v0/documents/*.jsonl.gz`.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `benchmark_test.go`: already contains the Phase 10 compaction benchmark harness, exact raw-vs-compact accounting helpers, and subbench leaves for size, encode, decode, and post-decode query checks.
- `serialize.go`: already exposes the compact ordered-string write/read path that Phase 11 needs to measure on real corpus shapes.
- `prefix.go`: already contains the front-coding primitive and block-size behavior that explain why some corpus slices may win more than others.

### Established Patterns
- Benchmark evidence in this repo is captured directly in `benchmark_test.go` and then interpreted in checked-in phase summary/verification docs.
- Optional external configuration already uses env vars in this repo; Phase 11 should follow that pattern for larger corpus paths.
- The repo does not yet have `testdata/` or dataset-management infrastructure, so the larger corpus path should remain opt-in and out-of-repo.

### Integration Points
- `benchmark_test.go`: add Phase 11 corpus loaders, tier gating, field-family/subset slicing, and real-corpus size/reporting metrics.
- Repository docs or phase report artifact: explain how to obtain `common-pile/github_archive`, how the env-var-gated tiers work, and how to interpret the results.
- Phase closeout docs: extend the existing summary/verification pattern with a real-corpus recommendation outcome.

</code_context>

<specifics>
## Specific Ideas

- The user wants robust JSON-based datasets on Hugging Face for larger corpus tests that can be downloaded on demand.
- `common-pile/github_archive` was selected as the primary large corpus because it is already published as a large JSON/JSONL corpus on Hugging Face and fits the "real external events corpus" role better than the smaller alternatives reviewed.
- `subset` and `large` should be activated only when the caller points benchmarks at a downloaded corpus root via env vars; the default developer path should remain lightweight.
- The final report should preserve explicit no-win or flat cases rather than collapsing them into an averaged success story.

</specifics>

<deferred>
## Deferred Ideas

- Add a second external contrast corpus such as HDFS-style logs only if `common-pile/github_archive` cannot produce a clear enough "helps vs flat" story within one corpus.

</deferred>

---

*Phase: 11-real-corpus-prefix-compression-benchmarking*
*Context gathered: 2026-04-20*
