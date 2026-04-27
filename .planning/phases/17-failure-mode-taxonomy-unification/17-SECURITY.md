---
phase: 17
slug: failure-mode-taxonomy-unification
status: verified
threats_open: 0
asvs_level: 1
created: 2026-04-23
verified: 2026-04-23
---

# Phase 17 - Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Summary

Phase 17 unifies the public ingest failure-mode API, preserves v9 transformer
wire-token compatibility, routes parser/transformer/numeric soft failures to
whole-document skips before durable mutation, and documents the behavior with a
deterministic example. All 20 plan-scoped threats from plans 17-01 through
17-04 are CLOSED against the current tree. All four execution summaries report
`Threat Flags: None`.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Caller config -> builder ingest routing | Caller-supplied parser/numeric/transformer failure modes decide whether malformed inputs are rejected or skipped. | Public `IngestFailureMode` strings and config literals |
| Public API -> serialized transformer metadata | Public `hard` / `soft` values must not change legacy v9 on-wire transformer tokens. | Transformer metadata in encoded config / representation sections |
| Encoded index bytes -> decoder | Persisted index bytes may contain legacy transformer failure-mode tokens or invalid metadata. | Serialized config JSON and representation metadata |
| Caller JSON bytes -> parser/staging | Untrusted JSON may fail parsing, transformer conversion, or numeric classification. | Caller-controlled document bytes and derived scalar values |
| Staged document state -> durable builder indexes | Soft skips must return before staged observations mutate shared index/bookkeeping state. | Staged paths, numeric stats, row-group membership, doc IDs |
| Internal merge invariant -> tragic builder state | Validator-missed merge panics must stay tragic instead of becoming soft ingest skips. | Panic type and builder error state |
| Public docs/examples -> caller behavior | Changelog/example copy influences whether callers keep hard defaults or opt into soft modes. | Public migration notes and example configuration |
| Fixed example input -> stdout/stderr | Example code processes malformed and invalid documents without leaking sensitive runtime data. | Fixed public sample documents, counts, and row-group output |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| 17-01/T-17-01 | Tampering / Repudiation | `GINConfig`, `DefaultConfig`, `WithParserFailureMode`, `WithNumericFailureMode` | mitigate | Closed: hard defaults are set in [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:722); public validation/normalization is enforced in [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:306), [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:430), and [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:833); struct-literal callers are normalized in [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:169); tests pin the contract in [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:11) and [gin_test.go](/Users/tazarov/experiments/amikos/custom-gin/gin_test.go:266). | closed |
| 17-01/T-17-02 | Tampering | `builder.go` soft-skip routing | transfer | Closed: transfer is documented in [17-01-PLAN.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/17-failure-mode-taxonomy-unification/17-01-PLAN.md:242); whole-document soft skips return before durable merge/bookkeeping in [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:578) and [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:769); tests cover whole-document discard and clean/full-byte equivalence in [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:428), [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:443), and [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:482). | closed |
| 17-01/T-17-03 | Tampering / Denial of Service | `runMergeWithRecover` | transfer | Closed: transfer is documented in [17-01-PLAN.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/17-failure-mode-taxonomy-unification/17-01-PLAN.md:243); recovered merge panics still close the builder in [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:797) and [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:804); the tragic path is pinned by [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:389). | closed |
| 17-01/T-17-04 | Tampering | `TransformerSpec.FailureMode`, serialized config | transfer | Closed: transfer is documented in [17-01-PLAN.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/17-failure-mode-taxonomy-unification/17-01-PLAN.md:244); transformer metadata uses the unified type in [transformer_registry.go](/Users/tazarov/experiments/amikos/custom-gin/transformer_registry.go:47); v9 wire-token projection and decode normalization are enforced in [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:127), [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:1735), [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:1773), and [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:1849); regression tests live in [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:658), [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:687), and [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:746). | closed |
| 17-01/T-17-05 | Information Disclosure | failure-mode handling paths | mitigate | Closed: ordinary soft-failure branches return `errSkipDocument` directly in [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:588), [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:604), [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:638), [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:699), and [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:715); the only runtime logging path in the touched builder logic is tragic merge recovery in [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:804), not soft skips. | closed |
| 17-02/T-17-01 | Tampering / Repudiation | decoded config defaults | transfer | Closed: transfer is documented in [17-02-PLAN.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/17-failure-mode-taxonomy-unification/17-02-PLAN.md:215); hard defaults and validation remain enforced in [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:306), [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:430), [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:722), [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:833), and [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:169); tests remain in [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:11) and [gin_test.go](/Users/tazarov/experiments/amikos/custom-gin/gin_test.go:266). | closed |
| 17-02/T-17-02 | Tampering | partially indexed rejected documents | transfer | Closed: transfer is documented in [17-02-PLAN.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/17-failure-mode-taxonomy-unification/17-02-PLAN.md:216); whole-document skip behavior is implemented in [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:578) and [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:769), with coverage in [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:428), [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:443), and [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:482). | closed |
| 17-02/T-17-03 | Tampering / Denial of Service | recovered merge panic handling | transfer | Closed: transfer is documented in [17-02-PLAN.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/17-failure-mode-taxonomy-unification/17-02-PLAN.md:217); merge panic recovery remains tragic in [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:797) and [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:804), with regression coverage in [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:389). | closed |
| 17-02/T-17-04 | Tampering | `writeConfig`, `writeRepresentations`, `readConfig`, `readRepresentations` | mitigate | Closed: write-side projection preserves legacy `strict` / `soft_fail` tokens in [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:127) and [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:1773); decode normalizes and validates legacy tokens in [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:1735) and [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:1849); the binary version remains v9 in [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:31); regression tests are in [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:658), [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:687), [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:727), and [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:746). | closed |
| 17-02/T-17-05 | Information Disclosure | serialized metadata tests | mitigate | Closed: serialization fixtures use fixed public sample values in [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:45); tests assert exact enum-token behavior in [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:658), [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:687), and [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:746); builder-only parser/numeric knobs are explicitly absent from encoded JSON in [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:803). | closed |
| 17-03/T-17-01 | Tampering / Repudiation | parser/numeric/transformer defaults | transfer | Closed: transfer is documented in [17-03-PLAN.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/17-failure-mode-taxonomy-unification/17-03-PLAN.md:291); hard defaults and invalid-value rejection remain enforced in [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:306), [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:430), [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:722), [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:169), and [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:11). | closed |
| 17-03/T-17-02 | Tampering | `AddDocument`, staging methods, `commitStagedPaths` | mitigate | Closed: parser, transformer, and numeric soft-failure paths short-circuit before durable mutation in [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:578), [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:601), [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:635), [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:769), and [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:790); dense packing / no partial indexing are verified in [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:173), [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:338), [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:428), [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:443), [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:482), and [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:508). | closed |
| 17-03/T-17-03 | Tampering / Denial of Service | `runMergeWithRecover` | mitigate | Closed: parser sink callbacks tag stage errors so parser-soft cannot swallow hard staging failures in [parser_sink.go](/Users/tazarov/experiments/amikos/custom-gin/parser_sink.go:42), [parser_sink.go](/Users/tazarov/experiments/amikos/custom-gin/parser_sink.go:46), and [parser_sink.go](/Users/tazarov/experiments/amikos/custom-gin/parser_sink.go:54); recovered merge panics still set tragic state in [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:797) and [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:804); tests cover both hard-stage and tragic-merge behavior in [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:186), [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:221), and [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:389). | closed |
| 17-03/T-17-04 | Tampering | serialized transformer metadata | transfer | Closed: transfer is documented in [17-03-PLAN.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/17-failure-mode-taxonomy-unification/17-03-PLAN.md:294); v9 wire compatibility is maintained in [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:127), [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:1735), [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:1773), and [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:1849), with coverage in [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:658), [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:687), and [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:746). | closed |
| 17-03/T-17-05 | Information Disclosure | soft failure handling | mitigate | Closed: ordinary soft skips do not log or emit telemetry; they return `errSkipDocument` directly in [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:588), [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:604), [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:638), [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:699), [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:715), and [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:769); [17-03-SUMMARY.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/17-failure-mode-taxonomy-unification/17-03-SUMMARY.md:133) reports `Threat Flags: None`. | closed |
| 17-04/T-17-01 | Tampering / Repudiation | `CHANGELOG.md`, `examples/failure-modes/main.go` | mitigate | Closed: the changelog documents the breaking rename and keeps hard mode as the default public posture in [CHANGELOG.md](/Users/tazarov/experiments/amikos/custom-gin/CHANGELOG.md:5); the example requires explicit soft options in [main.go](/Users/tazarov/experiments/amikos/custom-gin/examples/failure-modes/main.go:46) and [main.go](/Users/tazarov/experiments/amikos/custom-gin/examples/failure-modes/main.go:75); exact output is guarded in [main_test.go](/Users/tazarov/experiments/amikos/custom-gin/examples/failure-modes/main_test.go:9). | closed |
| 17-04/T-17-02 | Tampering | example soft output | mitigate | Closed: the example finalizes only two documents and prints dense row groups `[0 1]` in [main.go](/Users/tazarov/experiments/amikos/custom-gin/examples/failure-modes/main.go:99), [main.go](/Users/tazarov/experiments/amikos/custom-gin/examples/failure-modes/main.go:106), and [main.go](/Users/tazarov/experiments/amikos/custom-gin/examples/failure-modes/main.go:110); exact stdout is asserted in [main_test.go](/Users/tazarov/experiments/amikos/custom-gin/examples/failure-modes/main_test.go:25). | closed |
| 17-04/T-17-03 | Tampering / Denial of Service | example scope | transfer | Closed: transfer is documented in [17-04-PLAN.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/17-failure-mode-taxonomy-unification/17-04-PLAN.md:215); tragic merge recovery remains implemented in [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:797) and [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:804), with regression coverage in [failure_modes_test.go](/Users/tazarov/experiments/amikos/custom-gin/failure_modes_test.go:389). | closed |
| 17-04/T-17-04 | Tampering | docs/example serialization claims | mitigate | Closed: public docs/example copy stays focused on the API rename and ingest behavior in [CHANGELOG.md](/Users/tazarov/experiments/amikos/custom-gin/CHANGELOG.md:5) and [main.go](/Users/tazarov/experiments/amikos/custom-gin/examples/failure-modes/main.go:75); wire-format invariants remain separately enforced by [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:127), [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:746), and [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:803). | closed |
| 17-04/T-17-05 | Information Disclosure | `CHANGELOG.md`, example stdout/stderr | mitigate | Closed: the example uses fixed public sample documents in [main.go](/Users/tazarov/experiments/amikos/custom-gin/examples/failure-modes/main.go:18); stdout prints counts/results only in [main.go](/Users/tazarov/experiments/amikos/custom-gin/examples/failure-modes/main.go:66) and [main.go](/Users/tazarov/experiments/amikos/custom-gin/examples/failure-modes/main.go:110); [main_test.go](/Users/tazarov/experiments/amikos/custom-gin/examples/failure-modes/main_test.go:21) asserts stderr is empty. | closed |

*Status: open / closed*
*Disposition: mitigate (implementation required) / transfer (handled by another verified plan or control)*

---

## Accepted Risks Log

No accepted risks.

---

## Unregistered Flags

None. The phase summaries all report `Threat Flags: None` in
[17-01-SUMMARY.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/17-failure-mode-taxonomy-unification/17-01-SUMMARY.md:115),
[17-02-SUMMARY.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/17-failure-mode-taxonomy-unification/17-02-SUMMARY.md:124),
[17-03-SUMMARY.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/17-failure-mode-taxonomy-unification/17-03-SUMMARY.md:133),
and
[17-04-SUMMARY.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/17-failure-mode-taxonomy-unification/17-04-SUMMARY.md:135).

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-04-23 | 20 | 20 | 0 | Codex |

### Audit Commands

- `go test ./... -run 'Test(IngestFailureModeDefaultsAndValidation|ValidateIngestFailureModeRejectsLegacyTokens|RepresentationFailureModeRoundTrip|DecodeLegacyTransformerFailureModeTokens|ReadConfigRejectsUnknownTransformerFailureMode|TransformerFailureModeWireTokensStayV9|ParserFailureMode|TransformerFailureMode|NumericFailureMode|BuilderSoftFailSkipsDocumentWhenConfigured|TransformerFailureModeSoftDiscardsPartiallyStagedDocument|SoftSkippedDocIDCanBeRetriedWithoutPositionConsumption|SoftFailureModesMatchCleanCorpus|AllSoftFailureModesSilentlyDropFailures|AddDocumentPublicFailuresDoNotSetTragicErr|AddDocumentRefusesAfterRecoveredMergePanic)$' -count=1` - PASS
- `go test ./examples/failure-modes -run 'TestFailureModesExampleOutput$' -count=1` - PASS
- `go run ./examples/failure-modes/main.go` - PASS
- `! rg -n '\bTransformerFailureStrict\b|\bTransformerFailureSoft\b|\btype TransformerFailureMode\b' --glob '*.go'` - PASS, no output
- `! sh -c "rg -n '\bTransformerFailureMode\b' --glob '*.go' | rg -v '\bWithTransformerFailureMode\b'"` - PASS, no output
- `rg -n 'const Version = 9|Version\s*=\s*9' gin.go && ! rg -n 'ParserFailureMode|NumericFailureMode' serialize.go` - PASS

---

## Security Audit 2026-04-23

| Metric | Count |
|--------|-------|
| Threats found | 20 |
| Closed | 20 |
| Open | 0 |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-04-23
