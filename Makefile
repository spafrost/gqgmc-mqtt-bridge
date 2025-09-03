# Makefile for Go HTTP service

# Variables
BINARY_NAME=gqgmc-mqtt-bridge
GO_FILES=$(wildcard *.go)

# Default target
.PHONY: build
build: $(BINARY_NAME)

# Build the binary for Linux (Docker)
$(BINARY_NAME): $(GO_FILES) go.mod go.sum
	set CGO_ENABLED=0&&set GOOS=linux&&go build -a -installsuffix cgo -o gqgmc-mqtt-bridge .

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY_NAME)

# Run the service locally
.PHONY: run
run:
	go run .

# Download and tidy dependencies
.PHONY: deps
deps:
	go mod download
	go mod tidy

# Build for current platform (useful for local testing)
.PHONY: build-local
build-local:
	go build -o $(BINARY_NAME) .

