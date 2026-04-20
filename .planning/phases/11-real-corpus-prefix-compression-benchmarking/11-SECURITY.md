---
phase: 11
slug: real-corpus-prefix-compression-benchmarking
status: verified
threats_open: 0
asvs_level: 1
created: 2026-04-20
---

# Phase 11 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| checked-in smoke fixture -> benchmark credibility | The checked-in fixture must preserve the locked corpus shape and safe provenance so smoke results remain representative. | Dataset revision, fixture origin, synthetic corpus rows, nested metadata shape |
| env vars -> tier activation | External tiers must only run when explicitly enabled and must stay visibly distinct from smoke runs. | `GIN_PHASE11_ENABLE_SUBSET`, `GIN_PHASE11_ENABLE_LARGE`, benchmark tier labels |
| corpus root path -> benchmark setup | The external corpus root must resolve to the pinned shard layout or fail closed. | `GIN_PHASE11_GITHUB_ARCHIVE_ROOT`, `gharchive/v0/documents/*.jsonl.gz` |
| benchmark metrics -> artifact claims | Reported metrics must match the benchmark implementation so later artifact claims stay verifiable. | `legacy_raw_bytes`, `compact_raw_bytes`, `default_zstd_bytes`, string-payload metrics, `docs_indexed`, `shards_loaded`, `B/op`, `allocs/op` |
| raw results -> narrative report | The final recommendation must be derived from checked-in benchmark evidence rather than paraphrase or reruns. | `11-BENCHMARK-RESULTS.md`, `11-REAL-CORPUS-REPORT.md` |
| README workflow -> maintainer behavior | Reproduction docs must keep smoke default and make external tiers opt-in with pinned acquisition guidance. | README commands, env vars, dataset revision, tier resource notes |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-11-01 | R/L | smoke fixture provenance | mitigate | `testdata/phase11/README.md` pins dataset revision `93d90fbdbc8f06c1fab72e74d5270dc897e1a090`, records `Fixture origin: synthesized-from-shape`, and documents why direct redistribution was rejected. | closed |
| T-11-02 | T/R | tier activation | mitigate | `benchmark_test.go` gates external tiers behind `GIN_PHASE11_ENABLE_SUBSET` and `GIN_PHASE11_ENABLE_LARGE`, registers stable `tier=` labels, and skips opt-in tiers when the env vars are unset. | closed |
| T-11-03 | T | external root path | mitigate | `benchmark_test.go` requires `GIN_PHASE11_GITHUB_ARCHIVE_ROOT` for enabled external tiers and fails closed when shard discovery does not match `gharchive/v0/documents/*.jsonl.gz`. | closed |
| T-11-04 | R | benchmark metrics | mitigate | `benchmark_test.go` reports the exact Phase 10 size metrics plus `legacy_string_payload_bytes`, `compact_string_payload_bytes`, `docs_indexed`, and `shards_loaded` for every Phase 11 branch. | closed |
| T-11-05 | R | snapshot provenance | mitigate | `11-BENCHMARK-RESULTS.md` records the pinned dataset revision, exact snapshot root, env vars, shard counts, and document counts for smoke, subset, and large runs. | closed |
| T-11-06 | D/R | resource drift | mitigate | `11-BENCHMARK-RESULTS.md` records `B/op`, `allocs/op`, and tier growth (`640 < 60466 < 486239`), while the large text-heavy `Decode()` path stays capped and benchmark-skipped instead of weakening the 64 MiB decompression limit. | closed |
| T-11-07 | I | interpretive drift | mitigate | Raw evidence remains checked in separately in `11-BENCHMARK-RESULTS.md`, and the narrative report links back to that artifact instead of restating uncited conclusions. | closed |
| T-11-08 | R | interpretive report | mitigate | `11-REAL-CORPUS-REPORT.md` contains explicit `Helps`, `Flat / No-Win`, and `Recommendation` sections tied to `11-BENCHMARK-RESULTS.md`, including whether `default_zstd_bytes` masked raw wins. | closed |
| T-11-09 | T/R | README workflow | mitigate | `README.md` documents the exact smoke command, exact env vars, pinned dataset revision, locked shard layout, opt-in-only subset/large tiers, and links to the raw results and final report. | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

No accepted risks.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-04-20 | 9 | 9 | 0 | Codex `gsd-secure-phase` |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-04-20
