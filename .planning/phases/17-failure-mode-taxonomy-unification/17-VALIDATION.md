---
phase: 17
slug: failure-mode-taxonomy-unification
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-23
last_audit: 2026-04-23
---

# Phase 17 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing`, exact-output example regression tests, and static source-inspection checks |
| **Config file** | `go.mod`, `Makefile`, `.golangci.yml` |
| **Quick run command** | `go test ./... -run 'Test(IngestFailureMode|ValidateIngestFailureModeRejectsLegacyTokens|DeprecatedTransformerFailureAliasesRemainAccepted|RepresentationFailureModeRoundTrip|DecodeLegacyTransformerFailureModeTokens|ReadConfigRejectsUnknownTransformerFailureMode|ReadConfigRejectsCorruptJSONAsInvalidFormat|TransformerFailureModeWireTokensStayV9|ParserFailureMode|SoftSkippedDocIDCanBeRetriedWithoutPositionConsumption|SoftSkippedDocumentsAreObservable|NumericFailureMode|TransformerFailureMode|BuilderSoftFailSkipsOnlyRepresentationWhenConfigured|SoftFailureModesMatchCleanCorpus|AllSoftFailureModesApplyConfiguredScope|AddDocumentPublicFailuresDoNotSetTragicErr|AddDocumentRefusesAfterRecoveredMergePanic|FinalizeAfterTragedyReturnsNilAndLogs|AddDocumentAtomicityUnderSoftMode)$' -count=1 && go test ./examples/failure-modes -run 'TestFailureModesExampleOutput$' -count=1` |
| **Full suite command** | `make test && make lint && go build ./...` |
| **Estimated runtime** | Quick: under 15 seconds warm-cache; full: about 45 seconds plus lint/build |

---

## Sampling Rate

- **After every task commit:** Run the task-specific command from the table below; include the changelog static check when touching `CHANGELOG.md` or the public failure-mode rename surface.
- **After every plan wave:** Run the Quick run command above.
- **Before `$gsd-verify-work`:** `make test && make lint && go build ./...` must be green, and `go run ./examples/failure-modes/main.go` must print the expected three-line hard-vs-soft output.
- **Max feedback latency:** keep task-specific checks under 30 seconds; reserve the full suite for wave and phase gates.

---

## Threat References

| Threat Ref | Threat | Required Mitigation |
|------------|--------|---------------------|
| T-17-01 | Silent data loss through accidental soft defaults | Every layer defaults to `IngestFailureHard`; empty mode normalizes to hard; invalid values fail validation. |
| T-17-02 | False negatives from partially indexed rejected documents | Parser and numeric soft modes skip the whole document before durable merge; transformer soft mode skips only the derived representation while preserving the raw document. |
| T-17-03 | Internal invariant panic swallowed as a soft skip | `runMergeWithRecover` stays tragic and is never routed through soft failure mode handling. |
| T-17-04 | v9 wire-format drift without a version bump | Transformer metadata preserves legacy `strict` / `soft_fail` wire tokens or bumps `Version` with explicit tests. |
| T-17-05 | Raw input leakage while handling soft failures | Soft-skip observability is limited to bounded INFO attrs and aggregate counters; no raw document payloads or identifiers are logged. |

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 17-01-01 | 01 | 1 | FAIL-01 | T-17-01 | New `IngestFailureMode` API validates hard/soft and rejects invalid values; deprecated `TransformerFailure*` aliases remain source-compatible for pre-phase-17 callers. | unit | `go test ./... -run 'Test(IngestFailureModeDefaultsAndValidation|ValidateIngestFailureModeRejectsLegacyTokens|DeprecatedTransformerFailureAliasesRemainAccepted)$' -count=1` | `failure_modes_test.go`, `gin.go` | ✅ green |
| 17-01-02 | 02 | 1 | FAIL-01 | T-17-04 | Transformer failure-mode metadata round-trips, legacy `strict` / `soft_fail` tokens decode into the new type, unknown wire tokens are rejected, and v9 wire spelling is preserved without a version bump. | serialization unit | `go test ./... -run 'Test(RepresentationFailureModeRoundTrip|DecodeLegacyTransformerFailureModeTokens|ReadConfigRejectsUnknownTransformerFailureMode|TransformerFailureModeWireTokensStayV9)$' -count=1` | `serialize_security_test.go` | ✅ green |
| 17-02-01 | 03 | 2 | FAIL-02 | T-17-01 / T-17-02 / T-17-03 | Parser hard mode returns current parse errors; parser soft mode skips before durable mutation; contract violations stay hard; soft-skipped `DocID`s do not consume positions. | unit matrix | `go test ./... -run 'Test(ParserFailureMode|SoftSkippedDocIDCanBeRetriedWithoutPositionConsumption)$' -count=1` | `failure_modes_test.go` | ✅ green |
| 17-02-02 | 03 | 2 | FAIL-02 | T-17-02 / T-17-05 | Transformer hard mode returns the current companion error; transformer soft mode skips only the failed derived representation, preserves raw staged fields, and keeps raw rejected values queryable. | unit matrix | `go test ./... -run 'Test(TransformerFailureMode|BuilderSoftFailSkipsOnlyRepresentationWhenConfigured|TransformerFailureModeSoftKeepsPartiallyStagedDocumentWithoutCompanion)$' -count=1` | `failure_modes_test.go`, `transformers_test.go` | ✅ green |
| 17-02-03 | 03 | 2 | FAIL-02 | T-17-02 / T-17-03 / T-17-05 | Numeric soft mode covers malformed literals, non-finite native values, validator-rejected promotion, soft-skip observability, the scoped full-vs-clean oracle, and tragic merge refusal. | unit matrix + property | `go test ./... -run 'Test(NumericFailureMode|SoftSkippedDocumentsAreObservable|SoftFailureModesMatchCleanCorpus|AllSoftFailureModesApplyConfiguredScope|AddDocumentPublicFailuresDoNotSetTragicErr|AddDocumentRefusesAfterRecoveredMergePanic|FinalizeAfterTragedyReturnsNilAndLogs|AddDocumentAtomicityUnderSoftMode)$' -count=1` | `failure_modes_test.go`, `atomicity_test.go`, `gin_test.go` | ✅ green |
| 17-03-01 | 04 | 3 | FAIL-01 | T-17-01 / T-17-04 | `CHANGELOG.md` contains the breaking rename migration note and no repository-hosting or company-internal references. | docs static | `rg -n 'TransformerFailureMode.*IngestFailureMode|TransformerFailureStrict.*IngestFailureHard|TransformerFailureSoft.*IngestFailureSoft' CHANGELOG.md && ! rg -n 'https?://|/Users/|github\.teliacompany\.net|teliacompany' CHANGELOG.md` | `CHANGELOG.md` | ✅ green |
| 17-03-02 | 04 | 3 | FAIL-02 | T-17-01 / T-17-02 / T-17-05 | `examples/failure-modes/main.go` demonstrates rejecting config, scoped soft skips, soft-skip counters, and deterministic dense row-group packing. | example smoke + exact output regression | `go test ./examples/failure-modes -run 'TestFailureModesExampleOutput$' -count=1 && go run ./examples/failure-modes/main.go` | `examples/failure-modes/main.go`, `examples/failure-modes/main_test.go` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠ accepted/manual review*

---

## Wave 0 Requirements

- [x] `failure_modes_test.go` - cross-layer hard/soft semantics for parser, transformer, and numeric failure modes.
- [x] `serialize_security_test.go` legacy-token decode and v9 wire-token regression coverage - proves compatibility for transformer `strict` and `soft_fail` tokens after the API rename.
- [x] `examples/failure-modes/main.go` and `examples/failure-modes/main_test.go` - predictable hard rejecting config and soft skipping config with exact-output regression coverage.
- [x] `CHANGELOG.md` - Unreleased migration note for `IngestFailureMode` plus the restored deprecated `TransformerFailureMode` / `TransformerFailureStrict` / `TransformerFailureSoft` aliases.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Public API migration wording is clear for external callers | FAIL-01 | Changelog copy quality is editorial even though symbol/static checks are automated | Read `CHANGELOG.md` and confirm it only names public API symbols and the before/after snippet is accurate. |

---

## Validation Sign-Off

- [x] All planned task classes have automated verify commands or Wave 0 dependencies.
- [x] Sampling continuity: no 3 consecutive tasks without automated verify.
- [x] Wave 0 covers all missing references.
- [x] No watch-mode flags.
- [x] Feedback latency target is bounded by focused Go tests and static checks.
- [x] `nyquist_compliant: true` set in frontmatter.

**Approval:** approved 2026-04-23 after validation audit

---

## Validation Audit 2026-04-23

| Metric | Count |
|--------|-------|
| Tasks audited | 7 |
| Gaps found | 1 |
| Resolved | 1 |
| Escalated | 0 |

**Findings:**

- The existing `17-VALIDATION.md` was still in a pre-audit state: every row was `pending`, `wave_0_complete` was `false`, several `File Exists` cells used placeholder values, and the transformer regression command still referenced an obsolete soft-fail test name. This audit updated the validation artifact to match the live Phase 17 test suite and audited status. No code or test-file changes were required.

Audit evidence:

- Confirmed State A input: existing `17-VALIDATION.md`, all four Phase 17 plan files, all four Phase 17 summary artifacts, and `17-VERIFICATION.md` are present.
- Confirmed Nyquist validation is enabled in `.planning/config.json` via `workflow.nyquist_validation: true`.
- Rebuilt the requirement-to-task map from `17-01-PLAN.md` through `17-04-PLAN.md` and the corresponding summaries for `FAIL-01` and `FAIL-02`.
- Cross-referenced shipped coverage in `failure_modes_test.go`, `serialize_security_test.go`, `transformers_test.go`, `atomicity_test.go`, `examples/failure-modes/main_test.go`, and `CHANGELOG.md`.
- Re-ran focused verification commands for API validation, deprecated alias compatibility, legacy wire-token compatibility, parser soft skips, transformer representation-scoped soft skips, numeric/atomicity coverage, tragic finalize refusal, changelog static checks, and the failure-modes example regression; all passed.
- Re-ran the phase integration gate: `make test` passed with 926 tests and 1 skipped in 43.906s, `make lint` passed with 0 issues, and `go build ./...` passed.
- No COVERED/PARTIAL/MISSING code-validation gaps remained after the live audit, so the gap-fix gate and `gsd-nyquist-auditor` escalation path were not needed for Phase 17.
- No new test files were required by this audit; the shipped Phase 17 tests already satisfy the Nyquist coverage contract.
