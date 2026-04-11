// Example: Numeric range queries with GIN index
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
	builder, err := gin.NewBuilder(gin.DefaultConfig(), 5)
	if err != nil {
		return errors.Wrap(err, "create builder")
	}

	if err := addDocuments(builder,
		exampleDocument{rgID: 0, body: `{"name": "widget", "price": 19.99, "stock": 100}`},
		exampleDocument{rgID: 0, body: `{"name": "gadget", "price": 49.99, "stock": 50}`},
		exampleDocument{rgID: 1, body: `{"name": "device", "price": 79.99, "stock": 25}`},
		exampleDocument{rgID: 1, body: `{"name": "tool", "price": 99.99, "stock": 75}`},
		exampleDocument{rgID: 2, body: `{"name": "appliance", "price": 299.99, "stock": 10}`},
		exampleDocument{rgID: 2, body: `{"name": "equipment", "price": 449.99, "stock": 5}`},
		exampleDocument{rgID: 3, body: `{"name": "luxury item", "price": 999.99, "stock": 2}`},
		exampleDocument{rgID: 4, body: `{"name": "basic", "price": 9.99, "stock": 500}`},
		exampleDocument{rgID: 4, body: `{"name": "simple", "price": 14.99, "stock": 300}`},
	); err != nil {
		return err
	}

	idx := builder.Finalize()

	fmt.Println("=== Numeric Range Queries ===")

	// Greater than
	fmt.Println("--- Query: price > 100 ---")
	result := idx.Evaluate([]gin.Predicate{gin.GT("$.price", 100.0)})
	fmt.Printf("Row groups: %v\n\n", result.ToSlice())

	// Greater than or equal
	fmt.Println("--- Query: price >= 50 ---")
	result = idx.Evaluate([]gin.Predicate{gin.GTE("$.price", 50.0)})
	fmt.Printf("Row groups: %v\n\n", result.ToSlice())

	// Less than
	fmt.Println("--- Query: price < 50 ---")
	result = idx.Evaluate([]gin.Predicate{gin.LT("$.price", 50.0)})
	fmt.Printf("Row groups: %v\n\n", result.ToSlice())

	// Less than or equal
	fmt.Println("--- Query: price <= 100 ---")
	result = idx.Evaluate([]gin.Predicate{gin.LTE("$.price", 100.0)})
	fmt.Printf("Row groups: %v\n\n", result.ToSlice())

	// Range query (combined predicates)
	fmt.Println("--- Query: price >= 50 AND price <= 500 ---")
	result = idx.Evaluate([]gin.Predicate{
		gin.GTE("$.price", 50.0),
		gin.LTE("$.price", 500.0),
	})
	fmt.Printf("Row groups: %v\n\n", result.ToSlice())

	// Exact numeric match
	fmt.Println("--- Query: price = 79.99 ---")
	result = idx.Evaluate([]gin.Predicate{gin.EQ("$.price", 79.99)})
	fmt.Printf("Row groups: %v\n\n", result.ToSlice())

	// Combined with stock
	fmt.Println("--- Query: price < 100 AND stock > 50 ---")
	result = idx.Evaluate([]gin.Predicate{
		gin.LT("$.price", 100.0),
		gin.GT("$.stock", 50.0),
	})
	fmt.Printf("Row groups: %v\n\n", result.ToSlice())

	// Query that matches nothing (price out of range)
	fmt.Println("--- Query: price > 10000 (no matches) ---")
	result = idx.Evaluate([]gin.Predicate{gin.GT("$.price", 10000.0)})
	fmt.Printf("Row groups: %v (empty = no matches)\n", result.ToSlice())

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
