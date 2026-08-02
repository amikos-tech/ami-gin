package gin

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type simdWorkflowMatrixRow struct {
	runner   string
	goos     string
	goarch   string
	tier     string
	advisory string
	race     string
	release  string
	library  string
}

func TestSIMDWorkflowContract(t *testing.T) {
	workflow := string(readTestFile(t, filepath.Join(repositoryRoot(t), ".github", "workflows", "ci.yml")))

	t.Run("matrix_and_cache", func(t *testing.T) {
		for _, anchor := range []string{
			"permissions:\n  contents: read\n",
			"  test:\n",
			"  lint:\n",
			"  build:\n",
			"  simd:\n",
			"  govulncheck:\n",
		} {
			if !strings.Contains(workflow, anchor) {
				t.Errorf("workflow is missing required default anchor %q", anchor)
			}
		}

		simdJob := ciJobSection(t, workflow, "simd")
		expectedRows := []simdWorkflowMatrixRow{
			{runner: "ubuntu-latest", goos: "linux", goarch: "amd64", tier: "required", advisory: "false", race: "true", release: "linux-amd64", library: "libpure_simdjson.so"},
			{runner: "macos-15", goos: "darwin", goarch: "arm64", tier: "required", advisory: "false", race: "true", release: "darwin-arm64", library: "libpure_simdjson.dylib"},
			{runner: "ubuntu-24.04-arm", goos: "linux", goarch: "arm64", tier: "advisory", advisory: "true", race: "false", release: "linux-arm64", library: "libpure_simdjson.so"},
			{runner: "macos-15-intel", goos: "darwin", goarch: "amd64", tier: "advisory", advisory: "true", race: "false", release: "darwin-amd64", library: "libpure_simdjson.dylib"},
			{runner: "windows-latest", goos: "windows", goarch: "amd64", tier: "advisory", advisory: "true", race: "false", release: "windows-amd64-msvc", library: "pure_simdjson-msvc.dll"},
		}
		rows := parseSIMDWorkflowMatrix(t, simdJob)
		if !reflect.DeepEqual(rows, expectedRows) {
			t.Fatalf("SIMD matrix rows = %#v, want %#v", rows, expectedRows)
		}

		requiredRace := 0
		advisoryNoRace := 0
		for _, row := range rows {
			switch {
			case row.tier == "required" && row.advisory == "false" && row.race == "true":
				requiredRace++
			case row.tier == "advisory" && row.advisory == "true" && row.race == "false":
				advisoryNoRace++
			default:
				t.Errorf("runner %s has inconsistent tier/advisory/race policy: %#v", row.runner, row)
			}
		}
		if requiredRace != 2 || advisoryNoRace != 3 {
			t.Errorf("tier split = %d required/race and %d advisory/non-race, want 2 and 3", requiredRace, advisoryNoRace)
		}

		for _, anchor := range []string{
			"name: SIMD (${{ matrix.goos }}/${{ matrix.goarch }}, ${{ matrix.tier }})",
			"runs-on: ${{ matrix.runner }}",
			"continue-on-error: ${{ matrix.advisory }}",
			"fail-fast: false",
			"shell: bash",
			"id: simd-version",
			"{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}",
			"go run \"github.com/amikos-tech/pure-simdjson/cmd/pure-simdjson-bootstrap@${{ steps.simd-version.outputs.version }}\" fetch",
			"uses: actions/cache@v5",
			"path: ${{ runner.temp }}/pure-simdjson",
			"key: pure-simdjson-${{ runner.os }}-${{ runner.arch }}-${{ steps.simd-version.outputs.version }}",
			"AMI_GIN_SIMD_REQUIRED: \"1\"",
			"PURE_SIMDJSON_CACHE_DIR: ${{ runner.temp }}/pure-simdjson",
			"PURE_SIMDJSON_WARN_LEAKS: \"1\"",
			"if [[ \"${{ matrix.race }}\" == \"true\" ]]; then",
			"race_flags=(-race)",
			"go test -tags simdjson \"${race_flags[@]}\" -timeout=30m ./...",
		} {
			if !strings.Contains(simdJob, anchor) {
				t.Errorf("simd job is missing contract anchor %q", anchor)
			}
		}
		if strings.Contains(simdJob, "restore-keys:") {
			t.Error("simd cache must not use restore-keys")
		}
		if strings.Contains(simdJob, "continue-on-error: true") {
			t.Error("simd job must express advisory policy only at job level through matrix.advisory")
		}
		if got := strings.Count(simdJob, "AMI_GIN_SIMD_REQUIRED: \"1\""); got != 1 {
			t.Errorf("simd job required-flag declarations = %d, want one job-wide declaration covering all five rows", got)
		}
	})

	t.Run("explicit_path_and_docs", func(t *testing.T) {
		simdJob := ciJobSection(t, workflow, "simd")
		for _, anchor := range []string{
			"id: simd-library",
			"simd_library=\"${PURE_SIMDJSON_CACHE_DIR}/${{ steps.simd-version.outputs.version }}/${{ matrix.goos }}-${{ matrix.goarch }}/${{ matrix.library }}\"",
			"test -f \"$simd_library\"",
			"printf 'path=%s\\n' \"$simd_library\" >> \"$GITHUB_OUTPUT\"",
			"PURE_SIMDJSON_LIB_PATH: ${{ steps.simd-library.outputs.path }}",
			"go test -tags simdjson \"${race_flags[@]}\" -run '^(TestSIMDParserGoldenAuthoredFixtures|TestSIMDParserPhase20Parity)$' -timeout=30m .",
		} {
			if !strings.Contains(simdJob, anchor) {
				t.Errorf("simd job is missing explicit-path anchor %q", anchor)
			}
		}
		if got := strings.Count(simdJob, "go test -tags simdjson"); got != 2 {
			t.Errorf("simd tagged test runs = %d, want one full run and one explicit-path smoke", got)
		}
		if got := strings.Count(simdJob, "pure-simdjson-bootstrap@"); got != 1 {
			t.Errorf("simd upstream bootstrap invocations = %d, want exactly one", got)
		}

		explicitPathStep := workflowStepSection(t, simdJob, "Run explicit-path SIMD parity smoke")
		if !strings.Contains(explicitPathStep, "if: ${{ always() && steps.simd-library.outcome == 'success' }}") {
			t.Errorf("explicit-path smoke must run after a tagged-suite failure when native library resolution succeeded")
		}
		for _, forbidden := range []string{
			"PURE_SIMDJSON_BINARY_MIRROR",
			"PURE_SIMDJSON_DISABLE_GH_FALLBACK",
			"checksum",
			"pure-simdjson-bootstrap@",
			"curl ",
			"wget ",
		} {
			if strings.Contains(explicitPathStep, forbidden) {
				t.Errorf("explicit-path smoke contains forbidden second-path mechanism %q", forbidden)
			}
		}

		lintJob := ciJobSection(t, workflow, "lint")
		docsGuard := strings.Index(lintJob, "      - name: Check SIMD documentation contract\n        run: make check-simd-docs\n")
		golangCILint := strings.Index(lintJob, "      - name: Run golangci-lint\n")
		if docsGuard < 0 || golangCILint < 0 || docsGuard >= golangCILint {
			t.Errorf("lint job must run check-simd-docs before golangci-lint:\n%s", lintJob)
		}
	})

	t.Run("trend_artifact", func(t *testing.T) {
		for _, anchor := range []string{
			"  workflow_dispatch:\n",
			"# workflow_dispatch runs every CI job; the benchmark job has its own event guard.",
			"permissions:\n  contents: read\n",
		} {
			if !strings.Contains(workflow, anchor) {
				t.Errorf("workflow is missing trigger or least-privilege anchor %q", anchor)
			}
		}
		if strings.Contains(workflow, "  schedule:") {
			t.Error("workflow must not schedule shared-runner SIMD benchmarks")
		}

		trendJob := ciJobSection(t, workflow, "simd-benchmark-trend")
		for _, anchor := range []string{
			"if: github.event_name == 'workflow_dispatch' || (github.event_name == 'push' && github.ref == 'refs/heads/main')",
			"runs-on: ubuntu-latest",
			"continue-on-error: true",
			"{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}",
			"uses: actions/cache@v5",
			"path: ${{ runner.temp }}/pure-simdjson",
			"key: pure-simdjson-${{ runner.os }}-${{ runner.arch }}-${{ steps.simd-version.outputs.version }}",
			"go run \"github.com/amikos-tech/pure-simdjson/cmd/pure-simdjson-bootstrap@${{ steps.simd-version.outputs.version }}\" fetch",
			"env -u GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL -u GIN_PHASE20_SIMDJSON_DIR make bench-simd COUNT=1 BENCHTIME=1s > simd-benchmark.txt",
			"go run golang.org/x/perf/cmd/benchstat@v0.0.0-20260709024250-82a0b07e230d -col /parser simd-benchmark.txt > simd-benchstat.txt",
			"shared-runner-noisy, trend only; controlled committed Phase 22 evidence is authoritative",
			"uses: actions/upload-artifact@v7",
			"path: |\n            simd-benchmark.txt\n            simd-benchstat.txt",
			"if-no-files-found: error",
		} {
			if !strings.Contains(trendJob, anchor) {
				t.Errorf("SIMD trend job is missing contract anchor %q", anchor)
			}
		}
		for _, forbidden := range []string{
			"pull_request",
			"schedule",
			"threshold",
			"regression",
			".json",
			".tar",
			".zip",
		} {
			if strings.Contains(strings.ToLower(trendJob), forbidden) {
				t.Errorf("SIMD trend job contains forbidden gating or artifact token %q", forbidden)
			}
		}
	})

	t.Run("security_scan", func(t *testing.T) {
		makefile := string(readTestFile(t, filepath.Join(repositoryRoot(t), "Makefile")))
		securityTarget := "security-scan:\n\tgovulncheck ./...\n\tgovulncheck -tags simdjson ./...\n"
		if !strings.Contains(makefile, securityTarget) {
			t.Errorf("Makefile security-scan must cover default then tagged call graphs")
		}
		if got := strings.Count(makefile, "\tgovulncheck -tags simdjson ./..."); got != 1 {
			t.Errorf("tagged govulncheck invocations = %d, want exactly one", got)
		}
		if !strings.Contains(makefile, "security-scan - Run govulncheck against default and simdjson-tagged call graphs") {
			t.Error("Makefile help does not describe both security-scan call graphs")
		}
	})
}

func workflowStepSection(t *testing.T, job, stepName string) string {
	t.Helper()

	lines := strings.Split(job, "\n")
	header := "      - name: " + stepName
	start := -1
	for index, line := range lines {
		if line == header {
			start = index
			break
		}
	}
	if start < 0 {
		t.Fatalf("workflow job does not contain %q step", stepName)
	}

	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		if strings.HasPrefix(lines[index], "      - name: ") {
			end = index
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

func parseSIMDWorkflowMatrix(t *testing.T, simdJob string) []simdWorkflowMatrixRow {
	t.Helper()

	lines := strings.Split(simdJob, "\n")
	include := false
	var rows []simdWorkflowMatrixRow
	for _, line := range lines {
		if line == "        include:" {
			include = true
			continue
		}
		if !include {
			continue
		}
		if strings.HasPrefix(line, "      ") && !strings.HasPrefix(line, "          ") {
			break
		}
		if strings.HasPrefix(line, "          - runner: ") {
			rows = append(rows, simdWorkflowMatrixRow{runner: workflowScalar(strings.TrimPrefix(line, "          - runner: "))})
			continue
		}
		if len(rows) == 0 || !strings.HasPrefix(line, "            ") {
			continue
		}

		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		value = workflowScalar(value)
		row := &rows[len(rows)-1]
		switch key {
		case "goos":
			row.goos = value
		case "goarch":
			row.goarch = value
		case "tier":
			row.tier = value
		case "advisory":
			row.advisory = value
		case "race":
			row.race = value
		case "release":
			row.release = value
		case "library":
			row.library = value
		default:
			t.Fatalf("unexpected SIMD matrix key %q in %q", key, line)
		}
	}
	if len(rows) == 0 {
		t.Fatal("simd job does not contain matrix.include rows")
	}
	for index, row := range rows {
		if row.runner == "" || row.goos == "" || row.goarch == "" || row.tier == "" ||
			row.advisory == "" || row.race == "" || row.release == "" || row.library == "" {
			t.Fatalf("SIMD matrix row %d is incomplete: %s", index, formatSIMDWorkflowMatrixRow(row))
		}
	}
	return rows
}

func workflowScalar(value string) string {
	return strings.Trim(strings.TrimSpace(value), "\"'")
}

func formatSIMDWorkflowMatrixRow(row simdWorkflowMatrixRow) string {
	return fmt.Sprintf(
		"runner=%q goos=%q goarch=%q tier=%q advisory=%q race=%q release=%q library=%q",
		row.runner,
		row.goos,
		row.goarch,
		row.tier,
		row.advisory,
		row.race,
		row.release,
		row.library,
	)
}
