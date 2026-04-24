package gin

import (
	"encoding/json"
	"testing"
	"time"
)

func countRegisteredRepresentations(cfg *GINConfig) int {
	if cfg == nil {
		return 0
	}
	total := 0
	for _, specs := range cfg.representationSpecs {
		total += len(specs)
	}
	return total
}

func requireRepresentationSpec(t *testing.T, cfg *GINConfig, path, alias string) RepresentationSpec {
	t.Helper()
	specs := cfg.representationSpecs[path]
	for _, spec := range specs {
		if spec.Alias == alias {
			return spec
		}
	}
	t.Fatalf("expected representation spec for %s alias %q, got %v", path, alias, specs)
	return RepresentationSpec{}
}

func requireRegisteredRepresentation(t *testing.T, cfg *GINConfig, path, alias string) registeredRepresentation {
	t.Helper()
	registrations := cfg.representationTransformers[path]
	for _, registration := range registrations {
		if registration.Alias == alias {
			return registration
		}
	}
	t.Fatalf("expected registered representation for %s alias %q, got %v", path, alias, registrations)
	return registeredRepresentation{}
}

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
		WithISODateTransformer("$.timestamp", "epoch_ms"),
		WithToLowerTransformer("$.email", "lower"),
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
	if countRegisteredRepresentations(decoded.Config) != 2 {
		t.Errorf("registered representations = %d, want 2", countRegisteredRepresentations(decoded.Config))
	}
}

func TestConfigSerializationCanonicalPaths(t *testing.T) {
	cfg, err := NewConfig(
		WithISODateTransformer("$['timestamp']", "epoch_ms"),
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

	spec := requireRepresentationSpec(t, decoded.Config, "$.timestamp", "epoch_ms")
	if spec.SourcePath != "$.timestamp" {
		t.Fatalf("decoded transformer spec source path = %q, want %.12s", spec.SourcePath, "$.timestamp")
	}
	if spec.TargetPath != "__derived:$.timestamp#epoch_ms" {
		t.Fatalf("decoded transformer spec target path = %q, want %q", spec.TargetPath, "__derived:$.timestamp#epoch_ms")
	}
}

func TestConfigSerializationCanonicalQueryBehavior(t *testing.T) {
	cfg, err := NewConfig(
		WithISODateTransformer("$['timestamp']", "epoch_ms"),
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

	canonicalContains := decoded.Evaluate([]Predicate{Contains("$.description", "test")}).ToSlice()
	bracketContains := decoded.Evaluate([]Predicate{Contains("$['description']", "test")}).ToSlice()
	if len(canonicalContains) != 2 || canonicalContains[0] != 0 || canonicalContains[1] != 1 {
		t.Fatalf("Contains($.description, test) = %v, want [0 1]", canonicalContains)
	}
	if len(bracketContains) != len(canonicalContains) || bracketContains[0] != canonicalContains[0] || bracketContains[1] != canonicalContains[1] {
		t.Fatalf("Contains($['description'], test) = %v, want %v", bracketContains, canonicalContains)
	}
}

func TestConfigSerializationNumericTransformerPath(t *testing.T) {
	cfg, err := NewConfig(WithSemVerTransformer("$.version", "semver_int"))
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	builder, err := NewBuilder(cfg, 2)
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}

	docs := []string{
		`{"version":"v2.1.3"}`,
		`{"version":"v2.1.4"}`,
	}
	for i, doc := range docs {
		if err := builder.AddDocument(DocID(i), []byte(doc)); err != nil {
			t.Fatalf("AddDocument() error = %v", err)
		}
	}

	encoded, err := Encode(builder.Finalize())
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	registration := requireRegisteredRepresentation(t, decoded.Config, "$.version", "semver_int")
	transformed, ok := registration.FieldTransformer("v2.1.3")
	if !ok {
		t.Fatal("decoded semver transformer rejected valid input")
	}
	if transformed.(float64) != 2001003 {
		t.Fatalf("decoded semver transformer = %v, want 2001003", transformed)
	}

	pathID := requirePathID(t, decoded, "__derived:$.version#semver_int")
	entry := &decoded.PathDirectory[pathID]
	result := decoded.evaluateEQ(int(pathID), entry, float64(2001003))
	if !result.IsSet(0) || result.IsSet(1) {
		t.Fatalf("decoded derived transformer query result = %v, want [0]", result.ToSlice())
	}
}

func TestTransformerRoundTrip(t *testing.T) {
	cfg, err := NewConfig(
		WithISODateTransformer("$.created_at", "epoch_ms"),
		WithRegexExtractTransformer("$.log", "error_code", `ERROR\[(\w+)\]:`, 1),
		WithNumericBucketTransformer("$.price", "bucket_10", 10),
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

	if countRegisteredRepresentations(decoded.Config) != 3 {
		t.Errorf("registered representations = %d, want 3", countRegisteredRepresentations(decoded.Config))
	}

	spec := requireRepresentationSpec(t, decoded.Config, "$.created_at", "epoch_ms")
	if spec.Transformer.ID != TransformerISODateToEpochMs {
		t.Errorf("$.created_at transformer ID = %d, want %d", spec.Transformer.ID, TransformerISODateToEpochMs)
	}

	spec = requireRepresentationSpec(t, decoded.Config, "$.log", "error_code")
	if spec.Transformer.ID != TransformerRegexExtract {
		t.Errorf("$.log transformer ID = %d, want %d", spec.Transformer.ID, TransformerRegexExtract)
	}

	// Test that reconstructed transformer works
	registration := requireRegisteredRepresentation(t, decoded.Config, "$.created_at", "epoch_ms")
	result, ok := registration.FieldTransformer("2024-01-15T10:30:00Z")
	if !ok {
		t.Error("reconstructed transformer failed")
	}
	if result != float64(1705314600000) {
		t.Errorf("transformer result = %v, want %v", result, float64(1705314600000))
	}

	// Test regex transformer
	registration = requireRegisteredRepresentation(t, decoded.Config, "$.log", "error_code")
	result, ok = registration.FieldTransformer("ERROR[AUTH]: invalid")
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

func TestRejectsNegativeRegexGroup(t *testing.T) {
	tests := []struct {
		name string
		id   TransformerID
	}{
		{name: "RegexExtract", id: TransformerRegexExtract},
		{name: "RegexExtractInt", id: TransformerRegexExtractInt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReconstructTransformer(tt.id, json.RawMessage(`{"pattern":"(\\w+)","group":-1}`))
			if err == nil {
				t.Fatal("expected error for negative regex group")
			}
			if err.Error() != "regex group must be non-negative" {
				t.Fatalf("unexpected error = %v", err)
			}
		})
	}
}

func TestReconstructedRegexExtractIntRejectsMissingDigits(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty capture", input: "value:"},
		{name: "sign only capture", input: "value:-"},
		{name: "dot only capture", input: "value:."},
	}

	fn, err := ReconstructTransformer(
		TransformerRegexExtractInt,
		json.RawMessage(`{"pattern":"value:([-.0-9]*)","group":1}`),
	)
	if err != nil {
		t.Fatalf("ReconstructTransformer() error = %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, ok := fn(tt.input); ok {
				t.Fatalf("transformer() = %v, want failure", got)
			}
		})
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
