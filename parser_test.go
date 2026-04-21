package gin

import (
	"testing"
)

func TestWithParserRejectsNil(t *testing.T) {
	opt := WithParser(nil)
	b := &GINBuilder{}
	err := opt(b)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "parser cannot be nil" {
		t.Fatalf("want %q, got %q", "parser cannot be nil", err.Error())
	}
}

func TestStdlibParserName(t *testing.T) {
	if got := (stdlibParser{}).Name(); got != "stdlib" {
		t.Fatalf("stdlibParser.Name() = %q, want %q", got, "stdlib")
	}
}

var (
	_ parserSink = (*GINBuilder)(nil)
	_ Parser     = stdlibParser{}
)

func TestBuilderHasParserFields(t *testing.T) {
	b, err := NewBuilder(DefaultConfig(), 4)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	b.parserName = "xyz"
	b.currentDocState = newDocumentBuildState(0)
	if b.parserName != "xyz" || b.currentDocState == nil {
		t.Fatal("parser fields not writable/readable")
	}
}

func TestShouldBufferForTransformSignalWhenRegistered(t *testing.T) {
	cfg := DefaultConfig()
	if err := WithISODateTransformer("$.created_at", "epoch_ms")(&cfg); err != nil {
		t.Fatalf("WithISODateTransformer: %v", err)
	}
	b, err := NewBuilder(cfg, 4)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	if !b.ShouldBufferForTransform("$.created_at") {
		t.Errorf("expected ShouldBufferForTransform(\"$.created_at\") == true")
	}
	if b.ShouldBufferForTransform("$.other") {
		t.Errorf("expected ShouldBufferForTransform(\"$.other\") == false")
	}
}

func TestBeginDocumentStashesState(t *testing.T) {
	b, err := NewBuilder(DefaultConfig(), 4)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	state := b.BeginDocument(2)
	if state == nil {
		t.Fatal("BeginDocument returned nil")
	}
	if state.rgID != 2 {
		t.Errorf("state.rgID = %d, want 2", state.rgID)
	}
	if b.currentDocState != state {
		t.Error("b.currentDocState not stashed to returned state")
	}
}
