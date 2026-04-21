# Phase 06: Query Path Hot Path - Research

**Researched:** 2026-04-14
**Domain:** Query-time path canonicalization and lookup performance in Go
**Confidence:** HIGH

## User Constraints

### Canonical path contract
- **D-01:** Supported equivalent JSONPath spellings must resolve transparently at query time. For supported inputs, callers should be able to use forms such as `$.foo`, `$['foo']`, and `$["foo"]` interchangeably. [VERIFIED: .planning/phases/06-query-path-hot-path/06-CONTEXT.md]
- **D-02:** The index should store and surface one canonical path spelling everywhere. `PathDirectory`, serialized path names, CLI/info output, and tests should converge on the canonical form instead of preserving mixed source spellings. [VERIFIED: .planning/phases/06-query-path-hot-path/06-CONTEXT.md]

### Benchmark strategy
- **D-03:** PATH-03 should use a realistic blended benchmark family: wide path-count stress is the primary lookup pressure, but the same fixture family should also cover equivalent-spelling lookups and the EQ, CONTAINS, and REGEX operators. [VERIFIED: .planning/phases/06-query-path-hot-path/06-CONTEXT.md]
- **D-04:** Benchmark fixtures should prefer a recognizable public log-style corpus as the base shape when practical, then widen or reshape it synthetically as needed to create the high path-count scenarios Phase 06 needs. [VERIFIED: .planning/phases/06-query-path-hot-path/06-CONTEXT.md]

### Query fallback semantics
- **D-05:** After applying canonicalization for supported spellings, unresolved or unknown paths should keep the current safe fallback behavior for this phase: no explicit query-time error, and no pruning beyond what the index can prove safely. [VERIFIED: .planning/phases/06-query-path-hot-path/06-CONTEXT.md]

### the agent's Discretion
- Exact lookup data structure and caching strategy, as long as path resolution becomes constant or logarithmic time and preserves the canonical path contract. [VERIFIED: .planning/phases/06-query-path-hot-path/06-CONTEXT.md]
- Exact canonicalization implementation point(s) between builder, query lookup, and serialization, as long as the public representation is stable and consistent. [VERIFIED: .planning/phases/06-query-path-hot-path/06-CONTEXT.md]
- Exact public benchmark corpus choice, fixture-preparation flow, and synthetic widening mechanics, as long as the benchmark story remains credible and scoped to the hot path. [VERIFIED: .planning/phases/06-query-path-hot-path/06-CONTEXT.md]
- Exact benchmark names, helper layout, and regression thresholds. [VERIFIED: .planning/phases/06-query-path-hot-path/06-CONTEXT.md]

### Deferred Ideas
None — discussion stayed within phase scope. [VERIFIED: .planning/phases/06-query-path-hot-path/06-CONTEXT.md]

## Project Constraints (from AGENTS.md)

- `go build ./...`, `go test -v`, and focused `go test -v -run ...` commands are the repository-declared build/test entry points, so plan verification should stay on plain Go tooling instead of introducing custom harnesses. [VERIFIED: AGENTS.md]
- Supported JSONPath remains limited to `$`, `$.field`, `$['field']`, and `$[*]`; array indices, recursive descent, slices, and filters must stay unsupported in this phase. [VERIFIED: AGENTS.md]
- Query work must preserve the current safe unknown-path behavior unless the phase explicitly changes it, which Phase 06 does not. [VERIFIED: AGENTS.md; VERIFIED: .planning/phases/06-query-path-hot-path/06-CONTEXT.md]
- New code should keep using `github.com/pkg/errors` for error creation/wrapping rather than `fmt.Errorf(... %w ...)`. [VERIFIED: AGENTS.md]
- Configurable behavior should continue to follow the repo's functional-options conventions instead of ad hoc constructor arguments. [VERIFIED: AGENTS.md]
- The phase should stay inside the root `gin` package and existing benchmark/test files unless there is a compelling reason to add new files. [VERIFIED: AGENTS.md; VERIFIED: .planning/codebase/CONVENTIONS.md]

## Summary

Phase 06 is a focused correctness-plus-performance pass on path resolution, not a new query feature. `query.go` still resolves every predicate through `findPath()`, and that function currently linearly scans `idx.PathDirectory` for a string match before every operator dispatch. [VERIFIED: query.go] `builder.Finalize()` sorts path names once, assigns sequential `PathID`s, and materializes `PathDirectory`; `serialize.go` then persists only that directory representation and reconstructs it during `Decode()`. [VERIFIED: builder.go; VERIFIED: serialize.go] That means any fast lookup aid must either be serialized explicitly or rebuilt deterministically from the canonical stored path names after both finalize and decode. [ASSUMED]

The repo already has the key primitive Phase 06 needs: `jsonpath.go` exposes `ValidateJSONPath()`, `ParseJSONPath()`, and `NormalizePath()`, so the supported-path contract does not require a new parser or grammar layer. [VERIFIED: jsonpath.go] The benchmark path also already exists in `benchmark_test.go`, with operator-oriented query benchmarks and helper generators that can be extended instead of replaced. [VERIFIED: benchmark_test.go]

**Primary recommendation:** Canonicalize supported JSONPath spellings before they enter `PathDirectory`, keep `PathDirectory` and serialized path names in canonical form, rebuild a query-only canonical lookup table in both `Finalize()` and `Decode()`, and extend `benchmark_test.go` with wide-path log-style fixtures that cover EQ, CONTAINS, REGEX, and equivalent-spelling lookups. [ASSUMED]

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `NormalizePath()` + `ValidateJSONPath()` | repo-local | Canonicalize and validate supported JSONPath spellings | Existing code already defines the supported path surface and canonical string rendering. `[VERIFIED: jsonpath.go]` |
| Query-only path lookup table (`map[string]uint16` recommended) | Go stdlib | Constant-time lookup from canonical path to `PathID` | Lowest-risk way to satisfy PATH-01 without disturbing sorted `PathDirectory` semantics or adding dependencies. `[ASSUMED]` |
| `testing` benchmarks in `benchmark_test.go` | Go stdlib | Measure hot-path lookup cost and regression risk | The repository already keeps query benchmarks and fixture generators in one place. `[VERIFIED: benchmark_test.go]` |
| `PathDirectory` + decode/finalize rebuild hooks | repo-local | Rebuild derived lookup state after build and deserialization | `Finalize()` and `Decode()` are the two lifecycle points that see the complete path directory. `[VERIFIED: builder.go; VERIFIED: serialize.go]` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sort.SearchStrings` / `sort.Search` | Go stdlib | Fallback logarithmic lookup if planner chooses sorted auxiliary slices over a map | Acceptable if the planner prefers lower memory and still meets PATH-01. `[VERIFIED: query.go; ASSUMED]` |
| Existing trigram and regex literal extraction flow | repo-local | Keep CONTAINS/REGEX operator behavior stable while path lookup changes | Phase 06 is not the time to redesign text pruning. `[VERIFIED: query.go; VERIFIED: regex.go]` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Query-only canonical lookup table | Keep linear scan and rely only on canonical storage | Fails PATH-01 because query-time lookup still scales with `len(PathDirectory)`. `[VERIFIED: query.go; VERIFIED: .planning/REQUIREMENTS.md]` |
| Repo-local canonicalization helpers | New external JSONPath package or custom parser | Adds dependency/behavior churn without solving a documented gap in the current supported surface. `[VERIFIED: jsonpath.go; VERIFIED: AGENTS.md]` |
| Existing benchmark harness | Standalone perf binary or ad hoc script | Splits performance evidence away from the repo's current Go benchmark workflow and makes regression checks harder to keep consistent. `[VERIFIED: benchmark_test.go; VERIFIED: .planning/codebase/TESTING.md]` |

## Architecture Patterns

### Pattern 1: Build-time canonical storage, query-time canonical lookup
**What:** Normalize supported path spellings before path creation in the builder and before lookup in the query path. [VERIFIED: .planning/phases/06-query-path-hot-path/06-CONTEXT.md; VERIFIED: jsonpath.go]  
**When to use:** Any exact supported spelling variants such as `$.foo`, `$['foo']`, and `$["foo"]`. [VERIFIED: .planning/phases/06-query-path-hot-path/06-CONTEXT.md]  
**Example:**
```go
canonical := NormalizePath(inputPath)
pathID, ok := idx.pathLookup[canonical]
```
Source: recommended use of `NormalizePath()` from `jsonpath.go`; derived lookup structure inferred from Phase 06 goals. [VERIFIED: jsonpath.go; ASSUMED]

### Pattern 2: Keep fast lookup derived, not source-of-truth
**What:** Treat `PathDirectory` as the public/source-of-truth path catalog and rebuild any fast lookup aid from it during `Finalize()` and `Decode()`. [VERIFIED: builder.go; VERIFIED: serialize.go; ASSUMED]  
**When to use:** Any optimization that should affect both newly built indexes and decoded indexes without forcing immediate binary-format churn. [ASSUMED]  
**Anti-pattern to avoid:** Storing a fast lookup map only in builder memory, which would make decoded indexes regress to linear scans. [VERIFIED: serialize.go; ASSUMED]

### Pattern 3: Benchmark through the existing query harness
**What:** Extend `benchmark_test.go` with high-path-count fixtures and equivalent-spelling cases for EQ, CONTAINS, and REGEX. [VERIFIED: benchmark_test.go; VERIFIED: .planning/REQUIREMENTS.md]  
**When to use:** Every performance claim for Phase 06. [VERIFIED: .planning/PROJECT.md]  
**Anti-pattern to avoid:** Reporting "faster lookup" from only narrow-path fixtures or only one operator. [VERIFIED: .planning/REQUIREMENTS.md; VERIFIED: .planning/phases/06-query-path-hot-path/06-CONTEXT.md]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Supported JSONPath canonicalization | New parser or bespoke normalization rules | Existing `ValidateJSONPath()`, `ParseJSONPath()`, and `NormalizePath()` helpers | The repo already encodes the supported grammar and canonical dot-notation output. `[VERIFIED: jsonpath.go]` |
| Phase-06-specific benchmark runner | Custom CLI or external benchmark tool | `benchmark_test.go` plus `go test -bench ...` | Existing patterns already organize query benchmarks and helper generators there. `[VERIFIED: benchmark_test.go; VERIFIED: .planning/codebase/TESTING.md]` |
| Regex/text pruning redesign | Alternate regex engine or new n-gram layer | Existing trigram index and regex literal extraction flow | The requirement is path lookup speed plus canonicalization, not new REGEX semantics. `[VERIFIED: query.go; VERIFIED: regex.go]` |

**Key insight:** The lowest-risk plan is to add a derived canonical path lookup boundary around the already-supported path grammar, not to redesign path parsing or operator logic. [ASSUMED]

## Common Pitfalls

### Pitfall 1: Canonicalizing query input but not stored paths
**What goes wrong:** Equivalent spellings still land in different `PathDirectory` entries, so PATH-02 fails even if query lookup normalizes the predicate. [VERIFIED: .planning/REQUIREMENTS.md; VERIFIED: builder.go]  
**Why it happens:** Builder paths are currently materialized from raw traversal strings, and `Finalize()` preserves those names after sorting. [VERIFIED: builder.go]  
**How to avoid:** Apply canonicalization before `getOrCreatePath()` stores new path keys, and assert canonical path names in `PathDirectory`, serialization round-trips, and any info/reporting surfaces. [ASSUMED]  
**Warning signs:** Tests still need mixed raw spellings to find entries in `PathDirectory`, or decoded indexes expose bracket and dot variants as separate paths. [ASSUMED]

### Pitfall 2: Fixing the builder path but forgetting decoded indexes
**What goes wrong:** Newly built indexes get fast path lookup, but indexes loaded through `Decode()` silently fall back to linear scans. [VERIFIED: serialize.go; ASSUMED]  
**Why it happens:** `Decode()` currently reconstructs `PathDirectory` and indexes, but there is no derived path lookup structure to rebuild. [VERIFIED: serialize.go; VERIFIED: gin.go]  
**How to avoid:** Add a shared helper that rebuilds the lookup structure and call it from both `Finalize()` and `Decode()`. [ASSUMED]  
**Warning signs:** Benchmarks improve only on freshly built indexes, or decode/encode regression tests skip path-lookup assertions. [ASSUMED]

### Pitfall 3: Broadening unsupported path syntax by accident
**What goes wrong:** Unsupported expressions like array indices or filters could normalize into a supported spelling, risking incorrect pruning semantics. [VERIFIED: AGENTS.md; VERIFIED: jsonpath.go]  
**Why it happens:** It is tempting to normalize first and validate later. [ASSUMED]  
**How to avoid:** Preserve the existing validation gate for unsupported JSONPath fragments and only canonicalize paths already inside the supported surface. [VERIFIED: jsonpath.go; VERIFIED: AGENTS.md]  
**Warning signs:** New tests start accepting `$.items[0]` or similar expressions that are currently rejected. [ASSUMED]

### Pitfall 4: Benchmarking width without realistic operator coverage
**What goes wrong:** The phase appears faster in a synthetic microbenchmark but fails PATH-03 because EQ, CONTAINS, and REGEX were not all exercised on wide path sets or equivalent-spelling lookups. [VERIFIED: .planning/REQUIREMENTS.md; VERIFIED: .planning/phases/06-query-path-hot-path/06-CONTEXT.md]  
**Why it happens:** The easiest benchmark to write is a single EQ loop over a narrow fixture. [ASSUMED]  
**How to avoid:** Reuse one blended fixture family and add sub-benchmarks for operator mix plus equivalent-spelling lookups. [ASSUMED]  
**Warning signs:** Benchmark names mention only EQ or only narrow fixtures, or equivalent spelling never appears in benchmark input. [ASSUMED]

## Code Examples

### Existing hot path to replace
```go
func (idx *GINIndex) findPath(path string) (int, *PathEntry) {
	for i := range idx.PathDirectory {
		if idx.PathDirectory[i].PathName == path {
			return i, &idx.PathDirectory[i]
		}
	}
	return -1, nil
}
```
Source: `query.go`. [VERIFIED: query.go]

### Existing canonicalization primitive to reuse
```go
func NormalizePath(path string) string {
	expr, err := jp.ParseString(path)
	if err != nil {
		return path
	}
	return expr.String()
}
```
Source: `jsonpath.go`. [VERIFIED: jsonpath.go]

### Existing query benchmark pattern to extend
```go
func BenchmarkQueryEQ(b *testing.B) {
	idx := setupTestIndex(1000)
	for i := 0; i < b.N; i++ {
		idx.Evaluate([]Predicate{EQ("$.name", "user_42")})
	}
}
```
Source: `benchmark_test.go`. [VERIFIED: benchmark_test.go]

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Rebuilding a derived canonical lookup table after `Finalize()` and `Decode()` is preferable to introducing a new serialized format in Phase 06. | Summary / Architecture Patterns | Phase 06 may need extra format-version work and a broader plan boundary. |
| A2 | A repo-local recognizable access-log fixture family is sufficient to satisfy D-04 without vendoring a full external dataset. | Summary / Common Pitfalls | Benchmark planning may need an extra fixture-ingestion task or explicit corpus import decision. |

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | `go test` / Go benchmark tooling |
| Config file | `none — standard Go toolchain` |
| Quick run command | `go test ./... -run 'Test(JSONPath|QueryEQ|QueryContains|QueryRegex)' -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PATH-01 | Indexed predicate lookup resolves without linear path scans on the hot path | unit + benchmark | `go test ./... -run 'Test.*PathLookup|TestQueryEQ' -count=1` | ❌ Wave 0 |
| PATH-02 | Equivalent supported spellings resolve to one canonical stored/query path | unit + integration | `go test ./... -run 'TestJSONPath|Test.*Canonical.*Path|Test.*Serialize' -count=1` | ❌ Wave 0 |
| PATH-03 | Wide-path EQ, CONTAINS, and REGEX benchmarks exist and run from the repo benchmark harness | benchmark | `go test ./... -run '^$' -bench 'BenchmarkQuery(EQ|Contains|Regex)' -count=1` | ✅ |

### Sampling Rate
- **Per task commit:** `go test ./... -run 'Test(JSONPath|QueryEQ|QueryContains|QueryRegex)' -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** `go test ./... -count=1` and `go test ./... -run '^$' -bench 'BenchmarkQuery(EQ|Contains|Regex)' -count=1`

### Wave 0 Gaps
- `gin_test.go` or `query`-focused tests need explicit canonical path lookup coverage for PATH-01 and PATH-02. [VERIFIED: gin_test.go; ASSUMED]
- Existing serialization coverage needs a canonical-path round-trip assertion so decoded indexes prove the same lookup behavior as freshly built indexes. [VERIFIED: gin_test.go; VERIFIED: serialize.go; ASSUMED]

## Security Domain

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | N/A — Phase 06 does not introduce auth surfaces. `[VERIFIED: .planning/ROADMAP.md]` |
| V3 Session Management | no | N/A — Phase 06 is library/query internals only. `[VERIFIED: .planning/ROADMAP.md]` |
| V4 Access Control | no | N/A — no authorization surface changes in scope. `[VERIFIED: .planning/ROADMAP.md]` |
| V5 Input Validation | yes | Preserve `ValidateJSONPath()` and reject unsupported fragments before canonicalization. `[VERIFIED: jsonpath.go; VERIFIED: AGENTS.md]` |
| V6 Cryptography | no | N/A — no crypto surface in this phase. `[VERIFIED: .planning/ROADMAP.md]` |

### Known Threat Patterns for this phase
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Unsupported path canonicalizes into a supported lookup and prunes incorrectly | Tampering | Validate supported JSONPath before canonicalizing and preserve safe fallback on unknown paths. `[VERIFIED: jsonpath.go; VERIFIED: .planning/phases/06-query-path-hot-path/06-CONTEXT.md]` |
| Builder and decoded indexes disagree on canonical path identity | Tampering / Repudiation | Rebuild the same canonical lookup structure in both `Finalize()` and `Decode()` and cover both with tests. `[VERIFIED: builder.go; VERIFIED: serialize.go; ASSUMED]` |
| Benchmark-only optimization masks semantic regression | Denial of Service / Integrity | Pair benchmark additions with canonical-path and serialization regression tests, not benchmarks alone. `[VERIFIED: .planning/REQUIREMENTS.md; ASSUMED]` |

## Sources

### Primary (HIGH confidence)
- `AGENTS.md` — project-specific scope, testing, JSONPath, error-handling, and Makefile constraints.
- `.planning/phases/06-query-path-hot-path/06-CONTEXT.md` — locked decisions and phase boundary.
- `.planning/PROJECT.md` — milestone constraints and benchmark-backed-change requirement.
- `.planning/REQUIREMENTS.md` — PATH-01 through PATH-03 acceptance targets.
- `.planning/codebase/ARCHITECTURE.md` — query/build/serialize lifecycle and safe-fallback semantics.
- `.planning/codebase/CONVENTIONS.md` — package, error-handling, and hot-path coding conventions.
- `.planning/codebase/TESTING.md` — benchmark/test organization and command patterns.
- `query.go`, `jsonpath.go`, `builder.go`, `serialize.go`, `benchmark_test.go`, `regex.go`, `gin_test.go` — concrete implementation and test references used above.

### Secondary (MEDIUM confidence)
- None — current codebase and planning artifacts were sufficient for this phase.

### Tertiary (LOW confidence)
- None — external sources were not required for the current planning question.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — recommendations are anchored in existing repo primitives and Go stdlib usage.
- Architecture: HIGH — current builder/query/serialize flow is directly verified in code.
- Pitfalls: MEDIUM — root causes are verified, but a few mitigation details remain design recommendations for the planner.

**Research date:** 2026-04-14
**Valid until:** 2026-05-14
