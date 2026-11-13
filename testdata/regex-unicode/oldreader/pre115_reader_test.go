//go:build regex_oldreader

package gin

// This file is copied into a worktree of the pre-#115 commit 29a3e0c by
// `make old-reader-check`. It may only use API present at that commit. It
// decodes indexes written by the current (#115) builder and asserts exact
// sets with As and Normalized.
//
//	AMI_GIN_REGEX_OLDREADER_IN    directory with regex.gin, regex.unicode and frozen-garay-go1.27.3.gin
//	AMI_GIN_REGEX_OLDREADER_DONE  file the test writes after its assertions
//
// TestRegexNegativeControlPre115 is the negative control: the pre-#115 reader
// on Unicode 15.0.0 decodes legacy-regex-go1.27.3.gin (pre-#115 builder on
// 17.0.0, unfrozen patterns) and must return the known wrong sets. It proves
// that the exact-set checks above detect the drift.
//
//	AMI_GIN_REGEX_NEGCTL_IN    path of legacy-regex-go1.27.3.gin
//	AMI_GIN_REGEX_NEGCTL_DONE  file the test writes after its assertions

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unicode"
)

type regexOldReaderQuery struct {
	name       string
	predicates []Predicate
}

func regexOldReaderQueries() []regexOldReaderQuery {
	return []regexOldReaderQuery{
		{"upper Normalized ABC+A7DC", []Predicate{EQ("$.name", Normalized("upper", "ABCꟜ"))}},
		{"upper Normalized ABC", []Predicate{EQ("$.name", Normalized("upper", "ABC"))}},
		{"upper As ABC+A7DC", []Predicate{EQ("$.name", As("upper", "ABCꟜ"))}},
		{"upper As DEF", []Predicate{EQ("$.name", As("upper", "DEF"))}},
		{"n2 Normalized 1+A7DC+2A", []Predicate{EQ("$.ref", Normalized("n2", "1Ƛ2A"))}},
		{"n2 As 1", []Predicate{EQ("$.ref", As("n2", float64(1)))}},
		{"n2 As 2", []Predicate{EQ("$.ref", As("n2", float64(2)))}},
		{"num As 7", []Predicate{EQ("$.code", As("num", float64(7)))}},
		{"upper Normalized ABC+A7DC AND status error", []Predicate{
			EQ("$.name", Normalized("upper", "ABCꟜ")),
			EQ("$.status", "error"),
		}},
		{"ci Normalized A7DC", []Predicate{EQ("$.name", Normalized("ci", "Ƛ"))}},
		{"ci Normalized 019B", []Predicate{EQ("$.name", Normalized("ci", "ƛ"))}},
		{"ci As A7DC", []Predicate{EQ("$.name", As("ci", "Ƛ"))}},
		{"n2 GTE Normalized 1+A7DC+2A", []Predicate{GTE("$.ref", Normalized("n2", "1Ƛ2A"))}},
	}
}

// regexOldReaderBaselines duplicates regexUnicodeBaselines (regex_unicode_build_test.go).
var regexOldReaderBaselines = map[string][][]int{
	"15.0.0": {{0, 1}, {0, 1}, {}, {3}, {0, 1}, {2}, {0, 1}, {0, 3}, {0}, {0, 1, 2, 3}, {2}, {}, {0, 1}},
	"17.0.0": {{0}, {1}, {0}, {3}, {0, 2}, {0, 2}, {1}, {0, 3}, {0}, {0}, {2}, {0}, {0, 1, 2}},
}

func regexOldReaderDecode(t *testing.T, path string) *GINIndex {
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

func regexOldReaderRequire(t *testing.T, idx *GINIndex, label string, predicates []Predicate, want []int) {
	t.Helper()
	got := idx.Evaluate(predicates).ToSlice()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func TestRegexOldReaderPre115(t *testing.T) {
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
	want, ok := regexOldReaderBaselines[builderUnicode]
	if !ok {
		t.Fatalf("no baseline for builder unicode %q", builderUnicode)
	}
	t.Logf("reader %s (unicode %s), builder unicode %s", runtime.Version(), unicode.Version, builderUnicode)

	idx := regexOldReaderDecode(t, filepath.Join(in, "regex.gin"))
	for i, query := range regexOldReaderQueries() {
		regexOldReaderRequire(t, idx, "regex.gin "+query.name, query.predicates, want[i])
	}

	garay := regexOldReaderDecode(t, filepath.Join(in, "frozen-garay-go1.27.3.gin"))
	regexOldReaderRequire(t, garay, "garay As", []Predicate{EQ("$.name", As("garay", "\U00010D4A\U00010D4B"))}, []int{0})
	regexOldReaderRequire(t, garay, "garay Normalized", []Predicate{EQ("$.name", Normalized("garay", "x\U00010D4Cy"))}, []int{2})

	if t.Failed() {
		return
	}
	if err := os.WriteFile(done, []byte(runtime.Version()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRegexNegativeControlPre115(t *testing.T) {
	in := os.Getenv("AMI_GIN_REGEX_NEGCTL_IN")
	if in == "" {
		t.Skip("AMI_GIN_REGEX_NEGCTL_IN is not set; run make old-reader-check")
	}
	done := os.Getenv("AMI_GIN_REGEX_NEGCTL_DONE")
	if done == "" {
		t.Fatal("AMI_GIN_REGEX_NEGCTL_DONE is not set")
	}
	if unicode.Version != "15.0.0" {
		t.Fatalf("negative control needs a Unicode 15.0.0 reader, running %s", unicode.Version)
	}
	idx := regexOldReaderDecode(t, in)
	// The 17.0.0 builder baseline is {0} and {0, 1, 2}. The reader extracts
	// "ABC" and 2 with its 15.0.0 tables and prunes row groups that match.
	regexOldReaderRequire(t, idx, "negative control upper Normalized ABC+A7DC",
		[]Predicate{EQ("$.name", Normalized("upper", "ABC\uA7DC"))}, []int{1})
	regexOldReaderRequire(t, idx, "negative control n2 GTE Normalized 1+A7DC+2A",
		[]Predicate{GTE("$.ref", Normalized("n2", "1\uA7DC2A"))}, []int{1})
	if t.Failed() {
		return
	}
	if err := os.WriteFile(done, []byte(runtime.Version()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}
