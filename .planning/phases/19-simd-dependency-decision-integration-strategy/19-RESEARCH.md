# Phase 19: SIMD Dependency Decision & Integration Strategy - Research

**Researched:** 2026-04-27
**Domain:** Dependency, distribution, build-tag, and fallback strategy for opt-in `pure-simdjson` integration
**Confidence:** HIGH for local parser-seam integration points and current upstream v0.1.4 release facts; MEDIUM for future upstream stability because the dependency is still pre-v1.

## Summary

Phase 19 is a decision-finalization phase, not an implementation phase. The execution work should produce a durable strategy artifact that downstream Phase 21 and Phase 22 can consume without re-opening first-order dependency questions.

The earlier v1.1 research concern that `github.com/amikos-tech/pure-simdjson` had no license, no tags, and no binaries is stale as of 2026-04-27. Current upstream state supports adoption behind an explicit build tag:

- GitHub reports MIT license metadata for the repository.
- Tag `v0.1.4` exists as an annotated tag pointing to commit `0f53f3f2e8bb9608d6b79211ffc5fc7b53298617`.
- The `v0.1.4` release publishes prebuilt native libraries for `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, and `windows/amd64`, with `.pem`, `.sig`, and `SHA256SUMS` companion files.
- `docs/bootstrap.md` documents `NewParser()` auto-download, SHA-256 verification, OS-cache installation, `PURE_SIMDJSON_LIB_PATH`, `PURE_SIMDJSON_BINARY_MIRROR`, `PURE_SIMDJSON_DISABLE_GH_FALLBACK`, `PURE_SIMDJSON_CACHE_DIR`, and the `pure-simdjson-bootstrap` CLI.

Primary recommendation: execute Phase 19 as one documentation strategy plan. The deliverable should be `.planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md`, plus a small `.planning/STATE.md` update that records Phase 19 as strategy-complete. No `go.mod`, source, CI, README, NOTICE, or product documentation changes should land in Phase 19; those belong to Phase 21 and Phase 22.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SIMD-01 | Explicit SIMD dependency source, license/NOTICE obligations, version/tag pinning, and acceptability decision | Covered by upstream verification, `v0.1.4` pin, NOTICE posture, and manual license-drift-on-bump policy |
| SIMD-02 | Explicit shared-library distribution/loading strategy with unsupported-platform behavior and release guidance | Covered by pure delegation to upstream bootstrap, env-var strategy, cache-backed CI bootstrap, 5-platform support set, and tier-2 downgrade policy |
| SIMD-03 | Build tags, default stdlib behavior, opt-in API shape, CI expectations, and stop/fallback path | Covered by `//go:build simdjson`, `NewSIMDParser() (Parser, error)`, `WithParser(p)`, hard construction errors, explicit fallback recipe, 5-platform CI matrix, and hard/soft stop table |

## Standard Stack

### Existing Local Stack

| Library | Version | Purpose | Phase 19 Relevance |
|---------|---------|---------|--------------------|
| Go | `1.25.5` in `go.mod` | Root module language version | Default builds must remain Go-only and stdlib parser by default |
| `github.com/pkg/errors` | `v0.9.1` | Error creation/wrapping | `NewSIMDParser` construction failure should use `errors.Wrap` per project convention |
| `github.com/leanovate/gopter` | `v0.2.11` | Existing property tests | Phase 22 should extend parser parity coverage using existing generator patterns |
| `github.com/cespare/xxhash/v2`, `roaring/v2`, `klauspost/compress` | existing | Index internals | No Phase 19 change |

### Proposed Optional SIMD Dependency

| Dependency | Version | Purpose | Adoption Rule |
|------------|---------|---------|---------------|
| `github.com/amikos-tech/pure-simdjson` | `v0.1.4` | Opt-in cgo-free wrapper over simdjson via prebuilt native libraries loaded with `purego` | Add only in Phase 21, from `parser_simd.go` behind `//go:build simdjson`; default builds must not require native libraries |

### Upstream Facts Verified 2026-04-27

| Fact | Evidence |
|------|----------|
| License metadata | GitHub repository API reports MIT license |
| Release tag | `refs/tags/v0.1.4` annotated tag points to commit `0f53f3f2e8bb9608d6b79211ffc5fc7b53298617` |
| Release assets | `v0.1.4` publishes native artifacts for 5 supported platforms, signatures, certs, and `SHA256SUMS` |
| Bootstrap behavior | `docs/bootstrap.md` documents auto-download, SHA-256 verification, cache install, flock, GitHub fallback, ABI check, and env overrides |
| NOTICE chain | Upstream `NOTICE` states simdjson is vendored under `third_party/simdjson` and carries Apache-2.0 and MIT license texts |

## Architecture Patterns

### Pattern 1: Same-Package Build-Tagged Parser

`parser_simd.go` should live in the root `gin` package with `//go:build simdjson`. This preserves Phase 13's locked package-private `parserSink` design. External parser implementations remain deferred because `parserSink` is not exported.

Phase 21 constructor shape:

```go
func NewSIMDParser() (Parser, error)
```

Required semantics:

- Return `Name() == "pure-simdjson"`.
- Wrap `purejson.NewParser()` construction errors with:
  `errors.Wrap(err, "initialize pure-simdjson SIMD parser; set PURE_SIMDJSON_LIB_PATH or see docs/simd-deployment.md")`.
- Do not silently fall back to stdlib.
- Caller opt-in remains:
  `p, err := gin.NewSIMDParser(); builder, err := gin.NewBuilder(cfg, n, gin.WithParser(p))`.

### Pattern 2: Native Library Delegation

`ami-gin` should delegate bootstrap, cache, mirror, and ABI logic to upstream `pure-simdjson`. The local integration should add only friendlier error guidance and consumer docs.

Phase 21 docs should cover:

- Happy path: build with `-tags simdjson`; first `NewSIMDParser` construction triggers upstream bootstrap.
- Air-gapped path: set `PURE_SIMDJSON_LIB_PATH`.
- Corporate mirror path: set `PURE_SIMDJSON_BINARY_MIRROR`, optionally `PURE_SIMDJSON_DISABLE_GH_FALLBACK=1`.
- CI cache path: cache upstream library cache directory keyed on `pure-simdjson` tag, OS, and architecture.
- Explicit stdlib fallback recipe for callers that want degradation.

### Pattern 3: Default Behavior Is Untouched

Default `go build ./...` and default `NewBuilder` must continue using `stdlibParser{}`. The dependency and native loading behavior are acceptable only when both conditions are true:

1. The package is built with `-tags simdjson`.
2. The caller explicitly calls `NewSIMDParser()` and passes the returned parser with `WithParser`.

## Local Code Integration Findings

Current parser seam evidence:

- `parser.go` exports `Parser` and `WithParser`.
- `parser_sink.go` keeps `parserSink` package-private and exposes `BeginDocument`, `MarkPresent`, `StageScalar`, `StageJSONNumber`, `StageNativeNumeric`, `StageMaterialized`, and `ShouldBufferForTransform`.
- `builder.go` defaults `b.parser = stdlibParser{}` and caches `parserName` at `NewBuilder`.
- `AddDocument` delegates to `b.parser.Parse(jsonDoc, pos, b)`, then verifies `BeginDocument` was called exactly once with the expected row-group id.
- `parser_stdlib.go` preserves the existing `encoding/json.Decoder.UseNumber()` behavior and presents object/array roots via `MarkPresent`.
- `parser_parity_test.go` already provides authored golden parity and parser equivalence coverage; Phase 22 should reuse and extend this for SIMD.

Phase 21 implementation should avoid re-opening Phase 13's sink/export decision. The current sink is intentionally same-package, and that is exactly why the SIMD parser should live in root `package gin`.

## Risks and Mitigations

| Risk | Severity | Mitigation |
|------|----------|------------|
| SIMD numeric parsing loses exact-int semantics | HARD | SIMD walker must route raw or typed numbers through the existing sink; Phase 22 parity must compare encoded bytes and query results against stdlib |
| Upstream native library cannot load on one supported platform | SOFT if isolated to one platform | Document that platform as tier 2 and require manual release verification until fixed |
| Upstream archive or unrecoverable breaking shape change before implementation | HARD | Halt Phase 21/22 and defer SIMD to v1.4; re-evaluate dependency landscape |
| Default builds accidentally require native library or upstream module | HARD | Keep all imports in `parser_simd.go` behind `//go:build simdjson`; CI must run default build without tag |
| Consumers misunderstand silent fallback behavior | MEDIUM | Do not silently fall back; document explicit fallback recipe |
| License/NOTICE drift on future bump | MEDIUM | Pin-and-hold; manual LICENSE and NOTICE review on every bump PR |

## Validation Architecture

Phase 19 validation is artifact-focused because it is a strategy phase.

Automated checks should prove:

- `19-SIMD-STRATEGY.md` exists.
- The strategy artifact contains every requirement ID: `SIMD-01`, `SIMD-02`, `SIMD-03`.
- The strategy artifact contains the exact dependency pin string `github.com/amikos-tech/pure-simdjson v0.1.4`.
- The strategy artifact contains `NewSIMDParser() (Parser, error)`.
- The strategy artifact contains `//go:build simdjson`.
- The strategy artifact contains `PURE_SIMDJSON_LIB_PATH`.
- The strategy artifact contains the hard/soft stop table with `HARD` and `SOFT`.
- The plan remains documentation-only: it must not edit `go.mod`, `go.sum`, source files, CI workflows, README, NOTICE, or CHANGELOG in Phase 19.

Recommended focused command after the Phase 19 execution plan:

```bash
test -f .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md &&
grep -q 'SIMD-01' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md &&
grep -q 'SIMD-02' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md &&
grep -q 'SIMD-03' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md &&
grep -q 'github.com/amikos-tech/pure-simdjson v0.1.4' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md &&
grep -q 'NewSIMDParser() (Parser, error)' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md &&
grep -q '//go:build simdjson' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md &&
grep -q 'PURE_SIMDJSON_LIB_PATH' .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md
```

Full sanity command:

```bash
go test ./...
```

## Planning Recommendation

Create one plan:

- **19-01:** Finalize SIMD strategy artifact and update planning state.

Do not split Phase 19 into multiple implementation plans. The phase has one coherent deliverable: a durable decision record consumed by Phase 21 and Phase 22. Splitting would create coordination overhead without reducing risk.

## Out of Scope

- Adding `pure-simdjson` to `go.mod`.
- Creating `parser_simd.go`.
- Creating `NOTICE.md`.
- Creating `docs/simd-deployment.md`.
- Modifying `.github/workflows/ci.yml`.
- Modifying README or CHANGELOG.
- Running SIMD benchmarks.
- Writing parity tests for SIMD.

Those actions belong to Phase 21 and Phase 22.

---

## RESEARCH COMPLETE
