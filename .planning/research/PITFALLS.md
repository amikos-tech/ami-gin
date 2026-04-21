# Pitfalls Research — v1.1 Performance, Observability & Experimentation

**Domain:** Adding SIMD JSON parsing, observability primitives, and an experimentation CLI to an existing pure-Go pruning-index library
**Researched:** 2026-04-21
**Confidence:** HIGH for SIMD numeric drift and slog allocation traps (multiple verified sources); MEDIUM for OpenTelemetry span-explosion figures and CLI streaming behavior (single-source-verified); LOW for claims about `amikos-tech/pure-simdjson` specifically (library surface not reachable via WebSearch — treat as upstream-unknown until Phase starts).

## v1.0 Contract That Must Not Regress

Before enumerating pitfalls, the invariants every new pitfall is measured against:

| Invariant | Source | Regression signal |
|-----------|--------|-------------------|
| Exact-`int64` round-trip (BUILD-03) | `builder.go:654-750`, `gin_test.go:2988-3233` | Any path with integer-only values decodes to `float64` or loses precision above `2^53` |
| Atomic document ingest (BUILD-01) | `builder.go:287-345`, `gin_test.go:2933-2986` | Rejected doc leaves `docIDToPos`, `pathData`, or finalized indexes partially mutated |
| Pure Go, zero CGo | `go.mod`, `parquet.go`, existing deps | `go build` fails with `CGO_ENABLED=0` on any supported platform |
| Additive public API | v0.1.0 → v1.0 migration | Existing consumer code requires changes other than imports to keep compiling |
| Deterministic benchmark replay (BUILD-05) | `benchmark_test.go:206-229,564-588,972-1065` | Same corpus + same commit produces non-trivially different ns/op or alloc counts across runs |
| No false negatives in row-group selection | Phase 06 + Phase 08 evidence | Any predicate prunes a row group whose source doc would match |

Every pitfall below is a way these invariants get silently broken.

---

## Critical Pitfalls

### Pitfall 1: SIMD parser silently demotes large integers to float64

**What goes wrong:**
simdjson-family parsers classify a numeric token as `int64`, `uint64`, or `float64` based on *whether the decoded value fits*, not on *whether the source had a decimal point*. A value like `27670116110564327426` (overflows `uint64`) is returned as `2.767e+19`. A value like `10000000000000001` (>2^53, fits `int64`) often decodes fine, but anything beyond `uint64` silently loses precision. Phase 07's guard (`maxExactFloatInt := int64(1 << 53)`) is defensive on the *consumer* side of `json.Number`, but a SIMD parser that has already handed back a `float64` has erased the evidence.

**Why it happens:**
`json.Decoder.UseNumber()` returns `json.Number` (a `string`), which the Phase 07 classifier re-parses with `strconv.ParseInt` first, then `strconv.ParseFloat`. SIMD parsers return a pre-typed value (`int64`/`uint64`/`float64`) with no raw text preserved by default. The moment the parser hands back `float64`, the builder has no way to know whether the source was `1.0e19` or `10000000000000000000`.

**How to avoid:**
- Before adopting any SIMD parser, write a parity test (`TestSIMDParserPreservesExactInt64` / `TestSIMDParserRejectsLossyIntegerDecode`) that feeds the exact corpus used in `gin_test.go:2988-3233` through the SIMD path and asserts byte-for-byte identical `IntGlobalMin/Max` and `ValueType` state.
- Require the parser to expose either (a) the source text of the number, or (b) an overflow flag equivalent to simdjson's `FloatOverflowedInteger`. If neither exists, the parser cannot back the BUILD-03 contract.
- Implement SIMD as an *alternative* path routed through the existing `documentBuildState` staging so the classifier stays the single source of truth. Do not let the SIMD parser populate `NumericIndex` directly.
- Add a property-based test that generates integers in `[-2^63, 2^63)` and asserts decode parity between the legacy parser and the SIMD parser on the same document.

**Warning signs:**
- SIMD benchmark results show "better" numeric ingest but `IntGlobalMin`/`IntGlobalMax` differ from the legacy path on the same doc.
- `TestNumericIndexPreservesInt64Exactness` passes under legacy but fails or is silently skipped under SIMD.
- A reviewer notices the SIMD path returns `float64` and the code quietly `int64()`-casts it instead of erroring.
- `parser=simd` and `parser=explicit-number` rows in the Phase 07 matrix diverge on `shape=int-only`.

**Phase to address:** **SIMD-INTEGRATION (first SIMD phase).** BUILD-03 parity regression test must land in the same PR that introduces the SIMD dependency. Mergeability blocker.

---

### Pitfall 2: SIMD parser introduces CGo or non-portable assembly

**What goes wrong:**
`minio/simdjson-go` requires AVX2 + CLMUL (Haswell+ on Intel, Ryzen on AMD) and historically had undefined references on ARM64 (`find_structural_bits_in_slice`, `parse_string_simd_validate_only`). Adopting it naively breaks ARM64 builds, Apple Silicon, and any consumer who expects `CGO_ENABLED=0 GOARCH=arm64 go build` to work.

**Why it happens:**
Fastest-path SIMD in Go is built via `c2goasm` (translates C intrinsics into Go assembly). It targets x86-64 first; ARM64 NEON ports lag. Consumers of this library already treat "pure Go, zero CGo, cross-compiles anywhere" as a reproducibility feature.

**How to avoid:**
- Confirm on day one whether the chosen library (`amikos-tech/pure-simdjson` per `PROJECT.md`) ships ARM64 support and zero CGo. The name suggests "pure" Go, but verify with `go list -deps -f '{{.CgoFiles}}'` and a cross-compile test on `GOARCH=arm64` and `GOOS=linux GOARCH=amd64 CGO_ENABLED=0`.
- Make the SIMD path *opt-in* via a build tag (`//go:build simd`) or a separate import path (`github.com/amikos-tech/ami-gin/parser/simd`), not a transitive dependency of the root package. Consumers who don't opt in should not pay for it in `go.sum` size, build graph, or cross-compile failures.
- Add a CI matrix job that builds on `linux/amd64`, `linux/arm64`, `darwin/arm64`, `windows/amd64` with `CGO_ENABLED=0`. Fail the PR if any drop.
- Document the CPU feature detection fallback: if the runtime CPU lacks AVX2/NEON, the SIMD path MUST fall back to the explicit-number path, not panic.

**Warning signs:**
- `go.sum` grows by >5MB after adding the dependency (assembly blobs).
- Any file matches `//go:build amd64` without a corresponding `//go:build arm64` or generic fallback.
- Cross-compile fails but unit tests pass on dev machine.
- Consumer reports "works locally, fails in Docker" after pulling v1.1.

**Phase to address:** **SIMD-INTEGRATION.** Cross-compile CI matrix must land in the same phase as the SIMD dependency.

---

### Pitfall 3: SIMD parser reuse is not goroutine-safe

**What goes wrong:**
SIMD parsers amortize per-document cost by reusing a pre-allocated `ParsedJson` struct (aligned buffer + tape). Concurrent `AddDocument()` calls that share one parser produce corrupted tapes — races on the internal buffer show up as intermittent test failures under `-race`, or as silent wrong values in numeric stats.

**Why it happens:**
The builder is documented as single-threaded (see `ARCHITECTURE.md`: "No shared state or concurrency primitives"), but adding a parser that *holds internal buffers* changes the threading contract in a non-obvious way. Users who assume "I'll just run N builders in parallel" will hit it first.

**How to avoid:**
- Keep `GINBuilder` explicitly single-threaded in docs. If v1.1 adds a parallel ingest example, each goroutine owns its own `ParsedJson` instance.
- Wrap parser acquisition in `sync.Pool` only if benchmarks show it helps — and verify `-race` clean under a concurrent-ingest stress test.
- Add `TestConcurrentBuildersAreIndependent` that runs N=8 builders in parallel on disjoint corpora, calls `Finalize()`, and asserts encoded output is identical to serial runs.

**Warning signs:**
- `go test -race ./...` produces a new data race report on `NumericIndex` or `pathBuildData`.
- Intermittent test flakes in CI that reproduce only with `-count=10` or higher.
- Benchmark results vary widely run-to-run even on fixed corpora.

**Phase to address:** **SIMD-INTEGRATION.** Race-detector CI run required.

---

### Pitfall 4: Observability calls allocate on the disabled hot path

**What goes wrong:**
Structured-logging APIs allocate an attribute slice or a `slog.Record` *before* checking whether the handler is interested. A call like `logger.Info("pruned", slog.Int("rg", rg), slog.String("path", p))` allocates the attrs even if level is `LevelError`. At query hot-path rates (millions of RG decisions per evaluation), this turns a "zero-cost when disabled" claim into a measurable regression.

**Why it happens:**
`slog` is close to zero-alloc when the logger is disabled (benchmarks show ~4ns/op, 0 allocs with `DiscardHandler`), but only when you use `LogAttrs()` with pre-constructed `Attr` values *and* check `Enabled(ctx, level)` before constructing anything expensive. The ergonomic `logger.Info(...)` form with variadic key/value pairs allocates. Library authors routinely write the ergonomic form in hot paths and benchmark in release builds where the logger is set to discard — missing the allocation the ergonomic form still makes.

**How to avoid:**
- Hot-path logging uses the pattern `if h.Enabled(ctx, level) { h.LogAttrs(ctx, level, msg, attr1, attr2) }`. Benchmark this form with `testing.AllocsPerRun` and assert `0 allocs/op` with `slog.DiscardHandler` (Go 1.24+) or a custom nop handler.
- Provide a `Logger` interface the library controls (mirroring `go-logr/logr` style) and a default no-op implementation. Consumers adapt their `slog.Logger` via a thin wrapper. This lets the library avoid slog's variadic-attr allocation path entirely.
- Add a benchmark (`BenchmarkEvaluateDisabledLogging`) that compares hot-path query cost with and without a configured logger on the same corpus. Regression threshold: ≤1% ns/op, 0 additional allocs/op.
- Never call `fmt.Sprintf` inside a log-site argument — it evaluates before the level check.

**Warning signs:**
- `go test -bench=BenchmarkEvaluateEQ -benchmem` shows allocs/op increase after a telemetry phase even with no logger configured.
- Query hot path contains `logger.Info(...)` calls without a surrounding `Enabled()` guard.
- Profile shows `slog.Record.AddAttrs` or `runtime.convT` under `evaluateEQ`/`evaluateRegex`.

**Phase to address:** **OBSERVABILITY (first telemetry phase).** "Zero-cost when disabled" benchmark is a merge blocker.

---

### Pitfall 5: Forcing consumers onto a specific logging SDK

**What goes wrong:**
The library imports `log/slog` directly and exposes a `*slog.Logger` option. Consumers already using `zap`, `zerolog`, `logr`, or the legacy `log.Logger` (note: the current `query.go:17` uses `*log.Logger` for `adaptiveInvariantLogger`) now have two logger configurations to keep in sync. Worse, they pay for a stdlib dependency they didn't ask for.

**Why it happens:**
`log/slog` is in stdlib, so "it's free" feels right. But libraries that commit to one SDK's types in their public API create churn when consumers want a different backend — and `slog.Handler` is itself a customization seam, so consumers often already have a handler they'd rather plug in directly. The existing `*log.Logger` field in `query.go` already illustrates this: a v1.0 consumer is coupled to legacy `log`.

**How to avoid:**
- Define a minimal local interface (`type Logger interface { LogEvent(ctx, level, msg, attrs...) }`) or adopt `go-logr/logr` as the interop boundary. Do not expose `*slog.Logger` or `slog.Handler` in the public API.
- Ship adapters: `ginlog.FromSlog(h slog.Handler) Logger`, `ginlog.FromLogr(l logr.Logger) Logger`, `ginlog.FromStd(l *log.Logger) Logger`. Keep adapters in a sub-package so the root import doesn't pull stdlib `log/slog` unless used.
- Default to a no-op logger; never `nil`-check at every log site.
- Migrate the existing `adaptiveInvariantLogger *log.Logger` to the new interface in the *same* phase — leaving two telemetry conventions doubles maintenance.

**Warning signs:**
- `import "log/slog"` appears in `query.go`, `builder.go`, or `gin.go`.
- `GINConfig` grows a `SlogHandler slog.Handler` field.
- PR adds a second logger field (keeping `adaptiveInvariantLogger`) instead of unifying.

**Phase to address:** **OBSERVABILITY.** Interface choice is a phase-0 decision.

---

### Pitfall 6: Telemetry emits predicate values that may contain PII

**What goes wrong:**
A structured log line like `{"msg":"predicate evaluated","path":"$.email","value":"alice@example.com","matched":true}` leaks user data into operators' logs. Consumers may be indexing medical records, auth tokens, internal IDs — any of which regulators treat as sensitive. The library doesn't know what's in the values; it cannot decide safety unilaterally.

**Why it happens:**
"Log everything" is the default debugging posture, and predicate values are the most useful thing to see when diagnosing "why did this prune?". But once you emit them, every consumer of your library has silently inherited a data-handling obligation.

**How to avoid:**
- Emit values only at `DEBUG` level, never at `INFO` or above. Document this contract.
- Support a value-redaction hook (`WithValueRedactor(func(path, value any) any)`) so consumers redact or hash before emission.
- By default, log predicate *shape* (path, operator, value-type) but not the value itself. A predicate value can be reconstructed in dev by enabling a `RedactValues=false` override.
- Never log the JSON document being ingested — only counts and path/type structure.

**Warning signs:**
- A log line contains a string that resembles an email, token, or UUID.
- Sample logs in README contain realistic-looking values rather than `[REDACTED]` or `<string len=42>`.
- Consumer asks "how do I turn off logging of user data?" — the answer should be "it's off by default."

**Phase to address:** **OBSERVABILITY.**

---

### Pitfall 7: Tracing spans explode on hot paths

**What goes wrong:**
Adding a `tracer.Start(ctx, "evaluatePredicate")` call inside the per-predicate loop creates one span per predicate per RG. On a 10k-RG index with a 5-predicate query, that's 50k spans per query. OpenTelemetry benchmarks show ~35% CPU increase on high-throughput paths when instrumented this way; exporters back up; cardinality limits trip on dynamic span names like `evaluate:$.user.id=12345`.

**Why it happens:**
The instinct is "instrument everything then filter later." Tracing backends charge by span volume, and high-cardinality span names (containing predicate values or path-specific suffixes) are worse than log cardinality because they fan out across the sampled population.

**How to avoid:**
- Spans only at *coarse* boundaries: `Evaluate`, `Encode`, `Decode`, `BuildFromParquet`. Attach per-predicate outcomes as span events or attributes on the parent, not as child spans.
- Never include raw values in span names. Use `op=EQ path_id=7` (path IDs, not path strings) and attach the readable path as an attribute where cardinality-limited backends can drop it.
- Provide a `WithTracing(false)` default-off config option. Tracing on a pruning library is opt-in for specific performance investigations, not background telemetry.
- Benchmark `BenchmarkEvaluateWithTracer` vs `BenchmarkEvaluateNoTracer` on the Phase 11 real corpus. Regression threshold with tracing *off*: ≤0.5% ns/op.

**Warning signs:**
- `tracer.Start` appears inside `evaluatePerRG`, `for _, pred := range preds`, or `for _, rg := range rgs`.
- Span names include formatted values (`fmt.Sprintf("eq:%v", v)`).
- A test turns tracing on and the test suite runtime doubles.

**Phase to address:** **OBSERVABILITY.**

---

### Pitfall 8: CLI experimentation subcommand scope-creeps into a REPL

**What goes wrong:**
`gin-index experiment` starts as "read JSONL, build index, print summary." Pull requests add: interactive query mode, colored output, a TUI, auto-watching files, pretty graphs, per-path histograms with Unicode sparklines. Three phases later the CLI is 2000 lines and is where bugs live.

**Why it happens:**
CLIs are fun to extend. Each individual addition feels small. The CLAUDE.md user directive "Frustrations: scope-creep → do exactly what is asked" applies directly here.

**How to avoid:**
- Write the CLI charter in one sentence *before* coding: "Accept JSONL on stdin or a path, build an index with default config, emit a structured summary suitable for copy-paste into a bug report." Anything outside that charter is a separate phase.
- Emit summary as stable JSON by default (`--format json`), human-readable as an opt-in (`--format text`). JSON-first keeps the output testable by diffing and forces contributors to think about schema rather than layout.
- No color, no TTY detection, no spinners, no interactive mode in v1.1. Each is its own future phase if justified.
- Reuse existing subcommand code paths (`build`, `info`) rather than reimplementing summary formatting. Any duplicated code is a review blocker.

**Warning signs:**
- `cmd/gin-index/main.go` grows by >300 LOC in one PR.
- A flag named `--interactive`, `--watch`, or `--repl` appears.
- A dependency on `fatih/color`, `charmbracelet/lipgloss`, or `tview` shows up in `go.mod`.
- The summary-formatting code path diverges from `info`'s output structure.

**Phase to address:** **EXPERIMENTATION-CLI.** Charter approved in phase-0 plan.

---

### Pitfall 9: CLI chokes on large or malformed JSONL

**What goes wrong:**
The CLI uses `bufio.Scanner` with default buffer (64KB). Real logs routinely contain lines larger than 64KB — stack traces, base64 payloads, deeply nested documents. Scanner returns `bufio.ErrTooLong` and the CLI exits with a confusing message. Alternatively, the CLI buffers the full file into memory and OOMs on a 10GB JSONL.

**Why it happens:**
`bufio.Scanner` is the idiomatic "read lines" tool in Go, and its 64KB default is silent. The alternative `bufio.Reader.ReadString('\n')` handles arbitrarily long lines but is slightly more verbose, so tutorials skew toward Scanner.

**How to avoid:**
- Use `bufio.Reader.ReadBytes('\n')` or `bufio.Scanner` with an explicitly-sized buffer documented in `--max-line-bytes` (default: 16MB, ceiling: 1GB).
- Stream parse-and-ingest one line at a time. Never buffer the whole JSONL.
- On malformed JSON, emit `error: line 4211: unexpected token '}' at offset 327` (line number + offset + token). Emit to stderr, continue or abort based on `--on-error=continue|abort` (default: abort).
- Normalize line endings: accept `\n`, `\r\n`, and mixed. Reject `\r`-only (classic Mac) explicitly rather than silently ignoring data.
- Support stdin via `-` as the path argument so `cat foo.jsonl | gin-index experiment -` works in pipelines.

**Warning signs:**
- `bufio.Scanner` used without `Buffer(...)` call.
- Test suite has no "large line" fixture (>64KB single line).
- Error messages don't include line numbers.
- CLI OOMs on a test file `>1GB` that the library itself handles fine via streaming.

**Phase to address:** **EXPERIMENTATION-CLI.**

---

### Pitfall 10: CLI color codes leak into piped output

**What goes wrong:**
Summary output with ANSI escapes (`\x1b[32mOK\x1b[0m`) gets piped to `jq`, `grep`, a file, or CI logs. Downstream tools either choke or display garbage. On Windows terminals without VT support, the escapes are literal characters.

**Why it happens:**
Color libraries default to "color when stdout is a TTY" — but CI environments often have `TERM=xterm` set while redirecting stdout, fooling the detection.

**How to avoid:**
- v1.1 ships with zero color. Plain text only. If color is proposed later, it's a new phase.
- If color is unavoidable, honor `NO_COLOR` env var (it is explicit, widely respected) and `--no-color` flag. Detect TTY via `golang.org/x/term.IsTerminal(int(os.Stdout.Fd()))`, *not* `$TERM`.

**Warning signs:**
- `TERM`, `$NO_COLOR`, or `isatty` references appear in the CLI phase diff.
- ANSI escape literal (`\x1b[` or `\033[`) is in test golden files.

**Phase to address:** **EXPERIMENTATION-CLI.** Pre-emptive — default is no color.

---

### Pitfall 11: Mixing SIMD / telemetry / CLI in one phase

**What goes wrong:**
A single "v1.1 feature phase" tries to ship SIMD + logging + CLI together. When the SIMD parser has a numeric-fidelity bug, the CLI tests fail for reasons that look telemetry-related. Rollback of one feature requires rollback of all three. Benchmark evidence is contaminated — is the ingest speedup from SIMD, or from the new CLI bypassing allocator paths the old CLI used?

**Why it happens:**
They feel related under one milestone banner. But each has distinct risk surfaces (correctness, performance, UX) and distinct rollback cost.

**How to avoid:**
- Three phases, in this order: **OBSERVABILITY** first (instrumentation seams lets you measure SIMD impact), **SIMD-INTEGRATION** second (benefits from observability, still no user-facing churn), **EXPERIMENTATION-CLI** third (consumes the first two).
- Each phase ships independently behind an opt-in flag. If SIMD is rolled back post-merge, observability and CLI continue to work on the legacy parser.
- Benchmarks for each phase use the Phase 11 fixture baselines. Deltas are attributed to one feature at a time.

**Warning signs:**
- A PR title contains more than one of: "simd", "logger", "tracer", "cli".
- Phase plan lists goals across more than one of the three areas.
- Rollback discussion says "we'd have to revert all of it."

**Phase to address:** **Roadmap structure decision — before the first v1.1 phase begins.**

---

### Pitfall 12: Benchmark reproducibility drift from toolchain changes

**What goes wrong:**
SIMD support requires Go 1.25.x (current) or newer for certain intrinsics; observability may want `slog.DiscardHandler` (Go 1.24+); a future dev bumps to 1.26 for a language feature. Phase 11's pinned benchmark evidence is no longer comparable because the compiler changed inlining heuristics between versions. The "deterministic benchmark replay" invariant silently breaks.

**Why it happens:**
Go minor versions change benchmark numbers by 2-5% routinely on tight loops. Without a pinned toolchain (via `go.mod`'s `toolchain` directive + CI lockstep), "run the same bench" produces different numbers across machines and time.

**How to avoid:**
- Pin `toolchain go1.25.5` in `go.mod`. Bump only in explicit "toolchain upgrade" PRs that rerun the Phase 11 benchmark family and publish before/after deltas.
- CI uses the same toolchain as `go.mod`, not "latest Go."
- Benchmark runs record `runtime.Version()` in output. Any stored benchmark evidence file that doesn't include Go version is stale.
- When Phase 11 fixtures are re-run for v1.1, record both (Go version, commit SHA) alongside the numbers.

**Warning signs:**
- `go.mod` `toolchain` line missing or uses `go1.X` loose form.
- Benchmark result file names omit Go version.
- CI job uses `actions/setup-go@v5` without pinning.
- A regression is reported but the comparison files were produced on different Go versions.

**Phase to address:** **SIMD-INTEGRATION** (since SIMD often drives the toolchain bump). Enforce in all v1.1 phases.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Import `log/slog` directly in library API | Saves writing a `Logger` interface | Churn when consumers want zap/logr; stdlib lock-in | Never in public API; OK in a sub-package with an adapter |
| Ship SIMD as a root-package dep | Consumers get speedup automatically | Breaks CGo-free / ARM64 contract; bloats `go.sum` for everyone | Never — must be opt-in import or build tag |
| Use `bufio.Scanner` with default buffer in CLI | Simpler code | Truncates real-world JSONL lines >64KB | Never for user-facing file reading |
| Skip the `Enabled()` guard on hot-path log sites | Less verbose | Allocations on disabled log calls | Never on a path evaluated >1k times per query |
| Emit predicate values at `INFO` level | Easier debugging | PII leakage into consumer logs | Never — DEBUG only |
| Reuse `ParsedJson` across goroutines | Avoid allocation | Races, silent corruption | Never without `sync.Pool` + `-race` proof |
| Compare benchmarks across Go toolchain versions | Avoid re-running fixtures | Invalid performance claims | Never — pin toolchain |
| Let the CLI grow an interactive mode "just to demo" | Nice demo | 5x maintenance surface, bug haven | Only as a separate future phase with its own charter |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| SIMD parser ↔ `NumericIndex` | Let parser write numeric stats directly | Route all numeric staging through existing `documentBuildState` + classifier in `builder.go:598-750` |
| SIMD parser ↔ transformers | Bypass transformer pipeline for speed | Transformers apply *after* parse — same order as Phase 07/09 |
| Logger ↔ existing `adaptiveInvariantLogger *log.Logger` | Ship new logger; leave legacy field | Migrate in same phase; one telemetry convention per library |
| Tracer ↔ `Evaluate()` | Start a span per predicate | One span per `Evaluate()` call; predicate outcomes as events |
| CLI ↔ existing `build`/`info` subcommands | Reimplement summary formatting | Reuse path-info emission; JSON-first output shared across subcommands |
| CLI ↔ stdin | Assume file path argument always | `-` means stdin; test both |
| SIMD build tag ↔ cross-compile | Build locally only | CI matrix: amd64/arm64 × linux/darwin/windows × CGO=0 |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Log calls allocate on disabled levels | allocs/op increases on hot bench with nop logger | `Enabled()` guard + `LogAttrs`; assert 0 allocs/op | Any query path evaluated >10k/sec |
| Tracing spans per predicate | 10-30% CPU overhead, exporter backpressure | Coarse spans only; events for details | 1k+ predicates/query on production-sized corpus |
| SIMD parser cold-start dominates for small docs | SIMD slower than `UseNumber` on <1KB docs | Threshold-based dispatch, or benchmark-guided default per shape | Docs < ~1KB (the Phase 11 smoke corpus skews here) |
| CLI buffers whole JSONL in memory | CLI OOM on files larger than RAM | Stream line-by-line; never `io.ReadAll` | Files > 1GB (common for real log corpora) |
| Benchmark results drift across Go versions | Same commit, different numbers | Pin `toolchain`; record Go version in every benchmark file | Silent; surfaces on a toolchain bump |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Log raw predicate values at INFO | PII, auth tokens, health data leak into consumer logs | DEBUG-only; value redactor hook; default-off |
| Log full JSON documents during ingest | Mass PII leakage | Never log documents; only counts and path-type shape |
| CLI prints JSONL content in error messages | Sensitive payloads end up in CI logs and tickets | Error messages reference line/offset, not content |
| SIMD parser accepts attacker-crafted deeply-nested JSON without depth limit | Stack exhaustion, parse-bomb DoS | Enforce same max-depth as existing parser; fuzz with `go-fuzz` corpus on the SIMD path |
| Regex in log/tracer attribute construction | ReDoS via user-supplied path | No regex on log-attr paths; all paths are validated via `jsonpath.go` before reaching telemetry |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| CLI exits silently on malformed JSONL line | User sees "success" with missing docs | Fail-fast with line number + offset; `--on-error=continue` opt-in |
| Summary output format changes across patch versions | Breaks shell scripts piping to `jq` | `--format json` is stable; document schema with example |
| Flag names differ between `experiment` and `build`/`info` | Users learn two conventions | Shared flag set (`--config`, `--format`, `-`); deviations need justification |
| Error message from SIMD parser unlike legacy parser | Users can't tell what changed | Error shape normalization — same `path: context: cause` pattern as `builder.go:598-607` |

## "Looks Done But Isn't" Checklist

- [ ] **SIMD integration:** has a BUILD-03 parity test on the exact Phase 07 corpus — not just a smoke test.
- [ ] **SIMD integration:** builds under `CGO_ENABLED=0` on `linux/arm64` and `darwin/arm64` in CI — not just locally.
- [ ] **SIMD integration:** falls back to explicit-number parser when the runtime CPU lacks the required feature — tested on a synthetic no-AVX2 path.
- [ ] **SIMD integration:** `go test -race ./...` passes with SIMD enabled — not only without `-race`.
- [ ] **Observability:** hot-path `BenchmarkEvaluate` shows 0 allocs/op with disabled logger — not just "fast enough."
- [ ] **Observability:** migrated the existing `adaptiveInvariantLogger *log.Logger` onto the new interface — not left as dual logger conventions.
- [ ] **Observability:** has a "no values logged at INFO" test asserting a recorded log stream contains no predicate-value substrings.
- [ ] **CLI:** handles a >64KB single JSONL line — verified with a fixture.
- [ ] **CLI:** streams a >1GB JSONL without memory pressure — verified with `runtime.MemStats` assertion.
- [ ] **CLI:** no ANSI codes in output — grep golden files for `\x1b`.
- [ ] **CLI:** stdin (`-`) works end-to-end — verified with `cat ... | gin-index experiment -`.
- [ ] **Additive API:** `v1.0` example code compiles unchanged against v1.1 — verified by running `examples/` unmodified.
- [ ] **Additive API:** SIMD is behind a build tag or sub-import — `go list -deps github.com/amikos-tech/ami-gin` does not mention `simd` without opt-in.
- [ ] **Reproducibility:** Phase 11 benchmarks rerun on v1.1 HEAD with pinned toolchain produce numbers within 5% of v1.0 baselines — or the delta is attributed to a specific feature.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| SIMD numeric fidelity regression shipped | HIGH | Revert SIMD opt-in default; add regression to `gin_test.go:2988-3233` coverage; re-release as `v1.1.1` with SIMD opt-in off by default |
| Hot-path logging allocations regressed | MEDIUM | Add `Enabled()` guards at all hot sites; add `BenchmarkEvaluateDisabledLogging` with 0-alloc assertion; patch release |
| Consumer coupled to our exposed `slog.Logger` | HIGH | Deprecate `slog.Logger` field with adapter shim; ship interface-based API in v1.2; dual-support for one version |
| CLI scope-creeped into REPL | MEDIUM | Extract REPL code into a separate subcommand or sub-binary before v1.1 ships; restore the minimal `experiment` charter |
| SIMD broke ARM64 / CGo-free builds | HIGH | Move SIMD to build-tag-gated path; add cross-compile CI job; rebuild release binaries |
| PII leaked into a consumer's production logs | HIGH | Move offending logs to DEBUG; add redactor hook; CVE-style advisory; patch release |
| Benchmark drift from toolchain bump | LOW | Pin `toolchain` in `go.mod`; re-record baselines with Go version in filename |

## Pitfall-to-Phase Mapping

Proposed v1.1 phase ordering: **OBSERVABILITY → SIMD-INTEGRATION → EXPERIMENTATION-CLI** (per Pitfall 11 rationale).

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| 1. SIMD silently demotes int64 | SIMD-INTEGRATION | `TestSIMDParserPreservesExactInt64` passes on Phase 07 corpus; parity property test across `[-2^63, 2^63)` |
| 2. SIMD CGo / ARM64 breakage | SIMD-INTEGRATION | CI matrix green on amd64/arm64 × linux/darwin/windows × CGO=0 |
| 3. SIMD not goroutine-safe | SIMD-INTEGRATION | `go test -race ./...` green; `TestConcurrentBuildersAreIndependent` passes |
| 4. Disabled-log allocations | OBSERVABILITY | `BenchmarkEvaluateDisabledLogging` asserts 0 allocs/op |
| 5. Forcing slog on consumers | OBSERVABILITY | Public API has no `slog.*` type; `go list -deps` on root shows no `log/slog` import in library package |
| 6. PII in logs | OBSERVABILITY | `TestNoValueLeakAtInfoLevel` scans captured log stream for predicate values |
| 7. Span explosion | OBSERVABILITY | `BenchmarkEvaluateWithTracer` within 0.5% of `BenchmarkEvaluateNoTracer` with tracer off |
| 8. CLI scope creep | EXPERIMENTATION-CLI | CLI charter committed in phase plan; flag list reviewed against charter at phase close |
| 9. CLI large-line / OOM | EXPERIMENTATION-CLI | Fixtures: 128KB single line, 2GB streaming; memory-bounded test |
| 10. ANSI leakage | EXPERIMENTATION-CLI | Golden files grepped for `\x1b`; no color library in `go.mod` |
| 11. Feature-mixing in one phase | Roadmap structure (pre-phase) | Each phase ships independently behind opt-in; rollback test |
| 12. Benchmark toolchain drift | SIMD-INTEGRATION | `go.mod` carries `toolchain` directive; benchmark outputs record `runtime.Version()` |

## Sources

- [minio/simdjson-go — number parsing rules and `FloatOverflowedInteger` flag](https://github.com/minio/simdjson-go) — HIGH confidence (official README + issue #30)
- [simdjson-go Issue #30 — overflowing uint64 parsed as float64](https://github.com/minio/simdjson-go/issues/30) — HIGH confidence
- [simdjson-go Issue #13 — unsafe pointer arithmetic on Go 1.14](https://github.com/minio/simdjson-go/issues/13) — MEDIUM confidence (old but documents the pattern)
- [minio/minio Issue #9003 — simdjson-go breaks cross-compilation](https://github.com/minio/minio/issues/9003) — HIGH confidence (real-world ARM64 portability report)
- [Go `log/slog` docs — `LogAttrs`, `Enabled`, `DiscardHandler`](https://pkg.go.dev/log/slog) — HIGH confidence (stdlib)
- [Go issue #62005 — `slog.DiscardHandler`](https://github.com/golang/go/issues/62005) — HIGH confidence
- [`go-logr/logr` — logging interface pattern for libraries](https://pkg.go.dev/github.com/go-logr/logr) — HIGH confidence
- [OpenTelemetry Go performance report — 35% CPU overhead under load](https://www.infoq.com/news/2025/06/opentelemetry-go-performance/) — MEDIUM confidence (single summary, widely cited)
- [OpenTelemetry Go best practices — span cardinality](https://opentelemetry.io/docs/languages/go/instrumentation/) — HIGH confidence (official)
- [Go issue #43183 — `bufio.Scanner` buffer sizing](https://github.com/golang/go/issues/43183) — HIGH confidence
- [`bufio` docs — `Scanner` vs `Reader.ReadLine`](https://pkg.go.dev/bufio) — HIGH confidence (stdlib)
- [Phase 07 verification — exact-int contract](./../milestones/v1.0-phases/07-builder-parsing-numeric-fidelity/07-VERIFICATION.md) — local project evidence
- [v1.0 retrospective — research-first vendor evaluations, benchmark-must-include-boring-cases](./../RETROSPECTIVE.md) — local project evidence

---
*Pitfalls research for: v1.1 Performance, Observability & Experimentation*
*Researched: 2026-04-21*
