package gin

import (
	"math"
	"testing"
	"time"
)

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
		WithFieldTransformer("$.created_at", ISODateToEpochMs),
		WithFieldTransformer("$.birth_date", DateToEpochMs),
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

	var createdAtPathID uint16 = 0
	var birthDatePathID uint16 = 0
	found := 0
	for _, entry := range idx.PathDirectory {
		if entry.PathName == "$.created_at" {
			createdAtPathID = entry.PathID
			if entry.ObservedTypes&(TypeInt|TypeFloat) == 0 {
				t.Errorf("$.created_at should have numeric type, got %v", entry.ObservedTypes)
			}
			found++
		}
		if entry.PathName == "$.birth_date" {
			birthDatePathID = entry.PathID
			if entry.ObservedTypes&(TypeInt|TypeFloat) == 0 {
				t.Errorf("$.birth_date should have numeric type, got %v", entry.ObservedTypes)
			}
			found++
		}
	}
	if found != 2 {
		t.Fatalf("expected to find both date paths, found %d", found)
	}

	createdAtIndex, ok := idx.NumericIndexes[createdAtPathID]
	if !ok {
		t.Fatal("$.created_at should have a NumericIndex")
	}

	expectedMin := float64(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC).UnixMilli())
	expectedMax := float64(time.Date(2024, 1, 17, 9, 15, 0, 0, time.UTC).UnixMilli())

	if createdAtIndex.GlobalMin != expectedMin {
		t.Errorf("$.created_at GlobalMin = %v, want %v", createdAtIndex.GlobalMin, expectedMin)
	}
	if createdAtIndex.GlobalMax != expectedMax {
		t.Errorf("$.created_at GlobalMax = %v, want %v", createdAtIndex.GlobalMax, expectedMax)
	}

	birthDateIndex, ok := idx.NumericIndexes[birthDatePathID]
	if !ok {
		t.Fatal("$.birth_date should have a NumericIndex")
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
		WithFieldTransformer("$.timestamp", ISODateToEpochMs),
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
	result := idx.Evaluate([]Predicate{{
		Path:     "$.timestamp",
		Operator: OpGT,
		Value:    midYear,
	}})

	if result.Count() != 1 {
		t.Errorf("expected 1 match for timestamp > mid-2024, got %d", result.Count())
	}
	if !result.IsSet(2) {
		t.Error("expected doc at position 2 (Dec 31) to match")
	}

	startYear := float64(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli())
	result = idx.Evaluate([]Predicate{{
		Path:     "$.timestamp",
		Operator: OpGTE,
		Value:    startYear,
	}})

	if result.Count() != 3 {
		t.Errorf("expected 3 matches for timestamp >= start of 2024, got %d", result.Count())
	}
}

func TestDateTransformerCanonicalConfigPath(t *testing.T) {
	config, err := NewConfig(
		WithFieldTransformer("$['created_at']", ISODateToEpochMs),
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

	var pathEntry *PathEntry
	for i := range idx.PathDirectory {
		if idx.PathDirectory[i].PathName == "$.created_at" {
			pathEntry = &idx.PathDirectory[i]
			break
		}
	}
	if pathEntry == nil {
		t.Fatal("expected canonical $.created_at path to be present")
	}
	if pathEntry.ObservedTypes&(TypeInt|TypeFloat) == 0 {
		t.Fatalf("$.created_at should have numeric type after transform, got %v", pathEntry.ObservedTypes)
	}

	threshold := float64(time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC).UnixMilli())
	result := idx.Evaluate([]Predicate{GTE("$.created_at", threshold)})
	if got := result.ToSlice(); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("GTE($.created_at, threshold) = %v, want [1 2]", got)
	}
}

func TestDateTransformerDecodeCanonicalQueries(t *testing.T) {
	config, err := NewConfig(
		WithFieldTransformer("$['timestamp']", ISODateToEpochMs),
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
	canonical := decoded.Evaluate([]Predicate{GTE("$.timestamp", threshold)}).ToSlice()
	bracket := decoded.Evaluate([]Predicate{GTE("$['timestamp']", threshold)}).ToSlice()

	if len(canonical) != 1 || canonical[0] != 2 {
		t.Fatalf("GTE($.timestamp, threshold) = %v, want [2]", canonical)
	}
	if len(bracket) != len(canonical) || bracket[0] != canonical[0] {
		t.Fatalf("GTE($['timestamp'], threshold) = %v, want %v", bracket, canonical)
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
		WithFieldTransformer("$.client_ip", IPv4ToInt),
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
	result := idx.Evaluate([]Predicate{
		{Path: "$.client_ip", Operator: OpGTE, Value: start},
		{Path: "$.client_ip", Operator: OpLTE, Value: end},
	})

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
		WithFieldTransformer("$.version", SemVerToInt),
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
	result := idx.Evaluate([]Predicate{
		{Path: "$.version", Operator: OpGTE, Value: float64(2000000)},
	})

	if result.Count() != 2 {
		t.Errorf("expected 2 matches for version >= 2.0.0, got %d", result.Count())
	}
	if !result.IsSet(1) || !result.IsSet(2) {
		t.Error("expected docs 1 and 2 to match (versions >= 2.0.0)")
	}
}

func TestToLowerIntegration(t *testing.T) {
	config, err := NewConfig(
		WithFieldTransformer("$.email", ToLower),
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

	result := idx.Evaluate([]Predicate{
		{Path: "$.email", Operator: OpEQ, Value: "alice@example.com"},
	})

	if result.Count() != 1 {
		t.Errorf("expected 1 match for alice@example.com, got %d", result.Count())
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
	if predicates[0].Value != float64(3232235776) {
		t.Errorf("start value = %v, want 3232235776", predicates[0].Value)
	}
	if predicates[1].Value != float64(3232236031) {
		t.Errorf("end value = %v, want 3232236031", predicates[1].Value)
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
		WithFieldTransformer("$.client_ip", IPv4ToInt),
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
		WithFieldTransformer("$['metrics']", func(value any) (any, bool) {
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

	if _, exists := idx.pathLookup["$.metrics.cpu"]; exists {
		t.Fatal("transform-before-dispatch should not index child paths for transformed objects")
	}

	result := idx.Evaluate([]Predicate{EQ("$.metrics", float64(150))})
	if !result.IsSet(0) || result.IsSet(1) {
		t.Fatalf("EQ($.metrics, 150) = %v, want [0]", result.ToSlice())
	}
}
