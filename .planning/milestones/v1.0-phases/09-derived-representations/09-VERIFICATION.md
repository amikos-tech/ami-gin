---
phase: 09-derived-representations
verified: 2026-04-21T04:51:10Z
status: passed
score: 4/4 must-haves verified
overrides_applied: 0
---

# Phase 09: Derived Representations Verification Report

**Phase Goal:** Prove the shipped Phase 09 raw-plus-derived representation contract on the current tree, with explicit evidence for additive indexing, alias routing, representation metadata round-trip, and public docs/example coverage.
**Verified:** 2026-04-21T04:51:10Z
**Status:** passed
**Re-verification:** Yes — milestone evidence reconstruction on the current tree

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | DERIVE-01 remains live on the current tree: one source path can keep its raw value while companion representations stage under hidden internal targets, and internal `__derived:` paths stay reserved rather than becoming a public JSONPath contract. | ✓ VERIFIED | `gin.go:41,465-532,878`; `query.go:74-107`; `go test ./... -run 'Test(ConfigAllowsMultipleTransformersPerSourcePath|ConfigRejectsDuplicateTransformerAlias|BuilderIndexesRawAndCompanionRepresentations|BuilderFailsWhenCompanionTransformFails|SiblingTransformersDoNotChain|RebuildPathLookupPreservesInternalRepresentationPaths|ValidateJSONPathRejectsInternalRepresentationPaths)' -count=1` passed on HEAD. |
| 2 | DERIVE-02 remains explicit and additive: alias queries route through `gin.As(alias, value)`, raw-path queries remain the default behavior, fresh `Finalize()` output rebuilds alias lookup, and CLI diagnostics suppress raw hidden target paths. | ✓ VERIFIED | `gin.go:262-271,684,878`; `query.go:74-156`; `cmd/gin-index/main.go:445-477`; `go test ./... -run 'Test(QueryAliasRoutingUsesAsWrapper|RawPathQueriesRemainDefaultWithDerivedAliases|RepresentationsIntrospectionListsAliases|FinalizePopulatesRepresentationLookup|RepresentationMetadataRoundTrip|RepresentationFailureModeRoundTrip|DecodeRepresentationAliasParity|RepresentationMetadataSectionFollowsConfig|EncodeRejectsNonSerializableRepresentation|DecodeRejectsDuplicateRepresentationAlias|DecodeRejectsRepresentationTargetPathOutOfRange|DecodeRejectsOversizedRepresentationSection|DateTransformerAliasCoverage|NormalizedTextAliasCoverage|RegexExtractAliasCoverage)' -count=1` and `go test ./... -run 'TestCLIInfoSuppressesInternalRepresentationPaths' -count=1` both passed on HEAD. |
| 3 | DERIVE-03 remains pinned by explicit representation metadata: encode/decode preserves source-path, alias, target-path, transformer, and serializability metadata, and malformed or unreconstructable representation sections are rejected rather than inferred. | ✓ VERIFIED | `serialize.go:235,348,1597-1628`; `serialize_security_test.go:565,587,616,1053,1070,1096,1138,1283`; the targeted representation-metadata command passed on HEAD with `ok github.com/amikos-tech/ami-gin 0.445s` and `ok github.com/amikos-tech/ami-gin/cmd/gin-index 0.745s [no tests to run]`. |
| 4 | DERIVE-04 remains supported by current public artifacts, not just tests: README and both example programs teach raw-path coexistence plus explicit alias queries, the examples run successfully, and the acceptance tests still cover date/time, normalized text, and extracted-subfield companions. | ✓ VERIFIED | `README.md:151-212`; `examples/transformers/main.go:18-146`; `examples/transformers-advanced/main.go:18-349`; `transformers_test.go:1388,1443,1508`; `rg -n 'gin\.As\(|WithCustomTransformer\(' README.md examples/transformers/main.go examples/transformers-advanced/main.go` returned the public alias usage points, and no public `__derived:` references were found. |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `gin.go` | Additive representation registration, alias wrapper, and read-only representation metadata | ✓ VERIFIED | `gin.go:41,262-271,465-532,878` defines the reserved internal prefix, `RepresentationValue`, `As(...)`, built-in alias-bearing helpers, and `Representations(...)`. |
| `query.go` | Explicit raw-vs-alias predicate resolution without requiring hidden paths publicly | ✓ VERIFIED | `query.go:74-156` unwraps `RepresentationValue`, routes alias lookups through `representationLookup`, and leaves raw-path lookup unchanged. |
| `serialize.go` | Dedicated representation metadata trailer with explicit read/write helpers | ✓ VERIFIED | `serialize.go:235,348,1597-1628` writes and reads representation metadata as its own section rather than inferring alias bindings from hidden target names. |
| `serialize_security_test.go` | Round-trip parity plus malformed-section and non-serializable guardrails | ✓ VERIFIED | `serialize_security_test.go:565,587,616,1053,1070,1096,1138,1283` covers metadata round-trip, failure-mode round-trip, trailer order, duplicate aliases, invalid target paths, oversized sections, and non-serializable representations. |
| `README.md` | Public alias-aware contract and custom-transformer serialization caveat | ✓ VERIFIED | `README.md:151-212` states that raw-path queries stay raw, companions use `gin.As(alias, value)`, hidden paths are internal-only, and `WithCustomTransformer(...)` companions are not serializable. |
| `examples/transformers/main.go` | Runnable date/time example with raw-path coexistence plus alias queries | ✓ VERIFIED | `examples/transformers/main.go:18-146` registers `epoch_ms` companions and demonstrates both raw string queries and alias-based date range queries. |
| `examples/transformers-advanced/main.go` | Runnable normalized-text and extracted-subfield examples with alias queries | ✓ VERIFIED | `examples/transformers-advanced/main.go:164-243,249-301` demonstrates `lower`, `domain`, `error_code`, and `order_number` alias queries without exposing hidden target paths. |
| `cmd/gin-index/main.go` | Public diagnostics that suppress internal representation paths | ✓ VERIFIED | `cmd/gin-index/main.go:445-477` skips raw `__derived:` path entries and renders companion metadata via `Representations(...)`. |
| `gin_test.go` | Alias-routing and fresh-finalize coverage | ✓ VERIFIED | `gin_test.go:2566,2600,2629,2654` covers alias routing, raw-path defaults, representation introspection, and fresh-finalize alias lookup rebuilds. |
| `transformers_test.go` | End-to-end DERIVE-04 acceptance coverage | ✓ VERIFIED | `transformers_test.go:1388,1443,1508` covers date/time, normalized text, and regex extract alias parity before and after encode/decode. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Additive builder semantics and strict companion failure (Plan 09-01 / DERIVE-01) | `go test ./... -run 'Test(ConfigAllowsMultipleTransformersPerSourcePath|ConfigRejectsDuplicateTransformerAlias|BuilderIndexesRawAndCompanionRepresentations|BuilderFailsWhenCompanionTransformFails|SiblingTransformersDoNotChain|RebuildPathLookupPreservesInternalRepresentationPaths|ValidateJSONPathRejectsInternalRepresentationPaths)' -count=1` | Passed on HEAD: `ok github.com/amikos-tech/ami-gin 0.276s`; `ok github.com/amikos-tech/ami-gin/cmd/gin-index 1.047s [no tests to run]`. | ✓ PASS |
| Alias routing, representation metadata, and DERIVE-04 acceptance cluster (Plan 09-02 / 09-03) | `go test ./... -run 'Test(QueryAliasRoutingUsesAsWrapper|RawPathQueriesRemainDefaultWithDerivedAliases|RepresentationsIntrospectionListsAliases|FinalizePopulatesRepresentationLookup|RepresentationMetadataRoundTrip|RepresentationFailureModeRoundTrip|DecodeRepresentationAliasParity|RepresentationMetadataSectionFollowsConfig|EncodeRejectsNonSerializableRepresentation|DecodeRejectsDuplicateRepresentationAlias|DecodeRejectsRepresentationTargetPathOutOfRange|DecodeRejectsOversizedRepresentationSection|DateTransformerAliasCoverage|NormalizedTextAliasCoverage|RegexExtractAliasCoverage)' -count=1` | Passed on HEAD: `ok github.com/amikos-tech/ami-gin 0.445s`; `ok github.com/amikos-tech/ami-gin/cmd/gin-index 0.745s [no tests to run]`. | ✓ PASS |
| CLI hidden-path suppression remains live | `go test ./... -run 'TestCLIInfoSuppressesInternalRepresentationPaths' -count=1` | Passed on HEAD: `ok github.com/amikos-tech/ami-gin 0.506s [no tests to run]`; `ok github.com/amikos-tech/ami-gin/cmd/gin-index 0.741s`. | ✓ PASS |
| Public docs/examples still teach alias-aware usage without leaking `__derived:` | `rg -n 'gin\.As\(|WithCustomTransformer\(' README.md examples/transformers/main.go examples/transformers-advanced/main.go && ! rg -n '__derived:' README.md examples/transformers/main.go examples/transformers-advanced/main.go` | Passed on HEAD: README and both examples contain `gin.As(...)` and `WithCustomTransformer(...)` guidance, and no public `__derived:` references were returned. | ✓ PASS |
| Date/time example smoke | `go run ./examples/transformers/main.go` | Passed on HEAD and demonstrated raw-string plus `epoch_ms` alias querying. | ✓ PASS |
| Advanced example smoke | `go run ./examples/transformers-advanced/main.go` | Passed on HEAD and demonstrated normalized-text, domain, regex extract, and numeric derived queries. | ✓ PASS |

Observed stdout: `go run ./examples/transformers/main.go` -> `Row groups: [3 4] (expected: [3, 4] - September and December)`
Observed stdout: `go run ./examples/transformers-advanced/main.go` -> `Row groups: [0 2] (expected: [0, 2] - connection errors)`

### Requirements Coverage

| Requirement | Status | Phase 09 Plan Mapping | Evidence |
| --- | --- | --- | --- |
| `DERIVE-01` | ✓ SATISFIED | Plan `09-01` | The additive builder/config regression command passed on HEAD, proving raw-path retention, multiple alias registration, strict companion failure, sibling non-chaining, and internal-path handling. This matches the `09-VALIDATION.md` `09-01-01` and `09-01-02` proof surface. |
| `DERIVE-02` | ✓ SATISFIED | Plan `09-02` | `gin.As(...)`, `RepresentationValue`, `Representations(...)`, `resolvePredicatePath(...)`, fresh-finalize alias lookup, and CLI hidden-path suppression are present in the current tree and covered by the rerun alias-routing command plus `TestCLIInfoSuppressesInternalRepresentationPaths`. |
| `DERIVE-03` | ✓ SATISFIED | Plan `09-02` | `writeRepresentations(...)` and `readRepresentations(...)` persist explicit representation metadata, and the rerun round-trip/security cluster proved alias parity, failure-mode round-trip, trailer ordering, duplicate-alias rejection, target-path bounds checks, oversized-section rejection, and non-serializable encode failures. |
| `DERIVE-04` | ✓ SATISFIED | Plan `09-03` | `TestDateTransformerAliasCoverage`, `TestNormalizedTextAliasCoverage`, and `TestRegexExtractAliasCoverage` all passed on HEAD; README and both examples teach alias-aware raw-plus-companion semantics; both example programs ran successfully with representative alias-aware stdout captured above. |

### Gaps Summary

None. The current-tree proof surface is green for additive builder semantics, explicit alias routing, representation metadata round-trip, CLI hidden-path suppression, and public docs/example behavior. `09-VERIFICATION.md` now matches the live proof surface already encoded in `09-VALIDATION.md` instead of relying on retrospective summary claims alone.

---

_Verified: 2026-04-21T04:51:10Z_  
_Verifier: Codex (phase closeout verification)_
