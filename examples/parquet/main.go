package main

import (
	"fmt"
	"os"

	"github.com/parquet-go/parquet-go"
	"github.com/pkg/errors"

	gin "github.com/amikos-tech/gin-index"
)

type Record struct {
	ID         int64  `parquet:"id"`
	Attributes string `parquet:"attributes"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	tmpDir, err := os.MkdirTemp("", "gin-parquet-example")
	if err != nil {
		return errors.Wrap(err, "create temp dir")
	}
	defer os.RemoveAll(tmpDir)

	parquetFile := tmpDir + "/data.parquet"

	fmt.Println("=== Creating Parquet file with JSON column ===")
	if err := createParquetFile(parquetFile); err != nil {
		return errors.Wrap(err, "create parquet file")
	}
	fmt.Printf("Created: %s\n\n", parquetFile)

	fmt.Println("=== Building GIN index from Parquet ===")
	ginCfg := gin.DefaultConfig()
	idx, err := gin.BuildFromParquet(parquetFile, "attributes", ginCfg)
	if err != nil {
		return errors.Wrap(err, "build index")
	}
	fmt.Printf("Index built: %d row groups, %d paths, %d docs\n\n",
		idx.Header.NumRowGroups, idx.Header.NumPaths, idx.Header.NumDocs)

	fmt.Println("=== Sidecar Workflow ===")
	if err := gin.WriteSidecar(parquetFile, idx); err != nil {
		return errors.Wrap(err, "write sidecar")
	}
	fmt.Printf("Sidecar written: %s\n", gin.SidecarPath(parquetFile))

	loadedIdx, err := gin.ReadSidecar(parquetFile)
	if err != nil {
		return errors.Wrap(err, "read sidecar")
	}
	fmt.Printf("Sidecar loaded: %d row groups\n\n", loadedIdx.Header.NumRowGroups)

	fmt.Println("=== Embedded Workflow ===")
	pqCfg := gin.DefaultParquetConfig()
	if err := gin.RebuildWithIndex(parquetFile, idx, pqCfg); err != nil {
		return errors.Wrap(err, "embed index")
	}
	fmt.Println("Index embedded in Parquet file")

	hasIdx, err := gin.HasGINIndex(parquetFile, pqCfg)
	if err != nil {
		return errors.Wrap(err, "check index")
	}
	fmt.Printf("Has embedded index: %v\n", hasIdx)

	embeddedIdx, err := gin.ReadFromParquetMetadata(parquetFile, pqCfg)
	if err != nil {
		return errors.Wrap(err, "read embedded index")
	}
	fmt.Printf("Embedded index loaded: %d row groups\n\n", embeddedIdx.Header.NumRowGroups)

	fmt.Println("=== Auto-loading (tries embedded first, then sidecar) ===")
	autoIdx, err := gin.LoadIndex(parquetFile, pqCfg)
	if err != nil {
		return errors.Wrap(err, "load index")
	}
	fmt.Printf("Auto-loaded index: %d row groups\n\n", autoIdx.Header.NumRowGroups)

	fmt.Println("=== Querying the Index ===")
	queries := []gin.Predicate{
		gin.EQ("$.status", "error"),
		gin.EQ("$.status", "success"),
		gin.GT("$.count", 5.0),
		gin.Contains("$.message", "important"),
	}

	for _, q := range queries {
		result := autoIdx.Evaluate([]gin.Predicate{q})
		rgs := result.ToSlice()
		fmt.Printf("Query: %s\n", q)
		fmt.Printf("  Matching row groups: %v (%d/%d)\n\n", rgs, len(rgs), autoIdx.Header.NumRowGroups)
	}

	fmt.Println("=== Encode to Metadata (for use when creating Parquet) ===")
	key, value, err := gin.EncodeToMetadata(idx, pqCfg)
	if err != nil {
		return errors.Wrap(err, "encode metadata")
	}
	fmt.Printf("Metadata key: %s\n", key)
	fmt.Printf("Metadata value length: %d bytes (base64 encoded)\n\n", len(value))

	fmt.Println("=== Path Information ===")
	for _, pe := range autoIdx.PathDirectory {
		fmt.Printf("  %s (cardinality=%d)\n", pe.PathName, pe.Cardinality)
	}

	fmt.Println("\nDone!")

	return nil
}

func createParquetFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	records := []Record{
		{ID: 1, Attributes: `{"status": "success", "count": 10, "message": "All good"}`},
		{ID: 2, Attributes: `{"status": "error", "count": 0, "message": "Something went wrong"}`},
		{ID: 3, Attributes: `{"status": "success", "count": 5, "message": "Completed"}`},
		{ID: 4, Attributes: `{"status": "warning", "count": 3, "message": "This is important"}`},
		{ID: 5, Attributes: `{"status": "error", "count": 0, "message": "Critical failure"}`},
		{ID: 6, Attributes: `{"status": "success", "count": 15, "message": "Very important task done"}`},
		{ID: 7, Attributes: `{"status": "pending", "count": null, "message": null}`},
		{ID: 8, Attributes: `{"status": "success", "count": 8, "message": "Normal operation"}`},
	}

	writer := parquet.NewGenericWriter[Record](f,
		parquet.MaxRowsPerRowGroup(int64(2)),
	)

	for _, r := range records {
		if _, err := writer.Write([]Record{r}); err != nil {
			return fmt.Errorf("write record: %w", err)
		}
	}

	return writer.Close()
}
