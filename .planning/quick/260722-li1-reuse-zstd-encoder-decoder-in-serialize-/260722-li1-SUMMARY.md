---
quick_id: 260722-li1
slug: reuse-zstd-encoder-decoder-in-serialize-
date: 2026-07-22
issue: 43
status: complete
---

# Summary: Reuse zstd encoder/decoder in serialize.go

## What changed

`serialize.go` no longer constructs a zstd codec per call. Added lazily
initialized, package-level shared codecs:

- `sharedZstdEncoder(level)` — `sync.Mutex`-guarded `map[CompressionLevel]*zstd.Encoder`,
  each built once with `WithEncoderLevel` + `WithEncoderConcurrency(1)`.
- `sharedZstdDecoder()` — a single `sync.Once` decoder built with the existing
  `WithDecoderMaxMemory` / `WithDecoderMaxWindow` / `WithDecodeAllCapLimit(true)`
  safety limits.

`encodeWithLevel` and `Decode` now fetch the shared instances; the per-call
`defer Close()` calls were removed (shared instances live for process lifetime).

## Impact (Apple M3 Max, 16 cores)

`BenchmarkPhase20RealisticJSON/.../Encode`, before → after:

| Fixture | B/op before | B/op after | ns/op before | ns/op after |
| --- | ---: | ---: | ---: | ---: |
| number-heavy | 588 MB | 496 KB | 6.0 ms | 0.85 ms |
| nested-high-cardinality | 589 MB | 1.23 MB | 6.53 ms | 1.52 ms |
| combined | 589 MB | 2.20 MB | 6.90 ms | 2.52 ms |

~**1000×** fewer bytes allocated per Encode and ~3–7× faster. The encoder
construction (97% of prior allocations, one state per GOMAXPROCS core) is gone.

## Invariants preserved

- Public API unchanged.
- Decoder decode-bomb limits unchanged.
- The per-call `make([]byte, 0, maxDecodedIndexSize)` decode dst buffer is retained
  (a shared mutable output buffer would race; its cap also enforces the size limit).

## Follow-up (out of scope, noted)

`BenchmarkDecode` now shows ~67 MB/op, dominated by that 64 MiB dst buffer, not the
decoder object. Shrinking it while keeping the size ceiling (already independently
enforced by `WithDecoderMaxMemory`) is a separate optimization.

## Verification

- `go build ./...`, `go vet ./...`, `gofmt -l` — clean
- `go test ./...` — all pass (encode/decode lossless + equivalent-index property
  tests, decode-limit tests)
- `golangci-lint run` — no findings in `serialize.go`

Fixes #43.
