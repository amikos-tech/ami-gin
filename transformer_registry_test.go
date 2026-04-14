package gin

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTransformerReconstruction(t *testing.T) {
	tests := []struct {
		name     string
		id       TransformerID
		params   json.RawMessage
		input    any
		expected any
		wantOk   bool
	}{
		{
			name:     "ISODateToEpochMs",
			id:       TransformerISODateToEpochMs,
			params:   nil,
			input:    "2024-01-15T10:30:00Z",
			expected: float64(1705314600000),
			wantOk:   true,
		},
		{
			name:     "DateToEpochMs",
			id:       TransformerDateToEpochMs,
			params:   nil,
			input:    "2024-01-15",
			expected: float64(1705276800000),
			wantOk:   true,
		},
		{
			name:     "CustomDateToEpochMs",
			id:       TransformerCustomDateToEpochMs,
			params:   json.RawMessage(`{"layout":"2006/01/02"}`),
			input:    "2024/01/15",
			expected: float64(1705276800000),
			wantOk:   true,
		},
		{
			name:     "ToLower",
			id:       TransformerToLower,
			params:   nil,
			input:    "Alice@Example.COM",
			expected: "alice@example.com",
			wantOk:   true,
		},
		{
			name:     "IPv4ToInt",
			id:       TransformerIPv4ToInt,
			params:   nil,
			input:    "192.168.1.1",
			expected: float64(3232235777),
			wantOk:   true,
		},
		{
			name:     "SemVerToInt",
			id:       TransformerSemVerToInt,
			params:   nil,
			input:    "v2.1.3",
			expected: float64(2001003),
			wantOk:   true,
		},
		{
			name:     "RegexExtract",
			id:       TransformerRegexExtract,
			params:   json.RawMessage(`{"pattern":"ERROR\\[(\\w+)\\]:","group":1}`),
			input:    "ERROR[AUTH]: invalid token",
			expected: "AUTH",
			wantOk:   true,
		},
		{
			name:     "RegexExtractInt",
			id:       TransformerRegexExtractInt,
			params:   json.RawMessage(`{"pattern":"order-(\\d+)","group":1}`),
			input:    "order-12345",
			expected: float64(12345),
			wantOk:   true,
		},
		{
			name:     "DurationToMs",
			id:       TransformerDurationToMs,
			params:   nil,
			input:    "1h30m",
			expected: float64(5400000),
			wantOk:   true,
		},
		{
			name:     "EmailDomain",
			id:       TransformerEmailDomain,
			params:   nil,
			input:    "alice@example.com",
			expected: "example.com",
			wantOk:   true,
		},
		{
			name:     "URLHost",
			id:       TransformerURLHost,
			params:   nil,
			input:    "https://api.example.com/v1",
			expected: "api.example.com",
			wantOk:   true,
		},
		{
			name:     "NumericBucket",
			id:       TransformerNumericBucket,
			params:   json.RawMessage(`{"size":100}`),
			input:    float64(150),
			expected: float64(100),
			wantOk:   true,
		},
		{
			name:     "BoolNormalize true",
			id:       TransformerBoolNormalize,
			params:   nil,
			input:    "yes",
			expected: true,
			wantOk:   true,
		},
		{
			name:     "BoolNormalize false",
			id:       TransformerBoolNormalize,
			params:   nil,
			input:    "off",
			expected: false,
			wantOk:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, err := ReconstructTransformer(tt.id, tt.params)
			if err != nil {
				t.Fatalf("ReconstructTransformer() error = %v", err)
			}
			result, ok := fn(tt.input)
			if ok != tt.wantOk {
				t.Errorf("transformer() ok = %v, want %v", ok, tt.wantOk)
			}
			if result != tt.expected {
				t.Errorf("transformer() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRegexCompileTimeout(t *testing.T) {
	_, err := compileRegexWithTimeout("^[a-z]+$", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("compileRegexWithTimeout() error = %v", err)
	}
}

func TestUnknownTransformer(t *testing.T) {
	_, err := ReconstructTransformer(TransformerUnknown, nil)
	if err == nil {
		t.Error("expected error for unknown transformer")
	}
}

func TestConfigSerialization(t *testing.T) {
	cfg, err := NewConfig(
		WithISODateTransformer("$.timestamp"),
		WithToLowerTransformer("$.email"),
		WithFTSPaths("$.description", "$.title"),
	)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	builder, err := NewBuilder(cfg, 10)
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}

	docs := []string{
		`{"timestamp": "2024-01-15T10:30:00Z", "email": "Alice@Example.COM", "description": "test doc", "title": "Hello"}`,
		`{"timestamp": "2024-01-16T12:00:00Z", "email": "Bob@Test.ORG", "description": "another", "title": "World"}`,
	}
	for i, doc := range docs {
		if err := builder.AddDocument(DocID(i), []byte(doc)); err != nil {
			t.Fatalf("AddDocument() error = %v", err)
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

	if decoded.Config == nil {
		t.Fatal("decoded config is nil")
	}

	if decoded.Config.BloomFilterSize != cfg.BloomFilterSize {
		t.Errorf("BloomFilterSize = %d, want %d", decoded.Config.BloomFilterSize, cfg.BloomFilterSize)
	}
	if decoded.Config.EnableTrigrams != cfg.EnableTrigrams {
		t.Errorf("EnableTrigrams = %v, want %v", decoded.Config.EnableTrigrams, cfg.EnableTrigrams)
	}
	if len(decoded.Config.ftsPaths) != 2 {
		t.Errorf("ftsPaths len = %d, want 2", len(decoded.Config.ftsPaths))
	}
	if len(decoded.Config.fieldTransformers) != 2 {
		t.Errorf("fieldTransformers len = %d, want 2", len(decoded.Config.fieldTransformers))
	}
}

func TestConfigSerializationCanonicalPaths(t *testing.T) {
	cfg, err := NewConfig(
		WithISODateTransformer("$['timestamp']"),
		WithFTSPaths("$['description']", `$["title"]`),
	)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	builder, err := NewBuilder(cfg, 2)
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}

	docs := []string{
		`{"timestamp": "2024-01-15T10:30:00Z", "description": "test doc", "title": "Hello"}`,
		`{"timestamp": "2024-01-16T12:00:00Z", "description": "another test", "title": "World"}`,
	}
	for i, doc := range docs {
		if err := builder.AddDocument(DocID(i), []byte(doc)); err != nil {
			t.Fatalf("AddDocument() error = %v", err)
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

	if got, want := decoded.Config.ftsPaths, []string{"$.description", "$.title"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("decoded ftsPaths = %v, want %v", got, want)
	}

	if _, ok := decoded.Config.fieldTransformers["$.timestamp"]; !ok {
		t.Fatalf("expected decoded fieldTransformers to contain canonical key $.timestamp, got %v", decoded.Config.fieldTransformers)
	}

	spec, ok := decoded.Config.transformerSpecs["$.timestamp"]
	if !ok {
		t.Fatalf("expected decoded transformerSpecs to contain canonical key $.timestamp, got %v", decoded.Config.transformerSpecs)
	}
	if spec.Path != "$.timestamp" {
		t.Fatalf("decoded transformer spec path = %q, want %.12s", spec.Path, "$.timestamp")
	}
}

func TestTransformerRoundTrip(t *testing.T) {
	cfg, err := NewConfig(
		WithISODateTransformer("$.created_at"),
		WithRegexExtractTransformer("$.log", `ERROR\[(\w+)\]:`, 1),
		WithNumericBucketTransformer("$.price", 10),
	)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	builder, err := NewBuilder(cfg, 5)
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}

	doc := `{"created_at": "2024-06-15T08:30:00Z", "log": "ERROR[DB]: connection failed", "price": 45.99}`
	if err := builder.AddDocument(DocID(0), []byte(doc)); err != nil {
		t.Fatalf("AddDocument() error = %v", err)
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

	if decoded.Config == nil {
		t.Fatal("decoded config is nil")
	}

	if len(decoded.Config.transformerSpecs) != 3 {
		t.Errorf("transformerSpecs len = %d, want 3", len(decoded.Config.transformerSpecs))
	}

	spec, ok := decoded.Config.transformerSpecs["$.created_at"]
	if !ok {
		t.Error("missing transformer spec for $.created_at")
	} else if spec.ID != TransformerISODateToEpochMs {
		t.Errorf("$.created_at transformer ID = %d, want %d", spec.ID, TransformerISODateToEpochMs)
	}

	spec, ok = decoded.Config.transformerSpecs["$.log"]
	if !ok {
		t.Error("missing transformer spec for $.log")
	} else if spec.ID != TransformerRegexExtract {
		t.Errorf("$.log transformer ID = %d, want %d", spec.ID, TransformerRegexExtract)
	}

	// Test that reconstructed transformer works
	fn := decoded.Config.fieldTransformers["$.created_at"]
	result, ok := fn("2024-01-15T10:30:00Z")
	if !ok {
		t.Error("reconstructed transformer failed")
	}
	if result != float64(1705314600000) {
		t.Errorf("transformer result = %v, want %v", result, float64(1705314600000))
	}

	// Test regex transformer
	fn = decoded.Config.fieldTransformers["$.log"]
	result, ok = fn("ERROR[AUTH]: invalid")
	if !ok {
		t.Error("reconstructed regex transformer failed")
	}
	if result != "AUTH" {
		t.Errorf("regex transformer result = %v, want AUTH", result)
	}
}

func TestConfigSerializationNoConfig(t *testing.T) {
	cfg := DefaultConfig()
	builder, err := NewBuilder(cfg, 5)
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}

	doc := `{"name": "test"}`
	if err := builder.AddDocument(DocID(0), []byte(doc)); err != nil {
		t.Fatalf("AddDocument() error = %v", err)
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

	if decoded.Config == nil {
		t.Fatal("decoded config is nil (should have default config)")
	}

	if decoded.Config.BloomFilterSize != cfg.BloomFilterSize {
		t.Errorf("BloomFilterSize = %d, want %d", decoded.Config.BloomFilterSize, cfg.BloomFilterSize)
	}
}

func TestInvalidRegexParams(t *testing.T) {
	_, err := ReconstructTransformer(TransformerRegexExtract, json.RawMessage(`{"pattern":"[invalid"}`))
	if err == nil {
		t.Error("expected error for invalid regex pattern")
	}
}

func TestMissingCustomDateLayout(t *testing.T) {
	_, err := ReconstructTransformer(TransformerCustomDateToEpochMs, json.RawMessage(`{}`))
	if err == nil {
		t.Error("expected error for missing layout")
	}
}

func TestInvalidNumericBucketSize(t *testing.T) {
	_, err := ReconstructTransformer(TransformerNumericBucket, json.RawMessage(`{"size":0}`))
	if err == nil {
		t.Error("expected error for zero bucket size")
	}

	_, err = ReconstructTransformer(TransformerNumericBucket, json.RawMessage(`{"size":-10}`))
	if err == nil {
		t.Error("expected error for negative bucket size")
	}
}
