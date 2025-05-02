# /home/bsant/testing/go-api-template/makefile.mk

# Makefile for managing database migrations and development tasks

# --- Load .env file ---
# Use -include to ignore error if .env is missing
-include .env
# Export the variables loaded from .env so they are available to shell commands run by make
export

# Variables
MIGRATIONS_DIR := internal/database/migrations

# --- Tool Check Variables ---
MIGRATE_CMD := $(shell command -v migrate 2> /dev/null)
SWAG_CMD := $(shell command -v swag 2> /dev/null)
AIR_CMD := $(shell command -v air 2> /dev/null)

# Phony targets (targets that don't represent files)
.PHONY: help \
	migrate-create migrate-up migrate-down migrate-down-all migrate-force migrate-status \
	swagger-gen dev test \
	install-migrate install-swag install-air \
	check-migrate check-swag check-air check-db-url \
	docker-build docker-build-nocache docker-up docker-down docker-stop docker-logs docker-logs-api docker-logs-db docker-exec-api \
	docker-migrate-up docker-migrate-down docker-migrate-status docker-migrate-force docker-db-reset \
	mocks clean-mocks

# Default target when running 'make'
.DEFAULT_GOAL := help

help: ## Display this help screen
	@echo "Usage: make <command>"
	@echo ""
	@echo "Available migration commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep 'migrate-' | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Available development commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep -E '(dev|swagger-gen)' | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Available tool commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep 'install-' | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'


# --- Development Targets ---

dev: check-air swagger-gen ## Run the application with hot-reloading using Air (includes swagger generation)
	@echo "Starting application with hot-reloading (Air)..."
	@# Air will use the .air.toml configuration by default
	@$(AIR_CMD)

swagger-gen: check-swag ## Generate/Update Swagger documentation files in ./docs based on annotations
	@echo "Generating Swagger documentation..."
	@$(SWAG_CMD) init -g main.go # Specify main go file explicitly
	@echo "Swagger documentation generated in ./docs directory."

test: ## Run all Go unit and integration tests
	@echo "Running Go tests..."
	@go test ./... # Runs tests in current dir and all subdirs recursively


# --- Migration Commands ---

migrate-create: check-migrate ## Create new SQL migration files. Usage: make migrate-create NAME=your_migration_name
	@if [ -z "$(NAME)" ]; then \
		echo "Error: NAME variable is not set."; \
		echo "Usage: make migrate-create NAME=your_migration_name"; \
		exit 1; \
	fi
	@echo "Creating migration files for '$(NAME)' in $(MIGRATIONS_DIR)..."
	@$(MIGRATE_CMD) create -ext sql -dir $(MIGRATIONS_DIR) -seq "$(NAME)"

migrate-up: check-migrate check-db-url ## Apply all pending 'up' migrations
	@echo "Applying migrations from $(MIGRATIONS_DIR)..."
	@$(MIGRATE_CMD) -database "$(DATABASE_URL)" -path $(MIGRATIONS_DIR) up
	@echo "Migrations applied."

migrate-down: check-migrate check-db-url ## Revert the last applied migration
	@echo "Reverting last migration from $(MIGRATIONS_DIR)..."
	@$(MIGRATE_CMD) -database "$(DATABASE_URL)" -path $(MIGRATIONS_DIR) down 1
	@echo "Last migration reverted."

migrate-down-all: check-migrate check-db-url ## Revert all migrations (use with caution!)
	@echo "Reverting ALL migrations from $(MIGRATIONS_DIR)..."
	@read -p "This will revert all migrations. Are you sure? (y/N): " confirm && [ "$$confirm" = "y" ] || exit 1
	@$(MIGRATE_CMD) -database "$(DATABASE_URL)" -path $(MIGRATIONS_DIR) down -all
	@echo "All migrations reverted."

migrate-force: check-migrate check-db-url ## Force migration version (use with extreme caution!). Usage: make migrate-force VERSION=<version_number>
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION variable is not set."; \
		echo "Usage: make migrate-force VERSION=<version_number>"; \
		exit 1; \
	fi
	@echo "Forcing migration version to $(VERSION) in $(MIGRATIONS_DIR)... (Use with caution!)"
	@read -p "Forcing versions can lead to schema inconsistencies. Are you absolutely sure? (y/N): " confirm && [ "$$confirm" = "y" ] || exit 1
	@$(MIGRATE_CMD) -database "$(DATABASE_URL)" -path $(MIGRATIONS_DIR) force $(VERSION)
	@echo "Migration version forced to $(VERSION)."

migrate-status: check-migrate check-db-url ## Show current migration status and version
	@echo "Checking migration status for $(MIGRATIONS_DIR)..."
	@echo "--- Version ---"
	@$(MIGRATE_CMD) -database "$(DATABASE_URL)" -path $(MIGRATIONS_DIR) version
	@echo "--- Status ---"
	@$(MIGRATE_CMD) -database "$(DATABASE_URL)" -path $(MIGRATIONS_DIR) status || true # Allow non-zero exit if dirty/no migrations table


# --- Tool Installation ---

install-migrate: ## Install the golang-migrate CLI tool (requires Go)
	@echo "Installing golang-migrate CLI (with postgres tag)..."
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo "migrate installed. Ensure $(go env GOPATH)/bin is in your PATH."

install-swag: ## Install the swag CLI tool (requires Go)
	@echo "Installing swag CLI..."
	@go install github.com/swaggo/swag/cmd/swag@latest
	@echo "swag installed. Ensure $(go env GOPATH)/bin is in your PATH."

install-air: ## Install the air CLI tool for hot-reloading (requires Go)
	@echo "Installing air CLI..."
	@go install github.com/air-verse/air@latest
	@echo "air installed. Ensure $(go env GOPATH)/bin is in your PATH."

# --- Docker Compose Targets ---

docker-build: ## Build or rebuild the docker images using docker-compose
	@echo "Building Docker images..."
	@docker-compose build

docker-build-nocache: ## Build or rebuild the docker images using docker-compose
	@echo "Building Docker images..."
	@docker-compose build --no-cache

docker-up: ## Start services in the background using docker-compose
	@echo "Starting Docker services (db, api)..."
	@docker-compose up -d --remove-orphans
	@echo "Running migrations on the api..."
	@make docker-migrate-up
	@echo "Services started. API should be available shortly."
	@make docker-logs # Show logs briefly after starting

docker-down: ## Stop and remove docker-compose services and volumes
	@echo "Stopping Docker services..."
	@docker-compose down -v # -v removes named volumes like postgres_data (use with caution if you need data)
	@echo "Services stopped and volumes removed."

docker-stop: ## Stop docker-compose services without removing them
	@echo "Stopping Docker services..."
	@docker-compose stop
	@echo "Services stopped."

docker-logs: ## Follow logs from docker-compose services
	@echo "Following logs (Ctrl+C to stop)..."
	@docker-compose logs -f

docker-logs-api: ## Follow logs from the api service only
	@echo "Following api logs (Ctrl+C to stop)..."
	@docker-compose logs -f api

docker-logs-db: ## Follow logs from the db service only
	@echo "Following db logs (Ctrl+C to stop)..."
	@docker-compose logs -f db

docker-exec-api: ## Execute a command inside the running api container. Usage: make docker-exec-api CMD="ls -l"
	@docker-compose exec api $(CMD)

docker-migrate-up: check-db-url ## Run database migrations inside the api container
	@echo "Running migrations up inside the api container..."
	@# Construct the internal URL using Make variables
	@INTERNAL_DB_URL="postgres://$(DB_USER):$(DB_PASSWORD)@db:5432/$(DB_NAME)?sslmode=disable"; \
	echo "Using internal URL for -database flag: $$INTERNAL_DB_URL"; \
	docker-compose exec api migrate \
		-database "$$INTERNAL_DB_URL" \
		-path ./internal/database/migrations \
		up
	@echo "Migrations applied."

docker-migrate-down: check-db-url ## Revert last database migration inside the api container
	@echo "Reverting last migration inside the api container..."
	@INTERNAL_DB_URL="postgres://$(DB_USER):$(DB_PASSWORD)@db:5432/$(DB_NAME)?sslmode=disable"; \
	echo "Using internal URL for -database flag: $$INTERNAL_DB_URL"; \
	docker-compose exec api migrate \
		-database "$$INTERNAL_DB_URL" \
		-path ./internal/database/migrations \
		down 1
	@echo "Last migration reverted."

docker-migrate-status: check-db-url ## Check migration status inside the api container
	@echo "Checking migration status inside the api container..."
	@INTERNAL_DB_URL="postgres://$(DB_USER):$(DB_PASSWORD)@db:5432/$(DB_NAME)?sslmode=disable"; \
	echo "Using internal URL for -database flag: $$INTERNAL_DB_URL"; \
	docker-compose exec api migrate \
		-database "$$INTERNAL_DB_URL" \
		-path ./internal/database/migrations \
		status || true

docker-migrate-force: check-db-url ## Force migration version inside the api container (use with extreme caution!). Usage: make docker-migrate-force VERSION=<version_number>
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION variable is not set."; \
		echo "Usage: make docker-migrate-force VERSION=<version_number>"; \
		exit 1; \
	fi
	@echo "Forcing migration version to $(VERSION) inside the api container... (Use with caution!)"
	@# Note: Confirmation prompt is difficult in non-interactive exec, removed for simplicity. Be careful!
	@INTERNAL_DB_URL="postgres://$(DB_USER):$(DB_PASSWORD)@db:5432/$(DB_NAME)?sslmode=disable"; \
	echo "Using internal URL for -database flag: $$INTERNAL_DB_URL"; \
	docker-compose exec api migrate \
		-database "$$INTERNAL_DB_URL" \
		-path ./internal/database/migrations \
		force $(VERSION)
	@echo "Migration version forced to $(VERSION)."

docker-db-reset: ## !! Drops and recreates the database in the Docker container (DATA LOSS!) !!
	@echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
	@echo "!! WARNING: This will DROP and RECREATE the database '$(DB_NAME)'."
	@echo "!!          ALL DATA in '$(DB_NAME)' WILL BE PERMANENTLY LOST."
	@echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
	@# --- Add Checks for required variables ---
	@if [ -z "$(DB_USER)" ]; then \
		echo "Error: DB_USER environment variable is not set. Cannot proceed."; \
		exit 1; \
	fi
	@if [ -z "$(DB_PASSWORD)" ]; then \
		echo "Error: DB_PASSWORD environment variable is not set. Cannot proceed."; \
		exit 1; \
	fi
	@if [ -z "$(DB_NAME)" ]; then \
		echo "Error: DB_NAME environment variable is not set. Cannot proceed."; \
		exit 1; \
	fi
	@# --- End Checks ---
	@read -p "Are you absolutely sure you want to continue? (Type 'yes' to proceed): " confirm && \
	if [ "$$confirm" != "yes" ]; then \
		echo "Aborted."; \
		exit 1; \
	fi
	@echo "Proceeding with database reset..."
	@echo "Dropping database '$(DB_NAME)'..."
	@# Execute DROP command with quoted variables
	@docker-compose exec -T -e PGPASSWORD="$(DB_PASSWORD)" db \
		psql -U "$(DB_USER)" -h "db" "postgres" -c "DROP DATABASE IF EXISTS $(DB_NAME);"
	@echo "Creating database '$(DB_NAME)'..."
	@# Execute CREATE command with quoted variables
	@docker-compose exec -T -e PGPASSWORD="$(DB_PASSWORD)" db \
		psql -U "$(DB_USER)" -h "db" "postgres" -c "CREATE DATABASE $(DB_NAME);"
	@# Optional: Grant privileges
	@if [ "$(DB_USER)" != "" ] && [ "$(DB_USER)" != "$(DB_USER)" ]; then \
		echo "Granting privileges on '$(DB_NAME)' to user '$(DB_USER)'..."; \
		docker-compose exec -T -e PGPASSWORD="$(DB_PASSWORD)" db \
			psql -U "$(DB_USER)" -h "db" "postgres" -c "GRANT ALL PRIVILEGES ON DATABASE $(DB_NAME) TO \"$(DB_USER)\";"; \
	fi
	@echo "Database '$(DB_NAME)' reset successfully."
	@echo "Run 'make docker-migrate-up' to apply migrations to the fresh database."

# --- Helper Check Targets ---

check-migrate:
	@if [ -z "$(MIGRATE_CMD)" ]; then \
		echo "Error: 'migrate' command not found in PATH."; \
		echo "Install it using 'make install-migrate' or see https://github.com/golang-migrate/migrate"; \
		exit 1; \
	fi

check-swag:
	@if [ -z "$(SWAG_CMD)" ]; then \
		echo "Error: 'swag' command not found in PATH."; \
		echo "Install it using 'make install-swag' or see https://github.com/swaggo/swag"; \
		exit 1; \
	fi

check-air:
	@if [ -z "$(AIR_CMD)" ]; then \
		echo "Error: 'air' command not found in PATH."; \
		echo "Install it using 'make install-air' or see https://github.com/cosmtrek/air"; \
		exit 1; \
	fi

check-db-url:
	@# Check variables needed to construct the URL are present
	@if [ -z "$(DB_USER)" ] || [ -z "$(DB_PASSWORD)" ] || [ -z "$(DB_NAME)" ]; then \
		echo "Error: DB_USER, DB_PASSWORD, or DB_NAME not set."; \
		echo "       Please define them in .env or your environment."; \
		exit 1; \
	fi
	@# Optional: Check host DATABASE_URL for local commands
	@if [ -z "$(DATABASE_URL)" ]; then \
		echo "Warning: Host DATABASE_URL not set (needed for local 'make migrate-*')."; \
	fi

# --- Mocks for testing ---

# Generate mocks for repositories and services
mocks: clean-mocks ## Generate mocks for repositories and services using mockgen
	@echo "Generating mocks..."
	@go install github.com/golang/mock/mockgen@latest # Ensure mockgen is installed
	@mkdir -p internal/mocks # Ensure the directory exists
	@echo "Generating repository mocks (storage)..."
	@mockgen -package=mocks -destination=internal/mocks/mock_storage.go go-api-template/internal/storage UserRepository,JobRepository,InvoiceRepository
	@echo "Generating service mocks..."
	@mockgen -package=mocks -destination=internal/mocks/mock_services.go go-api-template/internal/services UserService,JobService,InvoiceService
	@echo "Mocks generated."

# Clean generated mocks
clean-mocks: ## Remove generated mock files from internal/mocks
	@echo "Cleaning mocks..."
	@rm -f internal/mocks/mock_storage.go
	@rm -f internal/mocks/mock_services.go
	@echo "Mocks cleaned."

# --- Update Help Target ---
help: ## Display this help screen
	@echo "Usage: make <command>"
	@echo ""
	@echo "Available migration commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep 'migrate-' | grep -v 'docker-' | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' # Show local migrate commands
	@echo ""
	@echo "Available development commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep -E '(dev|swagger-gen|test)' | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Available Docker commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep 'docker-' | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'	@echo ""
	@echo "Available tool commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep 'install-' | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Available Testing/Mocks commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep -E '(mocks|clean-mocks)' | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
