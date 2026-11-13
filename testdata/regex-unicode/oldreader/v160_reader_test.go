//go:build regex_oldreader_v160

package gin

// This file is copied into a worktree of v1.6.0 (2fbbd97) by
// `make old-reader-check`. v1.6.0 has no Normalized, so it uses As only. It
// may only use API present at that commit.
//
//	AMI_GIN_REGEX_OLDREADER_IN    directory with regex.gin, regex.unicode and frozen-garay-go1.27.3.gin
//	AMI_GIN_REGEX_OLDREADER_DONE  file the test writes after its assertions

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unicode"
)

type regexOldReaderV160Query struct {
	name      string
	predicate Predicate
}

// regexOldReaderV160Queries are the As rows of the regex.gin baseline.
func regexOldReaderV160Queries() []regexOldReaderV160Query {
	return []regexOldReaderV160Query{
		{"upper As ABC+A7DC", EQ("$.name", As("upper", "ABCꟜ"))},
		{"upper As DEF", EQ("$.name", As("upper", "DEF"))},
		{"n2 As 1", EQ("$.ref", As("n2", float64(1)))},
		{"n2 As 2", EQ("$.ref", As("n2", float64(2)))},
		{"num As 7", EQ("$.code", As("num", float64(7)))},
		{"ci As A7DC", EQ("$.name", As("ci", "Ƛ"))},
		{"ci As 019B", EQ("$.name", As("ci", "ƛ"))},
	}
}

var regexOldReaderV160Baselines = map[string][][]int{
	"15.0.0": {{}, {3}, {2}, {0, 1}, {0, 3}, {}, {2}},
	"17.0.0": {{0}, {3}, {0, 2}, {1}, {0, 3}, {0}, {2}},
}

func regexOldReaderV160Decode(t *testing.T, path string) *GINIndex {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	idx, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode(%s): %v", path, err)
	}
	return idx
}

func regexOldReaderV160Require(t *testing.T, idx *GINIndex, label string, predicate Predicate, want []int) {
	t.Helper()
	got := idx.Evaluate([]Predicate{predicate}).ToSlice()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func TestRegexOldReaderV160(t *testing.T) {
	in := os.Getenv("AMI_GIN_REGEX_OLDREADER_IN")
	if in == "" {
		t.Skip("AMI_GIN_REGEX_OLDREADER_IN is not set; run make old-reader-check")
	}
	done := os.Getenv("AMI_GIN_REGEX_OLDREADER_DONE")
	if done == "" {
		t.Fatal("AMI_GIN_REGEX_OLDREADER_DONE is not set")
	}
	unicodeData, err := os.ReadFile(filepath.Join(in, "regex.unicode"))
	if err != nil {
		t.Fatal(err)
	}
	builderUnicode := strings.TrimSpace(string(unicodeData))
	want, ok := regexOldReaderV160Baselines[builderUnicode]
	if !ok {
		t.Fatalf("no baseline for builder unicode %q", builderUnicode)
	}
	t.Logf("reader %s (unicode %s), builder unicode %s", runtime.Version(), unicode.Version, builderUnicode)

	idx := regexOldReaderV160Decode(t, filepath.Join(in, "regex.gin"))
	for i, query := range regexOldReaderV160Queries() {
		regexOldReaderV160Require(t, idx, "regex.gin "+query.name, query.predicate, want[i])
	}

	// The frozen ranges compile on Unicode 15.0.0; the raw \p{Garay} would not.
	garay := regexOldReaderV160Decode(t, filepath.Join(in, "frozen-garay-go1.27.3.gin"))
	regexOldReaderV160Require(t, garay, "garay As", EQ("$.name", As("garay", "\U00010D4A\U00010D4B")), []int{0})

	if t.Failed() {
		return
	}
	if err := os.WriteFile(done, []byte(runtime.Version()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}
