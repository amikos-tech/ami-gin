---
phase: 09
slug: derived-representations
status: verified
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-16
updated: 2026-04-17T07:59:41Z
---

# Phase 09 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` plus repo-local docs/example smoke commands |
| **Config file** | none - standard Go toolchain via `go.mod` |
| **Quick run command** | `go test ./... -run 'Test(ConfigAllowsMultipleTransformersPerSourcePath|ConfigRejectsDuplicateTransformerAlias|BuilderIndexesRawAndCompanionRepresentations|BuilderFailsWhenCompanionTransformFails|SiblingTransformersDoNotChain|RebuildPathLookupPreservesInternalRepresentationPaths|ValidateJSONPathRejectsInternalRepresentationPaths|QueryAliasRoutingUsesAsWrapper|RawPathQueriesRemainDefaultWithDerivedAliases|RepresentationsIntrospectionListsAliases|FinalizePopulatesRepresentationLookup|CLIInfoSuppressesInternalRepresentationPaths|RepresentationMetadataRoundTrip|RepresentationFailureModeRoundTrip|DecodeRepresentationAliasParity|RepresentationMetadataSectionFollowsConfig|EncodeRejectsNonSerializableRepresentation|DecodeRejectsDuplicateRepresentationAlias|DecodeRejectsRepresentationTargetPathOutOfRange|DecodeRejectsOversizedRepresentationSection|DateTransformerAliasCoverage|NormalizedTextAliasCoverage|RegexExtractAliasCoverage)' -count=1 && rg -n 'gin\.As\(|WithCustomTransformer\(' README.md examples/transformers/main.go examples/transformers-advanced/main.go && ! rg -n '__derived:' README.md examples/transformers/main.go examples/transformers-advanced/main.go && go run ./examples/transformers/main.go >/dev/null && go run ./examples/transformers-advanced/main.go >/dev/null` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | quick run ~3s, full suite ~114s |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -run 'Test(ConfigAllowsMultipleTransformersPerSourcePath|ConfigRejectsDuplicateTransformerAlias|BuilderIndexesRawAndCompanionRepresentations|BuilderFailsWhenCompanionTransformFails|SiblingTransformersDoNotChain|RebuildPathLookupPreservesInternalRepresentationPaths|ValidateJSONPathRejectsInternalRepresentationPaths|QueryAliasRoutingUsesAsWrapper|RawPathQueriesRemainDefaultWithDerivedAliases|RepresentationsIntrospectionListsAliases|FinalizePopulatesRepresentationLookup|CLIInfoSuppressesInternalRepresentationPaths|RepresentationMetadataRoundTrip|RepresentationFailureModeRoundTrip|DecodeRepresentationAliasParity|RepresentationMetadataSectionFollowsConfig|EncodeRejectsNonSerializableRepresentation|DecodeRejectsDuplicateRepresentationAlias|DecodeRejectsRepresentationTargetPathOutOfRange|DecodeRejectsOversizedRepresentationSection|DateTransformerAliasCoverage|NormalizedTextAliasCoverage|RegexExtractAliasCoverage)' -count=1 && rg -n 'gin\.As\(|WithCustomTransformer\(' README.md examples/transformers/main.go examples/transformers-advanced/main.go && ! rg -n '__derived:' README.md examples/transformers/main.go examples/transformers-advanced/main.go && go run ./examples/transformers/main.go >/dev/null && go run ./examples/transformers-advanced/main.go >/dev/null`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green and the docs/example smoke quick run must complete without hidden-path regressions
- **Max feedback latency:** 114 seconds for repo-local validation

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 09-01-01 | 01 | 1 | DERIVE-01 | T-09-01 | Duplicate aliases are rejected and alias registration is explicit per source path | unit | `go test ./... -run 'Test(ConfigAllowsMultipleTransformersPerSourcePath|ConfigRejectsDuplicateTransformerAlias)' -count=1` | ✅ `gin_test.go` | ✅ green |
| 09-01-02 | 01 | 1 | DERIVE-01 | T-09-02 / T-09-03 | Raw values remain indexed, sibling companions do not chain, failed companions abort the document before merge, and the reserved internal namespace bypasses lookup normalization without becoming a public JSONPath | unit + integration | `go test ./... -run 'Test(BuilderIndexesRawAndCompanionRepresentations|BuilderFailsWhenCompanionTransformFails|SiblingTransformersDoNotChain|RebuildPathLookupPreservesInternalRepresentationPaths|ValidateJSONPathRejectsInternalRepresentationPaths)' -count=1` | ✅ `gin_test.go`, `transformers_test.go` | ✅ green |
| 09-02-01 | 02 | 2 | DERIVE-02 | T-09-04 | Alias queries are explicit through `gin.As(...)`; raw-path queries remain default; fresh `Finalize()` output is alias-queryable; CLI/info output suppresses hidden target paths | unit | `go test ./... -run 'Test(QueryAliasRoutingUsesAsWrapper|RawPathQueriesRemainDefaultWithDerivedAliases|RepresentationsIntrospectionListsAliases|FinalizePopulatesRepresentationLookup|CLIInfoSuppressesInternalRepresentationPaths)' -count=1` | ✅ `gin_test.go`, `cmd/gin-index/main_test.go` | ✅ green |
| 09-02-02 | 02 | 2 | DERIVE-03 | T-09-05 / T-09-06 | Representation metadata and transformer failure modes round-trip explicitly, the versioned trailer order remains pinned, invalid target-path bindings are rejected, oversized metadata is rejected, and non-serializable custom companions fail `Encode()` clearly | unit + serialization | `go test ./... -run 'Test(RepresentationMetadataRoundTrip|RepresentationFailureModeRoundTrip|DecodeRepresentationAliasParity|RepresentationMetadataSectionFollowsConfig|EncodeRejectsNonSerializableRepresentation|DecodeRejectsDuplicateRepresentationAlias|DecodeRejectsRepresentationTargetPathOutOfRange|DecodeRejectsOversizedRepresentationSection)' -count=1` | ✅ `serialize_security_test.go` | ✅ green |
| 09-03-01 | 03 | 3 | DERIVE-04 | T-09-07 | README and examples teach additive alias-aware semantics and never require public hidden target paths | example + docs smoke | `rg -n 'gin\\.As\\(|WithCustomTransformer\\(' README.md examples/transformers/main.go examples/transformers-advanced/main.go && ! rg -n '__derived:' README.md examples/transformers/main.go examples/transformers-advanced/main.go && go run ./examples/transformers/main.go && go run ./examples/transformers-advanced/main.go` | ✅ `README.md`, `examples/transformers/main.go`, `examples/transformers-advanced/main.go` | ✅ green |
| 09-03-02 | 03 | 3 | DERIVE-04 | T-09-08 | Acceptance tests prove date/time, normalized text, and extracted-subfield aliases while raw paths remain queryable and each family preserves alias parity across `Encode()` / `Decode()` | unit + integration | `go test ./... -run 'Test(DateTransformerAliasCoverage|NormalizedTextAliasCoverage|RegexExtractAliasCoverage)' -count=1` | ✅ `transformers_test.go` | ✅ green |

*Status: ⬜ pending - ✅ green - ❌ red - ⚠ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements.

---

## Manual-Only Verifications

All phase behaviors remain automatable with repo-local Go tests and docs/example smoke commands.

---

## Validation Audit 2026-04-17 (initial)

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

Audit evidence:
- Confirmed State A input: existing `09-VALIDATION.md` plus executed `09-01` through `09-03` summary artifacts.
- Reviewed `09-01-PLAN.md`, `09-02-PLAN.md`, `09-03-PLAN.md`, and all three phase summaries to map `DERIVE-01` through `DERIVE-04` onto the implemented tasks and expected verification commands.
- Confirmed `.planning/config.json` does not set `workflow.nyquist_validation`; the key is absent rather than `false`, so Nyquist validation remains enabled.
- Cross-referenced the live verification assets in `gin_test.go`, `transformers_test.go`, `serialize_security_test.go`, `cmd/gin-index/main_test.go`, `README.md`, and both transformer example programs.
- Verified all six task-level commands from the verification map, including `EncodeRejectsNonSerializableRepresentation`, `DecodeRejectsDuplicateRepresentationAlias`, and the README/example hidden-path smoke for `DERIVE-04`; all passed.
- Updated the quick-run contract to include the previously omitted representation-serialization guardrail tests and docs/example smoke so the phase sampling command now covers every automated requirement.
- Verified the corrected quick run completed in `3s` on this machine.
- Verified the repo-wide regression sweep with `go test ./... -count=1`, which passed in `91.612s` for `github.com/amikos-tech/ami-gin` on this machine.

---

## Validation Audit 2026-04-17 (re-audit)

| Metric | Count |
|--------|-------|
| Gaps found | 1 |
| Resolved | 0 |
| Escalated | 1 |

Audit evidence:
- Re-read `09-01-PLAN.md`, `09-02-PLAN.md`, `09-03-PLAN.md`, the three summary files, and the existing validation map to compare the planned secure behaviors against the live test inventory.
- Confirmed `workflow.nyquist_validation` remains enabled because `.planning/config.json` does not set it to `false`.
- Replaced the lenient regression in [transformers_test.go](/Users/tazarov/experiments/amikos/custom-gin/transformers_test.go:481) with `TestBuilderFailsWhenCompanionTransformFails` so the builder verification command now exercises the documented strict-failure requirement directly.
- Verified the targeted regression command now fails as expected for the remaining gap:
  `go test ./... -run 'Test(BuilderIndexesRawAndCompanionRepresentations|BuilderFailsWhenCompanionTransformFails|SiblingTransformersDoNotChain|RebuildPathLookupPreservesInternalRepresentationPaths|ValidateJSONPathRejectsInternalRepresentationPaths)' -count=1`
- Captured the failure: `AddDocument(0) error = nil, want strict companion failure`.
- Verified the current quick-run contract also fails immediately on the same regression:
  `go test ./... -run 'Test(ConfigAllowsMultipleTransformersPerSourcePath|ConfigRejectsDuplicateTransformerAlias|BuilderIndexesRawAndCompanionRepresentations|BuilderFailsWhenCompanionTransformFails|SiblingTransformersDoNotChain|RebuildPathLookupPreservesInternalRepresentationPaths|ValidateJSONPathRejectsInternalRepresentationPaths|QueryAliasRoutingUsesAsWrapper|RawPathQueriesRemainDefaultWithDerivedAliases|RepresentationsIntrospectionListsAliases|FinalizePopulatesRepresentationLookup|CLIInfoSuppressesInternalRepresentationPaths|RepresentationMetadataRoundTrip|DecodeRepresentationAliasParity|RepresentationMetadataSectionFollowsConfig|EncodeRejectsNonSerializableRepresentation|DecodeRejectsDuplicateRepresentationAlias|DecodeRejectsRepresentationTargetPathOutOfRange|DecodeRejectsOversizedRepresentationSection|DateTransformerAliasCoverage|NormalizedTextAliasCoverage|RegexExtractAliasCoverage)' -count=1 && ...`
- Verified the full suite is now red for the same reason:
  `go test ./... -count=1`
- Confirmed the implementation defect in [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:575): `stageCompanionRepresentations()` still does `continue` when `registration.FieldTransformer(prepared)` returns `ok=false`, so failed companions are silently skipped instead of aborting the document before merge.
- Reclassified `09-01-02` from green to red because the remaining gap is a product-behavior mismatch, not missing test infrastructure.

---

## Validation Audit 2026-04-17 (current worktree)

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 1 |
| Escalated | 0 |

Audit evidence:
- Re-ran the State A audit against the same phase artifacts after confirming the current worktree now includes the strict-companion-failure follow-up in `builder.go`, `gin.go`, `transformer_registry.go`, `serialize.go`, `transformers_test.go`, `serialize_security_test.go`, and `phase09_review_test.go`.
- Confirmed `workflow.nyquist_validation` remains enabled because `.planning/config.json` still omits the key rather than setting it to `false`.
- Verified the previously blocked `09-01-02` task now passes on the live code: `stageCompanionRepresentations()` returns an explicit error on `ok=false` unless the registration opts into `WithTransformerFailureMode(TransformerFailureSoft)`.
- Verified the builder regression command is green again: `go test ./... -run 'Test(BuilderIndexesRawAndCompanionRepresentations|BuilderFailsWhenCompanionTransformFails|SiblingTransformersDoNotChain|RebuildPathLookupPreservesInternalRepresentationPaths|ValidateJSONPathRejectsInternalRepresentationPaths)' -count=1`.
- Verified the live representation-metadata surface now also round-trips failure-mode configuration with `go test ./... -run 'Test(RepresentationMetadataRoundTrip|RepresentationFailureModeRoundTrip|DecodeRepresentationAliasParity|RepresentationMetadataSectionFollowsConfig|EncodeRejectsNonSerializableRepresentation|DecodeRejectsDuplicateRepresentationAlias|DecodeRejectsRepresentationTargetPathOutOfRange|DecodeRejectsOversizedRepresentationSection)' -count=1`.
- Verified the full Phase 09 quick-run contract, including README/example smoke and hidden-path suppression, with the updated command that now includes `RepresentationFailureModeRoundTrip`; it passed on this machine.
- Verified the repo-wide regression sweep with `go test ./... -count=1`, which passed in `113.464s` for `github.com/amikos-tech/ami-gin` on this machine.
- Updated the per-task map to mark `09-01-02` green and expanded `09-02-02` automation to track the current serialized representation contract on the live worktree.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 114s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-04-17
