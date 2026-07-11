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
SERVICES := api-gateway auth-service user-service wallet-service ledger-service transaction-service payment-service scheduler-service

# Services that own a database and have migration files in db/migrations/
DB_SERVICES := auth-service user-service wallet-service ledger-service transaction-service payment-service

# MySQL connection details (read from .env, fallback to defaults)
DB_USER_VAR     := $(shell grep '^DB_USER=' .env 2>/dev/null | cut -d'=' -f2)
DB_PASSWORD_VAR := $(shell grep '^DB_PASSWORD=' .env 2>/dev/null | cut -d'=' -f2)
DB_HOST_VAR     := $(shell grep '^DB_HOST=' .env 2>/dev/null | cut -d'=' -f2)
DB_PORT_VAR     := $(shell grep '^DB_PORT=' .env 2>/dev/null | cut -d'=' -f2)

MYSQL_USER     := $(or $(DB_USER_VAR),gowallet_user)
MYSQL_PASSWORD := $(or $(DB_PASSWORD_VAR),gowallet_password)
MYSQL_HOST     := $(or $(DB_HOST_VAR),localhost)
MYSQL_PORT     := $(or $(DB_PORT_VAR),3306)

# Container name for executing mysql CLI inside Docker
MYSQL_CONTAINER := gowallet-mysql-ms

# Mark the targets below as abstract commands (not physical files)
.PHONY: all build-linux build-native clean help tidy tidy-shared migrate-up migrate-down migrate-status migrate-reset

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

# ==============================================================================
# DATABASE MIGRATIONS
# ==============================================================================
# All migration commands run .up.sql / .down.sql files inside each service's
# db/migrations/ directory against the matching database. They try the Docker
# container first (gowallet-mysql-ms); if Docker is not running they fall back
# to the local mysql CLI.

## migrate-up: Run all pending .up.sql migrations for every service
# Loops through each DB-owned service, finds all *.up.sql files in
# db/migrations/, and executes them in alphabetical order against the
# service's database.
migrate-up:
	@echo "Running UP migrations for all services..."
	@for service in $(DB_SERVICES); do \
		case $$service in \
			auth-service)        db_name=gowallet_auth ;; \
			user-service)        db_name=gowallet_user ;; \
			wallet-service)      db_name=gowallet_wallet ;; \
			ledger-service)      db_name=gowallet_ledger ;; \
			transaction-service) db_name=gowallet_transaction ;; \
			payment-service)     db_name=gowallet_payment ;; \
			*) db_name="" ;; \
		esac; \
		migration_dir="$$service/db/migrations"; \
		if [ ! -d "$$migration_dir" ]; then \
			echo "  $$service: no migrations directory, skipping"; \
			continue; \
		fi; \
		files=$$(ls $$migration_dir/*.up.sql 2>/dev/null | sort); \
		if [ -z "$$files" ]; then \
			echo "  $$service: no .up.sql files, skipping"; \
			continue; \
		fi; \
		echo "  $$service → database: $$db_name"; \
		for f in $$files; do \
			echo "    ↑ $$(basename $$f)"; \
			if docker exec $(MYSQL_CONTAINER) mysql -u$(MYSQL_USER) -p$(MYSQL_PASSWORD) -e "SELECT 1" >/dev/null 2>&1; then \
				docker exec -i $(MYSQL_CONTAINER) mysql -u$(MYSQL_USER) -p$(MYSQL_PASSWORD) $$db_name < $$f 2>/dev/null; \
			else \
				mysql -u$(MYSQL_USER) -p$(MYSQL_PASSWORD) -h$(MYSQL_HOST) -P$(MYSQL_PORT) $$db_name < $$f 2>/dev/null; \
			fi; \
		done; \
	done
	@echo "All UP migrations completed!"

## migrate-down: Run all .down.sql migrations for every service (reverse order)
# WARNING: This drops tables. Use with caution in production.
migrate-down:
	@echo "Running DOWN migrations for all services (reverse order)..."
	@for service in $(DB_SERVICES); do \
		case $$service in \
			auth-service)        db_name=gowallet_auth ;; \
			user-service)        db_name=gowallet_user ;; \
			wallet-service)      db_name=gowallet_wallet ;; \
			ledger-service)      db_name=gowallet_ledger ;; \
			transaction-service) db_name=gowallet_transaction ;; \
			payment-service)     db_name=gowallet_payment ;; \
			*) db_name="" ;; \
		esac; \
		migration_dir="$$service/db/migrations"; \
		if [ ! -d "$$migration_dir" ]; then \
			echo "  $$service: no migrations directory, skipping"; \
			continue; \
		fi; \
		files=$$(ls -r $$migration_dir/*.down.sql 2>/dev/null); \
		if [ -z "$$files" ]; then \
			echo "  $$service: no .down.sql files, skipping"; \
			continue; \
		fi; \
		echo "  $$service → database: $$db_name"; \
		for f in $$files; do \
			echo "    ↓ $$(basename $$f)"; \
			if docker exec $(MYSQL_CONTAINER) mysql -u$(MYSQL_USER) -p$(MYSQL_PASSWORD) -e "SELECT 1" >/dev/null 2>&1; then \
				docker exec -i $(MYSQL_CONTAINER) mysql -u$(MYSQL_USER) -p$(MYSQL_PASSWORD) $$db_name < $$f 2>/dev/null; \
			else \
				mysql -u$(MYSQL_USER) -p$(MYSQL_PASSWORD) -h$(MYSQL_HOST) -P$(MYSQL_PORT) $$db_name < $$f 2>/dev/null; \
			fi; \
		done; \
	done
	@echo "All DOWN migrations completed!"

## migrate-status: Show migration files for every service
migrate-status:
	@echo "Migration status for all services:"
	@echo ""
	@for service in $(DB_SERVICES); do \
		case $$service in \
			auth-service)        db_name=gowallet_auth ;; \
			user-service)        db_name=gowallet_user ;; \
			wallet-service)      db_name=gowallet_wallet ;; \
			ledger-service)      db_name=gowallet_ledger ;; \
			transaction-service) db_name=gowallet_transaction ;; \
			payment-service)     db_name=gowallet_payment ;; \
			*) db_name="" ;; \
		esac; \
		migration_dir="$$service/db/migrations"; \
		if [ ! -d "$$migration_dir" ]; then \
			echo "  $$service: no migrations directory"; \
			continue; \
		fi; \
		echo "  $$service (database: $$db_name):"; \
		files=$$(ls $$migration_dir/*.up.sql 2>/dev/null | sort); \
		if [ -z "$$files" ]; then \
			echo "    (no migration files)"; \
		else \
			for f in $$files; do \
				echo "    $$(basename $$f)"; \
			done; \
		fi; \
		echo ""; \
	done

## migrate-reset: Run all down migrations, then all up migrations (fresh start)
migrate-reset: migrate-down migrate-up
	@echo "Database reset complete (down + up)."

## help: Display the usage guide for available make commands
help:
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Build:"
	@echo "  make build-linux   - Compile all services for Linux ($(GOARCH)) to run in Docker"
	@echo "  make build-native  - Compile all services natively for macOS ($(ARCH))"
	@echo "  make clean         - Remove all compiled binaries"
	@echo ""
	@echo "Dependencies:"
	@echo "  make tidy          - Run go mod tidy on shared and all services"
	@echo "  make tidy-shared   - Run go mod tidy on the shared package only"
	@echo ""
	@echo "Migrations:"
	@echo "  make migrate-up     - Run all .up.sql migrations for every service"
	@echo "  make migrate-down   - Run all .down.sql migrations (reverse order, drops tables)"
	@echo "  make migrate-status - List all migration files per service"
	@echo "  make migrate-reset  - Run down then up (fresh start)"
	@echo ""
	@echo "  make help          - Show this help message"

