# IDAES Project Makefile
# Intelligent Document Analysis Engine System

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint

# Project parameters
BINARY_NAME=idaes
BINARY_UNIX=$(BINARY_NAME)_unix
MAIN_PATH=./cmd/main.go
BUILD_DIR=./build
COVERAGE_DIR=./coverage

# Version and build info
VERSION?=dev
BUILD_TIME=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Default target
.PHONY: all
all: clean deps test build

# Help target
.PHONY: help
help: ## Display this help screen
	@echo "IDAES Project Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build targets
.PHONY: build
build: ## Build the application
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PATH)

.PHONY: build-linux
build-linux: ## Build for Linux
	@echo "Building $(BINARY_NAME) for Linux..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_UNIX) $(MAIN_PATH)

.PHONY: build-release
build-release: ## Build optimized release version
	@echo "Building release version..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -a -installsuffix cgo -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)

# Development targets
.PHONY: run
run: ## Run the application
	$(GOBUILD) -o $(BINARY_NAME) $(MAIN_PATH) && ./$(BINARY_NAME)

.PHONY: run-dev
run-dev: ## Run the application in development mode
	$(GOCMD) run $(MAIN_PATH) -host=localhost -port=8080

# Testing targets
.PHONY: test
test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

.PHONY: test-short
test-short: ## Run short tests only
	$(GOTEST) -short -v ./...

.PHONY: test-race
test-race: ## Run tests with race detector
	$(GOTEST) -race -v ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	$(GOCMD) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "Coverage report generated: $(COVERAGE_DIR)/coverage.html"

.PHONY: benchmark
benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

# Code quality targets
.PHONY: fmt
fmt: ## Format Go code
	@echo "Formatting code..."
	$(GOFMT) -s -w .

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	$(GOCMD) vet ./...

.PHONY: lint
lint: ## Run golangci-lint (requires golangci-lint to be installed)
	@echo "Running golangci-lint..."
	@if command -v $(GOLINT) >/dev/null 2>&1; then \
		$(GOLINT) run ./...; \
	else \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

.PHONY: check
check: fmt vet lint test ## Run all code quality checks

# Dependency management
.PHONY: deps
deps: ## Download and tidy dependencies
	@echo "Managing dependencies..."
	$(GOGET) -d ./...
	$(GOMOD) tidy

.PHONY: deps-update
deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy

.PHONY: deps-vendor
deps-vendor: ## Vendor dependencies
	$(GOMOD) vendor

# Clean targets
.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)
	rm -rf $(BUILD_DIR)
	rm -rf $(COVERAGE_DIR)
	rm -rf vendor/

.PHONY: clean-cache
clean-cache: ## Clean Go module cache
	$(GOCMD) clean -modcache

# Installation targets
.PHONY: install
install: ## Install the application
	$(GOCMD) install $(LDFLAGS) $(MAIN_PATH)

# Docker targets (optional - uncomment if you add Docker support)
# .PHONY: docker-build
# docker-build: ## Build Docker image
# 	docker build -t $(BINARY_NAME):$(VERSION) .

# .PHONY: docker-run
# docker-run: ## Run Docker container
# 	docker run -p 8080:8080 $(BINARY_NAME):$(VERSION)

# Development utilities
.PHONY: watch
watch: ## Watch for changes and rebuild (requires entr or similar)
	@if command -v entr >/dev/null 2>&1; then \
		find . -name "*.go" | entr -r make run; \
	else \
		echo "entr not found. Install with your package manager for file watching."; \
	fi

.PHONY: tools
tools: ## Install development tools
	@echo "Installing development tools..."
	$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Project info
.PHONY: info
info: ## Show project information
	@echo "Project: IDAES (Intelligent Document Analysis Engine System)"
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Go Version: $$($(GOCMD) version)"
	@echo "Binary Name: $(BINARY_NAME)"
	@echo "Main Path: $(MAIN_PATH)"

# Quick development workflow
.PHONY: dev
dev: clean deps fmt vet test build ## Quick development workflow: clean, deps, format, vet, test, build

# Docker targets
.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t idaes:latest .

.PHONY: docker-up
docker-up: docker-build ## Start Docker services
	@echo "Starting Docker services..."
	docker compose up -d

.PHONY: docker-up-build
docker-up-build: ## Start Docker services with build (requires buildx 0.17+)
	@echo "Building and starting Docker services..."
	docker compose up --build -d

.PHONY: docker-down
docker-down: ## Stop Docker services
	@echo "Stopping Docker services..."
	docker compose down

.PHONY: docker-restart
docker-restart: docker-down docker-up ## Restart Docker services

.PHONY: docker-logs
docker-logs: ## View Docker logs
	docker compose logs -f

.PHONY: docker-clean
docker-clean: ## Clean Docker resources
	@echo "Cleaning Docker resources..."
	docker compose down -v
	docker system prune -f

.PHONY: docker-deploy
docker-deploy: ## Deploy with telemetry (full stack)
	@echo "Deploying IDAES with telemetry..."
	./scripts/deploy.sh

.PHONY: docker-status
docker-status: ## Show Docker service status
	@echo "Docker service status:"
	docker compose ps

.PHONY: docker-health
docker-health: ## Run comprehensive health check
	@./scripts/health-check.sh

.PHONY: docker-dev-up
docker-dev-up: ## Start Docker services in development mode (with build)
	@echo "Starting Docker services in development mode..."
	docker compose -f docker-compose.dev.yml up --build -d

.PHONY: docker-dev-down
docker-dev-down: ## Stop Docker services in development mode
	@echo "Stopping Docker services in development mode..."
	docker compose -f docker-compose.dev.yml down

.PHONY: docker-dev-logs
docker-dev-logs: ## View Docker logs in development mode
	docker compose -f docker-compose.dev.yml logs -f

.DEFAULT_GOAL := help