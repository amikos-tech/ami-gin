//go:build simdjson

// Shared helpers for Spike 003. Mirrors the parts of the repo's
// benchmark_test.go that Plan 22-05 builds on (phase20SmokeFixtures,
// phase20LoadRawJSONL, phase20BuildBenchmarkIndex) using only the public API,
// so this spike stays in its own module per CONVENTIONS.md.
package simdbenchspike

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	gin "github.com/amikos-tech/ami-gin"
)

// docsPerRowGroup mirrors benchmark_test.go:1762.
const docsPerRowGroup = 32

// fixtureRoot is the repo's checked-in Phase 20 smoke tier.
const fixtureRoot = "../../../testdata/phase20"

type fixture struct {
	name  string
	file  string
	docs  [][]byte
	bytes int64
}

// smokeFixtureNames mirrors phase20SmokeFixtures order (benchmark_test.go:1792).
var smokeFixtureNames = []struct{ name, file string }{
	{"nested-high-cardinality", "nested_high_cardinality.jsonl"},
	{"mixed-type-arrays", "mixed_type_arrays.jsonl"},
	{"number-heavy", "number_heavy.jsonl"},
	{"combined", "combined.jsonl"},
}

func loadFixtures(tb testing.TB) []fixture {
	tb.Helper()
	out := make([]fixture, 0, len(smokeFixtureNames))
	for _, f := range smokeFixtureNames {
		docs, err := loadJSONL(filepath.Join(fixtureRoot, f.file))
		if err != nil {
			tb.Fatalf("load fixture %s: %v", f.name, err)
		}
		var total int64
		for _, d := range docs {
			total += int64(len(d))
		}
		out = append(out, fixture{name: f.name, file: f.file, docs: docs, bytes: total})
	}
	return out
}

// loadJSONL mirrors phase20LoadRawJSONL (benchmark_test.go:1811).
func loadJSONL(path string) ([][]byte, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	docs := make([][]byte, 0)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if !json.Valid(line) {
			return nil, fmt.Errorf("invalid JSON record in %s", path)
		}
		docs = append(docs, append([]byte(nil), line...))
	}
	return docs, scanner.Err()
}

// buildIndex mirrors phase20BuildBenchmarkIndex (benchmark_test.go:1845).
// A nil parser selects the default stdlib path, exactly as Plan 22-05's
// stdlib arm does.
func buildIndex(docs [][]byte, parser gin.Parser) (*gin.GINIndex, error) {
	if len(docs) == 0 {
		return nil, fmt.Errorf("fixture contains no documents")
	}
	rowGroups := (len(docs) + docsPerRowGroup - 1) / docsPerRowGroup

	var opts []gin.BuilderOption
	if parser != nil {
		opts = append(opts, gin.WithParser(parser))
	}
	b, err := gin.NewBuilder(gin.DefaultConfig(), rowGroups, opts...)
	if err != nil {
		return nil, err
	}
	for i, doc := range docs {
		if err := b.AddDocument(gin.DocID(i/docsPerRowGroup), doc); err != nil {
			return nil, err
		}
	}
	idx := b.Finalize()
	if idx == nil {
		return nil, fmt.Errorf("Finalize returned nil")
	}
	return idx, nil
}

// benchFixture runs one fixture through buildIndex under testing.Benchmark,
// with parser construction and fixture I/O already outside the timed region —
// matching Plan 22-05's stated timing boundary.
func benchFixture(f fixture, parser gin.Parser) testing.BenchmarkResult {
	return testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(f.bytes)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			idx, err := buildIndex(f.docs, parser)
			if err != nil {
				b.Fatalf("buildIndex: %v", err)
			}
			_ = idx
		}
	})
}

// ---------------------------------------------------------------------------
// Native memory observation.
//
// Go's B/op counts only Go-heap allocations. pure-simdjson's parser buffer
// lives in native memory reached through purego, so Go's allocator never sees
// it. Upstream HAS native telemetry (NativeAllocStatsReset/Snapshot in
// benchmark_native_alloc_test.go) but it is package-internal — the exported
// Parser surface is only Parse/Close. Process RSS is therefore the only
// signal available from outside the module.
// ---------------------------------------------------------------------------

// rssKB reads this process's resident set size in kilobytes. darwin/linux.
func rssKB(tb testing.TB) int64 {
	tb.Helper()
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(os.Getpid())).Output()
	if err != nil {
		tb.Logf("rssKB: ps failed (%v) — native growth will be unobservable", err)
		return -1
	}
	v, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return -1
	}
	return v
}

func mib(kb int64) string {
	if kb < 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f MiB", float64(kb)/1024.0)
}

// pctDelta returns (b-a)/a as a signed percentage string.
func pctDelta(a, b float64) string {
	if a == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%+.1f%%", (b-a)/a*100.0)
}

func nsPerOp(r testing.BenchmarkResult) float64 {
	if r.N == 0 {
		return 0
	}
	return float64(r.T.Nanoseconds()) / float64(r.N)
}

func bytesPerOp(r testing.BenchmarkResult) float64 {
	if r.N == 0 {
		return 0
	}
	return float64(r.MemBytes) / float64(r.N)
}

func allocsPerOp(r testing.BenchmarkResult) float64 {
	if r.N == 0 {
		return 0
	}
	return float64(r.MemAllocs) / float64(r.N)
}
