//go:build regex_legacy_gen

package gin

// This file is copied into a worktree of the pre-#115 commit 29a3e0c by
// `make regex-unicode-legacy-goldens`. It may only use API present at that
// commit. It writes legacy (unfrozen) regex indexes to AMI_GIN_REGEX_GEN_OUT.

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unicode"
)

var regexLegacyGenDocs = []string{
	`{"name":"ABC` + "Ƛ" + `","ref":"1` + "Ƛ" + `2A","code":"order-7","status":"error"}`,
	`{"name":"ABC","ref":"2A","code":"order-8","status":"ok"}`,
	`{"name":"xyz` + "ƛ" + `","ref":"1B","code":"x","status":"error"}`,
	`{"name":"DEF","ref":"none","code":"order-7","status":"ok"}`,
}

var regexLegacyGenGarayDocs = []string{
	`{"name":"ab` + "\U00010D4A\U00010D4B" + `cd"}`,
	`{"name":"abcd"}`,
	`{"name":"` + "\U00010D4C" + `"}`,
}

func regexLegacyGenWrite(t *testing.T, out, name string, docs []string, opts ...ConfigOption) {
	t.Helper()
	cfg, err := NewConfig(opts...)
	if err != nil {
		t.Fatalf("%s: NewConfig: %v", name, err)
	}
	builder, err := NewBuilder(cfg, len(docs))
	if err != nil {
		t.Fatalf("%s: NewBuilder: %v", name, err)
	}
	for rg, doc := range docs {
		if err := builder.AddDocument(DocID(rg), []byte(doc)); err != nil {
			t.Fatalf("%s: AddDocument(%d): %v", name, rg, err)
		}
	}
	idx := builder.Finalize()
	if int(idx.Header.NumDocs) != len(docs) {
		t.Fatalf("%s: NumDocs = %d, want %d", name, idx.Header.NumDocs, len(docs))
	}
	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("%s: EncodeWithLevel: %v", name, err)
	}
	path := filepath.Join(out, name+"-"+runtime.Version()+".gin")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
	t.Logf("wrote %d bytes to %s (unicode %s)", len(data), path, unicode.Version)
}

func TestRegexLegacyGen(t *testing.T) {
	out := os.Getenv("AMI_GIN_REGEX_GEN_OUT")
	if out == "" {
		t.Skip("AMI_GIN_REGEX_GEN_OUT is not set; run make regex-unicode-legacy-goldens")
	}
	soft := WithTransformerFailureMode(IngestFailureSoft)
	regexLegacyGenWrite(t, out, "legacy-regex", regexLegacyGenDocs,
		WithRegexExtractTransformer("$.name", "upper", `(\p{Lu}+)`, 1, soft),
		WithRegexExtractIntTransformer("$.ref", "n2", `(\d+)\p{Lu}`, 1, soft),
		WithRegexExtractIntTransformer("$.code", "num", `order-(\d+)`, 1, soft),
	)
	regexLegacyGenWrite(t, out, "legacy-safe", regexLegacyGenDocs,
		WithRegexExtractIntTransformer("$.code", "num", `order-(\d+)`, 1, soft),
	)
	if unicode.Version == "17.0.0" {
		regexLegacyGenWrite(t, out, "legacy-garay", regexLegacyGenGarayDocs,
			WithRegexExtractTransformer("$.name", "garay", `(\p{Garay}+)`, 1, soft),
		)
	}
}
