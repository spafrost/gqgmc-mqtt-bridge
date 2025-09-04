# Makefile for Go HTTP service

# Variables
BINARY_NAME=gqgmc-mqtt-bridge
GO_FILES=$(wildcard *.go)

# Git information
GIT_BRANCH=$(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
VERSION=1.0.0

# Build flags
LDFLAGS=-ldflags "-X main.GitBranch=$(GIT_BRANCH) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME) -X main.Version=$(VERSION)"

# Default target
.PHONY: build
build: $(BINARY_NAME)

# Build the binary for Linux (Docker)
$(BINARY_NAME): $(GO_FILES) go.mod go.sum
	set CGO_ENABLED=0&&set GOOS=linux&&go build -a -installsuffix cgo $(LDFLAGS) -o gqgmc-mqtt-bridge .

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY_NAME)

# Run the service locally
.PHONY: run
run:
	go run $(LDFLAGS) .

# Download and tidy dependencies
.PHONY: deps
deps:
	go mod download
	go mod tidy

# Build for current platform (useful for local testing)
.PHONY: build-local
build-local:
	go build $(LDFLAGS) -o $(BINARY_NAME) .

# Show build information
.PHONY: info
info:
	@echo "Git Branch: $(GIT_BRANCH)"
	@echo "Git Commit: $(GIT_COMMIT)" 
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Version: $(VERSION)"

# Build with Docker (ensures consistent environment)
.PHONY: docker-build
docker-build:
	docker build --build-arg GIT_BRANCH="$(GIT_BRANCH)" --build-arg GIT_COMMIT="$(GIT_COMMIT)" --build-arg BUILD_TIME="$(BUILD_TIME)" --build-arg VERSION="$(VERSION)" -t $(BINARY_NAME):$(VERSION) .

