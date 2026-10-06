package gin

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"github.com/klauspost/compress/zstd"
)

// encoderProfileLevels covers one level per collapsed zstd mode plus 19,
// which shares SpeedBestCompression with 15.
var encoderProfileLevels = []CompressionLevel{
	CompressionFastest, CompressionBalanced, CompressionBetter, CompressionBest, CompressionMax,
}

// evictSharedZstdEncoder removes one cached encoder so a test can observe
// whether a code path constructs it, or a benchmark can measure what that
// construction retains. Callers must not run in parallel with other encodes
// at that key: Go runs top-level sequential tests before any t.Parallel test
// resumes, and benchmarks run after all tests, so both are safe.
func evictSharedZstdEncoder(level CompressionLevel, profile EncoderProfile) {
	key := sharedZstdEncoderKey(level, profile)
	zstdEncoderMu.Lock()
	defer zstdEncoderMu.Unlock()
	if enc, ok := zstdEncoders[key]; ok {
		delete(zstdEncoders, key)
		enc.Close()
	}
}

// sharedZstdEncoderCached reports whether the shared encoder for
// (level, profile) is currently in the cache. It mirrors
// evictSharedZstdEncoder's signature.
func sharedZstdEncoderCached(level CompressionLevel, profile EncoderProfile) bool {
	key := sharedZstdEncoderKey(level, profile)
	zstdEncoderMu.Lock()
	defer zstdEncoderMu.Unlock()
	_, ok := zstdEncoders[key]
	return ok
}

// E1: bounded and default encodings are byte-identical at every level and
// both decode to the original index. The small fixture fits in one zstd block
// (EncodeAll's no-history path); the multi-block fixture exceeds the 128 KB
// block size so the history-buffer path, where WithLowerEncoderMem changes
// buffer sizing, is exercised too.
func TestEncoderProfileBoundedOutputIdenticalToDefault(t *testing.T) {
	t.Parallel()
	const zstdBlockSize = 128 << 10
	fixtures := []struct {
		name       string
		idx        *GINIndex
		multiBlock bool
	}{
		{name: "single-block", idx: buildAdaptiveSerializationFixture(t, DefaultConfig())},
		{name: "multi-block", idx: buildHighCardinalityIndex(t, 200, 20), multiBlock: true},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			raw, err := EncodeWithLevel(fixture.idx, CompressionNone)
			if err != nil {
				t.Fatal(err)
			}
			if payload := len(raw) - len(uncompressedMagic); (payload > zstdBlockSize) != fixture.multiBlock {
				t.Fatalf("payload is %d bytes; multi-block = %v, want %v", payload, payload > zstdBlockSize, fixture.multiBlock)
			}
			assertEncoderProfilesByteIdentical(t, fixture.idx)
		})
	}
}

// buildHighCardinalityIndex builds an index where every document
// carries unique string values, so the string, trigram and HLL sections are
// large and the zstd payload is representative of a high-cardinality index.
func buildHighCardinalityIndex(tb testing.TB, numRGs, docsPerRG int) *GINIndex {
	tb.Helper()
	builder, err := NewBuilder(DefaultConfig(), numRGs)
	if err != nil {
		tb.Fatal(err)
	}
	n := 0
	for rg := 0; rg < numRGs; rg++ {
		for d := 0; d < docsPerRG; d++ {
			doc := fmt.Sprintf(`{"id":%d,"trace_id":"trace-%08x-%08x","user":"user_%d@example.com","status":%q,"latency_ms":%d}`,
				n, n*2654435761, n*40503, n, []string{"ok", "error", "timeout"}[n%3], (n*37)%5000)
			if err := builder.AddDocument(DocID(rg), []byte(doc)); err != nil {
				tb.Fatal(err)
			}
			n++
		}
	}
	return builder.Finalize()
}

func assertEncoderProfilesByteIdentical(t *testing.T, idx *GINIndex) {
	t.Helper()
	ctx := context.Background()
	for _, level := range encoderProfileLevels {
		want, err := EncodeWithLevelContext(ctx, idx, level, WithEncodeProfile(EncoderProfileDefault))
		if err != nil {
			t.Fatalf("default level %d: %v", level, err)
		}
		got, err := EncodeWithLevelContext(ctx, idx, level, WithEncodeProfile(EncoderProfileBoundedMemory))
		if err != nil {
			t.Fatalf("bounded level %d: %v", level, err)
		}
		if !bytes.Equal(want, got) {
			t.Fatalf("level %d: bounded output differs from default output", level)
		}
		decoded, err := Decode(got)
		if err != nil {
			t.Fatalf("decode bounded level %d: %v", level, err)
		}
		reEncoded, err := EncodeWithLevel(decoded, level)
		if err != nil {
			t.Fatalf("re-encode level %d: %v", level, err)
		}
		if !bytes.Equal(reEncoded, want) {
			t.Fatalf("level %d: decoded index re-encodes differently", level)
		}
	}
}

// E3: each (mode, profile) key caches exactly one encoder; the two profiles
// never share an instance; mode collisions (15 and 19) share within a profile.
func TestEncoderProfileCacheKeyedByModeAndProfile(t *testing.T) {
	def15, err := sharedZstdEncoder(CompressionBest, EncoderProfileDefault)
	if err != nil {
		t.Fatal(err)
	}
	def19, err := sharedZstdEncoder(CompressionMax, EncoderProfileDefault)
	if err != nil {
		t.Fatal(err)
	}
	bounded15, err := sharedZstdEncoder(CompressionBest, EncoderProfileBoundedMemory)
	if err != nil {
		t.Fatal(err)
	}
	bounded19, err := sharedZstdEncoder(CompressionMax, EncoderProfileBoundedMemory)
	if err != nil {
		t.Fatal(err)
	}
	if def15 != def19 {
		t.Error("default profile: levels 15 and 19 should share one SpeedBestCompression encoder")
	}
	if bounded15 != bounded19 {
		t.Error("bounded profile: levels 15 and 19 should share one SpeedBestCompression encoder")
	}
	if def15 == bounded15 {
		t.Error("default and bounded profiles must not share an encoder instance")
	}
	again, err := sharedZstdEncoder(CompressionBest, EncoderProfileBoundedMemory)
	if err != nil {
		t.Fatal(err)
	}
	if again != bounded15 {
		t.Error("repeated bounded lookup returned a different instance")
	}

	zstdEncoderMu.Lock()
	n := len(zstdEncoders)
	zstdEncoderMu.Unlock()
	if maxEntries := 4 * 2; n > maxEntries {
		t.Errorf("encoder cache holds %d entries, want at most %d (4 modes x 2 profiles)", n, maxEntries)
	}
}

// E5: a bounded config reaches every encode path that reads idx.Config, and
// the profile is never serialized.
func TestEncoderProfileConfigReachesAllEncodePaths(t *testing.T) {
	cfg, err := NewConfig(WithEncoderProfile(EncoderProfileBoundedMemory))
	if err != nil {
		t.Fatal(err)
	}
	idx := buildAdaptiveSerializationFixture(t, cfg)

	// S2: prove the config payload does not carry the profile by encoding the
	// same document set once through a bounded-profile config and once through
	// a default-profile config, both at Encode's default (CompressionBest)
	// level, and asserting the two outputs are byte-identical.
	t.Run("ConfigPayloadIndependentOfProfile", func(t *testing.T) {
		defaultIdx := buildAdaptiveSerializationFixture(t, DefaultConfig())
		boundedBytes, err := Encode(idx)
		if err != nil {
			t.Fatal(err)
		}
		defaultBytes, err := Encode(defaultIdx)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(boundedBytes, defaultBytes) {
			t.Error("serialized bytes differ between a bounded-profile config and a default-profile config over the same document set; the profile must be runtime-only")
		}
	})

	paths := map[string]func(t *testing.T) []byte{
		"Encode": func(t *testing.T) []byte {
			data, err := Encode(idx)
			if err != nil {
				t.Fatal(err)
			}
			return data
		},
		"EncodeContext": func(t *testing.T) []byte {
			data, err := EncodeContext(context.Background(), idx)
			if err != nil {
				t.Fatal(err)
			}
			return data
		},
		"WriteSidecar": func(t *testing.T) []byte {
			parquetFile := filepath.Join(t.TempDir(), "data.parquet")
			// WriteSidecar only stats the parquet file to mirror its permissions.
			if err := os.WriteFile(parquetFile, []byte("placeholder"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := WriteSidecar(parquetFile, idx); err != nil {
				t.Fatal(err)
			}
			loaded, err := ReadSidecar(parquetFile)
			if err != nil {
				t.Fatal(err)
			}
			data, err := Encode(loaded)
			if err != nil {
				t.Fatal(err)
			}
			return data
		},
		"EncodeToMetadata": func(t *testing.T) []byte {
			_, value, err := EncodeToMetadata(idx, DefaultParquetConfig())
			if err != nil {
				t.Fatal(err)
			}
			loaded, err := DecodeFromMetadata(value)
			if err != nil {
				t.Fatal(err)
			}
			data, err := Encode(loaded)
			if err != nil {
				t.Fatal(err)
			}
			return data
		},
	}

	for name, run := range paths {
		t.Run(name, func(t *testing.T) {
			evictSharedZstdEncoder(CompressionBest, EncoderProfileBoundedMemory)
			data := run(t)
			if !sharedZstdEncoderCached(CompressionBest, EncoderProfileBoundedMemory) {
				t.Errorf("%s did not use the bounded-memory encoder", name)
			}
			decoded, err := Decode(data)
			if err != nil {
				t.Fatal(err)
			}
			if decoded.Config == nil {
				t.Fatal("decoded index has nil config")
			}
			if decoded.Config.EncoderProfile != EncoderProfileDefault {
				t.Errorf("decoded EncoderProfile = %v, want default (profile must not be serialized)", decoded.Config.EncoderProfile)
			}
		})
	}
}

// I1: library helpers that only had an idx.Config-derived profile now also
// accept a per-call EncodeOption that overrides it. Build an index whose
// config carries the default profile (as every decoded index does), then
// prove WriteSidecar and EncodeToMetadata still honor an explicit
// WithEncodeProfile(EncoderProfileBoundedMemory) override.
func TestEncoderProfileLibraryHelpersAcceptPerCallOption(t *testing.T) {
	idx := buildAdaptiveSerializationFixture(t, DefaultConfig())

	t.Run("WriteSidecar", func(t *testing.T) {
		evictSharedZstdEncoder(CompressionBest, EncoderProfileBoundedMemory)
		parquetFile := filepath.Join(t.TempDir(), "data.parquet")
		if err := os.WriteFile(parquetFile, []byte("placeholder"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := WriteSidecar(parquetFile, idx, WithEncodeProfile(EncoderProfileBoundedMemory)); err != nil {
			t.Fatal(err)
		}
		if !sharedZstdEncoderCached(CompressionBest, EncoderProfileBoundedMemory) {
			t.Error("WriteSidecar with a per-call bounded profile did not use the bounded-memory encoder")
		}
	})

	t.Run("EncodeToMetadata", func(t *testing.T) {
		evictSharedZstdEncoder(CompressionBest, EncoderProfileBoundedMemory)
		if _, _, err := EncodeToMetadata(idx, DefaultParquetConfig(), WithEncodeProfile(EncoderProfileBoundedMemory)); err != nil {
			t.Fatal(err)
		}
		if !sharedZstdEncoderCached(CompressionBest, EncoderProfileBoundedMemory) {
			t.Error("EncodeToMetadata with a per-call bounded profile did not use the bounded-memory encoder")
		}
	})
}

// I2: prove the bounded profile actually configures the underlying zstd
// encoder with a single worker and lowMem=true, not just byte-identical
// output. This reaches into klauspost/compress internals via reflection
// because zstd.Encoder exposes no public accessor for worker count or lowMem;
// if a future klauspost/compress bump renames the `encoders` channel field or
// the `o.lowMem` field, inspectZstdEncoderInternals calls t.Fatal naming the
// missing field rather than passing silently -- see the failure messages
// below for the exact field names to check.
func TestEncoderProfileBoundedUsesSingleWorker(t *testing.T) {
	bounded, err := newZstdEncoder(zstd.SpeedBestCompression, EncoderProfileBoundedMemory)
	if err != nil {
		t.Fatalf("newZstdEncoder(bounded): %v", err)
	}
	defer bounded.Close()
	_ = bounded.EncodeAll([]byte("force lazy init"), nil)

	boundedWorkers, boundedLowMem := inspectZstdEncoderInternals(t, bounded)
	if boundedWorkers != 1 {
		t.Errorf("bounded profile: encoders channel capacity = %d, want 1 (depends on klauspost/compress internal field `encoders`)", boundedWorkers)
	}
	if !boundedLowMem {
		t.Error("bounded profile: o.lowMem = false, want true (depends on klauspost/compress internal field `o.lowMem`)")
	}

	def, err := newZstdEncoder(zstd.SpeedBestCompression, EncoderProfileDefault)
	if err != nil {
		t.Fatalf("newZstdEncoder(default): %v", err)
	}
	defer def.Close()
	_ = def.EncodeAll([]byte("force lazy init"), nil)

	defWorkers, _ := inspectZstdEncoderInternals(t, def)
	if gomaxprocs := runtime.GOMAXPROCS(0); gomaxprocs == 1 {
		t.Skipf("GOMAXPROCS=1: default and bounded worker counts are indistinguishable (both 1)")
	} else if defWorkers != gomaxprocs {
		t.Errorf("default profile: encoders channel capacity = %d, want GOMAXPROCS(0) = %d (depends on klauspost/compress internal field `encoders`)", defWorkers, gomaxprocs)
	}
}

// inspectZstdEncoderInternals reads the unexported `encoders` channel field
// (capacity == configured worker count) and the unexported `o.lowMem` field
// off a *zstd.Encoder via reflect+unsafe on the value newZstdEncoder returns.
func inspectZstdEncoderInternals(t *testing.T, enc *zstd.Encoder) (workers int, lowMem bool) {
	t.Helper()
	v := reflect.ValueOf(enc).Elem()

	encodersField := v.FieldByName("encoders")
	if !encodersField.IsValid() {
		t.Fatal("zstd.Encoder has no field named `encoders`; klauspost/compress internals changed")
	}
	encodersField = reflect.NewAt(encodersField.Type(), unsafe.Pointer(encodersField.UnsafeAddr())).Elem()
	workers = encodersField.Cap()

	optsField := v.FieldByName("o")
	if !optsField.IsValid() {
		t.Fatal("zstd.Encoder has no field named `o`; klauspost/compress internals changed")
	}
	lowMemField := optsField.FieldByName("lowMem")
	if !lowMemField.IsValid() {
		t.Fatal("zstd.Encoder.o has no field named `lowMem`; klauspost/compress internals changed")
	}
	lowMemField = reflect.NewAt(lowMemField.Type(), unsafe.Pointer(lowMemField.UnsafeAddr())).Elem()
	lowMem = lowMemField.Bool()

	return workers, lowMem
}

// E6: the per-call option wins over the config profile in both directions.
func TestEncoderProfilePerCallOverridesConfig(t *testing.T) {
	bounded, err := NewConfig(WithEncoderProfile(EncoderProfileBoundedMemory))
	if err != nil {
		t.Fatal(err)
	}
	boundedIdx := buildAdaptiveSerializationFixture(t, bounded)
	defaultIdx := buildAdaptiveSerializationFixture(t, DefaultConfig())
	ctx := context.Background()

	t.Run("config bounded, call default", func(t *testing.T) {
		evictSharedZstdEncoder(CompressionBalanced, EncoderProfileBoundedMemory)
		evictSharedZstdEncoder(CompressionBalanced, EncoderProfileDefault)
		if _, err := EncodeWithLevelContext(ctx, boundedIdx, CompressionBalanced, WithEncodeProfile(EncoderProfileDefault)); err != nil {
			t.Fatal(err)
		}
		if sharedZstdEncoderCached(CompressionBalanced, EncoderProfileBoundedMemory) {
			t.Error("explicit default per-call option did not override the bounded config")
		}
		if !sharedZstdEncoderCached(CompressionBalanced, EncoderProfileDefault) {
			t.Error("explicit default per-call option did not construct the default encoder")
		}
	})
	t.Run("config default, call bounded", func(t *testing.T) {
		evictSharedZstdEncoder(CompressionFastest, EncoderProfileBoundedMemory)
		if _, err := EncodeWithLevelContext(ctx, defaultIdx, CompressionFastest, WithEncodeProfile(EncoderProfileBoundedMemory)); err != nil {
			t.Fatal(err)
		}
		if !sharedZstdEncoderCached(CompressionFastest, EncoderProfileBoundedMemory) {
			t.Error("bounded per-call option did not override the default config")
		}
	})
	t.Run("nil config falls back to default", func(t *testing.T) {
		if got := configEncoderProfile(nil); got != EncoderProfileDefault {
			t.Errorf("configEncoderProfile(nil) = %v, want default", got)
		}
	})
}

// E7: an unknown profile fails loudly from the config option, from config
// validation, and from the encode call.
func TestEncoderProfileUnknownValueRejected(t *testing.T) {
	t.Parallel()
	const bogus = EncoderProfile(200)

	if _, err := NewConfig(WithEncoderProfile(bogus)); err == nil {
		t.Error("NewConfig(WithEncoderProfile(200)) returned nil error")
	} else if !strings.Contains(err.Error(), "unknown encoder profile 200") {
		t.Errorf("NewConfig error = %q, want mention of unknown encoder profile 200", err)
	}

	cfg := DefaultConfig()
	cfg.EncoderProfile = bogus
	if err := cfg.validate(); err == nil {
		t.Error("config validate accepted an unknown encoder profile")
	}

	idx := buildAdaptiveSerializationFixture(t, DefaultConfig())
	if _, err := EncodeWithLevelContext(context.Background(), idx, CompressionBest, WithEncodeProfile(bogus)); err == nil {
		t.Error("encode with unknown per-call profile returned nil error")
	}
	if _, err := EncodeWithLevelContext(context.Background(), idx, CompressionNone, WithEncodeProfile(bogus)); err == nil {
		t.Error("uncompressed encode with unknown per-call profile returned nil error")
	}
	if _, err := newZstdEncoder(zstd.SpeedDefault, bogus); err == nil {
		t.Error("newZstdEncoder accepted an unknown profile")
	}
}

func TestEncoderProfileString(t *testing.T) {
	t.Parallel()
	cases := map[EncoderProfile]string{
		EncoderProfileDefault:       "default",
		EncoderProfileBoundedMemory: "bounded-memory",
		EncoderProfile(7):           "EncoderProfile(7)",
	}
	for p, want := range cases {
		if got := p.String(); got != want {
			t.Errorf("EncoderProfile(%d).String() = %q, want %q", uint8(p), got, want)
		}
	}
}

// Uncached profile tests (issue #83).

var allEncoderProfiles = []EncoderProfile{
	EncoderProfileDefault, EncoderProfileBoundedMemory, EncoderProfileBoundedMemoryUncached,
}

func evictAllProfiles(level CompressionLevel) {
	for _, p := range allEncoderProfiles {
		evictSharedZstdEncoder(level, p)
	}
}

func assertNoCachedEncoder(t *testing.T, level CompressionLevel) {
	t.Helper()
	for _, p := range allEncoderProfiles {
		if sharedZstdEncoderCached(level, p) {
			t.Errorf("level %d: cache holds an encoder for profile %s", level, p)
		}
	}
}

// E1: uncached output is byte-identical to default output for single-block
// and multi-block payloads at every level.
func TestEncoderProfileUncachedOutputIdenticalToDefault(t *testing.T) {
	t.Parallel()
	fixtures := map[string]*GINIndex{
		"single-block": buildAdaptiveSerializationFixture(t, DefaultConfig()),
		"multi-block":  buildHighCardinalityIndex(t, 200, 20),
	}
	ctx := context.Background()
	for name, idx := range fixtures {
		for _, level := range encoderProfileLevels {
			want, err := EncodeWithLevelContext(ctx, idx, level, WithEncodeProfile(EncoderProfileDefault))
			if err != nil {
				t.Fatalf("%s default level %d: %v", name, level, err)
			}
			got, err := EncodeWithLevelContext(ctx, idx, level, WithEncodeProfile(EncoderProfileBoundedMemoryUncached))
			if err != nil {
				t.Fatalf("%s uncached level %d: %v", name, level, err)
			}
			if !bytes.Equal(want, got) {
				t.Fatalf("%s level %d: uncached output differs from default output", name, level)
			}
			if _, err := Decode(got); err != nil {
				t.Fatalf("%s level %d: decode: %v", name, level, err)
			}
		}
	}
}

// E2 and E5: an uncached encode leaves no entry in the cache, whether chosen
// per call over a default config or by the config itself. A per-call bounded
// override over an uncached config does populate the bounded entry.
func TestEncoderProfileUncachedLeavesCacheEmpty(t *testing.T) {
	idx := buildAdaptiveSerializationFixture(t, DefaultConfig())
	ctx := context.Background()
	for _, level := range encoderProfileLevels {
		evictAllProfiles(level)
		if _, err := EncodeWithLevelContext(ctx, idx, level, WithEncodeProfile(EncoderProfileBoundedMemoryUncached)); err != nil {
			t.Fatal(err)
		}
		assertNoCachedEncoder(t, level)
	}
	evictAllProfiles(CompressionBest)
	if _, err := EncodeContext(ctx, idx, WithEncodeProfile(EncoderProfileBoundedMemoryUncached)); err != nil {
		t.Fatal(err)
	}
	assertNoCachedEncoder(t, CompressionBest)

	cfg, err := NewConfig(WithEncoderProfile(EncoderProfileBoundedMemoryUncached))
	if err != nil {
		t.Fatal(err)
	}
	uncachedCfgIdx := buildAdaptiveSerializationFixture(t, cfg)
	if _, err := Encode(uncachedCfgIdx); err != nil {
		t.Fatal(err)
	}
	assertNoCachedEncoder(t, CompressionBest)
	if _, err := EncodeContext(ctx, uncachedCfgIdx, WithEncodeProfile(EncoderProfileBoundedMemory)); err != nil {
		t.Fatal(err)
	}
	if !sharedZstdEncoderCached(CompressionBest, EncoderProfileBoundedMemory) {
		t.Error("per-call bounded override should populate the bounded cache entry")
	}
	evictAllProfiles(CompressionBest)
}

// E6: an uncached config reaches every encode path and none caches an encoder.
func TestEncoderProfileUncachedConfigReachesAllEncodePaths(t *testing.T) {
	cfg, err := NewConfig(WithEncoderProfile(EncoderProfileBoundedMemoryUncached))
	if err != nil {
		t.Fatal(err)
	}
	idx := buildAdaptiveSerializationFixture(t, cfg)
	paths := map[string]func(t *testing.T){
		"Encode": func(t *testing.T) {
			if _, err := Encode(idx); err != nil {
				t.Fatal(err)
			}
		},
		"WriteSidecar": func(t *testing.T) {
			parquetFile := filepath.Join(t.TempDir(), "data.parquet")
			if err := os.WriteFile(parquetFile, []byte("placeholder"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := WriteSidecar(parquetFile, idx); err != nil {
				t.Fatal(err)
			}
		},
		"EncodeToMetadata": func(t *testing.T) {
			if _, _, err := EncodeToMetadata(idx, DefaultParquetConfig()); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, run := range paths {
		t.Run(name, func(t *testing.T) {
			evictAllProfiles(CompressionBest)
			run(t)
			assertNoCachedEncoder(t, CompressionBest)
		})
	}
}

// encodeWithProfileNoRetain runs one level-15 encode. It is noinline and
// returns nothing, so no stack slot in the caller keeps the encoder alive.
//
//go:noinline
func encodeWithProfileNoRetain(idx *GINIndex, profile EncoderProfile) {
	if _, err := EncodeWithLevelContext(context.Background(), idx, CompressionBest, WithEncodeProfile(profile)); err != nil {
		panic(err)
	}
}

func settledHeap() int64 {
	heapAllocAfterGC()
	return int64(heapAllocAfterGC())
}

// E3: after one level-15 uncached encode and GC, the heap is near the
// baseline. A cached bounded encoder keeps about 42 MB, which the control
// half of the test must see. Not parallel: it reads global heap state.
func TestEncoderProfileUncachedDoesNotRetainEncoder(t *testing.T) {
	idx := buildAdaptiveSerializationFixture(t, DefaultConfig())
	evictAllProfiles(CompressionBest)
	encodeWithProfileNoRetain(idx, EncoderProfileBoundedMemoryUncached) // warm up
	// Evict again: an encoder the warm-up wrongly cached would otherwise sit
	// in the baseline and hide the retention this test looks for.
	evictAllProfiles(CompressionBest)

	base := settledHeap()
	encodeWithProfileNoRetain(idx, EncoderProfileBoundedMemoryUncached)
	growth := settledHeap() - base
	if growth > 8<<20 {
		t.Errorf("uncached profile retained %d bytes after encode, want under 8 MiB", growth)
	}

	// Control: the cached bounded profile must show the retained encoder.
	base = settledHeap()
	encodeWithProfileNoRetain(idx, EncoderProfileBoundedMemory)
	controlGrowth := settledHeap() - base
	evictSharedZstdEncoder(CompressionBest, EncoderProfileBoundedMemory)
	if controlGrowth < 20<<20 {
		t.Errorf("control: cached bounded profile grew heap by %d bytes, want over 20 MiB", controlGrowth)
	}
}

// E4: the encoder built for the uncached profile has one worker and low-memory buffers.
func TestEncoderProfileUncachedUsesSingleWorker(t *testing.T) {
	enc, err := newZstdEncoder(zstd.SpeedBestCompression, EncoderProfileBoundedMemoryUncached)
	if err != nil {
		t.Fatal(err)
	}
	defer enc.Close()
	enc.EncodeAll([]byte("force lazy init"), nil)
	workers, lowMem := inspectZstdEncoderInternals(t, enc)
	if workers != 1 || !lowMem {
		t.Errorf("workers = %d, lowMem = %v, want 1 and true", workers, lowMem)
	}
}

// E7: the two older constants keep their values and still cache.
func TestEncoderProfileConstantValuesAndCachedProfiles(t *testing.T) {
	if EncoderProfileDefault != 0 || EncoderProfileBoundedMemory != 1 || EncoderProfileBoundedMemoryUncached != 2 {
		t.Fatal("encoder profile constants changed value")
	}
	idx := buildAdaptiveSerializationFixture(t, DefaultConfig())
	for _, p := range []EncoderProfile{EncoderProfileDefault, EncoderProfileBoundedMemory} {
		evictSharedZstdEncoder(CompressionBalanced, p)
		if _, err := EncodeWithLevelContext(context.Background(), idx, CompressionBalanced, WithEncodeProfile(p)); err != nil {
			t.Fatal(err)
		}
		if !sharedZstdEncoderCached(CompressionBalanced, p) {
			t.Errorf("profile %s did not populate the cache", p)
		}
	}
}

// E8: String, validation and error text for the new value.
func TestEncoderProfileUncachedStringAndValidation(t *testing.T) {
	t.Parallel()
	if got := EncoderProfileBoundedMemoryUncached.String(); got != "bounded-memory-uncached" {
		t.Errorf("String() = %q", got)
	}
	if _, err := NewConfig(WithEncoderProfile(EncoderProfileBoundedMemoryUncached)); err != nil {
		t.Errorf("WithEncoderProfile rejected the uncached profile: %v", err)
	}
	err := EncoderProfile(200).validate()
	if err == nil {
		t.Fatal("validate accepted profile 200")
	}
	for _, want := range []string{"unknown encoder profile 200", "EncoderProfileDefault", "EncoderProfileBoundedMemory", "EncoderProfileBoundedMemoryUncached"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

// E9: the profile is runtime-only. The wire format and decoded config stay unchanged.
func TestEncoderProfileUncachedWireFormat(t *testing.T) {
	t.Parallel()
	if Version != 11 {
		t.Fatalf("Version = %d, want 11", Version)
	}
	idx := buildAdaptiveSerializationFixture(t, DefaultConfig())
	data, err := EncodeContext(context.Background(), idx, WithEncodeProfile(EncoderProfileBoundedMemoryUncached))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Config.EncoderProfile != EncoderProfileDefault {
		t.Errorf("decoded profile = %s, want default", decoded.Config.EncoderProfile)
	}
}

// E10: CompressionNone returns before any encoder call (encodeWithLevel), so
// no encoder is reachable at level 0. The test checks the bytes, the empty
// cache and that allocation stays far below the cost of one encoder.
func TestEncoderProfileUncachedCompressionNone(t *testing.T) {
	idx := buildAdaptiveSerializationFixture(t, DefaultConfig())
	ctx := context.Background()
	for _, level := range encoderProfileLevels {
		evictAllProfiles(level)
	}
	want, err := EncodeWithLevelContext(ctx, idx, CompressionNone, WithEncodeProfile(EncoderProfileDefault))
	if err != nil {
		t.Fatal(err)
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	got, err := EncodeWithLevelContext(ctx, idx, CompressionNone, WithEncodeProfile(EncoderProfileBoundedMemoryUncached))
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(got, []byte(uncompressedMagic)) || !bytes.Equal(got, want) {
		t.Error("uncached CompressionNone output differs from default or lacks the GINu prefix")
	}
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 1<<20 {
		t.Errorf("CompressionNone allocated %d bytes, want under 1 MiB (no encoder)", alloc)
	}
	for _, level := range encoderProfileLevels {
		assertNoCachedEncoder(t, level)
	}
}
