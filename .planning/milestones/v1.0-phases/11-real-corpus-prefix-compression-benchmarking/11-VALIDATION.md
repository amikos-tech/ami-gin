---
phase: 11
slug: real-corpus-prefix-compression-benchmarking
status: verified
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-20
updated: 2026-04-20T12:47:16Z
---

# Phase 11 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

Formal requirement IDs are still `TBD` for Phase 11, so this validation map uses the roadmap success-criteria labels `SC-11-01` through `SC-11-04`.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing`, Go benchmark tooling, and repo-local docs/result grep checks |
| **Config file** | none - standard Go toolchain via `go.mod` |
| **Quick run command** | `go test ./... -run 'TestPhase11(DiscoverExternalShardsRejectsMissingLayout|LoadSmokeFixture|ShouldSkipBenchmarkDecodeOnConfiguredLimit)' -count=1 && go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=smoke' -benchtime=1x -count=1 && test -f testdata/phase11/github_archive_smoke.jsonl && rg -n 'Dataset: common-pile/github_archive|Dataset revision:|Fixture origin:|External layout: gharchive/v0/documents/\\*\\.jsonl\\.gz|Smoke rows:|Smoke bytes:' testdata/phase11/README.md && rg -n '^## (Dataset|Tier Matrix|Helps|Recommendation)$|^## Flat\\s*/\\s*No-Win$' .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-REAL-CORPUS-REPORT.md && rg -n 'GIN_PHASE11_GITHUB_ARCHIVE_ROOT|GIN_PHASE11_ENABLE_SUBSET|GIN_PHASE11_ENABLE_LARGE|BenchmarkPhase11RealCorpus/tier=smoke|common-pile/github_archive|gharchive/v0/documents/\\*\\.jsonl\\.gz|revision=|11-BENCHMARK-RESULTS\\.md|11-REAL-CORPUS-REPORT\\.md|opt-in only' README.md` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | quick run ~4s without external tiers; live subset proof ~75s; live large proof ~450s; full suite ~85s on the audit machine |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -run 'TestPhase11(DiscoverExternalShardsRejectsMissingLayout|LoadSmokeFixture|ShouldSkipBenchmarkDecodeOnConfiguredLimit)' -count=1 && go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=smoke' -benchtime=1x -count=1 && test -f testdata/phase11/github_archive_smoke.jsonl && rg -n 'Dataset: common-pile/github_archive|Dataset revision:|Fixture origin:|External layout: gharchive/v0/documents/\\*\\.jsonl\\.gz|Smoke rows:|Smoke bytes:' testdata/phase11/README.md && rg -n '^## (Dataset|Tier Matrix|Helps|Recommendation)$|^## Flat\\s*/\\s*No-Win$' .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-REAL-CORPUS-REPORT.md && rg -n 'GIN_PHASE11_GITHUB_ARCHIVE_ROOT|GIN_PHASE11_ENABLE_SUBSET|GIN_PHASE11_ENABLE_LARGE|BenchmarkPhase11RealCorpus/tier=smoke|common-pile/github_archive|gharchive/v0/documents/\\*\\.jsonl\\.gz|revision=|11-BENCHMARK-RESULTS\\.md|11-REAL-CORPUS-REPORT\\.md|opt-in only' README.md`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `$gsd-verify-work`:** Full suite must be green, the invalid-root failure contract must pass, the pinned external subset and large commands must rerun against the root recorded in `11-BENCHMARK-RESULTS.md`, and the README/report grep checks must pass
- **Max feedback latency:** 90 seconds for repo-local validation, excluding opt-in external corpus runtime

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 11-01-01 | 01 | 1 | SC-11-01, SC-11-02 | T-11-01 / T-11-02 | Default benchmarking uses a checked-in smoke corpus, keeps the nested `metadata` shape intact, and exposes smoke-only behavior unless external tiers are explicitly enabled | unit + benchmark + file smoke | `go test ./... -run 'TestPhase11LoadSmokeFixture' -count=1 && go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=smoke' -benchtime=1x -count=1 && test -f testdata/phase11/github_archive_smoke.jsonl && rg -n 'Dataset: common-pile/github_archive|Dataset revision:|Fixture origin:|External layout: gharchive/v0/documents/\\*\\.jsonl\\.gz|Smoke rows:|Smoke bytes:' testdata/phase11/README.md` | ✅ `benchmark_test.go`, `testdata/phase11/github_archive_smoke.jsonl`, `testdata/phase11/README.md` | ✅ green |
| 11-01-02 | 01 | 1 | SC-11-02 | T-11-03 | Explicit opt-in with an invalid corpus root fails closed with a precise env-var and layout error instead of silently skipping | unit + benchmark setup | `go test ./... -run 'TestPhase11DiscoverExternalShardsRejectsMissingLayout' -count=1 && LOG=$(mktemp) && set +e && GIN_PHASE11_ENABLE_SUBSET=1 GIN_PHASE11_GITHUB_ARCHIVE_ROOT=/definitely-missing go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=subset' -benchtime=1x -count=1 >"$LOG" 2>&1; STATUS=$?; set -e; test "$STATUS" -ne 0 && rg -n 'GIN_PHASE11_GITHUB_ARCHIVE_ROOT|gharchive/v0/documents' "$LOG" && rm -f "$LOG"` | ✅ `benchmark_test.go` | ✅ green |
| 11-02-01 | 02 | 2 | SC-11-01, SC-11-02, SC-11-03 | T-11-04 / T-11-05 / T-11-06 / T-11-07 | Pinned subset and large runs report exact raw/compact/zstd plus string-payload metrics for both projections, preserve tier scale/resource visibility, and keep the raw results separate from the narrative report | benchmark + artifact | `ROOT=$(sed -n 's/^Root path: `\\(.*\\)`$/\\1/p' .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-BENCHMARK-RESULTS.md) && test -d "$ROOT/gharchive/v0/documents" && GIN_PHASE11_GITHUB_ARCHIVE_ROOT="$ROOT" GIN_PHASE11_ENABLE_SUBSET=1 go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=subset' -benchtime=1x -count=1 && GIN_PHASE11_GITHUB_ARCHIVE_ROOT="$ROOT" GIN_PHASE11_ENABLE_LARGE=1 go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=large' -benchtime=1x -count=1 && rg -n 'Dataset revision: 93d90fbdbc8f06c1fab72e74d5270dc897e1a090|Root path:|Snapshot layout: gharchive/v0/documents/\\*\\.jsonl\\.gz|## Smoke|## Subset|## Large|projection=structured|projection=text-heavy|legacy_raw_bytes|compact_raw_bytes|default_zstd_bytes|legacy_string_payload_bytes|compact_string_payload_bytes|bytes_saved_pct' .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-BENCHMARK-RESULTS.md` | ✅ `benchmark_test.go`, `.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-BENCHMARK-RESULTS.md` | ✅ green |
| 11-03-01 | 03 | 3 | SC-11-04 | T-11-08 / T-11-09 | Final docs explicitly separate help from flat/no-win cases, pin the external workflow, and link the reader back to the raw results artifact instead of drifting into uncited conclusions | docs smoke | `rg -n '^## (Dataset|Tier Matrix|Helps|Recommendation)$|^## Flat\\s*/\\s*No-Win$' .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-REAL-CORPUS-REPORT.md && rg -n 'projection=structured|projection=text-heavy|11-BENCHMARK-RESULTS\\.md|Dataset revision:|default_zstd_bytes|zstd' .planning/phases/11-real-corpus-prefix-compression-benchmarking/11-REAL-CORPUS-REPORT.md && rg -n 'GIN_PHASE11_GITHUB_ARCHIVE_ROOT|GIN_PHASE11_ENABLE_SUBSET|GIN_PHASE11_ENABLE_LARGE|BenchmarkPhase11RealCorpus/tier=smoke|common-pile/github_archive|gharchive/v0/documents/\\*\\.jsonl\\.gz|revision=|11-BENCHMARK-RESULTS\\.md|11-REAL-CORPUS-REPORT\\.md|opt-in only' README.md` | ✅ `README.md`, `.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-REAL-CORPUS-REPORT.md` | ✅ green |

*Status: ⬜ pending - ✅ green - ❌ red - ⚠ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements.

---

## Manual-Only Verifications

All phase behaviors are automatable on this machine. The external proof remains opt-in, but the pinned snapshot root recorded in `11-BENCHMARK-RESULTS.md` was present locally and revalidated during this audit.

---

## Validation Audit 2026-04-20

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

Audit evidence:
- Confirmed State A input: existing `11-VALIDATION.md` plus executed `11-01` through `11-03` summary artifacts.
- Reviewed `11-CONTEXT.md`, `11-01-PLAN.md`, `11-02-PLAN.md`, `11-03-PLAN.md`, `.planning/ROADMAP.md`, and all three summaries to map `SC-11-01` through `SC-11-04` onto the implemented tasks and threat refs.
- Confirmed `.planning/config.json` does not set `workflow.nyquist_validation`; the key is absent rather than `false`, so Nyquist validation remains enabled.
- Cross-referenced the live verification assets in `benchmark_test.go`, `testdata/phase11/github_archive_smoke.jsonl`, `testdata/phase11/README.md`, `.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-BENCHMARK-RESULTS.md`, `.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-REAL-CORPUS-REPORT.md`, and `README.md`.
- Verified focused Phase 11 helper coverage with `go test ./... -run 'TestPhase11(DiscoverExternalShardsRejectsMissingLayout|LoadSmokeFixture|ShouldSkipBenchmarkDecodeOnConfiguredLimit)' -count=1`, which passed in `0.773s`.
- Verified the default smoke tier with `go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=smoke' -benchtime=1x -count=1`, which passed in `1.900s` and reported both `projection=structured` and `projection=text-heavy`.
- Verified the explicit bad-root failure contract with `GIN_PHASE11_ENABLE_SUBSET=1 GIN_PHASE11_GITHUB_ARCHIVE_ROOT=/definitely-missing go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=subset' -benchtime=1x -count=1`, which failed as expected and mentioned both `GIN_PHASE11_GITHUB_ARCHIVE_ROOT` and `gharchive/v0/documents/*.jsonl.gz`.
- Verified the pinned external snapshot root from `11-BENCHMARK-RESULTS.md` still exists locally and exposes `gharchive/v0/documents/*.jsonl.gz` under revision `93d90fbdbc8f06c1fab72e74d5270dc897e1a090`.
- Verified the live subset external tier with `GIN_PHASE11_GITHUB_ARCHIVE_ROOT="$ROOT" GIN_PHASE11_ENABLE_SUBSET=1 go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=subset' -benchtime=1x -count=1`, which passed in `70.546s`.
- Verified the live large external tier with `GIN_PHASE11_GITHUB_ARCHIVE_ROOT="$ROOT" GIN_PHASE11_ENABLE_LARGE=1 go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=large' -benchtime=1x -count=1`, which passed in `445.830s`; the large text-heavy branch emitted size, encode, and query-after-decode evidence and intentionally omitted `Decode`, matching the checked-in artifact.
- Verified the report and README docs smoke with the `rg` commands from the task map; both passed and confirmed the pinned revision, exact env vars, smoke command, report headings, and links back to `11-BENCHMARK-RESULTS.md`.
- Verified the repo-wide regression sweep with `go test ./... -count=1`, which passed in `83.771s` for `github.com/amikos-tech/ami-gin` on this machine.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 90s for repo-local checks
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-04-20
