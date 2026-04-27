---
phase: 11
phase_name: "Real-Corpus Prefix Compression Benchmarking"
project: "GIN Index"
generated: "2026-04-22"
counts:
  decisions: 9
  lessons: 6
  patterns: 8
  surprises: 5
missing_artifacts: []
---

# Phase 11 Learnings: Real-Corpus Prefix Compression Benchmarking

## Decisions

### Synthesized-from-shape smoke fixture instead of redistributing upstream rows
The checked-in smoke fixture (`testdata/phase11/github_archive_smoke.jsonl`) was synthesized to preserve the `common-pile/github_archive` record shape (nested `metadata.{repo,url,license,license_type}`, repeated-vs-unique distributions, representative string lengths) rather than copying raw upstream text verbatim.

**Rationale:** Redistribution safety for row-level data from the pinned `common-pile/github_archive` revision was not explicit, and the plan permitted a synthesized fallback whenever redistribution safety was unclear.
**Source:** 11-01-PLAN.md, 11-01-SUMMARY.md

---

### Pin the exact Hugging Face dataset revision
External tiers are reproduced against a single pinned revision `93d90fbdbc8f06c1fab72e74d5270dc897e1a090` recorded in `testdata/phase11/README.md` and reused by `11-BENCHMARK-RESULTS.md`.

**Rationale:** Latest-only snapshots would make subset/large comparisons irreproducible; later readers must be able to audit the evidence against the exact corpus the numbers came from.
**Source:** 11-01-PLAN.md, 11-02-PLAN.md, 11-REAL-CORPUS-REPORT.md

---

### Opt-in external tiers via explicit env vars; always register branches
`BenchmarkPhase11RealCorpus` always registers `tier=smoke`, `tier=subset`, `tier=large` with both `projection=structured` and `projection=text-heavy`, but subset and large only activate when `GIN_PHASE11_GITHUB_ARCHIVE_ROOT` + `GIN_PHASE11_ENABLE_{SUBSET,LARGE}` are set; missing vars cause `b.Skip`, not silent inclusion.

**Rationale:** Normal `go test` runs must stay lightweight, yet skipped-vs-enabled states must be obvious in benchmark output so opt-in activation is never silent.
**Source:** 11-01-PLAN.md, 11-01-SUMMARY.md

---

### Fail closed on misconfigured opt-in tiers
When a tier is explicitly enabled but `GIN_PHASE11_GITHUB_ARCHIVE_ROOT` is empty, missing, or lacks `gharchive/v0/documents/*.jsonl.gz`, the benchmark fails with `b.Fatalf` naming both the env var and the expected relative shard path.

**Rationale:** A silent fallback or generic error would hide misconfiguration and produce meaningless evidence; fail-closed turns operator mistakes into actionable messages.
**Source:** 11-01-PLAN.md

---

### Build both projections from the same corpus shape
`projection=structured` and `projection=text-heavy` share one corpus fixture; structured uses `source/created/metadata.repo/metadata.license/metadata.license_type`, text-heavy uses `source/created/text/metadata.url`.

**Rationale:** Contrasting repeated-metadata wins against text-heavy flat cases without introducing a second corpus keeps the comparison honest and the fixture surface minimal.
**Source:** 11-01-SUMMARY.md

---

### Reuse Phase 10 accounting helpers and metric names
Phase 11 extends `phase10LegacyStringPayloadBytes`, `phase10CompactStringPayloadBytes`, and `phase10ReportMetrics`, and reports the exact names `legacy_raw_bytes`, `compact_raw_bytes`, `default_zstd_bytes`, `legacy_string_payload_bytes`, `compact_string_payload_bytes`, `bytes_saved_pct` — no parallel legacy serializer, no second size vocabulary.

**Rationale:** Real-corpus evidence has to remain directly comparable with Phase 10's serializer-level evidence; a second vocabulary would fracture the historical record.
**Source:** 11-02-PLAN.md, 11-02-SUMMARY.md

---

### Preserve the production 64 MiB Decode() cap; skip oversized decode branches
The large text-heavy run produced a 194,555,530-byte decompressed payload that the default `Decode()` path rejects against the 64 MiB safety cap. Rather than loosening the cap, the benchmark records a `Decode: skipped` leaf while retaining `Size`, `Encode`, and `QueryAfterDecode` metrics.

**Rationale:** Weakening a production decompression guardrail for benchmark convenience would trade a real security property for a single measurement; the honest choice was a benchmark-local skip.
**Source:** 11-02-SUMMARY.md, 11-BENCHMARK-RESULTS.md

---

### Do not pursue further serialization-format work for prefix compaction now
Final recommendation: no additional format work now; revisit only if a different real corpus or more repetition-heavy workload shows materially larger raw savings.

**Rationale:** Subset and large raw savings stayed under 0.006% (~150 B on a 2.5 MB subset, ~3 KB on a 194 MB large text-heavy artifact), so the potential payoff is vanishingly small relative to the engineering cost.
**Source:** 11-03-SUMMARY.md, 11-REAL-CORPUS-REPORT.md

---

### Keep default README benchmark path smoke-only; document opt-in explicitly
README documents the smoke command as default, with pinned `common-pile/github_archive` acquisition snippet, all three env vars, shard layout, and links directly to `11-BENCHMARK-RESULTS.md` and `11-REAL-CORPUS-REPORT.md`.

**Rationale:** Normal developer runs must stay lightweight; readers must still be able to audit the provenance before acting on the recommendation.
**Source:** 11-03-SUMMARY.md

---

## Lessons

### `go test -bench` does not fail when a benchmark is missing
The original TDD red step relied on a missing `BenchmarkPhase11RealCorpus` to fail — but `go test -bench` simply reports no benchmark matched and exits 0.

**Context:** Forced a switch to helper unit tests (`TestPhase11DiscoverExternalShardsRejectsMissingLayout`, `TestPhase11LoadSmokeFixture`) as the red/green gate for shard validation and smoke-fixture loading.
**Source:** 11-01-SUMMARY.md

---

### Error messages for opt-in tiers need both the env var and the layout
The first invalid-root error mentioned only `GIN_PHASE11_GITHUB_ARCHIVE_ROOT` but omitted the expected `gharchive/v0/documents/*.jsonl.gz` layout, leaving operators to guess what "invalid" meant.

**Context:** The loader error contract was tightened to mention both the env var and the expected relative shard path before final verification.
**Source:** 11-01-SUMMARY.md

---

### Large real-corpus text-heavy artifacts can exceed the 64 MiB Decode() cap
A text-heavy decompressed payload of ~194 MB — far above the 64 MiB safety limit — emerged from the 32-shard large tier.

**Context:** This was not anticipated in the plan; the response was a benchmark-local decode skip plus `TestPhase11ShouldSkipBenchmarkDecodeOnConfiguredLimit`, not a cap increase. The guardrail remained intact.
**Source:** 11-02-SUMMARY.md, 11-BENCHMARK-RESULTS.md

---

### zstd masks raw prefix-compaction wins on real corpora
At subset and large scale, `default_zstd_bytes` captures almost all of the redundancy that prefix compaction also targets, so visible raw deltas shrink to near-zero once the final compressed artifact is considered.

**Context:** The subset structured projection saved only 150 raw bytes (0.0058%) and large text-heavy saved 3,039 bytes out of 194 MB (0.0016%). The report calls this out explicitly and notes the inference is partial because Phase 11 does not encode a legacy-zstd baseline.
**Source:** 11-REAL-CORPUS-REPORT.md

---

### Synthesized smoke fixtures overstate format-level wins relative to real corpora
Smoke structured saved 7.08% and smoke text-heavy saved 13.56% of raw bytes — an order-of-magnitude-plus divergence from the <0.006% seen at subset/large scale.

**Context:** Small checked-in fixtures with controlled repetition can signal a format opportunity that simply is not present once real corpus entropy dominates. Real-corpus tiers are required before making format-level decisions.
**Source:** 11-REAL-CORPUS-REPORT.md, 11-BENCHMARK-RESULTS.md

---

### Monotonic doc-count growth is a necessary tier-selection guard
The plan required `docs_indexed` to increase strictly Smoke → Subset → Large and stops the artifact commit if that invariant breaks.

**Context:** Without this guard, tier selection bugs (e.g., accidentally using the smoke fixture for subset) would silently produce equivalent numbers across tiers and go unnoticed. The recorded growth `640 < 60466 < 486239` makes the comparison defensible.
**Source:** 11-02-PLAN.md, 11-BENCHMARK-RESULTS.md

---

## Patterns

### Synthesized-from-shape fixture
A checked-in fixture that preserves field names, nesting, repeated-vs-unique distributions, and representative string lengths while inventing the actual string content, paired with a `testdata/*/README.md` that records `Dataset:`, `Dataset revision:`, `Fixture origin: synthesized-from-shape`, `External layout:`, and row/byte counts.

**When to use:** When a smoke fixture must mirror a real dataset's index-relevant shape but redistributable row-level licensing for that dataset is not explicit, and the benchmark needs a bounded in-repo input.
**Source:** 11-01-PLAN.md, 11-01-SUMMARY.md

---

### Env-gated external tiers with stable branch registration
Always register all tier/projection sub-benchmark names so the benchmark tree is discoverable, but gate activation behind named env vars and `b.Skip` with explicit messages.

**When to use:** Any benchmark family that needs both a repo-local default path and opt-in branches over large external corpora on caller-provided infrastructure.
**Source:** 11-01-PLAN.md, 11-01-SUMMARY.md

---

### Deterministic fixed-shard-count tiers from numbered shard layouts
`subset=4` and `large=32` shards taken from the first matching `gharchive/v0/documents/*.jsonl.gz` files — no random sampling, no tail-end drift.

**When to use:** Whenever the upstream dataset exposes deterministic numbered shards and reproducibility across maintainers matters more than statistical coverage.
**Source:** 11-01-PLAN.md

---

### Results artifact from captured bench output
A checked-in `11-BENCHMARK-RESULTS.md` with a top-level `## Benchmark Configuration` section followed by per-tier sections (`## Smoke`, `## Subset`, `## Large`) that each record the exact command, env vars, effective shard/document counts, B/op, allocs/op, and one metric table per projection.

**When to use:** Whenever benchmark results must survive separate from the narrative report; prevents "report drifted from bench output" failure mode.
**Source:** 11-02-PLAN.md, 11-BENCHMARK-RESULTS.md

---

### Extend prior-phase accounting instead of branching metric vocabularies
When a later phase's evidence must stay comparable with a prior phase's serializer-level evidence, reuse that phase's helpers and metric names verbatim rather than introducing a second vocabulary.

**When to use:** Multi-phase performance work where each phase adds evidence to the same claim (e.g., "compaction saves raw bytes").
**Source:** 11-02-PLAN.md, 11-02-SUMMARY.md

---

### Benchmark-skip on intentional production caps
When the standard API intentionally rejects oversized inputs, have the benchmark record `skipped` for that specific leaf (plus a regression test for the skip condition) instead of loosening the cap.

**When to use:** Any time a production safety limit would otherwise force the choice between weakening a guardrail or dropping an evidence branch.
**Source:** 11-02-SUMMARY.md, 11-BENCHMARK-RESULTS.md

---

### Narrative report cites the raw artifact, not memory
`11-REAL-CORPUS-REPORT.md` opens with a pointer to `11-BENCHMARK-RESULTS.md` and every numeric claim in the report can be grounded in that artifact's tables.

**When to use:** Whenever a phase write-up is expected to survive as institutional evidence past the session that produced it.
**Source:** 11-03-SUMMARY.md, 11-REAL-CORPUS-REPORT.md

---

### Smoke-first README with linked opt-in workflow
README documents the smoke command as the default path, then a separate opt-in subsection with the pinned revision, acquisition snippet, all env vars, shard layout, and links to both the raw results and the interpretive report.

**When to use:** Any benchmark surface where the full evidence base is heavy but a light default path must stay the norm.
**Source:** 11-03-SUMMARY.md

---

## Surprises

### Subset/large raw savings collapsed to under 0.006%
Smoke tier prefix-compaction wins of 7.08% (structured) and 13.56% (text-heavy) shrank to 0.0058% and 0.00015% at subset scale, and 0.0027% and 0.0016% at large scale.

**Impact:** Drove the phase recommendation against more format-level work for prefix compaction on this corpus and exposed that the smoke fixture was overstating the effect by two to three orders of magnitude.
**Source:** 11-REAL-CORPUS-REPORT.md, 11-BENCHMARK-RESULTS.md

---

### Large text-heavy decompressed payload exceeded the 64 MiB safety cap
The 194,555,530-byte decompressed large-tier artifact hit the `Decode()` guardrail during what was supposed to be a straightforward evidence capture.

**Impact:** Added a benchmark-local decode-skip path plus a guarding unit test (`TestPhase11ShouldSkipBenchmarkDecodeOnConfiguredLimit`) and delivered an unexpected positive signal: the production limit was correctly sized and already enforced.
**Source:** 11-02-SUMMARY.md, 11-BENCHMARK-RESULTS.md

---

### `go test -bench` exits 0 when no benchmark matches
Expected a missing `BenchmarkPhase11RealCorpus` to produce a red step; instead `go test -bench` reports "no benchmarks matched" and exits successfully.

**Impact:** Required restructuring the TDD flow to rely on helper unit tests for the red/green gate rather than a missing-benchmark failure.
**Source:** 11-01-SUMMARY.md

---

### Initial invalid-root error omitted the shard layout
The first implementation of the opt-in error mentioned only the env var, not the expected `gharchive/v0/documents/*.jsonl.gz` layout.

**Impact:** A small but meaningful tightening of the loader error contract before final verification; a reminder that "names the env var" is not equivalent to "tells the operator what to do".
**Source:** 11-01-SUMMARY.md

---

### Structured and text-heavy large tiers emit asymmetric benchmark leaves
The large text-heavy branch emits `Size`, `Encode`, and `QueryAfterDecode` only — no `Decode` leaf — while the structured branch emits all four.

**Impact:** Any downstream reader or tooling that expects uniform leaves across projections will see the asymmetry and needs the "Decode skipped by design" note in `11-BENCHMARK-RESULTS.md` to interpret it correctly.
**Source:** 11-BENCHMARK-RESULTS.md, 11-VERIFICATION.md
