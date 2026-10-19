---
phase: 22-simd-validation-benchmarks-ci
verified: 2026-10-19T00:00:00Z
status: passed
score: 4/4 roadmap success criteria verified (plus all plan-level must-haves)
overrides_applied: 0
re_verification: false
gaps: []
deferred: []
human_verification: []
---

# Phase 22: SIMD Validation, Benchmarks & CI Verification Report

**Phase Goal:** Prove the SIMD path is correct, measurable, and operationally shippable.
**Verified:** 2026-10-19
**Status:** passed
**Re-verification:** No (initial verification)

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Parity tests prove SIMD and stdlib produce identical encoded indexes and query results across authored fixtures and Phase 20 datasets | VERIFIED | `parser_parity_phase20_simd_test.go` builds all four Phase 20 smoke fixtures with both parsers, calls `Encode`, and hard-fails via `assertByteIdentical`. It also compares query results, plus the shared 24-case Evaluate matrix. I ran the tagged tests with `AMI_GIN_SIMD_REQUIRED=1`. They passed: `TestSIMDParserGoldenAuthoredFixtures`, `TestSIMDParserPhase20Parity` (4 subtests), `TestSIMDParserEvaluateParity`, `TestSIMDParserMalformedTrailingNumericKnownPolicyAsymmetry`, `FuzzParserParity` (5 seeds), the corpus and classifier tests, and the three-state policy tests. |
| 2 | Benchmarks compare stdlib vs SIMD typed-sink ingest on realistic fixtures and report CPU, allocation, and bytes/op deltas | VERIFIED | `benchmark_simd_test.go` has `BenchmarkSIMDTypedSinkIngest` with paired `parser=stdlib` and `parser=simd` arms on the 4 smoke fixtures. It uses `ReportAllocs` and `SetBytes`. I ran it with `-benchtime=1x`, and it produced ns/op, MB/s, B/op and allocs/op for all 8 arms. `make bench-simd` is anchored to this benchmark. `22-SIMD-BENCHMARK-RESULTS.md` has 80 samples (10 per arm per fixture), 0 external-tier lines and pinned benchstat output. `22-SIMD-BENCHMARK-REPORT.md` reports per-fixture CPU, throughput, B/op and allocs/op deltas. It ends in a single `Recommendation class: **defer**`, which passes the plan's regex check. |
| 3 | CI covers default and `-tags simdjson` builds with explicit skip/fail behavior when platform or shared-library requirements are unmet | VERIFIED | `.github/workflows/ci.yml` keeps the default `test`, `lint` and `build` jobs. It adds a `simd` job with exactly five matrix rows: 2 required (linux/amd64, darwin/arm64, with `-race`) and 3 advisory (`continue-on-error`). It sets `fail-fast: false` and `AMI_GIN_SIMD_REQUIRED=1` on all rows. The cache key is version and platform specific, with no `restore-keys`. An explicit-path rerun uses `PURE_SIMDJSON_LIB_PATH`. The trend job runs only on push to main and `workflow_dispatch`, is non-gating, and uploads text only. The lint job runs `make check-simd-docs`. `newTestSIMDParser` implements unsupported-platform skip, supported-host skip with remediation, and fatal on `AMI_GIN_SIMD_REQUIRED=1`. An AST guard (`TestSIMDConstructorCallPolicy`) blocks bypass. `TestSIMDWorkflowContract` and `make simd-isolation-check` pass. `make security-scan` runs both default and `-tags simdjson` govulncheck. |
| 4 | Runtime loading and release/distribution guidance explains how consumers enable SIMD without guesswork | VERIFIED | `docs/simd-deployment.md` (275 lines) covers: three independent enablement gates, a compile-checked snippet (`ExampleNewSIMDParser` with SIMD_EXAMPLE markers and no `// Output:`), automatic bootstrap, air-gapped `PURE_SIMDJSON_LIB_PATH`, mirror and hermetic variables (exactly four), a caller-owned stdlib fallback, an integrity table, depth and numeric limits, the qualified parity claim, and the five-platform tier table. `.goreleaser.yml` release header links the guide at `{{ .Tag }}`. `TestSIMDDocumentationContract` passes and is wired into `lint` through `check-simd-docs`. The guide makes no speedup claim, which is consistent with the `defer` recommendation. |

**Score:** 4/4 truths verified

### Plan-level must-haves (spot check beyond roadmap)

| Plan | Key must-have | Status | Evidence |
|------|---------------|--------|----------|
| 22-01 | Provenance approvals recorded; no dependency mutation | VERIFIED | `git diff --exit-code -- go.mod go.sum` clean. `golang.org/x/perf` is absent from `go.mod`. Both approvals are in the 22-01 SUMMARY. |
| 22-02 | Authored and Phase 20 byte and query parity; D-07 shared constructor; AST guard | VERIFIED | Tests pass (see above). `newTestSIMDParser` is used by parity and benchmark code. |
| 22-03 | Bounded fuzz with 5 seeds, no new fuzz job | VERIFIED | 5 corpus files in `testdata/fuzz/FuzzParserParity/`. The over-limit rejection, corpus and classifier tests pass. CI has no fuzz job. |
| 22-04 | Docs guard, Example, goreleaser signal, `check-simd-docs` in lint | VERIFIED | Files exist, tests pass, Makefile `lint:` depends on `check-simd-docs`. |
| 22-05 | Paired benchmark and anchored `bench-simd` | VERIFIED | Benchmark run output above. Makefile target uses `-bench '^BenchmarkSIMDTypedSinkIngest$'`. |
| 22-06 | COUNT=10 controlled results and a neutral report with one recommendation | VERIFIED | 80 samples, 0 external, benchstat pinned to the approved pseudo-version. The report states the steady-state caveat and the Go-heap-only `B/op` caveat. It keeps the D-04 qualified parity claim. |
| 22-07 | Five-leg matrix, cache, trend job, dual-graph security scan | VERIFIED | `ci.yml` and the `Makefile` `security-scan` target (`govulncheck ./...` then `govulncheck -tags simdjson ./...`). `TestSIMDWorkflowContract` passes. |
| 22-08 | Remote five-leg evidence, ruleset binds only the two required SIMD contexts, human approval of the recommendation | VERIFIED (external, accepted per instruction) | 22-08-SUMMARY records PR #55 run 30734887913 and main run 37498278533 (all five green), and the `Main` ruleset requiring exactly `SIMD (linux/amd64, required)` and `SIMD (darwin/arm64, required)`. The ruleset cannot be inspected from the repo. I accepted the record as instructed. |

### Build / Compile Integrity (Step 7d)

| Module | Command | Result | Status |
|--------|---------|--------|--------|
| root `github.com/amikos-tech/ami-gin` | `go build ./... && go vet ./...` | exit 0 | COMPILES |
| root, tagged | `go vet -tags simdjson ./...` (compiles tagged test files) | exit 0 | COMPILES |
| root, tagged benchmark | `go test -tags simdjson -run '^$' -bench ... -benchtime=1x` | ran | COMPILES |

Per the caller, the full gate (`go test ./...`, tagged `-race`, `make security-scan`) passed on 2026-10-19. I did not rerun the 30-minute race suite.

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|--------------|-------------|--------|----------|
| SIMD-08 | 22-02, 22-03, 22-06, 22-08 | Identical encoded indexes and query results across authored and realistic fixtures | SATISFIED | Truth 1 |
| SIMD-09 | 22-01, 22-05, 22-06, 22-07, 22-08 | Benchmarks report CPU, allocation and bytes/op deltas on realistic fixtures | SATISFIED | Truth 2 |
| SIMD-10 | 22-02, 22-04, 22-07, 22-08 | CI covers default and tagged builds with explicit unmet-requirement behavior | SATISFIED | Truth 3 |
| SIMD-11 | 22-04, 22-07, 22-08 | Runtime loading and release guidance for safe SIMD enablement | SATISFIED | Truth 4 |

All four IDs appear in PLAN frontmatter and are marked `[x]` and Complete in `.planning/REQUIREMENTS.md`. No orphaned requirements: REQUIREMENTS.md maps only SIMD-08 to SIMD-11 to Phase 22.

### Anti-Patterns Found

I found no `TBD`, `FIXME`, `XXX` or `TODO` markers in any Phase 22 file (test files, docs, `ci.yml`, `Makefile`). `22-REVIEW.md` reports 0 critical, 4 warning and 10 info findings. These are non-blocking for the goal. They are advisory polish and were not re-triaged here.

### Notes (non-blocking)

- The benchmark outcome is **defer**: SIMD is slower on all four fixtures (geomean +64.57% time, +28.50% Go-heap bytes, +56.12% allocs). The plan treats this as a legitimate outcome, not a phase failure. The phase goal is to prove the path is correct (parity), measurable (benchmarks and report) and operationally shippable (CI and docs). The phase delivers all three. It does not claim a performance win.
- The Windows advisory leg failed in the earlier PR run. The latest main run shows it green. Advisory legs do not gate merge, by design.
- Ruleset binding is external. It rests on the 22-08-SUMMARY record, as the caller instructed.

### Human Verification Required

None. The human checkpoints (provenance, ruleset and recommendation approval) were completed and recorded in 22-01-SUMMARY and 22-08-SUMMARY.

### Gaps Summary

No gaps. All four roadmap success criteria and all plan-level must-haves are verified against the codebase.

---

_Verified: 2026-10-19_
_Verifier: Claude (gsd-verifier)_
