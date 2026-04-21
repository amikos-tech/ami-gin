---
phase: 13
slug: parser-seam-extraction
status: verified
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-21
updated: 2026-04-21T12:54:35Z
---

# Phase 13 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Source of record: `13-01-PLAN.md`, `13-02-PLAN.md`, `13-03-PLAN.md`, and the current-tree audit evidence below.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing`, `github.com/leanovate/gopter`, Go benchmark tooling, and `.planning/` artifact inspection |
| **Config file** | `go.mod` / `Makefile` (no extra framework config) |
| **Quick run command** | `go test -run 'Test(WithParserRejectsNil|StdlibParserName|BuilderHasParserFields|ShouldBufferForTransformSignalWhenRegistered|BeginDocumentStashesState|NewBuilderDefaultsToStdlibParser|NewBuilderRejectsEmptyParserName|BuilderParserNameReachable|WithParserAcceptsCustomParser|AddDocumentRoundTripsThroughParser|AddDocumentReturnsParserErrorVerbatim|AddDocumentDefaultParserErrorStringsPreserved|AddDocumentRejectsParserSkippingBeginDocument|AddDocumentRejectsBeginDocumentRGIDMismatch|ParserParity_AuthoredFixtures|ParserSeam_DeterministicAcrossRuns|ParserParity_EvaluateMatrix|NumericIndexPreservesInt64Exactness)' -count=1 .` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | quick run ~2s, benchmark smoke ~4s, full suite ~41s, lint ~1s |

---

## Sampling Rate

- **After every task commit:** Run the task-specific command from the verification map below.
- **After every plan wave:** Run the quick run command above.
- **Before `$gsd-verify-work`:** `go test ./... -count=1`, `make lint`, and the benchmark/manual-review evidence in `13-03-BENCH.md` plus `13-SECURITY.md` must all be current.
- **Max feedback latency:** 45 seconds for repo-local validation.

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 13-01-01 | 01 | 1 | PARSER-01 | T-13-01 / T-13-03 | `Parser`, `WithParser`, `parserSink`, `stdlibParser`, builder seam fields, and the transform-buffer signal compile cleanly while preserving the locked seam contracts before hot-path wiring | unit + compile | `go test -run 'Test(WithParserRejectsNil|StdlibParserName|BuilderHasParserFields|ShouldBufferForTransformSignalWhenRegistered|BeginDocumentStashesState)' -count=1 .` | ✅ `parser.go`, ✅ `parser_sink.go`, ✅ `parser_stdlib.go`, ✅ `parser_test.go` | ✅ green |
| 13-02-01 | 02 | 2 | PARSER-01 | T-13-02 / T-13-07 | `NewBuilder` defaults to `stdlibParser`, caches `parserName`, rejects empty names, and honors custom parser installation | unit | `go test -run 'Test(NewBuilderDefaultsToStdlibParser|NewBuilderRejectsEmptyParserName|BuilderParserNameReachable|WithParserAcceptsCustomParser)' -count=1 .` | ✅ `builder.go`, ✅ `parser_test.go` | ✅ green |
| 13-02-02 | 02 | 2 | PARSER-01 | T-13-06 / T-13-06b / T-13-09 / T-13-10 | `AddDocument` routes through the parser seam, preserves parser error text, enforces the `BeginDocument` contract, and keeps exact-int64 behavior intact | unit + regression | `go test -run 'Test(AddDocumentRoundTripsThroughParser|AddDocumentReturnsParserErrorVerbatim|AddDocumentDefaultParserErrorStringsPreserved|AddDocumentRejectsParserSkippingBeginDocument|AddDocumentRejectsBeginDocumentRGIDMismatch|NumericIndexPreservesInt64Exactness)' -count=1 .` | ✅ `builder.go`, ✅ `parser_test.go`, ✅ `gin_test.go` | ✅ green |
| 13-02-03 | 02 | 2 | PARSER-01 | T-13-08 / T-13-10b | Focused benchmark smoke and parity-artifact infrastructure remain present after wire-up; goldens are committed and regeneration stays build-tagged | benchmark + artifact | `go test -run '^$' -bench 'Benchmark(AddDocument|AddDocumentPhase07)$' -benchmem -benchtime=1x -count=1 . && test -f parity_goldens_test.go && test -f testdata/parity-golden/README.md && test -f testdata/parity-golden/int64-boundaries.bin && test -f testdata/parity-golden/transformers-iso-date-and-lower.bin` | ✅ `benchmark_test.go`, ✅ `parity_goldens_test.go`, ✅ `testdata/parity-golden/*` | ✅ green |
| 13-03-01 | 03 | 3 | PARSER-01 | T-13-11 / T-13-13 / T-13-15 / T-13-17 | Authored golden fixtures, transformer canary, and the determinism canary continuously guard byte-level parity and fail closed on encode/build errors | integration + property | `go test -run 'Test(ParserParity_AuthoredFixtures|ParserSeam_DeterministicAcrossRuns)' -count=1 .` | ✅ `parser_parity_test.go`, ✅ `parser_parity_fixtures_test.go`, ✅ `testdata/parity-golden/*` | ✅ green |
| 13-03-02 | 03 | 3 | PARSER-01 | T-13-18 | The 12-operator Evaluate matrix exercises known paths only and preserves query semantics through the seam | integration | `go test -run 'TestParserParity_EvaluateMatrix' -count=1 .` | ✅ `parser_parity_test.go` | ✅ green |
| 13-03-03 | 03 | 3 | PARSER-01 | T-13-08 / T-13-14 | Repo-wide regressions stay green, lint is clean, and the benchmark-risk acceptance remains documented in the phase artifacts | regression + docs | `go test ./... -count=1 && make lint && test -f .planning/phases/13-parser-seam-extraction/13-03-BENCH.md && test -f .planning/phases/13-parser-seam-extraction/13-SECURITY.md && rg -n 'R-13-04|T-13-14|accepted as residual risk' .planning/phases/13-parser-seam-extraction/13-SECURITY.md >/dev/null` | ✅ `13-03-BENCH.md`, ✅ `13-SECURITY.md`, ✅ `parser_parity_test.go` | ✅ green |

*Status: ⬜ pending - ✅ green - ❌ red - ⚠ accepted/manual review*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements.

- [x] Parser seam source files and package-local seam tests are present.
- [x] Authored parity fixtures, committed goldens, and the build-tagged regenerator are present.
- [x] Benchmark artifacts, security acceptance, and executed phase summaries are present.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Transformer-heavy benchmark delta vs. v1.0 baseline | PARSER-01 / T-13-14 | The wall-clock threshold is machine-sensitive, and Phase 13 explicitly closed this as accepted residual performance risk instead of a universally green benchmark gate | `GOMAXPROCS=1 go test -run '^$' -bench '^BenchmarkAddDocument$|^BenchmarkAddDocumentPhase07/parser=explicit-number/docs=(1000|10000)/shape=(int-only|transformer-heavy)$' -benchmem -count=10 .`, compare against `.planning/phases/13-parser-seam-extraction/13-02-BASELINE.txt`, then review `13-03-BENCH.md` and `13-SECURITY.md` before changing the accepted-risk disposition |
| ROADMAP public-surface deviation confirmation | PARSER-01 / D-02 | Whether `parserSink` and `stdlibParser` remain package-private is an API-scope decision, not a runtime behavior that automated tests can approve | Confirm `parser.go` exports `Parser` and `WithParser`, `parser_sink.go` and `parser_stdlib.go` remain package-private, and keep the deviation note in the phase summary/security trail until roadmap wording is reconciled |

---

## Validation Audit 2026-04-21

| Metric | Count |
|--------|-------|
| Gaps found | 4 |
| Resolved | 4 |
| Escalated | 0 |

Audit evidence:
- Confirmed State A input: existing `13-VALIDATION.md`, all three Phase 13 plan files, and all three Phase 13 summary artifacts were present before the audit began.
- Confirmed Nyquist validation remains enabled because `.planning/config.json` does not set `workflow.nyquist_validation` to `false`.
- Re-read `13-CONTEXT.md`, `13-01-PLAN.md`, `13-02-PLAN.md`, `13-03-PLAN.md`, `13-parser-seam-extraction-01-SUMMARY.md`, `13-parser-seam-extraction-02-SUMMARY.md`, `13-parser-seam-extraction-03-SUMMARY.md`, `13-03-BENCH.md`, and `13-SECURITY.md` to rebuild the current `PARSER-01` requirement-to-task map.
- Cross-referenced the live proof surface in `parser.go`, `parser_sink.go`, `parser_stdlib.go`, `builder.go`, `parser_test.go`, `parser_parity_test.go`, `parser_parity_fixtures_test.go`, `parity_goldens_test.go`, `gin_test.go`, `benchmark_test.go`, and `testdata/parity-golden/*`; no missing product-test files were found.
- Re-ran seam-surface and builder-guard coverage with `go test -run 'Test(WithParserRejectsNil|StdlibParserName|BuilderHasParserFields|ShouldBufferForTransformSignalWhenRegistered|BeginDocumentStashesState|NewBuilderDefaultsToStdlibParser|NewBuilderRejectsEmptyParserName|BuilderParserNameReachable|WithParserAcceptsCustomParser|AddDocumentRoundTripsThroughParser|AddDocumentReturnsParserErrorVerbatim|AddDocumentDefaultParserErrorStringsPreserved|AddDocumentRejectsParserSkippingBeginDocument|AddDocumentRejectsBeginDocumentRGIDMismatch)' -count=1 .`, which passed with `ok github.com/amikos-tech/ami-gin 0.975s`.
- Re-ran parity, determinism, Evaluate-matrix, and exact-int64 coverage with `go test -run 'Test(ParserParity_AuthoredFixtures|ParserSeam_DeterministicAcrossRuns|ParserParity_EvaluateMatrix|NumericIndexPreservesInt64Exactness)' -count=1 .`, which passed with `ok github.com/amikos-tech/ami-gin 1.804s`.
- Re-ran benchmark smoke with `go test -run '^$' -bench 'Benchmark(AddDocument|AddDocumentPhase07)$' -benchmem -benchtime=1x -count=1 .`, which passed in `3.385s`; `BenchmarkAddDocument` reported `82384 B/op` and `734 allocs/op`, and the explicit-number `BenchmarkAddDocumentPhase07` probe surface remained present across `int-only`, `mixed-safe`, `wide-flat`, and `transformer-heavy`.
- Re-ran the repo-wide regression sweep with `go test ./... -count=1`, which passed in `39.134s` for `github.com/amikos-tech/ami-gin` and `1.738s` for `github.com/amikos-tech/ami-gin/cmd/gin-index`.
- Re-ran `make lint`, which returned `golangci-lint run` and `0 issues.`
- Closed four validation-contract gaps without adding new product tests: the stale Wave 0 draft table was replaced with task-scoped verification rows, the quick-run sample now covers parser contract/error-path/parity/evaluate/int64 checks, benchmark-threshold interpretation moved into manual-only accepted-risk review backed by `13-03-BENCH.md` plus `13-SECURITY.md`, and the frontmatter/sign-off state now reflects the executed phase rather than the pre-execution draft.
- No new test files were required for Phase 13. Existing repo-local tests, committed parity goldens, benchmark smoke, full-suite coverage, and lint coverage already satisfy the completed phase.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 45s for repo-local checks
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-04-21
