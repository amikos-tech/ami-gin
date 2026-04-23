package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestExperimentCommandHasNoForbiddenDependencies(t *testing.T) {
	moduleRoot := findExperimentModuleRoot(t)
	goModPath := filepath.Join(moduleRoot, "go.mod")

	data, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", goModPath, err)
	}

	for _, forbidden := range []string{
		"spf13/cobra",
		"spf13/pflag",
		"urfave/cli",
		"bubbletea",
		"lipgloss",
		"fatih/color",
		"chzyer/readline",
		"mattn/go-isatty",
	} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("go.mod unexpectedly contains forbidden dependency %q", forbidden)
		}
	}
}

func TestExperimentCommandHasNoForbiddenImports(t *testing.T) {
	files := readExperimentCommandFiles(t)

	forbiddenImports := []string{
		`"github.com/spf13/cobra"`,
		`"github.com/spf13/pflag"`,
		`"github.com/urfave/cli"`,
		`"github.com/charmbracelet/bubbletea"`,
		`"github.com/charmbracelet/lipgloss"`,
		`"github.com/fatih/color"`,
		`"github.com/chzyer/readline"`,
		`"github.com/mattn/go-isatty"`,
	}

	for path, data := range files {
		for _, forbidden := range forbiddenImports {
			if bytes.Contains(data, []byte(forbidden)) {
				t.Fatalf("%s unexpectedly imports %s", path, forbidden)
			}
		}
	}
}

func TestExperimentCommandHasNoTTYLogic(t *testing.T) {
	files := readExperimentCommandFiles(t)

	forbiddenPatterns := []string{
		"isatty",
		"go-isatty",
		"term.IsTerminal",
		"Stdout.Fd()",
		"color.NoColor",
		`\x1b[`,
		`\033[`,
	}

	for path, data := range files {
		text := string(data)
		for _, forbidden := range forbiddenPatterns {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s unexpectedly contains forbidden TTY/color pattern %q", path, forbidden)
			}
		}
	}
}

func TestExperimentUsageDoesNotExposeParserFlag(t *testing.T) {
	parserFlagToken := regexp.MustCompile(`(^|[^A-Za-z0-9_-])--parser([^A-Za-z0-9_-]|$)`)
	parserFlagRegistration := regexp.MustCompile(`\b(?:Bool|Duration|Int|String|Uint|Var)\("parser"`)

	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(): %v", err)
	}
	os.Stdout = w
	printUsage()
	_ = w.Close()
	os.Stdout = originalStdout

	usage, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("io.ReadAll(pipe): %v", err)
	}
	if parserFlagToken.Match(usage) {
		t.Fatalf("printUsage unexpectedly exposes --parser: %s", string(usage))
	}

	files := readExperimentCommandFiles(t)
	for path, data := range files {
		if parserFlagToken.Match(data) {
			t.Fatalf("%s unexpectedly mentions --parser", path)
		}
		if parserFlagRegistration.Match(data) {
			t.Fatalf("%s unexpectedly registers a parser flag", path)
		}
	}
}

func findExperimentModuleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd(): %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate module root from working directory")
		}
		dir = parent
	}
}

func readExperimentCommandFiles(t *testing.T) map[string][]byte {
	t.Helper()

	moduleRoot := findExperimentModuleRoot(t)
	matches, err := filepath.Glob(filepath.Join(moduleRoot, "cmd", "gin-index", "*.go"))
	if err != nil {
		t.Fatalf("Glob(cmd/gin-index/*.go): %v", err)
	}

	files := make(map[string][]byte, len(matches))
	for _, match := range matches {
		if strings.HasSuffix(match, "_test.go") {
			continue
		}
		data, err := os.ReadFile(match)
		if err != nil {
			t.Fatalf("ReadFile(%q): %v", match, err)
		}
		files[match] = data
	}
	return files
}
