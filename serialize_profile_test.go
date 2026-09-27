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

// sharedZstdEncoderCached mirrors evictSharedZstdEncoder's (level, profile)
// signature for symmetry. Every current call site checks the bounded-memory
// profile, which trips unparam; keep profile explicit rather than hardcoding
// it so a future default-profile assertion does not need a signature change.
//
//nolint:unparam // see comment above
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
	// level, and asserting the two outputs are byte-identical. A tautological
	// decoded.Header.Version == Version check previously stood in for this.
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
// the `o.lowMem` field, this test will fail with a reflect panic or a
// FieldByName zero Value, not a silent false pass -- see the failure messages
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
// off a *zstd.Encoder via reflect+unsafe. It does not import the zstd internal
// package; it only reflects on the exported *zstd.Encoder value returned by
// newZstdEncoder.
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
		if _, err := EncodeWithLevelContext(ctx, boundedIdx, CompressionBalanced, WithEncodeProfile(EncoderProfileDefault)); err != nil {
			t.Fatal(err)
		}
		if sharedZstdEncoderCached(CompressionBalanced, EncoderProfileBoundedMemory) {
			t.Error("explicit default per-call option did not override the bounded config")
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
