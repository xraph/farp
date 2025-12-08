.PHONY: help format lint check fmt vet tidy install-tools

# Default target
.DEFAULT_GOAL := help

# Colors for output
COLOR_RESET := \033[0m
COLOR_INFO := \033[1;34m
COLOR_SUCCESS := \033[1;32m
COLOR_WARNING := \033[1;33m
COLOR_ERROR := \033[1;31m

# Check if Go is installed
GO := $(shell command -v go 2> /dev/null)
GOLANGCI_LINT := $(shell command -v golangci-lint 2> /dev/null)

# Check if this is a Go project
GO_FILES := $(shell find . -name '*.go' -not -path './farp-rust/*' -not -path './.git/*' 2> /dev/null | head -1)
IS_GO_PROJECT := $(if $(GO_FILES),yes,no)

help: ## Show this help message
	@echo "$(COLOR_INFO)Available targets:$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_INFO)%-15s$(COLOR_RESET) %s\n", $$1, $$2}'

check-go: ## Check if Go is installed and project is Go-based
	@if [ "$(IS_GO_PROJECT)" != "yes" ]; then \
		echo "$(COLOR_WARNING)No Go files found. Skipping Go-specific targets.$(COLOR_RESET)"; \
		exit 0; \
	fi
	@if [ -z "$(GO)" ]; then \
		echo "$(COLOR_ERROR)Error: Go is not installed. Please install Go to use formatting and linting targets.$(COLOR_RESET)"; \
		exit 1; \
	fi
	@echo "$(COLOR_SUCCESS)Go found: $(shell go version)$(COLOR_RESET)"

install-tools: check-go ## Install required Go tools (golangci-lint, goimports)
	@echo "$(COLOR_INFO)Installing Go tools...$(COLOR_RESET)"
	@if [ -z "$(GOLANGCI_LINT)" ]; then \
		echo "$(COLOR_INFO)Installing golangci-lint...$(COLOR_RESET)"; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	else \
		echo "$(COLOR_SUCCESS)golangci-lint already installed$(COLOR_RESET)"; \
	fi
	@if ! command -v goimports > /dev/null 2>&1; then \
		echo "$(COLOR_INFO)Installing goimports...$(COLOR_RESET)"; \
		go install golang.org/x/tools/cmd/goimports@latest; \
	else \
		echo "$(COLOR_SUCCESS)goimports already installed$(COLOR_RESET)"; \
	fi

fmt: check-go ## Format Go code using gofmt
	@echo "$(COLOR_INFO)Formatting Go code...$(COLOR_RESET)"
	@go fmt ./...
	@if command -v goimports > /dev/null 2>&1; then \
		echo "$(COLOR_INFO)Organizing imports with goimports...$(COLOR_RESET)"; \
		find . -name '*.go' -not -path './farp-rust/*' -not -path './.git/*' -not -path './vendor/*' -exec goimports -w {} +; \
	else \
		echo "$(COLOR_WARNING)goimports not found. Install with: make install-tools$(COLOR_RESET)"; \
	fi
	@echo "$(COLOR_SUCCESS)Formatting complete$(COLOR_RESET)"

format: fmt ## Alias for fmt

lint: check-go ## Lint Go code using golangci-lint
	@if [ -z "$(GOLANGCI_LINT)" ]; then \
		echo "$(COLOR_ERROR)Error: golangci-lint is not installed. Install with: make install-tools$(COLOR_RESET)"; \
		exit 1; \
	fi
	@echo "$(COLOR_INFO)Linting Go code...$(COLOR_RESET)"
	@golangci-lint run
	@echo "$(COLOR_SUCCESS)Linting complete$(COLOR_RESET)"

vet: check-go ## Run go vet
	@echo "$(COLOR_INFO)Running go vet...$(COLOR_RESET)"
	@go vet ./...
	@echo "$(COLOR_SUCCESS)Vet check complete$(COLOR_RESET)"

tidy: check-go ## Run go mod tidy
	@echo "$(COLOR_INFO)Running go mod tidy...$(COLOR_RESET)"
	@go mod tidy
	@echo "$(COLOR_SUCCESS)Module dependencies updated$(COLOR_RESET)"

check: fmt lint vet ## Format, lint, and vet (full check)
	@echo "$(COLOR_SUCCESS)All checks passed!$(COLOR_RESET)"

test: check-go ## Run Go tests
	@echo "$(COLOR_INFO)Running Go tests...$(COLOR_RESET)"
	@go test -v -race -coverprofile=coverage.out ./...
	@echo "$(COLOR_SUCCESS)Tests complete$(COLOR_RESET)"

