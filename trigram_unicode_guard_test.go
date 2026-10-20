package gin

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/pkg/errors"
)

// The trigram Unicode guard (#99) walks the call graph from the gram-path
// roots over every non-test file of the package and fails when a reachable
// function or package variable reads the toolchain's Unicode tables. Method
// calls on non-import selectors link to every method of that name, whatever
// the receiver: a deliberate over-approximation.

var trigramGuardRootMethods = []string{"TrigramIndex.Add", "TrigramIndex.Search", "TrigramIndex.generateNGrams"}

var trigramGuardRootFuncs = []string{"GenerateNGrams", "isWhitespaceOnly", "isPunctuationOnly", "foldsUnderToLower"}

// trigramGuardUnicodeAllowed lists the unicode selectors that read only the
// tables passed to them or are constants.
var trigramGuardUnicodeAllowed = map[string]bool{
	"Is": true, "RangeTable": true, "Range16": true, "Range32": true, "MaxRune": true, "MaxLatin1": true,
}

var trigramGuardStringsDenied = map[string]bool{
	"ToLower": true, "ToUpper": true, "ToTitle": true, "ToLowerSpecial": true, "ToUpperSpecial": true,
	"ToTitleSpecial": true, "Title": true, "Map": true, "EqualFold": true, "Fields": true, "TrimSpace": true,
}

var trigramGuardBuildIgnore = regexp.MustCompile(`^//go:build\b.*\bignore\b`)

type trigramGuardFile struct {
	name    string
	ast     *ast.File
	imports map[string]string // local name -> import path
	dots    []string          // dot-imported paths
	paths   []string          // every imported path
}

type trigramGuardNode struct {
	name string
	body ast.Node
	file *trigramGuardFile
}

type trigramGuardResult struct {
	reachable  map[string]bool
	violations []string
}

func newTrigramGuardFile(name string, f *ast.File) *trigramGuardFile {
	gf := &trigramGuardFile{name: name, ast: f, imports: map[string]string{}}
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		gf.paths = append(gf.paths, path)
		local := path[strings.LastIndex(path, "/")+1:]
		if imp.Name != nil {
			local = imp.Name.Name
		}
		switch local {
		case "_":
		case ".":
			gf.dots = append(gf.dots, path)
		default:
			gf.imports[local] = path
		}
	}
	return gf
}

func trigramGuardRecvName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return trigramGuardRecvName(e.X)
	case *ast.IndexExpr:
		return trigramGuardRecvName(e.X)
	case *ast.IndexListExpr:
		return trigramGuardRecvName(e.X)
	case *ast.Ident:
		return e.Name
	}
	return ""
}

// checkTrigramUnicodeGuard runs the guard over parsed files.
func checkTrigramUnicodeGuard(fset *token.FileSet, files []*trigramGuardFile) (trigramGuardResult, error) {
	nodes := map[string]*trigramGuardNode{} // funcs, vars and consts by name; methods by Recv.Name
	methodsByName := map[string][]string{}
	for _, gf := range files {
		for _, decl := range gf.ast.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Body == nil {
					continue
				}
				name := d.Name.Name
				if d.Recv != nil && len(d.Recv.List) > 0 {
					name = trigramGuardRecvName(d.Recv.List[0].Type) + "." + d.Name.Name
					methodsByName[d.Name.Name] = append(methodsByName[d.Name.Name], name)
				}
				nodes[name] = &trigramGuardNode{name: name, body: d.Body, file: gf}
			case *ast.GenDecl:
				if d.Tok != token.VAR && d.Tok != token.CONST {
					continue
				}
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, ident := range vs.Names {
						if ident.Name == "_" {
							continue
						}
						nodes[ident.Name] = &trigramGuardNode{name: ident.Name, body: vs, file: gf}
					}
				}
			}
		}
	}

	var res trigramGuardResult
	res.reachable = map[string]bool{}
	var queue []string
	for _, root := range append(append([]string(nil), trigramGuardRootMethods...), trigramGuardRootFuncs...) {
		if _, ok := nodes[root]; !ok {
			return res, errors.Errorf("gram-path root %s not found", root)
		}
		res.reachable[root] = true
		queue = append(queue, root)
	}
	visit := func(name string) {
		if _, ok := nodes[name]; ok && !res.reachable[name] {
			res.reachable[name] = true
			queue = append(queue, name)
		}
	}

	importsChecked := map[*trigramGuardFile]bool{}
	for len(queue) > 0 {
		node := nodes[queue[0]]
		queue = queue[1:]
		gf := node.file
		report := func(pos token.Pos, msg string) {
			p := fset.Position(pos)
			res.violations = append(res.violations, fmt.Sprintf("%s (%s:%d): %s", node.name, p.Filename, p.Line, msg))
		}

		// File-level import rules, reported once per file.
		if !importsChecked[gf] {
			importsChecked[gf] = true
			for _, path := range gf.dots {
				if path == "unicode" || path == "strings" || path == "bytes" {
					report(gf.ast.Package, "file dot-imports "+path)
				}
			}
			for _, path := range gf.paths {
				if path == "golang.org/x/text" || strings.HasPrefix(path, "golang.org/x/text/") {
					report(gf.ast.Package, "file imports "+path)
				}
			}
		}

		var inspect func(n ast.Node) bool
		inspect = func(n ast.Node) bool {
			switch e := n.(type) {
			case *ast.SelectorExpr:
				if x, ok := e.X.(*ast.Ident); ok {
					if path, isImport := gf.imports[x.Name]; isImport {
						sel := e.Sel.Name
						switch path {
						case "unicode":
							if !trigramGuardUnicodeAllowed[sel] && (sel != "SimpleFold" || node.name != "foldsUnderToLower") {
								report(e.Pos(), "uses unicode."+sel)
							}
						case "strings", "bytes":
							if trigramGuardStringsDenied[sel] {
								report(e.Pos(), "uses "+path+"."+sel)
							}
						}
						return false
					}
				}
				for _, m := range methodsByName[e.Sel.Name] {
					visit(m)
				}
				ast.Inspect(e.X, inspect)
				return false
			case *ast.Ident:
				visit(e.Name)
			}
			return true
		}
		ast.Inspect(node.body, inspect)
	}
	sort.Strings(res.violations)
	return res, nil
}

func loadTrigramGuardPackage(t *testing.T) (*token.FileSet, []*trigramGuardFile) {
	t.Helper()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	fset := token.NewFileSet()
	var files []*trigramGuardFile
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		f, err := parser.ParseFile(fset, name, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ignored := false
		for _, group := range f.Comments {
			if group.Pos() >= f.Package {
				break
			}
			for _, c := range group.List {
				if trigramGuardBuildIgnore.MatchString(c.Text) {
					ignored = true
				}
			}
		}
		if ignored {
			continue
		}
		files = append(files, newTrigramGuardFile(name, f))
	}
	return fset, files
}

func TestTrigramUnicodeASTGuard(t *testing.T) {
	fset, files := loadTrigramGuardPackage(t)
	for _, gf := range files {
		if gf.name == "gen_trigram_unicode.go" {
			t.Fatalf("the //go:build ignore generator must not be scanned")
		}
	}
	res, err := checkTrigramUnicodeGuard(fset, files)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range res.violations {
		t.Errorf("toolchain Unicode table on the gram path: %s", v)
	}
	for _, name := range []string{
		"trigramLowerRune", "trigramLowerRunes", "trigramIsSpace", "trigramIsPunctOrSymbol",
		"trigramUnicodeTokenSupported", "trigramLowerRanges", "trigramSpaceTable", "trigramPunctOrSymbolTable",
	} {
		if !res.reachable[name] {
			t.Errorf("%s is not reachable from the gram-path roots; the guard does not walk into it", name)
		}
	}
}

// trigramGuardStubs declares every root; bodies overrides chosen root bodies.
func trigramGuardStubs(bodies map[string]string) string {
	body := func(name, def string) string {
		if b, ok := bodies[name]; ok {
			return b
		}
		return def
	}
	return `
type TrigramIndex struct{}

func (ti *TrigramIndex) Add(value string, rgID int) {` + body("Add", "") + `}
func (ti *TrigramIndex) Search(pattern string) {` + body("Search", "") + `}
func (ti *TrigramIndex) generateNGrams(runes []rune) {` + body("generateNGrams", "") + `}
func GenerateNGrams(text string) {` + body("GenerateNGrams", "") + `}
func isWhitespaceOnly(s string) bool {` + body("isWhitespaceOnly", "return false") + `}
func isPunctuationOnly(s string) bool {` + body("isPunctuationOnly", "return false") + `}
func foldsUnderToLower(runes []rune) bool {` + body("foldsUnderToLower", "return false") + `}
`
}

func TestTrigramUnicodeASTGuardSelfTest(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  []string // substrings of the only expected violation; nil means none
	}{
		{
			name: "aliased unicode import in a helper called from Search",
			files: map[string]string{"a.go": `package gin
import u "unicode"
` + trigramGuardStubs(map[string]string{"Search": "_ = lower('A')"}) + `
func lower(r rune) rune { return u.ToLower(r) }
`},
			want: []string{"lower (a.go:", "uses unicode.ToLower"},
		},
		{
			name: "strings.ToLower in a helper declared in a second file",
			files: map[string]string{
				"a.go": "package gin\n" + trigramGuardStubs(map[string]string{"Add": "_ = norm(value)"}),
				"textnorm.go": `package gin
import "strings"
func norm(s string) string { return strings.ToLower(s) }
`,
			},
			want: []string{"norm (textnorm.go:3)", "uses strings.ToLower"},
		},
		{
			name: "dot import of unicode",
			files: map[string]string{
				"a.go": "package gin\n" + trigramGuardStubs(map[string]string{"isWhitespaceOnly": "return space(' ')"}),
				"b.go": `package gin
import . "unicode"
func space(r rune) bool { return IsSpace(r) }
`,
			},
			want: []string{"space (b.go:1)", "file dot-imports unicode"},
		},
		{
			name: "golang.org/x/text import in a file holding a reachable func",
			files: map[string]string{
				"a.go": "package gin\n" + trigramGuardStubs(map[string]string{"GenerateNGrams": "fold(text)"}),
				"fold.go": `package gin
import "golang.org/x/text/cases"
var _ cases.Caser
func fold(s string) {}
`,
			},
			want: []string{"fold (fold.go:1)", "file imports golang.org/x/text/cases"},
		},
		{
			name: "unicode.Version reference",
			files: map[string]string{"a.go": `package gin
import "unicode"
var tableVersion = unicode.Version
` + trigramGuardStubs(map[string]string{"Search": "_ = tableVersion"})},
			want: []string{"tableVersion (a.go:3)", "uses unicode.Version"},
		},
		{
			name: "unicode.SimpleFold outside foldsUnderToLower",
			files: map[string]string{"a.go": `package gin
import "unicode"
` + trigramGuardStubs(map[string]string{"isPunctuationOnly": "return unicode.SimpleFold('a') == 'A'"})},
			want: []string{"isPunctuationOnly (a.go:", "uses unicode.SimpleFold"},
		},
		{
			name: "method-name edge to a method on another receiver",
			files: map[string]string{"a.go": `package gin
import "strings"
type normer struct{}
func (normer) norm(s string) string { return strings.ToLower(s) }
` + trigramGuardStubs(map[string]string{"Search": "var x interface{ norm(string) string }; _ = x.norm(pattern)"})},
			want: []string{"normer.norm (a.go:4)", "uses strings.ToLower"},
		},
		{
			name: "unreachable forbidden call is not reported",
			files: map[string]string{"a.go": `package gin
import "strings"
func unrelated(s string) string { return strings.ToLower(s) }
` + trigramGuardStubs(nil)},
		},
		{
			name: "unicode.SimpleFold inside foldsUnderToLower is allowed",
			files: map[string]string{"a.go": `package gin
import "unicode"
` + trigramGuardStubs(map[string]string{"foldsUnderToLower": "return unicode.SimpleFold(runes[0]) == runes[0]"})},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fset := token.NewFileSet()
			names := make([]string, 0, len(c.files))
			for name := range c.files {
				names = append(names, name)
			}
			sort.Strings(names)
			var files []*trigramGuardFile
			for _, name := range names {
				f, err := parser.ParseFile(fset, name, c.files[name], parser.ParseComments)
				if err != nil {
					t.Fatalf("parse %s: %v", name, err)
				}
				files = append(files, newTrigramGuardFile(name, f))
			}
			res, err := checkTrigramUnicodeGuard(fset, files)
			if err != nil {
				t.Fatal(err)
			}
			if c.want == nil {
				if len(res.violations) != 0 {
					t.Fatalf("unexpected violations: %q", res.violations)
				}
				return
			}
			if len(res.violations) != 1 {
				t.Fatalf("violations = %q, want exactly one containing %q", res.violations, c.want)
			}
			for _, want := range c.want {
				if !strings.Contains(res.violations[0], want) {
					t.Errorf("violation %q does not contain %q", res.violations[0], want)
				}
			}
		})
	}
}
