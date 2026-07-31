# Phase 22: SIMD Validation, Benchmarks & CI - Context

**Gathered:** 2026-07-31
**Status:** Ready for planning

<domain>
## Phase Boundary

Prove the SIMD path is correct, measurable, and operationally shippable. Satisfies SIMD-08..11 (ROADMAP Phase 22 success criteria 1-4).

This phase produces **evidence and enforcement infrastructure**. It does NOT change SIMD parsing behavior, does NOT change the parser API, does NOT add a CLI flag, and does NOT alter default stdlib behavior.

**Pre-locked by Phase 19 (`19-SIMD-STRATEGY.md`) — do NOT re-open:**
- Supported platform set: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`.
- Release asset labels: `linux-amd64`, `linux-arm64`, `darwin-amd64`, `darwin-arm64`, `windows-amd64-msvc`.
- Native loading (download, cache, mirror, flock, SHA-256, ABI) is **entirely upstream's**. ami-gin adds only API shape and a friendlier construction-error message. Phase 22 must not re-verify upstream's bootstrap suite from downstream.
- Stop table: non-parity encoded bytes or query results vs stdlib = **HARD** (halt, defer SIMD to v1.4). One of 5 CI platforms failing while others pass = **SOFT** (document as tier 2, manual verification until resolved). Speedup below expectations = "decide on evidence."
- Documented env vars: `PURE_SIMDJSON_LIB_PATH`, `PURE_SIMDJSON_BINARY_MIRROR`, `PURE_SIMDJSON_DISABLE_GH_FALLBACK`, `PURE_SIMDJSON_CACHE_DIR`.

**Pre-locked by Phase 21 (`21-CONTEXT.md`) — do NOT re-open:**
- `//go:build simdjson` isolation; `make simd-isolation-check` enforces that `go list -deps -test ./...` (test deps included) contains no `pure-simdjson` in a default build, and that every importer carries the tag on line 1 exactly.
- D-05's original framing is **superseded in the tree**: `routeSIMDWellFormedFallback` (`parser_simd.go:60`) reconciles well-formed >uint64 integers, so those documents are byte-identical in both soft and hard failure modes. See D-04 below for what actually survives.

**Ground-truth verified during discussion (2026-07-31, measured in a throwaway clone):**
- `parser_simd_integration_test.go:27` **already contains** `TestSIMDParserGoldenAuthoredFixtures`, running `NewSIMDParser()` over all 12 `authoredParityFixtures()` against `testdata/parity-golden/*.bin`. The authored-fixture half of SC#1 is already discharged. `parser_parity_simd_test.go` is not the whole SIMD parity surface.
- **stdlib and SIMD are already byte-identical on all four Phase 20 fixtures** — nested 10,920 B / mixed-arrays 7,478 B / number-heavy 6,432 B / combined 18,309 B; 0.12 s plain, 2.5 s with `-race` including build. Phase 22 is pinning a passing invariant, not discovering one.
- **`go.mod` is on `github.com/amikos-tech/pure-simdjson v0.1.7`, not the `v0.1.4` hardcoded into Phase 19's cache-key pattern.** Every version reference must derive from `go list -m`, never a literal.
- `parser_stdlib.go` is **untagged**, so under `-tags simdjson` both parsers are in scope in one binary. `parser_simd_integration_test.go:216` already compares `stdlibParser{}` against a SIMD parser in a single tagged file. There is no need for two-tag-set invocations anywhere in this phase.
- Two live doc defects exist: `docs/simd-deployment.md:91` links to upstream **`/blob/main/docs/bootstrap.md`** while NOTICE.md pins every upstream link to the effective version tag (`/blob/v0.1.7/...` verified to resolve 200); and nothing fails if `docs/simd-deployment.md` is renamed while `parser_simd.go:43`'s `initializationContext` string keeps naming it.

**Pre-plan de-risking checks (2026-07-31) — these questions are CLOSED, do not re-investigate:**
- **All five v0.1.7 release assets exist**, each with `.sig` + `.pem` alongside a signed `SHA256SUMS`: `libpure_simdjson-{darwin-amd64.dylib, darwin-arm64.dylib, linux-amd64.so, linux-arm64.so}` and `pure_simdjson-windows-amd64-msvc.dll`. D-05's platform set is asset-backed at the current pin; no leg is aspirational.
- **Upstream already validates the identical runner matrix nightly.** `pure-simdjson`'s `public-bootstrap-validation.yml` (cron `23 6 * * *`, `fail-fast: false`) runs exactly `ubuntu-latest` / `ubuntu-24.04-arm` / `macos-15-intel` / `macos-15` / `windows-latest` — the same five labels D-05 locks — and the last three scheduled runs (2026-07-29, 07-30, 07-31) were all green. This is independent confirmation that the labels are current and that bootstrap + native load work on all five. **Residual risk is only whether ami-gin's own tagged suite passes there, which is precisely what D-05's advisory tier absorbs — this does NOT need a spike.**
- **Every sentinel D-07 needs is exported by upstream**, and more besides: `ErrCPUUnsupported`, `ErrABIVersionMismatch`, `ErrChecksumMismatch`, `ErrAllSourcesFailed`, plus `ErrClosed`, `ErrParserBusy`, `ErrInvalidHandle`, `ErrDepthLimitExceeded`, `ErrCapacityLimitExceeded`, `ErrPrecisionLoss`, `ErrNumberOutOfRange`, `ErrWrongType`, `ErrPanic`, `ErrInternal`.
- **D-15's source-of-truth path is reachable.** `go list -m -f '{{.Dir}}'` resolves to the read-only module cache and all four env-var constants are present there. Upstream is on `github.com/ebitengine/purego v0.10.0` and ships a `library_windows.go`, so the Windows loader is a first-class upstream path rather than an untested edge.

</domain>

<decisions>
## Implementation Decisions

### Parity Coverage & Oracle (SIMD-08 / SC#1)

- **D-01: Tagged differential + Evaluate cross-check — no new goldens.** Three layers:
  1. **Authored fixtures keep the existing golden oracle.** `TestSIMDParserGoldenAuthoredFixtures` already does this; extend only if a fixture is missing.
  2. **Phase 20 datasets get a live stdlib-vs-SIMD byte comparison.** A `//go:build simdjson` test iterates `phase20SmokeFixtures` (untagged, `benchmark_test.go:1792`) via `phase20LoadRawJSONL` (`benchmark_test.go:1811`), builds+encodes under both parsers in one process, and asserts byte equality with the existing `assertByteIdentical`.
  3. **Query-result parity.** Reuse the `TestParserParity_EvaluateMatrix` 24-case matrix plus each fixture's `phase20SmokeQuery.predicate`. Requires a ~2-line untagged refactor of `buildEvaluateMatrixIndex` to accept a `WithParser` option.
  - **Rationale:** goldens are for oracles that cannot run in-process (Go's own `regexp` pins RE2 that way *because* RE2 is out-of-process). Both parsers here run in one binary under one build tag, so a live differential is the idiomatic oracle. Checked-in Phase 20 goldens were explicitly rejected — see D-03.
  - Byte-identical encoded indexes provably yield identical `Evaluate` results (every field `Evaluate` reads is serialized or derived from serialized state; `pathLookup` and friends are explicitly non-serialized derived state per `gin.go:75`). The query assertion is therefore defense-in-depth and a directly checkable restatement of SC#1, not new machinery.

- **D-02: Add `FuzzParserParity` — a tagged differential fuzz target, seed corpus executed in CI, real fuzzing on demand.**
  - Seeds drawn from authored fixtures + Phase 20 JSONL lines, committed under `testdata/fuzz/`.
  - **CI runs seeds only.** A Go fuzz target invoked without `-fuzz` executes its seed corpus as an ordinary deterministic test, so this adds near-zero time to the existing tagged run and no new job.
  - Real discovery is manual: `go test -tags simdjson -fuzz=FuzzParserParity`. Any crasher found gets promoted into the committed seed corpus so it becomes a permanent regression.
  - **Rationale:** matches the closest ecosystem precedent — minio/simdjson-go verifies `encoding/json` parity by differential fuzzing rather than goldens. D-01 closes SC#1; D-02 buys discovery of divergences nobody thought to author.

- **D-02a: MANDATORY input guard — skip inputs with array nesting depth > 8 or length > 4096 bytes.** Not optional and not a nicety. Spike 002 (Q5) proved `stageMaterializedValue` is **O(2^depth)** for nested arrays on *both* parser arms, because `builder.go:619-627` recurses twice per element (once as `[i]`, once as `[*]`). A 37-byte depth-18 input costs >3 s of CPU. Without the guard the fuzzer appears to hang; with a loose guard it silently under-fuzzes — measured A/B: depth 12 sat at 0 execs/sec for 30 of 60 seconds and found 21 new interesting inputs, while depth 8 found **85** in 45 seconds.

- **D-02b: The harness reuses ONE parser and needs no poisoned-parser recovery logic.** Construct before `f.Fuzz`, close in `f.Cleanup`. Spike 002 established: (a) the builder's tragic path requires a native *document-close* failure (`parser_simd.go:133-162`), which is unreachable from public-API input and already covered by Phase 21's injected-fake lifecycle tests — 88 swept inputs poisoned nothing; (b) one parser survived 25,200 builder cycles with zero drift, zero failures, and no `PURE_SIMDJSON_WARN_LEAKS` warnings; (c) per-iteration construction also works (~1.1 µs warm, 5,581 full cycles/sec) but buys nothing.

- **D-02c: Classify differential outcomes three ways.** Both arms ingest → assert byte equality; a mismatch is the Phase 19 **HARD stop**. Exactly one ingests → the D-04 documented-exclusion class; record, do not fail. Both reject → agreement. Spike 002's deterministic run over 88 seeds + adversarial inputs returned `agree=88, asymmetry=0, byteDivergence=0`, and ~361,000 fuzz executions produced zero crashers — **day-one expectation is zero divergences**, consistent with this phase pinning a passing invariant.

- **D-03: Do NOT extend the golden corpus to Phase 20 data.** Rejected: +~42 KB of incompressible binary (4.2× the current ~10 KB corpus), and a double-regeneration coupling where re-running `testdata/phase20/generate.go` silently invalidates the goldens. It also would not remove the need for a differential.

- **D-04: The surviving stdlib/SIMD divergence is asserted explicitly, not scoped out.** After `routeSIMDWellFormedFallback`, the only divergence is **failure-layer attribution on malformed documents** (`1e400 garbage` → stdlib reports numeric-layer soft-skip, SIMD reports a parser-layer error). It does not affect encoded bytes — both leave the builder uncommitted — and it is already pinned by `TestSIMDParserMalformedTrailingNumericKnownPolicyAsymmetry` (`parser_simd_integration_test.go:378`). No Phase 20 document triggers it (a scan found only `±9223372036854775807` and `9007199254740993`, all in range).
  - **SC#1 claim wording must be:** "identical encoded indexes and query results for documents that ingest without a parser-layer error," with failure-layer attribution named as an explicit, tested exclusion.
  - This keeps the Phase 19 HARD-stop trigger unambiguous: a byte or `Evaluate` diff halts the phase; a documented layer-attribution diff on malformed input does not.

### CI Matrix Scope & Policy (SIMD-10 / SC#3)

- **D-05: Tiered 5-platform matrix on every PR — 2 required, 3 advisory.**
  - **Required (block merge):** `linux/amd64` (`ubuntu-latest`), `darwin/arm64` (`macos-15`).
  - **Advisory (`continue-on-error: true`):** `linux/arm64` (`ubuntu-24.04-arm`), `darwin/amd64` (`macos-15-intel`), `windows/amd64` (`windows-latest`).
  - `fail-fast: false`.
  - **Rationale:** this encodes Phase 19's SOFT stop directly in YAML — a single-platform failure is demoted to tier 2 rather than blocking merge, which is exactly what the stop table prescribes. All five runner types are free and unmetered on public repos, so cost is not the constraint; flake-per-PR vs trust-per-PR is. The B+C hybrid (adding a nightly full run) was offered and declined.
  - **Runner labels are pinned deliberately:** `macos-13` was retired 2025-12-04, so darwin/amd64 must use `macos-15-intel`; `macos-latest` flips to `macos-26` between 2026-06-15 and 2026-07-15, so darwin/arm64 pins `macos-15` rather than `macos-latest`. **These exact five labels are what upstream's own nightly `public-bootstrap-validation.yml` uses, green as of 2026-07-31** — copy that matrix's label choices rather than re-deriving them.
  - **`darwin/amd64` is documented tier 2 from day one** on the strength of the Aug 2027 Intel-macOS sunset alone.
  - **Accepted risk:** advisory legs can rot unnoticed (yellow-check blindness). Accepted deliberately; no nightly notifier in this phase.

- **D-06: `-race` on the required tier only.** `linux/amd64` + `darwin/arm64` run `-tags simdjson -race`; the three advisory legs run without `-race`. Windows `-race` needs cgo + a C compiler and is the slowest leg by far. Advisory legs still prove the platform loads and passes, which is what tier-2 status asks of them.

- **D-07: Three-state skip/fail guard — this is what SIMD-10 actually requires.** One test helper:
  1. `runtime.GOOS/GOARCH` outside the locked 5 → `t.Skip("pure-simdjson unsupported platform")`.
  2. Supported platform **and** the CI-only required-flag env var is set → `NewSIMDParser()` failure is `t.Fatal`, with `errors.Is` against upstream sentinels surfaced in the failure message. **Verified exported, use this set:** `ErrCPUUnsupported`, `ErrABIVersionMismatch`, `ErrChecksumMismatch`, `ErrAllSourcesFailed`, and additionally `ErrInvalidHandle` and `ErrClosed` (both diagnostic of a load that half-succeeded).
  3. Supported platform without the flag (local dev, air-gapped) → `t.Skip` with the remediation string.
  - This makes "unsupported platform" a legitimate skip and "supported platform but library failed to load" a hard fail, and makes it impossible for CI to go green by skipping everything.

- **D-08: Cache key derives the version from `go list -m`.** Plain `actions/cache` on `PURE_SIMDJSON_CACHE_DIR`, keyed `pure-simdjson-${{ runner.os }}-${{ runner.arch }}-${{ steps.simd-version.outputs.version }}`, with **no `restore-keys`**. The payload is only ~0.5 MB, so the cache buys network-flake resilience rather than speed, and a fuzzy restore would risk loading an ABI-mismatched library. Phase 19's literal `v0.1.4` in the key pattern is superseded — the repo is on v0.1.7 and bumps are frequent.

### Benchmark Deliverable (SIMD-09 / SC#2)

- **D-09: One tagged dual-arm benchmark, both parsers in one process.** A `//go:build simdjson` benchmark file (e.g. `benchmark_simd_test.go`) with sub-benchmark names carrying a `parser=` key alongside the existing `tier=`/`fixture=` convention. Both arms measured in the same run, interleaved — more comparable than two invocations, not less. Passes `make simd-isolation-check` cleanly because the tagged file is excluded from the default build.
  - Comparison via `benchstat -col /parser out.txt` on a **single** output file. No cross-invocation stitching.

- **D-10: Committed evidence documents are the primary deliverable, following the Phase 11 precedent** — a raw-numbers doc with pinned commands/env/machine, plus a synthesized comparison report ending in a ship/defer/narrow recommendation. This is what Phase 19's stop table ("decide on evidence") actually needs a human to be able to read.

- **D-11: Non-gating CI benchmark artifacts on `push: main` + `workflow_dispatch`, `linux/amd64` only.** Uploads raw benchstat output as an artifact for trend data. **Zero PR cost.** Numbers must be explicitly labeled as shared-runner-noisy (GitHub-hosted runner variance is ~5× dedicated hardware) so nobody mistakes a 4% swing for signal. The committed doc stays authoritative.

- **D-12: No benchstat regression gate.** Rejected: needs `-count=10`+ per arm, and on shared runners any threshold below the ~3% natural movement becomes a flake generator. SIMD-09 asks to *report* a delta, not police one. Backlog it if dedicated bench hardware ever exists.

- **D-13: Report both allocation and throughput.** `-benchmem` B/op + allocs/op (SIMD-09's "allocation"/"bytes/op" clause) **and** `b.SetBytes(inputJSONLen)`-derived MB/s input throughput (already used at `benchmark_test.go:4607`), which is SIMD's headline metric and the one that makes a speedup claim legible. The report must state explicitly which is which.

- **D-14: Fixture tiers mirror the existing split.** Checked-in `testdata/phase20/*.jsonl` is the always-on smoke tier and the only tier whose numbers get committed. The external tier stays behind the already-defined `GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL` / `GIN_PHASE20_SIMDJSON_DIR` env vars as a local-run escape hatch.
  - Make target follows the `bench-phase20` precedent: scope with `-bench '^BenchmarkSIMD...$'` and add `-tags simdjson`. A bare `-bench .` under the tag would drag in the entire ~141 KB suite.
  - `COUNT ?= 1` stays the Makefile default; override to `COUNT=10` or higher when producing the committed snapshot.

- **D-20: The one-shared-SIMD-parser benchmark lifecycle is confirmed unbiased — keep it (Spike 003).** The cross-AI review raised that reusing one parser across fixtures, while the stdlib arm has no persistent state, could skew per-fixture evidence with no D-12 threshold to catch it. Measured and disproven on all four smoke fixtures:
  - **No order dependence** — a fixture measured first (fresh parser) vs. last (after three others) differs by ≤0.9% `ns/op` and 0.0% `B/op`/`allocs/op`.
  - **No lifecycle difference** — shared vs. fresh-per-fixture is within ±2.5% across 20 paired runs; `NewSIMDParser`+`Close` costs 828 ns/op, four orders of magnitude below the ingest deltas.
  - **Comparison is fair** — both arms produce byte-identical encoded indexes on every fixture.
  - Consequence: **no change to the benchmark structure.** Cite this evidence rather than re-litigating the lifecycle.

- **D-21: Evidence must state metric semantics explicitly — steady-state, and Go-heap-only `B/op` (Spike 003).** Two labelling obligations on D-10's documents, both measured rather than assumed:
  - **Steady-state, not cold.** `testing`'s chosen `b.N` lands at 61–312 for these fixtures, and first-iteration cost is not measurable above ±12% run-to-run noise. Every committed number is already a steady-state number; say so rather than leaving it ambiguous.
  - **`B/op` counts the Go heap only.** pure-simdjson's parser buffers live in native memory reached through purego, and the exported `Parser` surface (`Parse`/`Close`) exposes no native telemetry — upstream's `NativeAllocStats*` is package-internal. A favourable `B/op` must never be reported as a total-memory win. (Moot for the headline here — SIMD uses *more* Go heap — but the caveat must survive into the document.)

- **D-22: A single-run benchmark number is never authoritative (Spike 003).** A `COUNT=1` run produced a **3× wrong** result in one measured session, later collapsed to ±2.5% by 5 repeats × 4 fixtures. This makes two existing choices load-bearing rather than ceremonial: D-14's `COUNT=10` override for the committed snapshot, and D-11's "shared-runner-noisy, trend only" disclaimer. Neither may be relaxed for convenience.

- **D-23: The ship/defer/narrow recommendation must be authored neutrally — the measured prior is that SIMD loses.** On the Phase 20 smoke tier, SIMD is **1.45×–1.84× slower** than stdlib and allocates **18–36% more Go heap** on byte-identical output. A document-size sweep from 265 B to 262 KB found **no crossover** (1.24×–1.51× throughout), so this is not a small-record artifact. Consequences:
  - D-10's report template must not presuppose a speedup, and D-04's public guidance must claim no performance benefit that Plan 22-06 has not measured.
  - This is what PROJECT.md's correctness → usefulness → performance ordering is for: parity holding while performance does not is an acceptable, publishable outcome.
  - **Backlog, not Phase 22:** *why* SIMD is slower is uninstrumented. The per-scalar-FFI-crossing theory fits the evidence (cost tracks total volume, not document count) but confirming it needs the internal `NativeAllocStats` API or a profile.

### SIMD-11 Guidance Verification (SC#4)

- **D-15: Doc-drift guard is the primary artifact — a make target + Go test extending the `check-notice-version` precedent.** Asserts, against machine-readable sources of truth already on disk:
  - The four env var names, verified against upstream string constants reachable via `go list -m -f '{{.Dir}}'` (`libraryEnvPath` in `library_loading.go`, `mirrorEnvVar`/`disableGHEnvVar` in `internal/bootstrap/bootstrap.go`, `cacheDirEnvVar` in `internal/bootstrap/cache.go`).
  - The `docs/simd-deployment.md` path string hardcoded in `parser_simd.go:43`'s `initializationContext`.
  - README and CHANGELOG cross-references.
  - Upstream links pinned to the effective version tag, matching NOTICE.md's convention.
  - **Use a deliberate allowlist of the four loading vars.** A naive `PURE_SIMDJSON_*` sweep false-fails on the 40+ diagnostic/error-code tokens upstream defines (`WARN_LEAKS`, `DEBUG`, `ERR_*`) that are not loading variables.
  - Wire into `make lint` and the existing `lint` CI job, exactly like `check-notice-version`.

- **D-16: Fold a compile-checked snippet Example into the same guard (not a second mechanism).** A `//go:build simdjson` `Example` **without** an `// Output:` comment is compiled but never run, so it compile-checks `NewSIMDParser` / `CloseableParser` / `WithParser` / `NewBuilder` with no native library needed at test time. The same guard asserts the doc block matches.

- **D-17: One narrow CI step proves the air-gapped recipe — nothing wider.** The `simd` job already fetches the native artifact into `${{ runner.temp }}/pure-simdjson`, so re-running the tagged tests with `PURE_SIMDJSON_LIB_PATH` pointed at that already-fetched library proves the explicit-path recipe end-to-end with **zero extra network**.
  - **Explicitly out of scope:** the mirror / `DISABLE_GH_FALLBACK` / checksum matrix. Phase 19 delegated those to upstream, and asserting upstream error strings from downstream would couple ami-gin CI to text it does not own.

- **D-18: Fix the release surface (`.goreleaser.yml` header).** The generic release header points only at README and never mentions the optional native dependency, so a v1.3 adopter reading release notes gets no signal that SIMD exists or is opt-in. One-line consistency fix; CHANGELOG already links `docs/simd-deployment.md`. Note `builds: skip: true` means there are no release artifacts, so nothing else in `release.yml` is implicated.

- **D-19: Fix `docs/simd-deployment.md:91`** — repoint `/blob/main/docs/bootstrap.md` to the effective version tag, and let D-15's guard keep it pinned.

### Claude's Discretion

- Exact name of the CI-only "SIMD required" env var in D-07 (`AMI_GIN_SIMD_REQUIRED=1` was the research suggestion; planner's call).
- Exact file names for the new tagged test/benchmark files and the committed evidence docs, as long as they follow existing repo conventions and the `//go:build simdjson` line-1 rule.
- Exact benchmark sub-name keys beyond the required `parser=` dimension.
- Seed corpus size and composition for `FuzzParserParity`, as long as it draws from both authored fixtures and Phase 20 lines.
- Section structure of the benchmark evidence documents, following the Phase 11 shape.
- Whether the doc-drift guard lives in one new `_test.go` file or extends `notice_version_guard_test.go`.
- Wording of the `.goreleaser.yml` header addition and the tier-2 platform note.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents (researcher, planner) MUST read these before planning or implementing.**

### Phase specification
- `.planning/ROADMAP.md` §Phase 22 — goal + 4 success criteria
- `.planning/REQUIREMENTS.md` §SIMD Implementation — SIMD-08, SIMD-09, SIMD-10, SIMD-11 definitions
- `.planning/PROJECT.md` §Project Priorities — correctness → usefulness → performance; perf items gated by profiling data (999.5 precedent)

### De-risking evidence — Spike 002 (empirical; read BEFORE implementing D-02)
- `.planning/spikes/002-simd-differential-fuzz-harness/README.md` — measured answers for parser construction cost, poisoned-builder reachability, handle stability across 25,200 cycles, day-one divergence yield (~361k execs, zero crashers), and the unplanned O(2^depth) array-nesting finding with a full depth/time table. Drives D-02a / D-02b / D-02c.
- `.planning/spikes/MANIFEST.md` §From Spike 002 — the five requirements that emerged.
- `.planning/spikes/002-simd-differential-fuzz-harness/harness_test.go` — reference shape for the three-way outcome classification and the `arrayNestingDepth` guard helper.

### De-risking evidence — Spike 003 (empirical; read BEFORE implementing D-09/D-10)
- `.planning/spikes/003-simd-benchmark-parser-reuse/README.md` — measured answers for benchmark parser-lifecycle bias (none), order dependence (none), cold-vs-steady-state (steady-state already), the document-size crossover sweep (no crossover), and the headline stdlib-vs-SIMD deltas. Drives D-20 / D-21 / D-22 / D-23.
- `.planning/spikes/MANIFEST.md` §From Spike 003 — the six requirements that emerged.
- `.planning/spikes/003-simd-benchmark-parser-reuse/harness_test.go` — public-API mirror of `phase20BuildBenchmarkIndex`/`phase20LoadRawJSONL`, and the reason package-internal `_test.go` helpers cannot be imported by an isolated module.
- `.planning/spikes/003-simd-benchmark-parser-reuse/validity_test.go` — the byte-identical-across-arms guard that makes any performance delta trustworthy.

### Load-bearing prior-phase decisions (locked — do NOT reopen)
- `.planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md` — platform set, asset labels, cache-key pattern (version literal superseded, see D-08), upstream-ownership boundary, stop table, Phase 22 contract
- `.planning/phases/19-simd-dependency-decision-integration-strategy/19-CONTEXT.md` — D-01..D-13 (distribution, opt-in API, stop/fallback)
- `.planning/phases/21-simd-parser-adapter/21-CONTEXT.md` — typed sink (D-01), numeric routing (D-02), BIGINT divergence framing (D-05, superseded in tree — see D-04 above), hybrid walk (D-03), CLI deferral (D-04)
- `.planning/phases/20-realistic-benchmark-dataset-foundation/20-CONTEXT.md` — fixture policy, tier pattern, D-16 ("Phase 20 prepares fixtures so Phase 22 can reuse them for stdlib-vs-SIMD comparison")
- `.planning/phases/13-parser-seam-extraction/13-CONTEXT.md` — parser seam contract; the commutative-staging-order property that legitimizes byte-identical parity oracles

### Benchmark-report precedent (read before authoring D-10's docs)
- `.planning/milestones/v1.0-phases/11-real-corpus-prefix-compression-benchmarking/11-BENCHMARK-RESULTS.md` — raw-numbers doc shape: pinned commands, env, machine, revision
- `.planning/milestones/v1.0-phases/11-real-corpus-prefix-compression-benchmarking/11-REAL-CORPUS-REPORT.md` — synthesized Helps / Flat / Recommendation report shape

### Code anchors (current ami-gin tree)
- `parser_simd_integration_test.go:27` — `TestSIMDParserGoldenAuthoredFixtures` (authored-fixture SC#1 half, already green); `:216` — existing in-one-file stdlib-vs-SIMD comparison; `:378` — `TestSIMDParserMalformedTrailingNumericKnownPolicyAsymmetry` (D-04's exclusion)
- `parser_simd.go:43` — `initializationContext` string naming `docs/simd-deployment.md` (D-15 guards it); `:60`/`:80` — `routeSIMDWellFormedFallback` (supersedes D-05's original framing)
- `parser_parity_test.go` — `parityFixture`, `loadGolden`, `assertByteIdentical`, `buildAndEncodeWithParser`, `materializingParser`, `TestParserSeam_EquivalentAcrossParsers`, `TestParserParity_EvaluateMatrix`, `buildEvaluateMatrixIndex` (D-01 adds a `WithParser` param here)
- `parser_parity_fixtures_test.go` — `authoredParityFixtures()` (12 fixtures)
- `parity_goldens_test.go` — `TestRegenerateParityGoldens`
- `parser_stdlib.go` — **untagged**; this is why both arms coexist under `-tags simdjson`
- `benchmark_test.go:1783` `phase20SmokeFixture` / `:1792` `phase20SmokeFixtures` / `:1811` `phase20LoadRawJSONL` / `:1778` `phase20SmokeQuery` — untagged, freely callable from tagged files; `:2630` `BenchmarkPhase20RealisticJSON`; `:4607` `SetBytes` precedent; `:1765` external-tier env vars
- `Makefile` — `check-notice-version` (D-15's template), `simd-isolation-check`, `bench` / `bench-phase20` (D-14's target precedent), `lint`
- `notice_version_guard_test.go` — the repo's established doc-drift-guard idiom
- `.github/workflows/ci.yml` — `simd` job (currently ubuntu-only, no cache, hard-fails) becomes the D-05 matrix; `lint` job gains D-15's step; `build` job runs `simd-isolation-check`
- `.goreleaser.yml` §`release.header` — D-18's one-line fix
- `docs/simd-deployment.md:91` — the unpinned `/blob/main/` link (D-19)
- `gin.go:75` — non-serialized derived state note, supporting D-01's bytes⇒Evaluate argument

### Upstream (`pure-simdjson`, effective version from `go list -m` — currently v0.1.7)
- `library_loading.go` — `libraryEnvPath` constant (`PURE_SIMDJSON_LIB_PATH`)
- `internal/bootstrap/bootstrap.go` — `mirrorEnvVar`, `disableGHEnvVar`
- `internal/bootstrap/cache.go` — `cacheDirEnvVar`
- `https://github.com/amikos-tech/pure-simdjson/blob/v0.1.7/docs/bootstrap.md` — pinned target for D-19
- Upstream `public-bootstrap-validation.yml` — the 5-platform nightly-cron precedent (informative; this phase chose per-PR tiering instead)

### Ecosystem precedent (informative)
- `github.com/minio/simdjson-go` — differential-fuzz parity against `encoding/json` (D-02's model)
- `github.com/ebitengine/purego` — CI matrix with explicit per-leg skips and no Intel-macOS leg (D-05's model)
- `golang.org/x/perf/cmd/benchstat` — `-col` pivoting on `key=value` sub-benchmark names (D-09)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`phase20SmokeFixtures` + `phase20LoadRawJSONL` are untagged** (`benchmark_test.go`) — callable directly from `//go:build simdjson` files. No new fixture loader is needed for D-01 or D-09.
- **`assertByteIdentical` / `buildAndEncodeWithParser`** (`parser_parity_test.go`) — give per-fixture, per-offset failure messages for free.
- **`TestParserParity_EvaluateMatrix` (24 cases) + per-fixture `phase20SmokeQuery.predicate`** — the query-parity assertion in D-01 is assembled from these, not written fresh.
- **`check-notice-version` + `notice_version_guard_test.go`** — a working, CI-wired doc-drift guard the maintainer has already blessed twice; D-15 is the same shape applied to a second doc.
- **`simd` CI job** — already bootstraps the native lib with version derived from `go list -m` and sets mirror/cache env vars. D-05 turns it into a matrix; D-17 adds one step to it.
- **`bench-phase20` target** — the scoping precedent (`-bench '^Name$'`) D-14 follows.

### Established Patterns
- **`key=value` sub-benchmark naming** (`tier=smoke/fixture=%s`) already in use — benchstat's `-col /parser` needs no naming invention, just one more dimension.
- **Build-tagged same-package files, tag on line 1 exactly** — `simd-isolation-check` is stricter than Go itself and audits this.
- **Executable guards over prose promises** — validator markers, NOTICE version guard, `simd-isolation-check`. D-15 is consistent with this taste; a manual checklist would not be.
- **`b.SetBytes` for throughput** (`benchmark_test.go:4607`) and `b.ReportMetric` for custom metrics (Phase 10/11 precedent).

### Integration Points
- **New tagged test file** — Phase 20 differential + Evaluate cross-check (D-01).
- **New tagged fuzz file + `testdata/fuzz/` seeds** — `FuzzParserParity` (D-02).
- **New tagged benchmark file** — dual-arm stdlib/SIMD (D-09).
- **`parser_parity_test.go`** — ~2-line untagged change so `buildEvaluateMatrixIndex` accepts a parser option (D-01).
- **`.github/workflows/ci.yml`** — `simd` job becomes a 5-leg tiered matrix with `actions/cache` (D-05, D-06, D-08); gains the `PURE_SIMDJSON_LIB_PATH` re-run step (D-17); gains a main/dispatch bench-artifact step (D-11); `lint` job gains the doc-guard step (D-15).
- **`Makefile`** — new `bench-simd` target (D-14); new doc-guard target wired into `lint` (D-15).
- **New/extended `_test.go`** — doc-drift guard + tagged compile-checked Example (D-15, D-16).
- **`.goreleaser.yml`**, **`docs/simd-deployment.md`** — D-18, D-19.
- **Two committed evidence docs** under the phase directory (D-10).

### Constraints Enabled By Existing Architecture
- `parser_stdlib.go` being untagged is what makes single-process dual-arm benchmarking and differential parity possible at all. Build tags subtract files from a build; they do not create isolated worlds. `simd-isolation-check` reaches for `go list -deps -test ./...` precisely because the tagged build says nothing about the untagged one.
- Byte-identical encoded indexes imply identical `Evaluate` results (all read state is serialized or derived from serialized state, `gin.go:75`), so the query assertion is a restatement rather than an independent oracle.
- Phase 13's commutative-staging-order proof (`materializingParser`) is what makes byte comparison a legitimate parity oracle across differently-ordered walks.

</code_context>

<specifics>
## Specific Ideas

- **SC#1 claim wording is prescribed** (D-04): "identical encoded indexes and query results **for documents that ingest without a parser-layer error**," with failure-layer attribution on malformed input named as an explicit, tested exclusion. Verification should check this wording, not a looser one.
- **Runner labels are exact and non-negotiable** (D-05): `ubuntu-latest`, `ubuntu-24.04-arm`, `macos-15-intel`, `macos-15` (NOT `macos-latest`), `windows-latest`.
- **No literal version strings anywhere.** Cache key, doc links, and guards all derive from `go list -m`. The repo is on v0.1.7 and the last four commits on this branch were dependency bumps.
- **`-race` split is deliberate** (D-06): required tier only.
- **Fuzz in CI means seeds only** (D-02) — no `-fuzz` flag, no fuzztime, no new job.
- **Bench CI is `push: main` + `workflow_dispatch` only** (D-11) — never on PR.
- **The doc-guard env var check needs a 4-item allowlist**, not a prefix sweep (D-15).
- **CI benchmark numbers must carry a noise disclaimer** in the artifact or step summary (D-11).

</specifics>

<deferred>
## Deferred Ideas

- **Nightly/scheduled full-matrix run with an auto-filed issue notifier** — offered as the B+C hybrid and declined. Revisit if advisory-leg rot becomes visible in practice.
- **benchstat regression gate** (D-12) — blocked on dedicated benchmark hardware; shared GitHub runners cannot support a threshold below their own ~3% noise floor. Belongs with the Phase 25 / 999.5 profiling work if ever.
- **Timed differential fuzzing in CI** (`-fuzz -fuzztime=Nm` on a schedule) — deferred with the nightly-workflow decision.
- **Parser-target registry** (untagged `parityParsers()` + tagged `init()`) — pays off only when a third parser exists. Revisit if an arena/streaming parser lands.
- **Checked-in Phase 20 encoded-byte goldens** (D-03) — revisit only if the Phase 20 fixtures are ever frozen as a permanent compatibility corpus and `generate.go` stops being the source of truth.
- **Promoting `darwin/amd64` out of tier 2** — moot; Intel macOS on GitHub Actions sunsets Aug 2027.
- **Full documented-recipe CI matrix** (mirror, `DISABLE_GH_FALLBACK`, checksum) — out of scope per the Phase 19 ownership boundary. Belongs upstream in `pure-simdjson`.
- **CLI `--parser=simd` flag** — still deferred from Phase 21 D-04; revisit with backlog Phase 999.7 (SIMD activation UX).
- **O(2^depth) array-nesting cost in `stageMaterializedValue`** — discovered by Spike 002 (Q5). `builder.go:619-627` recurses twice per array element (`[i]` and `[*]`), so nested arrays cost 2^depth on the **default** path; a 37-byte depth-18 document burns >3 s of CPU. **Explicitly NOT Phase 22 work** — both parser arms behave identically, so parity (the thing Phase 22 must prove) is unaffected, and absorbing it would be scope creep into an evidence phase. It is an untrusted-input robustness concern for the default ingest path and deserves its own backlog item; the repo already has `serialize_security_test.go` and `security.yml` as precedent for that kind of hardening. Phase 22 only inherits the D-02a input guard.

</deferred>

---

*Phase: 22-simd-validation-benchmarks-ci*
*Context gathered: 2026-07-31*
