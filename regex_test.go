package gin

import (
	"testing"
)

func TestExtractLiterals(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{
			name:     "simple literal",
			pattern:  "hello",
			expected: []string{"hello"},
		},
		{
			name:     "alternation with common suffix",
			pattern:  "foo|bar|baz",
			expected: []string{"foo", "ba"}, // Simplify() factors bar|baz → ba[rz]
		},
		{
			name:     "literal with wildcard",
			pattern:  "error.*warning",
			expected: []string{"error", "warning"},
		},
		{
			name:     "prefix pattern",
			pattern:  "ERROR_[0-9]+",
			expected: []string{"ERROR_"},
		},
		{
			name:     "suffix pattern",
			pattern:  "[a-z]+_suffix",
			expected: []string{"_suffix"},
		},
		{
			name:     "complex alternation",
			pattern:  "Toyota|Tesla|Ford",
			expected: []string{"Toyota", "Tesla", "Ford"}, // T factored: T(oyota|esla)|Ford
		},
		{
			name:     "grouped alternation",
			pattern:  "(error|warn|info)_message",
			expected: []string{"error_message", "warn_message", "info_message"}, // Combined literals
		},
		{
			name:     "anchored pattern",
			pattern:  "^start.*end$",
			expected: []string{"start", "end"},
		},
		{
			name:     "optional suffix",
			pattern:  "test(ing)?",
			expected: []string{"test"}, // ? is optional, can't rely on "ing"
		},
		{
			name:     "repeated literal",
			pattern:  "ab+c",
			expected: []string{"abc"}, // + requires at least one b
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			literals, err := ExtractLiterals(tt.pattern)
			if err != nil {
				t.Fatalf("ExtractLiterals(%q) error: %v", tt.pattern, err)
			}

			if len(literals) != len(tt.expected) {
				t.Errorf("ExtractLiterals(%q) = %v, want %v", tt.pattern, literals, tt.expected)
				return
			}

			for i, lit := range literals {
				if lit != tt.expected[i] {
					t.Errorf("ExtractLiterals(%q)[%d] = %q, want %q", tt.pattern, i, lit, tt.expected[i])
				}
			}
		})
	}
}

func TestAnalyzeRegex(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		hasWildcard bool
		minLength   int
	}{
		{
			name:        "simple literal",
			pattern:     "hello",
			hasWildcard: false,
			minLength:   5,
		},
		{
			name:        "unbounded wildcard",
			pattern:     "foo.*bar",
			hasWildcard: true,
			minLength:   3,
		},
		{
			name:        "bounded repetition",
			pattern:     "a{2,5}",
			hasWildcard: false,
			minLength:   2, // {2,5} means min 2 repetitions → "aa"
		},
		{
			name:        "alternation with different lengths",
			pattern:     "ab|cdef|g",
			hasWildcard: false,
			minLength:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := AnalyzeRegex(tt.pattern)
			if err != nil {
				t.Fatalf("AnalyzeRegex(%q) error: %v", tt.pattern, err)
			}

			if info.HasWildcard != tt.hasWildcard {
				t.Errorf("AnalyzeRegex(%q).HasWildcard = %v, want %v", tt.pattern, info.HasWildcard, tt.hasWildcard)
			}

			if info.MinLength != tt.minLength {
				t.Errorf("AnalyzeRegex(%q).MinLength = %d, want %d", tt.pattern, info.MinLength, tt.minLength)
			}
		})
	}
}

func TestRegexQueryOperator(t *testing.T) {
	config := DefaultConfig()
	config.EnableTrigrams = true

	builder, err := NewBuilder(config, 5)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	// Row group 0: Toyota content
	builder.AddDocument(0, []byte(`{"brand": "Toyota Corolla is reliable"}`))

	// Row group 1: Tesla content
	builder.AddDocument(1, []byte(`{"brand": "Tesla Model 3 is electric"}`))

	// Row group 2: Ford content
	builder.AddDocument(2, []byte(`{"brand": "Ford Mustang is powerful"}`))

	// Row group 3: Mixed content
	builder.AddDocument(3, []byte(`{"brand": "Both Toyota and Tesla are popular"}`))

	// Row group 4: Unrelated content
	builder.AddDocument(4, []byte(`{"brand": "Honda Civic is economical"}`))

	idx := builder.Finalize()

	tests := []struct {
		name     string
		pattern  string
		expected []int
	}{
		{
			name:     "single literal regex",
			pattern:  "Toyota",
			expected: []int{0, 3},
		},
		{
			name:     "alternation regex",
			pattern:  "Toyota|Tesla",
			expected: []int{0, 1, 3},
		},
		{
			name:     "three-way alternation",
			pattern:  "Toyota|Tesla|Ford",
			expected: []int{0, 1, 2, 3},
		},
		{
			name:     "prefix pattern",
			pattern:  "Model.*electric",
			expected: []int{1},
		},
		{
			name:     "no match pattern",
			pattern:  "BMW|Mercedes",
			expected: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := idx.Evaluate([]Predicate{Regex("$.brand", tt.pattern)})
			got := result.ToSlice()

			if len(got) != len(tt.expected) {
				t.Errorf("Regex(%q) = %v, want %v", tt.pattern, got, tt.expected)
				return
			}

			for i, v := range got {
				if v != tt.expected[i] {
					t.Errorf("Regex(%q)[%d] = %d, want %d", tt.pattern, i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestRegexWithShortLiterals(t *testing.T) {
	config := DefaultConfig()
	config.EnableTrigrams = true

	builder, err := NewBuilder(config, 3)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	builder.AddDocument(0, []byte(`{"code": "AB123"}`))
	builder.AddDocument(1, []byte(`{"code": "CD456"}`))
	builder.AddDocument(2, []byte(`{"code": "EF789"}`))

	idx := builder.Finalize()

	// Pattern with literals shorter than trigram length should return all RGs
	result := idx.Evaluate([]Predicate{Regex("$.code", "AB|CD")})
	got := result.ToSlice()

	// "AB" and "CD" are only 2 chars, too short for trigrams
	// Should return all row groups (can't prune)
	if len(got) != 3 {
		t.Errorf("Regex with short literals should return all RGs, got %v", got)
	}

	// Pattern with longer literals should work
	result = idx.Evaluate([]Predicate{Regex("$.code", "AB123|CD456")})
	got = result.ToSlice()

	if len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Errorf("Regex with long literals = %v, want [0, 1]", got)
	}
}

func TestRegexCombinedWithOtherPredicates(t *testing.T) {
	config := DefaultConfig()
	config.EnableTrigrams = true

	builder, err := NewBuilder(config, 4)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	builder.AddDocument(0, []byte(`{"message": "ERROR: Connection failed", "level": "error"}`))
	builder.AddDocument(1, []byte(`{"message": "WARNING: Low memory", "level": "warning"}`))
	builder.AddDocument(2, []byte(`{"message": "ERROR: Timeout occurred", "level": "error"}`))
	builder.AddDocument(3, []byte(`{"message": "INFO: System started", "level": "info"}`))

	idx := builder.Finalize()

	// Combine regex with equality predicate
	result := idx.Evaluate([]Predicate{
		Regex("$.message", "ERROR|WARNING"),
		EQ("$.level", "error"),
	})
	got := result.ToSlice()

	// Should match RGs 0 and 2 (ERROR messages with level=error)
	if len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Errorf("Combined regex+EQ = %v, want [0, 2]", got)
	}
}
