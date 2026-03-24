// Example: Full-text search with trigram index (CONTAINS queries)
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
	// Enable trigrams in config (enabled by default)
	config := gin.DefaultConfig()
	config.EnableTrigrams = true

	builder, err := gin.NewBuilder(config, 4)
	if err != nil {
		return errors.Wrap(err, "create builder")
	}

	// Row group 0: Tech articles
	builder.AddDocument(0, []byte(`{
		"title": "Introduction to Machine Learning",
		"content": "Machine learning is a subset of artificial intelligence that enables systems to learn from data."
	}`))
	builder.AddDocument(0, []byte(`{
		"title": "Deep Learning Fundamentals",
		"content": "Deep learning uses neural networks with multiple layers to process complex patterns."
	}`))

	// Row group 1: Database articles
	builder.AddDocument(1, []byte(`{
		"title": "PostgreSQL Performance Tuning",
		"content": "Learn how to optimize your PostgreSQL database for better performance and scalability."
	}`))
	builder.AddDocument(1, []byte(`{
		"title": "Introduction to NoSQL Databases",
		"content": "NoSQL databases provide flexible schemas and horizontal scaling for modern applications."
	}`))

	// Row group 2: Web development
	builder.AddDocument(2, []byte(`{
		"title": "Building REST APIs with Go",
		"content": "Learn how to build performant REST APIs using the Go programming language."
	}`))
	builder.AddDocument(2, []byte(`{
		"title": "React Best Practices",
		"content": "Best practices for building scalable React applications with modern patterns."
	}`))

	// Row group 3: DevOps
	builder.AddDocument(3, []byte(`{
		"title": "Kubernetes for Beginners",
		"content": "Getting started with Kubernetes container orchestration and deployment."
	}`))
	builder.AddDocument(3, []byte(`{
		"title": "CI/CD Pipeline Setup",
		"content": "How to set up continuous integration and deployment pipelines for your team."
	}`))

	idx := builder.Finalize()

	fmt.Println("=== Full-Text Search (CONTAINS) ===")

	// Search for "learning"
	fmt.Println("--- Query: title CONTAINS 'learning' ---")
	result := idx.Evaluate([]gin.Predicate{gin.Contains("$.title", "learning")})
	fmt.Printf("Row groups: %v\n\n", result.ToSlice())

	// Search for "database"
	fmt.Println("--- Query: content CONTAINS 'database' ---")
	result = idx.Evaluate([]gin.Predicate{gin.Contains("$.content", "database")})
	fmt.Printf("Row groups: %v\n\n", result.ToSlice())

	// Search for "performance"
	fmt.Println("--- Query: content CONTAINS 'performance' ---")
	result = idx.Evaluate([]gin.Predicate{gin.Contains("$.content", "performance")})
	fmt.Printf("Row groups: %v\n\n", result.ToSlice())

	// Case insensitive (trigrams are lowercased)
	fmt.Println("--- Query: title CONTAINS 'POSTGRESQL' (case insensitive) ---")
	result = idx.Evaluate([]gin.Predicate{gin.Contains("$.title", "POSTGRESQL")})
	fmt.Printf("Row groups: %v\n\n", result.ToSlice())

	// Partial word match
	fmt.Println("--- Query: content CONTAINS 'scala' (matches 'scalable', 'scaling') ---")
	result = idx.Evaluate([]gin.Predicate{gin.Contains("$.content", "scala")})
	fmt.Printf("Row groups: %v\n\n", result.ToSlice())

	// Combined with equality
	fmt.Println("--- Query: title CONTAINS 'Introduction' AND content CONTAINS 'learn' ---")
	result = idx.Evaluate([]gin.Predicate{
		gin.Contains("$.title", "Introduction"),
		gin.Contains("$.content", "learn"),
	})
	fmt.Printf("Row groups: %v\n\n", result.ToSlice())

	// No matches
	fmt.Println("--- Query: content CONTAINS 'blockchain' (no matches) ---")
	result = idx.Evaluate([]gin.Predicate{gin.Contains("$.content", "blockchain")})
	fmt.Printf("Row groups: %v\n", result.ToSlice())

	return nil
}
