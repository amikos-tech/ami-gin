// Example: Advanced field transformers for IP ranges, semantic versions, emails, and regex extraction
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
	fmt.Println("=== Advanced Field Transformers ===")
	fmt.Println()

	if err := ipRangeExample(); err != nil {
		return err
	}
	if err := semverExample(); err != nil {
		return err
	}
	if err := caseInsensitiveExample(); err != nil {
		return err
	}
	if err := emailDomainExample(); err != nil {
		return err
	}
	if err := regexExtractExample(); err != nil {
		return err
	}
	if err := durationExample(); err != nil {
		return err
	}

	return nil
}

func ipRangeExample() error {
	fmt.Println("--- 1. IPv4ToInt + InSubnet: IP Subnet Queries ---")

	config, err := gin.NewConfig(
		gin.WithFieldTransformer("$.client_ip", gin.IPv4ToInt),
	)
	if err != nil {
		return errors.Wrap(err, "create config")
	}

	builder, err := gin.NewBuilder(config, 4)
	if err != nil {
		return errors.Wrap(err, "create builder")
	}

	if err := addDocuments(builder,
		exampleDocument{rgID: 0, body: `{"client_ip": "192.168.1.1", "action": "login"}`},
		exampleDocument{rgID: 0, body: `{"client_ip": "192.168.1.50", "action": "download"}`},
		exampleDocument{rgID: 1, body: `{"client_ip": "10.0.0.1", "action": "upload"}`},
		exampleDocument{rgID: 1, body: `{"client_ip": "10.0.0.100", "action": "login"}`},
		exampleDocument{rgID: 2, body: `{"client_ip": "192.168.2.1", "action": "admin"}`},
		exampleDocument{rgID: 3, body: `{"client_ip": "172.16.0.1", "action": "api_call"}`},
	); err != nil {
		return err
	}

	idx := builder.Finalize()

	// Query: Find all 192.168.x.x using InSubnet helper (CIDR notation)
	fmt.Println("Query: Find IPs in 192.168.0.0/16 subnet (using InSubnet)")
	result := idx.Evaluate(gin.InSubnet("$.client_ip", "192.168.0.0/16"))
	fmt.Printf("Row groups: %v (expected: [0, 2] - 192.168.x.x IPs)\n", result.ToSlice())

	// Query: Find all 10.x.x.x using InSubnet helper
	fmt.Println("Query: Find IPs in 10.0.0.0/8 subnet (using InSubnet)")
	result = idx.Evaluate(gin.InSubnet("$.client_ip", "10.0.0.0/8"))
	fmt.Printf("Row groups: %v (expected: [1] - 10.x.x.x IPs)\n", result.ToSlice())

	// Query: Find specific /24 subnet using CIDRToRange for manual control
	fmt.Println("Query: Find IPs in 192.168.1.0/24 subnet (using CIDRToRange)")
	start, end, _ := gin.CIDRToRange("192.168.1.0/24")
	result = idx.Evaluate([]gin.Predicate{
		gin.GTE("$.client_ip", start),
		gin.LTE("$.client_ip", end),
	})
	fmt.Printf("Row groups: %v (expected: [0] - only 192.168.1.x IPs)\n\n", result.ToSlice())

	return nil
}

func semverExample() error {
	fmt.Println("--- 2. SemVerToInt: Version Range Queries ---")

	config, err := gin.NewConfig(
		gin.WithFieldTransformer("$.version", gin.SemVerToInt),
	)
	if err != nil {
		return errors.Wrap(err, "create config")
	}

	builder, err := gin.NewBuilder(config, 4)
	if err != nil {
		return errors.Wrap(err, "create builder")
	}

	if err := addDocuments(builder,
		exampleDocument{rgID: 0, body: `{"name": "app-core", "version": "1.5.0"}`},
		exampleDocument{rgID: 1, body: `{"name": "app-ui", "version": "v2.1.3"}`},
		exampleDocument{rgID: 2, body: `{"name": "app-api", "version": "2.0.0-beta"}`},
		exampleDocument{rgID: 3, body: `{"name": "app-cli", "version": "3.0.0"}`},
	); err != nil {
		return err
	}

	idx := builder.Finalize()

	// Query: Find packages >= 2.0.0
	// 2.0.0 = 2*1000000 + 0*1000 + 0 = 2000000
	fmt.Println("Query: Find packages with version >= 2.0.0")
	result := idx.Evaluate([]gin.Predicate{
		gin.GTE("$.version", float64(2000000)),
	})
	fmt.Printf("Row groups: %v (expected: [1, 2, 3] - versions 2.1.3, 2.0.0, 3.0.0)\n", result.ToSlice())

	// Query: Find packages < 2.0.0
	fmt.Println("Query: Find packages with version < 2.0.0")
	result = idx.Evaluate([]gin.Predicate{
		gin.LT("$.version", float64(2000000)),
	})
	fmt.Printf("Row groups: %v (expected: [0] - version 1.5.0)\n", result.ToSlice())

	// Query: Find packages in range [2.0.0, 2.999.999]
	fmt.Println("Query: Find packages with version 2.x.x")
	result = idx.Evaluate([]gin.Predicate{
		gin.GTE("$.version", float64(2000000)),
		gin.LT("$.version", float64(3000000)),
	})
	fmt.Printf("Row groups: %v (expected: [1, 2] - versions 2.1.3, 2.0.0)\n\n", result.ToSlice())

	return nil
}

func caseInsensitiveExample() error {
	fmt.Println("--- 3. ToLower: Case-Insensitive Queries ---")

	config, err := gin.NewConfig(
		gin.WithFieldTransformer("$.email", gin.ToLower),
		gin.WithFieldTransformer("$.username", gin.ToLower),
	)
	if err != nil {
		return errors.Wrap(err, "create config")
	}

	builder, err := gin.NewBuilder(config, 3)
	if err != nil {
		return errors.Wrap(err, "create builder")
	}

	if err := addDocuments(builder,
		exampleDocument{rgID: 0, body: `{"username": "Alice", "email": "Alice@Example.COM"}`},
		exampleDocument{rgID: 1, body: `{"username": "BOB", "email": "bob@example.com"}`},
		exampleDocument{rgID: 2, body: `{"username": "charlie", "email": "CHARLIE@EXAMPLE.COM"}`},
	); err != nil {
		return err
	}

	idx := builder.Finalize()

	// Query: Find user "alice" (case-insensitive)
	fmt.Println("Query: Find username = 'alice' (originally 'Alice')")
	result := idx.Evaluate([]gin.Predicate{
		gin.EQ("$.username", "alice"),
	})
	fmt.Printf("Row groups: %v (expected: [0])\n", result.ToSlice())

	// Query: Find email "bob@example.com" (case-insensitive)
	fmt.Println("Query: Find email = 'bob@example.com'")
	result = idx.Evaluate([]gin.Predicate{
		gin.EQ("$.email", "bob@example.com"),
	})
	fmt.Printf("Row groups: %v (expected: [1])\n", result.ToSlice())

	// Query: Find all emails at example.com domain
	fmt.Println("Query: Find all @example.com emails")
	result = idx.Evaluate([]gin.Predicate{
		gin.IN("$.email", "alice@example.com", "bob@example.com", "charlie@example.com"),
	})
	fmt.Printf("Row groups: %v (expected: [0, 1, 2])\n\n", result.ToSlice())

	return nil
}

func emailDomainExample() error {
	fmt.Println("--- 4. EmailDomain: Filter by Email Domain ---")

	config, err := gin.NewConfig(
		gin.WithFieldTransformer("$.email", gin.EmailDomain),
	)
	if err != nil {
		return errors.Wrap(err, "create config")
	}

	builder, err := gin.NewBuilder(config, 4)
	if err != nil {
		return errors.Wrap(err, "create builder")
	}

	if err := addDocuments(builder,
		exampleDocument{rgID: 0, body: `{"name": "Alice", "email": "alice@company.com"}`},
		exampleDocument{rgID: 1, body: `{"name": "Bob", "email": "bob@GMAIL.COM"}`},
		exampleDocument{rgID: 2, body: `{"name": "Charlie", "email": "charlie@company.com"}`},
		exampleDocument{rgID: 3, body: `{"name": "Diana", "email": "diana@startup.io"}`},
	); err != nil {
		return err
	}

	idx := builder.Finalize()

	// Query: Find all company.com users
	fmt.Println("Query: Find all @company.com users")
	result := idx.Evaluate([]gin.Predicate{
		gin.EQ("$.email", "company.com"),
	})
	fmt.Printf("Row groups: %v (expected: [0, 2])\n", result.ToSlice())

	// Query: Find external users (gmail, startup.io)
	fmt.Println("Query: Find gmail.com or startup.io users")
	result = idx.Evaluate([]gin.Predicate{
		gin.IN("$.email", "gmail.com", "startup.io"),
	})
	fmt.Printf("Row groups: %v (expected: [1, 3])\n\n", result.ToSlice())

	return nil
}

func regexExtractExample() error {
	fmt.Println("--- 5. RegexExtract: Extract Structured Data ---")

	config, err := gin.NewConfig(
		// Extract error code from log messages: "ERROR[E1234]: msg" -> "E1234"
		gin.WithFieldTransformer("$.message", gin.RegexExtract(`ERROR\[(\w+)\]:`, 1)),
		// Extract order number from order IDs: "order-12345" -> 12345 (as float64)
		gin.WithFieldTransformer("$.order_id", gin.RegexExtractInt(`order-(\d+)`, 1)),
	)
	if err != nil {
		return errors.Wrap(err, "create config")
	}

	builder, err := gin.NewBuilder(config, 4)
	if err != nil {
		return errors.Wrap(err, "create builder")
	}

	if err := addDocuments(builder,
		exampleDocument{rgID: 0, body: `{"message": "ERROR[E1001]: Connection timeout", "order_id": "order-100"}`},
		exampleDocument{rgID: 1, body: `{"message": "ERROR[E2001]: Invalid input", "order_id": "order-200"}`},
		exampleDocument{rgID: 2, body: `{"message": "ERROR[E1001]: Connection refused", "order_id": "order-300"}`},
		exampleDocument{rgID: 3, body: `{"message": "INFO: Success", "order_id": "order-400"}`},
	); err != nil {
		return err
	}

	idx := builder.Finalize()

	// Query: Find all E1001 errors
	fmt.Println("Query: Find error code E1001")
	result := idx.Evaluate([]gin.Predicate{
		gin.EQ("$.message", "E1001"),
	})
	fmt.Printf("Row groups: %v (expected: [0, 2] - connection errors)\n", result.ToSlice())

	// Query: Find orders >= 200
	fmt.Println("Query: Find order_id >= 200")
	result = idx.Evaluate([]gin.Predicate{
		gin.GTE("$.order_id", float64(200)),
	})
	fmt.Printf("Row groups: %v (expected: [1, 2, 3] - orders 200, 300, 400)\n\n", result.ToSlice())

	return nil
}

func durationExample() error {
	fmt.Println("--- 6. DurationToMs: Latency Range Queries ---")

	config, err := gin.NewConfig(
		gin.WithFieldTransformer("$.latency", gin.DurationToMs),
	)
	if err != nil {
		return errors.Wrap(err, "create config")
	}

	builder, err := gin.NewBuilder(config, 4)
	if err != nil {
		return errors.Wrap(err, "create builder")
	}

	if err := addDocuments(builder,
		exampleDocument{rgID: 0, body: `{"endpoint": "/api/users", "latency": "50ms"}`},
		exampleDocument{rgID: 1, body: `{"endpoint": "/api/search", "latency": "1s"}`},
		exampleDocument{rgID: 2, body: `{"endpoint": "/api/export", "latency": "2m30s"}`},
		exampleDocument{rgID: 3, body: `{"endpoint": "/api/health", "latency": "5ms"}`},
	); err != nil {
		return err
	}

	idx := builder.Finalize()

	// Query: Find requests with latency > 100ms
	fmt.Println("Query: Find latency > 100ms")
	result := idx.Evaluate([]gin.Predicate{
		gin.GT("$.latency", float64(100)),
	})
	fmt.Printf("Row groups: %v (expected: [1, 2] - 1s and 2m30s)\n", result.ToSlice())

	// Query: Find requests with latency > 1 minute
	fmt.Println("Query: Find latency > 1 minute (60000ms)")
	result = idx.Evaluate([]gin.Predicate{
		gin.GT("$.latency", float64(60000)),
	})
	fmt.Printf("Row groups: %v (expected: [2] - 2m30s = 150000ms)\n\n", result.ToSlice())

	fmt.Println("=== Summary ===")
	fmt.Println("Field transformers convert values at index time for efficient queries:")
	fmt.Println("- IPv4ToInt + InSubnet: Filter by IP subnet using CIDR notation")
	fmt.Println("- CIDRToRange: Parse CIDR for manual range control")
	fmt.Println("- SemVerToInt: Compare software versions numerically")
	fmt.Println("- ToLower: Case-insensitive string matching")
	fmt.Println("- EmailDomain: Filter by organization/domain")
	fmt.Println("- RegexExtract: Parse structured IDs and error codes")
	fmt.Println("- DurationToMs: Query latency/timeout thresholds")

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
