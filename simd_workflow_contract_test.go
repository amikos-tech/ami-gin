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
