# Phase 07: Builder Parsing & Numeric Fidelity - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-15T09:37:56Z
**Phase:** 07-Builder Parsing & Numeric Fidelity
**Areas discussed:** Numeric support boundary, Unsupported-number failure policy, Mixed numeric semantics per path, Transformer compatibility

---

## Numeric support boundary

| Option | Description | Selected |
|--------|-------------|----------|
| Exact `int64` + existing `float64` decimals | Integers get exact parsing/fidelity, decimals stay on the current float-backed query/index path. This is the smallest Phase 07 change and matches the current `NumericIndex` shape. | ✓ |
| Exact `int64` only for now | Reject any non-integer JSON numbers in this phase. Simpler semantics, but it narrows today’s behavior. | |
| Broaden to big-int/decimal semantics | Add larger numeric types or exact decimals now. More complete, but it turns Phase 07 into a much larger redesign. | |
| You decide | Delegate this choice to the agent. | |

**User's choice:** Exact `int64` + existing `float64` decimals
**Notes:** Keep the current decimal/query surface; do not broaden Phase 07 into big-int or exact-decimal work.

---

## Unsupported-number failure policy

| Option | Description | Selected |
|--------|-------------|----------|
| Fail the whole document immediately | Return an error with path and offending token/value context. This is the safest match for `BUILD-04`. | ✓ |
| Skip only that field | Keep indexing the rest of the document. More permissive, but it creates partial-index semantics for one document. | |
| Keep the document but disable pruning for that path | Conservative for correctness, but it hides bad data and weakens the explicit-error requirement. | |
| You decide | Delegate this choice to the agent. | |

**User's choice:** Fail the whole document immediately
**Notes:** Unsupported or unrepresentable numbers should not degrade silently or partially index the document.

---

## Mixed numeric semantics per path

| Option | Description | Selected |
|--------|-------------|----------|
| One shared numeric domain | Keep the path queryable as one numeric field, but parse integers exactly first and only widen when the path actually contains decimals. This fits the current single `NumericIndex` model. | ✓ |
| Treat mixed int/float paths as an error | Stricter semantics, but likely too disruptive for real JSON data. | |
| Split into separate int and float index behavior internally | More precise, but significantly more structural work for this phase. | |
| You decide | Delegate this choice to the agent. | |

**User's choice:** One shared numeric domain
**Notes:** Mixed integer/decimal data should remain queryable as numeric data; exact integer parsing happens before any widening or stats decisions.

---

## Transformer compatibility

| Option | Description | Selected |
|--------|-------------|----------|
| Preserve current transformer API and behavior | Parser redesign stays internal; existing transformers keep working without config/query changes. | ✓ |
| Preserve the API, but allow transformer input/output numeric types to shift | Less implementation friction, but it risks subtle breakage in existing transformers and tests. | |
| Use Phase 07 to revise transformer semantics too | Cleaner long-term model, but this expands scope into derived-representation territory before Phase 09. | |
| You decide | Delegate this choice to the agent. | |

**User's choice:** Preserve current transformer API and behavior
**Notes:** Phase 07 should not turn into a transformer-semantics redesign.

---

## the agent's Discretion

- Exact parser implementation replacing `json.Unmarshal(..., &any)`.
- Exact internal representation and widening mechanics between exact integers and today’s float-backed numeric indexes.
- Exact benchmark fixture shape and reporting for `BUILD-05`.
- Exact wording of explicit numeric parse/indexing errors.

## Deferred Ideas

- Evaluate `github.com/lemire/constmap` or a similar immutable lookup structure for `pathLookup` as a separate query-path optimization experiment rather than folding it into Phase 07.
