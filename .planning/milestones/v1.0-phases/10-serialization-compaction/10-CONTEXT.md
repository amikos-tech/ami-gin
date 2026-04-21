# Phase 10: Serialization Compaction - Context

**Gathered:** 2026-04-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Shrink the encoded path and term storage now that the Phase 08 and Phase 09 functional layout is stable. This phase covers compact representation of sorted path names and string terms, explicit wire-format version handling, and benchmark proof that encoded indexes get smaller without query regressions. It does not add a migration layer for older formats, redesign the in-memory query model, or turn the file format into a self-contained random-access binary JSON structure.

</domain>

<decisions>
## Implementation Decisions

### Compatibility policy
- **D-01:** Phase 10 keeps the current strict rebuild-only compatibility policy. A compact-format version bump may reject older payloads rather than reading the immediately previous format.
- **D-02:** Format-version behavior must stay explicit and test-covered. The compact format should fail clearly with rebuild guidance instead of silently attempting best-effort compatibility logic.

### Compaction approach
- **D-03:** Use targeted block-based prefix compression/front coding for sorted `PathDirectory` names and sorted string term lists instead of a broader format redesign.
- **D-04:** Do not introduce a global shared string table, cross-section indirection layer, or Parquet-`VARIANT`-style metadata dictionary in this phase.

### Decode and runtime budget
- **D-05:** Optimize for clear size reduction with low decode overhead. Query evaluation should continue to run against the fully materialized in-memory index after `Decode()`.
- **D-06:** Avoid layouts whose main benefit depends on random access into the compact wire bytes themselves. This repo decodes the whole index once, so Phase 10 should not trade significant decode complexity for on-wire access patterns the runtime does not use.

### Evidence bar
- **D-07:** The proof bar should be broader than a single fixture. At minimum, benchmark and size-check a mixed representative fixture, a high-shared-prefix fixture, and a low-shared-prefix or random-like fixture.
- **D-08:** Report size deltas for both the raw wire layout (`CompressionNone`) and the default compressed output (`Encode()` / zstd), because front coding can materially shrink the uncompressed payload while producing smaller final gains after zstd.
- **D-09:** The size matrix must be paired with round-trip parity and query-regression checks, and it should report encode/decode cost deltas so Phase 10 does not hide startup regressions behind smaller bytes.

### the agent's Discretion
- Exact compact block layout for each section, including whether terms are written as a separate compressed subsection or in block-local groups adjacent to RG bitmap payloads.
- Exact default block size and whether a section may fall back to raw string encoding when block compression is not smaller on that section's data shape.
- Exact benchmark helper names, fixture generators, and reporting thresholds, as long as the locked broader proof bar is met.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and acceptance
- `.planning/ROADMAP.md` — Phase 10 goal, success criteria, and dependency sequencing after Phases 08 and 09.
- `.planning/REQUIREMENTS.md` — `SIZE-01` through `SIZE-03`, which define compact path encoding, compact term encoding, and explicit version handling.
- `.planning/PROJECT.md` — milestone-level constraints: protect correctness, keep format evolution explicit, and back size claims with benchmarks.
- `.planning/STATE.md` — carry-forward concern that Phase 10 must keep binary format evolution explicit and testable.

### Prior-phase constraints that still apply
- `.planning/phases/08-adaptive-high-cardinality-indexing/08-CONTEXT.md` — additive config/versioning discipline and adaptive string layout that Phase 10 must compact without changing semantics.
- `.planning/phases/09-derived-representations/09-CONTEXT.md` — explicit representation metadata and the constraint not to reopen the Phase 09 public alias contract for compaction convenience.
- `.planning/phases/09-derived-representations/09-03-SUMMARY.md` — documents that Phase 10 affects the finalized Phase 09 serialization shape and should preserve its public/docs contract.

### Existing implementation surfaces
- `prefix.go` — existing block-based front-coding primitive, on-disk block writer/reader helpers, and compression-stat helpers.
- `serialize.go` — current sectioned wire format, strict version rejection, and the path/string/adaptive writer/reader pairs that Phase 10 will compact.
- `gin.go` — binary format version history, `PathEntry`, `StringIndex`, `AdaptiveStringIndex`, and existing `PrefixBlockSize` config surface.
- `builder.go` — confirms that paths and term lists are already sorted before serialization, which makes front coding a natural fit without adding new sort passes.
- `benchmark_test.go` — current prefix-compression component benchmarks plus encode/decode/size benchmark scaffolding to extend for Phase 10 evidence.
- `serialize_security_test.go` — version mismatch, bounds, and malformed-payload regression coverage that must expand for the compact format.
- `README.md` — current public contract that `Decode()` rejects older wire versions and callers rebuild indexes across format changes.

### External design references
- `https://floedb.ai/blog/why-json-isnt-a-problem-for-databases-anymore` — original inspiration discussed during Phase 10; useful for understanding sorted-key layouts, relative-offset structures, and the broader design space between local layout simplicity and global dictionaries.
- `https://web.stanford.edu/class/cs276/19handouts/lecture4-compression-1per.pdf` — standard IR reference showing blocking plus front coding on sorted dictionaries and the associated space/lookup trade-off.
- `https://lucene.apache.org/core/5_5_5/core/org/apache/lucene/codecs/blocktree/BlockTreeTermsWriter.html` — mature block-based term dictionary design that groups terms by shared prefixes rather than introducing a fully global string table.
- `https://parquet.apache.org/docs/file-format/types/variantencoding/` — reference for a more aggressive alternative using shared dictionaries and variable-width offsets; useful as a contrast case rather than the target design for this repo.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `prefix.go`: `PrefixCompressor`, `WriteCompressedTerms`, `ReadCompressedTerms`, and `CompressionStats` already implement the core front-coding primitive Phase 10 needs.
- `serialize.go`: section-by-section writer/reader functions already isolate the path directory, classic string indexes, and adaptive string indexes, so compaction can stay localized.
- `benchmark_test.go`: existing size and throughput benchmarks, plus dedicated prefix-compression microbenchmarks, provide a direct harness for SIZE-01 and SIZE-02 evidence.

### Established Patterns
- `builder.go` finalizes paths in lexicographic order, and both classic and adaptive string indexes sort their terms before storing them; Phase 10 can exploit this existing sorted order directly.
- `Decode()` fully materializes a `GINIndex` and rebuilds lookup state before queries run. This means on-wire compaction affects encode/decode cost, not query hot-path string lookup logic.
- Wire-format evolution is already explicit and strict: `Version` is tracked in `gin.go`, `readHeader()` rejects mismatches, and `README.md` documents rebuild-only compatibility.
- Serialization is sectioned by `pathID`, not centered around a single shared dictionary object, so a local compaction strategy fits the existing layout better than a cross-section indirection layer.

### Integration Points
- `writePathDirectory()` / `readPathDirectory()` — replace raw repeated path strings with compact block encoding while preserving metadata ordering and validation.
- `writeStringIndexes()` / `readStringIndexes()` — compact sorted classic term lists without changing the term-to-`RGSet` pairing semantics.
- `writeAdaptiveStringIndexes()` / `readAdaptiveStringIndexes()` — compact promoted adaptive terms while leaving bucket RG bitmaps semantically unchanged.
- `serialize_security_test.go`, `gin_test.go`, and `benchmark_test.go` — extend round-trip, version, malformed-payload, size, and encode/decode regression coverage for the new wire layout.

</code_context>

<specifics>
## Specific Ideas

- The extra research pass confirmed the user-selected direction rather than refuting it: strict version gating, targeted front coding, low decode overhead, and a broader validation matrix are coherent with both the repo architecture and standard dictionary-compression practice.
- The strongest support for targeted front coding came from the IR literature and Lucene: sorted lexicons are a standard fit for block-based front coding, and mature search systems keep the compaction local to term blocks rather than forcing a global shared-string redesign.
- The strongest caution came from the same research: front coding helps most when sorted strings share prefixes. Random-like IDs and low-prefix corpora can show smaller gains or even raw-block wins, which is why the broader proof matrix and optional raw-section fallback matter.
- The original FloeDB article is still a useful inspiration, but it points more toward the overall design space than toward the exact Phase 10 implementation. Its stronger alternatives, like shared dictionaries and relative-offset structures, make more sense for self-contained random-access binary documents than for this repo's decode-once in-memory GIN layout.
- Parquet `VARIANT` is a useful contrast case: it uses shared metadata dictionaries and variable-width offsets for nested semi-structured values. That design confirms there are denser options available, but it also reinforces that those options come with more indirection and a different runtime target than this index format needs.

</specifics>

<deferred>
## Deferred Ideas

- Reading prior wire versions alongside the new compact format — intentionally deferred because Phase 10 keeps strict rebuild-only compatibility.
- Global cross-section string tables or variable-width dictionary/offset metadata — intentionally deferred because they would turn Phase 10 into a broader format redesign.
- JSONB- or `VARIANT`-style self-contained random-access binary subtree encoding — out of scope for this pruning index, which decodes into in-memory structures before query evaluation.

</deferred>

---

*Phase: 10-serialization-compaction*
*Context gathered: 2026-04-17*
