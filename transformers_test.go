package gin

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func requirePathID(t *testing.T, idx *GINIndex, path string) uint16 {
	t.Helper()
	pathID, ok := idx.pathLookup[path]
	if !ok {
		t.Fatalf("pathLookup[%q] missing", path)
	}
	return pathID
}

func mustRoundTripIndex(t *testing.T, idx *GINIndex) *GINIndex {
	t.Helper()
	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	return decoded
}

func requirePredicateResult(t *testing.T, idx *GINIndex, predicates []Predicate, want []int, label string) {
	t.Helper()
	if got := idx.Evaluate(predicates).ToSlice(); !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
}

func TestISODateToEpochMs(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantOk  bool
		wantVal float64
	}{
		{
			name:    "valid RFC3339",
			input:   "2024-01-15T10:30:00Z",
			wantOk:  true,
			wantVal: float64(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC).UnixMilli()),
		},
		{
			name:    "valid RFC3339 with timezone",
			input:   "2024-01-15T10:30:00+05:00",
			wantOk:  true,
			wantVal: float64(time.Date(2024, 1, 15, 5, 30, 0, 0, time.UTC).UnixMilli()),
		},
		{
			name:    "valid RFC3339Nano",
			input:   "2024-01-15T10:30:00.123456789Z",
			wantOk:  true,
			wantVal: float64(time.Date(2024, 1, 15, 10, 30, 0, 123456789, time.UTC).UnixMilli()),
		},
		{
			name:   "invalid date string",
			input:  "not-a-date",
			wantOk: false,
		},
		{
			name:   "non-string input",
			input:  12345,
			wantOk: false,
		},
		{
			name:   "nil input",
			input:  nil,
			wantOk: false,
		},
		{
			name:   "wrong date format",
			input:  "2024-01-15",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ISODateToEpochMs(tt.input)
			if ok != tt.wantOk {
				t.Errorf("ISODateToEpochMs() ok = %v, want %v", ok, tt.wantOk)
				return
			}
			if ok && result.(float64) != tt.wantVal {
				t.Errorf("ISODateToEpochMs() = %v, want %v", result, tt.wantVal)
			}
		})
	}
}

func TestDateToEpochMs(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantOk  bool
		wantVal float64
	}{
		{
			name:    "valid date",
			input:   "2024-01-15",
			wantOk:  true,
			wantVal: float64(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC).UnixMilli()),
		},
		{
			name:   "invalid date string",
			input:  "not-a-date",
			wantOk: false,
		},
		{
			name:   "ISO format not supported",
			input:  "2024-01-15T10:30:00Z",
			wantOk: false,
		},
		{
			name:   "non-string input",
			input:  12345,
			wantOk: false,
		},
		{
			name:   "nil input",
			input:  nil,
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := DateToEpochMs(tt.input)
			if ok != tt.wantOk {
				t.Errorf("DateToEpochMs() ok = %v, want %v", ok, tt.wantOk)
				return
			}
			if ok && result.(float64) != tt.wantVal {
				t.Errorf("DateToEpochMs() = %v, want %v", result, tt.wantVal)
			}
		})
	}
}

func TestCustomDateToEpochMs(t *testing.T) {
	tests := []struct {
		name    string
		layout  string
		input   any
		wantOk  bool
		wantVal float64
	}{
		{
			name:    "custom layout YYYY/MM/DD",
			layout:  "2006/01/02",
			input:   "2024/01/15",
			wantOk:  true,
			wantVal: float64(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC).UnixMilli()),
		},
		{
			name:    "custom layout with time",
			layout:  "2006/01/02 15:04",
			input:   "2024/01/15 10:30",
			wantOk:  true,
			wantVal: float64(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC).UnixMilli()),
		},
		{
			name:   "wrong format for layout",
			layout: "2006/01/02",
			input:  "2024-01-15",
			wantOk: false,
		},
		{
			name:   "non-string input",
			layout: "2006/01/02",
			input:  12345,
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer := CustomDateToEpochMs(tt.layout)
			result, ok := transformer(tt.input)
			if ok != tt.wantOk {
				t.Errorf("CustomDateToEpochMs(%q)() ok = %v, want %v", tt.layout, ok, tt.wantOk)
				return
			}
			if ok && result.(float64) != tt.wantVal {
				t.Errorf("CustomDateToEpochMs(%q)() = %v, want %v", tt.layout, result, tt.wantVal)
			}
		})
	}
}

func TestDateTransformerIntegration(t *testing.T) {
	config, err := NewConfig(
		WithISODateTransformer("$.created_at", "epoch_ms"),
		WithDateTransformer("$.birth_date", "epoch_ms"),
	)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	builder, err := NewBuilder(config, 3)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"name": "alice", "created_at": "2024-01-15T10:30:00Z", "birth_date": "1990-05-20"}`},
		{1, `{"name": "bob", "created_at": "2024-01-16T14:00:00Z", "birth_date": "1985-03-10"}`},
		{2, `{"name": "charlie", "created_at": "2024-01-17T09:15:00Z", "birth_date": "1995-12-01"}`},
	}

	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
	}

	idx := builder.Finalize()
	if got := idx.Evaluate([]Predicate{EQ("$.created_at", "2024-01-15T10:30:00Z")}).ToSlice(); len(got) != 1 || got[0] != 0 {
		t.Fatalf(`EQ("$.created_at", raw) = %v, want [0]`, got)
	}

	createdAtRawPathID := requirePathID(t, idx, "$.created_at")
	if idx.PathDirectory[createdAtRawPathID].ObservedTypes&TypeString == 0 {
		t.Fatalf("$.created_at raw path should keep string type, got %v", idx.PathDirectory[createdAtRawPathID].ObservedTypes)
	}

	birthDateRawPathID := requirePathID(t, idx, "$.birth_date")
	if idx.PathDirectory[birthDateRawPathID].ObservedTypes&TypeString == 0 {
		t.Fatalf("$.birth_date raw path should keep string type, got %v", idx.PathDirectory[birthDateRawPathID].ObservedTypes)
	}

	createdAtPathID := requirePathID(t, idx, "__derived:$.created_at#epoch_ms")
	createdAtIndex, ok := idx.NumericIndexes[createdAtPathID]
	if !ok {
		t.Fatal(`__derived:$.created_at#epoch_ms should have a NumericIndex`)
	}

	expectedMin := float64(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC).UnixMilli())
	expectedMax := float64(time.Date(2024, 1, 17, 9, 15, 0, 0, time.UTC).UnixMilli())

	if createdAtIndex.GlobalMin != expectedMin {
		t.Errorf("$.created_at GlobalMin = %v, want %v", createdAtIndex.GlobalMin, expectedMin)
	}
	if createdAtIndex.GlobalMax != expectedMax {
		t.Errorf("$.created_at GlobalMax = %v, want %v", createdAtIndex.GlobalMax, expectedMax)
	}

	birthDatePathID := requirePathID(t, idx, "__derived:$.birth_date#epoch_ms")
	birthDateIndex, ok := idx.NumericIndexes[birthDatePathID]
	if !ok {
		t.Fatal(`__derived:$.birth_date#epoch_ms should have a NumericIndex`)
	}

	expectedBirthMin := float64(time.Date(1985, 3, 10, 0, 0, 0, 0, time.UTC).UnixMilli())
	expectedBirthMax := float64(time.Date(1995, 12, 1, 0, 0, 0, 0, time.UTC).UnixMilli())

	if birthDateIndex.GlobalMin != expectedBirthMin {
		t.Errorf("$.birth_date GlobalMin = %v, want %v", birthDateIndex.GlobalMin, expectedBirthMin)
	}
	if birthDateIndex.GlobalMax != expectedBirthMax {
		t.Errorf("$.birth_date GlobalMax = %v, want %v", birthDateIndex.GlobalMax, expectedBirthMax)
	}
}

func TestDateTransformerRangeQuery(t *testing.T) {
	config, err := NewConfig(
		WithISODateTransformer("$.timestamp", "epoch_ms"),
	)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	builder, err := NewBuilder(config, 3)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"id": 1, "timestamp": "2024-01-01T00:00:00Z"}`},
		{1, `{"id": 2, "timestamp": "2024-06-15T12:00:00Z"}`},
		{2, `{"id": 3, "timestamp": "2024-12-31T23:59:59Z"}`},
	}

	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
	}

	idx := builder.Finalize()

	midYear := float64(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC).UnixMilli())
	pathID := requirePathID(t, idx, "__derived:$.timestamp#epoch_ms")
	result := idx.evaluateGT(int(pathID), midYear)

	if result.Count() != 1 {
		t.Errorf("expected 1 match for timestamp > mid-2024, got %d", result.Count())
	}
	if !result.IsSet(2) {
		t.Error("expected doc at position 2 (Dec 31) to match")
	}

	startYear := float64(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli())
	result = idx.evaluateGTE(int(pathID), startYear)

	if result.Count() != 3 {
		t.Errorf("expected 3 matches for timestamp >= start of 2024, got %d", result.Count())
	}
}

func TestDateTransformerCanonicalConfigPath(t *testing.T) {
	config, err := NewConfig(
		WithISODateTransformer("$['created_at']", "epoch_ms"),
	)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	builder, err := NewBuilder(config, 3)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"created_at": "2024-01-15T10:30:00Z"}`},
		{1, `{"created_at": "2024-01-16T14:00:00Z"}`},
		{2, `{"created_at": "2024-01-17T09:15:00Z"}`},
	}

	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
	}

	idx := builder.Finalize()

	rawPathID := requirePathID(t, idx, "$.created_at")
	if idx.PathDirectory[rawPathID].ObservedTypes&TypeString == 0 {
		t.Fatalf("$.created_at should keep string type after companion staging, got %v", idx.PathDirectory[rawPathID].ObservedTypes)
	}

	threshold := float64(time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC).UnixMilli())
	derivedPathID := requirePathID(t, idx, "__derived:$.created_at#epoch_ms")
	result := idx.evaluateGTE(int(derivedPathID), threshold)
	if got := result.ToSlice(); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("derived GTE(created_at, threshold) = %v, want [1 2]", got)
	}
}

func TestDateTransformerDecodeCanonicalQueries(t *testing.T) {
	config, err := NewConfig(
		WithISODateTransformer("$['timestamp']", "epoch_ms"),
	)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	builder, err := NewBuilder(config, 3)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"timestamp": "2024-01-01T00:00:00Z"}`},
		{1, `{"timestamp": "2024-06-15T12:00:00Z"}`},
		{2, `{"timestamp": "2024-12-31T23:59:59Z"}`},
	}
	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
	}

	idx := builder.Finalize()
	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	threshold := float64(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC).UnixMilli())
	canonical := decoded.Evaluate([]Predicate{EQ("$.timestamp", "2024-12-31T23:59:59Z")}).ToSlice()
	bracket := decoded.Evaluate([]Predicate{EQ("$['timestamp']", "2024-12-31T23:59:59Z")}).ToSlice()

	if len(canonical) != 1 || canonical[0] != 2 {
		t.Fatalf("EQ($.timestamp, raw) = %v, want [2]", canonical)
	}
	if len(bracket) != len(canonical) || bracket[0] != canonical[0] {
		t.Fatalf("EQ($['timestamp'], raw) = %v, want %v", bracket, canonical)
	}

	derivedPathID := requirePathID(t, decoded, "__derived:$.timestamp#epoch_ms")
	derived := decoded.evaluateGTE(int(derivedPathID), threshold).ToSlice()
	if len(derived) != 1 || derived[0] != 2 {
		t.Fatalf("decoded derived GTE(timestamp, threshold) = %v, want [2]", derived)
	}
}

func TestWildcardSubtreeTransformerNormalizesNestedNumbers(t *testing.T) {
	config, err := NewConfig(
		WithCustomTransformer("$.items[*].metrics", "summary", func(value any) (any, bool) {
			metrics, ok := value.(map[string]any)
			if !ok {
				return nil, false
			}

			count, ok := metrics["count"].(float64)
			if !ok {
				return nil, false
			}

			ratio, ok := metrics["ratio"].(float64)
			if !ok {
				return nil, false
			}

			return count + ratio, true
		}),
	)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	builder, err := NewBuilder(config, 2)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"items":[{"metrics":{"count":10,"ratio":0.25}}]}`},
		{1, `{"items":[{"metrics":{"count":20,"ratio":0.75}}]}`},
	}

	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument(%d) failed: %v", doc.docID, err)
		}
	}

	idx := builder.Finalize()
	if _, ok := idx.pathLookup["$.items[*].metrics.count"]; !ok {
		t.Fatal(`pathLookup["$.items[*].metrics.count"] missing`)
	}
	summaryPathID := requirePathID(t, idx, "__derived:$.items[*].metrics#summary")
	got := idx.evaluateGTE(int(summaryPathID), 20.5).ToSlice()
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("derived GTE($.items[*].metrics#summary, 20.5) = %v, want [1]", got)
	}
}

func TestBuilderFailsWhenCompanionTransformFails(t *testing.T) {
	config, err := NewConfig(
		WithCustomTransformer("$.email", "strict", func(value any) (any, bool) {
			s, ok := value.(string)
			if !ok || !strings.Contains(s, "@") {
				return nil, false
			}
			return s, true
		}),
	)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	builder, err := NewBuilder(config, 2)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	err = builder.AddDocument(0, []byte(`{"email":42}`))
	if err == nil {
		t.Fatal("AddDocument(0) error = nil, want strict companion failure")
	}
	if !strings.Contains(err.Error(), "$.email") || !strings.Contains(err.Error(), "strict") {
		t.Fatalf("AddDocument(0) error = %v, want source path and alias context", err)
	}

	if err := builder.AddDocument(1, []byte(`{"email":"bob@example.com"}`)); err != nil {
		t.Fatalf("AddDocument(1) failed after rejected document: %v", err)
	}

	idx := builder.Finalize()
	if idx.Header.NumDocs != 1 {
		t.Fatalf("Header.NumDocs = %d, want 1", idx.Header.NumDocs)
	}

	rawPathID := requirePathID(t, idx, "$.email")
	if _, ok := idx.NumericIndexes[rawPathID]; ok {
		t.Fatal(`NumericIndexes["$.email"] present, want rejected raw numeric value to be absent`)
	}
	rawIndex, ok := idx.StringIndexes[rawPathID]
	if !ok {
		t.Fatal(`StringIndexes["$.email"] missing`)
	}
	if got := rawIndex.Terms; len(got) != 1 || got[0] != "bob@example.com" {
		t.Fatalf(`raw terms = %v, want ["bob@example.com"]`, got)
	}

	if got := idx.Evaluate([]Predicate{EQ("$.email", "bob@example.com")}).ToSlice(); len(got) != 1 || got[0] != 0 {
		t.Fatalf(`EQ("$.email", "bob@example.com") = %v, want [0]`, got)
	}

	if got := idx.Evaluate([]Predicate{EQ("$.email", As("strict", "bob@example.com"))}).ToSlice(); len(got) != 1 || got[0] != 0 {
		t.Fatalf(`EQ("$.email", As("strict", "bob@example.com")) = %v, want [0]`, got)
	}
}

func TestBuilderSoftFailSkipsDocumentWhenConfigured(t *testing.T) {
	config, err := NewConfig(
		WithCustomTransformer("$.email", "strict", func(value any) (any, bool) {
			s, ok := value.(string)
			if !ok || !strings.Contains(s, "@") {
				return nil, false
			}
			return s, true
		}, WithTransformerFailureMode(IngestFailureSoft)),
	)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	builder, err := NewBuilder(config, 2)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	if err := builder.AddDocument(0, []byte(`{"email":42}`)); err != nil {
		t.Fatalf("AddDocument(0) error = %v, want soft-fail success", err)
	}
	if builder.numDocs != 0 {
		t.Fatalf("numDocs = %d, want 0 after soft-skipped document", builder.numDocs)
	}
	if builder.nextPos != 0 {
		t.Fatalf("nextPos = %d, want 0 after soft-skipped document", builder.nextPos)
	}
	if err := builder.AddDocument(1, []byte(`{"email":"bob@example.com"}`)); err != nil {
		t.Fatalf("AddDocument(1) failed: %v", err)
	}

	idx := builder.Finalize()
	if idx.Header.NumDocs != 1 {
		t.Fatalf("Header.NumDocs = %d, want 1", idx.Header.NumDocs)
	}

	rawPathID := requirePathID(t, idx, "$.email")
	if _, ok := idx.NumericIndexes[rawPathID]; ok {
		t.Fatal(`NumericIndexes["$.email"] present, want soft-skipped raw numeric value absent`)
	}

	if got := idx.Evaluate([]Predicate{EQ("$.email", As("strict", "bob@example.com"))}).ToSlice(); len(got) != 1 || got[0] != 0 {
		t.Fatalf(`EQ("$.email", As("strict", "bob@example.com")) = %v, want [0]`, got)
	}
}

func TestSiblingTransformersDoNotChain(t *testing.T) {
	config, err := NewConfig(
		WithToLowerTransformer("$.email", "lower"),
		WithCustomTransformer("$.email", "probe", func(value any) (any, bool) {
			s, ok := value.(string)
			if !ok {
				return nil, false
			}
			if s == "alice@example.com" {
				return "chained", true
			}
			return "raw", true
		}),
	)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	builder, err := NewBuilder(config, 1)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	if err := builder.AddDocument(0, []byte(`{"email":"Alice@Example.COM"}`)); err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}

	idx := builder.Finalize()

	probePathID, ok := idx.pathLookup["__derived:$.email#probe"]
	if !ok {
		t.Fatal(`pathLookup["__derived:$.email#probe"] missing`)
	}
	probeIndex, ok := idx.StringIndexes[probePathID]
	if !ok {
		t.Fatal(`StringIndexes["__derived:$.email#probe"] missing`)
	}
	if got := probeIndex.Terms; len(got) != 1 || got[0] != "raw" {
		t.Fatalf(`probe terms = %v, want ["raw"]`, got)
	}
}

func TestToLower(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantOk  bool
		wantVal string
	}{
		{"lowercase string", "hello", true, "hello"},
		{"uppercase string", "HELLO", true, "hello"},
		{"mixed case", "HeLLo WoRLd", true, "hello world"},
		{"email address", "Alice@Example.COM", true, "alice@example.com"},
		{"non-string input", 12345, false, ""},
		{"nil input", nil, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ToLower(tt.input)
			if ok != tt.wantOk {
				t.Errorf("ToLower() ok = %v, want %v", ok, tt.wantOk)
				return
			}
			if ok && result.(string) != tt.wantVal {
				t.Errorf("ToLower() = %v, want %v", result, tt.wantVal)
			}
		})
	}
}

func TestIPv4ToInt(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantOk  bool
		wantVal float64
	}{
		{"valid IPv4", "192.168.1.1", true, 3232235777},
		{"localhost", "127.0.0.1", true, 2130706433},
		{"min address", "0.0.0.0", true, 0},
		{"max address", "255.255.255.255", true, 4294967295},
		{"subnet start", "192.168.0.0", true, 3232235520},
		{"subnet end", "192.168.255.255", true, 3232301055},
		{"invalid IP", "999.999.999.999", false, 0},
		{"IPv6 address", "::1", false, 0},
		{"not an IP", "not-an-ip", false, 0},
		{"non-string input", 12345, false, 0},
		{"nil input", nil, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := IPv4ToInt(tt.input)
			if ok != tt.wantOk {
				t.Errorf("IPv4ToInt() ok = %v, want %v", ok, tt.wantOk)
				return
			}
			if ok && result.(float64) != tt.wantVal {
				t.Errorf("IPv4ToInt() = %v, want %v", result, tt.wantVal)
			}
		})
	}
}

func TestSemVerToInt(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantOk  bool
		wantVal float64
	}{
		{"simple version", "1.2.3", true, 1002003},
		{"with v prefix", "v2.1.3", true, 2001003},
		{"major only", "v1.0.0", true, 1000000},
		{"two parts", "1.2", true, 1002000},
		{"with v prefix two parts", "v3.5", true, 3005000},
		{"with prerelease", "1.2.3-beta", true, 1002003},
		{"with prerelease v prefix", "v2.0.0-rc1", true, 2000000},
		{"max supported", "999.999.999", true, 999999999},
		{"invalid - single part", "1", false, 0},
		{"invalid - four parts", "1.2.3.4", false, 0},
		{"invalid - major too large", "1000.0.0", false, 0},
		{"invalid - minor too large", "1.1000.0", false, 0},
		{"invalid - patch too large", "1.0.1000", false, 0},
		{"invalid - negative", "-1.0.0", false, 0},
		{"invalid - not a number", "a.b.c", false, 0},
		{"non-string input", 12345, false, 0},
		{"nil input", nil, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := SemVerToInt(tt.input)
			if ok != tt.wantOk {
				t.Errorf("SemVerToInt() ok = %v, want %v", ok, tt.wantOk)
				return
			}
			if ok && result.(float64) != tt.wantVal {
				t.Errorf("SemVerToInt() = %v, want %v", result, tt.wantVal)
			}
		})
	}
}

func TestRegexExtract(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		group   int
		input   any
		wantOk  bool
		wantVal string
	}{
		{"extract error code", `ERROR\[(\w+)\]:`, 1, "ERROR[E1234]: Connection failed", true, "E1234"},
		{"extract api version", `/api/(v\d+)/`, 1, "/api/v2/users/123", true, "v2"},
		{"extract request id", `req-(\w+)-`, 1, "req-abc123-def456", true, "abc123"},
		{"full match group 0", `ERROR\[(\w+)\]:`, 0, "ERROR[E1234]: Connection failed", true, "ERROR[E1234]:"},
		{"no match", `ERROR\[(\w+)\]:`, 1, "INFO: Everything is fine", false, ""},
		{"group out of range", `ERROR\[(\w+)\]:`, 2, "ERROR[E1234]: Connection failed", false, ""},
		{"non-string input", `\w+`, 0, 12345, false, ""},
		{"nil input", `\w+`, 0, nil, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer := RegexExtract(tt.pattern, tt.group)
			result, ok := transformer(tt.input)
			if ok != tt.wantOk {
				t.Errorf("RegexExtract(%q, %d)() ok = %v, want %v", tt.pattern, tt.group, ok, tt.wantOk)
				return
			}
			if ok && result.(string) != tt.wantVal {
				t.Errorf("RegexExtract(%q, %d)() = %v, want %v", tt.pattern, tt.group, result, tt.wantVal)
			}
		})
	}
}

func TestRegexExtractInt(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		group   int
		input   any
		wantOk  bool
		wantVal float64
	}{
		{"extract order number", `order-(\d+)`, 1, "order-12345", true, 12345},
		{"extract year", `(\d{4})-\d{2}-\d{2}`, 1, "2024-01-15", true, 2024},
		{"extract float", `price: (\d+\.?\d*)`, 1, "price: 99.99", true, 99.99},
		{"no match", `order-(\d+)`, 1, "item-abc", false, 0},
		{"not a number", `id-(\w+)`, 1, "id-abc", false, 0},
		{"non-string input", `\d+`, 0, 12345, false, 0},
		{"nil input", `\d+`, 0, nil, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer := RegexExtractInt(tt.pattern, tt.group)
			result, ok := transformer(tt.input)
			if ok != tt.wantOk {
				t.Errorf("RegexExtractInt(%q, %d)() ok = %v, want %v", tt.pattern, tt.group, ok, tt.wantOk)
				return
			}
			if ok && result.(float64) != tt.wantVal {
				t.Errorf("RegexExtractInt(%q, %d)() = %v, want %v", tt.pattern, tt.group, result, tt.wantVal)
			}
		})
	}
}

func TestDurationToMs(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantOk  bool
		wantVal float64
	}{
		{"hours and minutes", "1h30m", true, 5400000},
		{"milliseconds", "500ms", true, 500},
		{"seconds", "30s", true, 30000},
		{"minutes", "5m", true, 300000},
		{"hours", "2h", true, 7200000},
		{"mixed", "1h30m45s", true, 5445000},
		{"microseconds", "1500us", true, 1},
		{"nanoseconds", "1000000ns", true, 1},
		{"invalid duration", "not-a-duration", false, 0},
		{"non-string input", 12345, false, 0},
		{"nil input", nil, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := DurationToMs(tt.input)
			if ok != tt.wantOk {
				t.Errorf("DurationToMs() ok = %v, want %v", ok, tt.wantOk)
				return
			}
			if ok && result.(float64) != tt.wantVal {
				t.Errorf("DurationToMs() = %v, want %v", result, tt.wantVal)
			}
		})
	}
}

func TestEmailDomain(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantOk  bool
		wantVal string
	}{
		{"simple email", "user@example.com", true, "example.com"},
		{"uppercase domain", "Alice@Example.COM", true, "example.com"},
		{"subdomain", "user@mail.example.com", true, "mail.example.com"},
		{"no at sign", "not-an-email", false, ""},
		{"multiple at signs", "user@domain@example.com", false, ""},
		{"empty domain", "user@", false, ""},
		{"empty local", "@example.com", true, "example.com"},
		{"non-string input", 12345, false, ""},
		{"nil input", nil, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := EmailDomain(tt.input)
			if ok != tt.wantOk {
				t.Errorf("EmailDomain() ok = %v, want %v", ok, tt.wantOk)
				return
			}
			if ok && result.(string) != tt.wantVal {
				t.Errorf("EmailDomain() = %v, want %v", result, tt.wantVal)
			}
		})
	}
}

func TestURLHost(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantOk  bool
		wantVal string
	}{
		{"simple URL", "https://example.com/path", true, "example.com"},
		{"URL with port", "https://example.com:8080/path", true, "example.com:8080"},
		{"uppercase host", "https://EXAMPLE.COM/path", true, "example.com"},
		{"with subdomain", "https://api.example.com/v1", true, "api.example.com"},
		{"http scheme", "http://localhost:3000", true, "localhost:3000"},
		{"no scheme", "/path/to/resource", false, ""},
		{"empty string", "", false, ""},
		{"non-string input", 12345, false, ""},
		{"nil input", nil, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := URLHost(tt.input)
			if ok != tt.wantOk {
				t.Errorf("URLHost() ok = %v, want %v", ok, tt.wantOk)
				return
			}
			if ok && result.(string) != tt.wantVal {
				t.Errorf("URLHost() = %v, want %v", result, tt.wantVal)
			}
		})
	}
}

func TestNumericBucket(t *testing.T) {
	tests := []struct {
		name    string
		size    float64
		input   any
		wantOk  bool
		wantVal float64
	}{
		{"bucket by 100 - 150", 100, 150.0, true, 100},
		{"bucket by 100 - 250", 100, 250.0, true, 200},
		{"bucket by 100 - exact", 100, 200.0, true, 200},
		{"bucket by 100 - small", 100, 50.0, true, 0},
		{"bucket by 10", 10, 45.0, true, 40},
		{"bucket by 0.1", 0.1, 0.45, true, 0.4},
		{"negative value", 100, -150.0, true, -200},
		{"non-float input", 100, 150, false, 0},
		{"string input", 100, "150", false, 0},
		{"nil input", 100, nil, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer := NumericBucket(tt.size)
			result, ok := transformer(tt.input)
			if ok != tt.wantOk {
				t.Errorf("NumericBucket(%v)() ok = %v, want %v", tt.size, ok, tt.wantOk)
				return
			}
			if ok && math.Abs(result.(float64)-tt.wantVal) > 0.0001 {
				t.Errorf("NumericBucket(%v)() = %v, want %v", tt.size, result, tt.wantVal)
			}
		})
	}
}

func TestBoolNormalize(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantOk  bool
		wantVal bool
	}{
		{"bool true", true, true, true},
		{"bool false", false, true, false},
		{"string true", "true", true, true},
		{"string TRUE", "TRUE", true, true},
		{"string false", "false", true, false},
		{"string FALSE", "FALSE", true, false},
		{"string yes", "yes", true, true},
		{"string YES", "YES", true, true},
		{"string no", "no", true, false},
		{"string 1", "1", true, true},
		{"string 0", "0", true, false},
		{"string on", "on", true, true},
		{"string off", "off", true, false},
		{"float64 non-zero", 1.0, true, true},
		{"float64 zero", 0.0, true, false},
		{"float64 negative", -1.0, true, true},
		{"invalid string", "maybe", false, false},
		{"int input", 1, false, false},
		{"nil input", nil, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := BoolNormalize(tt.input)
			if ok != tt.wantOk {
				t.Errorf("BoolNormalize() ok = %v, want %v", ok, tt.wantOk)
				return
			}
			if ok && result.(bool) != tt.wantVal {
				t.Errorf("BoolNormalize() = %v, want %v", result, tt.wantVal)
			}
		})
	}
}

func TestIPv4ToIntRangeQuery(t *testing.T) {
	config, err := NewConfig(
		WithIPv4Transformer("$.client_ip", "ipv4_int"),
	)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	builder, err := NewBuilder(config, 3)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"client_ip": "192.168.1.1"}`},
		{1, `{"client_ip": "10.0.0.1"}`},
		{2, `{"client_ip": "192.168.1.100"}`},
	}

	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
	}

	idx := builder.Finalize()

	// Query: Find IPs in 192.168.0.0/16 range
	start := float64(3232235520) // 192.168.0.0
	end := float64(3232301055)   // 192.168.255.255
	pathID := requirePathID(t, idx, "__derived:$.client_ip#ipv4_int")
	result := idx.evaluateGTE(int(pathID), start).Intersect(idx.evaluateLTE(int(pathID), end))

	if result.Count() != 2 {
		t.Errorf("expected 2 matches in 192.168.x.x range, got %d", result.Count())
	}
	if !result.IsSet(0) || !result.IsSet(2) {
		t.Error("expected docs 0 and 2 to match (192.168.1.x addresses)")
	}
	if result.IsSet(1) {
		t.Error("doc 1 (10.0.0.1) should not match")
	}
}

func TestSemVerToIntRangeQuery(t *testing.T) {
	config, err := NewConfig(
		WithSemVerTransformer("$.version", "semver_int"),
	)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	builder, err := NewBuilder(config, 3)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"name": "app-a", "version": "1.5.0"}`},
		{1, `{"name": "app-b", "version": "v2.1.3"}`},
		{2, `{"name": "app-c", "version": "2.0.0-beta"}`},
	}

	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
	}

	idx := builder.Finalize()

	// Query: Find versions >= 2.0.0
	pathID := requirePathID(t, idx, "__derived:$.version#semver_int")
	result := idx.evaluateGTE(int(pathID), float64(2000000))

	if result.Count() != 2 {
		t.Errorf("expected 2 matches for version >= 2.0.0, got %d", result.Count())
	}
	if !result.IsSet(1) || !result.IsSet(2) {
		t.Error("expected docs 1 and 2 to match (versions >= 2.0.0)")
	}
}

func TestToLowerIntegration(t *testing.T) {
	config, err := NewConfig(
		WithToLowerTransformer("$.email", "lower"),
	)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	builder, err := NewBuilder(config, 3)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"email": "Alice@Example.COM"}`},
		{1, `{"email": "bob@example.com"}`},
		{2, `{"email": "CHARLIE@EXAMPLE.COM"}`},
	}

	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
	}

	idx := builder.Finalize()

	if got := idx.Evaluate([]Predicate{
		{Path: "$.email", Operator: OpEQ, Value: "Alice@Example.COM"},
	}).ToSlice(); len(got) != 1 || got[0] != 0 {
		t.Fatalf(`EQ("$.email", raw) = %v, want [0]`, got)
	}

	pathID := requirePathID(t, idx, "__derived:$.email#lower")
	entry := &idx.PathDirectory[pathID]
	result := idx.evaluateEQ(int(pathID), entry, "alice@example.com")

	if result.Count() != 1 {
		t.Errorf("expected 1 match for lower(email)=alice@example.com, got %d", result.Count())
	}
	if !result.IsSet(0) {
		t.Error("expected doc 0 to match")
	}
}

func TestCIDRToRange(t *testing.T) {
	tests := []struct {
		name      string
		cidr      string
		wantStart float64
		wantEnd   float64
		wantErr   bool
	}{
		{
			name:      "/24 subnet",
			cidr:      "192.168.1.0/24",
			wantStart: 3232235776, // 192.168.1.0
			wantEnd:   3232236031, // 192.168.1.255
			wantErr:   false,
		},
		{
			name:      "/16 subnet",
			cidr:      "192.168.0.0/16",
			wantStart: 3232235520, // 192.168.0.0
			wantEnd:   3232301055, // 192.168.255.255
			wantErr:   false,
		},
		{
			name:      "/8 subnet",
			cidr:      "10.0.0.0/8",
			wantStart: 167772160, // 10.0.0.0
			wantEnd:   184549375, // 10.255.255.255
			wantErr:   false,
		},
		{
			name:      "/32 single host",
			cidr:      "192.168.1.100/32",
			wantStart: 3232235876, // 192.168.1.100
			wantEnd:   3232235876, // 192.168.1.100
			wantErr:   false,
		},
		{
			name:      "/0 all IPs",
			cidr:      "0.0.0.0/0",
			wantStart: 0,
			wantEnd:   4294967295,
			wantErr:   false,
		},
		{
			name:    "invalid CIDR",
			cidr:    "not-a-cidr",
			wantErr: true,
		},
		{
			name:    "missing prefix",
			cidr:    "192.168.1.0",
			wantErr: true,
		},
		{
			name:    "IPv6 not supported",
			cidr:    "2001:db8::/32",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := CIDRToRange(tt.cidr)
			if (err != nil) != tt.wantErr {
				t.Errorf("CIDRToRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if start != tt.wantStart {
					t.Errorf("CIDRToRange() start = %v, want %v", start, tt.wantStart)
				}
				if end != tt.wantEnd {
					t.Errorf("CIDRToRange() end = %v, want %v", end, tt.wantEnd)
				}
			}
		})
	}
}

func TestInSubnet(t *testing.T) {
	predicates := InSubnet("$.client_ip", "192.168.1.0/24")

	if len(predicates) != 2 {
		t.Fatalf("InSubnet() returned %d predicates, want 2", len(predicates))
	}

	if predicates[0].Operator != OpGTE {
		t.Errorf("first predicate operator = %v, want OpGTE", predicates[0].Operator)
	}
	if predicates[1].Operator != OpLTE {
		t.Errorf("second predicate operator = %v, want OpLTE", predicates[1].Operator)
	}
	if predicates[0].Path != "$.client_ip" || predicates[1].Path != "$.client_ip" {
		t.Error("predicates should have path $.client_ip")
	}
	start, ok := predicates[0].Value.(RepresentationValue)
	if !ok {
		t.Fatalf("start value type = %T, want RepresentationValue", predicates[0].Value)
	}
	if start.Alias != "ipv4_int" || start.Value != float64(3232235776) {
		t.Errorf("start value = %#v, want alias ipv4_int and 3232235776", start)
	}

	end, ok := predicates[1].Value.(RepresentationValue)
	if !ok {
		t.Fatalf("end value type = %T, want RepresentationValue", predicates[1].Value)
	}
	if end.Alias != "ipv4_int" || end.Value != float64(3232236031) {
		t.Errorf("end value = %#v, want alias ipv4_int and 3232236031", end)
	}
}

func TestInSubnetPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("InSubnet with invalid CIDR should panic")
		}
	}()
	InSubnet("$.ip", "invalid-cidr")
}

func TestInSubnetIntegration(t *testing.T) {
	config, err := NewConfig(
		WithIPv4Transformer("$.client_ip", "ipv4_int"),
	)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	builder, err := NewBuilder(config, 4)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"client_ip": "192.168.1.10"}`},  // in 192.168.1.0/24
		{1, `{"client_ip": "192.168.2.10"}`},  // NOT in 192.168.1.0/24
		{2, `{"client_ip": "192.168.1.200"}`}, // in 192.168.1.0/24
		{3, `{"client_ip": "10.0.0.1"}`},      // NOT in 192.168.1.0/24
	}

	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
	}

	idx := builder.Finalize()

	// Use InSubnet helper
	result := idx.Evaluate(InSubnet("$.client_ip", "192.168.1.0/24"))

	if result.Count() != 2 {
		t.Errorf("expected 2 matches in 192.168.1.0/24, got %d", result.Count())
	}
	if !result.IsSet(0) || !result.IsSet(2) {
		t.Error("expected docs 0 and 2 to match")
	}
	if result.IsSet(1) || result.IsSet(3) {
		t.Error("docs 1 and 3 should not match")
	}

	// Test /16 subnet
	result = idx.Evaluate(InSubnet("$.client_ip", "192.168.0.0/16"))
	if result.Count() != 3 {
		t.Errorf("expected 3 matches in 192.168.0.0/16, got %d", result.Count())
	}
}

func TestTransformerNumericPathExplicitParserCompatibility(t *testing.T) {
	config, err := NewConfig(
		WithCustomTransformer("$['metrics']", "total", func(value any) (any, bool) {
			metrics, ok := value.(map[string]any)
			if !ok {
				return nil, false
			}
			cpu, ok := metrics["cpu"].(float64)
			if !ok {
				return nil, false
			}
			return cpu * 100, true
		}),
	)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	builder, err := NewBuilder(config, 2)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"metrics":{"cpu":1.5,"cores":4}}`},
		{1, `{"metrics":{"cpu":2.5,"cores":8}}`},
	}

	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
	}

	idx := builder.Finalize()

	if _, exists := idx.pathLookup["$.metrics.cpu"]; !exists {
		t.Fatal("raw subtree staging should index child paths for transformed objects")
	}

	if got := idx.Evaluate([]Predicate{EQ("$.metrics.cpu", float64(1.5))}).ToSlice(); len(got) != 1 || got[0] != 0 {
		t.Fatalf("EQ($.metrics.cpu, 1.5) = %v, want [0]", got)
	}

	derivedPathID := requirePathID(t, idx, "__derived:$.metrics#total")
	derivedEntry := &idx.PathDirectory[derivedPathID]
	result := idx.evaluateEQ(int(derivedPathID), derivedEntry, float64(150))
	if !result.IsSet(0) || result.IsSet(1) {
		t.Fatalf("derived EQ($.metrics#total, 150) = %v, want [0]", result.ToSlice())
	}
}

func TestTransformerNumericDecodeParity(t *testing.T) {
	config, err := NewConfig(
		WithRegexExtractIntTransformer("$.build_id", "build_number", `build-(\d+)`, 1),
	)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	builder, err := NewBuilder(config, 2)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"build_id":"build-9007199254740990"}`},
		{1, `{"build_id":"build-9007199254740991"}`},
	}

	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
	}

	idx := builder.Finalize()
	derivedPathID := requirePathID(t, idx, "__derived:$.build_id#build_number")
	derivedIndex, ok := idx.NumericIndexes[derivedPathID]
	if !ok {
		t.Fatal(`NumericIndexes["__derived:$.build_id#build_number"] missing`)
	}
	before := idx.evaluateIntOnlyEQ(derivedIndex, int(idx.Header.NumRowGroups), 9007199254740991)
	if before.Count() != 1 || !before.IsSet(1) || before.IsSet(0) {
		t.Fatalf("pre-encode derived EQ($.build_id#build_number, 9007199254740991) = %v, want [1]", before.ToSlice())
	}

	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	derivedPathID = requirePathID(t, decoded, "__derived:$.build_id#build_number")
	derivedIndex, ok = decoded.NumericIndexes[derivedPathID]
	if !ok {
		t.Fatal(`decoded NumericIndexes["__derived:$.build_id#build_number"] missing`)
	}
	after := decoded.evaluateIntOnlyEQ(derivedIndex, int(decoded.Header.NumRowGroups), 9007199254740991)
	if after.Count() != 1 || !after.IsSet(1) || after.IsSet(0) {
		t.Fatalf("decoded derived EQ($.build_id#build_number, 9007199254740991) = %v, want [1]", after.ToSlice())
	}
}

func TestDateTransformerAliasCoverage(t *testing.T) {
	config, err := NewConfig(
		WithISODateTransformer("$.created_at", "epoch_ms"),
	)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	builder, err := NewBuilder(config, 3)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"created_at":"2024-01-15T10:30:00Z"}`},
		{1, `{"created_at":"2024-07-10T09:00:00Z"}`},
		{2, `{"created_at":"2024-09-01T08:00:00Z"}`},
	}

	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
	}

	before := builder.Finalize()
	after := mustRoundTripIndex(t, before)

	july2024 := float64(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC).UnixMilli())
	cases := []struct {
		label      string
		predicates []Predicate
		want       []int
	}{
		{
			label:      `raw EQ("$.created_at", "2024-07-10T09:00:00Z")`,
			predicates: []Predicate{EQ("$.created_at", "2024-07-10T09:00:00Z")},
			want:       []int{1},
		},
		{
			label:      `alias GTE("$.created_at", As("epoch_ms", july2024))`,
			predicates: []Predicate{GTE("$.created_at", As("epoch_ms", july2024))},
			want:       []int{1, 2},
		},
	}

	for _, tc := range cases {
		requirePredicateResult(t, before, tc.predicates, tc.want, "before "+tc.label)
		requirePredicateResult(t, after, tc.predicates, tc.want, "after "+tc.label)
	}
}

func TestNormalizedTextAliasCoverage(t *testing.T) {
	config, err := NewConfig(
		WithToLowerTransformer("$.email", "lower"),
		WithEmailDomainTransformer("$.email", "domain"),
	)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	builder, err := NewBuilder(config, 3)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"email":"Alice@Example.COM"}`},
		{1, `{"email":"bob@company.io"}`},
		{2, `{"email":"CHARLIE@EXAMPLE.COM"}`},
	}

	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
	}

	before := builder.Finalize()
	after := mustRoundTripIndex(t, before)

	cases := []struct {
		label      string
		predicates []Predicate
		want       []int
	}{
		{
			label:      `raw EQ("$.email", "Alice@Example.COM")`,
			predicates: []Predicate{EQ("$.email", "Alice@Example.COM")},
			want:       []int{0},
		},
		{
			label:      `raw EQ("$.email", "alice@example.com")`,
			predicates: []Predicate{EQ("$.email", "alice@example.com")},
			want:       []int{},
		},
		{
			label:      `alias EQ("$.email", As("lower", "alice@example.com"))`,
			predicates: []Predicate{EQ("$.email", As("lower", "alice@example.com"))},
			want:       []int{0},
		},
		{
			label:      `alias EQ("$.email", As("domain", "example.com"))`,
			predicates: []Predicate{EQ("$.email", As("domain", "example.com"))},
			want:       []int{0, 2},
		},
	}

	for _, tc := range cases {
		requirePredicateResult(t, before, tc.predicates, tc.want, "before "+tc.label)
		requirePredicateResult(t, after, tc.predicates, tc.want, "after "+tc.label)
	}
}

func TestRegexExtractAliasCoverage(t *testing.T) {
	config, err := NewConfig(
		WithRegexExtractTransformer("$.message", "error_code", `ERROR\[(\w+)\]:`, 1),
		WithRegexExtractIntTransformer("$.order_id", "order_number", `order-(\d+)`, 1),
	)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	builder, err := NewBuilder(config, 3)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"message":"ERROR[E1001]: Connection timeout","order_id":"order-100"}`},
		{1, `{"message":"ERROR[W2002]: Retry scheduled","order_id":"order-250"}`},
		{2, `{"message":"ERROR[E1001]: Disk full","order_id":"order-400"}`},
	}

	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
	}

	before := builder.Finalize()
	after := mustRoundTripIndex(t, before)

	cases := []struct {
		label      string
		predicates []Predicate
		want       []int
	}{
		{
			label:      `raw EQ("$.message", "ERROR[E1001]: Connection timeout")`,
			predicates: []Predicate{EQ("$.message", "ERROR[E1001]: Connection timeout")},
			want:       []int{0},
		},
		{
			label:      `alias EQ("$.message", As("error_code", "E1001"))`,
			predicates: []Predicate{EQ("$.message", As("error_code", "E1001"))},
			want:       []int{0, 2},
		},
		{
			label:      `alias GTE("$.order_id", As("order_number", 200.0))`,
			predicates: []Predicate{GTE("$.order_id", As("order_number", float64(200)))},
			want:       []int{1, 2},
		},
	}

	for _, tc := range cases {
		requirePredicateResult(t, before, tc.predicates, tc.want, "before "+tc.label)
		requirePredicateResult(t, after, tc.predicates, tc.want, "after "+tc.label)
	}
}
