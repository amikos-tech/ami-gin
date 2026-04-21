# Phase 10: Serialization Compaction - Research

**Researched:** 2026-04-17
**Domain:** Compact wire encoding for sorted paths and terms in a Go JSON GIN index
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md and discussion log)

Everything in this block is copied or restated from `.planning/phases/10-serialization-compaction/10-CONTEXT.md`. [CITED: .planning/phases/10-serialization-compaction/10-CONTEXT.md]

### Locked Decisions

#### Compatibility policy
- **D-01:** Phase 10 keeps the current strict rebuild-only compatibility policy. A compact-format version bump may reject older payloads rather than reading the immediately previous format.
- **D-02:** Format-version behavior must stay explicit and test-covered. The compact format should fail clearly with rebuild guidance instead of silently attempting best-effort compatibility logic.

#### Compaction approach
- **D-03:** Use targeted block-based prefix compression/front coding for sorted `PathDirectory` names and sorted string term lists instead of a broader format redesign.
- **D-04:** Do not introduce a global shared string table, cross-section indirection layer, or Parquet-`VARIANT`-style metadata dictionary in this phase.

#### Decode and runtime budget
- **D-05:** Optimize for clear size reduction with low decode overhead. Query evaluation should continue to run against the fully materialized in-memory index after `Decode()`.
- **D-06:** Avoid layouts whose main benefit depends on random access into the compact wire bytes themselves. This repo decodes the whole index once, so Phase 10 should not trade significant decode complexity for on-wire access patterns the runtime does not use.

#### Evidence bar
- **D-07:** The proof bar should be broader than a single fixture. At minimum, benchmark and size-check a mixed representative fixture, a high-shared-prefix fixture, and a low-shared-prefix or random-like fixture.
- **D-08:** Report size deltas for both the raw wire layout (`CompressionNone`) and the default compressed output (`Encode()` / zstd), because front coding can materially shrink the uncompressed payload while producing smaller final gains after zstd.
- **D-09:** The size matrix must be paired with round-trip parity and query-regression checks, and it should report encode/decode cost deltas so Phase 10 does not hide startup regressions behind smaller bytes.

### Agent Discretion
- Exact block framing for compact path and term sections, as long as the format stays localized and low-risk.
- Exact fallback rule for poor-compression sections, as long as the rule stays explicit and testable.
- Exact fixture helpers and benchmark report naming, as long as the required matrix and regression bar are met.

### Deferred / Out of Scope
- Reading old wire versions alongside the new compact format.
- Global cross-section dictionaries or offset tables.
- Random-access binary JSON / `VARIANT`-style layout redesign.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SIZE-01 | Path directory serialization uses prefix compression or an equivalent compact representation. [CITED: .planning/REQUIREMENTS.md] | `Summary`, `Standard Stack`, and `Architecture Patterns` show `PathDirectory` is currently written as raw repeated strings and is a clean fit for block-local front coding via `prefix.go`. [VERIFIED: serialize.go, prefix.go, gin.go] |
| SIZE-02 | String term serialization uses prefix compression or block compaction instead of raw repeated strings. [CITED: .planning/REQUIREMENTS.md] | `Summary`, `Architecture Patterns`, and `Common Pitfalls` show both classic and adaptive promoted term sections currently emit raw term strings and can be compacted without changing RG bitmap semantics. [VERIFIED: serialize.go, prefix.go, gin.go] |
| SIZE-03 | Compact encoding introduces explicit format-version handling with round-trip coverage for legacy and new index formats. [CITED: .planning/REQUIREMENTS.md] | `Summary`, `Project Constraints`, `Common Pitfalls`, and `Validation Architecture` show the repo already uses strict version rejection, so Phase 10 should bump `Version`, keep rebuild-only behavior explicit, and expand malformed/round-trip coverage rather than add a migration layer. [VERIFIED: gin.go, serialize.go, serialize_security_test.go, README.md] |
</phase_requirements>

## Summary

Phase 10 is a localized wire-format compaction phase, not a storage-engine redesign. The current serialization path already has the right structural separation: `writePathDirectory()`, `writeStringIndexes()`, and `writeAdaptiveStringIndexes()` are isolated writers, and `Decode()` fully reconstructs in-memory indexes before query evaluation runs. That means the safest implementation is to compact the sorted string payloads inside those sections while preserving the existing decoded `GINIndex` shape and query semantics. [VERIFIED: serialize.go, gin.go, query.go]

The main inefficiency is straightforward. `writePathDirectory()` writes every path name as `uint16(len(path)) + raw bytes`, even though path names are sorted and often share long prefixes. `writeStringIndexes()` and `writeAdaptiveStringIndexes()` do the same for sorted term lists, again followed by RG bitmap payloads. The repository already contains a front-coding implementation in `prefix.go` (`PrefixCompressor`, `WriteCompressedTerms`, `ReadCompressedTerms`) that is explicitly designed for sorted strings sharing prefixes. [VERIFIED: serialize.go, prefix.go, builder.go]

**Primary recommendation:** keep the current section boundaries and decoded data structures, bump the binary `Version`, and replace raw repeated path/term string payloads with block-based compact string lists that are decoded eagerly during `Decode()`. Use the existing `PrefixBlockSize` config/default as the block-size control surface, and keep a per-section explicit mode or fallback rule so malformed payload detection stays precise. [VERIFIED: gin.go, serialize.go, prefix.go][CITED: .planning/phases/10-serialization-compaction/10-CONTEXT.md]

This approach aligns with every locked decision:
- It is block-based and localized, not a global dictionary redesign. [CITED: .planning/phases/10-serialization-compaction/10-CONTEXT.md]
- It preserves decode-once runtime behavior because compact bytes are expanded during `Decode()`. [VERIFIED: serialize.go, query.go]
- It works with existing sorted inputs: builder finalization already emits lexicographically ordered path names and term lists. [VERIFIED: builder.go, prefix.go]
- It preserves explicit version discipline because Phase 10 is already modifying the on-wire shape materially and current `Version = 8` cannot describe the compact layout. [VERIFIED: gin.go, serialize_security_test.go]

## Project Constraints

- Follow the repository’s functional-options pattern and additive config discipline; any new compaction toggle or section-mode metadata should remain explicit and versioned, not implicit. [CITED: AGENTS.md][VERIFIED: gin.go]
- Keep using `github.com/pkg/errors` for new encode/decode failures and version-rejection messages. [CITED: AGENTS.md][VERIFIED: serialize.go, gin.go]
- Preserve the milestone contract that binary format evolution remains explicit and test-backed. [CITED: .planning/PROJECT.md, .planning/STATE.md][VERIFIED: gin.go, serialize_security_test.go]
- Do not reopen the Phase 09 derived-representation public contract. Compaction may touch representation-bearing payloads indirectly through path/term storage, but it must not alter alias routing or metadata semantics. [CITED: .planning/phases/09-derived-representations/09-CONTEXT.md, .planning/phases/09-derived-representations/09-03-SUMMARY.md][VERIFIED: gin.go, serialize.go]
- Keep benchmark proof in the repo’s existing Go benchmark harness rather than inventing an external measurement stack. [CITED: AGENTS.md][VERIFIED: benchmark_test.go]
- Keep decode hardening parity with prior phases: every new length/count field introduced by compaction needs bounds checks and negative tests. [VERIFIED: serialize.go, serialize_security_test.go]

## Standard Stack

### Core

- `prefix.go` should be the compaction primitive. `PrefixCompressor`, `WriteCompressedTerms`, and `ReadCompressedTerms` already encode sorted strings into block-based front-coded payloads and decode them back deterministically. [VERIFIED: prefix.go]
- `serialize.go` should remain the orchestration point. The existing writer/reader split for `PathDirectory`, classic string indexes, and adaptive string indexes is already the correct integration surface for Phase 10. [VERIFIED: serialize.go]
- `GINConfig.PrefixBlockSize` should remain the public tuning surface for block size, because it already exists, defaults to `16`, and semantically matches the compact-string use case. [VERIFIED: gin.go]
- `GINIndex` decoded structures (`PathDirectory`, `StringIndex`, `AdaptiveStringIndex`) should remain unchanged at query time; compaction is a wire concern, not a query-path contract change. [VERIFIED: gin.go, query.go]

### Supporting

- `benchmark_test.go` already contains prefix microbenchmarks plus encode/decode and size benchmarks. Extend those harnesses with Phase 10 fixture families instead of creating a new benchmark subsystem. [VERIFIED: benchmark_test.go]
- `serialize_security_test.go` already covers version mismatch, malformed payloads, and round-trip regressions. Extend it for compact sections and the new version bump. [VERIFIED: serialize_security_test.go]
- The existing `CompressionNone` and compression-level benchmark helpers should be reused so the proof matrix reports raw-wire and zstd-compressed gains separately. [VERIFIED: benchmark_test.go, serialize.go]

### Alternatives Locked Out by Context

- Do not add a global string table shared across path and term sections. That would require new indirection layers and broader metadata, which conflicts with D-04. [CITED: .planning/phases/10-serialization-compaction/10-CONTEXT.md]
- Do not keep dual readers for old and new wire formats in this phase. The current format contract is rebuild-only, and strict version mismatch rejection is already documented and tested. [CITED: .planning/phases/10-serialization-compaction/10-CONTEXT.md][VERIFIED: gin.go, serialize_security_test.go, README.md]
- Do not optimize for partial wire reads or random-access decoding. The runtime decodes once into memory, so extra complexity for on-wire access patterns would not pay off here. [CITED: .planning/phases/10-serialization-compaction/10-CONTEXT.md][VERIFIED: query.go, serialize.go]

## Architecture Patterns

### Pattern 1: Compact Path Directory as a Separate Name Stream

**What:** Replace per-entry raw `PathName` writes with a compact sorted-name stream while leaving per-entry metadata (`PathID`, `ObservedTypes`, `Cardinality`, `Mode`, `Flags`) explicit and parallel. [ASSUMED]

**When to use:** `writePathDirectory()` / `readPathDirectory()`. [VERIFIED: serialize.go]

**Why:** Path names are globally sorted and often share long JSONPath prefixes like `$.user.` or `$.orders[*].`. The current encoding repeats those bytes for every path entry. [VERIFIED: builder.go, serialize.go]

**Recommended shape:** write metadata records in path order, but write names through a compact block payload:
1. `numPaths`
2. compact-name payload for all `PathName` values in order
3. parallel per-entry metadata fields in the same order

This keeps decode simple: decode the compact names list first, then bind each decoded name to the following metadata record. It also keeps bounds validation localized to one name stream rather than mixing compression markers into every metadata record. [ASSUMED][VERIFIED: serialize.go, prefix.go]

### Pattern 2: Compact Classic Terms per Path, Leave RG Bitmaps Parallel

**What:** Replace the per-term raw string write loop in `writeStringIndexes()` with a single compact term-list payload per path, followed by the RG bitmaps in decoded term order. [VERIFIED: serialize.go]

**When to use:** Classic exact string indexes only. [VERIFIED: gin.go]

**Why:** `StringIndex.Terms` is already sorted, and RG bitmaps are parallel to term order. Compacting the term list without changing the bitmap ordering preserves all semantics and keeps decode linear. [VERIFIED: gin.go, serialize.go, prefix.go]

**Example insertion point:** today each term is emitted as `len + bytes` immediately before its bitmap. That structure blocks cross-term prefix reuse. Replace it with:
1. pathID
2. term count
3. compact term block payload
4. RG bitmaps in term order

This keeps bitmap I/O unchanged while unlocking term-level front coding. [ASSUMED][VERIFIED: serialize.go]

### Pattern 3: Apply the Same Compact-Term Pattern to Adaptive Promoted Terms

**What:** Use the same compact term-list encoding for `AdaptiveStringIndex.Terms`, but leave bucket RG bitmaps untouched. [VERIFIED: serialize.go, gin.go]

**When to use:** `writeAdaptiveStringIndexes()` / `readAdaptiveStringIndexes()`. [VERIFIED: serialize.go]

**Why:** Adaptive promoted terms are also sorted and frequently share prefixes on the same hot path. Bucket RG bitmaps are not string payloads and do not need compaction in this phase. Reusing one compact-term layout across classic and adaptive sections reduces format surface area and test burden. [VERIFIED: gin.go, serialize.go, prefix.go]

### Pattern 4: Explicit Version Bump with Strict Legacy Rejection

**What:** Bump `Version` from `8` to the next value and keep `readHeader()` strict: if the incoming version is not the exact current one, return `ErrVersionMismatch`. [VERIFIED: gin.go, serialize.go]

**When to use:** As soon as any path or term section encoding changes. [VERIFIED: gin.go]

**Why:** The wire layout is changing materially, and the repo already documents rebuild-only compatibility. Trying to sneak compact decoding under the old version would make payload interpretation ambiguous and weaken existing security tests. [VERIFIED: gin.go, serialize_security_test.go, README.md]

### Pattern 5: Optional Section-Level Raw Fallback for Poor Prefix Gain

**What:** Allow a compactable string section to encode either as front-coded blocks or as raw strings when compact blocks are not smaller for that dataset shape. [ASSUMED]

**When to use:** Low-prefix or random-like path/term sets, especially under `CompressionNone`. [CITED: .planning/phases/10-serialization-compaction/10-CONTEXT.md]

**Why:** Prefix coding is strongest on shared prefixes and weaker on random-like IDs. A section-level explicit mode byte or tagged payload lets Phase 10 keep gains where they exist without forcing larger raw-wire payloads on worst cases. This is consistent with the context’s discretion allowance for per-section raw fallback. [CITED: .planning/phases/10-serialization-compaction/10-CONTEXT.md][VERIFIED: prefix.go, benchmark_test.go]

## Don’t Hand-Roll

| Problem | Don’t Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Compact sorted strings | A new ad hoc compression format | `PrefixCompressor`, `WriteCompressedTerms`, and `ReadCompressedTerms`. [VERIFIED: prefix.go] | The repo already has block-based front coding matched to sorted terms. |
| Benchmark framework | A separate CLI or external harness | Existing Go benchmarks in `benchmark_test.go`. [VERIFIED: benchmark_test.go] | Encode/decode/size and prefix microbenchmarks already live there. |
| Version enforcement | Best-effort compatibility branches | Existing strict `Version` mismatch rejection via `readHeader()`. [VERIFIED: gin.go, serialize.go] | The project already chose rebuild-only compatibility. |
| Query-path optimizations | New lazy-decoding query logic | Existing eager `Decode()` materialization. [VERIFIED: serialize.go, query.go] | The runtime does not query directly against encoded bytes. |

## Common Pitfalls

### Pitfall 1: Compacting Strings but Reordering Their Parallel RG Data

**What goes wrong:** Terms decode in a different order than the RG bitmap slice, so exact-match pruning becomes silently wrong. [ASSUMED]

**Why it happens:** The current layout interleaves term bytes and RG bitmaps, which naturally preserves order. A new compact term block could accidentally sort, deduplicate, or otherwise remap terms independently of the bitmap slice. [VERIFIED: serialize.go, prefix.go]

**How to avoid:** Treat compact term payloads as order-preserving encodings of the existing sorted term slice. Decode the full ordered term list first, then read or bind RG bitmaps in that same order. Add round-trip tests that compare decoded `Terms[i]` to decoded `RGBitmaps[i]` behavior, not just slice lengths. [VERIFIED: gin.go, serialize_security_test.go]

### Pitfall 2: Forgetting That zstd Can Hide Raw-Wire Tradeoffs

**What goes wrong:** A compact raw layout looks much smaller under `CompressionNone`, but the phase overclaims wins because zstd already compressed repeated strings well. [CITED: .planning/phases/10-serialization-compaction/10-CONTEXT.md]

**Why it happens:** Repetition-heavy string payloads are already friendly to downstream compression. Prefix coding can still help, but the gain may narrow materially after zstd. [ASSUMED][VERIFIED: benchmark_test.go]

**How to avoid:** Always report both raw-wire and default compressed sizes, and pair them with encode/decode cost deltas. That is a locked decision, not an optional bonus metric. [CITED: .planning/phases/10-serialization-compaction/10-CONTEXT.md]

### Pitfall 3: Expanding the Wire Surface Without Matching Bounds Checks

**What goes wrong:** New block counts, entry counts, or fallback-mode markers can drive oversized allocations or ambiguous payload parsing. [ASSUMED]

**Why it happens:** Existing decode hardening assumes every new count/length field gets validated explicitly. Compact sections add exactly those kinds of untrusted sizes. [VERIFIED: serialize.go, serialize_security_test.go]

**How to avoid:** Add max validations for block counts and decoded string counts wherever new compact readers are introduced, and extend `serialize_security_test.go` with truncated, oversized, duplicate, and wrong-mode payload cases. [VERIFIED: serialize.go, serialize_security_test.go]

### Pitfall 4: Letting Phase 10 Reopen Phase 09 Semantics

**What goes wrong:** Compaction work starts changing representation metadata, alias routing, or hidden path naming because those strings are now part of the serialized payload. [ASSUMED]

**Why it happens:** Phase 09 established explicit representation metadata and public alias routing, and Phase 10 naturally touches serialized sections that carry those names. [VERIFIED: gin.go, serialize.go, .planning/phases/09-derived-representations/09-03-SUMMARY.md]

**How to avoid:** Constrain Phase 10 to byte layout and versioning. Representation meaning, target selection, and alias contract should remain byte-for-byte semantically equivalent after decode. Add regression tests that compare alias routing before and after compaction-enabled round trips. [VERIFIED: serialize_security_test.go, gin.go]

### Pitfall 5: Optimizing for Random Access the Runtime Never Uses

**What goes wrong:** The design introduces offset tables or sparse seek metadata to support partial reads from encoded bytes, increasing decode complexity for little benefit. [ASSUMED]

**Why it happens:** More aggressive binary formats often optimize for on-wire traversal, but this repo calls `Decode()` once and then queries the in-memory index. [VERIFIED: serialize.go, query.go]

**How to avoid:** Keep the wire layout simple, sequential, and eager-decode friendly. If a feature does not reduce bytes or simplify eager decode, it probably belongs to a different roadmap phase. [CITED: .planning/phases/10-serialization-compaction/10-CONTEXT.md]

## Validation Architecture

Phase 10 needs both correctness and evidence validation. The minimum verification matrix should cover:

1. **Round-trip parity**
   - Encode/decode equality for path directory, classic string indexes, adaptive promoted terms, and representation-aware paths.
   - Query parity on representative fixtures before and after encode/decode.
   - Legacy payload rejection still returns `ErrVersionMismatch`.

2. **Malformed compact payload hardening**
   - Truncated compact name streams.
   - Oversized block counts / term counts.
   - Wrong section-mode markers or mismatched decoded counts.
   - Compact section claims that do not match header/path metadata.

3. **Benchmark evidence**
   - Mixed representative fixture.
   - High-shared-prefix fixture.
   - Low-shared-prefix or random-like fixture.
   - For each fixture: raw-wire size, zstd-compressed size, encode latency, decode latency, and at least one query regression check.

4. **Fallback behavior**
   - If section-level raw fallback is implemented, tests must prove fallback selection is deterministic, round-trippable, and never changes decoded semantics.

**Best fit files and commands:**
- Unit/security tests: `serialize_security_test.go`, `gin_test.go`, and focused benchmark helpers in `benchmark_test.go`. [VERIFIED: serialize_security_test.go, gin_test.go, benchmark_test.go]
- Quick command: `go test ./... -run 'TestDecode|TestEncode|TestSerialization|TestRepresentation' -count=1` [ASSUMED]
- Full suite command: `go test ./... -count=1`
- Benchmark command: `go test -run '^$' -bench 'Benchmark(Prefix|Encode|Decode|EncodedSize|Phase10)' -benchmem`

## Environment Availability

- No new external dependency is required for the recommended design. The repo already contains the prefix-compression primitive, benchmark harness, and decode-hardening patterns Phase 10 needs. [VERIFIED: prefix.go, benchmark_test.go, serialize_security_test.go]
- The main implementation touchpoints are already isolated and small enough for targeted plan slices: `serialize.go`, `benchmark_test.go`, `serialize_security_test.go`, and possibly small helper additions in `prefix.go` or `gin.go`. [VERIFIED: serialize.go, prefix.go, gin.go, benchmark_test.go]

## Recommended Plan Shape

The planner should almost certainly split Phase 10 into three coordinated work areas:

1. **Wire-format implementation**
   - compact path directory encoding
   - compact classic/adaptive term encoding
   - explicit version bump and decode support

2. **Correctness and hardening**
   - round-trip parity
   - legacy rejection and malformed compact payload tests
   - derived-representation regression coverage

3. **Evidence**
   - representative fixture generators
   - raw-wire vs zstd size reporting
   - encode/decode cost and query-regression benchmarks

That split mirrors the phase success criteria cleanly and keeps the benchmark/evidence work from being treated as optional cleanup. [CITED: .planning/ROADMAP.md, .planning/REQUIREMENTS.md]
