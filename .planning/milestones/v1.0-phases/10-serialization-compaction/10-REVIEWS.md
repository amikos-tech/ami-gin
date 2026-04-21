---
phase: 10
reviewers: [gemini, claude]
reviewed_at: 2026-04-17T13:13:08Z
plans_reviewed:
  - .planning/phases/10-serialization-compaction/10-01-PLAN.md
  - .planning/phases/10-serialization-compaction/10-02-PLAN.md
  - .planning/phases/10-serialization-compaction/10-03-PLAN.md
---

# Cross-AI Plan Review — Phase 10

## Gemini Review

# Review: Phase 10 — Serialization Compaction

This review covers implementation plans **10-01**, **10-02**, and **10-03**.

## Summary
The proposed plans provide a highly professional and disciplined approach to wire-format compaction. By leveraging the existing `PrefixCompressor` and introducing a localized "smaller-wins" fallback mechanism, the design achieves significant size reduction on sorted lexicons without over-engineering the index into a global dictionary. The strategy of separating metadata from string payloads within sections (Parallel Metadata Pattern) is architecturally sound and preserves the eager-decode performance characteristics of the GIN index.

## Strengths
- **Deterministic Fallback:** The `writeOrderedStrings` helper's ability to choose between `raw` and `front-coded` modes at the section level ensures that high-entropy or random-like data (e.g., UUIDs) does not suffer a size penalty compared to the v8 format.
- **Parallel Metadata Preservation:** In the `PathDirectory`, only names are compacted while metadata remains in a parallel, fixed-width-friendly stream. This simplifies decoding and avoids mixing compression markers with critical pruning metadata.
- **Security-First Hardening:** The inclusion of `expectedCount` validation in the decoder and the addition of specific corruption tests for the compact block layout demonstrate strong defensive programming.
- **Evidence-Backed Validation:** Plan 10-03 explicitly addresses the "Evidence Bar" by reporting raw-wire gains separately from zstd gains, preventing the downstream compressor from masking the efficiency of the front-coding.

## Concerns
- **Wire Layout Reordering (MEDIUM):** Transitioning from interleaved `[term, bitmap]` pairs to `[term-block] + [bitmap-block]` is a material change to the serialization flow. While correct for compression, any off-by-one error in the decoder will silently corrupt term-to-rowgroup bindings. *Mitigation: The planned round-trip tests in Task 2 specifically check RG alignment, which is the correct guard.*
- **Version History Documentation (LOW):** As the project approaches v9, ensuring the `gin.go` comments stay exhaustive is vital for long-term maintenance of the rebuild-only policy.
- **Config Fallback (LOW):** Task 1 mentions falling back to default block sizes for hand-built indexes. Ensure this logic is robust if `idx.Config` is nil, as `prefix.go` needs a valid `blockSize > 0`.

## Suggestions
- **Prefix Block Size Tuning:** Consider including a sub-benchmark in 10-03 that sweeps `PrefixBlockSize` (e.g., 8, 16, 32) on the `HighPrefix` fixture to validate if the current default of 16 is truly the "sweet spot" for this specific index structure.
- **Error Context:** When `readOrderedStrings` fails due to a count mismatch, include both `got` and `want` in the `github.com/pkg/errors` message to aid in debugging corrupt Parquet sidecars.
- **Empty List Optimization:** Ensure `writeOrderedStrings` handles a zero-length slice by writing a simple header (count=0) and exiting, avoiding any allocation in the compressor.

## Risk Assessment: LOW
The risk is low because the implementation is localized to the serialization layer and does not touch the query evaluation logic. The "decode-once" model acts as a natural safety barrier; as long as the round-trip parity tests pass, the rest of the system remains unaffected by the change in the wire bytes. The planned hardening and benchmark evidence provide sufficient safety and proof of value to proceed.

---

## Claude Review

# Phase 10 Plan Review

## Summary

The three plans form a clean split across implementation (10-01), hardening (10-02), and evidence (10-03) that mirrors both the research's recommended shape and the locked decisions in CONTEXT.md. The dependency graph is correct (10-02 and 10-03 fan out from 10-01) and the scope is genuinely conservative — compact bytes inside existing section boundaries, no global dictionary, no query-path changes. However, there is **one high-severity correctness issue** in 10-01 (`PrefixCompressor.Compress` sorts its input and will corrupt `PathDirectory` binding), and several ambiguities around helper signatures, baseline computation, and benchmark timing that will surface during execution if not resolved first.

---

## Plan 10-01 — Compact wire layout

### Strengths

- Explicit v8→v9 bump keeps the rebuild-only contract loud and testable.
- Section-mode marker pattern (`compactStringModeRaw` / `compactStringModeFrontCoded`) matches Research Pattern 5 and gives a clean fallback for random-like corpora.
- Reuses existing `PrefixCompressor` / `WriteCompressedTerms` / `ReadCompressedTerms` instead of hand-rolling a second compressor (Don't Hand-Roll table).
- Round-trip tests are scoped tightly to the new surface and called out as `CompressionNone` in 10-02, which makes byte-level mutation tests tractable.

### Concerns

- **HIGH — `PrefixCompressor.Compress` sorts its input.** Looking at `prefix.go:56–58`, `Compress` does `sort.Strings(sorted)` before blocking. This is fine for `StringIndex.Terms` and `AdaptiveStringIndex.Terms` (already lexically sorted — see `NewAdaptiveStringIndex`), but **`PathDirectory` names are ordered by `PathID` (insertion order), not lexically**. `rebuildPathLookup` enforces `entry.PathID == uint16(i)`. If `writeOrderedStrings` feeds `PathDirectory[].PathName` through `PrefixCompressor.Compress`, the decompressed names will come back sorted and `readPathDirectory` will bind metadata to the wrong names. The plan's behavior clause asserts "exact input order, never resorts" — but the primitive it mandates does resort. This needs to be resolved before Task 1 starts (either add an order-preserving `Compress` variant, presort the builder to emit `PathDirectory` lexically and reindex `PathID`s, or write path names as a separate lex-sorted stream plus an ID remap table).

- **MEDIUM — `writeOrderedStrings(w io.Writer, values []string)` signature can't see config.** The action text says "use `idx.Config.PrefixBlockSize` when available and falling back to the default block size", but the signature takes only `(w, values)`. Either thread `blockSize int` through the signature, or make it a method on something that already carries config, or document that it reads a package-level default. Decoded indexes constructed outside `NewConfig` will also hit this path.

- **MEDIUM — "Compare raw vs front-coded payload bytes before choosing a mode" requires double-encoding.** The helper must serialize both layouts to compare byte lengths (`CompressionStats` only reports payload bytes, not the `WriteCompressedTerms` framing overhead of `uint32 blocks + uint16 firstLen + uint16 entries + uint16 prefixLen + uint16 suffixLen` per block/entry). The plan should specify either (a) write to a `bytes.Buffer` twice and pick the smaller, or (b) an explicit cheap estimator that accounts for the framing overhead. Without this, implementers will underestimate front-coded payload size and pick the wrong mode on borderline cases.

- **LOW — Task 1 commits a version bump without changing on-wire layout.** This is atomically safe (no test fixtures on disk), but it means the repo briefly ships a v9 header with v8 section layout. If a contributor encodes with Task 1 landed and decodes after Task 2 lands, they get an `ErrInvalidFormat`. This is acceptable between internal task commits; worth noting so Task 1 and Task 2 ship in the same PR.

### Suggestions

- Resolve the `PrefixCompressor` sort issue first. Cheapest fix: add `CompressInOrder(terms []string)` that skips the internal sort, since all callers (finalized builder, decoded index) already guarantee their input order — and add a test that blocks re-sorting regressions.
- Change the helper signature to `writeOrderedStrings(w io.Writer, values []string, blockSize int)` and let callers pass `max(cfg.PrefixBlockSize, defaultBlockSize)`. Explicit > implicit.
- Specify the mode-selection mechanism: "encode to `bytes.Buffer` in both modes, write the smaller". This is also the honest semantics for the 10-02 hardening tests.

### Risk

**MEDIUM** — the sort issue is a clear bug waiting to happen, but it's detectable with the `TestPathDirectoryCompactionRoundTrip` test already scoped in Task 2. The test must assert name-to-PathID binding, not just "same names present".

---

## Plan 10-02 — Hardening & compatibility

### Strengths

- Corruption tests target both path and term sections, exercising both ordered-string entry points.
- Reuses the existing `TestDecodeVersionMismatch` / `TestDecodeLegacyRejected` anchors rather than creating parallel coverage.
- Representation alias parity test is the right guard against Pitfall 4 (Phase 10 accidentally changing Phase 09 public semantics).

### Concerns

- **MEDIUM — Boundary between 10-01 hardening and 10-02 hardening is fuzzy.** 10-01 Task 1 says `readOrderedStrings` rejects unknown modes and count mismatches; 10-02 Task 1 says "finish any remaining bounds checks". If 10-01 is correct, there should be nothing left for 10-02 to add in production code — only tests. Tighten 10-02 Task 1's action to be tests-only plus any bounds-check additions discovered **during** 10-02 (e.g., block count caps introduced by `WriteCompressedTerms`). As written, the split invites duplicated work or drift.

- **MEDIUM — No test that mode-selection is deterministic on equal-size payloads.** If raw and front-coded produce identical byte counts, which wins? Without a tiebreak rule and test, two encoders on the same data could emit different bytes, which breaks byte-stable round-trip expectations. Pick a rule ("prefer raw on tie" or "prefer front-coded on tie") in 10-01 and regression-test it here.

- **LOW — Mode-marker mutation coverage.** Task 1's corruption case should include a payload where the marker says `compactStringModeFrontCoded` but the following bytes are a valid raw payload (and vice versa). This is the classic "mode/payload disagreement" case and is qualitatively different from a truncated payload.

- **LOW — `TestDecodeLegacyRejected` upkeep after every version bump is toil.** Consider parametrizing the "rejected versions" list so future phases add one number rather than editing assertions.

### Suggestions

- Reframe 10-02 Task 1 as "tests only; if any hardening gap is discovered, patch it here with the test that found it, and note it as a deviation from 10-01's scope."
- Add a "mode lies about payload" mutation case.
- Add a tiebreak rule to 10-01 and a test here.

### Risk

**LOW** — this is a well-shaped hardening wave that mostly exercises existing patterns. The 10-01/10-02 scope split is the only real ambiguity.

---

## Plan 10-03 — Benchmark evidence

### Strengths

- Fixture matrix (Mixed / HighPrefix / RandomLike) directly satisfies D-07.
- Separates `compact_raw_bytes` from `default_zstd_bytes`, which is the specific concern in D-08 and Pitfall 2.
- Pairs size with encode/decode cost plus post-decode query smoke — addresses D-09.

### Concerns

- **HIGH — `legacy_raw_bytes` formula is under-specified.** Task 1 says "exact pre-Phase-10 raw-string formula for path and term payloads" but doesn't write it out. After 10-01 lands, the old code is gone. The formula for path directory names is `sum(2 + len(name))` per `PathEntry`, plus fixed metadata bytes that don't change. For classic string indexes it's `sum(2 + len(term))` per term, excluding RG bitmaps. For adaptive, same plus promoted-term bytes only. Implementers without this spec will likely (a) forget one of the three sections, (b) accidentally include RG bitmap bytes, or (c) include header/bloom/trigram bytes and produce a meaningless percentage. **Write the formula into the plan, ideally as a helper function the bench calls.**

- **MEDIUM — `-benchtime=1x` gives one sample.** For `compact_raw_bytes`/`legacy_raw_bytes` it's fine (deterministic bytes). For encode/decode cost it's useless — Go benchmarking needs `-benchtime=1s` or `-benchtime=Ns` minimum to produce stable timing. Either drop cost-timing claims, switch verification to `-benchtime=1s -count=3`, or explicitly flag timing as "smoke / qualitative" in the plan. Validation (`10-VALIDATION.md:33`) uses `-benchtime=1x` too — worth aligning both.

- **MEDIUM — Sub-bench naming `Fixture=Mixed`.** Go benchmark sub-name convention is `/` (e.g., `b.Run("Mixed", ...)` producing `BenchmarkX/Mixed`). `Fixture=Mixed` works with `-bench` filters, but breaks `benchstat` aggregation. Prefer `b.Run("Mixed", ...)` and rely on `BenchmarkPhase10SerializationCompaction` as the fixture-group anchor.

- **LOW — "include adaptive probe if the fixture includes one" is too soft.** Either the fixture builder guarantees an adaptive path (then require the probe) or it doesn't (then don't promise coverage). Remove the conditional — make one fixture include adaptive+representation by construction.

- **LOW — No threshold for success.** The plan reports percentages but doesn't define "meaningful reduction". Success Criteria #4 on the ROADMAP ("clear encoded-size reduction") should map to something like "≥ X% reduction on HighPrefix under `CompressionNone`, non-regression on RandomLike under `Encode()`". Without a threshold, future readers can't tell whether the evidence passed.

### Suggestions

- Add the baseline formula explicitly. A helper like `legacyRawPathTermBytes(idx)` in the bench file, documented, would remove ambiguity and become the regression anchor.
- Change verification to `-benchtime=1s -count=3 -benchmem` (or commit explicitly to 1x + "size-only" claims).
- Use `b.Run("Mixed", ...)` sub-bench style.
- Set an explicit threshold for "clear reduction" on at least the HighPrefix fixture.

### Risk

**MEDIUM** — the evidence shape is right but the under-specified baseline and 1x timing will produce numbers that look rigorous without actually being reproducible. These are fixable with small edits before execution.

---

## Overall Risk Assessment

**MEDIUM**

The plans capture the correct decomposition and honor the locked decisions from CONTEXT.md well. Two issues need edits before 10-01 starts: the `PrefixCompressor` sort behavior (HIGH — hard correctness bug if left), and the `writeOrderedStrings` signature/config source. Two issues should be fixed before 10-03 starts: the `legacy_raw_bytes` formula, and the `-benchtime=1x` cost-timing mismatch. 10-02 is healthy but its scope boundary with 10-01 should be tightened.

None of these are structural — the phase shape, fixture matrix, rebuild-only policy, and section-local strategy are all sound and consistent with the research. Fixing the above before execution removes the ambiguity that would otherwise surface as executor deviations or flaky benchmark numbers.

---

## Consensus Summary

Both reviewers agree the phase is well-shaped: localized compaction inside the existing serialization sections, explicit rebuild-only versioning, and a separate evidence wave are the right overall design.

### Agreed Strengths

- The phase decomposition is sound: implementation in `10-01`, hardening in `10-02`, and benchmark proof in `10-03`.
- The localized compaction strategy avoids scope creep into a global dictionary or query-path redesign.
- The plan correctly treats raw-wire and zstd-compressed size as separate signals and pairs compaction with round-trip and corruption coverage.

### Agreed Concerns

- Order preservation is the critical correctness boundary. Compacting path and term string payloads must not break path-to-metadata binding or term-to-`RGSet` alignment after decode.
- The block-size/config story should be explicit. The helper surface and default/fallback behavior need to be spelled out so hand-built or nil-config indexes do not drift into ambiguous behavior.
- Benchmark evidence should stay concrete and reproducible. The raw baseline, compressed-size reporting, and encode/decode measurement methodology need tighter specification before execution.

### Divergent Views

- Gemini rated the overall plan risk as `LOW`; Claude rated it `MEDIUM`.
- Claude raised a concrete pre-execution correctness concern: `PrefixCompressor.Compress` sorts input today, which would be incompatible with preserving `PathDirectory` order if reused directly for path-name compaction.
- Claude also called out two plan-level ambiguities not mentioned by Gemini: the `legacy_raw_bytes` formula is under-specified, and `-benchtime=1x` is not a stable basis for encode/decode timing claims.
