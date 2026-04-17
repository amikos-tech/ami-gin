---
status: complete
phase: 09-derived-representations
source: [09-01-SUMMARY.md, 09-02-SUMMARY.md, 09-03-SUMMARY.md]
started: 2026-04-17T05:47:38Z
updated: 2026-04-17T07:09:17Z
---

## Current Test

[testing complete]

## Tests

### 1. Additive Raw and Companion Builder Semantics
expected: Run `go test ./... -run 'Test(BuilderIndexesRawAndCompanionRepresentations|BuilderFailsWhenCompanionTransformFails|SiblingTransformersDoNotChain)' -count=1`. The tests should pass, proving raw source-path indexing still works alongside hidden companion representations, failed companion derivation aborts `AddDocument` explicitly, and sibling companions do not chain through one another.
result: pass

### 2. Internal Representation Path Guardrails
expected: Run `go test ./... -run 'Test(RebuildPathLookupPreservesInternalRepresentationPaths|ValidateJSONPathRejectsInternalRepresentationPaths)' -count=1`. The tests should pass, proving internal `__derived:` representation paths survive internal lookup rebuilds while public JSONPath validation rejects them as a supported user-facing query surface.
result: pass

### 3. Explicit Alias Routing and Public Introspection
expected: Run `go test ./... -run 'Test(QueryAliasRoutingUsesAsWrapper|RawPathQueriesRemainDefaultWithDerivedAliases|RepresentationsIntrospectionListsAliases|FinalizePopulatesRepresentationLookup|CLIInfoSuppressesInternalRepresentationPaths)' -count=1`. The tests should pass, proving `gin.As(alias, value)` routes companion queries explicitly, raw-path queries keep their original behavior, alias introspection is deterministic, and CLI diagnostics hide internal `__derived:` paths.
result: pass

### 4. Representation Metadata Serialization and Guardrails
expected: Run `go test ./... -run 'Test(RepresentationMetadataRoundTrip|DecodeRepresentationAliasParity|EncodeRejectsNonSerializableRepresentation|DecodeRejectsDuplicateRepresentationAlias|DecodeRejectsRepresentationTargetPathOutOfRange|DecodeRejectsOversizedRepresentationSection)' -count=1`. The tests should pass, proving v7 encode/decode preserves representation metadata, alias queries still work after decode, non-serializable custom companions are rejected during `Encode()`, and malformed metadata sections fail closed.
result: pass

### 5. Derived Alias Acceptance Coverage
expected: Run `go test ./... -run 'Test(DateTransformerAliasCoverage|NormalizedTextAliasCoverage|RegexExtractAliasCoverage)' -count=1`. The acceptance tests should pass, proving date, normalized-text, and extracted-subfield aliases coexist with raw source queries before and after encode/decode without hidden-path leakage.
result: pass

### 6. Public Examples and Documentation Contract
expected: Run `go run ./examples/transformers/main.go`, `go run ./examples/transformers-advanced/main.go`, and `rg -n 'WithFieldTransformer|__derived:' README.md examples/transformers/main.go examples/transformers-advanced/main.go`. The examples should execute successfully, and the ripgrep command should return no matches so public docs/examples only teach additive alias-aware APIs and do not expose replacement-era or hidden-path contracts.
result: pass

### 7. Full Suite Regression Check
expected: Run `go test ./... -count=1`. The full repository test suite should pass, confirming the derived-representation changes did not regress existing behavior outside the targeted alias/serialization coverage.
result: pass

## Summary

total: 7
passed: 7
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
