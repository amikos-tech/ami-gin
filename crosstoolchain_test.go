//go:build crosstoolchain

package gin

import (
	"os"
	"runtime"
	"testing"
	"unicode"
)

// These tests are the two halves of a real artifact exchange between Go
// toolchains with different Unicode tables (#102). `make cross-toolchain-check`
// runs the build half under each toolchain, requires the two artifacts to be
// byte-identical, and runs the query half under each toolchain against each
// artifact. Without the environment variables set, both tests skip.
//
//	AMI_GIN_XTC_WANT_UNICODE  unicode.Version the running toolchain must report
//	AMI_GIN_XTC_OUT           path the build half writes the encoded index to
//	AMI_GIN_XTC_IN            path the query half reads the encoded index from

const (
	xtcEnvWantUnicode = "AMI_GIN_XTC_WANT_UNICODE"
	xtcEnvOut         = "AMI_GIN_XTC_OUT"
	xtcEnvIn          = "AMI_GIN_XTC_IN"
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

	data, err := EncodeWithLevel(buildTransformerDriftIndex(t), CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel: %v", err)
	}
	if err := os.WriteFile(out, data, 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", out, err)
	}
	t.Logf("wrote %d bytes to %s", len(data), out)
}

func TestCrossToolchainQuery(t *testing.T) {
	in := os.Getenv(xtcEnvIn)
	if in == "" {
		t.Skipf("%s is not set; run make cross-toolchain-check", xtcEnvIn)
	}
	requireCrossToolchainUnicode(t)

	data, err := os.ReadFile(in)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", in, err)
	}
	idx, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	requireTransformerDriftQueries(t, idx, "exchanged")
}
