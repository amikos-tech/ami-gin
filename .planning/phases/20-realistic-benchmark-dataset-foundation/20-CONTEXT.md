# Phase 20: Realistic Benchmark Dataset Foundation - Context

**Gathered:** 2026-07-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 20 turns `SEED-001` into usable benchmark/test fixture infrastructure for realistic JSON shapes. It defines the fixture policy, adds default offline smoke-scale data, and prepares benchmark plumbing that later SIMD phases can reuse.

This phase owns DATA-01, DATA-02, and DATA-03 only. It does not implement the SIMD parser, does not add SIMD CI, does not change parser selection behavior, and does not require network access for default tests or benchmarks.

Carrying forward from prior phases:
- Phase 19 locks SIMD as later opt-in work through `NewSIMDParser() (Parser, error)`, `//go:build simdjson`, explicit `WithParser`, and default stdlib behavior.
- Phase 19 locks correctness as the hard stop: SIMD parity failures cannot ship.
- Phase 11 already established the useful pattern: checked-in smoke fixture by default, optional external tiers behind explicit env vars, pinned provenance, and clear row-count/byte-size notes.
- Phase 13 already established parser parity fixtures and encoded-byte goldens. Phase 20 should not replace that system, but its fixture choices should be easy for Phase 22 to reuse if parity coverage expands.

</domain>

<decisions>
## Implementation Decisions

### Dataset Policy

- **D-01:** Use a hybrid fixture policy. Checked-in synthesized smoke fixtures are the required Phase 20 deliverable; exact upstream simdjson examples are optional external/local inputs documented for richer benchmark runs.
- **D-02:** Do not vendor exact upstream JSON rows into the default smoke fixtures. Smoke data should be shaped after simdjson-style examples but generated/synthesized locally, similar to the Phase 11 smoke fixture precedent.
- **D-03:** Default tests and default smoke benchmarks must run offline. Network access, large downloads, and exact upstream datasets require explicit opt-in.
- **D-04:** Fixture provenance must be documented. The docs should state source inspiration, whether rows are synthesized or external, size limits, row counts, license/NOTICE handling, and the env vars needed for optional external tiers.
- **D-05:** If optional exact upstream simdjson examples are supported, treat them like Phase 11 external corpus tiers: local path required, explicit env vars required, and no automatic download during normal `go test` or default benchmarks.

### Smoke Fixture Shape

- **D-06:** Add three focused checked-in JSONL smoke fixtures plus one combined smoke fixture.
- **D-07:** `nested_high_cardinality.jsonl` should be Twitter/GitHub-like: nested objects, repeated top-level fields, high-cardinality IDs/users/repos, and text strings that exercise string, trigram, and HLL paths.
- **D-08:** `mixed_type_arrays.jsonl` should stress array wildcard staging: arrays containing strings, numbers, booleans, nulls, nested objects, empty arrays, and repeated `[*]` paths.
- **D-09:** `number_heavy.jsonl` should stress numeric parsing and indexing: int64 boundaries, overflow-sensitive JSON number literals where supported, decimals, timestamps/epoch fields, and repeated numeric ranges.
- **D-10:** The combined fixture should mix the three focused shapes into one realistic stream. It is useful for end-to-end smoke benchmarks, while the focused fixtures keep failures easy to diagnose.
- **D-11:** Committed JSONL files are the default source of truth. A small generator/refresh helper is allowed if it prevents hand-editing drift, but the checked-in fixture files remain what default tests and smoke benchmarks consume.

### Benchmark Integration

- **D-12:** Extend the existing Phase 11 tier pattern instead of creating a new standalone fixture system.
- **D-13:** Phase 20 benchmark plumbing should have a default smoke tier backed by checked-in files and optional external tiers guarded by explicit env vars.
- **D-14:** Use clear Phase 20-specific env vars for optional external simdjson examples. The planner should choose exact names, following the Phase 11 naming style (`GIN_PHASE11_*`) for discoverability.
- **D-15:** Keep the benchmark mental model simple: small fixture always works; bigger or exact upstream data only runs when the developer opts in.
- **D-16:** Phase 20 should prepare fixtures so Phase 22 can reuse them for stdlib-vs-SIMD comparison, but Phase 20 does not need to wire SIMD parser parity itself.

### the agent's Discretion

- Exact fixture row counts and byte caps, as long as they are smoke-scale, checked into the repo, documented, and large enough to produce multiple row groups.
- Exact file/directory names, as long as they are under `testdata/phase20/` or an equivalently clear phase-scoped location.
- Exact benchmark helper names and projection names, as long as they follow the existing `benchmark_test.go` style and do not add a parallel benchmark framework.
- Whether to add a tiny fixture generator. Keep it only if it reduces maintenance; the best code is still the code we do not have to write.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Scope And Requirements

- `.planning/ROADMAP.md` - Phase 20 goal and success criteria for fixture policy, shape coverage, and offline smoke benchmark behavior.
- `.planning/REQUIREMENTS.md` - DATA-01, DATA-02, and DATA-03 definitions.
- `.planning/PROJECT.md` - v1.3 milestone priority order, SIMD-first scope, correctness-first constraint, and benchmark-backed-change constraint.
- `.planning/STATE.md` - Current milestone state and the note that Phase 20 is ready to plan.
- `.planning/seeds/SEED-001-simdjson-test-datasets.md` - Original seed: use simdjson example datasets to improve realistic testing and benchmarking.

### Prior Phase Context

- `.planning/phases/19-simd-dependency-decision-integration-strategy/19-CONTEXT.md` - SIMD dependency, opt-in API, default stdlib behavior, CI expectations, and hard/soft stop policy.
- `.planning/phases/13-parser-seam-extraction/13-CONTEXT.md` - Parser seam and parity-harness decisions that Phase 22 may reuse with Phase 20 fixtures.
- `.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-BENCHMARK-RESULTS.md` - Prior benchmark reporting and smoke/external corpus precedent.
- `.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-REAL-CORPUS-REPORT.md` - Prior real-corpus provenance and interpretation precedent.

### Current Code Anchors

- `benchmark_test.go` - Existing benchmark suite and Phase 11 real-corpus tier implementation.
- `testdata/phase11/README.md` - Checked-in smoke fixture provenance format and synthesized-from-shape precedent.
- `testdata/phase11/github_archive_smoke.jsonl` - Existing default offline smoke fixture.
- `README.md` - Phase 11 real-corpus workflow docs, env-var pattern, and default/offline benchmark instructions.
- `parser_parity_fixtures_test.go` - Authored parser parity fixture definitions.
- `parser_parity_test.go` - Encoded-byte parser parity harness.
- `testdata/parity-golden/README.md` - Golden-file policy and refresh rules.
- `cmd/gin-index/experiment.go` - Existing JSONL experimentation CLI and row-group ingest behavior that benchmark fixtures may exercise.

### External Dataset References

- `https://github.com/simdjson/simdjson/tree/master/jsonexamples` - Upstream example JSON family that inspired SEED-001.
- `https://github.com/simdjson/simdjson/blob/master/jsonexamples/twitter.json` - Known nested/high-cardinality example referenced by simdjson docs.
- `https://github.com/simdjson/simdjson/blob/master/LICENSE` - Apache-2.0 license file.
- `https://github.com/simdjson/simdjson/blob/master/LICENSE-MIT` - MIT license file.
- `https://simdjson.org/software/` - simdjson quickstart references `jsonexamples/twitter.json`.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- `benchmark_test.go` already has Phase 11 smoke/subset/large tier structs, env-var gates, JSONL/gzip readers, validation helpers, and metric reporting. Phase 20 should extend this pattern.
- `testdata/phase11/README.md` already documents fixture path, source, revision, origin, row count, byte count, and redistribution rationale. Phase 20 should use the same style.
- `parser_parity_fixtures_test.go` and `testdata/parity-golden/` already provide a small-fixture parity model. Phase 20 fixtures can stay benchmark-first but should be shaped so Phase 22 can reuse them.
- `cmd/gin-index/experiment.go` already consumes JSONL, groups row groups, and reports structured ingest failures. Phase 20 JSONL fixtures should be compatible with that flow.

### Established Patterns

- Default benchmark paths must be lightweight and offline.
- External corpus tiers are opt-in through env vars and local paths, not automatic downloads.
- Fixture drift should fail clearly through validation, row/byte counts, or benchmark loader checks.
- Error handling in benchmark helpers should use `github.com/pkg/errors` for context.
- Planning should avoid new infrastructure if existing Phase 11 helpers can be reused or lightly generalized.

### Integration Points

- Add `testdata/phase20/` or equivalent with three focused JSONL fixtures, one combined fixture, and a README documenting policy/provenance.
- Add or extend benchmark helpers in `benchmark_test.go` using the Phase 11 tier style.
- Add README documentation for Phase 20 smoke benchmarks and optional external simdjson examples.
- Optional: add a small generator/refresh helper only if it keeps the checked-in fixture files deterministic and easier to maintain.

</code_context>

<specifics>
## Specific Ideas

- Use four smoke fixture files: `nested_high_cardinality.jsonl`, `mixed_type_arrays.jsonl`, `number_heavy.jsonl`, and `combined.jsonl`.
- Keep the basic rule easy to explain: committed smoke data is always available; exact upstream data is an optional local input.
- Use a Phase 20-specific README table similar to `testdata/phase11/README.md`.
- Use Phase 11-style benchmark naming, for example `BenchmarkPhase20RealisticJSON/tier=smoke/...`, but the planner can pick exact names.

</specifics>

<deferred>
## Deferred Ideas

- Vendoring exact upstream simdjson JSON examples into the repo by default was considered and not selected for Phase 20.
- Wiring SIMD-vs-stdlib parser parity and CI belongs to Phase 22, not Phase 20.

</deferred>

---

*Phase: 20-realistic-benchmark-dataset-foundation*
*Context gathered: 2026-07-21*
