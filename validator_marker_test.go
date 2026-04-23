package gin

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatorMarkersPresent(t *testing.T) {
	expected := map[string]struct{}{
		"mergeStagedPaths":          {},
		"mergeNumericObservation":   {},
		"promoteNumericPathToFloat": {},
	}

	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Glob(*.go) error = %v", err)
	}

	type markerLocation struct {
		file string
		line int
	}

	markerLines := make(map[markerLocation]struct{})
	seen := make(map[string]struct{})
	usedMarkerLines := make(map[markerLocation]struct{})

	for _, filename := range files {
		file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("ParseFile(%s) error = %v", filename, err)
		}

		for _, group := range file.Comments {
			if !isValidatorMarkerGroup(group) {
				continue
			}
			position := fset.Position(group.End())
			markerLines[markerLocation{file: filename, line: position.Line}] = struct{}{}
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			funcLine := fset.Position(fn.Pos()).Line
			location := markerLocation{file: filename, line: funcLine - 1}
			if _, ok := markerLines[location]; !ok {
				continue
			}
			usedMarkerLines[location] = struct{}{}

			name := fn.Name.Name
			if _, ok := expected[name]; !ok {
				t.Fatalf("unexpected MUST_BE_CHECKED_BY_VALIDATOR marker for %s in %s", name, filename)
			}
			if _, ok := seen[name]; ok {
				t.Fatalf("duplicate MUST_BE_CHECKED_BY_VALIDATOR marker for %s in %s", name, filename)
			}
			if funcReturnsError(fn) {
				t.Fatalf("MUST_BE_CHECKED_BY_VALIDATOR function %s in %s must not return error", name, filename)
			}
			seen[name] = struct{}{}
		}
	}

	for location := range markerLines {
		if _, ok := usedMarkerLines[location]; !ok {
			t.Fatalf("MUST_BE_CHECKED_BY_VALIDATOR marker at %s:%d must directly precede a function declaration", location.file, location.line)
		}
	}
	for name := range expected {
		if _, ok := seen[name]; !ok {
			t.Fatalf("missing MUST_BE_CHECKED_BY_VALIDATOR marker for %s", name)
		}
	}
	if len(seen) != len(expected) {
		t.Fatalf("saw %d validator markers, want %d", len(seen), len(expected))
	}
}

func isValidatorMarkerGroup(group *ast.CommentGroup) bool {
	if len(group.List) != 1 {
		return false
	}
	text := strings.TrimSpace(group.List[0].Text)
	text = strings.TrimPrefix(text, "//")
	return strings.TrimSpace(text) == "MUST_BE_CHECKED_BY_VALIDATOR"
}

func TestFuncReturnsErrorRecognizesErrorLikeReturns(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   bool
	}{
		{
			name:   "no results",
			source: "func f() {}",
		},
		{
			name:   "int only",
			source: "func f() int {}",
		},
		{
			name:   "bare error",
			source: "func f() error {}",
			want:   true,
		},
		{
			name:   "tuple error",
			source: "func f() (int, error) {}",
			want:   true,
		},
		{
			name:   "pointer error",
			source: "func f() *error {}",
			want:   true,
		},
		{
			name:   "qualified error",
			source: "func f() pkgerr.Error {}",
			want:   true,
		},
		{
			name:   "generic error",
			source: "func f() result[error] {}",
			want:   true,
		},
		{
			name:   "struct field name is not type",
			source: "func f() struct{ Error string } {}",
		},
		{
			name:   "struct field type is error",
			source: "func f() struct{ value error } {}",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := parseSingleFuncForMarkerTest(t, tt.source)
			if got := funcReturnsError(fn); got != tt.want {
				t.Fatalf("funcReturnsError(%s) = %v, want %v", tt.source, got, tt.want)
			}
		})
	}
}

func parseSingleFuncForMarkerTest(t *testing.T, source string) *ast.FuncDecl {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "snippet.go", "package gin\n"+source, 0)
	if err != nil {
		t.Fatalf("ParseFile(snippet.go) error = %v", err)
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			return fn
		}
	}
	t.Fatal("snippet did not contain a function declaration")
	return nil
}

func funcReturnsError(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil {
		return false
	}
	for _, field := range fn.Type.Results.List {
		if exprContainsErrorType(field.Type) {
			return true
		}
	}
	return false
}

func exprContainsErrorType(expr ast.Expr) bool {
	switch e := expr.(type) {
	case nil:
		return false
	case *ast.Ident:
		return isErrorTypeName(e.Name)
	case *ast.SelectorExpr:
		return isErrorTypeName(e.Sel.Name) || exprContainsErrorType(e.X)
	case *ast.StarExpr:
		return exprContainsErrorType(e.X)
	case *ast.ArrayType:
		return exprContainsErrorType(e.Elt)
	case *ast.MapType:
		return exprContainsErrorType(e.Key) || exprContainsErrorType(e.Value)
	case *ast.ChanType:
		return exprContainsErrorType(e.Value)
	case *ast.Ellipsis:
		return exprContainsErrorType(e.Elt)
	case *ast.ParenExpr:
		return exprContainsErrorType(e.X)
	case *ast.FuncType:
		return fieldListContainsErrorTypes(e.Params) || fieldListContainsErrorTypes(e.Results)
	case *ast.InterfaceType:
		return fieldListContainsErrorTypes(e.Methods)
	case *ast.StructType:
		return fieldListContainsErrorTypes(e.Fields)
	case *ast.IndexExpr:
		return exprContainsErrorType(e.X) || exprContainsErrorType(e.Index)
	case *ast.IndexListExpr:
		if exprContainsErrorType(e.X) {
			return true
		}
		for _, index := range e.Indices {
			if exprContainsErrorType(index) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func fieldListContainsErrorTypes(fields *ast.FieldList) bool {
	if fields == nil {
		return false
	}
	for _, field := range fields.List {
		if exprContainsErrorType(field.Type) {
			return true
		}
	}
	return false
}

func isErrorTypeName(name string) bool {
	return name == "error" || name == "Error"
}
