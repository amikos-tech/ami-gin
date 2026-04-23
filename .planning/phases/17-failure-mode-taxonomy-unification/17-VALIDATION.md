---
phase: 17
slug: failure-mode-taxonomy-unification
status: ready
nyquist_compliant: true
wave_0_complete: false
created: 2026-04-23
---

# Phase 17 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing`; existing gopter property tests remain available for atomicity checks |
| **Config file** | `go.mod`, `Makefile`, `.golangci.yml` |
| **Quick run command** | `go test ./... -run 'Test(IngestFailureMode|FailureMode|RepresentationFailureMode|AddDocumentPublicFailuresDoNotSetTragicErr)' -count=1` |
| **Full suite command** | `make test && make lint && go build ./...` |
| **Estimated runtime** | Quick: under 30 seconds; full: project-dependent |

---

## Sampling Rate

- **After every task commit:** Run the focused test command for the touched layer and include `go test ./... -run 'TestAddDocumentPublicFailuresDoNotSetTragicErr' -count=1` when touching builder routing.
- **After every plan wave:** Run `go test ./... -run 'Test(IngestFailureMode|FailureMode|RepresentationFailureMode|AddDocumentPublicFailuresDoNotSetTragicErr|AddDocumentAtomicityEncodeDeterminism)' -count=1`.
- **Before `$gsd-verify-work`:** `make test && make lint && go build ./...` must be green, and `go run ./examples/failure-modes/main.go` must print the expected hard-vs-soft output.
- **Max feedback latency:** keep focused test feedback under 30 seconds unless full-suite execution is explicitly required.

---

## Threat References

| Threat Ref | Threat | Required Mitigation |
|------------|--------|---------------------|
| T-17-01 | Silent data loss through accidental soft defaults | Every layer defaults to `IngestFailureHard`; empty mode normalizes to hard; invalid values fail validation. |
| T-17-02 | False negatives from partially indexed rejected documents | Soft mode skips the whole document before durable merge and before doc bookkeeping advances. |
| T-17-03 | Internal invariant panic swallowed as a soft skip | `runMergeWithRecover` stays tragic and is never routed through soft failure mode handling. |
| T-17-04 | v9 wire-format drift without a version bump | Transformer metadata preserves legacy `strict` / `soft_fail` wire tokens or bumps `Version` with explicit tests. |
| T-17-05 | Raw input leakage while handling soft failures | Phase 17 adds no soft-skip logging or telemetry payloads. |

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 17-01-01 | 01 | 1 | FAIL-01 | T-17-01 | New `IngestFailureMode` API validates hard/soft and rejects invalid values; old public `TransformerFailure*` names are absent from Go source. | unit + static | `go test ./... -run 'TestIngestFailureMode' -count=1 && ! rg -n 'TransformerFailureMode|TransformerFailureStrict|TransformerFailureSoft' --glob '*.go'` | W0 | pending |
| 17-01-02 | 01 | 1 | FAIL-01 | T-17-04 | Transformer failure-mode metadata round-trips and legacy `strict` / `soft_fail` tokens decode into the new type without a format-version bump. | serialization unit | `go test ./... -run 'Test(RepresentationFailureModeRoundTrip|DecodeLegacyTransformerFailureModeTokens)' -count=1` | partial | pending |
| 17-02-01 | 02 | 1 | FAIL-02 | T-17-01 / T-17-02 / T-17-03 | Parser hard mode returns current parse errors; parser soft mode skips before durable mutation; parser contract violations remain hard. | unit matrix | `go test ./... -run 'TestParserFailureMode' -count=1` | W0 | pending |
| 17-02-02 | 02 | 1 | FAIL-02 | T-17-02 | Transformer soft mode skips the whole failed document and does not index the raw rejected value or advance doc bookkeeping. | unit matrix | `go test ./... -run 'TestTransformerFailureMode' -count=1` | partial | pending |
| 17-02-03 | 02 | 1 | FAIL-02 | T-17-02 / T-17-03 | Numeric soft mode covers malformed literals, non-finite/unsupported native numeric values, and validator-rejected promotion while keeping merge recovery tragic. | unit matrix | `go test ./... -run 'TestNumericFailureMode' -count=1` | partial | pending |
| 17-03-01 | 03 | 2 | FAIL-01 | T-17-04 | `CHANGELOG.md` contains the breaking rename migration note and no internal repository or company references. | docs static | `rg -n 'TransformerFailureMode.*IngestFailureMode|TransformerFailureStrict.*IngestFailureHard|TransformerFailureSoft.*IngestFailureSoft' CHANGELOG.md` | W0 | pending |
| 17-03-02 | 03 | 2 | FAIL-02 | T-17-01 / T-17-02 | `examples/failure-modes/main.go` demonstrates rejecting and skipping configurations with deterministic output. | example smoke | `go run ./examples/failure-modes/main.go` | W0 | pending |

---

## Wave 0 Requirements

- [ ] `failure_modes_test.go` - cross-layer hard/soft semantics for parser, transformer, and numeric failure modes.
- [ ] `serialize_security_test.go` legacy-token decode case - proves v9 compatibility for transformer `strict` and `soft_fail` tokens after the API rename.
- [ ] `examples/failure-modes/main.go` - predictable hard rejecting config and soft skipping config.
- [ ] `CHANGELOG.md` - Unreleased breaking-change migration note for `TransformerFailureMode` / `TransformerFailureStrict` / `TransformerFailureSoft`.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Public API migration wording is clear and does not mention internal repository/company information | FAIL-01 | Changelog copy quality and forbidden-reference policy are editorial checks | Read `CHANGELOG.md` and confirm it only names public API symbols and migration guidance. |

---

## Validation Sign-Off

- [x] All planned task classes have automated verify commands or Wave 0 dependencies.
- [x] Sampling continuity: no 3 consecutive tasks without automated verify.
- [x] Wave 0 covers all missing references.
- [x] No watch-mode flags.
- [x] Feedback latency target is under 30 seconds for focused checks.
- [x] `nyquist_compliant: true` set in frontmatter.

**Approval:** approved 2026-04-23
