# Issue #83 - Bounded-memory encoder without retention - Research

**Researched:** 2026-10-06 | **Mode:** quick-task, pre-discussion | **Confidence:** HIGH on facts below, MEDIUM on cost of concurrency and decoder (not measured)

Tags: VERIFIED = read in code/docs or run in this session. ASSUMED = not checked.

## 1. Validation verdict

1. **The issue is correct.** The bounded-memory encoder is kept in a process-wide map and never released. [VERIFIED]
2. **One nuance:** "closed after use" does not release anything. For an encoder used only through `EncodeAll`, `Close()` is a no-op. Dropping the reference is what frees the memory. [VERIFIED]
3. **Nothing overstated.** The workaround is accurate. The 42.5 MB figure reproduces. Only "peak" is not in the doc today (see 7).

Evidence:
- Cache: `serialize.go:295-327`. `zstdEncoders map[zstdEncoderKey]*zstd.Encoder` under `zstdEncoderMu`. Key = `{collapsed zstd.EncoderLevel (4 modes), EncoderProfile}`, so at most 4 x 2 = 8 entries. Levels 15 and 19 share one entry.
- No eviction or release API in production. The comment at `serialize.go:~292` says "never Closed by production code". The only evictor is test-only: `evictSharedZstdEncoder` in `serialize_profile_test.go:29`.
- Doc: `docs/encoder-profile-benchmarks.md` retained table, L15 bounded = 42.5 MB at every GOMAXPROCS. I re-ran `BenchmarkEncoderProfile/bounded-memory/.../cold` (GOMAXPROCS=4, darwin/arm64): L15 retained 42.23 MB (small), 42.50 MB (highcard); alloc/op 44.4 / 46.0 MB. This matches the doc (linux/amd64). Doc header says klauspost v1.19.2, but `go.mod:7` pins v1.20.0. Numbers still match, but the doc header is stale.
- Workaround vs wire format: `serialize.go:505-515`. `CompressionNone` returns `"GINu"+body`. Otherwise the result is `"GINc"+encoder.EncodeAll(body)`. A caller who compresses the body with a same-level one-worker encoder and prepends `GINc` gets the same bytes. This depends on the caller knowing the two magics (`serialize.go:109-110`) and matching encoder options exactly. `BenchmarkEncoderProfile` already does the first half (`uncompressedPayload`, `benchmark_test.go`).

## 2. Where the retained memory comes from (klauspost v1.20.0, `zstd/encoder.go`)

- `NewWriter(nil, ...)` builds only options. `EncodeAll` calls `e.init.Do(e.initialize)` (`encoder.go:90-99`, `:722-727`). That makes a channel of `concurrent` inner `encoder` structs (match tables + history/window buffer). This is the retained state: tables sized by level, plus window buffers. L15 uses the "best" mode and allocates a full window regardless of payload size (doc). `lowMem` (`enc_base.go:87-93`) trims buffer sizing only. It does not change the window, so output is identical.
- Default profile = one such struct per GOMAXPROCS, about 34 MB each at L15 (doc).
- **No goroutines on the `EncodeAll` path.** The `go` statements are in `nextBlock` (streaming only, `encoder.go:393,420`). [VERIFIED by function lookup]
- `Close()` returns at once when `state.encoder == nil` (`encoder.go:589-592`). `state.encoder` is only set by `Reset`/streaming (`:144`). `NewWriter(nil)` + `EncodeAll` never sets it. So `Close()` is a no-op and GC reclaims an unreferenced encoder without it. The plan may still call `Close()` for hygiene. Do not claim it frees memory.
- Heap is garbage after the call, not live. The ~44 MB is still allocated until the next GC. This is what the issue wants (live heap sets the GC goal).

## 3. Current design map

- `serialize.go`: `EncoderProfile uint8` (`:206`), consts `EncoderProfileDefault` (iota 0), `EncoderProfileBoundedMemory` (1) (`:217-223`); `String()` (`:227`); `validate()` (`:238`, hard-coded error text names both values, `:243`); `EncodeOption func(*encodeRuntime)` (`:200`); `WithEncodeProfile` (`:250`); `encodeRuntime.profile` seeded from config then overridden by options (`:386-395`); `configEncoderProfile` (`:416`); `encodeWithLevel(idx, level, profile)` (`:424`, re-validates profile); `sharedZstdEncoderKey/sharedZstdEncoder` (`:311,:315`); `newZstdEncoder(level, profile)` (`:335`, already builds an uncached encoder; bounded = `WithEncoderConcurrency(1)` + `WithLowerEncoderMem(true)`).
- `gin.go`: `GINConfig.EncoderProfile` (`:459`), `WithEncoderProfile` (`:843`), validated in config validation (`:902`).
- **Not serialized.** `writeConfig` (`serialize.go:1843`) writes a JSON `sc` struct with no profile. The type doc says "runtime-only ... not written into the serialized index" (`serialize.go:202-205`; `gin.go:454-457`). A decoded index always has `EncoderProfileDefault`. A new enum value or new field cannot touch the wire format. [VERIFIED]
- Flow: `Encode`/`EncodeContext`/`EncodeWithLevel*` -> `encodeWithLevel`. `WriteSidecar`, `EncodeToMetadata`, `RebuildWithIndex` (`parquet.go:62,255,350`) and S3 `WriteSidecar[Context]` (`s3.go:283,290`) take `...EncodeOption` and read `idx.Config`. So any new setting carried by profile/option/config reaches Parquet and S3 with no code change. `TestEncoderProfileConfigReachesAllEncodePaths` (`serialize_profile_test.go:178`) already guards this. [path-through ASSUMED beyond signatures]
- CLI: `-low-memory` flag (`cmd/gin-index/main.go:113`), `encoderProfileFor(lowMemory)` (`:185`). `cmd/gin-index/experiment.go` also references the profile.
- Tests to extend: `serialize_profile_test.go:55` (byte identity), `:134` (cache key), `:178` (config reach), `:317` (single worker via `inspectZstdEncoderInternals` `:351`), `:377` (per-call override), `:417` (unknown rejected), `:445` (String); `serialize_concurrency_test.go:98` (mixed profiles). `encoderProfileLevels` at `serialize_profile_test.go:20`. Docs: README `:403-436`, CHANGELOG (v1.3.0 just cut), `Makefile:65` `bench-encoder-profile`.

## 4. API shape options

Key fact: one `EncodeAll` call uses exactly one inner encoder. An uncached default-profile encoder would build GOMAXPROCS workers and use one. At 16 CPUs that is 588 MB alloc and 121 ms per call (doc, "default L15 small cold"). So **uncached only makes sense as one-worker**. Using `lowMem` also lowers the build cost: 44.4 MB vs 52.9 MB alloc at L15 (doc). Therefore "default profile, uncached" has no valid meaning.

| Option | New public names | Fit | Problems |
|---|---|---|---|
| (a) third profile, e.g. `EncoderProfileBoundedMemoryUncached` | 1 const. Existing `WithEncoderProfile`/`WithEncodeProfile`/CLI mapping accept it. | Best. No new option pair, config field, or Parquet/S3 plumbing. Zero invalid combinations. | Long name. Changes: `String()`, `validate()` and its message, `sharedZstdEncoder` must bypass the cache for it (the key struct would otherwise cache it). Enum conflates two axes (workers, retention). |
| (b) orthogonal option, e.g. `WithEncoderRetention(false)` + per-call twin | 2 options + 1 `GINConfig` field | Matches the repo's option pairs. | Needs a rule: uncached forces one worker, else "default+uncached" is the wasteful state. That makes it a profile in disguise. Zero-value trap: a bool `retain` zero value is false, which flips the default for `GINConfig{}` literals. Store it inverted (`uncached bool`). More tests (matrix). |
| (c1) release function, e.g. `ReleaseEncoders()` | 1 func | Tiny. Safe vs in-flight encodes (`Close` is a no-op, GC keeps used encoders alive). | Global, racy with other goroutines that refill the cache. Does not meet "nothing added to the shared cache". Residency returns on next call. |
| (c2) caller-supplied compressor | new interface/func option | Flexible. | Bigger API, no byte-identity guarantee, leaks the format. Not recommended. |

Semver: all of (a), (b), (c1) are additive, so a minor release (v1.4.0). Do not renumber existing consts. Recommendation: **(a)**. Maintainer picks the name (alternatives: `EncoderProfileTransient`).

## 5. Scope edges (maintainer decisions)

- Default profile uncached: see section 4. Recommend not offering it.
- Concurrency: each concurrent call builds its own ~44 MB encoder (L15). N goroutines give N x 44 MB peak, where the shared bounded encoder queues them on one worker. Document it. A package semaphore is possible but is scope creep unless asked. [not measured]
- Decoder: `sharedZstdDecoder` (`serialize.go:346-354`) is `sync.Once`, never closed, `WithDecoderMaxMemory/Window(64 MiB)`, concurrency from GOMAXPROCS (`decoder_options.go:34`). Its retained size is NOT measured here [ASSUMED smaller; unknown]. Issue #83 is encode-only. Ask whether to open a follow-up.
- CLI: add a flag (e.g. `-low-memory-uncached`) or leave library-only? Not in the issue acceptance.
- Level 0 (`CompressionNone`) never touches an encoder (`serialize.go:505`). All profiles are no-ops there.

## 6. Testing "no encoder stays reachable"

Repo today: no `SetFinalizer`/`AddCleanup`/`weak` use in tests. Only `heapAllocAfterGC()` (`benchmark_test.go:4793`) in the benchmark. [VERIFIED by grep]

1. **Deterministic cache check:** `!sharedZstdEncoderCached(level, newProfile)` after the call, per level in `encoderProfileLevels` (helper exists, `serialize_profile_test.go:42`). Also assert the bounded entry did not appear.
2. **Reachability:** needs a seam, since the encoder is created inside the call. Add an unexported package hook (nil in production) that receives the built `*zstd.Encoder`; the test wraps it with `weak.Make` (Go 1.24+, `go.mod` is 1.25.5) or `runtime.AddCleanup`. After the call, loop `runtime.GC()` up to ~10 times until `weak.Pointer.Value()==nil`. Use a `//go:noinline` helper so no stack slot holds the pointer. Prefer weak/AddCleanup over `SetFinalizer`.
3. **Heap delta (the literal acceptance wording):** `heapAllocAfterGC()` before/after with a coarse threshold (e.g. under 10 MB vs 42 MB retained). Must be a non-parallel test: the package has `t.Parallel` tests whose allocations pollute `HeapAlloc`. Go runs sequential top-level tests first (comment at `serialize_profile_test.go:26-28`).
4. Flakiness risks: parallel tests, conservative-looking stack liveness in the test frame, and a runtime that caches large spans (`HeapAlloc` falls, `HeapSys` does not). Assert on `HeapAlloc`/weak pointer, not RSS.

## 7. Cost numbers

Reproduced locally (GOMAXPROCS=4, `-benchtime=10x`, darwin/arm64, no repo files changed; command: `go test -run '^$' -bench 'BenchmarkEncoderProfile/bounded-memory/L(15|3|1)/(small|highcard)/cold' -benchmem`):

| Level | Fixture | Time/op (construct+encode, uncached) | Alloc/op | Retained while alive |
|---|---|---:|---:|---:|
| 15 | small | 5.8 ms | 44.4 MB | 42.2 MB |
| 15 | highcard | 33.5 ms | 46.0 MB | 42.5 MB |
| 3 | small | 0.28 ms | 1.5 MB | 1.3 MB |
| 3 | highcard | 7.9 ms | 11.7 MB | 9.8 MB |

Construction overhead vs the cached path: about 4 ms at L15 on a small payload (cold 5.8 ms vs repeated about 1.8 ms in the doc); under 0.3 ms at L3 on a small payload. On highcard the construction is lost in the encode time. The existing `cold` sub-benchmark already builds the uncached path (`newZstdEncoder`, `EncodeAll`, `Close`), so it is the right method for the doc row's "construction time per call" and "alloc". Gaps for the new doc row:
- The doc has no **peak** column. `Alloc/op` is cumulative allocation, a close proxy for one encode, not true peak. Maintainer must choose a definition (Alloc/op, runtime/metrics sampling, or cgroup `memory.peak` in a subprocess).
- The "retained after call" figure for the new mode is expected to be about 0, and must be measured with `heapAllocAfterGC()` after the encoder is dropped. The current `cold` benchmark measures retained while the encoder is alive (`runtime.KeepAlive`), so a new sub-benchmark is needed.
- Refresh the doc header (klauspost v1.19.2 vs v1.20.0 in `go.mod`).

## 8. Vocabulary

- **Encoder profile:** `EncoderProfile` value that picks the zstd encoder shape (worker count, buffer sizing). Runtime only, never serialized.
- **Shared encoder cache:** `zstdEncoders` map. Holds one built encoder per (zstd mode, profile) for the process life.
- **Retained / residency:** heap still allocated after GC while the cached encoder sits idle (42.5 MB at L15 bounded).
- **Peak:** most memory live during one encode. Not yet a column in the benchmark doc.
- **Level / collapsed level:** `CompressionLevel` 0-19; the cache collapses it to 4 zstd modes, so 15 and 19 share an entry.
- **Bounded-memory profile:** one worker + `WithLowerEncoderMem`; same bytes as default.
- **Worker:** one inner zstd encoder struct. `EncodeAll` uses exactly one.
- **GINu / GINc:** 4-byte magics for the uncompressed and zstd-compressed payload (`serialize.go:109-110`).
- **Single-block / multi-block payload:** body at or under vs over zstd's 128 KB block size; multi-block exercises the history-buffer path where `lowMem` matters (test at `serialize_profile_test.go:55`).
- **Uncached / transient encoder:** an encoder built for one call and dropped; never put in the cache.
- **Byte-identical:** acceptance rule that all profiles give the same compressed bytes at each level.

## Decisions needed from the maintainer

1. API shape: (a) third profile (recommended), (b) option, or (c1) release function. Final name.
2. Confirm uncached means one worker + low-memory (no "default + uncached").
3. Define "peak" for the doc row and whether to re-run the whole table or add only the new rows.
4. Concurrency: document only, or add a limiter?
5. Include a CLI flag now, or library-only?
6. Decoder retention: measure and file a follow-up, or ignore?
7. Test seam: accept an unexported hook in production code for the weak-pointer test, or settle for cache-absence + `HeapAlloc` threshold only.

## Project constraints (from CLAUDE.md)

Functional options with error-returning options; `github.com/pkg/errors` (`errors.Wrap/Errorf`, no `fmt.Errorf %w`); no breaking API changes; no wire format change; Makefile targets `test`, `lint`; do exactly what is asked, no extra refactors; edits go through a GSD workflow.
