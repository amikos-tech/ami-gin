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

// The trigram Unicode guard (#99) fails when the gram path reads the
// toolchain's Unicode tables. The roots are every declaration (funcs,
// methods, types, vars and consts) in the gram-path files, the query-side
// callers evaluateContains and evaluateRegex, and every other function that
// feeds a trigram index: it calls Add or Search on a trigram index, or calls a
// func declared in a gram-path file. From the roots the guard walks every
// non-test file of the package. An identifier links to the declaration of
// that name, so the fields of a type used by reachable code are checked too.
// Method calls on non-import selectors link to every method of that name,
// whatever the receiver: a deliberate over-approximation.

var trigramGuardRequiredRoots = []string{
	"TrigramIndex.Add", "TrigramIndex.Search", "TrigramIndex.generateNGrams", "GenerateNGrams",
	"isWhitespaceOnly", "isPunctuationOnly", "foldsUnderToLower",
	"GINIndex.evaluateContains", "GINIndex.evaluateRegex",
}

const trigramGuardUnicodePkg = "unicode"

// trigramGuardTablesFile holds the frozen tables; only its package vars may
// be passed to unicode.Is and unicode.In.
const trigramGuardTablesFile = "trigram_unicode_tables.go"

// trigramGuardGramFile reports whether every declaration of the file is a
// guard root.
func trigramGuardGramFile(name string) bool {
	if strings.HasSuffix(name, "_test.go") {
		return false
	}
	return name == "trigram.go" || name == "regex.go" || strings.HasPrefix(name, "trigram_unicode")
}

// trigramGuardUnicodeAllowed lists the unicode selectors that are types of
// caller-supplied tables or constants. unicode.Is and unicode.In are checked
// separately: only direct calls with frozen tables are allowed.
// unicode.SimpleFold is allowed only in regex.go's foldsUnderToLower.
var trigramGuardUnicodeAllowed = map[string]bool{
	"RangeTable": true, "Range16": true, "Range32": true,
	"MaxRune": true, "MaxLatin1": true, "MaxASCII": true, "ReplacementChar": true,
}

// trigramGuardStringsDenied applies to strings and bytes. Map is denied
// whatever its mapping func: use a loop over the frozen lookups instead.
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
	decl ast.Node
	file *trigramGuardFile
}

type trigramGuardResult struct {
	reachable  map[string]bool
	feeders    map[string]bool
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

// trigramGuardFuncName returns Name for a func and Recv.Name for a method.
func trigramGuardFuncName(d *ast.FuncDecl) string {
	if d.Recv != nil && len(d.Recv.List) > 0 {
		return trigramGuardRecvName(d.Recv.List[0].Type) + "." + d.Name.Name
	}
	return d.Name.Name
}

// trigramGuardMentionsTrigram reports whether n names a trigram index, such
// as pd.trigrams or idx.TrigramIndexes[id].
func trigramGuardMentionsTrigram(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && strings.Contains(strings.ToLower(id.Name), "trigram") {
			found = true
		}
		return !found
	})
	return found
}

// trigramGuardFeeds reports whether fn passes values into a trigram index:
// it calls Add or Search on a trigram index, or calls one of gramFuncs.
func trigramGuardFeeds(fn *ast.FuncDecl, gramFuncs map[string]bool) bool {
	if fn.Body == nil {
		return false
	}
	locals := map[string]bool{} // locals assigned from a trigram index
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if as, ok := n.(*ast.AssignStmt); ok {
			for _, rhs := range as.Rhs {
				if trigramGuardMentionsTrigram(rhs) {
					for _, lhs := range as.Lhs {
						if id, ok := lhs.(*ast.Ident); ok {
							locals[id.Name] = true
						}
					}
				}
			}
		}
		return true
	})
	feeds := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return !feeds
		}
		switch f := call.Fun.(type) {
		case *ast.Ident:
			feeds = feeds || gramFuncs[f.Name]
		case *ast.SelectorExpr:
			if f.Sel.Name == "Add" || f.Sel.Name == "Search" {
				id, isIdent := f.X.(*ast.Ident)
				feeds = feeds || trigramGuardMentionsTrigram(f.X) || (isIdent && locals[id.Name])
			}
		}
		return !feeds
	})
	return feeds
}

// checkTrigramUnicodeGuard runs the guard over parsed files.
func checkTrigramUnicodeGuard(fset *token.FileSet, files []*trigramGuardFile) (trigramGuardResult, error) {
	nodes := map[string]*trigramGuardNode{} // funcs, types, vars and consts by name; methods by Recv.Name
	methodsByName := map[string][]string{}
	var gramRoots []string
	gramFuncs := map[string]bool{}
	add := func(name string, decl ast.Node, gf *trigramGuardFile) {
		nodes[name] = &trigramGuardNode{name: name, decl: decl, file: gf}
		if trigramGuardGramFile(gf.name) {
			gramRoots = append(gramRoots, name)
		}
	}
	for _, gf := range files {
		for _, decl := range gf.ast.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Body == nil {
					continue
				}
				name := trigramGuardFuncName(d)
				if d.Recv != nil {
					methodsByName[d.Name.Name] = append(methodsByName[d.Name.Name], name)
				} else if trigramGuardGramFile(gf.name) {
					gramFuncs[name] = true
				}
				add(name, d, gf)
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						add(s.Name.Name, s, gf)
					case *ast.ValueSpec:
						for _, ident := range s.Names {
							if ident.Name != "_" {
								add(ident.Name, s, gf)
							}
						}
					}
				}
			}
		}
	}

	var res trigramGuardResult
	res.reachable = map[string]bool{}
	res.feeders = map[string]bool{}
	var queue []string
	visit := func(name string) {
		if _, ok := nodes[name]; ok && !res.reachable[name] {
			res.reachable[name] = true
			queue = append(queue, name)
		}
	}
	for _, root := range trigramGuardRequiredRoots {
		if _, ok := nodes[root]; !ok {
			return res, errors.Errorf("gram-path root %s not found", root)
		}
		visit(root)
	}
	for _, root := range gramRoots {
		visit(root)
	}
	for _, gf := range files {
		if trigramGuardGramFile(gf.name) {
			continue
		}
		for _, decl := range gf.ast.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !trigramGuardFeeds(fn, gramFuncs) {
				continue
			}
			name := trigramGuardFuncName(fn)
			res.feeders[name] = true
			visit(name)
		}
	}

	// frozenTable reports whether expr names a frozen generated table.
	frozenTable := func(expr ast.Expr) bool {
		id, ok := expr.(*ast.Ident)
		if !ok {
			return false
		}
		n, ok := nodes[id.Name]
		if !ok || n.file.name != trigramGuardTablesFile {
			return false
		}
		_, isVar := n.decl.(*ast.ValueSpec)
		return isVar
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
				if path == trigramGuardUnicodePkg || path == "strings" || path == "bytes" {
					report(gf.ast.Package, "file dot-imports "+path)
				}
			}
			for _, path := range gf.paths {
				if path == "golang.org/x/text" || strings.HasPrefix(path, "golang.org/x/text/") {
					report(gf.ast.Package, "file imports "+path)
				}
			}
		}

		unicodeSel := func(expr ast.Expr) (string, bool) {
			sel, ok := expr.(*ast.SelectorExpr)
			if !ok {
				return "", false
			}
			x, ok := sel.X.(*ast.Ident)
			if !ok || gf.imports[x.Name] != trigramGuardUnicodePkg {
				return "", false
			}
			return sel.Sel.Name, true
		}

		var inspect func(n ast.Node) bool
		inspect = func(n ast.Node) bool {
			switch e := n.(type) {
			case *ast.CallExpr:
				fn, ok := unicodeSel(e.Fun)
				if !ok || (fn != "Is" && fn != "In") {
					return true
				}
				tables := e.Args
				if fn == "Is" && len(tables) > 0 {
					tables = tables[:1]
				} else if fn == "In" && len(tables) > 0 {
					tables = tables[1:]
				}
				for _, arg := range tables {
					if !frozenTable(arg) {
						report(arg.Pos(), "unicode."+fn+" with a table that is not frozen in "+trigramGuardTablesFile)
					}
				}
				for _, arg := range e.Args {
					ast.Inspect(arg, inspect)
				}
				return false
			case *ast.SelectorExpr:
				if x, ok := e.X.(*ast.Ident); ok {
					if path, isImport := gf.imports[x.Name]; isImport {
						sel := e.Sel.Name
						switch path {
						case trigramGuardUnicodePkg:
							simpleFold := sel == "SimpleFold" && node.name == "foldsUnderToLower" && gf.name == "regex.go"
							if !trigramGuardUnicodeAllowed[sel] && !simpleFold {
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
			case *ast.Field:
				// Field, parameter and interface method names are not
				// references; only the type is.
				ast.Inspect(e.Type, inspect)
				return false
			case *ast.Ident:
				visit(e.Name)
			}
			return true
		}
		ast.Inspect(node.decl, inspect)
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
		"ExtractLiterals", "AnalyzeRegex", "extractLiterals", "extractConcatLiterals", "crossJoin",
		"NewTrigramIndex", "WithN", "WithPadding", "TrigramIndex",
	} {
		if !res.reachable[name] {
			t.Errorf("%s is not reachable from the gram-path roots; the guard does not walk into it", name)
		}
	}
	for _, name := range []string{"GINIndex.evaluateContains", "GINIndex.evaluateRegex", "GINBuilder.addStringTerm"} {
		if !res.feeders[name] {
			t.Errorf("%s feeds a trigram index but was not detected as a feeder", name)
		}
	}
}

// trigramGuardStubs declares every required root; bodies overrides chosen
// root bodies.
func trigramGuardStubs(bodies map[string]string) string {
	body := func(name, def string) string {
		if b, ok := bodies[name]; ok {
			return b
		}
		return def
	}
	return `
type TrigramIndex struct{}
type GINIndex struct{}

func (ti *TrigramIndex) Add(value string, rgID int) {` + body("Add", "") + `}
func (ti *TrigramIndex) Search(pattern string) {` + body("Search", "") + `}
func (ti *TrigramIndex) generateNGrams(runes []rune) {` + body("generateNGrams", "") + `}
func GenerateNGrams(text string) {` + body("GenerateNGrams", "") + `}
func isWhitespaceOnly(s string) bool {` + body("isWhitespaceOnly", "return false") + `}
func isPunctuationOnly(s string) bool {` + body("isPunctuationOnly", "return false") + `}
func foldsUnderToLower(runes []rune) bool {` + body("foldsUnderToLower", "return false") + `}
func (idx *GINIndex) evaluateContains(pattern string) {` + body("evaluateContains", "") + `}
func (idx *GINIndex) evaluateRegex(pattern string) {` + body("evaluateRegex", "") + `}
`
}

func TestTrigramUnicodeASTGuardSelfTest(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  []string // substrings of one expected violation; nil means no violation
		count int      // expected number of violations when want is set; 0 means 1
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
			name: "strings.ToLower in a regex.go func that no root calls",
			files: map[string]string{
				"a.go": "package gin\n" + trigramGuardStubs(nil),
				"regex.go": `package gin
import "strings"
func extractLiterals(lit string) string { return strings.ToLower(lit) }
`,
			},
			want: []string{"extractLiterals (regex.go:3)", "uses strings.ToLower"},
		},
		{
			name: "strings.Map(unicode.ToLower) in AnalyzeRegex",
			files: map[string]string{
				"a.go": "package gin\n" + trigramGuardStubs(nil),
				"regex.go": `package gin
import (
	"strings"
	"unicode"
)
func AnalyzeRegex(lit string) string { return strings.Map(unicode.ToLower, lit) }
`,
			},
			want:  []string{"AnalyzeRegex (regex.go:6)", "uses unicode.ToLower"},
			count: 2, // strings.Map and unicode.ToLower
		},
		{
			name: "strings.ToLower in a trigram.go option func",
			files: map[string]string{
				"a.go": "package gin\n" + trigramGuardStubs(nil),
				"trigram.go": `package gin
import "strings"
type NGramConfig struct{ Padding string }
type NGramOption func(*NGramConfig) error
func WithPadding(pad string) NGramOption {
	return func(c *NGramConfig) error {
		c.Padding = strings.ToLower(pad)
		return nil
	}
}
`,
			},
			want: []string{"WithPadding (trigram.go:7)", "uses strings.ToLower"},
		},
		{
			name: "strings.ToLower in evaluateContains",
			files: map[string]string{"a.go": `package gin
import "strings"
` + trigramGuardStubs(map[string]string{"evaluateContains": "var ti *TrigramIndex; ti.Search(strings.ToLower(pattern))"})},
			want: []string{"GINIndex.evaluateContains (a.go:", "uses strings.ToLower"},
		},
		{
			name: "strings.ToLower in a builder func that calls Add on a trigram index",
			files: map[string]string{
				"a.go": "package gin\n" + trigramGuardStubs(nil),
				"builder.go": `package gin
import "strings"
type pathBuildData struct{ trigrams *TrigramIndex }
func addStringTerm(pd *pathBuildData, term string) { pd.trigrams.Add(strings.ToLower(term), 0) }
`,
			},
			want: []string{"addStringTerm (builder.go:4)", "uses strings.ToLower"},
		},
		{
			name: "strings.ToUpper in a func that searches a local trigram index",
			files: map[string]string{
				"a.go": "package gin\n" + trigramGuardStubs(nil),
				"lookup.go": `package gin
import "strings"
func lookup(idx *GINIndex, pattern string) {
	ti := idx.TrigramIndexes[0]
	ti.Search(strings.ToUpper(pattern))
}
`,
			},
			want: []string{"lookup (lookup.go:5)", "uses strings.ToUpper"},
		},
		{
			name: "struct field of type unicode.SpecialCase used by Search",
			files: map[string]string{
				"a.go": "package gin\n" + trigramGuardStubs(map[string]string{"Search": "var c caser; _ = c.special.ToLower('A')"}),
				"caser.go": `package gin
import "unicode"
type caser struct{ special unicode.SpecialCase }
`,
			},
			want: []string{"caser (caser.go:3)", "uses unicode.SpecialCase"},
		},
		{
			name: "unicode.In with a stdlib table",
			files: map[string]string{"a.go": `package gin
import "unicode"
` + trigramGuardStubs(map[string]string{"isPunctuationOnly": "return unicode.In('!', unicode.Punct)"})},
			want:  []string{"isPunctuationOnly (a.go:", "uses unicode.Punct"},
			count: 2, // not frozen, and a stdlib table reference
		},
		{
			name: "unicode.Is with a table declared outside the generated file",
			files: map[string]string{"a.go": `package gin
import "unicode"
var spaceTable = &unicode.RangeTable{}
` + trigramGuardStubs(map[string]string{"isWhitespaceOnly": "return unicode.Is(spaceTable, ' ')"})},
			want: []string{"isWhitespaceOnly (a.go:", "unicode.Is with a table that is not frozen"},
		},
		{
			name: "unicode.Is and unicode.In with frozen tables are allowed",
			files: map[string]string{
				"a.go": `package gin
import "unicode"
` + trigramGuardStubs(map[string]string{
					"isWhitespaceOnly":  "return unicode.In(' ', trigramSpaceTable, trigramPunctOrSymbolTable)",
					"isPunctuationOnly": "return unicode.Is(trigramPunctOrSymbolTable, '!')",
				}),
				trigramGuardTablesFile: `package gin
import "unicode"
var trigramSpaceTable = &unicode.RangeTable{R16: []unicode.Range16{{Lo: 0x20, Hi: 0x20, Stride: 1}}}
var trigramPunctOrSymbolTable = &unicode.RangeTable{R16: []unicode.Range16{{Lo: 0x21, Hi: 0x21, Stride: 1}}}
`,
			},
		},
		{
			name: "unreachable forbidden call is not reported",
			files: map[string]string{"a.go": `package gin
import "strings"
func unrelated(s string) string { return strings.ToLower(s) }
` + trigramGuardStubs(nil)},
		},
		{
			name: "unicode.SimpleFold inside regex.go foldsUnderToLower is allowed",
			files: map[string]string{"regex.go": `package gin
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
			count := c.count
			if count == 0 {
				count = 1
			}
			if len(res.violations) != count {
				t.Fatalf("violations = %q, want %d, one containing %q", res.violations, count, c.want)
			}
			for _, v := range res.violations {
				matched := true
				for _, want := range c.want {
					matched = matched && strings.Contains(v, want)
				}
				if matched {
					return
				}
			}
			t.Errorf("no violation in %q contains all of %q", res.violations, c.want)
		})
	}
}
