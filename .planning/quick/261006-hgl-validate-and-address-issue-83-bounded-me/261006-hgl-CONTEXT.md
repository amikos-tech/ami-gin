# Quick Task 261006-hgl: validate and address issue 83 - Context

**Gathered:** 2026-10-06
**Status:** Implemented and verified on 2026-10-06. E14 is closed: the maintainer approved the draft and it is filed as issue 87. Shipped as PR 86.
**Issue:** https://github.com/amikos-tech/ami-gin/issues/83
**Research:** `261006-hgl-RESEARCH.md`
**Grilling log (all proposed answers and recommendations):** `261006-hgl-GRILLING.jsonl`

<domain>
## Task boundary

Give callers an opt-in way to compress an index with a one-worker, low-memory zstd encoder that the library builds for one `Encode` call and drops after it. The library must not put that encoder in the shared encoder cache.

## Problem in one paragraph

`EncoderProfileBoundedMemory` keeps its encoder in a process-wide map (`zstdEncoders`, `serialize.go:295-327`). Production code never removes an entry. At level 15 the idle encoder holds about 42.5 MB. A service that encodes often wants this. A batch job that encodes one time per batch, in a 512 MiB container, does not: the 42.5 MB stays live through all later steps, and the Go collector lets the heap grow to about two times the live size before it collects.

## Terms (no ONTOLOGY.md exists in this repo; these come from the code and docs)

| Term | Meaning |
|---|---|
| Encoder | The zstd object that compresses the index body. It owns match tables and a window buffer. |
| Encoder profile | `EncoderProfile` value. It selects the encoder shape. Runtime only, never written to the index. |
| Default profile | One worker per `GOMAXPROCS`. About 34 MB per worker at level 15. |
| Bounded-memory profile | One worker plus `WithLowerEncoderMem`. Same output bytes as default. |
| Worker | One inner encoder struct. One `EncodeAll` call uses exactly one. |
| Shared encoder cache | `zstdEncoders` map. One built encoder per (zstd mode, profile), kept for the process life. |
| Retained (residency) | Heap that stays live after GC because the cache still points to the idle encoder. |
| Peak | The most memory live during one encode. |
| Level | `CompressionLevel` 0-19. The cache collapses levels to 4 zstd modes. Levels 15 and 19 share one entry. |
| `GINu` / `GINc` | 4-byte headers for the uncompressed and the compressed payload. |
| Uncached encoder | An encoder built for one call and dropped. Never in the cache. |
| Byte-identical | All profiles give the same compressed bytes at one level. |

## Verified facts (from research)

- F1. The issue is correct. No production release or eviction API exists.
- F2. 42.5 MB retained at level 15 reproduces (42.2-42.5 MB locally).
- F3. `Close()` is a no-op for an encoder used only through `EncodeAll` (klauspost v1.20.0, `encoder.go:589-592`). Dropping the reference frees the memory.
- F4. `EncoderProfile` is not serialized. A new value or field cannot change the wire format.
- F5. An uncached default-profile encoder builds `GOMAXPROCS` workers and uses one (588 MB alloc, 121 ms at 16 CPUs). Uncached is only sensible with one worker.
- F6. Cost of an uncached call at level 15: about 4 ms construction, 44-46 MB allocation per call. Level 3: under 0.3 ms, 1.5-11.7 MB.
- F7. Parquet, sidecar and S3 writers read the profile from `idx.Config` or `...EncodeOption`. A setting carried by the profile reaches them with no new plumbing.
- F8. Doc gaps: `docs/encoder-profile-benchmarks.md` has no peak column, and its header names klauspost v1.19.2 while `go.mod` pins v1.20.0.
- F9. The shared decoder is also never released. Its retained size is not measured.

</domain>

<ledger>
## Question ledger

| # | Round | Question | Options offered | Recommendation | Answer | Decision |
|---|---|---|---|---|---|---|
| P0 | 0 | Does the maintainer confirm the problem statement? | yes / correct it | n/a | "yes, both points confirmed" | Target is retained memory after the call, not the peak. The fix is a library setting. |
| Q1 | 1 | API shape | A third profile value / B separate retention option / C cache release function | A | A | Third profile value on the `EncoderProfile` enum. |
| Q2 | 1 | Concurrent calls (N x 44 MB) | A document only / B package-level limit | A | A | Document the cost. No limiter. |
| Q3 | 1 | Meaning of "peak" in the doc row | A Alloc/op / B sampled live heap / C cgroup memory.peak | A | A | `Alloc/op` from the `cold` benchmark, labelled "allocated per call". |
| Q4 | 1 | Host for the new doc numbers | A new section, this machine / B full rerun here / C maintainer runs on original host | A | A | New section with its own host line. Both modes measured together here. Old tables unchanged. |
| Q5 | 1 | Shared decoder retention | A deferred item / B follow-up issue / C measure now | A | "A (file a follow up)" | Out of scope, deferred item. GitHub issue or not: see Q8. |
| Q6 | 2 | Public name of the new profile | A `EncoderProfileBoundedMemoryUncached` / B `EncoderProfileTransient` / C `EncoderProfileBoundedMemoryOneShot` | A | A | `EncoderProfileBoundedMemoryUncached`, text form `"bounded-memory-uncached"`. |
| Q7 | 2 | Strength of the reachability test | A cache check + heap limit, no production hook / B A plus weak pointer through an unexported hook | A | A | Cache check plus coarse live-heap limit. No test hook in production code. |
| Q8 | 2 | Clarify the Q5 follow-up | A deferred item only / B deferred item plus GitHub issue | B | B | Deferred item plus a GitHub issue. Maintainer sees the title and body first. |

### Resolved without a question (from code and facts)

| # | Topic | Resolution | Basis |
|---|---|---|---|
| R1 | "Default profile, uncached" | Not offered. Uncached always means one worker plus low-memory buffers. | F5 |
| R2 | CLI flag | None. Each `gin-index` command encodes one time and exits, so retained memory has no effect there. | `cmd/gin-index/main.go:113,184` |
| R3 | `Close()` | The code can call it, but docs and tests must not claim that it frees memory. | F3 |
| R4 | Wire format, release type | No format change. Additive API, so a minor release. | F4 |

</ledger>

<decisions>
## Locked decisions

- **D1 (Q1). Shape.** A third value on the `EncoderProfile` enum. The existing `WithEncoderProfile` (config) and `WithEncodeProfile` (per call) accept it. No new option, no new `GINConfig` field.
- **D2 (Q6). Name.** `EncoderProfileBoundedMemoryUncached`. `String()` returns `"bounded-memory-uncached"`. The existing constants keep their values 0 and 1.
- **D3 (R1). Encoder shape.** The uncached encoder has one worker and low-memory buffers, the same as the bounded-memory profile. No "default profile, uncached".
- **D4 (Q2). Concurrent calls.** The library adds no limit. The godoc and README state that N calls at the same time build N encoders (about N x 44 MB at level 15).
- **D5 (Q3, Q4). Benchmark doc.** A new section in `docs/encoder-profile-benchmarks.md` with its own host line. It compares the cached bounded-memory profile and the uncached profile, measured together on this machine. Columns: allocated per call (`Alloc/op`), retained after the call, time per call. The old tables stay as they are.
- **D6 (Q7). Reachability test.** Cache-absence check plus a coarse live-heap limit after GC. No test hook in production code.
- **D7 (Q5, Q8). Decoder.** Out of scope. A deferred item in this task, plus a GitHub issue that the maintainer approves before creation.
- **D8 (R2). CLI.** No new flag.
- **D9 (R3). `Close()`.** Docs and tests do not claim that `Close()` frees memory.
- **D10 (R4). Compatibility.** No wire format change. Additive API. CHANGELOG entry under the next minor release.

### Claude's discretion

- Internal structure of the cache bypass in `serialize.go`.
- The exact heap limit in the test (order of 8 MB against a 42 MB signal).
- The wording of godoc, README and CHANGELOG text, inside D4 and D9.
- The name of the new benchmark sub-case that measures retained memory after the encoder is dropped.

</decisions>

<expectations>
## Expectations (hold-out set for post-implementation validation)

The verifier checks each item against the final code. "Uncached profile" means `EncoderProfileBoundedMemoryUncached`. "Cache" means the shared encoder cache (`zstdEncoders`).

**E1. Same bytes.**
Given an index and a level from `encoderProfileLevels`, with a single-block payload and with a multi-block payload,
When the caller encodes it with the uncached profile,
Then the output is byte-identical to the output of the default profile at that level.

**E2. The cache gets no entry.**
Given a cache with no entry for a level,
When one encode with the uncached profile returns,
Then the cache has no entry for that level under the uncached profile, the bounded-memory profile, or the default profile.

**E3. No retained encoder.**
Given the live heap after GC as a baseline,
When one level-15 encode with the uncached profile returns and GC runs,
Then the live heap is above the baseline by less than the coarse limit (far below the 42 MB that a retained encoder holds).

**E4. One worker.**
Given any `GOMAXPROCS`,
When the library builds an encoder for the uncached profile,
Then the encoder has one worker and low-memory buffers.

**E5. Per-call setting wins in both directions.**
Given an index whose config has the default profile,
When the caller passes `WithEncodeProfile(EncoderProfileBoundedMemoryUncached)` to `EncodeContext` or `EncodeWithLevelContext`,
Then E2 holds for that call.
And given a config with the uncached profile, when the caller passes `WithEncodeProfile(EncoderProfileBoundedMemory)`, then the cache gets a bounded-memory entry.

**E6. Config reaches every encode path.**
Given a `GINConfig` built with `WithEncoderProfile(EncoderProfileBoundedMemoryUncached)`,
When each encode path that `TestEncoderProfileConfigReachesAllEncodePaths` covers runs (plain encode, sidecar, Parquet metadata, rebuild),
Then E2 holds for each path.

**E7. Existing profiles do not change.**
Given the default profile or the bounded-memory profile,
When an encode returns,
Then the cache holds that profile's encoder as before, and `EncoderProfileDefault == 0` and `EncoderProfileBoundedMemory == 1`.

**E8. Validation and text form.**
Given a profile value that is not one of the three constants,
When the caller passes it to `WithEncoderProfile` or to an encode call,
Then the library returns an error whose text names all three valid values.
And `EncoderProfileBoundedMemoryUncached.String()` returns `"bounded-memory-uncached"`.

**E9. Wire format.**
Given an index encoded with the uncached profile,
When `Decode` reads it,
Then decode succeeds, the format `Version` constant is the same as on `main`, and the decoded config has the default profile.

**E10. No compression, no encoder.**
Given the uncached profile and `CompressionNone`,
When the caller encodes,
Then the output starts with `GINu` and the library builds no encoder.

**E11. Concurrent calls are correct.**
Given several goroutines that encode with the uncached profile at the same time,
When all calls return under the race detector,
Then each output is byte-identical to the default-profile output, no race is reported, and E2 holds.

**E12. Docs.**
Given the finished branch,
When a reader opens `docs/encoder-profile-benchmarks.md`, the godoc of the new constant, the README encoder-profile section and the CHANGELOG,
Then: the benchmark doc has a new section with a host line and rows for cached bounded-memory and uncached (allocated per call, retained after the call, time per call); the old tables have no diff; the godoc and README say when not to use the uncached profile (frequent encodes) and state the N x 44 MB cost of concurrent calls; no text says that `Close()` frees memory; the CHANGELOG has an entry.

**E13. Scope limits.**
Given the diff against `main`,
Then it has no new CLI flag, no concurrency limiter, no new option function, no new `GINConfig` field, no test-only hook in non-test files, and no change to decoder code.

**E14. Follow-up.**
Given the end of the task,
Then a deferred item records the unmeasured decoder retention, and the maintainer has seen a draft GitHub issue before its creation.

</expectations>

<canonical_refs>
## Canonical references

- Issue 83, PR 80 (bounded-memory profile), issues 43 and 44 (encoder reuse)
- `docs/encoder-profile-benchmarks.md`
- `serialize.go:200-345`, `gin.go:454-459,843,902`
- `serialize_profile_test.go`

</canonical_refs>
