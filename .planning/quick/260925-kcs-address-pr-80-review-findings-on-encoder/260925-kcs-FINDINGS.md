# PR 80 review findings to address (important + suggestions)

Source: /pr-review-toolkit:review-pr on branch claude/gallant-allen-72pzmi (PR #80, fixes #79).
All line numbers refer to that branch at commit 6cbd8b0.

## Important

I1. Library helpers cannot take a per-call EncodeOption. `WriteSidecar` (parquet.go:59), `EncodeToMetadata` (parquet.go:249), `RebuildWithIndex` (parquet.go:341) and `S3Client.WriteSidecarContext` (s3.go:284) read only idx.Config. A decoded index always carries EncoderProfileDefault, so a library user who loads an index and re-encodes it silently gets the default profile. Fix: add trailing `opts ...EncodeOption` to these four functions (and any non-Context wrapper that calls them, e.g. S3Client.WriteSidecar) and forward to EncodeContext / EncodeToMetadata. Backward compatible. Update their godoc. Add a test for at least WriteSidecar and EncodeToMetadata using the evict/observe pattern in serialize_profile_test.go (extend TestEncoderProfileConfigReachesAllEncodePaths or add a sibling).

I2. No test asserts the bounded encoder really has one worker + lowMem. Add `TestEncoderProfileBoundedUsesSingleWorker` in serialize_profile_test.go: construct via newZstdEncoder(zstd.SpeedBestCompression, EncoderProfileBoundedMemory), call EncodeAll once to force init, then via reflect read the unexported `encoders` chan field capacity (klauspost zstd/encoder.go: `e.encoders = make(chan encoder, e.o.concurrent)`) and assert == 1; also read `o.lowMem` == true. For the default profile assert cap == runtime.GOMAXPROCS(0) (skip that half if GOMAXPROCS==1). Use a clear failure message noting the reflection depends on klauspost internals. Close both encoders.

I3. Memory numbers in prose contradict the tables. Prose says "about 580 MB" and "36 MB per worker"; tables show 561 MB / 560.7 MB and the data works out to ~34 MB per additional worker; bounded is 42 MB (prose says ~40). Locations: serialize.go:209-217 (EncoderProfileDefault/BoundedMemory docs), README.md:397-398, CHANGELOG.md:7-9, docs/encoder-profile-benchmarks.md:15 and :79, cmd/gin-index/main.go -low-memory flag help (two places, "~40 MB"). Use one set: "about 34 MB per additional worker, about 560 MB at GOMAXPROCS=16, about 42 MB bounded". In the Go doc comments prefer pointing at docs/encoder-profile-benchmarks.md rather than hard numbers, or keep the numbers minimal.

I4. `experiment -low-memory` without `-o` is a silent no-op (cmd/gin-index/experiment.go:55,95,146). Print `Warning: -low-memory has no effect without -o` to stderr (match the existing warning style at main.go:222 for S3 --embed). Add a test.

## Suggestions

S1. CLI low-memory tests assert byte equality only (main_test.go:894-931, experiment_test.go:1669-1700), which passes even if the flag is ignored. Add a short comment on each saying they guard output and exit code only. Add a config-level unit test for runExperiment showing the profile lands on the config (mirror TestBuildGINConfig low-memory test at main_test.go:833). Add an end-to-end `runBuild -low-memory` test (sidecar and -embed modes) asserting exit 0 and a decodable sidecar, modelled on TestRunExtractLowMemoryWritesSameBytes.

S2. Tautological wire-format check at serialize_profile_test.go:247 compares decoded.Header.Version to the constant Version. Replace with: encode the same documents once with a bounded-profile config and once with a default-profile config, assert bytes.Equal (proves the config payload does not carry the profile).

S3. Doc comment on EncodeWithLevelContext (serialize.go:370-373) says only observability is seeded from idx.Config. Update: "Observability and the encoder profile are seeded from idx.Config when present; caller EncodeOptions (WithEncodeSignals, WithEncodeProfile) override them."

S4. README.md:395 says "one zstd encoder per compression level"; cache is per zstd mode (levels 10-19 share one) and per profile. README.md:432-434 says level 3 output is larger; only true for the high-cardinality fixture (~8%); the small fixture is smaller at level 3. Fix both. Also note in README/benchmarks doc summary table that level-3 rows are the highcard fixture and level-3 retention grows with the first payload.

S5. serialize.go:501-504 wraps encoder construction error as "create zstd encoder" without context. Use errors.Wrapf(err, "create zstd encoder (level %d, profile %s)", level, profile).

S6. serialize.go:316 passes `profile` to newZstdEncoder; use `key.profile` so the key is the single identity source.

S7. CLAUDE.md:215 pins klauspost/compress v1.18.3; go.mod has v1.19.2. Bump.

S8. serialize_concurrency_test.go:90-97 comment on TestEncodeDecodeConcurrentMixedProfiles claims it proves no goroutine received the other profile's settings, then admits it cannot. Rewrite: "output is byte-identical to the default-profile golden under contention, proving the single bounded worker is not corrupted when goroutines queue on it; run with -race to also check the cache for data races." Also restore the decode-and-re-encode check that sibling TestEncodeDecodeConcurrent has (lines 73-81) instead of only NumRowGroups.

S9. serialize.go:287-288 zstdEncoderKey comment says "Shared instances are never Closed"; tests do Close via evictSharedZstdEncoder. Change to "never Closed by production code; tests evict and Close entries through evictSharedZstdEncoder."

S10. serialize.go:209-212: note the worker pool size is read from GOMAXPROCS at the first encode for that (mode, profile) and is not resized later.

S11. serialize.go:238 validation message: "unknown encoder profile %d (want EncoderProfileDefault or EncoderProfileBoundedMemory)".

## Constraints
- No breaking API changes. Variadic opts are additive.
- Keep wire format v11. Profile stays runtime-only.
- Follow CLAUDE.md: github.com/pkg/errors, functional options, gci import order.
- Do NOT modify .planning/ files other than this task dir.
- Verify with: go build ./... && go vet ./... && make lint && go test -race -short . ./cmd/...
