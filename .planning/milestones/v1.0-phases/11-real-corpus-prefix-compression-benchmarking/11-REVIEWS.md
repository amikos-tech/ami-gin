---
phase: 11
reviewers: [gemini, claude]
reviewed_at: 2026-04-20T10:56:10Z
plans_reviewed:
  - .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-01-PLAN.md
  - .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-02-PLAN.md
  - .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-03-PLAN.md
---

# Cross-AI Plan Review — Phase 11

## Gemini Review

The following is a review of the implementation plans for **Phase 11: Real-Corpus Prefix Compression Benchmarking**.

### 1. Summary
The plan set is comprehensive, well-structured, and directly addresses the Phase 11 goal of validating prefix-compaction payoff on real-world data. By introducing a three-tiered benchmark strategy (smoke, subset, large) and a dual-projection approach (structured vs. text-heavy), the plans provide a robust framework for identifying where compression succeeds and where it remains flat. The reuse of Phase 10 accounting helpers ensures metric consistency across phases.

### 2. Strengths
- **Contrastive Analysis:** The use of `projection=structured` and `projection=text-heavy` from the same corpus is a clever design that isolates the effect of prefix compaction on different data distributions (high-repetition metadata vs. high-cardinality text).
- **Deterministic Tiering:** Using fixed shard counts (4 and 32) for the external tiers ensures that results are reproducible and comparable across different runs.
- **Safety & Portability:** The env-var gating and skip-logic ensure that the default development environment remains lightweight and free of external network dependencies.
- **Traceability:** The separation of raw results (`11-BENCHMARK-RESULTS.md`) and narrative interpretation (`11-REAL-CORPUS-REPORT.md`) protects against "interpretive drift" and maintains a clear audit trail of the evidence.
- **Proven Metric Reuse:** Leveraging the Phase 10 accounting model (`legacy_raw_bytes`, `compact_raw_bytes`, etc.) avoids re-inventing the wheel and maintains a consistent vocabulary for "compression quality."

### 3. Concerns
- **Memory Pressure in Large Tier (Medium):** Loading 32 shards of the `github_archive` corpus may result in high memory consumption. While `Encode` has safety limits (64MB), the `GINBuilder`'s in-memory maps and Roaring bitmaps for high-cardinality paths (like `text` or `url`) could significantly pressure the heap during the build phase of the `large` tier.
- **Dataset Volatility (Low):** The plan relies on a specific shard layout (`000xx_github-archive-threads.jsonl.gz`). If the upstream dataset on Hugging Face is restructured or the file naming convention changes, the deterministic shard loading logic will require updates.
- **Single-Threaded Ingestion (Low):** The benchmarks appear to load and index shards sequentially. For the `large` tier (32 shards), this might lead to long `benchtime=1x` runs. While acceptable for an opt-in benchmark, it is worth noting for maintainer patience.

### 4. Suggestions
- **Resource Monitoring:** In `11-02-PLAN.md`, consider adding a requirement to report memory allocations (`b.ReportAllocs()`) for the real-corpus tiers to quantify the build-time cost of real-world scale alongside the size metrics.
- **Smoke Fixture Representativeness:** Ensure `testdata/phase11/github_archive_smoke.jsonl` contains at least a few records that trigger both "helps" and "flat" scenarios (e.g., identical `metadata.repo` values for some records and completely unique `text` values for others).
- **Zstd Level Sensitivity:** The report in `11-03-PLAN.md` should briefly mention if zstd's effectiveness (at the default level 15) tends to "mask" the raw prefix-compaction wins, or if the wins persist through the compression layer.

### 5. Risk Assessment
**Overall Risk: LOW**

The plans stay strictly within the phase boundary and do not reopen the wire-format design. The dependency chain (`11-01` -> `11-02` -> `11-03`) is logical and the success criteria from the roadmap are fully addressed. The most significant risk is the resource overhead of the `large` tier, which is mitigated by making it an explicit, opt-in operation.

### Roadmap Alignment
| Requirement | Plan Support |
|-------------|--------------|
| Realistic log-style corpus | Satisfied by `common-pile/github_archive` integration. |
| Practical dataset scales | Satisfied by smoke/subset/large tier design. |
| Raw vs Encoded reporting | Satisfied by metrics in `11-02-PLAN.md`. |
| Explicit helps/flat write-up | Satisfied by `11-REAL-CORPUS-REPORT.md` requirements in `11-03-PLAN.md`. |

---

## Claude Review

# Phase 11 Plan Review — Real-Corpus Prefix Compression Benchmarking

## 1. Summary

The three plans (11-01, 11-02, 11-03) form a coherent, well-sequenced phase that honors the locked decisions in `11-CONTEXT.md` and satisfies the four roadmap success criteria structurally. They reuse Phase 10 accounting verbatim, keep the default benchmark surface smoke-only, and separate raw evidence from interpretation — a pattern Phase 10 already validated. The main gaps are around real-corpus reproducibility across time (no HF dataset revision pinning), smoke fixture realism (unspecified row count and license provenance), and ambiguous autonomy semantics on 11-02 when the external snapshot is unavailable. The plans are ready to execute with a handful of targeted tightenings — none of which change the phase scope.

## 2. Strengths

- **Tight honoring of locked decisions**: D-01 through D-13 each map to concrete acceptance criteria; no decision is quietly reinterpreted.
- **Metric continuity with Phase 10**: `legacy_raw_bytes`, `compact_raw_bytes`, `default_zstd_bytes`, `bytes_saved_pct` carry over verbatim, so historical and new numbers remain comparable.
- **Deterministic tiering**: Fixed shard counts (4 / 32) under a named path (`gharchive/v0/documents/*.jsonl.gz`) beat free-form local selection and protect future re-runs.
- **Fail-closed opt-in semantics**: `b.Skip` on missing env var, `b.Fatalf` on wrong root — exactly the behavior that prevents silent drift.
- **Projection contrast inside one corpus**: `structured` vs `text-heavy` produces the "helps vs flat" story SC-11-04 demands without a second corpus.
- **Grep-able acceptance criteria**: Verify blocks use concrete string assertions, making gates objective rather than narrative.
- **Separation of raw results from report**: `11-BENCHMARK-RESULTS.md` as data, `11-REAL-CORPUS-REPORT.md` as interpretation — prevents narrative drift from measured numbers.
- **Env-var convention reuse**: Follows `s3.go` and `cmd/gin-index/main_test.go` precedent instead of new flags or Makefile targets.

## 3. Concerns

### HIGH

- **License/provenance for checked-in fixture is unaddressed.** 11-01 checks a bounded sample derived from `common-pile/github_archive` into `testdata/phase11/`. Common-Pile datasets carry per-row license metadata and redistribution terms that must be compatible with the repo's MIT license. No task verifies this before commit, and no mention of upstream license appears in the provenance note requirements.
- **No Hugging Face dataset revision pinning.** Plans reference `common-pile/github_archive` by name but never by revision/commit hash. `snapshot_download` defaults to the latest revision; if the dataset is re-sharded or renamed after this phase ships, the README reproduction steps become stale and `11-BENCHMARK-RESULTS.md` becomes unreproducible. This directly undermines SC-11-02.
- **11-02 autonomy ambiguity blocks execution.** 11-02 is marked `autonomous: false` because Task 2 needs a local snapshot. But Task 1 (metric extension) is purely code-local. As written, an executor without the snapshot cannot even start. Recommend splitting 11-02 into code-autonomous and capture-manual steps, or making the autonomy condition task-level.

### MEDIUM

- **Smoke fixture size floor is undefined.** "A few hundred projected rows" invites a 50-row fixture with noise-level metrics. Phase 10 showed Mixed saved only 2.748% at 65KB and HighPrefix only 0.34% — signal-to-noise at hundreds of rows is already marginal. Recommend an explicit floor (>=500 rows, >=50KB string payload).
- **Shard-count determinism != row-count determinism.** GH Archive shards vary in row count; "first 4 shards" could mean 2M rows on one snapshot and 800K on another. `docs_indexed` is reported but not asserted or bounded.
- **No memory/wall-clock budget on `large` tier.** 32 shards can exceed 15–20GB uncompressed. Nothing warns an operator they're about to blow out their machine. Recommend expected wall-clock and memory footprint in the README opt-in section.
- **Adversarial content handling is not considered.** Real `text` fields contain arbitrary user-generated content: huge strings, control characters, malformed UTF-8. The builder calls `AddDocument` which could hit internal limits (`maxConfigSize`). A single bad document could abort an hours-long benchmark.
- **Dependency ordering gap inside 11-02.** If Task 1 ships broken metric names, Task 2's acceptance criteria (which `rg` for exact metric strings) fails. No "re-run Task 1 verify before Task 2" gate.
- **Full regression suite not in verification blocks.** Phase 11 touches `benchmark_test.go` heavily. Each plan's verify block runs only Phase 11 bench. Recommend adding `go test ./... -count=1` to at least one plan to catch collateral damage in Phase 10 fixtures.

### LOW

- **README scope creep.** README is public library doc. Phase 11 internal benchmark env vars belong more naturally in `CONTRIBUTING.md` or a `.planning/phases/11.../` benchmarks doc.
- **`rg` tooling assumption.** Every verify block uses `rg`. If an executor lacks ripgrep, verification fails for environment reasons rather than plan reasons.
- **Heading-matching regex in 11-03 is brittle.** `^## Flat / No-Win$` requires exact spacing; editors with trim-whitespace could break the gate. Consider `^## Flat\\s*/\\s*No-Win$` or renaming to `Flat-Or-No-Win`.
- **No smoke-tier CI integration.** `BenchmarkPhase11RealCorpus/tier=smoke` has no Makefile target or CI job. It becomes dead code until someone runs it manually.
- **Index configuration on real corpus is unspecified.** Plans don't pin `GINConfig` (trigrams on/off, transformers, etc.). Trigrams materially affect index size on text-heavy projections. Without pinning, re-runs are not comparable.

## 4. Suggestions

1. **Add a license-check task to 11-01** before the fixture lands; include upstream license in `testdata/phase11/README.md`. If MIT-incompatible, switch to a synthesized-from-shape fixture.
2. **Pin the HF dataset revision.** Add `dataset_revision: <git-sha>` to provenance notes and results artifact. README snippet should use `snapshot_download(..., revision=\"<sha>\")`.
3. **Split 11-02 autonomy.** Make Task 1 autonomous so harness work isn't blocked on external snapshot availability.
4. **Set an explicit smoke fixture floor** in 11-01 Task 1 acceptance: ">=500 rows" and ">=50KB of string payload after projection."
5. **Capture effective row counts in 11-02 acceptance.** Add `docs_indexed > <floor>` to subset/large acceptance criteria to catch shard-size drift.
6. **Document resource expectations in README** — one-liner per tier (disk, RAM, wall-clock).
7. **Add a skip-on-parse-failure policy.** Per-document parse failures should be counted as a benchmark metric rather than aborting the whole run.
8. **Pin the index configuration in the benchmark** and record it in the results artifact.
9. **Expand at least one verify block to `go test ./... -count=1`** — recommended in 11-02 Task 1.
10. **Add `make bench-phase11-smoke`** to keep the smoke tier exercised during normal contribution flow.
11. **Relocate README opt-in docs** to `CONTRIBUTING.md` or `docs/benchmarking.md`; link from README instead.

## 5. Risk Assessment

**Overall: MEDIUM**

- **Scope and correctness risk: LOW.** Plans stay within locked decisions, reuse proven Phase 10 accounting, and do not touch the wire format. No functional regression vector into the library itself.
- **Reproducibility risk: MEDIUM.** Without HF dataset revision pinning, the "real corpus" claim has a limited shelf life. Single most important item to fix — the phase's entire value is evidence that remains interpretable later.
- **Evidence-quality risk: MEDIUM.** Smoke fixture under-specified on size and licensing; large tier has no resource guardrails; adversarial-content handling unaddressed. Any of these can produce noisy numbers or a failed multi-hour run.
- **Coordination risk: LOW.** Plans sequence cleanly (11-01 -> 11-02 -> 11-03). Only real coupling gap is 11-02 autonomy ambiguity — operational rather than technical.

Net: execute with the HIGH-severity items (license check, dataset revision pinning, 11-02 autonomy split) addressed up front; MEDIUM items can ride along as targeted amendments to existing tasks without restructuring the phase.

---

## Consensus Summary

Both reviewers agree the phase is structurally sound: the three-plan split is coherent, the reuse of Phase 10 size-accounting is the right baseline, and the `structured` versus `text-heavy` contrast inside one external corpus is a good way to satisfy the "helps vs flat" reporting requirement without reopening scope.

### Agreed Strengths

- The phase decomposition is clean and dependency ordering is sensible: `11-01` establishes the benchmark surface, `11-02` captures measured evidence, and `11-03` turns that evidence into a reproducible report.
- Reusing the Phase 10 metrics (`legacy_raw_bytes`, `compact_raw_bytes`, `default_zstd_bytes`, `bytes_saved_pct`) keeps the evidence vocabulary stable and comparable.
- Fixed benchmark tiers plus env-var-gated opt-in behavior make the default developer path lightweight while still permitting larger real-corpus runs.
- Keeping raw benchmark output separate from the final narrative report improves auditability and reduces interpretive drift.

### Agreed Concerns

- Reproducibility needs to be tighter. Both reviewers flagged external dataset stability as a risk; the plans should pin the exact Hugging Face revision or otherwise record enough provenance that future reruns remain comparable.
- The `large` tier needs stronger operational guardrails. Both reviewers called out memory and run-time cost concerns for the 32-shard tier, so the plans should document expected resource usage and, ideally, capture allocation or scale metadata in the results.
- The benchmark inputs should be specified more concretely. Gemini focused on smoke-fixture representativeness; Claude extended that to fixture sizing and provenance. Tightening the smoke-fixture requirements would reduce evidence-quality drift.

### Divergent Views

- Gemini rated the overall risk as `LOW`; Claude rated it `MEDIUM`.
- Claude raised several plan-shaping issues that Gemini did not: fixture licensing/provenance, the lack of dataset revision pinning, and the `autonomous: false` ambiguity in `11-02`.
- Gemini emphasized optional but useful enhancements around allocation reporting and zstd masking analysis, while Claude focused more on reproducibility guarantees, resource budgets, and broader verification coverage.
