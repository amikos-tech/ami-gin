// Example: Field transformers for date indexing
package main

import (
	"fmt"
	"os"
	"time"

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
	// Configure field transformers to convert date strings to epoch milliseconds
	config, err := gin.NewConfig(
		gin.WithISODateTransformer("$.created_at", "epoch_ms"),
		gin.WithDateTransformer("$.birth_date", "epoch_ms"),
		gin.WithCustomDateTransformer("$.custom_ts", "epoch_ms", "2006/01/02 15:04"),
	)
	if err != nil {
		return errors.Wrap(err, "create config")
	}

	builder, err := gin.NewBuilder(config, 5)
	if err != nil {
		return errors.Wrap(err, "create builder")
	}

	if err := addDocuments(builder,
		exampleDocument{rgID: 0, body: `{
		"name": "alice",
		"created_at": "2024-01-15T10:30:00Z",
		"birth_date": "1990-05-20",
		"custom_ts": "2024/01/15 10:30"
	}`},
		exampleDocument{rgID: 0, body: `{
		"name": "bob",
		"created_at": "2024-01-20T14:00:00Z",
		"birth_date": "1985-03-10",
		"custom_ts": "2024/01/20 14:00"
	}`},
		exampleDocument{rgID: 1, body: `{
		"name": "charlie",
		"created_at": "2024-03-01T09:00:00Z",
		"birth_date": "1992-08-15",
		"custom_ts": "2024/03/01 09:00"
	}`},
		exampleDocument{rgID: 2, body: `{
		"name": "diana",
		"created_at": "2024-06-15T16:45:00Z",
		"birth_date": "1988-12-01",
		"custom_ts": "2024/06/15 16:45"
	}`},
		exampleDocument{rgID: 3, body: `{
		"name": "eve",
		"created_at": "2024-09-01T08:00:00Z",
		"birth_date": "1995-02-28",
		"custom_ts": "2024/09/01 08:00"
	}`},
		exampleDocument{rgID: 4, body: `{
		"name": "frank",
		"created_at": "2024-12-25T12:00:00Z",
		"birth_date": "1980-07-04",
		"custom_ts": "2024/12/25 12:00"
	}`},
	); err != nil {
		return err
	}

	idx := builder.Finalize()

	fmt.Println("=== Field Transformer Date Range Queries ===")
	fmt.Println()

	// Query: Find records created after July 1, 2024
	july2024 := float64(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC).UnixMilli())
	fmt.Printf("--- Query: created_at > July 1, 2024 (epoch: %.0f) ---\n", july2024)
	result := idx.Evaluate([]gin.Predicate{gin.GT("$.created_at", july2024)})
	fmt.Printf("Row groups: %v (expected: [3, 4] - September and December)\n\n", result.ToSlice())

	// Query: Find records created in Q1 2024 (Jan-Mar)
	jan2024 := float64(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli())
	apr2024 := float64(time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC).UnixMilli())
	fmt.Printf("--- Query: created_at >= Jan 1, 2024 AND created_at < Apr 1, 2024 ---\n")
	result = idx.Evaluate([]gin.Predicate{
		gin.GTE("$.created_at", jan2024),
		gin.LT("$.created_at", apr2024),
	})
	fmt.Printf("Row groups: %v (expected: [0, 1] - January and March)\n\n", result.ToSlice())

	// Query: Find people born before 1990
	year1990 := float64(time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli())
	fmt.Printf("--- Query: birth_date < 1990-01-01 (epoch: %.0f) ---\n", year1990)
	result = idx.Evaluate([]gin.Predicate{gin.LT("$.birth_date", year1990)})
	fmt.Printf("Row groups: %v (expected: [0, 2, 4] - bob 1985, diana 1988, frank 1980)\n\n", result.ToSlice())

	// Query: Find records from H2 2024 (July-December)
	fmt.Println("--- Query: created_at >= July 2024 (H2 2024) ---")
	result = idx.Evaluate([]gin.Predicate{gin.GTE("$.created_at", july2024)})
	fmt.Printf("Row groups: %v (expected: [3, 4])\n\n", result.ToSlice())

	// Query using custom timestamp format
	fmt.Println("--- Query: custom_ts > March 2024 ---")
	mar2024 := float64(time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC).UnixMilli())
	result = idx.Evaluate([]gin.Predicate{gin.GT("$.custom_ts", mar2024)})
	fmt.Printf("Row groups: %v (expected: [2, 3, 4] - June, September, December)\n\n", result.ToSlice())

	// Demonstrate the power: combining date range with other predicates
	fmt.Println("--- Combined: created_at in Q1 2024 AND birth_date before 1990 ---")
	result = idx.Evaluate([]gin.Predicate{
		gin.GTE("$.created_at", jan2024),
		gin.LT("$.created_at", apr2024),
		gin.LT("$.birth_date", year1990),
	})
	fmt.Printf("Row groups: %v (expected: [0] - bob created Jan 2024, born 1985)\n\n", result.ToSlice())

	fmt.Println("=== Benefits of Date Transformers ===")
	fmt.Println("1. Date strings are indexed as numeric epoch milliseconds")
	fmt.Println("2. Enables efficient range queries using GT/GTE/LT/LTE operators")
	fmt.Println("3. Per-row-group min/max stats allow fast pruning")
	fmt.Println("4. No need to parse dates at query time")

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
