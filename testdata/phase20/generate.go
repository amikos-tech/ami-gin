// Command generate refreshes the checked-in Phase 20 JSONL smoke fixtures.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

const fixtureDir = "testdata/phase20"

type generatedFixture struct {
	name    string
	records []string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "generate Phase 20 fixtures: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	for _, fixture := range generatedFixtures() {
		if err := writeFixture(fixture.name, fixture.records); err != nil {
			return errors.Wrapf(err, "generate fixture %q", fixture.name)
		}
	}
	return nil
}

func generatedFixtures() []generatedFixture {
	nested := make([]string, 0, 96)
	mixed := make([]string, 0, 96)
	numbers := make([]string, 0, 96)
	for i := 0; i < 96; i++ {
		nested = append(nested, nestedRecord(i))
		mixed = append(mixed, mixedArrayRecord(i))
		numbers = append(numbers, numberRecord(i))
	}

	combined := make([]string, 0, 288)
	for i := 0; i < 96; i++ {
		combined = append(combined, nested[i], mixed[i], numbers[i])
	}

	return []generatedFixture{
		{name: "nested_high_cardinality.jsonl", records: nested},
		{name: "mixed_type_arrays.jsonl", records: mixed},
		{name: "number_heavy.jsonl", records: numbers},
		{name: "combined.jsonl", records: combined},
	}
}

func renderFixture(records []string) []byte {
	return []byte(strings.Join(records, "\n") + "\n")
}

func writeFixture(name string, records []string) error {
	return writeFixtureToDir(fixtureDir, name, records)
}

func writeFixtureToDir(dir, name string, records []string) error {
	if filepath.Base(name) != name {
		return errors.Errorf("fixture name %q is not a base filename", name)
	}
	path := filepath.Join(dir, name)
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.Errorf("refusing to replace symlinked fixture %q", path)
		}
		if !info.Mode().IsRegular() {
			return errors.Errorf("refusing to replace non-regular fixture %q", path)
		}
	} else if !os.IsNotExist(err) {
		return errors.Wrapf(err, "inspect fixture %q", path)
	}

	temporary, err := os.CreateTemp(dir, ".phase20-fixture-*")
	if err != nil {
		return errors.Wrapf(err, "create temporary fixture for %q", path)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return errors.Wrapf(err, "set permissions on temporary fixture for %q", path)
	}
	if _, err := temporary.Write(renderFixture(records)); err != nil {
		temporary.Close()
		return errors.Wrapf(err, "write temporary fixture for %q", path)
	}
	if err := temporary.Close(); err != nil {
		return errors.Wrapf(err, "close temporary fixture for %q", path)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return errors.Wrapf(err, "replace fixture %q", path)
	}
	return nil
}

func nestedRecord(i int) string {
	categories := []string{"issue-comment", "pull-request", "commit", "discussion"}
	user := fmt.Sprintf("user-%03d", i)
	repo := fmt.Sprintf("org-%02d/service-%02d", i%12, i%24)
	url := fmt.Sprintf("https://example.test/%s/events/%04d", repo, 1000+i)
	timestamp := fmt.Sprintf("2024-02-%02dT%02d:%02d:00Z", (i%28)+1, i%24, (i*7)%60)
	text := fmt.Sprintf("%s event %04d from %s reviews searchable indexing behavior", categories[i%len(categories)], i, user)
	return fmt.Sprintf(`{"phase20_source_kind":"nested-high-cardinality","event_category":%q,"event_id":"evt-%04d","timestamp":%q,"actor":{"id":"actor-%04d","user":{"login":%q,"display_name":"Fixture User %03d"}},"repository":{"id":"repo-%04d","name":%q,"url":%q},"url":%q,"searchable_text":%q}`,
		categories[i%len(categories)], i, timestamp, 10000+i, user, i, 20000+i, repo, "https://example.test/"+repo, url, text)
}

func mixedArrayRecord(i int) string {
	return fmt.Sprintf(`{"phase20_source_kind":"mixed-type-arrays","record_id":"array-%04d","timestamp":"2024-03-%02dT12:%02d:00Z","wildcard_values":["tag-%02d",%d,%.2f,%t,null,{"kind":"object","id":"object-%04d"},[]],"searchable_text":"mixed wildcard array fixture %04d"}`,
		i, (i%28)+1, (i*11)%60, i%16, i, float64(i)/10.0+0.25, i%2 == 0, i, i)
}

func numberRecord(i int) string {
	cohorts := []string{"low", "mid", "high"}
	cohort := cohorts[i%len(cohorts)]
	base := []int{10, 5000, 900000}[i%len(cohorts)] + i
	return fmt.Sprintf(`{"phase20_source_kind":"number-heavy","record_id":"number-%04d","signed_int64_max":9223372036854775807,"signed_int64_min":-9223372036854775807,"exact_float_boundary":9007199254740993,"decimal_value":%.3f,"ratio":0.125,"timestamp":"2024-04-%02dT08:%02d:00Z","epoch_ms":%d,"range_cohort":%q,"range_value":%d,"measurements":[%d,%.2f]}`,
		i, float64(i)/3.0+0.5, (i%28)+1, (i*13)%60, 1711958400000+i*1000, cohort, base, base, float64(base)/10.0)
}
