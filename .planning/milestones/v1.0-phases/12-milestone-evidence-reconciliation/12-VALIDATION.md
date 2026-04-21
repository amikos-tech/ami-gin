---
phase: 12
slug: milestone-evidence-reconciliation
status: verified
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-20
updated: 2026-04-21T06:02:42Z
---

# Phase 12 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Repo-local Go `testing`, example smoke runs, and `.planning/` artifact inspection |
| **Config file** | none - standard Go toolchain via `go.mod` plus existing planning artifacts |
| **Quick run command** | `go test ./... -run 'Test(AddDocumentRejectsUnsupportedNumberWithoutPartialMutation|NumericIndexPreservesInt64Exactness|MixedNumericPathRejectsLossyPromotion|IntOnlyNumericDecodeParity|TransformerNumericPathExplicitParserCompatibility|TransformerNumericDecodeParity|QueryAliasRoutingUsesAsWrapper|RawPathQueriesRemainDefaultWithDerivedAliases|RepresentationsIntrospectionListsAliases|FinalizePopulatesRepresentationLookup|RepresentationMetadataRoundTrip|DecodeRepresentationAliasParity|DateTransformerAliasCoverage|NormalizedTextAliasCoverage|RegexExtractAliasCoverage)' -count=1 && go run ./examples/transformers/main.go >/dev/null && go run ./examples/transformers-advanced/main.go >/dev/null && test -f .planning/phases/07-builder-parsing-numeric-fidelity/07-VERIFICATION.md && test -f .planning/phases/09-derived-representations/09-VERIFICATION.md && rg -n 'BUILD-0[1-5].*Complete|DERIVE-0[1-4].*Complete' .planning/REQUIREMENTS.md >/dev/null && rg -n '^status: passed$|^## Milestone Pass Rationale$|go test ./\\.\\.\\. -count=1|go run ./examples/transformers/main.go|go run ./examples/transformers-advanced/main.go|^Phase 07 validation debt: closed on current-tree evidence\\.$|^Accepted validation gap:' .planning/v1.0-MILESTONE-AUDIT.md >/dev/null` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | quick run ~5s, benchmark check ~65s, full suite ~40s |

---

## Sampling Rate

- **After every task commit:** Run the task-specific command from the verification map below.
- **After every plan wave:** Run the quick run command above.
- **Before `$gsd-verify-work`:** Full suite must be green and the refreshed milestone audit artifact must cite current-tree command output.
- **Max feedback latency:** 120 seconds for repo-local validation.

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 12-01-01 | 01 | 1 | BUILD-01, BUILD-02, BUILD-03, BUILD-04 | T-12-01 / T-12-02 | `07-VERIFICATION.md` must bind explicit parser, exact-int, lossy-promotion rejection, and failure-atomicity claims to current-tree evidence instead of summary-only claims | unit + docs | `go test ./... -run 'Test(AddDocumentRejectsUnsupportedNumberWithoutPartialMutation|NumericIndexPreservesInt64Exactness|MixedNumericPathRejectsLossyPromotion|IntOnlyNumericDecodeParity|TransformerNumericPathExplicitParserCompatibility|TransformerNumericDecodeParity)' -count=1 && test -f .planning/phases/07-builder-parsing-numeric-fidelity/07-VERIFICATION.md` | ✅ `07-VERIFICATION.md` | ✅ green |
| 12-01-02 | 01 | 1 | BUILD-05 | T-12-03 | Phase 07 benchmark evidence must remain reproducible on the current tree and be cited in `07-VERIFICATION.md` | benchmark | `go test ./... -run '^$' -bench 'Benchmark(AddDocumentPhase07|BuildPhase07|FinalizePhase07)' -benchtime=1x -count=1 && test -f .planning/phases/07-builder-parsing-numeric-fidelity/07-VERIFICATION.md` | ✅ `07-VERIFICATION.md`, ✅ `benchmark_test.go` | ✅ green |
| 12-01-03 | 01 | 1 | BUILD-01, BUILD-02, BUILD-03, BUILD-04, BUILD-05 | T-12-04 | Phase 07 validation debt must be closed with a refreshed compliant `07-VALIDATION.md`, or the remaining gap must be explicitly recorded in the milestone audit | docs + validation | `test -f .planning/phases/07-builder-parsing-numeric-fidelity/07-VALIDATION.md && test -f .planning/v1.0-MILESTONE-AUDIT.md && rg -n '^status: verified$|^nyquist_compliant: true$|^\\*\\*Approval:\\*\\* approved 20[0-9]{2}-[0-9]{2}-[0-9]{2}$|^Accepted validation gap:' .planning/phases/07-builder-parsing-numeric-fidelity/07-VALIDATION.md && rg -n '^Phase 07 validation debt: closed on current-tree evidence\\.$|^Accepted validation gap:' .planning/v1.0-MILESTONE-AUDIT.md` | ✅ `07-VALIDATION.md`, ✅ `v1.0-MILESTONE-AUDIT.md` | ✅ green |
| 12-02-01 | 02 | 1 | DERIVE-01, DERIVE-02, DERIVE-03, DERIVE-04 | T-12-05 / T-12-06 | `09-VERIFICATION.md` must prove raw-plus-companion behavior, alias routing, metadata round-trip, and docs/example coverage on the current tree | unit + integration + example | `go test ./... -run 'Test(QueryAliasRoutingUsesAsWrapper|RawPathQueriesRemainDefaultWithDerivedAliases|RepresentationsIntrospectionListsAliases|FinalizePopulatesRepresentationLookup|RepresentationMetadataRoundTrip|DecodeRepresentationAliasParity|DateTransformerAliasCoverage|NormalizedTextAliasCoverage|RegexExtractAliasCoverage)' -count=1 && go run ./examples/transformers/main.go >/dev/null && go run ./examples/transformers-advanced/main.go >/dev/null && test -f .planning/phases/09-derived-representations/09-VERIFICATION.md` | ✅ `09-VERIFICATION.md`, ✅ `README.md`, ✅ transformer examples | ✅ green |
| 12-03-01 | 03 | 2 | BUILD-01, BUILD-02, BUILD-03, BUILD-04, BUILD-05, DERIVE-01, DERIVE-02, DERIVE-03, DERIVE-04 | T-12-07 / T-12-08 | `REQUIREMENTS.md` and `v1.0-MILESTONE-AUDIT.md` must not claim completion without matching phase verification artifacts and current-tree command evidence | docs + integration | `rg -n 'BUILD-0[1-5].*Complete|DERIVE-0[1-4].*Complete' .planning/REQUIREMENTS.md && test -f .planning/phases/07-builder-parsing-numeric-fidelity/07-VERIFICATION.md && test -f .planning/phases/09-derived-representations/09-VERIFICATION.md && rg -n '^status: passed$|^  requirements: 20/20$|^  phases: 6/6$|^  integration: 5/5$|^  flows: 3/3$|^  requirements: \\[\\]$|^  integration: \\[\\]$|^  flows: \\[\\]$|^## Milestone Pass Rationale$|go test ./\\.\\.\\. -count=1|go run ./examples/transformers/main.go|go run ./examples/transformers-advanced/main.go|^Phase 07 validation debt: closed on current-tree evidence\\.$|^Accepted validation gap:' .planning/v1.0-MILESTONE-AUDIT.md && go test ./... -count=1` | ✅ `REQUIREMENTS.md`, ✅ `v1.0-MILESTONE-AUDIT.md`, ✅ `07-VERIFICATION.md`, ✅ `09-VERIFICATION.md` | ✅ green |

*Status: ⬜ pending - ✅ green - ❌ red - ⚠ flaky*

---

## Wave 0 Requirements

- [x] `gin_test.go`, `transformers_test.go`, and `benchmark_test.go` already cover the BUILD evidence reconstruction surface reused by Plan `12-01`.
- [x] `gin_test.go`, `serialize_security_test.go`, `transformers_test.go`, `cmd/gin-index/main_test.go`, `README.md`, and both transformer examples already cover the DERIVE evidence reconstruction surface reused by Plan `12-02`.
- [x] `.planning/phases/07-builder-parsing-numeric-fidelity/07-VALIDATION.md`, `.planning/phases/07-builder-parsing-numeric-fidelity/07-VERIFICATION.md`, `.planning/phases/09-derived-representations/09-VERIFICATION.md`, `.planning/REQUIREMENTS.md`, and `.planning/v1.0-MILESTONE-AUDIT.md` provide the artifact-inspection surface required by Plan `12-03`.

---

## Manual-Only Verifications

All planned phase behaviors can be reduced to repo-local commands and artifact inspection. No manual-only verification is expected.

---

## Validation Audit 2026-04-21

| Metric | Count |
|--------|-------|
| Gaps found | 3 |
| Resolved | 3 |
| Escalated | 0 |

Audit evidence:
- Confirmed State A input: existing `12-VALIDATION.md`, all three Phase 12 plan summaries, and a passed Phase 12 verification report were already present.
- Confirmed Nyquist validation remains enabled because `.planning/config.json` omits `workflow.nyquist_validation` instead of setting it to `false`.
- Re-read `12-01-PLAN.md`, `12-02-PLAN.md`, `12-03-PLAN.md`, the three Phase 12 summary files, and `12-VERIFICATION.md` to rebuild the requirement-to-task map for `BUILD-01` through `BUILD-05` and `DERIVE-01` through `DERIVE-04`.
- Cross-referenced the live proof surface in `gin_test.go`, `transformers_test.go`, `serialize_security_test.go`, `benchmark_test.go`, `cmd/gin-index/main_test.go`, `README.md`, `examples/transformers/main.go`, and `examples/transformers-advanced/main.go`; no missing product tests were found.
- Re-ran the Phase 07 regression evidence check and it passed: `go test ./... -run 'Test(AddDocumentRejectsUnsupportedNumberWithoutPartialMutation|NumericIndexPreservesInt64Exactness|MixedNumericPathRejectsLossyPromotion|IntOnlyNumericDecodeParity|TransformerNumericPathExplicitParserCompatibility|TransformerNumericDecodeParity)' -count=1 && test -f .planning/phases/07-builder-parsing-numeric-fidelity/07-VERIFICATION.md` produced `ok github.com/amikos-tech/ami-gin 0.418s` and `ok github.com/amikos-tech/ami-gin/cmd/gin-index 0.755s [no tests to run]`.
- Re-ran the Phase 07 benchmark evidence check and it passed: `go test ./... -run '^$' -bench 'Benchmark(AddDocumentPhase07|BuildPhase07|FinalizePhase07)' -benchtime=1x -count=1 && test -f .planning/phases/07-builder-parsing-numeric-fidelity/07-VERIFICATION.md` completed with `PASS`, `ok github.com/amikos-tech/ami-gin 61.827s`, and `ok github.com/amikos-tech/ami-gin/cmd/gin-index 0.430s`; the benchmark surface still covers add/build/finalize across `int-only`, `mixed-safe`, `wide-flat`, and `transformer-heavy` shapes.
- Re-ran the Phase 09 alias/representation/example evidence check and it passed: `go test ./... -run 'Test(QueryAliasRoutingUsesAsWrapper|RawPathQueriesRemainDefaultWithDerivedAliases|RepresentationsIntrospectionListsAliases|FinalizePopulatesRepresentationLookup|RepresentationMetadataRoundTrip|DecodeRepresentationAliasParity|DateTransformerAliasCoverage|NormalizedTextAliasCoverage|RegexExtractAliasCoverage)' -count=1 && go run ./examples/transformers/main.go >/dev/null && go run ./examples/transformers-advanced/main.go >/dev/null && test -f .planning/phases/09-derived-representations/09-VERIFICATION.md` produced `ok github.com/amikos-tech/ami-gin 0.329s` and `ok github.com/amikos-tech/ami-gin/cmd/gin-index 0.513s [no tests to run]`.
- Re-ran the ledger/full-suite reconciliation command and it passed: `rg -n 'BUILD-0[1-5].*Complete|DERIVE-0[1-4].*Complete' .planning/REQUIREMENTS.md && test -f .planning/phases/07-builder-parsing-numeric-fidelity/07-VERIFICATION.md && test -f .planning/phases/09-derived-representations/09-VERIFICATION.md && go test ./... -count=1` confirmed all nine reconciled rows and produced `ok github.com/amikos-tech/ami-gin 38.416s` plus `ok github.com/amikos-tech/ami-gin/cmd/gin-index 0.548s`.
- Verified the milestone audit remains a pass-state rerun with current-tree evidence: `.planning/v1.0-MILESTONE-AUDIT.md` still contains `status: passed`, `requirements: 20/20`, `phases: 6/6`, `integration: 5/5`, `flows: 3/3`, empty `gaps` arrays, `## Milestone Pass Rationale`, the rerun evidence lines for `go test ./... -count=1`, `go run ./examples/transformers/main.go`, `go run ./examples/transformers-advanced/main.go`, and `Phase 07 validation debt: closed on current-tree evidence.`.
- Closed three validation-contract gaps without adding new product tests: the quick-run sample now checks both rebuilt verification artifacts plus milestone-audit pass-state, `12-01-03` now verifies the actual Phase 07 approval/pass-state line instead of loose header text, and `12-03-01` now asserts milestone-audit pass-state/evidence lines rather than relying on full-suite success alone.

No new test files were required for Phase 12. Existing repo-local tests, benchmark smoke, example runs, and `.planning/` artifact inspection already provide full coverage for the completed phase.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-04-21
