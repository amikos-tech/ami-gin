# Deferred items for quick task 261006-hgl

## Shared zstd decoder retention is unmeasured

- `sharedZstdDecoder` (`serialize.go`, about line 367) builds one decoder under `sync.Once`. Nothing closes it.
- It sets `WithDecoderMaxMemory(maxDecodedIndexSize)`, `WithDecoderMaxWindow(maxDecodedIndexSize)` and `WithDecodeAllCapLimit(true)`. `maxDecodedIndexSize` is 64 MiB (`serialize.go:32`).
- It sets no concurrency option, so it uses the klauspost default. This task did not read that default in the klauspost source, so the worker count is unverified.
- Issue #83 is about the encoder only. This task did not measure what the decoder keeps live after `Decode`.
- Out of scope per decision D7. Nothing in this task changes the decoder.

## Draft GitHub issue

The executor did NOT create this issue. The orchestrator creates it after the maintainer approves the draft.

**Title:** Measure and possibly bound the heap retained by the shared zstd decoder

**Body:**

Issue #83 and the `EncoderProfileBoundedMemoryUncached` profile cover the zstd encoder only. The decoder is also shared and never closed (`sharedZstdDecoder` in `serialize.go`). We have not measured what it keeps live after `Decode`.

What to measure:

- Live heap after one `Decode` of a small index and of a large index, with a forced GC before and after.
- The same at `GOMAXPROCS` 1, 4 and 16, since the decoder may scale its workers with it.
- Whether the 64 MiB window and memory limits change retained heap or only cap it.

Proposed acceptance:

- A benchmark in `benchmark_test.go` that prints retained MB per `GOMAXPROCS`.
- A table in `docs/` with the real numbers.
- A decision: leave as is, or add an opt-in bounded profile for the decoder, as was done for the encoder.

No code change is proposed until the numbers show a problem.
