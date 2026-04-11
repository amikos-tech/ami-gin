// Example: NULL handling queries
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
		exampleDocument{rgID: 0, body: `{"name": "alice", "email": "alice@example.com", "phone": "555-1234"}`},
		exampleDocument{rgID: 0, body: `{"name": "bob", "email": "bob@example.com", "phone": "555-5678"}`},
		exampleDocument{rgID: 1, body: `{"name": "charlie", "email": "charlie@example.com", "phone": null}`},
		exampleDocument{rgID: 1, body: `{"name": "diana", "email": "diana@example.com"}`},
		exampleDocument{rgID: 2, body: `{"name": "eve", "email": null, "phone": "555-9999"}`},
		exampleDocument{rgID: 2, body: `{"name": "frank", "phone": "555-0000"}`},
		exampleDocument{rgID: 3, body: `{"name": "grace", "email": null, "phone": null}`},
		exampleDocument{rgID: 3, body: `{"name": "henry"}`},
	); err != nil {
		return err
	}

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
