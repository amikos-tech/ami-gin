package main

import (
	"reflect"
	"testing"

	gin "github.com/amikos-tech/ami-gin"
)

func TestParsePredicateSupportedOperators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		predicate gin.Predicate
	}{
		{
			name:      "regex",
			input:     `$.brand REGEX "Toyota|Tesla"`,
			predicate: gin.Predicate{Path: "$.brand", Operator: gin.OpRegex, Value: "Toyota|Tesla"},
		},
		{
			name:      "contains",
			input:     `$.message CONTAINS "warn"`,
			predicate: gin.Predicate{Path: "$.message", Operator: gin.OpContains, Value: "warn"},
		},
		{
			name:      "in list",
			input:     `$.count IN (1, 2, "three")`,
			predicate: gin.Predicate{Path: "$.count", Operator: gin.OpIN, Value: []any{float64(1), float64(2), "three"}},
		},
		{
			name:      "is null",
			input:     `$.deleted_at IS NULL`,
			predicate: gin.IsNull("$.deleted_at"),
		},
		{
			name:      "is not null",
			input:     `$.deleted_at IS NOT NULL`,
			predicate: gin.IsNotNull("$.deleted_at"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parsePredicate(tt.input)
			if err != nil {
				t.Fatalf("parsePredicate(%q) returned error: %v", tt.input, err)
			}

			if got.Path != tt.predicate.Path {
				t.Fatalf("parsePredicate(%q) path = %q, want %q", tt.input, got.Path, tt.predicate.Path)
			}
			if got.Operator != tt.predicate.Operator {
				t.Fatalf("parsePredicate(%q) operator = %v, want %v", tt.input, got.Operator, tt.predicate.Operator)
			}
			if !reflect.DeepEqual(got.Value, tt.predicate.Value) {
				t.Fatalf("parsePredicate(%q) value = %#v, want %#v", tt.input, got.Value, tt.predicate.Value)
			}
		})
	}
}

