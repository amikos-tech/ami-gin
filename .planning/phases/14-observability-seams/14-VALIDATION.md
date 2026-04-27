---
phase: 14
slug: observability-seams
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-22
last_audit: 2026-04-22
---

# Phase 14 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing`, repo benchmark harnesses, and source-inspection policy tests |
| **Config file** | `go.mod` / `Makefile` (existing infrastructure) |
| **Quick run command** | `go test -run 'Test(DefaultConfigObservabilityDefaults|WithLoggerRejectsNil|LoggingAttrErrorTypeUnknownFallsBackToOther|SlogAdapterNilFallsBackToNoop|StdAdapterNilFallsBackToNoop|StdAdapterPrefixesSeverity|SignalsDisabledDefaults|SignalsShutdownNilContextNoop|RunBoundaryOperationNoop|EvaluateContextCompatibility|EvaluateContextNilConfigSilent|EvaluateContextPreservesResults|AdaptiveInvariantViolationUsesLoggerSeam|AdaptiveInvariantViolationSilentByDefault|AdaptiveInvariantViolationStillFailsOpen|BuildFromParquetContextCompatibility|BuildFromParquetContextPreservesResults|S3BuildFromParquetContextCompatibility|S3BuildFromParquetContextHonorsCancellationWithStubTransport|EncodeContextCompatibility|DecodeContextCompatibility|SerializationContextRoundTrip|MetadataAndSidecarHelpersUseContextSiblings|InfoLevelAttrAllowlist|InfoLevelEmissionsUseOnlyAllowlistedAttrs|NoLegacyQueryLoggerSurface|NoBackendTypeLeakage|RootModuleHasNoOtelSdkOrExporterDeps|ObservabilityDefaultsSurviveFinalizeAndDecode|ObservabilityEnabledDoesNotChangeFunctionalResults|ParquetAndSerializationObservabilityRoundTrip|EvaluateDisabledLoggingAllocsZero|EvaluateWithTracerWithinBudget)$' -count=1 .` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | quick run ~8s, benchmark smoke ~6s, full suite ~45s |

---

## Sampling Rate

- **After every task commit:** Run the task-specific command from the table below.
- **After every plan wave:** Run the quick run command above.
- **Before `$gsd-verify-work`:** `go test ./... -count=1` plus `go test -run '^$' -bench 'BenchmarkEvaluate(DisabledLogging|WithTracer)$' -benchmem -count=1 .` must be green/current.
- **Max feedback latency:** 60 seconds.

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 14-01-01 | 01 | 1 | OBS-01, OBS-04 | T-14-01 / T-14-02 | repo-owned logger contract uses typed attrs and only the frozen INFO-level keys | unit | `go test -run 'Test(DefaultConfigObservabilityDefaults|WithLoggerRejectsNil|LoggingAttrErrorTypeUnknownFallsBackToOther)$' -count=1 .` | ✅ `logging/logger.go`, ✅ `logging/attrs.go`, ✅ `logging/noop.go` | ✅ green |
| 14-01-02 | 01 | 1 | OBS-03 | T-14-03 | adapter packages are nil-safe and do not leak backend-specific types into the core API | unit | `go test -run 'Test(SlogAdapterNilFallsBackToNoop|StdAdapterNilFallsBackToNoop|StdAdapterPrefixesSeverity)$' -count=1 .` | ✅ `logging/slogadapter/slog.go`, ✅ `logging/stdadapter/std.go` | ✅ green |
| 14-01-03 | 01 | 1 | OBS-05 | T-14-04 | local `Signals` seam remains fail-open and the root module avoids OTEL SDK/exporter deps | unit + source policy | `go test -run 'Test(SignalsDisabledDefaults|SignalsShutdownNilContextNoop|RunBoundaryOperationNoop)$' -count=1 . && ! rg -n 'go\\.opentelemetry\\.io/otel/sdk|otlp' go.mod` | ✅ `telemetry/telemetry.go`, ✅ `telemetry/boundary.go`, ✅ `telemetry/doc.go` | ✅ green |
| 14-01-04 | 01 | 1 | OBS-01, OBS-05 | T-14-05 | `GINConfig` carries logger/signals with silent defaults and safe round-trip behavior | unit | `go test -run 'Test(DefaultConfigObservabilityDefaults|WithLoggerRejectsNil|ConfigRoundTripObservabilityDefaults)$' -count=1 .` | ✅ `gin.go`, ✅ `observability_test.go` | ✅ green |
| 14-02-01 | 02 | 2 | OBS-06, OBS-07 | T-14-06 | query observability is boundary-only and additive; nil-config query path stays silent and safe | unit | `go test -run 'Test(EvaluateContextCompatibility|EvaluateContextNilConfigSilent|EvaluateContextPreservesResults)$' -count=1 .` | ✅ `query.go`, ✅ `query_observability_test.go` | ✅ green |
| 14-02-02 | 02 | 2 | OBS-08 | T-14-07 | adaptive invariant violations use the new logger seam while preserving fail-open behavior | unit + source policy | `go test -run 'Test(AdaptiveInvariantViolationUsesLoggerSeam|AdaptiveInvariantViolationSilentByDefault|AdaptiveInvariantViolationStillFailsOpen)$' -count=1 . && ! rg -n 'SetAdaptiveInvariantLogger|adaptiveInvariantLogger|currentAdaptiveInvariantLogger' query.go gin_test.go` | ✅ `query.go`, ✅ `query_observability_test.go` | ✅ green |
| 14-02-03 | 02 | 2 | OBS-02 | T-14-08 / T-14-09 | disabled logging adds zero allocs and disabled tracer overhead stays within budget | benchmark + perf test | `go test -run 'Test(EvaluateDisabledLoggingAllocsZero|EvaluateWithTracerWithinBudget)$' -count=1 . && go test -run '^$' -bench 'BenchmarkEvaluate(DisabledLogging|WithTracer)$' -benchmem -count=1 .` | ✅ `benchmark_test.go`, ✅ `query_observability_test.go` | ✅ green |
| 14-03-01 | 03 | 2 | OBS-06, OBS-07 | T-14-10 | parquet/build observability is additive and boundary-only with wrapper compatibility preserved | unit | `go test -run 'Test(BuildFromParquetContextCompatibility|BuildFromParquetContextPreservesResults)$' -count=1 .` | ✅ `parquet.go`, ✅ `boundary_observability_test.go` | ✅ green |
| 14-03-02 | 03 | 2 | OBS-06, OBS-07 | T-14-11 | S3-backed builds compose caller context with timeout wrappers without widening scope into unrelated S3 API churn | unit | `go test -run 'Test(S3BuildFromParquetContextCompatibility|S3BuildFromParquetContextHonorsCancellationWithStubTransport)$' -count=1 .` | ✅ `s3.go`, ✅ `boundary_observability_test.go` | ✅ green |
| 14-03-03 | 03 | 2 | OBS-06 | T-14-12 | raw serialization has additive context-aware siblings and helper paths still round-trip cleanly | unit | `go test -run 'Test(EncodeContextCompatibility|DecodeContextCompatibility|SerializationContextRoundTrip|MetadataAndSidecarHelpersUseContextSiblings)$' -count=1 .` | ✅ `serialize.go`, ✅ `boundary_observability_test.go` | ✅ green |
| 14-04-01 | 04 | 3 | OBS-04 | T-14-13 | INFO-level attrs are enforced against the frozen allowlist using real emitted attrs | policy test | `go test -run 'Test(InfoLevelAttrAllowlist|InfoLevelEmissionsUseOnlyAllowlistedAttrs)$' -count=1 .` | ✅ `observability_policy_test.go` | ✅ green |
| 14-04-02 | 04 | 3 | OBS-03, OBS-05, OBS-08 | T-14-14 | no legacy logger surface or backend-type leakage survives in the core API or root module deps | policy test | `go test -run 'Test(NoLegacyQueryLoggerSurface|NoBackendTypeLeakage|RootModuleHasNoOtelSdkOrExporterDeps)$' -count=1 .` | ✅ `observability_policy_test.go` | ✅ green |
| 14-04-03 | 04 | 3 | OBS-05, OBS-06 | T-14-15 | finalized/decoded indexes and end-to-end flows remain silent by default and functionally transparent with observability enabled | integration | `go test -run 'Test(ObservabilityDefaultsSurviveFinalizeAndDecode|ObservabilityEnabledDoesNotChangeFunctionalResults|ParquetAndSerializationObservabilityRoundTrip)$' -count=1 .` | ✅ `boundary_observability_test.go`, ✅ `query_observability_test.go` | ✅ green |
| 14-04-04 | 04 | 3 | OBS-02, OBS-04 | T-14-16 | final perf/policy verification surface stays stable and repo-local | benchmark + policy | `go test -run 'Test(InfoLevelAttrAllowlist|NoLegacyQueryLoggerSurface|ObservabilityDefaultsSurviveFinalizeAndDecode|EvaluateDisabledLoggingAllocsZero|EvaluateWithTracerWithinBudget)$' -count=1 . && go test -run '^$' -bench 'BenchmarkEvaluate(DisabledLogging|WithTracer)$' -benchmem -count=1 .` | ✅ `benchmark_test.go`, ✅ `observability_policy_test.go` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠ accepted/manual review*

---

## Wave 0 Requirements

- [x] Existing Go unit-test and benchmark infrastructure already exists.
- [x] Existing `go.mod` / `Makefile` workflow is sufficient; no new test framework install is required.
- [x] Existing parquet/query/serialize fixtures are available to seed representative observability tests.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Root-module dependency review | OBS-05 | The exact shape of `go.mod` additions should be human-reviewed even though grep tests exist | Confirm the final `go.mod` adds only OTel API/noop packages and no SDK/exporters |
| Boundary-only instrumentation review | OBS-06 | Source-level loop placement is easier to audit visually than to prove mechanically in all cases | Inspect `query.go`, `parquet.go`, `serialize.go`, and `s3.go` to ensure new spans/log calls are only at coarse boundaries |

---

## Validation Sign-Off

- [x] All tasks have automated verify or existing-wave-0 infrastructure
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all missing infrastructure references
- [x] No watch-mode flags
- [x] Feedback latency < 60s (quick-run measured ~0.5s, benchmark smoke ~4.4s)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-04-22

---

## Validation Audit 2026-04-22

| Metric | Count |
|--------|-------|
| Tasks audited | 14 |
| Gaps found | 1 |
| Resolved | 1 |
| Escalated | 0 |

**Findings:**

- Row 14-03-02 documented `TestS3BuildFromParquetContextHonorsCancellation` with a `$`-anchored regex, but the actual test is `TestS3BuildFromParquetContextHonorsCancellationWithStubTransport`. The anchor silently skipped the test during the documented quick run while `go test` still exited 0. Fixed by updating the per-task command and the Quick run command to reference the real test name. No code change required; test infrastructure was live the entire time.

**Coverage:** all 14 tasks (OBS-01 through OBS-08) have passing automated tests. Source-policy greps for OTel SDK/OTLP imports in `go.mod` and for legacy `adaptiveInvariantLogger` symbols in `query.go`/`gin_test.go` both return no matches. Benchmark smoke (`BenchmarkEvaluateDisabledLogging`, `BenchmarkEvaluateWithTracer`) runs clean.
