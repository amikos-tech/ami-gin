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

.PHONY: lint
lint:
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
	@echo "  lint      - Run golangci-lint"
	@echo "  lint-fix  - Run golangci-lint with auto-fix"
	@echo "  security-scan - Run govulncheck against all packages"
	@echo "  clean     - Remove generated files"
	@echo "  help      - Show this help"
