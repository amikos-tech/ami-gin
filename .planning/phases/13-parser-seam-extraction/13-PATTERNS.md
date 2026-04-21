# Phase 13: Parser Seam Extraction - Pattern Map

**Mapped:** 2026-04-21
**Files analyzed:** 8 (7 new + 1 modified)
**Analogs found:** 8 / 8 (all have in-tree analogs; zero-greenfield phase)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `parser.go` (NEW) | interface + BuilderOption | request-response | `docid.go` (interface) + `builder.go:105` (WithCodec) | exact |
| `parser_sink.go` (NEW) | interface adapter (private) | request-response | `builder.go:473-654` (stage* methods) | exact |
| `parser_stdlib.go` (NEW) | default impl (JSON walker) | streaming | `builder.go:322-447` (parseAndStageDocument + stageStreamValue + decodeTransformedValue) | exact — code-move |
| `parser_parity_test.go` (NEW) | test harness (property + matrix) | batch / request-response | `integration_property_test.go` | exact |
| `parity_goldens_test.go` (NEW, build-tagged) | test utility (generator) | file-I/O (write-only) | `benchmark_test.go` (helpers) + stdlib `os.WriteFile` | role-match |
| `testdata/parity-golden/*.bin` (NEW, 7 files) | fixture data | n/a | none (first goldens in repo) | no analog — greenfield |
| `testdata/parity-golden/README.md` (NEW) | docs | n/a | none | no analog |
| `builder.go` (MODIFIED) | builder core | request-response | self (pre-refactor) | self-modify — keep signatures stable |

## Pattern Assignments

---

### `parser.go` (NEW — interface + WithParser BuilderOption)

**Role:** Defines `Parser` (exported interface), `parserSink` (package-private interface), `WithParser(Parser) BuilderOption`.

**Analog 1 — interface + named-impl layout:** `docid.go` (entire file, 73 LOC)

Why it's the closest match: single-file pattern where one interface (`DocIDCodec`) is defined next to one or more concrete implementations (`IdentityCodec`, `RowGroupCodec`) — each with a `Name() string` method used as a stable identifier. Parser's `Name()` requirement (D-06) maps 1:1 onto `DocIDCodec.Name()`. This is the template.

**Imports pattern (docid.go:1):**
```go
package gin
```
No imports — the interface is standalone, no stdlib or third-party deps needed in the types file. `parser.go` will need `github.com/pkg/errors` only for the `WithParser` nil-check; consider splitting if keeping parser.go import-free is desired.

**Name() convention (docid.go:31-33, 66-68):**
```go
func (c *IdentityCodec) Name() string {
    return "identity"
}
// ...
func (c *RowGroupCodec) Name() string {
    return "rowgroup"
}
```
Copy pattern: `stdlibParser.Name()` returns `"stdlib"` (D-06). Short, stable, lowercase, no version — exactly matches the `DocIDCodec` convention.

**Zero-field struct with value receiver (suggested by `IdentityCodec` struct shape, docid.go:14):**
```go
type IdentityCodec struct{}
```
`IdentityCodec` is a zero-field struct. `stdlibParser` should follow the same shape (`type stdlibParser struct{}`) — pointer receivers would force heap allocation via interface boxing (Pitfall 5 in 13-RESEARCH.md). Note: `IdentityCodec` actually uses pointer receivers (`*IdentityCodec`) in its methods; `stdlibParser` should use **value receivers** instead — diverges from this analog for a documented reason (hot-path alloc avoidance).

**Analog 2 — BuilderOption nil-check precedent:** `builder.go:105-113`

**Nil-check + setter pattern (builder.go:105-113):**
```go
func WithCodec(codec DocIDCodec) BuilderOption {
    return func(b *GINBuilder) error {
        if codec == nil {
            return errors.New("codec cannot be nil")
        }
        b.codec = codec
        return nil
    }
}
```
Copy verbatim for `WithParser` — rename `codec` → `p`, `DocIDCodec` → `Parser`, `"codec cannot be nil"` → `"parser cannot be nil"`. D-06 explicitly names this as the precedent.

---

### `parser_sink.go` (NEW — `*GINBuilder` parserSink method impls)

**Role:** Thin adapters: each `parserSink` method forwards to today's existing private `stage*` method on `*GINBuilder`, preserving signatures exactly.

**Analog:** `builder.go:473-654` (`stageScalarToken`, `stageMaterializedValue`, `stageJSONNumberLiteral`, `stageNativeNumeric`)

**Existing staging method signatures to forward to (verbatim — do not rename):**

| Sink method (D-01) | Forwards to (existing) | Source line |
|--------------------|------------------------|-------------|
| `BeginDocument(rgID int) *documentBuildState` | `newDocumentBuildState(rgID)` + stash on `b.currentDocState` | builder.go:85-90 |
| `StageScalar(state, canonicalPath, token any) error` | `b.stageScalarToken(canonicalPath, token, state)` | builder.go:473 |
| `StageJSONNumber(state, canonicalPath, raw string) error` | `b.stageJSONNumberLiteral(canonicalPath, raw, state)` | builder.go:598 |
| `StageNativeNumeric(state, canonicalPath, v any) error` | `b.stageNativeNumeric(canonicalPath, v, state)` | builder.go:629 |
| `StageMaterialized(state, path, value, allowTransform) error` | `b.stageMaterializedValue(path, value, state, allowTransform)` | builder.go:497 |
| `ShouldBufferForTransform(canonicalPath) bool` | `len(b.config.representations(canonicalPath)) > 0` | gin.go:397 + builder.go:438-441 |

**Core forwarding pattern (derived — mirror `WithCodec` 1-line-function style):**
```go
// parser_sink.go
func (b *GINBuilder) StageScalar(state *documentBuildState, canonicalPath string, token any) error {
    return b.stageScalarToken(canonicalPath, token, state)
}

func (b *GINBuilder) StageJSONNumber(state *documentBuildState, canonicalPath, raw string) error {
    return b.stageJSONNumberLiteral(canonicalPath, raw, state)
}

func (b *GINBuilder) ShouldBufferForTransform(canonicalPath string) bool {
    return len(b.config.representations(canonicalPath)) > 0
}
```

**Critical signature-alignment note (Pitfall 3 in 13-RESEARCH.md):**

Read `builder.go:473-654` directly before drafting the sink interface. Today's methods split responsibility:
- `stageScalarToken(canonicalPath, token, state)` — takes **already-normalized** path (builder.go:473).
- `stageMaterializedValue(path, value, state, allowTransform)` — takes **raw** path, normalizes inside via `normalizeWalkPath(path)` at builder.go:498.

Sink signatures must match this exactly, per-method — do not unify. The interface doc-comment should call out which methods expect canonical vs raw paths (suggested convention: name the parameter `canonicalPath` when pre-normalized, `path` when raw).

**State-stash field (new field on GINBuilder struct):**

Per 13-RESEARCH.md §Pattern 1 Open Question #2, add `currentDocState *documentBuildState` to `GINBuilder` (builder.go:20). `BeginDocument` sets it; `AddDocument` reads it after `Parse` returns. Safe because builder is single-threaded (ARCHITECTURE invariant).

```go
func (b *GINBuilder) BeginDocument(rgID int) *documentBuildState {
    s := newDocumentBuildState(rgID)
    b.currentDocState = s
    return s
}
```

---

### `parser_stdlib.go` (NEW — default JSON parser)

**Role:** Wraps today's `json.Decoder.UseNumber()` walk. Code moves verbatim from `builder.go:322-447`.

**Analog (code source):** `builder.go:322-447` — three functions move verbatim:
1. `parseAndStageDocument` (builder.go:322-334) → `stdlibParser.Parse`
2. `stageStreamValue` (builder.go:345-419) → `stdlibParser.streamValue` (unexported method)
3. `decodeTransformedValue` (builder.go:438-447) → inlined into `streamValue`, delegating config lookup to `sink.ShouldBufferForTransform`

**Imports pattern (builder.go:1-15 minus builder-only deps):**
```go
import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "sort"

    "github.com/pkg/errors"
)
```
Match builder.go's order: stdlib (alphabetical) → blank line → third-party. Drop `math`, `strconv`, `strings`, `xxhash` — those stay in `builder.go` with the classifier.

**Core parse pattern (builder.go:322-334 — move verbatim, adjust receiver and state-threading):**
```go
// builder.go:322 — CURRENT
func (b *GINBuilder) parseAndStageDocument(jsonDoc []byte, rgID int) (*documentBuildState, error) {
    decoder := json.NewDecoder(bytes.NewReader(jsonDoc))
    decoder.UseNumber()

    state := newDocumentBuildState(rgID)
    if err := b.stageStreamValue(decoder, "$", state); err != nil {
        return nil, err
    }
    if err := ensureDecoderEOF(decoder); err != nil {
        return nil, errors.Wrap(err, "failed to parse JSON")
    }
    return state, nil
}
```
**Post-refactor shape (parser_stdlib.go):**
```go
func (s stdlibParser) Parse(jsonDoc []byte, rgID int, sink parserSink) error {
    decoder := json.NewDecoder(bytes.NewReader(jsonDoc))
    decoder.UseNumber()

    state := sink.BeginDocument(rgID)
    if err := s.streamValue(decoder, "$", state, sink); err != nil {
        return err
    }
    if err := ensureDecoderEOF(decoder); err != nil {
        return errors.Wrap(err, "failed to parse JSON")
    }
    return nil
}
```
Differences: receiver `s stdlibParser` (value, zero-field), state comes from `sink.BeginDocument`, no state returned (stashed on builder via sink impl), `ensureDecoderEOF` stays as a package-level helper in builder.go (Claude's discretion per 13-CONTEXT.md).

**Transform-buffering hook pattern (builder.go:345-351 + 438-447 — control-flow mirror):**

CURRENT (two separate calls — buffer-check then token-read):
```go
// builder.go:345-351
func (b *GINBuilder) stageStreamValue(decoder *json.Decoder, path string, state *documentBuildState) error {
    canonicalPath := normalizeWalkPath(path)
    if transformed, handled, err := b.decodeTransformedValue(decoder, canonicalPath); err != nil {
        return errors.Wrapf(err, "parse transformed subtree at %s", canonicalPath)
    } else if handled {
        return b.stageMaterializedValue(path, transformed, state, true)
    }

    token, err := decoder.Token()
    // ...
}

// builder.go:438-447
func (b *GINBuilder) decodeTransformedValue(decoder *json.Decoder, canonicalPath string) (any, bool, error) {
    if len(b.config.representations(canonicalPath)) == 0 {
        return nil, false, nil
    }
    value, err := decodeAny(decoder)
    if err != nil {
        return nil, false, err
    }
    return value, true, nil
}
```

POST-REFACTOR (control flow mirrors exactly, config lookup moves to sink):
```go
func (s stdlibParser) streamValue(decoder *json.Decoder, path string, state *documentBuildState, sink parserSink) error {
    canonicalPath := normalizeWalkPath(path)

    // Buffering hook — MUST happen BEFORE decoder.Token() (Pitfall 2).
    if sink.ShouldBufferForTransform(canonicalPath) {
        value, err := decodeAny(decoder)
        if err != nil {
            return errors.Wrapf(err, "parse transformed subtree at %s", canonicalPath)
        }
        return sink.StageMaterialized(state, path, value, true)
    }

    token, err := decoder.Token()
    // ... rest unchanged from builder.go:353-418, but
    //     stageScalarToken → sink.StageScalar
    //     stageMaterializedValue → sink.StageMaterialized
}
```

**Object/array walk pattern (builder.go:358-418 — move verbatim, swap sink calls):**

Critical refactor note (from 13-RESEARCH.md §Pattern 1 "Key refactor note"): when a `{` or `[` delim is hit, today's code does NOT recurse via `stageStreamValue` again — it materializes each child via `decodeAny` and stages via `stageMaterializedValue`. The post-refactor parser must preserve this exactly:

```go
// builder.go:362-378 (object case — preserve this structure)
case '{':
    objectValues := make(map[string]any)
    for decoder.More() {
        keyToken, err := decoder.Token()
        // ...
        value, err := decodeAny(decoder)  // materialize subtree via json.Decoder.Decode
        // ...
        objectValues[key] = value
    }
    for _, key := range sortedObjectKeys(objectValues) {
        if err := b.stageMaterializedValue(path+"."+key, objectValues[key], state, true); err != nil {
            // post-refactor: sink.StageMaterialized(state, path+"."+key, objectValues[key], true)
            return err
        }
    }
```

Do not "improve" by recursing into `streamValue` for children — that changes the stage-call order and breaks byte parity.

**Error handling pattern (builder.go:347, 354, 367, 386 — use errors.Wrapf with path context):**
```go
return errors.Wrapf(err, "parse transformed subtree at %s", canonicalPath)
return errors.Wrap(err, "read JSON token")
return errors.Wrapf(err, "read object key at %s", canonicalPath)
return errors.Errorf("non-string object key at %s", canonicalPath)
```
Copy verbatim. Per D-07, `AddDocument` does NOT wrap these — they reach the caller with the wrap messages unchanged.

**Anti-patterns to avoid (from 13-RESEARCH.md §Anti-Patterns):**
- DO NOT call `strconv.ParseInt` or `strconv.ParseFloat` inside `parser_stdlib.go`. Pitfall 1 — classifier stays in builder.go.
- DO NOT call `json.Number.Float64()` on scalars — pass `.String()` to `StageJSONNumber` so the raw source text reaches the classifier.
- DO NOT use pointer receiver on `stdlibParser`. Value receiver on zero-field struct avoids heap alloc when boxed into `Parser` interface.

---

### `parser_parity_test.go` (NEW — four-dimension parity harness)

**Role:** Test harness covering D-05 dims #1-#4. Merge gate.

**Analog:** `integration_property_test.go` (lines 1-150) — existing shape-mirror gopter property test pattern.

**Imports pattern (integration_property_test.go:1-12):**
```go
package gin

import (
    "encoding/json"
    "strconv"
    "strings"
    "testing"

    "github.com/leanovate/gopter"
    "github.com/leanovate/gopter/gen"
    "github.com/leanovate/gopter/prop"
)
```
For parity test, add `bytes`, `os`, `path/filepath` — needed for goldens loading. Drop `strconv`/`strings` unless authored fixtures need them.

**Gopter property skeleton (integration_property_test.go:16-51):**
```go
func TestPropertyIntegrationFullPipelineNoFalseNegatives(t *testing.T) {
    properties := gopter.NewProperties(propertyTestParameters())

    properties.Property("query results superset of actual matches", prop.ForAll(
        func(docs []TestDoc, queryNameIdx int) bool {
            if len(docs) == 0 {
                return true
            }
            numRGs := len(docs)
            builder, _ := NewBuilder(DefaultConfig(), numRGs)
            for i, doc := range docs {
                _ = builder.AddDocument(DocID(i), doc.JSON)
            }
            idx := builder.Finalize()
            rgSet := idx.Evaluate([]Predicate{EQ("$.name", queryValue)})
            // ... assertion ...
            return true
        },
        GenTestDocs(50),
        gen.IntRange(0, 4),
    ))

    properties.TestingRun(t)
}
```
Copy pattern for `TestParserParity_GopterByteIdentical` — adapt to the byte-equality assertion (D-05 dim #2).

**Generators to reuse (generators_test.go — VERBATIM, no new generators):**

| Generator | Line | Purpose in parity harness |
|-----------|------|---------------------------|
| `GenJSONDocument(maxDepth)` | generators_test.go:87 | Arbitrary-shape JSON bytes |
| `GenTestDocs(maxCount)` | generators_test.go:303 | name/age/active/status fixtures |
| `GenTestDocsWithNulls(maxCount)` | generators_test.go:343 | Null-bearing docs (exercises null path) |
| `GenMixedTypeDocs(maxCount)` | generators_test.go:371 | Constrained-value docs for multi-predicate tests |

Example call (integration_property_test.go:46-48 + 130):
```go
GenTestDocs(50),          // 50-doc slices
GenMixedTypeDocs(50),     // mixed-type variant
```

**Round-trip byte-equality assertion pattern (integration_property_test.go:82-94):**
```go
encoded, err := Encode(original)
if err != nil {
    return true
}
decoded, err := Decode(encoded)
if err != nil {
    return false
}
// ... compare ...
```
Adapt to goldens comparison: compare `encoded` (from `stdlibParser` path) against bytes loaded from `testdata/parity-golden/<name>.bin`. Use `bytes.Equal(encoded, golden)`.

**12-operator Evaluate matrix pattern — reuse from `gin_test.go`:**

Existing fixtures at gin_test.go lines 139, 180, 1116, 2733, 2791, 2832 already exercise each of the 12 operators. Per D-05 dim #3, lift their predicate sets and wire them into a single table-driven test:
```go
cases := []struct {
    name    string
    pred    Predicate
    wantRGs []int
}{
    {"EQ-match", EQ("$.name", "alice"), []int{0, 2}},
    {"NE-match", NE("$.name", "alice"), []int{1, 3}},
    {"GT-match", GT("$.age", 25.0), []int{/* ... */}},
    // ... 12 operators × {match, prune} = 24 cases ...
}
```

**Goldens-loader helper (new — no analog, but follows stdlib os.ReadFile idiom):**
```go
func loadGolden(t *testing.T, name string) []byte {
    t.Helper()
    path := filepath.Join("testdata", "parity-golden", name+".bin")
    b, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("load golden %s: %v", name, err)
    }
    return b
}
```

---

### `parity_goldens_test.go` (NEW — build-tagged goldens regenerator)

**Role:** Run only under `//go:build regenerate_goldens` to write fresh golden blobs. Normal `go test ./...` never compiles this file.

**Analog:** No in-tree build-tagged files exist (`grep -r "//go:build" --include="*.go"` returns nothing for this repo). Pattern comes from stdlib convention + 13-RESEARCH.md §Pattern 3.

**Build tag pattern (canonical Go idiom — 13-RESEARCH.md §Pattern 3 example):**
```go
//go:build regenerate_goldens

package gin

import (
    "os"
    "path/filepath"
    "testing"
)

func TestRegenerateParityGoldens(t *testing.T) {
    dir := filepath.Join("testdata", "parity-golden")
    if err := os.MkdirAll(dir, 0o755); err != nil {
        t.Fatal(err)
    }
    for _, fx := range authoredParityFixtures() {
        builder, _ := NewBuilder(fx.Config(), fx.NumRGs)
        for i, doc := range fx.JSONDocs {
            _ = builder.AddDocument(DocID(i), doc)
        }
        idx := builder.Finalize()
        encoded, _ := Encode(idx)
        path := filepath.Join(dir, fx.Name+".bin")
        if err := os.WriteFile(path, encoded, 0o644); err != nil {
            t.Fatal(err)
        }
        t.Logf("wrote %s (%d bytes)", path, len(encoded))
    }
}
```

**Fixture sharing pattern:** `authoredParityFixtures()` is defined in `parser_parity_test.go` (non-tagged). The regen file imports no new types; both files live in `package gin` and share the fixture list. Standard Go practice.

---

### `testdata/parity-golden/*.bin` + `README.md` (NEW — 7 authored golden blobs)

**Role:** Committed byte-blob fixtures pinning v9-encoded `Encode()` output.

**Analog:** None — first goldens directory in repo. Pattern comes from stdlib `testdata/` convention (Go's standard location for test data) + 13-RESEARCH.md §Pattern 3 README template.

**Fixture list (from 13-RESEARCH.md §Open Questions #3 + D-05 dim #1/#4):**

| Fixture name (file: `<name>.bin`) | Content focus |
|-----------------------------------|---------------|
| `int64-boundaries` | MaxInt64, -MaxInt64, 2^53+1, 0 |
| `nulls-and-missing` | Explicit nulls vs absent paths |
| `deep-nested` | Object/array recursion |
| `unicode-keys` | Non-ASCII keys, requires `NormalizePath` |
| `empty-arrays` | `[]`, `[[], []]` edge cases |
| `large-strings` | Trigram-index stress |
| `transformers-iso-date-and-lower` | WithISODateTransformer + WithToLowerTransformer (dim #4) |

**README.md template (13-RESEARCH.md §Pattern 3):**
```markdown
# Parser Parity Goldens

Byte-level goldens pinning `Encode()` output for the `stdlibParser` path.
Generated from the v1.0 tag; regenerated only when serialization format
changes (v10+).

## Regenerate

```bash
git checkout v1.0
go test -tags regenerate_goldens -run TestRegenerateParityGoldens .
git checkout -
git add testdata/parity-golden/*.bin
git commit -m "chore(parity): refresh goldens to vN"
```

## Format

v9 encoded index bytes (see `gin.go:29 Version = uint16(9)`). One file per
authored fixture; names match `authoredParityFixtures()` in `parser_parity_test.go`.
```

---

### `builder.go` (MODIFIED — add fields, swap AddDocument call-site, move parse bodies out)

**Role:** Self-modification — add two fields, one default-assignment at NewBuilder, swap one call at AddDocument, delete three function bodies whose content moved to `parser_stdlib.go`.

**Analog:** Self — existing `builder.go` with minimal, strictly-additive changes.

**Change 1 — add fields to `GINBuilder` struct (current shape at builder.go:20-36):**
```go
type GINBuilder struct {
    config     GINConfig
    numRGs     int
    // ... existing fields ...
    poisonErr  error
    // NEW — Phase 13:
    parser           Parser
    parserName       string
    currentDocState  *documentBuildState // set by BeginDocument; read by AddDocument (single-threaded safe)
}
```

**Change 2 — default parser in `NewBuilder` (current at builder.go:115-141):**
```go
// builder.go:135-138 CURRENT (after options loop)
for _, opt := range opts {
    if err := opt(b); err != nil {
        return nil, err
    }
}
// INSERT HERE (before `return b, nil`):
if b.parser == nil {
    b.parser = stdlibParser{}
}
name := b.parser.Name()
if name == "" {
    return nil, errors.New("parser name cannot be empty")
}
b.parserName = name
return b, nil
```

**Change 3 — swap AddDocument call-site (current at builder.go:299-302):**
```go
// CURRENT (builder.go:299-302):
state, err := b.parseAndStageDocument(jsonDoc, pos)
if err != nil {
    return err
}
return b.mergeDocumentState(docID, pos, exists, state)

// POST-REFACTOR:
if err := b.parser.Parse(jsonDoc, pos, b); err != nil {
    return err  // D-07: verbatim, no wrapping
}
return b.mergeDocumentState(docID, pos, exists, b.currentDocState)
```

**Change 4 — delete function bodies moved to parser_stdlib.go:**
- `parseAndStageDocument` (builder.go:322-334) — DELETE, moved to `stdlibParser.Parse`
- `stageStreamValue` (builder.go:345-419) — DELETE, moved to `stdlibParser.streamValue`
- `decodeTransformedValue` (builder.go:438-447) — DELETE, logic inlined + config lookup moves to `ShouldBufferForTransform`

**Change 5 — KEEP unchanged (do not touch):**
- `walkJSON` (builder.go:314-320) — used by transformers, not on the parse path
- `ensureDecoderEOF` (builder.go:336-343) — stays as package-level helper; called by `stdlibParser.Parse`
- `decodeAny` (builder.go:421-427) — stays; called by `stdlibParser.streamValue`
- `sortedObjectKeys` (builder.go:429-436) — stays; called by `stdlibParser.streamValue`
- `normalizeWalkPath` (builder.go:307-312) — stays; called inside stage methods AND by the parser
- `stageScalarToken`, `stageMaterializedValue`, `stageJSONNumberLiteral`, `stageNativeNumeric`, `stageCompanionRepresentations`, `parseJSONNumberLiteral`, `stagedNumericFromValue`, `stageNumericObservation` — all stay verbatim (classifier lives here per Pitfall 1)

**Change 6 — add sink method adapters:**
Put in `parser_sink.go` (new file), NOT in builder.go — keeps the diff focused and the review surface small.

---

## Shared Patterns

### Error handling with `github.com/pkg/errors`
**Source:** `builder.go:14` (import), `builder.go:108, 117, 124, 331` (usage examples)
**Apply to:** All new source files that emit errors (`parser.go`, `parser_stdlib.go`)

```go
import "github.com/pkg/errors"

// Usage:
return errors.New("parser cannot be nil")                    // new error
return errors.Wrap(err, "failed to parse JSON")              // wrap with context
return errors.Wrapf(err, "parse transformed subtree at %s", path) // formatted wrap
return errors.Errorf("position %d exceeds numRGs %d", pos, b.numRGs) // formatted new
```

Conforms to project CLAUDE.md convention. Do NOT use `fmt.Errorf("%w", ...)` — deprecated per CLAUDE.md.

### BuilderOption functional-options convention
**Source:** `builder.go:103-113` (`type BuilderOption` + `WithCodec`)
**Apply to:** `WithParser` in `parser.go`

```go
type BuilderOption func(*GINBuilder) error

func WithX(x T) BuilderOption {
    return func(b *GINBuilder) error {
        if x == nil {
            return errors.New("x cannot be nil")
        }
        b.x = x
        return nil
    }
}
```

Last-wins for duplicate supplies (implicit from option-order application at builder.go:135-139). No explicit duplicate detection. Match exactly.

### Package-private state on GINBuilder
**Source:** `builder.go:20-36` (struct shape)
**Apply to:** `currentDocState` field addition

All GINBuilder fields are lowercase (package-private). New fields follow: `parser`, `parserName`, `currentDocState` — all unexported. Do NOT export.

### Test file naming
**Source:** Project convention from `.planning/phases/00-research-assets/` Technology-Stack discovery — "Specialized test files named by type: `property_test.go`, `benchmark_test.go`, `generators_test.go`, `integration_property_test.go`"
**Apply to:** `parser_parity_test.go`, `parity_goldens_test.go`

New test files follow `<subject>_<type>_test.go` pattern where appropriate, or `<subject>_test.go` for co-location with the subject file. Both chosen names fit.

### Import ordering (gci)
**Source:** `.golangci.yml` + observed in `builder.go:3-15`, `integration_property_test.go:3-12`
**Apply to:** All new `.go` files

```
stdlib imports (alphabetical)
<blank line>
third-party imports (alphabetical)
<blank line>
github.com/amikos-tech/ami-gin imports (if any)
```

Enforced by `gci` formatter (project-wide). `make lint` checks this.

---

## Cross-Cutting Behavioral Invariants

These are not "patterns to copy" but **contracts the refactor MUST NOT break**. Listed here so planner can include explicit verification tasks.

| Invariant | Source | Verification |
|-----------|--------|--------------|
| BUILD-03 int64 exact fidelity (values up to MaxInt64 round-trip exactly) | `builder.go:598-627` classifier | `TestNumericIndexPreservesInt64Exactness` (gin_test.go:2988-3233) + `int64-boundaries` golden |
| Transformer-buffering happens BEFORE `decoder.Token()` on the subtree root | `builder.go:347-351` (pre-refactor control flow) | `transformers-iso-date-and-lower` golden (D-05 dim #4) |
| `stdlibParser` is zero-alloc when boxed into `Parser` interface | Pitfall 5 in 13-RESEARCH.md | `go test -bench=BenchmarkAddDocument -benchmem -count=10` ≤2% ns/op, 0 extra allocs/op |
| Exported API surface unchanged except for `Parser` + `WithParser` | PROJECT.md constraint | Grep diff of exported symbols; CI would catch via downstream consumers |
| `ensureDecoderEOF` trailing-content check preserved | `builder.go:336-343` | Existing builder parse tests stay green |

---

## No Analog Found

| File | Role | Reason |
|------|------|--------|
| `testdata/parity-golden/*.bin` | test fixture (binary) | First goldens in repo; no prior binary test fixture pattern. Follows stdlib `testdata/` convention. |
| `testdata/parity-golden/README.md` | docs | First testdata README; template from 13-RESEARCH.md §Pattern 3. |
| `parity_goldens_test.go` build-tag | test utility | No in-tree `//go:build` files currently; standard Go idiom applied. |

All three are greenfield additions with no existing analog; planner should reference 13-RESEARCH.md §Pattern 3 (Build-Tag-Gated Goldens Regeneration) directly.

---

## Metadata

**Analog search scope:**
- Codebase root (all `.go` files in `package gin`)
- `testdata/` (none found — no prior binary fixtures)
- `.github/workflows/` (no build-tagged test precedent)
- 13-CONTEXT.md §Code Anchors (all 7 file references verified via Read)

**Files scanned:** 8 source files (`docid.go`, `builder.go`, `integration_property_test.go`, `generators_test.go`, `benchmark_test.go`, `gin.go` §representations) + 13-CONTEXT.md + 13-RESEARCH.md

**Pattern extraction date:** 2026-04-21

**Pattern quality:** 5 of 8 new files have exact in-tree analogs (code-move refactor). 3 files (goldens + README + build-tagged test) are greenfield but follow documented Go-stdlib idioms. Zero files require novel pattern invention.
