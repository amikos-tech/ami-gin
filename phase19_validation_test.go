package gin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPhase19SIMDStrategyArtifact(t *testing.T) {
	root := findPhase19ModuleRoot(t)
	path := filepath.Join(root, ".planning", "phases", "19-simd-dependency-decision-integration-strategy", "19-SIMD-STRATEGY.md")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	text := string(data)

	required := []string{
		"## Decision Summary",
		"## SIMD-01: Dependency Source, Pinning, License, and NOTICE",
		"## SIMD-02: Shared-Library Distribution and Loading",
		"## SIMD-03: Build Strategy, Opt-In API, CI, and Stop Conditions",
		"## Downstream Phase 21 Contract",
		"## Downstream Phase 22 Contract",
		"## Out of Scope for Phase 19",
		"Dependency: github.com/amikos-tech/pure-simdjson v0.1.4",
		"Tag commit: 0f53f3f2e8bb9608d6b79211ffc5fc7b53298617",
		"License: MIT",
		"NOTICE posture: add root NOTICE.md and README dependency credit in Phase 21",
		"PURE_SIMDJSON_LIB_PATH",
		"PURE_SIMDJSON_BINARY_MIRROR",
		"PURE_SIMDJSON_DISABLE_GH_FALLBACK",
		"PURE_SIMDJSON_CACHE_DIR",
		"windows-amd64-msvc",
		"pure-simdjson-${{ matrix.os }}-${{ matrix.arch }}-v0.1.4",
		"//go:build simdjson",
		"NewSIMDParser() (Parser, error)",
		"Name() == \"pure-simdjson\"",
		"WithParser",
		"gin.NewBuilder(cfg, numRGs, gin.WithParser(p))",
		"gin.NewBuilder(cfg, numRGs)",
		"No silent fallback: NewSIMDParser returns an error instead of internally selecting stdlib.",
		"On a HARD trigger, pause Phase 21/22 with /gsd-pause-work",
		"Phase 19 does not edit `go.mod`, `go.sum`, source files, CI workflows, README, NOTICE, CHANGELOG, or runtime docs.",
	}
	for _, want := range required {
		if !strings.Contains(text, want) {
			t.Fatalf("strategy artifact missing %q", want)
		}
	}

	forbidden := []string{
		"SIMD is enabled by default",
		"silently falls back to stdlib",
	}
	for _, phrase := range forbidden {
		if strings.Contains(text, phrase) {
			t.Fatalf("strategy artifact contains forbidden phrase %q", phrase)
		}
	}
}

func findPhase19ModuleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd(): %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate module root from working directory")
		}
		dir = parent
	}
}
