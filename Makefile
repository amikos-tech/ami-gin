GOTESTSUM_VERSION ?= v1.13.0

.PHONY: build
build:
	go build ./...

.PHONY: gotestsum-bin
gotestsum-bin:
	go install gotest.tools/gotestsum@$(GOTESTSUM_VERSION)

.PHONY: test
test: gotestsum-bin
	gotestsum \
		--format short-verbose \
		--packages="./..." \
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

.PHONY: lint
lint: check-validator-markers
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
	@echo "  test      - Run tests with coverage"
	@echo "  integration-test - Run integration test suite"
	@echo "  lint      - Run validator marker checks and golangci-lint"
	@echo "  lint-fix  - Run golangci-lint with auto-fix"
	@echo "  security-scan - Run govulncheck against all packages"
	@echo "  clean     - Remove generated files"
	@echo "  help      - Show this help"
