# Makefile for Go HTTP service

# Include repository configuration (create .makerc from .makerc.template)
-include .makerc

# Variables
BINARY_NAME=gqgmc-mqtt-bridge
GO_FILES=$(wildcard *.go)

# Git information (Windows compatible)
GIT_BRANCH=$(shell git rev-parse --abbrev-ref HEAD 2>nul || echo unknown)
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>nul || echo unknown)
BUILD_TIME=$(shell powershell -Command "[DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')")
VERSION=1.1.0

# Docker repository logic based on branch (using variables from .makerc)
ifeq ($(GIT_BRANCH),main)
    DOCKER_REPO=$(PROD_DOCKER_REPO)
else
    DOCKER_REPO=$(DEV_DOCKER_REPO)
endif

# Docker tag logic based on branch
ifeq ($(GIT_BRANCH),dev)
    DOCKER_TAG=dev
else
    DOCKER_TAG=$(VERSION)
endif

# Docker image name with repository
DOCKER_IMAGE=$(DOCKER_REPO)/$(BINARY_NAME)

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

# Setup development environment
.PHONY: setup
setup:
	@if not exist .makerc (echo Creating .makerc from template... && copy .makerc.template .makerc && echo Please edit .makerc with your Docker repository names) else (echo .makerc already exists)

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
	@echo "Docker Repo: $(DOCKER_REPO)"
	@echo "Docker Image: $(DOCKER_IMAGE)"
	@echo "Docker Tag: $(DOCKER_TAG)"

# Build with Docker (ensures consistent environment)
.PHONY: docker-build
docker-build:
	docker build --build-arg GIT_BRANCH="$(GIT_BRANCH)" --build-arg GIT_COMMIT="$(GIT_COMMIT)" --build-arg BUILD_TIME="$(BUILD_TIME)" --build-arg VERSION="$(VERSION)" -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Push to Docker repository
.PHONY: docker-push
docker-push: docker-build
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
ifeq ($(GIT_BRANCH),main)
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest
	docker push $(DOCKER_IMAGE):latest
endif

# Manually promote a specific version to latest (emergency use)
.PHONY: docker-promote-latest
docker-promote-latest:
	@echo "Promoting $(DOCKER_IMAGE):$(VERSION) to latest..."
	docker pull $(DOCKER_IMAGE):$(VERSION)
	docker tag $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest
	docker push $(DOCKER_IMAGE):latest

# Show Docker repository information
.PHONY: docker-info
docker-info:
	@echo "Docker Repository: $(DOCKER_REPO)"
	@echo "Docker Image: $(DOCKER_IMAGE)"
	@echo "Docker Tag: $(DOCKER_TAG)"
	@echo "Full Image Name: $(DOCKER_IMAGE):$(DOCKER_TAG)"

