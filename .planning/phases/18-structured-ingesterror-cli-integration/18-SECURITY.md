---
phase: 18
slug: structured-ingesterror-cli-integration
status: verified
threats_open: 0
asvs_level: phase-local
created: 2026-04-24
---

# Phase 18 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Caller document bytes -> public error API | Untrusted document content may be copied verbatim into `IngestError.Value`. | Raw document bytes and derived offending values; untrusted caller data |
| Parser/sink callback -> builder classification | Stage callback errors must preserve parser, transformer, numeric, or schema provenance before parser wrapping. | Internal stage errors and classification metadata |
| Future code changes -> ingest error API | New hard ingest returns could bypass `IngestError` if guard coverage drifts. | Source changes affecting hard failure return paths |
| JSONL input -> CLI report | Untrusted line values may be copied verbatim into text and JSON samples. | Untrusted CLI input echoed in diagnostics |
| CLI failure accounting -> row-group positions | Failed lines must not consume accepted-document row-group positions. | Accepted document counts, row-group allocation, and failure metadata |
| Public docs -> caller logging/redaction choices | Callers need clear warning that values are verbatim and caller-owned for redaction/output policy. | API docs, changelog text, and operator expectations |
| Validation artifact -> phase completion claim | Completion depends on recorded command evidence matching the delivered controls. | Validation commands, outputs, and release-facing evidence |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T18-01 | Information Disclosure | `IngestError.Value` | accept | `ingest_error.go` and `CHANGELOG.md` explicitly state values are verbatim, not redacted, not truncated, and caller-owned. Accepted risk logged below. | closed |
| T18-02 | Tampering / Repudiation | ingest failure classification | mitigate | `builder.go` preserves stage-callback provenance before parser wrapping; `failure_modes_test.go` asserts layer-specific `errors.As` extraction through outer wraps. | closed |
| T18-03 | Denial of Service | large error values | accept | Library preserves verbatim values; CLI bounds report growth by limiting stored samples per layer. Accepted risk logged below. | closed |
| T18-04 | Elevation of Privilege | parser contract errors | mitigate | Parser contract failures remain hard non-`IngestError` returns, with explicit coverage in `failure_modes_test.go`. | closed |
| T18-05 | Tampering | future hard ingest sites | mitigate | `ingest_error_guard_test.go` enforces a focused AST guard against direct plain error returns in named hard-ingest functions. | closed |
| T18-06 | Denial of Service | enforcement false positives | mitigate | Guard scope is limited to known hard-ingest functions and stdlib AST parsing, avoiding brittle grep or shell-specific enforcement. | closed |
| T18-07 | Repudiation | documented exceptions | mitigate | Parser contract, tragic, recovered panic, and soft-mode exception paths are explicit tests in `failure_modes_test.go`. | closed |
| T18-08 | Information Disclosure | CLI failure samples | accept | `cmd/gin-index/experiment.go` keeps values verbatim by design while capping stored samples to 3 per layer. Accepted risk logged below. | closed |
| T18-09 | Tampering | accepted document positions | mitigate | CLI tests confirm failed lines use `input_index` metadata while row groups derive only from accepted documents. | closed |
| T18-10 | Denial of Service | large failure streams | mitigate | `experimentFailureSampleLimit` bounds retained samples per layer while counts continue to accumulate. | closed |
| T18-11 | Repudiation | JSON/text ordering | mitigate | CLI groups are ordered deterministically and verified by experiment tests covering both ordering and sample structure. | closed |
| T18-12 | Information Disclosure | docs for `IngestError.Value` | mitigate | `ingest_error.go` Godoc and `CHANGELOG.md` state verbatim, non-redacted, non-truncated value handling and caller-owned policy. | closed |
| T18-13 | Repudiation | release notes and validation evidence | mitigate | `18-VALIDATION.md` records focused and full verification commands that support the completion claim. | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-18-01 | T18-01 | Phase 18 intentionally exposes `IngestError.Value` verbatim to preserve caller-visible ingest diagnostics. Redaction remains the caller's responsibility. | `gsd-secure-phase` audit | 2026-04-24 |
| AR-18-02 | T18-03 | No byte cap is applied in the library because truncation would violate the locked verbatim-value contract; callers own downstream output policy. | `gsd-secure-phase` audit | 2026-04-24 |
| AR-18-03 | T18-08 | CLI continue-mode samples retain verbatim values by design; risk is bounded by limiting retained samples to 3 per layer instead of truncating values. | `gsd-secure-phase` audit | 2026-04-24 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-04-24 | 13 | 13 | 0 | Codex + `gsd-security-auditor` |

### Verification Evidence

- `go test ./... -run 'Test(IngestErrorWrappingContract|HardIngestFailuresReturnIngestError|ParserContractErrorsRemainNonIngestError|SoftFailureModesDoNotReturnIngestError|HardIngestFunctionsDoNotReturnPlainErrors)$' -count=1` - PASS
- `go test ./cmd/gin-index -run 'TestRunExperiment(OnErrorContinue|OnErrorContinueMalformedJSONFromFile|OnErrorContinueIngestFailuresJSON|HundredDocsKnownIngestFailuresJSON|OnErrorAbort.*Ingest)' -count=1` - PASS
- `make lint` - PASS
- Auditor result: `## SECURED` with threat-by-threat evidence across builder, CLI, docs, and validation artifacts.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-04-24
