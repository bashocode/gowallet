# ==============================================================================
# DETECT HOST ARCHITECTURE
# ==============================================================================
# Get the processor architecture of your Mac (e.g. arm64 for Apple Silicon M1/M2/M3, x86_64 for Intel)
ARCH := $(shell uname -m)

# Determine the target compilation architecture (GOARCH) suitable for Linux in Docker
ifeq ($(ARCH),x86_64)
    GOARCH := amd64
else ifeq ($(ARCH),arm64)
    GOARCH := arm64
else
    GOARCH := amd64
endif

# ==============================================================================
# LIST OF MICROSERVICES
# ==============================================================================
# Names of the microservice directories to be built automatically
SERVICES := api-gateway auth-service user-service wallet-service ledger-service transaction-service payment-service

# Mark the targets below as abstract commands (not physical files)
.PHONY: all build-linux build-native clean tidy tidy-shared help

# Default target if you run the `make` command without any arguments
all: help

# ==============================================================================
# COMMAND TARGETS
# ==============================================================================

## build-linux: Compile all services into Linux binaries (to run inside Docker)
# How it works:
# 1. Loops through each registered service.
# 2. Creates the <service-name>/bin folder if it doesn't exist.
# 3. Runs cross-compilation with CGO disabled (CGO_ENABLED=0), targeting Linux OS (GOOS=linux),
#    and using the detected architecture (GOARCH).
# 4. Uses the -ldflags="-w -s" flag to reduce the binary size.
build-linux:
	@echo "Building Go binaries for Linux ($(GOARCH))..."
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		mkdir -p $$service/bin; \
		CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) go build -ldflags="-w -s" -o ./$$service/bin/$$service-linux ./$$service/cmd/main.go; \
	done
	@echo "All Linux binaries built successfully!"

## build-native: Compile all services natively for your Mac (to run directly without Docker)
# How it works:
# 1. Loops through each service.
# 2. Compiles a native binary matching your current Mac OS directly.
build-native:
	@echo "Building Go binaries for native macOS ($(ARCH))...."
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		mkdir -p $$service/bin; \
		go build -o ./$$service/bin/$$service ./$$service/cmd/main.go; \
	done
	@echo "All native macOS binaries built successfully!"

## clean: Remove the 'bin' folder in each service to clean the workspace
clean:
	@echo "Cleaning up build binaries..."
	@for service in $(SERVICES); do \
		rm -rf $$service/bin; \
		echo "Cleaned $$service/bin"; \
	done
	@echo "Clean completed!"

## tidy-shared: Run go mod tidy only on the shared package
# Run this first if you add/remove a dependency in the shared package.
tidy-shared:
	@echo "Running go mod tidy on shared..."
	@cd shared && go mod tidy
	@echo "Done tidying shared!"

## tidy: Run go mod tidy on ALL services (shared first, then each service)
# Useful after adding new dependencies or cloning the repository.
tidy: tidy-shared
	@echo "Running go mod tidy on all services..."
	@for service in $(SERVICES); do \
		echo "Tidying $$service..."; \
		cd $$service && go mod tidy && cd ..; \
	done
	@echo "All services tidied successfully!"

## help: Display the usage guide for available make commands
help:
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  make build-linux   - Compile all services for Linux ($(GOARCH)) to run in Docker (RAM-efficient)"
	@echo "  make build-native  - Compile all services natively for macOS ($(ARCH)) to run locally"
	@echo "  make tidy          - Run go mod tidy on shared and all services"
	@echo "  make tidy-shared   - Run go mod tidy on the shared package only"
	@echo "  make clean         - Remove all compiled binaries"
	@echo "  make help          - Show this help message"

