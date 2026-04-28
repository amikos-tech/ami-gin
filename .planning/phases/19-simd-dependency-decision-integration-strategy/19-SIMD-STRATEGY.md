# Phase 19 SIMD Strategy

## Decision Summary

Phase 19 makes SIMD executable by locking decisions only; no product code lands in Phase 19. The selected path is an optional, same-package parser adapter that Phase 21 will implement behind a build tag and Phase 22 will validate with parity, benchmark, and CI evidence.

This strategy satisfies SIMD-01, SIMD-02, and SIMD-03 by recording the dependency pin, license and NOTICE posture, shared-library loading delegation, opt-in API shape, default stdlib behavior, CI expectations, and stop/fallback policy.

## SIMD-01: Dependency Source, Pinning, License, and NOTICE

- Dependency: github.com/amikos-tech/pure-simdjson v0.1.4
- Tag commit: 0f53f3f2e8bb9608d6b79211ffc5fc7b53298617
- License: MIT
- NOTICE posture: add root NOTICE.md and README dependency credit in Phase 21
- Bump cadence: pin-and-hold; manual LICENSE and NOTICE review on every bump PR

The dependency is accepted for v1.3 only as an optional SIMD path. Default builds remain independent from this module and from its native shared-library bootstrap.

## SIMD-02: Shared-Library Distribution and Loading

`NewSIMDParser` delegates to upstream `purejson.NewParser()`. ami-gin does not reimplement download, cache, mirror, flock, SHA-256 verification, or ABI logic. ami-gin adds only local API shape and friendlier construction-error guidance.

Runtime loading and distribution remain owned by upstream `pure-simdjson` bootstrap behavior, including these documented environment variables:

- `PURE_SIMDJSON_LIB_PATH`
- `PURE_SIMDJSON_BINARY_MIRROR`
- `PURE_SIMDJSON_DISABLE_GH_FALLBACK`
- `PURE_SIMDJSON_CACHE_DIR`

The supported Go platform set for the SIMD path is:

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`
- `windows/amd64`

The corresponding GitHub release asset labels are:

- `linux-amd64`
- `linux-arm64`
- `darwin-amd64`
- `darwin-arm64`
- `windows-amd64-msvc`

Phase 22 CI should cache the upstream native library with this key pattern:

```text
pure-simdjson-${{ matrix.os }}-${{ matrix.arch }}-v0.1.4
```

## SIMD-03: Build Strategy, Opt-In API, CI, and Stop Conditions

The SIMD adapter is compiled only in a same-package file guarded by:

```go
//go:build simdjson
```

The Phase 21 API shape is:

```go
func NewSIMDParser() (Parser, error)
```

The parser name contract is `Name() == "pure-simdjson"`. Callers select SIMD explicitly through `WithParser`; default stdlib builds remain dependency-free and native-library-free. There is no behavior change unless the caller builds with `-tags simdjson`, successfully constructs a SIMD parser, and passes it to `NewBuilder`.

Construction errors are hard and wrapped with:

```text
initialize pure-simdjson SIMD parser; set PURE_SIMDJSON_LIB_PATH or see docs/simd-deployment.md
```

Caller-owned stdlib fallback stays explicit:

```go
p, err := gin.NewSIMDParser()
if err == nil {
	builder, err := gin.NewBuilder(cfg, numRGs, gin.WithParser(p))
	if err != nil {
		return nil, err
	}
	return builder, nil
}

builder, fallbackErr := gin.NewBuilder(cfg, numRGs)
if fallbackErr != nil {
	return nil, fallbackErr
}
return builder, err
```

No silent fallback: NewSIMDParser returns an error instead of internally selecting stdlib.

| Trigger | Severity | Action |
|---|---|---|
| SIMD path produces non-parity encoded bytes or query results vs. stdlib | HARD | Halt Phase 21/22; defer SIMD to v1.4 because correctness is broken. |
| `pure-simdjson` upstream archives or makes an unrecoverable breaking change mid-implementation | HARD | Halt Phase 21/22; defer SIMD to v1.4 and re-evaluate the dependency landscape. |
| One of the 5 `-tags simdjson` CI platforms fails while others pass | SOFT | Document the affected platform as tier 2 and require manual verification until resolved. |
| Measured SIMD speedup is below expectations on Phase 22 benchmarks | Decide on evidence | Use the benchmark report and workload context to decide whether to ship, defer, or narrow support. |

On a HARD trigger, pause Phase 21/22 with /gsd-pause-work and record the v1.4 deferral item before any further SIMD implementation work.

## Downstream Phase 21 Contract

Phase 21 owns `go.mod`, `go.sum`, `parser_simd.go`, `NOTICE.md`, `docs/simd-deployment.md`, README dependency credit, and the initial CHANGELOG v1.3 note.

Phase 21 must keep the default stdlib path unchanged. The optional SIMD adapter imports `github.com/amikos-tech/pure-simdjson v0.1.4` only from build-tagged code, returns `NewSIMDParser() (Parser, error)`, reports `Name() == "pure-simdjson"`, delegates native bootstrap to upstream `purejson.NewParser()`, and relies on explicit caller selection through `WithParser`.

## Downstream Phase 22 Contract

Phase 22 owns SIMD parity tests, realistic benchmark results, `-tags simdjson` CI matrix, release/distribution guidance verification, and stop-table enforcement.

Phase 22 depends on both Phase 20 datasets and Phase 21 adapter work. Phase 20 remains independent and must not be unnecessarily serialized behind Phase 19.

The `-tags simdjson` CI matrix must cover `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, and `windows/amd64`, with the Windows native asset label normalized as `windows-amd64-msvc`.

## Out of Scope for Phase 19

Phase 19 does not edit `go.mod`, `go.sum`, source files, CI workflows, README, NOTICE, CHANGELOG, or runtime docs.
