# Phase 19: SIMD Dependency Decision & Integration Strategy - Context

**Gathered:** 2026-04-27
**Status:** Ready for planning

<domain>
## Phase Boundary

Decision/strategy phase. Lock the four blockers that gate SIMD implementation so Phase 21 (parser adapter) and Phase 22 (validation/CI) can start without revisiting first-order questions:

1. Upstream coordination posture — pinning, bumping, license/NOTICE acknowledgement.
2. Shared-library distribution — runtime loading, CI bootstrap, consumer guidance.
3. Opt-in API + unsupported-platform behavior — constructor shape, load-failure semantics, CI matrix.
4. Stop/fallback condition — what counts as a HARD vs SOFT blocker if Phase 21/22 hits trouble.

**No code lands in this phase.** Output is `19-CONTEXT.md` (this file) + `19-PLAN.md`. Phase 21 implements `parser_simd.go` behind `//go:build simdjson`; Phase 22 wires parity tests, benchmarks, and CI.

**Live upstream state (verified 2026-04-27, supersedes 2026-04-21 STACK.md research):**
- ✅ MIT LICENSE present at `github.com/amikos-tech/pure-simdjson` root (gh API: `"license": {"key": "mit"}`)
- ✅ Tags published: `v0.1.0`, `v0.1.2`, `v0.1.3`, `v0.1.4` (latest, SHA `0f53f3f2e8bb9608d6b79211ffc5fc7b53298617`)
- ✅ Pre-built signed binaries for 5 platforms in every release: `darwin-amd64.dylib`, `darwin-arm64.dylib`, `linux-amd64.so`, `linux-arm64.so`, `windows-amd64-msvc.dll` — each with `.pem` + `.sig` + `SHA256SUMS`
- ✅ Built-in bootstrap: first `purejson.NewParser()` call auto-downloads from CloudFlare R2 (primary) with GitHub Releases fallback, SHA-256 verifies, atomically caches at `$XDG_CACHE_HOME/pure-simdjson` with flock
- ✅ ABI version check enforced inside `purejson.NewParser()`
- ✅ Env overrides: `PURE_SIMDJSON_LIB_PATH` (verbatim path, bypasses all bootstrap), `PURE_SIMDJSON_BINARY_MIRROR` (custom mirror), `PURE_SIMDJSON_DISABLE_GH_FALLBACK`, `PURE_SIMDJSON_CACHE_DIR`
- ✅ Air-gapped CLI: `pure-simdjson-bootstrap fetch --all-platforms --dest ...`

**Carrying forward from earlier phases:**
- **Phase 13 (D-01, D-02):** Same-package SIMD parser behind `//go:build simdjson`. Only `Parser` interface and `WithParser(Parser) BuilderOption` exported. Sink stays unexported. Numeric classifier stays in `builder.go` (Pitfall #1 defense).
- **Phase 13 (D-06):** `WithParser(nil)` returns error; empty `Name()` rejected at `NewBuilder`. Default parser returns `"stdlib"`.
- **Phase 13 (D-07):** Parsers wrap their own errors; `AddDocument` returns `parser.Parse(...)` errors verbatim. Phase 14 adds parser name as a structured telemetry attribute, not in the error string.
- **Phase 07 contract (BUILD-03):** Exact-int64 fidelity; SIMD must preserve `int64`/`uint64`/`float64` split. `pure-simdjson`'s typed accessors (`TypeInt64`/`TypeUint64`/`TypeFloat64` + `ErrPrecisionLoss`) satisfy this natively, stronger than `json.Number.Int64()`.
- **PROJECT.md:** correctness → usefulness → performance; "avoid gratuitous API churn"; default builds remain stdlib-only with no SIMD compile-time dep (REQUIREMENTS SIMD-05).

</domain>

<decisions>
## Implementation Decisions

### Upstream Coordination (SIMD-01)

- **D-01: Pin to tag `v0.1.4`** in `go.mod`. `go get github.com/amikos-tech/pure-simdjson@v0.1.4` — Go writes `v0.1.4` to `go.mod` and freezes the SHA in `go.sum` per Go convention. Human-readable changelog tracking + immutability.

- **D-02: Pin-and-hold bump cadence.** Stay on the pinned tag until ami-gin needs a specific upstream fix/feature, or upstream cuts a major-shape change (v0.2/v1.0). All bumps go through PR review. Avoids parity-test churn (Phase 22 goldens recompute on every bump).

- **D-03: NOTICE.md + README dependency credit.** Add `NOTICE.md` to ami-gin root mirroring `pure-simdjson`'s NOTICE (covers simdjson C++ Apache-2.0 chain). README "Dependencies" section credits `pure-simdjson` and links to upstream. No vendoring of upstream LICENSE (Go module graph distributes it transitively).

- **D-04: Manual license-drift review on bump.** No CI tooling for license-diff. Pin-and-hold cadence already gates bumps through PR review; reviewer manually verifies LICENSE+NOTICE haven't changed at the new tag. Matches the human-in-the-loop bump policy.

### Shared-Library Distribution (SIMD-02)

- **D-05: Pure delegation with friendlier error wrap.** `NewSIMDParser` is a thin wrapper over `purejson.NewParser()`. On construction error, ami-gin re-wraps with `errors.Wrap` and ami-gin-specific guidance pointing at `PURE_SIMDJSON_LIB_PATH` and `docs/simd-deployment.md`. ami-gin does **not** reimplement download, cache, or mirror logic.

- **D-06: CI bootstrap = `actions/cache` + auto-download.** ami-gin's `-tags simdjson` CI jobs let `purejson.NewParser()` auto-download on first run, with GitHub Actions cache keyed on the pinned `pure-simdjson` tag. Cold cache ≈ 30s for a download; warm cache near-zero. Exercises the same code path consumers hit.

- **D-07: Tiered consumer documentation.**
  - README: one-line happy path ("install with `-tags simdjson`; first call to `NewSIMDParser` auto-downloads the native lib").
  - `docs/simd-deployment.md`: ops scenarios (air-gapped, corporate mirrors, hermetic builds, `PURE_SIMDJSON_DISABLE_GH_FALLBACK=1`, `PURE_SIMDJSON_LIB_PATH` recipe), each cross-linking upstream `bootstrap.md` rather than duplicating.
  - CHANGELOG v1.3 entry: SIMD path landed, upstream dep credited.

- **D-08: No release-time binary-reachability check.** Pin-and-hold bump cadence (D-02) means the bump PR's CI already verified the binary is reachable. No additional release-time step needed.

### Opt-In API + Platform Fallback (SIMD-03)

- **D-09: Constructor shape — `NewSIMDParser() (Parser, error)`.** Two-step usage: `p, err := gin.NewSIMDParser(); ... gin.NewBuilder(gin.WithParser(p))`. Matches Phase 13 D-02 exactly; matches existing `New*` constructor pattern (`NewBuilder`, `NewBloomFilter`, `NewS3Client`). DX wins on error locality, parser inspection, reuse across builders, test substitution, future extensibility, pattern consistency. Loses only on caller line count (delta = 2 lines).

  Lives in `parser_simd.go` behind `//go:build simdjson`. Returns `Name() == "pure-simdjson"` (no version suffix in v1.3; can extend later if telemetry needs it).

- **D-10: Hard error on runtime load failure + documented fallback recipe.** When `-tags simdjson` is set but `purejson.NewParser()` cannot load the native library (download fail, ABI mismatch, missing CPU features, etc.), `NewSIMDParser` returns `(nil, error)` wrapped with the friendlier-error guidance from D-05. Caller decides whether to fall back to stdlib explicitly:

  ```go
  p, err := gin.NewSIMDParser()
  if err != nil {
      log.Warn("SIMD unavailable, falling back to stdlib", "err", err)
      return gin.NewBuilder() // default = stdlib
  }
  return gin.NewBuilder(gin.WithParser(p))
  ```

  Rationale: silent fallback would break audit trails (telemetry says `"stdlib"` for both intentional-stdlib and degraded-SIMD), surprise developers in production via missing perf gains, and force a one-size-fits-all policy on all callers. Hard error relocates the surprise to construction time where the developer can fix it. The fallback recipe ships in `docs/simd-deployment.md`.

- **D-11: Full 5-platform CI matrix for `-tags simdjson`.** Run linux-amd64, linux-arm64, darwin-amd64, darwin-arm64, windows-amd64 with `-tags simdjson` on every PR. Matches `pure-simdjson`'s supported-platform set; produces strong v1.3 acceptance evidence; catches platform-specific numeric/binding regressions before release. Default (non-SIMD) CI matrix is unchanged.

- **D-12: Parameter-free constructor for v1.3.** `NewSIMDParser() (Parser, error)` — no options. Ship minimal per Phase 13 "ship minimal now" precedent. Adding options later as `NewSIMDParser(opts ...SIMDOption) (Parser, error)` is additive and non-breaking, so deferral is cheap.

### Stop / Fallback Condition (SIMD-03 success criterion #3)

- **D-13: Tiered hard/soft policy.** ROADMAP success criterion #3 satisfied minimally via the switch table below. **No speculative runbook authored** (deferral plans, branch/PR steps) — those will be authored if and when triggered, using existing GSD commands. The original criterion was conservative when LICENSE/tags/binaries were missing; with those resolved, the switch table is sufficient.

  | Trigger | Severity | Action |
  |---|---|---|
  | SIMD path produces non-parity encoded bytes or query results vs. stdlib (Phase 22 parity harness fails) | **HARD** | Halt Phase 21/22, defer SIMD to v1.4. Violates Phase 07 BUILD-03 contract — cannot ship correctness-broken. |
  | `pure-simdjson` upstream archives or makes an unrecoverable v0.2 break mid-implementation | **HARD** | Halt Phase 21/22, defer SIMD to v1.4. Re-evaluate dep landscape in v1.4 milestone. |
  | One of the 5 `-tags simdjson` CI platforms flakes or fails while others pass | **SOFT** | Ship with that platform documented as "tier 2" in `docs/simd-deployment.md`; require manual verification per release until resolved. |
  | Measured SIMD speedup smaller than expected on Phase 22 benchmarks | **Decide on evidence** | No fixed numeric threshold pre-committed. Phase 22 reports actual numbers; ship/cut decision made then with workload context. |

### Claude's Discretion

- **`Name()` exact value** — `"pure-simdjson"` (no version suffix) for v1.3. Phase 13 D-06 cached `parserName` once at `NewBuilder`. If telemetry wants version pinning later, format becomes `"pure-simdjson/v0.1.4"` — non-breaking.
- **Where the friendlier-error message text lives** — inline string constant inside `parser_simd.go` is fine; no new error-message scaffolding needed.
- **`docs/simd-deployment.md` exact section structure** — planner's call. Suggested top-level sections: "Quick start (auto-download)", "Air-gapped deployment", "Corporate mirror / hermetic builds", "Falling back to stdlib programmatically", "CI integration".
- **CHANGELOG wording for SIMD landing** — planner drafts during Phase 21; reviewer signs off.
- **Whether `parser_simd.go` lives next to `parser.go` or in a `parser/` sub-directory** — Phase 13 already locked same-package, root-level. Don't revisit.
- **Whether to emit a one-time INFO log on SIMD parser construction success** — planner's call; not required.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents (researcher, planner) MUST read these before acting.**

### Phase specification
- `.planning/ROADMAP.md` §Phase 19 — goal, 3 success criteria, depends-on (Phase 13)
- `.planning/REQUIREMENTS.md` §SIMD Unblock And Strategy — SIMD-01, SIMD-02, SIMD-03 definitions
- `.planning/PROJECT.md` §Constraints — "avoid gratuitous API churn" informs D-12; correctness → usefulness → performance ordering informs D-13 hard/soft split

### Prior-phase decisions (load-bearing)
- `.planning/phases/13-parser-seam-extraction/13-CONTEXT.md` — D-01 (sink shape), D-02 (export minimization, build-tag strategy), D-06 (parser name caching, nil/empty rejection), D-07 (error-wrap policy)
- `.planning/phases/13-parser-seam-extraction/13-RESEARCH.md` — parser-seam architecture rationale
- `.planning/milestones/v1.0-phases/07-builder-parsing-numeric-fidelity/` — BUILD-03 exact-int64 contract that SIMD must preserve

### v1.1 research (still relevant; verify upstream state at consumption time)
- `.planning/research/STACK.md` §1 SIMD JSON Builder Path — `pure-simdjson` adoption rationale, alternatives rejected (`minio/simdjson-go`, `goccy/go-json`, CGo simdjson). **NOTE:** the LICENSE/tags/binaries blockers cited in this doc are stale as of 2026-04-27 — see `<domain>` "Live upstream state" above.
- `.planning/research/ARCHITECTURE.md` §Pattern 1 — concrete `Parser`/`ParserSink` integration sketch and new-files table
- `.planning/research/PITFALLS.md` §Pitfall 1 (SIMD numeric drift), §Pitfall 8 (ARM64 / CPU feature portability) — informs D-09, D-10, D-11

### Upstream documentation (`pure-simdjson` v0.1.4)
- `https://github.com/amikos-tech/pure-simdjson/blob/v0.1.4/README.md` — installation, supported platforms, ABI version-check semantics
- `https://github.com/amikos-tech/pure-simdjson/blob/v0.1.4/docs/bootstrap.md` — auto-download, cache layout, env vars (`PURE_SIMDJSON_LIB_PATH`, `_BINARY_MIRROR`, `_DISABLE_GH_FALLBACK`, `_CACHE_DIR`), air-gapped recipe, `pure-simdjson-bootstrap` CLI
- `https://github.com/amikos-tech/pure-simdjson/blob/v0.1.4/LICENSE` — MIT (verified 2026-04-27)
- `https://github.com/amikos-tech/pure-simdjson/blob/v0.1.4/NOTICE` — Apache-2.0 attribution chain to upstream simdjson C++

### Code anchors (current ami-gin tree)
- `builder.go:115` — `NewBuilder`, where parser default and `parserName` caching land (Phase 13)
- `builder.go:287` — `AddDocument` call site of `b.parser.Parse(jsonDoc, pos, b)` (Phase 13)
- Phase 21 will create `parser_simd.go` (new file, `//go:build simdjson`)
- `cmd/gin-index/experiment.go` — `--parser=simd` flag wiring planned in Phase 21/22

### Project hygiene
- `.planning/PROJECT.md` §Constraints — "avoid gratuitous API churn", "Benchmark-backed changes"
- `CHANGELOG.md` — v1.3 entry will be authored in Phase 21 with SIMD-landing note

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **Phase 13 parser seam** — `Parser` interface + `WithParser(Parser) BuilderOption` already exported. SIMD parser slot is empty and waiting; D-09 fills it via `NewSIMDParser`.
- **`parserSink` (unexported)** — Phase 13 D-01's six-method sink (`BeginDocument`, `StageScalar`, `StageJSONNumber`, `StageNativeNumeric`, `StageMaterialized`, `ShouldBufferForTransform`) is exactly the API a SIMD parser needs to walk `purejson.Element` and feed staging without round-tripping through `any`.
- **`stdlibParser` precedent** — `parser_stdlib.go` shows the exact integration shape (parser holds no per-Parse state, calls sink methods, emits `errors.Wrap`-prefixed parse errors). Phase 21 mirrors this for `simdParser`.
- **`generators_test.go` + `parser_parity_test.go` harness** (Phase 13 D-04, D-05) — the parity-test infrastructure already exists; Phase 22 just registers `simdParser` as a second target alongside `stdlibParser` and reuses the goldens.
- **`pkg/errors`** — established `errors.Wrap`/`errors.Errorf` pattern; D-05 friendlier-error wrap follows it.
- **`PROJECT.md` "Functional options" pattern** — `BuilderOption` returning `error` is the established convention; D-09 `NewSIMDParser` doesn't need a new convention.

### Established Patterns

- **`New*` constructors return `(*T, error)`** — `NewBuilder`, `NewBloomFilter`, `NewRGSet`, `NewS3Client`. D-09 matches this exactly.
- **Build-tagged same-package files** — Phase 13 anticipated this for SIMD; no precedent in current tree, but the design contract (D-02 of Phase 13) is locked.
- **Single flat package `gin`** — `parser_simd.go` lives at module root, not in a subdirectory.
- **CHANGELOG-flagged adds for non-default behavior** — v1.2 set this precedent (`IngestFailureMode` rename); v1.3 SIMD-landing CHANGELOG entry follows it.

### Integration Points

- **`parser_simd.go`** (new, Phase 21) — `//go:build simdjson` package file containing `NewSIMDParser`, `simdParser` struct (unexported), `simdParser.Name()` returning `"pure-simdjson"`, `simdParser.Parse(jsonDoc, rgID, sink)` walking `purejson.Element` and feeding the sink.
- **`go.mod`** — Phase 21 adds `github.com/amikos-tech/pure-simdjson v0.1.4` (only when `-tags simdjson` builds run; module graph is conditional via build tags). `go.sum` updated with SHA.
- **`NOTICE.md`** (new, root) — Phase 21 adds; mirrors `pure-simdjson` NOTICE for Apache-2.0 chain to upstream simdjson C++.
- **`docs/simd-deployment.md`** (new) — Phase 21 authors; tiered ops guide.
- **`.github/workflows/`** — Phase 22 adds `-tags simdjson` jobs across linux-amd64, linux-arm64, darwin-amd64, darwin-arm64, windows-amd64 with `actions/cache` keyed on `pure-simdjson` tag.
- **README** — Phase 21 adds happy-path SIMD note + Dependencies credit.
- **`cmd/gin-index/experiment.go`** — `--parser=simd` flag wiring; Phase 21/22 detail.

### Constraints Enabled By Existing Architecture

- **Single-threaded builder** — `simdParser` can hold per-Parse state without sync primitives; matches `stdlibParser`.
- **`purejson.Element` typed accessors** — `TypeInt64`/`TypeUint64`/`TypeFloat64` + `ErrPrecisionLoss` map cleanly to the sink's `StageNativeNumeric` and `StageJSONNumber`. No round-trip through `any` for scalar leaves.
- **`v9` serialization format** — already deterministic; Phase 22 parity goldens reuse Phase 13's `testdata/parity-golden/` infrastructure.

</code_context>

<specifics>
## Specific Ideas

- **Constructor signature is exact:** `func NewSIMDParser() (Parser, error)`. No variadic options in v1.3.
- **`Name()` returns the literal string `"pure-simdjson"`** — no version suffix in v1.3; format `"pure-simdjson/v0.1.4"` reserved for later if telemetry needs it.
- **`go.mod` line:** `require github.com/amikos-tech/pure-simdjson v0.1.4` (D-01 pinning).
- **Friendlier-error wrap message format** (D-05/D-10): `errors.Wrap(err, "initialize pure-simdjson SIMD parser; set PURE_SIMDJSON_LIB_PATH or see docs/simd-deployment.md")`. Inline string; no new error-class machinery.
- **Documented fallback recipe in `docs/simd-deployment.md`** is exactly the 4-line snippet from D-10. Planner copies it verbatim.
- **CI cache key:** `pure-simdjson-${{ matrix.os }}-${{ matrix.arch }}-v0.1.4` so cache invalidates automatically on bumps.
- **5-platform matrix labels:** `linux-amd64`, `linux-arm64`, `darwin-amd64`, `darwin-arm64`, `windows-amd64-msvc`.

</specifics>

<deferred>
## Deferred Ideas

- **`NewSIMDParser(opts ...SIMDOption)`** — speculative options surface. Add when a real caller asks (max-doc-size, parsing flags, etc.).
- **`Name()` versioned format** (`"pure-simdjson/v0.1.4"`) — defer until telemetry has a concrete need to disambiguate parser versions in dashboards.
- **CI release-time binary-reachability check** — D-08 explicitly defers; bump-PR CI already covers this.
- **License-drift CI tooling (`go-licenses` etc.)** — D-04 explicitly defers; manual review on bump is sufficient.
- **Stop-condition concrete deferral runbook** — D-13 explicitly defers; switch table is sufficient. Author the runbook only if a HARD trigger fires.
- **`WithSIMDParser()` one-step convenience option** — not shipping; would duplicate Phase 13 D-02's `WithParser` seam.
- **Configurable warn-and-fallback policy** — D-10 explicitly defers; default is hard error, callers wrap.
- **Per-document SIMD/stdlib failover** — out of scope; v1.3 is parser selection at builder construction, not per-document.

</deferred>

---

*Phase: 19-simd-dependency-decision-integration-strategy*
*Context gathered: 2026-04-27*
