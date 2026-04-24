# Phase 14: Observability Seams - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in `14-CONTEXT.md` — this log preserves the alternatives considered.

**Date:** 2026-04-22
**Phase:** 14-observability-seams
**Areas discussed:** go-wand alignment, carrier shape, serialization observability, legacy logger migration

---

## Reference Model Alignment

| Option | Description | Selected |
|--------|-------------|----------|
| Follow go-wand loosely | Reuse only the high-level ideas, but design a repo-specific logging/telemetry surface from scratch. | |
| Align near-verbatim with go-wand | Use `go-wand` as the canonical shape: split logging vs telemetry, repo-owned context-free `Logger`, `Signals`, adapter subpackages, boundary helper, CLI-owned `FromEnv(...)`, no trace IDs in logs, no global OTel mutation. | ✓ |
| Design a bespoke combined observability surface | Merge logger and telemetry concerns into one `ami-gin`-specific abstraction. | |

**User's choice:** Align near-verbatim with go-wand.
**Notes:** The user explicitly requested that Phase 14 align with `/Users/tazarov/experiments/amikos/go-wand` to ensure a uniform telemetry approach across repos. One repo-specific adaptation remains necessary: unlike go-wand, `ami-gin`'s Phase 14 success criteria forbid pulling OTel SDK/exporter deps into the root module, so exporter bootstrap must live outside the core module boundary even while the API and behavior mirror go-wand.

---

## Carrier Shape

| Option | Description | Selected |
|--------|-------------|----------|
| Exact go-wand boundary options | Add `WithXxxLogger` / `WithXxxTelemetry` at every major boundary. | |
| Carry go-wand contracts through `GINConfig` for build/query/parquet | Keep the go-wand `Logger` + `Signals` contracts, but fit them into `ami-gin`'s existing config-shaped public API. | ✓ |
| One combined observability bundle | Introduce one shared abstraction that hides logging vs telemetry split. | |

**User's choice:** `1B`
**Notes:** This keeps the seam contracts aligned to go-wand while respecting `ami-gin`'s existing `GINConfig`-centric API shape. Raw serialization remains the exception because `Encode` / `Decode` are top-level functions without a natural config carrier.

---

## Serialization Observability

| Option | Description | Selected |
|--------|-------------|----------|
| Add `EncodeContext` / `DecodeContext` siblings | Make raw serialization observable through additive context-aware entry points without globals. | ✓ |
| Observe only higher-level build/parquet paths | Keep raw `Encode` / `Decode` uninstrumented. | |
| Reintroduce package-global serialization hooks | Use globals for raw serialization observability. | |

**User's choice:** `2A`
**Notes:** This preserves compatibility while still making the raw serialization boundary observable. The exact additive signature is left to planning because decode does not have a pre-existing config carrier.

---

## Legacy Logger Migration

| Option | Description | Selected |
|--------|-------------|----------|
| Keep `SetAdaptiveInvariantLogger` as a deprecated bridge | Maintain compatibility for one milestone while routing through the new logger seam internally. | |
| Remove `SetAdaptiveInvariantLogger` in Phase 14 | Move directly to the repo-owned logger seam and leave no dual logger convention behind. | ✓ |

**User's choice:** `3B`
**Notes:** The user preferred one clean logging convention over a compatibility bridge. Phase 14 therefore removes the package-global stdlib logger seam instead of carrying it forward in deprecated form.

---

## Automatically Locked Follow-Through Decisions

- INFO-level attributes stay on the frozen allowlist only; parser identity remains trace/debug-only.
- Logging remains context-free and separate from telemetry; no trace IDs are injected into logs.
- The official Phase 14 adapters are `slog` and stdlib only; `zap` remains out of scope.
- CLI bootstrap and shutdown should mirror go-wand's root-owned pattern when Phase 15 wires the shipped CLI.

## Deferred Ideas

- `zap` adapter — revisit only on explicit demand.
- Per-predicate nested spans or per-row-group telemetry — explicitly rejected for this phase.
- Broader CLI verbosity surface and `--log-level` flags — Phase 15.

