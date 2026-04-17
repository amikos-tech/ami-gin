package gin

import (
	"math"
	"strings"
	"testing"
	"time"
)

func buildPhase09DerivedRepresentationFixture(t *testing.T) (*GINIndex, *GINIndex) {
	t.Helper()

	config, err := NewConfig(
		WithToLowerTransformer("$.email", "lower"),
		WithEmailDomainTransformer("$.email", "domain"),
	)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	builder := mustNewBuilder(t, config, 4)
	docs := []string{
		`{"email":"Alice@Example.COM"}`,
		`{"email":"bob@company.io"}`,
		`{"email":"CHARLIE@EXAMPLE.COM"}`,
		`{"email":"dana@other.dev"}`,
	}
	for i, doc := range docs {
		if err := builder.AddDocument(DocID(i), []byte(doc)); err != nil {
			t.Fatalf("AddDocument(%d) error = %v", i, err)
		}
	}

	before := builder.Finalize()
	return before, mustRoundTripIndex(t, before)
}

func buildPhase09IPv4Fixture(t *testing.T, alias string) *GINIndex {
	t.Helper()

	config, err := NewConfig(WithIPv4Transformer("$.client_ip", alias))
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	builder := mustNewBuilder(t, config, 4)
	docs := []string{
		`{"client_ip":"192.168.1.10"}`,
		`{"client_ip":"192.168.2.10"}`,
		`{"client_ip":"192.168.1.200"}`,
		`{"client_ip":"10.0.0.1"}`,
	}
	for i, doc := range docs {
		if err := builder.AddDocument(DocID(i), []byte(doc)); err != nil {
			t.Fatalf("AddDocument(%d) error = %v", i, err)
		}
	}

	return builder.Finalize()
}

func TestPhase09AliasedPredicateOperators(t *testing.T) {
	before, after := buildPhase09DerivedRepresentationFixture(t)

	cases := []struct {
		label      string
		predicates []Predicate
		want       []int
	}{
		{
			label:      `IN("$.email", As("domain", "example.com"), As("domain", "company.io"))`,
			predicates: []Predicate{IN("$.email", As("domain", "example.com"), As("domain", "company.io"))},
			want:       []int{0, 1, 2},
		},
		{
			label:      `NIN("$.email", As("domain", "example.com"), As("domain", "company.io"))`,
			predicates: []Predicate{NIN("$.email", As("domain", "example.com"), As("domain", "company.io"))},
			want:       []int{3},
		},
		{
			label:      `NE("$.email", As("lower", "alice@example.com"))`,
			predicates: []Predicate{NE("$.email", As("lower", "alice@example.com"))},
			want:       []int{1, 2, 3},
		},
		{
			label: `Predicate{Path: "$.email", Operator: OpContains, Value: As("lower", "example")}`,
			predicates: []Predicate{{
				Path:     "$.email",
				Operator: OpContains,
				Value:    As("lower", "example"),
			}},
			want: []int{0, 2},
		},
		{
			label: `Predicate{Path: "$.email", Operator: OpRegex, Value: As("lower", "example\\.com")}`,
			predicates: []Predicate{{
				Path:     "$.email",
				Operator: OpRegex,
				Value:    As("lower", `example\.com`),
			}},
			want: []int{0, 2},
		},
	}

	for _, tc := range cases {
		requirePredicateResult(t, before, tc.predicates, tc.want, "before "+tc.label)
		requirePredicateResult(t, after, tc.predicates, tc.want, "after "+tc.label)
	}
}

func TestPhase09UnknownAliasFallsBackToAllRowGroups(t *testing.T) {
	before, after := buildPhase09DerivedRepresentationFixture(t)

	predicates := []Predicate{EQ("$.email", As("missing_alias", "alice@example.com"))}
	want := []int{0, 1, 2, 3}

	requirePredicateResult(t, before, predicates, want, "before missing alias")
	requirePredicateResult(t, after, predicates, want, "after missing alias")
}

func TestPhase09MixedAliasSlicesFallBackToAllRowGroups(t *testing.T) {
	before, after := buildPhase09DerivedRepresentationFixture(t)

	cases := []struct {
		label      string
		predicates []Predicate
		want       []int
	}{
		{
			label:      `IN("$.email", As("lower", "alice@example.com"), As("domain", "example.com"))`,
			predicates: []Predicate{IN("$.email", As("lower", "alice@example.com"), As("domain", "example.com"))},
			want:       []int{0, 1, 2, 3},
		},
		{
			label:      `NIN("$.email", As("lower", "alice@example.com"), As("domain", "example.com"))`,
			predicates: []Predicate{NIN("$.email", As("lower", "alice@example.com"), As("domain", "example.com"))},
			want:       []int{0, 1, 2, 3},
		},
	}

	for _, tc := range cases {
		requirePredicateResult(t, before, tc.predicates, tc.want, "before "+tc.label)
		requirePredicateResult(t, after, tc.predicates, tc.want, "after "+tc.label)
	}
}

func TestPhase09InSubnetUsesDerivedAliases(t *testing.T) {
	defaultPredicates := InSubnet("$.client_ip", "192.168.1.0/24")
	if len(defaultPredicates) != 2 {
		t.Fatalf("InSubnet() len = %d, want 2", len(defaultPredicates))
	}

	for i, predicate := range defaultPredicates {
		value, ok := predicate.Value.(RepresentationValue)
		if !ok {
			t.Fatalf("InSubnet() predicate %d value type = %T, want RepresentationValue", i, predicate.Value)
		}
		if value.Alias != "ipv4_int" {
			t.Fatalf("InSubnet() predicate %d alias = %q, want %q", i, value.Alias, "ipv4_int")
		}
	}

	idx := buildPhase09IPv4Fixture(t, "ipv4_int")
	requirePredicateResult(t, idx, defaultPredicates, []int{0, 2}, `InSubnet("$.client_ip", "192.168.1.0/24")`)

	customPredicates := InSubnetAs("$.client_ip", "client_ip_num", "192.168.1.0/24")
	for i, predicate := range customPredicates {
		value, ok := predicate.Value.(RepresentationValue)
		if !ok {
			t.Fatalf("InSubnetAs() predicate %d value type = %T, want RepresentationValue", i, predicate.Value)
		}
		if value.Alias != "client_ip_num" {
			t.Fatalf("InSubnetAs() predicate %d alias = %q, want %q", i, value.Alias, "client_ip_num")
		}
	}

	customIdx := buildPhase09IPv4Fixture(t, "client_ip_num")
	requirePredicateResult(t, customIdx, customPredicates, []int{0, 2}, `InSubnetAs("$.client_ip", "client_ip_num", "192.168.1.0/24")`)
}

func TestPhase09CompanionTransformFailuresStayLenient(t *testing.T) {
	config, err := NewConfig(WithISODateTransformer("$.timestamp", "epoch_ms"))
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	builder := mustNewBuilder(t, config, 2)
	docs := []string{
		`{"timestamp":"not-a-date"}`,
		`{"timestamp":"2024-07-10T09:00:00Z"}`,
	}
	for i, doc := range docs {
		if err := builder.AddDocument(DocID(i), []byte(doc)); err != nil {
			t.Fatalf("AddDocument(%d) error = %v", i, err)
		}
	}

	before := builder.Finalize()
	after := mustRoundTripIndex(t, before)

	july2024 := float64(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC).UnixMilli())
	cases := []struct {
		label      string
		predicates []Predicate
		want       []int
	}{
		{
			label:      `raw EQ("$.timestamp", "not-a-date")`,
			predicates: []Predicate{EQ("$.timestamp", "not-a-date")},
			want:       []int{0},
		},
		{
			label:      `alias GTE("$.timestamp", As("epoch_ms", july2024))`,
			predicates: []Predicate{GTE("$.timestamp", As("epoch_ms", july2024))},
			want:       []int{1},
		},
	}

	for _, tc := range cases {
		requirePredicateResult(t, before, tc.predicates, tc.want, "before "+tc.label)
		requirePredicateResult(t, after, tc.predicates, tc.want, "after "+tc.label)
	}
}

func TestPhase09FinalizeOmitsNeverMaterializedRepresentations(t *testing.T) {
	config, err := NewConfig(WithISODateTransformer("$.timestamp", "epoch_ms"))
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	builder := mustNewBuilder(t, config, 2)
	docs := []string{
		`{"timestamp":"not-a-date"}`,
		`{"timestamp":"also-not-a-date"}`,
	}
	for i, doc := range docs {
		if err := builder.AddDocument(DocID(i), []byte(doc)); err != nil {
			t.Fatalf("AddDocument(%d) error = %v", i, err)
		}
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Finalize() panicked: %v", r)
		}
	}()

	before := builder.Finalize()
	if got := before.Representations("$.timestamp"); got != nil {
		t.Fatalf("Representations($.timestamp) = %v, want nil when no companion values materialize", got)
	}
	if len(before.representations) != 0 {
		t.Fatalf("len(idx.representations) = %d, want 0 when no companion values materialize", len(before.representations))
	}

	after := mustRoundTripIndex(t, before)
	requirePredicateResult(t, before, []Predicate{EQ("$.timestamp", "not-a-date")}, []int{0}, `before raw EQ("$.timestamp", "not-a-date")`)
	requirePredicateResult(t, after, []Predicate{EQ("$.timestamp", "also-not-a-date")}, []int{1}, `after raw EQ("$.timestamp", "also-not-a-date")`)
}

func TestPhase09RejectsInvalidAliases(t *testing.T) {
	for _, alias := range []string{"", "lower#extra"} {
		t.Run(alias, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("As(%q, ...) should panic for invalid alias", alias)
				}
			}()
			_ = As(alias, "value")
		})
	}

	_, err := NewConfig(WithCustomTransformer("$.email", "lower#extra", func(value any) (any, bool) {
		return strings.ToLower(value.(string)), true
	}))
	if err == nil {
		t.Fatal("NewConfig() error = nil, want invalid alias rejection")
	}
	if !strings.Contains(err.Error(), "alias") {
		t.Fatalf("NewConfig() error = %v, want alias validation", err)
	}
}

func TestPhase09ConfigValidationErrors(t *testing.T) {
	if _, err := NewConfig(WithFieldTransformer("$.email", ToLower)); err == nil {
		t.Fatal("NewConfig(WithFieldTransformer) error = nil, want deprecation error")
	} else if !strings.Contains(err.Error(), "no longer supported") {
		t.Fatalf("NewConfig(WithFieldTransformer) error = %v, want deprecation message", err)
	}

	cfg := DefaultConfig()
	err := cfg.addRepresentation("$.email", "lower", NewTransformerSpec("$.email", TransformerToLower, nil), true, nil)
	if err == nil {
		t.Fatal("addRepresentation(..., nil) error = nil, want transformer function validation")
	}
	if !strings.Contains(err.Error(), "requires a function") {
		t.Fatalf("addRepresentation(..., nil) error = %v, want function validation", err)
	}

	if _, err := NewConfig(WithNumericBucketTransformer("$.score", "bucket", math.NaN())); err == nil {
		t.Fatal("NewConfig(WithNumericBucketTransformer NaN) error = nil, want marshal error")
	} else if !strings.Contains(err.Error(), "marshal transformer params") {
		t.Fatalf("NewConfig(WithNumericBucketTransformer NaN) error = %v, want marshal transformer params", err)
	}
}

func TestPhase09RepresentationsInvalidPathReturnsNil(t *testing.T) {
	before, after := buildPhase09DerivedRepresentationFixture(t)

	for _, idx := range []*GINIndex{before, after} {
		if got := idx.Representations("$.email[0]"); got != nil {
			t.Fatalf("Representations($.email[0]) = %v, want nil for invalid path", got)
		}
	}
}
