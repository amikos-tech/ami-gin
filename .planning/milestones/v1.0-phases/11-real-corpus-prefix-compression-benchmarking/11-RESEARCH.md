# Phase 11: Real-Corpus Prefix Compression Benchmarking - Research

**Researched:** 2026-04-20
**Domain:** Real-corpus benchmark design for compact ordered-string serialization on external JSONL datasets
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

Everything in this block is copied or restated from `.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-CONTEXT.md`. [CITED: .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-CONTEXT.md]

### Locked Decisions

#### Corpus sourcing and ownership
- **D-01:** Keep one small bounded fixture in-repo for reproducible smoke coverage while larger corpus runs stay outside the repo.
- **D-02:** Larger real-corpus runs should use on-demand downloads rather than vendoring a large external dataset into the repository.
- **D-03:** Hugging Face is the preferred source class for the larger JSON/JSONL corpus used in this phase.
- **D-04:** The primary large external corpus is `common-pile/github_archive`.

#### Corpus mix and comparison shape
- **D-05:** Use a single external corpus, not a multi-corpus comparison matrix.
- **D-06:** Derive the "helps vs flat" conclusion from contrasting field families or subsets inside `common-pile/github_archive`.

#### Scale tiers and activation
- **D-07:** Define three benchmark tiers: `smoke`, `subset`, and `large`.
- **D-08:** Only `smoke` belongs in the normal benchmark surface; `subset` and `large` are explicit opt-in runs.
- **D-09:** Opt-in activation should be env-var gated only.
- **D-10:** Opt-in tiers should skip cleanly when the required external-corpus env vars are absent and fail only when the user opted in but configured the dataset path incorrectly.

#### Evidence and reporting
- **D-11:** Reuse the existing Phase 10 metric style where practical, including raw string-section deltas and final encoded artifact size.
- **D-12:** Produce a checked-in narrative report in addition to benchmark output.
- **D-13:** The final write-up must explicitly call out where prefix compaction helps, where it is flat, and whether further format work is justified.

### Agent Discretion
- Exact checked-in smoke-fixture contents and size, as long as the fixture stays lightweight and reproducible.
- Exact field-family slicing inside `common-pile/github_archive`, as long as the analysis can show both win and flat/no-win cases from the same corpus.
- Exact env var names, helper layout, and results-artifact naming.

### Deferred / Out of Scope
- A second external contrast corpus unless `common-pile/github_archive` cannot produce a clear enough within-corpus contrast.
- Downloading or snapshotting the large corpus automatically inside `go test`.
</user_constraints>

<phase_requirements>
## Phase Success Criteria Support

Formal phase requirement IDs are still `TBD` in `.planning/ROADMAP.md`, so this research maps directly to the roadmap success criteria instead of named REQ-IDs. [CITED: .planning/ROADMAP.md]

| Criterion | Description | Research Support |
|-----------|-------------|------------------|
| SC-11-01 | Benchmark coverage includes at least one realistic external log-style corpus large enough to stress repeated paths and repeated string terms. | `Summary`, `Standard Stack`, and `Sources` support `common-pile/github_archive` because the dataset card documents ~30.3M rows, ~54.7 UTF-8 GB, and shardable `jsonl.gz` storage under `gharchive/v0/documents/`. [CITED: https://huggingface.co/datasets/common-pile/github_archive, https://huggingface.co/datasets/common-pile/github_archive/tree/main/gharchive/v0/documents] |
| SC-11-02 | The benchmark plan defines practical dataset scales such as smoke, meaningful subset, and larger corpus runs instead of relying only on tiny synthetic fixtures. | `Summary`, `Architecture Patterns`, and `Common Pitfalls` recommend fixed tier sizing with one checked-in smoke projection plus deterministic shard-count external tiers so results stay reproducible and do not accidentally benchmark the whole corpus by default. [VERIFIED: benchmark_test.go][CITED: https://huggingface.co/docs/huggingface_hub/guides/download] |
| SC-11-03 | Results report both raw serialized string-section deltas and final encoded artifact size on those corpora. | `Summary`, `Standard Stack`, and `Validation Architecture` recommend reusing the exact Phase 10 accounting helpers (`legacy_raw_bytes`, `compact_raw_bytes`, `default_zstd_bytes`, string-payload accounting) on real-corpus projections rather than inventing new metrics. [VERIFIED: benchmark_test.go, serialize.go, prefix.go][CITED: .planning/phases/10-serialization-compaction/10-03-SUMMARY.md, .planning/phases/10-serialization-compaction/10-VERIFICATION.md] |
| SC-11-04 | The final write-up makes it explicit where prefix compaction helps, where it is flat, and whether further format work is justified. | `Summary`, `Architecture Patterns`, and `Common Pitfalls` recommend a checked-in raw-results artifact plus a separate interpretive report that contrasts a structured metadata-heavy projection against a text-heavy projection from the same corpus. [CITED: .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-CONTEXT.md, https://huggingface.co/datasets/common-pile/github_archive] |
</phase_requirements>

## Summary

Phase 11 should extend the existing repository benchmark harness instead of adding a downloader, a new benchmark framework, or a second corpus. The safest design is:

1. Keep one small checked-in smoke fixture derived from the locked `common-pile/github_archive` record shape so the default repo benchmark surface stays fast and reproducible.
2. Drive `subset` and `large` tiers from a caller-supplied local snapshot root using env vars only; `go test` must never download from Hugging Face on its own.
3. Reuse the exact Phase 10 size-accounting model so the benchmark continues to report `legacy_raw_bytes`, `compact_raw_bytes`, and `default_zstd_bytes`, plus explicit string-section payload deltas. [VERIFIED: benchmark_test.go]
4. Derive the "helps vs flat" story from two deterministic projections of the same corpus:
   - a **structured** projection dominated by repeated metadata fields such as `source`, `metadata.repo`, `metadata.license`, and `metadata.license_type`, where compact ordered strings should visibly help;
   - a **text-heavy** projection dominated by `text` plus a small amount of metadata, where high-cardinality thread bodies should stay closer to flat and expose where prefix coding stops paying off.

This design satisfies every locked decision without reopening Phase 10 scope. It keeps the dataset choice fixed, keeps default execution lightweight, and turns the real-corpus result into a question of evidence interpretation rather than more format work. [CITED: .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-CONTEXT.md][VERIFIED: benchmark_test.go, serialize.go, prefix.go]

## Project Constraints

- Keep the default benchmark surface repo-local and deterministic. Benchmarks that need a downloaded corpus must skip cleanly when the opt-in env vars are absent. [CITED: AGENTS.md, .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-CONTEXT.md][VERIFIED: cmd/gin-index/main_test.go, s3.go]
- Do not add a benchmark-specific network dependency to the library or benchmark runtime. A local snapshot root is enough because Hugging Face already supports selective file download outside the repo. [CITED: https://huggingface.co/docs/huggingface_hub/guides/download]
- Reuse the current metric/reporting idioms from Phase 10 instead of inventing a second evidence vocabulary. [CITED: .planning/phases/10-serialization-compaction/10-03-SUMMARY.md, .planning/phases/10-serialization-compaction/10-VERIFICATION.md][VERIFIED: benchmark_test.go]
- Preserve the milestone rule that broader format work stays benchmark-backed and explicitly justified. [CITED: .planning/PROJECT.md, .planning/STATE.md]
- Keep the opt-in mechanism env-var based rather than introducing new default `Makefile` targets or flag parsing surfaces. [CITED: .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-CONTEXT.md][VERIFIED: Makefile, s3.go, cmd/gin-index/main_test.go]

## Standard Stack

### Core

- `benchmark_test.go` remains the main implementation surface. Phase 10 already centralizes fixture builders, exact raw/compact accounting helpers, and benchmark sub-bench organization there. [VERIFIED: benchmark_test.go]
- Go stdlib I/O is sufficient for real-corpus ingestion in benchmarks: `os`, `path/filepath`, `compress/gzip`, `bufio`, and `encoding/json` are enough to stream local `jsonl.gz` shards. [ASSUMED]
- The Phase 10 helpers `phase10LegacyStringPayloadBytes`, `phase10CompactStringPayloadBytes`, and `EncodeWithLevel(..., CompressionNone)` should be reused for real-corpus size accounting. [VERIFIED: benchmark_test.go, serialize.go]
- A checked-in smoke fixture should live under `testdata/phase11/` with a provenance note. The benchmark can read that fixture directly without any external setup. [ASSUMED]

### Supporting

- A checked-in results artifact such as `.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-BENCHMARK-RESULTS.md` should capture exact commands, env vars, shard counts, projection names, and measured metrics from external runs. [ASSUMED]
- A separate narrative report such as `.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-REAL-CORPUS-REPORT.md` should interpret the results and make the explicit recommendation required by the roadmap. [ASSUMED]
- `README.md` should document the opt-in env vars plus one supported Hugging Face acquisition path so another maintainer can reproduce the external run shape. [VERIFIED: README.md][CITED: https://huggingface.co/docs/huggingface_hub/guides/download, https://huggingface.co/docs/datasets/loading]

### Alternatives Locked Out by Context

- Do not add automatic download logic to `go test` or to production code. The benchmark runtime should only consume local files. [CITED: .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-CONTEXT.md]
- Do not introduce a second external corpus just to produce a contrast story. The phase is explicitly single-corpus. [CITED: .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-CONTEXT.md]
- Do not hide flat or negative results by averaging them together with the structured projection. The report must surface where compaction is flat. [CITED: .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-CONTEXT.md, .planning/phases/10-serialization-compaction/10-CONTEXT.md]

## Architecture Patterns

### Pattern 1: Two Projections from One Corpus

Use the same `common-pile/github_archive` rows to build two benchmark document families:

- **Structured projection:** preserve `source`, `created`, and nested metadata fields such as `repo`, `license`, and `license_type`. This keeps repeated short strings and repeated nested paths visible. [CITED: https://huggingface.co/datasets/common-pile/github_archive]
- **Text-heavy projection:** preserve `text` plus minimal supporting metadata such as `source` and `metadata.url`. This keeps row shape realistic while stressing high-cardinality string payloads that should be flatter after compaction. [CITED: https://huggingface.co/datasets/common-pile/github_archive]

This satisfies the locked one-corpus rule while still producing an internal contrast that Phase 11 can interpret honestly.

### Pattern 2: Deterministic Tiering by Shard Count

Use one checked-in smoke fixture plus fixed external shard counts for `subset` and `large`. The dataset tree exposes many numbered shards under `gharchive/v0/documents/000xx_github-archive-threads.jsonl.gz`, which makes deterministic tier selection straightforward. [CITED: https://huggingface.co/datasets/common-pile/github_archive/tree/main/gharchive/v0/documents]

Recommended default planning target:

- `smoke`: checked-in bounded fixture only
- `subset`: first 4 external shards under `gharchive/v0/documents/`
- `large`: first 32 external shards under the same directory

The exact counts can still be adjusted during planning, but fixed shard-count tiers are better than free-form local selections because they keep historical comparisons reproducible.

### Pattern 3: Fail Closed on Opt-In Misconfiguration, Skip on Missing Opt-In

Always register `Smoke`, `Subset`, and `Large` benchmark branches so the benchmark shape is stable. Then:

- if the caller did not set the opt-in env var for `Subset` or `Large`, call `b.Skip` with a message naming the missing env var;
- if the caller explicitly enabled a tier but the corpus root does not contain `gharchive/v0/documents/*.jsonl.gz`, fail with `b.Fatalf` and include the exact expected path.

This matches the locked behavior in D-10 and the repo’s existing env-based optional configuration style. [VERIFIED: s3.go, cmd/gin-index/main_test.go]

### Pattern 4: Separate Raw Results from Interpretation

Keep the execution artifact and the recommendation artifact separate:

- `11-BENCHMARK-RESULTS.md` records commands, shard counts, projections, doc counts, and metrics.
- `11-REAL-CORPUS-REPORT.md` interprets those metrics under explicit headings like `Helps`, `Flat / No-Win`, and `Recommendation`.

That separation prevents the final report from drifting away from the measured numbers.

## Don’t Hand-Roll

| Problem | Don’t Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| External dataset acquisition | A custom Go downloader embedded in the benchmark path | `huggingface_hub.snapshot_download(..., allow_patterns=...)` or equivalent documented Hugging Face download flow outside the repo. [CITED: https://huggingface.co/docs/huggingface_hub/guides/download] | The benchmark only needs local shard paths; network I/O in `go test` would make results noisy and brittle. |
| Real-corpus loading | A second benchmark framework or CLI harness | Existing Go benchmarks in `benchmark_test.go`. [VERIFIED: benchmark_test.go] | The repo already uses `go test -bench` and Phase 10 metrics are there. |
| Real-vs-legacy comparison | A resurrected legacy serializer implementation | Existing Phase 10 exact accounting helpers over current in-memory structures. [VERIFIED: benchmark_test.go] | The repo already proved the accounting model in Phase 10. |
| Tier discovery | Ad hoc user-selected local files | Fixed shard-count tiers rooted at a local snapshot path. [CITED: https://huggingface.co/datasets/common-pile/github_archive/tree/main/gharchive/v0/documents] | Deterministic tiers are easier to compare over time. |

## Common Pitfalls

### Pitfall 1: Downloading Inside the Benchmark

If `go test` fetches data from Hugging Face, benchmark timing becomes a network test and default developer runs become unreliable.

**Avoid it:** keep download instructions in docs only; benchmark code reads local files only. [CITED: https://huggingface.co/docs/huggingface_hub/guides/download]

### Pitfall 2: Letting `subset` or `large` Run Accidentally

If external tiers run by default, the benchmark surface stops being lightweight and CI-safe.

**Avoid it:** gate each tier behind an explicit env var and make missing env vars skip instead of fail. [CITED: .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-CONTEXT.md]

### Pitfall 3: Comparing Different Corpus Shapes Across Tiers

If the structured projection on `smoke` is compared against a text-heavy projection on `large`, the result says more about the projection than the tier.

**Avoid it:** keep the projection names and field sets fixed across `smoke`, `subset`, and `large`.

### Pitfall 4: Reporting Only Final zstd Bytes

Phase 11 specifically needs raw string-section deltas plus final encoded size. Reporting only the post-zstd artifact hides where prefix compaction materially helped or was neutral.

**Avoid it:** always report Phase 10’s raw-vs-compact metrics plus explicit string-payload accounting. [VERIFIED: benchmark_test.go][CITED: .planning/phases/10-serialization-compaction/10-CONTEXT.md]

### Pitfall 5: Hiding Flat or Negative Results

The phase fails its goal if the final write-up only highlights wins.

**Avoid it:** keep the structured and text-heavy projections visible in both the raw results artifact and the report, even when one projection is flat.

## Environment Availability

- No new production dependency is required for the recommended design. External acquisition can stay documented and optional. [VERIFIED: go.mod, benchmark_test.go]
- The repo already has all local surfaces needed for the benchmark work: `benchmark_test.go`, Phase 10 accounting helpers, and standard env-var configuration precedents. [VERIFIED: benchmark_test.go, s3.go, cmd/gin-index/main_test.go]
- Hugging Face supports both selective repository download and loading JSON/JSONL data from specific files or patterns, which is enough for maintainers to stage the external corpus locally before running the opt-in tiers. [CITED: https://huggingface.co/docs/huggingface_hub/guides/download, https://huggingface.co/docs/datasets/loading]

## Validation Architecture

Phase 11 should validate four things:

1. **Default benchmark safety**
   - `BenchmarkPhase11RealCorpus` runs with the checked-in smoke fixture and does not require external data.
   - `Subset` and `Large` are still present in the benchmark tree but skip when opt-in env vars are absent.

2. **Misconfiguration handling**
   - If a user sets `GIN_PHASE11_ENABLE_SUBSET=1` or `GIN_PHASE11_ENABLE_LARGE=1` with an invalid `GIN_PHASE11_GITHUB_ARCHIVE_ROOT`, the benchmark fails with a precise path/layout message.

3. **Real-corpus metric capture**
   - External tiers report raw-wire, compact-wire, string-payload, and final artifact metrics for both projections.
   - A checked-in results artifact records the exact commands and environment used.

4. **Narrative conclusion**
   - The report cites the measured metrics and contains explicit sections for where compaction helps, where it is flat, and whether more format work is warranted.

Recommended commands:

- Default smoke + skip semantics:
  `go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus' -benchtime=1x -count=1`
- Opt-in subset:
  `GIN_PHASE11_GITHUB_ARCHIVE_ROOT=/path/to/snapshot GIN_PHASE11_ENABLE_SUBSET=1 go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=subset' -benchtime=1x -count=1`
- Opt-in large:
  `GIN_PHASE11_GITHUB_ARCHIVE_ROOT=/path/to/snapshot GIN_PHASE11_ENABLE_LARGE=1 go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=large' -benchtime=1x -count=1`

## Security Domain

The main security and integrity concerns are benchmark-trust issues rather than product attack surface:

- External file paths must fail closed when the caller explicitly enables a tier but points to the wrong root.
- Results artifacts must record enough provenance to prevent cherry-picked or irreproducible claims.
- The report must stay grounded in measured metrics rather than qualitative impressions.

## Sources

### Primary (HIGH confidence)

- Hugging Face dataset card for `common-pile/github_archive`: https://huggingface.co/datasets/common-pile/github_archive
- Hugging Face dataset tree for shard layout under `gharchive/v0/documents/`: https://huggingface.co/datasets/common-pile/github_archive/tree/main/gharchive/v0/documents
- Hugging Face Hub download guide (`snapshot_download`, `allow_patterns`): https://huggingface.co/docs/huggingface_hub/guides/download
- Hugging Face Datasets loading guide (`data_files`, remote/local JSON/JSONL loading): https://huggingface.co/docs/datasets/loading
- GH Archive official site and public dataset overview: https://www.gharchive.org/

### Repository (HIGH confidence)

- `.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-CONTEXT.md`
- `.planning/ROADMAP.md`
- `.planning/STATE.md`
- `.planning/PROJECT.md`
- `.planning/phases/10-serialization-compaction/10-03-SUMMARY.md`
- `.planning/phases/10-serialization-compaction/10-VERIFICATION.md`
- `benchmark_test.go`
- `serialize.go`
- `prefix.go`
- `s3.go`
- `cmd/gin-index/main_test.go`

## Metadata

- Phase: `11-real-corpus-prefix-compression-benchmarking`
- Research mode: default plan-phase research path
- Formal requirement IDs: none yet (`Requirements: TBD` in roadmap); planning should anchor on the roadmap success criteria plus the locked decisions above
