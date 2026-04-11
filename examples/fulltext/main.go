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

	if err := addDocuments(builder,
		exampleDocument{rgID: 0, body: `{
		"title": "Introduction to Machine Learning",
		"content": "Machine learning is a subset of artificial intelligence that enables systems to learn from data."
	}`},
		exampleDocument{rgID: 0, body: `{
		"title": "Deep Learning Fundamentals",
		"content": "Deep learning uses neural networks with multiple layers to process complex patterns."
	}`},
		exampleDocument{rgID: 1, body: `{
		"title": "PostgreSQL Performance Tuning",
		"content": "Learn how to optimize your PostgreSQL database for better performance and scalability."
	}`},
		exampleDocument{rgID: 1, body: `{
		"title": "Introduction to NoSQL Databases",
		"content": "NoSQL databases provide flexible schemas and horizontal scaling for modern applications."
	}`},
		exampleDocument{rgID: 2, body: `{
		"title": "Building REST APIs with Go",
		"content": "Learn how to build performant REST APIs using the Go programming language."
	}`},
		exampleDocument{rgID: 2, body: `{
		"title": "React Best Practices",
		"content": "Best practices for building scalable React applications with modern patterns."
	}`},
		exampleDocument{rgID: 3, body: `{
		"title": "Kubernetes for Beginners",
		"content": "Getting started with Kubernetes container orchestration and deployment."
	}`},
		exampleDocument{rgID: 3, body: `{
		"title": "CI/CD Pipeline Setup",
		"content": "How to set up continuous integration and deployment pipelines for your team."
	}`},
	); err != nil {
		return err
	}

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
