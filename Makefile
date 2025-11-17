.PHONY: build run clean test install help build-all build-linux build-darwin build-windows

# Build variables
BINARY_NAME=vpssetup
BUILD_DIR=bin
MAIN_PATH=./cmd
VERSION?=dev
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build for current OS
	@echo "Building $(BINARY_NAME) for current OS..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

build-linux: ## Build for Linux (amd64)
	@echo "Building $(BINARY_NAME) for Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64"

build-linux-arm: ## Build for Linux (arm64)
	@echo "Building $(BINARY_NAME) for Linux ARM..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64"

build-darwin: ## Build for macOS (amd64)
	@echo "Building $(BINARY_NAME) for macOS Intel..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64"

build-darwin-arm: ## Build for macOS (arm64/M1/M2)
	@echo "Building $(BINARY_NAME) for macOS Apple Silicon..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64"

build-windows: ## Build for Windows (amd64)
	@echo "Building $(BINARY_NAME) for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe"

build-all: build-linux build-linux-arm build-darwin build-darwin-arm build-windows ## Build for all platforms
	@echo "All builds complete!"
	@ls -lh $(BUILD_DIR)/

run: build ## Build and run the application
	@echo "Running $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME)

clean: ## Clean build files
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	@echo "Clean complete"

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v ./tests

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./tests
	$(GOCMD) tool cover -html=coverage.out

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

install: build ## Install the binary
	@echo "Installing $(BINARY_NAME)..."
	cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "Installation complete"

fmt: ## Format code
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

lint: ## Run linter
	@echo "Running linter..."
	golangci-lint run

dev: ## Run in development mode with hot reload
	@echo "Starting development mode..."
	air

.DEFAULT_GOAL := help
