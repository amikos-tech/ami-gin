---
phase: 20
slug: realistic-benchmark-dataset-foundation
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-07-21
---

# Phase 20 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard-library `testing` package; table-driven tests and `testing.B` sub-benchmarks in the root `gin` package. |
| **Config file** | `go.mod`; no Phase-20-specific configuration is required. |
| **Quick run command** | `go test ./... -run '^TestPhase20' -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | Under 60 seconds for focused tests and the one-iteration smoke benchmark. |

---

## Sampling Rate

- **After every fixture or fixture-policy task:** Run `go test ./... -run '^TestPhase20SmokeFixtures$' -count=1`.
- **After every benchmark/helper or README task:** Run `go test ./... -run '^TestPhase20' -count=1` and `go test ./... -run '^$' -bench '^BenchmarkPhase20RealisticJSON/tier=smoke' -benchtime=1x -count=1 -benchmem`.
- **After every plan wave:** Run `go test ./... -count=1`.
- **Before `$gsd-verify-work`:** Full suite must be green.
- **Max feedback latency:** 60 seconds.

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 20-01-01 | 01 | 1 | DATA-01 | T20-01 | The deterministic generator reproduces the committed corpus without drift, while `TestPhase20SmokeFixtures` checks required provenance, offline, license, and environment-variable policy markers without coupling correctness to README table formatting. | unit/source | `go run ./testdata/phase20/generate.go && git diff --exit-code -- testdata/phase20/*.jsonl`; `go test ./... -run '^TestPhase20SmokeFixtures$' -count=1` | ✅ `benchmark_test.go` | ✅ green |
| 20-01-02 | 01 | 1 | DATA-02 | T20-02 | `TestPhase20SmokeFixtures` verifies four valid, bounded, structurally representative multi-row-group JSONL streams and the exact raw combined-stream interleaving; `TestPhase20LoadRawJSONLRejectsMalformedLines` verifies blank/malformed diagnostics. | unit | `go test ./... -run '^TestPhase20(SmokeFixtures|LoadRawJSONLRejectsMalformedLines)$' -count=1` | ✅ `benchmark_test.go` | ✅ green |
| 20-02-01 | 02 | 2 | DATA-03 | T20-03 | Default smoke benchmarks use only committed files; focused tests cover exact external gating, deterministic local discovery, derived queries, invalid inputs, and resource limits. Registered external leaves skip with setup guidance unless the enable value is exactly `1`. | unit/benchmark | `go test ./... -run '^TestPhase20ExternalTier$' -count=1`; `env -u GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL -u GIN_PHASE20_SIMDJSON_DIR go test ./... -run '^$' -bench '^BenchmarkPhase20RealisticJSON/tier=smoke' -benchtime=1x -count=1 -benchmem`; `env -u GIN_PHASE20_SIMDJSON_DIR GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL=true go test ./... -run '^$' -bench 'BenchmarkPhase20RealisticJSON/tier=external/fixture=local-example' -benchtime=1x -count=1 -v` | ✅ `benchmark_test.go` | ✅ green |
| 20-02-02 | 02 | 2 | DATA-01, DATA-03 | T20-03 | README policy and command markers are source-checked, while the Phase-20 tests and offline smoke benchmark prove that the documented selectors target working behavior. | source/benchmark | `rg -n "Phase 20 Realistic JSON Workflow|BenchmarkPhase20RealisticJSON/tier=smoke|BenchmarkPhase20RealisticJSON/tier=external/fixture=local-example|GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL|GIN_PHASE20_SIMDJSON_DIR|download" README.md`; `go test ./... -run '^TestPhase20' -count=1`; smoke benchmark command above | ✅ `README.md`, `benchmark_test.go` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing Go test infrastructure covers all phase requirements. Plan 01 adds the focused fixture-contract test; no framework, package, configuration, or test runner setup is needed.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| A real local simdjson checkout is accepted as optional external input without a download. | DATA-03 | The checkout is intentionally not committed and must not be required for normal tests. | Set `GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL=1` and `GIN_PHASE20_SIMDJSON_DIR=/path/to/simdjson/jsonexamples`, then run `go test ./... -run '^$' -bench '^BenchmarkPhase20RealisticJSON/tier=external' -benchtime=1x -count=1 -benchmem`. |
| License/NOTICE review before a future direct upstream-data redistribution. | DATA-01 | Default data is synthesized; upstream licensing must be checked at the selected future revision. | Review that revision's LICENSE/NOTICE and update provenance documentation before committing copied upstream rows. |

---

## Validation Sign-Off

- [x] All tasks have automated verification or existing-infrastructure dependencies.
- [x] Sampling continuity: no three consecutive tasks lack automated verification.
- [x] Wave 0 infrastructure is already present.
- [x] No watch-mode flags.
- [x] Feedback latency is below 60 seconds for focused validation.
- [x] `nyquist_compliant: true` set in frontmatter.

**Approval:** approved by retroactive Nyquist audit on 2026-07-21

---

## Validation Audit 2026-07-21

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

All Phase 20 requirements have automated verification. The focused tests, offline smoke benchmark, disabled external-tier registration check, deterministic fixture refresh, and full regression suite passed during this audit.
