# Phase 17: Failure-Mode Taxonomy Unification - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md - this log preserves the alternatives considered.

**Date:** 2026-04-23
**Phase:** 17-failure-mode-taxonomy-unification
**Areas discussed:** Public API shape, soft-skip semantics, serialization compatibility, test/example surface
**Mode:** Interactive question tool unavailable in this session; workflow fallback selected recommended defaults from the roadmap, prior decisions, and codebase scout.

---

## Public API Shape

| Option | Description | Selected |
|--------|-------------|----------|
| Strict breaking rename | Replace old public transformer-specific type/constants with `IngestFailureMode`, `IngestFailureHard`, and `IngestFailureSoft`; no compatibility aliases. | yes |
| Compatibility aliases | Keep deprecated aliases for old transformer names while adding the new unified names. | |
| Agent decides later | Leave compatibility shape open for planning. | |

**Captured choice:** Strict breaking rename.
**Notes:** PROJECT.md and STATE.md already lock this as a deliberate breaking change for clarity over convenience. `WithTransformerFailureMode` remains as the transformer-specific option, but its parameter type changes to `IngestFailureMode`. New config options are `WithParserFailureMode` and `WithNumericFailureMode`.

---

## Soft-Skip Semantics

| Option | Description | Selected |
|--------|-------------|----------|
| Whole-document soft skip | Soft mode skips the failed document, returns nil, and leaves durable builder state unchanged. | yes |
| Layer-local skip | Preserve current transformer behavior where soft only skips the derived companion representation. | |
| Mixed behavior | Parser/numeric skip whole document; transformer remains companion-only. | |

**Captured choice:** Whole-document soft skip.
**Notes:** This follows the Phase 17 roadmap language directly: soft mode at any layer skips the failing document silently. It intentionally supersedes the current transformer-only soft behavior.

---

## Failure Boundaries

| Option | Description | Selected |
|--------|-------------|----------|
| Data failures only | Soft mode covers ordinary parser/numeric/transformer document failures; parser contract bugs and tragic merge recovery stay hard. | yes |
| Any layer error can soften | Soft mode can swallow all errors from the configured layer, including parser contract violations. | |
| Decide during implementation | Let planner choose exact failure boundaries later. | |

**Captured choice:** Data failures only.
**Notes:** Parser contract violations such as missing or repeated `BeginDocument` are implementation bugs. Validator-missed merge panics remain tragic per Phase 16.

---

## Serialization Compatibility

| Option | Description | Selected |
|--------|-------------|----------|
| Preserve v9 compatibility | Do not serialize parser/numeric modes; accept legacy transformer mode tokens and avoid a format bump unless the implementation deliberately changes on-wire enum strings. | yes |
| Bump format now | Change serialized transformer mode strings to the new names and bump `Version`. | |
| Ignore serialization | Treat the API rename as unrelated to encoded config compatibility. | |

**Captured choice:** Preserve v9 compatibility.
**Notes:** Parser and numeric failure modes are builder-time routing choices and do not affect query behavior after finalization. Transformer representation metadata is already serialized, so existing encoded indexes should keep decoding where possible.

---

## Test And Example Surface

| Option | Description | Selected |
|--------|-------------|----------|
| Per-layer matrix plus example | Add parser/transformer/numeric hard-soft tests and one `examples/failure-modes/main.go` with rejecting vs skipping configs. | yes |
| Minimal rename tests only | Test API rename and one soft path, leaving broader behavior to later phases. | |
| Full property suite expansion | Add new full 1000-document property coverage for every soft layer. | |

**Captured choice:** Per-layer matrix plus example.
**Notes:** This matches FAIL-01/FAIL-02 without turning Phase 17 into another large property-testing phase. Reuse Phase 16 fixtures where cheap.

---

## the agent's Discretion

- Internal skip-control representation: sentinel error, result type, or branch structure.
- Exact helper names and test table layout.
- Exact hard-mode error wording until Phase 18 introduces `IngestError`.

## Deferred Ideas

- Structured `IngestError` and CLI layer grouping stay in Phase 18.
- Telemetry counters for skipped documents are future/on-demand.
