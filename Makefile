.PHONY: help build build-release sign build-signed release-sign test test-ci test-unit \
	test-coverage bench clean install install-local uninstall uninstall-local release cross-compile \
	check ci lint lint-ci lint-fix fmt vet dev dev-check run version \
	deps check-version

# Variables
BINARY_NAME := operator
ALIAS_NAME := opr
VERSION := $(shell grep 'Version.*=' internal/constants/constants.go | sed 's/.*"\(.*\)".*/\1/')
VERSION_FILE := $(shell cat VERSION)
BUILD_DIR := bin
BINARY_PATH := $(BUILD_DIR)/$(BINARY_NAME)
DIST_DIR := dist

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := $(GOCMD) fmt
GOVET := $(GOCMD) vet
GOLANGCI_LINT := golangci-lint
GOLANGCI_LINT_VERSION ?= 2.13.2
LINT_TIMEOUT ?= 5m

# Build flags
LDFLAGS := -ldflags "-s -w"
SIGN_IDENTITY ?= -

# Default target
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo "Operator CLI - Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-22s %s\n", $$1, $$2}'

check-version: ## Ensure VERSION file matches constants.Version
	@const_ver="$(VERSION)"; file_ver="$(VERSION_FILE)"; \
	if [ "$$const_ver" != "$$file_ver" ]; then \
		echo "Version mismatch: internal/constants/constants.go=$$const_ver VERSION=$$file_ver"; \
		exit 1; \
	fi

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_PATH) .
	@echo "✓ Built: $(BINARY_PATH)"

build-release: ## Build optimized release binary for current platform
	@echo "Building release binary..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -trimpath -o $(BINARY_PATH) .
	@echo "✓ Release binary built: $(BINARY_PATH)"

sign: build ## Ad-hoc sign binary on macOS
	@if [ "$$(uname)" = "Darwin" ]; then \
		codesign --force -s "$(SIGN_IDENTITY)" $(BINARY_PATH); \
		xattr -d com.apple.quarantine $(BINARY_PATH) 2>/dev/null || true; \
		echo "✓ Signed: $(BINARY_PATH)"; \
	else \
		echo "ℹ️  Skipping sign (non-macOS)"; \
	fi

build-signed: build sign ## Build, sign, and install to ~/.local/bin (local debug)
	@mkdir -p ~/.local/bin
	@rm -f ~/.local/bin/$(BINARY_NAME)
	@cp $(BINARY_PATH) ~/.local/bin/$(BINARY_NAME)
	@ln -sf ~/.local/bin/$(BINARY_NAME) ~/.local/bin/$(ALIAS_NAME)
	@if [ "$$(uname)" = "Darwin" ]; then \
		codesign --force -s "$(SIGN_IDENTITY)" ~/.local/bin/$(BINARY_NAME); \
		xattr -d com.apple.quarantine ~/.local/bin/$(BINARY_NAME) 2>/dev/null || true; \
	fi
	@echo "✓ Installed: ~/.local/bin/$(BINARY_NAME)"
	@echo "✓ Symlink: ~/.local/bin/$(ALIAS_NAME)"

release-sign: ## Sign macOS release binaries in dist/ (optional; set RELEASE_SIGN=1)
	@if [ "$${RELEASE_SIGN:-0}" != "1" ]; then \
		echo "ℹ️  RELEASE_SIGN!=1; skipping release signing"; \
	elif [ "$$(uname)" != "Darwin" ]; then \
		echo "ℹ️  Release signing requires a macOS runner; skipping"; \
	else \
		for bin in "$(DIST_DIR)/$(BINARY_NAME)-darwin-amd64" "$(DIST_DIR)/$(BINARY_NAME)-darwin-arm64"; do \
			if [ -f "$$bin" ]; then \
				codesign -s "$(SIGN_IDENTITY)" -f "$$bin"; \
				echo "✓ Signed $$bin"; \
			else \
				echo "ℹ️  Missing $$bin (skip)"; \
			fi; \
		done; \
	fi

test: ## Run tests (deps + verify + go test)
	@echo "Downloading dependencies..."
	$(GOMOD) download
	@echo "Verifying dependencies..."
	$(GOMOD) verify
	@echo "Running tests..."
	@if [ "$$($(GOCMD) env CGO_ENABLED)" = "1" ]; then \
		$(GOTEST) -v -race ./...; \
	else \
		echo "CGO disabled; running tests without -race"; \
		$(GOTEST) -v ./...; \
	fi
	@echo "✓ Tests passed"

test-unit: ## Run Go unit tests
	@echo "Running unit tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "✓ Unit tests passed"

test-ci: ## Run CI tests
	@$(MAKE) --no-print-directory test

test-coverage: ## Run tests with coverage report
	@echo "Generating coverage report..."
	$(GOTEST) -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report generated: coverage.html"

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_PATH)
	rm -rf $(BUILD_DIR) $(DIST_DIR)
	rm -f coverage.out coverage.html
	@echo "✓ Cleaned"

install: build ## Install binary and opr alias to /usr/local/bin
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@sudo cp $(BINARY_PATH) /usr/local/bin/$(BINARY_NAME)
	@sudo ln -sf /usr/local/bin/$(BINARY_NAME) /usr/local/bin/$(ALIAS_NAME)
	@echo "✓ Installed: /usr/local/bin/$(BINARY_NAME)"
	@echo "✓ Alias: /usr/local/bin/$(ALIAS_NAME)"
	@echo ""
	@echo "Verify with: operator version && opr version"

uninstall: ## Uninstall binary from /usr/local/bin
	@echo "Uninstalling $(BINARY_NAME)..."
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@sudo rm -f /usr/local/bin/$(ALIAS_NAME)
	@echo "✓ Uninstalled"

install-local: build-signed ## Install to ~/.local/bin with alias

uninstall-local: ## Uninstall from ~/.local/bin
	@echo "Uninstalling $(BINARY_NAME) from ~/.local/bin..."
	@rm -f ~/.local/bin/$(BINARY_NAME)
	@rm -f ~/.local/bin/$(ALIAS_NAME)
	@echo "✓ Uninstalled from ~/.local/bin"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "✓ Dependencies updated"

fmt: ## Format Go code
	@echo "Formatting code..."
	$(GOFMT) ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	else \
		echo "goimports not found; skipping import formatting"; \
	fi
	@echo "✓ Code formatted"

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...
	@echo "✓ Vet passed"

lint: ## Run linters
	@echo "Running golangci-lint..."
	@command -v $(GOLANGCI_LINT) >/dev/null 2>&1 || (echo "$(GOLANGCI_LINT) not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	@actual_version="$$($(GOLANGCI_LINT) version | awk '{print $$4}' | sed 's/^v//')"; \
	echo "Using golangci-lint $$actual_version (CI pins $(GOLANGCI_LINT_VERSION))"
	$(GOLANGCI_LINT) run --timeout=$(LINT_TIMEOUT)
	@echo "✓ Linting complete"

lint-ci: ## Run linter in CI parity mode (clears cache first)
	@echo "Running CI-parity linter..."
	$(GOLANGCI_LINT) cache clean
	@$(MAKE) --no-print-directory lint

lint-fix: ## Run linter with auto-fix
	@echo "Running linter with auto-fix..."
	@command -v $(GOLANGCI_LINT) >/dev/null 2>&1 || (echo "$(GOLANGCI_LINT) not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	@actual_version="$$($(GOLANGCI_LINT) version | awk '{print $$4}' | sed 's/^v//')"; \
	echo "Using golangci-lint $$actual_version (CI pins $(GOLANGCI_LINT_VERSION))"
	$(GOLANGCI_LINT) run --timeout=$(LINT_TIMEOUT) --fix

check: ## Run full checks (fmt, vet, lint, test, build)
	@echo "==> make fmt"
	@$(MAKE) --no-print-directory fmt
	@echo "==> make vet"
	@$(MAKE) --no-print-directory vet
	@echo "==> make lint"
	@$(MAKE) --no-print-directory lint
	@echo "==> make test"
	@$(MAKE) --no-print-directory test
	@echo "==> make build"
	@$(MAKE) --no-print-directory build
	@echo "✓ Full checks complete"

cross-compile: ## Build for all platforms
	@echo "Cross-compiling for all platforms..."
	@mkdir -p $(DIST_DIR)

	@echo "Building for macOS (Apple Silicon)..."
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 .

	@echo "Building for macOS (Intel)..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 .

	@echo "Building for Linux (amd64)..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 .

	@echo "Building for Linux (arm64)..."
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 .

	@echo "✓ Cross-compilation complete"
	@ls -lh $(DIST_DIR)/

release: check-version clean test cross-compile ## Create release build
	@echo "Creating release archives..."
	@mkdir -p $(DIST_DIR)

	@cd $(DIST_DIR) && \
		tar -czf $(BINARY_NAME)-darwin-arm64-$(VERSION).tar.gz $(BINARY_NAME)-darwin-arm64 && \
		tar -czf $(BINARY_NAME)-darwin-amd64-$(VERSION).tar.gz $(BINARY_NAME)-darwin-amd64 && \
		tar -czf $(BINARY_NAME)-linux-amd64-$(VERSION).tar.gz $(BINARY_NAME)-linux-amd64 && \
		tar -czf $(BINARY_NAME)-linux-arm64-$(VERSION).tar.gz $(BINARY_NAME)-linux-arm64

	@cd $(DIST_DIR) && \
		shasum -a 256 *.tar.gz > checksums.txt

	@echo "✓ Release $(VERSION) ready in $(DIST_DIR)/"
	@echo ""
	@echo "Release files:"
	@ls -lh $(DIST_DIR)/*.tar.gz
	@echo ""
	@cat $(DIST_DIR)/checksums.txt

version: ## Show version
	@echo "$(BINARY_NAME) version $(VERSION)"

run: build ## Build and run
	./$(BINARY_PATH)

dev: ## Development mode - build and doctor
	@$(MAKE) build
	@./$(BINARY_PATH) doctor
	@echo "✓ Development environment ready"

dev-check: fmt vet test-unit ## Quick development check (fmt, vet, test-unit)
	@echo "✓ Development checks passed"

ci: check ## Run CI checks (full pipeline)
