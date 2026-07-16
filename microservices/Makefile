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
SERVICES := api-gateway auth-service user-service wallet-service ledger-service transaction-service payment-service scheduler-service notification-service audit-service

# Services that own a database and have migration files in db/migrations/
DB_SERVICES := auth-service user-service wallet-service ledger-service transaction-service payment-service notification-service

# Mark the targets below as abstract commands (not physical files)
.PHONY: all build-linux build-native clean help tidy tidy-shared test migrate-up migrate-down migrate-status migrate-reset migrate-create proto

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

## test: Run unit tests across all packages in the Go Workspace
test:
	@echo "Running unit tests for all microservices in the workspace..."
	@go test ./shared/... ./api-gateway/... ./auth-service/... ./user-service/... ./wallet-service/... ./ledger-service/... ./transaction-service/... ./payment-service/... ./scheduler-service/... ./notification-service/... ./audit-service/...

## proto: Generate gRPC code from protobuf definitions
# How it works:
# Can be run to generate code for all services, or a single service:
#   make proto
#   make proto service=auth (or service=auth-service)
proto:
	@if [ -n "$(service)" ]; then \
		base=$$(echo "$(service)" | sed 's/-service$$//'); \
		echo "Generating protobuf files for $$base..."; \
		protoc --go_out=. --go_opt=module=github.com/bashocode/gowallet/microservices \
		       --go-grpc_out=. --go-grpc_opt=module=github.com/bashocode/gowallet/microservices \
		       proto/$$base/$$base.proto; \
	else \
		echo "Generating all protobuf files..."; \
		protoc --go_out=. --go_opt=module=github.com/bashocode/gowallet/microservices \
		       --go-grpc_out=. --go-grpc_opt=module=github.com/bashocode/gowallet/microservices \
		       $(shell find proto -name "*.proto"); \
	fi
	@echo "Protobuf files generated successfully!"

# ==============================================================================
# DATABASE MIGRATIONS (golang-migrate)
# ==============================================================================
# Uses the `migrate` CLI (github.com/golang-migrate/migrate) to run migrations
# for every service. Each service has its own database and db/migrations/ dir.
# golang-migrate tracks applied migrations in a `schema_migrations` table.
#
# Prerequisite: install the CLI → brew install golang-migrate

# MySQL connection details (read from .env, fallback to localhost for local CLI usage)
# Note: .env uses DB_HOST=mysql (Docker network name), but migrate CLI runs on host.
# Override with: make migrate-up MYSQL_HOST=mysql
DB_USER_VAR     := $(shell grep '^DB_USER=' .env 2>/dev/null | cut -d'=' -f2)
DB_PASSWORD_VAR := $(shell grep '^DB_PASSWORD=' .env 2>/dev/null | cut -d'=' -f2)
DB_PORT_VAR     := $(shell grep '^DB_PORT=' .env 2>/dev/null | cut -d'=' -f2)

MYSQL_USER     := $(or $(DB_USER_VAR),gowallet_user)
MYSQL_PASSWORD := $(or $(DB_PASSWORD_VAR),gowallet_password)
MYSQL_HOST     := localhost
MYSQL_PORT     := $(or $(DB_PORT_VAR),3306)

# Build the golang-migrate connection string for a given database name.
# Usage: $(call migrate_url,gowallet_auth)
migrate_url = mysql://$(MYSQL_USER):$(MYSQL_PASSWORD)@tcp($(MYSQL_HOST):$(MYSQL_PORT))/$(1)

# Map service name → database name (shell case statement inside each target)

## migrate-up: Apply all pending migrations for every service
migrate-up:
	@echo "Applying UP migrations for all services..."
	@for service in $(DB_SERVICES); do \
		case $$service in \
			auth-service)        db_name=gowallet_auth ;; \
			user-service)        db_name=gowallet_user ;; \
			wallet-service)      db_name=gowallet_wallet ;; \
			ledger-service)      db_name=gowallet_ledger ;; \
			transaction-service) db_name=gowallet_transaction ;; \
			payment-service)     db_name=gowallet_payment ;; \
			notification-service) db_name=gowallet_notification ;; \
			*) continue ;; \
		esac; \
		migration_dir="$$service/db/migrations"; \
		if [ ! -d "$$migration_dir" ]; then \
			echo "  $$service: no migrations directory, skipping"; \
			continue; \
		fi; \
		echo "  $$service → $$db_name"; \
		migrate -path $$migration_dir \
			-database "mysql://$(MYSQL_USER):$(MYSQL_PASSWORD)@tcp($(MYSQL_HOST):$(MYSQL_PORT))/$$db_name" \
			up 2>&1 || true; \
	done
	@echo "UP migrations completed!"

## migrate-down: Rollback all migrations for every service
# WARNING: This drops all tables. Use with caution.
migrate-down:
	@echo "Rolling back DOWN migrations for all services..."
	@for service in $(DB_SERVICES); do \
		case $$service in \
			auth-service)        db_name=gowallet_auth ;; \
			user-service)        db_name=gowallet_user ;; \
			wallet-service)      db_name=gowallet_wallet ;; \
			ledger-service)      db_name=gowallet_ledger ;; \
			transaction-service) db_name=gowallet_transaction ;; \
			payment-service)     db_name=gowallet_payment ;; \
			notification-service) db_name=gowallet_notification ;; \
			*) continue ;; \
		esac; \
		migration_dir="$$service/db/migrations"; \
		if [ ! -d "$$migration_dir" ]; then continue; fi; \
		echo "  $$service → $$db_name"; \
		echo "y" | migrate -path $$migration_dir \
			-database "mysql://$(MYSQL_USER):$(MYSQL_PASSWORD)@tcp($(MYSQL_HOST):$(MYSQL_PORT))/$$db_name" \
			down 2>&1 || true; \
	done
	@echo "DOWN migrations completed!"

## migrate-status: Show current migration version for every service
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
			notification-service) db_name=gowallet_notification ;; \
			*) continue ;; \
		esac; \
		migration_dir="$$service/db/migrations"; \
		if [ ! -d "$$migration_dir" ]; then \
			echo "  $$service: no migrations directory"; \
			continue; \
		fi; \
		echo "  $$service (database: $$db_name):"; \
		migrate -path $$migration_dir \
			-database "mysql://$(MYSQL_USER):$(MYSQL_PASSWORD)@tcp($(MYSQL_HOST):$(MYSQL_PORT))/$$db_name" \
			version 2>&1 || true; \
		echo ""; \
	done

## migrate-reset: Rollback all, then re-apply all (fresh start)
migrate-reset: migrate-down migrate-up
	@echo "Database reset complete (down + up)."

## migrate-create: Create a new migration file pair for a service
# Usage: make migrate-create service=transaction-service name=add_new_table
migrate-create:
	@if [ -z "$(service)" ] || [ -z "$(name)" ]; then \
		echo "Usage: make migrate-create service=transaction-service name=add_new_table"; \
		exit 1; \
	fi
	@if [ ! -d "$(service)/db/migrations" ]; then \
		echo "Error: $(service)/db/migrations does not exist"; \
		exit 1; \
	fi
	@echo "Creating migration '$(name)' for $(service)..."
	@migrate create -ext sql -dir $(service)/db/migrations -seq $(name)
	@echo "Created! Edit the .up.sql and .down.sql files."

## help: Display the usage guide for available make commands
help:
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Build:"
	@echo "  make build-linux   - Compile all services for Linux ($(GOARCH)) to run in Docker"
	@echo "  make build-native  - Compile all services natively for macOS ($(ARCH))"
	@echo "  make clean         - Remove all compiled binaries"
	@echo "  make test          - Run unit tests across all packages in the Go Workspace"
	@echo "  make proto [service=<svc>] - Generate gRPC Go code from protobuf definitions (all if omitted)"
	@echo ""
	@echo "Dependencies:"
	@echo "  make tidy          - Run go mod tidy on shared and all services"
	@echo "  make tidy-shared   - Run go mod tidy on the shared package only"
	@echo ""
	@echo "Migrations (golang-migrate):"
	@echo "  make migrate-up      - Apply all pending migrations for every service"
	@echo "  make migrate-down    - Rollback all migrations (drops tables)"
	@echo "  make migrate-status  - Show current migration version per service"
	@echo "  make migrate-reset   - Rollback then re-apply (fresh start)"
	@echo "  make migrate-create service=<svc> name=<name> - Create new migration"
	@echo ""
	@echo "  make help          - Show this help message"
