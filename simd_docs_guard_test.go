package gin

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/pkg/errors"
)

const simdDeploymentGuidePath = "docs/simd-deployment.md"

var completeSIMDModuleVersion = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?(?:\+[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?$`)

var simdLoadingVariables = []struct {
	file      string
	constName string
	want      string
}{
	{file: "library_loading.go", constName: "libraryEnvPath", want: "PURE_SIMDJSON_LIB_PATH"},
	{file: "internal/bootstrap/bootstrap.go", constName: "mirrorEnvVar", want: "PURE_SIMDJSON_BINARY_MIRROR"},
	{file: "internal/bootstrap/bootstrap.go", constName: "disableGHEnvVar", want: "PURE_SIMDJSON_DISABLE_GH_FALLBACK"},
	{file: "internal/bootstrap/cache.go", constName: "cacheDirEnvVar", want: "PURE_SIMDJSON_CACHE_DIR"},
}

type simdDocumentationInputs struct {
	effectiveVersion string
	guide            string
	example          string
	release          string
	readme           string
	changelog        string
	parser           string
	libraryLoading   string
	bootstrap        string
	cache            string
}

type simdModuleListing struct {
	Version string
	Dir     string
	Replace *simdModuleReplacement
}

type simdModuleReplacement struct {
	Version string
	Dir     string
}

func TestSIMDDocumentationContract(t *testing.T) {
	inputs := loadSIMDDocumentationInputs(t)
	if err := validateSIMDDocumentationInputs(inputs); err != nil {
		t.Fatalf("aligned SIMD documentation: %v", err)
	}

	loadingMutations := []struct {
		name       string
		field      *string
		oldValue   string
		diagnostic string
	}{
		{name: "library path", field: &inputs.libraryLoading, oldValue: "PURE_SIMDJSON_LIB_PATH", diagnostic: "loading variable libraryEnvPath"},
		{name: "binary mirror", field: &inputs.bootstrap, oldValue: "PURE_SIMDJSON_BINARY_MIRROR", diagnostic: "loading variable mirrorEnvVar"},
		{name: "fallback control", field: &inputs.bootstrap, oldValue: "PURE_SIMDJSON_DISABLE_GH_FALLBACK", diagnostic: "loading variable disableGHEnvVar"},
		{name: "cache directory", field: &inputs.cache, oldValue: "PURE_SIMDJSON_CACHE_DIR", diagnostic: "loading variable cacheDirEnvVar"},
	}
	for _, mutation := range loadingMutations {
		t.Run("rejects changed "+mutation.name, func(t *testing.T) {
			mutated := inputs
			switch mutation.name {
			case "library path":
				mutated.libraryLoading = strings.Replace(mutated.libraryLoading, mutation.oldValue, mutation.oldValue+"_DRIFT", 1)
			case "binary mirror", "fallback control":
				mutated.bootstrap = strings.Replace(mutated.bootstrap, mutation.oldValue, mutation.oldValue+"_DRIFT", 1)
			case "cache directory":
				mutated.cache = strings.Replace(mutated.cache, mutation.oldValue, mutation.oldValue+"_DRIFT", 1)
			}
			requireSIMDDocumentationFailure(t, mutated, mutation.diagnostic)
		})
	}

	mutations := []struct {
		name       string
		mutate     func(*simdDocumentationInputs)
		diagnostic string
	}{
		{
			name: "parser guide path",
			mutate: func(in *simdDocumentationInputs) {
				in.parser = strings.Replace(in.parser, "docs/simd-deployment.md", "docs/simd-moved.md", 1)
			},
			diagnostic: "parser guide path",
		},
		{
			name: "README guide link",
			mutate: func(in *simdDocumentationInputs) {
				in.readme = strings.Replace(in.readme, "](docs/simd-deployment.md)", "](docs/simd-moved.md)", 1)
			},
			diagnostic: "README guide link",
		},
		{
			name: "CHANGELOG guide link",
			mutate: func(in *simdDocumentationInputs) {
				in.changelog = strings.Replace(in.changelog, "](docs/simd-deployment.md)", "](docs/simd-moved.md)", 1)
			},
			diagnostic: "CHANGELOG guide link",
		},
		{
			name: "effective bootstrap pin",
			mutate: func(in *simdDocumentationInputs) {
				in.guide = strings.Replace(in.guide, "/blob/"+in.effectiveVersion+"/docs/bootstrap.md", "/blob/main/docs/bootstrap.md", 1)
			},
			diagnostic: "effective-tag bootstrap link",
		},
		{
			name: "release guide link",
			mutate: func(in *simdDocumentationInputs) {
				in.release = strings.Replace(in.release, "/docs/simd-deployment.md", "/docs/simd-moved.md", 1)
			},
			diagnostic: "release header guide link",
		},
		{
			name: "example body",
			mutate: func(in *simdDocumentationInputs) {
				in.guide = strings.Replace(in.guide, "const numRGs = 1", "const numRGs = 2", 1)
			},
			diagnostic: "SIMD example snippet",
		},
	}
	for _, mutation := range mutations {
		t.Run("rejects "+mutation.name+" drift", func(t *testing.T) {
			mutated := inputs
			mutation.mutate(&mutated)
			requireSIMDDocumentationFailure(t, mutated, mutation.diagnostic)
		})
	}

	t.Run("ignores unrelated upstream diagnostics", func(t *testing.T) {
		mutated := inputs
		mutated.libraryLoading += "\nconst unrelatedDiagnostic = \"PURE_SIMDJSON_WARN_LEAKS\"\n"
		if err := validateSIMDDocumentationInputs(mutated); err != nil {
			t.Fatalf("unrelated diagnostic expanded loading-variable contract: %v", err)
		}
	})

	t.Run("versioned replacement is effective", func(t *testing.T) {
		version, dir, err := effectiveSIMDModule(simdModuleListing{
			Version: "v0.1.0",
			Dir:     "/module-cache",
			Replace: &simdModuleReplacement{Version: sentinelReplacementVersion, Dir: "/replacement-cache"},
		})
		if err != nil {
			t.Fatalf("effectiveSIMDModule: %v", err)
		}
		if version != sentinelReplacementVersion || dir != "/replacement-cache" {
			t.Fatalf("effective module = (%q, %q), want (%q, %q)", version, dir, sentinelReplacementVersion, "/replacement-cache")
		}
	})

	t.Run("unversioned local replacement fails closed", func(t *testing.T) {
		_, _, err := effectiveSIMDModule(simdModuleListing{
			Version: "v0.1.0",
			Dir:     "/module-cache",
			Replace: &simdModuleReplacement{Dir: "/local-replacement"},
		})
		if err == nil || !strings.Contains(err.Error(), "unversioned effective replacement") {
			t.Fatalf("effectiveSIMDModule error = %v, want unversioned replacement diagnostic", err)
		}
	})
}

func requireSIMDDocumentationFailure(t *testing.T, inputs simdDocumentationInputs, diagnostic string) {
	t.Helper()

	err := validateSIMDDocumentationInputs(inputs)
	if err == nil || !strings.Contains(err.Error(), diagnostic) {
		t.Fatalf("documentation mutation error = %v, want diagnostic %q", err, diagnostic)
	}
}

func loadSIMDDocumentationInputs(t *testing.T) simdDocumentationInputs {
	t.Helper()

	root := repositoryRoot(t)
	listing := resolveSIMDModuleListing(t, root)
	version, moduleDir, err := effectiveSIMDModule(listing)
	if err != nil {
		t.Fatalf("resolve effective pure-simdjson module: %v", err)
	}

	return simdDocumentationInputs{
		effectiveVersion: version,
		guide:            string(readTestFile(t, filepath.Join(root, simdDeploymentGuidePath))),
		example:          string(readTestFile(t, filepath.Join(root, "simd_example_test.go"))),
		release:          string(readTestFile(t, filepath.Join(root, ".goreleaser.yml"))),
		readme:           string(readTestFile(t, filepath.Join(root, "README.md"))),
		changelog:        string(readTestFile(t, filepath.Join(root, "CHANGELOG.md"))),
		parser:           string(readTestFile(t, filepath.Join(root, "parser_simd.go"))),
		libraryLoading:   string(readTestFile(t, filepath.Join(moduleDir, "library_loading.go"))),
		bootstrap:        string(readTestFile(t, filepath.Join(moduleDir, "internal", "bootstrap", "bootstrap.go"))),
		cache:            string(readTestFile(t, filepath.Join(moduleDir, "internal", "bootstrap", "cache.go"))),
	}
}

func resolveSIMDModuleListing(t *testing.T, root string) simdModuleListing {
	t.Helper()

	download := exec.Command("go", "mod", "download", pureSIMDJSONModule)
	download.Dir = root
	if output, err := download.CombinedOutput(); err != nil {
		t.Fatalf("%s failed: %v: %s", strings.Join(download.Args, " "), err, strings.TrimSpace(string(output)))
	}

	command := exec.Command("go", "list", "-m", "-json", pureSIMDJSONModule)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatalf("%s failed: %v", strings.Join(command.Args, " "), err)
	}

	var listing simdModuleListing
	if err := json.Unmarshal(output, &listing); err != nil {
		t.Fatalf("decode %s output: %v", strings.Join(command.Args, " "), err)
	}
	return listing
}

func effectiveSIMDModule(listing simdModuleListing) (string, string, error) {
	version := listing.Version
	dir := listing.Dir
	if listing.Replace != nil {
		version = listing.Replace.Version
		dir = listing.Replace.Dir
		if version == "" {
			return "", "", errors.New("unversioned effective replacement cannot produce a pinned upstream link")
		}
	}
	if !completeSIMDModuleVersion.MatchString(version) {
		return "", "", errors.Errorf("effective module version %q is not a complete Go semantic-version token", version)
	}
	if dir == "" {
		return "", "", errors.New("effective module directory is empty")
	}
	return version, dir, nil
}

func validateSIMDDocumentationInputs(inputs simdDocumentationInputs) error {
	if !completeSIMDModuleVersion.MatchString(inputs.effectiveVersion) {
		return errors.Errorf("effective module version %q is not a complete Go semantic-version token", inputs.effectiveVersion)
	}

	upstreamSources := map[string]string{
		"library_loading.go":              inputs.libraryLoading,
		"internal/bootstrap/bootstrap.go": inputs.bootstrap,
		"internal/bootstrap/cache.go":     inputs.cache,
	}
	wantGuideVariables := make([]string, 0, len(simdLoadingVariables))
	for _, spec := range simdLoadingVariables {
		value, err := namedStringConstant(upstreamSources[spec.file], spec.file, spec.constName)
		if err != nil {
			return errors.Wrapf(err, "loading variable %s", spec.constName)
		}
		if value != spec.want {
			return errors.Errorf("loading variable %s = %q, want %q", spec.constName, value, spec.want)
		}
		wantGuideVariables = append(wantGuideVariables, value)
	}

	guideVariables, err := guideLoadingVariableAllowlist(inputs.guide)
	if err != nil {
		return err
	}
	if !equalStringSets(guideVariables, wantGuideVariables) {
		return errors.Errorf("guide loading-variable allowlist = %v, want exactly %v", guideVariables, sortedCopy(wantGuideVariables))
	}

	if !strings.Contains(inputs.parser, "see "+simdDeploymentGuidePath) {
		return errors.Errorf("parser guide path does not name %s", simdDeploymentGuidePath)
	}
	if !strings.Contains(inputs.readme, "]("+simdDeploymentGuidePath+")") {
		return errors.Errorf("README guide link does not name %s", simdDeploymentGuidePath)
	}
	if !strings.Contains(inputs.changelog, "]("+simdDeploymentGuidePath+")") {
		return errors.Errorf("CHANGELOG guide link does not name %s", simdDeploymentGuidePath)
	}

	bootstrapLink := "https://github.com/amikos-tech/" + "pure-simdjson/blob/" + inputs.effectiveVersion + "/docs/bootstrap.md"
	if !strings.Contains(inputs.guide, bootstrapLink) || strings.Contains(inputs.guide, "/blob/main/docs/bootstrap.md") {
		return errors.Errorf("effective-tag bootstrap link must be %s", bootstrapLink)
	}

	releaseLink := "https://github.com/amikos-tech/ami-gin/blob/{{ .Tag }}/" + simdDeploymentGuidePath
	if !strings.Contains(inputs.release, releaseLink) {
		return errors.Errorf("release header guide link must be %s", releaseLink)
	}
	for _, phrase := range []string{"optional SIMD parser", "opt-in build tag", "native dependency"} {
		if !strings.Contains(inputs.release, phrase) {
			return errors.Errorf("release header opt-in native wording is missing %q", phrase)
		}
	}

	if firstLine, _, _ := strings.Cut(inputs.example, "\n"); firstLine != "//go:build simdjson" {
		return errors.New("SIMD example must keep the simdjson build tag on line 1")
	}
	if !strings.Contains(inputs.example, "package gin_test") || !strings.Contains(inputs.example, "func ExampleNewSIMDParser()") {
		return errors.New("SIMD example must remain the external-package ExampleNewSIMDParser")
	}
	if regexp.MustCompile(`(?m)^\s*//\s*(?:Unordered )?Output:`).MatchString(inputs.example) {
		return errors.New("SIMD example must remain compile-only without an output directive")
	}

	guideSnippet, err := simdExampleMarkerBody(inputs.guide)
	if err != nil {
		return errors.Wrap(err, "SIMD example snippet in deployment guide")
	}
	exampleSnippet, err := simdExampleMarkerBody(inputs.example)
	if err != nil {
		return errors.Wrap(err, "SIMD example snippet in compile-only Example")
	}
	if guideSnippet != exampleSnippet {
		return errors.New("SIMD example snippet differs between deployment guide and compile-only Example")
	}

	return nil
}

func namedStringConstant(source, filename, name string) (string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), filename, source, 0)
	if err != nil {
		return "", errors.Wrap(err, "parse Go source")
	}

	var matches []string
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for index, identifier := range spec.Names {
			if identifier.Name != name || index >= len(spec.Values) {
				continue
			}
			literal, ok := spec.Values[index].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				continue
			}
			value, unquoteErr := strconv.Unquote(literal.Value)
			if unquoteErr == nil {
				matches = append(matches, value)
			}
		}
		return true
	})
	if len(matches) != 1 {
		return "", errors.Errorf("%s contains %d string constants named %s, want exactly 1", filename, len(matches), name)
	}
	return matches[0], nil
}

func guideLoadingVariableAllowlist(guide string) ([]string, error) {
	const sectionStart = "## Corporate mirror and hermetic loading"
	const sectionEnd = "## Caller-owned stdlib fallback"
	start := strings.Index(guide, sectionStart)
	if start < 0 {
		return nil, errors.New("guide loading-variable allowlist section is missing")
	}
	end := strings.Index(guide[start+len(sectionStart):], sectionEnd)
	if end < 0 {
		return nil, errors.New("guide loading-variable allowlist section has no end boundary")
	}
	section := guide[start : start+len(sectionStart)+end]
	matches := regexp.MustCompile("`(PURE_SIMDJSON_[A-Z0-9_]+)`").FindAllStringSubmatch(section, -1)
	variables := make(map[string]struct{})
	for _, match := range matches {
		variables[match[1]] = struct{}{}
	}
	result := make([]string, 0, len(variables))
	for variable := range variables {
		result = append(result, variable)
	}
	sort.Strings(result)
	return result, nil
}

func simdExampleMarkerBody(source string) (string, error) {
	const startMarker = "// SIMD_EXAMPLE_START"
	const endMarker = "// SIMD_EXAMPLE_END"
	if strings.Count(source, startMarker) != 1 || strings.Count(source, endMarker) != 1 {
		return "", errors.New("expected exactly one start marker and one end marker")
	}
	start := strings.Index(source, startMarker) + len(startMarker)
	end := strings.Index(source[start:], endMarker)
	if end < 0 {
		return "", errors.New("end marker precedes start marker")
	}
	return strings.TrimSpace(source[start : start+end]), nil
}

func equalStringSets(left, right []string) bool {
	left = sortedCopy(left)
	right = sortedCopy(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func sortedCopy(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}
