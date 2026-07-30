GOTESTSUM_VERSION ?= v1.13.0

.PHONY: build
build:
	go build ./...

# Intentionally stricter than Go: native importers keep the build tag on line 1 for easy auditing.
.PHONY: simd-isolation-check
simd-isolation-check:
	@set -eu; \
	importers=0; \
	for file in $$(git ls-files '*.go' ':(exclude).planning/**'); do \
		if grep -Fq '"github.com/amikos-tech/pure-simdjson"' "$$file"; then \
			importers=$$((importers + 1)); \
			if [ "$$(sed -n '1p' "$$file")" != '//go:build simdjson' ]; then \
				printf 'pure-simdjson importer lacks exact first-line build tag: %s\n' "$$file" >&2; \
				exit 1; \
			fi; \
		fi; \
	done; \
	if [ "$$importers" -eq 0 ]; then \
		printf '%s\n' 'no tagged pure-simdjson product importer found' >&2; \
		exit 1; \
	fi
	@set -eu; \
	deps="$$(go list -deps -test ./...)"; \
	if printf '%s\n' "$$deps" | grep -Fxq 'github.com/amikos-tech/pure-simdjson'; then \
		printf '%s\n' 'default Go dependency graph includes github.com/amikos-tech/pure-simdjson' >&2; \
		exit 1; \
	fi
	go build ./...
	go vet ./...

.PHONY: gotestsum-bin
gotestsum-bin:
	go install gotest.tools/gotestsum@$(GOTESTSUM_VERSION)

.PHONY: test
test: simd-isolation-check gotestsum-bin
	gotestsum \
		--format short-verbose \
		--packages="./... ./testdata/phase20" \
		--junitfile unit.xml \
		-- \
		-v \
		-coverprofile=coverage.out \
		-timeout=30m

.PHONY: integration-test
integration-test: test

.PHONY: check-validator-markers
check-validator-markers:
	@awk '\
	BEGIN { \
		expected["mergeStagedPaths"] = 1; expected["mergeNumericObservation"] = 1; expected["promoteNumericPathToFloat"] = 1; \
	} \
	function trim(s) { sub(/^[[:space:]]+/, "", s); sub(/[[:space:]]+$$/, "", s); return s } \
	function skip_spaces(s, i,    n) { n = length(s); while (i <= n && substr(s, i, 1) ~ /[[:space:]]/) i++; return i } \
	function skip_parens(s, i,    n, depth, c) { n = length(s); depth = 0; for (; i <= n; i++) { c = substr(s, i, 1); if (c == "(") depth++; else if (c == ")") { depth--; if (depth == 0) return i + 1 } } return n + 1 } \
	function function_name(line,    rest, parts) { rest = line; sub(/^[[:space:]]*func[[:space:]]+/, "", rest); rest = trim(rest); if (substr(rest, 1, 1) == "(") { rest = substr(rest, skip_parens(rest, 1)); rest = trim(rest) } split(rest, parts, /[[:space:](]/); return parts[1] } \
	function return_type(sig,    s, i, n, brace) { s = sig; gsub(/[[:space:]]+/, " ", s); brace = index(s, "{"); if (brace > 0) s = substr(s, 1, brace - 1); sub(/^[[:space:]]*func[[:space:]]+/, "", s); i = skip_spaces(s, 1); if (substr(s, i, 1) == "(") { i = skip_parens(s, i); i = skip_spaces(s, i) } n = length(s); while (i <= n && substr(s, i, 1) != "(") i++; if (i > n) return ""; i = skip_parens(s, i); return trim(substr(s, i)) } \
	function returns_error(sig,    ret) { ret = tolower(return_type(sig)); return ret ~ /(^|[^[:alnum:]_])error([^[:alnum:]_]|$$)/ } \
	function fail_direct(file, line, text) { print "MUST_BE_CHECKED_BY_VALIDATOR marker must directly precede function declaration: " file ":" line ": " text; bad = 1 } \
	function check_signature(sig, file, line) { if (returns_error(sig)) { print "MUST_BE_CHECKED_BY_VALIDATOR function returns error: " file ":" line ": " sig; bad = 1 } } \
	function record_name(name, file, line) { marked_count++; seen[name]++; if (!(name in expected)) { print "unexpected MUST_BE_CHECKED_BY_VALIDATOR marker for " name ": " file ":" line; bad = 1 } if (seen[name] > 1) { print "duplicate MUST_BE_CHECKED_BY_VALIDATOR marker for " name ": " file ":" line; bad = 1 } } \
	FNR == 1 && NR > 1 && marker { fail_direct(marker_file, marker_line, "<end of file>"); marker = 0 } \
	collecting { signature = signature " " $$0; if (index($$0, "{") > 0) { check_signature(signature, signature_file, signature_line); collecting = 0 } next } \
	marker { if ($$0 !~ /^[[:space:]]*func[[:space:]]/) { fail_direct(marker_file, marker_line, $$0); marker = 0; next } signature = $$0; signature_file = FILENAME; signature_line = FNR; signature_name = function_name($$0); record_name(signature_name, FILENAME, FNR); marker = 0; if (index($$0, "{") > 0) check_signature(signature, FILENAME, FNR); else collecting = 1; next } \
	$$0 ~ /^[[:space:]]*\/\/[[:space:]]*MUST_BE_CHECKED_BY_VALIDATOR[[:space:]]*$$/ { marker = 1; marker_file = FILENAME; marker_line = FNR; next } \
	END { \
		if (marker) fail_direct(marker_file, marker_line, "<end of file>"); \
		if (collecting) check_signature(signature, signature_file, signature_line); \
		if (!seen["mergeStagedPaths"]) { print "missing MUST_BE_CHECKED_BY_VALIDATOR marker for mergeStagedPaths"; bad = 1 } \
		if (!seen["mergeNumericObservation"]) { print "missing MUST_BE_CHECKED_BY_VALIDATOR marker for mergeNumericObservation"; bad = 1 } \
		if (!seen["promoteNumericPathToFloat"]) { print "missing MUST_BE_CHECKED_BY_VALIDATOR marker for promoteNumericPathToFloat"; bad = 1 } \
		if (marked_count != 3) { print "expected exactly 3 MUST_BE_CHECKED_BY_VALIDATOR markers, found " marked_count; bad = 1 } \
		exit bad \
	}' *.go

.PHONY: check-notice-version
check-notice-version:
	@set -eu; \
	expected_version="$$(go list -m -f '{{.Version}}' github.com/amikos-tech/pure-simdjson)"; \
	version_pattern='v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?(\+[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?'; \
	if ! printf '%s\n' "$$expected_version" | grep -Exq "$$version_pattern"; then \
		printf 'NOTICE.md alignment cannot validate expected module version %s: not a complete Go semantic-version token\n' "$$expected_version" >&2; \
		exit 1; \
	fi; \
	pinned_lines="$$(grep -E "pure-simdjson.*$$version_pattern" NOTICE.md || true)"; \
	pinned_line_count="$$(printf '%s\n' "$$pinned_lines" | sed '/^$$/d' | wc -l | tr -d '[:space:]')"; \
	if [ "$$pinned_line_count" -ne 4 ]; then \
		printf 'NOTICE.md pure-simdjson pins must contain exactly 4 version-bearing lines; expected module version %s, found %s lines:\n' "$$expected_version" "$$pinned_line_count" >&2; \
		printf '%s\n' "$$pinned_lines" >&2; \
		exit 1; \
	fi; \
	pinned_versions="$$(printf '%s\n' "$$pinned_lines" | grep -oE "$$version_pattern" || true)"; \
	pinned_version_count="$$(printf '%s\n' "$$pinned_versions" | sed '/^$$/d' | wc -l | tr -d '[:space:]')"; \
	if [ "$$pinned_version_count" -ne 5 ]; then \
		printf 'NOTICE.md pure-simdjson pins must contain exactly 5 complete version tokens; expected module version %s, found %s tokens:\n%s\n' "$$expected_version" "$$pinned_version_count" "$$pinned_lines" >&2; \
		exit 1; \
	fi; \
	offending_versions="$$(printf '%s\n' "$$pinned_versions" | grep -Fvx "$$expected_version" || true)"; \
	if [ -n "$$offending_versions" ]; then \
		printf 'NOTICE.md pure-simdjson version drift; expected module version %s, found offending pinned versions:\n' "$$expected_version" >&2; \
		printf '%s\n' "$$offending_versions" >&2; \
		printf 'Pinned NOTICE.md lines:\n%s\n' "$$pinned_lines" >&2; \
		exit 1; \
	fi; \
	dependency_line="$$(printf '[`github.com/amikos-tech/pure-simdjson` %s](https://github.com/amikos-tech/pure-simdjson/tree/%s).' "$$expected_version" "$$expected_version")"; \
	heading_line="## pure-simdjson $$expected_version"; \
	license_line="$$(printf '[`LICENSE`](https://github.com/amikos-tech/pure-simdjson/blob/%s/LICENSE)' "$$expected_version")"; \
	notice_line="$$(printf '[`NOTICE`](https://github.com/amikos-tech/pure-simdjson/blob/%s/NOTICE)' "$$expected_version")"; \
	for required_line in "$$dependency_line" "$$heading_line" "$$license_line" "$$notice_line"; do \
		required_count="$$(grep -Fxc "$$required_line" NOTICE.md || true)"; \
		if [ "$$required_count" -ne 1 ]; then \
			printf 'NOTICE.md pure-simdjson pin shape mismatch; expected module version %s and exactly one required line:\n%s\nPinned lines:\n%s\n' "$$expected_version" "$$required_line" "$$pinned_lines" >&2; \
			exit 1; \
		fi; \
	done

.PHONY: lint
lint: check-validator-markers check-notice-version
	golangci-lint run

.PHONY: lint-fix
lint-fix:
	golangci-lint run --fix ./...

.PHONY: security-scan
security-scan:
	govulncheck ./...

.PHONY: clean
clean:
	rm -f coverage.out unit.xml

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build     - Build all packages"
	@echo "  simd-isolation-check - Verify optional SIMD code stays out of default builds"
	@echo "  test      - Run tests with coverage"
	@echo "  integration-test - Run integration test suite"
	@echo "  check-notice-version - Verify NOTICE pure-simdjson pins align with go.mod"
	@echo "  lint      - Run validator marker checks and golangci-lint"
	@echo "  lint-fix  - Run golangci-lint with auto-fix"
	@echo "  security-scan - Run govulncheck against all packages"
	@echo "  clean     - Remove generated files"
	@echo "  help      - Show this help"
