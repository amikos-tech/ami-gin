# Compression Levels and Benchmarking

## Overview

Currently the GIN index uses zstd with default settings. This task explores compression level options, benchmarks the tradeoffs, and establishes zstd-15 as the recommended default for production use.

## Goals

1. **Configurable Compression** - Allow users to choose compression level
2. **Benchmark All Levels** - Measure size vs encode/decode speed tradeoffs
3. **Establish Default** - Validate zstd-15 as optimal default
4. **Support No Compression** - For latency-critical use cases

## Current State

```go
// serialize.go - current implementation
encoder, err := zstd.NewWriter(nil)  // default level
```

Default zstd level is 3, which prioritizes speed over compression ratio.

## Proposed API

### Configuration

```go
type CompressionLevel int

const (
    CompressionNone    CompressionLevel = 0
    CompressionFastest CompressionLevel = 1   // zstd level 1
    CompressionDefault CompressionLevel = 3   // zstd level 3
    CompressionBetter  CompressionLevel = 9   // zstd level 9
    CompressionBest    CompressionLevel = 15  // zstd level 15 (recommended)
    CompressionMax     CompressionLevel = 19  // zstd level 19 (slow)
)

// Encode with specific compression level
func EncodeWithLevel(idx *GINIndex, level CompressionLevel) ([]byte, error)

// Encode with default (zstd-15)
func Encode(idx *GINIndex) ([]byte, error) {
    return EncodeWithLevel(idx, CompressionBest)
}
```

### Backward Compatibility

- Decode should auto-detect compression (zstd is self-describing)
- Magic bytes can include compression indicator for future formats
- Uncompressed format for debugging/testing

## Benchmark Plan

### Test Scenarios

1. **Small Index** - 100 RGs, 10 paths
2. **Medium Index** - 1K RGs, 20 paths
3. **Large Index** - 50K RGs, 10 paths (1M docs scenario)
4. **High Cardinality** - 10K RGs, high unique values
5. **With Trigrams** - FTS-enabled index

### Metrics to Capture

| Level | Size | Ratio | Encode Time | Decode Time | Encode MB/s | Decode MB/s |
|-------|------|-------|-------------|-------------|-------------|-------------|
| None  |      |       |             |             |             |             |
| 1     |      |       |             |             |             |             |
| 3     |      |       |             |             |             |             |
| 9     |      |       |             |             |             |             |
| 15    |      |       |             |             |             |             |
| 19    |      |       |             |             |             |             |

### Benchmark Code

```go
func BenchmarkCompressionLevels(b *testing.B) {
    idx := setupLargeIndex(50000)  // 50K RGs

    levels := []struct {
        name  string
        level int
    }{
        {"None", 0},
        {"Zstd-1", 1},
        {"Zstd-3", 3},
        {"Zstd-9", 9},
        {"Zstd-15", 15},
        {"Zstd-19", 19},
    }

    for _, l := range levels {
        b.Run(fmt.Sprintf("Encode/%s", l.name), func(b *testing.B) {
            for i := 0; i < b.N; i++ {
                _, _ = EncodeWithLevel(idx, l.level)
            }
        })
    }

    // Pre-encode for decode benchmarks
    encoded := make(map[string][]byte)
    for _, l := range levels {
        encoded[l.name], _ = EncodeWithLevel(idx, l.level)
    }

    for _, l := range levels {
        data := encoded[l.name]
        b.Run(fmt.Sprintf("Decode/%s", l.name), func(b *testing.B) {
            for i := 0; i < b.N; i++ {
                _, _ = Decode(data)
            }
        })

        b.Run(fmt.Sprintf("Size/%s", l.name), func(b *testing.B) {
            b.ReportMetric(float64(len(data)), "bytes")
            b.ReportMetric(float64(len(data))/1024, "KB")
        })
    }
}
```

## Expected Results (Hypothesis)

Based on zstd characteristics:

| Level | Relative Size | Encode Speed | Decode Speed | Use Case |
|-------|---------------|--------------|--------------|----------|
| None  | 1.0x          | Instant      | Instant      | Debugging, testing |
| 1     | ~0.7x         | Very Fast    | Very Fast    | Real-time streaming |
| 3     | ~0.6x         | Fast         | Very Fast    | Default (current) |
| 9     | ~0.5x         | Medium       | Very Fast    | Balanced |
| **15**| ~0.45x        | Slower       | Very Fast    | **Recommended** |
| 19    | ~0.42x        | Very Slow    | Very Fast    | Archival |

**Why zstd-15?**
- Decode speed is nearly identical across all levels (zstd asymmetry)
- Index is built once, read many times
- Size reduction saves bandwidth/storage
- Build time is amortized over many queries
- 15 is "sweet spot" before diminishing returns

## Implementation

### serialize.go Changes

```go
import "github.com/klauspost/compress/zstd"

type EncodeOptions struct {
    CompressionLevel int  // 0 = none, 1-19 = zstd level
}

func EncodeWithOptions(idx *GINIndex, opts EncodeOptions) ([]byte, error) {
    var buf bytes.Buffer

    // ... write index data ...

    if opts.CompressionLevel == 0 {
        // Prepend magic for uncompressed
        return append([]byte("GINu"), buf.Bytes()...), nil
    }

    encoder, err := zstd.NewWriter(nil,
        zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(opts.CompressionLevel)))
    if err != nil {
        return nil, err
    }
    defer encoder.Close()

    return encoder.EncodeAll(buf.Bytes(), nil), nil
}

func Decode(data []byte) (*GINIndex, error) {
    // Check for uncompressed magic
    if len(data) >= 4 && string(data[:4]) == "GINu" {
        return decodeUncompressed(data[4:])
    }

    // Otherwise assume zstd (auto-detected)
    decoder, _ := zstd.NewReader(nil)
    defer decoder.Close()

    decompressed, err := decoder.DecodeAll(data, nil)
    // ...
}
```

### Default Change

```go
// Change default from zstd-3 to zstd-15
func Encode(idx *GINIndex) ([]byte, error) {
    return EncodeWithOptions(idx, EncodeOptions{CompressionLevel: 15})
}
```

## Testing

- [ ] Benchmark all compression levels with multiple index sizes
- [ ] Verify decode works for all levels
- [ ] Test backward compatibility (decode old format)
- [ ] Test uncompressed format
- [ ] Memory usage during encode/decode
- [ ] Parallel encode/decode safety

## Documentation

- Add compression level table to README benchmarks section
- Document `EncodeWithOptions` API
- Explain when to use different levels

## Success Criteria

1. Benchmarks show zstd-15 decode is as fast as zstd-3
2. zstd-15 achieves 20-30% smaller size than zstd-3
3. API allows compression level selection
4. Default changed to zstd-15 with no breaking changes
5. README updated with compression tradeoffs

## Future Work

- Support other codecs (lz4, snappy) for specific use cases
- Dictionary compression for repeated patterns
- Streaming encode/decode for very large indexes
- Per-section compression (different levels for different parts)
