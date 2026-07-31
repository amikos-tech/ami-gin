---
phase: 22
slug: simd-validation-benchmarks-ci
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-07-31
---

# Phase 22 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing`, `testing.F`, and benchmarks (module baseline Go 1.25.5) |
| **Config file** | None — package tests, build tags, and Make targets define the test surface |
| **Quick run command** | `go test ./... && make simd-isolation-check` |
| **Focused tagged command** | `AMI_GIN_SIMD_REQUIRED=1 go test -tags simdjson -run '^(TestSIMDParserGoldenAuthoredFixtures|TestSIMDParserPhase20Parity|TestSIMDParserEvaluateParity|FuzzParserParity)$' -count=1 .` |
| **Full suite command** | `go test ./... && AMI_GIN_SIMD_REQUIRED=1 go test -tags simdjson -race -timeout=30m ./... && make security-scan` |
| **Estimated runtime** | Quick: under 2 minutes; full local suite: under 30 minutes; five-platform CI is remote |

---

## Sampling Rate

- **After every task commit:** Run the focused command for the changed contract plus `go test ./...`; importer changes also run `make simd-isolation-check`.
- **After every plan wave:** Run the full default and required local tagged/race commands.
- **Before `$gsd-verify-work`:** The full suite must be green, parity must pass before benchmark evidence is accepted, and remote matrix outcomes must be recorded.
- **Max feedback latency:** 30 minutes for local verification; remote runner latency is tracked separately.

---

## Threat References

| ID | Threat | Required mitigation |
|----|--------|---------------------|
| T-22-01 | A supported CI leg silently skips SIMD after native loading fails | `AMI_GIN_SIMD_REQUIRED=1` converts supported-platform construction failures into classified fatal errors |
| T-22-02 | A cache restores a native library for the wrong module version or platform | Exact version/platform cache key, no `restore-keys`, and version derived from `go list -m` |
| T-22-03 | Deeply nested fuzz input exhausts CPU in shared builder staging | Reject fuzz inputs over 4096 bytes or array nesting depth 8 before either parser arm runs |
| T-22-04 | CI tooling or uploaded artifacts expand the supply-chain or disclosure surface | Human-verify flagged modules, keep `contents: read`, and upload benchmark text only |

---

## Per-Task Verification Map

Task IDs are provisional planning anchors; the planner may renumber them but must preserve every requirement/command mapping.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 22-01-01 | 01 | 1 | SIMD-08 | — | Both parser arms receive identical options and fixtures | unit/integration | `go test -tags simdjson -run '^(TestSIMDParserGoldenAuthoredFixtures|TestSIMDParserPhase20Parity|TestSIMDParserEvaluateParity)$' .` | ❌ W0 | ⬜ pending |
| 22-01-02 | 01 | 1 | SIMD-08 | T-22-03 | Fuzz seeds are bounded before either parser executes; byte divergence hard-fails | fuzz-seed regression | `go test -tags simdjson -run '^FuzzParserParity$' .` | ❌ W0 | ⬜ pending |
| 22-02-01 | 02 | 2 | SIMD-09 | — | Setup is outside timing and paired output reports ns/op, B/op, allocs/op, and MB/s | benchmark smoke | `make bench-simd COUNT=1 BENCHTIME=100ms` | ❌ W0 | ⬜ pending |
| 22-02-02 | 02 | 2 | SIMD-09 | T-22-04 | Evidence uses the human-approved exact benchstat version and contains no environment dump | artifact check | `test -s .planning/phases/22-simd-validation-benchmarks-ci/22-SIMD-BENCHMARK-RESULTS.md && test -s .planning/phases/22-simd-validation-benchmarks-ci/22-SIMD-BENCHMARK-REPORT.md` | ❌ W0 | ⬜ pending |
| 22-03-01 | 03 | 1 | SIMD-11 | T-22-02 | Docs derive version/env/path facts from machine-readable sources and pin upstream links | contract test | `make check-simd-docs` | ❌ W0 | ⬜ pending |
| 22-03-02 | 03 | 1 | SIMD-11 | — | Consumer example compiles under the tag without loading the native library | compile test | `go test -tags simdjson -run '^Example' .` | ❌ W0 | ⬜ pending |
| 22-04-01 | 04 | 2 | SIMD-10 | T-22-01, T-22-02, T-22-04 | Five explicit legs, two required race legs, three advisory legs, exact cache, and explicit-path rerun are enforced | workflow contract | `go test -run '^TestSIMDWorkflowContract$' .` | ❌ W0 | ⬜ pending |
| 22-04-02 | 04 | 3 | SIMD-10 | T-22-01 | Default isolation stays green and every supported remote leg attempts SIMD without a permitted skip | build/remote integration | `make simd-isolation-check` plus the GitHub Actions `simd` matrix | Workflow update ❌ W0 | ⬜ pending |

---

## Wave 0 Requirements

- [ ] `parser_parity_phase20_simd_test.go` — realistic encoded-byte and query parity for SIMD-08.
- [ ] `parser_parity_fuzz_simd_test.go` and `testdata/fuzz/FuzzParserParity/*` — bounded seed replay for SIMD-08.
- [ ] Shared Evaluate-case extraction and variadic parser options in `parser_parity_test.go`; variadic Phase 20 build helper in `benchmark_test.go`.
- [ ] `benchmark_simd_test.go` and `bench-simd` — paired SIMD-09 measurement.
- [ ] `simd_contract_guard_test.go` and `check-simd-docs` — SIMD-10/11 workflow and documentation contracts.
- [ ] `simd_example_test.go` — tagged compile-only consumer example.
- [ ] CI matrix, explicit-path rerun, non-PR trend artifact, and the two benchmark evidence documents.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Accept `github.com/amikos-tech/pure-simdjson` and the selected `golang.org/x/perf` pseudo-version after the research legitimacy warnings | SIMD-08, SIMD-09, SIMD-10 | The package audit returned `[SUS]`, requiring human provenance review before first use/download | Verify module paths, resolved versions, and source origins; record approval before tagged evidence or pinned `go run benchstat` |
| Only the stable Linux/amd64 and Darwin/arm64 matrix check names block merge | SIMD-10 | Repository rulesets are external state and are not represented by workflow YAML | After the workflow lands, inspect the ruleset and require exactly the two required-tier check names; record advisory names as non-blocking |
| All five supported platform legs load and run under the intended required/advisory policy | SIMD-10 | Three target platforms are unavailable locally | Inspect one completed PR matrix: required legs green; advisory outcomes recorded; no supported leg skipped native construction |
| Controlled benchmark evidence supports a ship, defer, or narrow decision | SIMD-09 | Performance interpretation depends on pinned machine/environment metadata and measured variance | Run `COUNT=10` or higher after parity passes, analyze the single output with approved benchstat, and review both committed evidence documents |

---

## Validation Sign-Off

- [x] All prospective tasks have automated verification or explicit manual dependencies.
- [x] Sampling continuity: no three consecutive tasks lack automated verification.
- [x] Wave 0 lists every missing test or artifact named by the research map.
- [x] Commands contain no watch-mode flags.
- [x] Local feedback latency is bounded at 30 minutes.
- [x] `nyquist_compliant: true` is set in frontmatter.

**Approval:** pending plan-checker verification
