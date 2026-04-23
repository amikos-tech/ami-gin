---
phase: 17-failure-mode-taxonomy-unification
verified: 2026-04-23T17:05:07Z
status: passed
score: "15/15 must-haves verified"
overrides_applied: 0
requirements:
  - id: FAIL-01
    status: satisfied
    evidence: "IngestFailureMode replaces old public transformer failure-mode symbols, applies to parser/transformer/numeric config, and is documented in CHANGELOG.md."
  - id: FAIL-02
    status: satisfied
    evidence: "WithParserFailureMode and WithNumericFailureMode exist, default to hard, and soft mode skips failing documents without advancing durable builder state."
automated_checks:
  - command: "go test ./... -run 'Test(IngestFailureMode|ValidateIngestFailureMode|ParserFailureMode|TransformerFailureMode|NumericFailureMode|BuilderSoftFailSkipsDocumentWhenConfigured|TransformerFailureModeSoftDiscardsPartiallyStagedDocument|SoftSkippedDocIDCanBeRetriedWithoutPositionConsumption|SoftFailureModesMatchCleanCorpus|AllSoftFailureModesSilentlyDropFailures|RepresentationFailureMode|DecodeLegacyTransformerFailureModeTokens|ReadConfigRejectsUnknownTransformerFailureMode|TransformerFailureModeWireTokensStayV9|AddDocumentPublicFailuresDoNotSetTragicErr|AddDocumentRefusesAfterRecoveredMergePanic)$' -count=1"
    result: passed
  - command: "go test ./examples/failure-modes -run 'TestFailureModesExampleOutput$' -count=1"
    result: passed
  - command: "go run ./examples/failure-modes/main.go"
    result: passed
  - command: "rg -n 'const Version = 9|Version\\s*=\\s*9' gin.go"
    result: passed
  - command: "! rg -n '\\bTransformerFailureStrict\\b|\\bTransformerFailureSoft\\b|\\btype TransformerFailureMode\\b' --glob '*.go'"
    result: passed
  - command: "! sh -c \"rg -n '\\bTransformerFailureMode\\b' --glob '*.go' | rg -v '\\bWithTransformerFailureMode\\b'\""
    result: passed
user_provided_checks:
  - "go test ./..."
  - "make test"
  - "make lint"
  - "go build ./..."
residual_risks:
  - id: WR-01
    severity: advisory
    file: transformer_registry.go
    note: "Decoded/reconstructed regex transformers with negative capture groups can panic during ingest. This is an existing transformer reconstruction validation gap and should be fixed, but it does not invalidate the Phase 17 public taxonomy or configured parser/numeric/transformer soft-skip behavior verified here."
  - id: WR-02
    severity: advisory
    file: transformer_registry.go
    note: "RegexExtractInt can treat empty/sign-only/dot-only captures as numeric zero. This is a transformer parsing correctness follow-up and not a blocker for FAIL-01/FAIL-02."
  - id: WR-03
    severity: advisory
    file: serialize.go
    note: "Oversized config decode errors do not wrap ErrInvalidFormat. This affects decode error classification, not the failure-mode taxonomy goal."
  - id: META-01
    severity: info
    file: .planning/ROADMAP.md
    note: "ROADMAP.md still marks Phase 17 as 3/4 complete and 17-04 unchecked even though the 17-04 summary and artifacts exist."
---

# Phase 17: Failure-Mode Taxonomy Unification Verification Report

**Phase Goal:** Provide one mental model for document ingest failures by unifying the existing transformer-only failure-mode concept into a single `IngestFailureMode` type that applies to parser, transformer, and numeric-promotion layers.  
**Verified:** 2026-04-23T17:05:07Z  
**Status:** passed  
**Re-verification:** No - initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | New `IngestFailureMode` type with hard/soft replaces the old public transformer-only taxonomy. | VERIFIED | `gin.go:281-285` defines `IngestFailureMode`, `IngestFailureHard`, and `IngestFailureSoft`; static old-symbol checks returned no Go-source hits. |
| 2 | `WithTransformerFailureMode` remains and accepts the unified type. | VERIFIED | `gin.go:352-359` declares `func WithTransformerFailureMode(mode IngestFailureMode) TransformerOption`. |
| 3 | Parser and numeric config knobs exist and default to hard. | VERIFIED | `gin.go:393-397` adds `ParserFailureMode` and `NumericFailureMode`; `gin.go:430-447` defines both options; `gin.go:722-723` defaults both to `IngestFailureHard`. |
| 4 | Invalid parser, transformer, and numeric modes fail validation. | VERIFIED | `gin.go:306-337` has distinct public and transformer-metadata validators; `gin.go:833-837` validates parser/numeric config; `failure_modes_test.go:11-98` covers defaults, invalid values, and legacy token validator asymmetry. |
| 5 | Parser/numeric modes are builder-time only and are not serialized. | VERIFIED | `serialize.go:112-125` `SerializedConfig` has no parser/numeric failure-mode fields; `serialize_security_test.go:799-818` asserts encoded config and representation metadata omit those keys. |
| 6 | v9 transformer metadata compatibility is preserved. | VERIFIED | `serialize.go:127-136` maps public hard/soft to legacy `strict`/`soft_fail`; `serialize.go:1634-1636` and `serialize.go:1773-1779` apply copied wire-token projection; `Version = 9` at `gin.go:31`. |
| 7 | Legacy transformer failure-mode tokens decode and unknown tokens reject. | VERIFIED | `serialize.go:1735-1740` normalizes and registers decoded transformer specs; `serialize_security_test.go:687-744` tests legacy `strict`/`soft_fail` decode and unknown `panic` rejection. |
| 8 | Parser soft mode skips ordinary parser errors without mutating durable state. | VERIFIED | `builder.go:373-383` returns nil for ordinary parser errors when parser mode is soft; `failure_modes_test.go:172-184` verifies nil return, no position consumption, and later dense packing. |
| 9 | Parser contract violations and hard staging errors remain hard under parser soft mode. | VERIFIED | `builder.go:377-404` handles tagged stage errors before parser soft mode and leaves post-parse contract checks hard; `failure_modes_test.go:186-281` covers both behaviors and provenance non-leakage. |
| 10 | Transformer soft mode skips whole failed documents and discards partial staged/raw data. | VERIFIED | `builder.go:578-591` returns `errSkipDocument` on soft transformer failure; `failure_modes_test.go:428-465` and `transformers_test.go:538-583` verify skipped documents do not finalize raw or partial values. |
| 11 | Numeric soft mode covers malformed literals, non-finite native numerics, and mixed-promotion rejection. | VERIFIED | `builder.go:601-643` soft-skips parse/native numeric failures and `builder.go:663-719` soft-skips rejected mixed promotion; `failure_modes_test.go:303-407` covers hard and soft paths. |
| 12 | Soft-skipped documents do not consume positions, can be retried, and do not swallow tragic merge recovery. | VERIFIED | `builder.go:769-787` commits bookkeeping only after successful merge; `builder.go:790-800` keeps merge recovery tragic; `failure_modes_test.go:283-301` and `failure_modes_test.go:389-407` verify retry/dense packing and tragic recovery. |
| 13 | Soft mode across all layers silently drops configured failures and matches a clean-only corpus. | VERIFIED | `failure_modes_test.go:482-506` asserts full attempted corpus bytes equal clean-only corpus; `failure_modes_test.go:508-552` covers all-soft silent-drop behavior. |
| 14 | Breaking rename is documented. | VERIFIED | `CHANGELOG.md:3-9` contains the Unreleased migration note from old public symbols to `IngestFailureMode` / `IngestFailureHard` / `IngestFailureSoft`. |
| 15 | Public example demonstrates hard rejection and soft skipping with deterministic output. | VERIFIED | `examples/failure-modes/main.go:46-112` implements hard and soft configs; `examples/failure-modes/main_test.go:9-30` asserts exact three-line output. |

**Score:** 15/15 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `gin.go` | Unified type, constants, parser/numeric config, validators, default hard mode | VERIFIED | `type IngestFailureMode string`, hard/soft constants, config fields, and options exist and are validated. |
| `transformer_registry.go` | Transformer metadata uses unified mode type | VERIFIED | `TransformerSpec.FailureMode IngestFailureMode` at `transformer_registry.go:47-52`. |
| `builder.go` | Parser/transformer/numeric soft-skip routing before durable commit | VERIFIED | `errSkipDocument`, stage error tagging, parser/numeric/transformer soft branches, and merge tragic recovery are wired. |
| `parser_sink.go` | Error-carried parser callback provenance | VERIFIED | Sink methods wrap staging failures through `tagStageError` at `parser_sink.go:42-55`. |
| `serialize.go` | v9 wire-token compatibility and no parser/numeric serialized config | VERIFIED | `SerializedConfig` omits parser/numeric modes; transformer wire-token helper preserves `strict`/`soft_fail`. |
| `failure_modes_test.go` | Per-layer hard/soft/default/atomicity coverage | VERIFIED | Contains default/validation tests plus parser, transformer, numeric, all-soft, and clean-corpus tests. |
| `transformers_test.go` | Transformer whole-document soft-skip regression | VERIFIED | `TestBuilderSoftFailSkipsDocumentWhenConfigured` verifies no position consumption and dense packing. |
| `serialize_security_test.go` | Legacy token decode and wire-format regression coverage | VERIFIED | Contains legacy decode, unknown token rejection, exact `soft_fail`, and omitted parser/numeric key assertions. |
| `CHANGELOG.md` | Breaking rename migration note | VERIFIED | Unreleased section maps old public symbols to new public symbols. |
| `examples/failure-modes/main.go` | Runnable hard-vs-soft example | VERIFIED | Uses parser, numeric, and transformer soft options and prints deterministic outcomes. |
| `examples/failure-modes/main_test.go` | Exact example output regression | VERIFIED | Runs `go run .` and compares exact stdout and empty stderr. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `WithParserFailureMode` | `GINConfig.ParserFailureMode` | Validated option assignment | WIRED | `gin.go:430-447` validates and stores normalized public values. |
| `WithNumericFailureMode` | `GINConfig.NumericFailureMode` | Validated option assignment | WIRED | `gin.go:440-447` validates and stores normalized public values. |
| `WithTransformerFailureMode` | `TransformerSpec.FailureMode` | Registration options and `addRepresentation` | WIRED | `gin.go:352-359` and `gin.go:476-483` store unified transformer modes. |
| Parser errors | Parser soft mode | `AddDocument` parser error branch | WIRED | `builder.go:373-383` distinguishes skip, stage-hard, soft ordinary parser error, then hard fallback. |
| Parser callbacks | `AddDocument` hard stage classification | `tagStageError` / `isStageCallbackError` | WIRED | `parser_sink.go:42-55` and `builder.go:377-379`. |
| Transformer failure | Whole-document skip | `stageCompanionRepresentations` returns `errSkipDocument` | WIRED | `builder.go:584-591`. |
| Numeric ingest failure | Whole-document skip | `NumericFailureMode` checks in numeric staging and validation | WIRED | `builder.go:601-719` and `builder.go:790-795`. |
| Config serialization | v9 transformer tokens | Copy-before-marshal wire-token helper | WIRED | `serialize.go:127-136`, `serialize.go:1634-1636`, `serialize.go:1773-1779`. |
| Example soft config | Public APIs | `WithParserFailureMode`, `WithNumericFailureMode`, `WithTransformerFailureMode` | WIRED | `examples/failure-modes/main.go:75-84`. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|---|---|---|---|---|
| `gin.go` | `ParserFailureMode`, `NumericFailureMode`, transformer `FailureMode` | Public config options and `DefaultConfig` | Yes | FLOWING |
| `builder.go` | Configured failure modes | `GINBuilder.config` after `NewBuilder` normalization | Yes | FLOWING |
| `builder.go` | Soft skip sentinel | Parser/stage/numeric/transformer errors converted to `errSkipDocument` | Yes | FLOWING |
| `serialize.go` | Transformer failure-mode metadata | `GINConfig.representationSpecs` and `GINIndex.representations` | Yes | FLOWING |
| `examples/failure-modes/main.go` | Output row groups | Real builder ingest, finalize, and `Evaluate` call | Yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| Focused Phase 17 behavior tests pass | `go test ./... -run 'Test(IngestFailureMode|ValidateIngestFailureMode|ParserFailureMode|TransformerFailureMode|NumericFailureMode|BuilderSoftFailSkipsDocumentWhenConfigured|TransformerFailureModeSoftDiscardsPartiallyStagedDocument|SoftSkippedDocIDCanBeRetriedWithoutPositionConsumption|SoftFailureModesMatchCleanCorpus|AllSoftFailureModesSilentlyDropFailures|RepresentationFailureMode|DecodeLegacyTransformerFailureModeTokens|ReadConfigRejectsUnknownTransformerFailureMode|TransformerFailureModeWireTokensStayV9|AddDocumentPublicFailuresDoNotSetTragicErr|AddDocumentRefusesAfterRecoveredMergePanic)$' -count=1` | All listed packages passed | PASS |
| Example exact output regression passes | `go test ./examples/failure-modes -run 'TestFailureModesExampleOutput$' -count=1` | `ok github.com/amikos-tech/ami-gin/examples/failure-modes` | PASS |
| Example smoke prints deterministic output | `go run ./examples/failure-modes/main.go` | Printed the expected hard/soft three-line output | PASS |
| Version remains v9 | `rg -n 'const Version = 9|Version\s*=\s*9' gin.go` | `31: Version = 9` | PASS |
| Old public transformer failure constants/type absent | `! rg -n '\bTransformerFailureStrict\b|\bTransformerFailureSoft\b|\btype TransformerFailureMode\b' --glob '*.go'` | No matches | PASS |
| Old public type absent except `WithTransformerFailureMode` name | `! sh -c "rg -n '\bTransformerFailureMode\b' --glob '*.go' \| rg -v '\bWithTransformerFailureMode\b'"` | No matches | PASS |
| Full integration gates | User-provided: `go test ./...`, `make test`, `make lint`, `go build ./...` | Reported passed before verification | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|---|---|---|---|---|
| FAIL-01 | 17-01, 17-02, 17-04 | Unified `IngestFailureMode` replaces `TransformerFailureMode` constants, applies beyond transformer layer, and is documented as a breaking rename. | SATISFIED | `gin.go` defines the new API and old public symbols are absent; `transformer_registry.go` uses the unified type; `serialize.go` preserves v9 wire compatibility; `CHANGELOG.md` documents the migration. |
| FAIL-02 | 17-01, 17-03, 17-04 | Parser/numeric knobs default hard; soft mode skips failing document and returns nil. | SATISFIED | `WithParserFailureMode` and `WithNumericFailureMode` exist and default hard; builder routes parser, transformer, and numeric soft failures to pre-commit nil skips; tests cover per-layer behavior, atomicity, retry/dense packing, and all-soft behavior. |

No orphaned Phase 17 requirements were found in `.planning/REQUIREMENTS.md`; FAIL-01 and FAIL-02 are the only Phase 17 IDs.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|---|---:|---|---|---|
| `serialize.go` | 1807 | `return []RepresentationSpec{}, nil` | Info | Valid empty representation section handling, not a stub. |

No blocker stubs, placeholders, TODO/FIXME markers, console-only handlers, or hardcoded-empty user-facing data paths were found in Phase 17 implementation files.

### Residual Risks

These are advisory code review findings from `17-REVIEW.md`. They are real follow-ups, but they do not contradict FAIL-01/FAIL-02 or the roadmap success criteria verified above.

| ID | File | Risk | Recommendation |
|---|---|---|---|
| WR-01 | `transformer_registry.go:126-129` | Reconstructed regex transformers with negative capture groups can panic during ingest. | Validate `RegexParams.Group >= 0` in regex transformer reconstruction and keep runtime guards defensive. |
| WR-02 | `transformer_registry.go:192-224` | `RegexExtractInt` can treat empty/sign-only/dot-only captures as numeric zero. | Track digit consumption in `parseFloatSimple` and reject strings without digits. |
| WR-03 | `serialize.go:1665-1666` | Oversized config decode errors do not wrap `ErrInvalidFormat`. | Wrap `ErrInvalidFormat` and add a regression test for `errors.Is(err, ErrInvalidFormat)`. |
| META-01 | `.planning/ROADMAP.md` | Roadmap progress still shows Phase 17 as in progress with 17-04 unchecked. | Update planning metadata after this verification if the orchestrator expects roadmap state to match phase artifacts. |

### Human Verification Required

None. This phase is library/API, serialization, tests, and deterministic CLI example output; all required behaviors were verified programmatically.

### Gaps Summary

No blocking gaps found. The phase goal is achieved: callers now have one `IngestFailureMode` mental model across parser, transformer, and numeric ingest failures, with hard defaults, opt-in soft whole-document skips, v9 transformer metadata compatibility, documentation, and an executable example.

---

_Verified: 2026-04-23T17:05:07Z_  
_Verifier: Codex (gsd-verifier)_
