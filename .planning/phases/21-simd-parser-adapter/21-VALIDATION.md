---
phase: 21
slug: simd-parser-adapter
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-07-22
audited: 2026-07-23
audit_state: A
---

# Phase 21 — Validation Strategy

> Retrospectively audited against all five PLAN/SUMMARY pairs, the completed
> implementation, and the current default and `simdjson`-tagged test suites.
> Phase 21 owns the typed sink, tagged adapter, lifecycle integrity, numeric
> fixtures, documentation, and default-build isolation. Real native-runtime
> parity, benchmarks, and platform CI remain Phase 22 deliverables.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `gotest.tools/gotestsum` (via Makefile); `leanovate/gopter` for property tests |
| **Config file** | None (Go convention); commands are defined in `Makefile` |
| **Quick run command** | `go test -count=1 -run '^TestTypedSink|^TestParserParity_AuthoredFixtures/simd-numeric-parity$|^TestParserLifecycle|^TestNewBuilderDefaultsToStdlibParser$' .` |
| **Tagged lifecycle command** | `go test -tags simdjson -count=1 -run '^TestSIMDDocumentLifecycle' .` |
| **Full suite commands** | `make test`; `go test -tags simdjson -count=1 ./...`; default/tagged build and vet; `make simd-isolation-check` |
| **Observed runtime** | Focused checks under 1 second each; fresh default suite ~26 seconds; fresh tagged suite ~38 seconds |

---

## Sampling Rate

- **After typed-sink or numeric changes:** Run the quick command above.
- **After lifecycle changes:** Run both `TestParserLifecycle` and the tagged `TestSIMDDocumentLifecycle` suite.
- **After build-tag or dependency changes:** Run `make simd-isolation-check`, then tagged build and vet.
- **Before phase verification:** Run both full suites, default/tagged build and vet, documentation assertions, and module integrity checks.
- **Max focused feedback latency:** About 5 seconds.

---

## Per-Task Verification Map

| Plan / Task | Wave | Requirements | Automated proof | Test or evidence | Status |
|-------------|------|--------------|-----------------|------------------|--------|
| 21-01 / 1 — typed sink contract | 1 | SIMD-06, SIMD-07 | `go test -count=1 -run '^TestTypedSink' .` | `parser_simd_test.go` commits through `AddDocument` and covers string/bool parity, exact int64, float class, uint64 overflow, and non-finite hard/soft behavior | ✅ COVERED |
| 21-01 / 2 — numeric fixture/golden | 1 | SIMD-06 | `go test -count=1 -run '^TestParserParity_AuthoredFixtures/simd-numeric-parity$' .` | `parser_parity_fixtures_test.go` + non-empty `testdata/parity-golden/simd-numeric-parity.bin` | ✅ COVERED |
| 21-02 / 1 — attribution/deployment guide | 1 | SIMD-04, SIMD-05, SIMD-06 | File and `rg` assertions for NOTICE, all four bootstrap variables, failure modes, and constructor usage | `NOTICE.md`, `docs/simd-deployment.md` | ✅ COVERED |
| 21-02 / 2 — README/CHANGELOG activation | 1 | SIMD-04, SIMD-05, SIMD-06 | `rg` assertions for the optional parser section, two-step constructor, stdlib default, and BIGINT wording | `README.md`, `CHANGELOG.md` | ✅ COVERED |
| 21-03 / 1 — tagged SIMD adapter | 2 | SIMD-04, SIMD-05, SIMD-06, SIMD-07 | `go test -tags simdjson -count=1 -run '^$' ./... && go build -tags simdjson ./... && go vet -tags simdjson ./...` | Tagged compilation verifies the constructor, typed dispatch, transform materializer, and lifecycle wiring without loading a native library | ✅ COVERED |
| 21-03 / 2 — default isolation | 2 | SIMD-05 | `make simd-isolation-check` | Exact first-line tag scan plus default package/test dependency-graph exclusion, build, and vet | ✅ COVERED |
| 21-04 / 1 — fatal lifecycle routing | 3 | SIMD-04, SIMD-06, SIMD-07 | `go test -count=1 -run '^TestParserLifecycle' .` | `parser_lifecycle_test.go` covers multi-cause traversal, terminal builder state, zero soft skips, and retry refusal | ✅ COVERED |
| 21-04 / 2 — cleanup provenance | 3 | SIMD-04, SIMD-06, SIMD-07 | `go test -tags simdjson -count=1 -run '^TestSIMDDocumentLifecycle' .` | `parser_simd_lifecycle_test.go` covers close-only and hard/soft stage-plus-close failures through native-free callbacks | ✅ COVERED |
| 21-05 / 1 — panic-aware cleanup | 4 | SIMD-04 | `go test -tags simdjson -count=1 -run '^TestSIMDDocumentLifecycle' .` | Identity-preserving re-panic, panic-plus-close lifecycle error, caller recovery, and actual blocked retry dispatch | ✅ COVERED |

All Phase 21 requirements are covered by runnable automated commands. No task is
manual-only.

---

## Numeric Parity Fixture Coverage

| Fixture literal | Phase 21 proof | Status |
|-----------------|----------------|--------|
| `1` | Authored `simd-numeric-parity` fixture and stdlib golden | ✅ COVERED |
| `1.0`, `1e18` | Typed float-class tests plus fixture/golden | ✅ COVERED |
| `1.5` | Authored fixture/golden | ✅ COVERED |
| `9007199254740993` | Exact committed int64 test plus fixture/golden | ✅ COVERED |
| `18446744073709551615` | Typed uint64 hard rejection and atomic soft-skip tests | ✅ COVERED |
| Greater-than-uint64 BIGINT | Automated documentation assertions for the accepted parser-layer versus numeric-layer divergence | ✅ COVERED |

The oversized uint64 case is intentionally a behavior test rather than a byte
golden. The greater-than-uint64 case is an accepted, documented divergence and
is intentionally excluded from parity fixtures.

---

## Wave 0 Requirements

- [x] `parser_simd_test.go` covers typed wrapper and numeric semantics without a native runtime.
- [x] `simd-numeric-parity` is registered with a non-empty isolated golden.
- [x] `make simd-isolation-check` guards both source tags and the default dependency graph.
- [x] `parser_lifecycle_test.go` covers generic fatal lifecycle routing.
- [x] `parser_simd_lifecycle_test.go` covers returned-error and panic cleanup paths without constructing `NewSIMDParser`.

---

## Manual-Only Verifications

None for Phase 21.

## Deferred to Phase 22

| Behavior | Why deferred | Planned automated gate |
|----------|--------------|------------------------|
| Real native SIMD execution and encoded/query parity | Phase 22 owns runtime parity evidence | Tagged parity tests against authored and realistic fixtures |
| Native bootstrap/platform behavior and tagged CI | Phase 22 owns operational and platform evidence | Default and `simdjson`-tagged CI matrix with explicit native-library handling |
| SIMD benchmarks | Phase 22 owns performance evidence | Reproducible benchmark suite and comparison report |

These are later-phase requirements, not Phase 21 validation gaps.

---

## Validation Sign-Off

- [x] Every Phase 21 task has an automated verification command.
- [x] SIMD-04, SIMD-05, SIMD-06, and SIMD-07 each map to passing automated evidence.
- [x] No three consecutive implementation tasks lack automated feedback.
- [x] Default builds exclude the optional SIMD dependency.
- [x] Tagged lifecycle tests remain native-free and deterministic.
- [x] No watch-mode flags are used.
- [x] `nyquist_compliant: true` is set in frontmatter.

**Approval:** automated audit complete

---

## Validation Audit 2026-07-23

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |
| Phase requirements covered | 4 / 4 |
| Plan tasks covered | 9 / 9 |

Fresh audit evidence:

- Focused default checks passed.
- Focused tagged lifecycle checks passed.
- `make test` passed 1,042 tests; one unrelated Parquet fixture test was skipped because `testdata/test.parquet` is absent.
- Full tagged suite, tagged build, and tagged vet passed.
- Default build/vet and dependency isolation passed.
- Documentation assertions passed.
- `go mod verify` and `go mod tidy -diff` passed.
