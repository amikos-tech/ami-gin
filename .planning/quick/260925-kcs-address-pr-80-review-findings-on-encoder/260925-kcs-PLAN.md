---
phase: quick
plan: 260925-kcs
type: execute
wave: 1
depends_on: []
files_modified: [serialize.go, parquet.go, s3.go, serialize_profile_test.go, serialize_concurrency_test.go, cmd/gin-index/experiment.go, cmd/gin-index/main.go, cmd/gin-index/main_test.go, cmd/gin-index/experiment_test.go, README.md, CHANGELOG.md, docs/encoder-profile-benchmarks.md, CLAUDE.md]
autonomous: true
requirements: [PR-80-REVIEW]
must_haves:
  truths:
    - "Library helpers (WriteSidecar, EncodeToMetadata, RebuildWithIndex, S3Client.WriteSidecar/WriteSidecarContext) accept a per-call EncodeOption, so a caller can force a profile even when idx.Config carries the default (post-decode) profile"
    - "A test proves the bounded profile actually configures the underlying zstd encoder with one worker and lowMem=true (not just byte-identical output)"
    - "Memory numbers in prose (serialize.go doc comments, README.md, CHANGELOG.md, docs/encoder-profile-benchmarks.md, CLI flag help) agree with the measured benchmark tables: ~34 MB per additional default-profile worker, ~560 MB at GOMAXPROCS=16, ~42 MB bounded"
    - "gin-index experiment --low-memory without -o prints a stderr warning instead of silently doing nothing"
    - "CLI low-memory byte-equality tests are documented as guarding output bytes and exit code only, and are backed by a config-level assertion that the profile actually reaches GINConfig for both build and experiment, plus an end-to-end runBuild -low-memory test covering sidecar and -embed modes"
    - "The mixed-profile concurrency test and the config-reaches-all-paths test assert real profile-independence (cross-profile byte equality, decode-then-re-encode) instead of tautological checks"
    - "go build ./..., go vet ./..., make lint, and go test -race -short . ./cmd/... all pass"
  artifacts:
    - path: "serialize.go"
      provides: "Corrected EncoderProfile doc comments/numbers, GOMAXPROCS-fixed-at-first-encode note, key.profile fix, wrapped encoder-construction error, EncodeWithLevelContext doc fix, never-Closed comment fix, unknown-profile error text"
    - path: "parquet.go"
      provides: "WriteSidecar/EncodeToMetadata/RebuildWithIndex accept trailing opts ...EncodeOption forwarded to the encode call"
    - path: "s3.go"
      provides: "S3Client.WriteSidecar/WriteSidecarContext accept trailing opts ...EncodeOption forwarded to EncodeContext"
    - path: "serialize_profile_test.go"
      provides: "TestEncoderProfileBoundedUsesSingleWorker (reflect-based worker/lowMem assertion), per-call-option coverage for WriteSidecar/EncodeToMetadata, and a real cross-profile byte-equality check replacing the tautological version comparison"
    - path: "serialize_concurrency_test.go"
      provides: "Accurate comment on TestEncodeDecodeConcurrentMixedProfiles and a decode+re-encode correctness check instead of only NumRowGroups"
    - path: "cmd/gin-index/experiment.go"
      provides: "Stderr warning when -low-memory has no effect without -o, and an extracted config-building helper"
    - path: "cmd/gin-index/main_test.go"
      provides: "Guard-scope comment on TestRunExtractLowMemoryWritesSameBytes and a new end-to-end runBuild -low-memory test"
    - path: "cmd/gin-index/experiment_test.go"
      provides: "Guard-scope comment, low-memory warning test, and config-level profile test for runExperiment"
  key_links:
    - from: "parquet.go WriteSidecar/EncodeToMetadata/RebuildWithIndex"
      to: "EncodeContext / EncodeToMetadata"
      via: "opts ...EncodeOption forwarding"
      pattern: "EncodeContext\\(context\\.Background\\(\\), idx, opts\\.\\.\\.\\)|EncodeToMetadata\\(idx, cfg, opts\\.\\.\\.\\)"
    - from: "s3.go S3Client.WriteSidecarContext"
      to: "EncodeContext"
      via: "opts ...EncodeOption forwarding"
      pattern: "EncodeContext\\(ctx, idx, opts\\.\\.\\.\\)"
    - from: "cmd/gin-index/experiment.go runExperiment"
      to: "stderr"
      via: "low-memory-without-output warning"
      pattern: "-low-memory has no effect without -o"
---

<objective>
Address all 15 PR #80 code-review findings (I1-I4, S1-S11) on the bounded-memory
zstd encoder profile feature (issue #79) in one pass: close the real gap where
library helpers can't take a per-call encode option, add the missing worker/lowMem
assertion, fix every stale memory number, warn on the experiment CLI's silent
-low-memory no-op, strengthen weak/tautological tests, and correct several doc
comments and one dependency version pin.

Purpose: Land a clean, fully-reviewed encoder-profile feature before merging PR #80.
Output: Updated serialize.go/parquet.go/s3.go library surface with tests, an
experiment CLI warning with tests, and corrected docs/CHANGELOG/CLAUDE.md numbers.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@CLAUDE.md
@.planning/quick/260925-kcs-address-pr-80-review-findings-on-encoder/260925-kcs-FINDINGS.md
@serialize.go
@parquet.go
@s3.go
@serialize_profile_test.go
@serialize_concurrency_test.go
@cmd/gin-index/main.go
@cmd/gin-index/experiment.go

<interfaces>
From serialize.go (current EncoderProfile doc block, ~line 208-219 -- full rewrite target):

    const (
    	// EncoderProfileDefault keeps one zstd worker per GOMAXPROCS so concurrent
    	// encodes at the same level run in parallel. Each level-15 worker retains
    	// roughly 36 MB for the life of the process, so a 16-CPU host holds about
    	// 580 MB after the first level-15 encode.
    	EncoderProfileDefault EncoderProfile = iota
    	// EncoderProfileBoundedMemory keeps a single zstd worker with zstd's
    	// lower-memory buffers. Level 15 retains about 40 MB regardless of
    	// GOMAXPROCS. Concurrent encodes at the same level and profile queue on
    	// that one worker instead of running in parallel.
    	EncoderProfileBoundedMemory
    )

Measured numbers (docs/encoder-profile-benchmarks.md, klauspost/compress v1.19.2):
retained MB at level 15 by GOMAXPROCS: 1->50.6, 4->152.6, 16->560.7 (default);
42.5 at any GOMAXPROCS (bounded). That is ~34 MB per additional default worker.

From serialize.go (validate/cache/encode, exact current text to edit):
- line ~238: `return errors.Errorf("unknown encoder profile %d", uint8(p))`
- line ~287-288 (end of the zstdEncoderKey doc comment): "...The two profiles
  never share an instance. Shared instances are never Closed."
- line ~316: `enc, err := newZstdEncoder(key.level, profile)` (inside
  `sharedZstdEncoder`, which already computed `key := sharedZstdEncoderKey(level, profile)`)
- lines ~370-373 (EncodeWithLevelContext doc): "Observability is seeded from
  idx.Config when present; caller EncodeOptions override it."
- lines ~501-504:

        encoder, err := sharedZstdEncoder(level, profile)
        if err != nil {
        	return nil, errors.Wrap(err, "create zstd encoder")
        }

From klauspost/compress@v1.19.2 zstd/encoder.go (unexported fields to reach via
reflect+unsafe for I2 -- do not import the zstd internal package, use reflect
on the *zstd.Encoder value returned by newZstdEncoder):

    type Encoder struct {
    	o        encoderOptions // has unexported field `lowMem bool`
    	encoders chan encoder   // capacity == configured worker count
    	...
    }

`reflect.Value.Cap()` and `.Bool()` work on unexported fields once you rebuild
an addressable, non-read-only Value via `reflect.NewAt(field.Type(),
unsafe.Pointer(field.UnsafeAddr())).Elem()`. The Encoder must come from
`enc, err := newZstdEncoder(...)` (the uncached path these tests already use,
see `sharedZstdEncoderCached`/`evictSharedZstdEncoder` in serialize_profile_test.go)
so it is not shared with the production cache; call `EncodeAll` once first to
force lazy init, then reflect, then `enc.Close()`.

From cmd/gin-index/main.go (buildGINConfig -- pattern to mirror for experiment.go):

    func buildGINConfig(maxStagedPaths int, lowMemory bool) (gin.GINConfig, error) {
    	return gin.NewConfig(
    		gin.WithMaxStagedPaths(maxStagedPaths),
    		gin.WithEncoderProfile(encoderProfileFor(lowMemory)),
    	)
    }

main_test.go already has `TestBuildGINConfigLowMemorySelectsBoundedProfile` at
line ~833 asserting `cfg.EncoderProfile`. experiment.go currently builds its
config inline in `runExperiment` (lines ~86-98): `experimentConfigForLogLevel`
then `gin.WithMaxStagedPaths(*maxStagedPaths)(&config)` then
`gin.WithEncoderProfile(encoderProfileFor(*lowMemory))(&config)` -- there is no
extracted, independently testable function for this today.

From main.go's existing S3 --embed warning style (buildSingleFileWithIO, ~line
221): `fmt.Fprintf(stderr, "  Warning: --embed not supported for S3 paths, using sidecar\n")`.
experiment.go's own warnings use no leading spaces, e.g. (line ~110):
`fmt.Fprintf(stderr, "Warning: %v\n", cleanupErr)` -- match this experiment.go
style, not main.go's indented one.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Library API -- per-call EncodeOption on parquet/S3 helpers, doc/message fixes, and stronger encoder-profile tests</name>
  <files>serialize.go, parquet.go, s3.go, serialize_profile_test.go, serialize_concurrency_test.go</files>
  <behavior>
    - TestEncoderProfileBoundedUsesSingleWorker (new, I2): construct an encoder via
      newZstdEncoder(zstd.SpeedBestCompression, EncoderProfileBoundedMemory), call
      EncodeAll once on it to force lazy init, then via reflect+unsafe read the
      unexported `encoders` chan field's capacity and assert it equals 1, and read
      `o.lowMem` and assert it is true. Repeat for EncoderProfileDefault and assert
      the `encoders` channel capacity equals runtime.GOMAXPROCS(0), skipping that
      half of the assertion when runtime.GOMAXPROCS(0) == 1 (indistinguishable from
      bounded). Close both encoders when done. Failure messages must note the
      assertion depends on klauspost/compress internals (field names `encoders`,
      `o.lowMem`) so a future dependency bump that breaks this test is easy to
      diagnose.
    - New sibling test (I1, e.g. TestEncoderProfileLibraryHelpersAcceptPerCallOption):
      using the same evict/observe pattern as TestEncoderProfileConfigReachesAllEncodePaths
      (evictSharedZstdEncoder / sharedZstdEncoderCached), build an index with
      DefaultConfig() (so idx.Config.EncoderProfile is EncoderProfileDefault), then
      call WriteSidecar(path, idx, WithEncodeProfile(EncoderProfileBoundedMemory)) and
      assert sharedZstdEncoderCached(CompressionBest, EncoderProfileBoundedMemory) is
      true afterward (proving the per-call option overrode the default-profile
      config); do the same for
      EncodeToMetadata(idx, DefaultParquetConfig(), WithEncodeProfile(EncoderProfileBoundedMemory)).
    - TestEncoderProfileConfigReachesAllEncodePaths (S2): replace the tautological
      `decoded.Header.Version != Version` check (current lines ~247-249) with: encode
      the same idx document set once via Encode on an index built with
      DefaultConfig() and once via Encode on the existing bounded-profile idx, both
      at the same (default) compression level, and assert bytes.Equal on the two
      outputs -- proving the config's serialized bytes do not carry the profile.
    - TestEncodeDecodeConcurrentMixedProfiles (S8, in serialize_concurrency_test.go):
      rewrite the doc comment (currently claims the test "proves no goroutine
      received the other profile's settings" then admits it cannot) to instead say
      the output is byte-identical to the default-profile golden under contention,
      which proves the single bounded worker is not corrupted when goroutines queue
      on it, and note -race also checks the cache for data races. Replace the
      current `decoded.Header.NumRowGroups != idx.Header.NumRowGroups` check with
      the same decode-then-re-encode-then-bytes.Equal(reEncoded, want) check that
      the sibling TestEncodeDecodeConcurrent already does (lines ~73-81 of that
      file).
  </behavior>
  <action>
In serialize.go: (1) Rewrite the EncoderProfileDefault and EncoderProfileBoundedMemory
doc comments (the const block around line 208-219) to state the corrected numbers --
about 34 MB per additional default-profile worker, about 560 MB at GOMAXPROCS=16,
about 42 MB bounded regardless of GOMAXPROCS -- and add one sentence noting the
worker pool size is read from GOMAXPROCS once, when the shared encoder for a given
(mode, profile) key is first constructed, and is not resized later even if
GOMAXPROCS changes (this closes both I3's serialize.go location and S10 -- do not
duplicate this edit elsewhere; Task 3 only touches the non-serialize.go I3
locations). (2) In (EncoderProfile).validate(), change the error message to
"unknown encoder profile %d (want EncoderProfileDefault or EncoderProfileBoundedMemory)"
(S11). (3) At the end of the zstdEncoderKey doc comment (ending "...Shared instances
are never Closed."), change the last sentence to "Shared instances are never Closed
by production code; tests evict and Close entries through evictSharedZstdEncoder."
(S9). (4) In sharedZstdEncoder, change `newZstdEncoder(key.level, profile)` to
`newZstdEncoder(key.level, key.profile)` so the key is the single identity source
(S6). (5) In the EncodeWithLevelContext doc comment, change "Observability is
seeded from idx.Config when present; caller EncodeOptions override it." to
"Observability and the encoder profile are seeded from idx.Config when present;
caller EncodeOptions (WithEncodeSignals, WithEncodeProfile) override them." (S3).
(6) In encodeWithLevel, change `errors.Wrap(err, "create zstd encoder")` to
`errors.Wrapf(err, "create zstd encoder (level %d, profile %s)", level, profile)`
(S5).

In parquet.go (I1): add a doc comment and a trailing `opts ...EncodeOption`
parameter to WriteSidecar, EncodeToMetadata, and RebuildWithIndex. WriteSidecar
must call EncodeContext(context.Background(), idx, opts...) instead of
Encode(idx). EncodeToMetadata must call EncodeContext(context.Background(), idx,
opts...) instead of Encode(idx). RebuildWithIndex must forward its own opts into
its existing EncodeToMetadata(idx, cfg) call as EncodeToMetadata(idx, cfg, opts...).
Each doc comment must state that opts (e.g. WithEncodeProfile) override the
profile carried by idx.Config, since a decoded index always carries
EncoderProfileDefault.

In s3.go (I1): add a doc comment and a trailing `opts ...EncodeOption` parameter
to S3Client.WriteSidecar and S3Client.WriteSidecarContext. WriteSidecar forwards
opts... into its call to WriteSidecarContext. WriteSidecarContext forwards opts...
into its existing EncodeContext(ctx, idx) call, i.e. EncodeContext(ctx, idx,
opts...). Both additions are backward compatible (variadic); do not change any
call sites in cmd/gin-index/main.go or examples/parquet/main.go -- confirm with
`go build ./...` that they still compile unchanged.

Then implement the four test changes described in <behavior> in
serialize_profile_test.go (new TestEncoderProfileBoundedUsesSingleWorker; new
TestEncoderProfileLibraryHelpersAcceptPerCallOption; the S2 rewrite inside
TestEncoderProfileConfigReachesAllEncodePaths) and serialize_concurrency_test.go
(the S8 comment + assertion rewrite inside TestEncodeDecodeConcurrentMixedProfiles).
Add "reflect", "runtime", and "unsafe" imports to serialize_profile_test.go as
needed for the reflect-based worker/lowMem assertions; keep import grouping per
CLAUDE.md (stdlib, then third-party, then this module, gci-ordered).
  </action>
  <verify>
    <automated>cd /Users/tazarov/experiments/amikos/ami-gin && go build ./... && go vet ./... && go test -race -run 'TestEncoderProfile|TestEncodeDecodeConcurrent' -v .</automated>
  </verify>
  <done>parquet.go/s3.go helpers accept and forward opts ...EncodeOption; serialize.go doc comments/messages match S3/S5/S6/S9/S10/S11/I3(partial); TestEncoderProfileBoundedUsesSingleWorker, TestEncoderProfileLibraryHelpersAcceptPerCallOption pass; TestEncoderProfileConfigReachesAllEncodePaths no longer contains a Header.Version check; TestEncodeDecodeConcurrentMixedProfiles's comment and assertion match TestEncodeDecodeConcurrent's decode/re-encode pattern; all listed commands exit 0.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: CLI -- warn on experiment --low-memory without -o, and strengthen low-memory CLI test coverage</name>
  <files>cmd/gin-index/experiment.go, cmd/gin-index/main_test.go, cmd/gin-index/experiment_test.go</files>
  <behavior>
    - New test TestRunExperimentLowMemoryWarnsWithoutOutput (I4): run runExperiment
      with `--low-memory` and no `-o` against a small JSONL fixture (see
      writeJSONLFixture usage in experiment_test.go); assert exit code 0 and that
      stderr contains "Warning: -low-memory has no effect without -o". Also assert
      that passing both `--low-memory` and `-o` produces no such warning (reuse the
      existing TestRunExperimentLowMemoryWritesSameSidecarBytes fixture pattern for
      this negative case, or add it as a second subtest/case).
    - New test TestExperimentGINConfigLowMemorySelectsBoundedProfile (S1, mirrors
      TestBuildGINConfigLowMemorySelectsBoundedProfile in main_test.go): calls the
      new extracted config-building helper directly with lowMemory=true and
      lowMemory=false and asserts config.EncoderProfile in each case, without
      spinning up the full CLI I/O path.
    - New test(s) TestRunBuildLowMemoryProducesDecodableIndex (S1, modeled on
      TestRunExtractLowMemoryWritesSameBytes): run `runBuild` with `-low-memory`
      in sidecar mode (`-o out.gin`) against a fixture built with
      createCLIParquetFile, assert exit code 0, then gin.Decode the written file
      and assert no error; run `runBuild` with `-low-memory -embed` against a
      second copy of the fixture, assert exit code 0, then
      gin.ReadFromParquetMetadata the file and assert no error.
    - Amend the doc comments on TestRunExtractLowMemoryWritesSameBytes (main_test.go)
      and TestRunExperimentLowMemoryWritesSameSidecarBytes (experiment_test.go) to
      add one sentence each stating they guard output bytes and exit code only, and
      do not verify that -low-memory actually engaged the bounded encoder (that is
      covered by TestEncoderProfileForLowMemoryFlag /
      TestBuildGINConfigLowMemorySelectsBoundedProfile /
      TestExperimentGINConfigLowMemorySelectsBoundedProfile).
  </behavior>
  <action>
In cmd/gin-index/experiment.go: extract the config-construction block currently
inline in runExperiment (experimentConfigForLogLevel, then
gin.WithMaxStagedPaths(*maxStagedPaths)(&config), then
gin.WithEncoderProfile(encoderProfileFor(*lowMemory))(&config), around lines
86-98) into a standalone function, e.g.
`experimentGINConfig(logLevel string, maxStagedPaths int, lowMemory bool, stderr io.Writer) (gin.GINConfig, error)`,
and call it from runExperiment in place of the inline block. Immediately after
computing `config` in runExperiment (I4), add: if `*lowMemory` is true and
`*outputPath` is empty, print `fmt.Fprintln(stderr, "Warning: -low-memory has no
effect without -o")` -- match experiment.go's existing unindented warning style
(see the cleanup-error warning near line 110), not main.go's indented
"  Warning: ..." style. This is a warning only; do not change the function's
return/exit behavior.

Then add the tests described in <behavior> to cmd/gin-index/experiment_test.go
(TestRunExperimentLowMemoryWarnsWithoutOutput,
TestExperimentGINConfigLowMemorySelectsBoundedProfile, guard-scope comment
addition) and cmd/gin-index/main_test.go (TestRunBuildLowMemoryProducesDecodableIndex,
guard-scope comment addition). Reuse existing helpers already in these test files
(createCLIParquetFile, writeJSONLFixture) rather than writing new fixture
plumbing.
  </action>
  <verify>
    <automated>cd /Users/tazarov/experiments/amikos/ami-gin && go build ./... && go vet ./... && go test -run 'TestRunExperiment|TestRunBuild|TestRunExtract|TestExperimentGINConfig|TestBuildGINConfig|TestEncoderProfileForLowMemoryFlag' -v ./cmd/...</automated>
  </verify>
  <done>runExperiment warns on stderr exactly when -low-memory is set and -o is not; experimentGINConfig is an independently callable, independently tested function; TestRunBuildLowMemoryProducesDecodableIndex covers both sidecar and -embed modes with exit 0 and successful decode; the two existing byte-equality tests carry an added guard-scope comment; all listed commands exit 0.</done>
</task>

<task type="auto">
  <name>Task 3: Docs and numbers -- README/CHANGELOG/benchmarks-doc/CLI-help memory figures, README wording fixes, and CLAUDE.md dependency pin</name>
  <files>README.md, CHANGELOG.md, docs/encoder-profile-benchmarks.md, cmd/gin-index/main.go, CLAUDE.md</files>
  <action>
Note: serialize.go's portion of I3 (the EncoderProfileDefault/BoundedMemory doc
comments) was already corrected in Task 1 -- do not re-edit serialize.go here;
this task only touches the remaining I3 locations plus S4 and S7.

I3 (remaining locations): in README.md (~lines 397-398, prose above the "Encoder
memory profile" retained-heap table), change "Each level-15 worker holds about
36 MB of match tables, so a 16-CPU host retains roughly 580 MB" to use "about 34
MB" and "about 560 MB" so the prose matches the table beneath it (which already
shows 51/153/561 MB). In CHANGELOG.md (~lines 7-9, the "Unreleased" #79 bullet),
change "retains about 36 MB, so a 16-CPU host held roughly 580 MB" to "about 34
MB" / "about 560 MB", and change "(about 40 MB at level 15 regardless of
GOMAXPROCS)" to "(about 42 MB at level 15 regardless of GOMAXPROCS)". In
docs/encoder-profile-benchmarks.md line ~15 ("Default retention scales linearly
with it (about 36 MB per worker at level 15)"), change "36 MB" to "34 MB"; at
line ~79 ("Level 15 costs about 36 MB per worker whatever the profile..."),
change "36 MB" to "34 MB" (the "~42 MB total" on that same line is already
correct, leave it). In cmd/gin-index/main.go, change both occurrences of the
`-low-memory` flag help text containing "(~40 MB at level 15)" (lines ~113 and
~551) to "(~42 MB at level 15)".

S4: in README.md, the sentence "`Encode` reuses one zstd encoder per compression
level for the life of the process." (~line 395) is imprecise -- the cache key is
the collapsed zstd mode (levels 10-19 share SpeedBestCompression) plus the
profile, not the raw numeric level. Reword to state the encoder is reused per
zstd compression mode and encoder profile, noting adjacent levels within a mode
(e.g. 10-19) share one cached encoder. Also in README.md (~lines 432-434), "Level
3 retains much less under either profile (10 to 37 MB in the same runs) at a
larger output size, so lowering the level is a different trade-off" incorrectly
implies level 3 is always larger; per docs/encoder-profile-benchmarks.md's Full
table, level 3 is only larger than level 15 on the high-cardinality fixture
(about 8%) -- the small fixture is smaller at level 3 (7,142/7,138 B vs 7,157/7,153
B). Reword so the "larger output" claim is scoped to the high-cardinality
fixture only, making clear the trade-off is workload-dependent. Finally, in
docs/encoder-profile-benchmarks.md, add one sentence (as a note/footnote directly
under the "## Retained memory after first encode (MB)" table) stating that the
table's two "L3" columns are sourced from the highcard fixture (matching the
highcard rows in the Full table below, e.g. the level-3 default retained values
17.8/21.6/36.7 MB), and that unlike level 15 (which always allocates its full
window), level-3 retained memory is sensitive to the size of the first payload
encoded at that (mode, profile) key.

S7: in CLAUDE.md, change the pinned klauspost/compress version on the
`github.com/klauspost/compress` dependency line (~line 215, "supports
configurable compression levels 0-19") from v1.18.3 to v1.19.2 to match go.mod.
  </action>
  <verify>
    <automated>cd /Users/tazarov/experiments/amikos/ami-gin && grep -n "34 MB\|560 MB\|42 MB" README.md CHANGELOG.md docs/encoder-profile-benchmarks.md && grep -n "v1.19.2" CLAUDE.md && grep -n "~42 MB at level 15" cmd/gin-index/main.go && ! grep -n "36 MB\|580 MB\|~40 MB\|v1.18.3" README.md CHANGELOG.md docs/encoder-profile-benchmarks.md cmd/gin-index/main.go CLAUDE.md</automated>
  </verify>
  <done>No file contains a stale "36 MB"/"580 MB"/"~40 MB"/klauspost v1.18.3 reference; README's encoder-cache-key sentence and level-3-output-size sentence are workload/mode accurate; docs/encoder-profile-benchmarks.md's retained-memory table has a footnote naming the highcard fixture and the first-payload sensitivity of level 3.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

None crossed. This plan changes: (a) library function signatures to accept an
additional variadic, backward-compatible option (no new input parsing or trust
boundary), (b) test assertions and comments, (c) a CLI stderr warning derived
from already-parsed local flags (no new external input), and (d) documentation
prose and a dependency version comment. No new external input, deserialization
path, or credential handling is introduced.

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|------------------|
| T-quick-01 | N/A | serialize.go / parquet.go / s3.go | accept | Additive variadic API change only; no new deserialization or trust boundary; existing Decode/UnmarshalBinary validation paths (maxDecodedIndexSize, maxHeaderRowGroups, etc.) are unchanged |
| T-quick-02 | Tampering | N/A | accept | No new npm/pip/cargo/go module installs in this plan (klauspost/compress version comment corrected to match the already-vetted, already-in-go.sum v1.19.2; no new dependency added) |
</threat_model>

<verification>
Run the full prescribed gate after all three tasks:

    cd /Users/tazarov/experiments/amikos/ami-gin
    go build ./...
    go vet ./...
    make lint
    go test -race -short . ./cmd/...

All four must pass with no failures. Confirm only the files listed in
`files_modified` above were changed (`git status`), and that examples/parquet/main.go
and cmd/gin-index/main.go's existing WriteSidecar/RebuildWithIndex call sites still
compile unchanged (no opts required at call sites, per I1's backward-compatibility
requirement).
</verification>

<success_criteria>
- All 4 Important findings (I1-I4) and all 11 Suggestions (S1-S11) from FINDINGS.md
  are implemented exactly as prescribed, with no scope reduction.
- WriteSidecar, EncodeToMetadata, RebuildWithIndex, S3Client.WriteSidecar, and
  S3Client.WriteSidecarContext all accept a trailing `opts ...EncodeOption` and
  forward it into the underlying EncodeContext/EncodeToMetadata call.
- TestEncoderProfileBoundedUsesSingleWorker asserts worker-count and lowMem
  directly on the constructed zstd.Encoder via reflection.
- `gin-index experiment --low-memory` without `-o` prints a warning to stderr.
- Every "36 MB" / "580 MB" / "~40 MB" / klauspost v1.18.3 reference is corrected
  to "34 MB" / "560 MB" / "~42 MB" / v1.19.2 respectively, across serialize.go,
  README.md, CHANGELOG.md, docs/encoder-profile-benchmarks.md,
  cmd/gin-index/main.go, and CLAUDE.md.
- `go build ./...`, `go vet ./...`, `make lint`, and `go test -race -short . ./cmd/...`
  all pass.
- Changes committed on the current branch `claude/gallant-allen-72pzmi` with a
  conventional commit message, no AI attribution footer unless the repository's
  own convention requires it.
</success_criteria>

<output>
Create `.planning/quick/260925-kcs-address-pr-80-review-findings-on-encoder/260925-kcs-SUMMARY.md` when done
</output>
