//go:build simdjson

package gin

import (
	stderrors "errors"
	"fmt"
	"go/ast"
	"go/doc"
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	purejson "github.com/amikos-tech/pure-simdjson"
	"github.com/pkg/errors"
)

const simdRequiredEnv = "AMI_GIN_SIMD_REQUIRED"

type simdLoadErrorClass string

const (
	simdLoadCPUUnsupported     simdLoadErrorClass = "cpu-unsupported"
	simdLoadABIVersionMismatch simdLoadErrorClass = "abi-version-mismatch"
	simdLoadChecksumMismatch   simdLoadErrorClass = "checksum-mismatch"
	simdLoadAllSourcesFailed   simdLoadErrorClass = "all-sources-failed"
	simdLoadInvalidHandle      simdLoadErrorClass = "invalid-handle"
	simdLoadClosed             simdLoadErrorClass = "closed"
	simdLoadUnknown            simdLoadErrorClass = "unknown"
)

type simdConstructionAction string

const (
	simdConstructionUseParser        simdConstructionAction = "use-parser"
	simdConstructionSkipUnsupported  simdConstructionAction = "skip-unsupported"
	simdConstructionSkipUnavailable  simdConstructionAction = "skip-unavailable"
	simdConstructionFatalUnavailable simdConstructionAction = "fatal-unavailable"
)

type simdTestTB interface {
	Helper()
	Cleanup(func())
	Skip(...any)
	Skipf(string, ...any)
	Fatalf(string, ...any)
	Errorf(string, ...any)
}

func supportedSIMDPlatform(goos, goarch string) bool {
	switch goos + "/" + goarch {
	case "linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64":
		return true
	default:
		return false
	}
}

func classifySIMDLoadError(err error) simdLoadErrorClass {
	switch {
	case stderrors.Is(err, purejson.ErrCPUUnsupported):
		return simdLoadCPUUnsupported
	case stderrors.Is(err, purejson.ErrABIVersionMismatch):
		return simdLoadABIVersionMismatch
	case stderrors.Is(err, purejson.ErrChecksumMismatch):
		return simdLoadChecksumMismatch
	case stderrors.Is(err, purejson.ErrAllSourcesFailed):
		return simdLoadAllSourcesFailed
	case stderrors.Is(err, purejson.ErrInvalidHandle):
		return simdLoadInvalidHandle
	case stderrors.Is(err, purejson.ErrClosed):
		return simdLoadClosed
	default:
		return simdLoadUnknown
	}
}

func simdConstructionPolicy(
	goos string,
	goarch string,
	required bool,
	constructionErr error,
) simdConstructionAction {
	if !supportedSIMDPlatform(goos, goarch) {
		return simdConstructionSkipUnsupported
	}
	if constructionErr == nil {
		return simdConstructionUseParser
	}
	if required {
		return simdConstructionFatalUnavailable
	}
	return simdConstructionSkipUnavailable
}

func newTestSIMDParserWith(
	tb simdTestTB,
	goos string,
	goarch string,
	required bool,
	construct func() (CloseableParser, error),
) CloseableParser {
	tb.Helper()
	if simdConstructionPolicy(goos, goarch, required, nil) == simdConstructionSkipUnsupported {
		tb.Skip("pure-simdjson unsupported platform")
		return nil
	}

	parser, err := construct()
	switch simdConstructionPolicy(goos, goarch, required, err) {
	case simdConstructionSkipUnavailable:
		tb.Skipf("pure-simdjson unavailable locally: %v; see docs/simd-deployment.md", err)
		return nil
	case simdConstructionFatalUnavailable:
		tb.Fatalf(
			"pure-simdjson required on %s/%s: class=%s: %v",
			goos,
			goarch,
			classifySIMDLoadError(err),
			err,
		)
		return nil
	}

	tb.Cleanup(func() {
		if err := parser.Close(); err != nil {
			tb.Errorf("Close SIMD parser: %v", err)
		}
	})
	return parser
}

func newTestSIMDParser(tb testing.TB) CloseableParser {
	tb.Helper()
	return newTestSIMDParserWith(
		tb,
		runtime.GOOS,
		runtime.GOARCH,
		os.Getenv(simdRequiredEnv) == "1",
		func() (CloseableParser, error) {
			return NewSIMDParser()
		},
	)
}

type simdConstructorCall struct {
	file      string
	function  string
	qualified bool
	position  token.Position
}

func validateSIMDConstructorCalls(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errors.Wrap(err, "read SIMD test directory")
	}

	fset := token.NewFileSet()
	var helperCalls int
	var exampleCalls int
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		source, err := os.ReadFile(path)
		if err != nil {
			return errors.Wrapf(err, "read %s", name)
		}
		firstLine, _, _ := strings.Cut(string(source), "\n")
		if firstLine != "//go:build simdjson" {
			continue
		}

		file, err := goparser.ParseFile(fset, path, source, goparser.ParseComments)
		if err != nil {
			return errors.Wrapf(err, "parse %s", name)
		}
		hasExampleDirective, exampleFound := simdExampleOutputPolicy(file)
		for _, call := range simdRawConstructorCalls(fset, name, file) {
			switch {
			case name == "parser_simd_integration_test.go" &&
				file.Name.Name == "gin" &&
				call.function == "newTestSIMDParser" &&
				!call.qualified:
				helperCalls++
			case name == "simd_example_test.go" &&
				file.Name.Name == "gin_test" &&
				call.function == "ExampleNewSIMDParser" &&
				call.qualified:
				if !exampleFound {
					return errors.Errorf("%s:%s: ExampleNewSIMDParser is not recognized by go/doc", call.file, call.function)
				}
				if hasExampleDirective {
					return errors.Errorf("%s:%s: constructor Example has an output directive", call.file, call.function)
				}
				exampleCalls++
			default:
				return errors.Errorf(
					"%s:%d:%s: raw NewSIMDParser call bypasses newTestSIMDParser",
					call.file,
					call.position.Line,
					call.function,
				)
			}
		}
	}

	if helperCalls != 1 {
		return errors.Errorf("raw NewSIMDParser helper calls = %d, want exactly 1", helperCalls)
	}
	if exampleCalls > 1 {
		return errors.Errorf("compile-only ExampleNewSIMDParser calls = %d, want at most 1", exampleCalls)
	}
	return nil
}

func simdRawConstructorCalls(
	fset *token.FileSet,
	name string,
	file *ast.File,
) []simdConstructorCall {
	funcs := make([]*ast.FuncDecl, 0)
	for _, declaration := range file.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok {
			funcs = append(funcs, function)
		}
	}

	var calls []simdConstructorCall
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		qualified := false
		var finalName string
		switch callee := call.Fun.(type) {
		case *ast.Ident:
			finalName = callee.Name
		case *ast.SelectorExpr:
			finalName = callee.Sel.Name
			qualified = true
		default:
			return true
		}
		if finalName != "NewSIMDParser" {
			return true
		}

		functionName := "<package>"
		for _, function := range funcs {
			if function.Pos() <= call.Pos() && call.End() <= function.End() {
				functionName = function.Name.Name
				break
			}
		}
		calls = append(calls, simdConstructorCall{
			file:      name,
			function:  functionName,
			qualified: qualified,
			position:  fset.Position(call.Pos()),
		})
		return true
	})
	return calls
}

func simdExampleOutputPolicy(file *ast.File) (hasDirective bool, found bool) {
	for _, example := range doc.Examples(file) {
		if example.Name != "NewSIMDParser" {
			continue
		}
		return example.Output != "" || example.Unordered || example.EmptyOutput, true
	}
	return false, false
}

func TestSupportedSIMDPlatform(t *testing.T) {
	tests := []struct {
		name   string
		goos   string
		goarch string
		want   bool
	}{
		{name: "linux-amd64", goos: "linux", goarch: "amd64", want: true},
		{name: "linux-arm64", goos: "linux", goarch: "arm64", want: true},
		{name: "darwin-amd64", goos: "darwin", goarch: "amd64", want: true},
		{name: "darwin-arm64", goos: "darwin", goarch: "arm64", want: true},
		{name: "windows-amd64", goos: "windows", goarch: "amd64", want: true},
		{name: "linux-386", goos: "linux", goarch: "386", want: false},
		{name: "freebsd-amd64", goos: "freebsd", goarch: "amd64", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := supportedSIMDPlatform(tc.goos, tc.goarch); got != tc.want {
				t.Fatalf("supportedSIMDPlatform(%q, %q) = %v, want %v", tc.goos, tc.goarch, got, tc.want)
			}
		})
	}
}

func TestClassifySIMDLoadError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want simdLoadErrorClass
	}{
		{name: "cpu-unsupported", err: purejson.ErrCPUUnsupported, want: simdLoadCPUUnsupported},
		{name: "abi-version-mismatch", err: purejson.ErrABIVersionMismatch, want: simdLoadABIVersionMismatch},
		{name: "checksum-mismatch", err: purejson.ErrChecksumMismatch, want: simdLoadChecksumMismatch},
		{name: "all-sources-failed", err: purejson.ErrAllSourcesFailed, want: simdLoadAllSourcesFailed},
		{name: "invalid-handle", err: purejson.ErrInvalidHandle, want: simdLoadInvalidHandle},
		{name: "closed", err: purejson.ErrClosed, want: simdLoadClosed},
		{name: "unknown", err: errors.New("unrelated constructor failure"), want: simdLoadUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := errors.Wrap(tc.err, "outer construction context")
			if got := classifySIMDLoadError(wrapped); got != tc.want {
				t.Fatalf("classifySIMDLoadError(%v) = %q, want %q", wrapped, got, tc.want)
			}
		})
	}
}

func TestSIMDConstructionPolicy(t *testing.T) {
	loadErr := errors.New("load failed")
	tests := []struct {
		name     string
		goos     string
		goarch   string
		required bool
		err      error
		want     simdConstructionAction
	}{
		{name: "unsupported-skips-before-construction", goos: "freebsd", goarch: "amd64", err: loadErr, want: simdConstructionSkipUnsupported},
		{name: "supported-local-success", goos: "darwin", goarch: "arm64", want: simdConstructionUseParser},
		{name: "supported-local-failure-skips", goos: "darwin", goarch: "arm64", err: loadErr, want: simdConstructionSkipUnavailable},
		{name: "supported-required-failure-is-fatal", goos: "darwin", goarch: "arm64", required: true, err: loadErr, want: simdConstructionFatalUnavailable},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := simdConstructionPolicy(tc.goos, tc.goarch, tc.required, tc.err); got != tc.want {
				t.Fatalf("simdConstructionPolicy() = %q, want %q", got, tc.want)
			}
		})
	}
}

type fakeSIMDTestTB struct {
	cleanups []func()
	skipped  string
	fatal    string
	errors   []string
}

func (*fakeSIMDTestTB) Helper() {}

func (tb *fakeSIMDTestTB) Cleanup(cleanup func()) {
	tb.cleanups = append(tb.cleanups, cleanup)
}

func (tb *fakeSIMDTestTB) Skip(args ...any) {
	tb.skipped = fmt.Sprint(args...)
}

func (tb *fakeSIMDTestTB) Skipf(format string, args ...any) {
	tb.skipped = fmt.Sprintf(format, args...)
}

func (tb *fakeSIMDTestTB) Fatalf(format string, args ...any) {
	tb.fatal = fmt.Sprintf(format, args...)
}

func (tb *fakeSIMDTestTB) Errorf(format string, args ...any) {
	tb.errors = append(tb.errors, fmt.Sprintf(format, args...))
}

type fakeSIMDCloseableParser struct {
	closeCalls int
	closeErr   error
}

func (*fakeSIMDCloseableParser) Name() string { return "fake-simd" }

func (*fakeSIMDCloseableParser) Parse([]byte, int, parserSink) error { return nil }

func (p *fakeSIMDCloseableParser) Close() error {
	p.closeCalls++
	return p.closeErr
}

func TestNewTestSIMDParserLifecyclePolicy(t *testing.T) {
	t.Run("unsupported skips before construction", func(t *testing.T) {
		tb := &fakeSIMDTestTB{}
		constructCalls := 0
		got := newTestSIMDParserWith(tb, "freebsd", "amd64", true, func() (CloseableParser, error) {
			constructCalls++
			return &fakeSIMDCloseableParser{}, nil
		})
		if got != nil || constructCalls != 0 {
			t.Fatalf("result = (%v, constructCalls=%d), want (nil, 0)", got, constructCalls)
		}
		if tb.skipped != "pure-simdjson unsupported platform" {
			t.Fatalf("skip diagnostic = %q", tb.skipped)
		}
	})

	t.Run("supported local failure skips with remediation", func(t *testing.T) {
		tb := &fakeSIMDTestTB{}
		loadErr := errors.Wrap(purejson.ErrChecksumMismatch, "load")
		got := newTestSIMDParserWith(tb, "linux", "amd64", false, func() (CloseableParser, error) {
			return nil, loadErr
		})
		if got != nil || !strings.Contains(tb.skipped, "docs/simd-deployment.md") || !strings.Contains(tb.skipped, loadErr.Error()) {
			t.Fatalf("result = (%v, skip=%q), want remediation skip", got, tb.skipped)
		}
	})

	t.Run("supported required failure is classified fatal", func(t *testing.T) {
		tb := &fakeSIMDTestTB{}
		got := newTestSIMDParserWith(tb, "windows", "amd64", true, func() (CloseableParser, error) {
			return nil, errors.Wrap(purejson.ErrInvalidHandle, "load")
		})
		if got != nil || !strings.Contains(tb.fatal, string(simdLoadInvalidHandle)) {
			t.Fatalf("result = (%v, fatal=%q), want classified fatal", got, tb.fatal)
		}
	})

	t.Run("success registers one cleanup and reports close error", func(t *testing.T) {
		tb := &fakeSIMDTestTB{}
		closeErr := errors.New("close failed")
		parser := &fakeSIMDCloseableParser{closeErr: closeErr}
		got := newTestSIMDParserWith(tb, "darwin", "arm64", true, func() (CloseableParser, error) {
			return parser, nil
		})
		if got != parser || len(tb.cleanups) != 1 {
			t.Fatalf("result = (%v, cleanups=%d), want parser and one cleanup", got, len(tb.cleanups))
		}
		tb.cleanups[0]()
		if parser.closeCalls != 1 || len(tb.errors) != 1 || !strings.Contains(tb.errors[0], closeErr.Error()) {
			t.Fatalf("cleanup = (closeCalls=%d, errors=%v), want one reported close failure", parser.closeCalls, tb.errors)
		}
	})
}

func TestSIMDConstructorCallPolicy(t *testing.T) {
	if err := validateSIMDConstructorCalls("."); err != nil {
		t.Fatalf("validate repository SIMD constructor calls: %v", err)
	}

	const helperSource = `//go:build simdjson

package gin

func newTestSIMDParser() {
	NewSIMDParser()
}
`

	writeFixture := func(t *testing.T, name, source string) string {
		t.Helper()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "parser_simd_integration_test.go"), []byte(helperSource), 0o600); err != nil {
			t.Fatalf("write helper fixture: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return dir
	}

	t.Run("external_qualified_test_mutation", func(t *testing.T) {
		const source = `//go:build simdjson

package gin_test

import gin "github.com/amikos-tech/ami-gin"

func TestExternalConstructorBypass() {
	_, _ = gin.NewSIMDParser()
}
`
		err := validateSIMDConstructorCalls(writeFixture(t, "external_bypass_test.go", source))
		if err == nil || !strings.Contains(err.Error(), "external_bypass_test.go") || !strings.Contains(err.Error(), "TestExternalConstructorBypass") {
			t.Fatalf("mutation error = %v, want file/function diagnostic", err)
		}
	})

	t.Run("compile-only example is accepted", func(t *testing.T) {
		const source = `//go:build simdjson

package gin_test

import gin "github.com/amikos-tech/ami-gin"

func ExampleNewSIMDParser() {
	_, _ = gin.NewSIMDParser()
}
`
		if err := validateSIMDConstructorCalls(writeFixture(t, "simd_example_test.go", source)); err != nil {
			t.Fatalf("compile-only example rejected: %v", err)
		}
	})

	t.Run("empty output example is rejected", func(t *testing.T) {
		const source = `//go:build simdjson

package gin_test

import gin "github.com/amikos-tech/ami-gin"

func ExampleNewSIMDParser() {
	_, _ = gin.NewSIMDParser()
	// Output:
}
`
		err := validateSIMDConstructorCalls(writeFixture(t, "simd_example_test.go", source))
		if err == nil || !strings.Contains(err.Error(), "output directive") {
			t.Fatalf("empty-output example error = %v, want output-directive diagnostic", err)
		}
	})
}

func TestSIMDParserGoldenAuthoredFixtures(t *testing.T) {
	parser := newTestSIMDParser(t)

	for _, fx := range authoredParityFixtures() {
		fx := fx
		t.Run(fx.Name, func(t *testing.T) {
			encoded := buildAndEncodeWithParser(t, fx, parser)
			golden := loadGolden(t, fx.Name)
			assertByteIdentical(t, fx.Name, encoded, golden)
		})
	}
}

func TestSIMDParserNativeNumericFailuresUseNumericPolicy(t *testing.T) {
	tests := []struct {
		name    string
		jsonDoc []byte
	}{
		{
			name:    "overflowed-exponent",
			jsonDoc: []byte(`{"n":1e400}`),
		},
		{
			name:    "uint64-above-int64",
			jsonDoc: []byte(`{"n":9223372036854775808}`),
		},
		{
			name:    "larger-than-uint64",
			jsonDoc: []byte(`{"n":18446744073709551616}`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("hard-numeric-soft-parser", func(t *testing.T) {
				parser := newTestSIMDParser(t)
				cfg, err := NewConfig(
					WithParserFailureMode(IngestFailureSoft),
					WithNumericFailureMode(IngestFailureHard),
				)
				if err != nil {
					t.Fatalf("NewConfig: %v", err)
				}
				builder, err := NewBuilder(cfg, 1, WithParser(parser))
				if err != nil {
					t.Fatalf("NewBuilder: %v", err)
				}

				addErr := builder.AddDocument(DocID(0), tc.jsonDoc)
				var ingestErr *IngestError
				if !stderrors.As(addErr, &ingestErr) {
					t.Fatalf("AddDocument error = %v, want *IngestError", addErr)
				}
				if ingestErr.Layer() != IngestLayerNumeric || ingestErr.Path() != "$.n" {
					t.Fatalf(
						"IngestError = (layer=%q, path=%q), want (numeric, $.n)",
						ingestErr.Layer(),
						ingestErr.Path(),
					)
				}
				requireTypedSinkUncommitted(t, builder, 0)
			})

			t.Run("soft-numeric-hard-parser", func(t *testing.T) {
				parser := newTestSIMDParser(t)
				cfg, err := NewConfig(
					WithParserFailureMode(IngestFailureHard),
					WithNumericFailureMode(IngestFailureSoft),
				)
				if err != nil {
					t.Fatalf("NewConfig: %v", err)
				}
				builder, err := NewBuilder(cfg, 1, WithParser(parser))
				if err != nil {
					t.Fatalf("NewBuilder: %v", err)
				}

				if err := builder.AddDocument(DocID(0), tc.jsonDoc); err != nil {
					t.Fatalf("AddDocument: %v", err)
				}
				requireTypedSinkUncommitted(t, builder, 1)
			})

			t.Run("hard-numeric-hard-parser", func(t *testing.T) {
				parser := newTestSIMDParser(t)
				cfg, err := NewConfig(
					WithParserFailureMode(IngestFailureHard),
					WithNumericFailureMode(IngestFailureHard),
				)
				if err != nil {
					t.Fatalf("NewConfig: %v", err)
				}
				builder, err := NewBuilder(cfg, 1, WithParser(parser))
				if err != nil {
					t.Fatalf("NewBuilder: %v", err)
				}

				addErr := builder.AddDocument(DocID(0), tc.jsonDoc)
				var ingestErr *IngestError
				if !stderrors.As(addErr, &ingestErr) {
					t.Fatalf("AddDocument error = %v, want *IngestError", addErr)
				}
				if ingestErr.Layer() != IngestLayerNumeric || ingestErr.Path() != "$.n" {
					t.Fatalf(
						"IngestError = (layer=%q, path=%q), want (numeric, $.n)",
						ingestErr.Layer(),
						ingestErr.Path(),
					)
				}
				requireTypedSinkUncommitted(t, builder, 0)
			})

			t.Run("soft-numeric-soft-parser", func(t *testing.T) {
				parser := newTestSIMDParser(t)
				cfg, err := NewConfig(
					WithParserFailureMode(IngestFailureSoft),
					WithNumericFailureMode(IngestFailureSoft),
				)
				if err != nil {
					t.Fatalf("NewConfig: %v", err)
				}
				builder, err := NewBuilder(cfg, 1, WithParser(parser))
				if err != nil {
					t.Fatalf("NewBuilder: %v", err)
				}

				if err := builder.AddDocument(DocID(0), tc.jsonDoc); err != nil {
					t.Fatalf("AddDocument: %v", err)
				}
				requireTypedSinkUncommitted(t, builder, 1)
			})
		})
	}
}

func TestSIMDParserRejectedNumericFallbackPreservesTransformerPolicy(t *testing.T) {
	parsers := []struct {
		name   string
		parser Parser
	}{
		{name: "stdlib", parser: stdlibParser{}},
		{name: "simd", parser: newTestSIMDParser(t)},
	}

	for _, tc := range parsers {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := NewConfig(
				WithParserFailureMode(IngestFailureHard),
				WithNumericFailureMode(IngestFailureSoft),
				WithCustomTransformer("$.z", "reject", func(any) (any, bool) {
					return nil, false
				}),
			)
			if err != nil {
				t.Fatalf("NewConfig: %v", err)
			}
			builder, err := NewBuilder(cfg, 1, WithParser(tc.parser))
			if err != nil {
				t.Fatalf("NewBuilder: %v", err)
			}

			addErr := builder.AddDocument(DocID(0), []byte(`{"z":1e400}`))
			var ingestErr *IngestError
			if !stderrors.As(addErr, &ingestErr) {
				t.Fatalf("AddDocument error = %v, want *IngestError", addErr)
			}
			if ingestErr.Layer() != IngestLayerTransformer || ingestErr.Path() != "$.z" {
				t.Fatalf(
					"IngestError = (layer=%q, path=%q), want (transformer, $.z)",
					ingestErr.Layer(),
					ingestErr.Path(),
				)
			}
			requireTypedSinkUncommitted(t, builder, 0)
		})
	}
}

func TestSIMDParserRejectedNumericFallbackHonorsLastKeyWins(t *testing.T) {
	fx := parityFixture{
		Name:   "simd-rejected-numeric-last-key-wins",
		Config: DefaultConfig,
		NumRGs: 2,
		JSONDocs: [][]byte{
			[]byte(`{"n":1e400,"n":1}`),
			[]byte(`{"n":2}`),
		},
	}

	stdlibEncoded := buildAndEncodeWithParser(t, fx, stdlibParser{})
	simdEncoded := buildAndEncodeWithParser(t, fx, newTestSIMDParser(t))
	assertByteIdentical(t, fx.Name, simdEncoded, stdlibEncoded)
}

func TestSIMDParserNumericRoutingRecursesIntoArraysAndObjects(t *testing.T) {
	tests := []struct {
		name     string
		jsonDoc  []byte
		wantPath string
	}{
		{
			name:     "nested-in-array",
			jsonDoc:  []byte(`{"items":[1,1e400]}`),
			wantPath: "$.items[1]",
		},
		{
			name:     "nested-in-object",
			jsonDoc:  []byte(`{"a":{"b":1e400}}`),
			wantPath: "$.a.b",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parser := newTestSIMDParser(t)
			cfg, err := NewConfig(
				WithParserFailureMode(IngestFailureSoft),
				WithNumericFailureMode(IngestFailureHard),
			)
			if err != nil {
				t.Fatalf("NewConfig: %v", err)
			}
			builder, err := NewBuilder(cfg, 1, WithParser(parser))
			if err != nil {
				t.Fatalf("NewBuilder: %v", err)
			}

			addErr := builder.AddDocument(DocID(0), tc.jsonDoc)
			var ingestErr *IngestError
			if !stderrors.As(addErr, &ingestErr) {
				t.Fatalf("AddDocument error = %v, want *IngestError", addErr)
			}
			if ingestErr.Layer() != IngestLayerNumeric || ingestErr.Path() != tc.wantPath {
				t.Fatalf(
					"IngestError = (layer=%q, path=%q), want (numeric, %s)",
					ingestErr.Layer(),
					ingestErr.Path(),
					tc.wantPath,
				)
			}
		})
	}
}

func TestSIMDParserNumericRoutingPicksDeterministicFirstOffender(t *testing.T) {
	tests := []struct {
		name     string
		jsonDoc  []byte
		wantPath string
	}{
		{
			name:     "object-sorted-key-order",
			jsonDoc:  []byte(`{"z_bad":1e400,"a_bad":1e400}`),
			wantPath: "$.a_bad",
		},
		{
			name:     "array-ascending-index-order",
			jsonDoc:  []byte(`{"nums":[1e400,2e400]}`),
			wantPath: "$.nums[0]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parser := newTestSIMDParser(t)
			cfg, err := NewConfig(
				WithParserFailureMode(IngestFailureSoft),
				WithNumericFailureMode(IngestFailureHard),
			)
			if err != nil {
				t.Fatalf("NewConfig: %v", err)
			}
			builder, err := NewBuilder(cfg, 1, WithParser(parser))
			if err != nil {
				t.Fatalf("NewBuilder: %v", err)
			}

			addErr := builder.AddDocument(DocID(0), tc.jsonDoc)
			var ingestErr *IngestError
			if !stderrors.As(addErr, &ingestErr) {
				t.Fatalf("AddDocument error = %v, want *IngestError", addErr)
			}
			if ingestErr.Layer() != IngestLayerNumeric || ingestErr.Path() != tc.wantPath {
				t.Fatalf(
					"IngestError = (layer=%q, path=%q), want (numeric, %s)",
					ingestErr.Layer(),
					ingestErr.Path(),
					tc.wantPath,
				)
			}
		})
	}
}

func TestRouteSIMDWellFormedFallbackSkipsAtNestingDepthLimit(t *testing.T) {
	depth := simdNestingDepthLimit
	jsonDoc := []byte(strings.Repeat(`{"a":`, depth) + "1e400" + strings.Repeat("}", depth))

	builder, err := NewBuilder(DefaultConfig(), 1)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	fallbackErr, routed := routeSIMDWellFormedFallback(jsonDoc, 0, builder)
	if fallbackErr != nil || routed {
		t.Fatalf("routeSIMDWellFormedFallback() = (%v, %v), want (nil, false)", fallbackErr, routed)
	}
}

func TestSIMDParserNestingDepthBoundaryContract(t *testing.T) {
	acceptedFixture := parityFixture{
		Name:   "simd-nesting-depth-accepted-boundary",
		Config: DefaultConfig,
		NumRGs: 1,
		JSONDocs: [][]byte{
			nestedObjectJSON(simdNestingDepthLimit - 1),
		},
	}
	stdlibEncoded := buildAndEncodeWithParser(t, acceptedFixture, stdlibParser{})
	simdEncoded := buildAndEncodeWithParser(t, acceptedFixture, newTestSIMDParser(t))
	assertByteIdentical(t, acceptedFixture.Name, simdEncoded, stdlibEncoded)

	rejectedDoc := nestedObjectJSON(simdNestingDepthLimit)
	stdlibBuilder, err := NewBuilder(DefaultConfig(), 1)
	if err != nil {
		t.Fatalf("NewBuilder stdlib: %v", err)
	}
	if err := stdlibBuilder.AddDocument(DocID(0), rejectedDoc); err != nil {
		t.Fatalf("stdlib AddDocument at SIMD depth limit: %v", err)
	}

	simdBuilder, err := NewBuilder(DefaultConfig(), 1, WithParser(newTestSIMDParser(t)))
	if err != nil {
		t.Fatalf("NewBuilder SIMD: %v", err)
	}
	addErr := simdBuilder.AddDocument(DocID(0), rejectedDoc)
	var ingestErr *IngestError
	if !stderrors.As(addErr, &ingestErr) {
		t.Fatalf("SIMD AddDocument error = %v, want *IngestError", addErr)
	}
	if ingestErr.Layer() != IngestLayerParser {
		t.Fatalf("SIMD IngestError.Layer() = %q, want parser", ingestErr.Layer())
	}
	if !stderrors.Is(addErr, purejson.ErrDepthLimitExceeded) {
		t.Fatalf("errors.Is(%v, purejson.ErrDepthLimitExceeded) = false", addErr)
	}
	if simdBuilder.Err() != nil {
		t.Fatalf("SIMD builder.Err() = %v, want nil for document-local depth failure", simdBuilder.Err())
	}
}

func TestSIMDParserMalformedTrailingNumericKnownPolicyAsymmetry(t *testing.T) {
	cfg, err := NewConfig(
		WithParserFailureMode(IngestFailureHard),
		WithNumericFailureMode(IngestFailureSoft),
	)
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	jsonDoc := []byte(`1e400 garbage`)

	stdlibBuilder, err := NewBuilder(cfg, 1)
	if err != nil {
		t.Fatalf("NewBuilder stdlib: %v", err)
	}
	if err := stdlibBuilder.AddDocument(DocID(0), jsonDoc); err != nil {
		t.Fatalf("stdlib AddDocument: %v", err)
	}
	requireTypedSinkUncommitted(t, stdlibBuilder, 1)

	simdBuilder, err := NewBuilder(cfg, 1, WithParser(newTestSIMDParser(t)))
	if err != nil {
		t.Fatalf("NewBuilder SIMD: %v", err)
	}
	addErr := simdBuilder.AddDocument(DocID(0), jsonDoc)
	var ingestErr *IngestError
	if !stderrors.As(addErr, &ingestErr) {
		t.Fatalf("SIMD AddDocument error = %v, want *IngestError", addErr)
	}
	if ingestErr.Layer() != IngestLayerParser {
		t.Fatalf("SIMD IngestError.Layer() = %q, want parser", ingestErr.Layer())
	}
	requireTypedSinkUncommitted(t, simdBuilder, 0)
}

func nestedObjectJSON(depth int) []byte {
	return []byte(strings.Repeat(`{"a":`, depth) + "0" + strings.Repeat("}", depth))
}

func TestSIMDParserMalformedJSONStillUsesParserPolicy(t *testing.T) {
	parser := newTestSIMDParser(t)
	cfg, err := NewConfig(
		WithParserFailureMode(IngestFailureSoft),
		WithNumericFailureMode(IngestFailureHard),
	)
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	builder, err := NewBuilder(cfg, 1, WithParser(parser))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	if err := builder.AddDocument(DocID(0), []byte(`{"n":}`)); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	requireTypedSinkUncommitted(t, builder, 1)
}

func TestSIMDParserCloseIsIdempotent(t *testing.T) {
	parser := newTestSIMDParser(t)
	if err := parser.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := parser.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestSIMDParserCloseErrorPropagatesWhenParserBusy(t *testing.T) {
	cp := newTestSIMDParser(t)
	sp, ok := cp.(*simdParser)
	if !ok {
		t.Fatalf("NewSIMDParser() = %T, want *simdParser", cp)
	}

	doc, err := sp.parser.Parse([]byte(`{"a":1}`))
	if err != nil {
		t.Fatalf("underlying Parse: %v", err)
	}

	closeErr := cp.Close()
	if closeErr == nil {
		t.Fatal("Close() while document is live = nil, want ErrParserBusy")
	}
	if !stderrors.Is(closeErr, purejson.ErrParserBusy) {
		t.Fatalf("errors.Is(%v, purejson.ErrParserBusy) = false", closeErr)
	}
	if !strings.Contains(closeErr.Error(), "close pure-simdjson SIMD parser") {
		t.Fatalf("Close() error = %q, want cleanup context", closeErr.Error())
	}

	if err := doc.Close(); err != nil {
		t.Fatalf("doc.Close: %v", err)
	}
	if err := cp.Close(); err != nil {
		t.Fatalf("Close() after releasing live doc = %v, want nil", err)
	}
}

func TestSIMDParserParseAfterExternalCloseReturnsUsableBuilderError(t *testing.T) {
	parser := newTestSIMDParser(t)
	if err := parser.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	cfg, err := NewConfig(WithParserFailureMode(IngestFailureHard))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	builder, err := NewBuilder(cfg, 2, WithParser(parser))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	jsonDocs := [][]byte{[]byte(`{"a":1}`), []byte(`{"b":2}`)}
	for i, jsonDoc := range jsonDocs {
		addErr := builder.AddDocument(DocID(i), jsonDoc)
		var ingestErr *IngestError
		if !stderrors.As(addErr, &ingestErr) {
			t.Fatalf("AddDocument[%d] error = %v, want *IngestError", i, addErr)
		}
		if ingestErr.Layer() != IngestLayerParser {
			t.Fatalf("AddDocument[%d] IngestError.Layer() = %q, want parser", i, ingestErr.Layer())
		}
		if builder.Err() != nil {
			t.Fatalf("builder.Err() after AddDocument[%d] = %v, want nil", i, builder.Err())
		}
	}
}
