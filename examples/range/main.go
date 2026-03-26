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

	// Row group 0: products $10-$50
	builder.AddDocument(0, []byte(`{"name": "widget", "price": 19.99, "stock": 100}`))
	builder.AddDocument(0, []byte(`{"name": "gadget", "price": 49.99, "stock": 50}`))

	// Row group 1: products $50-$100
	builder.AddDocument(1, []byte(`{"name": "device", "price": 79.99, "stock": 25}`))
	builder.AddDocument(1, []byte(`{"name": "tool", "price": 99.99, "stock": 75}`))

	// Row group 2: products $100-$500
	builder.AddDocument(2, []byte(`{"name": "appliance", "price": 299.99, "stock": 10}`))
	builder.AddDocument(2, []byte(`{"name": "equipment", "price": 449.99, "stock": 5}`))

	// Row group 3: premium products $500+
	builder.AddDocument(3, []byte(`{"name": "luxury item", "price": 999.99, "stock": 2}`))

	// Row group 4: budget products under $20
	builder.AddDocument(4, []byte(`{"name": "basic", "price": 9.99, "stock": 500}`))
	builder.AddDocument(4, []byte(`{"name": "simple", "price": 14.99, "stock": 300}`))

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
