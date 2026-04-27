---
phase: 14
reviewers: [gemini, claude]
reviewed_at: 2026-04-22T07:00:22Z
plans_reviewed: [14-01-PLAN.md, 14-02-PLAN.md, 14-03-PLAN.md, 14-04-PLAN.md]
---

# Cross-AI Plan Review — Phase 14: Observability Seams

## Gemini Review

# Phase 14: Observability Seams — Plan Review

## Summary
The Phase 14 plan package is exceptionally well-structured, comprehensive, and strictly adheres to the project's architectural and dependency constraints. The wave-based approach—laying the core logging/telemetry foundations first, migrating the query and serialization boundaries, and culminating in strict policy and performance verifications—ensures a safe and zero-cost rollout. The plans correctly navigate the complex constraints around `GINConfig` serialization, zero-alloc disabled paths, and isolating OTel SDK dependencies from the core module.

## Strengths
- **Architectural Discipline:** Strict adherence to the `go-wand` reference model and dependency constraints. The decision to limit the core module to OTel API/noop packages is perfectly aligned with the project's goals.
- **Verification-Driven Execution:** Every plan includes dedicated tasks for benchmarking (0 allocs/op, 0.5% tracer overhead budget) and policy enforcement (INFO-level attr allowlist, legacy logger surface guards).
- **API Compatibility:** Excellent use of additive context-aware siblings (`EvaluateContext`, `BuildFromParquetContext`, `EncodeContext`) while preserving legacy functions as silent `context.Background()` wrappers.
- **Fail-Open Preservation:** Correctly identifies that `adaptiveInvariantAllRGs` must continue to return `AllRGs()` despite migrating to the new logger seam, ensuring no behavioral regressions in pruning.

## Concerns
- **MEDIUM: Serialization Option Signatures in 14-03.** Plan 14-03 Task 3 instructs the agent to "Introduce context-aware raw serialization siblings" but omits the explicit instruction to create the `EncodeOption` / `DecodeOption` types defined in the Research document (Pattern 2). Without this explicit instruction, an agent might attempt to pass the logger/signals directly as positional arguments, violating the repo's established functional option pattern.
- **MEDIUM: S3 Context Propagation vs `io.ReaderAt`.** Plan 14-03 Task 2 dictates composing caller context into S3 timeout wrappers. However, `parquet-go` consumes the standard `io.ReaderAt` interface (`ReadAt(p []byte, off int64) (n int, err error)`), which does not accept a `context.Context`. If the `s3ReaderAt` implementation is modified to store the caller's context in the struct to pass it to underlying `GetObject` calls, it will violate standard Go context guidelines.
- **LOW: `Parser.Name()` Integration Omitted.** Decision D-04 explicitly allows using `Parser.Name()` in traces or DEBUG logs, but forbids it from widening the INFO-level vocabulary. While this is in the CONTEXT, the specific implementation detail of actually attaching it to the build boundary span is missing from the task descriptions.

## Suggestions
1. **Update Plan 14-03 Task 3:** Explicitly mandate the creation of `EncodeOption` and `DecodeOption` functional options (as outlined in Research Pattern 2). Ensure `EncodeContext` and `DecodeContext` use a variadic `opts ...EncodeOption/DecodeOption` signature rather than directly accepting observability primitives.
2. **Refine Plan 14-03 Task 2:** Provide explicit guidance on how to handle context propagation within `s3ReaderAt`. Given the `io.ReaderAt` signature constraint, clarify whether the project accepts the pragmatic exception of storing the `context.Context` inside the `s3ReaderAt` struct upon initialization, or if `ReadAt` should continue using isolated, timeout-bounded background contexts.
3. **Update Plan 14-03 Task 1:** Add a bullet point instructing the agent to extract `config.ParserName` (or similar) and attach it to the build/parquet span as an attribute, while explicitly guarding against emitting it via INFO-level logs.

## Risk Assessment
**LOW.** The overall strategy is robust and heavily de-risked by the strong verification layer defined in Plan 14-04. The identified concerns are primarily implementation nuances that a senior engineer or agent might naturally resolve, but codifying them in the plan tasks will prevent API style drift and ensure flawless execution.

---

## Claude Review

# Phase 14 Peer Review: Observability Seams

## Summary

Phase 14 is a well-scoped plan to introduce backend-neutral logger and telemetry seams while preserving the v1.0 API surface and the root-module dependency boundary. Work is split into a clean Wave 1 foundation (`14-01`), two parallelizable Wave 2 boundary migrations (`14-02` query, `14-03` parquet/serialize/S3), and a Wave 3 verification/policy plan (`14-04`). Coverage of OBS-01..08 is explicit and traceable through the `must_haves` blocks. The research is exceptionally strong — the temp go-module experiment that proved nested-helper modules still leak transitive deps into the root `go.mod` is the kind of evidence that would otherwise have surfaced as a late surprise. The remaining concerns are mostly around CI test stability, a few edge cases around `EncodeWithLevel`, and the exact mechanism by which "no backend leakage" is policed.

## Strengths

- The dependency-boundary discipline (D-05) is protected at multiple layers: locked in CONTEXT, re-verified by a real go-module experiment in RESEARCH, enforced by `go.mod` grep in `14-01` Task 3 verification, and re-asserted as a regression guard in `14-04` Task 2.
- `GINConfig` is reused as the carrier for build/query/parquet (D-07) and the narrow exception for raw serialization (D-11) is justified explicitly — the plan does not invent a second observability container just because the third path is awkward.
- Nil-config safety is treated as a first-class requirement (D-09 → `14-01` defaulting helpers → `14-02` `EvaluateContextNilConfigSilent` test). This is the kind of trap that almost always lands as a runtime panic in early adopters.
- The frozen-vocabulary requirement is enforced by capturing-logger tests in `14-04` Task 1 rather than docstring conventions, and the typed `Attr` decision (deviating from go-wand's `...any`) is acknowledged in the assumptions log with rationale.
- The migration of `SetAdaptiveInvariantLogger` is correctly handled as a hard removal rather than a deprecated bridge (D-13), which avoids the dual-logger trap the research called out.
- Boundary-only instrumentation is restated at every plan with concrete don't-instrument call-out for `s3ReaderAt.ReadAt`, predicate loops, and row-group loops.

## Concerns

- **HIGH** — `14-02` Task 3's `BenchmarkEvaluateWithTracer` "≤ 1.005x baseline" gate is likely to flake under CI noise. `runtime.GOMAXPROCS(1)` reduces variance but Go's micro-benchmark noise floor on shared CI runners commonly exceeds 1–3%, and the roadmap success criterion already encodes 0.5% as a merge gate. The plan needs an explicit median-of-N strategy, an env-gated skip on noisy CI, or a relaxed CI threshold paired with a strict local one — otherwise this becomes a perpetual "rerun the benchmark" merge friction source.
- **HIGH** — `14-03` Task 2's `S3BuildFromParquetContextHonorsCancellation` test contract is unspecified. The current `s3.go` uses real AWS SDK clients and `s3ReaderAt`; a cancellation test either needs a fake `S3Client`/`http.RoundTripper`, an in-process MinIO/`httptest` server, or it ends up being a no-op assertion. Without this nailed down the cancellation guarantee is asserted but not proven.
- **MEDIUM** — `14-03` Task 3 lists `EncodeContext` and `DecodeContext` but is silent on `EncodeWithLevel`, even though `14-01` PATTERNS (and the underlying `serialize.go:166-330` reading list) include three-arity encoding. The acceptance criteria allows `EncodeWithLevel` to remain a sibling that bypasses the boundary helper entirely, which would silently exempt the configurable-compression path from observability. Either add `EncodeWithLevelContext` explicitly or document that `EncodeWithLevel` now routes through `EncodeWithLevelContext(ctx.Background(), …)`.
- **MEDIUM** — `14-04` Task 2's "no backend type leakage" guard is implemented via "source inspection over a bounded set of local files (`query.go`, `gin.go`, `go.mod`, and the new observability packages)." A grep over a fixed file list will not catch leakage from any new core file added later — for example, a future `index_metadata.go` that returns `*slog.Logger`. A package-level `go/types` walk over the public API of the root module would be more durable; at minimum the test should glob the root package, not enumerate filenames.
- **MEDIUM** — `14-02` Task 2 removes the legacy invariant logger but the test grep `! rg ... query.go gin_test.go` only covers two files. There is currently `SetAdaptiveInvariantLogger` usage discoverable via grep; if any internal example, benchmark, or doc references it, the migration will look complete but compile-break elsewhere. The check should run over the whole module (`./...`) to catch stragglers.
- **MEDIUM** — D-04 says `Parser.Name()` is restricted to traces/debug, but nothing in `14-02` or `14-03` codifies *where* it is emitted as a span attribute (build boundary? query boundary?). Without an explicit hook in one of the build-side plans, OBS goal #4 from the roadmap ("`Parser.Name()` is reachable from telemetry attribute sites… verified by a unit test") becomes an implicit obligation no plan owns. Worth adding a single line to `14-03` Task 1 acceptance criteria.
- **MEDIUM** — `14-01` Task 4 says decoded indexes "must never leave decoded indexes with an unsafe partially-initialized observability state" but does not specify the mechanism. If `readConfig` returns a `*GINConfig` with zero-value `Logger` and `Signals`, every observability call site must already collapse to noop via `configLogger`/`configSignals` helpers — but that contract lives in `14-02`/`14-03`, not `14-01`. It would be safer for `readConfig` itself (or a post-decode hook) to install noop defaults so downstream paths don't have to remember.
- **LOW** — `boundary_observability_test.go` is shared between `14-03` (writes) and `14-04` (extends). With Wave 2 plans running in parallel, two agents may each create and stage this file with conflicting initial scaffolding. A short note clarifying creation ownership would prevent merge churn.
- **LOW** — `14-02` Task 1 references `path_mode` as a low-cardinality attr but neither the plan nor `14-01`'s `logging/attrs.go` task definition lists its allowed values. Without an enumerated set ("raw" | "derived" | "adaptive_hot" | …), the allowlist test in `14-04` Task 1 cannot detect cardinality blow-up where the *key* is allowlisted but the *values* are not.
- **LOW** — There is no plan-level treatment of consumer-facing migration notes for `SetAdaptiveInvariantLogger` removal. Even if the API was effectively internal, a one-line CHANGELOG/UPGRADING entry should land alongside this phase since v1.1 is consumer-visible.
- **LOW** — `14-03` Task 2 deliberately scopes S3 changes narrowly ("derive timeout contexts from the supplied caller context… do not broaden this into context-aware variants"). Healthy restraint, but the resulting state — only build-path helpers honor caller context, others don't — is asymmetric. Worth a one-line code comment in `s3.go` so a future reader doesn't "fix" the inconsistency.

## Suggestions

- Lock `BenchmarkEvaluateWithTracer` to a median-of-N harness (e.g., `benchstat`-friendly output, ≥ 5 runs, take the median) and gate the 0.5% threshold on a `CI=local` env var or run it as a documented offline check, with a looser inline regression alarm.
- Define `EncodeWithLevelContext` in `14-03` Task 3 explicitly and make `EncodeWithLevel` its `context.Background()` wrapper, mirroring the `Encode`/`Decode` pattern. This keeps the configurable-compression path observable and avoids two compatibility-wrapper conventions.
- Replace the file-list grep in `14-04` Task 2 with a `go/types`-based check that walks every exported identifier in `github.com/amikos-tech/ami-gin` and asserts no return type or parameter type is `*slog.Logger`, `*log.Logger`, or anything in `go.opentelemetry.io/otel/sdk/...`.
- Add a `Parser.Name()` attribution test (probably under `14-03` build boundary) so the roadmap's verifiable success criterion #4 isn't orphaned across plans.
- Consider promoting `configLogger`/`configSignals` (currently sketched as root-package helpers in research) into `logging`/`telemetry` themselves so each consumer doesn't reinvent the nil-collapsing logic.
- Pin the `path_mode` value enumeration in `logging/attrs.go` and add a value-allowlist test variant in `14-04` Task 1 for at least the keys whose cardinality is bounded by design.

## Risk Assessment

- **Highest risk:** the disabled-tracer 0.5% wall-clock gate. Without a noise-tolerant harness this will gate merges on platform variance rather than real regressions, and the research itself flags the lack of an existing dedicated benchmark gate for this dimension.
- **Second-highest risk:** S3 cancellation test fidelity. The plan correctly avoids broadening the S3 API surface, but the cancellation guarantee is the only externally observable behavior change in `14-03` Task 2 and needs a real assertion path to be meaningful.
- **Module-boundary risk: low, well-mitigated.** The temp-module experiment plus the explicit `! rg ... go.mod` verification in both `14-01` and `14-04` make this the best-defended decision in the phase.
- **Behavioral-regression risk: low.** The fail-open `AllRGs()` semantics for invariant violations is preserved (D-14), boundary-only instrumentation eliminates hot-path span explosion, and the additive context siblings keep all existing call sites compiling and behaviorally identical.
- **Scope risk: low.** The plans resist the temptation to also tackle `cmd/gin-index` module split, `--log-level` flag wiring, or context-aware S3 sweeps — each is correctly punted to Phase 15 or out-of-phase. The deferred-ideas list in CONTEXT.md is consistent with what the plans actually leave undone.
- **Migration risk: low-medium.** Removing `SetAdaptiveInvariantLogger` is a real breaking change but appropriate for a v1.1 minor where this entry point is essentially internal; lack of an explicit changelog handoff is the only loose end.

`★ Insight ─────────────────────────────────────`
- The single most important decision in this phase is captured outside the plans themselves: in RESEARCH §Pitfall 1, the temp-module experiment proved that `replace` directives don't shield the root module from a nested helper's transitive dependencies. Without that experiment, `FromEnv` would almost certainly have been attempted in this phase and would have silently violated D-05 as soon as the helper compiled.
- The plan structure follows a well-known pattern for low-risk observability adoption: foundation → independent migrations → policy guards. The "policy guards as a final wave" approach (`14-04`) is what converts "we implemented it correctly today" into "we will detect when someone breaks it tomorrow." That's worth more than any single test in `14-02` or `14-03`.
- The choice to use a typed `Attr` rather than go-wand's `...any` is a small divergence with outsized payoff: it lets the allowlist test enforce keys at compile time on the constructor surface, instead of needing a runtime allowlist that has to keep up with new emission sites.
`─────────────────────────────────────────────────`

---

## Consensus Summary

Overall, the two reviewers agree the Phase 14 plan shape is strong and low-churn, but they also agree that a few execution details should be tightened before implementation starts. The architecture is not the problem; the remaining risk sits in benchmark methodology, raw-serialization API precision, and making the S3/context and decode-default contracts explicit. Net risk reads as **low-medium**: the phase is well designed, but a small number of ambiguous tasks could create avoidable rework or flaky verification.

### Agreed Strengths

- The wave structure is sound: `14-01` establishes the seam, `14-02` and `14-03` migrate independent boundaries, and `14-04` adds durable policy/perf guards.
- The plans respect the root-module dependency boundary and the `go-wand` reference model instead of leaking SDK/exporter types into the core package surface.
- Additive context-aware siblings (`EvaluateContext`, `BuildFromParquetContext`, `EncodeContext`) are the right compatibility strategy for this repo.
- Verification is treated as part of the design, not cleanup: both reviewers called out the allowlist/perf/policy gates as a major strength.
- The fail-open invariant behavior and single-logger migration direction are preserved correctly rather than mixing old and new observability paths.

### Agreed Concerns

- The disabled-tracer performance gate in `14-02` is underspecified and likely too noisy as written. Both reviewers want the benchmark methodology tightened so the 0.5% budget does not become a flaky merge blocker.
- The raw-serialization observability API in `14-03` needs more explicit shape. Gemini wants the option-based contract called out directly; Claude wants the plan to account for `EncodeWithLevel` so the configurable-compression path is not silently excluded.
- The S3 context path in `14-03` needs a clearer contract. Gemini questioned the `io.ReaderAt` context propagation design; Claude questioned how cancellation will be proved in tests. Both point to the same gap: the plan needs a more explicit mechanism and test strategy.
- The `Parser.Name()` trace/debug hook is not actually owned by a task yet. Both reviewers noticed that D-04 allows it, but no plan line clearly assigns the build-side attribution and test.
- Decoded/default observability normalization should be anchored more concretely. Gemini explicitly called out `readConfig`; Claude made the same point in more detail, warning that downstream callers should not have to remember to repair zero-value observability state.

### Divergent Views

- Claude pushed harder on verification durability beyond the shared concerns, especially replacing fixed-file greps with package-wide or `go/types`-based checks for backend-type leakage and legacy logger removal. Gemini did not flag those as problems.
- Gemini stayed focused on API-shape and execution clarity in `14-03`, while Claude spent more attention on CI realism and long-term regression-guard quality.
- Neither reviewer challenged the overall phase sequencing or the core observability architecture; disagreement is mostly about how much additional specificity the plans need before execution.

### Recommended Follow-Ups

- Tighten `14-02` Task 3 with an explicit, noise-tolerant tracer benchmark method.
- Clarify `14-03` Task 3 so the serialization path has a single explicit context-aware API story, including `EncodeWithLevel` and/or option types.
- Clarify `14-03` Task 2's S3 context propagation and cancellation-test strategy before implementation.
- Add an explicit `Parser.Name()` attribution/test hook to the build-side plan.
- Decide whether noop observability defaults should be normalized in `readConfig` itself or by a documented post-decode helper, and state that directly in the plan.
