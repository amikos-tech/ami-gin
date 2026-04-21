package gin

import (
	"strings"
	"testing"

	"github.com/pkg/errors"
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

func TestNewBuilderDefaultsToStdlibParser(t *testing.T) {
	b, err := NewBuilder(DefaultConfig(), 4)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	if b.parser == nil {
		t.Fatal("b.parser is nil; expected stdlibParser{}")
	}
	if b.parserName != "stdlib" {
		t.Errorf("b.parserName = %q, want %q", b.parserName, "stdlib")
	}
	if _, ok := b.parser.(stdlibParser); !ok {
		t.Errorf("b.parser concrete type = %T, want stdlibParser", b.parser)
	}
}

type emptyNameParser struct{}

func (emptyNameParser) Name() string                        { return "" }
func (emptyNameParser) Parse([]byte, int, parserSink) error { return nil }

func TestNewBuilderRejectsEmptyParserName(t *testing.T) {
	_, err := NewBuilder(DefaultConfig(), 4, WithParser(emptyNameParser{}))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "parser name cannot be empty" {
		t.Fatalf("want %q, got %q", "parser name cannot be empty", err.Error())
	}
}

func TestBuilderParserNameReachable(t *testing.T) {
	b, err := NewBuilder(DefaultConfig(), 4)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	if b.parserName != "stdlib" {
		t.Errorf("b.parserName = %q, want %q", b.parserName, "stdlib")
	}
}

type namedParser struct{ name string }

func (p namedParser) Name() string                        { return p.name }
func (p namedParser) Parse([]byte, int, parserSink) error { return nil }

func TestWithParserAcceptsCustomParser(t *testing.T) {
	b, err := NewBuilder(DefaultConfig(), 4, WithParser(namedParser{name: "custom-v1"}))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	if b.parserName != "custom-v1" {
		t.Errorf("b.parserName = %q, want %q", b.parserName, "custom-v1")
	}
}

func TestAddDocumentRoundTripsThroughParser(t *testing.T) {
	b, err := NewBuilder(DefaultConfig(), 4)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	if err := b.AddDocument(DocID(0), []byte(`{"a": 1, "b": "hello"}`)); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	idx := b.Finalize()
	got := idx.Evaluate([]Predicate{EQ("$.a", int64(1))}).ToSlice()
	want := []int{0}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("Evaluate got %v, want %v", got, want)
	}
}

type failingParser struct{ err error }

func (p failingParser) Name() string { return "failing" }

func (p failingParser) Parse(_ []byte, rgID int, sink parserSink) error {
	_ = sink.BeginDocument(rgID)
	return p.err
}

func TestAddDocumentReturnsParserErrorVerbatim(t *testing.T) {
	sentinel := errors.New("sentinel parse error")
	b, err := NewBuilder(DefaultConfig(), 4, WithParser(failingParser{err: sentinel}))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	got := b.AddDocument(DocID(0), []byte(`{"a": 1}`))
	if got == nil {
		t.Fatal("expected error from AddDocument, got nil")
	}
	if got.Error() != "sentinel parse error" {
		t.Fatalf("AddDocument err = %q, want %q", got.Error(), "sentinel parse error")
	}
}

func TestAddDocumentDefaultParserErrorStringsPreserved(t *testing.T) {
	b, err := NewBuilder(DefaultConfig(), 4)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	got := b.AddDocument(DocID(0), []byte("garbage"))
	if got == nil {
		t.Fatal("expected error from AddDocument on malformed JSON, got nil")
	}
	if msg := got.Error(); !strings.Contains(msg, "read JSON token") {
		t.Errorf("AddDocument err = %q, want substring %q", msg, "read JSON token")
	}
}

type skipBeginDocumentParser struct{}

func (skipBeginDocumentParser) Name() string { return "skip-begin" }

func (skipBeginDocumentParser) Parse([]byte, int, parserSink) error {
	return nil
}

func TestAddDocumentRejectsParserSkippingBeginDocument(t *testing.T) {
	b, err := NewBuilder(DefaultConfig(), 4, WithParser(skipBeginDocumentParser{}))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	err = b.AddDocument(DocID(0), []byte(`{"a":1}`))
	if err == nil {
		t.Fatal("expected error from AddDocument when parser skips BeginDocument, got nil")
	}
	if !strings.Contains(err.Error(), "did not call BeginDocument") {
		t.Fatalf("want error containing %q, got %q", "did not call BeginDocument", err.Error())
	}
}

type wrongRGIDParser struct{}

func (wrongRGIDParser) Name() string { return "wrong-rgid" }

func (wrongRGIDParser) Parse(_ []byte, rgID int, sink parserSink) error {
	_ = sink.BeginDocument(rgID + 7)
	return nil
}

func TestAddDocumentRejectsBeginDocumentRGIDMismatch(t *testing.T) {
	b, err := NewBuilder(DefaultConfig(), 4, WithParser(wrongRGIDParser{}))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	err = b.AddDocument(DocID(0), []byte(`{"a":1}`))
	if err == nil {
		t.Fatal("expected rgID-mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "BeginDocument rgID mismatch") {
		t.Fatalf("want error containing %q, got %q", "BeginDocument rgID mismatch", err.Error())
	}
}
