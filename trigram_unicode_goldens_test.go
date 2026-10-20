//go:build regenerate_goldens

package gin

import (
	"os"
	"path/filepath"
	"testing"
	"unicode"
)

func TestRegenerateTrigramUnicodeGoldens(t *testing.T) {
	if unicode.Version != trigramUnicodeVersion {
		t.Fatalf("unicode.Version is %s; regenerate the trigram goldens with Unicode %s (GOTOOLCHAIN=go1.26.9)", unicode.Version, trigramUnicodeVersion)
	}
	if err := os.MkdirAll(filepath.Dir(trigramUnicodeGramsGolden), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(trigramUnicodeGramsGolden, formatTrigramUnicodeGrams(t), 0o644); err != nil {
		t.Fatalf("write %s: %v", trigramUnicodeGramsGolden, err)
	}
	if err := os.WriteFile(trigramUnicodeDriftGolden, encodeTrigramUnicodeDriftIndex(t), 0o644); err != nil {
		t.Fatalf("write %s: %v", trigramUnicodeDriftGolden, err)
	}
}

func TestRegenerateTrigramUnicodeStdlibDiff(t *testing.T) {
	if unicode.Version != trigramUnicodeStdlibDiffVersion {
		t.Fatalf("unicode.Version is %s; regenerate the stdlib diff with Unicode %s (Go 1.27)", unicode.Version, trigramUnicodeStdlibDiffVersion)
	}
	if err := os.WriteFile(trigramUnicodeStdlibDiff, formatTrigramUnicodeStdlibDiff(), 0o644); err != nil {
		t.Fatalf("write %s: %v", trigramUnicodeStdlibDiff, err)
	}
}

func TestRegenerateTrigramUnicodeUnknownVersionGolden(t *testing.T) {
	if unicode.Version != trigramUnicodeVersion {
		t.Fatalf("unicode.Version is %s; regenerate the trigram goldens with Unicode %s (GOTOOLCHAIN=go1.26.9)", unicode.Version, trigramUnicodeVersion)
	}
	if err := os.WriteFile(trigramUnicodeUnknownGolden, encodeWithTrigramToken(t, trigramUnicodeForeignToken), 0o644); err != nil {
		t.Fatalf("write %s: %v", trigramUnicodeUnknownGolden, err)
	}
}
