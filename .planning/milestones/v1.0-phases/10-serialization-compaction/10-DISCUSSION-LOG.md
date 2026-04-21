# Phase 10: Serialization Compaction - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-17
**Phase:** 10-serialization-compaction
**Areas discussed:** Compatibility policy, Compaction aggressiveness, Decode cost budget, Proof bar

---

## Compatibility policy

| Option | Description | Selected |
|--------|-------------|----------|
| A. Strict rebuild-only | Keep the current `Decode()` contract and require rebuilds on a format-version bump. | ✓ |
| B. Read previous format | Decode the immediately previous wire format but only write the new compact one. | |
| C. Broad multi-version decode | Maintain a wider compatibility surface across multiple historical formats. | |

**User's choice:** `1A`
**Notes:** The follow-up research did not surface a strong reason to expand compatibility in Phase 10. The repo already documents rebuild-only behavior, and the research-backed conclusion was that adding compatibility logic here would buy smoother upgrades at the cost of a materially larger and riskier serialization surface.

---

## Compaction aggressiveness

| Option | Description | Selected |
|--------|-------------|----------|
| A. Targeted front coding | Compact sorted path names and term lists with block-based prefix compression using the existing direction in `prefix.go`. | ✓ |
| B. Front coding plus extra tuning | Add more aggressive section-specific tuning on top of front coding. | |
| C. Global shared dictionary | Introduce a denser cross-section string table / offset design. | |

**User's choice:** `2A`
**Notes:** Research reinforced that block-based front coding is a standard fit for sorted dictionaries. The Stanford IR reference and Lucene term-dictionary design both support the chosen direction. The Parquet `VARIANT` spec was useful as a contrast case for more aggressive dictionary/offset designs, but that style was judged too invasive for this repo's current sectioned layout.

---

## Decode cost budget

| Option | Description | Selected |
|--------|-------------|----------|
| A. Low decode overhead | Take clear size wins without materially changing decode/query startup behavior. | ✓ |
| B. Smallest payloads first | Push harder on bytes even if decode becomes materially heavier. | |
| C. Split policy | Keep one section lightweight and make another more aggressive. | |

**User's choice:** `3A`
**Notes:** Research strengthened this choice because the repo decodes the full index into in-memory structures before querying. That makes JSONB- or `VARIANT`-style on-wire random-access optimizations a poorer fit than a simple compaction layer that disappears after `Decode()`.

---

## Proof bar

| Option | Description | Selected |
|--------|-------------|----------|
| A. Minimal proof | One representative fixture plus one stress fixture and parity checks. | |
| B. Broader matrix | Validate on multiple fixture families and size/query metrics. | ✓ |
| C. Size-only proof | Focus mostly on bytes with a few spot query checks. | |

**User's choice:** `4B`
**Notes:** After the external research request, this was the right call. Front-coding gains depend heavily on prefix locality, so a single happy-path fixture would be weak evidence. The research-backed refinement is to include at least a representative mixed fixture, a high-shared-prefix fixture, and a low-shared-prefix/random-like fixture, and to report both `CompressionNone` and default zstd output sizes.

---

## Research Notes

- External inspiration reviewed: FloeDB article on binary JSON design and the broader storage/random-access trade-off space.
- Standard dictionary-compression reference reviewed: Stanford IR lecture on blocking plus front coding for sorted lexicons.
- Mature implementation contrast reviewed: Lucene BlockTree terms dictionary.
- More aggressive alternative reviewed: Apache Parquet `VARIANT` shared-dictionary and variable-width offset design.

## the agent's Discretion

- Exact block header/wire layout for compact path and term sections.
- Exact raw-section fallback heuristic when a given string set does not compress smaller under front coding.
- Exact benchmark names, fixture shapes, and acceptance thresholds inside the broader matrix chosen above.

## Deferred Ideas

- Backward-compatible decode for previous wire versions.
- Global shared string tables spanning multiple index sections.
- A self-contained binary subtree format aimed at random access on compact bytes instead of decode-once in-memory querying.
