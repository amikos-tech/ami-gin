//go:build regenerate_goldens

package gin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegenerateParityGoldens(t *testing.T) {
	dir := filepath.Join("testdata", "parity-golden")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	for _, fx := range authoredParityFixtures() {
		fx := fx
		t.Run(fx.Name, func(t *testing.T) {
			cfg := fx.Config()
			builder, err := NewBuilder(cfg, fx.NumRGs)
			if err != nil {
				t.Fatalf("NewBuilder for %s: %v", fx.Name, err)
			}
			for i, doc := range fx.JSONDocs {
				if err := builder.AddDocument(DocID(i), doc); err != nil {
					t.Fatalf("AddDocument[%d] for %s: %v", i, fx.Name, err)
				}
			}
			idx := builder.Finalize()
			encoded, err := Encode(idx)
			if err != nil {
				t.Fatalf("Encode for %s: %v", fx.Name, err)
			}
			path := filepath.Join(dir, fx.Name+".bin")
			if err := os.WriteFile(path, encoded, 0o644); err != nil {
				t.Fatalf("write %s: %v", path, err)
			}
			t.Logf("wrote %s (%d bytes)", path, len(encoded))
		})
	}
}
