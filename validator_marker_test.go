package gin

import (
	"go/ast"
	"go/parser"
	"go/token"
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
	file, err := parser.ParseFile(fset, "builder.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("ParseFile(builder.go) error = %v", err)
	}

	markerLines := make(map[int]struct{})
	for _, group := range file.Comments {
		if !isValidatorMarkerGroup(group) {
			continue
		}
		markerLines[fset.Position(group.End()).Line] = struct{}{}
	}

	seen := make(map[string]struct{})
	usedMarkerLines := make(map[int]struct{})
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		funcLine := fset.Position(fn.Pos()).Line
		markerLine := funcLine - 1
		if _, ok := markerLines[markerLine]; !ok {
			continue
		}
		usedMarkerLines[markerLine] = struct{}{}

		name := fn.Name.Name
		if _, ok := expected[name]; !ok {
			t.Fatalf("unexpected MUST_BE_CHECKED_BY_VALIDATOR marker for %s", name)
		}
		if _, ok := seen[name]; ok {
			t.Fatalf("duplicate MUST_BE_CHECKED_BY_VALIDATOR marker for %s", name)
		}
		if funcReturnsError(fn) {
			t.Fatalf("MUST_BE_CHECKED_BY_VALIDATOR function %s must not return error", name)
		}
		seen[name] = struct{}{}
	}

	for line := range markerLines {
		if _, ok := usedMarkerLines[line]; !ok {
			t.Fatalf("MUST_BE_CHECKED_BY_VALIDATOR marker at builder.go:%d must directly precede a function declaration", line)
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

func funcReturnsError(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil {
		return false
	}
	for _, field := range fn.Type.Results.List {
		if ident, ok := field.Type.(*ast.Ident); ok && ident.Name == "error" {
			return true
		}
	}
	return false
}
