// Example: Nested JSON objects and arrays
package main

import (
	"fmt"
	"os"

	"github.com/pkg/errors"

	gin "github.com/amikos-tech/ami-gin"
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

	if err := addDocuments(builder,
		exampleDocument{rgID: 0, body: `{
		"user": {
			"name": "alice",
			"address": {
				"city": "New York",
				"country": "USA"
			}
		},
		"tags": ["admin", "developer"]
	}`},
		exampleDocument{rgID: 1, body: `{
		"user": {
			"name": "bob",
			"address": {
				"city": "London",
				"country": "UK"
			}
		},
		"tags": ["developer", "reviewer"]
	}`},
		exampleDocument{rgID: 2, body: `{
		"user": {
			"name": "charlie",
			"address": {
				"city": "Los Angeles",
				"country": "USA"
			}
		},
		"tags": ["tester"]
	}`},
		exampleDocument{rgID: 3, body: `{
		"user": {
			"name": "diana",
			"address": {
				"city": "Paris",
				"country": "France"
			}
		},
		"tags": ["admin", "manager"]
	}`},
	); err != nil {
		return err
	}

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

type exampleDocument struct {
	rgID gin.DocID
	body string
}

func addDocuments(builder *gin.GINBuilder, docs ...exampleDocument) error {
	for _, doc := range docs {
		if err := builder.AddDocument(doc.rgID, []byte(doc.body)); err != nil {
			return errors.Wrapf(err, "add document to row group %d", doc.rgID)
		}
	}
	return nil
}
