---
phase: 09
phase_name: "derived-representations"
project: "GIN Index — v1.0 Query & Index Quality"
generated: "2026-04-17T00:00:00Z"
counts:
  decisions: 5
  lessons: 3
  patterns: 4
  surprises: 3
missing_artifacts:
  - "09-VERIFICATION.md"
---

# Phase 09 Learnings: derived-representations

## Decisions

### Derived Indexing Stays Additive
Derived representations were added alongside raw indexing instead of replacing the raw source value.

**Rationale:** The phase needed raw semantics to remain available while optimized representations became queryable companions.
**Source:** 09-01-SUMMARY.md

---

### Companion Failures Are Hard Errors
Failed companion derivation was treated as an explicit `AddDocument()` error instead of silently falling back to raw-only indexing.

**Rationale:** Silent fallback would weaken the feature contract and allow partial document merges that misrepresent indexed behavior.
**Source:** 09-01-SUMMARY.md

---

### Public Alias Queries Must Be Explicit
Companion queries were routed through `gin.As(alias, value)` while raw-path queries kept their existing default behavior.

**Rationale:** This preserved the raw path as the primary public contract and avoided ambiguous or accidental routing to hidden companion paths.
**Source:** 09-02-SUMMARY.md

---

### Representation Metadata Is Stored Explicitly
Alias bindings were persisted as explicit representation metadata in a dedicated v7 serialization section instead of being inferred from hidden target-path naming.

**Rationale:** Explicit metadata keeps decode behavior deterministic and avoids fragile name-based reconstruction.
**Source:** 09-02-SUMMARY.md

---

### Public Documentation Teaches One Contract
The README and examples were rewritten to use alias-aware helper registrations and `gin.As(...)`, and the replacement-era examples were removed instead of documenting both models side by side.

**Rationale:** A single public story reduces confusion and matches the shipped semantics more clearly than a dual-contract explanation.
**Source:** 09-03-SUMMARY.md

---

## Lessons

### Numeric Parity Tests Must Respect Documented Bounds
Decode-parity coverage cannot use integers beyond the supported exact-float boundary.

**Context:** An initial parity fixture used a value larger than `1<<53`, which implied a broader numeric guarantee than the library actually supports.
**Source:** 09-02-SUMMARY.md

---

### Strict Companion Rules Change Fixture Design
Acceptance fixtures for derived regex fields must ensure every indexed source value matches the configured extraction pattern.

**Context:** A non-matching regex fixture correctly failed indexing once strict companion-derivation rules were in place, so the test data had to be corrected to reflect the intended contract.
**Source:** 09-03-SUMMARY.md

---

### Executor Stalls Need Fast Local Recovery
Execution should resume locally after validating branch state when an orchestration subagent stalls without returning a checkpoint or completion marker.

**Context:** The initial `gsd-executor` subagent for Plan 01 never returned, and progress continued only after confirming the expected additive-config work was already present and no conflicting edits had landed.
**Source:** 09-01-SUMMARY.md

---

## Patterns

### Fan Out Companions From One Prepared Value
Stage the raw source path unchanged, then derive each sibling companion from the same prepared source value rather than chaining one transform into the next.

**When to use:** Use this when multiple derived views of the same field must coexist and registration order must not affect semantics.
**Source:** 09-01-PLAN.md

---

### Keep Internal Storage Paths Behind a Public Wrapper
Use hidden internal target paths for storage and verification first, then expose a separate public alias API instead of promoting the internal path format directly.

**When to use:** Use this when internal representation naming is implementation detail and the public query surface should remain stable.
**Source:** 09-01-SUMMARY.md

---

### Rebuild Alias Lookup in Both Fresh and Decoded Indexes
Populate the same alias-lookup structure after `Finalize()` and after `Decode()` from explicit representation metadata.

**When to use:** Use this when query behavior must be identical for in-memory indexes and round-tripped serialized indexes.
**Source:** 09-02-SUMMARY.md

---

### Pair Raw and Alias Evidence in Public Examples
Examples and acceptance tests should demonstrate both raw-path queries and explicit alias queries against the same source field.

**When to use:** Use this when a feature adds companion behavior without removing the original public contract, and documentation needs to prove both forms remain valid.
**Source:** 09-03-SUMMARY.md

---

## Surprises

### Hidden Paths Needed Special Decode Handling
Internal `__derived:` paths could not be treated like normal public JSONPaths during lookup rebuild.

**Impact:** The decode/rebuild path needed an explicit bypass so internal representation paths survived round trips without being normalized through the public JSONPath flow.
**Source:** 09-01-PLAN.md

---

### Fresh Finalize Needed Alias Lookup Too
Alias routing was not only a decode concern; a freshly finalized in-memory index also needed representation lookup rebuilt before queries ran.

**Impact:** Without this, alias queries would have behaved differently before and after encode/decode, which would have broken the public contract.
**Source:** 09-02-PLAN.md

---

### Public Docs Could Not Reuse Old Examples Safely
The replacement-era `WithFieldTransformer` examples were incompatible enough with the new feature that they had to be removed, not incrementally adapted in place.

**Impact:** README, examples, and acceptance coverage all had to be rewritten together so users would not learn a mixed or obsolete contract.
**Source:** 09-03-PLAN.md
