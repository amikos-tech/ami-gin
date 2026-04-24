package gin

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestHardIngestFunctionsDoNotReturnPlainErrors(t *testing.T) {
	targets := map[string]struct{}{
		"stageScalarToken":              {},
		"stageMaterializedValue":        {},
		"stageCompanionRepresentations": {},
		"stageJSONNumberLiteral":        {},
		"stageNativeNumeric":            {},
		"stageNumericObservation":       {},
		"validateStagedPaths":           {},
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "builder.go", nil, 0)
	if err != nil {
		t.Fatalf("parse builder.go: %v", err)
	}

	seen := make(map[string]bool, len(targets))
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		name := fn.Name.Name
		if _, ok := targets[name]; !ok {
			continue
		}
		seen[name] = true
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			ret, ok := node.(*ast.ReturnStmt)
			if !ok {
				return true
			}
			for _, result := range ret.Results {
				if containsIngestErrorWrapper(result) {
					continue
				}
				if containsPlainErrorsCall(result) {
					t.Errorf("%s returns a plain errors.* call at %s", name, fset.Position(result.Pos()))
				}
			}
			return true
		})
	}

	for name := range targets {
		if !seen[name] {
			t.Errorf("builder.go target function %s was not found", name)
		}
	}
}

func containsIngestErrorWrapper(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			if ident, ok := call.Fun.(*ast.Ident); ok {
				switch ident.Name {
				case "newIngestError", "newIngestErrorString":
					found = true
					return false
				}
			}
		}
		return !found
	})
	return found
}

func containsPlainErrorsCall(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok && isPlainErrorsCall(call) {
			found = true
			return false
		}
		return !found
	})
	return found
}

func isPlainErrorsCall(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "errors" {
		return false
	}
	switch selector.Sel.Name {
	case "New", "Errorf", "Wrap", "Wrapf":
		return true
	default:
		return false
	}
}

// This guard intentionally catches direct plain error returns in the current
// hard-ingest functions only. New hard-ingest functions still need behavior
// matrix coverage because this scoped test does not discover new surfaces.
