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
| **Focused tagged command** | `AMI_GIN_SIMD_REQUIRED=1 go test -v -tags simdjson -run '^(TestSupportedSIMDPlatform|TestClassifySIMDLoadError|TestSIMDConstructionPolicy|TestSIMDConstructorCallPolicy|TestSIMDParser.*|TestFuzzParserParityCorpus|TestClassifyFuzzParserOutcomes|FuzzParserParity)$' -count=1 .` |
| **Full suite command** | `go test ./... && AMI_GIN_SIMD_REQUIRED=1 go test -tags simdjson -race -timeout=30m ./... && make security-scan` (`security-scan` runs default and `-tags simdjson` govulncheck after Plan 22-07) |
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
| T-22-04 | CI tooling, optional tagged dependencies, or uploaded artifacts expand the supply-chain/disclosure surface | Blocking provenance decisions, default plus tagged govulncheck, `contents: read`, and benchmark-text-only uploads |

---

## Per-Task Verification Map

Task IDs match the revised eight-plan execution graph.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 22-01-01 | 01 | 1 | SIMD-09 prerequisite | T-22-04 | Current pure-simdjson pin is explicitly approved before new Phase 22 tagged evidence; dependency files remain unchanged | blocking decision + diff gate | `git diff --exit-code -- go.mod go.sum` | Existing files ✅ | ⬜ pending |
| 22-01-02 | 01 | 1 | SIMD-09 | T-22-04 | Exact x/perf pseudo-version is approved and remains ephemeral | blocking decision + source assertion | `! rg -q '^\s*golang.org/x/perf\s' go.mod` | Existing files ✅ | ⬜ pending |
| 22-02-01 | 02 | 2 | SIMD-08, SIMD-10 | T-22-01 | Three-state policy and all six sentinel classes are native-free; an AST guard inspects identifier and selector callees, permits only the shared helper plus the exact no-output compile-only Example, and an external-package `gin.NewSIMDParser()` test mutation proves every executable bypass is rejected | tagged unit/integration contract | `go test -tags simdjson -run '^(TestSupportedSIMDPlatform|TestClassifySIMDLoadError|TestSIMDConstructionPolicy|TestSIMDConstructorCallPolicy)$' . && AMI_GIN_SIMD_REQUIRED=1 go test -tags simdjson -run '^TestSIMDParser' .` | ❌ W0 | ⬜ pending |
| 22-02-02 | 02 | 2 | SIMD-08 | — | Authored plus all four Phase 20 byte/query oracles and 24 Evaluate cases hard-fail on divergence | tagged integration | `AMI_GIN_SIMD_REQUIRED=1 go test -tags simdjson -run '^(TestSIMDParserGoldenAuthoredFixtures|TestSIMDParserPhase20Parity|TestSIMDParserEvaluateParity|TestSIMDParserMalformedTrailingNumericKnownPolicyAsymmetry)$' .` | ❌ W0 | ⬜ pending |
| 22-03-01 | 03 | 3 | SIMD-08 | T-22-03 | Five standard corpus files cover two authored, two Phase 20, and one known-exclusion category | corpus source check | `test "$(find testdata/fuzz/FuzzParserParity -type f | wc -l | tr -d ' ')" -eq 5` | ❌ W0 | ⬜ pending |
| 22-03-02 | 03 | 3 | SIMD-08 | T-22-03 | Bounds run before either arm; committed state and attribution classes are explicit; verbose seed replay exposes non-failing one-sided outcomes under `SIMD_FUZZ_OUTCOME class=unexpected_one_sided_commit` without timed fuzzing | tagged unit/fuzz replay | `AMI_GIN_SIMD_REQUIRED=1 go test -v -tags simdjson -run '^(TestFuzzParserParityCorpus|TestClassifyFuzzParserOutcomes|TestFuzzParserParityRejectsOverLimitBeforeArms|FuzzParserParity)$' .` | ❌ W0 | ⬜ pending |
| 22-04-01 | 04 | 4 | SIMD-11 | T-22-02 | Current parity is rerun, guide uses effective-tag links, the tagged consumer Example compiles without native execution, and the constructor guard accepts only its exact no-output selector exception | tagged integration/compile | `go test -tags simdjson -run '^(TestSIMDConstructorCallPolicy|TestSIMDParserPhase20Parity|FuzzParserParity|ExampleNewSIMDParser)$' .` | ❌ W0 | ⬜ pending |
| 22-04-02 | 04 | 4 | SIMD-10, SIMD-11 | T-22-02, T-22-04 | One authoritative Go guard validates env/path/link/release/snippet contracts through Make | contract test | `make check-simd-docs` | ❌ W0 | ⬜ pending |
| 22-05-01 | 05 | 5 | SIMD-09 | T-22-04 | Paired steady-state smoke leaves report ns/op, B/op, allocs/op, and MB/s; external skip is subtree-only | benchmark smoke | `env -u GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL -u GIN_PHASE20_SIMDJSON_DIR AMI_GIN_SIMD_REQUIRED=1 go test -tags simdjson -run '^$' -bench '^BenchmarkSIMDTypedSinkIngest$' -benchmem -benchtime=100ms -count=1 .` | ❌ W0 | ⬜ pending |
| 22-05-02 | 05 | 5 | SIMD-09 | — | Make target stays anchored and overridable | command contract | `env -u GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL -u GIN_PHASE20_SIMDJSON_DIR AMI_GIN_SIMD_REQUIRED=1 make bench-simd COUNT=1 BENCHTIME=100ms` | ❌ W0 | ⬜ pending |
| 22-07-01 | 07 | 6 | SIMD-10 | T-22-01, T-22-02 | Full-workflow assertions cover top-level policy; job-local assertions cover exact five rows/cache | workflow contract | `go test -run '^TestSIMDWorkflowContract/matrix_and_cache$' .` | ❌ W0 | ⬜ pending |
| 22-07-02 | 07 | 6 | SIMD-09, SIMD-10, SIMD-11 | T-22-02, T-22-04 | Explicit path is file-validated, trend output is non-PR text only, docs lint is ordered, and security scan covers both call graphs | workflow/Make contract | `go test -run '^TestSIMDWorkflowContract$' . && rg -Uq 'govulncheck ./...\n\s+govulncheck -tags simdjson ./...' Makefile` | ❌ W0 | ⬜ pending |
| 22-06-01 | 06 | 7 | SIMD-08, SIMD-09 | T-22-02, T-22-04 | Controlled clean-source smoke capture has exactly 80 samples and no external tier | artifact cardinality | `test -s .planning/phases/22-simd-validation-benchmarks-ci/22-SIMD-BENCHMARK-RESULTS.md && ! rg '^Benchmark.*tier=external/' .planning/phases/22-simd-validation-benchmarks-ci/22-SIMD-BENCHMARK-RESULTS.md` | ❌ W0 | ⬜ pending |
| 22-06-02 | 06 | 7 | SIMD-09 | — | Report traces raw results and ends in exactly one ship/defer/narrow class | artifact contract | `rg -q '^Recommendation class: \*\*(ship|defer|narrow)\*\*$' .planning/phases/22-simd-validation-benchmarks-ci/22-SIMD-BENCHMARK-REPORT.md` | ❌ W0 | ⬜ pending |
| 22-08-01 | 08 | 8 | SIMD-08..11 | T-22-01..04 | Default, required tagged-race, and both default/tagged vulnerability scans are green | full phase gate | `go test ./... && AMI_GIN_SIMD_REQUIRED=1 go test -tags simdjson -race -timeout=30m ./... && make security-scan` | Prior artifacts | ⬜ pending |
| 22-08-02 | 08 | 8 | SIMD-10 | T-22-01 | A completed PR run for the exact workflow head SHA exists before remote inspection | human action | Run URL + head SHA | Remote state | ⬜ pending |
| 22-08-03 | 08 | 8 | SIMD-08..11 | T-22-01, T-22-02, T-22-04 | Five remote outcomes, SIMD-only required bindings, and controlled recommendation receive one read-only approval | human verification | Local workflow/artifact contracts plus completed-run/ruleset review | Remote state + evidence | ⬜ pending |

---

## Wave 0 Requirements

- [ ] `parser_parity_phase20_simd_test.go` — realistic encoded-byte and query parity for SIMD-08.
- [ ] `parser_parity_fuzz_simd_test.go` and `testdata/fuzz/FuzzParserParity/*` — bounded seed replay for SIMD-08.
- [ ] Shared Evaluate-case extraction and variadic parser options in `parser_parity_test.go`; variadic Phase 20 build helper in `benchmark_test.go`.
- [ ] `benchmark_simd_test.go` and `bench-simd` — paired SIMD-09 measurement.
- [ ] `simd_docs_guard_test.go` and `check-simd-docs` — authoritative SIMD-11 documentation contract.
- [ ] `simd_workflow_contract_test.go` — full-workflow plus job-local SIMD-10 CI contract.
- [ ] `simd_example_test.go` — tagged compile-only consumer example.
- [ ] CI matrix, validated explicit-path smoke, non-PR trend artifact, dual-call-graph security target, and two benchmark evidence documents.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Accept `github.com/amikos-tech/pure-simdjson` and the selected `golang.org/x/perf` pseudo-version after the research legitimacy warnings | SIMD-09 prerequisite | The package audit returned `[SUS]`, requiring a blocking decision before new Phase 22 evidence commands | Verify module paths, resolved versions, and source origins; approve or reject each exact module/tool without changing dependency files |
| Publish or identify a completed PR run for the exact Plan 22-07 workflow revision | SIMD-10 | A remote matrix does not exist until an authorized repository publication action triggers it | Supply run URL and head SHA; do not create/switch branches, merge, or mutate rules during the checkpoint |
| Verify all five supported legs, SIMD-only required bindings, and the controlled recommendation | SIMD-08..11 | Remote runner/ruleset state and performance interpretation are outside source-only proof | Required legs green, advisory outcomes recorded, no supported skip, only two required among SIMD contexts, and controlled evidence supports one recommendation |

---

## Validation Sign-Off

- [x] All prospective tasks have automated verification or explicit manual dependencies.
- [x] Sampling continuity: no three consecutive tasks lack automated verification.
- [x] Wave 0 lists every missing test or artifact named by the research map.
- [x] Commands contain no watch-mode flags.
- [x] Local feedback latency is bounded at 30 minutes.
- [x] `nyquist_compliant: true` is set in frontmatter.

**Approval:** pending plan-checker verification
