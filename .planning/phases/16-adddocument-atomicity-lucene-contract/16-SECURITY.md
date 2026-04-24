---
phase: 16
slug: adddocument-atomicity-lucene-contract
status: verified
threats_open: 0
asvs_level: 1
created: 2026-04-23
verified: 2026-04-23
---

# Phase 16 - Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Caller JSON bytes -> builder staging | Untrusted document bytes enter parser/staging and may contain malformed JSON, unsupported scalar values, transformer failures, or numeric forms that conflict with existing path state. | Caller-controlled JSON bytes and derived scalar values |
| Staged document state -> shared builder indexes | Validator-approved `documentBuildState` crosses into mutable `pathData`, Bloom/HLL/trigram state, and document bookkeeping. | Staged paths, numeric stats, row-group membership, document IDs |
| Merge callback -> recover helper | Internal merge code may panic after validator approval; recovery converts that invariant failure into terminal builder state. | Panic type and error state |
| Recover helper -> logger seam | Panic metadata crosses into caller-configured logging. | Sanitized error type and panic type only |
| Developer edit -> local/CI lint | Future edits to marker-protected merge functions must be caught before validation or pull-request completion. | Function signatures and marker placement |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| 16-01/T16-01 | Tampering | `validateStagedPaths`, `mergeStagedPaths`, numeric stats | mitigate | Closed: validator tests cover both lossy numeric directions and builder usability (`gin_test.go:3284`, `gin_test.go:3308`, `gin_test.go:3329`); staged numerics replay into a preview (`builder.go:736`, `builder.go:747`); marked merge signatures no longer return errors (`builder.go:759`, `builder.go:816`, `builder.go:870`); validator-missed branches panic as invariants (`builder.go:855`, `builder.go:876`). | closed |
| 16-01/T16-02 | Denial of Service | merge panic handling | transfer | Closed: transfer target documented in `16-01-PLAN.md:211`; Plan 16-02 wraps merge through `runMergeWithRecover` (`builder.go:704`, `builder.go:722`). | closed |
| 16-01/T16-03 | Information Disclosure | panic logging | transfer | Closed: transfer target documented in `16-01-PLAN.md:212`; recovery logging emits `error.type` and `panic_type` only (`builder.go:725`, `builder.go:726`, `builder.go:727`); tests assert no panic-value key or raw secret attr (`gin_test.go:465`, `gin_test.go:489`, `gin_test.go:494`). | closed |
| 16-01/T16-04 | Tampering | marker/signature invariant | transfer | Closed: transfer target documented in `16-01-PLAN.md:213`; local marker policy and CI step exist (`Makefile:25`, `Makefile:55`, `.github/workflows/ci.yml:68`). | closed |
| 16-02/T16-01 | Tampering | document bookkeeping after recovered merge panic | mitigate | Closed: `mergeDocumentState` sets `b.tragicErr` and returns before doc bookkeeping (`builder.go:704`, `builder.go:705`, `builder.go:706`, `builder.go:709`); builder-level test asserts no bookkeeping on recovered merge panic (`gin_test.go:500`, `gin_test.go:512`, `gin_test.go:515`, `gin_test.go:521`). | closed |
| 16-02/T16-02 | Denial of Service | panic during `mergeStagedPaths` | mitigate | Closed: recovery helper wraps the merge callback and converts panics to errors (`builder.go:704`, `builder.go:722`, `builder.go:729`); direct and builder-level tests cover recovery/refusal (`gin_test.go:452`, `gin_test.go:500`). | closed |
| 16-02/T16-03 | Information Disclosure | recovered panic logging | mitigate | Closed: log attrs are limited to `error.type` and `panic_type` (`builder.go:725`, `builder.go:726`, `builder.go:727`); test asserts Error-level logging, no panic-value key, and no raw secret attr (`gin_test.go:465`, `gin_test.go:477`, `gin_test.go:489`, `gin_test.go:494`); `rg -n 'panic_value' builder.go gin_test.go` returned no output. | closed |
| 16-02/T16-04 | Tampering | marker/signature invariant | transfer | Closed: transfer target documented in `16-02-PLAN.md:240`; local and CI marker checks are present (`Makefile:25`, `.github/workflows/ci.yml:68`). | closed |
| 16-03/T16-01 | Tampering | failed document partial mutation | mitigate | Closed: atomicity helper preserves original doc IDs and expected failures (`atomicity_test.go:17`, `atomicity_test.go:24`, `atomicity_test.go:36`, `atomicity_test.go:44`); deterministic encode and DocIDMapping tests exist (`atomicity_test.go:129`, `atomicity_test.go:145`, `atomicity_test.go:164`); public failures assert `tragicErr == nil` (`atomicity_test.go:205`, `atomicity_test.go:210`, `atomicity_test.go:230`); full-vs-clean property asserts byte equality (`atomicity_test.go:489`, `atomicity_test.go:507`, `atomicity_test.go:511`, `atomicity_test.go:517`). | closed |
| 16-03/T16-02 | Denial of Service | recovered merge panic | covered | Closed: covered disposition documented in `16-03-PLAN.md:304`; Plan 16-02 recovery is implemented (`builder.go:704`, `builder.go:722`); ordinary public failures assert non-tragic state (`atomicity_test.go:205`, `atomicity_test.go:210`). | closed |
| 16-03/T16-03 | Information Disclosure | accept | Closed: accepted below because Plan 16-03 adds tests only and does not introduce new logging, auth, network, file-access, or runtime trust-boundary surface (`16-03-PLAN.md:305`, `16-03-SUMMARY.md:129`, `16-03-SUMMARY.md:131`). Runtime recovered-panic logging is verified under `16-02/T16-03`. | closed |
| 16-03/T16-04 | Tampering | marker/signature invariant | transfer | Closed: transfer target documented in `16-03-PLAN.md:306`; local and CI marker checks are present (`Makefile:25`, `.github/workflows/ci.yml:68`). | closed |
| 16-04/T16-01 | Tampering | failed document mutation | covered | Closed: covered disposition documented in `16-04-PLAN.md:185`; validator tests and atomicity property are present (`gin_test.go:3284`, `gin_test.go:3308`, `atomicity_test.go:489`, `atomicity_test.go:517`). | closed |
| 16-04/T16-02 | Denial of Service | merge panic recovery | covered | Closed: covered disposition documented in `16-04-PLAN.md:186`; recovery is implemented and marker policy prevents reintroducing ordinary `error` returns on marked merge signatures (`builder.go:704`, `builder.go:722`, `Makefile:35`, `Makefile:38`). | closed |
| 16-04/T16-03 | Information Disclosure | recovered panic logging | covered | Closed: covered disposition documented in `16-04-PLAN.md:187`; recovery logging remains limited to type attrs and tests assert no raw panic attr (`builder.go:725`, `builder.go:726`, `builder.go:727`, `gin_test.go:465`, `gin_test.go:489`, `gin_test.go:494`). | closed |
| 16-04/T16-04 | Tampering | CI misses marker/signature invariant regression | mitigate | Closed: local marker policy enforces expected names, direct placement, and no `error` return (`Makefile:25`, `Makefile:29`, `Makefile:36`, `Makefile:37`, `Makefile:38`, `Makefile:47`, `Makefile:55`); CI runs `Check validator markers` before golangci-lint (`.github/workflows/ci.yml:68`, `.github/workflows/ci.yml:69`, `.github/workflows/ci.yml:72`, `.github/workflows/ci.yml:74`); `make check-validator-markers` exited 0 during audit. | closed |

*Status: open / closed*
*Disposition: mitigate (implementation required) / accept (documented risk) / transfer (handled by another plan or external control) / covered (closed by prior phase evidence in this phase set)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-16-01 | 16-03/T16-03 | Plan 16-03 adds tests only and introduces no new logging, auth, network, file-access, or runtime trust-boundary surface. Runtime recovered-panic logging is separately verified under 16-02/T16-03. | GSD security audit | 2026-04-23 |

---

## Unregistered Flags

None. Summary inputs reported no unregistered threat flags: 16-02 has no new recovery/logging flag beyond the plan model, 16-03 is tests-only with no new runtime surface, and 16-01/16-04 had no Threat Flags section in the provided phase summary input.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-04-23 | 16 | 16 | 0 | gsd-security-auditor |

### Audit Commands

- `make check-validator-markers` - PASS
- `go test -run 'Test(ValidateStagedPathsRejectsLossyPromotionBeforeMerge|ValidateStagedPathsRejectsUnsafeIntIntoFloatPath|MixedNumericPathRejectsLossyPromotionLeavesBuilderUsable|AddDocumentRefusesAfterTragicFailure|RunMergeWithRecoverConvertsPanicToTragicError|RunMergeWithRecoverLogsThroughLoggerWithoutPanicValue|AddDocumentRefusesAfterRecoveredMergePanic)$' ./... -count=1` - PASS
- `go test -run 'Test(AddDocumentPublicFailuresDoNotSetTragicErr|AddDocumentAtomicity|AddDocumentAtomicityEncodeDeterminism)$' ./... -count=1` - PASS
- `rg -n 'func .*mergeStagedPaths.*error|func .*mergeNumericObservation.*error|func .*promoteNumericPathToFloat.*error|panic_value|poisonErr|builder poisoned' builder.go gin_test.go atomicity_test.go Makefile .github/workflows/ci.yml` - PASS, no output

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer / covered)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-04-23
