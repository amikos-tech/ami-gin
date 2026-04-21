---
phase: 13
slug: parser-seam-extraction
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-21
---

# Phase 13 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Source of record: `13-RESEARCH.md` §Validation Architecture + §Verification Plan.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `github.com/leanovate/gopter` v0.2.11 |
| **Config file** | `go.mod` / `Makefile` (no extra framework config) |
| **Quick run command** | `go test -run TestParserParity -count=1 .` |
| **Full suite command** | `make test` |
| **Estimated runtime** | ~5s quick, ~60–120s full (existing suite baseline) |

---

## Sampling Rate

- **After every task commit:** `go test -run TestParserParity -count=1 .`
- **After every plan wave:** `make test` (full suite + 12-operator Evaluate matrix)
- **Before `/gsd-verify-work`:** Full suite green AND benchmark delta recorded — `go test -bench=BenchmarkAddDocument -benchmem -count=10 .` within **2% ns/op** and **0 additional allocs/op** vs. pre-refactor baseline captured on same commit
- **Max feedback latency:** 5 seconds (quick run)

---

## Per-Task Verification Map

> Populated by planner once plans are written. Until then, the requirement → test map below is the binding contract.

### PARSER-01 Requirement → Test Map

| # | Behavior | Test Type | Automated Command | File Exists |
|---|----------|-----------|-------------------|-------------|
| 1 | `WithParser(nil)` returns error | unit | `go test -run TestWithParserRejectsNil -x .` | ❌ W0 |
| 2 | `NewBuilder` defaults to `stdlibParser{}` with `Name() == "stdlib"` | unit | `go test -run TestNewBuilderDefaultsToStdlibParser -x .` | ❌ W0 |
| 3 | Empty `Name()` returns error at `NewBuilder` | unit | `go test -run TestNewBuilderRejectsEmptyParserName -x .` | ❌ W0 |
| 4 | `b.parserName` reachable, equals `"stdlib"` by default (Phase 14 hook) | unit | `go test -run TestBuilderParserNameReachable -x .` | ❌ W0 |
| 5 | Authored fixtures produce byte-identical `Encode()` vs goldens (D-05 #1) | integration | `go test -run TestParserParity_AuthoredFixtures -x .` | ❌ W0 |
| 6 | Gopter determinism: identical inputs → identical bytes (D-05 #2) | property | `go test -run TestParserParity_GopterByteIdentical .` | ❌ W0 |
| 7 | 12-operator Evaluate matrix parity (D-05 #3) | integration | `go test -run TestParserParity_EvaluateMatrix -x .` | ❌ W0 |
| 8 | Transformer-bearing fixture (ISO date + ToLower) byte-identical (D-05 #4) | integration | `go test -run TestParserParity_AuthoredFixtures/transformers_iso_date_and_lower -x .` | ❌ W0 |
| 9 | BUILD-03 int64 fidelity preserved through seam (Pitfall #1 guard) | regression | `go test -run TestNumericIndexPreservesInt64Exactness -x .` | ✅ `gin_test.go:2988` |
| 10 | Existing builder/query/serialize suite green | regression | `make test` | ✅ existing |
| 11 | `BenchmarkAddDocument` within 2% ns/op, 0 extra allocs/op | benchmark | `go test -bench=BenchmarkAddDocument -benchmem -count=10 .` | ✅ `benchmark_test.go:891` |
| 12 | `BenchmarkAddDocumentPhase07` within 2% ns/op, 0 extra allocs/op (int64 hot path) | benchmark | `go test -bench=BenchmarkAddDocumentPhase07 -benchmem -count=10 .` | ✅ `benchmark_test.go:972` |

*Status legend: ⬜ pending · ✅ green · ❌ W0 (needs Wave 0 creation) · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `parser.go` — new file: `Parser` interface + `WithParser` BuilderOption
- [ ] `parser_sink.go` — new file: `parserSink` interface (package-private, 6 methods per D-01)
- [ ] `parser_stdlib.go` — new file: `stdlibParser{}` zero-field struct; `Parse` + streaming walk
- [ ] `parser_parity_test.go` — new test file covering rows 1–8 above (authored fixtures, gopter determinism, 12-operator matrix, transformer parity)
- [ ] `parity_goldens_test.go` — new file gated by `//go:build regenerate_goldens`; writes goldens from v1.0 tag
- [ ] `testdata/parity-golden/` — new directory with 7 authored `.bin` fixtures + `README.md` documenting regeneration procedure

No existing shared fixtures need modification. `generators_test.go` gopter generators reused verbatim (locked by D-05).

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Benchmark delta vs pre-refactor baseline | PARSER-01 / merge gate | Requires stashing a baseline run on the same machine before starting the refactor; numeric drift is hardware-sensitive and cannot be pinned in a unit test | 1) Before any parser work starts: `go test -bench=BenchmarkAddDocument -benchmem -count=10 . > /tmp/phase13-baseline.txt` 2) After final implementation: `go test -bench=BenchmarkAddDocument -benchmem -count=10 . > /tmp/phase13-after.txt` 3) Confirm ns/op delta ≤ 2% and allocs/op delta == 0; paste both outputs into PR description |
| ROADMAP success-criterion #3 deviation flag | PARSER-01 | ROADMAP enumerates `ParserSink` and `stdlibParser` as public surface; Phase 13 ships them package-private per D-02. Needs human confirmation that the deviation is acceptable before close-out | Flag the deviation in PR description and `/gsd-verify-work` — reviewer confirms narrower surface is intentional |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all ❌ W0 references above
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s for quick run
- [ ] `nyquist_compliant: true` set in frontmatter after planner wires per-task verify commands

**Approval:** pending
