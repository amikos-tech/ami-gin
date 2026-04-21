package gin

import (
	"fmt"
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
	if got := (stdlibParser{}).Name(); got != stdlibParserName {
		t.Fatalf("stdlibParser.Name() = %q, want %q", got, stdlibParserName)
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
	if b.parserName != stdlibParserName {
		t.Errorf("b.parserName = %q, want %q", b.parserName, stdlibParserName)
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
	if b.parserName != stdlibParserName {
		t.Errorf("b.parserName = %q, want %q", b.parserName, stdlibParserName)
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

type recordingSink struct {
	events []string
}

func (s *recordingSink) BeginDocument(rgID int) *documentBuildState {
	s.events = append(s.events, fmt.Sprintf("begin:%d", rgID))
	return newDocumentBuildState(rgID)
}

func (s *recordingSink) StageScalar(_ *documentBuildState, canonicalPath string, _ any) error {
	s.events = append(s.events, "scalar:"+canonicalPath)
	return nil
}

func (s *recordingSink) StageJSONNumber(_ *documentBuildState, canonicalPath, raw string) error {
	s.events = append(s.events, "json-number:"+canonicalPath+"="+raw)
	return nil
}

func (s *recordingSink) StageNativeNumeric(_ *documentBuildState, canonicalPath string, _ any) error {
	s.events = append(s.events, "native-number:"+canonicalPath)
	return nil
}

func (s *recordingSink) StageMaterialized(_ *documentBuildState, path string, _ any, _ bool) error {
	s.events = append(s.events, "materialized:"+path)
	return nil
}

func (s *recordingSink) ShouldBufferForTransform(string) bool { return false }

func TestStdlibParserBeginsDocumentBeforeStaging(t *testing.T) {
	sink := &recordingSink{}

	if err := (stdlibParser{}).Parse([]byte(`{"a":1}`), 3, sink); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []string{"begin:3", "materialized:$.a"}
	if len(sink.events) < len(want) {
		t.Fatalf("events = %v, want prefix %v", sink.events, want)
	}
	for i := range want {
		if sink.events[i] != want[i] {
			t.Fatalf("events[%d] = %q, want %q (full events %v)", i, sink.events[i], want[i], sink.events)
		}
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
	cases := []struct {
		name       string
		jsonDoc    string
		wantSubstr string
	}{
		{name: "garbage", jsonDoc: "garbage", wantSubstr: "read JSON token"},
		{name: "bad-object-value", jsonDoc: `{"a":}`, wantSubstr: "parse object value at $.a"},
		{name: "bad-object-key", jsonDoc: `{1:true}`, wantSubstr: "read object key at $"},
		{name: "unterminated-object", jsonDoc: `{"a":1`, wantSubstr: "close object at $"},
		{name: "unterminated-array", jsonDoc: `[1,`, wantSubstr: "parse array element at $[1]"},
		{name: "trailing-json", jsonDoc: `{"a":1} []`, wantSubstr: "unexpected trailing JSON content"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := NewBuilder(DefaultConfig(), 4)
			if err != nil {
				t.Fatalf("NewBuilder: %v", err)
			}

			got := b.AddDocument(DocID(0), []byte(tc.jsonDoc))
			if got == nil {
				t.Fatalf("expected error from AddDocument(%q), got nil", tc.jsonDoc)
			}
			if msg := got.Error(); !strings.Contains(msg, tc.wantSubstr) {
				t.Fatalf("AddDocument err = %q, want substring %q", msg, tc.wantSubstr)
			}
		})
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

type doubleBeginDocumentParser struct{}

func (doubleBeginDocumentParser) Name() string { return "double-begin" }

func (doubleBeginDocumentParser) Parse(_ []byte, rgID int, sink parserSink) error {
	_ = sink.BeginDocument(rgID)
	_ = sink.BeginDocument(rgID)
	return nil
}

func TestAddDocumentRejectsParserCallingBeginDocumentTwice(t *testing.T) {
	b, err := NewBuilder(DefaultConfig(), 4, WithParser(doubleBeginDocumentParser{}))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	err = b.AddDocument(DocID(0), []byte(`{"a":1}`))
	if err == nil {
		t.Fatal("expected duplicate BeginDocument error, got nil")
	}
	if !strings.Contains(err.Error(), "called BeginDocument 2 times") {
		t.Fatalf("want duplicate BeginDocument error, got %q", err.Error())
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

type fixedRGIDParser struct {
	name string
	rgID int
}

func (p fixedRGIDParser) Name() string { return p.name }

func (p fixedRGIDParser) Parse(_ []byte, _ int, sink parserSink) error {
	_ = sink.BeginDocument(p.rgID)
	return nil
}

func TestAddDocumentRejectsOutOfRangeBeginDocumentRGID(t *testing.T) {
	cases := []struct {
		name string
		rgID int
	}{
		{name: "negative", rgID: -1},
		{name: "past-numrgs", rgID: 9},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := NewBuilder(DefaultConfig(), 4, WithParser(fixedRGIDParser{name: tc.name, rgID: tc.rgID}))
			if err != nil {
				t.Fatalf("NewBuilder: %v", err)
			}

			err = b.AddDocument(DocID(0), []byte(`{"a":1}`))
			if err == nil {
				t.Fatal("expected rgID-mismatch error, got nil")
			}
			if !strings.Contains(err.Error(), "BeginDocument rgID mismatch") {
				t.Fatalf("want mismatch error, got %q", err.Error())
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("got %d, want 0", tc.rgID)) {
				t.Fatalf("want mismatch details for rgID %d, got %q", tc.rgID, err.Error())
			}
		})
	}
}
