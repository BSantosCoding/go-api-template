# Go API Template

A template project for building RESTful APIs in Go using Gin, PostgreSQL (with pgx), Docker, and common development tools.

The goal of this repository is for me to experiment and learn, use new packages, but keep it readable and scalable. A barebones example of how I want to build my APIs.

## Features

- **Framework:** Gin Web Framework
- **Database:** PostgreSQL
- **Driver:** pgx (High-performance PostgreSQL driver)
- **Containerization:** Docker & Docker Compose setup for API and Database
- **Hot Reloading:** Air for live code reloading during development
- **Migrations:** golang-migrate for database schema migrations (managed via Makefile)
- **Configuration:** Viper for loading config from file (`config.yaml`) and environment variables (`.env` support)
- **API Documentation:** Swagger UI integration using Swag annotations
- **CORS:** Configurable Cross-Origin Resource Sharing middleware
- **Task Automation:** Makefile for common development tasks (running, building, migrations, etc.)
- **Code Structure:** Basic project layout following standard Go practices (`internal`, `pkg`, `config`, etc.)
- **Data Access:** Repository pattern example

## Prerequisites

Before you begin, ensure you have the following installed on your host machine:

- **Go:** Version 1.23 or higher (as specified in `go.mod`)
- **Docker:** Latest stable version
- **Docker Compose:** Latest stable version (v2+ recommended)
- **Make:** Standard build automation tool
- **Git:** For cloning the repository

The following Go tools are used, but the Makefile can install them for you:

- `golang-migrate` CLI
- `swag` CLI
- `air` CLI

## Getting Started

1.  **Clone the Repository:**

    ```bash
    git clone <your-repository-url>
    cd go-api-template
    ```

2.  **Create Environment File:**
    Copy the example environment file or create your own `.env` file in the project root. This file is used by Docker Compose and the Makefile (for local commands).

    ```bash
    # Example: Create .env with defaults
    cat << EOF > .env
    # Database credentials and connection details
    DB_USER=postgres
    DB_PASSWORD=postgres
    DB_NAME=api_db

    # Host port mapping for the database (optional, defaults to 5432)
    # DB_PORT_HOST=5432

    # Host port mapping for the API server (optional, defaults to 8080)
    # SERVER_PORT_HOST=8080

    # Database URL for local 'make migrate-*' commands (points to host port)
    DATABASE_URL=postgres://\${DB_USER}:\${DB_PASSWORD}@localhost:\${DB_PORT_HOST:-5432}/\${DB_NAME}?sslmode=disable

    # CORS Allowed Origins (comma-separated) - OVERRIDE FOR PRODUCTION!
    CORS_ALLOWED_ORIGINS=http://localhost:3000,http://127.0.0.1:3000

    # Blockchain listener configuration
    BLOCKCHAIN_RPC_URL="wss://ethereum-sepolia-rpc.publicnode.com"
    CONTRACT_ADDRESS="0x694AA1769357215DE4FAC081bf1f309aDC325306" # Chainlink ETH/USD on Sepolia
    CONTRACT_ABI_PATH="config/abi/AggregatorV3Interface.abi.json" # Relative path to ABI file
    EOF
    ```

    **IMPORTANT:** The `.env` file is ignored by Git (see `.gitignore`). **Never commit sensitive credentials.**

3.  **Install Go Tools (Optional):**
    If you don't have `migrate`, `swag`, or `air` installed globally, you can install them using Make:

    ```bash
    make install-migrate
    make install-swag
    make install-air
    ```

    _(Ensure `$GOPATH/bin` or `$HOME/go/bin` is in your system's PATH)_

4.  **Build Docker Images:**
    This builds the API image based on the `Dockerfile`.

    ```bash
    make docker-build
    ```

5.  **Start Services:**
    This starts the `db` and `api` containers in detached mode using Docker Compose.

    ```bash
    make docker-up
    ```

    The API container will wait for the database to be healthy before starting.

6.  **Run Initial Database Migrations:**
    Once the containers are up and running (check with `make docker-logs`), apply the database schema migrations. This command executes `migrate` inside the running `api` container.
    ```bash
    make docker-migrate-up
    ```

## Running the Application (Development)

The primary way to run the application during development is using Docker Compose with hot-reloading enabled:

```bash
make docker-up
```

- This starts the PostgreSQL database container.
- This starts the API container using the `builder` stage image (which includes Go tools).
- The API container runs an entrypoint script that first applies database migrations automatically.
- After migrations, `air` starts, watching for file changes in the mounted source code (`.go`, `.toml`, etc.).
- When you save a file, `air` automatically rebuilds and restarts the Go application inside the container.

**Accessing the API:**
By default, the API is available at `http://localhost:8080` (or the port specified by `SERVER_PORT_HOST` in your `.env` file).

**Accessing Swagger UI:**
API documentation is available at `http://localhost:8080/swagger/index.html` (adjust port if needed).

## Makefile Commands

The `Makefile` provides convenient shortcuts for common tasks:

- `make help`: Display available commands.

- **Development:**

  - `make dev`: (Alias for `make docker-up`) Starts services with hot-reloading.
  - `make swagger-gen`: Manually regenerate Swagger documentation files (`/docs`).
  - `make test`: Run all Go unit and integration tests.

- **Docker:**

  - `make docker-build`: Build/rebuild Docker images.
  - `make docker-build-nocache`: Build or rebuild the docker images using docker-compose without cache.
  - `make docker-up`: Start services in detached mode.
  - `make docker-down`: Stop and remove containers and associated volumes (including DB data!).
  - `make docker-stop`: Stop running containers without removing them.
  - `make docker-logs`: Follow logs from all services.
  - `make docker-logs-api`: Follow logs from the `api` service only.
  - `make docker-logs-db`: Follow logs from the `db` service only.
  - `make docker-exec-api CMD="..."`: Execute a command inside the running `api` container (e.g., `make docker-exec-api CMD="ls -l"`).
  - `make docker-db-reset`: !! Drops and recreates the database in the Docker container (DATA LOSS!) !!

- **Database Migrations (Docker):**

  - `make docker-migrate-up`: Apply pending migrations inside the `api` container. (Also runs automatically on `make docker-up`).
  - `make docker-migrate-down`: Revert the last applied migration inside the `api` container.
  - `make docker-migrate-status`: Check migration status inside the `api` container.
  - `make docker-migrate-force VERSION=...`: Force migration version inside the api container (use with extreme caution!).

- **Database Migrations (Local - requires `migrate` CLI and DB access from host):**

  - `make migrate-create NAME="..."`: Create new up/down migration SQL files.
  - `make migrate-up`: Apply pending migrations using `DATABASE_URL` from `.env`/environment.
  - `make migrate-down`: Revert the last migration using `DATABASE_URL`.
  - `make migrate-status`: Check migration status using `DATABASE_URL`.
  - `make migrate-down-all`: Revert all migrations (use with caution!).
  - `make migrate-force VERSION=...`: Force migration version (use with caution).

- **Tool Installation:**

  - `make install-migrate`: Install `golang-migrate` CLI.
  - `make install-swag`: Install `swag` CLI.
  - `make install-air`: Install `air` CLI.

- **Testing/Mocks:**
  - `make mocks`: Generate mocks for repositories and services using mockgen.
  - `make clean-mocks`: Remove generated mock files from internal/mocks.

## Configuration

Configuration is managed by Viper and loaded with the following priority:

1.  **Environment Variables:** Highest priority. Specific variables like `SERVER_PORT`, `DB_HOST`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `CORS_ALLOWED_ORIGINS` (comma-separated) override all other settings. Docker Compose sets these for the containers.
2.  **`.env` File:** Variables defined in the `.env` file in the project root are loaded by Make and Docker Compose. Useful for setting defaults for Compose and local Make commands.
3.  **`config.yaml` File:** Configuration file (optional). Can provide base settings. Copied into the Docker image if present.
4.  **Default Values:** Set within `config/config.go`.

## API Documentation (Swagger)

- Add Swagger annotations (`@Summary`, `@Description`, `@Param`, `@Success`, etc.) above your Gin handler functions.
- Run `make swagger-gen` or `make dev` to generate/update the documentation files in the `/docs` directory.
- Access the interactive Swagger UI at `/swagger/index.html` when the application is running.

## Database Migrations

Migrations are handled using `golang-migrate` and plain SQL files located in `internal/database/migrations`.

1.  **Create a new migration:**
    ```bash
    make migrate-create NAME=your_descriptive_migration_name
    ```
2.  **Edit SQL Files:** Add your schema changes to the generated `.up.sql` file and the corresponding rollback logic to the `.down.sql` file.
3.  **Apply Migrations:**
    - **Docker (Recommended):** Migrations run automatically when starting containers with `make docker-up`. To apply manually while containers are running: `make docker-migrate-up`.
    - **Local:** Ensure `DATABASE_URL` in `.env` points to your target database and run `make migrate-up`.
4.  **Check Status / Rollback:** Use `make docker-migrate-status` or `make docker-migrate-down`.
