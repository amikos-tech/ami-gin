package gin

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// The tests in this file pin Unicode 15.0.0 behavior with committed goldens
// and literal row sets. They run on every toolchain.
// transformer_lower_goldens_test.go regenerates the goldens; it does not
// compare them. A build and a query on one toolchain agree even on broken
// code. So the value of these tests is that the same literals must also hold
// on Go 1.27 (Unicode 17.0.0) (#102).

// transformerDriftDocs is the fixture: one document per row group. RG3 and RG4
// have no email and no url field.
var transformerDriftDocs = []map[string]string{
	{"name": "\uA7DCabc", "email": "u@\uA7DCabc.com", "url": "https://\uA7DCabc.example/p"},
	{"name": "\u019Babc", "email": "u@\u019Babc.com", "url": "https://\u019Babc.example/p"},
	{"name": "\uA7CB\u1C89xyz", "email": "u@\uA7CB\u1C89xyz.com", "url": "https://\uA7CB\u1C89xyz.example/p"},
	{"name": "\u0130stanbul"},
	{"name": "\u212Aelvin"},
	{"name": "Alice", "email": "bob@Example.COM", "url": "https://Example.COM/x"},
}

func transformerDriftConfig(t *testing.T) GINConfig {
	t.Helper()
	cfg, err := NewConfig(
		WithToLowerTransformer("$.name", "lower"),
		WithEmailDomainTransformer("$.email", "domain"),
		WithURLHostTransformer("$.url", "host"),
	)
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	return cfg
}

func buildTransformerDriftIndex(t *testing.T) *GINIndex {
	t.Helper()
	return buildTrigramUnicodeIndex(t, transformerDriftConfig(t), transformerDriftDocs)
}

type transformerDriftQuery struct {
	name      string
	predicate Predicate
	want      []int
}

// transformerDriftQueries holds exact row groups under Unicode 15.0.0
// semantics. The expected sets are literals, never computed with
// strings.ToLower at test time.
var transformerDriftQueries = []transformerDriftQuery{
	{"EQ lower A7DC", EQ("$.name", As("lower", "\uA7DCabc")), []int{0}},
	{"EQ lower 019B", EQ("$.name", As("lower", "\u019Babc")), []int{1}},
	{"EQ lower A7CB 1C89", EQ("$.name", As("lower", "\uA7CB\u1C89xyz")), []int{2}},
	{"EQ lower istanbul", EQ("$.name", As("lower", "istanbul")), []int{3}},
	{"EQ lower kelvin", EQ("$.name", As("lower", "kelvin")), []int{4}},
	{"EQ lower alice", EQ("$.name", As("lower", "alice")), []int{5}},
	{"EQ lower 17.0.0 lowering of RG2 does not match", EQ("$.name", As("lower", "\u0264\u1C8Axyz")), []int{}},
	{"IN lower A7DC alice", IN("$.name", As("lower", "\uA7DCabc"), As("lower", "alice")), []int{0, 5}},
	{"NE lower alice", NE("$.name", As("lower", "alice")), []int{0, 1, 2, 3, 4}},
	{"EQ domain A7DC", EQ("$.email", As("domain", "\uA7DCabc.com")), []int{0}},
	{"EQ domain example", EQ("$.email", As("domain", "example.com")), []int{5}},
	{"EQ host A7DC", EQ("$.url", As("host", "\uA7DCabc.example")), []int{0}},
	{"EQ host example", EQ("$.url", As("host", "example.com")), []int{5}},
	{"Contains lower A7DC", Predicate{Path: "$.name", Operator: OpContains, Value: As("lower", "\uA7DCab")}, []int{0}},
	{"Contains lower 019B", Predicate{Path: "$.name", Operator: OpContains, Value: As("lower", "\u019Bab")}, []int{1}},
}

// transformerDriftNormalizedQueries holds Normalized rows with the same
// semantics; they run through the same tests as transformerDriftQueries.
var transformerDriftNormalizedQueries = []transformerDriftQuery{
	{"EQ Normalized lower A7DC", EQ("$.name", Normalized("lower", "\uA7DCABC")), []int{0}},
	{"EQ Normalized lower 0130", EQ("$.name", Normalized("lower", "\u0130STANBUL")), []int{3}},
	{"EQ Normalized lower 212A", EQ("$.name", Normalized("lower", "\u212AELVIN")), []int{4}},
	{"EQ Normalized lower ALICE", EQ("$.name", Normalized("lower", "ALICE")), []int{5}},
	{"IN Normalized lower A7DC ALICE", IN("$.name", Normalized("lower", "\uA7DCABC"), Normalized("lower", "ALICE")), []int{0, 5}},
	{"EQ Normalized domain A7DC", EQ("$.email", Normalized("domain", "u@\uA7DCABC.com")), []int{0}},
	{"EQ Normalized host A7DC", EQ("$.url", Normalized("host", "https://\uA7DCABC.example/z")), []int{0}},
}

func requireTransformerDriftQueries(t *testing.T, idx *GINIndex, label string) {
	t.Helper()
	for _, tc := range transformerDriftQueries {
		t.Run(label+"/"+tc.name, func(t *testing.T) {
			requireRows(t, idx, tc.predicate, tc.want)
		})
	}
	for _, tc := range transformerDriftNormalizedQueries {
		t.Run(label+"/normalized/"+tc.name, func(t *testing.T) {
			requireRows(t, idx, tc.predicate, tc.want)
		})
	}
}

// transformerAggregatedDocs is the second cross-toolchain fixture. RG0 holds
// two documents on one DocID: U+A7DC has no lowercase mapping at 15.0.0, so
// the lower companion keeps two distinct values there. RG1 holds only the
// U+019B value. RG2 is a distractor.
var transformerAggregatedDocs = []struct {
	rg   DocID
	name string
}{
	{0, "\uA7DCabc"},
	{0, "\u019Babc"},
	{1, "\u019Babc"},
	{2, "Zeta"},
}

func buildTransformerAggregatedIndex(t *testing.T) *GINIndex {
	t.Helper()
	builder := mustNewBuilder(t, transformerDriftConfig(t), 3)
	for _, doc := range transformerAggregatedDocs {
		data, err := json.Marshal(map[string]string{"name": doc.name})
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if err := builder.AddDocument(doc.rg, data); err != nil {
			t.Fatalf("AddDocument(%d): %v", doc.rg, err)
		}
	}
	return builder.Finalize()
}

// transformerAggregatedQueries holds exact row groups under Unicode 15.0.0
// semantics, measured on go1.26.9 and go1.27.3. NE and NIN keep RG0 unless the
// query names both of its distinct values.
var transformerAggregatedQueries = []transformerDriftQuery{
	{"EQ As 019B", EQ("$.name", As("lower", "\u019Babc")), []int{0, 1}},
	{"EQ As A7DC", EQ("$.name", As("lower", "\uA7DCabc")), []int{0}},
	{"EQ Normalized A7DC", EQ("$.name", Normalized("lower", "\uA7DCABC")), []int{0}},
	{"EQ Normalized 019B", EQ("$.name", Normalized("lower", "\u019BABC")), []int{0, 1}},
	{"IN As both", IN("$.name", As("lower", "\u019Babc"), As("lower", "\uA7DCabc")), []int{0, 1}},
	{"NE As 019B", NE("$.name", As("lower", "\u019Babc")), []int{0, 2}},
	{"NIN As 019B", NIN("$.name", As("lower", "\u019Babc")), []int{0, 2}},
	{"NIN Normalized A7DC", NIN("$.name", Normalized("lower", "\uA7DCABC")), []int{0, 1, 2}},
	{"NIN As both", NIN("$.name", As("lower", "\u019Babc"), As("lower", "\uA7DCabc")), []int{2}},
	{"NIN Normalized both", NIN("$.name", Normalized("lower", "\u019BABC"), Normalized("lower", "\uA7DCABC")), []int{2}},
	{"Contains As 019B", Predicate{Path: "$.name", Operator: OpContains, Value: As("lower", "\u019Bab")}, []int{0, 1}},
}

func requireTransformerAggregatedQueries(t *testing.T, idx *GINIndex, label string) {
	t.Helper()
	for _, tc := range transformerAggregatedQueries {
		t.Run(label+"/aggregated/"+tc.name, func(t *testing.T) {
			requireRows(t, idx, tc.predicate, tc.want)
		})
	}
}

func TestTransformerLowerAggregatedQueries(t *testing.T) {
	built := buildTransformerAggregatedIndex(t)
	requireTransformerAggregatedQueries(t, built, "built")
	requireTransformerAggregatedQueries(t, mustRoundTripIndex(t, built), "decoded")
}

func TestTransformerLowerDriftQueriesBuilt(t *testing.T) {
	requireTransformerDriftQueries(t, buildTransformerDriftIndex(t), "built")
}

func TestFrozenToLowerMatchesStdlib15(t *testing.T) {
	if unicode.Version != trigramUnicodeVersion {
		t.Skipf("unicode.Version = %s; strings.ToLower equals the frozen tables only at %s", unicode.Version, trigramUnicodeVersion)
	}
	for r := rune(0); r <= unicode.MaxRune; r++ {
		s := string(r)
		if got, want := frozenToLower(s), strings.ToLower(s); got != want {
			t.Fatalf("frozenToLower(%U) = %q, strings.ToLower = %q", r, got, want)
		}
	}
	for _, s := range []string{"\u0130", "\u212A", "a\xffb\xc3", "\xed\xa0\x80x", "\xf4\x90\x80\x80", "\u03A3\u0391\u03A3"} {
		if got, want := frozenToLower(s), strings.ToLower(s); got != want {
			t.Fatalf("frozenToLower(%q) = %q, strings.ToLower = %q", s, got, want)
		}
	}
	rng := rand.New(rand.NewSource(102))
	buf := make([]byte, 0, 24)
	for i := 0; i < 200000; i++ {
		buf = buf[:0]
		n := rng.Intn(12)
		for j := 0; j < n; j++ {
			switch rng.Intn(4) {
			case 0:
				buf = append(buf, byte(rng.Intn(256)))
			case 1:
				buf = append(buf, byte('A'+rng.Intn(58)))
			default:
				buf = append(buf, string(rune(rng.Intn(0x30000)))...)
			}
		}
		s := string(buf)
		if got, want := frozenToLower(s), strings.ToLower(s); got != want {
			t.Fatalf("frozenToLower(%q) = %q, strings.ToLower = %q", s, got, want)
		}
	}
}

func TestFrozenToLowerDiffersAtUnicode17(t *testing.T) {
	if unicode.Version != "17.0.0" {
		t.Skipf("unicode.Version = %s; the 55-rune drift is measured at 17.0.0", unicode.Version)
	}
	var diffs []rune
	for r := rune(0); r <= unicode.MaxRune; r++ {
		s := string(r)
		got, std := frozenToLower(s), strings.ToLower(s)
		if got == std {
			continue
		}
		diffs = append(diffs, r)
		for i := 0; i < len(std); i++ {
			if std[i] < 0x80 {
				t.Fatalf("strings.ToLower(%U) = %q contains ASCII", r, std)
			}
		}
	}
	if !slices.Equal(diffs, unicode17LowerDrift) {
		t.Fatalf("frozenToLower differs from strings.ToLower at %d runes %U, want the %d committed runes %U",
			len(diffs), diffs, len(unicode17LowerDrift), unicode17LowerDrift)
	}
}

// unicode17LowerDrift is the exact set of runes whose strings.ToLower result
// differs between Unicode 15.0.0 (the frozen tables) and Unicode 17.0.0
// (Go 1.27), measured with go1.27.3. Unicode does not promise that only
// additions change, so the test compares this exact set, not a count.
var unicode17LowerDrift = []rune{
	0x1C89, 0xA7CB, 0xA7CC, 0xA7CE, 0xA7D2, 0xA7D4, 0xA7DA, 0xA7DC,
	0x10D50, 0x10D51, 0x10D52, 0x10D53, 0x10D54, 0x10D55, 0x10D56, 0x10D57,
	0x10D58, 0x10D59, 0x10D5A, 0x10D5B, 0x10D5C, 0x10D5D, 0x10D5E, 0x10D5F,
	0x10D60, 0x10D61, 0x10D62, 0x10D63, 0x10D64, 0x10D65, 0x16EA0, 0x16EA1,
	0x16EA2, 0x16EA3, 0x16EA4, 0x16EA5, 0x16EA6, 0x16EA7, 0x16EA8, 0x16EA9,
	0x16EAA, 0x16EAB, 0x16EAC, 0x16EAD, 0x16EAE, 0x16EAF, 0x16EB0, 0x16EB1,
	0x16EB2, 0x16EB3, 0x16EB4, 0x16EB5, 0x16EB6, 0x16EB7, 0x16EB8,
}

func TestFrozenToLowerBehavior(t *testing.T) {
	if n := testing.AllocsPerRun(100, func() { _ = frozenToLower("already lower 123") }); n != 0 {
		t.Errorf("frozenToLower(lowercase ASCII) allocs = %v, want 0", n)
	}
	cases := map[string]string{
		"Hello World":   "hello world",
		"\u0130stanbul": "istanbul",
		"\u212Aelvin":   "kelvin",
		"\uA7DCabc":     "\uA7DCabc",
		"a\xffB":        "a\uFFFDb",
		"":              "",
	}
	for in, want := range cases {
		if got := frozenToLower(in); got != want {
			t.Errorf("frozenToLower(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTransformerLowerNonStringAndFailureRules(t *testing.T) {
	for name, fn := range map[string]FieldTransformer{"ToLower": ToLower, "EmailDomain": EmailDomain, "URLHost": URLHost} {
		if v, ok := fn(42); ok || v != nil {
			t.Errorf("%s(42) = (%v, %t), want (nil, false)", name, v, ok)
		}
	}
	for _, in := range []string{"nodomain", "a@b@c.com", "a@"} {
		if _, ok := EmailDomain(in); ok {
			t.Errorf("EmailDomain(%q) ok = true", in)
		}
	}
	for _, in := range []string{"/relative", "mailto:x", "%zz://x"} {
		if _, ok := URLHost(in); ok {
			t.Errorf("URLHost(%q) ok = true", in)
		}
	}
}

var (
	transformerLowerOutputsGolden = filepath.Join("testdata", "transformer-lower", "lower-outputs.golden")
	transformerLowerDriftGolden   = filepath.Join("testdata", "transformer-lower", "drift-index.bin")
)

// transformerLowerDriftInputs holds runes whose lowercase differs between
// Unicode 15.0.0 and 17.0.0, plus context and invalid-UTF-8 cases.
var transformerLowerDriftInputs = []string{
	"\uA7DCabc", // lowers to U+019B at 17.0.0, not at 15.0.0
	"\u019Babc",
	"\uA7CB\u1C89xyz", // U+A7CB and U+1C89 have no lowercase mapping at 15.0.0
	"\u0130stanbul",
	"\u212Aelvin",
	"Alice.Example.COM",
	"example.com",
	"\u03A3\u0391\u03A3",
}

// transformerLowerInvalidInputs holds invalid UTF-8. They are not fed to the
// drift index fixture, only to the outputs golden.
var transformerLowerInvalidInputs = []string{
	"a\xffB\xc3",
	"\xed\xa0\x80X",
}

func formatTransformerLowerOutputs() []byte {
	var buf bytes.Buffer
	line := func(name string, fn FieldTransformer, arg string) {
		out := "FAIL"
		if v, ok := fn(arg); ok {
			out = strconv.QuoteToASCII(v.(string))
		}
		buf.WriteString(name + " " + strconv.QuoteToASCII(arg) + " -> " + out + "\n")
	}
	for _, in := range transformerLowerDriftInputs {
		line("ToLower", ToLower, in)
		line("EmailDomain", EmailDomain, "u@"+in)
		line("URLHost", URLHost, "https://"+in+"/p")
	}
	for _, in := range transformerLowerInvalidInputs {
		line("ToLower", ToLower, in)
		line("EmailDomain", EmailDomain, "u@"+in)
		line("URLHost", URLHost, "https://"+in+"/p")
	}
	line("EmailDomain", EmailDomain, "nodomain")
	line("URLHost", URLHost, "/relative")
	return buf.Bytes()
}

func encodeTransformerLowerDriftIndex(t *testing.T) []byte {
	t.Helper()
	return mustEncodeUncompressed(t, buildTransformerDriftIndex(t))
}

func TestTransformerLowerOutputsGolden(t *testing.T) {
	want := readTestFile(t, transformerLowerOutputsGolden)
	got := formatTransformerLowerOutputs()
	if bytes.Equal(got, want) {
		return
	}
	gotLines := strings.Split(string(got), "\n")
	wantLines := strings.Split(string(want), "\n")
	for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			t.Fatalf("outputs differ from %s at line %d:\n got: %s\nwant: %s", transformerLowerOutputsGolden, i+1, g, w)
		}
	}
	t.Fatalf("outputs differ from %s", transformerLowerOutputsGolden)
}

func TestTransformerLowerDriftIndexGolden(t *testing.T) {
	want := readTestFile(t, transformerLowerDriftGolden)
	if got := encodeTransformerLowerDriftIndex(t); !bytes.Equal(got, want) {
		t.Errorf("encoded drift index (%d bytes) differs from %s (%d bytes)", len(got), transformerLowerDriftGolden, len(want))
	}
	idx, err := Decode(want)
	if err != nil {
		t.Fatalf("Decode(%s): %v", transformerLowerDriftGolden, err)
	}
	requireTransformerDriftQueries(t, idx, "golden-decoded")
}
