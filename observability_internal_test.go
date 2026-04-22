package gin

import (
	"context"
	stderrors "errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"testing"

	pkgerrors "github.com/pkg/errors"
)

func TestErrorTypeNormalizersStayInSync(t *testing.T) {
	t.Parallel()

	telemetryKinds := loadNormalizeErrorTypeCases(t, "telemetry/boundary.go")
	loggingKinds := loadNormalizeErrorTypeCases(t, "logging/attrs.go")

	if diff := diffStringSets(telemetryKinds, loggingKinds); diff != "" {
		t.Fatalf("normalizeErrorType vocabularies diverged:\n%s", diff)
	}
}

func TestClassifySerializeError(t *testing.T) {
	t.Parallel()

	allowedKinds := loadNormalizeErrorTypeCases(t, "telemetry/boundary.go")
	// The classifier may legitimately return "" when there is no error.
	// The normalized vocabulary itself can also contain reserved values such as
	// "not_found" that these particular classifiers do not currently emit.
	allowedKinds[""] = true

	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil", err: nil, want: ""},
		{name: "canceled", err: context.Canceled, want: "other"},
		{name: "deadline", err: context.DeadlineExceeded, want: "other"},
		{name: "invalid format", err: pkgerrors.Wrap(ErrInvalidFormat, "decode"), want: "invalid_format"},
		{name: "version mismatch", err: pkgerrors.Wrap(ErrVersionMismatch, "decode"), want: "deserialization"},
		{name: "integrity", err: pkgerrors.Wrap(ErrDecodedSizeExceedsLimit, "decode"), want: "integrity"},
		{name: "other", err: stderrors.New("boom"), want: "other"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifySerializeError(tc.err)
			if got != tc.want {
				t.Fatalf("classifySerializeError(%v) = %q; want %q", tc.err, got, tc.want)
			}
			if !allowedKinds[got] {
				t.Fatalf("classifySerializeError(%v) = %q; want frozen vocabulary member", tc.err, got)
			}
		})
	}
}

func TestClassifyParquetError(t *testing.T) {
	t.Parallel()

	allowedKinds := loadNormalizeErrorTypeCases(t, "telemetry/boundary.go")
	allowedKinds[""] = true

	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil", err: nil, want: ""},
		{name: "canceled", err: context.Canceled, want: "other"},
		{name: "deadline", err: context.DeadlineExceeded, want: "other"},
		{name: "not found sentinel", err: pkgerrors.Wrap(os.ErrNotExist, "open parquet"), want: "io"},
		{name: "missing column", err: stderrors.New(`column "json" not found in parquet file`), want: "config"},
		{name: "open failure", err: stderrors.New("open parquet file: permission denied"), want: "io"},
		{name: "builder failure", err: stderrors.New("create builder: bad config"), want: "config"},
		{name: "other", err: stderrors.New("unexpected"), want: "other"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyParquetError(tc.err)
			if got != tc.want {
				t.Fatalf("classifyParquetError(%v) = %q; want %q", tc.err, got, tc.want)
			}
			if !allowedKinds[got] {
				t.Fatalf("classifyParquetError(%v) = %q; want frozen vocabulary member", tc.err, got)
			}
		})
	}
}

func loadNormalizeErrorTypeCases(t *testing.T, path string) map[string]bool {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "normalizeErrorType" || fn.Body == nil {
			continue
		}
		for _, stmt := range fn.Body.List {
			switchStmt, ok := stmt.(*ast.SwitchStmt)
			if !ok {
				continue
			}
			kinds := make(map[string]bool)
			for _, clauseStmt := range switchStmt.Body.List {
				clause, ok := clauseStmt.(*ast.CaseClause)
				if !ok {
					continue
				}
				for _, expr := range clause.List {
					lit, ok := expr.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					value, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatalf("unquote %s in %s: %v", lit.Value, path, err)
					}
					kinds[value] = true
				}
			}
			if len(kinds) == 0 {
				t.Fatalf("normalizeErrorType in %s has no string cases", path)
			}
			return kinds
		}
		t.Fatalf("normalizeErrorType in %s has no switch statement", path)
	}

	t.Fatalf("normalizeErrorType not found in %s", path)
	return nil
}

func diffStringSets(a, b map[string]bool) string {
	var onlyA []string
	for key := range a {
		if !b[key] {
			onlyA = append(onlyA, key)
		}
	}
	var onlyB []string
	for key := range b {
		if !a[key] {
			onlyB = append(onlyB, key)
		}
	}

	sort.Strings(onlyA)
	sort.Strings(onlyB)

	if len(onlyA) == 0 && len(onlyB) == 0 {
		return ""
	}

	return "only in first: " + joinOrNone(onlyA) + "\nonly in second: " + joinOrNone(onlyB)
}

func joinOrNone(vs []string) string {
	if len(vs) == 0 {
		return "<none>"
	}
	return strconv.Quote(vs[0]) + concatQuotedRest(vs[1:])
}

func concatQuotedRest(vs []string) string {
	out := ""
	for _, v := range vs {
		out += ", " + strconv.Quote(v)
	}
	return out
}
