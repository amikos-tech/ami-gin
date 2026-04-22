package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"

	gin "github.com/amikos-tech/ami-gin"
)

type experimentReport struct {
	Source        experimentSource           `json:"source"`
	Summary       experimentSummary          `json:"summary"`
	Paths         []experimentPathRow        `json:"paths"`
	PredicateTest *experimentPredicateResult `json:"predicate_test,omitempty"`
}

type experimentSource struct {
	Input string `json:"input"`
	Stdin bool   `json:"stdin"`
}

type experimentSummary struct {
	Documents      int    `json:"documents"`
	RowGroups      int    `json:"row_groups"`
	RGSize         int    `json:"rg_size"`
	SampleLimit    int    `json:"sample_limit"`
	ProcessedLines int    `json:"processed_lines"`
	SkippedLines   int    `json:"skipped_lines"`
	ErrorCount     int    `json:"error_count"`
	SidecarPath    string `json:"sidecar_path"`
}

type experimentPathRow struct {
	Path                   string   `json:"path"`
	PathID                 uint16   `json:"path_id"`
	Types                  []string `json:"types"`
	CardinalityEstimate    uint32   `json:"cardinality_estimate"`
	Mode                   string   `json:"mode"`
	BloomOccupancyEstimate float64  `json:"bloom_occupancy_estimate"`
	PromotedHotTerms       int      `json:"promoted_hot_terms"`
	BucketCount            int      `json:"bucket_count"`
	Representations        []string `json:"representations"`
}

type experimentPredicateResult struct {
	Predicate    string  `json:"predicate"`
	Matched      int     `json:"matched"`
	Pruned       int     `json:"pruned"`
	PruningRatio float64 `json:"pruning_ratio"`
}

func collectExperimentPathRows(idx *gin.GINIndex) []experimentPathRow {
	rows := make([]experimentPathRow, 0, len(idx.PathDirectory))
	for _, pe := range idx.PathDirectory {
		if strings.HasPrefix(pe.PathName, "__derived:") {
			continue
		}

		promotedHotTerms := 0
		bucketCount := 0
		if pe.Mode == gin.PathModeAdaptiveHybrid {
			promotedHotTerms, bucketCount = adaptivePathSummary(idx, pe)
		}

		representationsInfo := idx.Representations(pe.PathName)
		representations := make([]string, 0, len(representationsInfo))
		for _, representation := range representationsInfo {
			representations = append(representations, representation.Alias+":"+representation.Transformer)
		}

		rows = append(rows, experimentPathRow{
			Path:                   pe.PathName,
			PathID:                 pe.PathID,
			Types:                  collectExperimentTypes(pe.ObservedTypes),
			CardinalityEstimate:    pe.Cardinality,
			Mode:                   pe.Mode.String(),
			BloomOccupancyEstimate: estimateBloomOccupancy(pe.Cardinality, idx.GlobalBloom),
			PromotedHotTerms:       promotedHotTerms,
			BucketCount:            bucketCount,
			Representations:        representations,
		})
	}
	return rows
}

func collectExperimentTypes(types uint8) []string {
	out := make([]string, 0, 5)
	if types&gin.TypeString != 0 {
		out = append(out, "string")
	}
	if types&gin.TypeInt != 0 {
		out = append(out, "int")
	}
	if types&gin.TypeFloat != 0 {
		out = append(out, "float")
	}
	if types&gin.TypeBool != 0 {
		out = append(out, "bool")
	}
	if types&gin.TypeNull != 0 {
		out = append(out, "null")
	}
	return out
}

func estimateBloomOccupancy(cardinality uint32, bf *gin.BloomFilter) float64 {
	if bf == nil || bf.NumBits() == 0 || bf.NumHashes() == 0 || cardinality == 0 {
		return 0
	}

	n := float64(cardinality)
	k := float64(bf.NumHashes())
	m := float64(bf.NumBits())
	return 1 - math.Exp(-(k*n)/m)
}

func writeExperimentText(stdout io.Writer, report experimentReport, idx *gin.GINIndex) {
	fmt.Fprintln(stdout, "Experiment Summary:")
	fmt.Fprintf(stdout, "  Input: %s\n", report.Source.Input)
	fmt.Fprintf(stdout, "  Documents: %d\n", report.Summary.Documents)
	fmt.Fprintf(stdout, "  Row Groups: %d\n", report.Summary.RowGroups)
	fmt.Fprintf(stdout, "  RG Size: %d\n", report.Summary.RGSize)
	if report.Summary.SampleLimit > 0 {
		fmt.Fprintf(stdout, "  Sample Limit: %d\n", report.Summary.SampleLimit)
	}
	if report.Summary.ProcessedLines != report.Summary.Documents || report.Summary.SkippedLines > 0 || report.Summary.ErrorCount > 0 || report.Summary.SampleLimit > 0 {
		fmt.Fprintf(stdout, "  Processed Lines: %d\n", report.Summary.ProcessedLines)
		fmt.Fprintf(stdout, "  Skipped Lines: %d\n", report.Summary.SkippedLines)
		fmt.Fprintf(stdout, "  Error Count: %d\n", report.Summary.ErrorCount)
	}
	if report.Summary.SidecarPath != "" {
		fmt.Fprintf(stdout, "  Sidecar Path: %s\n", report.Summary.SidecarPath)
	}
	fmt.Fprintln(stdout)

	idxCopy := *idx
	idxCopy.Header.NumRowGroups = uint32(report.Summary.RowGroups)
	writeIndexInfo(stdout, &idxCopy)

	if report.PredicateTest == nil {
		return
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Predicate Test:")
	fmt.Fprintf(stdout, "  Predicate: %s\n", report.PredicateTest.Predicate)
	fmt.Fprintf(stdout, "  Matched: %d\n", report.PredicateTest.Matched)
	fmt.Fprintf(stdout, "  Pruned: %d\n", report.PredicateTest.Pruned)
	fmt.Fprintf(stdout, "  Pruning Ratio: %.4f\n", report.PredicateTest.PruningRatio)
}

func writeExperimentJSON(stdout io.Writer, report experimentReport) error {
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
