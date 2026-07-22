# Optional SIMD parser deployment

GIN Index offers an opt-in parser adapter for
[`github.com/amikos-tech/pure-simdjson` v0.1.4](https://github.com/amikos-tech/pure-simdjson/tree/v0.1.4).
Use the tagged module version `v0.1.4` in consumer dependency configuration.
The tag's recorded commit is repository audit evidence, not alternate
installation syntax.

## Default behavior

Ordinary builds remain stdlib-only. Without the `simdjson` build tag, the SIMD
adapter is not compiled, and neither `pure-simdjson` nor its native shared
library is part of the default runtime path. `NewBuilder` continues to select
the standard-library parser unless the caller explicitly supplies another
parser with `WithParser`.

Enabling SIMD has three independent gates:

1. **Build time:** compile the application with `-tags simdjson` so
   `NewSIMDParser` is available.
2. **Construction time:** call `NewSIMDParser()` and handle its error. This call
   resolves, loads, and ABI-checks the native library at runtime.
3. **Builder selection:** pass the successfully constructed parser to
   `NewBuilder(..., WithParser(p))`. A tagged build alone does not select SIMD.

There is no silent fallback at any gate.

## Enable the parser

Build the consuming application with the tag:

```bash
go build -tags simdjson ./...
```

Then construct the parser and select it explicitly. This complete example
handles both construction errors and builder errors:

```go
package app

import gin "github.com/amikos-tech/ami-gin"

func newSIMDBuilder(config gin.GINConfig, numRGs int) (*gin.GINBuilder, error) {
	p, err := gin.NewSIMDParser()
	if err != nil {
		return nil, err
	}

	builder, err := gin.NewBuilder(config, numRGs, gin.WithParser(p))
	if err != nil {
		return nil, err
	}
	return builder, nil
}
```

`NewSIMDParser` returns a hard construction error when the native library
cannot be resolved or loaded. The error points to `PURE_SIMDJSON_LIB_PATH` as
the recovery path. Do not pass the constructor call directly to `WithParser`:
first handle the returned `error`, then pass only the `Parser` value.

## Automatic bootstrap

When `PURE_SIMDJSON_LIB_PATH` is unset, the upstream v0.1.4 bootstrap performs
native resolution during `NewSIMDParser()`:

1. load a previously installed cache entry, if present;
2. otherwise download the platform artifact from the configured primary
   source;
3. fall back to the upstream release source unless that fallback is disabled;
4. verify a newly downloaded artifact against published SHA-256 metadata,
   install it atomically in the cache, load it, and verify its ABI.

Cache hits are not hashed again; upstream verifies them when they are first
installed. See the pinned
[`pure-simdjson` v0.1.4 bootstrap guide](https://github.com/amikos-tech/pure-simdjson/blob/v0.1.4/docs/bootstrap.md)
for the exact resolution order, supported artifact names, retry behavior, and
pre-fetch commands.

## Air-gapped and explicit-path loading

For an air-gapped host, fetch and verify the correct v0.1.4 artifact on a
connected machine, transport it through the operator's approved channel, and
set its absolute path on the target host:

```bash
export PURE_SIMDJSON_LIB_PATH=/approved/path/to/libpure_simdjson.so
```

The pinned upstream bootstrap guide contains the cross-platform pre-fetch and
offline-bundle verification commands. When `PURE_SIMDJSON_LIB_PATH` is set,
upstream loads that file verbatim and performs no network or cache operation.
It also bypasses the bootstrap download checksum check. The operator therefore
owns checksum and provenance validation for this supplied library.

## Corporate mirror and hermetic loading

The upstream bootstrap recognizes four variables:

| Variable | Purpose |
| --- | --- |
| `PURE_SIMDJSON_LIB_PATH` | Load an operator-supplied native library directly; bypass downloads and the cache. |
| `PURE_SIMDJSON_BINARY_MIRROR` | Replace the primary native-artifact base URL with an HTTPS mirror. |
| `PURE_SIMDJSON_DISABLE_GH_FALLBACK` | Set to `1` to prevent fallback to the upstream release source when the primary source or mirror fails. |
| `PURE_SIMDJSON_CACHE_DIR` | Override the base directory used for automatically downloaded artifacts. |

For a corporate mirror, publish the v0.1.4 assets using the layout documented
upstream, set `PURE_SIMDJSON_BINARY_MIRROR`, and decide explicitly whether
fallback egress is permitted. A hermetic deployment normally combines the
mirror with `PURE_SIMDJSON_DISABLE_GH_FALLBACK=1` and a controlled
`PURE_SIMDJSON_CACHE_DIR`. Automatically downloaded artifacts still follow
the upstream SHA-256 install check; an explicit `PURE_SIMDJSON_LIB_PATH` does
not.

## Caller-owned stdlib fallback

Construction failure does not make `NewSIMDParser` select the stdlib parser.
If fallback is acceptable, branch explicitly and record the degraded path in
the application's logs or telemetry:

```go
package app

import (
	"log"

	gin "github.com/amikos-tech/ami-gin"
)

func newBuilderWithFallback(config gin.GINConfig, numRGs int) (*gin.GINBuilder, error) {
	p, simdErr := gin.NewSIMDParser()
	if simdErr == nil {
		builder, err := gin.NewBuilder(config, numRGs, gin.WithParser(p))
		if err != nil {
			return nil, err
		}
		return builder, nil
	}

	log.Printf("SIMD parser unavailable; selecting stdlib parser: %v", simdErr)
	builder, err := gin.NewBuilder(config, numRGs)
	if err != nil {
		return nil, err
	}
	return builder, nil
}
```

This branch is caller policy. The library never hides a failed SIMD
construction behind an automatic parser change.

## Native artifact integrity

The loading routes have separate trust controls:

| Route | Integrity boundary |
| --- | --- |
| Go wrapper module | The Go tool verifies the `pure-simdjson` v0.1.4 module content against the checksum recorded in `go.sum`. This does not verify a separately supplied native library. |
| Automatic download or mirror | Upstream resolves published SHA-256 metadata and verifies the native artifact before installing it in the cache. ABI verification also runs when the library is loaded. |
| `PURE_SIMDJSON_LIB_PATH` | Download and cache verification are bypassed. The operator must verify the selected file's checksum, provenance, permissions, and update process. |

Upstream also documents optional cosign provenance verification. It is an
additional operator control; it does not replace the automatic SHA-256 check.

## BIGINT limitation

An integer larger than the `uint64` range is an accepted parser difference.
GIN Index does not pre-scan JSON for these values because doing so would add a
second text parse ahead of the SIMD parser.

- On the SIMD path, upstream `Parse` fails the whole input with a
  `BIGINT_ERROR` before a field path is staged. `AddDocument` therefore treats
  it as a parser-layer failure with an empty path, governed by
  `ParserFailureMode`.
- On the stdlib path, parsing reaches the field and GIN Index reports a
  path-aware numeric-layer failure, governed by `NumericFailureMode`.

In hard mode, the applicable layer returns an error. In soft mode, either
layer discards the failed document atomically; stdlib does not retain the
other fields from that document. Because the governing knobs differ, a caller
that configures parser and numeric failure modes differently can observe a
different acceptance result for the same BIGINT document.

## Validation scope

Phase 21 establishes the adapter contract and this operating guidance. Parser
parity, measured performance, five-platform runtime CI, and end-to-end
operational verification belong to Phase 22. No parity result, speedup, or
platform certification is claimed here.
