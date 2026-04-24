package gin

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestHardIngestFunctionsDoNotReturnPlainErrors(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "builder.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse builder.go: %v", err)
	}

	targets := hardIngestFunctions(file)
	if len(targets) == 0 {
		t.Fatal("hard-ingest guard found no target functions in builder.go")
	}

	for _, fn := range targets {
		name := fn.Name.Name
		tainted := make(map[string]bool)
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.AssignStmt:
				trackBadAssignments(tainted, n.Lhs, n.Rhs)
				return true
			case *ast.DeclStmt:
				decl, ok := n.Decl.(*ast.GenDecl)
				if !ok || decl.Tok != token.VAR {
					return true
				}
				for _, spec := range decl.Specs {
					valueSpec, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					trackBadAssignments(tainted, identExprs(valueSpec.Names), valueSpec.Values)
				}
				return true
			}

			ret, ok := node.(*ast.ReturnStmt)
			if !ok {
				return true
			}
			for _, result := range ret.Results {
				if containsIngestErrorWrapper(result) {
					continue
				}
				if containsDeniedErrorConstruction(result) {
					t.Errorf("%s returns a denied plain error construction at %s", name, fset.Position(result.Pos()))
					continue
				}
				if ident, ok := result.(*ast.Ident); ok && tainted[ident.Name] {
					t.Errorf("%s returns identifier %q backed by a denied plain error construction at %s", name, ident.Name, fset.Position(result.Pos()))
				}
			}
			return true
		})
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

func containsDeniedErrorConstruction(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.CallExpr:
			if isDeniedErrorCall(n) {
				found = true
				return false
			}
		case *ast.UnaryExpr:
			if n.Op == token.AND && isIngestErrorComposite(n.X) {
				found = true
				return false
			}
		case *ast.CompositeLit:
			if isIngestErrorComposite(n) {
				found = true
				return false
			}
		}
		return !found
	})
	return found
}

func isDeniedErrorCall(call *ast.CallExpr) bool {
	if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
		pkg, ok := selector.X.(*ast.Ident)
		if !ok {
			return false
		}
		switch pkg.Name {
		case "errors":
			switch selector.Sel.Name {
			case "New", "Errorf", "Wrap", "Wrapf", "WithMessage", "WithStack":
				return true
			}
		case "fmt":
			return selector.Sel.Name == "Errorf"
		}
	}
	return false
}

func isIngestErrorComposite(expr ast.Expr) bool {
	composite, ok := expr.(*ast.CompositeLit)
	if !ok {
		return false
	}
	switch typ := composite.Type.(type) {
	case *ast.Ident:
		return typ.Name == "IngestError"
	case *ast.SelectorExpr:
		return typ.Sel.Name == "IngestError"
	default:
		return false
	}
}

func hardIngestFunctions(file *ast.File) []*ast.FuncDecl {
	targets := make([]*ast.FuncDecl, 0)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !isGINBuilderMethod(fn) {
			continue
		}
		if strings.HasPrefix(fn.Name.Name, "stage") || hasHardIngestDirective(fn) {
			targets = append(targets, fn)
		}
	}
	return targets
}

func isGINBuilderMethod(fn *ast.FuncDecl) bool {
	if fn.Recv == nil || len(fn.Recv.List) != 1 {
		return false
	}
	star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	ident, ok := star.X.(*ast.Ident)
	return ok && ident.Name == "GINBuilder"
}

func hasHardIngestDirective(fn *ast.FuncDecl) bool {
	if fn.Doc == nil {
		return false
	}
	for _, comment := range fn.Doc.List {
		if strings.Contains(comment.Text, "+hard-ingest") {
			return true
		}
	}
	return false
}

func trackBadAssignments(tainted map[string]bool, lhs, rhs []ast.Expr) {
	for i, left := range lhs {
		ident, ok := left.(*ast.Ident)
		if !ok || ident.Name == "_" {
			continue
		}
		if i >= len(rhs) {
			delete(tainted, ident.Name)
			continue
		}
		if containsDeniedErrorConstruction(rhs[i]) {
			tainted[ident.Name] = true
			continue
		}
		delete(tainted, ident.Name)
	}
}

func identExprs(idents []*ast.Ident) []ast.Expr {
	exprs := make([]ast.Expr, 0, len(idents))
	for _, ident := range idents {
		exprs = append(exprs, ident)
	}
	return exprs
}

// This guard auto-discovers builder stage* methods plus any method annotated
// with // +hard-ingest. Behavior-matrix tests still cover semantic outcomes.
