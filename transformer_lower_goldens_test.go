//go:build regenerate_goldens

package gin

import (
	"os"
	"path/filepath"
	"testing"
	"unicode"
)

func TestRegenerateTransformerLowerGoldens(t *testing.T) {
	if unicode.Version != trigramUnicodeVersion {
		t.Fatalf("unicode.Version is %s; regenerate the transformer lowercase goldens with Unicode %s (GOTOOLCHAIN=go1.26.9)", unicode.Version, trigramUnicodeVersion)
	}
	if err := os.MkdirAll(filepath.Dir(transformerLowerOutputsGolden), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(transformerLowerOutputsGolden, formatTransformerLowerOutputs(), 0o644); err != nil {
		t.Fatalf("write %s: %v", transformerLowerOutputsGolden, err)
	}
	if err := os.WriteFile(transformerLowerDriftGolden, encodeTransformerLowerDriftIndex(t), 0o644); err != nil {
		t.Fatalf("write %s: %v", transformerLowerDriftGolden, err)
	}
}
