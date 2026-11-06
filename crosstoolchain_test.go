//go:build crosstoolchain

package gin

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unicode"
)

// These tests are the two halves of a real artifact exchange between Go
// toolchains with different Unicode tables (#102). `make cross-toolchain-check`
// runs the build half under each toolchain, requires the artifacts of the two
// toolchains to be byte-identical, and runs the query half under each
// toolchain against the artifacts of each toolchain. Without the environment
// variables set, both tests skip.
//
//	AMI_GIN_XTC_WANT_UNICODE  unicode.Version the running toolchain must report
//	AMI_GIN_XTC_OUT           directory the build half writes the artifacts to
//	AMI_GIN_XTC_IN            directory the query half reads the artifacts from
//	AMI_GIN_XTC_DONE          file the query half writes after its assertions,
//	                          so the Makefile can prove that the query ran

const (
	xtcEnvWantUnicode = "AMI_GIN_XTC_WANT_UNICODE"
	xtcEnvOut         = "AMI_GIN_XTC_OUT"
	xtcEnvIn          = "AMI_GIN_XTC_IN"
	xtcEnvDone        = "AMI_GIN_XTC_DONE"
)

// Artifact file names inside the AMI_GIN_XTC_OUT and AMI_GIN_XTC_IN
// directories. The Makefile compares the same names.
const (
	xtcDriftArtifact      = "drift.gin"
	xtcAggregatedArtifact = "aggregated.gin"
)

// requireCrossToolchainUnicode fails unless the toolchain reports the Unicode
// version the Makefile expects. This stops a degenerate run where both sides
// resolve to the same toolchain.
func requireCrossToolchainUnicode(t *testing.T) {
	t.Helper()
	want := os.Getenv(xtcEnvWantUnicode)
	if want == "" {
		t.Fatalf("%s is not set", xtcEnvWantUnicode)
	}
	t.Logf("toolchain %s, unicode.Version %s", runtime.Version(), unicode.Version)
	if unicode.Version != want {
		t.Fatalf("unicode.Version = %s on %s, want %s", unicode.Version, runtime.Version(), want)
	}
}

func TestCrossToolchainBuild(t *testing.T) {
	out := os.Getenv(xtcEnvOut)
	if out == "" {
		t.Skipf("%s is not set; run make cross-toolchain-check", xtcEnvOut)
	}
	requireCrossToolchainUnicode(t)

	for name, idx := range map[string]*GINIndex{
		xtcDriftArtifact:      buildTransformerDriftIndex(t),
		xtcAggregatedArtifact: buildTransformerAggregatedIndex(t),
	} {
		data, err := EncodeWithLevel(idx, CompressionNone)
		if err != nil {
			t.Fatalf("EncodeWithLevel(%s): %v", name, err)
		}
		path := filepath.Join(out, name)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", path, err)
		}
		t.Logf("wrote %d bytes to %s", len(data), path)
	}
}

func xtcDecode(t *testing.T, dir, name string) *GINIndex {
	t.Helper()
	path := filepath.Join(dir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	idx, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode(%s): %v", path, err)
	}
	return idx
}

func TestCrossToolchainQuery(t *testing.T) {
	in := os.Getenv(xtcEnvIn)
	if in == "" {
		t.Skipf("%s is not set; run make cross-toolchain-check", xtcEnvIn)
	}
	done := os.Getenv(xtcEnvDone)
	if done == "" {
		t.Fatalf("%s is not set", xtcEnvDone)
	}
	requireCrossToolchainUnicode(t)

	requireTransformerDriftQueries(t, xtcDecode(t, in, xtcDriftArtifact), "exchanged")
	requireTransformerAggregatedQueries(t, xtcDecode(t, in, xtcAggregatedArtifact), "exchanged")
	if t.Failed() {
		return
	}
	if err := os.WriteFile(done, []byte(runtime.Version()+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", done, err)
	}
}
