---
phase: quick-261006-hgl
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - serialize.go
  - serialize_profile_test.go
  - serialize_concurrency_test.go
  - benchmark_test.go
  - docs/encoder-profile-benchmarks.md
  - README.md
  - CHANGELOG.md
  - .planning/quick/261006-hgl-validate-and-address-issue-83-bounded-me/261006-hgl-DEFERRED.md
autonomous: true
requirements: [ISSUE-83]

must_haves:
  truths:
    - "EncoderProfileBoundedMemoryUncached is accepted by WithEncoderProfile and WithEncodeProfile and its String() is bounded-memory-uncached (E8)"
    - "Encoding with it gives bytes identical to the default profile at every level, for single-block and multi-block payloads (E1)"
    - "After one encode with it, zstdEncoders has no entry for that level under any of the three profiles (E2, E5, E6)"
    - "After one level-15 encode and GC, live heap is within a coarse limit of the baseline, far under the 42 MB a cached encoder holds (E3)"
    - "The encoder built for it has one worker and low-memory buffers (E4)"
    - "Default and bounded-memory profiles still cache their encoder; their constants stay 0 and 1 (E7)"
    - "Decode of an uncached-profile index succeeds, Version is unchanged, decoded config has the default profile (E9)"
    - "CompressionNone with it returns GINu output and builds no encoder (E10)"
    - "Concurrent uncached encodes are byte-identical to default output and race-free (E11)"
    - "Docs state when not to use it, the N x 44 MB concurrent cost, and never claim Close() frees memory; benchmark doc has a new section with real numbers (E12)"
    - "No CLI flag, limiter, option func, GINConfig field, test hook in non-test files, or decoder change (E13)"
    - "A deferred-items file records the unmeasured decoder retention (E14)"
  artifacts:
    - path: "serialize.go"
      provides: "EncoderProfileBoundedMemoryUncached const, String, validate, newZstdEncoder shape, cache bypass"
      contains: "EncoderProfileBoundedMemoryUncached"
    - path: "serialize_profile_test.go"
      provides: "Tests E1-E10"
    - path: "serialize_concurrency_test.go"
      provides: "Concurrent uncached test (E11)"
    - path: "docs/encoder-profile-benchmarks.md"
      provides: "New uncached section with its own host line"
    - path: ".planning/quick/261006-hgl-validate-and-address-issue-83-bounded-me/261006-hgl-DEFERRED.md"
      provides: "Decoder retention deferred item and draft GitHub issue text"
  key_links:
    - from: "serialize.go encodeWithLevel"
      to: "sharedZstdEncoder bypass"
      via: "profile == EncoderProfileBoundedMemoryUncached returns newZstdEncoder result before taking zstdEncoderMu"
      pattern: "EncoderProfileBoundedMemoryUncached"
    - from: "serialize.go newZstdEncoder"
      to: "zstd.WithEncoderConcurrency(1), zstd.WithLowerEncoderMem(true)"
      via: "shared branch for bounded and uncached profiles"
      pattern: "WithLowerEncoderMem"
---

<objective>
Add `EncoderProfileBoundedMemoryUncached` (issue #83): a third `EncoderProfile` value. It compresses with a one-worker, low-memory zstd encoder that the library builds for the call and drops, so nothing stays in the shared encoder cache. Locked decisions D1-D10 and expectations E1-E14 are in 261006-hgl-CONTEXT.md. Do not revisit them.

Purpose: a batch job that encodes once per batch must not keep 42.5 MB live (level 15) for the process life.
Output: serialize.go change, tests, benchmark rows, doc section, godoc, README, CHANGELOG, deferred-items file.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/quick/261006-hgl-validate-and-address-issue-83-bounded-me/261006-hgl-CONTEXT.md
@.planning/quick/261006-hgl-validate-and-address-issue-83-bounded-me/261006-hgl-RESEARCH.md
@CLAUDE.md

<interfaces>
Current code in serialize.go (verified at planning time):
- type EncoderProfile uint8 with consts EncoderProfileDefault = iota (0) and EncoderProfileBoundedMemory (1), lines ~206-224.
- func (p EncoderProfile) String() string, switch with default "EncoderProfile(N)" branch, ~227.
- func (p EncoderProfile) validate() error; the error text starts "unknown encoder profile %d (want EncoderProfileDefault or EncoderProfileBoundedMemory)", ~238. An existing test requires the substring "unknown encoder profile 200" (serialize_profile_test.go:423); keep that prefix.
- var zstdEncoderMu sync.Mutex, zstdEncoders map[zstdEncoderKey]*zstd.Encoder; zstdEncoderKey{level zstd.EncoderLevel, profile EncoderProfile}.
- func sharedZstdEncoderKey(level CompressionLevel, profile EncoderProfile) zstdEncoderKey.
- func sharedZstdEncoder(level CompressionLevel, profile EncoderProfile) (*zstd.Encoder, error): locks zstdEncoderMu with defer, looks up the key, else newZstdEncoder(key.level, key.profile) and stores it.
- func newZstdEncoder(level zstd.EncoderLevel, profile EncoderProfile) (*zstd.Encoder, error): validates profile; adds WithEncoderConcurrency(1) + WithLowerEncoderMem(true) only when profile == EncoderProfileBoundedMemory.
- encodeWithLevel (~line 424-515): re-validates profile, then at ~509 calls sharedZstdEncoder(level, profile) and encoder.EncodeAll(buf.Bytes(), nil); CompressionNone returns early at ~505 before any encoder.
Test helpers in serialize_profile_test.go: encoderProfileLevels (line 20), evictSharedZstdEncoder, sharedZstdEncoderCached(level, profile), inspectZstdEncoderInternals(t, enc) returning (workers, lowMem). Existing tests: byte identity (:55), cache key (:134), config reach (TestEncoderProfileConfigReachesAllEncodePaths :178), single worker (:317), per-call override (:377), unknown rejected (:417), String (:445). serialize_concurrency_test.go:98 has a mixed-profile concurrent test. benchmark_test.go: BenchmarkEncoderProfile (~4704) loops `profiles := []EncoderProfile{EncoderProfileDefault, EncoderProfileBoundedMemory}` over sub-cases `<profile>/L<level>/<fixture>/cold` and `/repeated`; heapAllocAfterGC() at ~4792.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: New profile constant, validation, cache bypass in serialize.go (test-first for E1-E11)</name>
  <files>serialize.go, serialize_profile_test.go, serialize_concurrency_test.go</files>
  <behavior>
    Write these tests FIRST, run them, see them fail to compile or fail, then implement. Follow the style of the neighbouring tests in each file (t.Parallel where the neighbours use it, except where noted).
    - TestEncoderProfileUncachedOutputIdenticalToDefault (E1): reuse the single-block and multi-block fixtures of the existing byte-identity test (:55); for every level in encoderProfileLevels, bytes.Equal of default vs uncached output; also Decode both and compare.
    - TestEncoderProfileUncachedLeavesCacheEmpty (E2): for each level in encoderProfileLevels, evict all three profiles for that level, encode once with the uncached profile via EncodeWithLevelContext plus WithEncodeProfile, then assert sharedZstdEncoderCached is false for default, bounded-memory and uncached at that level. Non-parallel: it evicts shared entries, so it must not run beside other encoders (see the comment above evictSharedZstdEncoder).
    - TestEncoderProfileUncachedDoesNotRetainEncoder (E3): NON-parallel top-level test, no t.Parallel anywhere in it. Use heapAllocAfterGC() (defined in benchmark_test.go, same package) as baseline. Warm up first with one uncached encode, then take baseline, encode one level-15 (CompressionBest) index with the uncached profile, call heapAllocAfterGC again, assert growth is under 8 MiB (a retained encoder holds about 42 MB). Compute the delta as a signed value so HeapAlloc falling below baseline does not underflow the uint64. Put the encode in a //go:noinline helper that returns only the byte length so no stack slot in the test frame holds the encoder. Do not skip under -short; do not use RSS or HeapSys. Add a control assertion in the same test: the same flow with EncoderProfileBoundedMemory (after evicting its entry) grows the heap by more than 20 MiB, so the test proves it can see a retained encoder; then evict that entry at the end.
    - TestEncoderProfileUncachedUsesSingleWorker (E4): newZstdEncoder(zstd.SpeedBestCompression, EncoderProfileBoundedMemoryUncached), force lazy init with EncodeAll, inspectZstdEncoderInternals gives workers == 1 and lowMem == true. Must not skip when GOMAXPROCS is 1 (one-worker holds there too). Also run it once with runtime.GOMAXPROCS(4) set and restored by t.Cleanup if the existing single-worker test shows a pattern for that; otherwise skip this extra.
    - TestEncoderProfileUncachedPerCallOverride (E5): config with default profile plus WithEncodeProfile(uncached) on EncodeContext and on EncodeWithLevelContext leaves the cache empty (as E2). Config with uncached profile plus WithEncodeProfile(EncoderProfileBoundedMemory) makes sharedZstdEncoderCached(level, EncoderProfileBoundedMemory) true. Non-parallel, evict first, evict after.
    - Extend TestEncoderProfileConfigReachesAllEncodePaths (E6) or add TestEncoderProfileUncachedConfigReachesAllEncodePaths with the same four paths (plain encode, sidecar, Parquet metadata, rebuild), config built with WithEncoderProfile(uncached); after each path the cache has no entry for the level under the uncached, bounded and default profiles. Copy the structure of the existing test; do not change its current assertions.
    - E7: extend the existing cache-key test (:134) or add a small test: default and bounded-memory encodes still populate their own cache entry; assert EncoderProfileDefault == 0 and EncoderProfileBoundedMemory == 1 and EncoderProfileBoundedMemoryUncached == 2.
    - Extend the unknown-profile test (:417) and String test (:445) (E8): String of the new value is "bounded-memory-uncached"; the error text for EncoderProfile(200) still contains "unknown encoder profile 200" and now also contains all three names EncoderProfileDefault, EncoderProfileBoundedMemory, EncoderProfileBoundedMemoryUncached; WithEncoderProfile(uncached) returns no error; an encode call with the bogus value returns an error.
    - TestEncoderProfileUncachedWireFormat (E9): encode with the uncached profile, Decode, no error; decoded.Config.EncoderProfile == EncoderProfileDefault; assert the Version constant equals 11 (the value on main; confirm with grep that Version is 11 before writing the literal, and use the same literal the CHANGELOG names, "v11").
    - TestEncoderProfileUncachedCompressionNone (E10): output has prefix uncompressedMagic ("GINu"); sharedZstdEncoderCached false for all levels/profiles; and the output equals the default-profile CompressionNone output. To prove no encoder is built without a test hook in production code, assert the bytes only (no encoder is reachable at CompressionNone by the code path at serialize.go ~505, which returns before any encoder call); state this in a test comment.
    - TestEncodeUncachedConcurrentMatchesDefault (E11) in serialize_concurrency_test.go next to the mixed-profile test (:98): 8 goroutines, each encodes the same index with the uncached profile at CompressionBest and CompressionBalanced, compare to a default-profile reference computed before the goroutines start; after wg.Wait the cache has no entry for those levels under the uncached profile. Report errors with t.Errorf from goroutines (never t.Fatal). Run with -race.
  </behavior>
  <action>
    In serialize.go (per D1, D2, D3, D4, D9):
    1. Add `EncoderProfileBoundedMemoryUncached` after `EncoderProfileBoundedMemory` in the const block, so it is 2 and the old values keep 0 and 1. Give it godoc that: says the library builds one one-worker low-memory encoder for each call and drops it, so nothing stays in the shared encoder cache and level-15 retained heap after the call is near zero; says output is byte-identical to the other profiles; says do NOT use it when the process encodes often, because each call pays encoder construction (about 4 ms and 44-46 MB allocated at level 15, from the new doc section; use the exact figures measured in Task 2 if they differ) and prefer EncoderProfileBoundedMemory there; says N concurrent calls build N encoders, so peak is about N x 44 MB at level 15 and the library adds no limit; says nothing about Close() freeing memory (the encoder is only dropped; zstd's Close is a no-op for EncodeAll-only use). Also fix the type doc comment line "selects how much memory the shared zstd encoder retains" only if needed to stay true; keep edits minimal.
    2. String(): add case returning "bounded-memory-uncached".
    3. validate(): accept the third value. Error text: "unknown encoder profile %d (want EncoderProfileDefault, EncoderProfileBoundedMemory or EncoderProfileBoundedMemoryUncached)". Keep errors.Errorf from github.com/pkg/errors.
    4. newZstdEncoder: apply WithEncoderConcurrency(1) and WithLowerEncoderMem(true) when profile is EncoderProfileBoundedMemory or EncoderProfileBoundedMemoryUncached. Update its comment.
    5. sharedZstdEncoder: at the top, before taking zstdEncoderMu, if profile == EncoderProfileBoundedMemoryUncached, return newZstdEncoder(zstd.EncoderLevelFromZstd(int(level)), profile) directly. The uncached path must not hold zstdEncoderMu while it builds or uses the encoder and must not write to zstdEncoders. Update the cache comment block (~285-294) to say the uncached profile bypasses the cache. Do not add a Close() call that is described as releasing memory; do not add any test hook, new option function, new GINConfig field, or CLI flag (E13). `encodeWithLevel` keeps its single call to sharedZstdEncoder and its CompressionNone early return, so no encoder is built at level 0 (E10).
    Do not touch the decoder code, gin.go, parquet.go, s3.go or cmd/ (E13); config validation in gin.go already calls profile.validate() so WithEncoderProfile accepts the new value with no edit. Confirm this with grep before finishing.
  </action>
  <verify>
    <automated>cd /Users/tazarov/experiments/amikos/ami-gin && go build ./... && go test -race -count=1 -run 'EncoderProfile|Uncached|ConcurrentMatchesDefault|TestEncodeConcurrent' . && go test -count=1 ./... && git diff --stat main -- gin.go parquet.go s3.go cmd decoder_options.go | tail -1</automated>
  </verify>
  <done>All new tests pass, including E3 and E11 under -race; the full package suite passes; build is clean; the diff touches no file outside serialize.go and the two test files in this task; the three profile values are 0, 1, 2.</done>
</task>

<task type="auto">
  <name>Task 2: Benchmark rows for the new profile and a new section in docs/encoder-profile-benchmarks.md</name>
  <files>benchmark_test.go, docs/encoder-profile-benchmarks.md</files>
  <action>
    Per D5, D6 and the E12 benchmark clause. Do NOT edit any existing table, header or prose in the doc; append one new section at the end (old tables must show no diff; check with git diff that only added lines exist in that file).
    1. benchmark_test.go: in BenchmarkEncoderProfile add EncoderProfileBoundedMemoryUncached to the `profiles` slice. No other change to existing sub-benchmark code, except one comment on the `repeated` case: for the uncached profile evictSharedZstdEncoder finds nothing and the heap delta measured around the public call is the heap still live after the call, which is the "retained after the call" figure. Existing sub-benchmark names stay unchanged. (Why no new sub-case: `repeated` already measures live heap after the public encode returns, and `cold` already supplies Alloc/op and construction time for a one-worker encoder.)
    2. Run on this machine (darwin/arm64) with the klauspost version from go.mod (read `go.mod` for it; Research recorded v1.20.0): `go test -run '^$' -bench 'BenchmarkEncoderProfile/bounded-memory/L(15|3)/(small|highcard)/(cold|repeated)' -benchmem -benchtime=10x -count=1 .` with GOMAXPROCS=4 (set it explicitly with the GOMAXPROCS env var). Because `-bench` elements are unanchored regexes, the pattern matches both bounded-memory and bounded-memory-uncached. Save the raw output in the scratchpad directory and copy only real figures into the doc. If the machine is noisy or numbers look wrong (for example uncached retained above 5 MB), rerun; do not invent or round away anomalies, report them in the doc.
    3. Append a section to docs/encoder-profile-benchmarks.md titled for issue #83 ("Uncached bounded-memory profile (issue #83)"). It must contain: its own host line (OS/arch, CPU model from `sysctl -n machdep.cpu.brand_string`, GOMAXPROCS, Go version from `go version`, klauspost/compress version from go.mod, benchtime, date, exact command); a note that this section was measured on a different host than the tables above and that the header of the older tables names v1.19.2 while go.mod pins v1.20.0 (state it as a fact, do not edit the old header); a table with rows for the cached bounded-memory profile and the uncached profile at L15 and L3 for the small and highcard fixtures, with columns: allocated per call (MB, from `cold` B/op), retained after the call (MB, from `repeated` retained_MB), time per call (ms; for cached bounded-memory use `repeated` ns/op, for uncached use `repeated` ns/op, which includes construction on each call). Define each column in one sentence under the table, name `Alloc/op` as the source of "allocated per call", and say it is cumulative allocation, a proxy for peak and not a measured peak. Add a short "when to use" paragraph (frequent encodes: use the cached profile; one encode per batch: uncached) and the N x 44 MB concurrent-call note. Do not write that Close() frees memory.
    Keep the section factual and short.
  </action>
  <verify>
    <automated>cd /Users/tazarov/experiments/amikos/ami-gin && go vet . && go test -run '^$' -bench 'BenchmarkEncoderProfile/bounded-memory-uncached/L3/small/(cold|repeated)' -benchtime=1x -count=1 . | grep -c 'bounded-memory-uncached' && git diff --numstat main -- docs/encoder-profile-benchmarks.md && git diff main -- docs/encoder-profile-benchmarks.md | grep -c '^-[^-]'</automated>
  </verify>
  <done>Benchmark runs for the new profile and prints cold and repeated results; the doc has the new section with a real host line and table; the last verify command prints 0 (no removed lines in the doc, so old tables are untouched); every number in the table traces to the saved raw output.</done>
</task>

<task type="auto">
  <name>Task 3: README, CHANGELOG and deferred-items file</name>
  <files>README.md, CHANGELOG.md, .planning/quick/261006-hgl-validate-and-address-issue-83-bounded-me/261006-hgl-DEFERRED.md</files>
  <action>
    Per D4, D7, D9, D10, E12, E14. No new CLI flag (D8): do not touch cmd/ or the README CLI section.
    1. README.md "Encoder memory profile" section (~lines 393-436): add a short paragraph and a minimal example after the existing bounded-memory example, using gin.WithEncodeProfile(gin.EncoderProfileBoundedMemoryUncached) and the config form gin.WithEncoderProfile. Text must say: the encoder is built for the call and dropped, so level-15 retained heap after the call is near zero (use the figure from Task 2); do not use it for services that encode often (each call pays construction, about 4 ms and 44-46 MB allocated at level 15 per Task 2 numbers); N concurrent calls build N encoders (about N x 44 MB at level 15), the library sets no limit; output is byte-identical and the wire format (v11) is unchanged. Link to the new doc section. No sentence may say Close() frees memory.
    2. CHANGELOG.md: add a new top section "## Unreleased" above "## v1.3.0" (do not pick a release date or number; the maintainer cuts releases) with one bullet in the style of the v1.3.0 entry: additive `EncoderProfileBoundedMemoryUncached` (#83), what it does, when not to use it, N x 44 MB note, byte-identical output, no wire format change, no new option or flag, selected through the existing `WithEncodeProfile` and `WithEncoderProfile`. It is additive, so a minor release.
    3. Write 261006-hgl-DEFERRED.md (a deferred-items file, not a report): item "Shared zstd decoder retention is unmeasured" with these facts: `sharedZstdDecoder` (serialize.go ~346-354) is created once under sync.Once, never closed, uses WithDecoderMaxMemory/Window of 64 MiB and GOMAXPROCS concurrency (decoder_options.go:34); issue #83 is encode-only; retained size not measured; out of scope per D7. Add a "Draft GitHub issue" subsection with a proposed title and body (problem, what to measure: retained heap after one Decode at GOMAXPROCS 1/4/16, proposed acceptance) so the orchestrator can show it to the maintainer. State plainly that the executor did NOT create the issue; the orchestrator does that after the maintainer approves the draft (E14).
  </action>
  <verify>
    <automated>cd /Users/tazarov/experiments/amikos/ami-gin && grep -c 'BoundedMemoryUncached' README.md CHANGELOG.md && (git diff -U0 main -- README.md CHANGELOG.md docs serialize.go | grep -i '^+' | grep -i 'close' | grep -i -v 'no-op\|does not\|not claim' || true) && test -s .planning/quick/261006-hgl-validate-and-address-issue-83-bounded-me/261006-hgl-DEFERRED.md && make lint && make test</automated>
  </verify>
  <done>README and CHANGELOG mention the constant and the N x 44 MB cost; no added line claims Close() frees memory; the deferred file exists with a draft issue and says the issue is not yet created; make lint and make test pass.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| caller to Encode | Caller picks the profile value; an unknown uint8 crosses here |
| concurrent callers to process memory | Many goroutines can each build an encoder |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-261006-01 | D | Concurrent uncached encodes (N x 44 MB at level 15) | accept | Locked by D4: no limiter. Mitigated by documentation in godoc, README and the doc section; E11 test checks correctness only |
| T-261006-02 | T | Unknown profile value | mitigate | `validate()` rejects values other than the three constants in `encodeWithLevel`, `newZstdEncoder` and config validation; E8 test covers it |
| T-261006-03 | T | Cache poisoning or data race on `zstdEncoders` | mitigate | Uncached path returns before locking and never writes the map; E2, E11 under -race |
| T-261006-04 | I | Wire format change leaking profile into the index | mitigate | Profile stays runtime-only; E9 test asserts Decode config has default profile and Version unchanged |
| T-261006-SC | T | npm/pip/cargo installs | accept | No new dependency is added in this plan |
</threat_model>

<verification>
- `go build ./... && go test -race -count=1 .` pass; `make lint` and `make test` pass.
- E1-E11 each map to a named test in Task 1 (E1 Identical, E2 LeavesCacheEmpty, E3 DoesNotRetainEncoder, E4 UsesSingleWorker, E5 PerCallOverride, E6 ConfigReachesAllEncodePaths, E7 cache-key/const-value test, E8 unknown/String tests, E9 WireFormat, E10 CompressionNone, E11 ConcurrentMatchesDefault).
- E12: Task 2 and 3 verify commands; E13: `git diff --stat main` shows only the files in `files_modified`; no cmd/, gin.go, parquet.go, s3.go, decoder files.
- E14: DEFERRED.md exists with a draft issue; the orchestrator creates the GitHub issue after the maintainer sees the draft.
</verification>

<success_criteria>
Callers can pass `EncoderProfileBoundedMemoryUncached` through the existing option functions and get byte-identical output with no encoder left in the shared cache. All expectations E1-E14 hold. Docs carry real measured numbers from this machine.
</success_criteria>

<output>
Create `.planning/quick/261006-hgl-validate-and-address-issue-83-bounded-me/261006-hgl-SUMMARY.md` when done
</output>

<checker_addenda>
## Plan-checker addenda (binding, they override the task text above where they differ)

1. **DEFERRED.md decoder facts.** `decoder_options.go` is not a file in this repo. If the text cites decoder concurrency, check the klauspost source in the module cache for the version in `go.mod` and cite it as a klauspost file, or drop the claim. `sharedZstdDecoder` in this repo sets max memory, max window and the decode size cap only.
2. **Benchmark retained delta.** `heapAllocAfterGC()-before` is `uint64` and underflows when the heap after the call is below the baseline, which is likely for the uncached profile. Compute a signed delta and clamp at 0 for the reported metric. This one change in `benchmark_test.go` is in scope for Task 2.
3. **Task 2 verify.** `grep -c` exits 1 on zero matches. Use `test "$(git diff main -- docs/encoder-profile-benchmarks.md | grep -c '^-[^-]')" = 0`.
4. **E10.** Add a coarse allocation assertion to the `CompressionNone` test: an uncached encode at level 0 allocates far less than an encoder costs (for example, `TotalAlloc` growth under 1 MiB above a default-profile level-0 encode of the same index, or a comparable relative check).
5. **Godoc figures and doc table note.** The godoc of the new constant states figures as "about" values from the research (about 44 MB allocated per call and about 4 ms construction at level 15). After Task 2, check that the measured numbers do not contradict them and correct the godoc if they do. Under the new doc table, state that "allocated per call" for the cached bounded-memory row is the first call (construction plus encode), not the steady state.
6. **E3 readings.** Take the baseline and the final reading after two GC cycles each (call `heapAllocAfterGC()` twice and use the second value).
7. Remove the `decoder_options.go` path from the Task 1 verify command.
</checker_addenda>
