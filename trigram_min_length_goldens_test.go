//go:build regenerate_goldens

package gin

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRegenerateTrigramMinLengthGolden writes the old-format fixture. Run it
// only on code that predates the #111 clamp: the file records what such a
// writer stored for TrigramMinLength 5, and later code would index different
// terms.
func TestRegenerateTrigramMinLengthGolden(t *testing.T) {
	if err := os.MkdirAll(filepath.Dir(trigramMinLengthLegacyGolden), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	encoded := buildLegacyTrigramMinLengthEncoded(t, DefaultConfig(), 5, tmlDocs)
	if err := os.WriteFile(trigramMinLengthLegacyGolden, encoded, 0o644); err != nil {
		t.Fatalf("write %s: %v", trigramMinLengthLegacyGolden, err)
	}
}
