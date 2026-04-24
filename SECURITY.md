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
| T18-01 | Information Disclosure | `IngestError.Value` stores verbatim offending values without redaction or truncation. | Phase 18 intentionally preserves caller-visible verbatim values; callers own redaction policy. |
| T18-03 | Denial of Service | Library-side `IngestError.Value` remains unbounded. | Phase 18 intentionally avoids a byte cap to preserve verbatim semantics; callers own logging and output-size policy. |
| T18-08 | Information Disclosure | CLI failure samples copy verbatim `IngestError.Value` into reports. | Phase 18 intentionally keeps values verbatim; CLI growth is bounded by at most 3 samples per layer. |

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
| T18-04 | Elevation of Privilege | mitigate | CLOSED | `builder.go:435` keeps parser contract checks outside `IngestError`; `failure_modes_test.go:184` verifies contract failures stay non-`IngestError`; `failure_modes_test.go:547` proves parser soft mode does not swallow contract errors. |
| T18-05 | Tampering | mitigate | CLOSED | `failure_modes_test.go:102` provides the hard-ingest behavior matrix; `ingest_error_guard_test.go:10` guards named hard-ingest functions against direct plain `errors.*` returns. |
| T18-06 | Denial of Service | mitigate | CLOSED | `ingest_error_guard_test.go:11`, `ingest_error_guard_test.go:143`, `ingest_error_guard_test.go:181`, and `ingest_error_guard_test.go:208` auto-discover hard-ingest surfaces, track tainted plain-error assignments, and document the `stage*`/`+hard-ingest` scope. |
| T18-07 | Repudiation | mitigate | CLOSED | `failure_modes_test.go:184`, `failure_modes_test.go:206`, `failure_modes_test.go:547`, `failure_modes_test.go:604`, and `failure_modes_test.go:782` keep parser-contract, soft-mode, tragic-state, and recovered-panic exceptions explicit in tests. |
| T18-08 | Information Disclosure | accept | CLOSED | Accepted risk logged in this file. |
| T18-09 | Tampering | mitigate | CLOSED | `cmd/gin-index/experiment.go:486` stores failed-line `input_index` separately from accepted docs; `cmd/gin-index/experiment_test.go:789` checks parser sample line/input-index; `cmd/gin-index/experiment_test.go:821` and `cmd/gin-index/experiment_test.go:827` assert 87 accepted docs collapse to 9 row groups. |
| T18-10 | Denial of Service | mitigate | CLOSED | `cmd/gin-index/experiment.go:476` increments failure counts before `cmd/gin-index/experiment.go:477` enforces the 3-sample cap; `cmd/gin-index/experiment_test.go:680` and `cmd/gin-index/experiment_test.go:744` verify counts accumulate while samples stay capped. |
| T18-11 | Repudiation | mitigate | CLOSED | `cmd/gin-index/experiment.go:524` sorts failure groups deterministically and `cmd/gin-index/experiment.go:563` plus `cmd/gin-index/experiment.go:565` pin parser/transformer/numeric/schema/unknown ordering; `cmd/gin-index/experiment_test.go:279`, `cmd/gin-index/experiment_test.go:774`, and `cmd/gin-index/experiment_test.go:844` assert stable order. |
| T18-12 | Information Disclosure | mitigate | CLOSED | `ingest_error.go:33` documents that `Value()` is verbatim and caller-owned for redaction/output policy, and `ingest_error.go:68` makes the public `Error()` string contract explicit; `CHANGELOG.md:5` repeats the release-facing warning. |
| T18-13 | Repudiation | mitigate | CLOSED | `.planning/phases/18-structured-ingesterror-cli-integration/18-VALIDATION.md:22`, `:23`, `:81`, and `:84` record focused and full verification commands plus execution results. |

#### Threat Flags

- `18-03-SUMMARY.md` reports one information-disclosure flag for verbatim CLI samples; it maps to accepted threat `T18-08`, so there are no unregistered flags.

#### Auditor Recheck

- `go test ./... -run 'Test(IngestErrorWrappingContract|HardIngestFailuresReturnIngestError|ParserContractErrorsRemainNonIngestError|SoftFailureModesDoNotReturnIngestError|HardIngestFunctionsDoNotReturnPlainErrors)$' -count=1` - PASS
- `go test ./cmd/gin-index -run 'TestRunExperiment(OnErrorContinue|OnErrorContinueMalformedJSONFromFile|OnErrorContinueIngestFailuresJSON|HundredDocsKnownIngestFailuresJSON|OnErrorAbort.*Ingest)' -count=1` - PASS
- `make lint` - PASS
