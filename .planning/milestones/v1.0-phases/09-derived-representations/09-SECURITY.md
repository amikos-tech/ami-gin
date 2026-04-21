---
phase: 09
slug: derived-representations
status: verified
threats_open: 0
asvs_level: 1
created: 2026-04-17
updated: 2026-04-17T08:07:31Z
---

# Phase 09 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| source path -> companion registrations | Duplicate or malformed alias registration can silently redirect derived data to the wrong internal path. | Canonical JSONPath strings, explicit aliases, hidden `__derived:` target paths |
| raw prepared source value -> sibling companions | Companion transforms must derive from the same raw prepared value or registration order changes query semantics. | Prepared scalar/object/array values, transformer outputs |
| companion failure -> transactional builder merge | A failed companion derivation must abort the document before merge or the index drifts from the declared contract. | `FieldTransformer` `(value, ok)` results, staged document state |
| raw path lookup -> alias lookup | Alias-aware predicates must resolve explicitly without changing default raw-path semantics. | Canonical source paths, `gin.As(alias, value)` wrapper values |
| in-memory representation metadata -> encoded bytes | Alias metadata must survive encode/decode without inferring relationships from hidden path naming. | Source path, alias, target path, transformer metadata |
| opaque custom companion -> encode/decode contract | Opaque runtime functions cannot be reconstructed after decode and must not be serialized as if they were portable. | Custom `FieldTransformer` closures, `Serializable` flag |
| implementation -> public docs/examples | Public guidance must reflect additive raw-plus-companion semantics, not the old replacement behavior. | README snippets, runnable examples, CLI info output |
| docs/examples -> user queries | Users must not be taught to query hidden internal paths directly. | Public predicates, alias names, CLI diagnostics |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-09-01 | T/I | config registration | mitigate | Closed: explicit alias validation and duplicate-alias rejection are enforced in [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:405) and [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:415), with coverage in [gin_test.go](/Users/tazarov/experiments/amikos/custom-gin/gin_test.go:1900) and [gin_test.go](/Users/tazarov/experiments/amikos/custom-gin/gin_test.go:1932). | closed |
| T-09-02 | T/I | builder fan-out | mitigate | Closed: companions fan out from one prepared source value in [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:575), raw-plus-companion staging is covered in [gin_test.go](/Users/tazarov/experiments/amikos/custom-gin/gin_test.go:1945), and non-chaining is locked by [transformers_test.go](/Users/tazarov/experiments/amikos/custom-gin/transformers_test.go:578). | closed |
| T-09-03 | T/R | strict failure semantics | mitigate | Closed: strict remains the default failure mode in [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:267), companion failures now abort `AddDocument()` in [builder.go](/Users/tazarov/experiments/amikos/custom-gin/builder.go:575), soft-fail is explicit opt-in via [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:314), and regressions are covered in [transformers_test.go](/Users/tazarov/experiments/amikos/custom-gin/transformers_test.go:481), [transformers_test.go](/Users/tazarov/experiments/amikos/custom-gin/transformers_test.go:538), [phase09_review_test.go](/Users/tazarov/experiments/amikos/custom-gin/phase09_review_test.go:181), and [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:454). | closed |
| T-09-04 | T/I | alias query routing | mitigate | Closed: alias-aware predicates unwrap through `RepresentationValue` in [query.go](/Users/tazarov/experiments/amikos/custom-gin/query.go:88), raw-path lookup remains the default resolution path, and coverage is in [gin_test.go](/Users/tazarov/experiments/amikos/custom-gin/gin_test.go:2158) and [gin_test.go](/Users/tazarov/experiments/amikos/custom-gin/gin_test.go:2192). | closed |
| T-09-05 | T/R | representation metadata | mitigate | Closed: encode writes config before explicit representation metadata in [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:231) and [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:1391), decode reads both sections then rebuilds alias lookup in [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:339) and [gin.go](/Users/tazarov/experiments/amikos/custom-gin/gin.go:770), with coverage in [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:432), [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:454), [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:483), and [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:503). | closed |
| T-09-06 | R | custom companion serialization | mitigate | Closed: non-serializable companions are rejected during encode in [serialize.go](/Users/tazarov/experiments/amikos/custom-gin/serialize.go:1400), with regression coverage in [serialize_security_test.go](/Users/tazarov/experiments/amikos/custom-gin/serialize_security_test.go:520). | closed |
| T-09-07 | I | README / examples | mitigate | Closed: the public contract teaches explicit alias queries and keeps hidden target paths internal in [README.md](/Users/tazarov/experiments/amikos/custom-gin/README.md:149), [examples/transformers/main.go](/Users/tazarov/experiments/amikos/custom-gin/examples/transformers/main.go:94), and [examples/transformers-advanced/main.go](/Users/tazarov/experiments/amikos/custom-gin/examples/transformers-advanced/main.go:194). | closed |
| T-09-08 | R | DERIVE-04 evidence | mitigate | Closed: end-to-end acceptance coverage for date, normalized text, and regex-extract aliases is present in [transformers_test.go](/Users/tazarov/experiments/amikos/custom-gin/transformers_test.go:1388), [transformers_test.go](/Users/tazarov/experiments/amikos/custom-gin/transformers_test.go:1443), and [transformers_test.go](/Users/tazarov/experiments/amikos/custom-gin/transformers_test.go:1508), and CLI hidden-path suppression is covered in [cmd/gin-index/main_test.go](/Users/tazarov/experiments/amikos/custom-gin/cmd/gin-index/main_test.go:404). | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

No accepted risks.

---

## Verification Evidence

- Threat sources audited from [09-01-PLAN.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/09-derived-representations/09-01-PLAN.md:185), [09-02-PLAN.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/09-derived-representations/09-02-PLAN.md:181), and [09-03-PLAN.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/09-derived-representations/09-03-PLAN.md:150).
- No `## Threat Flags` carry-forward sections are present in [09-01-SUMMARY.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/09-derived-representations/09-01-SUMMARY.md), [09-02-SUMMARY.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/09-derived-representations/09-02-SUMMARY.md), or [09-03-SUMMARY.md](/Users/tazarov/experiments/amikos/custom-gin/.planning/phases/09-derived-representations/09-03-SUMMARY.md).
- Builder and strict-failure security regressions passed on 2026-04-17:
  `go test ./... -run 'Test(BuilderIndexesRawAndCompanionRepresentations|BuilderFailsWhenCompanionTransformFails|SiblingTransformersDoNotChain|RebuildPathLookupPreservesInternalRepresentationPaths|ValidateJSONPathRejectsInternalRepresentationPaths)' -count=1`
- Representation metadata and failure-mode round-trip checks passed on 2026-04-17:
  `go test ./... -run 'Test(RepresentationMetadataRoundTrip|RepresentationFailureModeRoundTrip|DecodeRepresentationAliasParity|RepresentationMetadataSectionFollowsConfig|EncodeRejectsNonSerializableRepresentation|DecodeRejectsDuplicateRepresentationAlias|DecodeRejectsRepresentationTargetPathOutOfRange|DecodeRejectsOversizedRepresentationSection)' -count=1`
- DERIVE-04 acceptance coverage passed on 2026-04-17:
  `go test ./... -run 'Test(DateTransformerAliasCoverage|NormalizedTextAliasCoverage|RegexExtractAliasCoverage)' -count=1`
- Public docs/example smoke passed on 2026-04-17:
  `rg -n 'gin\.As\(|WithCustomTransformer\(' README.md examples/transformers/main.go examples/transformers-advanced/main.go && ! rg -n '__derived:' README.md examples/transformers/main.go examples/transformers-advanced/main.go`
  `go run ./examples/transformers/main.go >/dev/null`
  `go run ./examples/transformers-advanced/main.go >/dev/null`
- Full suite passed on 2026-04-17:
  `go test ./... -count=1`

## Security Audit 2026-04-17

| Metric | Count |
|--------|-------|
| Threats found | 8 |
| Closed | 8 |
| Open | 0 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-04-17 | 8 | 7 | 1 | Codex |
| 2026-04-17 | 8 | 8 | 0 | Codex |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** approved 2026-04-17
