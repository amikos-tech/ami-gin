# Security Policy

## Reporting a Vulnerability

If you suspect a vulnerability, do not open public issues, public pull requests, or public discussions. Use a private reporting channel so details can be triaged safely.

## Preferred Channel

When GitHub private vulnerability reporting is available for this repository, use that private GitHub flow first.

## Fallback Contact

If GitHub private vulnerability reporting is unavailable or unsuitable, email security@amikos.tech.

## Disclosure Expectations

We aim to acknowledge reports within 5 business days and send follow-up status updates as triage progresses.

## Supported Versions

Security support currently applies to the latest development line on main and, once releases exist, the latest tagged release.

## Accepted Risks Log

### Phase 18 - structured-ingesterror-cli-integration

| Threat ID | Category | Accepted Risk | Rationale |
|-----------|----------|---------------|-----------|
| T18-01 | Information Disclosure | `IngestError.Value()` stores verbatim offending values without redaction or truncation. | Phase 18 intentionally preserves caller-visible verbatim values; callers own redaction policy. |
| T18-03 | Denial of Service | Library-side `IngestError.Value()` remains unbounded. | Phase 18 intentionally avoids a byte cap to preserve verbatim semantics; callers own logging and output-size policy. |
| T18-08 | Information Disclosure | CLI failure samples copy verbatim `IngestError.Value()` into reports. | Phase 18 intentionally keeps values verbatim; CLI growth is bounded by at most 3 samples per layer. |

## Phase Audit Log

### Phase 18 - structured-ingesterror-cli-integration

- asvs_level: phase-local
- block_on: unresolved_phase18_threats
- threats_open: 0
- audit_date: 2026-04-24

#### Threat Verification

| Threat ID | Category | Disposition | Status | Evidence |
|-----------|----------|-------------|--------|----------|
| T18-01 | Information Disclosure | accept | CLOSED | Accepted risk logged in this file. |
| T18-02 | Tampering / Repudiation | mitigate | CLOSED | `builder.go:411`, `builder.go:425`, and `builder.go:432` preserve stage-callback provenance before parser wrapping; `failure_modes_test.go:60` and `failure_modes_test.go:102` assert per-layer `errors.As` extraction; `failure_modes_test.go:624` covers cross-document provenance reset. |
| T18-03 | Denial of Service | accept | CLOSED | Accepted risk logged in this file. |
| T18-04 | Elevation of Privilege | mitigate | CLOSED | `isParserLifecycleError` keeps parser contract checks outside `IngestError`; `TestParserContractErrorsRemainNonIngestError` verifies contract failures stay non-`IngestError`; `TestParserFailureModeSoftKeepsContractViolationsHard` proves parser soft mode does not swallow contract errors. |
| T18-05 | Tampering | mitigate | CLOSED | `TestHardIngestFailuresReturnIngestError` provides the hard-ingest behavior matrix; `TestHardIngestFunctionsDoNotReturnPlainErrors` executes the guard and `hardIngestFunctions` defines the auto-discovered hard-ingest surface set. |
| T18-06 | Denial of Service | mitigate | CLOSED | `getOrCreateStagedPath` enforces `MaxStagedPaths` against every staged path, including companion representations; `TestMaxStagedPaths` covers container-derived and deep-array rejection. `ingest_error_guard_test.go` keeps this hard-ingest boundary in the automated error-surface audit. The bound caps distinct staged paths, not peak memory: transformer output is fully materialized before rejection, so callers needing a hard memory ceiling must bound input size upstream. |
| T18-07 | Repudiation | mitigate | CLOSED | `TestParserContractErrorsRemainNonIngestError`, `TestSoftFailureModesDoNotReturnIngestError`, `TestParserFailureModeSoftKeepsTragicStateHard`, and `TestNumericFailureModeSoftKeepsMergeRecoveryTragic` keep parser-contract, soft-mode, tragic-state, and recovered-panic exceptions explicit in tests. |
| T18-08 | Information Disclosure | accept | CLOSED | Accepted risk logged in this file. |
| T18-09 | Tampering | mitigate | CLOSED | `recordExperimentIngestFailure` in `cmd/gin-index/experiment.go` stores failed-line `input_index` separately from accepted docs; `TestRunExperimentOnErrorContinueIngestFailuresJSON` checks parser sample line/input-index, and `TestRunExperimentHundredDocsKnownIngestFailuresJSON` asserts 87 accepted docs collapse to 9 row groups. |
| T18-10 | Denial of Service | mitigate | CLOSED | `recordExperimentIngestFailure` in `cmd/gin-index/experiment.go` increments failure counts before enforcing the sample cap; `TestRecordExperimentIngestFailureCapsSamplesInArrivalOrder` verifies counts accumulate while samples stay capped. |
| T18-11 | Repudiation | mitigate | CLOSED | `experimentIngestFailureGroups` and `experimentIngestLayerRank` pin parser/transformer/numeric/schema/resource/unknown ordering; `TestExperimentIngestFailureGroupsDeterministic` and `TestRunExperimentMaxStagedPathsReportsResourceFailureWithoutValue` assert stable order and resource grouping. |
| T18-12 | Information Disclosure | mitigate | CLOSED | `IngestError.Value` documents verbatim, caller-owned document-data values and empty resource values; see the IngestError type doc (ingest_error.go:31-45) for where resource diagnostics live. The Unreleased changelog repeats the release-facing contract. |
| T18-13 | Repudiation | mitigate | CLOSED | `.planning/phases/18-structured-ingesterror-cli-integration/18-VALIDATION.md:22`, `:23`, `:81`, and `:84` record focused and full verification commands plus execution results. |

#### Threat Flags

- `18-03-SUMMARY.md` reports one information-disclosure flag for verbatim CLI samples; it maps to accepted threat `T18-08`, so there are no unregistered flags.

#### Auditor Recheck

- `go test ./... -run 'Test(IngestErrorWrappingContract|HardIngestFailuresReturnIngestError|ParserContractErrorsRemainNonIngestError|SoftFailureModesDoNotReturnIngestError|HardIngestFunctionsDoNotReturnPlainErrors)$' -count=1` - PASS
- `go test ./cmd/gin-index -run 'TestRunExperiment(OnErrorContinue|OnErrorContinueMalformedJSONFromFile|OnErrorContinueIngestFailuresJSON|HundredDocsKnownIngestFailuresJSON|OnErrorAbort.*Ingest)' -count=1` - PASS
- `make lint` - PASS
