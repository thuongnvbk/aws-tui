.PHONY: build run clean install test lint deps all build-all run-profile test-coverage fmt setup-config help

BINARY_NAME=aws-tui
VERSION=0.1.0
BUILD_DIR=./build
GOCACHE:=$(abspath $(BUILD_DIR)/.gocache)
GOTMPDIR:=$(abspath $(BUILD_DIR)/.gotmp)
ENV_VARS=GOCACHE=$(GOCACHE) GOTMPDIR=$(GOTMPDIR)

# Detect main package location
ifneq ($(wildcard ./cmd/aws-tui/main.go),)
    MAIN_PATH=./cmd/aws-tui
else ifneq ($(wildcard ./main.go),)
    MAIN_PATH=.
else
    MAIN_PATH=./cmd/aws-tui
endif

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Default target
all: deps build

# Install dependencies
deps:
	go mod tidy
	go mod download

# Build the application
build:
	@mkdir -p $(BUILD_DIR)
	@mkdir -p $(GOCACHE) $(GOTMPDIR)
	@echo "Building from $(MAIN_PATH)..."
	$(ENV_VARS) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)"

# Build for multiple platforms
build-all: deps
	@mkdir -p $(BUILD_DIR)
	@mkdir -p $(GOCACHE) $(GOTMPDIR)
	$(ENV_VARS) GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	$(ENV_VARS) GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	$(ENV_VARS) GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	$(ENV_VARS) GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PATH)
	$(ENV_VARS) GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	@echo "Built binaries for all platforms in $(BUILD_DIR)/"

# Run the application
run: build
	$(BUILD_DIR)/$(BINARY_NAME)

# Run with specific profile
run-profile: build
	$(BUILD_DIR)/$(BINARY_NAME) --profile $(PROFILE)

# Install to GOPATH/bin
install:
	@mkdir -p $(GOCACHE) $(GOTMPDIR)
	$(ENV_VARS) go install $(LDFLAGS) $(MAIN_PATH)

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	$(ENV_VARS) go clean

# Run tests
test:
	@mkdir -p $(GOCACHE) $(GOTMPDIR)
	$(ENV_VARS) go test -v ./...

# Run tests with coverage
test-coverage:
	@mkdir -p $(GOCACHE) $(GOTMPDIR)
	$(ENV_VARS) go test -v -coverprofile=coverage.out ./...
	$(ENV_VARS) go tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	@mkdir -p $(GOCACHE) $(GOTMPDIR)
	$(ENV_VARS) golangci-lint run

# Format code
fmt:
	$(ENV_VARS) gofmt -w cmd/aws-tui/main.go internal/aws/*.go internal/ui/*.go

# Create config directory and example config
setup-config:
	@mkdir -p ~/.aws-tui
	@if [ ! -f ~/.aws-tui/config.yaml ]; then \
		cp config.example.yaml ~/.aws-tui/config.yaml; \
		echo "Created ~/.aws-tui/config.yaml"; \
	else \
		echo "Config already exists at ~/.aws-tui/config.yaml"; \
	fi

# Show help
help:
	@echo "AWS TUI - Terminal User Interface for AWS"
	@echo ""
	@echo "Detected MAIN_PATH: $(MAIN_PATH)"
	@echo ""
	@echo "Usage:"
	@echo "  make deps          Install dependencies"
	@echo "  make build         Build the application"
	@echo "  make build-all     Build for all platforms"
	@echo "  make run           Build and run"
	@echo "  make install       Install to GOPATH/bin"
	@echo "  make clean         Clean build artifacts"
	@echo "  make test          Run tests"
	@echo "  make lint          Run linter"
	@echo "  make fmt           Format code"
	@echo "  make setup-config  Create config directory"
	@echo ""
	@echo "Run with specific profile:"
	@echo "  make run-profile PROFILE=production"
