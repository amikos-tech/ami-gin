# Phase 13: Parser Seam Extraction - Research

**Researched:** 2026-04-21
**Domain:** Pure refactor — extract JSON-parse boundary in `builder.go` into a pluggable `Parser` interface backed by an unexported `stdlibParser`
**Confidence:** HIGH (existing code read directly; CONTEXT.md locks all interface shape decisions; prior phase evidence for numeric contract verified)

## Summary

Phase 13 is a **zero-behavior-change refactor** of the JSON-parse boundary in `builder.go`. The existing five staging methods on `*GINBuilder` (`stageScalarToken`, `stageMaterializedValue`, `stageJSONNumberLiteral`, `stageNativeNumeric`, `decodeTransformedValue`) plus the recursive walker (`stageStreamValue`) plus the top-level entry (`parseAndStageDocument`) get repartitioned: the **walking/decoding** moves into a new unexported `stdlibParser` type, while **classification-and-staging** stays on `*GINBuilder` reachable through a narrow package-private `parserSink` interface. The BUILD-03 int64-fidelity classifier at `builder.go:598-627` does **not** move — it stays in `builder.go` because the SIMD follow-on (v1.2, Pitfall #1) depends on that single source of truth.

The merge gate is a new `parser_parity_test.go` whose four parity dimensions (authored fixtures, gopter generators, full 12-operator Evaluate matrix, transformer-driven paths) all assert byte-identical `Encode()` output against committed goldens generated from the `v1.0` tag. The goldens live at `testdata/parity-golden/` and are generated via a `//go:build regenerate_goldens` gated test so normal `go test ./...` runs never rewrite them.

Only two new identifiers are exported: `Parser` (interface) and `WithParser` (BuilderOption constructor). `parserSink` and `stdlibParser` stay unexported — this is a documented deviation from ROADMAP §Phase 13 success criterion #3, flagged for verification and consistent with PROJECT.md's "avoid gratuitous API churn" constraint.

**Primary recommendation:** Land the seam in three small commits — (1) add `Parser` + unexported `parserSink` + `stdlibParser` with verbatim code move, (2) switch `AddDocument` to dispatch through the parser and wire `WithParser`, (3) add `parser_parity_test.go` with the goldens. All three commits pass `go test ./...` and `make lint` independently; the parity harness is the merge gate for the PR.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| JSON tokenization / walk | `stdlibParser` (new) | — | Isolated from staging so SIMD can swap in v1.2 |
| Transformer subtree buffering decision | `parserSink.ShouldBufferForTransform` (on `*GINBuilder`) | `stdlibParser` (caller) | Config lookup belongs with config owner; parser just asks |
| Scalar / numeric classification (int64 vs float64) | `*GINBuilder` staging methods (unchanged) | — | BUILD-03 contract; Pitfall #1 — SIMD will hand raw source text via `StageJSONNumber` so classifier stays authoritative |
| Document-state allocation | `parserSink.BeginDocument` (on `*GINBuilder`) | `stdlibParser` | `*documentBuildState` stays package-private; sink creates it, parser threads it |
| Path normalization | `normalizeWalkPath` (free function, `builder.go:307`) | — | Called from stage methods — unchanged |
| Merge into builder pathData | `*GINBuilder.mergeDocumentState` (unchanged) | — | Parser never sees the merge; stays on the builder side of the seam |
| Parser identity (telemetry hook) | `Parser.Name()` cached to `b.parserName` at `NewBuilder` | Phase 14 consumer | Read once, stored on builder; no per-document call |

## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01: ParserSink exposes `*documentBuildState` as an opaque handle.** Sink methods take `state *documentBuildState` as the first arg; parsers never touch its fields. Sink method set:
  ```
  BeginDocument(rgID int) *documentBuildState
  StageScalar(state *documentBuildState, path string, token any) error
  StageJSONNumber(state *documentBuildState, path, raw string) error
  StageNativeNumeric(state *documentBuildState, path string, v any) error
  StageMaterialized(state *documentBuildState, path string, value any, allowTransform bool) error
  ShouldBufferForTransform(path string) bool
  ```
- **D-02: Exported surface is narrower than ROADMAP criterion.** Only `Parser` (interface) and `WithParser(Parser) BuilderOption` are exported. `parserSink` (interface) and `stdlibParser` (struct) stay unexported. Flag deviation from ROADMAP success criterion #3 at verification.
- **D-03: Sink owns when-to-buffer, parser owns how-to-buffer.** `ShouldBufferForTransform(path)` delegates to `b.config.representations(path)`. If true, parser decodes whole subtree with its own decoder and calls `StageMaterialized(state, path, value, true)`. Mirrors today's `decodeTransformedValue` control flow exactly.
- **D-04: Golden bytes at `testdata/parity-golden/`.** Generate from `v1.0` tag before PR lands, commit byte blobs, assert `stdlibParser` reproduces them byte-identically. v9 serialization format. Regeneration procedure documented in `testdata/parity-golden/README.md`.
- **D-05: Parity coverage spans four dimensions.** (1) Authored fixtures (int64 boundaries, null vs missing, deep nesting, unicode keys, empty arrays, large strings); (2) Gopter generators (reuse `GenJSONDocument`, `GenTestDocs`, `GenTestDocsWithNulls`, `GenMixedTypeDocs`); (3) Full 12-operator Evaluate matrix; (4) Transformer-driven paths (at least one fixture with `WithISODateTransformer` + `WithToLowerTransformer`). Harness lives in `parser_parity_test.go`.
- **D-06: Name() cached once at NewBuilder.** After options loop: `b.parserName = parser.Name()` with empty-name rejection. `WithParser(nil)` → `errors.New("parser cannot be nil")`. Default parser returns `"stdlib"`.
- **D-07: No error wrapping at AddDocument.** Parsers wrap errors with their own context; `AddDocument` returns `parser.Parse(...)` errors verbatim.

### Claude's Discretion

- **Name() format beyond "stdlib"** — future parsers choose their own identifiers.
- **`ensureDecoderEOF` placement** — inside `stdlibParser.Parse`, wrapping with today's `"failed to parse JSON"` error.
- **Error sub-classes** — typed sentinels only if patterns emerge during implementation.
- **Benchmark coverage** — planner may add dedicated `BenchmarkAddDocumentThroughParser` vs relying on existing `BenchmarkAddDocument` / `BenchmarkAddDocumentPhase07`.

### Deferred Ideas (OUT OF SCOPE)

- Exporting `ParserSink` — revisit when v1.2+ needs a third-party parser.
- `Parser.Name()` format standard (e.g., `vendor/lib/version`) — defer until SIMD lands.
- Parser-level benchmarks beyond existing builder benchmarks.
- `EvaluateContext` / `BuildFromParquetContext` — scheduled for Phase 14 (OBS-07).
- Typed parse-error sentinels — introduce only if patterns emerge.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PARSER-01 | Builder exposes pluggable `Parser` interface with narrow `ParserSink` write-side, defaulting to `stdlibParser` wrapping current `json.Decoder.UseNumber()` path with zero behavior change, validated via parity test harness on existing corpus | Covered by §Refactor Mechanics (exact file layout + method migration), §Parity Harness Architecture (four D-05 dimensions), §Goldens Generation Procedure (v1.0-tag regen, build-tag gated), §Verification Plan (byte-identical + 12-operator Evaluate matrix + benchmark regression guard) |

## Standard Stack

Phase 13 adds **zero new dependencies**. All code uses existing deps:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `encoding/json` | stdlib | `json.Decoder.UseNumber()` — kept verbatim | Current v1.0 behavior is the exact contract to preserve [VERIFIED: builder.go:322-334] |
| `github.com/pkg/errors` | v0.9.1 | `errors.New`, `errors.Wrap`, `errors.Errorf` | Project convention (CLAUDE.md); stdlibParser's internal errors follow this [VERIFIED: go.mod + project CLAUDE.md] |
| `github.com/leanovate/gopter` | v0.2.11 | Property-based parity tests | Existing generators in `generators_test.go` reused verbatim (D-05 dim #2) [VERIFIED: go.mod] |

### Supporting (test-only)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `testing` (stdlib) | Go 1.25.5 | Unit + benchmark tests | Standard |
| `gotest.tools/gotestsum` | v1.13.0 | Test runner (via `make test`) | CI/dev parity |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Exposing `ParserSink` | Keeping it unexported (chosen) | Locked by D-02 — non-breaking to export later if a v1.2+ third-party parser needs it; exporting now is irreversible |
| Parser returning `*documentBuildState` | Opaque state threaded through sink (chosen) | D-01 — keeps zero allocation on the AddDocument hot path; parser never allocates state |
| One sink method per type | 6 narrow methods (chosen) | D-01 — mirrors today's 5 staging methods + 1 buffering hook; minimal diff, clear contract |

**Installation:** No new deps. `go.mod` unchanged.

**Version verification:** No version bumps required. Existing deps already pinned at `go.mod`:
- `github.com/pkg/errors v0.9.1` [VERIFIED: go.mod, grep]
- `github.com/leanovate/gopter v0.2.11` [VERIFIED: go.mod]

## Architecture Patterns

### System Architecture Diagram

```
                                    ┌─────────────────────────────┐
  user code ──AddDocument(id,bytes)─▶│       *GINBuilder           │
                                    │                             │
                                    │  ┌──────────────────────┐   │
                                    │  │ b.parser (Parser)    │   │
                                    │  └──────────┬───────────┘   │
                                    │             │ Parse(bytes,  │
                                    │             │   rgID, sink) │
                                    │             ▼               │
                       ┌────────────────────────────────────────┐ │
                       │            stdlibParser                │ │
                       │  (unexported, zero-value struct)       │ │
                       │                                        │ │
                       │  json.Decoder.UseNumber()              │ │
                       │  stage-stream walk (today's code)     │ │
                       │                                        │ │
                       │  on each path visit:                   │ │
                       │   1. sink.ShouldBufferForTransform(p)? │ │
                       │      YES → decode subtree → Stage-     │ │
                       │            Materialized                │ │
                       │   2. read token                        │ │
                       │      scalar → sink.StageScalar /       │ │
                       │                sink.StageJSONNumber    │ │
                       │      delim  → recurse                  │ │
                       └────────────┬───────────────────────────┘ │
                                    │                             │
                                    │ sink method calls return to:│
                                    │                             │
                                    │  ┌──────────────────────┐   │
                                    │  │ parserSink (impl by  │   │
                                    │  │ *GINBuilder)         │   │
                                    │  │                      │   │
                                    │  │ methods forward to:  │   │
                                    │  │  • stageScalarToken  │   │
                                    │  │  • stageJSONNumber-  │   │
                                    │  │       Literal        │   │
                                    │  │  • stageNative-      │   │
                                    │  │       Numeric        │   │
                                    │  │  • stageMaterialized-│   │
                                    │  │       Value          │   │
                                    │  │    (classifier stays │   │
                                    │  │     HERE — Pitfall #1)│  │
                                    │  └──────────────────────┘   │
                                    │                             │
                                    │  mergeDocumentState (unchanged)
                                    └─────────────────────────────┘
```

Data flow: a single `AddDocument` call does one parse (via the parser) then one merge (unchanged). The parser reads tokens and emits sink calls; the sink routes each call to today's existing staging method, which mutates the provided `*documentBuildState` the parser received from `BeginDocument`. After parser returns, `mergeDocumentState` folds the staged state into `b.pathData`. Only difference from v1.0: one indirection through `b.parser.Parse(...)` replacing the direct `b.parseAndStageDocument(...)` call.

### Recommended Project Structure

New files only — no directories added:

```
./
├── parser.go                  # NEW: Parser interface + parserSink interface
├── parser_stdlib.go           # NEW: stdlibParser struct + Parse method
├── parser_sink.go             # NEW: *GINBuilder parserSink method impls
├── parser_parity_test.go      # NEW: four-dimension parity harness
├── parity_goldens_test.go     # NEW (build-tagged): regenerate_goldens tool
├── builder.go                 # MODIFIED: AddDocument routes through b.parser
└── testdata/
    └── parity-golden/         # NEW: committed byte blobs (v9 encoded)
        ├── README.md
        ├── authored-fixture-01.bin  # one blob per authored fixture
        └── ...
```

Alternatively, the planner may keep everything in `parser.go` (all three types + sink impl) as a single-file introduction — this has less file churn and keeps the entire seam visible on one screen. Recommendation: **one file (`parser.go`)** for the interface + `stdlibParser`, **one file (`parser_sink.go`)** for the `*GINBuilder` sink method impls (short — each method is a 1-line forward), keeping diff review focused. This is the planner's call; both are acceptable.

### Pattern 1: Parser as Functional-Option Seam with Package-Private Sink

**What:** `Parser` is an exported interface. `parserSink` is an unexported interface implemented by `*GINBuilder`. `stdlibParser` is an unexported struct with no fields — zero value usable. `WithParser(p Parser) BuilderOption` sets `b.parser`; if nil at the end of the options loop, `NewBuilder` defaults to `stdlibParser{}`. After defaulting, `NewBuilder` calls `parser.Name()` and stores the result on `b.parserName`, erroring on empty.

**When to use:** Always — every `NewBuilder` construction gets a parser. Consumers who don't opt in get v1.0 behavior unchanged.

**Tradeoffs:**
- **Pro:** Minimal diff. The five staging methods keep their current signatures (receiver stays `*GINBuilder`); only a thin interface adapter is added.
- **Pro:** Zero-alloc hot path preserved — `parserSink` interface call is on `*GINBuilder` (pointer receiver), Go inlines pointer method calls through interface tables cheaply.
- **Pro:** BUILD-03 int64 contract stays in one place (`stageJSONNumberLiteral` at `builder.go:598`). SIMD v1.2 routes raw source text through `StageJSONNumber` and the classifier stays the single source of truth.
- **Con:** The `parserSink` interface "leaks" `*documentBuildState` type name, but only within `package gin`. No public surface leak.

**Example (interface + stdlib impl):**

```go
// parser.go (new, ~60 LOC)
// Source: .planning/research/ARCHITECTURE.md §Pattern 1 (lines 87-120), adapted to D-01/D-02
package gin

import "github.com/pkg/errors"

// Parser translates one JSON document into staged per-path observations,
// writing them through the supplied sink. Implementations MUST preserve
// exact-int semantics: integers outside the float64-exact range
// [-2^53, 2^53] must be reported via StageJSONNumber (raw source text) so
// the builder's classifier stays the single source of truth for numeric type.
type Parser interface {
    // Name returns a stable identifier for telemetry (e.g. "stdlib").
    // MUST NOT return the empty string.
    Name() string

    // Parse walks jsonDoc and stages observations for rgID via sink.
    // Errors returned by the sink MUST propagate verbatim (do not double-wrap).
    Parse(jsonDoc []byte, rgID int, sink parserSink) error
}

// parserSink is the narrow write contract a Parser uses to publish
// observations. It is intentionally package-private so alternative parsers
// cannot reach into the builder's internals, and exposes *documentBuildState
// as an opaque handle (parsers MUST NOT read its fields).
type parserSink interface {
    BeginDocument(rgID int) *documentBuildState
    StageScalar(state *documentBuildState, path string, token any) error
    StageJSONNumber(state *documentBuildState, path, raw string) error
    StageNativeNumeric(state *documentBuildState, path string, v any) error
    StageMaterialized(state *documentBuildState, path string, value any, allowTransform bool) error
    ShouldBufferForTransform(path string) bool
}

// WithParser sets a custom parser. nil parser is rejected. Last-wins if
// supplied multiple times (BuilderOption convention).
func WithParser(p Parser) BuilderOption {
    return func(b *GINBuilder) error {
        if p == nil {
            return errors.New("parser cannot be nil")
        }
        b.parser = p
        return nil
    }
}
```

```go
// parser_stdlib.go (new, ~140 LOC — body moves verbatim from builder.go)
// Source: builder.go:322-447 moved under pointer-free receiver
package gin

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "sort"

    "github.com/pkg/errors"
)

// stdlibParser is the default parser: json.Decoder with UseNumber() — byte-
// identical to v1.0 behavior.
type stdlibParser struct{}

func (stdlibParser) Name() string { return "stdlib" }

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

func (s stdlibParser) streamValue(decoder *json.Decoder, path string, state *documentBuildState, sink parserSink) error {
    canonicalPath := normalizeWalkPath(path)

    // Buffering hook: if a transformer is registered for this path, decode
    // the subtree and stage it as a materialized value (D-03).
    if sink.ShouldBufferForTransform(canonicalPath) {
        var value any
        if err := decoder.Decode(&value); err != nil {
            return errors.Wrapf(err, "parse transformed subtree at %s", canonicalPath)
        }
        return sink.StageMaterialized(state, path, value, true)
    }

    token, err := decoder.Token()
    if err != nil {
        return errors.Wrap(err, "read JSON token")
    }

    switch tok := token.(type) {
    case json.Delim:
        // object/array walk (mirrors builder.go:358-415 verbatim, but calls
        // sink.StageMaterialized for children and recursion-through-self for
        // nested decodes — see below for key refactor note)
        // ... (details below)
    default:
        return sink.StageScalar(state, canonicalPath, token)
    }
}
```

**Key refactor note for object/array handling:** Today `stageStreamValue` recursively calls `b.stageMaterializedValue` for children (after materializing the whole object/array via `decodeAny`). The stdlibParser must keep this exact semantics — so children get staged via `sink.StageMaterialized`, not by the parser re-walking. This preserves the today-behavior where, after hitting `{` or `[`, the parser materializes the subtree via `decoder.Decode(&anyValue)` and then emits one `StageMaterialized` call per top-level child key/element. Verify this by reading `builder.go:362-412` before writing the parser.

**Wiring in `builder.go`:**

```go
// builder.go: GINBuilder struct grows two fields
type GINBuilder struct {
    // ... existing fields ...
    parser     Parser
    parserName string
}

// builder.go:115 NewBuilder: after options loop, before return
if b.parser == nil {
    b.parser = stdlibParser{}
}
name := b.parser.Name()
if name == "" {
    return nil, errors.New("parser name cannot be empty")
}
b.parserName = name

// builder.go:287 AddDocument: replace direct call with parser dispatch
state, err := func() (*documentBuildState, error) {
    // sink is b itself; parser returns via Parse, but we need the state
    // back for mergeDocumentState. Adjust the sink contract so BeginDocument
    // is called before Parse, and AddDocument holds the state:
    s := b.BeginDocument(pos)
    if err := b.parser.Parse(jsonDoc, pos, b); err != nil {
        return nil, err  // D-07: no wrapping
    }
    return s, nil
}()
// ...
return b.mergeDocumentState(docID, pos, exists, state)
```

**Caveat on state threading (important planner decision):** Today `parseAndStageDocument` returns the `*documentBuildState` that `AddDocument` then passes to `mergeDocumentState`. The locked D-01 contract has `sink.BeginDocument(rgID) *documentBuildState` — so the sink creates state. Two options:

1. **AddDocument calls `BeginDocument` first, then `parser.Parse(jsonDoc, pos, b)`, then `mergeDocumentState(..., state)`** — parser discovers the state from the sink within `Parse` (first sink call must be `BeginDocument(rgID)`).
2. **`Parse` returns the `*documentBuildState`** — but this contradicts D-01's method set.

Recommended: option 1. The parser's first sink call inside `Parse` is `state := sink.BeginDocument(rgID)`. `AddDocument` does NOT call `BeginDocument`; it relies on the parser to do so and retrieves the state via a different path. Simplest: add a field to the builder (`b.currentDocState *documentBuildState`) that `BeginDocument` sets and `AddDocument` reads post-`Parse()`. Single-threaded builder (architecture invariant) makes this safe — no sync needed.

**Alternative (cleaner):** `BeginDocument` returns state AND stores it on the builder; `AddDocument` reads `b.currentDocState` after `Parse`. Plan this explicitly in the first task — it is the subtle point.

### Pattern 2: Golden-Byte Parity Harness

**What:** `parser_parity_test.go` is the merge-gate file. It has four sub-tests, one per D-05 dimension. Each asserts that:
- An index built through `stdlibParser` via the new seam produces byte-identical `Encode()` output vs. the committed goldens.
- `Evaluate` over each of the 12 operators returns the same `RGSet` as a freshly-built legacy-equivalent index (when goldens exist) or against a pre-computed baseline `RGSet` stored next to the bytes.

**Tradeoffs:**
- **Pro:** Goldens pin behavior forever — even a future change that *thinks* it is behavior-neutral gets caught.
- **Pro:** Parity for transformer-driven paths is covered (D-05 dim #4).
- **Con:** Regenerating goldens requires checking out `v1.0` and running a build-tagged test — extra step, but documented in `testdata/parity-golden/README.md`.
- **Con:** Golden bytes are format-version-sensitive (v9 today). Any future v10 bump requires a goldens refresh accompanied by a parity-preserving proof; that is healthy, not a bug.

**Example (harness skeleton):**

```go
// parser_parity_test.go (new)
package gin

import (
    "bytes"
    "encoding/json"
    "os"
    "path/filepath"
    "testing"

    "github.com/leanovate/gopter"
    "github.com/leanovate/gopter/prop"
)

// -----------------------------------------------------------------------------
// Goldens loader (D-04)
// -----------------------------------------------------------------------------

type parityGolden struct {
    Name     string // fixture identifier, maps to testdata/parity-golden/<name>.bin
    JSONDocs [][]byte
    Config   func() GINConfig // allows per-fixture transformer wiring
    NumRGs   int
}

func loadGolden(t *testing.T, name string) []byte {
    t.Helper()
    path := filepath.Join("testdata", "parity-golden", name+".bin")
    b, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("load golden %s: %v", name, err)
    }
    return b
}

// -----------------------------------------------------------------------------
// D-05 dim #1: authored fixtures
// -----------------------------------------------------------------------------

func TestParserParity_AuthoredFixtures(t *testing.T) {
    for _, fx := range authoredParityFixtures() {
        t.Run(fx.Name, func(t *testing.T) {
            builder, err := NewBuilder(fx.Config(), fx.NumRGs) // default parser
            if err != nil { t.Fatal(err) }
            for i, doc := range fx.JSONDocs {
                if err := builder.AddDocument(DocID(i), doc); err != nil {
                    t.Fatalf("doc %d: %v", i, err)
                }
            }
            idx := builder.Finalize()
            encoded, err := Encode(idx)
            if err != nil { t.Fatal(err) }

            golden := loadGolden(t, fx.Name)
            if !bytes.Equal(encoded, golden) {
                t.Fatalf("byte-level parity broken for %s: encoded=%d bytes, golden=%d bytes",
                    fx.Name, len(encoded), len(golden))
            }
        })
    }
}

// authoredParityFixtures returns hand-crafted corpus covering D-05 dim #1:
// int64 boundaries, null vs missing, deep nesting, unicode keys, empty arrays,
// large strings.
func authoredParityFixtures() []parityGolden {
    return []parityGolden{
        {
            Name:   "int64-boundaries",
            Config: DefaultConfig,
            NumRGs: 4,
            JSONDocs: [][]byte{
                []byte(`{"a": 9223372036854775807}`),   // MaxInt64
                []byte(`{"a": -9223372036854775807}`),  // -MaxInt64+1 (literal safe)
                []byte(`{"a": 9007199254740993}`),      // 2^53 + 1
                []byte(`{"a": 0}`),
            },
        },
        {Name: "nulls-and-missing", /* ... */},
        {Name: "deep-nested", /* ... */},
        {Name: "unicode-keys", /* ... */},
        {Name: "empty-arrays", /* ... */},
        {Name: "large-strings", /* ... */},
        // D-05 dim #4: transformer-driven
        {
            Name: "transformers-iso-date-and-lower",
            Config: func() GINConfig {
                cfg := DefaultConfig()
                _ = WithISODateTransformer("$.created_at")(&cfg)
                _ = WithToLowerTransformer("$.email")(&cfg)
                return cfg
            },
            NumRGs: 4,
            JSONDocs: [][]byte{
                []byte(`{"created_at": "2024-01-15T10:30:00Z", "email": "Alice@EXAMPLE.COM"}`),
                []byte(`{"created_at": "2024-02-20T08:00:00Z", "email": "bob@example.com"}`),
            },
        },
    }
}

// -----------------------------------------------------------------------------
// D-05 dim #2: gopter property — byte-identical output across random shapes
// -----------------------------------------------------------------------------

func TestParserParity_GopterByteIdentical(t *testing.T) {
    properties := gopter.NewProperties(propertyTestParameters())

    // Two builds with identical inputs → identical encoded bytes.
    // This is NOT a legacy-vs-seam comparison (there is no "legacy" path post-
    // refactor — the stdlibParser IS the legacy path). It asserts determinism
    // + zero-alloc-induced-drift.
    properties.Property("two-build determinism", prop.ForAll(
        func(docs []TestDoc) bool {
            if len(docs) == 0 { return true }
            return buildAndEncode(docs) == buildAndEncode(docs)
        },
        GenTestDocs(25),
    ))
    // ... GenMixedTypeDocs, GenTestDocsWithNulls similarly ...
    properties.TestingRun(t)
}

// -----------------------------------------------------------------------------
// D-05 dim #3: 12-operator Evaluate matrix
// -----------------------------------------------------------------------------

func TestParserParity_EvaluateMatrix(t *testing.T) {
    // For each of the 12 operators, exercise one matching and one pruning
    // predicate against the same fixture and compare to goldens.
    cases := []struct {
        name  string
        pred  Predicate
        wantRGs []int // documented alongside goldens
    }{
        {"EQ-match", EQ("$.name", "alice"), []int{0, 2}},
        {"EQ-prune", EQ("$.name", "nobody"), []int{}},
        {"NE-match", NE("$.name", "alice"), []int{1, 3}},
        // GT, GTE, LT, LTE, IN, NIN, IsNull, IsNotNull, Contains, Regex ...
    }
    idx := buildEvaluateMatrixFixture(t)
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            got := idx.Evaluate([]Predicate{c.pred}).ToSlice()
            if !intSliceEqual(got, c.wantRGs) {
                t.Errorf("op=%s got=%v want=%v", c.name, got, c.wantRGs)
            }
        })
    }
}
```

### Pattern 3: Build-Tag-Gated Goldens Regeneration

**What:** `parity_goldens_test.go` with `//go:build regenerate_goldens` at the top. Running `go test -tags regenerate_goldens -run TestRegenerateParityGoldens` against the `v1.0` tag writes fresh blobs to `testdata/parity-golden/`. Normal `go test ./...` runs don't compile this file, so goldens are never accidentally rewritten.

**Example:**

```go
//go:build regenerate_goldens

// parity_goldens_test.go
package gin

import (
    "os"
    "path/filepath"
    "testing"
)

// TestRegenerateParityGoldens writes fresh golden blobs. Run only when
// deliberately refreshing parity pins. See testdata/parity-golden/README.md.
func TestRegenerateParityGoldens(t *testing.T) {
    dir := filepath.Join("testdata", "parity-golden")
    if err := os.MkdirAll(dir, 0o755); err != nil { t.Fatal(err) }
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

**`testdata/parity-golden/README.md` content:**

```markdown
# Parser Parity Goldens

Byte-level goldens pinning `Encode()` output for the `stdlibParser` path.
Generated from the v1.0 tag; regenerated only when serialization format
changes (v10+).

## Regenerate

```bash
git checkout v1.0
go test -tags regenerate_goldens -run TestRegenerateParityGoldens .
git checkout -   # back to the feature branch
git add testdata/parity-golden/*.bin
git commit -m "chore(parity): refresh goldens to vN"
```

## Format

v9 encoded index bytes (see `gin.go:29 Version = uint16(9)`). One file per
authored fixture; names match `authoredParityFixtures()`.
```

### Anti-Patterns to Avoid

- **Moving the int64 classifier into the parser:** Violates Pitfall #1. Classification MUST stay on `*GINBuilder` — SIMD v1.2 routes raw source text through `StageJSONNumber` and relies on the classifier as the single source of truth.
- **Signature churn on the staging methods:** Don't rename `stageScalarToken`, `stageMaterializedValue`, etc. Add thin sink method forwarders (`func (b *GINBuilder) StageScalar(...)`) that internally call today's methods. Keeps diff minimal and reviewable.
- **Wrapping parser errors at `AddDocument`:** D-07 forbids this. The parser wraps its own errors; `AddDocument` returns verbatim.
- **Pointer receiver on `stdlibParser`:** `stdlibParser{}` has no fields — use value receiver. Pointer receiver forces heap allocation when boxed into the `Parser` interface. Verify with `go vet` / benchmark regression.
- **Exposing `*documentBuildState`:** Stays package-private; parser holds it as opaque handle only. Any field read from parser code is a review blocker.
- **Re-walking objects/arrays inside the parser:** Today's `stageStreamValue` materializes subtrees via `decodeAny` then hands off to `stageMaterializedValue` — the parser MUST preserve this split to stay byte-identical.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON decoder | Custom tokenizer | `encoding/json.Decoder.UseNumber()` | Same as v1.0; any deviation breaks BUILD-03 parity |
| Property-test shrinking | Custom generator | Existing `GenJSONDocument`, `GenTestDocs`, `GenTestDocsWithNulls`, `GenMixedTypeDocs` from `generators_test.go` | Reuse mandated by D-05 dim #2; existing generators already tuned |
| Error wrapping | `fmt.Errorf("%w", ...)` | `errors.Wrap(err, "...")` | Project convention (CLAUDE.md); captures stack trace at wrap point |
| Golden regeneration runner | Custom script | Build-tag-gated test (`//go:build regenerate_goldens`) | Keeps goldens reproducible via the same Go toolchain; no external script to maintain |
| Parity comparison | Hand-rolled diff | `bytes.Equal(encoded, golden)` + `RGSet.ToSlice()` + `intSliceEqual` | Simplest reliable assertion; byte-level catches ALL drift |

**Key insight:** Every reusable asset for this phase already exists in the codebase. The refactor is ~90% code movement, ~10% new code (the interface + sink adapter). Resist the urge to "improve" anything while refactoring — Pitfall #11 (behavior-neutral PR discipline).

## Runtime State Inventory

Phase 13 is a **pure refactor** — code-only changes, no runtime state migration. For completeness, each category is checked:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — no database, no user_ids, no cached collections affected. Goldens committed as new files under `testdata/parity-golden/`. | None — new file addition only |
| Live service config | None — library only, no running services | None |
| OS-registered state | None — no tasks, daemons, or scheduler entries | None |
| Secrets/env vars | None — no credentials or env vars introduced | None |
| Build artifacts | None — no installed binaries or caches to invalidate. `go build ./...` stays green across the refactor. | None |

**Nothing found in any category** — verified by grep on identifiers (`parser`, `ParserSink`) and inspection of `Makefile`, `go.mod`, and `.github/workflows/`.

## Common Pitfalls

### Pitfall 1: Silent int64 fidelity drift during refactor (blast radius: CRITICAL)

**What goes wrong:** The classifier at `builder.go:598-627` (`stageJSONNumberLiteral` + `parseJSONNumberLiteral`) accepts raw JSON number source text and carefully routes via `strconv.ParseInt` (integer path) before `strconv.ParseFloat` (float path) to preserve exact-int64 for values up to `MaxInt64`. If refactor moves this logic into the parser, or if `StageJSONNumber` is called with a pre-parsed float64 instead of raw text, BUILD-03 breaks silently for values in `(2^53, 2^63)`.

**Why it happens:** Copy-paste regressions; over-eager "clean up" that converts `(path, raw string)` to `(path, float64)` because "it's simpler." Also happens if the parser decodes via `decoder.Decode(&value)` for scalars (returning `json.Number` which is a string, fine) but then a future "optimization" converts to native float early.

**How to avoid:**
- Keep `stageJSONNumberLiteral` on `*GINBuilder`. The `StageJSONNumber(state, path, raw string)` sink method calls it directly with the raw source text.
- `stdlibParser` reads scalars via `decoder.Token()` which returns `json.Number` (a string wrapper); extract `.String()` and pass as `raw` to `StageJSONNumber`. Do not call `json.Number.Float64()` in the parser.
- Parity harness authored fixture **MUST** include `int64-boundaries` with `MaxInt64`, `2^53 + 1`, and `-MaxInt64`.
- Run `gin_test.go:2988-3233` (existing Phase 07 corpus — `TestNumericIndexPreservesInt64Exactness` and friends) through the new path. These tests must pass unchanged.

**Warning signs:**
- A diff introduces `strconv.ParseFloat` inside `parser_stdlib.go`.
- `StageJSONNumber` signature changes from `(state, path, raw string)` to `(state, path, v float64)`.
- An existing `gin_test.go` Phase 07 test starts failing after the refactor.

### Pitfall 2: Transformer buffering hook misrouted (blast radius: HIGH)

**What goes wrong:** Today's `decodeTransformedValue` (`builder.go:438-447`) checks `b.config.representations(canonicalPath)` BEFORE reading the next token — if non-empty, the whole subtree is decoded via `decodeAny` and routed through `stageMaterializedValue` (applying the transformer). After refactor, if `ShouldBufferForTransform` is called AFTER reading the token, or if the parser recurses into the subtree anyway, transformer-bearing fixtures produce different indexes.

**Why it happens:** Subtle control-flow mismatch. Today the check is at the top of `stageStreamValue`; if the refactored parser moves the check to after `decoder.Token()`, one extra token has been consumed and `decodeAny` sees the wrong position in the stream.

**How to avoid:**
- Mirror today's control flow exactly: `ShouldBufferForTransform(path)` check BEFORE `decoder.Token()`. If true, call `decoder.Decode(&value)` to consume the whole subtree starting at that position.
- D-05 dim #4 parity fixture (`transformers-iso-date-and-lower`) exercises this path — if it passes, the wiring is right.

**Warning signs:**
- `decoder.Token()` appears before the transformer check in `parser_stdlib.go`.
- `transformers-iso-date-and-lower` golden does not match.

### Pitfall 3: Accidental signature drift in sink methods (blast radius: MEDIUM)

**What goes wrong:** Sink method signatures in `parserSink` interface drift from the staging method signatures on `*GINBuilder`. E.g., sink has `(state, path, token)` but staging has `(canonicalPath, token, state)` — order mismatch, silent compile error, panic, or worse, passes the wrong `path` variant (normalized vs. non-normalized).

**Why it happens:** Today's `stageScalarToken` takes `canonicalPath` (already normalized) while `stageMaterializedValue` takes the raw `path` (and normalizes inside). After refactor, the sink method might accept the wrong one.

**How to avoid:**
- Before writing the sink interface, re-read `builder.go:473-654` and document exactly which existing methods take `canonicalPath` vs `path`.
- Draft the sink interface so each method matches today's semantics. Specifically: `StageScalar(state, canonicalPath, token)` — parser passes already-normalized path. `StageMaterialized(state, path, value, allow)` — parser passes raw path, sink's impl normalizes via `normalizeWalkPath` (matching today's `stageMaterializedValue`).
- Add a unit test that calls each sink method with a known path/value and asserts the same pathData[] entry results as the pre-refactor code would produce (lifted from existing `gin_test.go` patterns).

**Warning signs:**
- Sink interface has both `path` and `canonicalPath` parameters — likely semantic confusion.
- `normalizeWalkPath` is called inside the parser (should be inside sink impls, matching today).

### Pitfall 4: PR bundles non-refactor changes (blast radius: MEDIUM — Pitfall #11)

**What goes wrong:** PR mixes the seam extraction with tangential improvements ("while I'm here, let me also clean up X"). When the parity harness fails, it's unclear which change caused the drift.

**Why it happens:** Refactor phases invite scope creep. The test move or a logger rename feels too small to separate.

**How to avoid:**
- This PR is **behavior-neutral**. Any diff hunk that isn't direct consequence of the seam is a review blocker.
- Recommended commit breakdown (see Execution Plan below): three commits, each green on its own.
- Include "behavior-neutral" in the PR description; reviewer can grep for non-neutral changes.

**Warning signs:**
- Commit includes changes outside `builder.go`, new `parser_*.go` files, and `testdata/parity-golden/`.
- Commit renames a field unrelated to the seam.
- CI benchmark regression >2% on `BenchmarkAddDocument` / `BenchmarkAddDocumentPhase07`.

### Pitfall 5: Benchmark regression without visibility (blast radius: LOW-MEDIUM)

**What goes wrong:** The extra interface-call indirection (`b.parser.Parse(...)`) + the sink-call-per-staged-observation adds nanoseconds. On a hot benchmark, a >5% regression is a signal something deeper is wrong (e.g., heap allocation of `stdlibParser`).

**Why it happens:** Go interface dispatch is fast but not free. If `stdlibParser{}` is stored as a Parser interface and accidentally allocated on the heap each AddDocument call (e.g., constructed via `Parser(stdlibParser{})` rather than set once at NewBuilder), allocs increase.

**How to avoid:**
- Set `b.parser = stdlibParser{}` ONCE at `NewBuilder`. Never reconstruct per call.
- Run existing `BenchmarkAddDocument`, `BenchmarkAddDocumentPhase07`, `BenchmarkBuildPhase07` both before and after. Record deltas.
- Merge gate: ≤2% ns/op regression, 0 additional allocs/op vs. v1.0 baseline (same commit measured before the refactor begins).
- If exceeded, investigate inlining — `go build -gcflags="-m=2" ./..` shows whether `Parse` was inlined.

**Warning signs:**
- `BenchmarkAddDocument` shows +5% or more ns/op.
- Benchmark `allocs/op` increases by ≥1.
- Profile (`go test -bench=. -cpuprofile=cpu.out`) shows runtime.convT or runtime.mallocgc under `AddDocument` that wasn't there before.

## Code Examples

### Common Operation 1: Sink method adapter on `*GINBuilder`

```go
// parser_sink.go (new) — thin adapters forward to existing private methods.
// Source: sink contract per D-01, forwards unchanged to builder.go:473-654
package gin

func (b *GINBuilder) BeginDocument(rgID int) *documentBuildState {
    s := newDocumentBuildState(rgID)
    b.currentDocState = s // single-threaded builder — safe to stash
    return s
}

func (b *GINBuilder) StageScalar(state *documentBuildState, canonicalPath string, token any) error {
    return b.stageScalarToken(canonicalPath, token, state)
}

func (b *GINBuilder) StageJSONNumber(state *documentBuildState, canonicalPath, raw string) error {
    return b.stageJSONNumberLiteral(canonicalPath, raw, state)
}

func (b *GINBuilder) StageNativeNumeric(state *documentBuildState, canonicalPath string, v any) error {
    return b.stageNativeNumeric(canonicalPath, v, state)
}

func (b *GINBuilder) StageMaterialized(state *documentBuildState, path string, value any, allowTransform bool) error {
    return b.stageMaterializedValue(path, value, state, allowTransform)
}

func (b *GINBuilder) ShouldBufferForTransform(canonicalPath string) bool {
    return len(b.config.representations(canonicalPath)) > 0
}
```

### Common Operation 2: AddDocument wiring

```go
// builder.go:287 (modified — mirrors today plus one indirection)
func (b *GINBuilder) AddDocument(docID DocID, jsonDoc []byte) error {
    if b.poisonErr != nil {
        return errors.Wrap(b.poisonErr, "builder poisoned by prior merge failure; discard and rebuild")
    }
    pos, exists := b.docIDToPos[docID]
    if !exists {
        pos = b.nextPos
        if pos >= b.numRGs {
            return errors.Errorf("position %d exceeds numRGs %d", pos, b.numRGs)
        }
    }

    // Parser dispatch — D-07: no wrapping here.
    if err := b.parser.Parse(jsonDoc, pos, b); err != nil {
        return err
    }

    return b.mergeDocumentState(docID, pos, exists, b.currentDocState)
}
```

### Common Operation 3: Assert byte-parity in test

```go
// parser_parity_test.go (excerpt)
// Source: project test convention (github.com/pkg/errors + t.Fatalf)
func assertByteIdentical(t *testing.T, fixtureName string, encoded, golden []byte) {
    t.Helper()
    if len(encoded) != len(golden) {
        t.Fatalf("parity %s: byte length differs (encoded=%d golden=%d)",
            fixtureName, len(encoded), len(golden))
    }
    if !bytes.Equal(encoded, golden) {
        // Find first diff offset for faster debugging
        for i := range encoded {
            if encoded[i] != golden[i] {
                t.Fatalf("parity %s: first diff at byte offset %d (encoded=0x%02x golden=0x%02x)",
                    fixtureName, i, encoded[i], golden[i])
            }
        }
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Parser logic inline on `*GINBuilder` methods | Parser as interface, sink as package-private narrow write contract | Phase 13 (this phase) | Enables SIMD v1.2 without further builder churn |
| `json.Decoder.UseNumber()` hard-coded in `parseAndStageDocument` | Wrapped in `stdlibParser` (unexported, default) | Phase 13 | No runtime behavior change; opt-in custom parsers possible |

**Not deprecated:** All existing code paths remain valid. Existing exported API unchanged. Zero migration cost for consumers.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `stdlibParser{}` as a value-receiver zero-struct avoids heap allocation when stored in a `Parser` interface field | Pattern 1 | [ASSUMED] Go allocates even zero-size structs in some cases; if benchmark regression shows +1 allocs/op, switch to `var defaultStdlibParser = stdlibParser{}` global. Low risk. |
| A2 | The parser's first sink call can be `BeginDocument` and `AddDocument` reads `b.currentDocState` post-`Parse` | Pattern 1 "caveat on state threading" | [ASSUMED] This is a planner decision; alternative is to reshape D-01 to return state from `Parse`. CONTEXT.md locks D-01, so the stashed-state approach is the path. Verified in builder (single-threaded). |
| A3 | Running `TestRegenerateParityGoldens` against the `v1.0` tag produces byte-identical output to running it against `HEAD` after the refactor | Pattern 3 | [VERIFIED: git tag confirms `v1.0` exists; serialization format v9 is stable per Phase 10 evidence] If format drift exists, the parity harness catches it — this IS the merge gate. |
| A4 | Existing `BenchmarkAddDocument` / `BenchmarkAddDocumentPhase07` are sufficient regression guards; dedicated parser benchmark is optional | Validation Architecture | [ASSUMED] Existing benchmarks cover the hot path. Claude's discretion per CONTEXT.md — planner may add one. |

**Zero ASSUMED claims left undeferred:** A1 and A4 are low-risk perf/benchmark assumptions; A2 is a planner decision with CONTEXT.md cover; A3 is verified by `git tag` + Phase 10 format-stability evidence.

## Open Questions

1. **Sink method signature: `path` or `canonicalPath`?**
   - What we know: Today, `stageScalarToken` takes `canonicalPath` (already normalized); `stageMaterializedValue` takes raw `path` and normalizes inside. Mirroring exactly preserves behavior.
   - What's unclear: Whether the sink interface should normalize internally OR require parsers to pre-normalize. Locked D-01 sink set implies per-today semantics.
   - Recommendation: Match today's semantics exactly per-method. `StageScalar` accepts already-normalized `canonicalPath`; `StageMaterialized` accepts raw `path`. Document this in doc-comments on the interface.

2. **How to acquire `*documentBuildState` in `AddDocument` after `Parse` returns?**
   - What we know: D-01 locks `BeginDocument` on the sink; parser calls it first; `Parse` doesn't return state.
   - What's unclear: Stash on builder field (`b.currentDocState`)? Or another mechanism?
   - Recommendation: Stash on builder field. Single-threaded builder (architectural invariant) makes this trivially safe. Document in comment on the field: `// set by parserSink.BeginDocument; read by AddDocument; safe because builder is single-threaded`.

3. **Goldens count — how many authored fixtures?**
   - What we know: D-05 lists six categories (int64 boundaries, nulls vs missing, deep nesting, unicode keys, empty arrays, large strings) plus transformer paths.
   - What's unclear: One golden per category, or multiple?
   - Recommendation: One authored fixture per category (7 fixtures × ~1-5KB each = ~20KB committed). Minimal, covers the named risks, doesn't bloat the repo.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build + test | ✓ | 1.25.5 (from `go.mod`) | — |
| `golangci-lint` | `make lint` | ✓ (project standard) | via CI and local | — |
| `gotestsum` | `make test` | ✓ (auto-installed via `make gotestsum-bin`) | v1.13.0 | — |
| `git` | Checkout v1.0 for goldens regen | ✓ | — | — |
| v1.0 tag | Goldens regeneration | ✓ | `v1.0` present locally [VERIFIED: `git tag`] | — |

**Missing dependencies with no fallback:** None. All required tools are present locally or auto-provisioned via `make`.

**Missing dependencies with fallback:** None.

## Validation Architecture

> **Nyquist validation plan enabled** — `.planning/config.json` does not disable it.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `github.com/leanovate/gopter` v0.2.11 |
| Config file | None beyond `go.mod` (standard Go layout) |
| Quick run command | `go test -run TestParserParity -count=1 .` |
| Full suite command | `make test` (or `gotestsum --format short-verbose -- -coverprofile=coverage.out -timeout=30m ./...`) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PARSER-01 | `WithParser(nil)` returns error | unit | `go test -run TestWithParserRejectsNil -x .` | Wave 0 |
| PARSER-01 | `NewBuilder` defaults to `stdlibParser{}` with `Name() == "stdlib"` when `WithParser` omitted | unit | `go test -run TestNewBuilderDefaultsToStdlibParser -x .` | Wave 0 |
| PARSER-01 | Empty `Name()` returns error at `NewBuilder` | unit | `go test -run TestNewBuilderRejectsEmptyParserName -x .` | Wave 0 |
| PARSER-01 | `b.parserName` reachable and equals `"stdlib"` by default (Phase 14 hook) | unit | `go test -run TestBuilderParserNameReachable -x .` | Wave 0 |
| PARSER-01 | Authored fixtures produce byte-identical `Encode()` output vs goldens (D-05 dim #1) | integration | `go test -run TestParserParity_AuthoredFixtures -x .` | Wave 0 |
| PARSER-01 | Gopter property: two builds with identical inputs produce identical bytes (D-05 dim #2, determinism) | property | `go test -run TestParserParity_GopterByteIdentical .` | Wave 0 |
| PARSER-01 | 12-operator Evaluate matrix returns expected `RGSet` for each operator (D-05 dim #3) | integration | `go test -run TestParserParity_EvaluateMatrix -x .` | Wave 0 |
| PARSER-01 | Transformer-bearing fixture (`ISODateTransformer` + `ToLowerTransformer`) produces byte-identical output (D-05 dim #4) | integration | `go test -run TestParserParity_AuthoredFixtures/transformers_iso_date_and_lower -x .` | Wave 0 |
| PARSER-01 | BUILD-03 (int64 exact fidelity) preserved through the seam | regression | `go test -run TestNumericIndexPreservesInt64Exactness -x .` | ✅ (exists in `gin_test.go:2988-3233`) |
| PARSER-01 | Existing builder/query/serialize test suite green | regression | `make test` | ✅ (existing) |
| PARSER-01 | `BenchmarkAddDocument` within 2% ns/op, 0 extra allocs/op | benchmark | `go test -bench=BenchmarkAddDocument -benchmem -count=10 .` | ✅ (exists in `benchmark_test.go:891`) |
| PARSER-01 | `BenchmarkAddDocumentPhase07` within 2% ns/op, 0 extra allocs/op (int64 hot path) | benchmark | `go test -bench=BenchmarkAddDocumentPhase07 -benchmem -count=10 .` | ✅ (exists in `benchmark_test.go:972`) |

### Sampling Rate

- **Per task commit:** `go test -run TestParserParity -count=1 .` (≈<5s)
- **Per wave merge:** `make test` — full suite including 12-operator Evaluate matrix and transformer parity
- **Phase gate (before `/gsd-verify-work`):** Full suite green + benchmark delta recorded (`go test -bench=BenchmarkAddDocument -benchmem -count=10 .` vs v1.0 baseline)

### Wave 0 Gaps

- [ ] `parser_parity_test.go` — new file covering D-05 dims #1–#4 and PARSER-01 unit tests
- [ ] `parity_goldens_test.go` — new file, `//go:build regenerate_goldens` tag, writes goldens from v1.0
- [ ] `testdata/parity-golden/` — new directory with 7 authored `.bin` fixtures + README.md
- [ ] `parser.go`, `parser_stdlib.go`, `parser_sink.go` — new source files

No existing shared fixtures need modification. `generators_test.go` reused verbatim (D-05 locks this).

## Security Domain

> **`security_enforcement` not set in `.planning/config.json`** — absent = enabled. Included for completeness; Phase 13 is low-risk.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — (library, no auth surface) |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes | Existing `encoding/json.Decoder.UseNumber()` validation path preserved verbatim; no new inputs introduced |
| V6 Cryptography | no | — (no crypto in this phase) |
| V7 Error Handling | yes | `github.com/pkg/errors` with wrap-at-source (D-07); no sensitive content in error messages (error text references path names only, not values) |
| V8 Data Protection | no | — |

### Known Threat Patterns for Go library + JSON input

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Deeply-nested JSON DoS (parse bomb) | Denial of Service | Unchanged from v1.0 — `encoding/json` stdlib handles recursion depth; the refactor does not add or remove depth limits |
| Integer overflow via large numeric literal | Tampering | Preserved via Pitfall #1 — classifier on `*GINBuilder` rejects out-of-range integers with typed errors (`"unsupported integer literal"`) |
| Untrusted JSON error messages leaking content into consumer logs | Information Disclosure | D-07 keeps error text stable across parser swaps — paths referenced but values not embedded (existing behavior preserved) |
| Parser swap injection (third-party `Parser` implementation) | Tampering / Information Disclosure | `Parser` interface is NARROW — implementations receive bytes + rgID, cannot reach into builder state. Sink method inputs validated by existing staging methods (path normalization, type checks). |

**Posture:** Phase 13 is code-only refactor with zero new inputs, zero new network paths, zero new crypto. ASVS review confirms no additional surface area introduced.

## Verification Plan — "Zero Behavior Change" Proof

The merge gate is **four concurrent assertions**, all of which MUST pass:

1. **Byte-level parity (D-04 + D-05 dims #1/#4):**
   `TestParserParity_AuthoredFixtures` passes with `bytes.Equal(encoded, golden)` for every authored fixture. If any byte differs, the refactor has drifted. The 7 authored fixtures span int64 boundaries, null vs missing, deep nesting, unicode keys, empty arrays, large strings, transformer paths.

2. **Evaluate matrix parity (D-05 dim #3):**
   `TestParserParity_EvaluateMatrix` passes for all 12 operators (`EQ`, `NE`, `GT`, `GTE`, `LT`, `LTE`, `IN`, `NIN`, `IsNull`, `IsNotNull`, `Contains`, `Regex`), each with one matching and one pruning predicate. `RGSet.ToSlice()` comparison via `intSliceEqual`.

3. **Property-test determinism (D-05 dim #2):**
   `TestParserParity_GopterByteIdentical` passes across `GenTestDocs`, `GenTestDocsWithNulls`, `GenMixedTypeDocs`, `GenJSONDocument` — two builds with identical inputs produce identical bytes. If property fails with a shrunk counter-example, that case becomes an authored regression fixture.

4. **Existing suite + benchmarks (regression guard):**
   - `make test` green (all pre-existing tests pass unchanged).
   - `go test -run TestNumericIndexPreservesInt64Exactness` passes (BUILD-03 preserved — Pitfall #1).
   - `go test -bench=BenchmarkAddDocument -benchmem -count=10` within **2% ns/op** and **0 additional allocs/op** vs. baseline captured before refactor on same commit.

**Proof artifact to attach to PR description:**

```
# Captured on <commit-sha-before-refactor>
BenchmarkAddDocument-8          XXX ns/op   XXX B/op   XXX allocs/op
BenchmarkAddDocumentPhase07-8   XXX ns/op   XXX B/op   XXX allocs/op

# Captured on <commit-sha-after-refactor>
BenchmarkAddDocument-8          XXX ns/op   XXX B/op   XXX allocs/op
BenchmarkAddDocumentPhase07-8   XXX ns/op   XXX B/op   XXX allocs/op

Delta: <=2% ns/op, 0 allocs/op
```

If any of the four fails, the refactor is not behavior-neutral — either the seam leaked a bug or the code move is incomplete. Do not merge.

## Risk Register — SIMD-Prep Discipline

Concrete risks ordered by blast radius:

### Risk 1 (CRITICAL): BUILD-03 int64 fidelity silently breaks
**Why this is THE risk for Phase 13:** The seam exists *so that* SIMD v1.2 can slot in without re-implementing the int64 classifier. If that classifier moves into the parser (or gets subtly duplicated), v1.2 starts from the wrong foundation.

**Mitigation:**
- Classifier (`stageJSONNumberLiteral`, `parseJSONNumberLiteral`) stays on `*GINBuilder`. Verified at PR review by grepping `strconv.ParseInt` / `strconv.ParseFloat` — they must appear ONLY in `builder.go`, not in `parser_*.go`.
- `StageJSONNumber(state, path, raw string)` sink method signature preserves raw source text.
- `TestNumericIndexPreservesInt64Exactness` (existing, `gin_test.go`) runs through the new path without modification.

### Risk 2 (HIGH): Transformer buffering hook misrouted
See Pitfall 2 above. Mitigation is the `transformers-iso-date-and-lower` authored fixture.

### Risk 3 (HIGH): PR bundles non-neutral changes (Pitfall #11)
See Pitfall 4. Mitigation is three-commit structure and reviewer grep.

### Risk 4 (MEDIUM): Heap allocation via interface boxing
See Pitfall 5. Mitigation is `stdlibParser` as value-typed zero struct, set once at NewBuilder.

### Risk 5 (MEDIUM): Goldens-format version drift
If a future PR bumps the serialization format (v10+) without refreshing goldens, the parity harness produces a false failure.

**Mitigation:**
- `testdata/parity-golden/README.md` documents the regeneration step.
- README explicitly says: "If the parity harness fails AND you just bumped the format version AND the legacy-vs-new path is proven equivalent at the code level, regenerate using `go test -tags regenerate_goldens`."
- In the PR that bumps the format, require refreshed goldens alongside.

### Risk 6 (LOW): `stdlibParser` being accidentally reconstructed per call
See Pitfall 5. Mitigation: set at `NewBuilder`, verify via benchmark.

## Sources

### Primary (HIGH confidence)
- `./builder.go` lines 1-130, 280-380, 438-654 — direct read confirming current staging pipeline [VERIFIED: Read tool]
- `./gin.go` lines 13, 29 — `MagicBytes = "GIN\x01"`, `Version = uint16(9)` — serialization format confirmation [VERIFIED: Grep]
- `./gin.go:397` — `func (c *GINConfig) representations(canonicalPath string)` — transformer lookup used by `ShouldBufferForTransform` [VERIFIED: Grep]
- `./generators_test.go` lines 87, 276-373 — `GenJSONDocument`, `GenTestDocs`, `GenTestDocsWithNulls`, `GenMixedTypeDocs` [VERIFIED: Read tool]
- `./integration_property_test.go` lines 1-100 — gopter property-test template [VERIFIED: Read tool]
- `./benchmark_test.go` lines 891, 972 — `BenchmarkAddDocument` and `BenchmarkAddDocumentPhase07` existence [VERIFIED: Grep]
- `./gin_test.go` lines 139, 180, 1116, 2733, 2791, 2832 — existing Evaluate test matrices [VERIFIED: Grep]
- `.planning/research/ARCHITECTURE.md` §Pattern 1 (lines 57-173) — interface sketch adapted to D-01/D-02 [CITED]
- `.planning/research/PITFALLS.md` §Pitfall 1 (lines 26-46), §Pitfall 11 (lines 258-276) — SIMD int demotion + behavior-neutral PR discipline [CITED]
- `.planning/phases/13-parser-seam-extraction/13-CONTEXT.md` — locked decisions D-01..D-07 [VERIFIED: Read tool]
- `./go.mod` — dependency versions: `pkg/errors v0.9.1`, `gopter v0.2.11` [VERIFIED: Grep for versions]
- `git tag` — `v1.0` present locally, enabling goldens regeneration per D-04 [VERIFIED: Bash `git tag`]

### Secondary (MEDIUM confidence)
- `.planning/research/FEATURES.md` — TS-SIMD-1/2/4 parity infrastructure matrix [CITED]
- `.planning/milestones/v1.0-phases/07-builder-parsing-numeric-fidelity/` — BUILD-03 contract referenced [CITED]
- `.planning/milestones/v1.0-phases/10-serialization-compaction/` — v9 format stability [CITED]

### Tertiary (LOW confidence)
- None — all load-bearing claims verified via direct code read or explicit CONTEXT.md lock.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies, all versions verified against `go.mod`
- Architecture: HIGH — interface shape locked by CONTEXT.md D-01/D-02; refactor mechanics grounded in direct read of `builder.go`
- Pitfalls: HIGH — CRITICAL pitfalls (#1 int64 drift, #11 PR discipline) explicitly flagged and mitigated; MEDIUM pitfalls have concrete warning signs
- Validation Architecture: HIGH — existing test fixtures reused; goldens strategy locked by D-04; four-dimension harness specified with runnable commands

**Research date:** 2026-04-21
**Valid until:** 2026-05-21 (stable — pure refactor, no fast-moving upstream concerns)
