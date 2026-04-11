// Example: Comprehensive GIN index usage demonstrating all index types and query operators
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
		"name": "Laptop Pro",
		"description": "High performance laptop for developers",
		"price": 1299.99,
		"quantity": 50,
		"in_stock": true,
		"tags": ["electronics", "computers"]
	}`},
		exampleDocument{rgID: 1, body: `{
		"name": "Go Programming",
		"description": "Learn Go programming language",
		"price": 49.99,
		"quantity": 200,
		"in_stock": true,
		"tags": ["books", "programming"]
	}`},
		exampleDocument{rgID: 2, body: `{
		"name": "Vintage Keyboard",
		"description": "Classic mechanical keyboard",
		"price": 299.99,
		"quantity": 0,
		"in_stock": false,
		"tags": ["electronics", "accessories"]
	}`},
		exampleDocument{rgID: 3, body: `{
		"name": "Server Rack",
		"description": "Enterprise server rack for data centers",
		"price": 5999.99,
		"quantity": 5,
		"in_stock": true,
		"tags": ["electronics", "enterprise"]
	}`},
	); err != nil {
		return err
	}

	idx := builder.Finalize()

	fmt.Printf("Index built: %d docs, %d row groups, %d paths\n",
		idx.Header.NumDocs, idx.Header.NumRowGroups, idx.Header.NumPaths)

	// String equality
	fmt.Println("\n--- String: EQ ---")
	fmt.Println("name = 'Laptop Pro':", idx.Evaluate([]gin.Predicate{
		gin.EQ("$.name", "Laptop Pro"),
	}).ToSlice()) // [0]

	// Boolean equality
	fmt.Println("\n--- Boolean: EQ ---")
	fmt.Println("in_stock = true:", idx.Evaluate([]gin.Predicate{
		gin.EQ("$.in_stock", true),
	}).ToSlice()) // [0, 1, 3]

	fmt.Println("in_stock = false:", idx.Evaluate([]gin.Predicate{
		gin.EQ("$.in_stock", false),
	}).ToSlice()) // [2]

	// Float range queries
	fmt.Println("\n--- Float: Range ---")
	fmt.Println("price >= 100 AND price < 500:", idx.Evaluate([]gin.Predicate{
		gin.GTE("$.price", 100.0),
		gin.LT("$.price", 500.0),
	}).ToSlice()) // [2]

	fmt.Println("price > 1000:", idx.Evaluate([]gin.Predicate{
		gin.GT("$.price", 1000.0),
	}).ToSlice()) // [0, 3]

	fmt.Println("price <= 50:", idx.Evaluate([]gin.Predicate{
		gin.LTE("$.price", 50.0),
	}).ToSlice()) // [1]

	// Integer comparison
	fmt.Println("\n--- Integer: Range ---")
	fmt.Println("quantity > 10:", idx.Evaluate([]gin.Predicate{
		gin.GT("$.quantity", 10),
	}).ToSlice()) // [0, 1]

	fmt.Println("quantity = 0:", idx.Evaluate([]gin.Predicate{
		gin.EQ("$.quantity", 0),
	}).ToSlice()) // [2]

	// IN on array elements
	fmt.Println("\n--- Array: IN ---")
	fmt.Println("tags IN ['books', 'accessories']:", idx.Evaluate([]gin.Predicate{
		gin.IN("$.tags[*]", "books", "accessories"),
	}).ToSlice()) // [1, 2]

	// NOT IN
	fmt.Println("\n--- Array: NIN (NOT IN) ---")
	fmt.Println("tags NIN ['enterprise']:", idx.Evaluate([]gin.Predicate{
		gin.NIN("$.tags[*]", "enterprise"),
	}).ToSlice()) // [0, 1, 2]

	// Full-text search (CONTAINS uses trigram index)
	fmt.Println("\n--- String: CONTAINS (trigram) ---")
	fmt.Println("description CONTAINS 'keyboard':", idx.Evaluate([]gin.Predicate{
		gin.Contains("$.description", "keyboard"),
	}).ToSlice()) // [2]

	fmt.Println("description CONTAINS 'server':", idx.Evaluate([]gin.Predicate{
		gin.Contains("$.description", "server"),
	}).ToSlice()) // [3]

	// Combined queries (predicates are ANDed)
	fmt.Println("\n--- Combined Queries ---")
	fmt.Println("in_stock AND electronics AND price < 2000:", idx.Evaluate([]gin.Predicate{
		gin.EQ("$.in_stock", true),
		gin.EQ("$.tags[*]", "electronics"),
		gin.LT("$.price", 2000.0),
	}).ToSlice()) // [0]

	fmt.Println("quantity > 0 AND price < 100:", idx.Evaluate([]gin.Predicate{
		gin.GT("$.quantity", 0),
		gin.LT("$.price", 100.0),
	}).ToSlice()) // [1]

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
