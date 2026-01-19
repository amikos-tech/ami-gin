// Example: Basic GIN index usage with equality queries
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
	// Create a builder for 4 row groups
	builder, err := gin.NewBuilder(gin.DefaultConfig(), 4)
	if err != nil {
		return errors.Wrap(err, "create builder")
	}

	// Add documents to different row groups
	// Row group 0: users from engineering
	builder.AddDocument(0, []byte(`{"name": "alice", "department": "engineering", "level": "senior"}`))
	builder.AddDocument(0, []byte(`{"name": "bob", "department": "engineering", "level": "junior"}`))

	// Row group 1: users from sales
	builder.AddDocument(1, []byte(`{"name": "charlie", "department": "sales", "level": "senior"}`))
	builder.AddDocument(1, []byte(`{"name": "diana", "department": "sales", "level": "manager"}`))

	// Row group 2: users from marketing
	builder.AddDocument(2, []byte(`{"name": "eve", "department": "marketing", "level": "senior"}`))

	// Row group 3: more engineering
	builder.AddDocument(3, []byte(`{"name": "frank", "department": "engineering", "level": "manager"}`))

	// Build the index
	idx := builder.Finalize()

	fmt.Printf("Index built: %d docs, %d row groups, %d paths\n",
		idx.Header.NumDocs, idx.Header.NumRowGroups, idx.Header.NumPaths)

	// Query 1: Find row groups with engineering department
	fmt.Println("\n--- Query: department = 'engineering' ---")
	result := idx.Evaluate([]gin.Predicate{
		gin.EQ("$.department", "engineering"),
	})
	fmt.Printf("Matching row groups: %v\n", result.ToSlice())

	// Query 2: Find row groups with senior level
	fmt.Println("\n--- Query: level = 'senior' ---")
	result = idx.Evaluate([]gin.Predicate{
		gin.EQ("$.level", "senior"),
	})
	fmt.Printf("Matching row groups: %v\n", result.ToSlice())

	// Query 3: Combined query - engineering AND senior
	fmt.Println("\n--- Query: department = 'engineering' AND level = 'senior' ---")
	result = idx.Evaluate([]gin.Predicate{
		gin.EQ("$.department", "engineering"),
		gin.EQ("$.level", "senior"),
	})
	fmt.Printf("Matching row groups: %v\n", result.ToSlice())

	// Query 4: IN query - multiple departments
	fmt.Println("\n--- Query: department IN ('engineering', 'marketing') ---")
	result = idx.Evaluate([]gin.Predicate{
		gin.IN("$.department", "engineering", "marketing"),
	})
	fmt.Printf("Matching row groups: %v\n", result.ToSlice())

	// Query 5: NOT EQUAL
	fmt.Println("\n--- Query: department != 'sales' ---")
	result = idx.Evaluate([]gin.Predicate{
		gin.NE("$.department", "sales"),
	})
	fmt.Printf("Matching row groups: %v\n", result.ToSlice())

	// Validate a path before querying
	if err := gin.ValidateJSONPath("$.department"); err != nil {
		return errors.Wrap(err, "validate path")
	}

	return nil
}
