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
	swagger-gen dev test \
	atlas-diff atlas-apply atlas-status \
	install-atlas install-swag install-air \
	check-swag check-air check-db-url check-test-db-url \
	docker-build docker-build-nocache docker-up docker-down docker-stop docker-logs docker-logs-api docker-logs-db docker-exec-api \
	 docker-db-reset \
	mocks clean-mocks

# Default target when running 'make'
.DEFAULT_GOAL := help

# --- Development Targets ---

dev: check-air swagger-gen ## Run the application with hot-reloading using Air (includes swagger generation)
	@echo "Starting application with hot-reloading (Air)..."
	@# Air will use the .air.toml configuration by default
	@$(AIR_CMD)

swagger-gen: check-swag ## Generate/Update Swagger documentation files in ./docs based on annotations
	@echo "Generating Swagger documentation..."
	@$(SWAG_CMD) init -g main.go # Specify main go file explicitly
	@echo "Swagger documentation generated in ./docs directory."

test: ## Run all Service Integration tests 
	@echo "Running Redis test instance..."
	@-docker run --name test-redis -d -p 6379:6379 redis:7-alpine
	@echo "Running Go tests..."
	@-go test -v -cover -coverpkg=./internal/services -coverprofile=coverage.out ./internal/services/...
	@echo "Generating HTML coverage report..."
	@-go tool cover -html=coverage.out -o coverage.html
	@echo "Shutdown Redis test instance..."
	@-docker ps --filter name=test-redis --filter status=running -aq | xargs docker stop
	@-docker ps --filter name=test-redis -aq | xargs docker container rm


# --- Atlas Commands ---

atlas-diff: 
	@atlas migrate diff automatic_migration \
	--dir "file://ent/migrate/migrations" \
	--to "ent://ent/schema" \
	--dev-url "$(TEST_DATABASE_URL)&search_path=public" 

atlas-apply:
	@atlas migrate apply \
	--dir "file://ent/migrate/migrations" \
	--url "$(DATABASE_URL)&search_path=public" 

atlas-status:
	@atlas migrate status \
	--dir "file://ent/migrate/migrations" \
	--url "$(DATABASE_URL)&search_path=public" 

# --- Tool Installation ---

install-atlas: ## Install the atlas CLI tool (requires Go)
	@echo "Installing atlas CLI..."
	@curl -sSf https://atlasgo.sh | sh
	@echo "atlas installed."

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
	@echo "Checking for atlas migrations on the api..."
	@make atlas-diff
	@echo "Running atlas migrations on the api..."
	@make atlas-apply
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

# --- Helper Check Targets ---

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

check-test-db-url: ## Check if TEST_DATABASE_URL is set
	@if [ -z "$(TEST_DATABASE_URL)" ]; then \
		echo "Error: TEST_DATABASE_URL environment variable is not set."; \
		echo "       Please define it in .env or your environment for integration tests."; \
		exit 1; \
	fi

# --- Go Generate ---

generate:
	@echo "Generating ent files..."
	@go generate ./...
	@echo "Code generated."


# --- Update Help Target ---
help: ## Display this help screen
	@echo "Usage: make <command>"
	@echo ""
	@echo "Available atlas commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep 'atlas-' | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
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
