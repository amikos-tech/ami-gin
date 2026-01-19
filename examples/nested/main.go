// Example: Nested JSON objects and arrays
package main

import (
	"fmt"
	"os"

	"github.com/pkg/errors"

	gin "github.com/amikos-tech/gin-index"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	builder, err := gin.NewBuilder(gin.DefaultConfig(), 4)
	if err != nil {
		return errors.Wrap(err, "create builder")
	}

	// Row group 0: User with nested address
	builder.AddDocument(0, []byte(`{
		"user": {
			"name": "alice",
			"address": {
				"city": "New York",
				"country": "USA"
			}
		},
		"tags": ["admin", "developer"]
	}`))

	// Row group 1: User from different city
	builder.AddDocument(1, []byte(`{
		"user": {
			"name": "bob",
			"address": {
				"city": "London",
				"country": "UK"
			}
		},
		"tags": ["developer", "reviewer"]
	}`))

	// Row group 2: User from same country as alice
	builder.AddDocument(2, []byte(`{
		"user": {
			"name": "charlie",
			"address": {
				"city": "Los Angeles",
				"country": "USA"
			}
		},
		"tags": ["tester"]
	}`))

	// Row group 3: Deep nesting
	builder.AddDocument(3, []byte(`{
		"user": {
			"name": "diana",
			"address": {
				"city": "Paris",
				"country": "France"
			}
		},
		"tags": ["admin", "manager"]
	}`))

	idx := builder.Finalize()

	fmt.Println("=== Nested JSON Queries ===")

	// Query nested field
	fmt.Println("\n--- Query: $.user.address.country = 'USA' ---")
	result := idx.Evaluate([]gin.Predicate{
		gin.EQ("$.user.address.country", "USA"),
	})
	fmt.Printf("Row groups: %v\n", result.ToSlice())

	// Query deep nested field
	fmt.Println("\n--- Query: $.user.address.city = 'London' ---")
	result = idx.Evaluate([]gin.Predicate{
		gin.EQ("$.user.address.city", "London"),
	})
	fmt.Printf("Row groups: %v\n", result.ToSlice())

	// Query array element with wildcard
	fmt.Println("\n--- Query: $.tags[*] = 'admin' ---")
	result = idx.Evaluate([]gin.Predicate{
		gin.EQ("$.tags[*]", "admin"),
	})
	fmt.Printf("Row groups: %v\n", result.ToSlice())

	// Query array element - developer
	fmt.Println("\n--- Query: $.tags[*] = 'developer' ---")
	result = idx.Evaluate([]gin.Predicate{
		gin.EQ("$.tags[*]", "developer"),
	})
	fmt.Printf("Row groups: %v\n", result.ToSlice())

	// Combined nested and array query
	fmt.Println("\n--- Query: $.user.address.country = 'USA' AND $.tags[*] = 'admin' ---")
	result = idx.Evaluate([]gin.Predicate{
		gin.EQ("$.user.address.country", "USA"),
		gin.EQ("$.tags[*]", "admin"),
	})
	fmt.Printf("Row groups: %v\n", result.ToSlice())

	// Using IN with array elements
	fmt.Println("\n--- Query: $.tags[*] IN ('admin', 'manager') ---")
	result = idx.Evaluate([]gin.Predicate{
		gin.IN("$.tags[*]", "admin", "manager"),
	})
	fmt.Printf("Row groups: %v\n", result.ToSlice())

	// CONTAINS on nested field
	fmt.Println("\n--- Query: $.user.address.city CONTAINS 'York' ---")
	result = idx.Evaluate([]gin.Predicate{
		gin.Contains("$.user.address.city", "York"),
	})
	fmt.Printf("Row groups: %v\n", result.ToSlice())

	// Show indexed paths
	fmt.Println("\n=== Indexed Paths ===")
	for _, p := range idx.PathDirectory {
		fmt.Printf("  %s (cardinality: %d)\n", p.PathName, p.Cardinality)
	}

	return nil
}
