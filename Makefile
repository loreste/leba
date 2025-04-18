.PHONY: build test clean lint vet fmt check release

# Get the git commit hash
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
# Get the current date in ISO-8601 format
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
# Get the Go version
GO_VERSION := $(shell go version | awk '{print $$3}')
# Get the OS/Arch
PLATFORM := $(shell go env GOOS)/$(shell go env GOARCH)

# Set build flags
LDFLAGS := -ldflags "-X 'leba/internal/version.GitCommit=$(GIT_COMMIT)' \
                      -X 'leba/internal/version.BuildDate=$(BUILD_DATE)' \
                      -X 'leba/internal/version.GoVersion=$(GO_VERSION)' \
                      -X 'leba/internal/version.Platform=$(PLATFORM)'"

# Default target
all: check build

# Build the binary
build:
	@echo "Building LEBA..."
	@go build $(LDFLAGS) -o leba ./cmd/server
	@echo "Done! Binary available at ./leba"

# Run tests
test:
	@echo "Running tests..."
	@go test -tags basic ./internal/...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f leba
	@rm -f coverage.out
	@echo "Done!"

# Run golangci-lint
lint:
	@echo "Running linter..."
	@golangci-lint run ./...

# Run go vet
vet:
	@echo "Running vet..."
	@go vet ./...

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Run all checks (fmt, vet, lint, test)
check: fmt vet lint test

# Create a release build
release: check
	@echo "Creating release build..."
	@go build $(LDFLAGS) -o leba ./cmd/server
	@echo "Release build created at ./leba"
	@echo "Version info:"
	@./leba --version

# Show help
help:
	@echo "Available targets:"
	@echo "  build    - Build the binary"
	@echo "  test     - Run tests"
	@echo "  clean    - Clean build artifacts"
	@echo "  lint     - Run linter"
	@echo "  vet      - Run go vet"
	@echo "  fmt      - Format code"
	@echo "  check    - Run all checks"
	@echo "  release  - Create a release build"
	@echo "  help     - Show this help"