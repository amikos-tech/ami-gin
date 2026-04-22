---
phase: 14
slug: observability-seams
audit_date: 2026-04-22
asvs_level: 2
threats_total: 7
threats_closed: 7
threats_open: 0
status: SECURED
---

# Phase 14 — Security Audit

## Summary

Phase 14 introduces repo-owned `logging` and `telemetry` seams, an additive
context-aware boundary surface (`EvaluateContext`, `EncodeContext`,
`DecodeContext`, `BuildFromParquetContext`), and removes the legacy
`SetAdaptiveInvariantLogger` global. The PLAN.md files do not contain an
explicit `<threat_model>` block, so the register below is derived from plan
constraints, SUMMARY flags, and the project's observability policy. All seven
derived threats are verified CLOSED against the implemented code and guard
tests: INFO-level attrs are frozen to a five-key allowlist enforced against
real emitted attrs; no helper surface exists for high-cardinality user data;
no backend types leak through the exported root-module API; `go.mod` carries
OTel API/noop packages only; legacy query-logger identifiers are gone; and
the logger contract is intentionally context-free so no span/trace IDs are
injected into log records.

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Query boundary (`EvaluateContext`) | Coarse telemetry span + INFO log around predicate evaluation | operation, predicate_op, path_mode, status, error.type — no raw predicate values, paths, or doc IDs |
| Build boundary (`BuildFromParquetContext`) | Coarse span around parquet ingestion | operation/status only; parser identity debug-only, never INFO |
| Serialize boundary (`EncodeContext` / `DecodeContext`) | Coarse span around encode/decode | operation/status/error.type only |
| Logging adapter seam (`logging/slogadapter`, `logging/stdadapter`) | Opt-in subpackages mapping repo Attr → backend | Low-cardinality Attr (Key,Value) pairs only |
| Serialized config on-wire | `writeConfig` / `readConfig` of index metadata | Logger/Signals are NOT serialized; `normalizeObservability` restores silent defaults on decode |

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-14-01 | Information Disclosure | INFO-level log attrs | mitigate | Frozen five-key allowlist in `logging/attrs.go:17-32`; enforced by `TestInfoLevelAttrAllowlist` (observability_policy_test.go:42). Constructors restricted to `AttrOperation/AttrPredicateOp/AttrPathMode/AttrStatus/AttrErrorType`; `path_mode` bounded to `exact`, `bloom-only`, `adaptive-hybrid`. | CLOSED |
| T-14-02 | Information Disclosure | Query-value leakage into logs/traces | mitigate | No helper constructors exist for raw path names, predicate values, doc IDs, RG IDs, or term IDs (verified in `logging/attrs.go`). `TestInfoLevelEmissionsUseOnlyAllowlistedAttrs` (observability_policy_test.go:112) captures real attrs from `EvaluateContext` and asserts every emitted key is in the allowlist. `query.go:57-59` emits only operation+status; `query.go:232-234` emits operation+path_mode. | CLOSED |
| T-14-03 | DoS (cardinality explosion) | Metric / trace label cardinality | mitigate | The same allowlist contains only low-cardinality attrs (operation, predicate_op ∈ operator enum, path_mode ∈ 3 values, status ∈ {ok,error}, error.type ∈ 7 normalized values). `telemetry/boundary.go:82-93` emits exactly `operation`, optional `error.type`, and `status` as span/metric attrs. Error classifier collapses unknown kinds to `other` (`telemetry/boundary.go:102-109` and `logging/attrs.go:63-74`). | CLOSED |
| T-14-04 | API contract / backend leakage | Core API exports | mitigate | `TestNoBackendTypeLeakage` (observability_policy_test.go:214) walks exported root-module decls with `go/parser`+`go/ast` and rejects selector expressions matching `slog.Logger`, `log.Logger`, `otlp`, or `otel/sdk`. Passes against `gin.go`, `query.go`, `parquet.go`, `serialize.go`, `s3.go`. | CLOSED |
| T-14-05 | Supply-chain surface | Root module dependencies | mitigate | `TestRootModuleHasNoOtelSdkOrExporterDeps` (observability_policy_test.go:293) scans `go.mod` for disallowed substrings (`otel/sdk`, `otel/exporters`, `otlpgrpc`, `otlphttp`, `otlptrace`, `otlpmetric`). `go.mod:47-50` contains only `go.opentelemetry.io/otel{,/metric,/trace}` API packages. Note: `go.opentelemetry.io/auto/sdk v1.2.1` is a pulled-in indirect from the OTel API itself; it is NOT the SDK module flagged by the guard and is an internal auto-instrumentation helper, not an exporter. | CLOSED |
| T-14-06 | Legacy API revival | query.go / root package | mitigate | `TestNoLegacyQueryLoggerSurface` (observability_policy_test.go:156) AST-walks every non-test `.go` file and rejects `SetAdaptiveInvariantLogger` / `adaptiveInvariantLogger` identifiers. Implementation source carries neither (only test file references them as forbidden strings). query.go no longer imports stdlib `log`. | CLOSED |
| T-14-07 | Information Disclosure | Trace ID ↔ log correlation | mitigate | `logging/logger.go:13-20` defines the contract as context-free; `Log(Level, string, ...Attr)` takes no `context.Context`. `logging/slogadapter/slog.go:30,41` pass `context.Background()` to slog explicitly (no caller-span context propagation). `logging/stdadapter/std.go` never touches span/trace APIs. `logging/noop.go:24` is a no-op. `telemetry/boundary.go` records span-side attrs only; it does not mutate log records with trace IDs. No `TraceID` / `SpanContextFromContext` / `span.SpanContext()` reference found across `logging/**`, `telemetry/**`, `query.go`, `parquet.go`, `serialize.go`. | CLOSED |

## Accepted Risks Log

No accepted risks.

## Unregistered Threat Flags

All four plan summaries (`14-01` through `14-04`) declare "Threat Flags: None".
The audit independently verified — via guard tests and code inspection — that
no new network endpoints, auth paths, or schema boundaries were introduced
that warrant registration beyond T-14-01 through T-14-07.

Notable narrow exception explicitly reviewed:

- `s3ReaderAt.parentCtx` field (`s3.go`) — documented scoped exception to the
  no-context-in-struct guideline. Reader is private, short-lived, one-per-build.
  Does not widen the S3 security surface; cancellation tests (`TestS3BuildFromParquetContextHonorsCancellationWithStubTransport`) pass.
  Not a new threat.

## Verification Evidence

Guard test results (executed 2026-04-22):

```
$ go test -run '^TestInfoLevelAttrAllowlist$|^TestInfoLevelEmissionsUseOnlyAllowlistedAttrs$|^TestNoLegacyQueryLoggerSurface$|^TestNoBackendTypeLeakage$|^TestRootModuleHasNoOtelSdkOrExporterDeps$' -count=1 .
ok  	github.com/amikos-tech/ami-gin	0.590s
```

Static inspection for T-14-07 (trace ID injection) — no matches across
observability packages:

```
$ grep -rn "span.SpanContext\|trace.SpanContextFromContext\|TraceID" logging/ telemetry/ query.go parquet.go serialize.go
(no output)
```

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-04-22 | 7 | 7 | 0 | gsd-security-auditor |

## Security Audit 2026-04-22

| Metric | Count |
|--------|-------|
| Threats found | 7 |
| Closed | 7 |
| Open | 0 |

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented (none)
- [x] `threats_open: 0` confirmed
- [x] `status: SECURED` set in frontmatter

**Approval:** verified 2026-04-22
