package gin

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const pureSIMDJSONModule = "github.com/amikos-tech/pure-simdjson"

func TestNoticeVersionGuard(t *testing.T) {
	t.Run("aligned notice passes", func(t *testing.T) {
		dir := copyNoticeGuardInputs(t)
		runNoticeVersionGuard(t, dir)
	})

	t.Run("stale version anywhere in NOTICE fails", func(t *testing.T) {
		dir := copyNoticeGuardInputs(t)
		appendNotice(t, dir, "\nHistorical note: previously pinned v0.1.2 before the current pin.\n")

		output := runNoticeVersionGuardFailure(t, dir)
		requireOutputContains(t, output, "NOTICE.md", "v0.1.7")
	})

	t.Run("replacement version is effective", func(t *testing.T) {
		dir := copyNoticeGuardInputs(t)
		appendFileText(t, filepath.Join(dir, "go.mod"), "\nreplace "+pureSIMDJSONModule+" v0.1.7 => "+pureSIMDJSONModule+" v0.1.8\n")
		replaceFileText(t, filepath.Join(dir, "NOTICE.md"), "v0.1.7", "v0.1.8")

		runNoticeVersionGuard(t, dir)
	})

	t.Run("unversioned local replacement fails", func(t *testing.T) {
		dir := copyNoticeGuardInputs(t)
		localModule := filepath.Join(dir, "local-pure-simdjson")
		if err := os.Mkdir(localModule, 0o700); err != nil {
			t.Fatalf("Mkdir(%s) error = %v", localModule, err)
		}
		if err := os.WriteFile(filepath.Join(localModule, "go.mod"), []byte("module "+pureSIMDJSONModule+"\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s/go.mod) error = %v", localModule, err)
		}
		appendFileText(t, filepath.Join(dir, "go.mod"), "\nreplace "+pureSIMDJSONModule+" v0.1.7 => ./local-pure-simdjson\n")

		output := runNoticeVersionGuardFailure(t, dir)
		requireOutputContains(t, output, "effective replacement version", "not a complete Go semantic-version token")
	})

	t.Run("invisible version character is escaped", func(t *testing.T) {
		dir := copyNoticeGuardInputs(t)
		replaceFileText(t, filepath.Join(dir, "NOTICE.md"), "## pure-simdjson v0.1.7", "## pure-simdjson v\u200b0.1.7")

		output := runNoticeVersionGuardFailure(t, dir)
		requireOutputContains(t, output, "NOTICE.md", "\\342\\200\\213")
	})

	t.Run("CI runs the dedicated guard before golangci-lint", func(t *testing.T) {
		workflow := readTestFile(t, filepath.Join(repositoryRoot(t), ".github", "workflows", "ci.yml"))
		lintJob := ciJobSection(t, string(workflow), "lint")

		checkout := strings.Index(lintJob, "      - name: Checkout repository\n")
		setupGo := strings.Index(lintJob, "      - name: Set up Go\n")
		validatorMarkers := strings.Index(lintJob, "      - name: Check validator markers\n")
		noticeGuard := strings.Index(lintJob, "      - name: Check NOTICE version alignment\n        run: make check-notice-version\n")
		golangCILint := strings.Index(lintJob, "      - name: Run golangci-lint\n        uses: golangci/golangci-lint-action@")

		if checkout < 0 || setupGo < 0 || validatorMarkers < 0 || noticeGuard < 0 || golangCILint < 0 {
			t.Fatalf("lint job does not contain the required dedicated NOTICE guard sequence:\n%s", lintJob)
		}
		if checkout >= setupGo || setupGo >= validatorMarkers || validatorMarkers >= noticeGuard || noticeGuard >= golangCILint {
			t.Fatalf("lint job NOTICE guard step has the wrong order:\n%s", lintJob)
		}
	})
}

func copyNoticeGuardInputs(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	for _, name := range []string{"Makefile", "NOTICE.md", "go.mod", "go.sum"} {
		source := filepath.Join(repositoryRoot(t), name)
		destination := filepath.Join(dir, name)
		contents := readTestFile(t, source)
		if err := os.WriteFile(destination, contents, 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", destination, err)
		}
	}
	return dir
}

func runNoticeVersionGuard(t *testing.T, dir string) {
	t.Helper()

	command := noticeVersionGuardCommand(dir)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed: %v\n%s", strings.Join(command.Args, " "), err, output)
	}
}

func runNoticeVersionGuardFailure(t *testing.T, dir string) string {
	t.Helper()

	command := noticeVersionGuardCommand(dir)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("%s unexpectedly succeeded\n%s", strings.Join(command.Args, " "), output)
	}
	return string(output)
}

func noticeVersionGuardCommand(dir string) *exec.Cmd {
	command := exec.Command("make", "--no-print-directory", "-C", dir, "check-notice-version")
	command.Env = append(os.Environ(), "LC_ALL=en_US.UTF-8")
	return command
}

func appendNotice(t *testing.T, dir, extra string) {
	t.Helper()

	appendFileText(t, filepath.Join(dir, "NOTICE.md"), extra)
}

func appendFileText(t *testing.T, path, extra string) {
	t.Helper()

	contents := readTestFile(t, path)
	if err := os.WriteFile(path, append(contents, extra...), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func replaceFileText(t *testing.T, path, old, new string) {
	t.Helper()

	contents := readTestFile(t, path)
	if !strings.Contains(string(contents), old) {
		t.Fatalf("%s does not contain %q", path, old)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(contents), old, new)), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return contents
}

func repositoryRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Dir(filename)
}

func ciJobSection(t *testing.T, workflow, job string) string {
	t.Helper()

	lines := strings.Split(workflow, "\n")
	jobHeader := "  " + job + ":"
	start := -1
	for index, line := range lines {
		if line == jobHeader {
			start = index
			break
		}
	}
	if start < 0 {
		t.Fatalf("workflow does not contain %q job", job)
	}

	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		if strings.HasPrefix(lines[index], "  ") && !strings.HasPrefix(lines[index], "    ") && strings.HasSuffix(lines[index], ":") {
			end = index
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

func requireOutputContains(t *testing.T, output string, expected ...string) {
	t.Helper()

	for _, value := range expected {
		if !strings.Contains(output, value) {
			t.Fatalf("guard output does not contain %q:\n%s", value, output)
		}
	}
}
