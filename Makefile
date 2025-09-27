.PHONY: build test clean docker run dev lint benchmark install format tidy deps publish help

# Build variables
BINARY_NAME=sentinel
BUILD_DIR=bin
VERSION?=0.1.0
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Go build flags
GOFLAGS=-trimpath
CGO_ENABLED=0

# Default target
all: build

# Build the binary
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) go build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/sentinel

# Build ETL binary
build-etl:
	@echo "Building ETL pipeline..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) go build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/sentinel-etl ./cmd/etl

# Build for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/sentinel
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/sentinel
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/sentinel
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/sentinel

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

# Run tests with coverage report
test-coverage: test
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	go clean

# Build Docker image
docker:
	@echo "Building Docker image..."
	docker build -t llm-sentinel/core:$(VERSION) .
	docker tag llm-sentinel/core:$(VERSION) llm-sentinel/core:latest

# Run the application locally
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME) --config configs/default.yaml

# Run in development mode with live reload
dev:
	@echo "Running in development mode..."
	go run ./cmd/sentinel --config configs/default.yaml

# Run ETL pipeline with sample data
etl-sample:
	@echo "Running ETL pipeline with sample data..."
	go run ./cmd/etl --config configs/default.yaml --input data/sample_security_dataset.csv --batch-size 10

# Show database statistics
etl-stats:
	@echo "Showing database statistics..."
	go run ./cmd/etl --config configs/default.yaml --stats

# Install the binary to $GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	go install $(LDFLAGS) ./cmd/sentinel

# Run linter
lint:
	@echo "Running linter..."
	if ! command -v golangci-lint > /dev/null; then go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; fi
	$(shell go env GOPATH)/bin/golangci-lint run --fix

# Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	go mod tidy

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download

# Build and publish to Docker Hub
publish: docker
	@echo "Publishing to Docker Hub..."
	@read -p "Enter your Docker Hub username: " username; \
	docker tag $(IMAGE_NAME):$(VERSION) $$username/$(IMAGE_NAME):$(VERSION); \
	docker tag $(IMAGE_NAME):$(VERSION) $$username/$(IMAGE_NAME):latest; \
	echo "Tagged images:"; \
	docker images $$username/$(IMAGE_NAME); \
	echo "Pushing to Docker Hub..."; \
	docker push $$username/$(IMAGE_NAME):$(VERSION); \
	docker push $$username/$(IMAGE_NAME):latest; \
	echo "✅ Published successfully!"; \
	echo "Users can now run:"; \
	echo "  docker run -p 5052:8080 --name llm-sentinel $$username/$(IMAGE_NAME):latest"

# Show help
help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  build-all    - Build for multiple platforms"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  clean        - Clean build artifacts"
	@echo "  docker       - Build Docker image"
	@echo "  run          - Run the application locally"
	@echo "  dev          - Run in development mode"
	@echo "  install      - Install binary to GOPATH/bin"
	@echo "  lint         - Run linter"
	@echo "  benchmark    - Run benchmarks"
	@echo "  fmt          - Format code"
	@echo "  tidy         - Tidy dependencies"
	@echo "  deps         - Download dependencies"
	@echo "  publish      - Build and publish to Docker Hub"
	@echo "  help         - Show this help"
