//go:build simdjson

package gin_test

import (
	"log"

	gin "github.com/amikos-tech/ami-gin"
)

func ExampleNewSIMDParser() {
	// SIMD_EXAMPLE_START
	config := gin.DefaultConfig()
	const numRGs = 1

	parser, err := gin.NewSIMDParser()
	if err != nil {
		log.Printf("SIMD parser unavailable: %v", err)
		return
	}
	defer func() {
		if err := parser.Close(); err != nil {
			log.Printf("close SIMD parser: %v", err)
		}
	}()

	builder, err := gin.NewBuilder(config, numRGs, gin.WithParser(parser))
	if err != nil {
		log.Printf("create SIMD builder: %v", err)
		return
	}
	if err := builder.AddDocument(0, []byte(`{"status":"ready"}`)); err != nil {
		log.Printf("add document: %v", err)
		return
	}
	_ = builder.Finalize()
	// SIMD_EXAMPLE_END
}
