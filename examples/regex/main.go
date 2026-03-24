// Example: Regex pattern matching with trigram-based candidate selection
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
	// Enable trigrams (required for regex candidate selection)
	config := gin.DefaultConfig()
	config.EnableTrigrams = true

	builder, err := gin.NewBuilder(config, 5)
	if err != nil {
		return errors.Wrap(err, "create builder")
	}

	// Row group 0: Toyota content
	builder.AddDocument(0, []byte(`{
		"brand": "Toyota Corolla",
		"category": "sedan",
		"log": "INFO: Vehicle started successfully"
	}`))

	// Row group 1: Tesla content
	builder.AddDocument(1, []byte(`{
		"brand": "Tesla Model 3",
		"category": "electric",
		"log": "ERROR: Battery low warning triggered"
	}`))

	// Row group 2: Ford content
	builder.AddDocument(2, []byte(`{
		"brand": "Ford Mustang",
		"category": "sports",
		"log": "WARNING: Engine temperature high"
	}`))

	// Row group 3: Mixed brands
	builder.AddDocument(3, []byte(`{
		"brand": "Toyota Camry and Tesla Model S comparison",
		"category": "review",
		"log": "INFO: Comparison completed"
	}`))

	// Row group 4: Honda (no match for Toyota|Tesla|Ford)
	builder.AddDocument(4, []byte(`{
		"brand": "Honda Civic",
		"category": "sedan",
		"log": "DEBUG: Diagnostics running"
	}`))

	idx := builder.Finalize()

	fmt.Println("=== Regex Pattern Matching ===")

	// Simple alternation: Toyota|Tesla
	fmt.Println("--- Query: brand REGEX 'Toyota|Tesla' ---")
	result := idx.Evaluate([]gin.Predicate{gin.Regex("$.brand", "Toyota|Tesla")})
	fmt.Printf("Row groups: %v (expected: [0, 1, 3])\n\n", result.ToSlice())

	// Three-way alternation: Toyota|Tesla|Ford
	fmt.Println("--- Query: brand REGEX 'Toyota|Tesla|Ford' ---")
	result = idx.Evaluate([]gin.Predicate{gin.Regex("$.brand", "Toyota|Tesla|Ford")})
	fmt.Printf("Row groups: %v (expected: [0, 1, 2, 3])\n\n", result.ToSlice())

	// Log level pattern: ERROR|WARNING
	fmt.Println("--- Query: log REGEX 'ERROR|WARNING' ---")
	result = idx.Evaluate([]gin.Predicate{gin.Regex("$.log", "ERROR|WARNING")})
	fmt.Printf("Row groups: %v (expected: [1, 2])\n\n", result.ToSlice())

	// Prefix pattern with wildcard (extracts "INFO:" and "completed" as separate literals)
	// Returns candidates containing either literal - actual regex matching happens at query time
	fmt.Println("--- Query: log REGEX 'INFO:.*completed' ---")
	result = idx.Evaluate([]gin.Predicate{gin.Regex("$.log", "INFO:.*completed")})
	fmt.Printf("Row groups: %v (candidates with 'INFO:' or 'completed')\n\n", result.ToSlice())

	// Grouped alternation: (electric|sports) car categories
	fmt.Println("--- Query: category REGEX 'electric|sports' ---")
	result = idx.Evaluate([]gin.Predicate{gin.Regex("$.category", "electric|sports")})
	fmt.Printf("Row groups: %v (expected: [1, 2])\n\n", result.ToSlice())

	// Combined with other predicates
	fmt.Println("--- Query: brand REGEX 'Toyota|Tesla' AND category = 'sedan' ---")
	result = idx.Evaluate([]gin.Predicate{
		gin.Regex("$.brand", "Toyota|Tesla"),
		gin.EQ("$.category", "sedan"),
	})
	fmt.Printf("Row groups: %v (expected: [0])\n\n", result.ToSlice())

	// No match pattern
	fmt.Println("--- Query: brand REGEX 'BMW|Mercedes' (no matches) ---")
	result = idx.Evaluate([]gin.Predicate{gin.Regex("$.brand", "BMW|Mercedes")})
	fmt.Printf("Row groups: %v (expected: [])\n\n", result.ToSlice())

	// Pattern with character class (extracts literal prefix)
	fmt.Println("--- Query: log REGEX 'ERROR:.*[a-z]+' ---")
	result = idx.Evaluate([]gin.Predicate{gin.Regex("$.log", "ERROR:.*[a-z]+")})
	fmt.Printf("Row groups: %v (expected: [1])\n", result.ToSlice())

	return nil
}
