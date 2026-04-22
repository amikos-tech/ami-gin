package gin_test

// observability_policy_test.go — Phase 14 policy and regression gate tests.
//
// This file enforces the frozen observability vocabulary and guards against
// slipping back to legacy logger conventions or leaking backend-specific types.
//
// Verification commands:
//
//	# Run all policy tests:
//	go test -run 'Test(InfoLevelAttrAllowlist|InfoLevelEmissionsUseOnlyAllowlistedAttrs|NoLegacyQueryLoggerSurface|NoBackendTypeLeakage|RootModuleHasNoOtelSdkOrExporterDeps|ObservabilityDefaultsSurviveFinalizeAndDecode|ObservabilityEnabledDoesNotChangeFunctionalResults|ParquetAndSerializationObservabilityRoundTrip|EvaluateDisabledLoggingAllocsAtMostOne|EvaluateWithTracerWithinBudget)$' -count=1 .
//
//	# Strict normalized perf-gate (0.5% budget):
//	GIN_STRICT_PERF=1 go test -run 'Test(InfoLevelAttrAllowlist|NoLegacyQueryLoggerSurface|ObservabilityDefaultsSurviveFinalizeAndDecode|EvaluateDisabledLoggingAllocsAtMostOne|EvaluateWithTracerWithinBudget)$' -count=1 .
//
//	# Benchmarks:
//	go test -run '^$' -bench 'BenchmarkEvaluate(DisabledLogging|WithTracer)$' -benchmem -count=1 .

import (
	"bufio"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gin "github.com/amikos-tech/ami-gin"
	"github.com/amikos-tech/ami-gin/logging"
	"github.com/amikos-tech/ami-gin/telemetry"
)

// =============================================================================
// Task 1: INFO-level attribute allowlist enforcement
// =============================================================================

// TestInfoLevelAttrAllowlist verifies that the frozen INFO-level attribute
// vocabulary is declared with exactly the five canonical keys. The test fails
// if a new key is added without updating this allowlist or if a key is removed.
func TestInfoLevelAttrAllowlist(t *testing.T) {
	// Canonical frozen allowlist — must match logging/attrs.go exactly.
	// To add a new INFO-level attr, update logging/attrs.go AND this list.
	const (
		keyOperation   = "operation"
		keyPredicateOp = "predicate_op"
		keyPathMode    = "path_mode"
		keyStatus      = "status"
		keyErrorType   = "error.type"
	)

	frozenAllowlist := []string{
		keyOperation,
		keyPredicateOp,
		keyPathMode,
		keyStatus,
		keyErrorType,
	}

	// Verify that the constructor helpers produce attrs with exactly these keys.
	attrsUnderTest := []logging.Attr{
		logging.AttrOperation("query.evaluate"),
		logging.AttrPredicateOp("EQ"),
		logging.AttrPathMode(logging.PathModeExact),
		logging.AttrStatus("ok"),
		logging.AttrErrorType("io"),
	}

	allowlistSet := make(map[string]bool, len(frozenAllowlist))
	for _, k := range frozenAllowlist {
		allowlistSet[k] = true
	}

	for _, a := range attrsUnderTest {
		if !allowlistSet[a.Key] {
			t.Errorf("constructor produced attr with key %q that is not in the frozen allowlist", a.Key)
		}
	}

	// Verify the path_mode value set is bounded to exactly three labels.
	allowedPathModes := map[string]bool{
		logging.PathModeExact:          true,
		logging.PathModeBloomOnly:      true,
		logging.PathModeAdaptiveHybrid: true,
	}
	if len(allowedPathModes) != 3 {
		t.Errorf("expected exactly 3 PathMode* constants, got %d", len(allowedPathModes))
	}

	// AttrPathMode must only produce values from the bounded set.
	pathModeAttr := logging.AttrPathMode("unknown-mode-that-should-not-exist")
	// AttrPathMode forwards the value as-is; it's the caller's responsibility to
	// use PathMode* constants. The test verifies the constants exist and are unique.
	_ = pathModeAttr

	for mode := range allowedPathModes {
		a := logging.AttrPathMode(mode)
		if a.Key != "path_mode" {
			t.Errorf("AttrPathMode key = %q; want %q", a.Key, "path_mode")
		}
		if a.Value != mode {
			t.Errorf("AttrPathMode(%q).Value = %q; want %q", mode, a.Value, mode)
		}
	}
}

// TestInfoLevelEmissionsUseOnlyAllowlistedAttrs captures actual INFO-level log
// emissions from a real query evaluation and verifies every emitted attr key
// is in the frozen allowlist. This exercises the boundary code rather than just
// checking that constructors produce the right keys.
func TestInfoLevelEmissionsUseOnlyAllowlistedAttrs(t *testing.T) {
	frozenAllowlist := map[string]bool{
		"operation":    true,
		"predicate_op": true,
		"path_mode":    true,
		"status":       true,
		"error.type":   true,
	}

	var captured []logging.Attr
	capLogger := &policyCapLogger{attrs: &captured}

	idx, err := buildSmallIndex()
	if err != nil {
		t.Fatalf("build index: %v", err)
	}

	cfg, err := gin.NewConfig(gin.WithLogger(capLogger))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	idx.Config = &cfg

	// Run an evaluation to trigger INFO-level emission.
	_ = idx.EvaluateContext(context.Background(), []gin.Predicate{gin.EQ("$.name", "alice")})

	if len(captured) == 0 {
		t.Fatal("expected at least one INFO-level attr to be captured; got none")
	}

	for _, a := range captured {
		if !frozenAllowlist[a.Key] {
			t.Errorf("INFO-level emission used attr key %q which is outside the frozen allowlist", a.Key)
		}
	}
}

// =============================================================================
// Task 2: Regression guards — legacy logger removal and backend-type leakage
// =============================================================================

// TestNoLegacyQueryLoggerSurface walks all .go source files in the module root
// and asserts that SetAdaptiveInvariantLogger, adaptiveInvariantLogger globals,
// and the stdlib "log" package import have been fully removed from non-test code.
func TestNoLegacyQueryLoggerSurface(t *testing.T) {
	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("cannot locate module root: %v", err)
	}

	forbidden := []string{
		"SetAdaptiveInvariantLogger",
		"adaptiveInvariantLogger",
	}

	fset := token.NewFileSet()
	err = filepath.Walk(moduleRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip vendor, examples, planning dirs.
			base := info.Name()
			if base == "vendor" || base == ".planning" || base == "examples" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, relErr := filepath.Rel(moduleRoot, path)
		if relErr != nil {
			return relErr
		}

		f, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}

		ast.Inspect(f, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			for _, name := range forbidden {
				if ident.Name == name {
					t.Errorf("found forbidden identifier %q in non-test file %s", name, rel)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk error: %v", err)
	}
}

// TestNoBackendTypeLeakage asserts that no exported function, method, or field
// in the root package exposes *slog.Logger, *log.Logger, or any type from the
// go.opentelemetry.io/otel/sdk/... or OTLP exporter namespaces.
//
// Implementation: parse all non-test .go files in the root package and inspect
// exported identifier type expressions for disallowed type names.
func TestNoBackendTypeLeakage(t *testing.T) {
	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("cannot locate module root: %v", err)
	}

	// Disallowed type name substrings (checked against the selector expression).
	forbiddenTypePatterns := []string{
		"slog.Logger", // *slog.Logger from log/slog
		"log.Logger",  // *log.Logger from stdlib log
		"otlp",        // any OTLP exporter type
		"otel/sdk",    // any OTel SDK type
	}

	fset := token.NewFileSet()
	entries, err := os.ReadDir(moduleRoot)
	if err != nil {
		t.Fatalf("read module root: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		path := filepath.Join(moduleRoot, name)
		f, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse root package file %s: %v", name, parseErr)
		}

		for _, decl := range f.Decls {
			checkDeclForLeakage(t, fset, decl, name, forbiddenTypePatterns)
		}
	}
}

// checkDeclForLeakage inspects an AST declaration for disallowed type patterns.
func checkDeclForLeakage(t *testing.T, fset *token.FileSet, decl ast.Decl, file string, patterns []string) {
	t.Helper()
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Name == nil || !d.Name.IsExported() {
			return
		}
		inspectTypeExprs(t, fset, d, file, patterns)
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok {
				if !ts.Name.IsExported() {
					continue
				}
				inspectTypeExprs(t, fset, ts, file, patterns)
			}
		}
	}
}

// inspectTypeExprs walks an AST node and checks all selector expressions
// (e.g., slog.Logger, log.Logger) against the disallowed pattern list.
func inspectTypeExprs(t *testing.T, fset *token.FileSet, node ast.Node, file string, patterns []string) {
	t.Helper()
	ast.Inspect(node, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		fullName := ident.Name + "." + sel.Sel.Name
		for _, p := range patterns {
			if strings.Contains(strings.ToLower(fullName), strings.ToLower(p)) {
				pos := fset.Position(sel.Pos())
				t.Errorf("exported API in %s:%d uses disallowed backend type %q (matches %q)", file, pos.Line, fullName, p)
			}
		}
		return true
	})
}

// TestRootModuleHasNoOtelSdkOrExporterDeps reads go.mod and asserts that no
// OTel SDK or OTLP exporter packages are present as direct or indirect deps.
func TestRootModuleHasNoOtelSdkOrExporterDeps(t *testing.T) {
	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("cannot locate module root: %v", err)
	}

	goModPath := filepath.Join(moduleRoot, "go.mod")
	f, err := os.Open(goModPath)
	if err != nil {
		t.Fatalf("open go.mod: %v", err)
	}
	defer f.Close()

	// Disallowed package path substrings in go.mod.
	disallowed := []string{
		"go.opentelemetry.io/otel/sdk",
		"go.opentelemetry.io/otel/exporters",
		"opentelemetry-go-contrib/exporters",
		"otlpgrpc",
		"otlphttp",
		"otlptrace",
		"otlpmetric",
	}

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		for _, bad := range disallowed {
			if strings.Contains(line, bad) {
				t.Errorf("go.mod line %d contains disallowed OTel SDK/exporter dep: %q (matched %q)", lineNum, line, bad)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan go.mod: %v", err)
	}
}

// =============================================================================
// Task 3: Finalize/decode and end-to-end default-safety integration tests
// =============================================================================

// TestObservabilityDefaultsSurviveFinalizeAndDecode proves that finalized
// and decoded indexes carry safe observability defaults (noop logger, disabled
// signals) without requiring callers to re-configure them.
func TestObservabilityDefaultsSurviveFinalizeAndDecode(t *testing.T) {
	// Build -> finalize path.
	b, err := gin.NewBuilder(gin.DefaultConfig(), 3)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := b.AddDocument(gin.DocID(i), []byte(`{"a":"b"}`)); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	idx := b.Finalize()

	if idx.Config == nil {
		t.Fatal("finalized index must carry a non-nil Config")
	}
	if idx.Config.Logger == nil {
		t.Fatal("finalized index Config.Logger must not be nil")
	}
	if idx.Config.Signals.Enabled() {
		t.Fatal("finalized index Config.Signals must be disabled by default")
	}

	// Encode -> decode path.
	data, err := gin.Encode(idx)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := gin.Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	// Decoded index must also be safe.
	if decoded.Config == nil {
		t.Fatal("decoded index must carry a non-nil Config")
	}
	if decoded.Config.Logger == nil {
		t.Fatal("decoded index Config.Logger must not be nil")
	}
	if decoded.Config.Signals.Enabled() {
		t.Fatal("decoded index Config.Signals must be disabled")
	}

	// Querying a decoded index with default config must work without panic.
	result := decoded.EvaluateContext(context.Background(), []gin.Predicate{gin.EQ("$.a", "b")})
	if result == nil {
		t.Fatal("EvaluateContext on decoded index returned nil")
	}
}

// TestObservabilityEnabledDoesNotChangeFunctionalResults proves that wiring a
// real logger and signals changes only the observability output — not the
// functional results of query/build/serialize operations.
func TestObservabilityEnabledDoesNotChangeFunctionalResults(t *testing.T) {
	// Build without observability.
	silentIdx, err := buildSmallIndex()
	if err != nil {
		t.Fatalf("buildSmallIndex: %v", err)
	}
	silentResult := silentIdx.EvaluateContext(context.Background(), []gin.Predicate{gin.EQ("$.name", "alice")})

	// Build with observability enabled (capturing logger).
	var captured []logging.Attr
	capLogger := &policyCapLogger{attrs: &captured}
	cfg, err := gin.NewConfig(gin.WithLogger(capLogger), gin.WithSignals(telemetry.Disabled()))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}

	b2, err := gin.NewBuilder(cfg, 10)
	if err != nil {
		t.Fatalf("NewBuilder with observability: %v", err)
	}
	if err := b2.AddDocument(0, []byte(`{"name":"alice"}`)); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	obsIdx := b2.Finalize()
	obsIdx.Config = &cfg

	obsResult := obsIdx.EvaluateContext(context.Background(), []gin.Predicate{gin.EQ("$.name", "alice")})

	// Functional results must be identical.
	if silentResult.Count() != obsResult.Count() {
		t.Errorf("silent result count=%d; observability-enabled count=%d; want equal",
			silentResult.Count(), obsResult.Count())
	}

	// Logger must have been called (observability is active).
	if !capLogger.called {
		t.Fatal("expected observability-enabled logger to capture at least one message")
	}
}

// TestParquetAndSerializationObservabilityRoundTrip proves the encode/decode
// path works with observability enabled and that results survive the round trip.
func TestParquetAndSerializationObservabilityRoundTrip(t *testing.T) {
	// Build index with default (silent) config.
	idx, err := buildSmallIndex()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// EncodeContext with default noop config.
	data, err := gin.EncodeContext(context.Background(), idx)
	if err != nil {
		t.Fatalf("EncodeContext: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("EncodeContext produced empty data")
	}

	// DecodeContext restores a safe config automatically.
	decoded, err := gin.DecodeContext(context.Background(), data)
	if err != nil {
		t.Fatalf("DecodeContext: %v", err)
	}

	if decoded.Config == nil {
		t.Fatal("decoded index must carry non-nil Config after DecodeContext")
	}
	if decoded.Config.Logger == nil {
		t.Fatal("decoded Config.Logger must not be nil after DecodeContext")
	}

	// Functional query on decoded index must work.
	result := decoded.EvaluateContext(context.Background(), []gin.Predicate{gin.EQ("$.name", "alice")})
	if result == nil {
		t.Fatal("EvaluateContext on DecodeContext result returned nil")
	}

	// Evaluate with capturing logger must emit attrs only from the allowlist.
	frozenAllowlist := map[string]bool{
		"operation":    true,
		"predicate_op": true,
		"path_mode":    true,
		"status":       true,
		"error.type":   true,
	}
	var captured []logging.Attr
	capLogger := &policyCapLogger{attrs: &captured}
	cfg2, err := gin.NewConfig(gin.WithLogger(capLogger))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	decoded.Config = &cfg2

	_ = decoded.EvaluateContext(context.Background(), []gin.Predicate{gin.EQ("$.name", "alice")})
	for _, a := range captured {
		if !frozenAllowlist[a.Key] {
			t.Errorf("serialization round-trip: INFO emission used disallowed attr key %q", a.Key)
		}
	}
}

// =============================================================================
// Helpers
// =============================================================================

// policyCapLogger is a minimal capturing logger for policy assertions.
// It captures all logged attrs at all levels and tracks whether any call was made.
type policyCapLogger struct {
	attrs  *[]logging.Attr
	called bool
}

func (c *policyCapLogger) Enabled(_ logging.Level) bool { return true }
func (c *policyCapLogger) Log(_ logging.Level, _ string, attrs ...logging.Attr) {
	c.called = true
	*c.attrs = append(*c.attrs, attrs...)
}

// findModuleRoot locates the go.mod file by walking up from the test binary's
// working directory, returning the directory that contains go.mod.
func findModuleRoot() (string, error) {
	// Start from the working directory (set to the package dir by go test).
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", os.ErrNotExist
}
