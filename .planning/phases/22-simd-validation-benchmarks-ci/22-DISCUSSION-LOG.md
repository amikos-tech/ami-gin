# Phase 22: SIMD Validation, Benchmarks & CI - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-31
**Phase:** 22-simd-validation-benchmarks-ci
**Areas discussed:** Parity coverage & oracle, CI matrix scope & policy, Benchmark deliverable, SIMD-11 verification
**Mode:** advisor (research-backed comparison tables; calibration tier `full_maturity` from `vendor_philosophy: thorough-evaluator`)

---

## Corrections surfaced during research

Recorded because they invalidated the framing the areas were originally presented under:

1. **The SIMD parity surface was larger than a filename grep suggested.** `parser_simd_integration_test.go:27` already contains `TestSIMDParserGoldenAuthoredFixtures` covering all 12 authored fixtures against checked-in goldens. Only Phase 20 dataset coverage and query-result parity were actually missing.
2. **stdlib and SIMD were measured byte-identical on all four Phase 20 fixtures** (0.12 s plain, 2.5 s with `-race`). The phase pins a passing invariant rather than discovering one.
3. **`go.mod` is on `pure-simdjson v0.1.7`**, not the `v0.1.4` literal baked into Phase 19's cache-key pattern.
4. **Phase 21's D-05 was superseded in-tree** by `routeSIMDWellFormedFallback` — well-formed >uint64 integers are reconciled and byte-identical. Only malformed-document failure-layer attribution still diverges.
5. **Two live doc defects found:** `docs/simd-deployment.md:91` links to upstream `/blob/main/` while NOTICE.md pins every link to the version tag; and nothing fails if the doc is renamed while `parser_simd.go:43` keeps naming it.

---

## Parity coverage & oracle

| Option | Description | Selected |
|--------|-------------|----------|
| A. Tagged differential + Evaluate | Authored fixtures keep existing goldens; Phase 20 gets live stdlib-vs-SIMD byte comparison reusing untagged `phase20SmokeFixtures`; plus Evaluate cross-check. No new goldens. | |
| A + D (add fuzz) | Option A plus a tagged `FuzzParserParity` seeded from authored + Phase 20 lines, for discovery of unauthored divergences. Matches minio/simdjson-go's approach. | ✓ |
| C. Extend goldens to Phase 20 | Check in encoded-byte goldens for all four datasets (+~42 KB, 4.2× current corpus). Independent oracle, but double-regeneration coupling with `generate.go`. | |
| B. Parser-target registry | Untagged `parityParsers()` table with tagged `init()` registration. One test body covers all parsers; costs indirection and isolation-check fragility. | |

**User's choice:** A + D — differential as the SC#1 closer, fuzz layered on for discovery.
**Notes:** D-05's surviving divergence (failure-layer attribution on malformed docs) is asserted explicitly rather than scoped out, since it does not touch encoded bytes and is already pinned by an existing test. The SC#1 claim is narrowed in wording, not in dataset coverage. Golden extension (C) was declined on repo-weight and regeneration-coupling grounds; the ecosystem precedent agrees — goldens are conventionally reserved for oracles that cannot run in-process.

---

## CI matrix scope & policy

| Option | Description | Selected |
|--------|-------------|----------|
| B. Tiered per-PR (2 required, 3 advisory) | All 5 legs every PR; linux/amd64 + darwin/arm64 required, other 3 `continue-on-error`. Encodes Phase 19's SOFT stop directly in YAML. | ✓ |
| C. Two-speed (PR fast, nightly full) | linux/amd64 required on PR; full matrix on main + nightly cron + dispatch with issue notifier. What upstream `pure-simdjson` itself ships. | |
| B + C hybrid | Tiered per-PR plus a nightly full run so advisory rot gets surfaced. | |
| A. All 5 required every PR | Maximum rigor; converts Phase 19's documented tier-2 SOFT demotion into a hard merge blocker. | |

**User's choice:** B — tiered per-PR, no nightly.
**Notes:** Advisory-leg rot (yellow-check blindness) accepted as a known risk; the hybrid was offered and declined. Runner labels pinned exactly (`macos-15-intel` since `macos-13` retired 2025-12-04; `macos-15` rather than `macos-latest`, which flips to `macos-26` mid-2026). `darwin/amd64` documented tier 2 from day one on the Aug 2027 Intel-macOS sunset. Cost was not a factor — all five runner types are free and unmetered on public repos.

### Follow-up: `-race` scope

| Option | Description | Selected |
|--------|-------------|----------|
| `-race` on required tier only | linux/amd64 + darwin/arm64 with `-race`; three advisory legs plain. Windows `-race` needs cgo+mingw and is the slowest leg. | ✓ |
| `-race` everywhere | All 5 legs. Max coverage, slowest advisory legs — and slow-plus-ignorable legs rot fastest. | |
| `-race` on linux/amd64 only | Matches today's behavior; cheapest. Builder is single-threaded, so cross-platform race coverage arguably buys little. | |

**User's choice:** Required tier only.

---

## Benchmark deliverable

| Option | Description | Selected |
|--------|-------------|----------|
| b. Bench + committed evidence doc | Tagged dual-arm benchmark (`benchstat -col /parser`), make target, committed results + comparison docs per the Phase 11 precedent. | |
| b + c (docs + CI artifacts) | Option b plus a non-gating CI step uploading benchstat artifacts for trend data, explicitly labeled shared-runner-noisy. | ✓ |
| a. Bench functions only | Tagged benchmark + make target, no committed numbers. Smallest surface; nothing durable for the stop-table decision. | |
| d. benchstat regression gate | Fail CI on perf deltas. Statistically principled but a flake generator below the ~3% shared-runner noise floor. | |

**User's choice:** b + c.
**Notes:** The build-tag obstacle dissolved on inspection — `parser_stdlib.go` is untagged, so both parsers coexist under `-tags simdjson` in one binary, and `parser_simd_integration_test.go:216` already does this. One benchmark, both arms, one process, one benchstat output file. Gate (d) declined: SIMD-09 asks to report a delta, not police one.

### Follow-up: CI benchmark trigger

| Option | Description | Selected |
|--------|-------------|----------|
| main + dispatch, linux/amd64 only | Bench artifacts on `push: main` and `workflow_dispatch`. Zero PR cost; trend data on merges. | ✓ |
| Every PR, linux/amd64 only | Reviewer sees a perf delta pre-merge; adds minutes to every PR and risks noise being read as signal. | |
| Dispatch only | Strictly on-demand; no recurring cost at all. | |

**User's choice:** main + dispatch, linux/amd64 only.

---

## SIMD-11 verification

| Option | Description | Selected |
|--------|-------------|----------|
| B + D + narrow C + E | Doc-drift guard extending `check-notice-version`, with a tagged no-`Output` Example compile-checking the snippets; one CI step re-running tagged tests via `PURE_SIMDJSON_LIB_PATH` against the already-fetched artifact; plus the goreleaser header fix and the `/blob/main` link pin. | ✓ |
| B only (drift guard) | Guard alone, wired into `make lint`. Skips the CI recipe proof and release-header fix. | |
| B + narrow C | Guard plus the near-free `LIB_PATH` CI step; skips the Example and goreleaser change. | |
| A. Manual review checklist | One-time editorial sign-off, re-reviewed by hand at each bump. Zero repo surface. | |

**User's choice:** B + D + narrow C + E — the full composition.
**Notes:** Scoped strictly to ami-gin's ownership boundary. The mirror / `DISABLE_GH_FALLBACK` / checksum recipe matrix stays out: Phase 19 delegated those to upstream, and asserting upstream error strings from downstream would couple ami-gin CI to text it does not own. The narrow C slice costs zero extra network because the `simd` job already fetches the artifact. Option A was rejected on evidence — the last four commits on this branch were dependency bumps, so "re-review at each bump" is a frequent manual cost.

---

## Claude's Discretion

- Exact name of the CI-only "SIMD required" env var (research suggested `AMI_GIN_SIMD_REQUIRED=1`).
- File names for the new tagged test / fuzz / benchmark files and the committed evidence docs.
- Benchmark sub-name keys beyond the required `parser=` dimension.
- Seed corpus size and composition for `FuzzParserParity`.
- Section structure of the benchmark evidence documents (following the Phase 11 shape).
- Whether the doc-drift guard is a new `_test.go` or extends `notice_version_guard_test.go`.
- Wording of the `.goreleaser.yml` header addition and the tier-2 platform note.

## Deferred Ideas

- Nightly/scheduled full-matrix run with an auto-filed issue notifier (offered as B+C, declined).
- benchstat regression gate — blocked on dedicated benchmark hardware.
- Timed differential fuzzing in CI (`-fuzz -fuzztime=Nm` on a schedule).
- Parser-target registry — pays off only when a third parser exists.
- Checked-in Phase 20 encoded-byte goldens — only if those fixtures are ever frozen as a permanent compatibility corpus.
- Full documented-recipe CI matrix (mirror / fallback / checksum) — belongs upstream in `pure-simdjson`.
- CLI `--parser=simd` flag — still deferred from Phase 21 D-04; revisit with backlog Phase 999.7.
