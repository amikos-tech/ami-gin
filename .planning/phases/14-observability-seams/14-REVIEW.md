---
status: has_findings
phase: 14
files_reviewed: 22
findings:
  critical: 0
  warning: 2
  info: 2
  total: 4
---

# Code Review — Phase 14: Observability Seams

## WARNING Findings

### W-1: Operation name constants unused — serialize.go and parquet.go use raw strings

**Files:** `serialize.go` (lines 221, 339), `parquet.go` (line 143), `telemetry/attrs.go`

`EncodeWithLevelContext` passes `Operation: "encode"` and `DecodeContext` passes `Operation: "decode"` to `RunBoundaryOperation`. The frozen constants in `telemetry/attrs.go` are `OperationEncode = "serialize.encode"` and `OperationDecode = "serialize.decode"`. The emitted span names don't match — three of four constants are dead code.

`query.go` line 41 correctly references `telemetry.OperationEvaluate`. The fix is to apply the same pattern in serialize.go and parquet.go:

```go
// serialize.go EncodeWithLevelContext
Operation: telemetry.OperationEncode,

// serialize.go DecodeContext
Operation: telemetry.OperationDecode,

// parquet.go BuildFromParquetReaderContext
Operation: telemetry.OperationBuildFromParquet,
```

---

### W-2: EvaluateContext uses manual span lifecycle instead of RunBoundaryOperation — duration metric never recorded

**File:** `query.go` (lines 40–56)

`EvaluateContext` manually starts a span and defers `span.End()`, but unlike every other boundary in `serialize.go` and `parquet.go`, it never calls `RunBoundaryOperation`. As a result:

- `ami_gin.operation.duration` histogram is never recorded for evaluations.
- `ami_gin.operation.failures` counter is never incremented.

Fix: Wrap the core evaluation in `RunBoundaryOperation` (returning `nil` error, since `EvaluateContext` has no error return):

```go
var result *RGSet
_ = telemetry.RunBoundaryOperation(ctx, signals, telemetry.BoundaryConfig{
    Scope:     queryScope,
    Operation: telemetry.OperationEvaluate,
}, func(ctx context.Context) error {
    result = idx.evaluatePredicates(predicates)
    return nil
})
```

---

## INFO Findings

### I-1: go.opentelemetry.io/auto/sdk present as indirect dependency

**File:** `go.mod` (line 47)

```
go.opentelemetry.io/auto/sdk v1.2.1 // indirect
```

`telemetry/doc.go` states the package depends only on OTel API and noop packages. `auto/sdk` is OTel auto-instrumentation SDK weight pulled in transitively. The policy test `TestRootModuleHasNoOtelSdkOrExporterDeps` doesn't match this module name. Consider adding `"go.opentelemetry.io/auto/sdk"` to the disallowed list to gate future occurrences.

---

### I-2: EvaluateContext emits INFO log unconditionally on every non-empty query

**File:** `query.go` (lines 52–55)

Every non-empty predicate evaluation emits an INFO log unconditionally. In high-throughput workloads this produces O(query) INFO entries. The `Debug` helper already guards on `Enabled(LevelDebug)` before constructing args — applying the same guard here would be consistent.

---

## Verified Clean

- Noop-by-default contract: `DefaultConfig()` and `readConfig()` both call `normalizeObservability`
- No nil pointer dereferences: `configLogger`/`configSignals` guard nil cfg; adapters nil-collapse to noop
- No global OTel state mutation: no `otel.Set*` calls anywhere
- No legacy global logger leakage: `SetAdaptiveInvariantLogger` absent from all non-test files
- No backend-type leakage in exported API surface
- Context propagation: all Context-variant functions normalize nil ctx to `context.Background()`
- Panic re-panic in `RunBoundaryOperation`: `span.End()` called before re-panic — no span leaks
- `SerializedConfig` correctly omits Logger and Signals fields
- Adapter nil guards: both slogadapter and stdadapter collapse nil to noop
- No race conditions: no shared mutable state introduced by observability seam
