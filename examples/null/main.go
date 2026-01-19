// Example: NULL handling queries
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

	// Row group 0: Complete records
	builder.AddDocument(0, []byte(`{"name": "alice", "email": "alice@example.com", "phone": "555-1234"}`))
	builder.AddDocument(0, []byte(`{"name": "bob", "email": "bob@example.com", "phone": "555-5678"}`))

	// Row group 1: Missing phone
	builder.AddDocument(1, []byte(`{"name": "charlie", "email": "charlie@example.com", "phone": null}`))
	builder.AddDocument(1, []byte(`{"name": "diana", "email": "diana@example.com"}`)) // phone field absent

	// Row group 2: Missing email
	builder.AddDocument(2, []byte(`{"name": "eve", "email": null, "phone": "555-9999"}`))
	builder.AddDocument(2, []byte(`{"name": "frank", "phone": "555-0000"}`)) // email field absent

	// Row group 3: Multiple nulls
	builder.AddDocument(3, []byte(`{"name": "grace", "email": null, "phone": null}`))
	builder.AddDocument(3, []byte(`{"name": "henry"}`)) // both absent

	idx := builder.Finalize()

	fmt.Println("=== NULL Handling Queries ===")

	// Find records where phone IS NULL
	fmt.Println("\n--- Query: phone IS NULL ---")
	result := idx.Evaluate([]gin.Predicate{gin.IsNull("$.phone")})
	fmt.Printf("Row groups: %v\n", result.ToSlice())

	// Find records where phone IS NOT NULL
	fmt.Println("\n--- Query: phone IS NOT NULL ---")
	result = idx.Evaluate([]gin.Predicate{gin.IsNotNull("$.phone")})
	fmt.Printf("Row groups: %v\n", result.ToSlice())

	// Find records where email IS NULL
	fmt.Println("\n--- Query: email IS NULL ---")
	result = idx.Evaluate([]gin.Predicate{gin.IsNull("$.email")})
	fmt.Printf("Row groups: %v\n", result.ToSlice())

	// Find records where email IS NOT NULL
	fmt.Println("\n--- Query: email IS NOT NULL ---")
	result = idx.Evaluate([]gin.Predicate{gin.IsNotNull("$.email")})
	fmt.Printf("Row groups: %v\n", result.ToSlice())

	// Combined: has email but no phone
	fmt.Println("\n--- Query: email IS NOT NULL AND phone IS NULL ---")
	result = idx.Evaluate([]gin.Predicate{
		gin.IsNotNull("$.email"),
		gin.IsNull("$.phone"),
	})
	fmt.Printf("Row groups: %v\n", result.ToSlice())

	// Find complete records (both email and phone present)
	fmt.Println("\n--- Query: email IS NOT NULL AND phone IS NOT NULL ---")
	result = idx.Evaluate([]gin.Predicate{
		gin.IsNotNull("$.email"),
		gin.IsNotNull("$.phone"),
	})
	fmt.Printf("Row groups: %v\n", result.ToSlice())

	return nil
}
