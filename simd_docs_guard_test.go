package gin

import (
	"strings"
	"testing"
)

type simdDocumentationInputs struct {
	effectiveVersion string
	guide            string
	example          string
	release          string
	readme           string
	changelog        string
	parser           string
	libraryLoading   string
	bootstrap        string
	cache            string
}

type simdModuleListing struct {
	Version string
	Dir     string
	Replace *simdModuleReplacement
}

type simdModuleReplacement struct {
	Version string
	Dir     string
}

func TestSIMDDocumentationContract(t *testing.T) {
	inputs := loadSIMDDocumentationInputs(t)
	if err := validateSIMDDocumentationInputs(inputs); err != nil {
		t.Fatalf("aligned SIMD documentation: %v", err)
	}

	loadingMutations := []struct {
		name       string
		field      *string
		oldValue   string
		diagnostic string
	}{
		{name: "library path", field: &inputs.libraryLoading, oldValue: "PURE_SIMDJSON_LIB_PATH", diagnostic: "loading variable libraryEnvPath"},
		{name: "binary mirror", field: &inputs.bootstrap, oldValue: "PURE_SIMDJSON_BINARY_MIRROR", diagnostic: "loading variable mirrorEnvVar"},
		{name: "fallback control", field: &inputs.bootstrap, oldValue: "PURE_SIMDJSON_DISABLE_GH_FALLBACK", diagnostic: "loading variable disableGHEnvVar"},
		{name: "cache directory", field: &inputs.cache, oldValue: "PURE_SIMDJSON_CACHE_DIR", diagnostic: "loading variable cacheDirEnvVar"},
	}
	for _, mutation := range loadingMutations {
		t.Run("rejects changed "+mutation.name, func(t *testing.T) {
			mutated := inputs
			switch mutation.name {
			case "library path":
				mutated.libraryLoading = strings.Replace(mutated.libraryLoading, mutation.oldValue, mutation.oldValue+"_DRIFT", 1)
			case "binary mirror", "fallback control":
				mutated.bootstrap = strings.Replace(mutated.bootstrap, mutation.oldValue, mutation.oldValue+"_DRIFT", 1)
			case "cache directory":
				mutated.cache = strings.Replace(mutated.cache, mutation.oldValue, mutation.oldValue+"_DRIFT", 1)
			}
			requireSIMDDocumentationFailure(t, mutated, mutation.diagnostic)
		})
	}

	mutations := []struct {
		name       string
		mutate     func(*simdDocumentationInputs)
		diagnostic string
	}{
		{
			name: "parser guide path",
			mutate: func(in *simdDocumentationInputs) {
				in.parser = strings.Replace(in.parser, "docs/simd-deployment.md", "docs/simd-moved.md", 1)
			},
			diagnostic: "parser guide path",
		},
		{
			name: "README guide link",
			mutate: func(in *simdDocumentationInputs) {
				in.readme = strings.Replace(in.readme, "docs/simd-deployment.md", "docs/simd-moved.md", 1)
			},
			diagnostic: "README guide link",
		},
		{
			name: "CHANGELOG guide link",
			mutate: func(in *simdDocumentationInputs) {
				in.changelog = strings.Replace(in.changelog, "docs/simd-deployment.md", "docs/simd-moved.md", 1)
			},
			diagnostic: "CHANGELOG guide link",
		},
		{
			name: "effective bootstrap pin",
			mutate: func(in *simdDocumentationInputs) {
				in.guide = strings.Replace(in.guide, "/blob/"+in.effectiveVersion+"/docs/bootstrap.md", "/blob/main/docs/bootstrap.md", 1)
			},
			diagnostic: "effective-tag bootstrap link",
		},
		{
			name: "release guide link",
			mutate: func(in *simdDocumentationInputs) {
				in.release = strings.Replace(in.release, "/docs/simd-deployment.md", "/docs/simd-moved.md", 1)
			},
			diagnostic: "release header guide link",
		},
		{
			name: "example body",
			mutate: func(in *simdDocumentationInputs) {
				in.guide = strings.Replace(in.guide, "const numRGs = 1", "const numRGs = 2", 1)
			},
			diagnostic: "SIMD example snippet",
		},
	}
	for _, mutation := range mutations {
		t.Run("rejects "+mutation.name+" drift", func(t *testing.T) {
			mutated := inputs
			mutation.mutate(&mutated)
			requireSIMDDocumentationFailure(t, mutated, mutation.diagnostic)
		})
	}

	t.Run("ignores unrelated upstream diagnostics", func(t *testing.T) {
		mutated := inputs
		mutated.libraryLoading += "\nconst unrelatedDiagnostic = \"PURE_SIMDJSON_WARN_LEAKS\"\n"
		if err := validateSIMDDocumentationInputs(mutated); err != nil {
			t.Fatalf("unrelated diagnostic expanded loading-variable contract: %v", err)
		}
	})

	t.Run("versioned replacement is effective", func(t *testing.T) {
		version, dir, err := effectiveSIMDModule(simdModuleListing{
			Version: "v0.1.0",
			Dir:     "/module-cache",
			Replace: &simdModuleReplacement{Version: sentinelReplacementVersion, Dir: "/replacement-cache"},
		})
		if err != nil {
			t.Fatalf("effectiveSIMDModule: %v", err)
		}
		if version != sentinelReplacementVersion || dir != "/replacement-cache" {
			t.Fatalf("effective module = (%q, %q), want (%q, %q)", version, dir, sentinelReplacementVersion, "/replacement-cache")
		}
	})

	t.Run("unversioned local replacement fails closed", func(t *testing.T) {
		_, _, err := effectiveSIMDModule(simdModuleListing{
			Version: "v0.1.0",
			Dir:     "/module-cache",
			Replace: &simdModuleReplacement{Dir: "/local-replacement"},
		})
		if err == nil || !strings.Contains(err.Error(), "unversioned effective replacement") {
			t.Fatalf("effectiveSIMDModule error = %v, want unversioned replacement diagnostic", err)
		}
	})
}

func requireSIMDDocumentationFailure(t *testing.T, inputs simdDocumentationInputs, diagnostic string) {
	t.Helper()

	err := validateSIMDDocumentationInputs(inputs)
	if err == nil || !strings.Contains(err.Error(), diagnostic) {
		t.Fatalf("documentation mutation error = %v, want diagnostic %q", err, diagnostic)
	}
}
