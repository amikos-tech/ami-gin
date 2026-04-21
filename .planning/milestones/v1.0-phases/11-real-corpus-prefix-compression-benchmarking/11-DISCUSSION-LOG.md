# Phase 11: Real-Corpus Prefix Compression Benchmarking - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-20
**Phase:** 11-real-corpus-prefix-compression-benchmarking
**Areas discussed:** Corpus sourcing and fixture ownership, Corpus mix, Scale tiers and where they run, Evidence and final write-up shape

---

## Corpus sourcing and fixture ownership

| Option | Description | Selected |
|--------|-------------|----------|
| Checked-in bounded fixture(s) plus optional larger local corpus path | Small representative corpus lives in-repo for reproducible smoke and meaningful runs; larger runs use a local file path or env var outside the repo. | ✓ |
| Download-on-demand with pinned source + checksum | Bench code or a helper fetches the corpus when needed, which keeps the repo lean but adds dataset tooling and network assumptions. | |
| Bring-your-own local path only | Simplest implementation, but weakest reproducibility story for future benchmark comparisons. | |

**User's choice:** Checked-in bounded fixture(s) plus optional larger local corpus path.
**Notes:** The user added an explicit preference to research robust JSON-based datasets on Hugging Face for larger corpus tests that can be downloaded on demand.

### Follow-up: lock the large corpus source

| Option | Description | Selected |
|--------|-------------|----------|
| Lock `common-pile/github_archive` as the primary large corpus now | Planner can still add one smaller contrast corpus later, but the main source is decided. | ✓ |
| Lock a Hugging Face shortlist only | Planner compares `github_archive` against 1-2 other Hugging Face candidates during research before choosing. | |
| Avoid locking a dataset yet | Only Hugging Face is locked as the source class; exact corpus choice stays open for planning. | |

**User's choice:** Lock `common-pile/github_archive` now.
**Notes:** Research during discussion surfaced `common-pile/github_archive` as the strongest fit among the Hugging Face JSON/JSONL candidates reviewed.

---

## Corpus mix

| Option | Description | Selected |
|--------|-------------|----------|
| `github_archive` only, derive "helps vs flat" from different path classes within that corpus | Keeps Phase 11 tightly scoped: one external corpus, multiple measured subsets or field families. | ✓ |
| `github_archive` plus one second external contrast corpus | Stronger cross-corpus story, but expands dataset and fixture management work. | |
| `github_archive` plus existing synthetic Phase 10 fixtures as the explicit contrast | Avoids a second external dataset while preserving real-corpus-vs-controlled comparison. | |

**User's choice:** `github_archive` only, with comparison derived from subsets inside that corpus.
**Notes:** The user explicitly kept the phase to one external corpus.

---

## Scale tiers and where they run

| Option | Description | Selected |
|--------|-------------|----------|
| Three tiers: `smoke`, `subset`, `large`; only `smoke` is part of the normal repo benchmark surface | Keeps recurring runs lightweight while preserving a meaningful middle tier and a larger opt-in tier. | ✓ |
| Two tiers only: `smoke` and `large` | Simpler, but skips the roadmap's "meaningful subset" style middle ground. | |
| Three tiers, but both `smoke` and `subset` are standard recurring benchmarks | Better recurring evidence, but raises baseline runtime and dataset friction. | |

**User's choice:** Three tiers, with only `smoke` in the default benchmark surface.
**Notes:** The user asked specifically how `subset` and `large` should trigger when opt-in.

### Follow-up: opt-in trigger style

| Option | Description | Selected |
|--------|-------------|----------|
| Env-var gated tiers only | Simple, idiomatic for this repo, and matches existing env-based optional configuration patterns. | ✓ |
| Env vars plus convenience `make` targets | Same underlying mechanism, but easier to run repeatedly. | |
| Explicit benchmark flags only, no env vars | Less stateful, but awkward when dataset paths and tier controls must be threaded through each command. | |

**User's choice:** Env-var gated tiers only.
**Notes:** The discussion captured the intended behavior: `smoke` always runs; `subset` and `large` activate only when the external-corpus env vars are present, skip cleanly when absent, and fail only on opted-in misconfiguration.

---

## Evidence and final write-up shape

| Option | Description | Selected |
|--------|-------------|----------|
| Benchmarks plus a checked-in Phase 11 report that interprets the results | Produces a narrative conclusion that says where compaction helps, where it is flat, and whether more format work is justified. | ✓ |
| Benchmarks only; rely on benchmark output and normal phase summary files | Lower writing overhead, but weaker against the roadmap's explicit-conclusion requirement. | |
| Benchmarks plus a machine-readable checked-in results artifact | Adds a generated CSV/JSON-style artifact in addition to the narrative report. | |

**User's choice:** Benchmarks plus a checked-in interpretive report.
**Notes:** This locks the expectation that benchmark output alone is not enough for Phase 11 closeout.

---

## the agent's Discretion

- Exact env var names for the opt-in tiers.
- Exact checked-in bounded fixture size and shape for `smoke`.
- Exact subset slicing inside `common-pile/github_archive`.
- Exact filename and structure of the checked-in Phase 11 report.

## Deferred Ideas

- Add a second external contrast corpus only if `common-pile/github_archive` cannot produce a clear enough within-corpus contrast story.
