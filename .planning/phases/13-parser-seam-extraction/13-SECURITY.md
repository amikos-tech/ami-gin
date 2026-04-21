---
phase: 13
slug: parser-seam-extraction
status: verified
threats_open: 0
asvs_level: 1
created: 2026-04-21
---

# Phase 13 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| consumer -> library API | Untrusted JSON document bytes cross into `AddDocument`, which now routes through the parser seam while preserving the `encoding/json.Decoder.UseNumber()` validation path. | JSON document bytes, parser errors, row-group ids |
| custom parser -> builder internals | Same-package parser implementations can drive the sink contract and therefore influence staged document state before merge. | `parserSink` calls, `BeginDocument` state, staged observations |
| test harness -> filesystem | The parity harness reads committed golden blobs, while regeneration is isolated behind a build tag. | `testdata/parity-golden/*.bin`, test-generated fixtures |
| test input -> encode/query path | Authored fixtures and gopter-generated docs exercise the real build, encode, and query pipeline. | JSON fixtures, encoded index bytes, predicate results |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-13-01 | Tampering | `WithParser(nil)` error contract | mitigate | Closed: [parser.go](../../../parser.go:34) rejects nil with the exact `"parser cannot be nil"` string, and [parser_test.go](../../../parser_test.go:10) pins that contract. | closed |
| T-13-02 | Information Disclosure | `Parser.Name()` telemetry value | mitigate | Closed: [builder.go](../../../builder.go:150) defaults the parser and rejects empty names, [parser_stdlib.go](../../../parser_stdlib.go:11) fixes the default identifier to `"stdlib"`, and [parser_test.go](../../../parser_test.go:22) plus [parser_test.go](../../../parser_test.go:100) verify both the default and the empty-name guard. | closed |
| T-13-03 | Tampering | `parserSink` surface widening | mitigate | Closed: [parser.go](../../../parser.go:17) documents the package-private sink boundary, [parser_sink.go](../../../parser_sink.go:3) keeps the interface unexported with the locked 6-method set, and [parser_sink.go](../../../parser_sink.go:48) asserts `*GINBuilder` satisfies it. | closed |
| T-13-04 | Denial of Service | recursive descent in `stdlibParser` | accept | Accepted: [parser_stdlib.go](../../../parser_stdlib.go:58) and [parser_stdlib.go](../../../parser_stdlib.go:138) preserve the pre-phase recursive traversal unchanged. Phase 13 carried this risk forward as a code-move-only posture rather than adding new recursion controls. | closed |
| T-13-05 | Tampering | int64 classifier moving into parser | mitigate | Closed: [parser_stdlib.go](../../../parser_stdlib.go:138) stages through the sink instead of parsing numeric classes itself, [parser_sink.go](../../../parser_sink.go:32) forwards JSON-number and native-numeric staging back into builder-owned logic, and current-tree `TestNumericIndexPreservesInt64Exactness` passed on 2026-04-21. | closed |
| T-13-06 | Tampering | parser error wrapping at `AddDocument` | mitigate | Closed: [builder.go](../../../builder.go:317) returns parser errors verbatim, while [parser_test.go](../../../parser_test.go:160) and [parser_test.go](../../../parser_test.go:175) pin both custom-parser and default-parser error text behavior. | closed |
| T-13-06b | Tampering | parser skips or misuses `BeginDocument` | mitigate | Closed: [builder.go](../../../builder.go:334) resets stale state and enforces both post-parse guards, and [parser_test.go](../../../parser_test.go:197) plus [parser_test.go](../../../parser_test.go:220) pin the missing-`BeginDocument` and wrong-rgID failures. | closed |
| T-13-07 | Information Disclosure | empty `parserName` cache | mitigate | Closed: [builder.go](../../../builder.go:153) rejects empty names before caching, and [parser_test.go](../../../parser_test.go:100) verifies the exact rejection string. | closed |
| T-13-08 | Denial of Service | seam-path allocation/perf regression at wire-up | mitigate | Closed: the wire-up benchmark artifact stayed within gate in [13-02-BENCH.md](./13-02-BENCH.md:37), with flat allocs/op across all focused benchmark families, and current reruns on 2026-04-21 still showed no allocation growth (`734`, `207`, `852` allocs/op). | closed |
| T-13-09 | Tampering | BUILD-03 int64 fidelity through the seam | mitigate | Closed: current-tree `TestNumericIndexPreservesInt64Exactness` passed on 2026-04-21, and the authored-golden parity harness passed the `int64-boundaries` fixture in [parser_parity_test.go](../../../parser_parity_test.go:58). | closed |
| T-13-10 | Repudiation | parser error-message drift | mitigate | Closed: [parser_stdlib.go](../../../parser_stdlib.go:68) preserves the `"read JSON token"` family of errors, [builder.go](../../../builder.go:317) does not wrap parser errors, and [parser_test.go](../../../parser_test.go:175) verifies the default malformed-JSON message still contains the expected text. | closed |
| T-13-10b | Tampering | parity golden drift during post-wire-up capture | mitigate | Closed: [13-02-BENCH.md](./13-02-BENCH.md:45) recorded the post-wire-up benchmark gate as within tolerance before the goldens were captured, [parity_goldens_test.go](../../../parity_goldens_test.go:1) isolates regeneration behind `regenerate_goldens`, and [parser_parity_test.go](../../../parser_parity_test.go:58) now continuously asserts authored-golden parity. | closed |
| T-13-11 | Tampering | accidental goldens rewrite | mitigate | Closed: [parity_goldens_test.go](../../../parity_goldens_test.go:1) build-tags the writer path, while [parser_parity_test.go](../../../parser_parity_test.go:14) only reads the committed blobs during normal test runs. | closed |
| T-13-12 | Information Disclosure | committed test fixtures | accept | Accepted: the phase threat model classified the fixtures as contrived and non-sensitive; [parser_parity_test.go](../../../parser_parity_test.go:157) uses synthetic fixture data only, and the committed golden files are encoded index bytes rather than secrets-bearing source payloads. | closed |
| T-13-13 | Tampering | int64 drift despite parity green | mitigate | Closed: [parser_parity_test.go](../../../parser_parity_test.go:58) continuously verifies the `int64-boundaries` golden, and current-tree `TestNumericIndexPreservesInt64Exactness` passed on 2026-04-21 as defense in depth. | closed |
| T-13-14 | Denial of Service | performance regression escaping the parity harness | accept | Accepted: the merge-gate benchmark artifact in [13-03-BENCH.md](./13-03-BENCH.md:10) still records `FAIL` on both transformer-heavy focused subbenchmarks, but allocs/op stayed flat and correctness/parity gates remained green. Per explicit user direction on 2026-04-21, this benchmark noise is accepted as residual performance risk rather than treated as unresolved mitigation debt. The benchmark evidence remains preserved for future follow-up. | closed |
| T-13-15 | Tampering | transformer buffering hook misroute | mitigate | Closed: [parser_stdlib.go](../../../parser_stdlib.go:58) and [parser_stdlib.go](../../../parser_stdlib.go:139) check `ShouldBufferForTransform` before token reads, and the transformer canary fixture passed in [parser_parity_test.go](../../../parser_parity_test.go:58). | closed |
| T-13-16 | Information Disclosure | gopter shrinker output | accept | Accepted: the generators operate on synthetic `TestDoc` JSON only, and the shrinker output remains local test output rather than product telemetry or user-facing logs, as exercised in [parser_parity_test.go](../../../parser_parity_test.go:69). | closed |
| T-13-17 | Tampering | gopter determinism property masking encode failure | mitigate | Closed: [parser_parity_test.go](../../../parser_parity_test.go:79) and [parser_parity_test.go](../../../parser_parity_test.go:131) return explicit errors from `encodeDocs` and fail the property on any encode/build error instead of allowing `bytes.Equal(nil, nil)` to mask the failure. | closed |
| T-13-18 | Tampering | Evaluate-matrix prune cases falling into `AllRGs` fallback | mitigate | Closed: [parser_parity_test.go](../../../parser_parity_test.go:152) documents the unknown-path trap, and [parser_parity_test.go](../../../parser_parity_test.go:199) exercises 24 known-path matrix cases without relying on nonexistent-path pruning. | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| R-13-01 | T-13-04 | `stdlibParser` keeps the pre-phase recursive traversal unchanged; this phase was scoped as a behavior-neutral code move, not a recursion-hardening change. | Phase 13 plan threat model | 2026-04-21 |
| R-13-02 | T-13-12 | The committed fixtures and goldens are synthetic and contain no production or sensitive data. | Phase 13 plan threat model | 2026-04-21 |
| R-13-03 | T-13-16 | Gopter shrink output is local-only and derived from synthetic generator data, so no sensitive disclosure surface was introduced. | Phase 13 plan threat model | 2026-04-21 |
| R-13-04 | T-13-14 | Transformer-heavy explicit-number benchmarks remained slightly above the `+2%` wall-clock target on this machine, but allocs/op stayed flat and all correctness/parity gates passed. User explicitly accepted this benchmark noise as residual risk. | User via Codex | 2026-04-21 |

---

## Verification Evidence

- Threat sources audited from [13-01-PLAN.md](./13-01-PLAN.md:740), [13-02-PLAN.md](./13-02-PLAN.md:954), and [13-03-PLAN.md](./13-03-PLAN.md:741).
- No `## Threat Flags` carry-forward sections are present in [13-parser-seam-extraction-01-SUMMARY.md](./13-parser-seam-extraction-01-SUMMARY.md:1), [13-parser-seam-extraction-02-SUMMARY.md](./13-parser-seam-extraction-02-SUMMARY.md:1), or [13-parser-seam-extraction-03-SUMMARY.md](./13-parser-seam-extraction-03-SUMMARY.md:1).
- Current-tree targeted re-audit passed on 2026-04-21 with `go test -run 'TestWithParserRejectsNil|TestStdlibParserName|TestBuilderHasParserFields|TestShouldBufferForTransformSignalWhenRegistered|TestBeginDocumentStashesState|TestNewBuilderDefaultsToStdlibParser|TestNewBuilderRejectsEmptyParserName|TestBuilderParserNameReachable|TestWithParserAcceptsCustomParser|TestAddDocumentRoundTripsThroughParser|TestAddDocumentReturnsParserErrorVerbatim|TestAddDocumentDefaultParserErrorStringsPreserved|TestAddDocumentRejectsParserSkippingBeginDocument|TestAddDocumentRejectsBeginDocumentRGIDMismatch|TestParserParity_AuthoredFixtures|TestParserSeam_DeterministicAcrossRuns|TestParserParity_EvaluateMatrix|TestNumericIndexPreservesInt64Exactness' -count=1 -v .`, returning `ok github.com/amikos-tech/ami-gin 1.966s`.
- Current-tree full-suite re-audit passed on 2026-04-21 with `go test ./... -count=1`, returning `ok github.com/amikos-tech/ami-gin 37.868s` and `ok github.com/amikos-tech/ami-gin/cmd/gin-index 0.593s`.
- Current-tree lint re-audit passed on 2026-04-21 with `make lint`, returning `golangci-lint run` and `0 issues.`.
- The committed merge-gate artifact captures the benchmark risk being accepted here: [13-03-BENCH.md](./13-03-BENCH.md:12) through [13-03-BENCH.md](./13-03-BENCH.md:16) show the transformer-heavy failures, and [13-parser-seam-extraction-03-SUMMARY.md](./13-parser-seam-extraction-03-SUMMARY.md:77) through [13-parser-seam-extraction-03-SUMMARY.md](./13-parser-seam-extraction-03-SUMMARY.md:79) record the original review hold that this security step is now accepting as residual risk.
- Current-tree benchmark re-audit reran `GOMAXPROCS=1 go test -bench='^BenchmarkAddDocument$|^BenchmarkAddDocumentPhase07/parser=explicit-number/docs=(1000|10000)/shape=(int-only|transformer-heavy)$' -benchmem -count=10 -run=^$ .` and `GOMAXPROCS=1 go test -bench='^BenchmarkAddDocumentPhase07/parser=explicit-number/docs=(1000|10000)/shape=(int-only|transformer-heavy)$' -benchmem -count=10 -run=^$ .` on 2026-04-21. Both reruns preserved flat allocs/op (`734`, `207`, `852`) but did not cleanly clear the wall-clock threshold across the explicit-number benchmark family; the user accepted that remaining drift as residual risk.
- Phase tracking is still intentionally blocked at the roadmap/state layer: [STATE.md](../../STATE.md:15) records `Phase: 13 (parser-seam-extraction) — EXECUTING`, and [ROADMAP.md](../../ROADMAP.md:47) still leaves `13-03-PLAN.md` unchecked.

## Security Audit 2026-04-21

| Metric | Count |
|--------|-------|
| Threats found | 20 |
| Closed | 20 |
| Open | 0 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-04-21 | 20 | 19 | 1 | Codex `gsd-secure-phase` |
| 2026-04-21 | 20 | 20 | 0 | Codex `gsd-secure-phase` benchmark-risk acceptance |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-04-21
