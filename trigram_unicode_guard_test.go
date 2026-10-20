package gin

import (
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The trigram Unicode guard (#99) scans every non-test file of the package
// and fails on any read of the toolchain's Unicode tables. TrigramIndex grams
// must come from the frozen helpers in trigram_unicode.go. A flat scan has no
// call graph to get wrong: moving a forbidden call into a helper, an option
// func or a builder method does not hide it. Code that never feeds
// TrigramIndex.Add or Search may opt out through trigramGuardAllowlist.

const (
	trigramGuardUnicodePkg = "unicode"
	// trigramGuardTablesFile holds the frozen tables; only its package vars
	// may be passed to unicode.Is and unicode.In.
	trigramGuardTablesFile = "trigram_unicode_tables.go"
)

// trigramGuardUnicodeAllowed lists the unicode selectors that hold no
// toolchain table data: the table types and the constants. unicode.Is and
// unicode.In are checked at the call: their tables must be frozen.
var trigramGuardUnicodeAllowed = map[string]bool{
	"RangeTable": true, "Range16": true, "Range32": true,
	"MaxRune": true, "ReplacementChar": true, "MaxASCII": true, "MaxLatin1": true,
	"UpperCase": true, "LowerCase": true, "TitleCase": true, "MaxCase": true, "UpperLower": true,
}

// trigramGuardCaseDenied applies to strings and bytes. strings.Map is not
// listed: a unicode mapping func passed to it is a denied unicode selector.
var trigramGuardCaseDenied = map[string]bool{
	"ToLower": true, "ToUpper": true, "ToTitle": true, "ToLowerSpecial": true,
	"ToUpperSpecial": true, "ToTitleSpecial": true, "Title": true, "EqualFold": true,
}

// trigramGuardAllow permits sel (import path + "." + name) inside the
// top-level declaration decl of file.
type trigramGuardAllow struct{ file, decl, sel, reason string }

var trigramGuardAllowlist = []trigramGuardAllow{
	{"transformers.go", "ToLower", "strings.ToLower", "D3: derived companion transformer, out of scope (#102)"},
	{"transformers.go", "EmailDomain", "strings.ToLower", "D3: derived companion transformer, out of scope (#102)"},
	{"transformers.go", "URLHost", "strings.ToLower", "D3: derived companion transformer, out of scope (#102)"},
	{"transformers.go", "BoolNormalize", "strings.ToLower", "matches ASCII keywords and returns a bool, never a gram"},
	{"regex.go", "foldsUnderToLower", "unicode.SimpleFold", "D4: the orbit must match the reader's regexp, not the frozen tables"},
}

type trigramGuardFile struct {
	name string
	ast  *ast.File
}

// trigramGuardDecls returns the top-level declarations of f by name: Name for
// funcs, Recv.Name for methods, and the spec name for types, vars and consts.
func trigramGuardDecls(f *ast.File) (names []string, nodes []ast.Node) {
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			name := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) > 0 {
				recv := d.Recv.List[0].Type
				if star, ok := recv.(*ast.StarExpr); ok {
					recv = star.X
				}
				if id, ok := recv.(*ast.Ident); ok {
					name = id.Name + "." + name
				}
			}
			names, nodes = append(names, name), append(nodes, d)
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				name := "import"
				switch s := spec.(type) {
				case *ast.TypeSpec:
					name = s.Name.Name
				case *ast.ValueSpec:
					name = s.Names[0].Name
				}
				names, nodes = append(names, name), append(nodes, spec)
			}
		}
	}
	return names, nodes
}

// checkTrigramUnicodeGuard returns one violation per forbidden use, plus one
// per allowlist entry that matches nothing.
func checkTrigramUnicodeGuard(fset *token.FileSet, files []*trigramGuardFile, allow []trigramGuardAllow) []string {
	frozen := map[string]bool{}
	for _, gf := range files {
		if gf.name != trigramGuardTablesFile {
			continue
		}
		for _, decl := range gf.ast.Decls {
			if d, ok := decl.(*ast.GenDecl); ok && d.Tok == token.VAR {
				for _, spec := range d.Specs {
					for _, id := range spec.(*ast.ValueSpec).Names {
						frozen[id.Name] = true
					}
				}
			}
		}
	}
	used := make([]bool, len(allow))
	var violations []string
	for _, gf := range files {
		report := func(pos token.Pos, decl, msg string) {
			violations = append(violations, fmt.Sprintf("%s: %s: %s", fset.Position(pos), decl, msg))
		}
		imports := map[string]string{} // local name -> import path
		for _, imp := range gf.ast.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			local := path[strings.LastIndex(path, "/")+1:]
			if imp.Name != nil {
				local = imp.Name.Name
			}
			switch {
			case path == "golang.org/x/text" || strings.HasPrefix(path, "golang.org/x/text/"):
				report(imp.Pos(), "import", "imports "+path)
			case local == "." && (path == trigramGuardUnicodePkg || path == "strings" || path == "bytes"):
				report(imp.Pos(), "import", "dot-imports "+path)
			}
			imports[local] = path
		}
		names, nodes := trigramGuardDecls(gf.ast)
		for i, node := range nodes {
			decl := names[i]
			calls := map[ast.Expr]bool{} // Fun of checked unicode.Is/In calls
			ast.Inspect(node, func(n ast.Node) bool {
				switch e := n.(type) {
				case *ast.CallExpr:
					sel, ok := e.Fun.(*ast.SelectorExpr)
					if !ok || trigramGuardImportOf(imports, sel) != trigramGuardUnicodePkg ||
						(sel.Sel.Name != "Is" && sel.Sel.Name != "In") || len(e.Args) == 0 {
						return true
					}
					calls[e.Fun] = true
					tables := e.Args[1:] // unicode.In(r, tables...)
					if sel.Sel.Name == "Is" {
						tables = e.Args[:1] // unicode.Is(table, r)
					}
					for _, arg := range tables {
						if id, ok := arg.(*ast.Ident); !ok || !frozen[id.Name] {
							report(arg.Pos(), decl, "unicode."+sel.Sel.Name+" with a table not frozen in "+trigramGuardTablesFile)
						}
					}
				case *ast.SelectorExpr:
					path := trigramGuardImportOf(imports, e)
					if path != trigramGuardUnicodePkg && path != "strings" && path != "bytes" {
						return true
					}
					name := path + "." + e.Sel.Name
					denied := trigramGuardCaseDenied[e.Sel.Name]
					if path == trigramGuardUnicodePkg {
						denied = !trigramGuardUnicodeAllowed[e.Sel.Name] && !calls[e]
					}
					if !denied {
						return false
					}
					for j, a := range allow {
						if a.file == gf.name && a.decl == decl && a.sel == name {
							used[j], denied = true, false
						}
					}
					if denied {
						report(e.Pos(), decl, "uses "+name)
					}
					return false
				}
				return true
			})
		}
	}
	for j, a := range allow {
		if !used[j] {
			violations = append(violations, fmt.Sprintf("allowlist entry %s %s %s matches nothing; remove it", a.file, a.decl, a.sel))
		}
	}
	sort.Strings(violations)
	return violations
}

// trigramGuardImportOf returns the import path that sel's qualifier names,
// or "" when sel is not a package selector.
func trigramGuardImportOf(imports map[string]string, sel *ast.SelectorExpr) string {
	if x, ok := sel.X.(*ast.Ident); ok {
		return imports[x.Name]
	}
	return ""
}

// loadTrigramGuardPackage parses every non-test .go file of the package. A
// file is skipped only when its build constraint needs the ignore tag to be
// false; every other tag counts as set, so tagged files like parser_simd.go
// are still scanned.
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
		included := true
		for _, group := range f.Comments {
			for _, c := range group.List {
				if c.Pos() >= f.Package || !constraint.IsGoBuild(c.Text) {
					continue
				}
				expr, err := constraint.Parse(c.Text)
				if err != nil {
					t.Fatalf("%s: build constraint: %v", name, err)
				}
				included = expr.Eval(func(tag string) bool { return tag != "ignore" })
			}
		}
		if included {
			files = append(files, &trigramGuardFile{name: name, ast: f})
		}
	}
	return fset, files
}

func TestTrigramUnicodeASTGuard(t *testing.T) {
	fset, files := loadTrigramGuardPackage(t)
	scanned := map[string]bool{}
	for _, gf := range files {
		scanned[gf.name] = true
	}
	if scanned["gen_trigram_unicode.go"] {
		t.Errorf("the //go:build ignore generator must not be scanned")
	}
	for _, name := range []string{"parser_simd.go", "trigram.go", "regex.go", "query.go", "builder.go", trigramGuardTablesFile} {
		if !scanned[name] {
			t.Errorf("%s is not scanned", name)
		}
	}
	for _, v := range checkTrigramUnicodeGuard(fset, files, trigramGuardAllowlist) {
		t.Errorf("%s\nthe gram path must use the frozen helpers in trigram_unicode.go; add an allowlist entry only for code that never feeds TrigramIndex.Add/Search", v)
	}
}

func TestTrigramUnicodeASTGuardSelfTest(t *testing.T) {
	frozenTables := `package gin
import "unicode"
var trigramSpaceTable = &unicode.RangeTable{R16: []unicode.Range16{{Lo: 0x20, Hi: 0x20, Stride: 1}}}
var trigramPunctOrSymbolTable = &unicode.RangeTable{R16: []unicode.Range16{{Lo: 0x21, Hi: 0x21, Stride: 1}}}
`
	simpleFold := trigramGuardAllow{"regex.go", "foldsUnderToLower", "unicode.SimpleFold", "test"}
	cases := []struct {
		name  string
		files map[string]string
		allow []trigramGuardAllow
		want  []string // substrings of one expected violation; nil means no violation
		count int      // expected number of violations when want is set; 0 means 1
	}{
		{
			name: "aliased unicode import",
			files: map[string]string{"a.go": `package gin
import u "unicode"
func lower(r rune) rune { return u.ToLower(r) }
`},
			want: []string{"a.go:3:", "lower: uses unicode.ToLower"},
		},
		{
			name: "dot import of unicode",
			files: map[string]string{"b.go": `package gin
import . "unicode"
func space(r rune) bool { return IsSpace(r) }
`},
			want: []string{"b.go:2:", "dot-imports unicode"},
		},
		{
			name: "golang.org/x/text import",
			files: map[string]string{"fold.go": `package gin
import "golang.org/x/text/cases"
var _ cases.Caser
`},
			want: []string{"fold.go:2:", "imports golang.org/x/text/cases"},
		},
		{
			name: "unicode.Version reference",
			files: map[string]string{"a.go": `package gin
import "unicode"
var tableVersion = unicode.Version
`},
			want: []string{"a.go:3:", "tableVersion: uses unicode.Version"},
		},
		{
			name: "strings.ToLower in regex.go extractLiterals",
			files: map[string]string{"regex.go": `package gin
import "strings"
func extractLiterals(lit string) string { return strings.ToLower(lit) }
`},
			want: []string{"regex.go:3:", "extractLiterals: uses strings.ToLower"},
		},
		{
			name: "strings.Map(unicode.ToLower) in AnalyzeRegex",
			files: map[string]string{"regex.go": `package gin
import (
	"strings"
	"unicode"
)
func AnalyzeRegex(lit string) string { return strings.Map(unicode.ToLower, lit) }
`},
			want: []string{"regex.go:6:", "AnalyzeRegex: uses unicode.ToLower"},
		},
		{
			name: "strings.ToLower in a trigram.go option func",
			files: map[string]string{"trigram.go": `package gin
import "strings"
type NGramConfig struct{ Padding string }
type NGramOption func(*NGramConfig) error
func WithPadding(pad string) NGramOption {
	return func(c *NGramConfig) error {
		c.Padding = strings.ToLower(pad)
		return nil
	}
}
`},
			want: []string{"trigram.go:7:", "WithPadding: uses strings.ToLower"},
		},
		{
			name: "strings.ToLower in evaluateContains",
			files: map[string]string{"query.go": `package gin
import "strings"
type GINIndex struct{}
func (idx *GINIndex) evaluateContains(pattern string) string { return strings.ToLower(pattern) }
`},
			want: []string{"query.go:4:", "GINIndex.evaluateContains: uses strings.ToLower"},
		},
		{
			name: "bytes.ToLower in a builder method that calls Add",
			files: map[string]string{"builder.go": `package gin
import "bytes"
type GINBuilder struct{ trigrams interface{ Add(string, int) } }
func (b *GINBuilder) addStringTerm(term []byte) { b.trigrams.Add(string(bytes.ToLower(term)), 0) }
`},
			want: []string{"builder.go:4:", "GINBuilder.addStringTerm: uses bytes.ToLower"},
		},
		{
			name: "struct field of type unicode.SpecialCase",
			files: map[string]string{"caser.go": `package gin
import "unicode"
type caser struct{ special unicode.SpecialCase }
`},
			want: []string{"caser.go:3:", "caser: uses unicode.SpecialCase"},
		},
		{
			name: "unicode.In with a stdlib table",
			files: map[string]string{"a.go": `package gin
import "unicode"
func punct(r rune) bool { return unicode.In(r, unicode.Punct) }
`},
			want:  []string{"a.go:3:", "punct: uses unicode.Punct"},
			count: 2, // not frozen, and a stdlib table reference
		},
		{
			name: "unicode.Is with a table declared outside the tables file",
			files: map[string]string{"a.go": `package gin
import "unicode"
var spaceTable = &unicode.RangeTable{}
func space(r rune) bool { return unicode.Is(spaceTable, r) }
`},
			want: []string{"a.go:4:", "space: unicode.Is with a table not frozen"},
		},
		{
			name: "unicode.Is used as a value",
			files: map[string]string{"a.go": `package gin
import "unicode"
var is = unicode.Is
`},
			want: []string{"a.go:3:", "is: uses unicode.Is"},
		},
		{
			name: "unicode.Is and unicode.In with frozen tables are allowed",
			files: map[string]string{
				"a.go": `package gin
import "unicode"
func space(r rune) bool { return unicode.In(r, trigramSpaceTable, trigramPunctOrSymbolTable) }
func punct(r rune) bool { return unicode.Is(trigramPunctOrSymbolTable, r) }
`,
				trigramGuardTablesFile: frozenTables,
			},
		},
		{
			name: "forbidden call in a func unrelated to trigrams is reported",
			files: map[string]string{"a.go": `package gin
import "strings"
func unrelated(s string) string { return strings.ToLower(s) }
`},
			want: []string{"a.go:3:", "unrelated: uses strings.ToLower"},
		},
		{
			name: "allowlisted call is accepted",
			files: map[string]string{"regex.go": `package gin
import "unicode"
func foldsUnderToLower(r rune) bool { return unicode.SimpleFold(r) == r }
`},
			allow: []trigramGuardAllow{simpleFold},
		},
		{
			name: "allowlisted selector outside its func is rejected",
			files: map[string]string{"regex.go": `package gin
import "unicode"
func foldsUnderToLower(r rune) bool { return unicode.SimpleFold(r) == r }
func isPunctuationOnly(r rune) bool { return unicode.SimpleFold(r) == r }
`},
			allow: []trigramGuardAllow{simpleFold},
			want:  []string{"regex.go:4:", "isPunctuationOnly: uses unicode.SimpleFold"},
		},
		{
			name:  "stale allowlist entry is reported",
			files: map[string]string{"regex.go": "package gin\n"},
			allow: []trigramGuardAllow{simpleFold},
			want:  []string{"allowlist entry regex.go foldsUnderToLower unicode.SimpleFold matches nothing"},
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
				files = append(files, &trigramGuardFile{name: name, ast: f})
			}
			violations := checkTrigramUnicodeGuard(fset, files, c.allow)
			if c.want == nil {
				if len(violations) != 0 {
					t.Fatalf("unexpected violations: %q", violations)
				}
				return
			}
			count := c.count
			if count == 0 {
				count = 1
			}
			if len(violations) != count {
				t.Fatalf("violations = %q, want %d, one containing %q", violations, count, c.want)
			}
			for _, v := range violations {
				matched := true
				for _, want := range c.want {
					matched = matched && strings.Contains(v, want)
				}
				if matched {
					return
				}
			}
			t.Errorf("no violation in %q contains all of %q", violations, c.want)
		})
	}
}
