// Example: Serializing and deserializing GIN index
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
	// Build an index
	builder, err := gin.NewBuilder(gin.DefaultConfig(), 3)
	if err != nil {
		return errors.Wrap(err, "create builder")
	}

	builder.AddDocument(0, []byte(`{"product": "laptop", "brand": "acme", "price": 999.99}`))
	builder.AddDocument(0, []byte(`{"product": "mouse", "brand": "acme", "price": 29.99}`))
	builder.AddDocument(1, []byte(`{"product": "keyboard", "brand": "techco", "price": 79.99}`))
	builder.AddDocument(1, []byte(`{"product": "monitor", "brand": "viewmax", "price": 349.99}`))
	builder.AddDocument(2, []byte(`{"product": "webcam", "brand": "acme", "price": 89.99}`))

	idx := builder.Finalize()

	fmt.Println("=== Original Index ===")
	fmt.Printf("Documents: %d, Row Groups: %d, Paths: %d\n",
		idx.Header.NumDocs, idx.Header.NumRowGroups, idx.Header.NumPaths)

	// Test query on original
	result := idx.Evaluate([]gin.Predicate{gin.EQ("$.brand", "acme")})
	fmt.Printf("Query 'brand=acme' on original: %v\n", result.ToSlice())

	// Serialize to bytes (zstd compressed)
	encoded, err := gin.Encode(idx)
	if err != nil {
		return errors.Wrap(err, "encode index")
	}
	fmt.Printf("\nSerialized size: %d bytes\n", len(encoded))

	// Save to file
	filename := "/tmp/gin_index.bin"
	if err := os.WriteFile(filename, encoded, 0644); err != nil {
		return errors.Wrap(err, "write index file")
	}
	fmt.Printf("Saved to: %s\n", filename)

	// Read from file
	data, err := os.ReadFile(filename)
	if err != nil {
		return errors.Wrap(err, "read index file")
	}

	// Deserialize
	loaded, err := gin.Decode(data)
	if err != nil {
		return errors.Wrap(err, "decode index")
	}

	fmt.Println("\n=== Loaded Index ===")
	fmt.Printf("Documents: %d, Row Groups: %d, Paths: %d\n",
		loaded.Header.NumDocs, loaded.Header.NumRowGroups, loaded.Header.NumPaths)

	// Test same query on loaded index
	result = loaded.Evaluate([]gin.Predicate{gin.EQ("$.brand", "acme")})
	fmt.Printf("Query 'brand=acme' on loaded: %v\n", result.ToSlice())

	// Test range query
	result = loaded.Evaluate([]gin.Predicate{
		gin.GTE("$.price", 50.0),
		gin.LTE("$.price", 500.0),
	})
	fmt.Printf("Query 'price 50-500' on loaded: %v\n", result.ToSlice())

	// Test CONTAINS query (trigram index)
	result = loaded.Evaluate([]gin.Predicate{gin.Contains("$.product", "board")})
	fmt.Printf("Query 'product contains board' on loaded: %v\n", result.ToSlice())

	// Cleanup
	os.Remove(filename)
	fmt.Printf("\nCleaned up %s\n", filename)

	return nil
}
